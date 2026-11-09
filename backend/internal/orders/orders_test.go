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
	mu      sync.Mutex
	users   map[string]*auth.User
	byEmail map[string]*auth.UserWithHash
	seq     int
}

func (s *testUserStore) FindByEmail(_ context.Context, email string) (*auth.UserWithHash, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byEmail != nil {
		if u, ok := s.byEmail[strings.ToLower(email)]; ok {
			return u, nil
		}
	}
	return nil, auth.ErrNotFound
}

func (s *testUserStore) FindByID(_ context.Context, id string) (*auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func (s *testUserStore) CreateCustomer(_ context.Context, email, name, contact, passwordHash string) (*auth.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byEmail == nil {
		s.byEmail = make(map[string]*auth.UserWithHash)
	}
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if _, exists := s.byEmail[normEmail]; exists {
		return nil, auth.ErrEmailTaken
	}
	for _, existing := range s.users {
		if strings.EqualFold(existing.Email, normEmail) {
			return nil, auth.ErrEmailTaken
		}
	}
	s.seq++
	id := fmt.Sprintf("99999999-9999-4000-8000-%012d", s.seq)
	u := auth.User{
		ID:        id,
		Email:     normEmail,
		Name:      name,
		Contact:   contact,
		Role:      auth.RoleCustomer,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
	}
	s.users[id] = &u
	s.byEmail[normEmail] = &auth.UserWithHash{User: u, PasswordHash: passwordHash}
	return &u, nil
}

type testEnv struct {
	db             *memDB
	router         *gin.Engine
	snackBoxID     string
	tartID         string
	flourID        string
	managerToken   string
	ownerToken     string
	staffToken     string
	customerID     string
	customerToken  string
	customer2Token string
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
	mdb.ingredientDetails[flourID] = memIngredient{
		ID:          flourID,
		Name:        "Tepung Terigu Protein Sedang",
		UseUnit:     "kg",
		BuyUnit:     "sak",
		BuyUnitSize: decimal.NewFromInt(25),
		BuyPrice:    decimal.NewFromInt(275000),
		MinStock:    decimal.NewFromInt(5),
		Rounding:    "loose",
		IsActive:    true,
	}
	mdb.recipes[snackBoxID] = map[string]decimal.Decimal{
		flourID: decimal.RequireFromString("0.12"), // 0,12 kg tepung per snack box
	}
	mdb.recipes[tartID] = map[string]decimal.Decimal{
		flourID: decimal.RequireFromString("0.40"), // 0,40 kg tepung per tart
	}

	// 4. Pengguna uji (owner, manager, staff, & customer)
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
			"44444444-4444-4444-4444-444444444444": {
				ID: "44444444-4444-4444-4444-444444444444", Email: "customer@test.local", Name: "Pelanggan Uji", Contact: "081234567890", Role: auth.RoleCustomer, IsActive: true, CreatedAt: now,
			},
			"55555555-5555-5555-5555-555555555555": {
				ID: "55555555-5555-5555-5555-555555555555", Email: "customer2@test.local", Name: "Pelanggan Kedua", Contact: "081299990000", Role: auth.RoleCustomer, IsActive: true, CreatedAt: now,
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
	custTok, _, _ := auth.IssueToken(secret, "44444444-4444-4444-4444-444444444444", auth.RoleCustomer, time.Now(), auth.TokenTTL)
	cust2Tok, _, _ := auth.IssueToken(secret, "55555555-5555-5555-5555-555555555555", auth.RoleCustomer, time.Now(), auth.TokenTTL)

	return &testEnv{
		db:             mdb,
		router:         r,
		snackBoxID:     snackBoxID,
		tartID:         tartID,
		flourID:        flourID,
		managerToken:   mgrTok,
		ownerToken:     ownerTok,
		staffToken:     staffTok,
		customerID:     "44444444-4444-4444-4444-444444444444",
		customerToken:  custTok,
		customer2Token: cust2Tok,
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
			tok := env.customerToken
			if idx == 1 {
				tok = env.customer2Token
			}
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
				"Authorization":   "Bearer " + tok,
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
	}, map[string]string{
		"Authorization": "Bearer " + env.customerToken,
	})
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
	}, map[string]string{
		"Authorization": "Bearer " + env.customerToken,
	})
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
	}, map[string]string{
		"Authorization": "Bearer " + env.customer2Token,
	})
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
	}, map[string]string{
		"Authorization": "Bearer " + env.customerToken,
	})
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
	}, map[string]string{
		"Authorization": "Bearer " + env.customerToken,
	})
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
	}, map[string]string{
		"Authorization": "Bearer " + env.customerToken,
	})
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

	// 6. Transisi confirmed -> in_production: staff ditolak (403)
	wStaffInProd := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status": orders.StatusInProduction,
	}, map[string]string{
		"Authorization": "Bearer " + env.staffToken,
	})
	if wStaffInProd.Code != http.StatusForbidden {
		t.Fatalf("step 6 staff confirmed->in_production expected 403, got %d", wStaffInProd.Code)
	}

	// 6a. Uji validasi tanggal operasional: pada demo_today 2026-11-10, pesanan dengan jadwal produksi 2026-11-14 ditolak 422
	wMgrEarly := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+pubOrd.Data.ID+"/transition", map[string]any{
		"to_status": orders.StatusInProduction,
	}, map[string]string{
		"Authorization": "Bearer " + env.managerToken,
	})
	if wMgrEarly.Code != http.StatusUnprocessableEntity {
		t.Fatalf("step 6a manager early in_production expected 422, got %d (%s)", wMgrEarly.Code, wMgrEarly.Body.String())
	}

	// Majukan demo_today ke Hari-H tanggal produksi & acara (2026-11-14)
	env.db.mu.Lock()
	env.db.settings["demo_today"] = `"2026-11-14"`
	env.db.mu.Unlock()

	// 6b. Setelah tanggal jadwal produksi tiba, manager berhasil memulai status in_production (200)
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

