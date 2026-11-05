package engine

import "github.com/shopspring/decimal"

// recalibrateMinSamples berasal dari BR-12.
const recalibrateMinSamples = 5

// recalibrateThreshold mengembalikan ambang |deviasi| BR-12 (0,10). Berupa
// fungsi, bukan variabel paket, agar engine tidak memiliki state global.
func recalibrateThreshold() decimal.Decimal { return decimal.New(10, -2) }

// Average menghitung rata-rata nilai; slice kosong menghasilkan 0.
// Dipakai untuk rata-rata durasi aktual sebelum Deviation (TC-07).
func Average(values []decimal.Decimal) decimal.Decimal {
	if len(values) == 0 {
		return decimal.Zero
	}
	sum := decimal.Zero
	for _, v := range values {
		sum = sum.Add(v)
	}
	return sum.DivRound(decimal.NewFromInt(int64(len(values))), divPrecision)
}

// Deviation = (rata-rata aktual − estimasi) ÷ estimasi (BR-12).
// Estimasi <= 0 menghasilkan 0.
func Deviation(avgActual, estimate decimal.Decimal) decimal.Decimal {
	if !estimate.IsPositive() {
		return decimal.Zero
	}
	return avgActual.Sub(estimate).DivRound(estimate, divPrecision)
}

// ShouldRecalibrate bernilai true bila |deviasi| > 0,10 dan sampel >= 5 (BR-12).
func ShouldRecalibrate(dev decimal.Decimal, samples int) bool {
	return dev.Abs().GreaterThan(recalibrateThreshold()) && samples >= recalibrateMinSamples
}
