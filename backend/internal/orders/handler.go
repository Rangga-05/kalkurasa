package orders

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/httpx"
)

// Handler menyediakan endpoint pesanan publik dan manual (Tahap 3A).
type Handler struct {
	DB db.Conn
}

// PublicProduct adalah tampilan katalog produk aktif untuk pelanggan publik (tanpa biaya/margin).
type PublicProduct struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Category string          `json:"category"`
	Price    decimal.Decimal `json:"price"`
}

// PublicProducts menangani GET /api/v1/public/products.
func (h *Handler) PublicProducts(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	rows, err := h.DB.Query(c.Request.Context(), `
		SELECT id::text, name, category, price
		FROM products
		WHERE is_active = true
		ORDER BY category ASC, name ASC, id ASC
	`)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	defer rows.Close()

	items := make([]PublicProduct, 0)
	for rows.Next() {
		var p PublicProduct
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price); err != nil {
			httpx.HandleError(c, err)
			return
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, items)
}

type availabilityItemReq struct {
	ProductID string `json:"product_id"`
	Qty       *int   `json:"qty"`
}

type availabilityRequest struct {
	Items []availabilityItemReq `json:"items"`
	From  string                `json:"from"`
	To    string                `json:"to"`
}

// PublicAvailability menangani POST /api/v1/public/availability.
// Body: {items:[{product_id,qty}], from, to} -> daftar hari {date, status, utilization_after, available} (G1 & G4).
func (h *Handler) PublicAvailability(c *gin.Context) {
	var req availabilityRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	v := httpx.NewValidation()
	if len(req.Items) == 0 {
		v.Add("Minimal satu produk wajib dipilih.")
	} else if len(req.Items) > 50 {
		v.Add("Maksimal 50 baris item dalam satu pengecekan.")
	}

	cleanItems := make([]OrderItemInput, 0, len(req.Items))
	for i, it := range req.Items {
		pid := strings.ToLower(strings.TrimSpace(it.ProductID))
		if !httpx.ValidUUID(pid) {
			v.Add(fmt.Sprintf("Item #%d: product_id harus berformat UUID yang valid.", i+1))
		}
		qty := v.Int(fmt.Sprintf("Jumlah item #%d (qty)", i+1), it.Qty, 1, 100000)
		cleanItems = append(cleanItems, OrderItemInput{ProductID: pid, Qty: qty})
	}

	fromDate, errFrom := db.ParseDate(strings.TrimSpace(req.From))
	if errFrom != nil {
		v.Add("Tanggal awal (from) harus berformat YYYY-MM-DD.")
	}
	toDate, errTo := db.ParseDate(strings.TrimSpace(req.To))
	if errTo != nil {
		v.Add("Tanggal akhir (to) harus berformat YYYY-MM-DD.")
	}
	if errFrom == nil && errTo == nil {
		if toDate.Before(fromDate) {
			v.Add("Tanggal akhir (to) tidak boleh sebelum tanggal awal (from).")
		} else if toDate.Sub(fromDate).Hours()/24 > 62 {
			v.Add("Rentang pengecekan ketersediaan maksimal 62 hari.")
		}
	}

	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	if !EnforceRateLimit(c, h.DB, "avail", PublicAvailRateLimitPerMin) {
		return
	}

	days, err := CheckAvailability(c.Request.Context(), h.DB, cleanItems, fromDate, toDate)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, days)
}

type orderItemReq struct {
	ProductID string           `json:"product_id"`
	Qty       *int             `json:"qty"`
	UnitPrice *decimal.Decimal `json:"unit_price,omitempty"`
}

type createOrderRequest struct {
	CustomerName    string         `json:"customer_name"`
	CustomerContact string         `json:"customer_contact"`
	DeliveryType    string         `json:"delivery_type"`
	Address         string         `json:"address"`
	Notes           string         `json:"notes"`
	EventDate       string         `json:"event_date"`
	ProductionDate  string         `json:"production_date"`
	Items           []orderItemReq `json:"items"`
}