// TestTahap4APlus_CustomerAuthOrdersAndActiveLimit menguji seluruh ketentuan Tahap 4A+:
//  1. POST /api/v1/auth/register berhasil (201 + cookie + token), duplikat email ditolak (409), validasi gagal (422).
//  2. POST /api/v1/public/orders tanpa login -> 401; dengan peran non-customer -> 403.
//  3. POST /api/v1/public/orders dengan login customer mengisi customer_id dan membaca nama/kontak dari profil bila kosong.
//  4. Batas max_active_unpaid_orders (default 2): pesanan belum bayar DP ke-3 ditolak dengan 422; setelah salah satu dibayar DP, pesanan ke-3 berhasil.
//  5. GET /api/v1/me/orders mengembalikan daftar pesanan milik pelanggan dengan field aman saja.
//  6. Peran customer ditolak (403) pada endpoint internal.
func TestTahap4APlus_CustomerAuthOrdersAndActiveLimit(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// 1a. Registrasi validasi gagal (kata sandi < 8 karakter, email salah) -> 422
	wRegInvalid := performJSON(env.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"name":     "Siti",
		"email":    "bukan-email",
		"contact":  "081200001111",
		"password": "123",
	}, nil)
	if wRegInvalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("register invalid expected 422, got %d (%s)", wRegInvalid.Code, wRegInvalid.Body.String())
	}

	// 1b. Registrasi berhasil -> 201 Created + cookie sesi + user role customer
	wRegOK := performJSON(env.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"name":     "Siti Rahmawati",
		"email":    "siti.rahma@demo.test",
		"contact":  "081288889999",
		"password": "PasswordAman123!",
	}, nil)
	if wRegOK.Code != http.StatusCreated {
		t.Fatalf("register valid expected 201, got %d (%s)", wRegOK.Code, wRegOK.Body.String())
	}
	var regResp struct {
		Data struct {
			Token string    `json:"token"`
			User  auth.User `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wRegOK.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("unmarshal register response: %v", err)
	}
	if regResp.Data.User.Role != auth.RoleCustomer || regResp.Data.User.Contact != "081288889999" || regResp.Data.Token == "" {
		t.Fatalf("unexpected registered user: %+v", regResp.Data)
	}
	sitiID := regResp.Data.User.ID
	sitiHdr := map[string]string{"Authorization": "Bearer " + regResp.Data.Token}

	// 1c. Registrasi email duplikat -> 409 Conflict
	wRegDup := performJSON(env.router, http.MethodPost, "/api/v1/auth/register", map[string]any{
		"name":     "Siti Kedua",
		"email":    "SITI.RAHMA@demo.test",
		"contact":  "081200009999",
		"password": "PasswordAman123!",
	}, nil)
	if wRegDup.Code != http.StatusConflict {
		t.Fatalf("register duplicate email expected 409, got %d (%s)", wRegDup.Code, wRegDup.Body.String())
	}

	// 2a. POST /public/orders tanpa login -> 401 Unauthorized
	wUnauth := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"delivery_type": "pickup",
		"event_date":    "2026-11-15",
		"items":         []map[string]any{{"product_id": env.snackBoxID, "qty": 10}},
	}, nil)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("POST /public/orders unauthenticated expected 401, got %d (%s)", wUnauth.Code, wUnauth.Body.String())
	}

	// 2b. POST /public/orders dengan peran staff -> 403 Forbidden
	wStaffOrd := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Staff",
		"customer_contact": "0811",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-15",
		"items":            []map[string]any{{"product_id": env.snackBoxID, "qty": 10}},
	}, map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wStaffOrd.Code != http.StatusForbidden {
		t.Fatalf("POST /public/orders by staff expected 403, got %d (%s)", wStaffOrd.Code, wStaffOrd.Body.String())
	}

	// 3. POST /public/orders dengan login customer (tanpa mengisi customer_name & customer_contact -> dibaca dari profil Siti)
	wOrd1 := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"delivery_type": "pickup",
		"event_date":    "2026-11-15",
		"items":         []map[string]any{{"product_id": env.snackBoxID, "qty": 10}},
	}, sitiHdr)
	if wOrd1.Code != http.StatusCreated {
		t.Fatalf("order 1 by customer expected 201, got %d (%s)", wOrd1.Code, wOrd1.Body.String())
	}
	var ord1 struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wOrd1.Body.Bytes(), &ord1)
	if ord1.Data.CustomerID == nil || *ord1.Data.CustomerID != sitiID {
		t.Fatalf("expected customer_id=%s, got %v", sitiID, ord1.Data.CustomerID)
	}

	// 4. Pesanan aktif belum bayar DP ke-2 untuk Siti -> berhasil (201, masuk batas max_active_unpaid_orders = 2)
	wOrd2 := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Siti Rahmawati (Kantor)",
		"customer_contact": "081277770000",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-16",
		"items":            []map[string]any{{"product_id": env.snackBoxID, "qty": 10}},
	}, sitiHdr)
	if wOrd2.Code != http.StatusCreated {
		t.Fatalf("order 2 by customer expected 201, got %d (%s)", wOrd2.Code, wOrd2.Body.String())
	}

	// Pesanan ke-3 sementara pesanan 1 & 2 belum bayar DP -> ditolak 422 (active_order_limit)
	wOrd3Blocked := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"delivery_type": "pickup",
		"event_date":    "2026-11-17",
		"items":         []map[string]any{{"product_id": env.snackBoxID, "qty": 10}},
	}, sitiHdr)
	if wOrd3Blocked.Code != http.StatusUnprocessableEntity {
		t.Fatalf("3rd active unpaid order expected 422, got %d (%s)", wOrd3Blocked.Code, wOrd3Blocked.Body.String())
	}
	if !strings.Contains(wOrd3Blocked.Body.String(), "active_order_limit") {
		t.Errorf("expected error code active_order_limit, got %s", wOrd3Blocked.Body.String())
	}

	// Setelah pesanan ke-1 dibayar DP (status berubah ke confirmed), Siti boleh membuat pesanan baru lagi!
	wPay1 := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+ord1.Data.ID+"/payments", map[string]any{
		"amount": ord1.Data.DPRequired,
		"method": "transfer",
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wPay1.Code != http.StatusCreated {
		t.Fatalf("pay DP order 1 expected 201, got %d (%s)", wPay1.Code, wPay1.Body.String())
	}

	wOrd3Allowed := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"delivery_type": "pickup",
		"event_date":    "2026-11-17",
		"items":         []map[string]any{{"product_id": env.snackBoxID, "qty": 10}},
	}, sitiHdr)
	if wOrd3Allowed.Code != http.StatusCreated {
		t.Fatalf("order 3 after paying DP on order 1 expected 201, got %d (%s)", wOrd3Allowed.Code, wOrd3Allowed.Body.String())
	}

	// 5. GET /api/v1/me/orders oleh Siti mengembalikan 3 pesanan milik Siti saja, hanya field aman
	wMyOrders := performJSON(env.router, http.MethodGet, "/api/v1/me/orders", nil, sitiHdr)
	if wMyOrders.Code != http.StatusOK {
		t.Fatalf("GET /me/orders expected 200, got %d (%s)", wMyOrders.Code, wMyOrders.Body.String())
	}
	rawMyOrders := wMyOrders.Body.String()
	for _, secretField := range []string{"unit_cost", "margin", "snapshot", "cash_lowest"} {
		if strings.Contains(rawMyOrders, secretField) {
			t.Errorf("GET /me/orders leaked secret field %q: %s", secretField, rawMyOrders)
		}
	}
	var myOrdersResp struct {
		Data []orders.PublicTrackResponse `json:"data"`
	}
	if err := json.Unmarshal(wMyOrders.Body.Bytes(), &myOrdersResp); err != nil {
		t.Fatalf("unmarshal /me/orders: %v", err)
	}
	if len(myOrdersResp.Data) != 3 {
		t.Fatalf("expected 3 orders in /me/orders for Siti, got %d", len(myOrdersResp.Data))
	}

	// 6. Peran customer ditolak (403 Forbidden) pada endpoint internal
	internalEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/orders"},
		{http.MethodPost, "/api/v1/orders"},
		{http.MethodGet, "/api/v1/decision-queue"},
		{http.MethodGet, "/api/v1/capacity/calendar"},
		{http.MethodGet, "/api/v1/settings"},
		{http.MethodPut, "/api/v1/settings"},
		{http.MethodGet, "/api/v1/products"},
		{http.MethodGet, "/api/v1/ingredients"},
		{http.MethodGet, "/api/v1/users"},
		{http.MethodGet, "/api/v1/tasks"},
	}
	for _, ep := range internalEndpoints {
		w := performJSON(env.router, ep.method, ep.path, nil, sitiHdr)
		if w.Code != http.StatusForbidden {
			t.Fatalf("customer accessing internal %s %s expected 403, got %d (%s)", ep.method, ep.path, w.Code, w.Body.String())
		}
	}
}

// TestTahap4B_TasksAndOrderDetailStaffRedaction menguji endpoint Tahap 4B:
//  1. GET /api/v1/orders dengan filter status dan date.
//  2. GET /api/v1/orders/:id untuk manager/owner memuat items, evaluation.gates, payments, tasks, dan audits.
//  3. GET /api/v1/orders dan GET /api/v1/orders/:id untuk staff membuang seluruh field sensitif
//     (unit_price, unit_cost, subtotal, total, dp_required, paid, evaluation, margin, cash_lowest, payments).
//  4. GET /api/v1/tasks (filter mine, status, date) dan PATCH /api/v1/tasks/:id (status & actual_minutes);
//     respons untuk staff tidak memuat harga, margin, atau kas; customer ditolak 403.
func TestTahap4B_TasksAndOrderDetailStaffRedaction(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Buat pesanan manual pada 2026-11-18 -> otomatis accepted
	wCreate := performJSON(env.router, http.MethodPost, "/api/v1/orders", map[string]any{
		"customer_name":    "Dinas Pendidikan Kota",
		"customer_contact": "081233334444",
		"delivery_type":    "delivery",
		"address":          "Jl. Merdeka No. 10",
		"notes":            "Antar pagi jam 08.00",
		"event_date":       "2026-11-18",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 20},
		},
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create manual order: expected 201, got %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var ord struct {
		Data orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wCreate.Body.Bytes(), &ord)

	// Catat pembayaran DP -> status confirmed + 3 tugas (shop, produce, deliver) dibuat
	wPay := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+ord.Data.ID+"/payments", map[string]any{
		"amount": 150000,
		"method": "transfer",
		"note":   "DP 30%",
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wPay.Code != http.StatusCreated {
		t.Fatalf("record payment: expected 201, got %d (%s)", wPay.Code, wPay.Body.String())
	}

	// 1. GET /api/v1/orders?status=confirmed&date=2026-11-18 oleh manager
	wListMgr := performJSON(env.router, http.MethodGet, "/api/v1/orders?status=confirmed&date=2026-11-18", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wListMgr.Code != http.StatusOK {
		t.Fatalf("GET /orders by manager: expected 200, got %d (%s)", wListMgr.Code, wListMgr.Body.String())
	}
	var listMgr struct {
		Data []orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wListMgr.Body.Bytes(), &listMgr)
	if len(listMgr.Data) != 1 || listMgr.Data[0].ID != ord.Data.ID {
		t.Fatalf("expected 1 filtered order on 2026-11-18, got %d", len(listMgr.Data))
	}

	// Filter tanggal lain harus kosong
	wListOther := performJSON(env.router, http.MethodGet, "/api/v1/orders?date=2026-11-25", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	var listOther struct {
		Data []orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wListOther.Body.Bytes(), &listOther)
	if len(listOther.Data) != 0 {
		t.Fatalf("expected 0 orders on 2026-11-25, got %d", len(listOther.Data))
	}

	// Fitur 5: Pencarian cepat order (search by code, with hash, and customer name)
	wSearchCode := performJSON(env.router, http.MethodGet, "/api/v1/orders?search="+ord.Data.Code, nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wSearchCode.Code != http.StatusOK {
		t.Fatalf("search by code: expected 200, got %d (%s)", wSearchCode.Code, wSearchCode.Body.String())
	}
	var searchCodeRes struct {
		Data []orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wSearchCode.Body.Bytes(), &searchCodeRes)
	if len(searchCodeRes.Data) != 1 || searchCodeRes.Data[0].ID != ord.Data.ID {
		t.Fatalf("expected search by code to find 1 order, got %d", len(searchCodeRes.Data))
	}

	// Search with '#' prefix
	wSearchHash := performJSON(env.router, http.MethodGet, "/api/v1/orders?search=%23"+ord.Data.Code, nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wSearchHash.Code != http.StatusOK {
		t.Fatalf("search with hash: expected 200, got %d", wSearchHash.Code)
	}
	var searchHashRes struct {
		Data []orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wSearchHash.Body.Bytes(), &searchHashRes)
	if len(searchHashRes.Data) != 1 || searchHashRes.Data[0].ID != ord.Data.ID {
		t.Fatalf("expected search with hash prefix to find 1 order, got %d", len(searchHashRes.Data))
	}

	// Search with non-matching term should return 0
	wSearchEmpty := performJSON(env.router, http.MethodGet, "/api/v1/orders?search=NONEXISTENT_QUERY_XYZ", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	var searchEmptyRes struct {
		Data []orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wSearchEmpty.Body.Bytes(), &searchEmptyRes)
	if len(searchEmptyRes.Data) != 0 {
		t.Fatalf("expected 0 orders for non-existent search, got %d", len(searchEmptyRes.Data))
	}

	// 2. GET /api/v1/orders/:id oleh manager -> wajib memuat items, evaluation.gates, payments, tasks, audits
	wDetMgr := performJSON(env.router, http.MethodGet, "/api/v1/orders/"+ord.Data.ID, nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wDetMgr.Code != http.StatusOK {
		t.Fatalf("GET /orders/:id by manager: expected 200, got %d (%s)", wDetMgr.Code, wDetMgr.Body.String())
	}
	var detMgr struct {
		Data orders.OrderDetail `json:"data"`
	}
	if err := json.Unmarshal(wDetMgr.Body.Bytes(), &detMgr); err != nil {
		t.Fatalf("unmarshal manager order detail: %v", err)
	}
	if len(detMgr.Data.Items) != 1 || len(detMgr.Data.Evaluation.Gates) != 4 ||
		len(detMgr.Data.Payments) != 1 || len(detMgr.Data.Tasks) != 3 || len(detMgr.Data.Audits) == 0 {
		t.Fatalf("manager order detail incomplete: items=%d gates=%d payments=%d tasks=%d audits=%d",
			len(detMgr.Data.Items), len(detMgr.Data.Evaluation.Gates),
			len(detMgr.Data.Payments), len(detMgr.Data.Tasks), len(detMgr.Data.Audits))
	}

	// 3. GET /api/v1/orders/:id dan GET /api/v1/orders oleh staff -> TIDAK BOLEH memuat field harga/margin/kas/pembayaran
	wDetStaff := performJSON(env.router, http.MethodGet, "/api/v1/orders/"+ord.Data.ID, nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wDetStaff.Code != http.StatusOK {
		t.Fatalf("GET /orders/:id by staff: expected 200, got %d (%s)", wDetStaff.Code, wDetStaff.Body.String())
	}
	rawStaffBody := wDetStaff.Body.String()
	for _, forbidden := range []string{
		"unit_price", "unit_cost", "subtotal", `"total"`, "dp_required", `"paid"`,
		"margin", "cash_lowest", "payments", "evaluation", "counter_offer",
	} {
		if strings.Contains(rawStaffBody, forbidden) {
			t.Errorf("staff GET /orders/:id leaked sensitive field %s: %s", forbidden, rawStaffBody)
		}
	}

	// 4. GET /api/v1/tasks oleh staff (filter mine=true, status=todo, date=2026-11-18)
	wTasksStaff := performJSON(env.router, http.MethodGet, "/api/v1/tasks?mine=true&status=todo&date=2026-11-18", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wTasksStaff.Code != http.StatusOK {
		t.Fatalf("GET /tasks by staff: expected 200, got %d (%s)", wTasksStaff.Code, wTasksStaff.Body.String())
	}
	rawTasksStaff := wTasksStaff.Body.String()
	for _, forbidden := range []string{"unit_price", "unit_cost", "margin", "cash", "dp_required", `"total"`} {
		if strings.Contains(rawTasksStaff, forbidden) {
			t.Errorf("staff GET /tasks leaked sensitive field %s: %s", forbidden, rawTasksStaff)
		}
	}
	var tasksResp struct {
		Data []orders.TaskItem `json:"data"`
	}
	_ = json.Unmarshal(wTasksStaff.Body.Bytes(), &tasksResp)
	// Pada tanggal 2026-11-18 terdapat tugas produce dan deliver (tugas shop jatuh pada H-2 = 2026-11-16)
	if len(tasksResp.Data) != 2 {
		t.Fatalf("expected 2 tasks due on 2026-11-18 (produce & deliver), got %d", len(tasksResp.Data))
	}
	var produceTaskID string
	for _, tk := range tasksResp.Data {
		if tk.Type == "produce" {
			produceTaskID = tk.ID
		}
	}
	if produceTaskID == "" {
		t.Fatalf("produce task not found on 2026-11-18")
	}

	// 5. PATCH /api/v1/tasks/:id oleh staff (ubah status ke doing lalu done dengan actual_minutes = 230)
	wPatchDoing := performJSON(env.router, http.MethodPatch, "/api/v1/tasks/"+produceTaskID, map[string]any{
		"status": "doing",
	}, map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wPatchDoing.Code != http.StatusOK {
		t.Fatalf("PATCH /tasks/:id doing: expected 200, got %d (%s)", wPatchDoing.Code, wPatchDoing.Body.String())
	}

	wPatchDone := performJSON(env.router, http.MethodPatch, "/api/v1/tasks/"+produceTaskID, map[string]any{
		"status":         "done",
		"actual_minutes": 230,
	}, map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wPatchDone.Code != http.StatusOK {
		t.Fatalf("PATCH /tasks/:id done: expected 200, got %d (%s)", wPatchDone.Code, wPatchDone.Body.String())
	}
	var patchedTask struct {
		Data orders.TaskItem `json:"data"`
	}
	_ = json.Unmarshal(wPatchDone.Body.Bytes(), &patchedTask)
	if patchedTask.Data.Status != "done" || patchedTask.Data.ActualMinutes == nil || *patchedTask.Data.ActualMinutes != 230 {
		t.Fatalf("unexpected patched task: %+v", patchedTask.Data)
	}

	// 6. Customer ditolak 403 saat mencoba PATCH /api/v1/tasks/:id
	wCustPatch := performJSON(env.router, http.MethodPatch, "/api/v1/tasks/"+produceTaskID, map[string]any{
		"status": "todo",
	}, map[string]string{"Authorization": "Bearer " + env.customerToken})
	if wCustPatch.Code != http.StatusForbidden {
		t.Fatalf("customer PATCH /tasks/:id expected 403, got %d (%s)", wCustPatch.Code, wCustPatch.Body.String())
	}
}

// TestTahap5_CashProjectionTC04AndTC06AndRBAC menguji skenario TC-04, TC-06, dan RBAC kas pada tingkat layanan:
//   - TC-04: Saldo awal H-2 (2026-11-10) = 1.800.000, belanja = 1.300.000 di H-2, DP pesanan lain +500.000 di H-1 (2026-11-11),
//     biaya tetap = 400.000/hari, buffer = 400.000, pesanan 100 snack box @ 23.750 (total 2.375.000) di H0 (2026-11-12):
//     titik terendah tanpa DP = -200.000, MinDP = 600.000, DPCharged = 712.500, titik terendah dengan DP = 512.500.
//   - RBAC GET /api/v1/cash/projection: owner melihat lengkap; manager hanya status lolos/tidak dan titik terendah tanpa opening_cash; staff 403.
//   - POST /api/v1/cash/entries: owner berhasil (201), manager & staff ditolak (403).
//   - TC-06: Pembatalan pesanan dengan belanja 1.300.000 dan DP diterima 712.500 menghasilkan cancellation_exposure = 587.500
//     dan menghitung ulang proyeksi kas.
func TestTahap5_CashProjectionTC04AndTC06AndRBAC(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Setel parameter sesuai vektor uji TC-04:
	// Today = 2026-11-10 (H-2), EventDate = 2026-11-12 (H0), OpeningCash = 1.800.000, FixedCost = 400.000, Buffer = 400.000.
	// Agar purchase_cost_per_unit snack box tepat 13.000:
	// recipeCost = 10.476,190476... * 1,05 + packaging 2.000 = 13.000 -> kita setel waste_pct = 0.05, recipeCost = 10000, packaging = 2500 -> 10000*1.05 + 2500 = 13.000!
	env.db.mu.Lock()
	env.db.settings["opening_cash"] = "1800000"
	env.db.settings["fixed_cost_daily"] = "400000"
	env.db.settings["cash_buffer"] = "400000"
	env.db.settings["dp_pct"] = "0.30"
	env.db.settings["shopping_lead_days"] = "2"
	env.db.settings["lead_time_days"] = "2"
	sp := env.db.products[env.snackBoxID]
	sp.PackagingCost = decimal.NewFromInt(2500) // 10.000 * 1,05 + 2.500 = 13.000 purchase_cost_per_unit
	sp.OverheadCost = decimal.NewFromInt(900)   // 10.500 + 2.500 + 4.800 + 900 = 18.700 unit_cost
	env.db.products[env.snackBoxID] = sp
	// Tambahkan DP pesanan lain +500.000 pada H-1 (2026-11-11)
	hMinus1, _ := time.Parse("2006-01-02", "2026-11-11")
	env.db.cashEntries = append(env.db.cashEntries, memCashEntry{
		ID:        "ce-tc04-other-dp",
		EntryDate: hMinus1,
		Amount:    decimal.NewFromInt(500000),
		Note:      "DP pesanan lain H-1",
	})
	env.db.mu.Unlock()

	// 1. Buat pesanan manual 100 box @ 23.750 pada H0 (2026-11-12) -> total = 2.375.000, belanja = 1.300.000
	wCreate := performJSON(env.router, http.MethodPost, "/api/v1/orders", map[string]any{
		"customer_name":    "Pelanggan TC-04",
		"customer_contact": "081200000004",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-12",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 100, "unit_price": 23750},
		},
	}, map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create TC-04 order: expected 201, got %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var created struct {
		Data orders.OrderDetail `json:"data"`
	}
	if err := json.Unmarshal(wCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal TC-04 order: %v", err)
	}
	if !created.Data.Total.Equal(decimal.NewFromInt(2375000)) {
		t.Fatalf("TC-04 expected total 2.375.000, got %s", created.Data.Total)
	}
	if !created.Data.DPRequired.Equal(decimal.NewFromInt(712500)) {
		t.Fatalf("TC-04 expected DPCharged 712.500, got %s", created.Data.DPRequired)
	}
	if !created.Data.Evaluation.Impact.CashLowestAfter.Equal(decimal.NewFromInt(512500)) {
		t.Fatalf("TC-04 expected CashLowestAfter 512.500, got %s", created.Data.Evaluation.Impact.CashLowestAfter)
	}

	// Setujui pesanan oleh owner (karena 100 unit >= big_order_units 50) dan catat pembayaran DP 712.500 pada H-2
	wDecide := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+created.Data.ID+"/decision", map[string]any{
		"action": "accept",
	}, map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wDecide.Code != http.StatusOK {
		t.Fatalf("decide accept TC-04 order: expected 200, got %d (%s)", wDecide.Code, wDecide.Body.String())
	}

	wPayDP := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+created.Data.ID+"/payments", map[string]any{
		"amount": 712500,
		"method": "transfer",
		"note":   "DP TC-04",
	}, map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wPayDP.Code != http.StatusCreated {
		t.Fatalf("pay DP TC-04: expected 201, got %d (%s)", wPayDP.Code, wPayDP.Body.String())
	}

	// 2. Uji GET /api/v1/cash/projection?days=4 oleh Owner (jendela H-2 s.d. H+1)
	wCashOwner := performJSON(env.router, http.MethodGet, "/api/v1/cash/projection?days=4", nil,
		map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wCashOwner.Code != http.StatusOK {
		t.Fatalf("GET /cash/projection by owner: expected 200, got %d (%s)", wCashOwner.Code, wCashOwner.Body.String())
	}
	var cashOwner struct {
		Data orders.CashProjectionResult `json:"data"`
	}
	if err := json.Unmarshal(wCashOwner.Body.Bytes(), &cashOwner); err != nil {
		t.Fatalf("unmarshal owner cash projection: %v", err)
	}
	if !cashOwner.Data.Passed {
		t.Fatalf("TC-04 owner cash projection expected passed=true, got false")
	}
	if !cashOwner.Data.Lowest.Equal(decimal.NewFromInt(512500)) || cashOwner.Data.LowestDate != "2026-11-12" {
		t.Fatalf("TC-04 expected lowest=512500 on 2026-11-12, got %s on %s", cashOwner.Data.Lowest, cashOwner.Data.LowestDate)
	}
	if cashOwner.Data.OpeningCash == nil || !cashOwner.Data.OpeningCash.Equal(decimal.NewFromInt(1800000)) {
		t.Fatalf("TC-04 owner expected opening_cash=1800000, got %v", cashOwner.Data.OpeningCash)
	}
	if len(cashOwner.Data.Days) != 4 {
		t.Fatalf("TC-04 owner expected 4 daily balances, got %d", len(cashOwner.Data.Days))
	}

	// 3. Uji GET /api/v1/cash/projection?days=4 oleh Manager: hanya status lolos/tidak dan titik terendah tanpa opening_cash/days
	wCashMgr := performJSON(env.router, http.MethodGet, "/api/v1/cash/projection?days=4", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wCashMgr.Code != http.StatusOK {
		t.Fatalf("GET /cash/projection by manager: expected 200, got %d (%s)", wCashMgr.Code, wCashMgr.Body.String())
	}
	rawMgrCash := wCashMgr.Body.String()
	for _, hidden := range []string{"opening_cash", "fixed_cost_daily", `"days"`, `"entries"`} {
		if strings.Contains(rawMgrCash, hidden) {
			t.Errorf("manager GET /cash/projection leaked %s: %s", hidden, rawMgrCash)
		}
	}
	var cashMgr struct {
		Data orders.CashProjectionResult `json:"data"`
	}
	_ = json.Unmarshal(wCashMgr.Body.Bytes(), &cashMgr)
	if !cashMgr.Data.Passed || !cashMgr.Data.Lowest.Equal(decimal.NewFromInt(512500)) {
		t.Fatalf("manager cash projection mismatch: passed=%v lowest=%s", cashMgr.Data.Passed, cashMgr.Data.Lowest)
	}

	// 4. Uji GET /api/v1/cash/projection oleh Staff -> 403 Forbidden
	wCashStaff := performJSON(env.router, http.MethodGet, "/api/v1/cash/projection?days=14", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wCashStaff.Code != http.StatusForbidden {
		t.Fatalf("staff GET /cash/projection expected 403, got %d", wCashStaff.Code)
	}

	// 5. Uji TC-06: Batalkan pesanan (belanja 1.300.000, DP diterima 712.500 -> eksposur 587.500)
	wCancel := performJSON(env.router, http.MethodPost, "/api/v1/orders/"+created.Data.ID+"/cancel", map[string]any{
		"reason":         "Uji TC-06 Pembatalan",
		"dp_disposition": "forfeited",
	}, map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wCancel.Code != http.StatusOK {
		t.Fatalf("TC-06 cancel order: expected 200, got %d (%s)", wCancel.Code, wCancel.Body.String())
	}
	var cancelRes struct {
		Data orders.CancelOrderResult `json:"data"`
	}
	_ = json.Unmarshal(wCancel.Body.Bytes(), &cancelRes)
	if !cancelRes.Data.ShoppingCost.Equal(decimal.NewFromInt(1300000)) {
		t.Fatalf("TC-06 expected shopping_cost=1.300.000, got %s", cancelRes.Data.ShoppingCost)
	}
	if !cancelRes.Data.DPReceived.Equal(decimal.NewFromInt(712500)) {
		t.Fatalf("TC-06 expected dp_received=712.500, got %s", cancelRes.Data.DPReceived)
	}
	if !cancelRes.Data.CancellationExposure.Equal(decimal.NewFromInt(587500)) {
		t.Fatalf("TC-06 expected cancellation_exposure=587.500, got %s", cancelRes.Data.CancellationExposure)
	}

	// 6. Uji POST /api/v1/cash/entries: manager ditolak 403, owner berhasil 201
	wEntryMgr := performJSON(env.router, http.MethodPost, "/api/v1/cash/entries", map[string]any{
		"entry_date": "2026-11-10",
		"amount":     250000,
		"note":       "Suntikan kas",
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wEntryMgr.Code != http.StatusForbidden {
		t.Fatalf("manager POST /cash/entries expected 403, got %d", wEntryMgr.Code)
	}

	wEntryOwner := performJSON(env.router, http.MethodPost, "/api/v1/cash/entries", map[string]any{
		"entry_date": "2026-11-10",
		"amount":     250000,
		"note":       "Suntikan modal kerja",
	}, map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wEntryOwner.Code != http.StatusCreated {
		t.Fatalf("owner POST /cash/entries expected 201, got %d (%s)", wEntryOwner.Code, wEntryOwner.Body.String())
	}
}

// TestTahap5_ShoppingListTC05PurchasesAndStockAdjust menguji skenario TC-05, pembelian, koreksi stok, dan RBAC:
//   - TC-05: 6 tart (6 x 0,4 = 2,4 kg) + 100 snack box (100 x 0,12 = 12,0 kg) = 14,4 kg tepung;
//     buffer 10% (0,10); stok awal 3 kg -> need = 12,84 kg;
//     aturan loose -> buy = 13 kg; aturan pack (25 kg) -> buy = 25 kg (1 sak).
//   - Tanggal beli = tanggal acara dikurangi shopping_lead_days.
//   - Staff melihat daftar belanja tanpa buy_price dan estimated_cost.
//   - POST /api/v1/purchases menambah stok, memperbarui buy_price, menulis stock_movements & cash_entries.
//   - POST /api/v1/stock/adjust menolak tanpa alasan (422) dan berhasil dengan alasan.
func TestTahap5_ShoppingListTC05PurchasesAndStockAdjust(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// Setel stok tepung = 3 kg (di bawah min_stock = 5 kg), rounding = "loose", packSize = 25 kg
	env.db.mu.Lock()
	env.db.ingredients[env.flourID] = decimal.NewFromInt(3)
	im := env.db.ingredientDetails[env.flourID]
	im.Rounding = "loose"
	im.BuyUnitSize = decimal.NewFromInt(25)
	im.BuyPrice = decimal.NewFromInt(275000)
	im.MinStock = decimal.NewFromInt(5)
	env.db.ingredientDetails[env.flourID] = im

	// Buat pesanan confirmed berisi 6 tart + 100 snack box pada tanggal acara 2026-11-15
	evDate, _ := time.Parse("2006-01-02", "2026-11-15")
	ordID := "dddd0000-0000-4000-8000-000000000005"
	env.db.orders = append(env.db.orders, memOrder{
		ID:             ordID,
		Code:           "TC05SHOP99",
		Source:         "manual",
		CustomerName:   "Pelanggan TC-05",
		DeliveryType:   "pickup",
		EventDate:      evDate,
		ProductionDate: evDate,
		Status:         orders.StatusConfirmed,
		Total:          decimal.NewFromInt(4180000),
		DPRequired:     decimal.NewFromInt(1254000),
		Paid:           decimal.NewFromInt(1254000),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	})
	env.db.orderItems = append(env.db.orderItems,
		memOrderItem{ID: "oi-tc05-1", OrderID: ordID, ProductID: env.tartID, Qty: 6, UnitPrice: decimal.NewFromInt(280000), UnitCost: decimal.NewFromInt(180000)},
		memOrderItem{ID: "oi-tc05-2", OrderID: ordID, ProductID: env.snackBoxID, Qty: 100, UnitPrice: decimal.NewFromInt(25000), UnitCost: decimal.NewFromInt(19000)},
	)
	env.db.mu.Unlock()

	// 1. Uji TC-05 dengan rounding = "loose" -> required = 14.4, need = 12.84, buy = 13 kg, purchase_date = 2026-11-13
	wShopLoose := performJSON(env.router, http.MethodGet, "/api/v1/shopping-list?from=2026-11-10&to=2026-11-20", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wShopLoose.Code != http.StatusOK {
		t.Fatalf("GET /shopping-list (loose): expected 200, got %d (%s)", wShopLoose.Code, wShopLoose.Body.String())
	}
	var shopLoose struct {
		Data orders.ShoppingListResult `json:"data"`
	}
	if err := json.Unmarshal(wShopLoose.Body.Bytes(), &shopLoose); err != nil {
		t.Fatalf("unmarshal shopping list loose: %v", err)
	}
	if len(shopLoose.Data.Items) != 1 {
		t.Fatalf("expected 1 shopping item, got %d", len(shopLoose.Data.Items))
	}
	itLoose := shopLoose.Data.Items[0]
	if !itLoose.RequiredQty.Equal(decimal.RequireFromString("14.4")) {
		t.Fatalf("TC-05 expected required_qty=14.4, got %s", itLoose.RequiredQty)
	}
	if !itLoose.NeedQty.Equal(decimal.RequireFromString("12.84")) {
		t.Fatalf("TC-05 expected need_qty=12.84, got %s", itLoose.NeedQty)
	}
	if !itLoose.BuyUseUnits.Equal(decimal.NewFromInt(13)) {
		t.Fatalf("TC-05 loose expected buy_use_units=13, got %s", itLoose.BuyUseUnits)
	}
	if itLoose.PurchaseDate != "2026-11-13" {
		t.Fatalf("TC-05 expected purchase_date=2026-11-13 (2026-11-15 - 2), got %s", itLoose.PurchaseDate)
	}
	if !itLoose.BelowMinStock || len(shopLoose.Data.LowStockItems) != 1 {
		t.Fatalf("expected below_min_stock=true and 1 low_stock_item, got below=%v lowStock=%d", itLoose.BelowMinStock, len(shopLoose.Data.LowStockItems))
	}

	// 2. Uji TC-05 dengan rounding = "pack" (packSize = 25 kg) -> buy_use_units = 25 kg, qty_buy_units = 1 sak
	env.db.mu.Lock()
	imPack := env.db.ingredientDetails[env.flourID]
	imPack.Rounding = "pack"
	env.db.ingredientDetails[env.flourID] = imPack
	env.db.mu.Unlock()

	wShopPack := performJSON(env.router, http.MethodGet, "/api/v1/shopping-list?from=2026-11-10&to=2026-11-20", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	var shopPack struct {
		Data orders.ShoppingListResult `json:"data"`
	}
	_ = json.Unmarshal(wShopPack.Body.Bytes(), &shopPack)
	itPack := shopPack.Data.Items[0]
	if !itPack.BuyUseUnits.Equal(decimal.NewFromInt(25)) || !itPack.QtyBuyUnits.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("TC-05 pack expected buy_use_units=25 and qty_buy_units=1, got buy_use=%s qty_buy=%s",
			itPack.BuyUseUnits, itPack.QtyBuyUnits)
	}

	// 3. Uji RBAC Staff pada GET /api/v1/shopping-list -> buy_price dan estimated_cost tidak boleh dikirim
	wShopStaff := performJSON(env.router, http.MethodGet, "/api/v1/shopping-list?from=2026-11-10&to=2026-11-20", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wShopStaff.Code != http.StatusOK {
		t.Fatalf("GET /shopping-list by staff: expected 200, got %d", wShopStaff.Code)
	}
	rawShopStaff := wShopStaff.Body.String()
	for _, forbidden := range []string{"buy_price", "estimated_cost", "total_estimated_cost"} {
		if strings.Contains(rawShopStaff, forbidden) {
			t.Errorf("staff GET /shopping-list leaked %s: %s", forbidden, rawShopStaff)
		}
	}

	// 4. Uji POST /api/v1/purchases: beli 1 sak (25 kg) dengan harga aktual Rp 280.000
	//    -> stok naik dari 3 kg menjadi 28 kg, buy_price menjadi 280.000, stock_movements & cash_entries tercatat
	wPurchase := performJSON(env.router, http.MethodPost, "/api/v1/purchases", map[string]any{
		"ingredient_id": env.flourID,
		"qty_buy_units": 1,
		"unit_price":    280000,
		"note":          "Beli 1 sak tepung terigu",
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wPurchase.Code != http.StatusCreated {
		t.Fatalf("POST /purchases: expected 201, got %d (%s)", wPurchase.Code, wPurchase.Body.String())
	}
	var purchRes struct {
		Data orders.PurchaseResult `json:"data"`
	}
	_ = json.Unmarshal(wPurchase.Body.Bytes(), &purchRes)
	if !purchRes.Data.StockBefore.Equal(decimal.NewFromInt(3)) || !purchRes.Data.StockAfter.Equal(decimal.NewFromInt(28)) {
		t.Fatalf("purchase stock expected 3 -> 28, got %s -> %s", purchRes.Data.StockBefore, purchRes.Data.StockAfter)
	}
	if purchRes.Data.BelowMinStock {
		t.Fatalf("after buying 25 kg, below_min_stock should be false")
	}

	// 5. Uji POST /api/v1/stock/adjust: tanpa alasan -> 422; dengan alasan -> berhasil
	wAdjNoReason := performJSON(env.router, http.MethodPost, "/api/v1/stock/adjust", map[string]any{
		"ingredient_id": env.flourID,
		"delta":         -2,
		"reason":        "   ",
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wAdjNoReason.Code != http.StatusUnprocessableEntity {
		t.Fatalf("stock adjust without reason expected 422, got %d (%s)", wAdjNoReason.Code, wAdjNoReason.Body.String())
	}

	wAdjOK := performJSON(env.router, http.MethodPost, "/api/v1/stock/adjust", map[string]any{
		"ingredient_id": env.flourID,
		"delta":         -2.5,
		"reason":        "Koreksi stok opname akhir pekan",
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wAdjOK.Code != http.StatusOK {
		t.Fatalf("stock adjust with reason expected 200, got %d (%s)", wAdjOK.Code, wAdjOK.Body.String())
	}
	var adjRes struct {
		Data orders.StockAdjustResult `json:"data"`
	}
	_ = json.Unmarshal(wAdjOK.Body.Bytes(), &adjRes)
	if !adjRes.Data.StockAfter.Equal(decimal.RequireFromString("25.5")) {
		t.Fatalf("expected stock_after=25.5, got %s", adjRes.Data.StockAfter)
	}
}

// TestTahap6_KPICalculationsAndDashboardsAndAuditLogsRBAC menguji seluruh perhitungan KPI Bagian 10,
// ketiga endpoint dashboard per peran (/dashboard/owner, /dashboard/manager, /dashboard/staff),
// endpoint /audit-logs dengan filter & paginasi, serta penegakan RBAC ketat (staff tidak menerima
// harga, margin, atau kas dari endpoint dashboard manapun).
func TestTahap6_KPICalculationsAndDashboardsAndAuditLogsRBAC(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	ev12, _ := time.Parse("2006-01-02", "2026-11-12")
	ev13, _ := time.Parse("2006-01-02", "2026-11-13")
	ev14, _ := time.Parse("2006-01-02", "2026-11-14")
	onTimeComplete := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC) // selesai tepat pada 2026-11-12 (<= event_date 2026-11-12)
	lateComplete := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)   // selesai 2026-11-15 (> event_date 2026-11-13)
	counterJSON := []byte(`{"date":"2026-11-13","min_price_per_item":{"bbbb0000-0000-4000-8000-000000000001":"25000"},"dp":"525000"}`)

	env.db.mu.Lock()
	// Setel stok tepung = 3 kg (< min_stock 5 kg) untuk memicu low_stock_alerts
	env.db.ingredients[env.flourID] = decimal.NewFromInt(3)
	// Harga bahan aktual naik +10%: dari 10.000 (snapshot saat pesanan dibuat -> unit_cost 18.500)
	// menjadi 11.000 (biaya bahan aktual + waste 5% = 11.550 -> actual unit_cost = 19.550)
	env.db.recipeCosts[env.snackBoxID] = decimal.NewFromInt(11000)

	// Fixture 7 pesanan untuk menguji seluruh rumus KPI:
	// 1. Completed tepat waktu (30 snack box @ 25.000, snapshot unit_cost 18.500, prod 2026-11-12)
	// 2. Completed terlambat & berasal dari counter_offer yang diterima (70 snack box @ 25.000, snapshot unit_cost 18.500, prod 2026-11-13)
	// 3. Cancelled (pernah diterima)
	// 4. Expired (pernah diterima)
	// 5. Countered (masih menunggu respons pelanggan)
	// 6. Rejected setelah counter_offer
	// 7. Rejected langsung tanpa counter_offer
	env.db.orders = []memOrder{
		{
			ID: "e6000000-0000-4000-8000-000000000001", Code: "KPI0000001", Source: "public",
			CustomerName: "Pelanggan Tepat Waktu", CustomerContact: "0811000001", DeliveryType: "delivery", Address: "Jl. Melati 1",
			EventDate: ev12, ProductionDate: ev12, Status: orders.StatusCompleted,
			Total: decimal.NewFromInt(750000), DPRequired: decimal.NewFromInt(225000), Paid: decimal.NewFromInt(750000),
			Recommendation: "accept", CreatedAt: onTimeComplete.Add(-48 * time.Hour), UpdatedAt: onTimeComplete,
		},
		{
			ID: "e6000000-0000-4000-8000-000000000002", Code: "KPI0000002", Source: "public",
			CustomerName: "Pelanggan Telat Counter", CustomerContact: "0811000002", DeliveryType: "pickup",
			EventDate: ev13, ProductionDate: ev13, Status: orders.StatusCompleted,
			Total: decimal.NewFromInt(1750000), DPRequired: decimal.NewFromInt(525000), Paid: decimal.NewFromInt(1750000),
			Recommendation: "accept_with_conditions", CounterOfferJSON: counterJSON,
			CreatedAt: onTimeComplete.Add(-48 * time.Hour), UpdatedAt: lateComplete,
		},
		{
			ID: "e6000000-0000-4000-8000-000000000003", Code: "KPI0000003", Source: "public",
			CustomerName: "Pelanggan Batal", CustomerContact: "0811000003", DeliveryType: "pickup",
			EventDate: ev14, ProductionDate: ev14, Status: orders.StatusCancelled,
			Total: decimal.NewFromInt(500000), DPRequired: decimal.NewFromInt(150000), Paid: decimal.NewFromInt(150000),
			Recommendation: "accept", CreatedAt: onTimeComplete, UpdatedAt: onTimeComplete,
		},
		{
			ID: "e6000000-0000-4000-8000-000000000004", Code: "KPI0000004", Source: "public",
			CustomerName: "Pelanggan Expired", CustomerContact: "0811000004", DeliveryType: "pickup",
			EventDate: ev14, ProductionDate: ev14, Status: orders.StatusExpired,
			Total: decimal.NewFromInt(500000), DPRequired: decimal.NewFromInt(150000), Paid: decimal.Zero,
			Recommendation: "accept", CreatedAt: onTimeComplete, UpdatedAt: onTimeComplete,
		},
		{
			ID: "e6000000-0000-4000-8000-000000000005", Code: "KPI0000005", Source: "public",
			CustomerName: "Pelanggan Masih Counter", CustomerContact: "0811000005", DeliveryType: "pickup",
			EventDate: ev14, ProductionDate: ev14, Status: orders.StatusCountered,
			Total: decimal.NewFromInt(500000), DPRequired: decimal.NewFromInt(150000), Paid: decimal.Zero,
			Recommendation: "accept_with_conditions", CounterOfferJSON: counterJSON,
			CreatedAt: onTimeComplete, UpdatedAt: onTimeComplete,
		},
		{
			ID: "e6000000-0000-4000-8000-000000000006", Code: "KPI0000006", Source: "public",
			CustomerName: "Pelanggan Tolak Counter", CustomerContact: "0811000006", DeliveryType: "pickup",
			EventDate: ev14, ProductionDate: ev14, Status: orders.StatusRejected,
			Total: decimal.NewFromInt(500000), DPRequired: decimal.NewFromInt(150000), Paid: decimal.Zero,
			Recommendation: "accept_with_conditions", CounterOfferJSON: counterJSON,
			CreatedAt: onTimeComplete, UpdatedAt: onTimeComplete,
		},
		{
			ID: "e6000000-0000-4000-8000-000000000007", Code: "KPI0000007", Source: "public",
			CustomerName: "Pelanggan Tolak Langsung", CustomerContact: "0811000007", DeliveryType: "pickup",
			EventDate: ev14, ProductionDate: ev14, Status: orders.StatusRejected,
			Total: decimal.NewFromInt(500000), DPRequired: decimal.NewFromInt(150000), Paid: decimal.Zero,
			Recommendation: "reject",
			CreatedAt:      onTimeComplete, UpdatedAt: onTimeComplete,
		},
	}

	env.db.orderItems = []memOrderItem{
		{ID: "oi-kpi-1", OrderID: "e6000000-0000-4000-8000-000000000001", ProductID: env.snackBoxID, Qty: 30, UnitPrice: decimal.NewFromInt(25000), UnitCost: decimal.NewFromInt(18500)},
		{ID: "oi-kpi-2", OrderID: "e6000000-0000-4000-8000-000000000002", ProductID: env.snackBoxID, Qty: 70, UnitPrice: decimal.NewFromInt(25000), UnitCost: decimal.NewFromInt(18500)},
	}

	// Tambahkan tugas operasional untuk menguji /dashboard/staff (hari ini = 2026-11-10)
	env.db.tasks = []memTask{
		{
			ID: "dddd0000-0000-4000-8000-000000000011", OrderID: "e6000000-0000-4000-8000-000000000001",
			Type: "produce", Title: "Produksi KPI0000001", DueDate: "2026-11-10", Status: "doing", CreatedAt: onTimeComplete,
		},
		{
			ID: "dddd0000-0000-4000-8000-000000000012", OrderID: "e6000000-0000-4000-8000-000000000001",
			Type: "deliver", Title: "Antar KPI0000001", DueDate: "2026-11-12", Status: "todo", CreatedAt: onTimeComplete,
		},
	}

	// Tambahkan catatan audit untuk menguji /audit-logs
	env.db.auditEntries = []memAuditEntry{
		{
			ID: "aaaa1111-0000-4000-8000-000000000001", ActorID: "11111111-1111-1111-1111-111111111111",
			Action: "override", Entity: "orders", EntityID: "e6000000-0000-4000-8000-000000000001",
			Detail: `{"override_reason":"Pelanggan prioritas"}`, CreatedAt: onTimeComplete,
		},
		{
			ID: "aaaa1111-0000-4000-8000-000000000002", ActorID: "11111111-1111-1111-1111-111111111111",
			Action: "setting", Entity: "settings", EntityID: "",
			Detail: `{"key":"margin_min","value":"0.20"}`, CreatedAt: onTimeComplete.Add(time.Minute),
		},
	}
	env.db.mu.Unlock()

	// 1. Uji GET /api/v1/kpi oleh Owner
	wKPI := performJSON(env.router, http.MethodGet, "/api/v1/kpi", nil,
		map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wKPI.Code != http.StatusOK {
		t.Fatalf("GET /kpi by owner: expected 200, got %d (%s)", wKPI.Code, wKPI.Body.String())
	}
	var kpiResp struct {
		Data orders.KPIResult `json:"data"`
	}
	if err := json.Unmarshal(wKPI.Body.Bytes(), &kpiResp); err != nil {
		t.Fatalf("unmarshal KPI: %v", err)
	}
	kpi := kpiResp.Data

	// Verifikasi KPI 1: Ketepatan waktu (1 tepat waktu dari 2 selesai = 0.5000)
	if kpi.CompletedOrders != 2 || kpi.OnTimeOrders != 1 || !kpi.OnTimeRate.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("KPI on_time_rate mismatch: completed=%d on_time=%d rate=%s", kpi.CompletedOrders, kpi.OnTimeOrders, kpi.OnTimeRate)
	}

	// Verifikasi KPI 2: Utilisasi rata-rata (hari 1: 360/1428 = 0.25210084, hari 2: 840/1428 = 0.58823529 -> rata-rata = 0.4202)
	if kpi.ActiveProductionDays != 2 || !kpi.AvgUtilization.Equal(decimal.RequireFromString("0.4202")) {
		t.Fatalf("KPI avg_utilization expected 0.4202 across 2 days, got %s across %d days", kpi.AvgUtilization, kpi.ActiveProductionDays)
	}

	// Verifikasi KPI 3: Selisih biaya bahan ((1.155.000 - 1.050.000) / 1.050.000 = 0.1000)
	if !kpi.EstimatedIngredientCost.Equal(decimal.NewFromInt(1050000)) ||
		!kpi.ActualIngredientCost.Equal(decimal.NewFromInt(1155000)) ||
		!kpi.IngredientCostVariance.Equal(decimal.RequireFromString("0.1")) {
		t.Fatalf("KPI ingredient_cost_variance mismatch: est=%s act=%s var=%s",
			kpi.EstimatedIngredientCost, kpi.ActualIngredientCost, kpi.IngredientCostVariance)
	}

	// Verifikasi KPI 4: Tingkat pembatalan (2 cancelled/expired dari 4 accepted-or-beyond = 0.5000)
	if kpi.CancelledOrders != 1 || kpi.ExpiredOrders != 1 || kpi.AcceptedOrdersTotal != 4 ||
		!kpi.CancellationRate.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("KPI cancellation_rate mismatch: cancelled=%d expired=%d total=%d rate=%s",
			kpi.CancelledOrders, kpi.ExpiredOrders, kpi.AcceptedOrdersTotal, kpi.CancellationRate)
	}

	// Verifikasi KPI 5: Pesanan ditawar balik atau ditolak
	cr := kpi.CounteredOrRejected
	if cr.TotalCountered != 3 || cr.CounterPending != 1 || cr.CounterAccepted != 1 ||
		cr.CounterRejected != 1 || cr.DirectRejected != 1 || cr.TotalRejected != 2 || cr.TotalCounteredOrRejected != 4 {
		t.Fatalf("KPI countered_or_rejected mismatch: %+v", cr)
	}

	// Verifikasi KPI 6: Margin realisasi (est = (2.500.000 - 1.850.000)/2.500.000 = 0.26, real = (2.500.000 - 1.955.000)/2.500.000 = 0.218)
	if !kpi.EstimatedMargin.Equal(decimal.RequireFromString("0.26")) ||
		!kpi.RealizedMargin.Equal(decimal.RequireFromString("0.218")) ||
		len(kpi.OrdersMargin) != 2 {
		t.Fatalf("KPI margin mismatch: est=%s real=%s len(orders)=%d",
			kpi.EstimatedMargin, kpi.RealizedMargin, len(kpi.OrdersMargin))
	}

	// 2. Uji RBAC pada GET /api/v1/kpi: Staff ditolak 403
	wKPIStaff := performJSON(env.router, http.MethodGet, "/api/v1/kpi", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wKPIStaff.Code != http.StatusForbidden {
		t.Fatalf("staff GET /kpi expected 403, got %d", wKPIStaff.Code)
	}

	// 3. Uji GET /api/v1/dashboard/owner: Owner 200, Manager 403, Staff 403
	wDashOwner := performJSON(env.router, http.MethodGet, "/api/v1/dashboard/owner", nil,
		map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wDashOwner.Code != http.StatusOK {
		t.Fatalf("owner GET /dashboard/owner expected 200, got %d (%s)", wDashOwner.Code, wDashOwner.Body.String())
	}
	wDashOwnerByMgr := performJSON(env.router, http.MethodGet, "/api/v1/dashboard/owner", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wDashOwnerByMgr.Code != http.StatusForbidden {
		t.Fatalf("manager GET /dashboard/owner expected 403, got %d", wDashOwnerByMgr.Code)
	}
	wDashOwnerByStaff := performJSON(env.router, http.MethodGet, "/api/v1/dashboard/owner", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wDashOwnerByStaff.Code != http.StatusForbidden {
		t.Fatalf("staff GET /dashboard/owner expected 403, got %d", wDashOwnerByStaff.Code)
	}

	// 4. Uji GET /api/v1/dashboard/manager: Manager 200 (tanpa opening_cash), Staff 403
	wDashMgr := performJSON(env.router, http.MethodGet, "/api/v1/dashboard/manager", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wDashMgr.Code != http.StatusOK {
		t.Fatalf("manager GET /dashboard/manager expected 200, got %d (%s)", wDashMgr.Code, wDashMgr.Body.String())
	}
	if strings.Contains(wDashMgr.Body.String(), "opening_cash") {
		t.Fatalf("manager dashboard leaked opening_cash: %s", wDashMgr.Body.String())
	}
	wDashMgrByStaff := performJSON(env.router, http.MethodGet, "/api/v1/dashboard/manager", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wDashMgrByStaff.Code != http.StatusForbidden {
		t.Fatalf("staff GET /dashboard/manager expected 403, got %d", wDashMgrByStaff.Code)
	}

	// 5. Uji GET /api/v1/dashboard/staff: Staff 200 dan dijamin TANPA harga, margin, atau kas
	wDashStaff := performJSON(env.router, http.MethodGet, "/api/v1/dashboard/staff", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wDashStaff.Code != http.StatusOK {
		t.Fatalf("staff GET /dashboard/staff expected 200, got %d (%s)", wDashStaff.Code, wDashStaff.Body.String())
	}
	rawStaffDash := wDashStaff.Body.String()
	for _, forbidden := range []string{
		"unit_price", "unit_cost", "subtotal", `"total"`, `"price"`,
		"margin", "dp_required", `"paid"`, "opening_cash", "cash_buffer",
		"buy_price", "estimated_cost", "cash_lowest",
	} {
		if strings.Contains(rawStaffDash, forbidden) {
			t.Errorf("staff GET /dashboard/staff leaked %s: %s", forbidden, rawStaffDash)
		}
	}
	var staffDash struct {
		Data orders.StaffDashboardResult `json:"data"`
	}
	if err := json.Unmarshal(wDashStaff.Body.Bytes(), &staffDash); err != nil {
		t.Fatalf("unmarshal staff dashboard: %v", err)
	}
	if len(staffDash.Data.TodayTasks) != 1 || len(staffDash.Data.Deliveries) != 1 || len(staffDash.Data.LowStockAlerts) != 1 {
		t.Fatalf("unexpected staff dashboard counts: today=%d deliveries=%d lowStock=%d",
			len(staffDash.Data.TodayTasks), len(staffDash.Data.Deliveries), len(staffDash.Data.LowStockAlerts))
	}

	// 6. Uji GET /api/v1/audit-logs: Owner 200 dengan filter & paginasi, Manager & Staff ditolak 403
	wAudMgr := performJSON(env.router, http.MethodGet, "/api/v1/audit-logs", nil,
		map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wAudMgr.Code != http.StatusForbidden {
		t.Fatalf("manager GET /audit-logs expected 403, got %d", wAudMgr.Code)
	}
	wAudStaff := performJSON(env.router, http.MethodGet, "/api/v1/audit-logs", nil,
		map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wAudStaff.Code != http.StatusForbidden {
		t.Fatalf("staff GET /audit-logs expected 403, got %d", wAudStaff.Code)
	}

	wAudOwner := performJSON(env.router, http.MethodGet, "/api/v1/audit-logs?action=override&page=1&limit=10", nil,
		map[string]string{"Authorization": "Bearer " + env.ownerToken})
	if wAudOwner.Code != http.StatusOK {
		t.Fatalf("owner GET /audit-logs expected 200, got %d (%s)", wAudOwner.Code, wAudOwner.Body.String())
	}
	var audResp struct {
		Data []orders.AuditLogItem `json:"data"`
		Meta struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(wAudOwner.Body.Bytes(), &audResp); err != nil {
		t.Fatalf("unmarshal audit logs: %v", err)
	}
	if audResp.Meta.Total != 1 || len(audResp.Data) != 1 || audResp.Data[0].Action != "override" {
		t.Fatalf("audit logs filter mismatch: meta=%+v data=%+v", audResp.Meta, audResp.Data)
	}
}

// TestTaxPctAndPlatformFee_DynamicCalculationAndPublicTracking menguji kalkulasi PPN & biaya aplikasi:
// 1. Pengaturan tax_pct = 0.11 (11%) dan platform_fee = 2000 (Rp 2.000) di settings.
// 2. Pembuatan pesanan publik/manual menghasilkan Subtotal, PPN (tax_amount), Biaya Aplikasi (platform_fee), dan Total yang tepat.
// 3. GET /api/v1/public/config mengembalikan tax_pct & platform_fee publik yang aman.
// 4. Lacak pesanan publik (GET /api/v1/public/orders/:code) menyajikan rincian subtotal, PPN, dan platform_fee tanpa membocorkan margin/biaya hpp.
func TestTaxPctAndPlatformFee_DynamicCalculationAndPublicTracking(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// 1. Set tax_pct = 0.11 dan platform_fee = 2000 di settings memDB
	env.db.mu.Lock()
	env.db.settings["tax_pct"] = "0.11"
	env.db.settings["platform_fee"] = "2000"
	env.db.mu.Unlock()

	// 2. Verifikasi GET /api/v1/public/config
	wCfg := performJSON(env.router, http.MethodGet, "/api/v1/public/config", nil, nil)
	if wCfg.Code != http.StatusOK {
		t.Fatalf("GET /public/config expected 200, got %d", wCfg.Code)
	}
	var cfgResp struct {
		Data struct {
			TaxPct      decimal.Decimal `json:"tax_pct"`
			PlatformFee decimal.Decimal `json:"platform_fee"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wCfg.Body.Bytes(), &cfgResp); err != nil {
		t.Fatalf("unmarshal public config: %v", err)
	}
	if !cfgResp.Data.TaxPct.Equal(decimal.RequireFromString("0.11")) {
		t.Fatalf("expected public config tax_pct 0.11, got %s", cfgResp.Data.TaxPct)
	}
	if !cfgResp.Data.PlatformFee.Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("expected public config platform_fee 2000, got %s", cfgResp.Data.PlatformFee)
	}

	// 3. Buat pesanan publik 10 box @ 25.000 = Subtotal 250.000
	// PPN 11% = 27.500, Biaya Layanan = 2.000, Total = 279.500
	wCreate := performJSON(env.router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    "Pelanggan Uji PPN & Biaya Layanan",
		"customer_contact": "081299990001",
		"delivery_type":    "pickup",
		"event_date":       "2026-11-20",
		"items": []map[string]any{
			{"product_id": env.snackBoxID, "qty": 10},
		},
	}, map[string]string{"Authorization": "Bearer " + env.customerToken})
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("create public order with PPN & fee: expected 201, got %d (%s)", wCreate.Code, wCreate.Body.String())
	}
	var created struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	if err := json.Unmarshal(wCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created order: %v", err)
	}

	// 4. Lacak pesanan publik: verifikasi subtotal, tax_amount, platform_fee, total
	wTrack := performJSON(env.router, http.MethodGet, "/api/v1/public/orders/"+created.Data.Code, nil, nil)
	if wTrack.Code != http.StatusOK {
		t.Fatalf("track order: expected 200, got %d (%s)", wTrack.Code, wTrack.Body.String())
	}
	var trackResp struct {
		Data orders.PublicTrackResponse `json:"data"`
	}
	if err := json.Unmarshal(wTrack.Body.Bytes(), &trackResp); err != nil {
		t.Fatalf("unmarshal track response: %v", err)
	}
	expectedSubtotal := decimal.NewFromInt(250_000)
	expectedTax := decimal.NewFromInt(27_500)
	expectedFee := decimal.NewFromInt(2_000)
	expectedTotal := decimal.NewFromInt(279_500)

	if trackResp.Data.Subtotal == nil || !trackResp.Data.Subtotal.Equal(expectedSubtotal) {
		t.Fatalf("expected subtotal %s, got %v", expectedSubtotal, trackResp.Data.Subtotal)
	}
	if trackResp.Data.TaxAmount == nil || !trackResp.Data.TaxAmount.Equal(expectedTax) {
		t.Fatalf("expected tax amount %s, got %v", expectedTax, trackResp.Data.TaxAmount)
	}
	if trackResp.Data.PlatformFee == nil || !trackResp.Data.PlatformFee.Equal(expectedFee) {
		t.Fatalf("expected platform fee %s, got %v", expectedFee, trackResp.Data.PlatformFee)
	}
	if !trackResp.Data.Total.Equal(expectedTotal) {
		t.Fatalf("expected total %s, got %s", expectedTotal, trackResp.Data.Total)
	}
}

