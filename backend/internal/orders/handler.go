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

// PublicOrderSummary adalah respons ringkas dan aman untuk POST /public/orders (Bagian 9).
type PublicOrderSummary struct {
	ID             string            `json:"id"`
	Code           string            `json:"code"`
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

// CreatePublicOrder menangani POST /api/v1/public/orders (dibatasi laju 30/menit/IP via tabel rate_limits).
func (h *Handler) CreatePublicOrder(c *gin.Context) {
	var req createOrderRequest
	if !httpx.BindJSON(c, &req) {
		return
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

