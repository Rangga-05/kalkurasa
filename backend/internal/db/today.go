package db

import (
	"context"
	"time"
	_ "time/tzdata" // embed timezone data for environments without system tzdata

	"github.com/jackc/pgx/v5/pgxpool"
)

var jakarta *time.Location

func init() {
	var err error
	jakarta, err = time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// With time/tzdata imported, this should never fail.
		jakarta = time.FixedZone("WIB", 7*60*60)
	}
}

// Today returns the current date in Asia/Jakarta timezone.
// If the setting "demo_today" exists and contains a valid date string,
// that date is used instead (for demo time-travel).
//
// Today is called lazily — it is safe to call even if the settings table
// or the demo_today row does not exist. Errors are silently ignored and
// the real date is returned.
func Today(ctx context.Context, pool *pgxpool.Pool) time.Time {
	if pool != nil {
		var raw *string
		err := pool.QueryRow(ctx,
			`SELECT value#>>'{}'  FROM settings WHERE key = 'demo_today'`,
		).Scan(&raw)
		if err == nil && raw != nil && *raw != "" && *raw != "null" {
			if parsed, parseErr := time.ParseInLocation("2006-01-02", *raw, jakarta); parseErr == nil {
				return parsed
			}
		}
	}

	return time.Now().In(jakarta).Truncate(24 * time.Hour)
}

