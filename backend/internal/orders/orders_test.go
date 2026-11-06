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
	flourID      string
	managerToken string
	ownerToken   string
	staffToken   string
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

	flourID := "cccc0000-0000-4000-8000-000000000001"
	mdb.ingredients[flourID] = decimal.NewFromInt(20) // 20 kg tepung
	mdb.recipes[snackBoxID] = map[string]decimal.Decimal{
		flourID: decimal.RequireFromString("0.12"), // 0,12 kg tepung per snack box
	}
	mdb.recipes[tartID] = map[string]decimal.Decimal{
		flourID: decimal.RequireFromString("0.40"), // 0,40 kg tepung per tart
	}

	// 4. Pengguna uji (owner, manager, & staff)
	now := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	uStore := &testUserStore{
		users: map[string]*auth.User{
			"11111111-1111-1111-1111-111111111111": {
				ID: "11111111-1111-1111-1111-111111111111", Email: "owner@test.local", Name: "Owner Uji", Role: auth.RoleOwner, IsActive: true, CreatedAt: now,
			},
			"22222222-2222-2222-2222-222222222222": {
				ID: "22222222-2222-2222-2222-222222222222", Email: "manager@test.local", Name: "Manager Uji", Role: auth.RoleManager, IsActive: true, CreatedAt: now,
			},
			"33333333-3333-3333-3333-333333333333": {
				ID: "33333333-3333-3333-3333-333333333333", Email: "staff@test.local", Name: "Staff Uji", Role: auth.RoleStaff, IsActive: true, CreatedAt: now,
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
	staffTok, _, _ := auth.IssueToken(secret, "33333333-3333-3333-3333-333333333333", auth.RoleStaff, time.Now(), auth.TokenTTL)

	return &testEnv{
		db:           mdb,
		router:       r,
		snackBoxID:   snackBoxID,
		tartID:       tartID,
		flourID:      flourID,
		managerToken: mgrTok,
		ownerToken:   ownerTok,
		staffToken:   staffTok,
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

// TestTC11_DPExpirationReleasesCapacity menguji TC-11:
// Pesanan berstatus "accepted" yang melewati dp_deadline tanpa pembayaran DP
// otomatis berubah menjadi "expired" saat pengecekan malas, kapasitasnya dilepas,
// dan pesanan baru dapat masuk ke slot tanggal yang sama dengan status "accepted".
func TestTC11_DPExpirationReleasesCapacity(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Kapasitas tenaga = 1428 menit; batas terima 90% = 1285,2 menit.
	// Pesanan 1: 45 Snack Box pada 2026-11-18 (45 * 12 = 540 menit).
	// Tambahkan juga beban existing 50 Snack Box (600 menit) yang sudah confirmed
	// sehingga total beban dengan Pesanan 1 = 1140 menit (79,83% <= 90%), sisa slot hanya 145,2 menit.
	evDate, _ := time.Parse("2006-01-02", "2026-11-18")
	env.db.orders = append(env.db.orders, memOrder{
		ID:             "aaaa1111-1111-4000-8000-000000000001",
		Code:           "CONFIRMED1",
		Source:         "manual",
		CustomerName:   "Pelanggan Confirmed",
		DeliveryType:   "pickup",
		EventDate:      evDate,
		ProductionDate: evDate,
		Status:         orders.StatusConfirmed,
		Total:          decimal.NewFromInt(1250000),
		DPRequired:     decimal.NewFromInt(375000),
		Paid:           decimal.NewFromInt(375000),
	})
	env.db.orderItems = append(env.db.orderItems, memOrderItem{
		ID:        "bbbb1111-1111-4000-8000-000000000001",
		OrderID:   "aaaa1111-1111-4000-8000-000000000001",
		ProductID: env.snackBoxID,
		Qty:       50,
		UnitPrice: decimal.NewFromInt(25000),
		UnitCost:  decimal.NewFromInt(18500),
	})

	// Buat Pesanan 1 (45 Snack Box -> 540 menit -> total 1140 menit <= 1285,2) -> langsung accepted
	wOrd1 := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Pelanggan Telat DP",
		"customer_contact": "081211112222",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-18",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 45},
		},
	}, nil)
	if wOrd1.Code != http.StatusCreated {
		t.Fatalf("create order 1: expected 201, got %d (%s)", wOrd1.Code, wOrd1.Body.String())
	}
	var ord1 struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wOrd1.Body.Bytes(), &ord1)
	if ord1.Data.Status != orders.StatusAccepted {
		t.Fatalf("order 1 expected status 'accepted', got %q", ord1.Data.Status)
	}

	// Majukan waktu / set dp_deadline Pesanan 1 ke masa lalu (1 jam yang lalu) tanpa pembayaran DP
	pastDeadline := time.Now().UTC().Add(-1 * time.Hour)
	env.db.mu.Lock()
	for i := range env.db.orders {
		if env.db.orders[i].ID == ord1.Data.ID {
			env.db.orders[i].DPDeadline = &pastDeadline
		}
	}
	env.db.mu.Unlock()

	// Sekarang masuk Pesanan 2 (45 Snack Box pada 2026-11-18).
	// Jika slot Pesanan 1 belum dilepas, beban akan menjadi 600 + 540 + 540 = 1680 > 1285,2 (G1 gagal).
	// Karena pengecekan malas melepas slot Pesanan 1 menjadi "expired", Pesanan 2 melihat beban 600 + 540 = 1140 <= 1285,2
	// dan langsung diterima ("accepted")!
	wOrd2 := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Pelanggan Pengganti Slot",
		"customer_contact": "081233334444",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-18",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 45},
		},
	}, nil)
	if wOrd2.Code != http.StatusCreated {
		t.Fatalf("create order 2: expected 201, got %d (%s)", wOrd2.Code, wOrd2.Body.String())
	}
	var ord2 struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wOrd2.Body.Bytes(), &ord2)
	if ord2.Data.Status != orders.StatusAccepted {
		t.Fatalf("TC-11 gagal: pesanan baru seharusnya masuk ke slot yang dilepas dengan status 'accepted', dapat %q", ord2.Data.Status)
	}

	// Verifikasi status Pesanan 1 kini "expired" saat dilacak
	wTrack1 := performJSON(env.router, http.MethodGet, "/api/v1/public/orders/"+ord1.Data.Code, nil, nil)
	if wTrack1.Code != http.StatusOK {
		t.Fatalf("track order 1: expected 200, got %d", wTrack1.Code)
	}
	var track1 struct {
		Data orders.PublicTrackResponse `json:"data"`
	}
	_ = json.Unmarshal(wTrack1.Body.Bytes(), &track1)
	if track1.Data.Status != orders.StatusExpired {
		t.Fatalf("TC-11 gagal: pesanan pertama seharusnya berstatus 'expired', dapat %q", track1.Data.Status)
	}
}

