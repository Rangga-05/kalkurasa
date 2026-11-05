package engine

import "github.com/shopspring/decimal"

// CancellationExposure = max(0, belanja terpesan − DP diterima) (BR-11).
func CancellationExposure(shoppingCost, dpReceived decimal.Decimal) decimal.Decimal {
	e := shoppingCost.Sub(dpReceived)
	if e.IsNegative() {
		return decimal.Zero
	}
	return e
}

