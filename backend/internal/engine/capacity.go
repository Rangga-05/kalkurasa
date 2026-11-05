package engine

import (
	"sort"

	"github.com/shopspring/decimal"
)

// Capacity menghitung kapasitas harian sumber daya (BR-01):
// jumlah × jumlah per unit per hari × efisiensi.
func Capacity(r Resource) decimal.Decimal {
	return decimal.NewFromInt(int64(r.Count)).
		Mul(r.AmountPerUnitDay).
		Mul(r.Efficiency)
}

// DayLoads menjumlahkan qty × beban per kode sumber daya (BR-02).
func DayLoads(items []LoadItem) map[string]decimal.Decimal {
	loads := make(map[string]decimal.Decimal)
	for _, it := range items {
		loads[it.ResourceCode] = loads[it.ResourceCode].Add(it.Qty.Mul(it.Load))
	}
	return loads
}

// Utilization = beban ÷ kapasitas (BR-03). Kapasitas <= 0 menghasilkan 0
// (tidak terjadi pada data valid: skema menjamin kapasitas > 0).
func Utilization(load, capacity decimal.Decimal) decimal.Decimal {
	if !capacity.IsPositive() {
		return decimal.Zero
	}
	return load.DivRound(capacity, divPrecision)
}

// DayUtilization mengembalikan utilisasi tertinggi antar sumber daya pada
// caps beserta kode sumber daya penghambatnya (BR-03). Bila seri, kode yang
// lebih dulu secara alfabet dipilih agar hasil deterministik. Sumber daya
// tanpa beban dihitung sebagai utilisasi 0.
func DayUtilization(loads, caps map[string]decimal.Decimal) (util decimal.Decimal, bottleneck string) {
	codes := make([]string, 0, len(caps))
	for code := range caps {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	util = decimal.Zero
	for i, code := range codes {
		u := Utilization(loads[code], caps[code])
		if i == 0 || u.GreaterThan(util) {
			util = u
			bottleneck = code
		}
	}
	return util, bottleneck
}

// DayStatus memetakan utilisasi ke status hari (BR-04):
// green < greenBelow; yellow < yellowBelow; red sampai 1,00; full di atas 1,00.
// Batas bawah bersifat inklusif untuk status berikutnya (0,70 → yellow,
// 0,90 → red). Batas 1,00 tetap dan tidak dapat diatur.
func DayStatus(util, greenBelow, yellowBelow decimal.Decimal) string {
	switch {
	case util.GreaterThan(decimal.NewFromInt(1)):
		return StatusFull
	case util.LessThan(greenBelow):
		return StatusGreen
	case util.LessThan(yellowBelow):
		return StatusYellow
	default:
		return StatusRed
	}
}
