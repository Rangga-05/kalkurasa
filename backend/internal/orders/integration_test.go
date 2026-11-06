//go:build integration

package orders_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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

// TestTC08_PostgresIntegration20Rounds menguji TC-08 langsung terhadap database
// PostgreSQL sungguhan (DATABASE_URL) dengan konfigurasi pool produksi
// (db.Connect: QueryExecModeSimpleProtocol dan DB_MAX_CONNS).
//
// Skenario:
//   - Fixture memakai awalan unik "ittest-" dan tanggal acara mulai 60 hari ke depan.
//   - Beban oven untuk 1 pesanan produk "ittest-" disetel sebesar 60% kapasitas harian oven
//     (0,60 <= capacity_limit 0,90), sehingga satu hari hanya muat tepat SATU pesanan
//     (dua pesanan = 120% > 90%).
//   - Diulangi sebanyak 20 putaran pada 20 tanggal berbeda (H+60 s.d. H+79).
//   - Pada setiap putaran, 2 goroutine menembak POST /api/v1/public/orders secara serentak:
//     harus tepat 1 berstatus "accepted" dan 1 berstatus "pending_decision", serta
//     total beban tereservasi pada hari itu tidak melebihi batas terima (capacity_limit).
//   - Di akhir test, seluruh baris ber-awalan "ittest-" dibersihkan dan diverifikasi 0.
func TestTC08_PostgresIntegration20Rounds(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Gunakan konstruktor pool produksi persis seperti cmd/api (SimpleProtocol + DB_MAX_CONNS)
	pool, err := db.New(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer pool.Close()

	// Periksa isolation level transaksi default yang dipakai db.WithTx / pool.Begin
	var isoLevel string
	if err := pool.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isoLevel); err != nil {
		t.Fatalf("SHOW transaction_isolation: %v", err)
	}
	t.Logf("Postgres transaction_isolation=%s, DB_MAX_CONNS=%d", isoLevel, cfg.DBMaxConns)

	runSuffix, err := orders.GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	runPrefix := "ittest-" + strings.ToLower(runSuffix[:6]) + "-"

	today := db.Today(ctx, pool)
	startDate := today.AddDate(0, 0, 60)
	const rounds = 20
	endLockCleanupDate := startDate.AddDate(0, 0, rounds+20)

	var createdResourceIDs []string
	var productID string

	cleanup := func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanCancel()

		// 1. Hapus audit_logs untuk pesanan ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM audit_logs
			WHERE entity = 'orders'
			  AND entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
		`)
		// 2. Hapus orders (cascade ke order_items dan order_evaluations)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM orders WHERE customer_name LIKE 'ittest-%'`)
		// 3. Hapus product_loads dan products ber-awalan ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM product_loads
			WHERE product_id IN (SELECT id FROM products WHERE name LIKE 'ittest-%')
		`)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM products WHERE name LIKE 'ittest-%'`)
		// 4. Hapus cash_entries ber-awalan ittest-
		_, _ = pool.Exec(cleanCtx, `DELETE FROM cash_entries WHERE note LIKE 'ittest-%'`)
		// 5. Hapus resources hanya bila dibuat oleh test ini
		for _, rid := range createdResourceIDs {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM resources WHERE id = $1::uuid`, rid)
		}
		// 6. Hapus day_locks untuk tanggal-tanggal tes (H+60 s.d. H+100) & rate_limits uji
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM day_locks
			WHERE day >= $1::date AND day <= $2::date
		`, engine.DateKey(startDate), engine.DateKey(endLockCleanupDate))
		_, _ = pool.Exec(cleanCtx, `DELETE FROM rate_limits WHERE key LIKE 'order:198.51.100.%'`)
	}

	// Bersihkan sisa bila pernah ada, dan pastikan dibersihkan kembali di akhir
	cleanup()
	defer cleanup()

	// Pastikan sumber daya oven dan labor tersedia (baca yang ada tanpa mengubah data demo)
	var ovenID, laborID string
	var ovenCount int
	var ovenAmt, ovenEff decimal.Decimal

	err = pool.QueryRow(ctx, `
		SELECT id::text, count, amount_per_unit_day, efficiency
		FROM resources WHERE code = 'oven'
	`).Scan(&ovenID, &ovenCount, &ovenAmt, &ovenEff)
	if err != nil {
		// Jika tabel resources masih kosong, buat sementara dan tandai untuk dihapus saat cleanup
		if err := pool.QueryRow(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ('oven', 'ittest-oven', 'oven_hour', 2, 8.0000, 0.8500)
			RETURNING id::text, count, amount_per_unit_day, efficiency
		`).Scan(&ovenID, &ovenCount, &ovenAmt, &ovenEff); err != nil {
			t.Fatalf("ensure oven resource: %v", err)
		}
		createdResourceIDs = append(createdResourceIDs, ovenID)
	}

	err = pool.QueryRow(ctx, `SELECT id::text FROM resources WHERE code = 'labor'`).Scan(&laborID)
	if err != nil {
		if err := pool.QueryRow(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ('labor', 'ittest-labor', 'minute', 4, 420.0000, 0.8500)
			RETURNING id::text
		`).Scan(&laborID); err != nil {
			t.Fatalf("ensure labor resource: %v", err)
		}
		createdResourceIDs = append(createdResourceIDs, laborID)
	}

	params, err := orders.LoadSettingsParams(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSettingsParams: %v", err)
	}

	ovenCap := engine.Capacity(engine.Resource{
		Code:             "oven",
		Count:            ovenCount,
		AmountPerUnitDay: ovenAmt,
		Efficiency:       ovenEff,
	})
	// Beban 1 pesanan = 60% dari kapasitas harian oven (0,60 <= capacity_limit 0,90; 2 pesanan = 1,20 > 0,90)
	singleOrderOvenLoad := ovenCap.Mul(decimal.RequireFromString("0.60")).Round(4)
	maxAllowedOvenLoad := ovenCap.Mul(params.EvalParams.CapacityLimit)

	// Buat produk uji "ittest-..." dengan harga yang menjamin G2 & G3 lolos serta di bawah ambang big_order
	productName := runPrefix + "paket-slot-tunggal"
	err = pool.QueryRow(ctx, `
		INSERT INTO products (name, category, price, packaging_cost, overhead_cost, is_active)
		VALUES ($1, 'ittest-cat', 50000.0000, 5000.0000, 2000.0000, true)
		RETURNING id::text
	`, productName).Scan(&productID)
	if err != nil {
		t.Fatalf("insert test product: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO product_loads (product_id, resource_id, load) VALUES
		($1::uuid, $2::uuid, $3),
		($1::uuid, $4::uuid, 5.0000)
	`, productID, ovenID, singleOrderOvenLoad, laborID)
	if err != nil {
		t.Fatalf("insert test product_loads: %v", err)
	}

	// Tambahkan entri kas uji (note = 'ittest-...') pada today agar biaya tetap harian
	// selama horizon 60..100 hari ke depan tertutup dan G3 (kas) lolos.
	_, err = pool.Exec(ctx, `
		INSERT INTO cash_entries (entry_date, amount, note)
		VALUES ($1::date, 100000000.0000, $2)
	`, engine.DateKey(today), runPrefix+"cash-buffer")
	if err != nil {
		t.Fatalf("insert test cash_entry: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := server.New(cfg, pool)

	type roundCallResult struct {
		StatusCode int
		Body       string
		Data       orders.PublicOrderSummary
	}

	for r := 0; r < rounds; r++ {
		// Gunakan kelipatan 2 hari agar tawaran balik H+1 dari putaran r tidak mengisi tanggal putaran r+1
		eventDate := startDate.AddDate(0, 0, r*2)
		dateStr := engine.DateKey(eventDate)

		var results [2]roundCallResult
		startBarrier := make(chan struct{})
		var wg sync.WaitGroup

		for g := 0; g < 2; g++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-startBarrier

				custName := fmt.Sprintf("%scust-r%02d-g%d", runPrefix, r+1, idx+1)
				ip := fmt.Sprintf("198.51.100.%d", r*2+idx+1)
				w := performJSON(router, http.MethodPost, "/api/v1/public/orders", map[string]any{
					"customer_name":    custName,
					"customer_contact": "081200001111",
					"delivery_type":    "pickup",
					"notes":            runPrefix + "note",
					"event_date":       dateStr,
					"items": []map[string]any{
						{"product_id": productID, "qty": 1},
					},
				}, map[string]string{
					"X-Forwarded-For": ip,
				})

				var parsed struct {
					Data orders.PublicOrderSummary `json:"data"`
				}
				_ = json.Unmarshal(w.Body.Bytes(), &parsed)
				results[idx] = roundCallResult{
					StatusCode: w.Code,
					Body:       w.Body.String(),
					Data:       parsed.Data,
				}
			}(g)
		}

		close(startBarrier)
		wg.Wait()

		accepted := 0
		pending := 0
		for g := 0; g < 2; g++ {
			res := results[g]
			if res.StatusCode != http.StatusCreated {
				t.Fatalf("putaran %d goroutine %d: diharapkan 201, dapat %d (%s)", r+1, g+1, res.StatusCode, res.Body)
			}
			switch res.Data.Status {
			case orders.StatusAccepted:
				accepted++
			case orders.StatusPendingDecision:
				pending++
			default:
				t.Fatalf("putaran %d goroutine %d: status tidak terduga %q", r+1, g+1, res.Data.Status)
			}
		}

		if accepted != 1 || pending != 1 {
			t.Fatalf("putaran %d (%s): diharapkan tepat 1 accepted dan 1 pending_decision, dapat accepted=%d pending=%d",
				r+1, dateStr, accepted, pending)
		}

		// Verifikasi di Postgres: total beban oven yang direservasi pada tanggal itu <= batas terima
		var reservedOvenLoad decimal.Decimal
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(oi.qty * pl.load), 0)
			FROM orders o
			JOIN order_items oi ON oi.order_id = o.id
			JOIN product_loads pl ON pl.product_id = oi.product_id
			WHERE o.production_date = $1::date
			  AND o.status IN ('accepted', 'confirmed', 'in_production')
			  AND pl.resource_id = $2::uuid
		`, dateStr, ovenID).Scan(&reservedOvenLoad)
		if err != nil {
			t.Fatalf("putaran %d cek beban tereservasi: %v", r+1, err)
		}
		if reservedOvenLoad.GreaterThan(maxAllowedOvenLoad) {
			t.Fatalf("putaran %d (%s): overbooking! beban tereservasi %s > batas terima %s",
				r+1, dateStr, reservedOvenLoad, maxAllowedOvenLoad)
		}
		if !reservedOvenLoad.Equal(singleOrderOvenLoad) {
			t.Fatalf("putaran %d (%s): diharapkan beban tereservasi %s, dapat %s",
				r+1, dateStr, singleOrderOvenLoad, reservedOvenLoad)
		}
	}

	// Sebelum cleanup: verifikasi baris audit_logs benar-benar tertulis untuk pesanan publik (actor_id IS NULL)
	var auditCountBefore int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE entity = 'orders'
		  AND actor_id IS NULL
		  AND entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
	`).Scan(&auditCountBefore); err != nil {
		t.Fatalf("count audit_logs before cleanup: %v", err)
	}
	if auditCountBefore != rounds*2 {
		t.Fatalf("diharapkan %d baris audit_logs (actor_id IS NULL) sebelum cleanup, dapat %d", rounds*2, auditCountBefore)
	}

	var sampleAuditID, sampleAction, sampleEntity, sampleEntityID, sampleDetail string
	var sampleActorID *string
	var sampleCreatedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT id::text, actor_id::text, action, entity, entity_id::text, detail::text, created_at
		FROM audit_logs
		WHERE entity = 'orders'
		  AND entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
		ORDER BY created_at DESC LIMIT 1
	`).Scan(&sampleAuditID, &sampleActorID, &sampleAction, &sampleEntity, &sampleEntityID, &sampleDetail, &sampleCreatedAt); err != nil {
		t.Fatalf("read sample audit_log row: %v", err)
	}
	t.Logf("Bukti baris audit_logs sebelum cleanup (total=%d): id=%s actor_id=%v action=%s entity=%s entity_id=%s detail=%s created_at=%s",
		auditCountBefore, sampleAuditID, sampleActorID, sampleAction, sampleEntity, sampleEntityID, sampleDetail, sampleCreatedAt.Format(time.RFC3339))

	// Jalankan pembersihan eksplisit lalu pastikan jumlah baris ber-awalan "ittest-" di DB adalah 0
	cleanup()

	var remProducts, remLoads, remOrders, remItems, remEvals, remCash, remAudit, remDayLocks int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE name LIKE 'ittest-%'`).Scan(&remProducts); err != nil {
		t.Fatalf("count remaining products: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM product_loads pl
		JOIN products p ON p.id = pl.product_id
		WHERE p.name LIKE 'ittest-%'
	`).Scan(&remLoads); err != nil {
		t.Fatalf("count remaining product_loads: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE customer_name LIKE 'ittest-%'`).Scan(&remOrders); err != nil {
		t.Fatalf("count remaining orders: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remItems); err != nil {
		t.Fatalf("count remaining order_items: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_evaluations oe
		JOIN orders o ON o.id = oe.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remEvals); err != nil {
		t.Fatalf("count remaining order_evaluations: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM cash_entries WHERE note LIKE 'ittest-%'`).Scan(&remCash); err != nil {
		t.Fatalf("count remaining cash_entries: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE detail::text LIKE '%ittest-%'
		   OR entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
	`).Scan(&remAudit); err != nil {
		t.Fatalf("count remaining audit_logs: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM day_locks
		WHERE day >= $1::date AND day <= $2::date
	`, engine.DateKey(startDate), engine.DateKey(endLockCleanupDate)).Scan(&remDayLocks); err != nil {
		t.Fatalf("count remaining day_locks: %v", err)
	}

	t.Logf("Sisa baris ittest- setelah cleanup: products=%d, product_loads=%d, orders=%d, order_items=%d, order_evaluations=%d, cash_entries=%d, audit_logs=%d, day_locks=%d",
		remProducts, remLoads, remOrders, remItems, remEvals, remCash, remAudit, remDayLocks)

	if remProducts != 0 || remLoads != 0 || remOrders != 0 || remItems != 0 || remEvals != 0 || remCash != 0 || remAudit != 0 || remDayLocks != 0 {
		t.Fatalf("masih ada baris ittest- yang belum bersih: products=%d, loads=%d, orders=%d, items=%d, evals=%d, cash=%d, audit=%d, day_locks=%d",
			remProducts, remLoads, remOrders, remItems, remEvals, remCash, remAudit, remDayLocks)
	}
}

// TestDecision_PostgresConcurrentAccept menguji keamanan keputusan serentak
// terhadap database PostgreSQL sungguhan (DATABASE_URL) dengan pool produksi:
//   - Dua pesanan berstatus pending_decision dibuat pada tanggal acara yang sama
//     (masing-masing memakai 60% kapasitas harian oven, sehingga satu hari hanya muat satu pesanan).
//   - Dua goroutine memanggil POST /api/v1/orders/:id/decision dengan {"action":"accept"}
//     (tanpa override_reason) secara serentak.
//   - Hasil yang benar: tepat satu berhasil "accepted" (200 OK), dan yang lain ditolak
//     dengan 422 Unprocessable Entity (memerlukan override_reason karena evaluasi ulang
//     di dalam transaksi day_locks melihat kapasitas sudah terisi).
//   - Di akhir test, seluruh baris ber-awalan "ittest-" dibersihkan dan diverifikasi 0.
func TestDecision_PostgresConcurrentAccept(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer pool.Close()

	runSuffix, err := orders.GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	runPrefix := "ittest-dec-" + strings.ToLower(runSuffix[:6]) + "-"

	today := db.Today(ctx, pool)
	startDate := today.AddDate(0, 0, 90)
	const rounds = 5
	endLockCleanupDate := startDate.AddDate(0, 0, rounds*2+20)

	var createdResourceIDs []string
	var productID, ownerUserID string

	cleanup := func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanCancel()

		// 1. Hapus audit_logs terkait pesanan atau pengguna ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM audit_logs
			WHERE detail::text LIKE '%ittest-%'
			   OR entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
			   OR actor_id IN (SELECT id FROM users WHERE email LIKE 'ittest-%')
		`)
		// 2. Hapus orders (cascade ke order_items, order_evaluations, payments, tasks)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM orders WHERE customer_name LIKE 'ittest-%'`)
		// 3. Hapus product_loads dan products ber-awalan ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM product_loads
			WHERE product_id IN (SELECT id FROM products WHERE name LIKE 'ittest-%')
		`)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM products WHERE name LIKE 'ittest-%'`)
		// 4. Hapus cash_entries dan users ber-awalan ittest-
		_, _ = pool.Exec(cleanCtx, `DELETE FROM cash_entries WHERE note LIKE 'ittest-%'`)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM users WHERE email LIKE 'ittest-%'`)
		// 5. Hapus resources hanya bila dibuat sementara oleh test ini
		for _, rid := range createdResourceIDs {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM resources WHERE id = $1::uuid`, rid)
		}
		// 6. Hapus day_locks pada rentang tanggal tes & rate_limits uji
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM day_locks
			WHERE day >= $1::date AND day <= $2::date
		`, engine.DateKey(startDate), engine.DateKey(endLockCleanupDate))
		_, _ = pool.Exec(cleanCtx, `DELETE FROM rate_limits WHERE key LIKE 'order:198.51.100.%'`)
	}

	cleanup()
	defer cleanup()

	// Pastikan sumber daya oven dan labor tersedia
	var ovenID, laborID string
	var ovenCount int
	var ovenAmt, ovenEff decimal.Decimal

	err = pool.QueryRow(ctx, `
		SELECT id::text, count, amount_per_unit_day, efficiency
		FROM resources WHERE code = 'oven'
	`).Scan(&ovenID, &ovenCount, &ovenAmt, &ovenEff)
	if err != nil {
		if err := pool.QueryRow(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ('oven', 'ittest-oven', 'oven_hour', 2, 8.0000, 0.8500)
			RETURNING id::text, count, amount_per_unit_day, efficiency
		`).Scan(&ovenID, &ovenCount, &ovenAmt, &ovenEff); err != nil {
			t.Fatalf("ensure oven resource: %v", err)
		}
		createdResourceIDs = append(createdResourceIDs, ovenID)
	}

	err = pool.QueryRow(ctx, `SELECT id::text FROM resources WHERE code = 'labor'`).Scan(&laborID)
	if err != nil {
		if err := pool.QueryRow(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ('labor', 'ittest-labor', 'minute', 4, 420.0000, 0.8500)
			RETURNING id::text
		`).Scan(&laborID); err != nil {
			t.Fatalf("ensure labor resource: %v", err)
		}
		createdResourceIDs = append(createdResourceIDs, laborID)
	}

	// Buat pengguna owner uji dengan awalan ittest-
	ownerEmail := runPrefix + "owner@demo.test"
	err = pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, password_hash, is_active)
		VALUES ($1, $2, 'owner', 'ittest-hash-not-used-for-password-login', true)
		RETURNING id::text
	`, ownerEmail, runPrefix+"Owner").Scan(&ownerUserID)
	if err != nil {
		t.Fatalf("insert test owner user: %v", err)
	}

	ownerToken, _, err := auth.IssueToken([]byte(cfg.JWTSecret), ownerUserID, auth.RoleOwner, time.Now(), auth.TokenTTL)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	params, err := orders.LoadSettingsParams(ctx, pool)
	if err != nil {
		t.Fatalf("LoadSettingsParams: %v", err)
	}

	ovenCap := engine.Capacity(engine.Resource{
		Code:             "oven",
		Count:            ovenCount,
		AmountPerUnitDay: ovenAmt,
		Efficiency:       ovenEff,
	})
	// Beban 1 pesanan = 60% dari kapasitas harian oven (0,60 <= capacity_limit 0,90; 2 pesanan = 1,20 > 0,90)
	singleOrderOvenLoad := ovenCap.Mul(decimal.RequireFromString("0.60")).Round(4)
	maxAllowedOvenLoad := ovenCap.Mul(params.EvalParams.CapacityLimit)

	// Produk uji punya harga di atas big_order_total (Rp 2.500.000 >= Rp 2.000.000) sehingga
	// saat dibuat lewat POST /public/orders, semua gerbang G1..G4 lolos (recommendation="accept"),
	// tetapi status awalnya adalah "pending_decision" (kapasitas belum direservasi).
	bigPrice := params.BigOrderTotal.Add(decimal.NewFromInt(500_000))
	productName := runPrefix + "paket-keputusan-serentak"
	err = pool.QueryRow(ctx, `
		INSERT INTO products (name, category, price, packaging_cost, overhead_cost, is_active)
		VALUES ($1, 'ittest-cat', $2, 50000.0000, 20000.0000, true)
		RETURNING id::text
	`, productName, bigPrice).Scan(&productID)
	if err != nil {
		t.Fatalf("insert test product: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO product_loads (product_id, resource_id, load) VALUES
		($1::uuid, $2::uuid, $3),
		($1::uuid, $4::uuid, 10.0000)
	`, productID, ovenID, singleOrderOvenLoad, laborID)
	if err != nil {
		t.Fatalf("insert test product_loads: %v", err)
	}

	// Tambahkan entri kas uji pada today agar G3 (kas) lolos pada horizon H+90..H+110
	_, err = pool.Exec(ctx, `
		INSERT INTO cash_entries (entry_date, amount, note)
		VALUES ($1::date, 200000000.0000, $2)
	`, engine.DateKey(today), runPrefix+"cash-buffer")
	if err != nil {
		t.Fatalf("insert test cash_entry: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := server.New(cfg, pool)

	type decisionCallResult struct {
		StatusCode int
		Body       string
		Data       orders.OrderDetail
	}

	for r := 0; r < rounds; r++ {
		eventDate := startDate.AddDate(0, 0, r*2)
		dateStr := engine.DateKey(eventDate)

		// Buat 2 pesanan pending_decision untuk tanggal yang sama
		var orderIDs [2]string
		for i := 0; i < 2; i++ {
			custName := fmt.Sprintf("%scust-r%02d-p%d", runPrefix, r+1, i+1)
			ip := fmt.Sprintf("198.51.100.%d", 100+r*2+i)
			wCreate := performJSON(router, http.MethodPost, "/api/v1/public/orders", map[string]any{
				"customer_name":    custName,
				"customer_contact": "081200009999",
				"delivery_type":    "pickup",
				"notes":            runPrefix + "note",
				"event_date":       dateStr,
				"items": []map[string]any{
					{"product_id": productID, "qty": 1},
				},
			}, map[string]string{
				"X-Forwarded-For": ip,
			})
			if wCreate.Code != http.StatusCreated {
				t.Fatalf("putaran %d buat pesanan %d: diharapkan 201, dapat %d (%s)", r+1, i+1, wCreate.Code, wCreate.Body.String())
			}
			var created struct {
				Data orders.PublicOrderSummary `json:"data"`
			}
			_ = json.Unmarshal(wCreate.Body.Bytes(), &created)
			if created.Data.Status != orders.StatusPendingDecision {
				t.Fatalf("putaran %d pesanan %d: diharapkan status awal pending_decision, dapat %q", r+1, i+1, created.Data.Status)
			}
			orderIDs[i] = created.Data.ID
		}

		// Dua goroutine memanggil POST /api/v1/orders/:id/decision (action="accept" tanpa override_reason) secara serentak
		var results [2]decisionCallResult
		startBarrier := make(chan struct{})
		var wg sync.WaitGroup

		for g := 0; g < 2; g++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-startBarrier

				wDec := performJSON(router, http.MethodPost, "/api/v1/orders/"+orderIDs[idx]+"/decision", map[string]any{
					"action": "accept",
				}, map[string]string{
					"Authorization": "Bearer " + ownerToken,
				})
				var parsed struct {
					Data orders.OrderDetail `json:"data"`
				}
				_ = json.Unmarshal(wDec.Body.Bytes(), &parsed)
				results[idx] = decisionCallResult{
					StatusCode: wDec.Code,
					Body:       wDec.Body.String(),
					Data:       parsed.Data,
				}
			}(g)
		}

		close(startBarrier)
		wg.Wait()

		acceptedCount := 0
		rejectedNeedOverrideCount := 0
		for g := 0; g < 2; g++ {
			res := results[g]
			switch res.StatusCode {
			case http.StatusOK:
				if res.Data.Status == orders.StatusAccepted {
					acceptedCount++
				} else {
					t.Fatalf("putaran %d goroutine %d: 200 OK tetapi status %q", r+1, g+1, res.Data.Status)
				}
			case http.StatusUnprocessableEntity:
				// Ditolak karena evaluasi ulang melihat kapasitas sudah terisi sehingga memerlukan override_reason
				rejectedNeedOverrideCount++
			default:
				t.Fatalf("putaran %d goroutine %d: status HTTP tidak terduga %d (%s)", r+1, g+1, res.StatusCode, res.Body)
			}
		}

		if acceptedCount != 1 || rejectedNeedOverrideCount != 1 {
			t.Fatalf("putaran %d (%s): diharapkan tepat 1 berhasil accepted (200) dan 1 ditolak butuh override (422), dapat accepted=%d rejected422=%d",
				r+1, dateStr, acceptedCount, rejectedNeedOverrideCount)
		}

		// Verifikasi di Postgres: beban tereservasi pada tanggal tersebut tidak melebihi batas terima
		var reservedOvenLoad decimal.Decimal
		err := pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(oi.qty * pl.load), 0)
			FROM orders o
			JOIN order_items oi ON oi.order_id = o.id
			JOIN product_loads pl ON pl.product_id = oi.product_id
			WHERE o.production_date = $1::date
			  AND o.status IN ('accepted', 'confirmed', 'in_production')
			  AND pl.resource_id = $2::uuid
		`, dateStr, ovenID).Scan(&reservedOvenLoad)
		if err != nil {
			t.Fatalf("putaran %d cek beban tereservasi: %v", r+1, err)
		}
		if reservedOvenLoad.GreaterThan(maxAllowedOvenLoad) || !reservedOvenLoad.Equal(singleOrderOvenLoad) {
			t.Fatalf("putaran %d (%s): beban tereservasi tidak sesuai! dapat %s, diharapkan %s (batas %s)",
				r+1, dateStr, reservedOvenLoad, singleOrderOvenLoad, maxAllowedOvenLoad)
		}
	}

	// Bersihkan seluruh fixture "ittest-" dan hitung sisa baris di semua tabel terkait
	cleanup()

	var remUsers, remProducts, remLoads, remOrders, remItems, remEvals, remPayments, remTasks, remCash, remAudit, remDayLocks int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email LIKE 'ittest-%'`).Scan(&remUsers)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE name LIKE 'ittest-%'`).Scan(&remProducts)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM product_loads pl
		JOIN products p ON p.id = pl.product_id
		WHERE p.name LIKE 'ittest-%'
	`).Scan(&remLoads)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE customer_name LIKE 'ittest-%'`).Scan(&remOrders)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remItems)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_evaluations oe
		JOIN orders o ON o.id = oe.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remEvals)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments p
		JOIN orders o ON o.id = p.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remPayments)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tasks tk
		JOIN orders o ON o.id = tk.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remTasks)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM cash_entries WHERE note LIKE 'ittest-%'`).Scan(&remCash)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE detail::text LIKE '%ittest-%'
		   OR entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
		   OR actor_id IN (SELECT id FROM users WHERE email LIKE 'ittest-%')
	`).Scan(&remAudit)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM day_locks
		WHERE day >= $1::date AND day <= $2::date
	`, engine.DateKey(startDate), engine.DateKey(endLockCleanupDate)).Scan(&remDayLocks)

	t.Logf("Sisa baris ittest- setelah cleanup (TestDecision): users=%d, products=%d, product_loads=%d, orders=%d, order_items=%d, order_evaluations=%d, payments=%d, tasks=%d, cash_entries=%d, audit_logs=%d, day_locks=%d",
		remUsers, remProducts, remLoads, remOrders, remItems, remEvals, remPayments, remTasks, remCash, remAudit, remDayLocks)

	if remUsers != 0 || remProducts != 0 || remLoads != 0 || remOrders != 0 || remItems != 0 || remEvals != 0 || remPayments != 0 || remTasks != 0 || remCash != 0 || remAudit != 0 || remDayLocks != 0 {
		t.Fatalf("masih ada baris ittest- yang belum bersih: users=%d, products=%d, loads=%d, orders=%d, items=%d, evals=%d, payments=%d, tasks=%d, cash=%d, audit=%d, day_locks=%d",
			remUsers, remProducts, remLoads, remOrders, remItems, remEvals, remPayments, remTasks, remCash, remAudit, remDayLocks)
	}
}

// TestLifecycle_PostgresAU1FullFlow menguji alur lengkap AU-1 langsung terhadap
// database PostgreSQL asli (DATABASE_URL) melalui router Gin:
//  1. Pembuatan pesanan publik (Order 1 mengisi slot di dateA; Order 2 di dateA masuk pending_decision dengan counter_offer ke dateB).
//  2. Antrian keputusan (GET /api/v1/decision-queue) beserta pratinjau dampak.
//  3. Keputusan tawaran balik (POST /api/v1/orders/:id/decision action=counter) dan respons pelanggan (POST /api/v1/public/orders/:code/counter-response accept=true -> accepted di dateB).
//  4. Kalender kapasitas (GET /api/v1/capacity/calendar) dan rincian kapasitas harian (GET /api/v1/capacity/day/:date).
//  5. Pembayaran DP (POST /api/v1/orders/:id/payments) -> otomatis confirmed + 3 tugas (produce, shop, deliver).
//  6. Transisi status (POST /api/v1/orders/:id/transition) sampai completed:
//     confirmed -> in_production -> ready (potong stok bahan & catat stock_movements) -> delivering -> pelunasan -> completed.
//  7. Pesanan kedua (Order 1 di dateA) untuk pembatalan (POST /api/v1/orders/:id/cancel): verifikasi cancellation_exposure, tugas dibatalkan, dan kapasitas dateA dilepas.
//  8. Pesanan ketiga (Order 3 di dateC) untuk kedaluwarsa DP malas (dp_deadline lewat -> expired, kapasitas dateC dilepas).
//  9. Cleanup seluruh baris ber-awalan "ittest-" dengan defer dan verifikasi sisa baris = 0 di semua tabel yang disentuh.
func TestLifecycle_PostgresAU1FullFlow(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer pool.Close()

	runSuffix, err := orders.GenerateCode()
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	runPrefix := "ittest-lc-" + strings.ToLower(runSuffix[:6]) + "-"

	today := db.Today(ctx, pool)
	startDate := today.AddDate(0, 0, 120)
	dateA := startDate
	dateB := startDate.AddDate(0, 0, 1)
	dateC := startDate.AddDate(0, 0, 3)
	dateAStr := engine.DateKey(dateA)
	dateBStr := engine.DateKey(dateB)
	dateCStr := engine.DateKey(dateC)
	endLockCleanupDate := startDate.AddDate(0, 0, 25)

	var createdResourceIDs []string

	cleanup := func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanCancel()

		// 1. Hapus stock_movements yang merujuk ke bahan atau pesanan ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM stock_movements
			WHERE ingredient_id IN (SELECT id FROM ingredients WHERE name LIKE 'ittest-%')
			   OR ref_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
		`)
		// 2. Hapus recipe_items yang merujuk ke produk atau bahan ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM recipe_items
			WHERE product_id IN (SELECT id FROM products WHERE name LIKE 'ittest-%')
			   OR ingredient_id IN (SELECT id FROM ingredients WHERE name LIKE 'ittest-%')
		`)
		// 3. Hapus audit_logs yang terkait entitas atau aktor ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM audit_logs
			WHERE detail::text LIKE '%ittest-%'
			   OR entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
			   OR actor_id IN (SELECT id FROM users WHERE email LIKE 'ittest-%')
		`)
		// 4. Hapus tasks dan payments (dan orders yang juga cascade ke order_items, order_evaluations, payments, tasks)
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM tasks
			WHERE order_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
		`)
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM payments
			WHERE order_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
		`)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM orders WHERE customer_name LIKE 'ittest-%'`)
		// 5. Hapus product_loads, products, dan ingredients ber-awalan ittest-
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM product_loads
			WHERE product_id IN (SELECT id FROM products WHERE name LIKE 'ittest-%')
		`)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM products WHERE name LIKE 'ittest-%'`)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM ingredients WHERE name LIKE 'ittest-%'`)
		// 6. Hapus cash_entries dan users ber-awalan ittest-
		_, _ = pool.Exec(cleanCtx, `DELETE FROM cash_entries WHERE note LIKE 'ittest-%'`)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM users WHERE email LIKE 'ittest-%'`)
		// 7. Hapus resources hanya bila dibuat sementara oleh test ini
		for _, rid := range createdResourceIDs {
			_, _ = pool.Exec(cleanCtx, `DELETE FROM resources WHERE id = $1::uuid`, rid)
		}
		// 8. Hapus day_locks pada rentang tanggal tes & rate_limits uji
		_, _ = pool.Exec(cleanCtx, `
			DELETE FROM day_locks
			WHERE day >= $1::date AND day <= $2::date
		`, engine.DateKey(startDate), engine.DateKey(endLockCleanupDate))
		_, _ = pool.Exec(cleanCtx, `DELETE FROM rate_limits WHERE key LIKE 'order:198.51.100.%'`)
	}

	cleanup()
	defer cleanup()

	// Pastikan sumber daya oven dan labor tersedia
	var ovenID, laborID string
	var ovenCount int
	var ovenAmt, ovenEff decimal.Decimal

	err = pool.QueryRow(ctx, `
		SELECT id::text, count, amount_per_unit_day, efficiency
		FROM resources WHERE code = 'oven'
	`).Scan(&ovenID, &ovenCount, &ovenAmt, &ovenEff)
	if err != nil {
		if err := pool.QueryRow(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ('oven', 'ittest-oven', 'oven_hour', 2, 8.0000, 0.8500)
			RETURNING id::text, count, amount_per_unit_day, efficiency
		`).Scan(&ovenID, &ovenCount, &ovenAmt, &ovenEff); err != nil {
			t.Fatalf("ensure oven resource: %v", err)
		}
		createdResourceIDs = append(createdResourceIDs, ovenID)
	}

	err = pool.QueryRow(ctx, `SELECT id::text FROM resources WHERE code = 'labor'`).Scan(&laborID)
	if err != nil {
		if err := pool.QueryRow(ctx, `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ('labor', 'ittest-labor', 'minute', 4, 420.0000, 0.8500)
			RETURNING id::text
		`).Scan(&laborID); err != nil {
			t.Fatalf("ensure labor resource: %v", err)
		}
		createdResourceIDs = append(createdResourceIDs, laborID)
	}

	// Buat 3 pengguna uji (owner, manager, staff) dengan awalan ittest-
	var ownerID, managerID, staffID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, password_hash, is_active)
		VALUES ($1, $2, 'owner', 'ittest-hash', true) RETURNING id::text
	`, runPrefix+"owner@demo.test", runPrefix+"Owner").Scan(&ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, password_hash, is_active)
		VALUES ($1, $2, 'manager', 'ittest-hash', true) RETURNING id::text
	`, runPrefix+"manager@demo.test", runPrefix+"Manager").Scan(&managerID); err != nil {
		t.Fatalf("insert manager: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (email, name, role, password_hash, is_active)
		VALUES ($1, $2, 'staff', 'ittest-hash', true) RETURNING id::text
	`, runPrefix+"staff@demo.test", runPrefix+"Staff").Scan(&staffID); err != nil {
		t.Fatalf("insert staff: %v", err)
	}

	ownerToken, _, err := auth.IssueToken([]byte(cfg.JWTSecret), ownerID, auth.RoleOwner, time.Now(), auth.TokenTTL)
	if err != nil {
		t.Fatalf("IssueToken owner: %v", err)
	}
	managerToken, _, err := auth.IssueToken([]byte(cfg.JWTSecret), managerID, auth.RoleManager, time.Now(), auth.TokenTTL)
	if err != nil {
		t.Fatalf("IssueToken manager: %v", err)
	}
	staffToken, _, err := auth.IssueToken([]byte(cfg.JWTSecret), staffID, auth.RoleStaff, time.Now(), auth.TokenTTL)
	if err != nil {
		t.Fatalf("IssueToken staff: %v", err)
	}

	// Buat bahan uji ittest- (stok awal 20 kg, harga 12.000/kg -> sak 25 kg @ 300.000)
	var ingredientID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO ingredients (name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active)
		VALUES ($1, 'kg', 'sak', 25.0000, 300000.0000, 20.0000, 5.0000, 'loose', true)
		RETURNING id::text
	`, runPrefix+"tepung").Scan(&ingredientID); err != nil {
		t.Fatalf("insert ingredient: %v", err)
	}

	ovenCap := engine.Capacity(engine.Resource{
		Code:             "oven",
		Count:            ovenCount,
		AmountPerUnitDay: ovenAmt,
		Efficiency:       ovenEff,
	})
	// Pesanan uji berjumlah 25 unit; setel beban oven per unit sehingga 25 unit = 60% kapasitas harian oven
	perUnitOvenLoad := ovenCap.Mul(decimal.RequireFromString("0.60")).Div(decimal.NewFromInt(25)).Round(4)

	// Buat produk uji ittest- (harga 25.000, kemasan 2.000, overhead 1.175)
	var productID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO products (name, category, price, packaging_cost, overhead_cost, is_active)
		VALUES ($1, 'ittest-cat', 25000.0000, 2000.0000, 1175.0000, true)
		RETURNING id::text
	`, runPrefix+"snack-box").Scan(&productID); err != nil {
		t.Fatalf("insert product: %v", err)
	}

	// Resep: 0,12 kg tepung per unit (biaya bahan 1.440/unit -> unit cost rendah, margin sangat sehat)
	if _, err := pool.Exec(ctx, `
		INSERT INTO recipe_items (product_id, ingredient_id, qty_per_unit)
		VALUES ($1::uuid, $2::uuid, 0.1200)
	`, productID, ingredientID); err != nil {
		t.Fatalf("insert recipe_item: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO product_loads (product_id, resource_id, load) VALUES
		($1::uuid, $2::uuid, $3),
		($1::uuid, $4::uuid, 12.0000)
	`, productID, ovenID, perUnitOvenLoad, laborID); err != nil {
		t.Fatalf("insert product_loads: %v", err)
	}

	// Tambahkan buffer kas pada today agar biaya tetap harian s.d. H+145 tertutup dan G3 lolos
	if _, err := pool.Exec(ctx, `
		INSERT INTO cash_entries (entry_date, amount, note)
		VALUES ($1::date, 200000000.0000, $2)
	`, engine.DateKey(today), runPrefix+"cash-buffer"); err != nil {
		t.Fatalf("insert cash_entry: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := server.New(cfg, pool)

	// =========================================================================
	// 1. Pembuatan Pesanan Publik (Order 1 & Order 2 pada dateA)
	// =========================================================================
	// Order 1 (25 unit pada dateA -> utilisasi 60% <= 90%, langsung accepted)
	wOrd1 := performJSON(router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    runPrefix + "pelanggan-1",
		"customer_contact": "081200000001",
		"delivery_type":    "pickup",
		"notes":            runPrefix + "order-1",
		"event_date":       dateAStr,
		"items": []map[string]any{
			{"product_id": productID, "qty": 25},
		},
	}, map[string]string{"X-Forwarded-For": "198.51.100.201"})
	if wOrd1.Code != http.StatusCreated {
		t.Fatalf("Order 1 POST /public/orders: diharapkan 201, dapat %d (%s)", wOrd1.Code, wOrd1.Body.String())
	}
	var ord1Resp struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wOrd1.Body.Bytes(), &ord1Resp)
	if ord1Resp.Data.Status != orders.StatusAccepted {
		t.Fatalf("Order 1: diharapkan status accepted, dapat %q", ord1Resp.Data.Status)
	}
	order1ID := ord1Resp.Data.ID

	// Order 2 (25 unit pada dateA -> total utilisasi 120% > 90% -> G1 gagal -> pending_decision + counter_offer ke dateB)
	wOrd2 := performJSON(router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    runPrefix + "pelanggan-2",
		"customer_contact": "081200000002",
		"delivery_type":    "delivery",
		"address":          "Jl. Merdeka No. 10",
		"notes":            runPrefix + "order-2",
		"event_date":       dateAStr,
		"items": []map[string]any{
			{"product_id": productID, "qty": 25},
		},
	}, map[string]string{"X-Forwarded-For": "198.51.100.202"})
	if wOrd2.Code != http.StatusCreated {
		t.Fatalf("Order 2 POST /public/orders: diharapkan 201, dapat %d (%s)", wOrd2.Code, wOrd2.Body.String())
	}
	var ord2Resp struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wOrd2.Body.Bytes(), &ord2Resp)
	if ord2Resp.Data.Status != orders.StatusPendingDecision {
		t.Fatalf("Order 2: diharapkan status pending_decision, dapat %q", ord2Resp.Data.Status)
	}
	if ord2Resp.Data.CounterOffer == nil || ord2Resp.Data.CounterOffer.Date != dateBStr {
		t.Fatalf("Order 2: diharapkan counter_offer.date=%s, dapat %+v", dateBStr, ord2Resp.Data.CounterOffer)
	}
	order2ID := ord2Resp.Data.ID
	order2Code := ord2Resp.Data.Code

	// =========================================================================
	// 2. Antrian Keputusan (GET /api/v1/decision-queue)
	// =========================================================================
	wQueue := performJSON(router, http.MethodGet, "/api/v1/decision-queue", nil, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wQueue.Code != http.StatusOK {
		t.Fatalf("GET /decision-queue: diharapkan 200, dapat %d (%s)", wQueue.Code, wQueue.Body.String())
	}
	var queueResp struct {
		Data []orders.DecisionQueueItem `json:"data"`
	}
	_ = json.Unmarshal(wQueue.Body.Bytes(), &queueResp)
	var foundInQueue *orders.DecisionQueueItem
	for i := range queueResp.Data {
		if queueResp.Data[i].ID == order2ID {
			foundInQueue = &queueResp.Data[i]
			break
		}
	}
	if foundInQueue == nil {
		t.Fatalf("Order 2 (%s) tidak ditemukan di GET /decision-queue", order2ID)
	}
	if !foundInQueue.CanDecide || foundInQueue.Recommendation != "accept_with_conditions" {
		t.Fatalf("DecisionQueueItem tidak sesuai: can_decide=%v recommendation=%q", foundInQueue.CanDecide, foundInQueue.Recommendation)
	}
	if foundInQueue.Evaluation.Impact.UtilizationAfter.LessThanOrEqual(foundInQueue.Evaluation.Impact.UtilizationBefore) {
		t.Fatalf("Pratinjau dampak utilisasi tidak valid: before=%s after=%s",
			foundInQueue.Evaluation.Impact.UtilizationBefore, foundInQueue.Evaluation.Impact.UtilizationAfter)
	}

	// =========================================================================
	// 3. Keputusan Tawaran Balik & Respons Pelanggan
	// =========================================================================
	wDec := performJSON(router, http.MethodPost, "/api/v1/orders/"+order2ID+"/decision", map[string]any{
		"action": "counter",
	}, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wDec.Code != http.StatusOK {
		t.Fatalf("POST /orders/:id/decision (counter): diharapkan 200, dapat %d (%s)", wDec.Code, wDec.Body.String())
	}
	var decResp struct {
		Data orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wDec.Body.Bytes(), &decResp)
	if decResp.Data.Status != orders.StatusCountered {
		t.Fatalf("setelah decision=counter: diharapkan status countered, dapat %q", decResp.Data.Status)
	}

	// Pelanggan menyetujui tawaran balik lewat POST /api/v1/public/orders/:code/counter-response
	wCounterResp := performJSON(router, http.MethodPost, "/api/v1/public/orders/"+order2Code+"/counter-response", map[string]any{
		"accept": true,
	}, nil)
	if wCounterResp.Code != http.StatusOK {
		t.Fatalf("POST /public/orders/:code/counter-response: diharapkan 200, dapat %d (%s)", wCounterResp.Code, wCounterResp.Body.String())
	}
	var counterSummary struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wCounterResp.Body.Bytes(), &counterSummary)
	if counterSummary.Data.Status != orders.StatusAccepted || counterSummary.Data.EventDate != dateBStr {
		t.Fatalf("setelah counter-response accept=true: diharapkan status=accepted & event_date=%s, dapat status=%q event_date=%s",
			dateBStr, counterSummary.Data.Status, counterSummary.Data.EventDate)
	}

	// =========================================================================
	// 4. Pembayaran DP & Pembuatan 3 Tugas Otomatis (produce, shop, deliver)
	// =========================================================================
	dpAmount := counterSummary.Data.DPRequired
	totalAmount := counterSummary.Data.Total
	wPayDP := performJSON(router, http.MethodPost, "/api/v1/orders/"+order2ID+"/payments", map[string]any{
		"amount": dpAmount.StringFixed(2),
		"method": "transfer",
		"note":   runPrefix + "dp-payment",
	}, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wPayDP.Code != http.StatusCreated {
		t.Fatalf("POST /orders/:id/payments (DP): diharapkan 201, dapat %d (%s)", wPayDP.Code, wPayDP.Body.String())
	}
	var payDPResp struct {
		Data orders.PaymentResult `json:"data"`
	}
	_ = json.Unmarshal(wPayDP.Body.Bytes(), &payDPResp)
	if payDPResp.Data.Status != orders.StatusConfirmed {
		t.Fatalf("setelah bayar DP: diharapkan status confirmed, dapat %q", payDPResp.Data.Status)
	}
	if len(payDPResp.Data.TasksCreated) != 3 {
		t.Fatalf("setelah bayar DP: diharapkan 3 tugas dibuat (produce, shop, deliver), dapat %d", len(payDPResp.Data.TasksCreated))
	}

	// =========================================================================
	// 5. Kalender Kapasitas & Rincian Kapasitas Hari (dateB)
	// =========================================================================
	monthStr := dateB.Format("2006-01")
	wCal := performJSON(router, http.MethodGet, "/api/v1/capacity/calendar?month="+monthStr, nil, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wCal.Code != http.StatusOK {
		t.Fatalf("GET /capacity/calendar: diharapkan 200, dapat %d (%s)", wCal.Code, wCal.Body.String())
	}
	var calResp struct {
		Data []orders.CapacityCalendarDay `json:"data"`
	}
	_ = json.Unmarshal(wCal.Body.Bytes(), &calResp)
	var foundCalB *orders.CapacityCalendarDay
	for i := range calResp.Data {
		if calResp.Data[i].Date == dateBStr {
			foundCalB = &calResp.Data[i]
			break
		}
	}
	if foundCalB == nil || foundCalB.Utilization.LessThan(decimal.RequireFromString("0.59")) {
		t.Fatalf("kalender kapasitas untuk %s tidak sesuai: %+v", dateBStr, foundCalB)
	}

	wDayB := performJSON(router, http.MethodGet, "/api/v1/capacity/day/"+dateBStr, nil, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wDayB.Code != http.StatusOK {
		t.Fatalf("GET /capacity/day/%s: diharapkan 200, dapat %d (%s)", dateBStr, wDayB.Code, wDayB.Body.String())
	}
	var dayBResp struct {
		Data orders.CapacityDayDetail `json:"data"`
	}
	_ = json.Unmarshal(wDayB.Body.Bytes(), &dayBResp)
	if len(dayBResp.Data.Orders) != 1 || dayBResp.Data.Orders[0].ID != order2ID {
		t.Fatalf("GET /capacity/day/%s: diharapkan 1 pesanan (%s), dapat %+v", dateBStr, order2ID, dayBResp.Data.Orders)
	}

	// =========================================================================
	// 6. Transisi Status sampai Completed + Pengurangan Stok Bahan & Stock Movements
	// =========================================================================
	// 6a. confirmed -> in_production (oleh manager)
	wTr1 := performJSON(router, http.MethodPost, "/api/v1/orders/"+order2ID+"/transition", map[string]any{
		"to_status": orders.StatusInProduction,
	}, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wTr1.Code != http.StatusOK {
		t.Fatalf("transition confirmed -> in_production: diharapkan 200, dapat %d (%s)", wTr1.Code, wTr1.Body.String())
	}

	// 6b. in_production -> ready (oleh staff, dengan actual_minutes = 290)
	wTr2 := performJSON(router, http.MethodPost, "/api/v1/orders/"+order2ID+"/transition", map[string]any{
		"to_status":      orders.StatusReady,
		"actual_minutes": 290,
	}, map[string]string{
		"Authorization": "Bearer " + staffToken,
	})
	if wTr2.Code != http.StatusOK {
		t.Fatalf("transition in_production -> ready: diharapkan 200, dapat %d (%s)", wTr2.Code, wTr2.Body.String())
	}

	// Verifikasi stok bahan berkurang di Postgres (20.0000 - 25*0.12 = 17.0000) dan stock_movements tercatat
	var stockAfter decimal.Decimal
	if err := pool.QueryRow(ctx, `SELECT stock FROM ingredients WHERE id = $1::uuid`, ingredientID).Scan(&stockAfter); err != nil {
		t.Fatalf("cek stok bahan setelah ready: %v", err)
	}
	expectedStock := decimal.RequireFromString("17.0000")
	if !stockAfter.Equal(expectedStock) {
		t.Fatalf("stok bahan setelah in_production->ready: diharapkan %s, dapat %s", expectedStock, stockAfter)
	}

	var movCount int
	var movDelta decimal.Decimal
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(delta), 0)
		FROM stock_movements
		WHERE ingredient_id = $1::uuid AND ref_type = 'order' AND ref_id = $2::uuid AND reason = 'production'
	`, ingredientID, order2ID).Scan(&movCount, &movDelta); err != nil {
		t.Fatalf("cek stock_movements: %v", err)
	}
	if movCount != 1 || !movDelta.Equal(decimal.RequireFromString("-3.0000")) {
		t.Fatalf("stock_movements tidak sesuai: count=%d delta=%s (diharapkan count=1 delta=-3.0000)", movCount, movDelta)
	}

	// 6c. ready -> delivering (oleh staff)
	wTr3 := performJSON(router, http.MethodPost, "/api/v1/orders/"+order2ID+"/transition", map[string]any{
		"to_status": orders.StatusDelivering,
	}, map[string]string{
		"Authorization": "Bearer " + staffToken,
	})
	if wTr3.Code != http.StatusOK {
		t.Fatalf("transition ready -> delivering: diharapkan 200, dapat %d (%s)", wTr3.Code, wTr3.Body.String())
	}

	// 6d. Pelunasan sisa pembayaran sebelum completed
	remainingPay := totalAmount.Sub(dpAmount)
	wPayFinal := performJSON(router, http.MethodPost, "/api/v1/orders/"+order2ID+"/payments", map[string]any{
		"amount": remainingPay.StringFixed(2),
		"method": "cash",
		"note":   runPrefix + "pelunasan",
	}, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wPayFinal.Code != http.StatusCreated {
		t.Fatalf("POST /orders/:id/payments (pelunasan): diharapkan 201, dapat %d (%s)", wPayFinal.Code, wPayFinal.Body.String())
	}

	// 6e. delivering -> completed (oleh manager)
	wTr4 := performJSON(router, http.MethodPost, "/api/v1/orders/"+order2ID+"/transition", map[string]any{
		"to_status": orders.StatusCompleted,
	}, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wTr4.Code != http.StatusOK {
		t.Fatalf("transition delivering -> completed: diharapkan 200, dapat %d (%s)", wTr4.Code, wTr4.Body.String())
	}
	var tr4Resp struct {
		Data orders.OrderDetail `json:"data"`
	}
	_ = json.Unmarshal(wTr4.Body.Bytes(), &tr4Resp)
	if tr4Resp.Data.Status != orders.StatusCompleted {
		t.Fatalf("setelah delivering -> completed: diharapkan status completed, dapat %q", tr4Resp.Data.Status)
	}

	// =========================================================================
	// 7. Pembatalan Pesanan Kedua (Order 1 di dateA) -> Eksposur & Pelepasan Kapasitas
	// =========================================================================
	// Catat pembayaran DP sebagian (Rp 50.000) pada Order 1 agar eksposur pembatalan > 0 dapat diverifikasi
	wPayOrd1 := performJSON(router, http.MethodPost, "/api/v1/orders/"+order1ID+"/payments", map[string]any{
		"amount": "50000.00",
		"method": "transfer",
		"note":   runPrefix + "dp-sebagian-ord1",
	}, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wPayOrd1.Code != http.StatusCreated {
		t.Fatalf("bayar sebagian Order 1: diharapkan 201, dapat %d (%s)", wPayOrd1.Code, wPayOrd1.Body.String())
	}

	wCancel := performJSON(router, http.MethodPost, "/api/v1/orders/"+order1ID+"/cancel", map[string]any{
		"reason":         "Pelanggan membatalkan acara",
		"dp_disposition": "refunded",
		"note":           runPrefix + "cancel-note",
	}, map[string]string{
		"Authorization": "Bearer " + ownerToken,
	})
	if wCancel.Code != http.StatusOK {
		t.Fatalf("POST /orders/:id/cancel: diharapkan 200, dapat %d (%s)", wCancel.Code, wCancel.Body.String())
	}
	var cancelResp struct {
		Data orders.CancelOrderResult `json:"data"`
	}
	_ = json.Unmarshal(wCancel.Body.Bytes(), &cancelResp)
	if cancelResp.Data.Status != orders.StatusCancelled {
		t.Fatalf("setelah cancel: diharapkan status cancelled, dapat %q", cancelResp.Data.Status)
	}
	expectedExposure := engine.CancellationExposure(cancelResp.Data.ShoppingCost, cancelResp.Data.DPReceived).Round(2)
	if !cancelResp.Data.CancellationExposure.Equal(expectedExposure) {
		t.Fatalf("cancellation_exposure tidak cocok: dapat %s, diharapkan %s", cancelResp.Data.CancellationExposure, expectedExposure)
	}

	// Pastikan kapasitas di dateA sudah dilepas (0 pesanan aktif)
	wDayA := performJSON(router, http.MethodGet, "/api/v1/capacity/day/"+dateAStr, nil, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wDayA.Code != http.StatusOK {
		t.Fatalf("GET /capacity/day/%s: diharapkan 200, dapat %d", dateAStr, wDayA.Code)
	}
	var dayAResp struct {
		Data orders.CapacityDayDetail `json:"data"`
	}
	_ = json.Unmarshal(wDayA.Body.Bytes(), &dayAResp)
	if len(dayAResp.Data.Orders) != 0 || !dayAResp.Data.Utilization.IsZero() {
		t.Fatalf("setelah Order 1 dibatalkan, kapasitas %s harus 0: orders=%d util=%s",
			dateAStr, len(dayAResp.Data.Orders), dayAResp.Data.Utilization)
	}

	// =========================================================================
	// 8. Pesanan Ketiga (Order 3 di dateC) -> Kedaluwarsa DP Malas (Lazy Check)
	// =========================================================================
	wOrd3 := performJSON(router, http.MethodPost, "/api/v1/public/orders", map[string]any{
		"customer_name":    runPrefix + "pelanggan-3-expire",
		"customer_contact": "081200000003",
		"delivery_type":    "pickup",
		"notes":            runPrefix + "order-3",
		"event_date":       dateCStr,
		"items": []map[string]any{
			{"product_id": productID, "qty": 25},
		},
	}, map[string]string{"X-Forwarded-For": "198.51.100.203"})
	if wOrd3.Code != http.StatusCreated {
		t.Fatalf("Order 3 POST /public/orders: diharapkan 201, dapat %d (%s)", wOrd3.Code, wOrd3.Body.String())
	}
	var ord3Resp struct {
		Data orders.PublicOrderSummary `json:"data"`
	}
	_ = json.Unmarshal(wOrd3.Body.Bytes(), &ord3Resp)
	if ord3Resp.Data.Status != orders.StatusAccepted {
		t.Fatalf("Order 3: diharapkan status awal accepted, dapat %q", ord3Resp.Data.Status)
	}
	order3ID := ord3Resp.Data.ID

	// Majukan dp_deadline Order 3 ke masa lalu (2 jam yang lalu) di Postgres
	if _, err := pool.Exec(ctx, `
		UPDATE orders SET dp_deadline = NOW() - INTERVAL '2 hours' WHERE id = $1::uuid
	`, order3ID); err != nil {
		t.Fatalf("set dp_deadline lampau untuk Order 3: %v", err)
	}

	// Panggil GET /api/v1/capacity/day/:dateC -> memicu pengecekan malas ExpireOverdueOrders
	wDayC := performJSON(router, http.MethodGet, "/api/v1/capacity/day/"+dateCStr, nil, map[string]string{
		"Authorization": "Bearer " + managerToken,
	})
	if wDayC.Code != http.StatusOK {
		t.Fatalf("GET /capacity/day/%s: diharapkan 200, dapat %d (%s)", dateCStr, wDayC.Code, wDayC.Body.String())
	}
	var dayCResp struct {
		Data orders.CapacityDayDetail `json:"data"`
	}
	_ = json.Unmarshal(wDayC.Body.Bytes(), &dayCResp)
	if len(dayCResp.Data.Orders) != 0 || !dayCResp.Data.Utilization.IsZero() {
		t.Fatalf("setelah dp_deadline lewat, kapasitas %s harus dilepas (0): orders=%d util=%s",
			dateCStr, len(dayCResp.Data.Orders), dayCResp.Data.Utilization)
	}

	var ord3StatusAfter string
	if err := pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1::uuid`, order3ID).Scan(&ord3StatusAfter); err != nil {
		t.Fatalf("baca status Order 3: %v", err)
	}
	if ord3StatusAfter != orders.StatusExpired {
		t.Fatalf("Order 3 setelah lazy check: diharapkan status expired, dapat %q", ord3StatusAfter)
	}

	// =========================================================================
	// 9. Cleanup Eksplisit & Verifikasi Sisa Baris "ittest-" = 0 di Semua Tabel
	// =========================================================================
	cleanup()

	var (
		remUsers, remProducts, remLoads, remIngredients, remRecipes int
		remOrders, remItems, remEvals, remPayments, remTasks        int
		remMovements, remCash, remAudit, remDayLocks                int
	)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE email LIKE 'ittest-%'`).Scan(&remUsers)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE name LIKE 'ittest-%'`).Scan(&remProducts)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM product_loads pl
		JOIN products p ON p.id = pl.product_id
		WHERE p.name LIKE 'ittest-%'
	`).Scan(&remLoads)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM ingredients WHERE name LIKE 'ittest-%'`).Scan(&remIngredients)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM recipe_items ri
		LEFT JOIN products p ON p.id = ri.product_id
		LEFT JOIN ingredients i ON i.id = ri.ingredient_id
		WHERE p.name LIKE 'ittest-%' OR i.name LIKE 'ittest-%'
	`).Scan(&remRecipes)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE customer_name LIKE 'ittest-%'`).Scan(&remOrders)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remItems)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM order_evaluations oe
		JOIN orders o ON o.id = oe.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remEvals)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM payments p
		JOIN orders o ON o.id = p.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remPayments)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tasks tk
		JOIN orders o ON o.id = tk.order_id
		WHERE o.customer_name LIKE 'ittest-%'
	`).Scan(&remTasks)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM stock_movements sm
		LEFT JOIN ingredients i ON i.id = sm.ingredient_id
		WHERE i.name LIKE 'ittest-%'
	`).Scan(&remMovements)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM cash_entries WHERE note LIKE 'ittest-%'`).Scan(&remCash)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM audit_logs
		WHERE detail::text LIKE '%ittest-%'
		   OR entity_id IN (SELECT id FROM orders WHERE customer_name LIKE 'ittest-%')
		   OR actor_id IN (SELECT id FROM users WHERE email LIKE 'ittest-%')
	`).Scan(&remAudit)
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM day_locks
		WHERE day >= $1::date AND day <= $2::date
	`, engine.DateKey(startDate), engine.DateKey(endLockCleanupDate)).Scan(&remDayLocks)

	t.Logf("Sisa baris ittest- setelah cleanup (TestLifecycle): users=%d, products=%d, product_loads=%d, ingredients=%d, recipe_items=%d, orders=%d, order_items=%d, order_evaluations=%d, payments=%d, tasks=%d, stock_movements=%d, cash_entries=%d, audit_logs=%d, day_locks=%d",
		remUsers, remProducts, remLoads, remIngredients, remRecipes,
		remOrders, remItems, remEvals, remPayments, remTasks,
		remMovements, remCash, remAudit, remDayLocks)

	if remUsers != 0 || remProducts != 0 || remLoads != 0 || remIngredients != 0 || remRecipes != 0 ||
		remOrders != 0 || remItems != 0 || remEvals != 0 || remPayments != 0 || remTasks != 0 ||
		remMovements != 0 || remCash != 0 || remAudit != 0 || remDayLocks != 0 {
		t.Fatalf("masih ada baris ittest- yang belum bersih: users=%d, products=%d, loads=%d, ingredients=%d, recipes=%d, orders=%d, items=%d, evals=%d, payments=%d, tasks=%d, movements=%d, cash=%d, audit=%d, day_locks=%d",
			remUsers, remProducts, remLoads, remIngredients, remRecipes,
			remOrders, remItems, remEvals, remPayments, remTasks,
			remMovements, remCash, remAudit, remDayLocks)
	}
}

