package engine

import "testing"

func TestTC03_NegotiatedPrice(t *testing.T) {
	price, cost, marginMin, normal := d("21000"), d("19000"), d("0.20"), d("25000")

	minPrice := MinPrice(cost, marginMin)
	margin := Margin(price, cost)

	assertRounded(t, "margin", margin, d("0.0952"), 4)
	assertExact(t, "harga minimal", minPrice, d("23750"))
	assertExact(t, "diskon maksimum", MaxDiscount(normal, minPrice), d("0.05"))

	// G2 gagal: margin < margin minimum.
	t.Logf("%-28s dihasilkan=%v diharapkan=false", "G2 lolos", margin.GreaterThanOrEqual(marginMin))
	if margin.GreaterThanOrEqual(marginMin) {
		t.Errorf("G2 seharusnya gagal: margin %s < %s", margin, marginMin)
	}
}

func TestMarginAtNormalPrice(t *testing.T) {
	// Harga normal 25.000, biaya 19.000 → margin 24% (contoh respons Bagian 9).
	assertExact(t, "margin normal", Margin(d("25000"), d("19000")), d("0.24"))
}

func TestUnitCost(t *testing.T) {
	// Snack box: bahan 10.500 (waste 5%) + kemasan 2.000 + 12 menit × 400 + overhead 1.200.
	// 10.500×1,05 = 11.025; +2.000 = 13.025; +4.800 = 17.825; +1.200 = 19.025.
	got := UnitCost(CostInput{
		IngredientCost: d("10500"),
		WastePct:       d("0.05"),
		PackagingCost:  d("2000"),
		LaborMinutes:   d("12"),
		LaborRate:      d("400"),
		OverheadCost:   d("1200"),
	})
	assertExact(t, "biaya per unit", got, d("19025"))
}

func TestCostingGuards(t *testing.T) {
	assertExact(t, "margin harga 0", Margin(d("0"), d("100")), d("0"))
	assertExact(t, "harga minimal margin 100%", MinPrice(d("100"), d("1")), d("0"))
	assertExact(t, "diskon saat min > normal", MaxDiscount(d("20000"), d("23750")), d("0"))
}
