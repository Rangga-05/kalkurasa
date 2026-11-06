package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/httpx"
)

// Status pesanan sesuai check constraint tabel orders.
const (
	StatusSubmitted       = "submitted"
	StatusPendingDecision = "pending_decision"
	StatusCountered       = "countered"
	StatusAccepted        = "accepted"
	StatusConfirmed       = "confirmed"
	StatusInProduction    = "in_production"
	StatusReady           = "ready"
	StatusDelivering      = "delivering"
	StatusCompleted       = "completed"
	StatusRejected        = "rejected"
	StatusCancelled       = "cancelled"
	StatusExpired         = "expired"
)

// SettingsParams memuat seluruh parameter bisnis dari tabel settings.
type SettingsParams struct {
	EvalParams        engine.EvalParams
	StatusGreenBelow  decimal.Decimal
	StatusYellowBelow decimal.Decimal
	LaborRatePerMin   decimal.Decimal
	WastePct          decimal.Decimal
	ShoppingBufferPct decimal.Decimal
	DPDeadlineHours   int
	BigOrderTotal     decimal.Decimal
	BigOrderUnits     int
}

// DefaultSettingsParams mengembalikan nilai default sesuai Bagian 5 brief.
func DefaultSettingsParams() SettingsParams {
	return SettingsParams{
		EvalParams: engine.EvalParams{
			CapacityLimit:    decimal.RequireFromString("0.90"),
			MarginMin:        decimal.RequireFromString("0.20"),
			CashBuffer:       decimal.NewFromInt(400_000),
			FixedCostDaily:   decimal.NewFromInt(400_000),
			OpeningCash:      decimal.NewFromInt(1_800_000),
			DPPct:            decimal.RequireFromString("0.30"),
			LeadTimeDays:     2,
			ShoppingLeadDays: 2,
		},
		StatusGreenBelow:  decimal.RequireFromString("0.70"),
		StatusYellowBelow: decimal.RequireFromString("0.90"),
		LaborRatePerMin:   decimal.NewFromInt(400),
		WastePct:          decimal.RequireFromString("0.05"),
		ShoppingBufferPct: decimal.RequireFromString("0.10"),
		DPDeadlineHours:   24,
		BigOrderTotal:     decimal.NewFromInt(2_000_000),
		BigOrderUnits:     50,
	}
}

// LoadSettingsParams membaca tabel settings dan menimpanya di atas default.
func LoadSettingsParams(ctx context.Context, q db.DBTX) (SettingsParams, error) {
	p := DefaultSettingsParams()
	rows, err := q.Query(ctx, `SELECT key, value::text FROM settings`)
	if err != nil {
		return p, err
	}
	defer rows.Close()

	parseDec := func(raw string, fallback decimal.Decimal) decimal.Decimal {
		var d decimal.Decimal
		if err := json.Unmarshal([]byte(raw), &d); err == nil {
			return d
		}
		return fallback
	}
	parseInt := func(raw string, fallback int) int {
		var n int
		if err := json.Unmarshal([]byte(raw), &n); err == nil {
			return n
		}
		return fallback
	}

	for rows.Next() {
		var k, raw string
		if err := rows.Scan(&k, &raw); err != nil {
			return p, err
		}
		switch k {
		case "capacity_limit":
			p.EvalParams.CapacityLimit = parseDec(raw, p.EvalParams.CapacityLimit)
		case "status_green_below":
			p.StatusGreenBelow = parseDec(raw, p.StatusGreenBelow)
		case "status_yellow_below":
			p.StatusYellowBelow = parseDec(raw, p.StatusYellowBelow)
		case "margin_min":
			p.EvalParams.MarginMin = parseDec(raw, p.EvalParams.MarginMin)
		case "labor_rate_per_minute":
			p.LaborRatePerMin = parseDec(raw, p.LaborRatePerMin)
		case "waste_pct":
			p.WastePct = parseDec(raw, p.WastePct)
		case "shopping_buffer_pct":
			p.ShoppingBufferPct = parseDec(raw, p.ShoppingBufferPct)
		case "cash_buffer":
			p.EvalParams.CashBuffer = parseDec(raw, p.EvalParams.CashBuffer)
		case "fixed_cost_daily":
			p.EvalParams.FixedCostDaily = parseDec(raw, p.EvalParams.FixedCostDaily)
		case "opening_cash":
			p.EvalParams.OpeningCash = parseDec(raw, p.EvalParams.OpeningCash)
		case "lead_time_days":
			p.EvalParams.LeadTimeDays = parseInt(raw, p.EvalParams.LeadTimeDays)
		case "dp_pct":
			p.EvalParams.DPPct = parseDec(raw, p.EvalParams.DPPct)
		case "dp_deadline_hours":
			p.DPDeadlineHours = parseInt(raw, p.DPDeadlineHours)
		case "big_order_total":
			p.BigOrderTotal = parseDec(raw, p.BigOrderTotal)
		case "big_order_units":
			p.BigOrderUnits = parseInt(raw, p.BigOrderUnits)
		case "shopping_lead_days":
			p.EvalParams.ShoppingLeadDays = parseInt(raw, p.EvalParams.ShoppingLeadDays)
		}
	}
	return p, rows.Err()
}