func TestRecalibrateWorkload_ManagerAndOwner(t *testing.T) {
	env := setupIsolatedTestEnv(t)

	// 1. Staff mencoba kalibrasi -> harus 403 Forbidden
	wStaff := performJSON(env.router, http.MethodPost, "/api/v1/capacity/recalibrate", map[string]any{
		"new_minutes": 102,
	}, map[string]string{"Authorization": "Bearer " + env.staffToken})
	if wStaff.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for staff, got %d", wStaff.Code)
	}

	// 2. Manager mengkalibrasi durasi kerja produk (S6) -> 200 OK
	wMgr := performJSON(env.router, http.MethodPost, "/api/v1/capacity/recalibrate", map[string]any{
		"new_minutes": 102,
	}, map[string]string{"Authorization": "Bearer " + env.managerToken})
	if wMgr.Code != http.StatusOK {
		t.Fatalf("expected 200 for manager, got %d (%s)", wMgr.Code, wMgr.Body.String())
	}
	var res struct {
		Data struct {
			Message    string          `json:"message"`
			NewMinutes decimal.Decimal `json:"new_minutes"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wMgr.Body.Bytes(), &res)
	if !res.Data.NewMinutes.Equal(decimal.NewFromInt(102)) {
		t.Fatalf("expected new_minutes 102, got %s", res.Data.NewMinutes)
	}
}