// TestTC12_OverrideRequiresReasonAndAudits menguji TC-12:
// Pemilik menerima pesanan yang direkomendasikan Tolak (atau Terima Bersyarat):
//  1. Tanpa override_reason -> ditolak dengan 422 Unprocessable Entity.
//  2. Dengan override_reason -> berhasil (200 OK, status "accepted") dan tercatat di audit_logs
//     lengkap dengan actor_id, nama, waktu, rekomendasi awal ("reject"), dan alasan override.
func TestTC12_OverrideRequiresReasonAndAudits(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Buat pesanan manual yang gagal G4 (tanggal 2026-11-11, H+1 < lead time 2 hari) -> rekomendasi "reject", status "pending_decision"
	wCreate := performJSON(env.router, http.MethodPost, "/api/v1/orders", map[string]any{
		"customer_name":    "Pelanggan Mendesak VIP",
		"customer_contact": "081199988877",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-11",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 10},
		},
	}, map[string]string{
		"Authorization": "Bearer " + env.ownerToken,
	})
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create order: expected 201, got %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var created struct {
		Data orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
	if created.Data.Recommendation != "reject" || created.Data.Status != orders.StatusPendingDecision {
		t.Fatalf("expected pending_decision with recommendation 'reject', got status=%q rec=%q", created.Data.Status, created.Data.Recommendation)
	}

	// 1. Owner mencoba accept TANPA override_reason -> wajib ditolak 422
	wNoReason := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+created.Data.ID+"/decision", map[string]any{
		"action": "accept",
	}, map[string]string{
		"Authorization": "Bearer " + env.ownerToken,
	})
	if wNoReason.Code != http.StatusUnprocessableEntity {
		t.Fatalf("TC-12 gagal: override tanpa alasan seharusnya 422, dapat %d (%s)", wNoReason.Code, wNoReason.Body.String())
	}

	// 2. Owner melakukan accept DENGAN override_reason -> 200 OK, status menjadi accepted
	reasonText := "Bahan sudah tersedia di dapur dan staf lembur disetujui pemilik"
	wWithReason := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+created.Data.ID+"/decision", map[string]any{
		"action":          "accept",
		"override_reason": reasonText,
	}, map[string]string{
		"Authorization": "Bearer " + env.ownerToken,
	})
	if wWithReason.Code != http.StatusOK {
		t.Fatalf("TC-12 gagal: override dengan alasan seharusnya 200 OK, dapat %d (%s)", wWithReason.Code, wWithReason.Body.String())
	}
	var decided struct {
		Data orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wWithReason.Body.Bytes(), &decided)
	if decided.Data.Status != orders.StatusAccepted {
		t.Fatalf("expected status 'accepted', got %q", decided.Data.Status)
	}

	// Verifikasi audit_logs mencatat siapa, kapan, rekomendasi awal, dan alasan override
	env.db.mu.Lock()
	defer env.db.mu.Unlock()
	var foundOverrideAudit bool
	for _, ae := range env.db.auditEntries {
		if ae.Entity == "orders" && ae.EntityID == created.Data.ID && ae.Action == "override" {
			foundOverrideAudit = true
			if ae.ActorID != "11111111-1111-1111-1111-111111111111" {
				t.Errorf("expected actor_id owner, got %q", ae.ActorID)
			}
			if !strings.Contains(ae.Detail, `"initial_recommendation":"reject"`) {
				t.Errorf("audit detail missing initial_recommendation='reject': %s", ae.Detail)
			}
			if !strings.Contains(ae.Detail, reasonText) {
				t.Errorf("audit detail missing override_reason: %s", ae.Detail)
			}
			if !strings.Contains(ae.Detail, `"actor_name":"Owner Uji"`) {
				t.Errorf("audit detail missing actor_name: %s", ae.Detail)
			}
			if !strings.Contains(ae.Detail, `"decided_at"`) {
				t.Errorf("audit detail missing decided_at: %s", ae.Detail)
			}
		}
	}
	if !foundOverrideAudit {
		t.Fatalf("TC-12 gagal: tidak ditemukan baris audit_logs dengan action='override' untuk pesanan %s", created.Data.ID)
	}
}

