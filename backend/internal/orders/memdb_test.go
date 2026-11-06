package orders_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
)

// memDB adalah implementasi in-memory dari db.Conn (dan pgx.Tx) khusus untuk
// pengujian integrasi alur pesanan secara terisolasi tanpa menyentuh database
// produksi. Mendukung penguncian baris day_locks (SELECT ... FOR UPDATE) yang
// sesungguhnya antar goroutine menggunakan sync.Mutex per tanggal.
type memDB struct {
	mu      sync.Mutex
	idSeq   uint64
	dayMu   map[string]*sync.Mutex
	settings map[string]string

	resources   []memResource
	products    map[string]memProduct
	loads       map[string]map[string]decimal.Decimal // productID -> resourceCode -> load
	recipeCosts map[string]decimal.Decimal            // productID -> ingredient cost per unit

	orders      []memOrder
	orderItems  []memOrderItem
	evaluations []memEval
	rateLimits  map[string]int
	auditLogs   int
}

type memResource struct {
	ID               string
	Code             string
	Name             string
	Unit             string
	Count            int
	AmountPerUnitDay decimal.Decimal
	Efficiency       decimal.Decimal
}

type memProduct struct {
	ID            string
	Name          string
	Category      string
	Price         decimal.Decimal
	PackagingCost decimal.Decimal
	OverheadCost  decimal.Decimal
	IsActive      bool
}

