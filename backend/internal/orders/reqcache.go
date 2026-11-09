package orders

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
)

type reqCacheKeyType struct{}

var reqCacheKey = reqCacheKeyType{}

// orderReqCache menyimpan hasil pembacaan master & snapshot per-request (hanya untuk koneksi non-transaksi)
// guna menghilangkan duplikasi query berulang pada endpoint agregat seperti dashboard & KPI.
type orderReqCache struct {
	mu sync.Mutex

	expiredDone bool

	hasSettings bool
	settings    SettingsParams
	hasToday    bool
	today       time.Time

	hasCaps bool
	caps    map[string]decimal.Decimal

	hasCosting bool
	costing    []ProductCostingDTO

	hasCashEvents bool
	cashEventsKey string
	cashEvents    []engine.CashEvent

	hasFullOrders bool
	fullOrders    []OrderDetail
}

func withOrderReqCache(ctx context.Context) (context.Context, *orderReqCache) {
	if existing, ok := ctx.Value(reqCacheKey).(*orderReqCache); ok && existing != nil {
		return ctx, existing
	}
	rc := &orderReqCache{}
	return context.WithValue(ctx, reqCacheKey, rc), rc
}

func getOrderReqCache(ctx context.Context, q db.DBTX) *orderReqCache {
	if q == nil {
		return nil
	}
	// Jangan gunakan cache lintas-request di dalam transaksi tulis (pgx.Tx)
	if _, isTx := q.(pgx.Tx); isTx {
		return nil
	}
	if rc, ok := ctx.Value(reqCacheKey).(*orderReqCache); ok {
		return rc
	}
	return nil
}

func isConcurrentPool(q db.DBTX) bool {
	_, ok := q.(*pgxpool.Pool)
	return ok
}

// todayQ mengambil tanggal hari ini (mendukung demo_today) dengan memanfaatkan cache request bila tersedia.
func todayQ(ctx context.Context, q db.DBTX) time.Time {
	if rc := getOrderReqCache(ctx, q); rc != nil {
		rc.mu.Lock()
		if rc.hasToday {
			t := rc.today
			rc.mu.Unlock()
			return t
		}
		rc.mu.Unlock()
		// Memuat settings sekaligus mengisi rc.today dari key 'demo_today'
		if _, err := LoadSettingsParams(ctx, q); err == nil {
			rc.mu.Lock()
			if rc.hasToday {
				t := rc.today
				rc.mu.Unlock()
				return t
			}
			rc.mu.Unlock()
		}
	}
	return db.TodayQ(ctx, q)
}