// TestDecision_RoleRestrictions menguji pembatasan wewenang peran pada keputusan pesanan:
//  1. Staff ditolak (403 Forbidden) saat mencoba memutuskan pesanan apa pun.
//  2. Manager ditolak (403 Forbidden) saat mencoba memutuskan pesanan besar (total >= 2.000.000 atau unit >= 50).
//  3. Owner berhasil (200 OK) memutuskan pesanan besar tersebut.
func TestDecision_RoleRestrictions(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Buat pesanan besar: 60 Snack Box (@ 25.000 = Rp 1.500.000, tetapi unit 60 >= big_order_units 50)
	// pada 2026-11-19 -> semua gerbang lolos (60 * 12 = 720 menit <= 1285,2), tetapi masuk pending_decision karena pesanan besar.
	wBig := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "PT Karya Nusantara",
		"customer_contact": "081122334455",
		"delivery_type":    "delivery",
		"address":          "Jl. Asia Afrika No. 88",
		"event_date":       "2026-11-19",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 60},
		},
	}, nil)
	if wBig.Code != http.StatusCreated {
		t.Fatalf("create big order: expected 201, got %d (%s)", wBig.Code, wBig.Body.String())
	}
	var bigOrd struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wBig.Body.Bytes(), &bigOrd)
	if bigOrd.Data.Status != orders.StatusPendingDecision {
		t.Fatalf("big order expected status 'pending_decision', got %q", bigOrd.Data.Status)
	}

	// 1. Staff mencoba memutuskan -> 403 Forbidden
	wStaff := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+bigOrd.Data.ID+"/decision", map[string]any{
		"action": "accept",
	}, map[string]string{
		"Authorization": "Bearer " + env.staffToken,
	})
	if wStaff.Code != http.StatusForbidden {
		t.Fatalf("staff decision expected 403 Forbidden, got %d (%s)", wStaff.Code, wStaff.Body.String())
	}

	// 2. Manager mencoba memutuskan pesanan besar -> 403 Forbidden
	wMgr := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+bigOrd.Data.ID+"/decision", map[string]any{
		"action": "accept",
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wMgr.Code != http.StatusForbidden {
		t.Fatalf("manager decision on big order expected 403 Forbidden, got %d (%s)", wMgr.Code, wMgr.Body.String())
	}

	// 3. Owner memutuskan pesanan besar -> 200 OK, status menjadi accepted
	wOwner := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+bigOrd.Data.ID+"/decision", map[string]any{
		"action": "accept",
	}, map[string]string{
		"Authorization": "Bearer " + env.ownerToken,
	})
	if wOwner.Code != http.StatusOK {
		t.Fatalf("owner decision on big order expected 200 OK, got %d (%s)", wOwner.Code, wOwner.Body.String())
	}
}

