package engine

import (
	"testing"
	"time"
)

// day membuat tanggal kalender (UTC) dari "YYYY-MM-DD".
func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func ev(date, amount string) CashEvent { return CashEvent{Date: day(date), Amount: d(amount)} }

// Garis waktu TC-04: H-2 = 2026-11-12, H-1 = 11-13, H0 = 11-14 (hari acara), H+1 = 11-15.
const (
	hMinus2 = "2026-11-12"
	hMinus1 = "2026-11-13"
	h0      = "2026-11-14"
	hPlus1  = "2026-11-15"
)

func balancesOf(p CashProjection) []string {
	out := make([]string, len(p.Days))
	for i, b := range p.Days {
		out[i] = b.Balance.String()
	}
	return out
}

func TestTC04_CashModel(t *testing.T) {
	opening, fixed, buffer := d("1800000"), d("400000"), d("400000")
	total, dpPct := d("2375000"), d("0.30")

	// Peristiwa pesanan lain dan pesanan ini TANPA DP:
	// belanja −1.300.000 di H-2, DP pesanan lain +500.000 di H-1,
	// pelunasan penuh 2.375.000 masuk H+1.
	noDP := []CashEvent{
		ev(hMinus2, "-1300000"),
		ev(hMinus1, "500000"),
		ev(hPlus1, "2375000"),
	}
	projNoDP := ProjectCash(opening, day(hMinus2), 4, noDP, fixed)

	t.Run("titik terendah tanpa DP", func(t *testing.T) {
		t.Logf("saldo harian tanpa DP: %v", balancesOf(projNoDP))
		assertExact(t, "titik terendah", projNoDP.Lowest, d("-200000"))
		assertString(t, "tanggal terendah", DateKey(projNoDP.LowestDate), h0)
	})

	minDP := MinDP(buffer, projNoDP.Lowest)
	t.Run("DP minimal", func(t *testing.T) {
		assertExact(t, "MinDP", minDP, d("600000"))
	})

	charged := DPCharged(dpPct, total, minDP)
	t.Run("DP ditagih", func(t *testing.T) {
		assertExact(t, "DPCharged", charged, d("712500"))
	})

	t.Run("titik terendah dengan DP dan G3 lolos", func(t *testing.T) {
		withDP := []CashEvent{
			ev(hMinus2, "-1300000"),
			ev(hMinus2, charged.String()), // DP diterima di H-2
			ev(hMinus1, "500000"),
			ev(hPlus1, total.Sub(charged).String()), // pelunasan = sisa
		}
		proj := ProjectCash(opening, day(hMinus2), 4, withDP, fixed)
		t.Logf("saldo harian dengan DP: %v", balancesOf(proj))
		assertExact(t, "titik terendah dg DP", proj.Lowest, d("512500"))
		assertString(t, "tanggal terendah", DateKey(proj.LowestDate), h0)

		g3 := proj.Lowest.GreaterThanOrEqual(buffer)
		t.Logf("%-28s dihasilkan=%v diharapkan=true", "G3 lolos", g3)
		if !g3 {
			t.Errorf("G3 seharusnya lolos: %s >= %s", proj.Lowest, buffer)
		}
	})
}

func TestProjectCashSchedule(t *testing.T) {
	t.Run("biaya tetap berlaku di hari mulai", func(t *testing.T) {
		p := ProjectCash(d("1000"), day("2026-11-12"), 3, nil, d("100"))
		if got := balancesOf(p); got[0] != "900" || got[1] != "800" || got[2] != "700" {
			t.Errorf("saldo harian = %v, diharapkan [900 800 700]", got)
		}
		assertExact(t, "titik terendah", p.Lowest, d("700"))
		assertString(t, "tanggal terendah", DateKey(p.LowestDate), "2026-11-14")
	})
	t.Run("peristiwa di luar jendela diabaikan", func(t *testing.T) {
		p := ProjectCash(d("1000"), day("2026-11-12"), 2, []CashEvent{ev("2026-11-11", "-500"), ev("2026-11-14", "-500")}, d("0"))
		assertExact(t, "titik terendah", p.Lowest, d("1000"))
	})
	t.Run("tanggal sama dijumlahkan", func(t *testing.T) {
		p := ProjectCash(d("0"), day("2026-11-12"), 1, []CashEvent{ev("2026-11-12", "300"), ev("2026-11-12", "-100")}, d("0"))
		assertExact(t, "saldo", p.Days[0].Balance, d("200"))
	})
	t.Run("seri memilih tanggal paling awal", func(t *testing.T) {
		p := ProjectCash(d("100"), day("2026-11-12"), 3, []CashEvent{ev("2026-11-13", "100")}, d("100"))
		// saldo: 0, 0, −100 → terendah −100 di hari ke-3
		assertString(t, "tanggal terendah", DateKey(p.LowestDate), "2026-11-14")
		q := ProjectCash(d("100"), day("2026-11-12"), 3, []CashEvent{ev("2026-11-13", "100"), ev("2026-11-14", "100")}, d("100"))
		// saldo: 0, 0, 0 → seri, ambil yang paling awal
		assertString(t, "tanggal seri", DateKey(q.LowestDate), "2026-11-12")
	})
	t.Run("zona waktu tidak menggeser tanggal kalender", func(t *testing.T) {
		wib := time.FixedZone("WIB", 7*3600)
		e := CashEvent{Date: time.Date(2026, 11, 13, 1, 0, 0, 0, wib), Amount: d("500")}
		p := ProjectCash(d("0"), day("2026-11-12"), 3, []CashEvent{e}, d("0"))
		assertExact(t, "saldo 13 Nov", p.Days[1].Balance, d("500"))
	})
	t.Run("days nol", func(t *testing.T) {
		p := ProjectCash(d("1000"), day("2026-11-12"), 0, nil, d("100"))
		assertExact(t, "titik terendah", p.Lowest, d("1000"))
		if len(p.Days) != 0 {
			t.Errorf("Days harus kosong, dapat %d", len(p.Days))
		}
	})
}

func TestMinDPAndDPCharged(t *testing.T) {
	tests := []struct {
		name            string
		buffer, lowest  string
		pct, total      string
		wantMin, wantDP string
	}{
		{"kas aman → DP minimal 0", "400000", "900000", "0.30", "1000000", "0", "300000"},
		{"kas tepat buffer → 0", "400000", "400000", "0.30", "1000000", "0", "300000"},
		{"DP minimal mengalahkan persen", "400000", "-200000", "0.30", "2375000", "600000", "712500"},
		{"DP minimal lebih besar dari persen", "400000", "-1000000", "0.30", "2000000", "1400000", "1400000"},
		{"DP minimal melebihi total pesanan", "400000", "-400000", "0.30", "500000", "800000", "800000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			minDP := MinDP(d(tc.buffer), d(tc.lowest))
			assertExact(t, "MinDP", minDP, d(tc.wantMin))
			assertExact(t, "DPCharged", DPCharged(d(tc.pct), d(tc.total), minDP), d(tc.wantDP))
		})
	}

	t.Run("MinDP > total → G3 gagal tanpa perbaikan", func(t *testing.T) {
		total := d("500000")
		minDP := MinDP(d("400000"), d("-400000"))
		if !minDP.GreaterThan(total) {
			t.Fatalf("prasyarat: MinDP %s harus > total %s", minDP, total)
		}
		assertExact(t, "DPCharged", DPCharged(d("0.30"), total, minDP), d("800000"))
	})
}
