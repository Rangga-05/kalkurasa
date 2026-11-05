package engine

import (
	"time"

	"github.com/shopspring/decimal"
)

// ProjectCash memproyeksikan saldo harian selama days hari mulai dari start,
// termasuk hari mulai (BR-08):
//
//	saldo(t) = saldo(t−1) + Σ peristiwa hari t − biaya tetap harian
//
// saldo(−1) = opening. Biaya tetap berlaku setiap hari termasuk hari mulai.
// Peristiwa di luar jendela [start, start+days) diabaikan. Titik terendah
// adalah saldo harian minimum; bila seri, tanggal paling awal. Untuk days <= 0
// proyeksi kosong dengan Lowest = opening.
func ProjectCash(opening decimal.Decimal, start time.Time, days int, events []CashEvent, fixedDaily decimal.Decimal) CashProjection {
	proj := CashProjection{Lowest: opening, LowestDate: civil(start)}
	if days <= 0 {
		return proj
	}

	byDay := make(map[string]decimal.Decimal, len(events))
	for _, e := range events {
		k := DateKey(e.Date)
		byDay[k] = byDay[k].Add(e.Amount)
	}

	balance := opening
	proj.Days = make([]DailyBalance, 0, days)
	for i := 0; i < days; i++ {
		date := addDays(start, i)
		balance = balance.Add(byDay[DateKey(date)]).Sub(fixedDaily)
		proj.Days = append(proj.Days, DailyBalance{Date: date, Balance: balance})
		if i == 0 || balance.LessThan(proj.Lowest) {
			proj.Lowest = balance
			proj.LowestDate = date
		}
	}
	return proj
}

// MinDP = max(0, buffer kas − titik terendah tanpa DP) (BR-09).
func MinDP(buffer, lowestWithoutDP decimal.Decimal) decimal.Decimal {
	m := buffer.Sub(lowestWithoutDP)
	if m.IsNegative() {
		return decimal.Zero
	}
	return m
}

// DPCharged = max(persen DP × total, DP minimal) (BR-09). Hasil dapat melebihi
// total pesanan bila DP minimal melebihi total; pemanggil (EvaluateOrder)
// menganggap kasus itu sebagai G3 gagal tanpa perbaikan.
func DPCharged(dpPct, total, minDP decimal.Decimal) decimal.Decimal {
	d := dpPct.Mul(total)
	if minDP.GreaterThan(d) {
		return minDP
	}
	return d
}

// CancellationExposure = max(0, belanja terpesan − DP diterima) (BR-11).
func CancellationExposure(shoppingCost, dpReceived decimal.Decimal) decimal.Decimal {
	e := shoppingCost.Sub(dpReceived)
	if e.IsNegative() {
		return decimal.Zero
	}
	return e
}