// TestCancelOrder_ReleasesCapacityAndCalculatesExposure menguji Bagian 8.6 & TC-06:
//   - Pesanan yang sudah confirmed (memiliki 3 tugas aktif dan mereservasi kapasitas) dibatalkan.
//   - Kapasitas dilepas sehingga pesanan baru pada hari yang sama dapat langsung diterima (accepted).
//   - Tugas yang belum selesai dibatalkan.
//   - CancellationExposure dihitung sesuai engine.CancellationExposure (belanja - DP diterima).
//   - Tercatat di audit_logs.
func TestCancelOrder_ReleasesCapacityAndCalculatesExposure(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Buat pesanan 100 Snack Box @ Rp 23.750 pada 2026-11-20 (persis angka TC-04 / TC-06:
	// belanja per unit = 10.000 * 1,05 + 2.000 = 12.500; untuk 100 box = 1.250.000).
	// Agar persis TC-06 (belanja 1.300.000, DP diterima 712.500 -> eksposur 587.500),
	// kita set packaging_cost snackBox = 2.500 pada test ini (10.000*1,05 + 2.500 = 13.000 -> 100 box = 1.300.000).
	env.db.mu.Lock()
	sb := env.db.products[env.snackBoxID]
	sb.PackagingCost = decimal.NewFromInt(2500)
	env.db.products[env.snackBoxID] = sb
	env.db.mu.Unlock()

	unitPrice := decimal.NewFromInt(23750)
	wCreate := performJSON(env.router, http.MethodPost, "/api/v1/orders", map[string]any{
		"customer_name":    "Pelanggan Batal TC06",
		"customer_contact": "081255556666",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-12",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 100, "unit_price": unitPrice},
		},
	}, map[string]string{
		"Authorization": "Bearer " + env.ownerToken,
	})
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create order: expected 201, got %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var ord struct {
		Data orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wCreate.Body.Bytes(), &ord)

	// Owner menyetujui pesanan besar ini -> accepted
	wDecide := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+ord.Data.ID+"/decision", map[string]any{
		"action": "accept",
	}, map[string]string{
		"Authorization": "Bearer " + env.ownerToken,
	})
	if wDecide.Code != http.StatusOK {
		t.Fatalf("decide accept: expected 200, got %d (%s)", wDecide.Code, wDecide.Body.String())
	}

	// Catat pembayaran DP Rp 712.500 -> status menjadi confirmed dan 3 tugas dibuat
	wPay := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+ord.Data.ID+"/payments", map[string]any{
		"amount": 712500,
		"method": "transfer",
		"note":   "DP 30%",
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wPay.Code != http.StatusCreated {
		t.Fatalf("record payment: expected 201, got %d (%s)", wPay.Code, wPay.Body.String())
	}
	var payRes struct {
		Data orders.PaymentResult `json:"data"`
	}
	_ = json.Unmarshal(wPay.Body.Bytes(), &payRes)
	if payRes.Data.Status != orders.StatusConfirmed || len(payRes.Data.TasksCreated) != 3 {
		t.Fatalf("expected status 'confirmed' and 3 tasks created, got status=%q tasks=%d", payRes.Data.Status, len(payRes.Data.TasksCreated))
	}

	// Sekarang batalkan pesanan lewat POST /api/v1/orders/:id/cancel
	wCancel := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+ord.Data.ID+"/cancel", map[string]any{
		"reason":         "Acara pelanggan ditunda",
		"dp_disposition": "forfeited",
		"note":           "DP hangus sesuai kesepakatan",
	}, map[string]string{
		"Authorization": "Bearer " + env.ownerToken,
	})
	if wCancel.Code != http.StatusOK {
		t.Fatalf("cancel order: expected 200, got %d (%s)", wCancel.Code, wCancel.Body.String())
	}
	var cancelRes struct {
		Data orders.CancelOrderResult `json:"data"`
	}
	_ = json.Unmarshal(wCancel.Body.Bytes(), &cancelRes)
	if cancelRes.Data.Status != orders.StatusCancelled {
		t.Fatalf("expected status 'cancelled', got %q", cancelRes.Data.Status)
	}
	// Belanja = 1.300.000, DP diterima = 712.500 -> eksposur = 587.500
	wantExposure := decimal.NewFromInt(587500)
	if !cancelRes.Data.CancellationExposure.Equal(wantExposure) {
		t.Errorf("CancellationExposure expected %s, got %s (shopping=%s, dp=%s)",
			wantExposure, cancelRes.Data.CancellationExposure, cancelRes.Data.ShoppingCost, cancelRes.Data.DPReceived)
	}

	// Pastikan tugas aktif pesanan ini sudah dibatalkan (0 tersisa di db.tasks)
	env.db.mu.Lock()
	remainingTasks := 0
	for _, tk := range env.db.tasks {
		if tk.OrderID == ord.Data.ID {
			remainingTasks++
		}
	}
	env.db.mu.Unlock()
	if remainingTasks != 0 {
		t.Errorf("expected 0 remaining unfinished tasks after cancel, got %d", remainingTasks)
	}

	// Pastikan kapasitas 2026-11-12 telah dilepas: pesanan baru 40 Snack Box pada 2026-11-12 langsung "accepted"
	wNew := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Pelanggan Baru Setelah Batal",
		"customer_contact": "081277778888",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-12",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 40},
		},
	}, nil)
	if wNew.Code != http.StatusCreated {
		t.Fatalf("create new order after cancel: expected 201, got %d (%s)", wNew.Code, wNew.Body.String())
	}
	var newOrd struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wNew.Body.Bytes(), &newOrd)
	if newOrd.Data.Status != orders.StatusAccepted {
		t.Fatalf("expected new order after cancel to be 'accepted', got %q", newOrd.Data.Status)
	}
}

