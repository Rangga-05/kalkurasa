// cmd/seed mengisi data awal yang idempoten untuk Tahap 2 & Tahap 3C:
// - 4 pengguna demo (@demo.test) dengan kata sandi dari env SEED_DEMO_PASSWORD
// - 2 sumber daya (oven dan labor)
// - 17 parameter settings sesuai Bagian 5 brief
// - 15 bahan baku (termasuk Tepung Terigu: stok 3 kg, satuan beli sak 25 kg, pembulatan loose)
// - 6 produk, beban produksi (product_loads), dan resep (recipe_items)
//   dengan biaya per unit Snack Box tepat Rp 19.000 (dihitung via internal/engine).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/engine"
)

type ingredientSeed struct {
	id          string
	name        string
	useUnit     string
	buyUnit     string
	buyUnitSize string
	buyPrice    string
	stock       string
	minStock    string
	rounding    string
}

type recipeSeed struct {
	ingredientID string
	qtyPerUnit   string
}

type productSeed struct {
	id            string
	name          string
	category      string
	price         string
	packagingCost string
	overheadCost  string
	ovenLoad      string
	laborLoad     string
	recipes       []recipeSeed
}

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

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("parse database config: %v", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Seed 5 pengguna demo termasuk customer@demo.test (idempoten pada email)
	demoUsers := []struct {
		email   string
		name    string
		contact string
		role    string
	}{
		{email: "owner@demo.test", name: "Bu Sari (Pemilik)", contact: "081100000001", role: auth.RoleOwner},
		{email: "manager@demo.test", name: "Rina (Manajer Operasional)", contact: "081100000002", role: auth.RoleManager},
		{email: "staff@demo.test", name: "Budi (Karyawan Produksi)", contact: "081100000003", role: auth.RoleStaff},
		{email: "admin@demo.test", name: "Admin Sistem", contact: "081100000004", role: auth.RoleAdmin},
		{email: "customer@demo.test", name: "Dewi Lestari (Pelanggan Demo)", contact: "081234567890", role: auth.RoleCustomer},
	}
	for _, u := range demoUsers {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (email, name, contact, role, password_hash, is_active)
			VALUES ($1, $2, $3, $4, $5, true)
			ON CONFLICT (email) DO UPDATE SET
				name = EXCLUDED.name,
				contact = EXCLUDED.contact,
				role = EXCLUDED.role,
				password_hash = EXCLUDED.password_hash,
				is_active = true
		`, u.email, u.name, u.contact, u.role, pwHash)
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
	resIDByCode := make(map[string]string, len(resources))
	for _, r := range resources {
		var rid string
		err := tx.QueryRow(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric)
			ON CONFLICT (code) DO UPDATE SET
				name = EXCLUDED.name,
				unit = EXCLUDED.unit,
				count = EXCLUDED.count,
				amount_per_unit_day = EXCLUDED.amount_per_unit_day,
				efficiency = EXCLUDED.efficiency
			RETURNING id::text
		`, r.code, r.name, r.unit, r.count, r.amountPerUnitDay, r.efficiency).Scan(&rid)
		if err != nil {
			log.Fatalf("seed resource %s: %v", r.code, err)
		}
		resIDByCode[r.code] = rid
	}

	// 3. Seed 17 parameter settings sesuai Bagian 5 brief (idempoten pada key)
	settingsSeed := []struct {
		key     string
		jsonVal string
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
		{key: "max_active_unpaid_orders", jsonVal: "2"},
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

	// 4. Seed 15 bahan baku (idempoten berdasarkan ID / nama)
	const (
		ingTepungID  = "10000000-0000-4000-8000-000000000001"
		ingGulaID    = "10000000-0000-4000-8000-000000000002"
		ingTelurID   = "10000000-0000-4000-8000-000000000003"
		ingMentegaID = "10000000-0000-4000-8000-000000000004"
		ingCokelatID = "10000000-0000-4000-8000-000000000005"
		ingKejuID    = "10000000-0000-4000-8000-000000000006"
		ingSusuID    = "10000000-0000-4000-8000-000000000007"
		ingKrimID    = "10000000-0000-4000-8000-000000000008"
		ingAyamID    = "10000000-0000-4000-8000-000000000009"
		ingBerasID   = "10000000-0000-4000-8000-000000000010"
		ingMinyakID  = "10000000-0000-4000-8000-000000000011"
		ingNanasID   = "10000000-0000-4000-8000-000000000012"
		ingKakaoID   = "10000000-0000-4000-8000-000000000013"
		ingRagiID    = "10000000-0000-4000-8000-000000000014"
		ingBumbuID   = "10000000-0000-4000-8000-000000000015"
	)

	ingredients := []ingredientSeed{
		// Tepung: stok awal 3 kg, pembulatan loose, satuan beli sak 25 kg, harga Rp 300.000/sak (Rp 12.000/kg)
		{id: ingTepungID, name: "Tepung Terigu", useUnit: "kg", buyUnit: "sak", buyUnitSize: "25.0000", buyPrice: "300000.0000", stock: "3.0000", minStock: "5.0000", rounding: "loose"},
		// Gula Pasir: Rp 160.000 / karung 10 kg (Rp 16.000/kg)
		{id: ingGulaID, name: "Gula Pasir", useUnit: "kg", buyUnit: "karung", buyUnitSize: "10.0000", buyPrice: "160000.0000", stock: "8.0000", minStock: "4.0000", rounding: "loose"},
		// Telur Ayam: Rp 60.000 / tray 30 butir (Rp 2.000/butir)
		{id: ingTelurID, name: "Telur Ayam", useUnit: "butir", buyUnit: "tray", buyUnitSize: "30.0000", buyPrice: "60000.0000", stock: "60.0000", minStock: "30.0000", rounding: "pack"},
		// Mentega: Rp 450.000 / dus 15 kg (Rp 30.000/kg)
		{id: ingMentegaID, name: "Mentega", useUnit: "kg", buyUnit: "dus", buyUnitSize: "15.0000", buyPrice: "450000.0000", stock: "6.0000", minStock: "3.0000", rounding: "loose"},
		// Cokelat Blok: Rp 350.000 / blok 5 kg (Rp 70.000/kg)
		{id: ingCokelatID, name: "Cokelat Blok", useUnit: "kg", buyUnit: "blok", buyUnitSize: "5.0000", buyPrice: "350000.0000", stock: "4.0000", minStock: "2.0000", rounding: "pack"},
		// Keju Cheddar: Rp 160.000 / blok 2 kg (Rp 80.000/kg)
		{id: ingKejuID, name: "Keju Cheddar", useUnit: "kg", buyUnit: "blok", buyUnitSize: "2.0000", buyPrice: "160000.0000", stock: "3.0000", minStock: "2.0000", rounding: "pack"},
		// Susu Cair: Rp 216.000 / karton 12 liter (Rp 18.000/liter)
		{id: ingSusuID, name: "Susu Cair Full Cream", useUnit: "liter", buyUnit: "karton", buyUnitSize: "12.0000", buyPrice: "216000.0000", stock: "10.0000", minStock: "6.0000", rounding: "pack"},
		// Krim Kocok: Rp 65.000 / liter
		{id: ingKrimID, name: "Krim Kocok (Whipping Cream)", useUnit: "liter", buyUnit: "kotak", buyUnitSize: "1.0000", buyPrice: "65000.0000", stock: "5.0000", minStock: "3.0000", rounding: "loose"},
		// Daging Ayam Fillet: Rp 55.000 / kg
		{id: ingAyamID, name: "Daging Ayam Fillet", useUnit: "kg", buyUnit: "kg", buyUnitSize: "1.0000", buyPrice: "55000.0000", stock: "5.0000", minStock: "3.0000", rounding: "loose"},
		// Beras Pandan Wangi: Rp 375.000 / karung 25 kg (Rp 15.000/kg)
		{id: ingBerasID, name: "Beras Pandan Wangi", useUnit: "kg", buyUnit: "karung", buyUnitSize: "25.0000", buyPrice: "375000.0000", stock: "20.0000", minStock: "10.0000", rounding: "pack"},
		// Minyak Goreng: Rp 90.000 / jerigen 5 liter (Rp 18.000/liter)
		{id: ingMinyakID, name: "Minyak Goreng", useUnit: "liter", buyUnit: "jerigen", buyUnitSize: "5.0000", buyPrice: "90000.0000", stock: "8.0000", minStock: "5.0000", rounding: "pack"},
		// Selai Nanas: Rp 45.000 / kg
		{id: ingNanasID, name: "Selai Nanas", useUnit: "kg", buyUnit: "kg", buyUnitSize: "1.0000", buyPrice: "45000.0000", stock: "4.0000", minStock: "2.0000", rounding: "loose"},
		// Bubuk Kakao: Rp 90.000 / pak 1 kg
		{id: ingKakaoID, name: "Bubuk Kakao", useUnit: "kg", buyUnit: "pak", buyUnitSize: "1.0000", buyPrice: "90000.0000", stock: "2.5000", minStock: "1.0000", rounding: "pack"},
		// Ragi & Pengembang: Rp 40.000 / kotak 0,5 kg (Rp 80.000/kg)
		{id: ingRagiID, name: "Ragi & Pengembang Kue", useUnit: "kg", buyUnit: "kotak", buyUnitSize: "0.5000", buyPrice: "40000.0000", stock: "1.0000", minStock: "0.5000", rounding: "pack"},
		// Bumbu Rempah & Bawang: Rp 40.000 / kg
		{id: ingBumbuID, name: "Bumbu Rempah & Bawang", useUnit: "kg", buyUnit: "kg", buyUnitSize: "1.0000", buyPrice: "40000.0000", stock: "4.0000", minStock: "2.0000", rounding: "loose"},
	}

	ingActualID := make(map[string]string, len(ingredients))
	for _, ing := range ingredients {
		var existingID string
		err := tx.QueryRow(ctx, `SELECT id::text FROM ingredients WHERE name = $1 LIMIT 1`, ing.name).Scan(&existingID)
		if err == nil && existingID != "" {
			_, err = tx.Exec(ctx, `
				UPDATE ingredients
				SET use_unit = $1,
				    buy_unit = $2,
				    buy_unit_size = $3::numeric,
				    buy_price = $4::numeric,
				    stock = $5::numeric,
				    min_stock = $6::numeric,
				    rounding = $7,
				    is_active = true
				WHERE id = $8::uuid
			`, ing.useUnit, ing.buyUnit, ing.buyUnitSize, ing.buyPrice, ing.stock, ing.minStock, ing.rounding, existingID)
			if err != nil {
				log.Fatalf("update ingredient %s: %v", ing.name, err)
			}
			ingActualID[ing.id] = existingID
			continue
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO ingredients (id, name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active)
			VALUES ($1::uuid, $2, $3, $4, $5::numeric, $6::numeric, $7::numeric, $8::numeric, $9, true)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				use_unit = EXCLUDED.use_unit,
				buy_unit = EXCLUDED.buy_unit,
				buy_unit_size = EXCLUDED.buy_unit_size,
				buy_price = EXCLUDED.buy_price,
				stock = EXCLUDED.stock,
				min_stock = EXCLUDED.min_stock,
				rounding = EXCLUDED.rounding,
				is_active = true
		`, ing.id, ing.name, ing.useUnit, ing.buyUnit, ing.buyUnitSize, ing.buyPrice, ing.stock, ing.minStock, ing.rounding)
		if err != nil {
			log.Fatalf("insert ingredient %s: %v", ing.name, err)
		}
		ingActualID[ing.id] = ing.id
	}

	// 5. Seed 6 produk beserta beban produksi (product_loads) dan resep (recipe_items)
	// Perhitungan biaya per unit Snack Box:
	//   Tepung Terigu: 0,12 kg * 12.000 = 1.440
	//   Gula Pasir:    0,05 kg * 16.000 =   800
	//   Telur Ayam:    1 butir *  2.000 = 2.000
	//   Mentega:       0,04 kg * 30.000 = 1.200
	//   Ayam Fillet:   0,06 kg * 55.000 = 3.300
	//   Keju Cheddar:  0,022 kg * 80.000 = 1.760
	//   -----------------------------------------
	//   Biaya bahan sebelum waste       = 10.500
	//   Biaya bahan + waste 5% (×1,05)  = 11.025
	//   Kemasan                         =  2.000
	//   Tenaga (12 menit × Rp 400)      =  4.800
	//   Overhead                        =  1.175
	//   -----------------------------------------
	//   UnitCost Snack Box              = 19.000
	const (
		prodSnackBoxID = "20000000-0000-4000-8000-000000000001"
		prodTartID     = "20000000-0000-4000-8000-000000000002"
		prodCupcakeID  = "20000000-0000-4000-8000-000000000003"
		prodNasiBoxID  = "20000000-0000-4000-8000-000000000004"
		prodKueKering  = "20000000-0000-4000-8000-000000000005"
		prodBrowniesID = "20000000-0000-4000-8000-000000000006"
	)

	products := []productSeed{
		{
			id:            prodSnackBoxID,
			name:          "Snack Box",
			category:      "snack_box",
			price:         "25000.0000",
			packagingCost: "2000.0000",
			overheadCost:  "1175.0000",
			ovenLoad:      "0.0500",
			laborLoad:     "12.0000",
			recipes: []recipeSeed{
				{ingredientID: ingTepungID, qtyPerUnit: "0.120000"},
				{ingredientID: ingGulaID, qtyPerUnit: "0.050000"},
				{ingredientID: ingTelurID, qtyPerUnit: "1.000000"},
				{ingredientID: ingMentegaID, qtyPerUnit: "0.040000"},
				{ingredientID: ingAyamID, qtyPerUnit: "0.060000"},
				{ingredientID: ingKejuID, qtyPerUnit: "0.022000"},
			},
		},
		{
			id:            prodTartID,
			name:          "Tart Ulang Tahun",
			category:      "kue_tart",
			price:         "280000.0000",
			packagingCost: "15000.0000",
			overheadCost:  "10000.0000",
			ovenLoad:      "0.5000",
			laborLoad:     "90.0000",
			recipes: []recipeSeed{
				{ingredientID: ingTepungID, qtyPerUnit: "0.400000"},
				{ingredientID: ingGulaID, qtyPerUnit: "0.300000"},
				{ingredientID: ingTelurID, qtyPerUnit: "8.000000"},
				{ingredientID: ingMentegaID, qtyPerUnit: "0.350000"},
				{ingredientID: ingCokelatID, qtyPerUnit: "0.400000"},
				{ingredientID: ingKrimID, qtyPerUnit: "0.500000"},
				{ingredientID: ingSusuID, qtyPerUnit: "0.250000"},
			},
		},
		{
			id:            prodCupcakeID,
			name:          "Cupcake Box (Isi 6)",
			category:      "kue_tart",
			price:         "95000.0000",
			packagingCost: "6000.0000",
			overheadCost:  "3500.0000",
			ovenLoad:      "0.2500",
			laborLoad:     "35.0000",
			recipes: []recipeSeed{
				{ingredientID: ingTepungID, qtyPerUnit: "0.200000"},
				{ingredientID: ingGulaID, qtyPerUnit: "0.150000"},
				{ingredientID: ingTelurID, qtyPerUnit: "3.000000"},
				{ingredientID: ingMentegaID, qtyPerUnit: "0.150000"},
				{ingredientID: ingKrimID, qtyPerUnit: "0.200000"},
				{ingredientID: ingKakaoID, qtyPerUnit: "0.050000"},
			},
		},
		{
			id:            prodNasiBoxID,
			name:          "Nasi Box Ayam Spesial",
			category:      "katering",
			price:         "35000.0000",
			packagingCost: "3000.0000",
			overheadCost:  "1500.0000",
			ovenLoad:      "0.0200",
			laborLoad:     "15.0000",
			recipes: []recipeSeed{
				{ingredientID: ingBerasID, qtyPerUnit: "0.150000"},
				{ingredientID: ingAyamID, qtyPerUnit: "0.180000"},
				{ingredientID: ingTelurID, qtyPerUnit: "1.000000"},
				{ingredientID: ingMinyakID, qtyPerUnit: "0.050000"},
				{ingredientID: ingBumbuID, qtyPerUnit: "0.050000"},
			},
		},
		{
			id:            prodKueKering,
			name:          "Kue Kering Nastar (Toples 500g)",
			category:      "kue_kering",
			price:         "110000.0000",
			packagingCost: "7000.0000",
			overheadCost:  "4500.0000",
			ovenLoad:      "0.4000",
			laborLoad:     "45.0000",
			recipes: []recipeSeed{
				{ingredientID: ingTepungID, qtyPerUnit: "0.350000"},
				{ingredientID: ingMentegaID, qtyPerUnit: "0.250000"},
				{ingredientID: ingGulaID, qtyPerUnit: "0.100000"},
				{ingredientID: ingTelurID, qtyPerUnit: "3.000000"},
				{ingredientID: ingNanasID, qtyPerUnit: "0.250000"},
				{ingredientID: ingKejuID, qtyPerUnit: "0.050000"},
			},
		},
		{
			id:            prodBrowniesID,
			name:          "Brownies Panggang Fudgy",
			category:      "kue_loyang",
			price:         "85000.0000",
			packagingCost: "5000.0000",
			overheadCost:  "3000.0000",
			ovenLoad:      "0.3500",
			laborLoad:     "30.0000",
			recipes: []recipeSeed{
				{ingredientID: ingTepungID, qtyPerUnit: "0.150000"},
				{ingredientID: ingCokelatID, qtyPerUnit: "0.250000"},
				{ingredientID: ingMentegaID, qtyPerUnit: "0.150000"},
				{ingredientID: ingGulaID, qtyPerUnit: "0.150000"},
				{ingredientID: ingTelurID, qtyPerUnit: "3.000000"},
				{ingredientID: ingKakaoID, qtyPerUnit: "0.050000"},
				{ingredientID: ingRagiID, qtyPerUnit: "0.010000"},
			},
		},
	}

	var snackBoxActualID string
	for _, p := range products {
		actualPID := p.id
		var existingID string
		err := tx.QueryRow(ctx, `SELECT id::text FROM products WHERE name = $1 LIMIT 1`, p.name).Scan(&existingID)
		if err == nil && existingID != "" {
			actualPID = existingID
			_, err = tx.Exec(ctx, `
				UPDATE products
				SET category = $1,
				    price = $2::numeric,
				    packaging_cost = $3::numeric,
				    overhead_cost = $4::numeric,
				    is_active = true
				WHERE id = $5::uuid
			`, p.category, p.price, p.packagingCost, p.overheadCost, actualPID)
			if err != nil {
				log.Fatalf("update product %s: %v", p.name, err)
			}
		} else {
			_, err = tx.Exec(ctx, `
				INSERT INTO products (id, name, category, price, packaging_cost, overhead_cost, is_active)
				VALUES ($1::uuid, $2, $3, $4::numeric, $5::numeric, $6::numeric, true)
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					category = EXCLUDED.category,
					price = EXCLUDED.price,
					packaging_cost = EXCLUDED.packaging_cost,
					overhead_cost = EXCLUDED.overhead_cost,
					is_active = true
			`, actualPID, p.name, p.category, p.price, p.packagingCost, p.overheadCost)
			if err != nil {
				log.Fatalf("insert product %s: %v", p.name, err)
			}
		}

		if p.id == prodSnackBoxID {
			snackBoxActualID = actualPID
		}

		// Upsert product_loads (oven & labor)
		for _, ld := range []struct {
			code string
			val  string
		}{
			{code: "oven", val: p.ovenLoad},
			{code: "labor", val: p.laborLoad},
		} {
			rid := resIDByCode[ld.code]
			_, err := tx.Exec(ctx, `
				INSERT INTO product_loads (product_id, resource_id, load)
				VALUES ($1::uuid, $2::uuid, $3::numeric)
				ON CONFLICT (product_id, resource_id) DO UPDATE SET
					load = EXCLUDED.load
			`, actualPID, rid, ld.val)
			if err != nil {
				log.Fatalf("upsert product_load %s/%s: %v", p.name, ld.code, err)
			}
		}

		// Bersihkan recipe_items lama untuk produk ini lalu isi resep yang telah ditetapkan
		if _, err := tx.Exec(ctx, `DELETE FROM recipe_items WHERE product_id = $1::uuid`, actualPID); err != nil {
			log.Fatalf("clear recipe_items %s: %v", p.name, err)
		}
		for _, rec := range p.recipes {
			iid := ingActualID[rec.ingredientID]
			_, err := tx.Exec(ctx, `
				INSERT INTO recipe_items (product_id, ingredient_id, qty_per_unit)
				VALUES ($1::uuid, $2::uuid, $3::numeric)
				ON CONFLICT (product_id, ingredient_id) DO UPDATE SET
					qty_per_unit = EXCLUDED.qty_per_unit
			`, actualPID, iid, rec.qtyPerUnit)
			if err != nil {
				log.Fatalf("upsert recipe_item %s: %v", p.name, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit seed: %v", err)
	}

	// Hitung jumlah baris tiap tabel dan verifikasi biaya per unit Snack Box lewat internal/engine
	tables := []string{"users", "resources", "settings", "products", "ingredients", "recipe_items", "product_loads"}
	fmt.Println("=== Ringkasan Jumlah Baris Tabel ===")
	for _, tbl := range tables {
		var cnt int
		if err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM "+tbl).Scan(&cnt); err != nil {
			log.Fatalf("count %s: %v", tbl, err)
		}
		fmt.Printf("  %-15s : %d\n", tbl, cnt)
	}

	// Verifikasi UnitCost Snack Box dari data yang baru di-seed di database menggunakan engine.UnitCost
	var sbPrice, sbPackaging, sbOverhead, sbIngCost, sbLaborMin decimal.Decimal
	if err := conn.QueryRow(ctx, `
		SELECT price, packaging_cost, overhead_cost
		FROM products WHERE id = $1::uuid
	`, snackBoxActualID).Scan(&sbPrice, &sbPackaging, &sbOverhead); err != nil {
		log.Fatalf("query snack box: %v", err)
	}
	if err := conn.QueryRow(ctx, `
		SELECT COALESCE(SUM(ri.qty_per_unit * i.buy_price / i.buy_unit_size), 0)
		FROM recipe_items ri
		JOIN ingredients i ON i.id = ri.ingredient_id
		WHERE ri.product_id = $1::uuid
	`, snackBoxActualID).Scan(&sbIngCost); err != nil {
		log.Fatalf("query snack box recipe cost: %v", err)
	}
	if err := conn.QueryRow(ctx, `
		SELECT pl.load
		FROM product_loads pl
		JOIN resources r ON r.id = pl.resource_id
		WHERE pl.product_id = $1::uuid AND r.code = 'labor'
	`, snackBoxActualID).Scan(&sbLaborMin); err != nil {
		log.Fatalf("query snack box labor load: %v", err)
	}

	wastePct := decimal.RequireFromString("0.05")
	laborRate := decimal.NewFromInt(400)
	sbUnitCost := engine.UnitCost(engine.CostInput{
		IngredientCost: sbIngCost,
		WastePct:       wastePct,
		PackagingCost:  sbPackaging,
		LaborMinutes:   sbLaborMin,
		LaborRate:      laborRate,
		OverheadCost:   sbOverhead,
	})
	ingAfterWaste := sbIngCost.Mul(decimal.NewFromInt(1).Add(wastePct))

	fmt.Println("=== Verifikasi Biaya Snack Box (internal/engine) ===")
	fmt.Printf("  Harga jual               : Rp %s\n", sbPrice.StringFixed(0))
	fmt.Printf("  Biaya bahan (sebelum 5%%) : Rp %s\n", sbIngCost.StringFixed(0))
	fmt.Printf("  Biaya bahan (setelah 5%%) : Rp %s\n", ingAfterWaste.StringFixed(0))
	fmt.Printf("  Kemasan                  : Rp %s\n", sbPackaging.StringFixed(0))
	fmt.Printf("  Tenaga (12 mnt x Rp 400) : Rp %s\n", sbLaborMin.Mul(laborRate).StringFixed(0))
	fmt.Printf("  Overhead                 : Rp %s\n", sbOverhead.StringFixed(0))
	fmt.Printf("  UnitCost (engine)        : Rp %s\n", sbUnitCost.StringFixed(0))
	fmt.Println("✓ Seed katalog dasar selesai.")
}
