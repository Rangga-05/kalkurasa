package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX adalah subset operasi yang dimiliki bersama oleh *pgxpool.Pool dan
// pgx.Tx, sehingga helper dapat dipakai di dalam maupun di luar transaksi.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Conn adalah koneksi yang dapat memulai transaksi (*pgxpool.Pool).
type Conn interface {
	DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// WithTx menjalankan fn di dalam satu transaksi. Transaksi di-commit bila fn
// mengembalikan nil dan di-rollback bila sebaliknya.
func WithTx(ctx context.Context, c Conn, fn func(tx pgx.Tx) error) error {
	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// Kode error PostgreSQL yang ditangani secara khusus.
const (
	codeUniqueViolation = "23505"
	codeForeignKey      = "23503"
	codeCheckViolation  = "23514"
)

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// IsUniqueViolation melaporkan pelanggaran constraint unik.
func IsUniqueViolation(err error) bool { return pgCode(err) == codeUniqueViolation }

// IsForeignKeyViolation melaporkan pelanggaran foreign key.
func IsForeignKeyViolation(err error) bool { return pgCode(err) == codeForeignKey }

// IsCheckViolation melaporkan pelanggaran constraint check.
func IsCheckViolation(err error) bool { return pgCode(err) == codeCheckViolation }

