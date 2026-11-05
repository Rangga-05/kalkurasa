package engine

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// Hari ini = Minggu 2026-11-08, sehingga Sabtu 14 Nov berjarak 6 hari
// (sama dengan contoh respons EvaluateOrder di Bagian 9 brief).
const (
	evToday    = "2026-11-08"
	evSaturday = "2026-11-14"
	evSunday   = "2026-11-15"
	evMonday   = "2026-11-16"
)

func evalParams() EvalParams {
	return EvalParams{
		CapacityLimit:    d("0.90"),
		MarginMin:        d("0.20"),
		CashBuffer:       d("400000"),
		FixedCostDaily:   d("400000"),
		OpeningCash:      d("10000000"), // kas longgar kecuali diuji lain
		DPPct:            d("0.30"),
		LeadTimeDays:     2,
		ShoppingLeadDays: 2,
	}
}

func snackBoxItem(qty int, price string) EvalItem {
	return EvalItem{
		ProductID:           "snack-box",
		Qty:                 qty,
		UnitPrice:           d(price),
		UnitCost:            d("19000"),
		PurchaseCostPerUnit: d("13000"),
		UnitLoads:           map[string]decimal.Decimal{"labor": d("12"), "oven": d("0.05")},
	}
}

func baseInput(eventDate string, price string) EvalInput {
	return EvalInput{
		Today:          day(evToday),
		EventDate:      day(eventDate),
		ProductionDate: day(eventDate),
		Items:          []EvalItem{snackBoxItem(100, price)},
		Params:         evalParams(),
		Capacities:     testCaps(),
		ExistingLoads:  map[string]map[string]decimal.Decimal{},
	}
}

func gate(t *testing.T, r EvalResult, name string) GateResult {
	t.Helper()
	for _, g := range r.Gates {
		if g.Gate == name {
			return g
		}
	}
	t.Fatalf("gerbang %s tidak ada", name)
	return GateResult{}
}

func assertBool(t *testing.T, label string, got, want bool) {
	t.Helper()
	t.Logf("%-28s dihasilkan=%v diharapkan=%v", label, got, want)
	if got != want {
		t.Errorf("%s: dihasilkan %v, diharapkan %v", label, got, want)
	}
}

func TestEvaluateOrder_TC01_G1FailsCounterNextDay(t *testing.T) {
	in := baseInput(evSaturday, "25000")
	in.ExistingLoads = map[string]map[string]decimal.Decimal{
		evSaturday: {"labor": d("1020"), "oven": d("5")},
		evSunday:   {"labor": d("60")},
	}

	r := EvaluateOrder(in)
	t.Logf("alasan: %s", r.Reason)

	assertString(t, "rekomendasi", r.Recommendation, RecAcceptWithConditions)
	assertString(t, "penghambat", r.Bottleneck, "labor")

	g1, g2, g3, g4 := gate(t, r, "G1"), gate(t, r, "G2"), gate(t, r, "G3"), gate(t, r, "G4")
	assertBool(t, "G1 lolos", g1.Passed, false)
	assertRounded(t, "G1 nilai", g1.Value, d("1.5546"), 4)
	assertExact(t, "G1 ambang", g1.Threshold, d("0.90"))
	assertBool(t, "G2 lolos", g2.Passed, true)
	assertExact(t, "G2 nilai", g2.Value, d("0.24"))
	assertBool(t, "G3 lolos", g3.Passed, true)
	assertBool(t, "G4 lolos", g4.Passed, true)
	assertExact(t, "G4 nilai (hari)", g4.Value, d("6"))

	assertRounded(t, "util sebelum", r.Impact.UtilizationBefore, d("0.7143"), 4)
	assertRounded(t, "util sesudah", r.Impact.UtilizationAfter, d("1.5546"), 4)

	if r.CounterOffer == nil {
		t.Fatal("tawaran balik seharusnya ada")
	}
	assertString(t, "tanggal tawaran", DateKey(r.CounterOffer.Date), evSunday)
	assertRounded(t, "util tawaran", r.CounterOffer.UtilizationAfter, d("0.8824"), 4)
	assertExact(t, "harga minimal item", r.CounterOffer.MinPricePerItem["snack-box"], d("23750"))
	assertExact(t, "DP tawaran", r.CounterOffer.DP, d("750000")) // 30% × 2.500.000

	for _, want := range []string{"155,5%", "15 Nov", "88,2%"} {
		if !strings.Contains(r.Reason, want) {
			t.Errorf("alasan %q harus memuat %q", r.Reason, want)
		}
	}
}

