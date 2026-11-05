// cmd/migrate runs SQL migration files from backend/migrations/ in order.
// It tracks applied migrations in the schema_migrations table.
// Uses DATABASE_URL_MIGRATE (session pooler / direct connection).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env if present (development only); never overwrite existing env vars.
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL_MIGRATE")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		log.Fatal("DATABASE_URL_MIGRATE (or DATABASE_URL) must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	// Ensure schema_migrations table exists
	_, err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		log.Fatalf("create schema_migrations: %v", err)
	}

	// Find migration files
	migrationsDir := findMigrationsDir()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		log.Fatalf("glob: %v", err)
	}
	sort.Strings(files)

	if len(files) == 0 {
		log.Println("No migration files found")
		return
	}

	// Get already applied migrations
	applied := make(map[string]bool)
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		log.Fatalf("query applied: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			log.Fatalf("scan: %v", err)
		}
		applied[v] = true
	}
	rows.Close()

	// Apply pending migrations
	for _, f := range files {
		version := strings.TrimSuffix(filepath.Base(f), ".sql")
		if applied[version] {
			log.Printf("SKIP %s (already applied)", version)
			continue
		}

		sql, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}

		log.Printf("APPLY %s ...", version)

		tx, err := conn.Begin(ctx)
		if err != nil {
			log.Fatalf("begin tx: %v", err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("exec %s: %v", version, err)
		}

		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			log.Fatalf("record %s: %v", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			log.Fatalf("commit %s: %v", version, err)
		}

		fmt.Printf("  ✓ %s applied\n", version)
	}

	log.Println("All migrations applied")
}

// findMigrationsDir looks for the migrations directory relative to the
// working directory. It checks both "migrations/" (when run from backend/)
// and "backend/migrations/" (when run from repo root).
func findMigrationsDir() string {
	candidates := []string{
		"migrations",
		filepath.Join("backend", "migrations"),
	}
	for _, d := range candidates {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	log.Fatal("migrations directory not found (run from backend/ or repo root)")
	return ""
}
