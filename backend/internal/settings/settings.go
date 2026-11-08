// Package settings menyediakan endpoint pembacaan dan pembaruan parameter
// usaha (tabel settings) beserta validasi nilai dan pencatatan audit log.
package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
)

// Handler menangani GET /api/v1/settings dan PUT /api/v1/settings.
type Handler struct {
	DB db.Conn
}

// Kunci kas yang dirahasiakan dari peran selain owner (Bagian 4 & 9).
var cashKeys = map[string]bool{
	"opening_cash":     true,
	"fixed_cost_daily": true,
	"cash_buffer":      true,
}

// Default nilai awal sesuai Bagian 5 brief.
func defaultSettings() map[string]any {
	return map[string]any{
		"capacity_limit":        0.90,
		"status_green_below":    0.70,
		"status_yellow_below":   0.90,
		"margin_min":            0.20,
		"labor_rate_per_minute": 400,
		"waste_pct":             0.05,
		"shopping_buffer_pct":   0.10,
		"cash_buffer":           400000,
		"fixed_cost_daily":      400000,
		"opening_cash":          1800000,
		"lead_time_days":        2,
		"dp_pct":                0.30,
		"dp_deadline_hours":     24,
		"big_order_total":       2000000,
		"big_order_units":       50,
		"shopping_lead_days":       2,
		"max_active_unpaid_orders": 2,
		"tax_pct":                  0.0,
		"demo_today":               nil,
	}
}

// Get mengembalikan seluruh parameter untuk owner, atau parameter tanpa kunci
// kas untuk manager.
func (h *Handler) Get(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	out, err := loadSettingsMap(c.Request.Context(), h.DB)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	u := auth.CurrentUser(c)
	if u != nil && u.Role != auth.RoleOwner {
		for k := range cashKeys {
			delete(out, k)
		}
	}

	httpx.OK(c, out)
}

func loadSettingsMap(ctx context.Context, q db.DBTX) (map[string]any, error) {
	out := defaultSettings()
	rows, err := q.Query(ctx, `SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var k string
		var raw []byte
		if err := rows.Scan(&k, &raw); err != nil {
			return nil, err
		}
		var val any
		if err := json.Unmarshal(raw, &val); err != nil {
			return nil, fmt.Errorf("settings: unmarshal %s: %w", k, err)
		}
		out[k] = val
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Put memperbarui satu atau beberapa parameter usaha (khusus owner) dan
// mencatat audit log.
func (h *Handler) Put(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.BadRequest(c, "invalid_json", "Format data tidak valid. Kirim JSON yang benar.")
		return
	}
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		httpx.BadRequest(c, "invalid_json", "Isi permintaan kosong. Kirim data dalam format JSON.")
		return
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(bodyBytes, &rawMap); err != nil {
		httpx.BadRequest(c, "invalid_json", "Format data tidak valid. Kirim objek JSON.")
		return
	}
	// Dukung bentuk {"settings": {...}} maupun objek datar {"capacity_limit": 0.90, ...}
	if nested, ok := rawMap["settings"]; ok && len(rawMap) == 1 {
		var inner map[string]json.RawMessage
		if err := json.Unmarshal(nested, &inner); err == nil && inner != nil {
			rawMap = inner
		}
	}

	v := httpx.NewValidation()
	if len(rawMap) == 0 {
		v.Add("Minimal satu parameter wajib dikirim.")
		v.Respond(c)
		return
	}

	normalized := make(map[string]string, len(rawMap))
	parsedVals := make(map[string]any, len(rawMap))
	decimals := make(map[string]decimal.Decimal)

	for k, raw := range rawMap {
		normJSON, parsed, decVal := validateSettingEntry(v, k, raw)
		normalized[k] = normJSON
		parsedVals[k] = parsed
		decimals[k] = decVal
	}

	if g, okG := decimals["status_green_below"]; okG {
		if y, okY := decimals["status_yellow_below"]; okY {
			if !g.LessThan(y) {
				v.Add("status_green_below harus lebih kecil dari status_yellow_below.")
			}
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

	actor := auth.CurrentUser(c)
	var updated map[string]any

	err = db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		for k, jsonVal := range normalized {
			_, err := tx.Exec(c.Request.Context(), `
				INSERT INTO settings (key, value)
				VALUES ($1, $2::jsonb)
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
			`, k, jsonVal)
			if err != nil {
				return err
			}
		}

		if err := audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionSetting, "settings", "", map[string]any{
			"updated": parsedVals,
		}); err != nil {
			return err
		}

		var err error
		updated, err = loadSettingsMap(c.Request.Context(), tx)
		return err
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, updated)
}

func validateSettingEntry(v *httpx.Validation, key string, raw json.RawMessage) (string, any, decimal.Decimal) {
	parseDec := func(min, max decimal.Decimal, minExcl bool, maxScale int32) (string, any, decimal.Decimal) {
		var d decimal.Decimal
		if err := json.Unmarshal(raw, &d); err != nil {
			v.Add(fmt.Sprintf("%s harus berupa angka.", key))
			return "", nil, decimal.Zero
		}
		res := v.Dec(key, &d, httpx.DecOpts{
			Min:          min,
			Max:          max,
			MinExclusive: minExcl,
			MaxScale:     maxScale,
		})
		f, _ := res.Float64()
		b, _ := json.Marshal(f)
		return string(b), f, res
	}

	parseInt := func(min, max int) (string, any, decimal.Decimal) {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil {
			v.Add(fmt.Sprintf("%s harus berupa bilangan bulat.", key))
			return "", nil, decimal.Zero
		}
		res := v.Int(key, &n, min, max)
		return fmt.Sprintf("%d", res), res, decimal.NewFromInt(int64(res))
	}

	one := decimal.NewFromInt(1)
	maxMoney := decimal.NewFromInt(999_999_999_999)

	switch key {
	case "capacity_limit", "status_green_below", "status_yellow_below":
		return parseDec(decimal.Zero, one, true, 4)
	case "margin_min":
		return parseDec(decimal.Zero, decimal.RequireFromString("0.95"), false, 4)
	case "waste_pct", "shopping_buffer_pct", "dp_pct", "tax_pct":
		return parseDec(decimal.Zero, one, false, 4)
	case "labor_rate_per_minute", "cash_buffer", "fixed_cost_daily", "opening_cash":
		return parseDec(decimal.Zero, maxMoney, false, 4)
	case "big_order_total":
		return parseDec(decimal.Zero, maxMoney, true, 4)
	case "lead_time_days", "shopping_lead_days":
		return parseInt(0, 60)
	case "dp_deadline_hours":
		return parseInt(1, 720)
	case "big_order_units":
		return parseInt(1, 100000)
	case "max_active_unpaid_orders":
		return parseInt(1, 100)
	case "demo_today":
		trimmed := bytes.TrimSpace(raw)
		if bytes.Equal(trimmed, []byte("null")) {
			return "null", nil, decimal.Zero
		}
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			v.Add("demo_today harus berupa tanggal YYYY-MM-DD atau null.")
			return "", nil, decimal.Zero
		}
		if s == "" {
			return "null", nil, decimal.Zero
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			v.Add("demo_today harus berformat YYYY-MM-DD.")
			return "", nil, decimal.Zero
		}
		b, _ := json.Marshal(s)
		return string(b), s, decimal.Zero
	default:
		v.Add(fmt.Sprintf("Parameter tidak dikenal: %s.", key))
		return "", nil, decimal.Zero
	}
}

