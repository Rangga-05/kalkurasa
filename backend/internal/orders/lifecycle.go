package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/httpx"
)

// ExpireOverdueOrders menjalankan pengecekan malas (lazy check) sesuai Bagian 8.4:
// pesanan berstatus 'accepted' yang sudah melewati dp_deadline dan paid < dp_required
// diubah menjadi 'expired' sehingga kapasitasnya dilepas, serta dicatat di audit_logs.
func ExpireOverdueOrders(ctx context.Context, q db.DBTX, now time.Time) (int, error) {
	rows, err := q.Query(ctx, `
		UPDATE orders
		SET status = 'expired', updated_at = $1
		WHERE status = 'accepted'
		  AND dp_deadline IS NOT NULL
		  AND dp_deadline <= $1
		  AND paid < dp_required
		RETURNING id::text, code, dp_required, paid, dp_deadline
	`, now.UTC())
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type expiredItem struct {
		ID         string
		Code       string
		DPRequired decimal.Decimal
		Paid       decimal.Decimal
		DPDeadline time.Time
	}
	var expired []expiredItem
	for rows.Next() {
		var it expiredItem
		if err := rows.Scan(&it.ID, &it.Code, &it.DPRequired, &it.Paid, &it.DPDeadline); err != nil {
			return 0, err
		}
		expired = append(expired, it)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()

	for _, it := range expired {
		if err := audit.Log(ctx, q, "", audit.ActionStatus, "orders", it.ID, map[string]any{
			"op":          "expire_overdue_dp",
			"code":        it.Code,
			"from_status": StatusAccepted,
			"to_status":   StatusExpired,
			"dp_required": it.DPRequired.Round(2),
			"paid":        it.Paid.Round(2),
			"dp_deadline": it.DPDeadline.UTC().Format(time.RFC3339),
		}); err != nil {
			return len(expired), err
		}
	}
	return len(expired), nil
}

// TaskItem adalah representasi tugas operasional (produce, shop, deliver).
type TaskItem struct {
	ID            string    `json:"id"`
	OrderID       string    `json:"order_id"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
	DueDate       string    `json:"due_date"`
	AssigneeID    *string   `json:"assignee_id,omitempty"`
	Status        string    `json:"status"`
	ActualMinutes *int      `json:"actual_minutes,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// PaymentItem adalah representasi pembayaran pesanan.
type PaymentItem struct {
	ID         string          `json:"id"`
	OrderID    string          `json:"order_id"`
	Amount     decimal.Decimal `json:"amount"`
	Method     string          `json:"method"`
	PaidAt     time.Time       `json:"paid_at"`
	Note       string          `json:"note,omitempty"`
	RecordedBy *string         `json:"recorded_by,omitempty"`
}

// DecideOrderInput adalah masukan untuk POST /api/v1/orders/:id/decision (Bagian 8.2).
type DecideOrderInput struct {
	OrderID        string
	ActorID        string
	ActorName      string
	ActorRole      string
	Action         string // "accept" | "counter" | "reject"
	OverrideReason string
	Counter        *CounterOfferDTO
}

// DecideOrder menjalankan alur Bagian 8.2 dalam satu transaksi dengan kunci hari:
//  1. Kunci hari di day_locks (tanggal produksi + 15 hari).
//  2. Jalankan pengecekan malas kedaluwarsa DP.
//  3. Kunci baris pesanan (FOR UPDATE) dan pastikan berstatus pending_decision.
//  4. Evaluasi ulang gerbang G1..G4 menggunakan internal/engine.
//  5. Validasi wewenang peran (owner untuk semua pesanan; manager hanya di bawah ambang pesanan besar).
//  6. Bila action == "accept" padahal rekomendasi bukan "accept", wajibkan override_reason (422)
//     dan catat di audit_logs (siapa, kapan, rekomendasi awal).
func DecideOrder(ctx context.Context, conn db.Conn, req DecideOrderInput) (*OrderDetail, error) {
	if req.ActorRole != auth.RoleOwner && req.ActorRole != auth.RoleManager {
		return nil, httpx.Err(http.StatusForbidden, "forbidden", "Anda tidak memiliki izin untuk mengambil keputusan pesanan.")
	}

	var out *OrderDetail
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		// 1. Ambil tanggal pesanan terlebih dahulu untuk mengunci day_locks dengan urutan terurut
		var preEventDate, preProdDate time.Time
		err := tx.QueryRow(ctx, `
			SELECT event_date, production_date
			FROM orders
			WHERE id = $1::uuid
		`, req.OrderID).Scan(&preEventDate, &preProdDate)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Pesanan tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		lockDates := make([]time.Time, 0, 18)
		lockDates = append(lockDates, preProdDate, preEventDate)
		for i := 1; i <= 15; i++ {
			lockDates = append(lockDates, preProdDate.AddDate(0, 0, i))
		}
		if req.Counter != nil && strings.TrimSpace(req.Counter.Date) != "" {
			if cd, err := db.ParseDate(strings.TrimSpace(req.Counter.Date)); err == nil {
				lockDates = append(lockDates, cd)
			}
		}
		if err := LockDays(ctx, tx, lockDates); err != nil {
			return err
		}

		now := time.Now().UTC()
		if _, err := ExpireOverdueOrders(ctx, tx, now); err != nil {
			return err
		}

		params, err := LoadSettingsParams(ctx, tx)
		if err != nil {
			return err
		}
		today := db.TodayQ(ctx, tx)

		// 2. Kunci baris pesanan FOR UPDATE dan muat data terbarunya
		var (
			orderID, code, source, custName, custContact, deliveryType string
			addrPtr, notesPtr, recPtr, recReasonPtr                    *string
			eventDate, prodDate                                        time.Time
			status                                                     string
			total, dpRequired, paid                                    decimal.Decimal
			dpDeadline                                                 *time.Time
			counterBytes                                               []byte
			createdAt, updatedAt                                       time.Time
		)
		err = tx.QueryRow(ctx, `
			SELECT id::text, code, source, customer_name, customer_contact, delivery_type,
			       address, notes, event_date, production_date,
			       status, total, dp_required, dp_deadline, paid,
			       recommendation, recommendation_reason, counter_offer,
			       created_at, updated_at
			FROM orders
			WHERE id = $1::uuid
			FOR UPDATE
		`, req.OrderID).Scan(
			&orderID, &code, &source, &custName, &custContact, &deliveryType,
			&addrPtr, &notesPtr, &eventDate, &prodDate,
			&status, &total, &dpRequired, &dpDeadline, &paid,
			&recPtr, &recReasonPtr, &counterBytes,
			&createdAt, &updatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Pesanan tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		if status != StatusPendingDecision {
			return httpx.Err(http.StatusConflict, "conflict", fmt.Sprintf("Pesanan tidak dapat diputuskan karena sudah berstatus %q.", status))
		}

		initialRec := ""
		if recPtr != nil {
			initialRec = *recPtr
		}

		// 3. Muat item pesanan dan metadata produk untuk evaluasi ulang
		itemDetails, evalItems, totalUnits, err := loadOrderItemsAndEval(ctx, tx, orderID, params)
		if err != nil {
			return err
		}

		evalRes, err := EvaluateOrder(ctx, tx, today, eventDate, prodDate, evalItems, params)
		if err != nil {
			return err
		}
		evalDTO := ToEvaluationDTO(evalRes)

		// 4. Cek ambang pesanan besar: manager hanya boleh memutuskan pesanan di bawah ambang
		isBigOrder := evalRes.Total.GreaterThanOrEqual(params.BigOrderTotal) || totalUnits >= params.BigOrderUnits
		if req.ActorRole == auth.RoleManager && isBigOrder {
			return httpx.Err(http.StatusForbidden, "forbidden", "Pesanan besar hanya dapat diputuskan oleh pemilik (owner).")
		}

		var (
			newStatus      string
			newDPRequired  = evalRes.DPCharged.Round(4)
			newDPDeadline  *time.Time
			finalCounter   *CounterOfferDTO
			overrideReason *string
			isOverride     bool
		)

		switch req.Action {
		case "accept":
			// Evaluasi ulang wajib lolos semua gerbang ("accept") untuk terima tanpa override.
			// Jika rekomendasi awal atau hasil evaluasi ulang bukan "accept", wajib override_reason.
			isOverride = evalRes.Recommendation != engine.RecAccept || (initialRec != "" && initialRec != engine.RecAccept)
			trimmedReason := strings.TrimSpace(req.OverrideReason)
			if isOverride && trimmedReason == "" {
				return httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Alasan override (override_reason) wajib diisi karena rekomendasi sistem bukan Terima.")
			}
			if trimmedReason != "" {
				overrideReason = &trimmedReason
			}
			newStatus = StatusAccepted
			dl := now.Add(time.Duration(params.DPDeadlineHours) * time.Hour)
			newDPDeadline = &dl
			finalCounter = evalDTO.CounterOffer

		case "counter":
			if req.Counter != nil {
				cDateStr := strings.TrimSpace(req.Counter.Date)
				if cDateStr == "" && evalDTO.CounterOffer != nil {
					cDateStr = evalDTO.CounterOffer.Date
				}
				if _, err := db.ParseDate(cDateStr); err != nil {
					return httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Tanggal tawaran balik (counter.date) wajib berformat YYYY-MM-DD.")
				}
				minPrices := req.Counter.MinPricePerItem
				if len(minPrices) == 0 && evalDTO.CounterOffer != nil {
					minPrices = evalDTO.CounterOffer.MinPricePerItem
				}
				if minPrices == nil {
					minPrices = make(map[string]decimal.Decimal)
					for _, it := range evalItems {
						minPrices[it.ProductID] = engine.MinPrice(it.UnitCost, params.EvalParams.MarginMin).Round(2)
					}
				}
				dpVal := req.Counter.DP
				if !dpVal.IsPositive() {
					dpVal = evalRes.DPCharged.Round(2)
				}
				finalCounter = &CounterOfferDTO{
					Date:            cDateStr,
					MinPricePerItem: minPrices,
					DP:              dpVal.Round(2),
				}
			} else if evalDTO.CounterOffer != nil {
				finalCounter = evalDTO.CounterOffer
			} else {
				return httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Tawaran balik otomatis tidak tersedia; silakan isi parameter counter atau pilih tolak.")
			}
			newStatus = StatusCountered
			newDPRequired = finalCounter.DP.Round(4)
			evalDTO.CounterOffer = finalCounter

		case "reject":
			newStatus = StatusRejected
			finalCounter = evalDTO.CounterOffer

		default:
			return httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Aksi keputusan harus salah satu dari: accept, counter, reject.")
		}

		var counterJSON any
		if finalCounter != nil {
			b, err := json.Marshal(finalCounter)
			if err != nil {
				return err
			}
			counterJSON = string(b)
		}

		snapPayload := map[string]any{
			"evaluated_at":           now.Format(time.RFC3339),
			"decided_at":             now.Format(time.RFC3339),
			"decided_by":             req.ActorID,
			"initial_recommendation": initialRec,
			"today":                  engine.DateKey(today),
			"evaluation":             evalDTO,
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
		snapBytes, err := json.Marshal(snapPayload)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET status = $1,
			    total = $2,
			    dp_required = $3,
			    dp_deadline = $4,
			    recommendation = $5,
			    recommendation_reason = $6,
			    counter_offer = $7::jsonb,
			    decided_by = $8::uuid,
			    decided_at = $9,
			    override_reason = $10,
			    snapshot = $11::jsonb,
			    updated_at = $9
			WHERE id = $12::uuid
		`,
			newStatus,
			evalRes.Total.Round(4),
			newDPRequired,
			newDPDeadline,
			evalRes.Recommendation,
			evalRes.Reason,
			counterJSON,
			req.ActorID,
			now,
			overrideReason,
			string(snapBytes),
			orderID,
		)
		if err != nil {
			return err
		}

		// Perbarui 4 baris order_evaluations dengan hasil evaluasi ulang
		if err := replaceOrderEvaluations(ctx, tx, orderID, evalRes); err != nil {
			return err
		}

		// Tulis audit_logs (ActionOverride bila override, ActionDecision bila normal)
		auditAction := audit.ActionDecision
		auditDetail := map[string]any{
			"op":                         "decision_" + req.Action,
			"action":                     req.Action,
			"code":                       code,
			"actor_id":                   req.ActorID,
			"actor_name":                 req.ActorName,
			"actor_role":                 req.ActorRole,
			"decided_at":                 now.Format(time.RFC3339),
			"initial_recommendation":     initialRec,
			"reevaluated_recommendation": evalRes.Recommendation,
			"status":                     newStatus,
		}
		if isOverride && req.Action == "accept" {
			auditAction = audit.ActionOverride
			auditDetail["op"] = "decision_override"
			auditDetail["override_reason"] = strings.TrimSpace(req.OverrideReason)
		}
		if err := audit.Log(ctx, tx, req.ActorID, auditAction, "orders", orderID, auditDetail); err != nil {
			return err
		}

		var addrStr, notesStr string
		if addrPtr != nil {
			addrStr = *addrPtr
		}
		if notesPtr != nil {
			notesStr = *notesPtr
		}

		out = &OrderDetail{
			ID:                   orderID,
			Code:                 code,
			Source:               source,
			CustomerName:         custName,
			CustomerContact:      custContact,
			DeliveryType:         deliveryType,
			Address:              addrStr,
			Notes:                notesStr,
			EventDate:            engine.DateKey(eventDate),
			ProductionDate:       engine.DateKey(prodDate),
			Status:               newStatus,
			Total:                evalRes.Total.Round(2),
			DPRequired:           newDPRequired.Round(2),
			DPDeadline:           newDPDeadline,
			Paid:                 paid.Round(2),
			Recommendation:       evalRes.Recommendation,
			RecommendationReason: evalRes.Reason,
			CounterOffer:         finalCounter,
			Evaluation:           evalDTO,
			Items:                itemDetails,
			CreatedAt:            createdAt,
			UpdatedAt:            now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RespondCounterOffer menangani POST /api/v1/public/orders/:code/counter-response (Bagian 8.3).
// Jika pelanggan menerima (accept=true), pesanan dievaluasi ulang dalam transaksi dengan
// kunci hari baru dan harga/tanggal dari counter_offer -> status 'accepted' atau 'rejected'.
// Jika pelanggan menolak (accept=false) -> status 'rejected'.
func RespondCounterOffer(ctx context.Context, conn db.Conn, rawCode string, accept bool) (*PublicTrackResponse, error) {
	code, ok := NormalizeCode(rawCode)
	if !ok {
		return nil, httpx.Err(http.StatusNotFound, "not_found", "Pesanan dengan kode tersebut tidak ditemukan.")
	}

	var out *PublicTrackResponse
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		var (
			orderID, custName, deliveryType, status string
			eventDate, prodDate                     time.Time
			total, dpRequired, paid                 decimal.Decimal
			dpDeadline                              *time.Time
			counterBytes                            []byte
			createdAt, updatedAt                    time.Time
		)
		err := tx.QueryRow(ctx, `
			SELECT id::text, customer_name, delivery_type, event_date, production_date, status,
			       total, dp_required, dp_deadline, paid, counter_offer, created_at, updated_at
			FROM orders
			WHERE code = $1
		`, code).Scan(
			&orderID, &custName, &deliveryType, &eventDate, &prodDate, &status,
			&total, &dpRequired, &dpDeadline, &paid, &counterBytes, &createdAt, &updatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Pesanan dengan kode tersebut tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		if status != StatusCountered {
			return httpx.Err(http.StatusConflict, "conflict", "Pesanan ini tidak sedang dalam status tawaran balik (countered).")
		}

		var co CounterOfferDTO
		if len(counterBytes) > 0 && string(counterBytes) != "null" {
			_ = json.Unmarshal(counterBytes, &co)
		}
		if co.Date == "" {
			return httpx.Err(http.StatusConflict, "conflict", "Data tawaran balik pada pesanan ini tidak valid.")
		}

		newEventDate, err := db.ParseDate(co.Date)
		if err != nil {
			return httpx.Err(http.StatusConflict, "conflict", "Tanggal tawaran balik tidak valid.")
		}
		offsetDays := int(eventDate.Sub(prodDate).Hours() / 24)
		newProdDate := newEventDate.AddDate(0, 0, -offsetDays)

		lockDates := make([]time.Time, 0, 18)
		lockDates = append(lockDates, prodDate, eventDate, newProdDate, newEventDate)
		for i := 1; i <= 15; i++ {
			lockDates = append(lockDates, newProdDate.AddDate(0, 0, i))
		}
		if err := LockDays(ctx, tx, lockDates); err != nil {
			return err
		}

		now := time.Now().UTC()
		if _, err := ExpireOverdueOrders(ctx, tx, now); err != nil {
			return err
		}

		// Kunci baris pesanan FOR UPDATE untuk memastikan status masih countered
		var curStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1::uuid FOR UPDATE`, orderID).Scan(&curStatus); err != nil {
			return err
		}
		if curStatus != StatusCountered {
			return httpx.Err(http.StatusConflict, "conflict", "Pesanan ini sudah tidak berstatus tawaran balik.")
		}

		params, err := LoadSettingsParams(ctx, tx)
		if err != nil {
			return err
		}
		today := db.TodayQ(ctx, tx)

		itemDetails, evalItems, _, err := loadOrderItemsAndEval(ctx, tx, orderID, params)
		if err != nil {
			return err
		}

		if !accept {
			_, err := tx.Exec(ctx, `
				UPDATE orders
				SET status = $1, updated_at = $2
				WHERE id = $3::uuid
			`, StatusRejected, now, orderID)
			if err != nil {
				return err
			}
			if err := audit.Log(ctx, tx, "", audit.ActionDecision, "orders", orderID, map[string]any{
				"op":     "counter_response",
				"code":   code,
				"accept": false,
				"status": StatusRejected,
			}); err != nil {
				return err
			}

			pubItems := toPublicOrderItems(itemDetails)
			out = &PublicTrackResponse{
				Code:         code,
				CustomerName: custName,
				DeliveryType: deliveryType,
				EventDate:    engine.DateKey(eventDate),
				Status:       StatusRejected,
				Items:        pubItems,
				Total:        total.Round(2),
				DPRequired:   dpRequired.Round(2),
				DPDeadline:   nil,
				Paid:         paid.Round(2),
				CounterOffer: &co,
				CreatedAt:    createdAt,
				UpdatedAt:    now,
			}
			return nil
		}

		// Pelanggan menerima tawaran balik: sesuaikan harga item dengan min_price_per_item dari counter_offer
		for i := range evalItems {
			pid := evalItems[i].ProductID
			if minP, ok := co.MinPricePerItem[pid]; ok && evalItems[i].UnitPrice.LessThan(minP) {
				evalItems[i].UnitPrice = minP
				itemDetails[i].UnitPrice = minP
				itemDetails[i].Subtotal = minP.Mul(decimal.NewFromInt(int64(itemDetails[i].Qty)))
			}
		}

		evalRes, err := EvaluateOrder(ctx, tx, today, newEventDate, newProdDate, evalItems, params)
		if err != nil {
			return err
		}
		evalDTO := ToEvaluationDTO(evalRes)

		newStatus := StatusRejected
		newDPReq := evalRes.DPCharged.Round(4)
		if co.DP.GreaterThan(newDPReq) {
			newDPReq = co.DP.Round(4)
		}
		var newDPDeadline *time.Time
		if evalRes.Recommendation == engine.RecAccept {
			newStatus = StatusAccepted
			dl := now.Add(time.Duration(params.DPDeadlineHours) * time.Hour)
			newDPDeadline = &dl

			// Perbarui harga satuan pada order_items
			for _, it := range itemDetails {
				if _, err := tx.Exec(ctx, `
					UPDATE order_items
					SET unit_price = $1, unit_cost = $2
					WHERE id = $3::uuid
				`, it.UnitPrice.Round(4), it.UnitCost.Round(4), it.ID); err != nil {
					return err
				}
			}
		}

		snapBytes, _ := json.Marshal(map[string]any{
			"evaluated_at":     now.Format(time.RFC3339),
			"counter_accepted": true,
			"today":            engine.DateKey(today),
			"evaluation":       evalDTO,
		})

		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET event_date = $1::date,
			    production_date = $2::date,
			    status = $3,
			    total = $4,
			    dp_required = $5,
			    dp_deadline = $6,
			    recommendation = $7,
			    recommendation_reason = $8,
			    snapshot = $9::jsonb,
			    updated_at = $10
			WHERE id = $11::uuid
		`,
			engine.DateKey(newEventDate),
			engine.DateKey(newProdDate),
			newStatus,
			evalRes.Total.Round(4),
			newDPReq,
			newDPDeadline,
			evalRes.Recommendation,
			evalRes.Reason,
			string(snapBytes),
			now,
			orderID,
		)
		if err != nil {
			return err
		}

		if err := replaceOrderEvaluations(ctx, tx, orderID, evalRes); err != nil {
			return err
		}

		if err := audit.Log(ctx, tx, "", audit.ActionDecision, "orders", orderID, map[string]any{
			"op":         "counter_response",
			"code":       code,
			"accept":     true,
			"status":     newStatus,
			"event_date": engine.DateKey(newEventDate),
			"total":      evalRes.Total.Round(2),
		}); err != nil {
			return err
		}

		out = &PublicTrackResponse{
			Code:         code,
			CustomerName: custName,
			DeliveryType: deliveryType,
			EventDate:    engine.DateKey(newEventDate),
			Status:       newStatus,
			Items:        toPublicOrderItems(itemDetails),
			Total:        evalRes.Total.Round(2),
			DPRequired:   newDPReq.Round(2),
			DPDeadline:   newDPDeadline,
			Paid:         paid.Round(2),
			CounterOffer: &co,
			CreatedAt:    createdAt,
			UpdatedAt:    now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RecordPaymentInput adalah masukan untuk POST /api/v1/orders/:id/payments (Bagian 8.4).
type RecordPaymentInput struct {
	OrderID    string
	ActorID    string
	Amount     decimal.Decimal
	Method     string
	Note       string
}

// PaymentResult adalah hasil pencatatan pembayaran beserta status pesanan dan tugas yang dibuat.
type PaymentResult struct {
	Payment      PaymentItem   `json:"payment"`
	OrderID      string        `json:"order_id"`
	Code         string        `json:"code"`
	Status       string        `json:"status"`
	Total        decimal.Decimal `json:"total"`
	DPRequired   decimal.Decimal `json:"dp_required"`
	Paid         decimal.Decimal `json:"paid"`
	Remaining    decimal.Decimal `json:"remaining"`
	TasksCreated []TaskItem    `json:"tasks_created,omitempty"`
}

// RecordPayment mencatat pembayaran manual dan mengubah status pesanan menjadi 'confirmed'
// beserta pembuatan 3 tugas (produce, shop, deliver) saat paid >= dp_required (Bagian 8.4).
func RecordPayment(ctx context.Context, conn db.Conn, req RecordPaymentInput) (*PaymentResult, error) {
	var out *PaymentResult
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		now := time.Now().UTC()
		if _, err := ExpireOverdueOrders(ctx, tx, now); err != nil {
			return err
		}

		params, err := LoadSettingsParams(ctx, tx)
		if err != nil {
			return err
		}

		var (
			code, custName, status   string
			eventDate, prodDate      time.Time
			total, dpRequired, paid  decimal.Decimal
		)
		err = tx.QueryRow(ctx, `
			SELECT code, customer_name, event_date, production_date, status, total, dp_required, paid
			FROM orders
			WHERE id = $1::uuid
			FOR UPDATE
		`, req.OrderID).Scan(&code, &custName, &eventDate, &prodDate, &status, &total, &dpRequired, &paid)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Pesanan tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		if status == StatusExpired {
			return httpx.Err(http.StatusConflict, "conflict", "Pesanan sudah kedaluwarsa karena melewati tenggat pembayaran DP.")
		}
		switch status {
		case StatusAccepted, StatusConfirmed, StatusInProduction, StatusReady, StatusDelivering:
			// boleh menerima pembayaran
		default:
			return httpx.Err(http.StatusConflict, "conflict", fmt.Sprintf("Pembayaran tidak dapat dicatat pada pesanan berstatus %q.", status))
		}

		var notePtr, actorPtr any
		if strings.TrimSpace(req.Note) != "" {
			notePtr = strings.TrimSpace(req.Note)
		}
		if req.ActorID != "" {
			actorPtr = req.ActorID
		}

		var paymentID string
		err = tx.QueryRow(ctx, `
			INSERT INTO payments (order_id, amount, method, paid_at, note, recorded_by)
			VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)
			RETURNING id::text
		`, req.OrderID, req.Amount.Round(4), req.Method, now, notePtr, actorPtr).Scan(&paymentID)
		if err != nil {
			return err
		}

		newPaid := paid.Add(req.Amount).Round(4)
		newStatus := status
		var createdTasks []TaskItem

		// Bagian 8.4: Saat paid >= dp_required dan status accepted -> status confirmed + buat 3 tugas
		if status == StatusAccepted && newPaid.GreaterThanOrEqual(dpRequired) {
			newStatus = StatusConfirmed
			createdTasks, err = createOrderTasks(ctx, tx, req.OrderID, code, custName, eventDate, prodDate, params.EvalParams.ShoppingLeadDays, now)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(ctx, `
			UPDATE orders
			SET paid = $1, status = $2, updated_at = $3
			WHERE id = $4::uuid
		`, newPaid, newStatus, now, req.OrderID)
		if err != nil {
			return err
		}

		if err := audit.Log(ctx, tx, req.ActorID, audit.ActionPayment, "orders", req.OrderID, map[string]any{
			"op":          "record_payment",
			"payment_id":  paymentID,
			"code":        code,
			"amount":      req.Amount.Round(2),
			"method":      req.Method,
			"paid_before": paid.Round(2),
			"paid_after":  newPaid.Round(2),
			"from_status": status,
			"to_status":   newStatus,
		}); err != nil {
			return err
		}

		rem := total.Sub(newPaid).Round(2)
		if rem.IsNegative() {
			rem = decimal.Zero
		}
		var recBy *string
		if req.ActorID != "" {
			aid := req.ActorID
			recBy = &aid
		}
		out = &PaymentResult{
			Payment: PaymentItem{
				ID:         paymentID,
				OrderID:    req.OrderID,
				Amount:     req.Amount.Round(2),
				Method:     req.Method,
				PaidAt:     now,
				Note:       strings.TrimSpace(req.Note),
				RecordedBy: recBy,
			},
			OrderID:      req.OrderID,
			Code:         code,
			Status:       newStatus,
			Total:        total.Round(2),
			DPRequired:   dpRequired.Round(2),
			Paid:         newPaid.Round(2),
			Remaining:    rem,
			TasksCreated: createdTasks,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func createOrderTasks(ctx context.Context, tx pgx.Tx, orderID, code, custName string, eventDate, prodDate time.Time, shoppingLeadDays int, now time.Time) ([]TaskItem, error) {
	shopDue := eventDate.AddDate(0, 0, -shoppingLeadDays)
	specs := []struct {
		Type    string
		Title   string
		DueDate time.Time
	}{
		{
			Type:    "shop",
			Title:   fmt.Sprintf("Belanja bahan pesanan %s (%s)", code, custName),
			DueDate: shopDue,
		},
		{
			Type:    "produce",
			Title:   fmt.Sprintf("Produksi pesanan %s (%s)", code, custName),
			DueDate: prodDate,
		},
		{
			Type:    "deliver",
			Title:   fmt.Sprintf("Penyerahan pesanan %s (%s)", code, custName),
			DueDate: eventDate,
		},
	}

	out := make([]TaskItem, 0, len(specs))
	for _, s := range specs {
		dueKey := engine.DateKey(s.DueDate)
		var taskID string
		err := tx.QueryRow(ctx, `
			INSERT INTO tasks (order_id, type, title, due_date, status, created_at)
			VALUES ($1::uuid, $2, $3, $4::date, 'todo', $5)
			RETURNING id::text
		`, orderID, s.Type, s.Title, dueKey, now).Scan(&taskID)
		if err != nil {
			return nil, err
		}
		out = append(out, TaskItem{
			ID:        taskID,
			OrderID:   orderID,
			Type:      s.Type,
			Title:     s.Title,
			DueDate:   dueKey,
			Status:    "todo",
			CreatedAt: now,
		})
	}
	return out, nil
}

// TransitionOrderInput adalah masukan untuk POST /api/v1/orders/:id/transition (Bagian 8.5).
type TransitionOrderInput struct {
	OrderID       string
	ActorID       string
	ActorRole     string
	ToStatus      string
	ActualMinutes *int
}

// TransitionOrder memvalidasi dan menjalankan transisi status pesanan sesuai Bagian 8.5:
//   - confirmed -> in_production (manager, owner)
//   - in_production -> ready (staff, manager, owner; tugas produce selesai & stok bahan berkurang)
//   - ready -> delivering (staff, manager, owner)
//   - delivering -> completed (manager, owner; wajib paid >= total)
//   - ready -> completed (manager, owner; wajib paid >= total, mis. untuk pickup)
func TransitionOrder(ctx context.Context, conn db.Conn, req TransitionOrderInput) (*OrderDetail, error) {
	if req.ActorRole != auth.RoleOwner && req.ActorRole != auth.RoleManager && req.ActorRole != auth.RoleStaff {
		return nil, httpx.Err(http.StatusForbidden, "forbidden", "Anda tidak memiliki izin untuk mengubah status pesanan.")
	}

	var out *OrderDetail
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		now := time.Now().UTC()
		var (
			code, curStatus string
			total, paid     decimal.Decimal
		)
		err := tx.QueryRow(ctx, `
			SELECT code, status, total, paid
			FROM orders
			WHERE id = $1::uuid
			FOR UPDATE
		`, req.OrderID).Scan(&code, &curStatus, &total, &paid)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Pesanan tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		// Validasi transisi status (Bagian 8.5)
		validTransition := false
		staffAllowed := false
		switch curStatus {
		case StatusConfirmed:
			if req.ToStatus == StatusInProduction {
				validTransition = true
				staffAllowed = false
			}
		case StatusInProduction:
			if req.ToStatus == StatusReady {
				validTransition = true
				staffAllowed = true
			}
		case StatusReady:
			if req.ToStatus == StatusDelivering {
				validTransition = true
				staffAllowed = true
			} else if req.ToStatus == StatusCompleted {
				validTransition = true
				staffAllowed = false
			}
		case StatusDelivering:
			if req.ToStatus == StatusCompleted {
				validTransition = true
				staffAllowed = false
			}
		}

		if !validTransition {
			return httpx.Err(http.StatusUnprocessableEntity, "validation_error",
				fmt.Sprintf("Transisi status dari %q ke %q tidak valid.", curStatus, req.ToStatus))
		}
		if req.ActorRole == auth.RoleStaff && !staffAllowed {
			return httpx.Err(http.StatusForbidden, "forbidden",
				"Karyawan hanya berhak mengubah status dari Dalam Produksi ke Siap, atau dari Siap ke Diantar/Siap Diambil.")
		}

		// Bagian 8.4: Saat paid >= total pada status delivering atau ready -> boleh completed
		if req.ToStatus == StatusCompleted && paid.LessThan(total) {
			return httpx.Err(http.StatusUnprocessableEntity, "validation_error",
				"Pesanan belum dapat diselesaikan (completed) karena pembayaran belum lunas.")
		}

		// Sinkronisasi tugas dan pengurangan stok bahan (Bagian 8.5)
		switch req.ToStatus {
		case StatusInProduction:
			_, err := tx.Exec(ctx, `
				UPDATE tasks SET status = 'doing'
				WHERE order_id = $1::uuid AND type = 'produce' AND status = 'todo'
			`, req.OrderID)
			if err != nil {
				return err
			}

		case StatusReady:
			if req.ActualMinutes != nil {
				_, err = tx.Exec(ctx, `
					UPDATE tasks SET status = 'done', actual_minutes = $2
					WHERE order_id = $1::uuid AND type = 'produce'
				`, req.OrderID, *req.ActualMinutes)
			} else {
				_, err = tx.Exec(ctx, `
					UPDATE tasks SET status = 'done'
					WHERE order_id = $1::uuid AND type = 'produce'
				`, req.OrderID)
			}
			if err != nil {
				return err
			}
			// Kurangi stok bahan sesuai resep dan catat stock_movements
			if err := deductStockForProduction(ctx, tx, req.OrderID, now); err != nil {
				return err
			}

		case StatusDelivering:
			_, err := tx.Exec(ctx, `
				UPDATE tasks SET status = 'doing'
				WHERE order_id = $1::uuid AND type = 'deliver' AND status = 'todo'
			`, req.OrderID)
			if err != nil {
				return err
			}

		case StatusCompleted:
			_, err := tx.Exec(ctx, `
				UPDATE tasks SET status = 'done'
				WHERE order_id = $1::uuid AND type = 'deliver'
			`, req.OrderID)
			if err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx, `
			UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3::uuid
		`, req.ToStatus, now, req.OrderID); err != nil {
			return err
		}

		if err := audit.Log(ctx, tx, req.ActorID, audit.ActionStatus, "orders", req.OrderID, map[string]any{
			"op":          "transition_order",
			"code":        code,
			"from_status": curStatus,
			"to_status":   req.ToStatus,
		}); err != nil {
			return err
		}

		detail, err := loadOrderDetailTx(ctx, tx, req.OrderID, req.ActorRole)
		if err != nil {
			return err
		}
		out = detail
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// deductStockForProduction mengurangi stok bahan berdasarkan resep produk pesanan
// dan menulis baris stock_movements (Bagian 8.5). Bersifat idempoten per pesanan.
func deductStockForProduction(ctx context.Context, tx pgx.Tx, orderID string, now time.Time) error {
	var alreadyDeducted bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM stock_movements
			WHERE ref_type = 'order' AND ref_id = $1::uuid AND reason = 'production'
		)
	`, orderID).Scan(&alreadyDeducted); err != nil {
		return err
	}
	if alreadyDeducted {
		return nil
	}

	rows, err := tx.Query(ctx, `
		SELECT ri.ingredient_id::text, COALESCE(SUM(oi.qty * ri.qty_per_unit), 0)
		FROM order_items oi
		JOIN recipe_items ri ON ri.product_id = oi.product_id
		WHERE oi.order_id = $1::uuid
		GROUP BY ri.ingredient_id
	`, orderID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type usage struct {
		IngredientID string
		QtyUsed      decimal.Decimal
	}
	var usages []usage
	for rows.Next() {
		var u usage
		if err := rows.Scan(&u.IngredientID, &u.QtyUsed); err != nil {
			return err
		}
		if u.QtyUsed.IsPositive() {
			usages = append(usages, u)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	for _, u := range usages {
		if _, err := tx.Exec(ctx, `
			UPDATE ingredients
			SET stock = GREATEST(0, stock - $1)
			WHERE id = $2::uuid
		`, u.QtyUsed.Round(4), u.IngredientID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_movements (ingredient_id, delta, reason, ref_type, ref_id, created_at)
			VALUES ($1::uuid, $2, 'production', 'order', $3::uuid, $4)
		`, u.IngredientID, u.QtyUsed.Neg().Round(4), orderID, now); err != nil {
			return err
		}
	}
	return nil
}

// CancelOrderInput adalah masukan untuk POST /api/v1/orders/:id/cancel (Bagian 8.6).
type CancelOrderInput struct {
	OrderID       string
	ActorID       string
	Reason        string
	DPDisposition string // opsional: "forfeited" | "refunded" | "pending"
	Note          string
}

// CancelOrderResult adalah respons pembatalan pesanan beserta eksposur pembatalan (BR-11).
type CancelOrderResult struct {
	OrderID              string          `json:"order_id"`
	Code                 string          `json:"code"`
	PreviousStatus       string          `json:"previous_status"`
	Status               string          `json:"status"`
	ShoppingCost         decimal.Decimal `json:"shopping_cost"`
	DPReceived           decimal.Decimal `json:"dp_received"`
	CancellationExposure decimal.Decimal `json:"cancellation_exposure"`
	DPDisposition        string          `json:"dp_disposition,omitempty"`
	Reason               string          `json:"reason,omitempty"`
	Note                 string          `json:"note,omitempty"`
	CancelledAt          time.Time       `json:"cancelled_at"`
}

// CancelOrder membatalkan pesanan dalam satu transaksi (Bagian 8.6):
//   - status diubah ke 'cancelled' (kapasitas otomatis dilepas)
//   - tugas yang belum selesai dibatalkan (dihapus dari antrian tugas aktif)
//   - eksposur pembatalan dihitung via engine.CancellationExposure
//   - dicatat di audit_logs beserta keputusan DP (bila diisi).
func CancelOrder(ctx context.Context, conn db.Conn, req CancelOrderInput) (*CancelOrderResult, error) {
	var out *CancelOrderResult
	err := db.WithTx(ctx, conn, func(tx pgx.Tx) error {
		var preEventDate, preProdDate time.Time
		err := tx.QueryRow(ctx, `
			SELECT event_date, production_date
			FROM orders
			WHERE id = $1::uuid
		`, req.OrderID).Scan(&preEventDate, &preProdDate)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Pesanan tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		if err := LockDays(ctx, tx, []time.Time{preProdDate, preEventDate}); err != nil {
			return err
		}

		params, err := LoadSettingsParams(ctx, tx)
		if err != nil {
			return err
		}

		var (
			code, curStatus string
			paid            decimal.Decimal
			notesPtr        *string
		)
		err = tx.QueryRow(ctx, `
			SELECT code, status, paid, notes
			FROM orders
			WHERE id = $1::uuid
			FOR UPDATE
		`, req.OrderID).Scan(&code, &curStatus, &paid, &notesPtr)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Err(http.StatusNotFound, "not_found", "Pesanan tidak ditemukan.")
		}
		if err != nil {
			return err
		}

		switch curStatus {
		case StatusCompleted, StatusCancelled, StatusRejected, StatusExpired:
			return httpx.Err(http.StatusConflict, "conflict",
				fmt.Sprintf("Pesanan berstatus %q tidak dapat dibatalkan.", curStatus))
		}

		// Hitung total biaya belanja pesanan dan eksposur pembatalan (BR-11)
		_, evalItems, _, err := loadOrderItemsAndEval(ctx, tx, req.OrderID, params)
		if err != nil {
			return err
		}
		shoppingCost := decimal.Zero
		for _, it := range evalItems {
			q := decimal.NewFromInt(int64(it.Qty))
			shoppingCost = shoppingCost.Add(q.Mul(it.PurchaseCostPerUnit))
		}
		exposure := engine.CancellationExposure(shoppingCost, paid)

		// Batalkan tugas yang belum selesai untuk pesanan ini
		if _, err := tx.Exec(ctx, `
			DELETE FROM tasks
			WHERE order_id = $1::uuid AND status != 'done'
		`, req.OrderID); err != nil {
			return err
		}

		now := time.Now().UTC()
		newNotes := ""
		if notesPtr != nil {
			newNotes = *notesPtr
		}
		cancelSummary := strings.TrimSpace(req.Reason)
		if strings.TrimSpace(req.Note) != "" {
			if cancelSummary != "" {
				cancelSummary += " — " + strings.TrimSpace(req.Note)
			} else {
				cancelSummary = strings.TrimSpace(req.Note)
			}
		}
		if strings.TrimSpace(req.DPDisposition) != "" {
			if cancelSummary != "" {
				cancelSummary += " [DP: " + strings.TrimSpace(req.DPDisposition) + "]"
			} else {
				cancelSummary = "[DP: " + strings.TrimSpace(req.DPDisposition) + "]"
			}
		}
		if cancelSummary != "" {
			if newNotes != "" {
				newNotes += "\n[Batal: " + cancelSummary + "]"
			} else {
				newNotes = "[Batal: " + cancelSummary + "]"
			}
		}
		var updatedNotesPtr any
		if newNotes != "" {
			updatedNotesPtr = newNotes
		}

		if _, err := tx.Exec(ctx, `
			UPDATE orders
			SET status = $1, notes = $2, updated_at = $3
			WHERE id = $4::uuid
		`, StatusCancelled, updatedNotesPtr, now, req.OrderID); err != nil {
			return err
		}

		if err := audit.Log(ctx, tx, req.ActorID, audit.ActionStatus, "orders", req.OrderID, map[string]any{
			"op":                    "cancel_order",
			"code":                  code,
			"from_status":           curStatus,
			"to_status":             StatusCancelled,
			"shopping_cost":         shoppingCost.Round(2),
			"dp_received":           paid.Round(2),
			"cancellation_exposure": exposure.Round(2),
			"dp_disposition":        strings.TrimSpace(req.DPDisposition),
			"reason":                strings.TrimSpace(req.Reason),
			"note":                  strings.TrimSpace(req.Note),
		}); err != nil {
			return err
		}

		out = &CancelOrderResult{
			OrderID:              req.OrderID,
			Code:                 code,
			PreviousStatus:       curStatus,
			Status:               StatusCancelled,
			ShoppingCost:         shoppingCost.Round(2),
			DPReceived:           paid.Round(2),
			CancellationExposure: exposure.Round(2),
			DPDisposition:        strings.TrimSpace(req.DPDisposition),
			Reason:               strings.TrimSpace(req.Reason),
			Note:                 strings.TrimSpace(req.Note),
			CancelledAt:          now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DecisionQueueItem adalah elemen respons GET /api/v1/decision-queue (Bagian 9).
type DecisionQueueItem struct {
	ID                   string            `json:"id"`
	Code                 string            `json:"code"`
	Source               string            `json:"source"`
	CustomerName         string            `json:"customer_name"`
	CustomerContact      string            `json:"customer_contact"`
	DeliveryType         string            `json:"delivery_type"`
	EventDate            string            `json:"event_date"`
	ProductionDate       string            `json:"production_date"`
	Status               string            `json:"status"`
	Total                decimal.Decimal   `json:"total"`
	TotalUnits           int               `json:"total_units"`
	DPRequired           decimal.Decimal   `json:"dp_required"`
	IsBigOrder           bool              `json:"is_big_order"`
	CanDecide            bool              `json:"can_decide"`
	Recommendation       string            `json:"recommendation"`
	RecommendationReason string            `json:"recommendation_reason"`
	CounterOffer         *CounterOfferDTO  `json:"counter_offer"`
	Evaluation           EvaluationDTO     `json:"evaluation"`
	Items                []OrderItemDetail `json:"items"`
	CreatedAt            time.Time         `json:"created_at"`
}

// GetDecisionQueue memuat daftar pesanan berstatus pending_decision beserta
// pratinjau dampak terkini (utilisasi, margin, titik terendah kas sebelum & sesudah).
func GetDecisionQueue(ctx context.Context, conn db.Conn, actorRole string) ([]DecisionQueueItem, error) {
	if _, err := ExpireOverdueOrders(ctx, conn, time.Now().UTC()); err != nil {
		return nil, err
	}
	params, err := LoadSettingsParams(ctx, conn)
	if err != nil {
		return nil, err
	}
	today := db.TodayQ(ctx, conn)

	rows, err := conn.Query(ctx, `
		SELECT id::text, code, source, customer_name, customer_contact, delivery_type,
		       event_date, production_date, status, total, dp_required, created_at
		FROM orders
		WHERE status = 'pending_decision'
		ORDER BY event_date ASC, created_at ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rawOrder struct {
		ID              string
		Code            string
		Source          string
		CustomerName    string
		CustomerContact string
		DeliveryType    string
		EventDate       time.Time
		ProductionDate  time.Time
		Status          string
		Total           decimal.Decimal
		DPRequired      decimal.Decimal
		CreatedAt       time.Time
	}
	var pending []rawOrder
	for rows.Next() {
		var r rawOrder
		if err := rows.Scan(
			&r.ID, &r.Code, &r.Source, &r.CustomerName, &r.CustomerContact, &r.DeliveryType,
			&r.EventDate, &r.ProductionDate, &r.Status, &r.Total, &r.DPRequired, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		pending = append(pending, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	out := make([]DecisionQueueItem, 0, len(pending))
	for _, r := range pending {
		itemDetails, evalItems, totalUnits, err := loadOrderItemsAndEval(ctx, conn, r.ID, params)
		if err != nil {
			return nil, err
		}
		evalRes, err := EvaluateOrder(ctx, conn, today, r.EventDate, r.ProductionDate, evalItems, params)
		if err != nil {
			return nil, err
		}
		evalDTO := ToEvaluationDTO(evalRes)
		isBig := evalRes.Total.GreaterThanOrEqual(params.BigOrderTotal) || totalUnits >= params.BigOrderUnits
		canDecide := actorRole == auth.RoleOwner || (actorRole == auth.RoleManager && !isBig)

		out = append(out, DecisionQueueItem{
			ID:                   r.ID,
			Code:                 r.Code,
			Source:               r.Source,
			CustomerName:         r.CustomerName,
			CustomerContact:      r.CustomerContact,
			DeliveryType:         r.DeliveryType,
			EventDate:            engine.DateKey(r.EventDate),
			ProductionDate:       engine.DateKey(r.ProductionDate),
			Status:               r.Status,
			Total:                evalRes.Total.Round(2),
			TotalUnits:           totalUnits,
			DPRequired:           evalRes.DPCharged.Round(2),
			IsBigOrder:           isBig,
			CanDecide:            canDecide,
			Recommendation:       evalRes.Recommendation,
			RecommendationReason: evalRes.Reason,
			CounterOffer:         evalDTO.CounterOffer,
			Evaluation:           evalDTO,
			Items:                itemDetails,
			CreatedAt:            r.CreatedAt,
		})
	}
	return out, nil
}

// CapacityCalendarDay adalah ringkasan kapasitas satu hari pada GET /api/v1/capacity/calendar.
type CapacityCalendarDay struct {
	Date        string                     `json:"date"`
	Status      string                     `json:"status"`
	Utilization decimal.Decimal            `json:"utilization"`
	Bottleneck  string                     `json:"bottleneck"`
	Loads       map[string]decimal.Decimal `json:"loads"`
	Capacities  map[string]decimal.Decimal `json:"capacities"`
}

// CapacityResourceDetail adalah rincian beban & kapasitas per sumber daya pada satu hari.
type CapacityResourceDetail struct {
	Code        string          `json:"code"`
	Name        string          `json:"name"`
	Unit        string          `json:"unit"`
	Load        decimal.Decimal `json:"load"`
	Capacity    decimal.Decimal `json:"capacity"`
	Utilization decimal.Decimal `json:"utilization"`
}

// CapacityOrderItem adalah ringkasan pesanan yang mereservasi kapasitas pada tanggal tertentu.
type CapacityOrderItem struct {
	ID             string          `json:"id"`
	Code           string          `json:"code"`
	CustomerName   string          `json:"customer_name"`
	Status         string          `json:"status"`
	EventDate      string          `json:"event_date"`
	ProductionDate string          `json:"production_date"`
	Total          decimal.Decimal `json:"total"`
}

// CapacityDayDetail adalah respons GET /api/v1/capacity/day/:date.
type CapacityDayDetail struct {
	Date        string                   `json:"date"`
	Status      string                   `json:"status"`
	Utilization decimal.Decimal          `json:"utilization"`
	Bottleneck  string                   `json:"bottleneck"`
	Resources   []CapacityResourceDetail `json:"resources"`
	Orders      []CapacityOrderItem      `json:"orders"`
}

// GetCapacityCalendar mengembalikan kalender kapasitas bulanan (YYYY-MM) (Bagian 9).
func GetCapacityCalendar(ctx context.Context, conn db.Conn, monthStr string) ([]CapacityCalendarDay, error) {
	if _, err := ExpireOverdueOrders(ctx, conn, time.Now().UTC()); err != nil {
		return nil, err
	}
	params, err := LoadSettingsParams(ctx, conn)
	if err != nil {
		return nil, err
	}

	var startDate time.Time
	if strings.TrimSpace(monthStr) == "" {
		today := db.TodayQ(ctx, conn)
		startDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	} else {
		parsed, err := time.Parse("2006-01", strings.TrimSpace(monthStr))
		if err != nil {
			return nil, httpx.Err(http.StatusUnprocessableEntity, "validation_error", "Parameter month harus berformat YYYY-MM.")
		}
		startDate = time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	endDate := startDate.AddDate(0, 1, -1)

	caps, err := LoadCapacities(ctx, conn)
	if err != nil {
		return nil, err
	}
	existingMap, err := LoadExistingLoads(ctx, conn, startDate, endDate)
	if err != nil {
		return nil, err
	}

	roundedCaps := make(map[string]decimal.Decimal, len(caps))
	for k, v := range caps {
		roundedCaps[k] = v.Round(2)
	}

	days := make([]CapacityCalendarDay, 0, 31)
	for cur := startDate; !cur.After(endDate); cur = cur.AddDate(0, 0, 1) {
		dk := engine.DateKey(cur)
		loadItems := make([]engine.LoadItem, 0, len(caps))
		for code, ld := range existingMap[dk] {
			loadItems = append(loadItems, engine.LoadItem{
				ResourceCode: code,
				Qty:          decimal.NewFromInt(1),
				Load:         ld,
			})
		}
		dayLoads := engine.DayLoads(loadItems)
		util, bottleneck := engine.DayUtilization(dayLoads, caps)
		st := engine.DayStatus(util, params.StatusGreenBelow, params.StatusYellowBelow)

		loadsOut := make(map[string]decimal.Decimal, len(caps))
		for code := range caps {
			loadsOut[code] = dayLoads[code].Round(2)
		}

		days = append(days, CapacityCalendarDay{
			Date:        dk,
			Status:      st,
			Utilization: util.Round(4),
			Bottleneck:  bottleneck,
			Loads:       loadsOut,
			Capacities:  roundedCaps,
		})
	}
	return days, nil
}

// GetCapacityDay mengembalikan rincian kapasitas per sumber daya dan daftar pesanan
// pada satu tanggal produksi (GET /api/v1/capacity/day/:date).
func GetCapacityDay(ctx context.Context, conn db.Conn, date time.Time) (*CapacityDayDetail, error) {
	if _, err := ExpireOverdueOrders(ctx, conn, time.Now().UTC()); err != nil {
		return nil, err
	}
	params, err := LoadSettingsParams(ctx, conn)
	if err != nil {
		return nil, err
	}

	dk := engine.DateKey(date)
	existingMap, err := LoadExistingLoads(ctx, conn, date, date)
	if err != nil {
		return nil, err
	}

	// Muat daftar resources lengkap dengan nama & unit
	resRows, err := conn.Query(ctx, `
		SELECT code, name, unit, count, amount_per_unit_day, efficiency
		FROM resources
		ORDER BY code ASC
	`)
	if err != nil {
		return nil, err
	}
	defer resRows.Close()

	type resInfo struct {
		Code string
		Name string
		Unit string
		Cap  decimal.Decimal
	}
	var resList []resInfo
	caps := make(map[string]decimal.Decimal)
	for resRows.Next() {
		var r engine.Resource
		var name, unit string
		if err := resRows.Scan(&r.Code, &name, &unit, &r.Count, &r.AmountPerUnitDay, &r.Efficiency); err != nil {
			return nil, err
		}
		c := engine.Capacity(r)
		caps[r.Code] = c
		resList = append(resList, resInfo{Code: r.Code, Name: name, Unit: unit, Cap: c})
	}
	if err := resRows.Err(); err != nil {
		return nil, err
	}
	resRows.Close()
	sort.Slice(resList, func(i, j int) bool { return resList[i].Code < resList[j].Code })

	loadItems := make([]engine.LoadItem, 0, len(resList))
	for code, ld := range existingMap[dk] {
		loadItems = append(loadItems, engine.LoadItem{
			ResourceCode: code,
			Qty:          decimal.NewFromInt(1),
			Load:         ld,
		})
	}
	dayLoads := engine.DayLoads(loadItems)
	util, bottleneck := engine.DayUtilization(dayLoads, caps)
	st := engine.DayStatus(util, params.StatusGreenBelow, params.StatusYellowBelow)

	details := make([]CapacityResourceDetail, 0, len(resList))
	for _, ri := range resList {
		ld := dayLoads[ri.Code]
		u := engine.Utilization(ld, ri.Cap)
		details = append(details, CapacityResourceDetail{
			Code:        ri.Code,
			Name:        ri.Name,
			Unit:        ri.Unit,
			Load:        ld.Round(2),
			Capacity:    ri.Cap.Round(2),
			Utilization: u.Round(4),
		})
	}

	// Muat daftar pesanan yang mereservasi kapasitas pada tanggal produksi ini
	ordRows, err := conn.Query(ctx, `
		SELECT id::text, code, customer_name, status, event_date, production_date, total
		FROM orders
		WHERE production_date = $1::date
		  AND status IN ('accepted', 'confirmed', 'in_production')
		ORDER BY created_at ASC, id ASC
	`, dk)
	if err != nil {
		return nil, err
	}
	defer ordRows.Close()

	ordList := make([]CapacityOrderItem, 0)
	for ordRows.Next() {
		var o CapacityOrderItem
		var ev, pr time.Time
		if err := ordRows.Scan(&o.ID, &o.Code, &o.CustomerName, &o.Status, &ev, &pr, &o.Total); err != nil {
			return nil, err
		}
		o.EventDate = engine.DateKey(ev)
		o.ProductionDate = engine.DateKey(pr)
		o.Total = o.Total.Round(2)
		ordList = append(ordList, o)
	}
	if err := ordRows.Err(); err != nil {
		return nil, err
	}

	return &CapacityDayDetail{
		Date:        dk,
		Status:      st,
		Utilization: util.Round(4),
		Bottleneck:  bottleneck,
		Resources:   details,
		Orders:      ordList,
	}, nil
}

// GetOrderByID memuat detail pesanan berdasarkan ID dan menyaring informasi harga/margin/kas
// bila pemanggil berperan staff.
func GetOrderByID(ctx context.Context, conn db.Conn, orderID, actorRole string) (*OrderDetail, error) {
	if _, err := ExpireOverdueOrders(ctx, conn, time.Now().UTC()); err != nil {
		return nil, err
	}
	return loadOrderDetailTx(ctx, conn, orderID, actorRole)
}

// ListOrders memuat daftar pesanan internal (disaring bila peran staff).
func ListOrders(ctx context.Context, conn db.Conn, statusFilter, actorRole string) ([]OrderDetail, error) {
	if _, err := ExpireOverdueOrders(ctx, conn, time.Now().UTC()); err != nil {
		return nil, err
	}

	rows, err := conn.Query(ctx, `
		SELECT id::text
		FROM orders
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	out := make([]OrderDetail, 0, len(ids))
	for _, id := range ids {
		d, err := loadOrderDetailTx(ctx, conn, id, actorRole)
		if err != nil {
			return nil, err
		}
		if statusFilter != "" && d.Status != statusFilter {
			continue
		}
		out = append(out, *d)
	}
	return out, nil
}

func loadOrderDetailTx(ctx context.Context, q db.DBTX, orderID, actorRole string) (*OrderDetail, error) {
	var (
		id, code, source, custName, custContact, deliveryType string
		custIDPtr, addrPtr, notesPtr, recPtr, recReasonPtr    *string
		eventDate, prodDate                                   time.Time
		status                                                string
		total, dpRequired, paid                               decimal.Decimal
		dpDeadline                                            *time.Time
		counterBytes                                          []byte
		createdAt, updatedAt                                  time.Time
	)
	err := q.QueryRow(ctx, `
		SELECT id::text, code, source, customer_id::text, customer_name, customer_contact, delivery_type,
		       address, notes, event_date, production_date,
		       status, total, dp_required, dp_deadline, paid,
		       recommendation, recommendation_reason, counter_offer,
		       created_at, updated_at
		FROM orders
		WHERE id = $1::uuid
	`, orderID).Scan(
		&id, &code, &source, &custIDPtr, &custName, &custContact, &deliveryType,
		&addrPtr, &notesPtr, &eventDate, &prodDate,
		&status, &total, &dpRequired, &dpDeadline, &paid,
		&recPtr, &recReasonPtr, &counterBytes,
		&createdAt, &updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.Err(http.StatusNotFound, "not_found", "Pesanan tidak ditemukan.")
	}
	if err != nil {
		return nil, err
	}

	rows, err := q.Query(ctx, `
		SELECT oi.id::text, oi.product_id::text, p.name, oi.qty, oi.unit_price, oi.unit_cost
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = $1::uuid
		ORDER BY oi.id ASC
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]OrderItemDetail, 0)
	for rows.Next() {
		var it OrderItemDetail
		if err := rows.Scan(&it.ID, &it.ProductID, &it.ProductName, &it.Qty, &it.UnitPrice, &it.UnitCost); err != nil {
			return nil, err
		}
		if actorRole == auth.RoleStaff {
			it.UnitPrice = decimal.Zero
			it.UnitCost = decimal.Zero
			it.Subtotal = decimal.Zero
		} else {
			it.UnitPrice = it.UnitPrice.Round(2)
			it.UnitCost = it.UnitCost.Round(2)
			it.Subtotal = it.UnitPrice.Mul(decimal.NewFromInt(int64(it.Qty))).Round(2)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var addrStr, notesStr, recStr, recReasonStr string
	if addrPtr != nil {
		addrStr = *addrPtr
	}
	if notesPtr != nil {
		notesStr = *notesPtr
	}
	if recPtr != nil {
		recStr = *recPtr
	}
	if recReasonPtr != nil {
		recReasonStr = *recReasonPtr
	}

	var co *CounterOfferDTO
	if actorRole != auth.RoleStaff && len(counterBytes) > 0 && string(counterBytes) != "null" {
		var parsed CounterOfferDTO
		if err := json.Unmarshal(counterBytes, &parsed); err == nil && parsed.Date != "" {
			co = &parsed
		}
	}

	detail := &OrderDetail{
		ID:                   id,
		Code:                 code,
		Source:               source,
		CustomerID:           custIDPtr,
		CustomerName:         custName,
		CustomerContact:      custContact,
		DeliveryType:         deliveryType,
		Address:              addrStr,
		Notes:                notesStr,
		EventDate:            engine.DateKey(eventDate),
		ProductionDate:       engine.DateKey(prodDate),
		Status:               status,
		Total:                total.Round(2),
		DPRequired:           dpRequired.Round(2),
		DPDeadline:           dpDeadline,
		Paid:                 paid.Round(2),
		Recommendation:       recStr,
		RecommendationReason: recReasonStr,
		CounterOffer:         co,
		Items:                items,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}

	if actorRole == auth.RoleStaff {
		detail.Total = decimal.Zero
		detail.DPRequired = decimal.Zero
		detail.Paid = decimal.Zero
		detail.Recommendation = ""
		detail.RecommendationReason = ""
	}
	return detail, nil
}

// ListCustomerOrders memuat daftar pesanan milik pelanggan tertentu (GET /api/v1/me/orders)
// hanya dengan field aman (PublicTrackResponse, tanpa unit_cost/margin/snapshot).
func ListCustomerOrders(ctx context.Context, conn db.Conn, customerID string) ([]PublicTrackResponse, error) {
	if _, err := ExpireOverdueOrders(ctx, conn, time.Now().UTC()); err != nil {
		return nil, err
	}

	rows, err := conn.Query(ctx, `
		SELECT id::text, code, customer_name, delivery_type, event_date, status,
		       total, dp_required, dp_deadline, paid, counter_offer,
		       created_at, updated_at
		FROM orders
		WHERE customer_id = $1::uuid
		ORDER BY created_at DESC, id DESC
	`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type rowRec struct {
		ID           string
		Track        PublicTrackResponse
		EventDate    time.Time
		CounterBytes []byte
	}
	var rawList []rowRec
	for rows.Next() {
		var r rowRec
		if err := rows.Scan(
			&r.ID, &r.Track.Code, &r.Track.CustomerName, &r.Track.DeliveryType, &r.EventDate, &r.Track.Status,
			&r.Track.Total, &r.Track.DPRequired, &r.Track.DPDeadline, &r.Track.Paid, &r.CounterBytes,
			&r.Track.CreatedAt, &r.Track.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rawList = append(rawList, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	out := make([]PublicTrackResponse, 0, len(rawList))
	for _, r := range rawList {
		tr := r.Track
		tr.EventDate = engine.DateKey(r.EventDate)
		tr.Total = tr.Total.Round(2)
		tr.DPRequired = tr.DPRequired.Round(2)
		tr.Paid = tr.Paid.Round(2)
		if len(r.CounterBytes) > 0 && string(r.CounterBytes) != "null" {
			var co CounterOfferDTO
			if err := json.Unmarshal(r.CounterBytes, &co); err == nil && co.Date != "" {
				tr.CounterOffer = &co
			}
		}

		itemRows, err := conn.Query(ctx, `
			SELECT oi.product_id::text, p.name, oi.qty, oi.unit_price
			FROM order_items oi
			JOIN products p ON p.id = oi.product_id
			WHERE oi.order_id = $1::uuid
			ORDER BY oi.id ASC
		`, r.ID)
		if err != nil {
			return nil, err
		}
		items := make([]PublicOrderItem, 0)
		for itemRows.Next() {
			var it PublicOrderItem
			if err := itemRows.Scan(&it.ProductID, &it.ProductName, &it.Qty, &it.UnitPrice); err != nil {
				itemRows.Close()
				return nil, err
			}
			it.UnitPrice = it.UnitPrice.Round(2)
			it.Subtotal = it.UnitPrice.Mul(decimal.NewFromInt(int64(it.Qty))).Round(2)
			items = append(items, it)
		}
		if err := itemRows.Err(); err != nil {
			itemRows.Close()
			return nil, err
		}
		itemRows.Close()

		tr.Items = items
		out = append(out, tr)
	}

	return out, nil
}

func loadOrderItemsAndEval(ctx context.Context, q db.DBTX, orderID string, params SettingsParams) ([]OrderItemDetail, []engine.EvalItem, int, error) {
	rows, err := q.Query(ctx, `
		SELECT oi.id::text, oi.product_id::text, p.name, oi.qty, oi.unit_price, oi.unit_cost
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = $1::uuid
		ORDER BY oi.id ASC
	`, orderID)
	if err != nil {
		return nil, nil, 0, err
	}
	defer rows.Close()

	var itemDetails []OrderItemDetail
	var pids []string
	seen := make(map[string]bool)
	for rows.Next() {
		var it OrderItemDetail
		if err := rows.Scan(&it.ID, &it.ProductID, &it.ProductName, &it.Qty, &it.UnitPrice, &it.UnitCost); err != nil {
			return nil, nil, 0, err
		}
		it.Subtotal = it.UnitPrice.Mul(decimal.NewFromInt(int64(it.Qty))).Round(2)
		itemDetails = append(itemDetails, it)
		if !seen[it.ProductID] {
			seen[it.ProductID] = true
			pids = append(pids, it.ProductID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, 0, err
	}
	rows.Close()

	prodMeta, err := LoadProductsMeta(ctx, q, pids, params)
	if err != nil {
		return nil, nil, 0, err
	}

	evalItems := make([]engine.EvalItem, 0, len(itemDetails))
	totalUnits := 0
	for i := range itemDetails {
		it := &itemDetails[i]
		pm := prodMeta[it.ProductID]
		it.UnitCost = pm.UnitCost
		evalItems = append(evalItems, engine.EvalItem{
			ProductID:           it.ProductID,
			Qty:                 it.Qty,
			UnitPrice:           it.UnitPrice,
			UnitCost:            pm.UnitCost,
			PurchaseCostPerUnit: pm.PurchaseCostPerUnit,
			UnitLoads:           pm.UnitLoads,
		})
		totalUnits += it.Qty
	}
	return itemDetails, evalItems, totalUnits, nil
}

func replaceOrderEvaluations(ctx context.Context, tx pgx.Tx, orderID string, evalRes engine.EvalResult) error {
	if _, err := tx.Exec(ctx, `DELETE FROM order_evaluations WHERE order_id = $1::uuid`, orderID); err != nil {
		return err
	}
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
	return nil
}

func toPublicOrderItems(items []OrderItemDetail) []PublicOrderItem {
	out := make([]PublicOrderItem, len(items))
	for i, it := range items {
		out[i] = PublicOrderItem{
			ProductID:   it.ProductID,
			ProductName: it.ProductName,
			Qty:         it.Qty,
			UnitPrice:   it.UnitPrice.Round(2),
			Subtotal:    it.Subtotal.Round(2),
		}
	}
	return out
}

