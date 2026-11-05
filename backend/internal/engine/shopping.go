package engine

import "github.com/shopspring/decimal"

// ShoppingNeed menghitung kebutuhan beli bahan (BR-07).
//
//	need = max(0, req × (1 + bufferPct) − stock)
//
// buy adalah jumlah yang dibeli dalam satuan pakai:
//   - "pack": dibulatkan ke atas ke kelipatan packSize (kemasan penuh)
//   - "loose" (dan nilai lain): dibulatkan ke atas ke bilangan bulat satuan pakai
//
// Bila rounding "pack" tetapi packSize <= 0, dipakai pembulatan loose.
// need = 0 selalu menghasilkan buy = 0.
func ShoppingNeed(req, stock, bufferPct, packSize decimal.Decimal, rounding string) (need, buy decimal.Decimal) {
	need = req.Mul(decimal.NewFromInt(1).Add(bufferPct)).Sub(stock)
	if !need.IsPositive() {
		return decimal.Zero, decimal.Zero
	}

	if rounding == RoundingPack && packSize.IsPositive() {
		packs := need.DivRound(packSize, divPrecision).Ceil()
		return need, packs.Mul(packSize)
	}
	return need, need.Ceil()
}

