package catalog

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
)

// ProductLoad merepresentasikan baris tabel product_loads beserta detail sumber daya.
type ProductLoad struct {
	ProductID    string          `json:"product_id"`
	ResourceID   string          `json:"resource_id"`
	ResourceCode string          `json:"resource_code,omitempty"`
	ResourceName string          `json:"resource_name,omitempty"`
	Unit         string          `json:"unit,omitempty"`
	Load         decimal.Decimal `json:"load"`
}

type productLoadInput struct {
	ResourceID string           `json:"resource_id"`
	Load       *decimal.Decimal `json:"load"`
}

type productLoadUpsertRequest struct {
	ProductID  string             `json:"product_id"`
	ResourceID string             `json:"resource_id"`
	Load       *decimal.Decimal   `json:"load"`
	Items      []productLoadInput `json:"items"`
	ReplaceAll bool               `json:"replace_all"`
}

// ListProductLoads mengembalikan daftar beban sumber daya produk (opsional ?product_id=...).
func (h *Handler) ListProductLoads(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	productID := strings.TrimSpace(c.Query("product_id"))
	if productID == "" {
		productID = strings.TrimSpace(c.Param("id"))
	}
	if productID != "" && !httpx.ValidUUID(productID) {
		httpx.UnprocessableEntity(c, "validation_error", "product_id harus berformat UUID yang valid.")
		return
	}

	items, err := queryProductLoads(c, h.DB, productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, items)
}

func queryProductLoads(c *gin.Context, q db.DBTX, productID string) ([]ProductLoad, error) {
	sql := `
		SELECT pl.product_id::text, pl.resource_id::text, r.code, r.name, r.unit, pl.load
		FROM product_loads pl
		JOIN resources r ON r.id = pl.resource_id
	`
	var args []any
	if productID != "" {
		sql += ` WHERE pl.product_id = $1::uuid`
		args = append(args, productID)
	}
	sql += ` ORDER BY pl.product_id ASC, r.code ASC`

	rows, err := q.Query(c.Request.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ProductLoad, 0)
	for rows.Next() {
		var it ProductLoad
		if err := rows.Scan(&it.ProductID, &it.ResourceID, &it.ResourceCode, &it.ResourceName, &it.Unit, &it.Load); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpsertProductLoad menambahkan atau memperbarui beban sumber daya per unit produk.
func (h *Handler) UpsertProductLoad(c *gin.Context) {
	var req productLoadUpsertRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	if pathPID := c.Param("id"); pathPID != "" && req.ProductID == "" {
		req.ProductID = pathPID
	}

	v := httpx.NewValidation()
	productID := strings.ToLower(strings.TrimSpace(req.ProductID))
	if !httpx.ValidUUID(productID) {
		v.Add("product_id harus berformat UUID yang valid.")
	}

	rawItems := req.Items
	if len(rawItems) == 0 && (req.ResourceID != "" || req.Load != nil) {
		rawItems = []productLoadInput{
			{ResourceID: req.ResourceID, Load: req.Load},
		}
	}
	if len(rawItems) == 0 && !req.ReplaceAll {
		v.Add("Minimal satu item beban produk wajib diisi.")
	}

	loadOpts := httpx.DecOpts{
		Min:          decimal.Zero,
		Max:          decimal.NewFromInt(99_999_999),
		MinExclusive: false,
		MaxScale:     4,
	}

	type cleanLoad struct {
		ResourceID string          `json:"resource_id"`
		Load       decimal.Decimal `json:"load"`
	}
	seen := make(map[string]bool, len(rawItems))
	clean := make([]cleanLoad, 0, len(rawItems))
	for idx, it := range rawItems {
		rid := strings.ToLower(strings.TrimSpace(it.ResourceID))
		if !httpx.ValidUUID(rid) {
			v.Add(fmt.Sprintf("Item beban #%d: resource_id harus berformat UUID yang valid.", idx+1))
		} else if seen[rid] {
			v.Add(fmt.Sprintf("Item beban #%d: resource_id duplikat dalam satu permintaan.", idx+1))
		} else {
			seen[rid] = true
		}
		ld := v.Dec(fmt.Sprintf("Beban produksi #%d (load)", idx+1), it.Load, loadOpts)
		clean = append(clean, cleanLoad{ResourceID: rid, Load: ld})
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
	var out []ProductLoad
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		if req.ReplaceAll {
			if _, err := tx.Exec(c.Request.Context(), `DELETE FROM product_loads WHERE product_id = $1::uuid`, productID); err != nil {
				return err
			}
		}
		for _, it := range clean {
			_, err := tx.Exec(c.Request.Context(), `
				INSERT INTO product_loads (product_id, resource_id, load)
				VALUES ($1::uuid, $2::uuid, $3)
				ON CONFLICT (product_id, resource_id) DO UPDATE
				SET load = EXCLUDED.load
			`, productID, it.ResourceID, it.Load)
			if err != nil {
				return err
			}
		}

		if err := audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "product_loads", productID, map[string]any{
			"op":          "upsert",
			"product_id":  productID,
			"replace_all": req.ReplaceAll,
			"items":       clean,
		}); err != nil {
			return err
		}

		var err error
		out, err = queryProductLoads(c, tx, productID)
		return err
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, out)
}

// DeleteProductLoad menghapus satu baris product_loads berdasarkan product_id dan resource_id.
func (h *Handler) DeleteProductLoad(c *gin.Context) {
	productID, ok := httpx.ParamUUID(c, "product_id")
	if !ok {
		return
	}
	resourceID, ok := httpx.ParamUUID(c, "resource_id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(c.Request.Context(), `
			DELETE FROM product_loads
			WHERE product_id = $1::uuid AND resource_id = $2::uuid
		`, productID, resourceID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.Err(http.StatusNotFound, "not_found", "Beban produk tidak ditemukan.")
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "product_loads", productID, map[string]any{
			"op":          "delete",
			"product_id":  productID,
			"resource_id": resourceID,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, gin.H{
		"product_id":  productID,
		"resource_id": resourceID,
		"deleted":     true,
	})
}

