package engine

import (
	"testing"

	"github.com/shopspring/decimal"
)

// d membuat decimal dari string literal agar tidak ada galat float.
func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// assertExact membandingkan nilai persis dan mencatat hasil vs harapan.
func assertExact(t *testing.T, label string, got, want decimal.Decimal) {
	t.Helper()
	t.Logf("%-28s dihasilkan=%s diharapkan=%s", label, got.String(), want.String())
	if !got.Equal(want) {
		t.Errorf("%s: dihasilkan %s, diharapkan %s", label, got, want)
	}
}

// assertRounded membandingkan nilai setelah dibulatkan ke places desimal
// (untuk rasio yang berekspansi tak hingga, mis. 1,5546...).
func assertRounded(t *testing.T, label string, got, want decimal.Decimal, places int32) {
	t.Helper()
	t.Logf("%-28s dihasilkan=%s (dibulatkan %s) diharapkan=%s", label, got.String(), got.Round(places).String(), want.String())
	if !got.Round(places).Equal(want) {
		t.Errorf("%s: dihasilkan %s (dibulatkan %s), diharapkan %s", label, got, got.Round(places), want)
	}
}

func assertString(t *testing.T, label, got, want string) {
	t.Helper()
	t.Logf("%-28s dihasilkan=%s diharapkan=%s", label, got, want)
	if got != want {
		t.Errorf("%s: dihasilkan %q, diharapkan %q", label, got, want)
	}
}

// Sumber daya awal dari Bagian 13 spesifikasi.
var (
	testLabor = Resource{Code: "labor", Count: 4, AmountPerUnitDay: d("420"), Efficiency: d("0.85")}
	testOven  = Resource{Code: "oven", Count: 2, AmountPerUnitDay: d("8"), Efficiency: d("0.85")}
)

func testCaps() map[string]decimal.Decimal {
	return map[string]decimal.Decimal{
		"labor": Capacity(testLabor),
		"oven":  Capacity(testOven),
	}
}

