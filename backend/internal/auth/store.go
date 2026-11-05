package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
)

var (
	// ErrNotFound berarti pengguna tidak ada.
	ErrNotFound = errors.New("pengguna tidak ditemukan")
	// ErrUnavailable berarti database belum dikonfigurasi atau tidak terjangkau.
	ErrUnavailable = errors.New("database tidak tersedia")
)

// UserWithHash adalah pengguna beserta hash kata sandi (hanya untuk login).
type UserWithHash struct {
	User
	PasswordHash string
}

// UserStore adalah akses data pengguna untuk autentikasi. Dibuat sebagai
// antarmuka agar login dan middleware dapat diuji tanpa database.
type UserStore interface {
	FindByEmail(ctx context.Context, email string) (*UserWithHash, error)
	FindByID(ctx context.Context, id string) (*User, error)
}

// PGStore mengimplementasikan UserStore di atas PostgreSQL.
type PGStore struct{ DB db.DBTX }

const userCols = `id::text, email, name, role, is_active, created_at`

// FindByEmail mencari pengguna menurut email (huruf kecil).
func (s PGStore) FindByEmail(ctx context.Context, email string) (*UserWithHash, error) {
	var u UserWithHash
	err := s.DB.QueryRow(ctx,
		`SELECT `+userCols+`, password_hash FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: find by email: %w", err)
	}
	return &u, nil
}

// FindByID mencari pengguna menurut ID.
func (s PGStore) FindByID(ctx context.Context, id string) (*User, error) {
	if !httpx.ValidUUID(id) {
		return nil, ErrNotFound
	}
	var u User
	err := s.DB.QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE id = $1::uuid`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.IsActive, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("auth: find by id: %w", err)
	}
	return &u, nil
}

// UnavailableStore dipakai saat DATABASE_URL tidak diatur.
type UnavailableStore struct{}

// FindByEmail selalu mengembalikan ErrUnavailable.
func (UnavailableStore) FindByEmail(context.Context, string) (*UserWithHash, error) {
	return nil, ErrUnavailable
}

// FindByID selalu mengembalikan ErrUnavailable.
func (UnavailableStore) FindByID(context.Context, string) (*User, error) {
	return nil, ErrUnavailable
}