// ProductMeta menyimpan informasi produk beserta biaya dan beban per unit.
type ProductMeta struct {
	ID                  string
	Name                string
	Category            string
	DefaultPrice        decimal.Decimal
	PackagingCost       decimal.Decimal
	OverheadCost        decimal.Decimal
	IngredientCost      decimal.Decimal
	UnitCost            decimal.Decimal
	PurchaseCostPerUnit decimal.Decimal
	UnitLoads           map[string]decimal.Decimal
}

// LoadProductsMeta memuat produk aktif berdasarkan daftar ID dan menghitung
// UnitCost serta PurchaseCostPerUnit menggunakan internal/engine.
func LoadProductsMeta(ctx context.Context, q db.DBTX, ids []string, params SettingsParams) (map[string]ProductMeta, error) {
	out := make(map[string]ProductMeta, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	rows, err := q.Query(ctx, `
		SELECT id::text, name, category, price, packaging_cost, overhead_cost, is_active
		FROM products
		WHERE id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var pm ProductMeta
		var isActive bool
		if err := rows.Scan(&pm.ID, &pm.Name, &pm.Category, &pm.DefaultPrice, &pm.PackagingCost, &pm.OverheadCost, &isActive); err != nil {
			return nil, err
		}
		if !isActive {
			return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", fmt.Sprintf("Produk %q sedang tidak aktif.", pm.Name))
		}
		pm.UnitLoads = make(map[string]decimal.Decimal)
		out[pm.ID] = pm
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, id := range ids {
		if _, ok := out[id]; !ok {
			return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Salah satu produk tidak ditemukan.")
		}
	}

	// Muat beban sumber daya per produk (product_loads JOIN resources)
	loadRows, err := q.Query(ctx, `
		SELECT pl.product_id::text, r.code, pl.load
		FROM product_loads pl
		JOIN resources r ON r.id = pl.resource_id
		WHERE pl.product_id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, err
	}
	defer loadRows.Close()

	for loadRows.Next() {
		var pid, code string
		var load decimal.Decimal
		if err := loadRows.Scan(&pid, &code, &load); err != nil {
			return nil, err
		}
		if pm, ok := out[pid]; ok {
			pm.UnitLoads[code] = load
			out[pid] = pm
		}
	}
	if err := loadRows.Err(); err != nil {
		return nil, err
	}

	// Muat biaya bahan dari resep (recipe_items JOIN ingredients)
	recRows, err := q.Query(ctx, `
		SELECT ri.product_id::text, ri.qty_per_unit, i.buy_price, i.buy_unit_size
		FROM recipe_items ri
		JOIN ingredients i ON i.id = ri.ingredient_id
		WHERE ri.product_id = ANY($1::uuid[])
	`, ids)
	if err != nil {
		return nil, err
	}
	defer recRows.Close()

	for recRows.Next() {
		var pid string
		var qty, buyPrice, buyUnitSize decimal.Decimal
		if err := recRows.Scan(&pid, &qty, &buyPrice, &buyUnitSize); err != nil {
			return nil, err
		}
		if pm, ok := out[pid]; ok && buyUnitSize.IsPositive() {
			costPart := qty.Mul(buyPrice).DivRound(buyUnitSize, 8)
			pm.IngredientCost = pm.IngredientCost.Add(costPart)
			out[pid] = pm
		}
	}
	if err := recRows.Err(); err != nil {
		return nil, err
	}

	// Hitung UnitCost (engine.UnitCost) dan PurchaseCostPerUnit (bahan*(1+waste) + kemasan)
	onePlusWaste := decimal.NewFromInt(1).Add(params.WastePct)
	for id, pm := range out {
		laborMin := pm.UnitLoads["labor"]
		pm.UnitCost = engine.UnitCost(engine.CostInput{
			IngredientCost: pm.IngredientCost,
			WastePct:       params.WastePct,
			PackagingCost:  pm.PackagingCost,
			LaborMinutes:   laborMin,
			LaborRate:      params.LaborRatePerMin,
			OverheadCost:   pm.OverheadCost,
		})
		pm.PurchaseCostPerUnit = pm.IngredientCost.Mul(onePlusWaste).Add(pm.PackagingCost)
		out[id] = pm
	}

	return out, nil
}

// LockDays mengunci baris pada tabel day_locks sesuai Bagian 8.1 dengan dua
// statement per transaksi (INSERT unnest ON CONFLICT DO NOTHING, lalu
// SELECT ... WHERE day = ANY($1::date[]) ORDER BY day FOR UPDATE) dengan
// urutan tanggal menaik (ascending) untuk mencegah deadlock.
func LockDays(ctx context.Context, tx pgx.Tx, dates []time.Time) error {
	seen := make(map[string]bool, len(dates))
	keys := make([]string, 0, len(dates))
	for _, d := range dates {
		k := engine.DateKey(d)
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)

	if _, err := tx.Exec(ctx, `INSERT INTO day_locks(day) SELECT unnest($1::date[]) ON CONFLICT DO NOTHING`, keys); err != nil {
		return fmt.Errorf("lock days insert: %w", err)
	}

	rows, err := tx.Query(ctx, `SELECT day FROM day_locks WHERE day = ANY($1::date[]) ORDER BY day FOR UPDATE`, keys)
	if err != nil {
		return fmt.Errorf("lock days for update: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var locked time.Time
		if err := rows.Scan(&locked); err != nil {
			return fmt.Errorf("lock days scan: %w", err)
		}
	}
	return rows.Err()
}

// LoadCapacities memuat seluruh sumber daya dan menghitung kapasitas efektif harian via engine.Capacity.
func LoadCapacities(ctx context.Context, q db.DBTX) (map[string]decimal.Decimal, error) {
	rows, err := q.Query(ctx, `SELECT code, count, amount_per_unit_day, efficiency FROM resources`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	caps := make(map[string]decimal.Decimal)
	for rows.Next() {
		var r engine.Resource
		if err := rows.Scan(&r.Code, &r.Count, &r.AmountPerUnitDay, &r.Efficiency); err != nil {
			return nil, err
		}
		caps[r.Code] = engine.Capacity(r)
	}
	return caps, rows.Err()
}

// LoadExistingLoads memuat beban existing pada rentang tanggal produksi [fromDate, toDate]
// dari pesanan berstatus accepted, confirmed, dan in_production (Bagian 8.1).
func LoadExistingLoads(ctx context.Context, q db.DBTX, fromDate, toDate time.Time) (map[string]map[string]decimal.Decimal, error) {
	fromKey := engine.DateKey(fromDate)
	toKey := engine.DateKey(toDate)

	rows, err := q.Query(ctx, `
		SELECT o.production_date::text, r.code, COALESCE(SUM(oi.qty * pl.load), 0)
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		JOIN product_loads pl ON pl.product_id = oi.product_id
		JOIN resources r ON r.id = pl.resource_id
		WHERE o.status IN ('accepted', 'confirmed', 'in_production')
		  AND o.production_date >= $1::date
		  AND o.production_date <= $2::date
		GROUP BY o.production_date, r.code
	`, fromKey, toKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]map[string]decimal.Decimal)
	for rows.Next() {
		var dayStr, code string
		var totalLoad decimal.Decimal
		if err := rows.Scan(&dayStr, &code, &totalLoad); err != nil {
			return nil, err
		}
		if len(dayStr) >= 10 {
			dayStr = dayStr[:10]
		}
		if out[dayStr] == nil {
			out[dayStr] = make(map[string]decimal.Decimal)
		}
		out[dayStr][code] = totalLoad
	}
	return out, rows.Err()
}

// LoadCashEvents memuat peristiwa kas dari tabel cash_entries dan jadwal kas
// pesanan aktif lain (status accepted, confirmed, in_production, ready, delivering).
func LoadCashEvents(ctx context.Context, q db.DBTX, today time.Time, params SettingsParams) ([]engine.CashEvent, error) {
	todayKey := engine.DateKey(today)
	events := make([]engine.CashEvent, 0)

	// 1. Entri kas manual (cash_entries) mulai dari hari ini
	ceRows, err := q.Query(ctx, `
		SELECT entry_date, amount
		FROM cash_entries
		WHERE entry_date >= $1::date
	`, todayKey)
	if err != nil {
		return nil, err
	}
	defer ceRows.Close()

	for ceRows.Next() {
		var dt time.Time
		var amt decimal.Decimal
		if err := ceRows.Scan(&dt, &amt); err != nil {
			return nil, err
		}
		events = append(events, engine.CashEvent{Date: dt, Amount: amt})
	}
	if err := ceRows.Err(); err != nil {
		return nil, err
	}

	// 2. Jadwal kas dari pesanan aktif yang sudah direservasi
	onePlusWaste := decimal.NewFromInt(1).Add(params.WastePct)
	ordRows, err := q.Query(ctx, `
		SELECT o.id::text, o.status, o.event_date, o.total, o.dp_required, o.dp_deadline, o.paid,
		       COALESCE((
		           SELECT SUM(oi.qty * (
		               COALESCE((
		                   SELECT SUM(ri.qty_per_unit * i.buy_price / i.buy_unit_size)
		                   FROM recipe_items ri
		                   JOIN ingredients i ON i.id = ri.ingredient_id
		                   WHERE ri.product_id = p.id
		               ), 0) * $2::numeric + p.packaging_cost
		           ))
		           FROM order_items oi
		           JOIN products p ON p.id = oi.product_id
		           WHERE oi.order_id = o.id
		       ), 0) AS shopping_cost
		FROM orders o
		WHERE o.status IN ('accepted', 'confirmed', 'in_production', 'ready', 'delivering')
		  AND o.event_date >= $1::date - interval '1 day'
	`, todayKey, onePlusWaste)
	if err != nil {
		return nil, err
	}
	defer ordRows.Close()

	for ordRows.Next() {
		var oid, status string
		var eventDate time.Time
		var total, dpReq, paid, shoppingCost decimal.Decimal
		var dpDeadline *time.Time
		if err := ordRows.Scan(&oid, &status, &eventDate, &total, &dpReq, &dpDeadline, &paid, &shoppingCost); err != nil {
			return nil, err
		}

		// DP yang belum terbayar masuk pada tanggal dp_deadline (atau hari ini bila sudah lewat)
		unpaidDP := dpReq.Sub(paid)
		if unpaidDP.IsPositive() {
			dpDate := today
			if dpDeadline != nil {
				dl := time.Date(dpDeadline.Year(), dpDeadline.Month(), dpDeadline.Day(), 0, 0, 0, 0, time.UTC)
				if dl.After(today) {
					dpDate = dl
				}
			}
			events = append(events, engine.CashEvent{Date: dpDate, Amount: unpaidDP})
		}

		// Pelunasan masuk pada hari setelah tanggal acara (Bagian 7)
		expectedPaid := paid
		if dpReq.GreaterThan(expectedPaid) {
			expectedPaid = dpReq
		}
		remaining := total.Sub(expectedPaid)
		if remaining.IsPositive() {
			settleDate := eventDate.AddDate(0, 0, 1)
			events = append(events, engine.CashEvent{Date: settleDate, Amount: remaining})
		}

		// Belanja bahan keluar pada tanggal acara - shopping_lead_days untuk pesanan yang belum diproduksi
		if (status == StatusAccepted || status == StatusConfirmed) && shoppingCost.IsPositive() {
			shopDate := eventDate.AddDate(0, 0, -params.EvalParams.ShoppingLeadDays)
			if shopDate.Before(today) {
				shopDate = today
			}
			events = append(events, engine.CashEvent{Date: shopDate, Amount: shoppingCost.Neg()})
		}
	}
	return events, ordRows.Err()
}

// GateDTO adalah format JSON gerbang evaluasi sesuai Bagian 9 brief.
type GateDTO struct {
	Gate      string          `json:"gate"`
	Passed    bool            `json:"passed"`
	Value     decimal.Decimal `json:"value"`
	Threshold decimal.Decimal `json:"threshold"`
}

// ImpactDTO adalah format JSON pratinjau dampak sesuai Bagian 9 brief.
type ImpactDTO struct {
	UtilizationBefore decimal.Decimal `json:"utilization_before"`
	UtilizationAfter  decimal.Decimal `json:"utilization_after"`
	Margin            decimal.Decimal `json:"margin"`
	CashLowestBefore  decimal.Decimal `json:"cash_lowest_before"`
	CashLowestAfter   decimal.Decimal `json:"cash_lowest_after"`
}

// CounterOfferDTO adalah format JSON tawaran balik sesuai Bagian 9 brief.
type CounterOfferDTO struct {
	Date             string                     `json:"date"`
	MinPricePerItem  map[string]decimal.Decimal `json:"min_price_per_item"`
	DP               decimal.Decimal            `json:"dp"`
	UtilizationAfter decimal.Decimal            `json:"utilization_after,omitempty"`
}

// EvaluationDTO adalah format JSON hasil EvaluateOrder sesuai Bagian 9 brief.
type EvaluationDTO struct {
	Recommendation string           `json:"recommendation"`
	Reason         string           `json:"reason"`
	Gates          []GateDTO        `json:"gates"`
	Impact         ImpactDTO        `json:"impact"`
	CounterOffer   *CounterOfferDTO `json:"counter_offer"`
}

// ToEvaluationDTO mengonversi engine.EvalResult ke kontrak JSON Bagian 9.
func ToEvaluationDTO(r engine.EvalResult) EvaluationDTO {
	gates := make([]GateDTO, len(r.Gates))
	for i, g := range r.Gates {
		gates[i] = GateDTO{
			Gate:      g.Gate,
			Passed:    g.Passed,
			Value:     g.Value.Round(4),
			Threshold: g.Threshold.Round(4),
		}
	}
	var co *CounterOfferDTO
	if r.CounterOffer != nil {
		minPrices := make(map[string]decimal.Decimal, len(r.CounterOffer.MinPricePerItem))
		for k, v := range r.CounterOffer.MinPricePerItem {
			minPrices[k] = v.Round(2)
		}
		co = &CounterOfferDTO{
			Date:             engine.DateKey(r.CounterOffer.Date),
			MinPricePerItem:  minPrices,
			DP:               r.CounterOffer.DP.Round(2),
			UtilizationAfter: r.CounterOffer.UtilizationAfter.Round(4),
		}
	}
	return EvaluationDTO{
		Recommendation: r.Recommendation,
		Reason:         r.Reason,
		Gates:          gates,
		Impact: ImpactDTO{
			UtilizationBefore: r.Impact.UtilizationBefore.Round(4),
			UtilizationAfter:  r.Impact.UtilizationAfter.Round(4),
			Margin:            r.Impact.Margin.Round(4),
			CashLowestBefore:  r.Impact.CashLowestBefore.Round(2),
			CashLowestAfter:   r.Impact.CashLowestAfter.Round(2),
		},
		CounterOffer: co,
	}
}

// OrderItemInput adalah masukan satu item saat membuat pesanan atau mengecek ketersediaan.
type OrderItemInput struct {
	ProductID string           `json:"product_id"`
	Qty       int              `json:"qty"`
	UnitPrice *decimal.Decimal `json:"unit_price,omitempty"` // opsional untuk pesanan manual
}

// AvailabilityDay adalah elemen hasil POST /public/availability (Bagian 9).
type AvailabilityDay struct {
	Date             string          `json:"date"`
	Status           string          `json:"status"`
	UtilizationAfter decimal.Decimal `json:"utilization_after"`
	Available        bool            `json:"available"`
}

// CheckAvailability menghitung status dan ketersediaan hari pada rentang [from, to]
// berdasarkan G1 (utilisasi <= capacity_limit) dan G4 (lead time >= lead_time_days)
// menggunakan internal/engine.
func CheckAvailability(ctx context.Context, q db.DBTX, items []OrderItemInput, fromDate, toDate time.Time) ([]AvailabilityDay, error) {
	if _, err := ExpireOverdueOrders(ctx, q, time.Now().UTC()); err != nil {
		return nil, err
	}
	params, err := LoadSettingsParams(ctx, q)
	if err != nil {
		return nil, err
	}
	today := db.TodayQ(ctx, q)

	pids := make([]string, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		if !seen[it.ProductID] {
			seen[it.ProductID] = true
			pids = append(pids, it.ProductID)
		}
	}

	prodMeta, err := LoadProductsMeta(ctx, q, pids, params)
	if err != nil {
		return nil, err
	}
	caps, err := LoadCapacities(ctx, q)
	if err != nil {
		return nil, err
	}
	existingMap, err := LoadExistingLoads(ctx, q, fromDate, toDate)
	if err != nil {
		return nil, err
	}

	// Beban tambahan dari pesanan yang sedang dicek
	orderLoadItems := make([]engine.LoadItem, 0, len(items)*2)
	for _, it := range items {
		pm := prodMeta[it.ProductID]
		qtyDec := decimal.NewFromInt(int64(it.Qty))
		for code, ld := range pm.UnitLoads {
			orderLoadItems = append(orderLoadItems, engine.LoadItem{
				ResourceCode: code,
				Qty:          qtyDec,
				Load:         ld,
			})
		}
	}

	todayCivil := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	cur := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, time.UTC)

	out := make([]AvailabilityDay, 0)
	for !cur.After(end) {
		dk := engine.DateKey(cur)
		allLoads := make([]engine.LoadItem, 0, len(orderLoadItems)+2)
		for code, ld := range existingMap[dk] {
			allLoads = append(allLoads, engine.LoadItem{
				ResourceCode: code,
				Qty:          decimal.NewFromInt(1),
				Load:         ld,
			})
		}
		allLoads = append(allLoads, orderLoadItems...)

		dayLoadMap := engine.DayLoads(allLoads)
		utilAfter, _ := engine.DayUtilization(dayLoadMap, caps)
		dayStatus := engine.DayStatus(utilAfter, params.StatusGreenBelow, params.StatusYellowBelow)

		daysAhead := int(cur.Sub(todayCivil).Hours() / 24)
		g1Passed := utilAfter.LessThanOrEqual(params.EvalParams.CapacityLimit)
		g4Passed := daysAhead >= params.EvalParams.LeadTimeDays

		out = append(out, AvailabilityDay{
			Date:             dk,
			Status:           dayStatus,
			UtilizationAfter: utilAfter.Round(4),
			Available:        g1Passed && g4Passed,
		})
		cur = cur.AddDate(0, 0, 1)
	}

	return out, nil
}

// CreateOrderInput adalah parameter pembuatan pesanan (publik maupun manual).
type CreateOrderInput struct {
	Source          string // "public" | "manual"
	CustomerName    string
	CustomerContact string
	DeliveryType    string // "delivery" | "pickup"
	Address         string
	Notes           string
	EventDate       time.Time
	ProductionDate  time.Time // bila zero, disamakan dengan EventDate
	Items           []OrderItemInput
	CreatedBy       string // UUID pengguna untuk pesanan manual; kosong untuk publik
}

// OrderItemDetail adalah rincian item pesanan yang disimpan.
type OrderItemDetail struct {
	ID          string          `json:"id"`
	ProductID   string          `json:"product_id"`
	ProductName string          `json:"product_name"`
	Qty         int             `json:"qty"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	UnitCost    decimal.Decimal `json:"unit_cost,omitempty"`
	Subtotal    decimal.Decimal `json:"subtotal"`
}

// OrderDetail adalah representasi lengkap pesanan setelah dibuat.
type OrderDetail struct {
	ID                   string           `json:"id"`
	Code                 string           `json:"code"`
	Source               string           `json:"source"`
	CustomerName         string           `json:"customer_name"`
	CustomerContact      string           `json:"customer_contact"`
	DeliveryType         string           `json:"delivery_type"`
	Address              string           `json:"address,omitempty"`
	Notes                string           `json:"notes,omitempty"`
	EventDate            string           `json:"event_date"`
	ProductionDate       string           `json:"production_date"`
	Status               string           `json:"status"`
	Total                decimal.Decimal  `json:"total"`
	DPRequired           decimal.Decimal  `json:"dp_required"`
	DPDeadline           *time.Time       `json:"dp_deadline"`
	Paid                 decimal.Decimal  `json:"paid"`
	Recommendation       string           `json:"recommendation"`
	RecommendationReason string           `json:"recommendation_reason"`
	CounterOffer         *CounterOfferDTO `json:"counter_offer"`
	Evaluation           EvaluationDTO    `json:"evaluation"`
	Items                []OrderItemDetail `json:"items"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

// EvaluateOrder di lapisan layanan merangkai EvalInput dari database dan
// memanggil engine.EvaluateOrder (Bagian 7 & 8).
func EvaluateOrder(
	ctx context.Context,
	q db.DBTX,
	today, eventDate, prodDate time.Time,
	evalItems []engine.EvalItem,
	params SettingsParams,
) (engine.EvalResult, error) {
	caps, err := LoadCapacities(ctx, q)
	if err != nil {
		return engine.EvalResult{}, err
	}

	minDate := prodDate
	if today.Before(minDate) {
		minDate = today
	}
	maxDate := eventDate.AddDate(0, 0, 16)
	if leadEnd := today.AddDate(0, 0, params.EvalParams.LeadTimeDays+16); leadEnd.After(maxDate) {
		maxDate = leadEnd
	}

	existingLoads, err := LoadExistingLoads(ctx, q, minDate, maxDate)
	if err != nil {
		return engine.EvalResult{}, err
	}
	cashEvents, err := LoadCashEvents(ctx, q, today, params)
	if err != nil {
		return engine.EvalResult{}, err
	}

	in := engine.EvalInput{
		Today:          today,
		EventDate:      eventDate,
		ProductionDate: prodDate,
		DPDate:         today,
		Items:          evalItems,
		Params:         params.EvalParams,
		Capacities:     caps,
		ExistingLoads:  existingLoads,
		CashEvents:     cashEvents,
	}
	return engine.EvaluateOrder(in), nil
}

// CreateOrder menjalankan alur Bagian 8.1 dalam satu transaksi database:
//  1. Ambil harga & biaya produk terkini; hitung total & unit.
//  2. Kunci hari di day_locks (INSERT ON CONFLICT DO NOTHING + SELECT FOR UPDATE, terurut).
//  3. Muat beban existing, kapasitas sumber daya, dan peristiwa kas.
//  4. Jalankan EvaluateOrder → simpan 4 baris order_evaluations, snapshot,
//     recommendation, recommendation_reason, dan counter_offer.
//  5. Tentukan status awal (accepted vs pending_decision) beserta dp_required & dp_deadline.
//  6. Simpan item pesanan, audit log, dan commit.
func CreateOrder(ctx context.Context, conn db.Conn, req CreateOrderInput) (*OrderDetail, error) {
	if req.ProductionDate.IsZero() {
		req.ProductionDate = req.EventDate
	}

	var out *OrderDetail
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		params, err := LoadSettingsParams(ctx, tx)
		if err != nil {
			return err
		}
		today := db.TodayQ(ctx, tx)

		// Kumpulkan ID produk
		pids := make([]string, 0, len(req.Items))
		seen := make(map[string]bool, len(req.Items))
		for _, it := range req.Items {
			if !seen[it.ProductID] {
				seen[it.ProductID] = true
				pids = append(pids, it.ProductID)
			}
		}
		prodMeta, err := LoadProductsMeta(ctx, tx, pids, params)
		if err != nil {
			return err
		}

		// Susun tanggal yang perlu dikunci di day_locks (tanggal produksi + 14 hari kandidat tawaran balik)
		lockDates := make([]time.Time, 0, 16)
		lockDates = append(lockDates, req.ProductionDate, req.EventDate)
		for i := 1; i <= 15; i++ {
			lockDates = append(lockDates, req.ProductionDate.AddDate(0, 0, i))
		}
		if err := LockDays(ctx, tx, lockDates); err != nil {
			return err
		}
		if _, err := ExpireOverdueOrders(ctx, tx, time.Now().UTC()); err != nil {
			return err
		}

		// Bangun EvalItem untuk engine.EvaluateOrder
		evalItems := make([]engine.EvalItem, 0, len(req.Items))
		itemDetails := make([]OrderItemDetail, 0, len(req.Items))
		totalUnits := 0
		for _, it := range req.Items {
			pm := prodMeta[it.ProductID]
			unitPrice := pm.DefaultPrice
			if req.Source == "manual" && it.UnitPrice != nil {
				unitPrice = *it.UnitPrice
			}
			evalItems = append(evalItems, engine.EvalItem{
				ProductID:           it.ProductID,
				Qty:                 it.Qty,
				UnitPrice:           unitPrice,
				UnitCost:            pm.UnitCost,
				PurchaseCostPerUnit: pm.PurchaseCostPerUnit,
				UnitLoads:           pm.UnitLoads,
			})
			itemDetails = append(itemDetails, OrderItemDetail{
				ProductID:   it.ProductID,
				ProductName: pm.Name,
				Qty:         it.Qty,
				UnitPrice:   unitPrice,
				UnitCost:    pm.UnitCost,
				Subtotal:    unitPrice.Mul(decimal.NewFromInt(int64(it.Qty))),
			})
			totalUnits += it.Qty
		}

		evalRes, err := EvaluateOrder(ctx, tx, today, req.EventDate, req.ProductionDate, evalItems, params)
		if err != nil {
			return err
		}
		evalDTO := ToEvaluationDTO(evalRes)

		// Bagian 8.1 butir 5: Tentukan status awal
		allGatesPassed := evalRes.Recommendation == engine.RecAccept
		underBigOrder := evalRes.Total.LessThan(params.BigOrderTotal) && totalUnits < params.BigOrderUnits

		status := StatusPendingDecision
		dpRequired := evalRes.DPCharged.Round(4)
		var dpDeadline *time.Time
		if allGatesPassed && underBigOrder {
			status = StatusAccepted
			dl := time.Now().UTC().Add(time.Duration(params.DPDeadlineHours) * time.Hour)
			dpDeadline = &dl
		}

		// Siapkan JSON counter_offer dan snapshot
		var counterJSON any
		if evalDTO.CounterOffer != nil {
			b, err := json.Marshal(evalDTO.CounterOffer)
			if err != nil {
				return err
			}
			counterJSON = string(b)
		}

		snapshotPayload := map[string]any{
			"evaluated_at": time.Now().UTC().Format(time.RFC3339),
			"today":        engine.DateKey(today),
			"evaluation":   evalDTO,
			"params": map[string]any{
				"capacity_limit":    params.EvalParams.CapacityLimit,
				"margin_min":        params.EvalParams.MarginMin,
				"cash_buffer":       params.EvalParams.CashBuffer,
				"fixed_cost_daily":  params.EvalParams.FixedCostDaily,
				"opening_cash":      params.EvalParams.OpeningCash,
				"dp_pct":            params.EvalParams.DPPct,
				"lead_time_days":    params.EvalParams.LeadTimeDays,
				"big_order_total":   params.BigOrderTotal,
				"big_order_units":   params.BigOrderUnits,
				"dp_deadline_hours": params.DPDeadlineHours,
			},
		}
		snapBytes, err := json.Marshal(snapshotPayload)
		if err != nil {
			return err
		}

		// Buat kode unik 10 karakter (ulangi bila kebetulan tabrakan)
		var code string
		for attempt := 0; attempt < 5; attempt++ {
			c, err := GenerateCode()
			if err != nil {
				return err
			}
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE code = $1)`, c).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				code = c
				break
			}
		}
		if code == "" {
			return fmt.Errorf("gagal menghasilkan kode pesanan unik")
		}

		var createdBy any
		if req.CreatedBy != "" {
			createdBy = req.CreatedBy
		}
		var addrPtr, notesPtr any
		if req.Address != "" {
			addrPtr = req.Address
		}
		if req.Notes != "" {
			notesPtr = req.Notes
		}

		eventDateKey := engine.DateKey(req.EventDate)
		prodDateKey := engine.DateKey(req.ProductionDate)

		var orderID string
		var createdAt, updatedAt time.Time
		err = tx.QueryRow(ctx, `
			INSERT INTO orders (
				code, source, customer_name, customer_contact, delivery_type,
				address, notes, event_date, production_date, status,
				total, dp_required, dp_deadline, paid,
				recommendation, recommendation_reason, counter_offer, snapshot, created_by
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8::date, $9::date, $10,
				$11, $12, $13, 0,
				$14, $15, $16::jsonb, $17::jsonb, $18::uuid
			)
			RETURNING id::text, created_at, updated_at
		`,
			code, req.Source, req.CustomerName, req.CustomerContact, req.DeliveryType,
			addrPtr, notesPtr, eventDateKey, prodDateKey, status,
			evalRes.Total.Round(4), dpRequired, dpDeadline,
			evalRes.Recommendation, evalRes.Reason, counterJSON, string(snapBytes), createdBy,
		).Scan(&orderID, &createdAt, &updatedAt)
		if err != nil {
			return err
		}

		// Simpan order_items
		for i := range itemDetails {
			it := &itemDetails[i]
			err := tx.QueryRow(ctx, `
				INSERT INTO order_items (order_id, product_id, qty, unit_price, unit_cost)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5)
				RETURNING id::text
			`, orderID, it.ProductID, it.Qty, it.UnitPrice.Round(4), it.UnitCost.Round(4)).Scan(&it.ID)
			if err != nil {
				return err
			}
		}

		// Simpan 4 baris order_evaluations (G1..G4)
		for _, g := range evalRes.Gates {
			detailBytes, _ := json.Marshal(map[string]any{
				"bottleneck": evalRes.Bottleneck,
			})
			_, err := tx.Exec(ctx, `
				INSERT INTO order_evaluations (order_id, gate, value, threshold, passed, detail)
				VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb)
			`, orderID, g.Gate, g.Value.Round(6), g.Threshold.Round(6), g.Passed, string(detailBytes))
			if err != nil {
				return err
			}
		}

		// Catat audit log
		if err := audit.Log(ctx, tx, req.CreatedBy, audit.ActionDecision, "orders", orderID, map[string]any{
			"op":             "create_order",
			"code":           code,
			"source":         req.Source,
			"status":         status,
			"recommendation": evalRes.Recommendation,
			"total":          evalRes.Total.Round(2),
			"dp_required":    dpRequired,
		}); err != nil {
			return err
		}

		out = &OrderDetail{
			ID:                   orderID,
			Code:                 code,
			Source:               req.Source,
			CustomerName:         req.CustomerName,
			CustomerContact:      req.CustomerContact,
			DeliveryType:         req.DeliveryType,
			Address:              req.Address,
			Notes:                req.Notes,
			EventDate:            eventDateKey,
			ProductionDate:       prodDateKey,
			Status:               status,
			Total:                evalRes.Total.Round(2),
			DPRequired:           dpRequired.Round(2),
			DPDeadline:           dpDeadline,
			Paid:                 decimal.Zero,
			Recommendation:       evalRes.Recommendation,
			RecommendationReason: evalRes.Reason,
			CounterOffer:         evalDTO.CounterOffer,
			Evaluation:           evalDTO,
			Items:                itemDetails,
			CreatedAt:            createdAt,
			UpdatedAt:            updatedAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

