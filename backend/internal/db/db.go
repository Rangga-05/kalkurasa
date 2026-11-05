package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates a pgxpool configured for Supabase Transaction pooler.
// Transaction pooler (port 6543) does not support prepared statements,
// so we must use QueryExecModeSimpleProtocol.
//
// Error messages returned from this function never contain the DSN or password.
func New(ctx context.Context, databaseURL string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// ParseConfig errors may echo the DSN — return a safe message.
		return nil, fmt.Errorf("db: invalid DATABASE_URL format")
	}

	cfg.MaxConns = maxConns

	// Force simple protocol — required for Supabase Transaction pooler.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		// Connect errors may contain the hostname but not the password.
		// Still wrap generically to be safe.
		return nil, fmt.Errorf("db: could not connect to database (check DATABASE_URL and network)")
	}

	return pool, nil
}
