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
	mu       sync.Mutex
	idSeq    uint64
	dayMu    map[string]*sync.Mutex
	settings map[string]string

	resources         []memResource
	products          map[string]memProduct
	loads             map[string]map[string]decimal.Decimal // productID -> resourceCode -> load
	recipeCosts       map[string]decimal.Decimal            // productID -> ingredient cost per unit
	ingredients       map[string]decimal.Decimal            // ingredientID -> current stock
	ingredientDetails map[string]memIngredient              // ingredientID -> full metadata
	recipes           map[string]map[string]decimal.Decimal // productID -> ingredientID -> qty_per_unit

	orders         []memOrder
	orderItems     []memOrderItem
	evaluations    []memEval
	payments       []memPayment
	tasks          []memTask
	purchases      []memPurchase
	cashEntries    []memCashEntry
	stockMovements []memStockMovement
	rateLimits     map[string]int
	auditLogs      int
	auditEntries   []memAuditEntry
}

type memIngredient struct {
	ID          string
	Name        string
	UseUnit     string
	BuyUnit     string
	BuyUnitSize decimal.Decimal
	BuyPrice    decimal.Decimal
	MinStock    decimal.Decimal
	Rounding    string
	IsActive    bool
}

type memPurchase struct {
	ID           string
	IngredientID string
	QtyBuyUnits  decimal.Decimal
	UnitPrice    decimal.Decimal
	BoughtAt     time.Time
	BoughtBy     string
}

type memCashEntry struct {
	ID        string
	EntryDate time.Time
	Amount    decimal.Decimal
	Note      string
	CreatedBy string
}

type memAuditEntry struct {
	ID        string
	ActorID   string
	Action    string
	Entity    string
	EntityID  string
	Detail    string
	CreatedAt time.Time
}

type memPayment struct {
	ID         string
	OrderID    string
	Amount     decimal.Decimal
	Method     string
	PaidAt     time.Time
	Note       string
	RecordedBy string
}

type memTask struct {
	ID            string
	OrderID       string
	Type          string
	Title         string
	DueDate       string
	AssigneeID    string
	Status        string
	ActualMinutes *int
	CreatedAt     time.Time
}

