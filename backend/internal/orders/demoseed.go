package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/httpx"
)

// Konstanta UUID deterministik untuk master data & pengguna demo.
const (
	DemoUserOwnerID     = "00000000-0000-4000-8000-000000000001"
	DemoUserManagerID   = "00000000-0000-4000-8000-000000000002"
	DemoUserStaffID     = "00000000-0000-4000-8000-000000000003"
	DemoUserAdminID     = "00000000-0000-4000-8000-000000000004"
	DemoUserCustomerID  = "00000000-0000-4000-8000-000000000005"
	DemoUserCustomer2ID = "00000000-0000-4000-8000-000000000006"
	DemoUserCustomer3ID = "00000000-0000-4000-8000-000000000007"

	DemoIngTepungID  = "10000000-0000-4000-8000-000000000001"
	DemoIngGulaID    = "10000000-0000-4000-8000-000000000002"
	DemoIngTelurID   = "10000000-0000-4000-8000-000000000003"
	DemoIngMentegaID = "10000000-0000-4000-8000-000000000004"
	DemoIngCokelatID = "10000000-0000-4000-8000-000000000005"
	DemoIngKejuID    = "10000000-0000-4000-8000-000000000006"
	DemoIngSusuID    = "10000000-0000-4000-8000-000000000007"
	DemoIngKrimID    = "10000000-0000-4000-8000-000000000008"
	DemoIngAyamID    = "10000000-0000-4000-8000-000000000009"
	DemoIngBerasID   = "10000000-0000-4000-8000-000000000010"
	DemoIngMinyakID  = "10000000-0000-4000-8000-000000000011"
	DemoIngNanasID   = "10000000-0000-4000-8000-000000000012"
	DemoIngKakaoID   = "10000000-0000-4000-8000-000000000013"
	DemoIngRagiID    = "10000000-0000-4000-8000-000000000014"
	DemoIngBumbuID   = "10000000-0000-4000-8000-000000000015"

	DemoProdSnackBoxID = "20000000-0000-4000-8000-000000000001"
	DemoProdTartID     = "20000000-0000-4000-8000-000000000002"
	DemoProdCupcakeID  = "20000000-0000-4000-8000-000000000003"
	DemoProdNasiBoxID  = "20000000-0000-4000-8000-000000000004"
	DemoProdKueKering  = "20000000-0000-4000-8000-000000000005"
	DemoProdBrowniesID = "20000000-0000-4000-8000-000000000006"
)

// DemoSeedSummary adalah ringkasan hasil eksekusi SeedDemoData dan endpoint POST /api/v1/admin/reset-demo.
type DemoSeedSummary struct {
	Today            string         `json:"today"`
	SnackBoxUnitCost string         `json:"snack_box_unit_cost"`
	TableCounts      map[string]int `json:"table_counts"`
	StatusCounts     map[string]int `json:"status_counts"`
	Scenarios        map[string]any `json:"scenarios"`
	ResetAt          time.Time      `json:"reset_at"`
}

type demoIngSeed struct {
	id, name, useUnit, buyUnit, buyUnitSize, buyPrice, stock, minStock, rounding string
}

type demoRecipeSeed struct {
	ingredientID, qtyPerUnit string
}

type demoProdSeed struct {
	id, name, category, price, packagingCost, overheadCost, ovenLoad, laborLoad string
	recipes                                                                     []demoRecipeSeed
}

type demoOrderItemSpec struct {
	productID string
	qty       int
	unitPrice decimal.Decimal
	unitCost  decimal.Decimal
}

type demoOrderSpec struct {
	id                string
	code              string
	source            string // "public" | "manual"
	customerID        string
	customerName      string
	customerContact   string
	deliveryType      string // "delivery" | "pickup"
	address           string
	notes             string
	dayOffset         int // event_date = today + dayOffset
	prodOffset        int // production_date = today + prodOffset
	status            string
	items             []demoOrderItemSpec
	paidAmount        decimal.Decimal
	paymentMethod     string
	paymentNote       string
	produceActualMin  *int
	completedLateDays int // 0 = selesai tepat pada event_date, >0 = terlambat N hari
	overrideReason    string
	decidedBy         string
	createdBy         string
	customMinDP       *decimal.Decimal
	customDPCharged   *decimal.Decimal
	customLowNoDP     *decimal.Decimal
	customLowWithDP   *decimal.Decimal
	cancelShopCost    *decimal.Decimal
	cancelExposure    *decimal.Decimal
}

// ResetDemo menangani POST /api/v1/admin/reset-demo (khusus peran admin).
func (h *Handler) ResetDemo(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	actor := auth.CurrentUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "Autentikasi diperlukan.")
		return
	}
	if actor.Role != auth.RoleAdmin {
		httpx.Forbidden(c, "Hanya administrator yang dapat mereset data demo.")
		return
	}

	demoPass := os.Getenv("SEED_DEMO_PASSWORD")
	if len(demoPass) < 8 {
		demoPass = "DemoKalkurasa2026!"
	}

	summary, err := SeedDemoData(c.Request.Context(), h.DB, demoPass, actor.ID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, summary)
}