func TestEvaluateOrder_CounterSkipsFullDays(t *testing.T) {
	in := baseInput(evSaturday, "25000")
	in.ExistingLoads = map[string]map[string]decimal.Decimal{
		evSaturday: {"labor": d("1020"), "oven": d("5")},
		evSunday:   {"labor": d("1500")}, // Minggu juga penuh
	}
	r := EvaluateOrder(in)
	if r.CounterOffer == nil {
		t.Fatal("tawaran balik seharusnya ada")
	}
	assertString(t, "tanggal tawaran", DateKey(r.CounterOffer.Date), evMonday)
}

func TestEvaluateOrder_TC03_G2FailsMinPrice(t *testing.T) {
	r := EvaluateOrder(baseInput(evSaturday, "21000"))
	t.Logf("alasan: %s", r.Reason)

	g2 := gate(t, r, "G2")
	assertBool(t, "G2 lolos", g2.Passed, false)
	assertRounded(t, "G2 margin", g2.Value, d("0.0952"), 4)
	assertBool(t, "G1 lolos", gate(t, r, "G1").Passed, true)
	assertBool(t, "G3 lolos", gate(t, r, "G3").Passed, true)
	assertBool(t, "G4 lolos", gate(t, r, "G4").Passed, true)

	assertString(t, "rekomendasi", r.Recommendation, RecAcceptWithConditions)
	if r.CounterOffer == nil {
		t.Fatal("tawaran balik seharusnya ada")
	}
	assertExact(t, "harga minimal", r.CounterOffer.MinPricePerItem["snack-box"], d("23750"))
	assertString(t, "tanggal tawaran", DateKey(r.CounterOffer.Date), evSaturday) // cukup perbaikan harga
	assertExact(t, "DP tawaran", r.CounterOffer.DP, d("712500"))                 // 30% × 2.375.000
	assertExact(t, "total asli", r.Total, d("2100000"))
}

func TestEvaluateOrder_AllPass_Accept(t *testing.T) {
	r := EvaluateOrder(baseInput(evSaturday, "25000"))
	t.Logf("alasan: %s", r.Reason)

	assertString(t, "rekomendasi", r.Recommendation, RecAccept)
	if r.CounterOffer != nil {
		t.Error("accept tidak boleh punya tawaran balik")
	}
	if len(r.Gates) != 4 {
		t.Fatalf("harus 4 gerbang, dapat %d", len(r.Gates))
	}
	for _, g := range r.Gates {
		assertBool(t, g.Gate+" lolos", g.Passed, true)
	}
}

func TestEvaluateOrder_G4Fails_Reject(t *testing.T) {
	// Acara besok (1 hari) < lead time 2 hari; G1, G2, G3 lolos.
	in := baseInput("2026-11-09", "25000")
	r := EvaluateOrder(in)
	t.Logf("alasan: %s", r.Reason)

	assertBool(t, "G1 lolos", gate(t, r, "G1").Passed, true)
	assertBool(t, "G2 lolos", gate(t, r, "G2").Passed, true)
	assertBool(t, "G3 lolos", gate(t, r, "G3").Passed, true)
	g4 := gate(t, r, "G4")
	assertBool(t, "G4 lolos", g4.Passed, false)
	assertExact(t, "G4 nilai (hari)", g4.Value, d("1"))
	assertExact(t, "G4 ambang", g4.Threshold, d("2"))
	assertString(t, "rekomendasi", r.Recommendation, RecReject)
}