func validateOrderRequest(req createOrderRequest, allowManualFields bool) (CreateOrderInput, *httpx.Validation) {
	v := httpx.NewValidation()
	custName := v.Str("Nama pelanggan", req.CustomerName, 120)
	custContact := v.Str("Kontak pelanggan", req.CustomerContact, 80)
	deliveryType := v.OneOf("Metode pengambilan (delivery_type)", strings.TrimSpace(req.DeliveryType), "delivery", "pickup")
	var address string
	if deliveryType == "delivery" {
		address = v.Str("Alamat pengantaran", req.Address, 500)
	} else {
		address = v.OptStr("Alamat pengantaran", req.Address, 500)
	}
	notes := v.OptStr("Catatan", req.Notes, 1000)

	eventDate, errEv := db.ParseDate(strings.TrimSpace(req.EventDate))
	if errEv != nil {
		v.Add("Tanggal acara (event_date) wajib diisi dengan format YYYY-MM-DD.")
	}

	var prodDate time.Time
	if strings.TrimSpace(req.ProductionDate) != "" {
		if !allowManualFields {
			v.Add("Tanggal produksi (production_date) hanya dapat diatur pada pesanan manual.")
		} else {
			var errPr error
			prodDate, errPr = db.ParseDate(strings.TrimSpace(req.ProductionDate))
			if errPr != nil {
				v.Add("Tanggal produksi (production_date) harus berformat YYYY-MM-DD.")
			} else if errEv == nil && prodDate.After(eventDate) {
				v.Add("Tanggal produksi tidak boleh setelah tanggal acara.")
			}
		}
	}

	if len(req.Items) == 0 {
		v.Add("Pesanan wajib memiliki minimal satu item produk.")
	} else if len(req.Items) > 50 {
		v.Add("Maksimal 50 baris item dalam satu pesanan.")
	}

	cleanItems := make([]OrderItemInput, 0, len(req.Items))
	for i, it := range req.Items {
		pid := strings.ToLower(strings.TrimSpace(it.ProductID))
		if !httpx.ValidUUID(pid) {
			v.Add(fmt.Sprintf("Item #%d: product_id harus berformat UUID yang valid.", i+1))
		}
		qty := v.Int(fmt.Sprintf("Jumlah item #%d (qty)", i+1), it.Qty, 1, 100000)
		var unitPrice *decimal.Decimal
		if it.UnitPrice != nil {
			if !allowManualFields {
				v.Add("Harga satuan khusus hanya dapat diatur pada pesanan manual.")
			} else {
				up := v.Dec(fmt.Sprintf("Harga satuan item #%d", i+1), it.UnitPrice, httpx.MoneyOpts(false, false))
				unitPrice = &up
			}
		}
		cleanItems = append(cleanItems, OrderItemInput{
			ProductID: pid,
			Qty:       qty,
			UnitPrice: unitPrice,
		})
	}

	return CreateOrderInput{
		CustomerName:    custName,
		CustomerContact: custContact,
		DeliveryType:    deliveryType,
		Address:         address,
		Notes:           notes,
		EventDate:       eventDate,
		ProductionDate:  prodDate,
		Items:           cleanItems,
	}, v
}

// PublicOrderSummary adalah respons ringkas dan aman untuk POST /public/orders (Bagian 9 & Tahap 4A+).
type PublicOrderSummary struct {
	ID             string            `json:"id"`
	Code           string            `json:"code"`
	CustomerID     *string           `json:"customer_id,omitempty"`
	Status         string            `json:"status"`
	Recommendation string            `json:"recommendation"`
	EventDate      string            `json:"event_date"`
	Total          decimal.Decimal   `json:"total"`
	DPRequired     decimal.Decimal   `json:"dp_required"`
	DPDeadline     *time.Time        `json:"dp_deadline"`
	CounterOffer   *CounterOfferDTO  `json:"counter_offer,omitempty"`
	Items          []PublicOrderItem `json:"items"`
	CreatedAt      time.Time         `json:"created_at"`
}

// PublicOrderItem adalah item pesanan tanpa unit_cost (aman untuk pelanggan publik).
type PublicOrderItem struct {
	ProductID   string          `json:"product_id"`
	ProductName string          `json:"product_name"`
	Qty         int             `json:"qty"`
	UnitPrice   decimal.Decimal `json:"unit_price"`
	Subtotal    decimal.Decimal `json:"subtotal"`
}

