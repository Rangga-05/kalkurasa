// Package orders mengimplementasikan alur pesanan inti (Bagian 8 dan 9):
// pengecekan ketersediaan tanggal, pembuatan pesanan transaksional dengan
// penguncian baris day_locks, evaluasi gerbang G1..G4 via internal/engine,
// pelacakan pesanan publik, dan pembatasan laju berbasis database.
package orders

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// base32Alphabet berisi 32 karakter tanpa karakter ambigu (0, O, 1, I).
const (
	base32Alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	OrderCodeLen   = 10
)

// GenerateCode membuat kode pesanan acak sepanjang 10 karakter menggunakan crypto/rand.
func GenerateCode() (string, error) {
	b := make([]byte, OrderCodeLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("orders: generate code: %w", err)
	}
	out := make([]byte, OrderCodeLen)
	for i, v := range b {
		out[i] = base32Alphabet[int(v)&31]
	}
	return string(out), nil
}

// NormalizeCode merapikan kode pesanan menjadi huruf besar dan memeriksa panjang serta karakter alfanumerik (A-Z, 0-9).
func NormalizeCode(raw string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if len(s) != OrderCodeLen {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return "", false
		}
	}
	return s, true
}