type memStockMovement struct {
	ID           string
	IngredientID string
	Delta        decimal.Decimal
	Reason       string
	RefType      string
	RefID        string
	CreatedAt    time.Time
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
	CustomerID           string
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
	DecidedBy            string
	DecidedAt            *time.Time
	OverrideReason       string
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
		dayMu:             make(map[string]*sync.Mutex),
		settings:          make(map[string]string),
		products:          make(map[string]memProduct),
		loads:             make(map[string]map[string]decimal.Decimal),
		recipeCosts:       make(map[string]decimal.Decimal),
		ingredients:       make(map[string]decimal.Decimal),
		ingredientDetails: make(map[string]memIngredient),
		recipes:           make(map[string]map[string]decimal.Decimal),
		rateLimits:        make(map[string]int),
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

func parseTimeArg(arg any) *time.Time {
	if arg == nil {
		return nil
	}
	switch v := arg.(type) {
	case *time.Time:
		return v
	case time.Time:
		return &v
	}
	return nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	cp := s
	return &cp
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

	case strings.HasPrefix(q, "delete from order_evaluations where order_id"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		filtered := tx.db.evaluations[:0]
		for _, ev := range tx.db.evaluations {
			if ev.OrderID != oid {
				filtered = append(filtered, ev)
			}
		}
		tx.db.evaluations = filtered
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("DELETE 1"), nil

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
		id := tx.db.nextUUID()
		tx.db.mu.Lock()
		tx.db.auditLogs++
		actorID := ""
		if args[0] != nil {
			actorID = fmt.Sprint(args[0])
		}
		entityID := ""
		if args[3] != nil {
			entityID = fmt.Sprint(args[3])
		}
		tx.db.auditEntries = append(tx.db.auditEntries, memAuditEntry{
			ID:        id,
			ActorID:   actorID,
			Action:    fmt.Sprint(args[1]),
			Entity:    fmt.Sprint(args[2]),
			EntityID:  entityID,
			Detail:    fmt.Sprint(args[4]),
			CreatedAt: time.Now().UTC(),
		})
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("INSERT 0 1"), nil

	case strings.HasPrefix(q, "update orders set status = $1, total = $2, dp_required = $3, dp_deadline = $4"):
		// DecideOrder update
		oid := fmt.Sprint(args[11])
		tx.db.mu.Lock()
		for i := range tx.db.orders {
			if tx.db.orders[i].ID == oid {
				tx.db.orders[i].Status = fmt.Sprint(args[0])
				tx.db.orders[i].Total = args[1].(decimal.Decimal)
				tx.db.orders[i].DPRequired = args[2].(decimal.Decimal)
				tx.db.orders[i].DPDeadline = parseTimeArg(args[3])
				tx.db.orders[i].Recommendation = fmt.Sprint(args[4])
				tx.db.orders[i].RecommendationReason = fmt.Sprint(args[5])
				if args[6] != nil {
					tx.db.orders[i].CounterOfferJSON = []byte(fmt.Sprint(args[6]))
				}
				tx.db.orders[i].DecidedBy = fmt.Sprint(args[7])
				tx.db.orders[i].DecidedAt = parseTimeArg(args[8])
				if args[9] != nil {
					if sp, ok := args[9].(*string); ok && sp != nil {
						tx.db.orders[i].OverrideReason = *sp
					} else {
						tx.db.orders[i].OverrideReason = fmt.Sprint(args[9])
					}
				}
				if args[10] != nil {
					tx.db.orders[i].SnapshotJSON = []byte(fmt.Sprint(args[10]))
				}
				if u := parseTimeArg(args[8]); u != nil {
					tx.db.orders[i].UpdatedAt = *u
				}
				break
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update orders set event_date = $1::date, production_date = $2::date"):
		// RespondCounterOffer accept=true
		oid := fmt.Sprint(args[10])
		evDate, _ := time.Parse("2006-01-02", fmt.Sprint(args[0])[:10])
		prDate, _ := time.Parse("2006-01-02", fmt.Sprint(args[1])[:10])
		tx.db.mu.Lock()
		for i := range tx.db.orders {
			if tx.db.orders[i].ID == oid {
				tx.db.orders[i].EventDate = evDate
				tx.db.orders[i].ProductionDate = prDate
				tx.db.orders[i].Status = fmt.Sprint(args[2])
				tx.db.orders[i].Total = args[3].(decimal.Decimal)
				tx.db.orders[i].DPRequired = args[4].(decimal.Decimal)
				tx.db.orders[i].DPDeadline = parseTimeArg(args[5])
				tx.db.orders[i].Recommendation = fmt.Sprint(args[6])
				tx.db.orders[i].RecommendationReason = fmt.Sprint(args[7])
				if args[8] != nil {
					tx.db.orders[i].SnapshotJSON = []byte(fmt.Sprint(args[8]))
				}
				if u := parseTimeArg(args[9]); u != nil {
					tx.db.orders[i].UpdatedAt = *u
				}
				break
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update orders set paid = $1, status = $2, updated_at = $3 where id = $4::uuid"):
		oid := fmt.Sprint(args[3])
		tx.db.mu.Lock()
		for i := range tx.db.orders {
			if tx.db.orders[i].ID == oid {
				tx.db.orders[i].Paid = args[0].(decimal.Decimal)
				tx.db.orders[i].Status = fmt.Sprint(args[1])
				if u := parseTimeArg(args[2]); u != nil {
					tx.db.orders[i].UpdatedAt = *u
				}
				break
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update orders set status = $1, notes = $2, updated_at = $3 where id = $4::uuid"):
		oid := fmt.Sprint(args[3])
		tx.db.mu.Lock()
		for i := range tx.db.orders {
			if tx.db.orders[i].ID == oid {
				tx.db.orders[i].Status = fmt.Sprint(args[0])
				if args[1] != nil {
					tx.db.orders[i].Notes = fmt.Sprint(args[1])
				}
				if u := parseTimeArg(args[2]); u != nil {
					tx.db.orders[i].UpdatedAt = *u
				}
				break
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update orders set status = $1, updated_at = $2 where id = $3::uuid"):
		oid := fmt.Sprint(args[2])
		tx.db.mu.Lock()
		for i := range tx.db.orders {
			if tx.db.orders[i].ID == oid {
				tx.db.orders[i].Status = fmt.Sprint(args[0])
				if u := parseTimeArg(args[1]); u != nil {
					tx.db.orders[i].UpdatedAt = *u
				}
				break
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update order_items set unit_price = $1, unit_cost = $2 where id = $3::uuid"):
		iid := fmt.Sprint(args[2])
		tx.db.mu.Lock()
		for i := range tx.db.orderItems {
			if tx.db.orderItems[i].ID == iid {
				tx.db.orderItems[i].UnitPrice = args[0].(decimal.Decimal)
				tx.db.orderItems[i].UnitCost = args[1].(decimal.Decimal)
				break
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update tasks set status = 'doing' where order_id = $1::uuid and type = 'produce'"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		for i := range tx.db.tasks {
			if tx.db.tasks[i].OrderID == oid && tx.db.tasks[i].Type == "produce" && tx.db.tasks[i].Status == "todo" {
				tx.db.tasks[i].Status = "doing"
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update tasks set status = 'done', actual_minutes = $2 where order_id = $1::uuid and type = 'produce'"):
		oid := fmt.Sprint(args[0])
		mins := args[1].(int)
		tx.db.mu.Lock()
		for i := range tx.db.tasks {
			if tx.db.tasks[i].OrderID == oid && tx.db.tasks[i].Type == "produce" {
				tx.db.tasks[i].Status = "done"
				m := mins
				tx.db.tasks[i].ActualMinutes = &m
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update tasks set status = 'done' where order_id = $1::uuid and type = 'produce'"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		for i := range tx.db.tasks {
			if tx.db.tasks[i].OrderID == oid && tx.db.tasks[i].Type == "produce" {
				tx.db.tasks[i].Status = "done"
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update tasks set status = 'doing' where order_id = $1::uuid and type = 'deliver'"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		for i := range tx.db.tasks {
			if tx.db.tasks[i].OrderID == oid && tx.db.tasks[i].Type == "deliver" && tx.db.tasks[i].Status == "todo" {
				tx.db.tasks[i].Status = "doing"
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update tasks set status = 'done' where order_id = $1::uuid and type = 'deliver'"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		for i := range tx.db.tasks {
			if tx.db.tasks[i].OrderID == oid && tx.db.tasks[i].Type == "deliver" {
				tx.db.tasks[i].Status = "done"
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update tasks set status = $1, actual_minutes = $2 where id = $3::uuid"):
		tid := fmt.Sprint(args[2])
		var minsPtr *int
		if args[1] != nil {
			switch v := args[1].(type) {
			case *int:
				if v != nil {
					m := *v
					minsPtr = &m
				}
			case int:
				m := v
				minsPtr = &m
			}
		}
		tx.db.mu.Lock()
		for i := range tx.db.tasks {
			if tx.db.tasks[i].ID == tid {
				tx.db.tasks[i].Status = fmt.Sprint(args[0])
				tx.db.tasks[i].ActualMinutes = minsPtr
				break
			}
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "delete from tasks where order_id = $1::uuid and status != 'done'"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		filtered := tx.db.tasks[:0]
		for _, tk := range tx.db.tasks {
			if tk.OrderID == oid && tk.Status != "done" {
				continue
			}
			filtered = append(filtered, tk)
		}
		tx.db.tasks = filtered
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("DELETE 1"), nil

	case strings.HasPrefix(q, "update ingredients set stock = greatest(0, stock - $1) where id = $2::uuid"):
		used := args[0].(decimal.Decimal)
		iid := fmt.Sprint(args[1])
		tx.db.mu.Lock()
		cur := tx.db.ingredients[iid].Sub(used)
		if cur.IsNegative() {
			cur = decimal.Zero
		}
		tx.db.ingredients[iid] = cur
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update ingredients set stock = $1, buy_price = $2 where id = $3::uuid"):
		newSt := args[0].(decimal.Decimal)
		newBP := args[1].(decimal.Decimal)
		iid := fmt.Sprint(args[2])
		tx.db.mu.Lock()
		tx.db.ingredients[iid] = newSt
		if im, ok := tx.db.ingredientDetails[iid]; ok {
			im.BuyPrice = newBP
			tx.db.ingredientDetails[iid] = im
		}
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "update ingredients set stock = $1 where id = $2::uuid"):
		newSt := args[0].(decimal.Decimal)
		iid := fmt.Sprint(args[1])
		tx.db.mu.Lock()
		tx.db.ingredients[iid] = newSt
		tx.db.mu.Unlock()
		return pgconn.NewCommandTag("UPDATE 1"), nil

	case strings.HasPrefix(q, "insert into stock_movements"):
		id := tx.db.nextUUID()
		now := time.Now().UTC()
		if u := parseTimeArg(args[3]); u != nil {
			now = *u
		}
		tx.db.mu.Lock()
		tx.db.stockMovements = append(tx.db.stockMovements, memStockMovement{
			ID:           id,
			IngredientID: fmt.Sprint(args[0]),
			Delta:        args[1].(decimal.Decimal),
			Reason:       "production",
			RefType:      "order",
			RefID:        fmt.Sprint(args[2]),
			CreatedAt:    now,
		})
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

	case strings.Contains(q, "select event_date, production_date from orders where id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.ID == oid {
				return &memRow{vals: []any{o.EventDate, o.ProductionDate}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.Contains(q, "select count(*) from orders where customer_id = $1::uuid"):
		cid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		cnt := 0
		for _, o := range tx.db.orders {
			if o.CustomerID == cid &&
				(o.Status == "pending_decision" || o.Status == "countered" || o.Status == "accepted") &&
				o.Paid.LessThan(o.DPRequired) {
				cnt++
			}
		}
		tx.db.mu.Unlock()
		return &memRow{vals: []any{cnt}}

	case strings.HasPrefix(q, "select id::text, code, source, customer_id::text, customer_name, customer_contact, delivery_type, address, notes, event_date, production_date"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.ID == oid {
				custID := strPtrOrNil(o.CustomerID)
				addr := strPtrOrNil(o.Address)
				notes := strPtrOrNil(o.Notes)
				rec := strPtrOrNil(o.Recommendation)
				reason := strPtrOrNil(o.RecommendationReason)
				decidedBy := strPtrOrNil(o.DecidedBy)
				overrideReason := strPtrOrNil(o.OverrideReason)
				return &memRow{vals: []any{
					o.ID, o.Code, o.Source, custID, o.CustomerName, o.CustomerContact, o.DeliveryType,
					addr, notes, o.EventDate, o.ProductionDate,
					o.Status, o.Total, o.DPRequired, o.DPDeadline, o.Paid,
					rec, reason, o.CounterOfferJSON,
					decidedBy, o.DecidedAt, overrideReason, o.SnapshotJSON,
					o.CreatedAt, o.UpdatedAt,
				}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "select t.id::text, t.order_id::text, t.type, t.title, t.due_date"):
		tid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, tk := range tx.db.tasks {
			if tk.ID == tid {
				for _, o := range tx.db.orders {
					if o.ID == tk.OrderID {
						dueTime, _ := time.Parse("2006-01-02", tk.DueDate[:10])
						assigneePtr := strPtrOrNil(tk.AssigneeID)
						return &memRow{vals: []any{
							tk.ID, tk.OrderID, tk.Type, tk.Title, dueTime, assigneePtr,
							tk.Status, tk.ActualMinutes, tk.CreatedAt,
							o.Code, o.CustomerName, o.CustomerContact, o.DeliveryType,
							o.Address, o.Notes,
							o.EventDate, o.ProductionDate, o.Status,
						}}
					}
				}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "select id::text, code, source, customer_name, customer_contact, delivery_type, address, notes, event_date, production_date"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.ID == oid {
				addr := strPtrOrNil(o.Address)
				notes := strPtrOrNil(o.Notes)
				rec := strPtrOrNil(o.Recommendation)
				reason := strPtrOrNil(o.RecommendationReason)
				return &memRow{vals: []any{
					o.ID, o.Code, o.Source, o.CustomerName, o.CustomerContact, o.DeliveryType,
					addr, notes, o.EventDate, o.ProductionDate,
					o.Status, o.Total, o.DPRequired, o.DPDeadline, o.Paid,
					rec, reason, o.CounterOfferJSON,
					o.CreatedAt, o.UpdatedAt,
				}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "select id::text, customer_name, delivery_type, event_date, production_date, status, total, dp_required, dp_deadline, paid, counter_offer, created_at, updated_at from orders where code = $1"):
		code := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.Code == code {
				return &memRow{vals: []any{
					o.ID, o.CustomerName, o.DeliveryType, o.EventDate, o.ProductionDate, o.Status,
					o.Total, o.DPRequired, o.DPDeadline, o.Paid, o.CounterOfferJSON, o.CreatedAt, o.UpdatedAt,
				}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "select status from orders where id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.ID == oid {
				return &memRow{vals: []any{o.Status}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "select code, customer_name, event_date, production_date, status, total, dp_required, paid from orders where id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.ID == oid {
				return &memRow{vals: []any{
					o.Code, o.CustomerName, o.EventDate, o.ProductionDate, o.Status, o.Total, o.DPRequired, o.Paid,
				}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "select code, status, total, paid from orders where id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.ID == oid {
				return &memRow{vals: []any{o.Code, o.Status, o.Total, o.Paid}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "select code, status, paid, notes from orders where id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		for _, o := range tx.db.orders {
			if o.ID == oid {
				notes := strPtrOrNil(o.Notes)
				return &memRow{vals: []any{o.Code, o.Status, o.Paid, notes}}
			}
		}
		return &memRow{err: pgx.ErrNoRows}

	case strings.HasPrefix(q, "insert into payments"):
		id := tx.db.nextUUID()
		now := time.Now().UTC()
		if u := parseTimeArg(args[3]); u != nil {
			now = *u
		}
		note := ""
		if args[4] != nil {
			note = fmt.Sprint(args[4])
		}
		recBy := ""
		if args[5] != nil {
			recBy = fmt.Sprint(args[5])
		}
		tx.db.mu.Lock()
		tx.db.payments = append(tx.db.payments, memPayment{
			ID:         id,
			OrderID:    fmt.Sprint(args[0]),
			Amount:     args[1].(decimal.Decimal),
			Method:     fmt.Sprint(args[2]),
			PaidAt:     now,
			Note:       note,
			RecordedBy: recBy,
		})
		tx.db.mu.Unlock()
		return &memRow{vals: []any{id}}

	case strings.HasPrefix(q, "insert into tasks"):
		id := tx.db.nextUUID()
		now := time.Now().UTC()
		if u := parseTimeArg(args[4]); u != nil {
			now = *u
		}
		tx.db.mu.Lock()
		tx.db.tasks = append(tx.db.tasks, memTask{
			ID:        id,
			OrderID:   fmt.Sprint(args[0]),
			Type:      fmt.Sprint(args[1]),
			Title:     fmt.Sprint(args[2]),
			DueDate:   fmt.Sprint(args[3])[:10],
			Status:    "todo",
			CreatedAt: now,
		})
		tx.db.mu.Unlock()
		return &memRow{vals: []any{id}}

	case strings.Contains(q, "from stock_movements where ref_type = 'order'"):
		oid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		exists := false
		for _, sm := range tx.db.stockMovements {
			if sm.RefType == "order" && sm.RefID == oid && sm.Reason == "production" {
				exists = true
				break
			}
		}
		tx.db.mu.Unlock()
		return &memRow{vals: []any{exists}}

	case strings.HasPrefix(q, "insert into orders"):
		now := time.Now().UTC()
		custID := ""
		if args[2] != nil {
			custID = fmt.Sprint(args[2])
		}
		evDate, _ := time.Parse("2006-01-02", fmt.Sprint(args[8])[:10])
		prDate, _ := time.Parse("2006-01-02", fmt.Sprint(args[9])[:10])
		dpDeadline := parseTimeArg(args[13])
		var counterBytes []byte
		if args[16] != nil {
			counterBytes = []byte(fmt.Sprint(args[16]))
		}
		var snapBytes []byte
		if args[17] != nil {
			snapBytes = []byte(fmt.Sprint(args[17]))
		}
		addr := ""
		if args[6] != nil {
			addr = fmt.Sprint(args[6])
		}
		notes := ""
		if args[7] != nil {
			notes = fmt.Sprint(args[7])
		}
		id := tx.db.nextUUID()
		ord := memOrder{
			ID:                   id,
			Code:                 fmt.Sprint(args[0]),
			Source:               fmt.Sprint(args[1]),
			CustomerID:           custID,
			CustomerName:         fmt.Sprint(args[3]),
			CustomerContact:      fmt.Sprint(args[4]),
			DeliveryType:         fmt.Sprint(args[5]),
			Address:              addr,
			Notes:                notes,
			EventDate:            evDate,
			ProductionDate:       prDate,
			Status:               fmt.Sprint(args[10]),
			Total:                args[11].(decimal.Decimal),
			DPRequired:           args[12].(decimal.Decimal),
			DPDeadline:           dpDeadline,
			Paid:                 decimal.Zero,
			Recommendation:       fmt.Sprint(args[14]),
			RecommendationReason: fmt.Sprint(args[15]),
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

	case strings.Contains(q, "from ingredients where id = $1::uuid for update"):
		iid := fmt.Sprint(args[0])
		tx.db.mu.Lock()
		defer tx.db.mu.Unlock()
		im, ok := tx.db.ingredientDetails[iid]
		if !ok {
			return &memRow{err: pgx.ErrNoRows}
		}
		st := tx.db.ingredients[iid]
		return &memRow{vals: []any{
			im.ID, im.Name, im.UseUnit, im.BuyUnit, im.BuyUnitSize, im.BuyPrice, st, im.MinStock, im.Rounding, im.IsActive,
		}}

	case strings.HasPrefix(q, "insert into purchases"):
		id := tx.db.nextUUID()
		now := time.Now().UTC()
		if u := parseTimeArg(args[3]); u != nil {
			now = *u
		}
		boughtBy := ""
		if args[4] != nil {
			boughtBy = fmt.Sprint(args[4])
		}
		tx.db.mu.Lock()
		tx.db.purchases = append(tx.db.purchases, memPurchase{
			ID:           id,
			IngredientID: fmt.Sprint(args[0]),
			QtyBuyUnits:  args[1].(decimal.Decimal),
			UnitPrice:    args[2].(decimal.Decimal),
			BoughtAt:     now,
			BoughtBy:     boughtBy,
		})
		tx.db.mu.Unlock()
		return &memRow{vals: []any{id}}

	case strings.HasPrefix(q, "insert into stock_movements"):
		id := tx.db.nextUUID()
		now := time.Now().UTC()
		reason := "production"
		refType := "order"
		refID := ""
		if strings.Contains(q, "'purchase', 'purchase'") {
			reason = "purchase"
			refType = "purchase"
			refID = fmt.Sprint(args[2])
			if u := parseTimeArg(args[3]); u != nil {
				now = *u
			}
		} else if strings.Contains(q, "'adjustment'") {
			reason = fmt.Sprint(args[2])
			refType = "adjustment"
			if u := parseTimeArg(args[3]); u != nil {
				now = *u
			}
		}
		tx.db.mu.Lock()
		tx.db.stockMovements = append(tx.db.stockMovements, memStockMovement{
			ID:           id,
			IngredientID: fmt.Sprint(args[0]),
			Delta:        args[1].(decimal.Decimal),
			Reason:       reason,
			RefType:      refType,
			RefID:        refID,
			CreatedAt:    now,
		})
		tx.db.mu.Unlock()
		return &memRow{vals: []any{id}}

	case strings.HasPrefix(q, "insert into cash_entries"):
		id := tx.db.nextUUID()
		dt, _ := time.Parse("2006-01-02", fmt.Sprint(args[0])[:10])
		note := ""
		if args[2] != nil {
			note = fmt.Sprint(args[2])
		}
		createdBy := ""
		if args[3] != nil {
			createdBy = fmt.Sprint(args[3])
		}
		tx.db.mu.Lock()
		tx.db.cashEntries = append(tx.db.cashEntries, memCashEntry{
			ID:        id,
			EntryDate: dt,
			Amount:    args[1].(decimal.Decimal),
			Note:      note,
			CreatedBy: createdBy,
		})
		tx.db.mu.Unlock()
		return &memRow{vals: []any{id}}

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
	case strings.HasPrefix(q, "update orders set status = 'expired'"):
		now := args[0].(time.Time)
		var rows [][]any
		for i := range tx.db.orders {
			o := &tx.db.orders[i]
			if o.Status == "accepted" && o.DPDeadline != nil && !o.DPDeadline.After(now) && o.Paid.LessThan(o.DPRequired) {
				o.Status = "expired"
				o.UpdatedAt = now
				rows = append(rows, []any{o.ID, o.Code, o.DPRequired, o.Paid, *o.DPDeadline})
			}
		}
		return &memRows{rows: rows}, nil

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

	case strings.HasPrefix(q, "select id::text, name, category, price, packaging_cost, overhead_cost, is_active from products order by name asc, id asc"):
		var rows [][]any
		for _, p := range tx.db.products {
			rows = append(rows, []any{p.ID, p.Name, p.Category, p.Price, p.PackagingCost, p.OverheadCost, p.IsActive})
		}
		return &memRows{rows: rows}, nil

	case strings.HasPrefix(q, "select id::text, name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active from ingredients where is_active = true"):
		var rows [][]any
		for id, im := range tx.db.ingredientDetails {
			if !im.IsActive {
				continue
			}
			st := tx.db.ingredients[id]
			rows = append(rows, []any{
				im.ID, im.Name, im.UseUnit, im.BuyUnit, im.BuyUnitSize, im.BuyPrice, st, im.MinStock, im.Rounding, im.IsActive,
			})
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "count(distinct o.id) as order_count"):
		fromKey := fmt.Sprint(args[0])[:10]
		toKey := fmt.Sprint(args[1])[:10]
		type agg struct {
			qty      decimal.Decimal
			minEvent time.Time
			orders   map[string]bool
		}
		byIng := make(map[string]*agg)
		for _, o := range tx.db.orders {
			if o.Status != "confirmed" && o.Status != "in_production" {
				continue
			}
			ek := o.EventDate.Format("2006-01-02")
			if ek < fromKey || ek > toKey {
				continue
			}
			for _, it := range tx.db.orderItems {
				if it.OrderID != o.ID {
					continue
				}
				qDec := decimal.NewFromInt(int64(it.Qty))
				for ingID, qtyPerUnit := range tx.db.recipes[it.ProductID] {
					a, ok := byIng[ingID]
					if !ok {
						a = &agg{
							qty:      decimal.Zero,
							minEvent: o.EventDate,
							orders:   make(map[string]bool),
						}
						byIng[ingID] = a
					}
					a.qty = a.qty.Add(qDec.Mul(qtyPerUnit))
					if o.EventDate.Before(a.minEvent) {
						a.minEvent = o.EventDate
					}
					a.orders[o.ID] = true
				}
			}
		}
		var rows [][]any
		for ingID, a := range byIng {
			rows = append(rows, []any{ingID, a.qty, a.minEvent, len(a.orders)})
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

	case strings.Contains(q, "select code, name, unit, count, amount_per_unit_day, efficiency from resources"):
		var rows [][]any
		for _, r := range tx.db.resources {
			rows = append(rows, []any{r.Code, r.Name, r.Unit, r.Count, r.AmountPerUnitDay, r.Efficiency})
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

	case strings.Contains(q, "select id::text, entry_date, amount, coalesce(note, ''), created_by::text from cash_entries"):
		fromKey := fmt.Sprint(args[0])[:10]
		toKey := fmt.Sprint(args[1])[:10]
		var rows [][]any
		for _, ce := range tx.db.cashEntries {
			dk := ce.EntryDate.Format("2006-01-02")
			if dk >= fromKey && dk <= toKey {
				cb := strPtrOrNil(ce.CreatedBy)
				rows = append(rows, []any{ce.ID, ce.EntryDate, ce.Amount, ce.Note, cb})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from cash_entries"):
		fromKey := fmt.Sprint(args[0])[:10]
		var rows [][]any
		for _, ce := range tx.db.cashEntries {
			dk := ce.EntryDate.Format("2006-01-02")
			if dk >= fromKey {
				rows = append(rows, []any{ce.EntryDate, ce.Amount})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.HasPrefix(q, "select oi.id::text, oi.product_id::text, p.name, oi.qty, oi.unit_price, oi.unit_cost from order_items"):
		oid := fmt.Sprint(args[0])
		var rows [][]any
		for _, it := range tx.db.orderItems {
			if it.OrderID == oid {
				p := tx.db.products[it.ProductID]
				rows = append(rows, []any{it.ID, it.ProductID, p.Name, it.Qty, it.UnitPrice, it.UnitCost})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.HasPrefix(q, "select oi.product_id::text, p.name, oi.qty from order_items oi join products p"):
		oid := fmt.Sprint(args[0])
		var rows [][]any
		for _, it := range tx.db.orderItems {
			if it.OrderID == oid {
				p := tx.db.products[it.ProductID]
				rows = append(rows, []any{it.ProductID, p.Name, it.Qty})
			}
		}
		return &memRows{rows: rows}, nil

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

	case strings.Contains(q, "from order_evaluations where order_id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		var rows [][]any
		for _, ev := range tx.db.evaluations {
			if ev.OrderID == oid {
				rows = append(rows, []any{ev.Gate, ev.Value, ev.Threshold, ev.Passed})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from payments where order_id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		var rows [][]any
		for _, pm := range tx.db.payments {
			if pm.OrderID == oid {
				recBy := strPtrOrNil(pm.RecordedBy)
				rows = append(rows, []any{pm.ID, pm.OrderID, pm.Amount, pm.Method, pm.PaidAt, pm.Note, recBy})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.HasPrefix(q, "select id::text, order_id::text, type, title, due_date, assignee_id::text, status, actual_minutes, created_at from tasks where order_id = $1::uuid"):
		oid := fmt.Sprint(args[0])
		var rows [][]any
		for _, tk := range tx.db.tasks {
			if tk.OrderID == oid {
				dueTime, _ := time.Parse("2006-01-02", tk.DueDate[:10])
				assigneePtr := strPtrOrNil(tk.AssigneeID)
				rows = append(rows, []any{tk.ID, tk.OrderID, tk.Type, tk.Title, dueTime, assigneePtr, tk.Status, tk.ActualMinutes, tk.CreatedAt})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.HasPrefix(q, "select t.id::text, t.order_id::text, t.type, t.title, t.due_date"):
		var rows [][]any
		for _, tk := range tx.db.tasks {
			for _, o := range tx.db.orders {
				if o.ID == tk.OrderID {
					dueTime, _ := time.Parse("2006-01-02", tk.DueDate[:10])
					assigneePtr := strPtrOrNil(tk.AssigneeID)
					rows = append(rows, []any{
						tk.ID, tk.OrderID, tk.Type, tk.Title, dueTime, assigneePtr,
						tk.Status, tk.ActualMinutes, tk.CreatedAt,
						o.Code, o.CustomerName, o.CustomerContact, o.DeliveryType,
						o.Address, o.Notes,
						o.EventDate, o.ProductionDate, o.Status,
					})
					break
				}
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from audit_logs a left join users u"):
		oid := fmt.Sprint(args[0])
		var rows [][]any
		for i := len(tx.db.auditEntries) - 1; i >= 0; i-- {
			ae := tx.db.auditEntries[i]
			if ae.Entity == "orders" && ae.EntityID == oid {
				id := ae.ID
				if id == "" {
					id = fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
				}
				actorPtr := strPtrOrNil(ae.ActorID)
				actorName := ""
				if ae.ActorID != "" {
					actorName = "Pengguna Internal"
				}
				rows = append(rows, []any{id, actorPtr, actorName, ae.Action, ae.Detail, ae.CreatedAt})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from order_items oi join recipe_items ri"):
		oid := fmt.Sprint(args[0])
		byIng := make(map[string]decimal.Decimal)
		for _, it := range tx.db.orderItems {
			if it.OrderID != oid {
				continue
			}
			qDec := decimal.NewFromInt(int64(it.Qty))
			for ingID, qtyPerUnit := range tx.db.recipes[it.ProductID] {
				byIng[ingID] = byIng[ingID].Add(qDec.Mul(qtyPerUnit))
			}
		}
		var rows [][]any
		for ingID, used := range byIng {
			rows = append(rows, []any{ingID, used})
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from orders where status = 'pending_decision'"):
		var rows [][]any
		for _, o := range tx.db.orders {
			if o.Status == "pending_decision" {
				rows = append(rows, []any{
					o.ID, o.Code, o.Source, o.CustomerName, o.CustomerContact, o.DeliveryType,
					o.EventDate, o.ProductionDate, o.Status, o.Total, o.DPRequired, o.CreatedAt,
				})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from orders where production_date = $1::date and status in ('accepted', 'confirmed', 'in_production')"):
		dk := fmt.Sprint(args[0])[:10]
		var rows [][]any
		for _, o := range tx.db.orders {
			if o.ProductionDate.Format("2006-01-02") == dk &&
				(o.Status == "accepted" || o.Status == "confirmed" || o.Status == "in_production") {
				rows = append(rows, []any{o.ID, o.Code, o.CustomerName, o.Status, o.EventDate, o.ProductionDate, o.Total})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.Contains(q, "from orders where customer_id = $1::uuid"):
		cid := fmt.Sprint(args[0])
		var rows [][]any
		for i := len(tx.db.orders) - 1; i >= 0; i-- {
			o := tx.db.orders[i]
			if o.CustomerID == cid {
				rows = append(rows, []any{
					o.ID, o.Code, o.CustomerName, o.DeliveryType, o.EventDate, o.Status,
					o.Total, o.DPRequired, o.DPDeadline, o.Paid, o.CounterOfferJSON,
					o.CreatedAt, o.UpdatedAt,
				})
			}
		}
		return &memRows{rows: rows}, nil

	case strings.HasPrefix(q, "select id::text from orders"):
		var rows [][]any
		for i := len(tx.db.orders) - 1; i >= 0; i-- {
			rows = append(rows, []any{tx.db.orders[i].ID})
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