// CreatePublicOrder menangani POST /api/v1/public/orders (wajib login sebagai customer; dibatasi laju 30/menit/IP via tabel rate_limits).
func (h *Handler) CreatePublicOrder(c *gin.Context) {
	actor := auth.CurrentUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "Autentikasi diperlukan. Silakan masuk sebagai pelanggan terlebih dahulu.")
		return
	}
	if actor.Role != auth.RoleCustomer {
		httpx.Forbidden(c, "Anda tidak memiliki izin untuk membuat pesanan sebagai pelanggan.")
		return
	}

	var req createOrderRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if strings.TrimSpace(req.CustomerName) == "" && strings.TrimSpace(actor.Name) != "" {
		req.CustomerName = actor.Name
	}
	if strings.TrimSpace(req.CustomerContact) == "" && strings.TrimSpace(actor.Contact) != "" {
		req.CustomerContact = actor.Contact
	}
	in, v := validateOrderRequest(req, false)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	if !EnforceRateLimit(c, h.DB, "order", PublicOrderRateLimitPerMin) {
		return
	}

	in.Source = "public"
	in.CustomerID = actor.ID
	detail, err := CreateOrder(c.Request.Context(), h.DB, in)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	pubItems := make([]PublicOrderItem, len(detail.Items))
	for i, it := range detail.Items {
		pubItems[i] = PublicOrderItem{
			ProductID:   it.ProductID,
			ProductName: it.ProductName,
			Qty:         it.Qty,
			UnitPrice:   it.UnitPrice,
			Subtotal:    it.Subtotal,
		}
	}

	httpx.Created(c, PublicOrderSummary{
		ID:             detail.ID,
		Code:           detail.Code,
		CustomerID:     detail.CustomerID,
		Status:         detail.Status,
		Recommendation: detail.Recommendation,
		EventDate:      detail.EventDate,
		Total:          detail.Total,
		DPRequired:     detail.DPRequired,
		DPDeadline:     detail.DPDeadline,
		CounterOffer:   detail.CounterOffer,
		Items:          pubItems,
		CreatedAt:      detail.CreatedAt,
	})
}

// MyOrders menangani GET /api/v1/me/orders (khusus peran customer).
func (h *Handler) MyOrders(c *gin.Context) {
	actor := auth.CurrentUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "Autentikasi diperlukan. Silakan masuk terlebih dahulu.")
		return
	}
	if actor.Role != auth.RoleCustomer {
		httpx.Forbidden(c, "Hanya akun pelanggan yang dapat melihat daftar pesanan pelanggan.")
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	items, err := ListCustomerOrders(c.Request.Context(), h.DB, actor.ID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, items)
}

// CreateManualOrder menangani POST /api/v1/orders (khusus manager dan owner).
func (h *Handler) CreateManualOrder(c *gin.Context) {
	var req createOrderRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, v := validateOrderRequest(req, true)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	in.Source = "manual"
	if actor != nil {
		in.CreatedBy = actor.ID
	}

	detail, err := CreateOrder(c.Request.Context(), h.DB, in)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.Created(c, detail)
}