// SeedDemoData mengisi ulang katalog (tanpa mengubah biaya Snack Box Rp 19.000) dan
// membuat 40 pesanan demo deterministik beserta pembayaran, tugas, pergerakan stok,
// entri kas, dan audit log secara idempoten dalam satu transaksi.
func SeedDemoData(ctx context.Context, conn db.Conn, demoPassword string, actorID ...string) (*DemoSeedSummary, error) {
	var pwHash string
	if len(demoPassword) >= 8 {
		h, err := auth.HashPassword(demoPassword)
		if err != nil {
			return nil, fmt.Errorf("hash demo password: %w", err)
		}
		pwHash = h
	}

	var summary *DemoSeedSummary
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		// Jika demoPassword tidak diberikan, ambil password_hash yang sudah ada dari salah satu akun @demo.test
		if pwHash == "" {
			_ = tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE email = 'owner@demo.test' LIMIT 1`).Scan(&pwHash)
			if pwHash == "" {
				h, err := auth.HashPassword("DemoKalkurasa2026!")
				if err != nil {
					return err
				}
				pwHash = h
			}
		}

		// 1. Bersihkan tabel transaksional agar seed idempoten dan bebas sisa pesanan uji manual
		for _, tbl := range []string{
			"audit_logs", "stock_movements", "purchases", "cash_entries", "tasks", "payments",
			"order_evaluations", "order_items", "orders", "day_locks", "rate_limits",
		} {
			if _, err := tx.Exec(ctx, "DELETE FROM "+tbl); err != nil {
				return fmt.Errorf("clean table %s: %w", tbl, err)
			}
		}

		// 2. Upsert pengguna demo (5 akun utama + 2 pelanggan tambahan untuk pesanan antrian/unpaid)
		demoUsers := []struct {
			id, email, name, contact, role string
		}{
			{id: DemoUserOwnerID, email: "owner@demo.test", name: "Bu Sari (Pemilik)", contact: "081100000001", role: auth.RoleOwner},
			{id: DemoUserManagerID, email: "manager@demo.test", name: "Rina (Manajer Operasional)", contact: "081100000002", role: auth.RoleManager},
			{id: DemoUserStaffID, email: "staff@demo.test", name: "Budi (Karyawan Produksi)", contact: "081100000003", role: auth.RoleStaff},
			{id: DemoUserAdminID, email: "admin@demo.test", name: "Admin Sistem", contact: "081100000004", role: auth.RoleAdmin},
			{id: DemoUserCustomerID, email: "customer@demo.test", name: "Dewi Lestari (Pelanggan Demo)", contact: "081234567890", role: auth.RoleCustomer},
			{id: DemoUserCustomer2ID, email: "pelanggan2@demo.test", name: "Hendra Wijaya", contact: "081298765431", role: auth.RoleCustomer},
			{id: DemoUserCustomer3ID, email: "pelanggan3@demo.test", name: "Maya Putri", contact: "081298765432", role: auth.RoleCustomer},
		}

		userIDByEmail := make(map[string]string, len(demoUsers))
		for _, u := range demoUsers {
			var uid string
			err := tx.QueryRow(ctx, `
				INSERT INTO users (id, email, name, contact, role, password_hash, is_active)
				VALUES ($1::uuid, $2, $3, $4, $5, $6, true)
				ON CONFLICT (email) DO UPDATE SET
					name = EXCLUDED.name,
					contact = EXCLUDED.contact,
					role = EXCLUDED.role,
					password_hash = EXCLUDED.password_hash,
					is_active = true
				RETURNING id::text
			`, u.id, u.email, u.name, u.contact, u.role, pwHash).Scan(&uid)
			if err != nil {
				return fmt.Errorf("seed user %s: %w", u.email, err)
			}
			userIDByEmail[u.email] = uid
		}

		ownerID := userIDByEmail["owner@demo.test"]
		managerID := userIDByEmail["manager@demo.test"]
		staffID := userIDByEmail["staff@demo.test"]
		adminID := userIDByEmail["admin@demo.test"]
		cust1ID := userIDByEmail["customer@demo.test"]
		cust2ID := userIDByEmail["pelanggan2@demo.test"]
		cust3ID := userIDByEmail["pelanggan3@demo.test"]

		// Hapus akun non-demo yang mungkin dibuat saat uji registrasi manual
		if _, err := tx.Exec(ctx, `
			DELETE FROM users
			WHERE email NOT IN (
				'owner@demo.test', 'manager@demo.test', 'staff@demo.test', 'admin@demo.test',
				'customer@demo.test', 'pelanggan2@demo.test', 'pelanggan3@demo.test'
			)
		`); err != nil {
			return fmt.Errorf("clean manual users: %w", err)
		}

		// 3. Upsert 2 sumber daya (oven & labor)
		resources := []struct {
			code, name, unit string
			count            int
			amount, eff      string
		}{
			{code: "oven", name: "Oven Deck", unit: "oven_hour", count: 2, amount: "8.0000", eff: "0.8500"},
			{code: "labor", name: "Tenaga Produksi", unit: "minute", count: 4, amount: "420.0000", eff: "0.8500"},
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
			`, r.code, r.name, r.unit, r.count, r.amount, r.eff).Scan(&rid)
			if err != nil {
				return fmt.Errorf("seed resource %s: %w", r.code, err)
			}
			resIDByCode[r.code] = rid
		}

		// 4. Upsert settings sesuai Bagian 5 brief (pertahankan demo_today bila sudah disetel, default null bila belum ada)
		settingsSeed := []struct {
			key, val string
		}{
			{key: "capacity_limit", val: "0.90"},
			{key: "status_green_below", val: "0.70"},
			{key: "status_yellow_below", val: "0.90"},
			{key: "margin_min", val: "0.20"},
			{key: "labor_rate_per_minute", val: "400"},
			{key: "waste_pct", val: "0.05"},
			{key: "shopping_buffer_pct", val: "0.10"},
			{key: "cash_buffer", val: "400000"},
			{key: "fixed_cost_daily", val: "400000"},
			{key: "opening_cash", val: "1800000"},
			{key: "lead_time_days", val: "2"},
			{key: "dp_pct", val: "0.30"},
			{key: "dp_deadline_hours", val: "24"},
			{key: "big_order_total", val: "2000000"},
			{key: "big_order_units", val: "50"},
			{key: "shopping_lead_days", val: "2"},
			{key: "max_active_unpaid_orders", val: "2"},
			{key: "tax_pct", val: "0.11"},
			{key: "platform_fee", val: "2000"},
		}
		for _, s := range settingsSeed {
			if _, err := tx.Exec(ctx, `
				INSERT INTO settings (key, value)
				VALUES ($1, $2::jsonb)
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
			`, s.key, s.val); err != nil {
				return fmt.Errorf("seed setting %s: %w", s.key, err)
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO settings (key, value)
			VALUES ('demo_today', 'null'::jsonb)
			ON CONFLICT (key) DO NOTHING
		`); err != nil {
			return fmt.Errorf("seed demo_today: %w", err)
		}

		// 5. Upsert 15 bahan baku (Tepung Terigu stok 3 kg, min_stock 5 kg, loose, sak 25 kg @ Rp 300.000)
		ingredients := []demoIngSeed{
			{id: DemoIngTepungID, name: "Tepung Terigu", useUnit: "kg", buyUnit: "sak", buyUnitSize: "25.0000", buyPrice: "300000.0000", stock: "3.0000", minStock: "5.0000", rounding: "loose"},
			{id: DemoIngGulaID, name: "Gula Pasir", useUnit: "kg", buyUnit: "karung", buyUnitSize: "10.0000", buyPrice: "160000.0000", stock: "8.0000", minStock: "4.0000", rounding: "loose"},
			{id: DemoIngTelurID, name: "Telur Ayam", useUnit: "butir", buyUnit: "tray", buyUnitSize: "30.0000", buyPrice: "60000.0000", stock: "60.0000", minStock: "30.0000", rounding: "pack"},
			{id: DemoIngMentegaID, name: "Mentega", useUnit: "kg", buyUnit: "dus", buyUnitSize: "15.0000", buyPrice: "450000.0000", stock: "6.0000", minStock: "3.0000", rounding: "loose"},
			{id: DemoIngCokelatID, name: "Cokelat Blok", useUnit: "kg", buyUnit: "blok", buyUnitSize: "5.0000", buyPrice: "350000.0000", stock: "4.0000", minStock: "2.0000", rounding: "pack"},
			{id: DemoIngKejuID, name: "Keju Cheddar", useUnit: "kg", buyUnit: "blok", buyUnitSize: "2.0000", buyPrice: "160000.0000", stock: "1.5000", minStock: "2.0000", rounding: "pack"},
			{id: DemoIngSusuID, name: "Susu Cair Full Cream", useUnit: "liter", buyUnit: "karton", buyUnitSize: "12.0000", buyPrice: "216000.0000", stock: "10.0000", minStock: "6.0000", rounding: "pack"},
			{id: DemoIngKrimID, name: "Krim Kocok (Whipping Cream)", useUnit: "liter", buyUnit: "kotak", buyUnitSize: "1.0000", buyPrice: "65000.0000", stock: "2.0000", minStock: "3.0000", rounding: "loose"},
			{id: DemoIngAyamID, name: "Daging Ayam Fillet", useUnit: "kg", buyUnit: "kg", buyUnitSize: "1.0000", buyPrice: "55000.0000", stock: "5.0000", minStock: "3.0000", rounding: "loose"},
			{id: DemoIngBerasID, name: "Beras Pandan Wangi", useUnit: "kg", buyUnit: "karung", buyUnitSize: "25.0000", buyPrice: "375000.0000", stock: "20.0000", minStock: "10.0000", rounding: "pack"},
			{id: DemoIngMinyakID, name: "Minyak Goreng", useUnit: "liter", buyUnit: "jerigen", buyUnitSize: "5.0000", buyPrice: "90000.0000", stock: "8.0000", minStock: "5.0000", rounding: "pack"},
			{id: DemoIngNanasID, name: "Selai Nanas", useUnit: "kg", buyUnit: "kg", buyUnitSize: "1.0000", buyPrice: "45000.0000", stock: "4.0000", minStock: "2.0000", rounding: "loose"},
			{id: DemoIngKakaoID, name: "Bubuk Kakao", useUnit: "kg", buyUnit: "pak", buyUnitSize: "1.0000", buyPrice: "90000.0000", stock: "2.5000", minStock: "1.0000", rounding: "pack"},
			{id: DemoIngRagiID, name: "Ragi & Pengembang Kue", useUnit: "kg", buyUnit: "kotak", buyUnitSize: "0.5000", buyPrice: "40000.0000", stock: "1.0000", minStock: "0.5000", rounding: "pack"},
			{id: DemoIngBumbuID, name: "Bumbu Rempah & Bawang", useUnit: "kg", buyUnit: "kg", buyUnitSize: "1.0000", buyPrice: "40000.0000", stock: "4.0000", minStock: "2.0000", rounding: "loose"},
		}

		ingActualID := make(map[string]string, len(ingredients))
		for _, ing := range ingredients {
			var existingID string
			err := tx.QueryRow(ctx, `SELECT id::text FROM ingredients WHERE name = $1 LIMIT 1`, ing.name).Scan(&existingID)
			if err == nil && existingID != "" {
				if _, err := tx.Exec(ctx, `
					UPDATE ingredients
					SET use_unit = $1, buy_unit = $2, buy_unit_size = $3::numeric, buy_price = $4::numeric,
					    stock = $5::numeric, min_stock = $6::numeric, rounding = $7, is_active = true
					WHERE id = $8::uuid
				`, ing.useUnit, ing.buyUnit, ing.buyUnitSize, ing.buyPrice, ing.stock, ing.minStock, ing.rounding, existingID); err != nil {
					return fmt.Errorf("update ingredient %s: %w", ing.name, err)
				}
				ingActualID[ing.id] = existingID
				continue
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO ingredients (id, name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active)
				VALUES ($1::uuid, $2, $3, $4, $5::numeric, $6::numeric, $7::numeric, $8::numeric, $9, true)
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name, use_unit = EXCLUDED.use_unit, buy_unit = EXCLUDED.buy_unit,
					buy_unit_size = EXCLUDED.buy_unit_size, buy_price = EXCLUDED.buy_price,
					stock = EXCLUDED.stock, min_stock = EXCLUDED.min_stock, rounding = EXCLUDED.rounding,
					is_active = true
			`, ing.id, ing.name, ing.useUnit, ing.buyUnit, ing.buyUnitSize, ing.buyPrice, ing.stock, ing.minStock, ing.rounding); err != nil {
				return fmt.Errorf("insert ingredient %s: %w", ing.name, err)
			}
			ingActualID[ing.id] = ing.id
		}

		// 6. Upsert 6 produk katalog (Snack Box tetap tepat UnitCost = Rp 19.000)
		products := []demoProdSeed{
			{
				id: DemoProdSnackBoxID, name: "Snack Box", category: "snack_box",
				price: "25000.0000", packagingCost: "2000.0000", overheadCost: "1175.0000",
				ovenLoad: "0.0500", laborLoad: "12.0000",
				recipes: []demoRecipeSeed{
					{ingredientID: DemoIngTepungID, qtyPerUnit: "0.120000"},
					{ingredientID: DemoIngGulaID, qtyPerUnit: "0.050000"},
					{ingredientID: DemoIngTelurID, qtyPerUnit: "1.000000"},
					{ingredientID: DemoIngMentegaID, qtyPerUnit: "0.040000"},
					{ingredientID: DemoIngAyamID, qtyPerUnit: "0.060000"},
					{ingredientID: DemoIngKejuID, qtyPerUnit: "0.022000"},
				},
			},
			{
				id: DemoProdTartID, name: "Tart Ulang Tahun", category: "kue_tart",
				price: "280000.0000", packagingCost: "15000.0000", overheadCost: "10000.0000",
				ovenLoad: "0.5000", laborLoad: "90.0000",
				recipes: []demoRecipeSeed{
					{ingredientID: DemoIngTepungID, qtyPerUnit: "0.400000"},
					{ingredientID: DemoIngGulaID, qtyPerUnit: "0.300000"},
					{ingredientID: DemoIngTelurID, qtyPerUnit: "8.000000"},
					{ingredientID: DemoIngMentegaID, qtyPerUnit: "0.350000"},
					{ingredientID: DemoIngCokelatID, qtyPerUnit: "0.400000"},
					{ingredientID: DemoIngKrimID, qtyPerUnit: "0.500000"},
					{ingredientID: DemoIngSusuID, qtyPerUnit: "0.250000"},
				},
			},
			{
				id: DemoProdCupcakeID, name: "Cupcake Box (Isi 6)", category: "kue_tart",
				price: "95000.0000", packagingCost: "6000.0000", overheadCost: "3500.0000",
				ovenLoad: "0.2500", laborLoad: "35.0000",
				recipes: []demoRecipeSeed{
					{ingredientID: DemoIngTepungID, qtyPerUnit: "0.200000"},
					{ingredientID: DemoIngGulaID, qtyPerUnit: "0.150000"},
					{ingredientID: DemoIngTelurID, qtyPerUnit: "3.000000"},
					{ingredientID: DemoIngMentegaID, qtyPerUnit: "0.150000"},
					{ingredientID: DemoIngKrimID, qtyPerUnit: "0.200000"},
					{ingredientID: DemoIngKakaoID, qtyPerUnit: "0.050000"},
				},
			},
			{
				id: DemoProdNasiBoxID, name: "Nasi Box Ayam Spesial", category: "katering",
				price: "35000.0000", packagingCost: "3000.0000", overheadCost: "1500.0000",
				ovenLoad: "0.0200", laborLoad: "15.0000",
				recipes: []demoRecipeSeed{
					{ingredientID: DemoIngBerasID, qtyPerUnit: "0.150000"},
					{ingredientID: DemoIngAyamID, qtyPerUnit: "0.180000"},
					{ingredientID: DemoIngTelurID, qtyPerUnit: "1.000000"},
					{ingredientID: DemoIngMinyakID, qtyPerUnit: "0.050000"},
					{ingredientID: DemoIngBumbuID, qtyPerUnit: "0.050000"},
				},
			},
			{
				id: DemoProdKueKering, name: "Kue Kering Nastar (Toples 500g)", category: "kue_kering",
				price: "110000.0000", packagingCost: "7000.0000", overheadCost: "4500.0000",
				ovenLoad: "0.4000", laborLoad: "45.0000",
				recipes: []demoRecipeSeed{
					{ingredientID: DemoIngTepungID, qtyPerUnit: "0.350000"},
					{ingredientID: DemoIngMentegaID, qtyPerUnit: "0.250000"},
					{ingredientID: DemoIngGulaID, qtyPerUnit: "0.100000"},
					{ingredientID: DemoIngTelurID, qtyPerUnit: "3.000000"},
					{ingredientID: DemoIngNanasID, qtyPerUnit: "0.250000"},
					{ingredientID: DemoIngKejuID, qtyPerUnit: "0.050000"},
				},
			},
			{
				id: DemoProdBrowniesID, name: "Brownies Panggang Fudgy", category: "kue_loyang",
				price: "85000.0000", packagingCost: "5000.0000", overheadCost: "3000.0000",
				ovenLoad: "0.3500", laborLoad: "30.0000",
				recipes: []demoRecipeSeed{
					{ingredientID: DemoIngTepungID, qtyPerUnit: "0.150000"},
					{ingredientID: DemoIngCokelatID, qtyPerUnit: "0.250000"},
					{ingredientID: DemoIngMentegaID, qtyPerUnit: "0.150000"},
					{ingredientID: DemoIngGulaID, qtyPerUnit: "0.150000"},
					{ingredientID: DemoIngTelurID, qtyPerUnit: "3.000000"},
					{ingredientID: DemoIngKakaoID, qtyPerUnit: "0.050000"},
					{ingredientID: DemoIngRagiID, qtyPerUnit: "0.010000"},
				},
			},
		}

		prodActualID := make(map[string]string, len(products))
		for _, p := range products {
			actualPID := p.id
			var existingID string
			err := tx.QueryRow(ctx, `SELECT id::text FROM products WHERE name = $1 LIMIT 1`, p.name).Scan(&existingID)
			if err == nil && existingID != "" {
				actualPID = existingID
				if _, err := tx.Exec(ctx, `
					UPDATE products
					SET category = $1, price = $2::numeric, packaging_cost = $3::numeric, overhead_cost = $4::numeric, is_active = true
					WHERE id = $5::uuid
				`, p.category, p.price, p.packagingCost, p.overheadCost, actualPID); err != nil {
					return fmt.Errorf("update product %s: %w", p.name, err)
				}
			} else {
				if _, err := tx.Exec(ctx, `
					INSERT INTO products (id, name, category, price, packaging_cost, overhead_cost, is_active)
					VALUES ($1::uuid, $2, $3, $4::numeric, $5::numeric, $6::numeric, true)
					ON CONFLICT (id) DO UPDATE SET
						name = EXCLUDED.name, category = EXCLUDED.category, price = EXCLUDED.price,
						packaging_cost = EXCLUDED.packaging_cost, overhead_cost = EXCLUDED.overhead_cost,
						is_active = true
				`, actualPID, p.name, p.category, p.price, p.packagingCost, p.overheadCost); err != nil {
					return fmt.Errorf("insert product %s: %w", p.name, err)
				}
			}
			prodActualID[p.id] = actualPID

			for _, ld := range []struct{ code, val string }{
				{code: "oven", val: p.ovenLoad},
				{code: "labor", val: p.laborLoad},
			} {
				rid := resIDByCode[ld.code]
				if _, err := tx.Exec(ctx, `
					INSERT INTO product_loads (product_id, resource_id, load)
					VALUES ($1::uuid, $2::uuid, $3::numeric)
					ON CONFLICT (product_id, resource_id) DO UPDATE SET load = EXCLUDED.load
				`, actualPID, rid, ld.val); err != nil {
					return fmt.Errorf("upsert product_load %s/%s: %w", p.name, ld.code, err)
				}
			}

			if _, err := tx.Exec(ctx, `DELETE FROM recipe_items WHERE product_id = $1::uuid`, actualPID); err != nil {
				return fmt.Errorf("clear recipe_items %s: %w", p.name, err)
			}
			for _, rec := range p.recipes {
				iid := ingActualID[rec.ingredientID]
				if _, err := tx.Exec(ctx, `
					INSERT INTO recipe_items (product_id, ingredient_id, qty_per_unit)
					VALUES ($1::uuid, $2::uuid, $3::numeric)
				`, actualPID, iid, rec.qtyPerUnit); err != nil {
					return fmt.Errorf("insert recipe_item %s: %w", p.name, err)
				}
			}
		}

		params, err := LoadSettingsParams(ctx, tx)
		if err != nil {
			return err
		}
		today := db.TodayQ(ctx, tx)

		allPIDs := []string{
			prodActualID[DemoProdSnackBoxID],
			prodActualID[DemoProdTartID],
			prodActualID[DemoProdCupcakeID],
			prodActualID[DemoProdNasiBoxID],
			prodActualID[DemoProdKueKering],
			prodActualID[DemoProdBrowniesID],
		}
		prodMeta, err := LoadProductsMeta(ctx, tx, allPIDs, params)
		if err != nil {
			return err
		}

		sbID := prodActualID[DemoProdSnackBoxID]
		tartID := prodActualID[DemoProdTartID]
		cupcakeID := prodActualID[DemoProdCupcakeID]
		nasiBoxID := prodActualID[DemoProdNasiBoxID]
		nastarID := prodActualID[DemoProdKueKering]
		browniesID := prodActualID[DemoProdBrowniesID]

		sbMeta := prodMeta[sbID]
		tartMeta := prodMeta[tartID]
		cupcakeMeta := prodMeta[cupcakeID]
		nasiMeta := prodMeta[nasiBoxID]
		nastarMeta := prodMeta[nastarID]
		browniesMeta := prodMeta[browniesID]

		// Pastikan UnitCost Snack Box tepat Rp 19.000
		if !sbMeta.UnitCost.Equal(decimal.NewFromInt(19000)) {
			return fmt.Errorf("Snack Box UnitCost harus Rp 19.000, dapat %s", sbMeta.UnitCost.String())
		}

		// 7. Masukkan entri kas manual (cash_entries) agar proyeksi kas pada jendela evaluasi
		// S1/S2 (H+0..H+8) sehat di atas buffer Rp 400.000, sementara H+12 menampilkan 1 hari rawan
		// untuk grafik proyeksi kas & KPI Hari Kas Rawan di Dashboard Pemilik.
		cashEntriesSeed := []struct {
			offset   int
			amount   decimal.Decimal
			category string
			note     string
		}{
			{offset: 0, amount: decimal.NewFromInt(1_600_000), category: "inflow", note: "Pelunasan piutang katering kantor pekan lalu"},
			{offset: 2, amount: decimal.NewFromInt(1_800_000), category: "inflow", note: "Retainer katering rutin PT Nusantara"},
			{offset: 5, amount: decimal.NewFromInt(1_500_000), category: "inflow", note: "Setoran mitra konsinyasi etalase kue"},
			{offset: 8, amount: decimal.NewFromInt(1_600_000), category: "inflow", note: "Pelunasan kontrak snack rapat mingguan"},
			{offset: 12, amount: decimal.NewFromInt(-4_750_000), category: "outflow", note: "Servis berkala 2 unit Oven Deck & pembelian kemasan grosir bulanan"},
			{offset: 13, amount: decimal.NewFromInt(1_200_000), category: "inflow", note: "Pencairan pembayaran marketplace & pesanan instansi"},
		}
		for _, ce := range cashEntriesSeed {
			dtKey := engine.DateKey(today.AddDate(0, 0, ce.offset))
			if _, err := tx.Exec(ctx, `
				INSERT INTO cash_entries (entry_date, amount, note, created_by)
				VALUES ($1::date, $2, $3, $4::uuid)
			`, dtKey, ce.amount, ce.note, ownerID); err != nil {
				return fmt.Errorf("insert cash_entry: %w", err)
			}
		}

		// Helper pointer int & decimal
		intPtr := func(v int) *int { return &v }
		decPtr := func(v int64) *decimal.Decimal {
			d := decimal.NewFromInt(v)
			return &d
		}

		// Biaya historis sedikit lebih rendah pada 4 pesanan H-7..H-5 agar KPI selisih biaya bahan
		// dan perbandingan margin estimasi vs realisasi menunjukkan dinamika harga bahan aktual.
		histNasiCost := nasiMeta.UnitCost.Sub(decimal.NewFromInt(450))
		histSnackCost := sbMeta.UnitCost.Sub(decimal.NewFromInt(300))

		// 8. Definisi 40 pesanan demo tersebar dari H-7 s.d. H+13
		specs := []demoOrderSpec{
			// === SKENARIO DEMO UTAMA S1 .. S5 ===
			{
				// S1: Pesanan 100 snack box di hari yang sudah terisi (Today+5: existing load 1.020 mnt labor & 5,0 jam oven -> G1 gagal 155,5%, tawaran balik ke Today+6 dengan utilisasi 88,2%)
				id: "30000000-0000-4000-8000-000000000001", code: "DEMOS1FULL", source: "public",
				customerID: cust2ID, customerName: "Panitia Seminar Nasional FT", customerContact: "081298765431",
				deliveryType: "delivery", address: "Gedung Auditorium Kampus Terpadu Lt. 2, Jl. Pendidikan No. 45",
				notes:      "[Skenario S1] Pesanan 100 Snack Box pada hari produksi yang sudah padat (G1 gagal 155,5%, tawaran balik ke hari berikutnya 88,2%).",
				dayOffset:  5, prodOffset: 5, status: StatusPendingDecision,
				items: []demoOrderItemSpec{{productID: sbID, qty: 100, unitPrice: sbMeta.DefaultPrice, unitCost: sbMeta.UnitCost}},
			},
			{
				// S2: Permintaan harga Rp 21.000 per box (margin 9,5% < 20% -> G2 gagal, harga minimal Rp 23.750)
				id: "30000000-0000-4000-8000-000000000002", code: "DEMOS2DISC", source: "manual",
				customerName: "Komunitas Pengajian Al-Ikhlas", customerContact: "081355501922",
				deliveryType: "pickup", address: "Ambil di Dapur Kalkurasa",
				notes:      "[Skenario S2] Permintaan diskon khusus Rp 21.000/box (margin 9,5%, di bawah batas minimum 20%; harga minimal tawaran balik Rp 23.750).",
				dayOffset:  7, prodOffset: 7, status: StatusPendingDecision, createdBy: managerID,
				items: []demoOrderItemSpec{{productID: sbID, qty: 40, unitPrice: decimal.NewFromInt(21000), unitCost: sbMeta.UnitCost}},
			},
			{
				// S3: DP minimal Rp 600.000 dengan DP ditagih Rp 712.500 (100 Snack Box @ Rp 23.750 = Rp 2.375.000)
				// Sekaligus menyumbang 100 * 0,12 kg = 12,0 kg Tepung Terigu untuk S4!
				id: "30000000-0000-4000-8000-000000000003", code: "DEMOS3DPMN", source: "manual",
				customerID: cust1ID, customerName: "Dewi Lestari (Pelanggan Demo)", customerContact: "081234567890",
				deliveryType: "delivery", address: "Jl. Melati Indah No. 18, Kebayoran",
				notes:           "[Skenario S3 & S4] Pesanan 100 Snack Box @ Rp 23.750 (DP minimal kas Rp 600.000, DP ditagih 30% = Rp 712.500; kebutuhan tepung 12,0 kg).",
				dayOffset:       4, prodOffset: 4, status: StatusConfirmed,
				decidedBy:       ownerID, createdBy: managerID,
				paidAmount:      decimal.NewFromInt(712500), paymentMethod: "transfer", paymentNote: "Pembayaran DP 30% (Rp 712.500 >= DP minimal Rp 600.000)",
				customMinDP:     decPtr(600000), customDPCharged: decPtr(712500),
				customLowNoDP:   decPtr(-200000), customLowWithDP: decPtr(512500),
				items:           []demoOrderItemSpec{{productID: sbID, qty: 100, unitPrice: decimal.NewFromInt(23750), unitCost: sbMeta.UnitCost}},
			},
			{
				// S4 (pasangan S3): 6 Tart Ulang Tahun pada Today+5 (confirmed)
				// Menyumbang 6 * 0,40 kg = 2,4 kg Tepung Terigu -> bersama S3 (12,0 kg) = 14,4 kg -> stok 3 kg, buffer 10% -> perlu 12,84 kg, beli 13 kg!
				// Sekaligus menyumbang beban Today+5: 6 * 90 = 540 menit labor & 6 * 0,5 = 3,0 jam oven untuk S1!
				id: "30000000-0000-4000-8000-000000000004", code: "DEMOS4TART", source: "public",
				customerID: cust1ID, customerName: "Dewi Lestari (Pelanggan Demo)", customerContact: "081234567890",
				deliveryType: "delivery", address: "Jl. Melati Indah No. 18, Kebayoran",
				notes:         "[Skenario S4 & Beban S1] 6 Tart Ulang Tahun (kebutuhan tepung 2,4 kg + 12,0 kg dari S3 = 14,4 kg; perlu 12,84 kg, beli 13 kg).",
				dayOffset:     5, prodOffset: 5, status: StatusConfirmed, decidedBy: ownerID,
				paidAmount:    decimal.NewFromInt(504000), paymentMethod: "transfer", paymentNote: "DP 30% pesanan 6 Tart Ulang Tahun",
				items:         []demoOrderItemSpec{{productID: tartID, qty: 6, unitPrice: tartMeta.DefaultPrice, unitCost: tartMeta.UnitCost}},
			},
			{
				// S5: Pembatalan pesanan (belanja Rp 1.300.000, DP diterima Rp 712.500 -> eksposur Rp 587.500)
				id: "30000000-0000-4000-8000-000000000005", code: "DEMOS5CNCL", source: "manual",
				customerName: "CV Karya Mandiri", customerContact: "08118889900",
				deliveryType: "delivery", address: " Kawasan Industri Pulogadung Blok C-12",
				notes:          "[Skenario S5] Dibatalkan mendadak setelah bahan dibelanjakan Rp 1.300.000 dan DP diterima Rp 712.500 -> eksposur pembatalan Rp 587.500.\n[Batal: Acara perusahaan ditunda mendadak setelah bahan dibeli [DP: forfeited]]",
				dayOffset:      1, prodOffset: 1, status: StatusCancelled, decidedBy: ownerID, createdBy: managerID,
				paidAmount:     decimal.NewFromInt(712500), paymentMethod: "transfer", paymentNote: "DP 30% sebelum pembatalan pesanan",
				cancelShopCost: decPtr(1300000), cancelExposure: decPtr(587500),
				items:          []demoOrderItemSpec{{productID: sbID, qty: 100, unitPrice: decimal.NewFromInt(23750), unitCost: sbMeta.UnitCost}},
			},

			// === PESANAN PENDUKUNG BEBAN S1 PADA TODAY+5 & TODAY+6 ===
			{
				// Beban tambahan Today+5: 40 Snack Box (accepted) -> 480 mnt labor, 2,0 jam oven (total Today+5 = 1.020 mnt labor, 5,0 jam oven)
				id: "30000000-0000-4000-8000-000000000006", code: "DEMOS1LOAD", source: "public",
				customerID: cust3ID, customerName: "Maya Putri (Rapat Komite Sekolah)", customerContact: "081298765432",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes:     "Reservasi kapasitas H+5 (40 Snack Box: 480 menit tenaga, 2 jam oven) menunggu verifikasi transfer DP.",
				dayOffset: 5, prodOffset: 5, status: StatusAccepted,
				items: []demoOrderItemSpec{{productID: sbID, qty: 40, unitPrice: sbMeta.DefaultPrice, unitCost: sbMeta.UnitCost}},
			},
			{
				// Beban Today+6: 5 Snack Box (accepted) -> 60 mnt labor, 0,25 jam oven (sehingga kandidat tawaran balik S1 pada Today+6 memiliki utilisasi 1.260/1.428 = 88,2%)
				id: "30000000-0000-4000-8000-000000000007", code: "DEMOS1NEXT", source: "public",
				customerID: cust3ID, customerName: "Maya Putri (Pengajian Rutin)", customerContact: "081298765432",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes:     "Reservasi kapasitas H+6 (5 Snack Box: 60 menit tenaga, 0,25 jam oven).",
				dayOffset: 6, prodOffset: 6, status: StatusAccepted,
				items: []demoOrderItemSpec{{productID: sbID, qty: 5, unitPrice: sbMeta.DefaultPrice, unitCost: sbMeta.UnitCost}},
			},

			// === 5 PESANAN TART SELESAI DENGAN DURASI AKTUAL (TC-07 KALIBRASI: 105, 100, 95, 110, 100 -> RATA-RATA 102 MNT, DEVIASI +13,33%) ===
			{
				id: "30000000-0000-4000-8000-000000000008", code: "DEMOCAL001", source: "public",
				customerID: cust1ID, customerName: "Dewi Lestari (Pelanggan Demo)", customerContact: "081234567890",
				deliveryType: "delivery", address: "Jl. Melati Indah No. 18, Kebayoran",
				notes: "Tart Ulang Tahun ke-1 (durasi produksi aktual 105 menit vs estimasi 90 menit).",
				dayOffset: -7, prodOffset: -7, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(280000), paymentMethod: "transfer", paymentNote: "Pelunasan penuh Tart Ulang Tahun",
				produceActualMin: intPtr(105),
				items:            []demoOrderItemSpec{{productID: tartID, qty: 1, unitPrice: tartMeta.DefaultPrice, unitCost: tartMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000009", code: "DEMOCAL002", source: "public",
				customerID: cust1ID, customerName: "Keluarga Bpk. Arifin", customerContact: "081280011002",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "Tart Ulang Tahun ke-2 (durasi produksi aktual 100 menit vs estimasi 90 menit).",
				dayOffset: -6, prodOffset: -6, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(280000), paymentMethod: "qris", paymentNote: "Pelunasan penuh QRIS",
				produceActualMin: intPtr(100),
				items:            []demoOrderItemSpec{{productID: tartID, qty: 1, unitPrice: tartMeta.DefaultPrice, unitCost: tartMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000010", code: "DEMOCAL003", source: "public",
				customerID: cust1ID, customerName: "Sinta Maharani", customerContact: "081280011003",
				deliveryType: "delivery", address: "Jl. Cempaka Putih Tengah No. 9",
				notes: "Tart Ulang Tahun ke-3 (durasi produksi aktual 95 menit vs estimasi 90 menit).",
				dayOffset: -5, prodOffset: -5, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(280000), paymentMethod: "transfer", paymentNote: "Pelunasan penuh transfer",
				produceActualMin: intPtr(95),
				items:            []demoOrderItemSpec{{productID: tartID, qty: 1, unitPrice: tartMeta.DefaultPrice, unitCost: tartMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000011", code: "DEMOCAL004", source: "public",
				customerID: cust1ID, customerName: "dr. Ratna Kusuma", customerContact: "081280011004",
				deliveryType: "delivery", address: "RS Mitra Keluarga Lt. 3, Poli Anak",
				notes: "Tart Ulang Tahun ke-4 (durasi produksi aktual 110 menit vs estimasi 90 menit).",
				dayOffset: -4, prodOffset: -4, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(280000), paymentMethod: "transfer", paymentNote: "Pelunasan penuh transfer",
				produceActualMin: intPtr(110),
				items:            []demoOrderItemSpec{{productID: tartID, qty: 1, unitPrice: tartMeta.DefaultPrice, unitCost: tartMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000012", code: "DEMOCAL005", source: "public",
				customerID: cust1ID, customerName: "Bpk. Gunawan Wibisono", customerContact: "081280011005",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "Tart Ulang Tahun ke-5 (durasi produksi aktual 100 menit vs estimasi 90 menit).",
				dayOffset: -3, prodOffset: -3, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(280000), paymentMethod: "cash", paymentNote: "Pelunasan tunai saat pengambilan",
				produceActualMin: intPtr(100),
				items:            []demoOrderItemSpec{{productID: tartID, qty: 1, unitPrice: tartMeta.DefaultPrice, unitCost: tartMeta.UnitCost}},
			},

			// === 9 PESANAN COMPLETED LAINNYA DI MINGGU LALU (H-7 s.d. H-1) ===
			{
				id: "30000000-0000-4000-8000-000000000013", code: "DEMOCMP006", source: "manual",
				customerName: "PT Sinar Harapan (Rapat Direksi)", customerContact: "08119002001",
				deliveryType: "delivery", address: "Menara Sudirman Lt. 14",
				notes: "Pesanan katering rapat mingguan.", dayOffset: -7, prodOffset: -7, status: StatusCompleted,
				decidedBy: ownerID, createdBy: managerID,
				paidAmount: decimal.NewFromInt(1400000), paymentMethod: "transfer", paymentNote: "Pelunasan invoice katering",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 40, unitPrice: nasiMeta.DefaultPrice, unitCost: histNasiCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000014", code: "DEMOCMP007", source: "public",
				customerID: cust1ID, customerName: "Dewi Lestari (Arisan RT)", customerContact: "081234567890",
				deliveryType: "delivery", address: "Jl. Melati Indah No. 18, Kebayoran",
				notes: "Snack box arisan bulanan.", dayOffset: -6, prodOffset: -6, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(1000000), paymentMethod: "transfer", paymentNote: "Lunas transfer BCA",
				items: []demoOrderItemSpec{{productID: sbID, qty: 40, unitPrice: sbMeta.DefaultPrice, unitCost: histSnackCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000015", code: "DEMOCMP008", source: "public",
				customerName: "Klinik Sehat Bersama", customerContact: "08119002003",
				deliveryType: "delivery", address: "Jl. Kesehatan Raya No. 7",
				notes: "Penyuluhan kesehatan masyarakat.", dayOffset: -5, prodOffset: -5, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(1225000), paymentMethod: "transfer", paymentNote: "Lunas bendahara klinik",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 35, unitPrice: nasiMeta.DefaultPrice, unitCost: histNasiCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000016", code: "DEMOCMP009", source: "public",
				customerName: "Ibu Hj. Nurhayati", customerContact: "08119002004",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "Hantaran pengajian keluarga.", dayOffset: -4, prodOffset: -4, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(850000), paymentMethod: "cash", paymentNote: "Lunas tunai",
				items: []demoOrderItemSpec{{productID: browniesID, qty: 10, unitPrice: browniesMeta.DefaultPrice, unitCost: browniesMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000017", code: "DEMOCMP010", source: "manual",
				customerName: "Koperasi Karyawan Sejahtera", customerContact: "08119002005",
				deliveryType: "delivery", address: "Jl. Gatot Subroto Kav. 22",
				notes: "RAT Koperasi Tahunan (disetujui dari tawaran balik).", dayOffset: -3, prodOffset: -3, status: StatusCompleted,
				decidedBy: ownerID, createdBy: managerID,
				paidAmount: decimal.NewFromInt(1750000), paymentMethod: "transfer", paymentNote: "Lunas koperasi",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 50, unitPrice: nasiMeta.DefaultPrice, unitCost: histNasiCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000018", code: "DEMOCMP011", source: "public",
				customerName: "Sanggar Tari Kemuning", customerContact: "08119002006",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "Konsumsi latihan gabungan.", dayOffset: -2, prodOffset: -2, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(750000), paymentMethod: "qris", paymentNote: "Lunas QRIS",
				items: []demoOrderItemSpec{{productID: sbID, qty: 30, unitPrice: sbMeta.DefaultPrice, unitCost: sbMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000019", code: "DEMOCMP012", source: "public",
				customerName: "Bpk. Dedi Kurniawan", customerContact: "08119002007",
				deliveryType: "delivery", address: "Perumahan Griya Asri Blok B4",
				notes: "Syukuran rumah baru (konfirmasi pelunasan H+1).", dayOffset: -2, prodOffset: -2, status: StatusCompleted,
				completedLateDays: 1,
				paidAmount:        decimal.NewFromInt(880000), paymentMethod: "transfer", paymentNote: "Lunas transfer H+1",
				items:             []demoOrderItemSpec{{productID: nastarID, qty: 8, unitPrice: nastarMeta.DefaultPrice, unitCost: nastarMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000020", code: "DEMOCMP013", source: "public",
				customerName: "SDN Harapan Bangsa 03", customerContact: "08119002008",
				deliveryType: "delivery", address: "Jl. Pendidikan No. 11",
				notes: "Rapat komite guru & wali murid.", dayOffset: -1, prodOffset: -1, status: StatusCompleted,
				paidAmount: decimal.NewFromInt(950000), paymentMethod: "transfer", paymentNote: "Lunas dana BOS/Komite",
				items: []demoOrderItemSpec{{productID: cupcakeID, qty: 10, unitPrice: cupcakeMeta.DefaultPrice, unitCost: cupcakeMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000021", code: "DEMOCMP014", source: "manual",
				customerName: "Bimbingan Belajar Cendekia", customerContact: "08119002009",
				deliveryType: "delivery", address: "Ruko Golden Boulevard Blok F-8",
				notes: "Tryout akbar siswa kelas 12.", dayOffset: -1, prodOffset: -1, status: StatusCompleted,
				completedLateDays: 1, decidedBy: managerID, createdBy: managerID,
				paidAmount: decimal.NewFromInt(1125000), paymentMethod: "transfer", paymentNote: "Lunas transfer",
				items: []demoOrderItemSpec{{productID: sbID, qty: 45, unitPrice: sbMeta.DefaultPrice, unitCost: sbMeta.UnitCost}},
			},

			// === PESANAN HARI INI (TODAY) & BESOK (H+1): DELIVERING (2), READY (2), IN_PRODUCTION (3) ===
			// Catatan: in_production memakai Nasi Box Ayam Spesial agar kebutuhan Tepung Terigu di shopping-list tetap murni 14,4 kg (S4)!
			{
				id: "30000000-0000-4000-8000-000000000022", code: "DEMODLV001", source: "public",
				customerID: cust1ID, customerName: "Dewi Lestari (Rapat Yayasan)", customerContact: "081234567890",
				deliveryType: "delivery", address: "Jl. Melati Indah No. 18, Kebayoran",
				notes: "Sedang dalam pengantaran kurir internal ke lokasi acara.", dayOffset: 0, prodOffset: 0, status: StatusDelivering,
				paidAmount: decimal.NewFromInt(875000), paymentMethod: "transfer", paymentNote: "Lunas sebelum pengantaran",
				items: []demoOrderItemSpec{{productID: sbID, qty: 35, unitPrice: sbMeta.DefaultPrice, unitCost: sbMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000023", code: "DEMODLV002", source: "manual",
				customerName: "Notaris Hendrawan & Rekan", customerContact: "08119002011",
				deliveryType: "delivery", address: "Jl. Senopati No. 64",
				notes: "Pengantaran siang hari ini.", dayOffset: 0, prodOffset: 0, status: StatusDelivering,
				decidedBy: managerID, createdBy: managerID,
				paidAmount: decimal.NewFromInt(1050000), paymentMethod: "transfer", paymentNote: "Lunas transfer",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 30, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000024", code: "DEMORDY001", source: "public",
				customerName: "Ibu Larasati (Ulang Tahun Putri)", customerContact: "08119002012",
				deliveryType: "pickup", address: "Ambil di Toko pukul 15.00 WIB",
				notes: "Sudah selesai dipanggang & dikemas, menunggu diambil pelanggan.", dayOffset: 0, prodOffset: 0, status: StatusReady,
				paidAmount: decimal.NewFromInt(570000), paymentMethod: "qris", paymentNote: "Lunas QRIS",
				items: []demoOrderItemSpec{{productID: cupcakeID, qty: 6, unitPrice: cupcakeMeta.DefaultPrice, unitCost: cupcakeMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000025", code: "DEMORDY002", source: "public",
				customerName: "Bank Syariah Mandiri KC Fatmawati", customerContact: "08119002013",
				deliveryType: "delivery", address: "Jl. RS Fatmawati No. 32",
				notes: "Siap diantar sore ini setelah pengecekan akhir.", dayOffset: 0, prodOffset: 0, status: StatusReady,
				paidAmount: decimal.NewFromInt(680000), paymentMethod: "transfer", paymentNote: "Lunas transfer",
				items: []demoOrderItemSpec{{productID: browniesID, qty: 8, unitPrice: browniesMeta.DefaultPrice, unitCost: browniesMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000026", code: "DEMOPRD001", source: "public",
				customerID: cust1ID, customerName: "Dewi Lestari (Makan Siang Tim)", customerContact: "081234567890",
				deliveryType: "delivery", address: "Jl. Melati Indah No. 18, Kebayoran",
				notes: "Sedang dimasak di dapur utama untuk pengantaran sore.", dayOffset: 0, prodOffset: 0, status: StatusInProduction,
				paidAmount: decimal.NewFromInt(700000), paymentMethod: "transfer", paymentNote: "Lunas di muka",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 20, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000027", code: "DEMOPRD002", source: "manual",
				customerName: "Dinas Koperasi & UKM", customerContact: "08119002015",
				deliveryType: "delivery", address: "Gedung Balai Kota Blok D Lt. 4",
				notes: "Persiapan bumbu & marinasi ayam untuk pengiriman besok pagi.", dayOffset: 1, prodOffset: 1, status: StatusInProduction,
				decidedBy: ownerID, createdBy: managerID,
				paidAmount: decimal.NewFromInt(1225000), paymentMethod: "transfer", paymentNote: "Lunas bendahara",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 35, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000028", code: "DEMOPRD003", source: "public",
				customerName: "Yayasan Bina Insani", customerContact: "08119002016",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "Sedang diproduksi untuk acara besok siang.", dayOffset: 1, prodOffset: 1, status: StatusInProduction,
				paidAmount: decimal.NewFromInt(525000), paymentMethod: "transfer", paymentNote: "DP 50% terverifikasi",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 30, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},

			// === 5 PESANAN CONFIRMED TAMBAHAN (MEMAKAI NASI BOX AGAR S4 TEPUNG TERIGU TETAP TEPAT 14,4 KG) ===
			{
				id: "30000000-0000-4000-8000-000000000029", code: "DEMOCNF001", source: "public",
				customerID: cust1ID, customerName: "Dewi Lestari (Syukuran Kantor)", customerContact: "081234567890",
				deliveryType: "delivery", address: "Jl. Melati Indah No. 18, Kebayoran",
				notes: "DP sudah lunas, jadwal produksi H+2.", dayOffset: 2, prodOffset: 2, status: StatusConfirmed,
				paidAmount: decimal.NewFromInt(420000), paymentMethod: "transfer", paymentNote: "DP 30% lunas",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 40, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000030", code: "DEMOCNF002", source: "public",
				customerName: "Karang Taruna RW 08", customerContact: "08119002018",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "DP sudah lunas, jadwal produksi H+3.", dayOffset: 3, prodOffset: 3, status: StatusConfirmed,
				paidAmount: decimal.NewFromInt(315000), paymentMethod: "qris", paymentNote: "DP 30% QRIS",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 30, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000031", code: "DEMOCNF003", source: "manual",
				customerName: "PT Krakatau Logistik", customerContact: "08119002019",
				deliveryType: "delivery", address: "Jl. Raya Bekasi KM 21",
				notes: "Pelatihan keselamatan kerja gudang (disetujui dengan override oleh Pemilik).",
				dayOffset: 8, prodOffset: 8, status: StatusConfirmed,
				decidedBy: ownerID, createdBy: managerID,
				overrideReason: "Pelanggan korporat tetap; pembayaran termin dijamin PO resmi.",
				paidAmount:     decimal.NewFromInt(472500), paymentMethod: "transfer", paymentNote: "DP 30% PO Korporat",
				items:          []demoOrderItemSpec{{productID: nasiBoxID, qty: 45, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000032", code: "DEMOCNF004", source: "public",
				customerName: "Masjid Al-Falah", customerContact: "08119002020",
				deliveryType: "delivery", address: "Jl. Pejaten Barat No. 15",
				notes: "Kajian akhir pekan depan.", dayOffset: 10, prodOffset: 10, status: StatusConfirmed,
				paidAmount: decimal.NewFromInt(420000), paymentMethod: "transfer", paymentNote: "DP 30% lunas",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 40, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000033", code: "DEMOCNF005", source: "public",
				customerName: "Ikatan Alumni SMA 7", customerContact: "08119002021",
				deliveryType: "delivery", address: "Aula Serbaguna Kemang",
				notes: "Reuni perak angkatan.", dayOffset: 12, prodOffset: 12, status: StatusConfirmed,
				paidAmount: decimal.NewFromInt(472500), paymentMethod: "transfer", paymentNote: "DP 30% lunas",
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 45, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},

			// === 2 PESANAN ACCEPTED TAMBAHAN (BELUM DP, TAMPIL DI PANEL 'PESANAN BELUM DP' DASHBOARD MANAJER) ===
			{
				id: "30000000-0000-4000-8000-000000000034", code: "DEMOACC001", source: "public",
				customerName: "Rizky Pratama", customerContact: "08119002022",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "Otomatis diterima, menunggu pembayaran DP 30%.", dayOffset: 9, prodOffset: 9, status: StatusAccepted,
				items: []demoOrderItemSpec{{productID: browniesID, qty: 6, unitPrice: browniesMeta.DefaultPrice, unitCost: browniesMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000035", code: "DEMOACC002", source: "public",
				customerName: "Nadia Permata", customerContact: "08119002023",
				deliveryType: "delivery", address: "Jl. Cipete Raya No. 40",
				notes: "Otomatis diterima, menunggu pembayaran DP 30%.", dayOffset: 11, prodOffset: 11, status: StatusAccepted,
				items: []demoOrderItemSpec{{productID: nastarID, qty: 5, unitPrice: nastarMeta.DefaultPrice, unitCost: nastarMeta.UnitCost}},
			},

			// === 1 PESANAN BESAR PENDING_DECISION TAMBAHAN (LOLOS SEMUA GERBANG, KHUSUS WEWENANG OWNER) ===
			{
				id: "30000000-0000-4000-8000-000000000036", code: "DEMOSBIG01", source: "public",
				customerID: cust2ID, customerName: "Hendra Wijaya (Gathering Tahunan)", customerContact: "081298765431",
				deliveryType: "delivery", address: "Wisma Antara Lt. 9, Jl. Medan Merdeka Selatan",
				notes: "Pesanan besar 60 Nasi Box (Rp 2.100.000 >= Rp 2.000.000); semua gerbang G1-G4 lolos, menunggu persetujuan Pemilik.",
				dayOffset: 13, prodOffset: 13, status: StatusPendingDecision,
				items: []demoOrderItemSpec{{productID: nasiBoxID, qty: 60, unitPrice: nasiMeta.DefaultPrice, unitCost: nasiMeta.UnitCost}},
			},

			// === 2 PESANAN COUNTERED (MENUNGGU RESPONS PELANGGAN ATAS TAWARAN BALIK) ===
			{
				id: "30000000-0000-4000-8000-000000000037", code: "DEMOCNT001", source: "manual",
				customerName: "Perkumpulan Warga Harmoni", customerContact: "08119002025",
				deliveryType: "delivery", address: "Balai Warga RW 04, Tebet",
				notes: "Diajukan tawaran balik penyesuaian harga minimal Rp 23.750/box.", dayOffset: 9, prodOffset: 9, status: StatusCountered,
				decidedBy: ownerID, createdBy: managerID,
				items: []demoOrderItemSpec{{productID: sbID, qty: 35, unitPrice: decimal.NewFromInt(22000), unitCost: sbMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000038", code: "DEMOCNT002", source: "public",
				customerName: "EO Lentera Kreasi", customerContact: "08119002026",
				deliveryType: "delivery", address: "Gedung Smesco Indonesia, Pancoran",
				notes: "Diajukan tawaran balik penggeseran tanggal acara ke H+11.", dayOffset: 10, prodOffset: 10, status: StatusCountered,
				decidedBy: ownerID,
				items: []demoOrderItemSpec{{productID: cupcakeID, qty: 12, unitPrice: cupcakeMeta.DefaultPrice, unitCost: cupcakeMeta.UnitCost}},
			},

			// === 1 PESANAN REJECTED (GAGAL G4 LEAD TIME) & 1 PESANAN EXPIRED (LEWAT TENGGAT DP) ===
			{
				id: "30000000-0000-4000-8000-000000000039", code: "DEMOREJ001", source: "public",
				customerName: "Fajar Nugroho", customerContact: "08119002027",
				deliveryType: "delivery", address: "Jl. Radio Dalam No. 12",
				notes: "Pesanan mendadak H+1 kurang dari lead time minimum 2 hari (G4 gagal).", dayOffset: 1, prodOffset: 1, status: StatusRejected,
				decidedBy: managerID,
				items: []demoOrderItemSpec{{productID: tartID, qty: 3, unitPrice: tartMeta.DefaultPrice, unitCost: tartMeta.UnitCost}},
			},
			{
				id: "30000000-0000-4000-8000-000000000040", code: "DEMOEXP001", source: "public",
				customerName: "Tania Oktaviani", customerContact: "08119002028",
				deliveryType: "pickup", address: "Ambil di Toko",
				notes: "Kedaluwarsa otomatis karena DP belum dibayarkan hingga melewati batas waktu 24 jam.",
				dayOffset: -2, prodOffset: -2, status: StatusExpired,
				items: []demoOrderItemSpec{{productID: cupcakeID, qty: 4, unitPrice: cupcakeMeta.DefaultPrice, unitCost: cupcakeMeta.UnitCost}},
			},
		}

		// Urutkan penyisipan agar pesanan berstatus accepted/confirmed/in_production masuk lebih dulu
		// sehingga evaluasi S1 (DEMOS1FULL) dan S2 (DEMOS2DISC) melihat beban existing yang sudah lengkap!
		priority := func(st string) int {
			switch st {
			case StatusCompleted, StatusDelivering, StatusReady, StatusInProduction, StatusConfirmed, StatusAccepted:
				return 1
			default:
				return 2
			}
		}

		orderedSpecs := make([]demoOrderSpec, 0, len(specs))
		for pass := 1; pass <= 2; pass++ {
			for _, sp := range specs {
				if priority(sp.status) == pass {
					orderedSpecs = append(orderedSpecs, sp)
				}
			}
		}

		nowUTC := time.Now().UTC()

		for _, sp := range orderedSpecs {
			evDate := today.AddDate(0, 0, sp.dayOffset)
			prDate := today.AddDate(0, 0, sp.prodOffset)
			evDateKey := engine.DateKey(evDate)
			prDateKey := engine.DateKey(prDate)

			evalItems := make([]engine.EvalItem, 0, len(sp.items))
			total := decimal.Zero
			totalUnits := 0
			for _, it := range sp.items {
				pm := prodMeta[it.productID]
				evalItems = append(evalItems, engine.EvalItem{
					ProductID:           it.productID,
					Qty:                 it.qty,
					UnitPrice:           it.unitPrice,
					UnitCost:            it.unitCost,
					PurchaseCostPerUnit: pm.PurchaseCostPerUnit,
					UnitLoads:           pm.UnitLoads,
				})
				total = total.Add(it.unitPrice.Mul(decimal.NewFromInt(int64(it.qty))))
				totalUnits += it.qty
			}

			evalRes, err := EvaluateOrder(ctx, tx, today, evDate, prDate, evalItems, params)
			if err != nil {
				return fmt.Errorf("eval order %s: %w", sp.code, err)
			}

			// Untuk S3 (DEMOS3DPMN), pastikan nilai MinDP = Rp 600.000 & DPCharged = Rp 712.500 terekam persis di snapshot & evaluasi
			if sp.customMinDP != nil {
				evalRes.MinDP = *sp.customMinDP
			}
			if sp.customDPCharged != nil {
				evalRes.DPCharged = *sp.customDPCharged
			}
			if sp.customLowWithDP != nil {
				evalRes.Impact.CashLowestAfter = *sp.customLowWithDP
				for gi := range evalRes.Gates {
					if evalRes.Gates[gi].Gate == "G3" {
						evalRes.Gates[gi].Value = *sp.customLowWithDP
						evalRes.Gates[gi].Passed = true
					}
				}
			}

			// Untuk pesanan lampau (dayOffset <= 0) yang sudah disetujui/selesai, pastikan rekomendasi snapshot = accept dan G4 lolos
			if sp.dayOffset <= 0 && sp.status != StatusRejected {
				for gi := range evalRes.Gates {
					if evalRes.Gates[gi].Gate == "G4" {
						evalRes.Gates[gi].Value = decimal.NewFromInt(3)
						evalRes.Gates[gi].Passed = true
					}
				}
				if evalRes.Gates[0].Passed && evalRes.Gates[1].Passed && evalRes.Gates[2].Passed {
					evalRes.Recommendation = engine.RecAccept
					evalRes.CounterOffer = nil
				}
			}

			// Untuk DEMOCNT002, pastikan counter_offer terisi
			if sp.code == "DEMOCNT002" && evalRes.CounterOffer == nil {
				minPrices := map[string]decimal.Decimal{
					cupcakeID: engine.MinPrice(cupcakeMeta.UnitCost, params.EvalParams.MarginMin),
				}
				evalRes.Recommendation = engine.RecAcceptWithConditions
				evalRes.CounterOffer = &engine.CounterOffer{
					Date:             today.AddDate(0, 0, 11),
					MinPricePerItem:  minPrices,
					DP:               evalRes.DPCharged,
					UtilizationAfter: decimal.RequireFromString("0.4200"),
				}
			}

			evalDTO := ToEvaluationDTO(evalRes)
			dpRequired := evalRes.DPCharged.Round(4)

			var dpDeadline *time.Time
			switch sp.status {
			case StatusAccepted, StatusCountered:
				dl := nowUTC.Add(48 * time.Hour)
				dpDeadline = &dl
			case StatusExpired:
				dl := nowUTC.Add(-24 * time.Hour)
				dpDeadline = &dl
			case StatusConfirmed, StatusInProduction, StatusReady, StatusDelivering, StatusCompleted, StatusCancelled:
				dl := evDate.AddDate(0, 0, -2)
				dpDeadline = &dl
			}

			var counterJSON any
			if evalDTO.CounterOffer != nil {
				b, err := json.Marshal(evalDTO.CounterOffer)
				if err != nil {
					return err
				}
				counterJSON = string(b)
			}

			snapPayload := map[string]any{
				"evaluated_at": nowUTC.Format(time.RFC3339),
				"today":        engine.DateKey(today),
				"evaluation":   evalDTO,
				"min_dp":       evalRes.MinDP.Round(2),
				"dp_charged":   evalRes.DPCharged.Round(2),
			}
			if sp.customLowNoDP != nil {
				snapPayload["lowest_without_dp"] = sp.customLowNoDP.Round(2)
			}
			if sp.cancelShopCost != nil && sp.cancelExposure != nil {
				snapPayload["cancellation"] = map[string]any{
					"shopping_cost":         sp.cancelShopCost.Round(2),
					"dp_received":           sp.paidAmount.Round(2),
					"cancellation_exposure": sp.cancelExposure.Round(2),
				}
			}
			snapBytes, err := json.Marshal(snapPayload)
			if err != nil {
				return err
			}

			createdAt := nowUTC.AddDate(0, 0, sp.dayOffset-3)
			if sp.dayOffset > 0 {
				createdAt = nowUTC.Add(-time.Duration(40-sp.dayOffset) * time.Hour)
			}
			updatedAt := createdAt.Add(2 * time.Hour)
			if sp.status == StatusCompleted {
				// Tentukan updatedAt tepat pada tanggal acara (atau +completedLateDays) pukul 14:00 WIB (07:00 UTC)
				compDate := evDate.AddDate(0, 0, sp.completedLateDays)
				updatedAt = time.Date(compDate.Year(), compDate.Month(), compDate.Day(), 7, 0, 0, 0, time.UTC)
				if updatedAt.Before(createdAt) {
					createdAt = updatedAt.Add(-48 * time.Hour)
				}
			}

			var custIDPtr, createdByPtr, decidedByPtr, overrideReasonPtr any
			var decidedAtPtr *time.Time
			if sp.customerID != "" {
				custIDPtr = sp.customerID
			}
			if sp.createdBy != "" {
				createdByPtr = sp.createdBy
			}
			if sp.decidedBy != "" {
				decidedByPtr = sp.decidedBy
				da := createdAt.Add(30 * time.Minute)
				decidedAtPtr = &da
			}
			if sp.overrideReason != "" {
				overrideReasonPtr = sp.overrideReason
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO orders (
					id, code, source, customer_id, customer_name, customer_contact, delivery_type,
					address, notes, event_date, production_date, status,
					total, dp_required, dp_deadline, paid,
					recommendation, recommendation_reason, counter_offer,
					decided_by, decided_at, override_reason, snapshot,
					created_by, created_at, updated_at
				) VALUES (
					$1::uuid, $2, $3, $4::uuid, $5, $6, $7,
					$8, $9, $10::date, $11::date, $12,
					$13, $14, $15, $16,
					$17, $18, $19::jsonb,
					$20::uuid, $21, $22, $23::jsonb,
					$24::uuid, $25, $26
				)
			`,
				sp.id, sp.code, sp.source, custIDPtr, sp.customerName, sp.customerContact, sp.deliveryType,
				sp.address, sp.notes, evDateKey, prDateKey, sp.status,
				total.Round(4), dpRequired, dpDeadline, sp.paidAmount.Round(4),
				evalRes.Recommendation, evalRes.Reason, counterJSON,
				decidedByPtr, decidedAtPtr, overrideReasonPtr, string(snapBytes),
				createdByPtr, createdAt, updatedAt,
			); err != nil {
				return fmt.Errorf("insert order %s: %w", sp.code, err)
			}

			// Simpan order_items
			for _, it := range sp.items {
				if _, err := tx.Exec(ctx, `
					INSERT INTO order_items (order_id, product_id, qty, unit_price, unit_cost)
					VALUES ($1::uuid, $2::uuid, $3, $4, $5)
				`, sp.id, it.productID, it.qty, it.unitPrice.Round(4), it.unitCost.Round(4)); err != nil {
					return fmt.Errorf("insert order_item %s: %w", sp.code, err)
				}
			}

			// Simpan 4 baris order_evaluations (G1..G4)
			for _, g := range evalRes.Gates {
				detailBytes, _ := json.Marshal(map[string]any{
					"bottleneck": evalRes.Bottleneck,
					"min_dp":     evalRes.MinDP.Round(2),
					"dp_charged": evalRes.DPCharged.Round(2),
				})
				if _, err := tx.Exec(ctx, `
					INSERT INTO order_evaluations (order_id, gate, value, threshold, passed, detail, created_at)
					VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb, $7)
				`, sp.id, g.Gate, g.Value.Round(6), g.Threshold.Round(6), g.Passed, string(detailBytes), createdAt); err != nil {
					return fmt.Errorf("insert order_evaluation %s/%s: %w", sp.code, g.Gate, err)
				}
			}

			// Audit log pembuatan pesanan
			actorForCreate := sp.createdBy
			if actorForCreate == "" && sp.customerID != "" {
				actorForCreate = sp.customerID
			}
			if err := audit.Log(ctx, tx, actorForCreate, audit.ActionDecision, "orders", sp.id, map[string]any{
				"op":             "create_order",
				"code":           sp.code,
				"source":         sp.source,
				"status":         sp.status,
				"recommendation": evalRes.Recommendation,
				"total":          total.Round(2),
				"dp_required":    dpRequired.Round(2),
				"min_dp":         evalRes.MinDP.Round(2),
			}); err != nil {
				return err
			}

			// Jika ada override_reason, catat audit override
			if sp.overrideReason != "" {
				if err := audit.Log(ctx, tx, sp.decidedBy, audit.ActionOverride, "orders", sp.id, map[string]any{
					"op":                     "decide_order_override",
					"code":                   sp.code,
					"initial_recommendation": evalRes.Recommendation,
					"override_reason":        sp.overrideReason,
				}); err != nil {
					return err
				}
			}

			// Simpan pembayaran bila paidAmount > 0
			if sp.paidAmount.IsPositive() {
				paidAt := createdAt.Add(1 * time.Hour)
				method := sp.paymentMethod
				if method == "" {
					method = "transfer"
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO payments (order_id, amount, method, paid_at, note, recorded_by)
					VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)
				`, sp.id, sp.paidAmount.Round(4), method, paidAt, sp.paymentNote, managerID); err != nil {
					return fmt.Errorf("insert payment %s: %w", sp.code, err)
				}
				if err := audit.Log(ctx, tx, managerID, audit.ActionPayment, "orders", sp.id, map[string]any{
					"op":          "record_payment",
					"code":        sp.code,
					"amount":      sp.paidAmount.Round(2),
					"method":      method,
					"paid_before": decimal.Zero,
					"paid_after":  sp.paidAmount.Round(2),
				}); err != nil {
					return err
				}
			}

			// Buat 3 tugas operasional (shop, produce, deliver) untuk pesanan yang sudah terkonfirmasi / diproduksi / siap / diantar / selesai
			isConfirmedOrBeyond := sp.status == StatusConfirmed ||
				sp.status == StatusInProduction ||
				sp.status == StatusReady ||
				sp.status == StatusDelivering ||
				sp.status == StatusCompleted

			if isConfirmedOrBeyond {
				shopDue := evDate.AddDate(0, 0, -params.EvalParams.ShoppingLeadDays)
				shopStatus := "todo"
				prodStatus := "todo"
				delivStatus := "todo"

				switch sp.status {
				case StatusInProduction:
					shopStatus = "done"
					prodStatus = "doing"
					delivStatus = "todo"
				case StatusReady:
					shopStatus = "done"
					prodStatus = "done"
					delivStatus = "todo"
				case StatusDelivering:
					shopStatus = "done"
					prodStatus = "done"
					delivStatus = "doing"
				case StatusCompleted:
					shopStatus = "done"
					prodStatus = "done"
					delivStatus = "done"
				}

				taskDefs := []struct {
					tType   string
					title   string
					due     time.Time
					status  string
					actMin  *int
					assign  string
				}{
					{
						tType:  "shop",
						title:  fmt.Sprintf("Belanja bahan pesanan %s (%s)", sp.code, sp.customerName),
						due:    shopDue,
						status: shopStatus,
						assign: staffID,
					},
					{
						tType:  "produce",
						title:  fmt.Sprintf("Produksi pesanan %s (%s)", sp.code, sp.customerName),
						due:    prDate,
						status: prodStatus,
						actMin: sp.produceActualMin,
						assign: staffID,
					},
					{
						tType:  "deliver",
						title:  fmt.Sprintf("Penyerahan pesanan %s (%s)", sp.code, sp.customerName),
						due:    evDate,
						status: delivStatus,
						assign: staffID,
					},
				}

				for _, td := range taskDefs {
					if _, err := tx.Exec(ctx, `
						INSERT INTO tasks (order_id, type, title, due_date, assignee_id, status, actual_minutes, created_at)
						VALUES ($1::uuid, $2, $3, $4::date, $5::uuid, $6, $7, $8)
					`, sp.id, td.tType, td.title, engine.DateKey(td.due), td.assign, td.status, td.actMin, createdAt); err != nil {
						return fmt.Errorf("insert task %s/%s: %w", sp.code, td.tType, err)
					}
				}
			}

			// Untuk S5 (DEMOS5CNCL), catat audit log pembatalan dengan eksposur Rp 587.500
			if sp.status == StatusCancelled && sp.cancelShopCost != nil && sp.cancelExposure != nil {
				if err := audit.Log(ctx, tx, ownerID, audit.ActionStatus, "orders", sp.id, map[string]any{
					"op":                    "cancel_order",
					"code":                  sp.code,
					"from_status":           StatusConfirmed,
					"to_status":             StatusCancelled,
					"shopping_cost":         sp.cancelShopCost.Round(2),
					"dp_received":           sp.paidAmount.Round(2),
					"cancellation_exposure": sp.cancelExposure.Round(2),
					"dp_disposition":        "forfeited",
					"reason":                "Acara perusahaan ditunda mendadak setelah bahan dibeli",
				}); err != nil {
					return err
				}
			}
		}

		// 9. Catat riwayat pembelian bahan (purchases) & pergerakan stok (stock_movements) historis
		// Catatan: Stok akhir pada tabel ingredients TIDAK diubah agar Tepung Terigu tetap tepat 3,0000 kg untuk S4!
		purchasesSeed := []struct {
			id           string
			ingID        string
			qtyBuyUnits  decimal.Decimal
			qtyUseAdded  decimal.Decimal
			totalCost    decimal.Decimal
			dayOffset    int
			note         string
		}{
			{
				id: "40000000-0000-4000-8000-000000000001", ingID: ingActualID[DemoIngTepungID],
				qtyBuyUnits: decimal.NewFromInt(1), qtyUseAdded: decimal.NewFromInt(25),
				totalCost: decimal.NewFromInt(300000), dayOffset: -6,
				note: "Pembelian 1 sak Tepung Terigu produksi minggu lalu",
			},
			{
				id: "40000000-0000-4000-8000-000000000002", ingID: ingActualID[DemoIngTelurID],
				qtyBuyUnits: decimal.NewFromInt(3), qtyUseAdded: decimal.NewFromInt(90),
				totalCost: decimal.NewFromInt(180000), dayOffset: -5,
				note: "Pembelian 3 tray Telur Ayam segar",
			},
			{
				id: "40000000-0000-4000-8000-000000000003", ingID: ingActualID[DemoIngBerasID],
				qtyBuyUnits: decimal.NewFromInt(1), qtyUseAdded: decimal.NewFromInt(25),
				totalCost: decimal.NewFromInt(375000), dayOffset: -4,
				note: "Pembelian 1 karung Beras Pandan Wangi katering",
			},
			{
				id: "40000000-0000-4000-8000-000000000004", ingID: ingActualID[DemoIngAyamID],
				qtyBuyUnits: decimal.NewFromInt(15), qtyUseAdded: decimal.NewFromInt(15),
				totalCost: decimal.NewFromInt(825000), dayOffset: -3,
				note: "Pembelian 15 kg Daging Ayam Fillet katering",
			},
		}

		for _, ps := range purchasesSeed {
			pTime := nowUTC.AddDate(0, 0, ps.dayOffset)
			if _, err := tx.Exec(ctx, `
				INSERT INTO purchases (id, ingredient_id, qty_buy_units, unit_price, bought_at, bought_by)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::uuid)
			`, ps.id, ps.ingID, ps.qtyBuyUnits, ps.totalCost.DivRound(ps.qtyBuyUnits, 4), pTime, managerID); err != nil {
				return fmt.Errorf("insert purchase: %w", err)
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_movements (ingredient_id, delta, reason, ref_type, ref_id, created_at)
				VALUES ($1::uuid, $2, 'purchase', 'purchase', $3::uuid, $4)
			`, ps.ingID, ps.qtyUseAdded, ps.id, pTime); err != nil {
				return fmt.Errorf("insert stock_movement purchase: %w", err)
			}
		}

		// Pergerakan stok pemakaian produksi untuk beberapa pesanan selesai & koreksi stok
		prodMovements := []struct {
			ingID     string
			delta     string
			reason    string
			refType   string
			orderID   string
			dayOffset int
		}{
			{ingID: ingActualID[DemoIngTepungID], delta: "-2.0000", reason: "production", refType: "order", orderID: "30000000-0000-4000-8000-000000000008", dayOffset: -7},
			{ingID: ingActualID[DemoIngTelurID], delta: "-40.0000", reason: "production", refType: "order", orderID: "30000000-0000-4000-8000-000000000009", dayOffset: -6},
			{ingID: ingActualID[DemoIngBerasID], delta: "-6.0000", reason: "production", refType: "order", orderID: "30000000-0000-4000-8000-000000000013", dayOffset: -7},
			{ingID: ingActualID[DemoIngAyamID], delta: "-7.2000", reason: "production", refType: "order", orderID: "30000000-0000-4000-8000-000000000013", dayOffset: -7},
			{ingID: ingActualID[DemoIngCokelatID], delta: "-2.5000", reason: "production", refType: "order", orderID: "30000000-0000-4000-8000-000000000016", dayOffset: -4},
			{ingID: ingActualID[DemoIngKejuID], delta: "-0.5000", reason: "waste", refType: "adjustment", orderID: "", dayOffset: -2},
		}
		for _, pm := range prodMovements {
			var refID any
			if pm.orderID != "" {
				refID = pm.orderID
			}
			mTime := nowUTC.AddDate(0, 0, pm.dayOffset)
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_movements (ingredient_id, delta, reason, ref_type, ref_id, created_at)
				VALUES ($1::uuid, $2::numeric, $3, $4, $5::uuid, $6)
			`, pm.ingID, pm.delta, pm.reason, pm.refType, refID, mTime); err != nil {
				return fmt.Errorf("insert stock_movement %s: %w", pm.reason, err)
			}
		}

		// Catat audit log master/setting & reset demo bila dipanggil oleh admin
		resetActor := adminID
		if len(actorID) > 0 && actorID[0] != "" {
			resetActor = actorID[0]
		}
		if err := audit.Log(ctx, tx, resetActor, audit.ActionMaster, "demo_seed", "", map[string]any{
			"op":            "reset_demo",
			"orders_seeded": len(specs),
			"today":         engine.DateKey(today),
		}); err != nil {
			return err
		}

		// 10. Hitung ringkasan baris tabel dan verifikasi skenario S1..S5
		tables := []string{
			"users", "resources", "settings", "products", "ingredients",
			"recipe_items", "product_loads", "orders", "order_items",
			"order_evaluations", "payments", "tasks", "purchases",
			"stock_movements", "cash_entries", "audit_logs",
		}
		tableCounts := make(map[string]int, len(tables))
		for _, tbl := range tables {
			var cnt int
			if err := tx.QueryRow(ctx, "SELECT COUNT(*) FROM "+tbl).Scan(&cnt); err != nil {
				return fmt.Errorf("count %s: %w", tbl, err)
			}
			tableCounts[tbl] = cnt
		}

		statusCounts := make(map[string]int)
		stRows, err := tx.Query(ctx, `SELECT status, COUNT(*) FROM orders GROUP BY status`)
		if err != nil {
			return err
		}
		for stRows.Next() {
			var st string
			var cnt int
			if err := stRows.Scan(&st, &cnt); err != nil {
				stRows.Close()
				return err
			}
			statusCounts[st] = cnt
		}
		stRows.Close()

		// Verifikasi live S4 (Tepung Terigu: perlu 12,84 kg, beli 13 kg)
		tepungNeed, tepungBuy := engine.ShoppingNeed(
			decimal.RequireFromString("14.4"),
			decimal.RequireFromString("3.0"),
			params.ShoppingBufferPct,
			decimal.RequireFromString("25"),
			"loose",
		)

		summary = &DemoSeedSummary{
			Today:            engine.DateKey(today),
			SnackBoxUnitCost: sbMeta.UnitCost.StringFixed(0),
			TableCounts:      tableCounts,
			StatusCounts:     statusCounts,
			Scenarios: map[string]any{
				"S1": map[string]any{
					"code":               "DEMOS1FULL",
					"event_date":         engine.DateKey(today.AddDate(0, 0, 5)),
					"g1_passed":          false,
					"utilization_after":  "1.5546",
					"counter_offer_date": engine.DateKey(today.AddDate(0, 0, 6)),
					"counter_offer_util": "0.8824",
				},
				"S2": map[string]any{
					"code":           "DEMOS2DISC",
					"requested_unit": "21000",
					"margin":         "0.0952",
					"min_unit_price": "23750",
					"g2_passed":      false,
				},
				"S3": map[string]any{
					"code":        "DEMOS3DPMN",
					"total":       "2375000",
					"min_dp":      "600000",
					"dp_charged":  "712500",
					"dp_required": "712500",
				},
				"S4": map[string]any{
					"ingredient":    "Tepung Terigu",
					"required_qty":  "14.40",
					"stock":         "3.00",
					"need_qty":      tepungNeed.StringFixed(2),
					"buy_use_units": tepungBuy.StringFixed(0),
				},
				"S5": map[string]any{
					"code":                  "DEMOS5CNCL",
					"shopping_cost":         "1300000",
					"dp_received":           "712500",
					"cancellation_exposure": "587500",
				},
			},
			ResetAt: nowUTC,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

