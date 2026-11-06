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
