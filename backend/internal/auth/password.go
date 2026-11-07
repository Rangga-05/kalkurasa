// Package auth menangani login, token sesi (JWT HS256), dan otorisasi peran.
// Seluruh state sesi ada di token dan database; tidak ada state in-memory
// yang penting (cocok untuk serverless).
package auth

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Peran pengguna (sesuai kolom users.role).
const (
	RoleOwner    = "owner"
	RoleManager  = "manager"
	RoleStaff    = "staff"
	RoleAdmin    = "admin"
	RoleCustomer = "customer"
)

// Roles adalah daftar peran yang valid.
func Roles() []string {
	return []string{RoleOwner, RoleManager, RoleStaff, RoleAdmin, RoleCustomer}
}

// User adalah pengguna tanpa data rahasia.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Contact   string    `json:"contact"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// HashPassword menghasilkan hash bcrypt. Kata sandi lebih dari 72 byte ditolak
// (batas bcrypt) agar tidak terpotong diam-diam.
func HashPassword(password string) (string, error) {
	if len(password) > 72 {
		return "", errors.New("kata sandi lebih dari 72 byte")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword membandingkan kata sandi dengan hash bcrypt.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

var (
	dummyOnce sync.Once
	dummyHash string
)

// burnPasswordCheck menjalankan satu perbandingan bcrypt palsu agar waktu
// respons login untuk email yang tidak ada setara dengan email yang ada,
// sehingga keberadaan akun tidak bocor lewat waktu respons.
func burnPasswordCheck(password string) {
	dummyOnce.Do(func() {
		b, _ := bcrypt.GenerateFromPassword([]byte("kalkurasa-dummy-password"), bcrypt.DefaultCost)
		dummyHash = string(b)
	})
	_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
}

