// cmd/seed mengisi data awal yang idempoten untuk Tahap 2:
// - 4 pengguna demo (@demo.test) dengan kata sandi dari env SEED_DEMO_PASSWORD
// - 2 sumber daya (oven dan labor)
// - 17 parameter settings sesuai Bagian 5 brief
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"kalkurasa/backend/internal/auth"
)

func main() {
	// Muat .env bila ada (baik dijalankan dari backend/ maupun root repo), tanpa menimpa env yang sudah ada.
	_ = godotenv.Load(".env")
	_ = godotenv.Load("backend/.env")

	dsn := os.Getenv("DATABASE_URL_MIGRATE")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		log.Fatal("DATABASE_URL_MIGRATE atau DATABASE_URL wajib diatur")
	}

	demoPassword := os.Getenv("SEED_DEMO_PASSWORD")
	if len(demoPassword) < 8 {
		log.Fatal("SEED_DEMO_PASSWORD wajib diatur (minimal 8 karakter)")
	}

	pwHash, err := auth.HashPassword(demoPassword)
	if err != nil {
		log.Fatalf("hash SEED_DEMO_PASSWORD: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Seed 4 pengguna demo (idempoten pada email)
	demoUsers := []struct {
		email string
		name  string
		role  string
	}{
		{email: "owner@demo.test", name: "Bu Sari (Pemilik)", role: auth.RoleOwner},
		{email: "manager@demo.test", name: "Rina (Manajer Operasional)", role: auth.RoleManager},
		{email: "staff@demo.test", name: "Budi (Karyawan Produksi)", role: auth.RoleStaff},
		{email: "admin@demo.test", name: "Admin Sistem", role: auth.RoleAdmin},
	}
	for _, u := range demoUsers {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (email, name, role, password_hash, is_active)
			VALUES ($1, $2, $3, $4, true)
			ON CONFLICT (email) DO UPDATE SET
				name = EXCLUDED.name,
				role = EXCLUDED.role,
				password_hash = EXCLUDED.password_hash,
				is_active = true
		`, u.email, u.name, u.role, pwHash)
		if err != nil {
			log.Fatalf("seed user %s: %v", u.email, err)
		}
	}

	// 2. Seed 2 sumber daya (idempoten pada code)
	resources := []struct {
		code             string
		name             string
		unit             string
		count            int
		amountPerUnitDay string
		efficiency       string
	}{
		{code: "oven", name: "Oven Deck", unit: "oven_hour", count: 2, amountPerUnitDay: "8.0000", efficiency: "0.8500"},
		{code: "labor", name: "Tenaga Produksi", unit: "minute", count: 4, amountPerUnitDay: "420.0000", efficiency: "0.8500"},
	}
	for _, r := range resources {
		_, err := tx.Exec(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				unit = EXCLUDED.unit,
				count = EXCLUDED.count,
				amount_per_unit_day = EXCLUDED.amount_per_unit_day,
				efficiency = EXCLUDED.efficiency
		`, r.code, r.name, r.unit, r.count, r.amountPerUnitDay, r.efficiency)
		if err != nil {
			log.Fatalf("seed resource %s: %v", r.code, err)
		}
	}

	// 3. Seed 17 parameter settings sesuai Bagian 5 brief (idempoten pada key)
	settingsSeed := []struct {
		key      string
		jsonVal  string
	}{
		{key: "capacity_limit", jsonVal: "0.90"},
		{key: "status_green_below", jsonVal: "0.70"},
		{key: "status_yellow_below", jsonVal: "0.90"},
		{key: "margin_min", jsonVal: "0.20"},
		{key: "labor_rate_per_minute", jsonVal: "400"},
		{key: "waste_pct", jsonVal: "0.05"},
		{key: "shopping_buffer_pct", jsonVal: "0.10"},
		{key: "cash_buffer", jsonVal: "400000"},
		{key: "fixed_cost_daily", jsonVal: "400000"},
		{key: "opening_cash", jsonVal: "1800000"},
		{key: "lead_time_days", jsonVal: "2"},
		{key: "dp_pct", jsonVal: "0.30"},
		{key: "dp_deadline_hours", jsonVal: "24"},
		{key: "big_order_total", jsonVal: "2000000"},
		{key: "big_order_units", jsonVal: "50"},
		{key: "shopping_lead_days", jsonVal: "2"},
		{key: "demo_today", jsonVal: "null"},
	}
	for _, s := range settingsSeed {
		_, err := tx.Exec(ctx, `
			INSERT INTO settings (key, value)
			VALUES ($1, $2::jsonb)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
		`, s.key, s.jsonVal)
		if err != nil {
			log.Fatalf("seed setting %s: %v", s.key, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit seed: %v", err)
	}

	fmt.Println("✓ Seed Tahap 2 selesai (4 pengguna @demo.test, 2 sumber daya, 17 settings)")
}