// PublicTrackResponse adalah respons GET /public/orders/:code yang hanya
// memuat field aman (status, item tanpa unit_cost, total, tagihan DP, tenggat, tawaran balik).
type PublicTrackResponse struct {
	Code         string            `json:"code"`
	CustomerName string            `json:"customer_name"`
	DeliveryType string            `json:"delivery_type"`
	EventDate    string            `json:"event_date"`
	Status       string            `json:"status"`
	Items        []PublicOrderItem `json:"items"`
	Total        decimal.Decimal   `json:"total"`
	DPRequired   decimal.Decimal   `json:"dp_required"`
	DPDeadline   *time.Time        `json:"dp_deadline"`
	Paid         decimal.Decimal   `json:"paid"`
	CounterOffer *CounterOfferDTO  `json:"counter_offer"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// TrackPublicOrder menangani GET /api/v1/public/orders/:code (Bagian 6 & 9; TC-09 bagian pelanggan).
func (h *Handler) TrackPublicOrder(c *gin.Context) {
	code, ok := NormalizeCode(c.Param("code"))
	if !ok {
		httpx.NotFound(c, "Pesanan dengan kode tersebut tidak ditemukan.")
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	ctx := c.Request.Context()
	if _, err := ExpireOverdueOrders(ctx, h.DB, time.Now().UTC()); err != nil {
		httpx.HandleError(c, err)
		return
	}

	var (
		orderID      string
		out          PublicTrackResponse
		eventDate    time.Time
		counterBytes []byte
		rec          *string
		recReason    *string
	)
	err := h.DB.QueryRow(ctx, `
		SELECT id::text, code, customer_name, delivery_type, event_date, status,
		       total, dp_required, dp_deadline, paid,
		       recommendation, recommendation_reason, counter_offer,
		       created_at, updated_at
		FROM orders
		WHERE code = $1
	`, code).Scan(
		&orderID, &out.Code, &out.CustomerName, &out.DeliveryType, &eventDate, &out.Status,
		&out.Total, &out.DPRequired, &out.DPDeadline, &out.Paid,
		&rec, &recReason, &counterBytes,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.NotFound(c, "Pesanan dengan kode tersebut tidak ditemukan.")
		return
	}
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	out.EventDate = engine.DateKey(eventDate)
	out.Total = out.Total.Round(2)
	out.DPRequired = out.DPRequired.Round(2)
	out.Paid = out.Paid.Round(2)
	if len(counterBytes) > 0 && string(counterBytes) != "null" {
		var co CounterOfferDTO
		if err := json.Unmarshal(counterBytes, &co); err == nil && co.Date != "" {
			out.CounterOffer = &co
		}
	}

	// Ambil item pesanan (tanpa unit_cost!)
	rows, err := h.DB.Query(ctx, `
		SELECT oi.product_id::text, p.name, oi.qty, oi.unit_price
		FROM order_items oi
		JOIN products p ON p.id = oi.product_id
		WHERE oi.order_id = $1::uuid
		ORDER BY oi.id ASC
	`, orderID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	defer rows.Close()

	items := make([]PublicOrderItem, 0)
	for rows.Next() {
		var it PublicOrderItem
		if err := rows.Scan(&it.ProductID, &it.ProductName, &it.Qty, &it.UnitPrice); err != nil {
			httpx.HandleError(c, err)
			return
		}
		it.UnitPrice = it.UnitPrice.Round(2)
		it.Subtotal = it.UnitPrice.Mul(decimal.NewFromInt(int64(it.Qty))).Round(2)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		httpx.HandleError(c, err)
		return
	}
	out.Items = items

	httpx.OK(c, out)
}

type counterResponseRequest struct {
	Accept *bool `json:"accept"`
}

// CounterResponse menangani POST /api/v1/public/orders/:code/counter-response (Bagian 8.3 & 9).
func (h *Handler) CounterResponse(c *gin.Context) {
	var req counterResponseRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	v := httpx.NewValidation()
	if req.Accept == nil {
		v.Add("Field accept (true atau false) wajib diisi.")
	}
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	if !EnforceRateLimit(c, h.DB, "order", PublicOrderRateLimitPerMin) {
		return
	}

	out, err := RespondCounterOffer(c.Request.Context(), h.DB, c.Param("code"), *req.Accept)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, out)
}

// ListOrders menangani GET /api/v1/orders (filter: status, date, event_date, from, to).
func (h *Handler) ListOrders(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	actor := auth.CurrentUser(c)
	role := ""
	if actor != nil {
		role = actor.Role
	}

	v := httpx.NewValidation()
	statusFilter := strings.TrimSpace(c.Query("status"))
	dateFilter := strings.TrimSpace(c.Query("date"))
	eventDateFilter := strings.TrimSpace(c.Query("event_date"))
	fromFilter := strings.TrimSpace(c.Query("from"))
	toFilter := strings.TrimSpace(c.Query("to"))

	if dateFilter != "" {
		if _, err := db.ParseDate(dateFilter); err != nil {
			v.Add("Parameter date harus berformat YYYY-MM-DD.")
		}
	}
	if eventDateFilter != "" {
		if _, err := db.ParseDate(eventDateFilter); err != nil {
			v.Add("Parameter event_date harus berformat YYYY-MM-DD.")
		}
	}
	if fromFilter != "" {
		if _, err := db.ParseDate(fromFilter); err != nil {
			v.Add("Parameter from harus berformat YYYY-MM-DD.")
		}
	}
	if toFilter != "" {
		if _, err := db.ParseDate(toFilter); err != nil {
			v.Add("Parameter to harus berformat YYYY-MM-DD.")
		}
	}
	if !v.OK() {
		v.Respond(c)
		return
	}

	items, err := ListOrders(c.Request.Context(), h.DB, OrderListFilter{
		Status:    statusFilter,
		Date:      dateFilter,
		EventDate: eventDateFilter,
		From:      fromFilter,
		To:        toFilter,
		ActorRole: role,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, items)
}

// GetOrder menangani GET /api/v1/orders/:id.
func (h *Handler) GetOrder(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	actor := auth.CurrentUser(c)
	role := ""
	if actor != nil {
		role = actor.Role
	}
	detail, err := GetOrderByID(c.Request.Context(), h.DB, id, role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, detail)
}

type decisionRequest struct {
	Action         string           `json:"action"`
	OverrideReason string           `json:"override_reason"`
	Counter        *CounterOfferDTO `json:"counter,omitempty"`
}

// DecideOrder menangani POST /api/v1/orders/:id/decision (Bagian 8.2 & 9).
func (h *Handler) DecideOrder(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req decisionRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	v := httpx.NewValidation()
	action := v.OneOf("Aksi keputusan (action)", strings.TrimSpace(req.Action), "accept", "counter", "reject")
	overrideReason := v.OptStr("Alasan override (override_reason)", req.OverrideReason, 500)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "Sesi login tidak ditemukan.")
		return
	}

	detail, err := DecideOrder(c.Request.Context(), h.DB, DecideOrderInput{
		OrderID:        id,
		ActorID:        actor.ID,
		ActorName:      actor.Name,
		ActorRole:      actor.Role,
		Action:         action,
		OverrideReason: overrideReason,
		Counter:        req.Counter,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, detail)
}

type paymentRequest struct {
	Amount *decimal.Decimal `json:"amount"`
	Method string           `json:"method"`
	Note   string           `json:"note"`
}

// RecordPayment menangani POST /api/v1/orders/:id/payments (Bagian 8.4 & 9).
func (h *Handler) RecordPayment(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req paymentRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	v := httpx.NewValidation()
	amount := v.Dec("Jumlah pembayaran (amount)", req.Amount, httpx.MoneyOpts(false, false))
	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = "transfer"
	} else {
		method = v.Str("Metode pembayaran (method)", method, 50)
	}
	note := v.OptStr("Catatan pembayaran (note)", req.Note, 500)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}

	res, err := RecordPayment(c.Request.Context(), h.DB, RecordPaymentInput{
		OrderID: id,
		ActorID: actorID,
		Amount:  amount,
		Method:  method,
		Note:    note,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.Created(c, res)
}

type transitionRequest struct {
	ToStatus      string `json:"to_status"`
	Status        string `json:"status"`
	ActualMinutes *int   `json:"actual_minutes,omitempty"`
}

// TransitionOrder menangani POST /api/v1/orders/:id/transition (Bagian 8.5 & 9).
func (h *Handler) TransitionOrder(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req transitionRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	target := strings.TrimSpace(req.ToStatus)
	if target == "" {
		target = strings.TrimSpace(req.Status)
	}
	v := httpx.NewValidation()
	toStatus := v.OneOf("Status tujuan (to_status)", target,
		StatusInProduction, StatusReady, StatusDelivering, StatusCompleted)
	if req.ActualMinutes != nil && *req.ActualMinutes <= 0 {
		v.Add("Durasi aktual (actual_minutes) harus lebih dari 0 menit.")
	}
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "Sesi login tidak ditemukan.")
		return
	}

	detail, err := TransitionOrder(c.Request.Context(), h.DB, TransitionOrderInput{
		OrderID:       id,
		ActorID:       actor.ID,
		ActorRole:     actor.Role,
		ToStatus:      toStatus,
		ActualMinutes: req.ActualMinutes,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, detail)
}

type cancelOrderRequest struct {
	Reason        string `json:"reason"`
	DPDisposition string `json:"dp_disposition"`
	Note          string `json:"note"`
}

// CancelOrder menangani POST /api/v1/orders/:id/cancel (Bagian 8.6 & 9).
func (h *Handler) CancelOrder(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req cancelOrderRequest
	if c.Request.ContentLength != 0 {
		if !httpx.BindJSON(c, &req) {
			return
		}
	}
	v := httpx.NewValidation()
	reason := v.OptStr("Alasan pembatalan (reason)", req.Reason, 500)
	note := v.OptStr("Catatan pembatalan (note)", req.Note, 500)
	dpDisp := strings.TrimSpace(req.DPDisposition)
	if dpDisp != "" {
		dpDisp = v.OneOf("Keputusan DP (dp_disposition)", dpDisp, "forfeited", "refunded", "pending")
	}
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}

	res, err := CancelOrder(c.Request.Context(), h.DB, CancelOrderInput{
		OrderID:       id,
		ActorID:       actorID,
		Reason:        reason,
		DPDisposition: dpDisp,
		Note:          note,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, res)
}

// DecisionQueue menangani GET /api/v1/decision-queue (Bagian 9).
func (h *Handler) DecisionQueue(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	actor := auth.CurrentUser(c)
	role := ""
	if actor != nil {
		role = actor.Role
	}
	items, err := GetDecisionQueue(c.Request.Context(), h.DB, role)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, items)
}

// CapacityCalendar menangani GET /api/v1/capacity/calendar?month=YYYY-MM (Bagian 9).
func (h *Handler) CapacityCalendar(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	days, err := GetCapacityCalendar(c.Request.Context(), h.DB, c.Query("month"))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, days)
}

// CapacityDay menangani GET /api/v1/capacity/day/:date (Bagian 9).
func (h *Handler) CapacityDay(c *gin.Context) {
	dateStr := strings.TrimSpace(c.Param("date"))
	parsed, err := db.ParseDate(dateStr)
	if err != nil {
		httpx.UnprocessableEntity(c, "validation_error", "Tanggal (date) harus berformat YYYY-MM-DD.")
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	detail, err := GetCapacityDay(c.Request.Context(), h.DB, parsed)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, detail)
}

// ListTasks menangani GET /api/v1/tasks (filter: mine, status, type, date/due_date, from, to).
func (h *Handler) ListTasks(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	actor := auth.CurrentUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "Sesi login tidak ditemukan.")
		return
	}

	v := httpx.NewValidation()
	statusFilter := strings.TrimSpace(c.Query("status"))
	if statusFilter != "" {
		statusFilter = v.OneOf("Status tugas (status)", statusFilter, "todo", "doing", "done")
	}
	typeFilter := strings.TrimSpace(c.Query("type"))
	if typeFilter != "" {
		typeFilter = v.OneOf("Tipe tugas (type)", typeFilter, "produce", "shop", "deliver")
	}
	dateFilter := strings.TrimSpace(c.Query("date"))
	if dateFilter == "" {
		dateFilter = strings.TrimSpace(c.Query("due_date"))
	}
	if dateFilter != "" {
		if _, err := db.ParseDate(dateFilter); err != nil {
			v.Add("Parameter tanggal (date) harus berformat YYYY-MM-DD.")
		}
	}
	fromFilter := strings.TrimSpace(c.Query("from"))
	if fromFilter != "" {
		if _, err := db.ParseDate(fromFilter); err != nil {
			v.Add("Parameter from harus berformat YYYY-MM-DD.")
		}
	}
	toFilter := strings.TrimSpace(c.Query("to"))
	if toFilter != "" {
		if _, err := db.ParseDate(toFilter); err != nil {
			v.Add("Parameter to harus berformat YYYY-MM-DD.")
		}
	}
	if !v.OK() {
		v.Respond(c)
		return
	}

	mineStr := strings.ToLower(strings.TrimSpace(c.Query("mine")))
	mine := mineStr == "true" || mineStr == "1"

	items, err := ListTasks(c.Request.Context(), h.DB, TaskFilter{
		Mine:      mine,
		Status:    statusFilter,
		Type:      typeFilter,
		Date:      dateFilter,
		From:      fromFilter,
		To:        toFilter,
		ActorID:   actor.ID,
		ActorRole: actor.Role,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, items)
}

type updateTaskRequest struct {
	Status        string `json:"status"`
	ActualMinutes *int   `json:"actual_minutes,omitempty"`
}

// UpdateTask menangani PATCH /api/v1/tasks/:id (status todo|doing|done dan actual_minutes).
func (h *Handler) UpdateTask(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req updateTaskRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	v := httpx.NewValidation()
	statusVal := strings.TrimSpace(req.Status)
	if statusVal == "" && req.ActualMinutes == nil {
		v.Add("Minimal salah satu dari status atau actual_minutes wajib diisi.")
	}
	if statusVal != "" {
		statusVal = v.OneOf("Status tugas (status)", statusVal, "todo", "doing", "done")
	}
	if req.ActualMinutes != nil && *req.ActualMinutes <= 0 {
		v.Add("Durasi aktual (actual_minutes) harus lebih dari 0 menit.")
	}
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	if actor == nil {
		httpx.Unauthorized(c, "Sesi login tidak ditemukan.")
		return
	}

	updated, err := UpdateTask(c.Request.Context(), h.DB, UpdateTaskInput{
		TaskID:        id,
		ActorID:       actor.ID,
		ActorRole:     actor.Role,
		Status:        statusVal,
		ActualMinutes: req.ActualMinutes,
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	httpx.OK(c, updated)
}