type memOrder struct {
	ID                   string
	Code                 string
	Source               string
	CustomerName         string
	CustomerContact      string
	DeliveryType         string
	Address              string
	Notes                string
	EventDate            time.Time
	ProductionDate       time.Time
	Status               string
	Total                decimal.Decimal
	DPRequired           decimal.Decimal
	DPDeadline           *time.Time
	Paid                 decimal.Decimal
	Recommendation       string
	RecommendationReason string
	CounterOfferJSON     []byte
	SnapshotJSON         []byte
	CreatedBy            string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type memOrderItem struct {
	ID        string
	OrderID   string
	ProductID string
	Qty       int
	UnitPrice decimal.Decimal
	UnitCost  decimal.Decimal
}

type memEval struct {
	OrderID   string
	Gate      string
	Value     decimal.Decimal
	Threshold decimal.Decimal
	Passed    bool
	Detail    string
}

func newMemDB() *memDB {
	return &memDB{
		dayMu:       make(map[string]*sync.Mutex),
		settings:    make(map[string]string),
		products:    make(map[string]memProduct),
		loads:       make(map[string]map[string]decimal.Decimal),
		recipeCosts: make(map[string]decimal.Decimal),
		rateLimits:  make(map[string]int),
	}
}

func (m *memDB) nextUUID() string {
	n := atomic.AddUint64(&m.idSeq, 1)
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", n)
}

func (m *memDB) dayMutex(dateKey string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lk, ok := m.dayMu[dateKey]
	if !ok {
		lk = &sync.Mutex{}
		m.dayMu[dateKey] = lk
	}
	return lk
}

func (m *memDB) Begin(ctx context.Context) (pgx.Tx, error) {
	return &memTx{db: m}, nil
}

func (m *memDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx := &memTx{db: m}
	return tx.Exec(ctx, sql, args...)
}

func (m *memDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx := &memTx{db: m}
	return tx.Query(ctx, sql, args...)
}

func (m *memDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx := &memTx{db: m}
	return tx.QueryRow(ctx, sql, args...)
}

type memTx struct {
	pgx.Tx
	db        *memDB
	heldLocks []*sync.Mutex
	heldDays  map[string]bool
	closed    bool
}

func (tx *memTx) unlockAll() {
	if tx.closed {
		return
	}
	tx.closed = true
	for i := len(tx.heldLocks) - 1; i >= 0; i-- {
		tx.heldLocks[i].Unlock()
	}
	tx.heldLocks = nil
}

func (tx *memTx) Commit(ctx context.Context) error {
	tx.unlockAll()
	return nil
}

func (tx *memTx) Rollback(ctx context.Context) error {
	tx.unlockAll()
	return nil
}

func (tx *memTx) Begin(ctx context.Context) (pgx.Tx, error) { return tx, nil }
func (tx *memTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (tx *memTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults { return nil }
func (tx *memTx) LargeObjects() pgx.LargeObjects                               { return pgx.LargeObjects{} }
func (tx *memTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (tx *memTx) Conn() *pgx.Conn { return nil }

func normSQL(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func (tx *memTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q := normSQL(sql)
	switch {
	case strings.HasPrefix(q, "delete from rate_limits"):
		return pgconn.NewCommandTag("DELETE 0"), nil

	case strings.HasPrefix(q, "insert into day_locks"):
		if keys, ok := args[0].([]string); ok {
			for _, k := range keys {
				_ = tx.db.dayMutex(k[:10])
			}
		} else {
			dayKey := fmt.Sprint(args[0])[:10]
			_ = tx.db.dayMutex(dayKey)
		}
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	case strings.HasPrefix(q, "insert into order_evaluations"):
		tx.db.mu.Lock()
		tx.db.evaluations = append(tx.db.evaluations, memEval{
			OrderID:   fmt.Sprint(args[0]),
			Gate:      fmt.Sprint(args[1]),
			Value:     args[2].(decimal.Decimal),
			Threshold: args[3].(decimal.Decimal),
			Passed:    args[4].(bool),
			Detail:    fmt.Sprint(args[5]),
		})
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	case strings.HasPrefix(q, "insert into audit_logs"):
		tx.db.mu.Lock()
		tx.db.auditLogs++
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	default:
		return pgconn.NewCommandTag("OK"), nil
	}
}

func (tx *memTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	q := normSQL(sql)

	switch {
	case strings.Contains(q, "from settings where key = 'demo_today'"):
		tx.db.mu.Lock()
		raw, ok := tx.db.settings["demo_today"]
		tx.db.mu.Unlock()
		if !ok {
			return &memRow{err: pgx.ErrNoRows}
		}
		trimmed := strings.Trim(raw, `"`)
		return &memRow{vals: []any{&trimmed}}

	case strings.HasPrefix(q, "insert into rate_limits"):
		key := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		tx.db.rateLimits[key]++
		cnt := tx.db.rateLimits[key]
		tx.db.mu.Unlock()
		return &memRow{vals: []any{cnt}}

	case strings.Contains(q, "from day_locks where day = $1::date for update"):
		dayKey := fmt.Sprint(args[0])[:10]
		if tx.heldDays == nil {
			tx.heldDays = make(map[string]bool)
		}
		if !tx.heldDays[dayKey] {
			lk := tx.db.dayMutex(dayKey)
			lk.Lock()
			tx.heldDays[dayKey] = true
			tx.heldLocks = append(tx.heldLocks, lk)
		}
		t, _ := time.Parse("2006-01-02", dayKey)
		return &memRow{vals: []any{t}}

	case strings.Contains(q, "select exists(select 1 from orders where code = $1)"):
		code := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		exists := false
		for _, o := range tx.db.orders {
			if o.Code == code {
				exists = true
				break
			}
		}
		tx.db.mu.Unlock()
		return &memRow{vals: []any{exists}}

	case strings.HasPrefix(q, "insert into orders"):
		now := time.Now().UTC()
		evDate, _ := time.Parse("2006-01-02", fmt.Sprint(args[7])[:10])
		prDate, _ := time.Parse("2006-01-02", fmt.Sprint(args[8])[:10])
		var dpDeadline *time.Time
		if args[12] != nil {
			if dl, ok := args[12].(*time.Time); ok {
				dpDeadline = dl
			} else if dlVal, ok := args[12].(time.Time); ok {
				dpDeadline = &dlVal
			}
		}
		var counterBytes []byte
		if args[15] != nil {
			counterBytes = []byte(fmt.Sprint(args[15]))
		}
		var snapBytes []byte
		if args[16] != nil {
			snapBytes = []byte(fmt.Sprint(args[16]))
		}
		id := tx.db.nextUUID()
		ord := memOrder{
			ID:                   id,
			Code:                 fmt.Sprint(args[0]),
			Source:               fmt.Sprint(args[1]),
			CustomerName:         fmt.Sprint(args[2]),
			CustomerContact:      fmt.Sprint(args[3]),
			DeliveryType:         fmt.Sprint(args[4]),
			EventDate:            evDate,
			ProductionDate:       prDate,
			Status:               fmt.Sprint(args[9]),
			Total:                args[10].(decimal.Decimal),
			DPRequired:           args[11].(decimal.Decimal),
			DPDeadline:           dpDeadline,
			Paid:                 decimal.Zero,
			Recommendation:       fmt.Sprint(args[13]),
			RecommendationReason: fmt.Sprint(args[14]),
			CounterOfferJSON:     counterBytes,
			SnapshotJSON:         snapBytes,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		tx.db.mu.Lock()
		tx.db.orders = append(tx.db.orders, ord)
		tx.db.mu.Unlock()
		return &memRow{vals: []any{id, now, now}}

	case strings.HasPrefix(q, "insert into order_items"):
		id := tx.db.nextUUID()
		it := memOrderItem{
			ID:        id,
			OrderID:   fmt.Sprint(args[0]),
			ProductID: fmt.Sprint(args[1]),
			Qty:       args[2].(int),
			UnitPrice: args[3].(decimal.Decimal),
			UnitCost:  args[4].(decimal.Decimal),
		}
		tx.db.mu.Lock()
		tx.db.orderItems = append(tx.db.orderItems, it)
		tx.db.mu.Unlock()
		return &memRow{vals: []any{id}}

	case strings.Contains(q, "from orders where code = $1"):
		code := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.Code == code {
				rec := o.Recommendation
				reason := o.RecommendationReason
				return &memRow{vals: []any{
					o.ID, o.Code, o.CustomerName, o.DeliveryType, o.EventDate, o.Status,
					o.Total, o.DPRequired, o.DPDeadline, o.Paid,
					&rec, &reason, o.CounterOfferJSON,
					o.CreatedAt, o.UpdatedAt,
				}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.Contains(q, "select count(*) from order_evaluations where order_id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		cnt := 0
		for _, ev := range tx.db.evaluations {
			if ev.OrderID == oid {
				cnt++
			}
		}
		tx.db.mu.Unlock()
		return &memRow{vals: []any{cnt}}

	default:
		return &memRow{err: fmt.Errorf("memTx.QueryRow unsupported SQL: %s", q)}
	}
}

func (tx *memTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	q := normSQL(sql)
	if strings.Contains(q, "from day_locks where day = any($1::date[]) order by day for update") {
		keys := args[0].([]string)
		if tx.heldDays == nil {
			tx.heldDays = make(map[string]bool)
		}
		var rows [][]any
		for _, k := range keys {
			dayKey := k[:10]
			if !tx.heldDays[dayKey] {
				lk := tx.db.dayMutex(dayKey)
				lk.Lock()
				tx.heldDays[dayKey] = true
				tx.heldLocks = append(tx.heldLocks, lk)
			}
			t, _ := time.Parse("2006-01-02", dayKey)
			rows = append(rows, []any{t})
		}
		return &memRows{rows: rows}, nil
	}

	tx.db.mu.Lock()
	defer tx.db.mu.Unlock()

	switch {
	case strings.Contains(q, "select key, value::text from settings"):
		var rows [][]any
		for k, v := range tx.db.settings {
			rows = append(rows, []any{k, v})
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "as shopping_cost from orders o"):
		onePlusWaste := args[1].(decimal.Decimal)
		var rows [][]any
		for _, o := range tx.db.orders {
			if o.Status != "accepted" && o.Status != "confirmed" && o.Status != "in_production" && o.Status != "ready" && o.Status != "delivering" {
				continue
			}
			shopCost := decimal.Zero
			for _, it := range tx.db.orderItems {
				if it.OrderID != o.ID {
					continue
				}
				p := tx.db.products[it.ProductID]
				ingCost := tx.db.recipeCosts[it.ProductID]
				unitShop := ingCost.Mul(onePlusWaste).Add(p.PackagingCost)
				shopCost = shopCost.Add(unitShop.Mul(decimal.NewFromInt(int64(it.Qty))))
			}
			rows = append(rows, []any{o.ID, o.Status, o.EventDate, o.Total, o.DPRequired, o.DPDeadline, o.Paid, shopCost})
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from products where is_active = true"):
		var rows [][]any
		for _, p := range tx.db.products {
			if p.IsActive {
				rows = append(rows, []any{p.ID, p.Name, p.Category, p.Price})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from products where id = any"):
		ids := args[0].([]string)
		var rows [][]any
		for _, id := range ids {
			if p, ok := tx.db.products[id]; ok {
				rows = append(rows, []any{p.ID, p.Name, p.Category, p.Price, p.PackagingCost, p.OverheadCost, p.IsActive})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from product_loads pl join resources r"):
		ids := args[0].([]string)
		var rows [][]any
		for _, pid := range ids {
			for code, ld := range tx.db.loads[pid] {
				rows = append(rows, []any{pid, code, ld})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from recipe_items ri join ingredients i"):
		ids := args[0].([]string)
		var rows [][]any
		for _, pid := range ids {
			if cost, ok := tx.db.recipeCosts[pid]; ok {
				rows = append(rows, []any{pid, decimal.NewFromInt(1), cost, decimal.NewFromInt(1)})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "select code, count, amount_per_unit_day, efficiency from resources"):
		var rows [][]any
		for _, r := range tx.db.resources {
			rows = append(rows, []any{r.Code, r.Count, r.AmountPerUnitDay, r.Efficiency})
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "group by o.production_date, r.code"):
		fromKey := fmt.Sprint(args[0])[:10]
		toKey := fmt.Sprint(args[1])[:10]
		totals := make(map[string]map[string]decimal.Decimal)
		for _, o := range tx.db.orders {
			if o.Status != "accepted" && o.Status != "confirmed" && o.Status != "in_production" {
				continue
			}
			pk := o.ProductionDate.Format("2006-01-02")
			if pk < fromKey || pk > toKey {
				continue
			}
			if totals[pk] == nil {
				totals[pk] = make(map[string]decimal.Decimal)
			}
			for _, it := range tx.db.orderItems {
				if it.OrderID != o.ID {
					continue
				}
				qtyDec := decimal.NewFromInt(int64(it.Qty))
				for code, ld := range tx.db.loads[it.ProductID] {
					totals[pk][code] = totals[pk][code].Add(qtyDec.Mul(ld))
				}
			}
		}
		var rows [][]any
		for dayStr, byCode := range totals {
			for code, sum := range byCode {
				rows = append(rows, []any{dayStr, code, sum})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from cash_entries"):
		return &memRows{}, nil

	case strings.Contains(q, "from order_items oi join products p"):
		oid := fmt.Sprint(args[0])
		var rows [][]any
		for _, it := range tx.db.orderItems {
			if it.OrderID == oid {
				p := tx.db.products[it.ProductID]
				rows = append(rows, []any{it.ProductID, p.Name, it.Qty, it.UnitPrice})
			}
		}
		return &memRows{rows: rows}, nil

	default:
		return nil, fmt.Errorf("memTx.Query unsupported SQL: %s", q)
	}
}

type memRow struct {
	vals []any
	err  error
}

func (r *memRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScan(r.vals, dest)
}

type memRows struct {
	pgx.Rows
	rows [][]any
	idx  int
}

func (r *memRows) Close()                                       {}
func (r *memRows) Err() error                                   { return nil }
func (r *memRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("SELECT") }
func (r *memRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *memRows) Values() ([]any, error)                       { return r.rows[r.idx-1], nil }
func (r *memRows) RawValues() [][]byte                          { return nil }
func (r *memRows) Conn() *pgx.Conn                              { return nil }

func (r *memRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}

func (r *memRows) Scan(dest ...any) error {
	return assignScan(r.rows[r.idx-1], dest)
}

func assignScan(src []any, dest []any) error {
	if len(src) != len(dest) {
		return fmt.Errorf("scan count mismatch: have %d, want %d", len(src), len(dest))
	}
	for i, s := range src {
		if s == nil {
			continue
		}
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr || dv.IsNil() {
			return fmt.Errorf("dest %d not a non-nil pointer", i)
		}
		sv := reflect.ValueOf(s)
		if sv.Type().AssignableTo(dv.Elem().Type()) {
			dv.Elem().Set(sv)
			continue
		}
		if sv.Type().ConvertibleTo(dv.Elem().Type()) {
			dv.Elem().Set(sv.Convert(dv.Elem().Type()))
			continue
		}
		return fmt.Errorf("cannot assign %T to %T at index %d", s, dest[i], i)
	}
	return nil
}
