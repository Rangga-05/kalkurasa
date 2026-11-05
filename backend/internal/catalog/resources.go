package catalog

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/engine"
	"kalkurasa/backend/internal/httpx"
)

// Resource merepresentasikan baris tabel resources beserta kapasitas efektif harian.
type Resource struct {
	ID               string          `json:"id"`
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Unit             string          `json:"unit"`
	Count            int             `json:"count"`
	AmountPerUnitDay decimal.Decimal `json:"amount_per_unit_day"`
	Efficiency       decimal.Decimal `json:"efficiency"`
	DailyCapacity    decimal.Decimal `json:"daily_capacity"`
}

func (r *Resource) computeCapacity() {
	r.DailyCapacity = engine.Capacity(engine.Resource{
		Code:             r.Code,
		Count:            r.Count,
		AmountPerUnitDay: r.AmountPerUnitDay,
		Efficiency:       r.Efficiency,
	})
}

type resourceRequest struct {
	Code             string           `json:"code"`
	Name             string           `json:"name"`
	Unit             string           `json:"unit"`
	Count            *int             `json:"count"`
	AmountPerUnitDay *decimal.Decimal `json:"amount_per_unit_day"`
	Efficiency       *decimal.Decimal `json:"efficiency"`
}

func validateResourceRequest(req resourceRequest, requireCode bool) (Resource, *httpx.Validation) {
	v := httpx.NewValidation()
	var code string
	if requireCode || req.Code != "" {
		code = v.OneOf("Kode sumber daya", req.Code, "oven", "labor")
	}
	name := v.Str("Nama sumber daya", req.Name, 120)
	unit := v.OneOf("Satuan kapasitas", req.Unit, "oven_hour", "minute")
	count := v.Int("Jumlah unit", req.Count, 1, 1000)
	amount := v.Dec("Kapasitas per unit per hari", req.AmountPerUnitDay, httpx.DecOpts{
		Min:          decimal.Zero,
		Max:          decimal.NewFromInt(99_999_999),
		MinExclusive: true,
		MaxScale:     4,
	})
	eff := v.Dec("Efisiensi", req.Efficiency, httpx.DecOpts{
		Min:          decimal.Zero,
		Max:          decimal.NewFromInt(1),
		MinExclusive: true,
		MaxScale:     4,
	})

	return Resource{
		Code:             code,
		Name:             name,
		Unit:             unit,
		Count:            count,
		AmountPerUnitDay: amount,
		Efficiency:       eff,
	}, v
}

const resourceCols = `id::text, code, name, unit, count, amount_per_unit_day, efficiency`

// ListResources mengembalikan semua sumber daya produksi.
func (h *Handler) ListResources(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	rows, err := h.DB.Query(c.Request.Context(),
		`SELECT `+resourceCols+` FROM resources ORDER BY code ASC`,
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	defer rows.Close()

	items := make([]Resource, 0)
	for rows.Next() {
		var r Resource
		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &r.Unit, &r.Count, &r.AmountPerUnitDay, &r.Efficiency); err != nil {
			httpx.HandleError(c, err)
			return
		}
		r.computeCapacity()
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, items)
}

// GetResource mengembalikan satu sumber daya menurut ID.
func (h *Handler) GetResource(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	var r Resource
	err := h.DB.QueryRow(c.Request.Context(),
		`SELECT `+resourceCols+` FROM resources WHERE id = $1::uuid`, id,
	).Scan(&r.ID, &r.Code, &r.Name, &r.Unit, &r.Count, &r.AmountPerUnitDay, &r.Efficiency)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	r.computeCapacity()

	httpx.OK(c, r)
}

// CreateResource menambahkan atau memperbarui sumber daya produksi dan mencatat audit log.
func (h *Handler) CreateResource(c *gin.Context) {
	var req resourceRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, v := validateResourceRequest(req, true)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	var out Resource
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(c.Request.Context(), `
			INSERT INTO resources (code, name, unit, count, amount_per_unit_day, efficiency)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING `+resourceCols,
			in.Code, in.Name, in.Unit, in.Count, in.AmountPerUnitDay, in.Efficiency,
		).Scan(&out.ID, &out.Code, &out.Name, &out.Unit, &out.Count, &out.AmountPerUnitDay, &out.Efficiency)
		if err != nil {
			return err
		}
		out.computeCapacity()
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "resources", out.ID, map[string]any{
			"op":       "create",
			"resource": out,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.Created(c, out)
}

// UpdateResource memperbarui sumber daya produksi dan mencatat audit log.
func (h *Handler) UpdateResource(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req resourceRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, v := validateResourceRequest(req, false)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	var out Resource
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		var err error
		if in.Code != "" {
			err = tx.QueryRow(c.Request.Context(), `
				UPDATE resources
				SET code = $2, name = $3, unit = $4, count = $5, amount_per_unit_day = $6, efficiency = $7
				WHERE id = $1::uuid
				RETURNING `+resourceCols,
				id, in.Code, in.Name, in.Unit, in.Count, in.AmountPerUnitDay, in.Efficiency,
			).Scan(&out.ID, &out.Code, &out.Name, &out.Unit, &out.Count, &out.AmountPerUnitDay, &out.Efficiency)
		} else {
			err = tx.QueryRow(c.Request.Context(), `
				UPDATE resources
				SET name = $2, unit = $3, count = $4, amount_per_unit_day = $5, efficiency = $6
				WHERE id = $1::uuid
				RETURNING `+resourceCols,
				id, in.Name, in.Unit, in.Count, in.AmountPerUnitDay, in.Efficiency,
			).Scan(&out.ID, &out.Code, &out.Name, &out.Unit, &out.Count, &out.AmountPerUnitDay, &out.Efficiency)
		}
		if err != nil {
			return err
		}
		out.computeCapacity()
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "resources", out.ID, map[string]any{
			"op":       "update",
			"resource": out,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, out)
}

// DeleteResource menolak penghapusan bila sumber daya sedang digunakan pada
// product_loads (409 Conflict) agar beban produk tidak ikut terhapus.
func (h *Handler) DeleteResource(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM resources WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return httpx.Err(http.StatusNotFound, "not_found", "Sumber daya tidak ditemukan.")
		}

		var inUse bool
		if err := tx.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM product_loads WHERE resource_id = $1::uuid)`, id).Scan(&inUse); err != nil {
			return err
		}
		if inUse {
			return httpx.Err(http.StatusConflict, "conflict", "Sumber daya sedang digunakan pada beban produk dan tidak dapat dihapus.")
		}

		if _, err := tx.Exec(c.Request.Context(), `DELETE FROM resources WHERE id = $1::uuid`, id); err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "resources", id, map[string]any{
			"op": "delete",
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, gin.H{
		"id":      id,
		"deleted": true,
	})
}

