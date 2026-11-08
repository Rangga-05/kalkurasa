package orders

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/httpx"
)

// ShoppingListItem merepresentasikan kebutuhan belanja satu bahan baku pada rentang tanggal.
// Untuk peran staff, field BuyPrice dan EstimatedCost bernilai nil (omitempty).
type ShoppingListItem struct {
	IngredientID      string           `json:"ingredient_id"`
	IngredientName    string           `json:"ingredient_name"`
	UseUnit           string           `json:"use_unit"`
	BuyUnit           string           `json:"buy_unit"`
	BuyUnitSize       decimal.Decimal  `json:"buy_unit_size"`
	Rounding          string           `json:"rounding"`
	RequiredQty       decimal.Decimal  `json:"required_qty"`
	BufferPct         decimal.Decimal  `json:"buffer_pct"`
	Stock             decimal.Decimal  `json:"stock"`
	MinStock          decimal.Decimal  `json:"min_stock"`
	BelowMinStock     bool             `json:"below_min_stock"`
	NeedQty           decimal.Decimal  `json:"need_qty"`
	BuyUseUnits       decimal.Decimal  `json:"buy_use_units"`
	QtyBuyUnits       decimal.Decimal  `json:"qty_buy_units"`
	BuyPrice          *decimal.Decimal `json:"buy_price,omitempty"`
	EstimatedCost     *decimal.Decimal `json:"estimated_cost,omitempty"`
	EarliestEventDate string           `json:"earliest_event_date,omitempty"`
	PurchaseDate      string           `json:"purchase_date,omitempty"`
	OrderCount        int              `json:"order_count"`
}

// LowStockAlertItem merepresentasikan peringatan bahan baku yang stoknya di bawah minimum.
type LowStockAlertItem struct {
	IngredientID   string           `json:"ingredient_id"`
	IngredientName string           `json:"ingredient_name"`
	UseUnit        string           `json:"use_unit"`
	BuyUnit        string           `json:"buy_unit"`
	BuyUnitSize    decimal.Decimal  `json:"buy_unit_size"`
	Rounding       string           `json:"rounding"`
	Stock          decimal.Decimal  `json:"stock"`
	MinStock       decimal.Decimal  `json:"min_stock"`
	Deficit        decimal.Decimal  `json:"deficit"`
	BuyPrice       *decimal.Decimal `json:"buy_price,omitempty"`
}

// ShoppingListResult adalah respons GET /api/v1/shopping-list.
type ShoppingListResult struct {
	From               string              `json:"from"`
	To                 string              `json:"to"`
	BufferPct          decimal.Decimal     `json:"buffer_pct"`
	ShoppingLeadDays   int                 `json:"shopping_lead_days"`
	TotalEstimatedCost *decimal.Decimal    `json:"total_estimated_cost,omitempty"`
	Items              []ShoppingListItem  `json:"items"`
	LowStockItems      []LowStockAlertItem `json:"low_stock_items"`
}

