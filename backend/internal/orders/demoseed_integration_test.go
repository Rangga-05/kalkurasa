//go:build integration

package orders_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/config"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/orders"
	"kalkurasa/backend/internal/server"
)

// TestDemoSeed_PostgresResetAndScenarios menguji POST /api/v1/admin/reset-demo (RBAC) dan
// memverifikasi skenario S1..S5 secara langsung dari data hasil seed (bukan angka hardcode).
func TestDemoSeed_PostgresResetAndScenarios(t *testing.T) {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load("backend/.env")

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		t.Fatal("DATABASE_URL wajib diatur untuk menjalankan test integrasi Postgres")
	}
	if len(cfg.JWTSecret) < 32 {
		cfg.JWTSecret = "0123456789abcdef0123456789abcdef-ittest"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer pool.Close()

	gin.SetMode(gin.TestMode)
	router := server.New(cfg, pool)

	tokenFor := func(email, role string) string {
		var id string
		if err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, email).Scan(&id); err != nil {
			// Pengguna demo mungkin belum ada (DB kosong): buat lewat seed terlebih dahulu.
			if _, serr := orders.SeedDemoData(ctx, pool, "DemoKalkurasa2026!"); serr != nil {
				t.Fatalf("seed awal: %v", serr)
			}
			if err := pool.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1`, email).Scan(&id); err != nil {
				t.Fatalf("user %s: %v", email, err)
			}
		}
		tok, _, err := auth.IssueToken([]byte(cfg.JWTSecret), id, role, time.Now(), auth.TokenTTL)
		if err != nil {
			t.Fatalf("IssueToken: %v", err)
		}
		return "Bearer " + tok
	}
	adminTok := tokenFor("admin@demo.test", auth.RoleAdmin)
	ownerTok := tokenFor("owner@demo.test", auth.RoleOwner)
	custTok := tokenFor("customer@demo.test", auth.RoleCustomer)

	// RBAC: tanpa token 401; owner & customer 403.
	if w := performJSON(router, http.MethodPost, "/api/v1/admin/reset-demo", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa token: status %d, ingin 401: %s", w.Code, w.Body.String())
	}
	for name, tok := range map[string]string{"owner": ownerTok, "customer": custTok} {
		if w := performJSON(router, http.MethodPost, "/api/v1/admin/reset-demo", nil, map[string]string{"Authorization": tok}); w.Code != http.StatusForbidden {
			t.Fatalf("%s: status %d, ingin 403: %s", name, w.Code, w.Body.String())
		}
	}

	// Sisipkan pesanan uji manual yang harus hilang setelah reset.
	if _, err := pool.Exec(ctx, `
		INSERT INTO orders (code, source, customer_name, customer_contact, delivery_type, event_date, production_date, status, total, dp_required, paid)
		VALUES ('ZZMANUAL01', 'manual', 'ittest-manual-residu', '0800', 'pickup', current_date + 90, current_date + 90, 'pending_decision', 1000, 300, 0)
	`); err != nil {
		t.Fatalf("insert pesanan manual: %v", err)
	}

	// Admin reset dua kali (idempoten).
	var summary orders.DemoSeedSummary
	for i := 1; i <= 2; i++ {
		w := performJSON(router, http.MethodPost, "/api/v1/admin/reset-demo", nil, map[string]string{"Authorization": adminTok})
		if w.Code != http.StatusOK {
			t.Fatalf("reset ke-%d: status %d: %s", i, w.Code, w.Body.String())
		}
		var env struct {
			Data orders.DemoSeedSummary `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("parse reset: %v", err)
		}
		summary = env.Data
		if summary.TableCounts["orders"] != 40 {
			t.Fatalf("reset ke-%d: orders = %d, ingin 40", i, summary.TableCounts["orders"])
		}
	}
	if summary.SnackBoxUnitCost != "19000" {
		t.Fatalf("UnitCost Snack Box = %s, ingin 19000", summary.SnackBoxUnitCost)
	}

	var residu int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE code = 'ZZMANUAL01'`).Scan(&residu); err != nil || residu != 0 {
		t.Fatalf("pesanan manual residu masih ada (n=%d, err=%v)", residu, err)
	}

	today := db.Today(ctx, pool)

	dec := func(s string) decimal.Decimal { return decimal.RequireFromString(s) }

	// S1: G1 gagal 155,5%, tawaran balik ke hari berikutnya dengan utilisasi 88,2%.
	var g1Val decimal.Decimal
	var g1Passed bool
	if err := pool.QueryRow(ctx, `
		SELECT e.value, e.passed FROM order_evaluations e JOIN orders o ON o.id = e.order_id
		WHERE o.code = 'DEMOS1FULL' AND e.gate = 'G1'`).Scan(&g1Val, &g1Passed); err != nil {
		t.Fatalf("S1 G1: %v", err)
	}
	if g1Passed || g1Val.Round(3).String() != "1.555" {
		t.Errorf("S1: G1 passed=%v value=%s, ingin gagal 1.555", g1Passed, g1Val)
	}
	var coRaw []byte
	if err := pool.QueryRow(ctx, `SELECT counter_offer FROM orders WHERE code = 'DEMOS1FULL'`).Scan(&coRaw); err != nil {
		t.Fatalf("S1 counter: %v", err)
	}
	var co orders.CounterOfferDTO
	if err := json.Unmarshal(coRaw, &co); err != nil {
		t.Fatalf("S1 parse counter: %v", err)
	}
	if co.Date != engine.DateKey(today.AddDate(0, 0, 6)) || co.UtilizationAfter.Round(3).String() != "0.882" {
		t.Errorf("S1: counter date=%s util=%s, ingin %s / 0.882", co.Date, co.UtilizationAfter, engine.DateKey(today.AddDate(0, 0, 6)))
	}

	// S2: margin 9,5% (G2 gagal) dan harga minimal Rp 23.750.
	var g2Val decimal.Decimal
	var g2Passed bool
	if err := pool.QueryRow(ctx, `
		SELECT e.value, e.passed FROM order_evaluations e JOIN orders o ON o.id = e.order_id
		WHERE o.code = 'DEMOS2DISC' AND e.gate = 'G2'`).Scan(&g2Val, &g2Passed); err != nil {
		t.Fatalf("S2 G2: %v", err)
	}
	if g2Passed || g2Val.Round(3).String() != "0.095" {
		t.Errorf("S2: G2 passed=%v value=%s, ingin gagal 0.095", g2Passed, g2Val)
	}
	if err := pool.QueryRow(ctx, `SELECT counter_offer FROM orders WHERE code = 'DEMOS2DISC'`).Scan(&coRaw); err != nil {
		t.Fatalf("S2 counter: %v", err)
	}
	var co2 orders.CounterOfferDTO
	if err := json.Unmarshal(coRaw, &co2); err != nil {
		t.Fatalf("S2 parse counter: %v", err)
	}
	var minPrice decimal.Decimal
	for _, v := range co2.MinPricePerItem {
		minPrice = v
	}
	if !minPrice.Equal(dec("23750")) {
		t.Errorf("S2: harga minimal = %s, ingin 23750", minPrice)
	}

	// S3: DP minimal 600.000 dan DP ditagih 712.500.
	var dpReq, total decimal.Decimal
	var snapRaw []byte
	if err := pool.QueryRow(ctx, `SELECT dp_required, total, snapshot FROM orders WHERE code = 'DEMOS3DPMN'`).Scan(&dpReq, &total, &snapRaw); err != nil {
		t.Fatalf("S3: %v", err)
	}
	var snap struct {
		MinDP decimal.Decimal `json:"min_dp"`
	}
	_ = json.Unmarshal(snapRaw, &snap)
	if !dpReq.Equal(dec("712500")) || !total.Equal(dec("2375000")) || !snap.MinDP.Equal(dec("600000")) {
		t.Errorf("S3: dp_required=%s total=%s min_dp=%s", dpReq, total, snap.MinDP)
	}

	// S4: lewat endpoint nyata GET /shopping-list (rentang default hari ini s.d. +14).
	w := performJSON(router, http.MethodGet, "/api/v1/shopping-list?from="+engine.DateKey(today)+"&to="+engine.DateKey(today.AddDate(0, 0, 14)), nil, map[string]string{"Authorization": ownerTok})
	if w.Code != http.StatusOK {
		t.Fatalf("shopping-list: %d %s", w.Code, w.Body.String())
	}
	var sl struct {
		Data orders.ShoppingListResult `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &sl); err != nil {
		t.Fatalf("parse shopping-list: %v", err)
	}
	foundTepung := false
	for _, it := range sl.Data.Items {
		if it.IngredientName == "Tepung Terigu" {
			foundTepung = true
			if !it.RequiredQty.Equal(dec("14.4")) || !it.NeedQty.Round(2).Equal(dec("12.84")) || !it.BuyUseUnits.Equal(dec("13")) {
				t.Errorf("S4: required=%s need=%s buy=%s, ingin 14.4 / 12.84 / 13", it.RequiredQty, it.NeedQty, it.BuyUseUnits)
			}
		}
	}
	if !foundTepung {
		t.Errorf("S4: Tepung Terigu tidak ada di shopping-list")
	}

	// S5: eksposur pembatalan Rp 587.500 tercatat di audit log.
	var exposure string
	if err := pool.QueryRow(ctx, `
		SELECT a.detail->>'cancellation_exposure' FROM audit_logs a JOIN orders o ON o.id = a.entity_id
		WHERE o.code = 'DEMOS5CNCL' AND a.detail->>'op' = 'cancel_order'`).Scan(&exposure); err != nil {
		t.Fatalf("S5: %v", err)
	}
	if !dec(exposure).Equal(dec("587500")) {
		t.Errorf("S5: eksposur = %s, ingin 587500", exposure)
	}
}

