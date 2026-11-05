package engine

import "github.com/shopspring/decimal"

// UnitCost = bahan × (1 + waste) + kemasan + menit tenaga × tarif + overhead (BR-05).
func UnitCost(c CostInput) decimal.Decimal {
	one := decimal.NewFromInt(1)
	return c.IngredientCost.Mul(one.Add(c.WastePct)).
		Add(c.PackagingCost).
		Add(c.LaborMinutes.Mul(c.LaborRate)).
		Add(c.OverheadCost)
}

// Margin = (harga − biaya) ÷ harga (BR-06). Harga <= 0 menghasilkan 0.
func Margin(price, cost decimal.Decimal) decimal.Decimal {
	if !price.IsPositive() {
		return decimal.Zero
	}
	return price.Sub(cost).DivRound(price, divPrecision)
}

// MinPrice = biaya ÷ (1 − margin minimum) (BR-06). Margin minimum >= 1
// tidak punya harga yang masuk akal dan menghasilkan 0.
func MinPrice(cost, marginMin decimal.Decimal) decimal.Decimal {
	denom := decimal.NewFromInt(1).Sub(marginMin)
	if !denom.IsPositive() {
		return decimal.Zero
	}
	return cost.DivRound(denom, divPrecision)
}

// MaxDiscount = 1 − harga minimal ÷ harga normal (BR-06). Bila harga minimal
// melebihi harga normal (tidak ada ruang diskon) hasilnya dibatasi 0.
func MaxDiscount(price, minPrice decimal.Decimal) decimal.Decimal {
	if !price.IsPositive() {
		return decimal.Zero
	}
	d := decimal.NewFromInt(1).Sub(minPrice.DivRound(price, divPrecision))
	if d.IsNegative() {
		return decimal.Zero
	}
	return d
}