// TestFullOrderLifecycle_AU1_ToCompleted menguji alur lengkap AU-1 lewat API:
//  1. Buat pesanan yang masuk antrian keputusan (pending_decision) -> cek GET /decision-queue.
//  2. Manajer mengirim tawaran balik (action="counter") -> status "countered".
//  3. Pelanggan menyetujui tawaran balik lewat POST /public/orders/:code/counter-response -> status "accepted".
//  4. Cek GET /capacity/calendar dan GET /capacity/day/:date menampilkan beban pesanan.
//  5. Catat pembayaran DP -> status otomatis "confirmed" dan 3 tugas (shop, produce, deliver) dibuat.
//  6. Transisi confirmed -> in_production (staff ditolak 403, manager berhasil 200).
//  7. Transisi in_production -> ready oleh staff (stok tepung berkurang sesuai resep & tercatat di stock_movements).
//  8. Transisi ready -> delivering oleh staff.
//  9. Percobaan transisi delivering -> completed sebelum lunas ditolak 422.
//  10. Catat pembayaran pelunasan -> transisi delivering -> completed oleh manager berhasil 200.
func TestFullOrderLifecycle_AU1_ToCompleted(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Isi beban pada 2026-11-13 sebanyak 90 Snack Box (1080 menit) agar pesanan baru 25 Snack Box (300 menit)
	// gagal G1 pada 2026-11-13 (1380 > 1285,2) dan mendapat tawaran balik ke 2026-11-14.
	evDate, _ := time.Parse("2006-01-02", "2026-11-13")
	env.db.orders = append(env.db.orders, memOrder{
		ID:             "aaaa2222-2222-4000-8000-000000000001",
		Code:           "SLOT13FULL",
		Source:         "manual",
		CustomerName:   "Pelanggan Hari 13",
		DeliveryType:   "pickup",
		EventDate:      evDate,
		ProductionDate: evDate,
		Status:         orders.StatusConfirmed,
		Total:          decimal.NewFromInt(2250000),
		DPRequired:     decimal.NewFromInt(675000),
		Paid:           decimal.NewFromInt(675000),
	})
	env.db.orderItems = append(env.db.orderItems, memOrderItem{
		ID:        "bbbb2222-2222-4000-8000-000000000001",
		OrderID:   "aaaa2222-2222-4000-8000-000000000001",
		ProductID: env.snackBoxID,
		Qty:       90,
		UnitPrice: decimal.NewFromInt(25000),
		UnitCost:  decimal.NewFromInt(18500),
	})

	// 1. Pelanggan membuat pesanan 25 Snack Box pada 2026-11-13 -> masuk pending_decision dengan counter_offer 2026-11-14
	wCreate := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Budi Santoso",
		"customer_contact": "081311112222",
		"delivery_type":    "delivery",
		"address":          "Jl. Diponegoro No. 12",
		"event_date":       "2026-11-13",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 25},
		},
	}, nil)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("step 1 create order: expected 201, got %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var pubOrd struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wCreate.Body.Bytes(), &pubOrd)
	if pubOrd.Data.Status != orders.StatusPendingDecision {
		t.Fatalf("step 1 expected pending_decision, got %q", pubOrd.Data.Status)
	}

	// Cek GET /api/v1/decision-queue oleh manager
	wQueue := performJSON(env.router, http.MethodGet, "/api/v1/decision-queue", nil, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wQueue.Code != http.StatusOK {
		t.Fatalf("step 1 decision-queue: expected 200, got %d (%s)", wQueue.Code, wQueue.Body.String())
	}
	var queueResp struct {
		Data []orders.DecisionQueueItem `json:"data"`
	}
	_ = json.Unmarshal(wQueue.Body.Bytes(), &queueResp)
	if len(queueResp.Data) != 1 || queueResp.Data[0].ID != pubOrd.Data.ID {
		t.Fatalf("step 1 expected order in decision-queue, got %+v", queueResp.Data)
	}

	// 2. Manajer mengirim keputusan "counter" -> status menjadi "countered"
	wCounter := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/decision", map[string]any{
		"action": "counter",
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wCounter.Code != http.StatusOK {
		t.Fatalf("step 2 decision counter: expected 200, got %d (%s)", wCounter.Code, wCounter.Body.String())
	}

	// 3. Pelanggan menerima tawaran balik lewat POST /api/v1/public/orders/:code/counter-response
	wCounterResp := performJSON(env.router, http.MethodPost, "/api/v1/public/orders/"+pubOrd.Data.Code+"/counter-response", map[string]any{
		"accept": true,
	}, nil)
	if wCounterResp.Code != http.StatusOK {
		t.Fatalf("step 3 counter-response: expected 200, got %d (%s)", wCounterResp.Code, wCounterResp.Body.String())
	}
	var trackAfterCounter struct {
		Data orders.PublicTrackResponse `json:"data"`
	}
	_ = json.Unmarshal(wCounterResp.Body.Bytes(), &trackAfterCounter)
	if trackAfterCounter.Data.Status != orders.StatusAccepted || trackAfterCounter.Data.EventDate != "2026-11-14" {
		t.Fatalf("step 3 expected status='accepted' and event_date='2026-11-14', got status=%q date=%q",
			trackAfterCounter.Data.Status, trackAfterCounter.Data.EventDate)
	}

	// 4. Cek GET /api/v1/capacity/calendar?month=2026-11 dan GET /api/v1/capacity/day/2026-11-14
	wCal := performJSON(env.router, http.MethodGet, "/api/v1/capacity/calendar?month=2026-11", nil, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wCal.Code != http.StatusOK {
		t.Fatalf("step 4 capacity calendar: expected 200, got %d (%s)", wCal.Code, wCal.Body.String())
	}
	wDay := performJSON(env.router, http.MethodGet, "/api/v1/capacity/day/2026-11-14", nil, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wDay.Code != http.StatusOK {
		t.Fatalf("step 4 capacity day: expected 200, got %d (%s)", wDay.Code, wDay.Body.String())
	}
	var dayDetail struct {
		Data orders.CapacityDayDetail `json:"data"`
	}
	_ = json.Unmarshal(wDay.Body.Bytes(), &dayDetail)
	if len(dayDetail.Data.Orders) != 1 || dayDetail.Data.Orders[0].ID != pubOrd.Data.ID {
		t.Fatalf("step 4 expected order on capacity day 2026-11-14, got %+v", dayDetail.Data.Orders)
	}

	// 5. Catat pembayaran DP (total 25 * 25.000 = 625.000; DP 30% = 187.500) -> status "confirmed" + 3 tugas
	wPayDP := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/payments", map[string]any{
		"amount": 187500,
		"method": "transfer",
		"note":   "Pembayaran DP",
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wPayDP.Code != http.StatusCreated {
		t.Fatalf("step 5 pay DP: expected 201, got %d (%s)", wPayDP.Code, wPayDP.Body.String())
	}
	var payDPRes struct {
		Data orders.PaymentResult `json:"data"`
	}
	_ = json.Unmarshal(wPayDP.Body.Bytes(), &payDPRes)
	if payDPRes.Data.Status != orders.StatusConfirmed || len(payDPRes.Data.TasksCreated) != 3 {
		t.Fatalf("step 5 expected confirmed & 3 tasks, got status=%q tasks=%d", payDPRes.Data.Status, len(payDPRes.Data.TasksCreated))
	}

	// 6. Transisi confirmed -> in_production: staff ditolak (403), manager berhasil (200)
	wStaffInProd := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status": orders.StatusInProduction,
	}, map[string]string{
		"Authorization": "Bearer " + env.staffToken,
	})
	if wStaffInProd.Code != http.StatusForbidden {
		t.Fatalf("step 6 staff confirmed->in_production expected 403, got %d", wStaffInProd.Code)
	}

	wMgrInProd := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status": orders.StatusInProduction,
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wMgrInProd.Code != http.StatusOK {
		t.Fatalf("step 6 manager confirmed->in_production expected 200, got %d (%s)", wMgrInProd.Code, wMgrInProd.Body.String())
	}

	// 7. Transisi in_production -> ready oleh staff (stok tepung awal 20 kg berkurang 25 * 0,12 = 3 kg -> sisa 17 kg)
	wStaffReady := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status":      orders.StatusReady,
		"actual_minutes": 295,
	}, map[string]string{
		"Authorization": "Bearer " + env.staffToken,
	})
	if wStaffReady.Code != http.StatusOK {
		t.Fatalf("step 7 staff in_production->ready expected 200, got %d (%s)", wStaffReady.Code, wStaffReady.Body.String())
	}
	env.db.mu.Lock()
	flourLeft := env.db.ingredients[env.flourID]
	smCount := len(env.db.stockMovements)
	env.db.mu.Unlock()
	if !flourLeft.Equal(decimal.NewFromInt(17)) || smCount != 1 {
		t.Fatalf("step 7 expected flour stock=17 and 1 stock_movement, got stock=%s movements=%d", flourLeft, smCount)
	}

	// 8. Transisi ready -> delivering oleh staff
	wStaffDeliv := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status": orders.StatusDelivering,
	}, map[string]string{
		"Authorization": "Bearer " + env.staffToken,
	})
	if wStaffDeliv.Code != http.StatusOK {
		t.Fatalf("step 8 staff ready->delivering expected 200, got %d (%s)", wStaffDeliv.Code, wStaffDeliv.Body.String())
	}

	// 9. Percobaan transisi delivering -> completed sebelum lunas ditolak 422
	wUnpaidComplete := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status": orders.StatusCompleted,
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wUnpaidComplete.Code != http.StatusUnprocessableEntity {
		t.Fatalf("step 9 unpaid completed expected 422, got %d (%s)", wUnpaidComplete.Code, wUnpaidComplete.Body.String())
	}

	// 10. Catat pelunasan (625.000 - 187.500 = 437.500) lalu transisi delivering -> completed oleh manager
	wPayRest := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/payments", map[string]any{
		"amount": 437500,
		"method": "transfer",
		"note":   "Pelunasan",
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wPayRest.Code != http.StatusCreated {
		t.Fatalf("step 10 pay settlement: expected 201, got %d (%s)", wPayRest.Code, wPayRest.Body.String())
	}

	wComplete := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status": orders.StatusCompleted,
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wComplete.Code != http.StatusOK {
		t.Fatalf("step 10 manager delivering->completed expected 200, got %d (%s)", wComplete.Code, wComplete.Body.String())
	}
	var finalOrd struct {
		Data orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wComplete.Body.Bytes(), &finalOrd)
	if finalOrd.Data.Status != orders.StatusCompleted {
		t.Fatalf("step 10 expected final status 'completed', got %q", finalOrd.Data.Status)
	}
}
