package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// TokenTTL adalah masa berlaku token (Bagian 6: 12 jam).
	TokenTTL = 12 * time.Hour
	// MinSecretLen adalah panjang minimal JWT_SECRET.
	MinSecretLen = 32
	// CookieName adalah nama cookie sesi.
	CookieName = "session"
)

// Claims adalah klaim token: sub (ID pengguna) dan role.
type Claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// SecretOK melaporkan apakah secret cukup panjang untuk dipakai.
func SecretOK(secret []byte) bool { return len(secret) >= MinSecretLen }

// IssueToken membuat JWT HS256. Waktu sekarang diberikan oleh pemanggil.
func IssueToken(secret []byte, userID, role string, now time.Time, ttl time.Duration) (string, time.Time, error) {
	if !SecretOK(secret) {
		return "", time.Time{}, errors.New("JWT_SECRET belum diatur atau terlalu pendek")
	}
	exp := now.Add(ttl)
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ParseToken memverifikasi tanda tangan (hanya HS256), masa berlaku, dan
// keberadaan sub. Algoritma lain, termasuk "none", ditolak.
func ParseToken(secret []byte, token string, now time.Time) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return nil, err
	}
	if claims.Subject == "" {
		return nil, errors.New("token tanpa subjek")
	}
	return claims, nil
}