func TestEvaluateOrder_G3FailsNoFix_Reject(t *testing.T) {
	// Kas awal 0 dengan biaya tetap harian: DP minimal melebihi total pesanan
	// pada semua tanggal, jadi G3 gagal tanpa perbaikan (BR-09).
	in := baseInput(evSaturday, "25000")
	in.Params.OpeningCash = decimal.Zero
	r := EvaluateOrder(in)
	t.Logf("alasan: %s", r.Reason)

	assertBool(t, "G3 lolos", gate(t, r, "G3").Passed, false)
	assertBool(t, "MinDP > total", r.MinDP.GreaterThan(r.Total), true)
	assertExact(t, "DPCharged = MinDP", r.DPCharged, r.MinDP)
	assertString(t, "rekomendasi", r.Recommendation, RecReject)
	if r.CounterOffer != nil {
		t.Error("tidak boleh ada tawaran balik bila tidak ada perbaikan")
	}
}

func TestEvaluateOrder_TC04_CashGateFromScenario(t *testing.T) {
	// Skenario TC-04 melalui EvaluateOrder: 100 box @ 23.750 = 2.375.000,
	// belanja 1.300.000 di H-2 (= hari ini), saldo awal 1.800.000, biaya tetap
	// 400.000/hari, DP pesanan lain 500.000 di H-1, buffer 400.000.
	in := baseInput(h0, "23750")
	in.Today = day(hMinus2)
	in.Params.OpeningCash = d("1800000")
	in.CashEvents = []CashEvent{ev(hMinus1, "500000")}

	r := EvaluateOrder(in)
	t.Logf("alasan: %s", r.Reason)

	assertExact(t, "total", r.Total, d("2375000"))
	assertExact(t, "MinDP", r.MinDP, d("600000"))
	assertExact(t, "DPCharged", r.DPCharged, d("712500"))
	g3 := gate(t, r, "G3")
	assertExact(t, "G3 nilai", g3.Value, d("512500"))
	assertBool(t, "G3 lolos", g3.Passed, true)
	// Tanpa pesanan ini: 1,4jt → 1,5jt → 1,1jt → 0,7jt (H+1); terendah 700.000.
	assertExact(t, "kas terendah sebelum", r.Impact.CashLowestBefore, d("700000"))
	assertExact(t, "kas terendah sesudah", r.Impact.CashLowestAfter, d("512500"))
	assertString(t, "rekomendasi", r.Recommendation, RecAccept)
}

func TestEvaluateOrder_DoesNotMutateInput(t *testing.T) {
	in := baseInput(evSaturday, "21000")
	in.CashEvents = []CashEvent{ev(evSunday, "1000")}
	_ = EvaluateOrder(in)
	if len(in.CashEvents) != 1 {
		t.Errorf("CashEvents berubah: %d", len(in.CashEvents))
	}
	assertExact(t, "harga item asli", in.Items[0].UnitPrice, d("21000"))
}

func TestFormatHelpers(t *testing.T) {
	assertString(t, "fmtPct", fmtPct(d("1.5546")), "155,5%")
	assertString(t, "fmtPct 2", fmtPct(d("0.8824")), "88,2%")
	assertString(t, "fmtRp", fmtRp(d("712500")), "Rp 712.500")
	assertString(t, "fmtRp negatif", fmtRp(d("-200000")), "-Rp 200.000")
	assertString(t, "fmtRp kecil", fmtRp(d("950")), "Rp 950")
	assertString(t, "fmtRp juta", fmtRp(d("2375000")), "Rp 2.375.000")
	assertString(t, "fmtDate", fmtDate(day("2026-11-15")), "15 Nov")
}