// GetShoppingList menghitung daftar kebutuhan belanja bahan dari pesanan berstatus
// confirmed dan in_production pada rentang tanggal acara [fromDate, toDate]
// menggunakan engine.ShoppingNeed.
func GetShoppingList(ctx context.Context, q db.DBTX, fromDate, toDate time.Time, actorRole string) (*ShoppingListResult, error) {
	if _, err := ExpireOverdueOrders(ctx, q, time.Now().UTC()); err != nil {
		return nil, err
	}
	params, err := LoadSettingsParams(ctx, q)
	if err != nil {
		return nil, err
	}

	fromKey := engine.DateKey(fromDate)
	toKey := engine.DateKey(toDate)

	// 1. Muat seluruh bahan aktif
	ingRows, err := q.Query(ctx, `
		SELECT id::text, name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active
		FROM ingredients
		WHERE is_active = true
		ORDER BY name ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer ingRows.Close()

	type ingMeta struct {
		ID          string
		Name        string
		UseUnit     string
		BuyUnit     string
		BuyUnitSize decimal.Decimal
		BuyPrice    decimal.Decimal
		Stock       decimal.Decimal
		MinStock    decimal.Decimal
		Rounding    string
		IsActive    bool
	}
	ings := make([]ingMeta, 0)
	ingByID := make(map[string]ingMeta)
	for ingRows.Next() {
		var im ingMeta
		if err := ingRows.Scan(
			&im.ID, &im.Name, &im.UseUnit, &im.BuyUnit,
			&im.BuyUnitSize, &im.BuyPrice, &im.Stock, &im.MinStock,
			&im.Rounding, &im.IsActive,
		); err != nil {
			return nil, err
		}
		ings = append(ings, im)
		ingByID[im.ID] = im
	}
	if err := ingRows.Err(); err != nil {
		return nil, err
	}
	ingRows.Close()

	// 2. Hitung kebutuhan bahan dari pesanan confirmed & in_production pada rentang [fromDate, toDate]
	reqRows, err := q.Query(ctx, `
		SELECT ri.ingredient_id::text,
		       COALESCE(SUM(oi.qty * ri.qty_per_unit), 0) AS req_qty,
		       MIN(o.event_date) AS min_event_date,
		       COUNT(DISTINCT o.id) AS order_count
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		JOIN recipe_items ri ON ri.product_id = oi.product_id
		WHERE o.status IN ('confirmed', 'in_production')
		  AND o.event_date >= $1::date
		  AND o.event_date <= $2::date
		GROUP BY ri.ingredient_id
	`, fromKey, toKey)
	if err != nil {
		return nil, err
	}
	defer reqRows.Close()

	type reqInfo struct {
		ReqQty       decimal.Decimal
		MinEventDate time.Time
		OrderCount   int
	}
	reqs := make(map[string]reqInfo)
	for reqRows.Next() {
		var iid string
		var rq reqInfo
		if err := reqRows.Scan(&iid, &rq.ReqQty, &rq.MinEventDate, &rq.OrderCount); err != nil {
			return nil, err
		}
		reqs[iid] = rq
	}
	if err := reqRows.Err(); err != nil {
		return nil, err
	}
	reqRows.Close()

	isStaff := actorRole == auth.RoleStaff
	items := make([]ShoppingListItem, 0)
	lowStock := make([]LowStockAlertItem, 0)
	totalEstCost := decimal.Zero

	for _, im := range ings {
		belowMin := im.Stock.LessThan(im.MinStock)
		if belowMin {
			var bpPtr *decimal.Decimal
			if !isStaff {
				bp := im.BuyPrice.Round(2)
				bpPtr = &bp
			}
			lowStock = append(lowStock, LowStockAlertItem{
				IngredientID:   im.ID,
				IngredientName: im.Name,
				UseUnit:        im.UseUnit,
				BuyUnit:        im.BuyUnit,
				BuyUnitSize:    im.BuyUnitSize.Round(4),
				Rounding:       im.Rounding,
				Stock:          im.Stock.Round(4),
				MinStock:       im.MinStock.Round(4),
				Deficit:        im.MinStock.Sub(im.Stock).Round(4),
				BuyPrice:       bpPtr,
			})
		}

		rq, hasReq := reqs[im.ID]
		if !hasReq || !rq.ReqQty.IsPositive() {
			continue
		}

		need, buyUse := engine.ShoppingNeed(
			rq.ReqQty,
			im.Stock,
			params.ShoppingBufferPct,
			im.BuyUnitSize,
			im.Rounding,
		)

		qtyBuyUnits := decimal.Zero
		if im.BuyUnitSize.IsPositive() && buyUse.IsPositive() {
			qtyBuyUnits = buyUse.DivRound(im.BuyUnitSize, 4)
		}

		estCost := decimal.Zero
		if im.BuyUnitSize.IsPositive() && buyUse.IsPositive() {
			estCost = buyUse.Mul(im.BuyPrice).DivRound(im.BuyUnitSize, 2)
		}
		totalEstCost = totalEstCost.Add(estCost)

		earliestKey := engine.DateKey(rq.MinEventDate)
		purchaseDate := rq.MinEventDate.AddDate(0, 0, -params.EvalParams.ShoppingLeadDays)
		purchaseKey := engine.DateKey(purchaseDate)

		var bpPtr, estPtr *decimal.Decimal
		if !isStaff {
			bp := im.BuyPrice.Round(2)
			ec := estCost.Round(2)
			bpPtr = &bp
			estPtr = &ec
		}

		items = append(items, ShoppingListItem{
			IngredientID:      im.ID,
			IngredientName:    im.Name,
			UseUnit:           im.UseUnit,
			BuyUnit:           im.BuyUnit,
			BuyUnitSize:       im.BuyUnitSize.Round(4),
			Rounding:          im.Rounding,
			RequiredQty:       rq.ReqQty.Round(4),
			BufferPct:         params.ShoppingBufferPct.Round(4),
			Stock:             im.Stock.Round(4),
			MinStock:          im.MinStock.Round(4),
			BelowMinStock:     belowMin,
			NeedQty:           need.Round(4),
			BuyUseUnits:       buyUse.Round(4),
			QtyBuyUnits:       qtyBuyUnits,
			BuyPrice:          bpPtr,
			EstimatedCost:     estPtr,
			EarliestEventDate: earliestKey,
			PurchaseDate:      purchaseKey,
			OrderCount:        rq.OrderCount,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].PurchaseDate != items[j].PurchaseDate {
			return items[i].PurchaseDate < items[j].PurchaseDate
		}
		return items[i].IngredientName < items[j].IngredientName
	})

	var totalCostPtr *decimal.Decimal
	if !isStaff {
		tc := totalEstCost.Round(2)
		totalCostPtr = &tc
	}

	return &ShoppingListResult{
		From:               fromKey,
		To:                 toKey,
		BufferPct:          params.ShoppingBufferPct.Round(4),
		ShoppingLeadDays:   params.EvalParams.ShoppingLeadDays,
		TotalEstimatedCost: totalCostPtr,
		Items:              items,
		LowStockItems:      lowStock,
	}, nil
}

// PurchaseInput adalah masukan untuk POST /api/v1/purchases.
type PurchaseInput struct {
	IngredientID string
	QtyBuyUnits  decimal.Decimal
	UnitPrice    *decimal.Decimal // bila nil, gunakan buy_price bahan saat ini
	ActorID      string
	ActorRole    string
	Note         string
}

// PurchaseResult adalah respons POST /api/v1/purchases.
type PurchaseResult struct {
	ID             string           `json:"id"`
	IngredientID   string           `json:"ingredient_id"`
	IngredientName string           `json:"ingredient_name"`
	UseUnit        string           `json:"use_unit"`
	BuyUnit        string           `json:"buy_unit"`
	BuyUnitSize    decimal.Decimal  `json:"buy_unit_size"`
	QtyBuyUnits    decimal.Decimal  `json:"qty_buy_units"`
	DeltaUseUnits  decimal.Decimal  `json:"delta_use_units"`
	UnitPrice      *decimal.Decimal `json:"unit_price,omitempty"`
	TotalCost      *decimal.Decimal `json:"total_cost,omitempty"`
	StockBefore    decimal.Decimal  `json:"stock_before"`
	StockAfter     decimal.Decimal  `json:"stock_after"`
	BelowMinStock  bool             `json:"below_min_stock"`
	MovementID     string           `json:"movement_id"`
	CashEntryID    string           `json:"cash_entry_id,omitempty"`
	BoughtAt       time.Time        `json:"bought_at"`
}

// RecordPurchase mencatat pembelian bahan baku dalam satu transaksi:
//  1. Menambah stok bahan (qty_buy_units * buy_unit_size) dan memperbarui buy_price aktual.
//  2. Menulis baris purchases.
//  3. Menulis baris stock_movements (reason = 'purchase', ref_type = 'purchase').
//  4. Menulis baris cash_entries negatif sebesar total biaya pembelian pada tanggal hari ini.
//  5. Menulis audit_logs.
func RecordPurchase(ctx context.Context, conn db.Conn, req PurchaseInput) (*PurchaseResult, error) {
	if !req.QtyBuyUnits.IsPositive() {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Jumlah beli (qty_buy_units) harus lebih besar dari 0.")
	}
	if req.UnitPrice != nil && req.UnitPrice.IsNegative() {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Harga beli per satuan (unit_price) tidak boleh negatif.")
	}

	var out *PurchaseResult
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		now := time.Now().UTC()
		today := db.TodayQ(ctx, tx)

		var (
			ingID, name, useUnit, buyUnit, rounding string
			buyUnitSize, curBuyPrice, stock, minSt  decimal.Decimal
			isActive                                bool
		)
		err := tx.QueryRow(ctx, `
			SELECT id::text, name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active
			FROM ingredients
			WHERE id = $1::uuid
			FOR UPDATE
		`, req.IngredientID).Scan(
			&ingID, &name, &useUnit, &buyUnit, &buyUnitSize, &curBuyPrice, &stock, &minSt, &rounding, &isActive,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Bahan baku tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		actualUnitPrice := curBuyPrice
		if req.UnitPrice != nil {
			actualUnitPrice = *req.UnitPrice
		}

		qtyBuy := req.QtyBuyUnits.Round(4)
		deltaUse := qtyBuy.Mul(buyUnitSize).Round(4)
		newStock := stock.Add(deltaUse).Round(4)
		totalCost := qtyBuy.Mul(actualUnitPrice).Round(4)

		var actorPtr any
		if req.ActorID != "" {
			actorPtr = req.ActorID
		}

		// 1. Simpan ke tabel purchases
		var purchaseID string
		err = tx.QueryRow(ctx, `
			INSERT INTO purchases (ingredient_id, qty_buy_units, unit_price, bought_at, bought_by)
			VALUES ($1::uuid, $2, $3, $4, $5::uuid)
			RETURNING id::text
		`, ingID, qtyBuy, actualUnitPrice.Round(4), now, actorPtr).Scan(&purchaseID)
		if err != nil {
			return err
		}

		// 2. Perbarui stok dan harga beli aktual di tabel ingredients
		_, err = tx.Exec(ctx, `
			UPDATE ingredients
			SET stock = $1, buy_price = $2
			WHERE id = $3::uuid
		`, newStock, actualUnitPrice.Round(4), ingID)
		if err != nil {
			return err
		}

		// 3. Catat mutasi stok di stock_movements
		var movementID string
		err = tx.QueryRow(ctx, `
			INSERT INTO stock_movements (ingredient_id, delta, reason, ref_type, ref_id, created_at)
			VALUES ($1::uuid, $2, 'purchase', 'purchase', $3::uuid, $4)
			RETURNING id::text
		`, ingID, deltaUse, purchaseID, now).Scan(&movementID)
		if err != nil {
			return err
		}

		// 4. Catat kas keluar di cash_entries (bila totalCost > 0)
		var cashEntryID string
		if totalCost.IsPositive() {
			cashNote := fmt.Sprintf("Pembelian bahan: %s (%s %s)", name, qtyBuy.StringFixedBank(2), buyUnit)
			if strings.TrimSpace(req.Note) != "" {
				cashNote += " — " + strings.TrimSpace(req.Note)
			}
			err = tx.QueryRow(ctx, `
				INSERT INTO cash_entries (entry_date, amount, note, created_by)
				VALUES ($1::date, $2, $3, $4::uuid)
				RETURNING id::text
			`, engine.DateKey(today), totalCost.Neg().Round(4), cashNote, actorPtr).Scan(&cashEntryID)
			if err != nil {
				return err
			}
		}

		// 5. Catat audit_logs
		if err := audit.Log(ctx, tx, req.ActorID, audit.ActionMaster, "purchases", purchaseID, map[string]any{
			"op":              "record_purchase",
			"ingredient_id":   ingID,
			"ingredient_name": name,
			"qty_buy_units":   qtyBuy,
			"delta_use_units": deltaUse,
			"unit_price":      actualUnitPrice.Round(2),
			"total_cost":      totalCost.Round(2),
			"stock_before":    stock.Round(4),
			"stock_after":     newStock.Round(4),
			"cash_entry_id":   cashEntryID,
		}); err != nil {
			return err
		}

		isStaff := req.ActorRole == auth.RoleStaff
		var upPtr, tcPtr *decimal.Decimal
		if !isStaff {
			up := actualUnitPrice.Round(2)
			tc := totalCost.Round(2)
			upPtr = &up
			tcPtr = &tc
		}
		ceID := cashEntryID
		if isStaff {
			ceID = ""
		}

		out = &PurchaseResult{
			ID:             purchaseID,
			IngredientID:   ingID,
			IngredientName: name,
			UseUnit:        useUnit,
			BuyUnit:        buyUnit,
			BuyUnitSize:    buyUnitSize.Round(4),
			QtyBuyUnits:    qtyBuy,
			DeltaUseUnits:  deltaUse,
			UnitPrice:      upPtr,
			TotalCost:      tcPtr,
			StockBefore:    stock.Round(4),
			StockAfter:     newStock,
			BelowMinStock:  newStock.LessThan(minSt),
			MovementID:     movementID,
			CashEntryID:    ceID,
			BoughtAt:       now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StockAdjustInput adalah masukan untuk POST /api/v1/stock/adjust.
type StockAdjustInput struct {
	IngredientID string
	Delta        *decimal.Decimal // selisih perubahan dalam use_unit
	NewStock     *decimal.Decimal // alternatif: nilai stok akhir baru
	Reason       string
	ActorID      string
	ActorRole    string
}

// StockAdjustResult adalah respons POST /api/v1/stock/adjust.
type StockAdjustResult struct {
	MovementID     string          `json:"movement_id"`
	IngredientID   string          `json:"ingredient_id"`
	IngredientName string          `json:"ingredient_name"`
	UseUnit        string          `json:"use_unit"`
	Delta          decimal.Decimal `json:"delta"`
	StockBefore    decimal.Decimal `json:"stock_before"`
	StockAfter     decimal.Decimal `json:"stock_after"`
	MinStock       decimal.Decimal `json:"min_stock"`
	BelowMinStock  bool            `json:"below_min_stock"`
	Reason         string          `json:"reason"`
	CreatedAt      time.Time       `json:"created_at"`
}

// AdjustStock melakukan koreksi stok bahan baku dengan alasan wajib (POST /api/v1/stock/adjust).
func AdjustStock(ctx context.Context, conn db.Conn, req StockAdjustInput) (*StockAdjustResult, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Alasan koreksi stok (reason) wajib diisi.")
	}
	if len(reason) > 300 {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Alasan koreksi stok maksimal 300 karakter.")
	}
	if req.Delta == nil && req.NewStock == nil {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Perubahan stok (delta) atau stok akhir (new_stock) wajib diisi.")
	}

	var out *StockAdjustResult
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		now := time.Now().UTC()

		var (
			ingID, name, useUnit, buyUnit, rounding string
			buyUnitSize, curBuyPrice, stock, minSt  decimal.Decimal
			isActive                                bool
		)
		err := tx.QueryRow(ctx, `
			SELECT id::text, name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active
			FROM ingredients
			WHERE id = $1::uuid
			FOR UPDATE
		`, req.IngredientID).Scan(
			&ingID, &name, &useUnit, &buyUnit, &buyUnitSize, &curBuyPrice, &stock, &minSt, &rounding, &isActive,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Bahan baku tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		var delta decimal.Decimal
		if req.Delta != nil {
			delta = req.Delta.Round(4)
		} else if req.NewStock != nil {
			delta = req.NewStock.Sub(stock).Round(4)
		}

		if delta.IsZero() {
			return httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Perubahan stok (delta) tidak boleh 0.")
		}

		newStock := stock.Add(delta).Round(4)
		if newStock.IsNegative() {
			return httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Stok akhir tidak boleh negatif.")
		}

		_, err = tx.Exec(ctx, `
			UPDATE ingredients
			SET stock = $1
			WHERE id = $2::uuid
		`, newStock, ingID)
		if err != nil {
			return err
		}

		var movementID string
		err = tx.QueryRow(ctx, `
			INSERT INTO stock_movements (ingredient_id, delta, reason, ref_type, ref_id, created_at)
			VALUES ($1::uuid, $2, $3, 'adjustment', NULL, $4)
			RETURNING id::text
		`, ingID, delta, reason, now).Scan(&movementID)
		if err != nil {
			return err
		}

		if err := audit.Log(ctx, tx, req.ActorID, audit.ActionMaster, "stock_movements", movementID, map[string]any{
			"op":              "stock_adjust",
			"ingredient_id":   ingID,
			"ingredient_name": name,
			"delta":           delta,
			"stock_before":    stock.Round(4),
			"stock_after":     newStock,
			"reason":          reason,
		}); err != nil {
			return err
		}

		out = &StockAdjustResult{
			MovementID:     movementID,
			IngredientID:   ingID,
			IngredientName: name,
			UseUnit:        useUnit,
			Delta:          delta,
			StockBefore:    stock.Round(4),
			StockAfter:     newStock,
			MinStock:       minSt.Round(4),
			BelowMinStock:  newStock.LessThan(minSt),
			Reason:         reason,
			CreatedAt:      now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CashProjectionDayDTO adalah rincian saldo kas harian untuk peran owner.
type CashProjectionDayDTO struct {
	Date        string          `json:"date"`
	Inflow      decimal.Decimal `json:"inflow"`
	Outflow     decimal.Decimal `json:"outflow"`
	NetEvents   decimal.Decimal `json:"net_events"`
	FixedCost   decimal.Decimal `json:"fixed_cost"`
	Balance     decimal.Decimal `json:"balance"`
	BelowBuffer bool            `json:"below_buffer"`
	IsLowest    bool            `json:"is_lowest"`
}

// CashEntryDTO merepresentasikan satu baris entri kas manual/pembelian dari tabel cash_entries.
type CashEntryDTO struct {
	ID        string          `json:"id"`
	EntryDate string          `json:"entry_date"`
	Amount    decimal.Decimal `json:"amount"`
	Note      string          `json:"note,omitempty"`
	CreatedBy *string         `json:"created_by,omitempty"`
}

// CashProjectionResult adalah respons GET /api/v1/cash/projection.
// Untuk peran manager, OpeningCash, FixedCostDaily, CashBuffer, Days, dan Entries
// dikosongkan (omitempty) sehingga manager hanya melihat status lolos/tidak dan titik terendah.
type CashProjectionResult struct {
	StartDate       string                 `json:"start_date"`
	DaysCount       int                    `json:"days_count"`
	Passed          bool                   `json:"passed"`
	Lowest          decimal.Decimal        `json:"lowest"`
	LowestDate      string                 `json:"lowest_date"`
	DaysBelowBuffer []string               `json:"days_below_buffer"`
	OpeningCash     *decimal.Decimal       `json:"opening_cash,omitempty"`
	FixedCostDaily  *decimal.Decimal       `json:"fixed_cost_daily,omitempty"`
	CashBuffer      *decimal.Decimal       `json:"cash_buffer,omitempty"`
	Days            []CashProjectionDayDTO `json:"days,omitempty"`
	Entries         []CashEntryDTO         `json:"entries,omitempty"`
}

// GetCashProjection menghitung proyeksi arus kas harian selama `days` hari ke depan
// menggunakan engine.ProjectCash.
func GetCashProjection(ctx context.Context, q db.DBTX, days int, actorRole string) (*CashProjectionResult, error) {
	if actorRole != auth.RoleOwner && actorRole != auth.RoleManager {
		return nil, httpx.Err(http.StatusForbidden, "forbidden", "Anda tidak memiliki izin untuk melihat proyeksi kas.")
	}
	if days < 1 || days > 90 {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Parameter days harus berada di antara 1 sampai 90.")
	}

	if _, err := ExpireOverdueOrders(ctx, q, time.Now().UTC()); err != nil {
		return nil, err
	}
	params, err := LoadSettingsParams(ctx, q)
	if err != nil {
		return nil, err
	}
	today := db.TodayQ(ctx, q)

	events, err := LoadCashEvents(ctx, q, today, params)
	if err != nil {
		return nil, err
	}

	proj := engine.ProjectCash(
		params.EvalParams.OpeningCash,
		today,
		days,
		events,
		params.EvalParams.FixedCostDaily,
	)

	// Kelompokkan inflow dan outflow per tanggal untuk rincian harian owner
	inflowByDay := make(map[string]decimal.Decimal)
	outflowByDay := make(map[string]decimal.Decimal)
	for _, ev := range events {
		dk := engine.DateKey(ev.Date)
		if ev.Amount.IsPositive() {
			inflowByDay[dk] = inflowByDay[dk].Add(ev.Amount)
		} else if ev.Amount.IsNegative() {
			outflowByDay[dk] = outflowByDay[dk].Add(ev.Amount.Abs())
		}
	}

	lowestKey := engine.DateKey(proj.LowestDate)
	daysBelowBuffer := make([]string, 0)
	dayDTOs := make([]CashProjectionDayDTO, 0, len(proj.Days))

	for _, d := range proj.Days {
		dk := engine.DateKey(d.Date)
		below := d.Balance.LessThan(params.EvalParams.CashBuffer)
		if below {
			daysBelowBuffer = append(daysBelowBuffer, dk)
		}
		inf := inflowByDay[dk].Round(2)
		outf := outflowByDay[dk].Round(2)
		dayDTOs = append(dayDTOs, CashProjectionDayDTO{
			Date:        dk,
			Inflow:      inf,
			Outflow:     outf,
			NetEvents:   inf.Sub(outf).Round(2),
			FixedCost:   params.EvalParams.FixedCostDaily.Round(2),
			Balance:     d.Balance.Round(2),
			BelowBuffer: below,
			IsLowest:    dk == lowestKey && d.Balance.Equal(proj.Lowest),
		})
	}

	passed := proj.Lowest.GreaterThanOrEqual(params.EvalParams.CashBuffer)
	res := &CashProjectionResult{
		StartDate:       engine.DateKey(today),
		DaysCount:       days,
		Passed:          passed,
		Lowest:          proj.Lowest.Round(2),
		LowestDate:      lowestKey,
		DaysBelowBuffer: daysBelowBuffer,
	}

	// Hanya owner yang boleh melihat saldo kas awal, biaya tetap harian, buffer, rincian harian, dan daftar entri
	if actorRole == auth.RoleOwner {
		oc := params.EvalParams.OpeningCash.Round(2)
		fc := params.EvalParams.FixedCostDaily.Round(2)
		cb := params.EvalParams.CashBuffer.Round(2)
		res.OpeningCash = &oc
		res.FixedCostDaily = &fc
		res.CashBuffer = &cb
		res.Days = dayDTOs

		endKey := engine.DateKey(today.AddDate(0, 0, days-1))
		ceRows, err := q.Query(ctx, `
			SELECT id::text, entry_date, amount, COALESCE(note, ''), created_by::text
			FROM cash_entries
			WHERE entry_date >= $1::date AND entry_date <= $2::date
			ORDER BY entry_date ASC, id ASC
		`, engine.DateKey(today), endKey)
		if err != nil {
			return nil, err
		}
		defer ceRows.Close()

		entries := make([]CashEntryDTO, 0)
		for ceRows.Next() {
			var ce CashEntryDTO
			var dt time.Time
			if err := ceRows.Scan(&ce.ID, &dt, &ce.Amount, &ce.Note, &ce.CreatedBy); err != nil {
				return nil, err
			}
			ce.EntryDate = engine.DateKey(dt)
			ce.Amount = ce.Amount.Round(2)
			entries = append(entries, ce)
		}
		if err := ceRows.Err(); err != nil {
			return nil, err
		}
		res.Entries = entries
	}

	return res, nil
}

// CreateCashEntryInput adalah masukan untuk POST /api/v1/cash/entries.
type CreateCashEntryInput struct {
	EntryDate string
	Amount    decimal.Decimal
	Note      string
	ActorID   string
}

// CreateCashEntry menambahkan entri kas manual (khusus owner) dan mencatat audit log.
func CreateCashEntry(ctx context.Context, conn db.Conn, req CreateCashEntryInput) (*CashEntryDTO, error) {
	if req.Amount.IsZero() {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Nominal entri kas (amount) tidak boleh 0.")
	}
	note := strings.TrimSpace(req.Note)
	if len(note) > 300 {
		return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Catatan kas maksimal 300 karakter.")
	}

	var out *CashEntryDTO
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		var entryDate time.Time
		if strings.TrimSpace(req.EntryDate) == "" {
			entryDate = db.TodayQ(ctx, tx)
		} else {
			var err error
			entryDate, err = db.ParseDate(strings.TrimSpace(req.EntryDate))
			if err != nil {
				return httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Tanggal entri kas (entry_date) harus berformat YYYY-MM-DD.")
			}
		}

		dateKey := engine.DateKey(entryDate)
		var actorPtr any
		if req.ActorID != "" {
			actorPtr = req.ActorID
		}
		var notePtr any
		if note != "" {
			notePtr = note
		}

		var entryID string
		err := tx.QueryRow(ctx, `
			INSERT INTO cash_entries (entry_date, amount, note, created_by)
			VALUES ($1::date, $2, $3, $4::uuid)
			RETURNING id::text
		`, dateKey, req.Amount.Round(4), notePtr, actorPtr).Scan(&entryID)
		if err != nil {
			return err
		}

		if err := audit.Log(ctx, tx, req.ActorID, audit.ActionMaster, "cash_entries", entryID, map[string]any{
			"op":         "create_cash_entry",
			"entry_date": dateKey,
			"amount":     req.Amount.Round(2),
			"note":       note,
		}); err != nil {
			return err
		}

		var createdBy *string
		if req.ActorID != "" {
			aid := req.ActorID
			createdBy = &aid
		}
		out = &CashEntryDTO{
			ID:        entryID,
			EntryDate: dateKey,
			Amount:    req.Amount.Round(2),
			Note:      note,
			CreatedBy: createdBy,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ProductCostingDTO memuat rincian HPP, margin, harga minimal, dan diskon maksimum produk.
type ProductCostingDTO struct {
	ProductID           string                     `json:"product_id"`
	Name                string                     `json:"name"`
	Category            string                     `json:"category"`
	IsActive            bool                       `json:"is_active"`
	Price               decimal.Decimal            `json:"price"`
	IngredientCost      decimal.Decimal            `json:"ingredient_cost"`
	WastePct            decimal.Decimal            `json:"waste_pct"`
	PackagingCost       decimal.Decimal            `json:"packaging_cost"`
	LaborMinutes        decimal.Decimal            `json:"labor_minutes"`
	LaborCost           decimal.Decimal            `json:"labor_cost"`
	OverheadCost        decimal.Decimal            `json:"overhead_cost"`
	UnitCost            decimal.Decimal            `json:"unit_cost"`
	PurchaseCostPerUnit decimal.Decimal            `json:"purchase_cost_per_unit"`
	Margin              decimal.Decimal            `json:"margin"`
	MarginMin           decimal.Decimal            `json:"margin_min"`
	MinPrice            decimal.Decimal            `json:"min_price"`
	MaxDiscount         decimal.Decimal            `json:"max_discount"`
	Loads               map[string]decimal.Decimal `json:"loads"`
}

// GetProductsCosting menghitung biaya per unit (engine.UnitCost), margin (engine.Margin),
// harga minimal (engine.MinPrice), dan diskon maksimum (engine.MaxDiscount) untuk seluruh produk.
func GetProductsCosting(ctx context.Context, q db.DBTX) ([]ProductCostingDTO, error) {
	params, err := LoadSettingsParams(ctx, q)
	if err != nil {
		return nil, err
	}

	rows, err := q.Query(ctx, `
		SELECT id::text, name, category, price, packaging_cost, overhead_cost, is_active
		FROM products
		ORDER BY name ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type prodRow struct {
		ID            string
		Name          string
		Category      string
		Price         decimal.Decimal
		PackagingCost decimal.Decimal
		OverheadCost  decimal.Decimal
		IsActive      bool
	}
	prods := make([]prodRow, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var pr prodRow
		if err := rows.Scan(&pr.ID, &pr.Name, &pr.Category, &pr.Price, &pr.PackagingCost, &pr.OverheadCost, &pr.IsActive); err != nil {
			return nil, err
		}
		prods = append(prods, pr)
		ids = append(ids, pr.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	if len(prods) == 0 {
		return []ProductCostingDTO{}, nil
	}

	loadsByProd := make(map[string]map[string]decimal.Decimal, len(prods))
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
		var ld decimal.Decimal
		if err := loadRows.Scan(&pid, &code, &ld); err != nil {
			return nil, err
		}
		if loadsByProd[pid] == nil {
			loadsByProd[pid] = make(map[string]decimal.Decimal)
		}
		loadsByProd[pid][code] = ld
	}
	if err := loadRows.Err(); err != nil {
		return nil, err
	}
	loadRows.Close()

	ingCostByProd := make(map[string]decimal.Decimal, len(prods))
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
		var qty, bp, bus decimal.Decimal
		if err := recRows.Scan(&pid, &qty, &bp, &bus); err != nil {
			return nil, err
		}
		if bus.IsPositive() {
			ingCostByProd[pid] = ingCostByProd[pid].Add(qty.Mul(bp).DivRound(bus, 8))
		}
	}
	if err := recRows.Err(); err != nil {
		return nil, err
	}
	recRows.Close()

	onePlusWaste := decimal.NewFromInt(1).Add(params.WastePct)
	out := make([]ProductCostingDTO, 0, len(prods))

	for _, pr := range prods {
		loads := loadsByProd[pr.ID]
		if loads == nil {
			loads = make(map[string]decimal.Decimal)
		}
		ingCost := ingCostByProd[pr.ID]
		laborMin := loads["labor"]
		laborCost := laborMin.Mul(params.LaborRatePerMin)

		unitCost := engine.UnitCost(engine.CostInput{
			IngredientCost: ingCost,
			WastePct:       params.WastePct,
			PackagingCost:  pr.PackagingCost,
			LaborMinutes:   laborMin,
			LaborRate:      params.LaborRatePerMin,
			OverheadCost:   pr.OverheadCost,
		})
		purchaseCost := ingCost.Mul(onePlusWaste).Add(pr.PackagingCost)
		margin := engine.Margin(pr.Price, unitCost)
		minPrice := engine.MinPrice(unitCost, params.EvalParams.MarginMin)
		maxDisc := engine.MaxDiscount(pr.Price, minPrice)

		out = append(out, ProductCostingDTO{
			ProductID:           pr.ID,
			Name:                pr.Name,
			Category:            pr.Category,
			IsActive:            pr.IsActive,
			Price:               pr.Price.Round(2),
			IngredientCost:      ingCost.Round(2),
			WastePct:            params.WastePct.Round(4),
			PackagingCost:       pr.PackagingCost.Round(2),
			LaborMinutes:        laborMin.Round(2),
			LaborCost:           laborCost.Round(2),
			OverheadCost:        pr.OverheadCost.Round(2),
			UnitCost:            unitCost.Round(2),
			PurchaseCostPerUnit: purchaseCost.Round(2),
			Margin:              margin.Round(4),
			MarginMin:           params.EvalParams.MarginMin.Round(4),
			MinPrice:            minPrice.Round(2),
			MaxDiscount:         maxDisc.Round(4),
			Loads:               loads,
		})
	}

	return out, nil
}

// --- HTTP Handlers untuk Tahap 5 ---

// ShoppingList menangani GET /api/v1/shopping-list?from=&to=.
func (h *Handler) ShoppingList(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}

	today := db.TodayQ(c.Request.Context(), h.DB)
	fromDate := today
	toDate := today.AddDate(0, 0, 13)

	if rawFrom := strings.TrimSpace(c.Query("from")); rawFrom != "" {
		parsed, err := db.ParseDate(rawFrom)
		if err != nil {
			httpx.UnprocessableEntity(c, "validation_error", "Parameter from harus berformat YYYY-MM-DD.")
			return
		}
		fromDate = parsed
	}
	if rawTo := strings.TrimSpace(c.Query("to")); rawTo != "" {
		parsed, err := db.ParseDate(rawTo)
		if err != nil {
			httpx.UnprocessableEntity(c, "validation_error", "Parameter to harus berformat YYYY-MM-DD.")
			return
		}
		toDate = parsed
	}
	if toDate.Before(fromDate) {
		httpx.UnprocessableEntity(c, "validation_error", "Tanggal akhir (to) tidak boleh sebelum tanggal awal (from).")
		return
	}

	res, err := GetShoppingList(c.Request.Context(), h.DB, fromDate, toDate, u.Role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

type recordPurchaseRequest struct {
	IngredientID string           `json:"ingredient_id"`
	QtyBuyUnits  *decimal.Decimal `json:"qty_buy_units"`
	UnitPrice    *decimal.Decimal `json:"unit_price"`
	TotalCost    *decimal.Decimal `json:"total_cost"`
	PurchasedAt  *string          `json:"purchased_at"`
	Supplier     *string          `json:"supplier"`
	Note         string           `json:"note"`
}

// RecordPurchase menangani POST /api/v1/purchases.
func (h *Handler) RecordPurchase(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}

	var req recordPurchaseRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	v := httpx.NewValidation()
	ingID := strings.ToLower(strings.TrimSpace(req.IngredientID))
	if !httpx.ValidUUID(ingID) {
		v.Add("ingredient_id harus berformat UUID yang valid.")
	}
	qtyBuy := v.Dec("Jumlah beli (qty_buy_units)", req.QtyBuyUnits, httpx.DecOpts{
		Min:          decimal.Zero,
		Max:          decimal.NewFromInt(999_999_999),
		MinExclusive: true,
		MaxScale:     4,
	})
	var unitPricePtr *decimal.Decimal
	if req.UnitPrice != nil {
		up := v.Dec("Harga beli per satuan (unit_price)", req.UnitPrice, httpx.MoneyOpts(false, true))
		unitPricePtr = &up
	} else if req.TotalCost != nil && req.QtyBuyUnits != nil && req.QtyBuyUnits.IsPositive() {
		tc := v.Dec("Total biaya beli (total_cost)", req.TotalCost, httpx.MoneyOpts(false, true))
		up := tc.DivRound(*req.QtyBuyUnits, 2)
		unitPricePtr = &up
	}
	note := v.OptStr("Catatan pembelian", req.Note, 300)
	if note == "" && req.Supplier != nil && *req.Supplier != "" {
		note = "Pemasok: " + strings.TrimSpace(*req.Supplier)
	}

	if !v.OK() {
		v.Respond(c)
		return
	}

	res, err := RecordPurchase(c.Request.Context(), h.DB, PurchaseInput{
		IngredientID: ingID,
		QtyBuyUnits:  qtyBuy,
		UnitPrice:    unitPricePtr,
		ActorID:      u.ID,
		ActorRole:    u.Role,
		Note:         note,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.Created(c, res)
}

type stockAdjustRequest struct {
	IngredientID string           `json:"ingredient_id"`
	Delta        *decimal.Decimal `json:"delta"`
	Qty          *decimal.Decimal `json:"qty"`
	NewStock     *decimal.Decimal `json:"new_stock"`
	Reason       string           `json:"reason"`
}

// AdjustStock menangani POST /api/v1/stock/adjust.
func (h *Handler) AdjustStock(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}

	var req stockAdjustRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	v := httpx.NewValidation()
	ingID := strings.ToLower(strings.TrimSpace(req.IngredientID))
	if !httpx.ValidUUID(ingID) {
		v.Add("ingredient_id harus berformat UUID yang valid.")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		v.Add("Alasan koreksi stok (reason) wajib diisi.")
	} else if len(reason) > 300 {
		v.Add("Alasan koreksi stok maksimal 300 karakter.")
	}
	deltaPtr := req.Delta
	if deltaPtr == nil && req.Qty != nil {
		deltaPtr = req.Qty
	}
	if deltaPtr == nil && req.NewStock == nil {
		v.Add("Perubahan stok (delta) atau stok akhir (new_stock) wajib diisi.")
	}
	if !v.OK() {
		v.Respond(c)
		return
	}

	res, err := AdjustStock(c.Request.Context(), h.DB, StockAdjustInput{
		IngredientID: ingID,
		Delta:        deltaPtr,
		NewStock:     req.NewStock,
		Reason:       reason,
		ActorID:      u.ID,
		ActorRole:    u.Role,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

// CashProjection menangani GET /api/v1/cash/projection?days=14.
func (h *Handler) CashProjection(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}

	days := 14
	if rawDays := strings.TrimSpace(c.Query("days")); rawDays != "" {
		parsed, err := strconv.Atoi(rawDays)
		if err != nil || parsed < 1 || parsed > 90 {
			httpx.UnprocessableEntity(c, "validation_error", "Parameter days harus berupa bilangan bulat antara 1 sampai 90.")
			return
		}
		days = parsed
	}

	res, err := GetCashProjection(c.Request.Context(), h.DB, days, u.Role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

type createCashEntryRequest struct {
	EntryDate string           `json:"entry_date"`
	Amount    *decimal.Decimal `json:"amount"`
	Note      string           `json:"note"`
}

// CreateCashEntry menangani POST /api/v1/cash/entries (khusus owner).
func (h *Handler) CreateCashEntry(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	u := auth.CurrentUser(c)
	if u == nil {
		httpx.Unauthorized(c, "Sesi tidak valid. Silakan masuk kembali.")
		return
	}

	var req createCashEntryRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	v := httpx.NewValidation()
	if req.Amount == nil {
		v.Add("Nominal entri kas (amount) wajib diisi.")
	} else if req.Amount.IsZero() {
		v.Add("Nominal entri kas (amount) tidak boleh 0.")
	}
	note := v.OptStr("Catatan kas", req.Note, 300)
	if !v.OK() {
		v.Respond(c)
		return
	}

	res, err := CreateCashEntry(c.Request.Context(), h.DB, CreateCashEntryInput{
		EntryDate: req.EntryDate,
		Amount:    *req.Amount,
		Note:      note,
		ActorID:   u.ID,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.Created(c, res)
}

// ProductsCosting menangani GET /api/v1/costing/products.
func (h *Handler) ProductsCosting(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	items, err := GetProductsCosting(c.Request.Context(), h.DB)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, items)
}

