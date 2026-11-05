// Package engine berisi seluruh rumus bisnis Kalkurasa sebagai fungsi murni:
// tanpa akses database, tanpa time.Now(), tanpa state global. Semua input
// lewat parameter dan semua nilai uang/rasio memakai decimal.Decimal.
package engine

import "github.com/shopspring/decimal"

// divPrecision adalah jumlah digit desimal untuk pembagian, dibuat eksplisit
// agar hasil deterministik dan tidak bergantung pada decimal.DivisionPrecision.
const divPrecision = 18

// Kode status hari (BR-04).
const (
	StatusGreen  = "green"
	StatusYellow = "yellow"
	StatusRed    = "red"
	StatusFull   = "full"
)

// Aturan pembulatan belanja bahan (ingredients.rounding).
const (
	RoundingLoose = "loose"
	RoundingPack  = "pack"
)

// Resource adalah sumber daya produksi (oven, tenaga kerja).
type Resource struct {
	Code             string
	Count            int
	AmountPerUnitDay decimal.Decimal // jam (oven) atau menit (tenaga) per unit per hari
	Efficiency       decimal.Decimal // 0 < efisiensi <= 1
}

// LoadItem adalah satu kontribusi beban pada satu sumber daya:
// Qty × Load. Beban yang sudah ada dapat dinyatakan sebagai Qty=1.
type LoadItem struct {
	ResourceCode string
	Qty          decimal.Decimal
	Load         decimal.Decimal
}

// CostInput adalah komponen biaya per satu unit produk (BR-05).
type CostInput struct {
	IngredientCost decimal.Decimal // biaya bahan per unit sebelum waste
	WastePct       decimal.Decimal // mis. 0,05
	PackagingCost  decimal.Decimal
	LaborMinutes   decimal.Decimal
	LaborRate      decimal.Decimal // rupiah per menit
	OverheadCost   decimal.Decimal
}

