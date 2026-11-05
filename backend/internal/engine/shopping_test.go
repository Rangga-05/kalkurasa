package engine

import "testing"

func TestTC05_FlourShopping(t *testing.T) {
	// 6 tart × 0,4 + 100 box × 0,12 = 14,4 kg.
	req := d("6").Mul(d("0.4")).Add(d("100").Mul(d("0.12")))
	assertExact(t, "kebutuhan", req, d("14.4"))

	buffer, stock := d("0.10"), d("3")

	tests := []struct {
		name     string
		rounding string
		packSize string
		wantNeed string
		wantBuy  string
	}{
		{"loose", RoundingLoose, "1", "12.84", "13"},
		{"pack 25", RoundingPack, "25", "12.84", "25"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			need, buy := ShoppingNeed(req, stock, buffer, d(tc.packSize), tc.rounding)
			assertExact(t, "perlu beli (need)", need, d(tc.wantNeed))
			assertExact(t, "dibeli (buy)", buy, d(tc.wantBuy))
		})
	}

	t.Run("kebutuhan dengan buffer", func(t *testing.T) {
		assertExact(t, "dengan buffer", req.Mul(d("1.10")), d("15.84"))
	})
}

func TestShoppingNeedRounding(t *testing.T) {
	tests := []struct {
		name                  string
		req, stock, buf, pack string
		rounding              string
		wantNeed, wantBuy     string
	}{
		{"loose naik ke bilangan bulat", "10", "0", "0", "1", RoundingLoose, "10", "10"},
		{"loose pecahan kecil dibulatkan naik", "10", "0", "0.001", "1", RoundingLoose, "10.01", "11"},
		{"loose tepat bulat tidak naik", "5", "1", "0", "1", RoundingLoose, "4", "4"},
		{"pack tepat kelipatan", "50", "0", "0", "25", RoundingPack, "50", "50"},
		{"pack lewat kelipatan naik satu kemasan", "50.01", "0", "0", "25", RoundingPack, "50.01", "75"},
		{"pack kebutuhan kecil tetap satu kemasan", "0.5", "0", "0", "25", RoundingPack, "0.5", "25"},
		{"stok cukup → 0", "5", "10", "0.10", "25", RoundingPack, "0", "0"},
		{"stok persis menutup → 0", "10", "11", "0.10", "1", RoundingLoose, "0", "0"},
		{"pack dengan packSize 0 → loose", "3.2", "0", "0", "0", RoundingPack, "3.2", "4"},
		{"rounding tak dikenal → loose", "3.2", "0", "0", "25", "lainnya", "3.2", "4"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			need, buy := ShoppingNeed(d(tc.req), d(tc.stock), d(tc.buf), d(tc.pack), tc.rounding)
			assertExact(t, "need", need, d(tc.wantNeed))
			assertExact(t, "buy", buy, d(tc.wantBuy))
		})
	}
}
