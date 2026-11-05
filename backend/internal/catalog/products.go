// Package catalog menangani CRUD data master: produk, bahan baku, resep
// (recipe_items), sumber daya (resources), dan beban produk (product_loads).
package catalog

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
)

// Handler menyediakan endpoint CRUD katalog dan master data.
type Handler struct {
	DB db.Conn
}

// Product merepresentasikan baris tabel products.
type Product struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Category      string          `json:"category"`
	Price         decimal.Decimal `json:"price"`
	PackagingCost decimal.Decimal `json:"packaging_cost"`
	OverheadCost  decimal.Decimal `json:"overhead_cost"`
	IsActive      bool            `json:"is_active"`
}

type productRequest struct {
	Name          string           `json:"name"`
	Category      string           `json:"category"`
	Price         *decimal.Decimal `json:"price"`
	PackagingCost *decimal.Decimal `json:"packaging_cost"`
	OverheadCost  *decimal.Decimal `json:"overhead_cost"`
	IsActive      *bool            `json:"is_active"`
}

func validateProductRequest(req productRequest) (Product, *httpx.Validation) {
	v := httpx.NewValidation()
	name := v.Str("Nama produk", req.Name, 120)
	cat := v.OptStr("Kategori", req.Category, 60)
	if cat == "" {
		cat = "umum"
	}
	price := v.Dec("Harga jual", req.Price, httpx.MoneyOpts(false, false))
	packaging := v.Dec("Biaya kemasan", req.PackagingCost, httpx.MoneyOpts(false, true))
	overhead := v.Dec("Biaya overhead", req.OverheadCost, httpx.MoneyOpts(false, true))
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	return Product{
		Name:          name,
		Category:      cat,
		Price:         price,
		PackagingCost: packaging,
		OverheadCost:  overhead,
		IsActive:      active,
	}, v
}

const productCols = `id::text, name, category, price, packaging_cost, overhead_cost, is_active`

// ListProducts mengembalikan daftar produk dengan paginasi.
func (h *Handler) ListProducts(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	page, limit, offset := httpx.Paginate(c)
	onlyActive := c.Query("active") == "true"

	ctx := c.Request.Context()
	var total int
	countSQL := `SELECT COUNT(*) FROM products`
	listSQL := `SELECT ` + productCols + ` FROM products ORDER BY name ASC, id ASC LIMIT $1 OFFSET $2`
	if onlyActive {
		countSQL = `SELECT COUNT(*) FROM products WHERE is_active = true`
		listSQL = `SELECT ` + productCols + ` FROM products WHERE is_active = true ORDER BY name ASC, id ASC LIMIT $1 OFFSET $2`
	}

	if err := h.DB.QueryRow(ctx, countSQL).Scan(&total); err != nil {
		httpx.HandleError(c, err)
		return
	}

	rows, err := h.DB.Query(ctx, listSQL, limit, offset)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	defer rows.Close()

	items := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.Price, &p.PackagingCost, &p.OverheadCost, &p.IsActive); err != nil {
			httpx.HandleError(c, err)
			return
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.List(c, items, httpx.Meta{Page: page, Limit: limit, Total: total})
}

// GetProduct mengembalikan satu produk berdasarkan ID.
func (h *Handler) GetProduct(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	var p Product
	err := h.DB.QueryRow(c.Request.Context(),
		`SELECT `+productCols+` FROM products WHERE id = $1::uuid`, id,
	).Scan(&p.ID, &p.Name, &p.Category, &p.Price, &p.PackagingCost, &p.OverheadCost, &p.IsActive)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, p)
}

// CreateProduct menambahkan produk baru dan mencatat audit log.
func (h *Handler) CreateProduct(c *gin.Context) {
	var req productRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, v := validateProductRequest(req)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	var out Product
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(c.Request.Context(), `
			INSERT INTO products (name, category, price, packaging_cost, overhead_cost, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING `+productCols,
			in.Name, in.Category, in.Price, in.PackagingCost, in.OverheadCost, in.IsActive,
		).Scan(&out.ID, &out.Name, &out.Category, &out.Price, &out.PackagingCost, &out.OverheadCost, &out.IsActive)
		if err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "products", out.ID, map[string]any{
			"op":      "create",
			"product": out,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.Created(c, out)
}

// UpdateProduct memperbarui data produk dan mencatat audit log.
func (h *Handler) UpdateProduct(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req productRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, v := validateProductRequest(req)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	var out Product
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(c.Request.Context(), `
			UPDATE products
			SET name = $2, category = $3, price = $4, packaging_cost = $5, overhead_cost = $6, is_active = $7
			WHERE id = $1::uuid
			RETURNING `+productCols,
			id, in.Name, in.Category, in.Price, in.PackagingCost, in.OverheadCost, in.IsActive,
		).Scan(&out.ID, &out.Name, &out.Category, &out.Price, &out.PackagingCost, &out.OverheadCost, &out.IsActive)
		if err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "products", out.ID, map[string]any{
			"op":      "update",
			"product": out,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, out)
}

// DeleteProduct mengarsipkan produk (is_active=false) bila sudah terpakai di
// pesanan/resep/beban (FR-MD-08), atau menghapusnya bila belum pernah dipakai.
func (h *Handler) DeleteProduct(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	var archived bool
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return httpx.Err(http.StatusNotFound, "not_found", "Produk tidak ditemukan.")
		}

		var inUse bool
		if err := tx.QueryRow(c.Request.Context(), `
			SELECT EXISTS(SELECT 1 FROM order_items WHERE product_id = $1::uuid)
			    OR EXISTS(SELECT 1 FROM recipe_items WHERE product_id = $1::uuid)
			    OR EXISTS(SELECT 1 FROM product_loads WHERE product_id = $1::uuid)
		`, id).Scan(&inUse); err != nil {
			return err
		}

		if inUse {
			archived = true
			if _, err := tx.Exec(c.Request.Context(), `UPDATE products SET is_active = false WHERE id = $1::uuid`, id); err != nil {
				return err
			}
			return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "products", id, map[string]any{
				"op":       "archive",
				"archived": true,
			})
		}

		if _, err := tx.Exec(c.Request.Context(), `DELETE FROM products WHERE id = $1::uuid`, id); err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "products", id, map[string]any{
			"op":       "delete",
			"archived": false,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, gin.H{
		"id":        id,
		"archived":  archived,
		"is_active": false,
	})
}

