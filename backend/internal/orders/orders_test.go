package orders_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/config"
	"kalkurasa/backend/internal/orders"
	"kalkurasa/backend/internal/server"
)

type testUserStore struct {
	users map[string]*auth.User
}

func (s *testUserStore) FindByEmail(_ context.Context, _ string) (*auth.UserWithHash, error) {
	return nil, auth.ErrNotFound
}

func (s *testUserStore) FindByID(_ context.Context, id string) (*auth.User, error) {
	u, ok := s.users[id]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

type testEnv struct {
	db           *memDB
	router       *gin.Engine
	snackBoxID   string
	tartID       string
	managerToken string
	ownerToken   string
}

// setupIsolatedTestEnv membuat fixture data uji langsung di dalam test
// (bukan di database produksi) dengan memDB yang mengimplementasikan db.Conn
// dan pgx.Tx beserta penguncian baris day_locks per tanggal.
func setupIsolatedTestEnv(t *testing.T) *testEnv {
	t.Helper()

	mdb := newMemDB()

	// 1. Settings sesuai Bagian 5 brief + demo_today = 2026-11-10
	mdb.settings = map[string]string{
		"capacity_limit":        "0.90",
		"status_green_below":    "0.70",
		"status_yellow_below":   "0.90",
		"margin_min":            "0.20",
		"labor_rate_per_minute": "400",
		"waste_pct":             "0.05",
		"shopping_buffer_pct":   "0.10",
		"cash_buffer":           "400000",
		"fixed_cost_daily":      "400000",
		"opening_cash":          "5000000",
		"lead_time_days":        "2",
		"dp_pct":                "0.30",
		"dp_deadline_hours":     "24",
		"big_order_total":       "2000000",
		"big_order_units":       "50",
		"shopping_lead_days":    "2",
		"demo_today":            `"2026-11-10"`,
	}

	// 2. Sumber daya sesuai spesifikasi: oven (2 x 8 x 0.85 = 13,6) dan labor (4 x 420 x 0.85 = 1.428)
	mdb.resources = []memResource{
		{
			ID:               "aaaa0000-0000-4000-8000-000000000001",
			Code:             "oven",
			Name:             "Oven Deck",
			Unit:             "oven_hour",
			Count:            2,
			AmountPerUnitDay: decimal.NewFromInt(8),
			Efficiency:       decimal.RequireFromString("0.85"),
		},
		{
			ID:               "aaaa0000-0000-4000-8000-000000000002",
			Code:             "labor",
			Name:             "Tenaga Produksi",
			Unit:             "minute",
			Count:            4,
			AmountPerUnitDay: decimal.NewFromInt(420),
			Efficiency:       decimal.RequireFromString("0.85"),
		},
	}

	// 3. Produk, resep, dan beban produksi
	snackBoxID := "bbbb0000-0000-4000-8000-000000000001"
	tartID := "bbbb0000-0000-4000-8000-000000000002"

	mdb.products[snackBoxID] = memProduct{
		ID:            snackBoxID,
		Name:          "Snack Box Rapat",
		Category:      "snack_box",
		Price:         decimal.NewFromInt(25000),
		PackagingCost: decimal.NewFromInt(2000),
		OverheadCost:  decimal.NewFromInt(1200),
		IsActive:      true,
	}
	mdb.products[tartID] = memProduct{
		ID:            tartID,
		Name:          "Tart Ulang Tahun",
		Category:      "kue_tart",
		Price:         decimal.NewFromInt(280000),
		PackagingCost: decimal.NewFromInt(15000),
		OverheadCost:  decimal.NewFromInt(10000),
		IsActive:      true,
	}

	mdb.loads[snackBoxID] = map[string]decimal.Decimal{
		"oven":  decimal.RequireFromString("0.05"),
		"labor": decimal.NewFromInt(12),
	}
	mdb.loads[tartID] = map[string]decimal.Decimal{
		"oven":  decimal.RequireFromString("0.50"),
		"labor": decimal.NewFromInt(90),
	}

	mdb.recipeCosts[snackBoxID] = decimal.NewFromInt(10000)
	mdb.recipeCosts[tartID] = decimal.NewFromInt(120000)

	// 4. Pengguna uji (owner & manager)
	now := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	uStore := &testUserStore{
		users: map[string]*auth.User{
			"11111111-1111-1111-1111-111111111111": {
				ID: "11111111-1111-1111-1111-111111111111", Email: "owner@test.local", Name: "Owner Uji", Role: auth.RoleOwner, IsActive: true, CreatedAt: now,
			},
			"22222222-2222-2222-2222-222222222222": {
				ID: "22222222-2222-2222-2222-222222222222", Email: "manager@test.local", Name: "Manager Uji", Role: auth.RoleManager, IsActive: true, CreatedAt: now,
			},
		},
	}

	gin.SetMode(gin.TestMode)
	secret := []byte("0123456789abcdef0123456789abcdef-kalkurasa-t3a")
	cfg := &config.Config{
		Port:           "8080",
		JWTSecret:      string(secret),
		AppEnv:         "development",
		AllowedOrigins: "http://localhost:3000",
		CookieSecure:   false,
		TZ:             "Asia/Jakarta",
	}
	r := server.NewWithConn(cfg, nil, mdb, uStore)

	ownerTok, _, _ := auth.IssueToken(secret, "11111111-1111-1111-1111-111111111111", auth.RoleOwner, time.Now(), auth.TokenTTL)
	mgrTok, _, _ := auth.IssueToken(secret, "22222222-2222-2222-2222-222222222222", auth.RoleManager, time.Now(), auth.TokenTTL)

	return &testEnv{
		db:           mdb,
		router:       r,
		snackBoxID:   snackBoxID,
		tartID:       tartID,
		managerToken: mgrTok,
		ownerToken:   ownerTok,
	}
}

func performJSON(r http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestTC08_ConcurrentOrdersLastSlot menguji TC-08:
// Dua goroutine memesan slot terakhir pada tanggal produksi yang sama secara
// serentak. Penguncian baris day_locks (SELECT ... FOR UPDATE) memastikan
// tepat satu pesanan berstatus "accepted" dan pesanan lainnya gagal G1 sehingga
// berstatus "pending_decision" dengan rekomendasi "accept_with_conditions"
// dan tawaran balik ke hari berikutnya.
func TestTC08_ConcurrentOrdersLastSlot(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Tanggal target: 2026-11-14 (H+4 dari demo_today 2026-11-10, lolos lead time 2 hari).
	// Kapasitas tenaga = 1428 menit; batas terima 90% = 1285,2 menit.
	// Isi beban awal 75 Snack Box (75 * 12 = 900 menit, util 63,03%) lewat pesanan accepted.
	evDate, _ := time.Parse("2006-01-02", "2026-11-14")
	env.db.orders = append(env.db.orders, memOrder{
		ID:             "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Code:           "EXISTING01",
		Source:         "manual",
		CustomerName:   "Pelanggan Awal",
		DeliveryType:   "pickup",
		EventDate:      evDate,
		ProductionDate: evDate,
		Status:         orders.StatusAccepted,
		Total:          decimal.NewFromInt(1875000),
		DPRequired:     decimal.NewFromInt(562500),
		Paid:           decimal.NewFromInt(562500),
	})
	env.db.orderItems = append(env.db.orderItems, memOrderItem{
		ID:        "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		OrderID:   "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		ProductID: env.snackBoxID,
		Qty:       75,
		UnitPrice: decimal.NewFromInt(25000),
		UnitCost:  decimal.NewFromInt(18500),
	})

	// Sisa kapasitas sebelum batas 90% (1285,2 menit) adalah 385,2 menit.
	// Dua pesanan masuk bersamaan masing-masing 25 Snack Box (25 * 12 = 300 menit; total Rp 625.000 < 2.000.000; unit 25 < 50):
	// - Pesanan pertama: 900 + 300 = 1200 menit (84,03% <= 90%) -> LOLOS semua gerbang -> status "accepted".
	// - Pesanan kedua: 1200 + 300 = 1500 menit (105,04% > 90%) -> GAGAL G1 -> status "pending_decision",
	//   recommendation "accept_with_conditions", counter_offer.date = "2026-11-15".
	type orderResp struct {
		StatusCode int
		Body       string
		Data       orders.PublicOrderSummary
	}

	results := make([]orderResp, 2)
	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-startBarrier
			w := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
				"customer_name":    fmt.Sprintf("Pelanggan Serentak %d", idx+1),
				"customer_contact": fmt.Sprintf("0812345678%d", idx+1),
				"delivery_type":    "pickup",
				"event_date":       "2026-11-14",
				"items": []map[string]any{
					{"product_id": env.snackBoxID, "qty": 25},
				},
			}, map[string]string{
				"X-Forwarded-For": fmt.Sprintf("203.0.113.%d", idx+10),
			})
			var envResp struct {
				Data orders.PublicOrderSummary `json:"data"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &envResp)
			results[idx] = orderResp{
				StatusCode: w.Code,
				Body:       w.Body.String(),
				Data:       envResp.Data,
			}
		}(i)
	}

	close(startBarrier)
	wg.Wait()

	acceptedCount := 0
	pendingCount := 0
	var counterOrder orders.PublicOrderSummary

	for i, r := range results {
		if r.StatusCode != http.StatusCreated {
			t.Fatalf("goroutine %d: expected 201 Created, got %d (%s)", i, r.StatusCode, r.Body)
		}
		switch r.Data.Status {
		case orders.StatusAccepted:
			acceptedCount++
			if r.Data.Recommendation != "accept" {
				t.Errorf("accepted order expected recommendation 'accept', got %q", r.Data.Recommendation)
			}
			if r.Data.DPDeadline == nil {
				t.Errorf("accepted order expected non-nil dp_deadline")
			}
		case orders.StatusPendingDecision:
			pendingCount++
			counterOrder = r.Data
		default:
			t.Errorf("unexpected status %q", r.Data.Status)
		}
	}

	if acceptedCount != 1 || pendingCount != 1 {
		t.Fatalf("TC-08 gagal: diharapkan tepat 1 accepted dan 1 pending_decision, dapat accepted=%d pending=%d",
			acceptedCount, pendingCount)
	}
	if counterOrder.Recommendation != "accept_with_conditions" {
		t.Errorf("TC-08: pesanan kedua diharapkan recommendation='accept_with_conditions', dapat %q", counterOrder.Recommendation)
	}
	if counterOrder.CounterOffer == nil || counterOrder.CounterOffer.Date != "2026-11-15" {
		t.Errorf("TC-08: pesanan kedua diharapkan tawaran balik ke 2026-11-15, dapat %+v", counterOrder.CounterOffer)
	}
}

// TestTC10_LeadTimeAndAvailability menguji TC-10:
// Tanggal acara kurang dari lead_time_days (2 hari dari demo_today 2026-11-10):
//  1. POST /api/v1/public/availability menandai tanggal < 2026-11-12 sebagai available=false.
//  2. Bila pesanan tetap dikirim untuk 2026-11-11 (selisih 1 hari < 2 hari),
//     gerbang G4 gagal (passed=false, value=1, threshold=2), rekomendasi "reject",
//     status "pending_decision", dan 4 baris order_evaluations tersimpan.
func TestTC10_LeadTimeAndAvailability(t *testing.T) {
	env := setupIsolatedTestEnv(t)
	ctx := context.Background()

	// 1. Cek ketersediaan 2026-11-10 s.d. 2026-11-13
	wAvail := performJSON(env.router, http.MethodPost, "/api/v1/public/availability", map[string]any{
		"from": "2026-11-10",
		"to":   "2026-11-13",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 10},
		},
	}, nil)
	if wAvail.Code != http.StatusOK {
		t.Fatalf("POST /public/availability: expected 200, got %d (%s)", wAvail.Code, wAvail.Body.String())
	}

	var availResp struct {
		Data []orders.AvailabilityDay `json:"data"`
	}
	if err := json.Unmarshal(wAvail.Body.Bytes(), &availResp); err != nil {
		t.Fatalf("unmarshal availability: %v", err)
	}
	if len(availResp.Data) != 4 {
		t.Fatalf("expected 4 availability days, got %d", len(availResp.Data))
	}

	wantAvail := map[string]bool{
		"2026-11-10": false, // H+0 (< 2 hari)
		"2026-11-11": false, // H+1 (< 2 hari)
		"2026-11-12": true,  // H+2 (>= 2 hari)
		"2026-11-13": true,  // H+3 (>= 2 hari)
	}
	for _, d := range availResp.Data {
		if want, ok := wantAvail[d.Date]; ok && d.Available != want {
			t.Errorf("TC-10 availability tanggal %s: expected available=%v, got %v (status=%s, util=%s)",
				d.Date, want, d.Available, d.Status, d.UtilizationAfter)
		}
	}

	// 2. Kirim pesanan untuk 2026-11-11 (H+1, kurang dari lead time 2 hari)
	wOrder := performJSON(env.router, http.MethodPost, "/api/v1/orders", map[string]any{
		"customer_name":    "Ibu Ratna",
		"customer_contact": "081299998888",
		"delivery_type":    "delivery",
		"address":          "Jl. Merdeka No. 10, Bandung",
		"event_date":       "2026-11-11",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 10},
		},
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wOrder.Code != http.StatusCreated {
		t.Fatalf("POST /orders TC-10: expected 201, got %d (%s)", wOrder.Code, wOrder.Body.String())
	}

	var ordResp struct {
		Data orders.OrderDetail `json:"data"`
	}
	if err := json.Unmarshal(wOrder.Body.Bytes(), &ordResp); err != nil {
		t.Fatalf("unmarshal order detail: %v", err)
	}

	if ordResp.Data.Recommendation != "reject" {
		t.Errorf("TC-10: expected recommendation 'reject', got %q", ordResp.Data.Recommendation)
	}
	if ordResp.Data.Status != orders.StatusPendingDecision {
		t.Errorf("TC-10: expected status 'pending_decision', got %q", ordResp.Data.Status)
	}

	// Pastikan G4 bernilai passed=false, value=1, threshold=2
	var foundG4 bool
	for _, g := range ordResp.Data.Evaluation.Gates {
		if g.Gate == "G4" {
			foundG4 = true
			if g.Passed || !g.Value.Equal(decimal.NewFromInt(1)) || !g.Threshold.Equal(decimal.NewFromInt(2)) {
				t.Errorf("TC-10 G4 unexpected: %+v", g)
			}
		}
	}
	if !foundG4 {
		t.Errorf("TC-10: gate G4 tidak ditemukan pada evaluasi")
	}

	// Pastikan 4 baris order_evaluations tersimpan
	var evalCount int
	if err := env.db.QueryRow(ctx, `SELECT COUNT(*) FROM order_evaluations WHERE order_id = $1::uuid`, ordResp.Data.ID).Scan(&evalCount); err != nil {
		t.Fatalf("count order_evaluations: %v", err)
	}
	if evalCount != 4 {
		t.Errorf("expected 4 rows in order_evaluations, got %d", evalCount)
	}
}

// TestTC09_CustomerOrderTrackingAndGuessingCode menguji TC-09 bagian pelanggan:
//  1. GET /api/v1/public/orders/:code untuk kode yang benar mengembalikan 200 OK
//     hanya dengan field yang aman (tanpa unit_cost, margin, snapshot, atau kas).
//  2. Pelanggan yang menebak kode pesanan lain (format valid 10 karakter maupun
//     format salah) mendapat 404 Not Found dengan pesan jelas berbahasa Indonesia.
func TestTC09_CustomerOrderTrackingAndGuessingCode(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Buat satu pesanan publik yang sah
	wCreate := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Dewi Lestari",
		"customer_contact": "081377776666",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-16",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 10},
		},
	}, nil)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create order: expected 201, got %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var created struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	if err := json.Unmarshal(wCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created order: %v", err)
	}
	if len(created.Data.Code) != 10 {
		t.Fatalf("expected 10-char order code, got %q", created.Data.Code)
	}

	// 1. Lacak pesanan dengan kode yang benar -> 200 OK dan hanya field aman
	wTrack := performJSON(env.router, http.MethodGet, "/api/v1/public/orders/"+created.Data.Code, nil, nil)
	if wTrack.Code != http.StatusOK {
		t.Fatalf("GET /public/orders/%s: expected 200, got %d (%s)", created.Data.Code, wTrack.Code, wTrack.Body.String())
	}
	rawTrack := wTrack.Body.String()
	for _, forbiddenField := range []string{"unit_cost", "margin", "snapshot", "cash_lowest", "decided_by"} {
		if strings.Contains(rawTrack, forbiddenField) {
			t.Errorf("TC-09: respons lacak pesanan publik membocorkan field rahasia %q: %s", forbiddenField, rawTrack)
		}
	}

	// 2. Pelanggan menebak kode pesanan lain -> 404 Not Found
	guessedCodes := []string{
		"ZZZZZZZZZZ", // format base32 10 karakter valid tetapi tidak ada
		"ABCDEFGHJK", // tebakan acak 10 karakter lainnya
		"ORD-12345",  // format salah
	}
	for _, guess := range guessedCodes {
		if guess == created.Data.Code {
			continue
		}
		wGuess := performJSON(env.router, http.MethodGet, "/api/v1/public/orders/"+guess, nil, nil)
		if wGuess.Code != http.StatusNotFound {
			t.Fatalf("TC-09 tebak kode %q: expected 404, got %d (%s)", guess, wGuess.Code, wGuess.Body.String())
		}
		if strings.Contains(wGuess.Body.String(), "Dewi Lestari") {
			t.Errorf("TC-09: data pelanggan bocor pada respons 404")
		}
	}
}
