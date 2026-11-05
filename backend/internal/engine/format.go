package engine

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Pemformat teks untuk alasan rekomendasi yang terbaca manusia (Bahasa
// Indonesia). Hanya dipakai untuk tampilan; tidak memengaruhi perhitungan.

// fmtPct: 1,5546 → "155,5%".
func fmtPct(r decimal.Decimal) string {
	return strings.Replace(r.Mul(decimal.NewFromInt(100)).StringFixed(1), ".", ",", 1) + "%"
}

// fmtRp: 712500 → "Rp 712.500"; -200000 → "-Rp 200.000".
func fmtRp(v decimal.Decimal) string {
	r := v.Round(0)
	neg := r.IsNegative()
	digits := r.Abs().String()

	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-Rp " + b.String()
	}
	return "Rp " + b.String()
}

// fmtDate: 2026-11-14 → "14 Nov".
func fmtDate(t time.Time) string {
	months := [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	return strings.TrimSpace(strings.Join([]string{itoa(t.Day()), months[t.Month()-1]}, " "))
}

func itoa(n int) string { return decimal.NewFromInt(int64(n)).String() }

func resourceLabel(code string) string {
	switch code {
	case "labor":
		return "tenaga"
	case "oven":
		return "oven"
	default:
		return code
	}
}

