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

// Ingredient merepresentasikan baris tabel ingredients.
// BuyPrice bertipe pointer dengan omitempty agar dapat dihilangkan sepenuhnya
// saat diakses oleh peran staff (Bagian 6 & Bagian 9).
type Ingredient struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	UseUnit     string           `json:"use_unit"`
	BuyUnit     string           `json:"buy_unit"`
	BuyUnitSize decimal.Decimal  `json:"buy_unit_size"`
	BuyPrice    *decimal.Decimal `json:"buy_price,omitempty"`
	Stock       decimal.Decimal  `json:"stock"`
	MinStock    decimal.Decimal  `json:"min_stock"`
	Rounding    string           `json:"rounding"`
	IsActive    bool             `json:"is_active"`
}

func (ing *Ingredient) stripForRole(role string) {
	if role == auth.RoleStaff {
		ing.BuyPrice = nil
	}
}

type ingredientRequest struct {
	Name        string           `json:"name"`
	UseUnit     string           `json:"use_unit"`
	BuyUnit     string           `json:"buy_unit"`
	BuyUnitSize *decimal.Decimal `json:"buy_unit_size"`
	BuyPrice    *decimal.Decimal `json:"buy_price"`
	Stock       *decimal.Decimal `json:"stock"`
	MinStock    *decimal.Decimal `json:"min_stock"`
	Rounding    string           `json:"rounding"`
	IsActive    *bool            `json:"is_active"`
}

func validateIngredientRequest(req ingredientRequest) (Ingredient, decimal.Decimal, *httpx.Validation) {
	v := httpx.NewValidation()
	name := v.Str("Nama bahan", req.Name, 120)
	useUnit := v.Str("Satuan pakai", req.UseUnit, 30)
	buyUnit := v.Str("Satuan beli", req.BuyUnit, 30)
	buyUnitSize := v.Dec("Isi satuan beli", req.BuyUnitSize, httpx.DecOpts{
		Min:          decimal.Zero,
		Max:          decimal.NewFromInt(99_999_999),
		MinExclusive: true,
		MaxScale:     4,
	})
	buyPrice := v.Dec("Harga beli", req.BuyPrice, httpx.MoneyOpts(false, false))
	stock := v.Dec("Stok", req.Stock, httpx.MoneyOpts(false, true))
	minStock := v.Dec("Stok minimum", req.MinStock, httpx.MoneyOpts(false, true))

	rounding := req.Rounding
	if rounding == "" {
		rounding = "loose"
	} else {
		rounding = v.OneOf("Aturan pembulatan", rounding, "loose", "pack")
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	bpCopy := buyPrice
	return Ingredient{
		Name:        name,
		UseUnit:     useUnit,
		BuyUnit:     buyUnit,
		BuyUnitSize: buyUnitSize,
		BuyPrice:    &bpCopy,
		Stock:       stock,
		MinStock:    minStock,
		Rounding:    rounding,
		IsActive:    active,
	}, buyPrice, v
}

const ingredientCols = `id::text, name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active`

func scanIngredient(row pgx.Row) (Ingredient, error) {
	var ing Ingredient
	var bp decimal.Decimal
	err := row.Scan(
		&ing.ID, &ing.Name, &ing.UseUnit, &ing.BuyUnit,
		&ing.BuyUnitSize, &bp, &ing.Stock, &ing.MinStock,
		&ing.Rounding, &ing.IsActive,
	)
	if err != nil {
		return Ingredient{}, err
	}
	ing.BuyPrice = &bp
	return ing, nil
}

// ListIngredients mengembalikan daftar bahan baku. Bila diakses oleh staff,
// field buy_price dihilangkan dari setiap elemen.
func (h *Handler) ListIngredients(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	page, limit, offset := httpx.Paginate(c)
	onlyActive := c.Query("active") == "true"

	ctx := c.Request.Context()
	var total int
	countSQL := `SELECT COUNT(*) FROM ingredients`
	listSQL := `SELECT ` + ingredientCols + ` FROM ingredients ORDER BY name ASC, id ASC LIMIT $1 OFFSET $2`
	if onlyActive {
		countSQL = `SELECT COUNT(*) FROM ingredients WHERE is_active = true`
		listSQL = `SELECT ` + ingredientCols + ` FROM ingredients WHERE is_active = true ORDER BY name ASC, id ASC LIMIT $1 OFFSET $2`
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

	role := ""
	if u := auth.CurrentUser(c); u != nil {
		role = u.Role
	}

	items := make([]Ingredient, 0)
	for rows.Next() {
		var ing Ingredient
		var bp decimal.Decimal
		if err := rows.Scan(
			&ing.ID, &ing.Name, &ing.UseUnit, &ing.BuyUnit,
			&ing.BuyUnitSize, &bp, &ing.Stock, &ing.MinStock,
			&ing.Rounding, &ing.IsActive,
		); err != nil {
			httpx.HandleError(c, err)
			return
		}
		ing.BuyPrice = &bp
		ing.stripForRole(role)
		items = append(items, ing)
	}
	if err := rows.Err(); err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.List(c, items, httpx.Meta{Page: page, Limit: limit, Total: total})
}

// GetIngredient mengembalikan satu bahan baku menurut ID.
func (h *Handler) GetIngredient(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	ing, err := scanIngredient(h.DB.QueryRow(c.Request.Context(),
		`SELECT `+ingredientCols+` FROM ingredients WHERE id = $1::uuid`, id,
	))
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	if u := auth.CurrentUser(c); u != nil {
		ing.stripForRole(u.Role)
	}

	httpx.OK(c, ing)
}

// CreateIngredient menambahkan bahan baku baru dan mencatat audit log.
func (h *Handler) CreateIngredient(c *gin.Context) {
	var req ingredientRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, buyPrice, v := validateIngredientRequest(req)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	var out Ingredient
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		var err error
		out, err = scanIngredient(tx.QueryRow(c.Request.Context(), `
			INSERT INTO ingredients (name, use_unit, buy_unit, buy_unit_size, buy_price, stock, min_stock, rounding, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING `+ingredientCols,
			in.Name, in.UseUnit, in.BuyUnit, in.BuyUnitSize, buyPrice, in.Stock, in.MinStock, in.Rounding, in.IsActive,
		))
		if err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "ingredients", out.ID, map[string]any{
			"op":         "create",
			"ingredient": out,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.Created(c, out)
}

// UpdateIngredient memperbarui bahan baku dan mencatat audit log.
func (h *Handler) UpdateIngredient(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	var req ingredientRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	in, buyPrice, v := validateIngredientRequest(req)
	if !v.OK() {
		v.Respond(c)
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	actor := auth.CurrentUser(c)
	var out Ingredient
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		var err error
		out, err = scanIngredient(tx.QueryRow(c.Request.Context(), `
			UPDATE ingredients
			SET name = $2, use_unit = $3, buy_unit = $4, buy_unit_size = $5,
			    buy_price = $6, stock = $7, min_stock = $8, rounding = $9, is_active = $10
			WHERE id = $1::uuid
			RETURNING `+ingredientCols,
			id, in.Name, in.UseUnit, in.BuyUnit, in.BuyUnitSize, buyPrice, in.Stock, in.MinStock, in.Rounding, in.IsActive,
		))
		if err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "ingredients", out.ID, map[string]any{
			"op":         "update",
			"ingredient": out,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, out)
}

// DeleteIngredient mengarsipkan bahan baku (is_active=false) bila sudah dipakai
// pada resep/pembelian/mutasi stok (FR-MD-08), atau menghapusnya bila belum terpakai.
func (h *Handler) DeleteIngredient(c *gin.Context) {
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
		if err := tx.QueryRow(c.Request.Context(), `SELECT EXISTS(SELECT 1 FROM ingredients WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return httpx.Err(http.StatusNotFound, "not_found", "Bahan baku tidak ditemukan.")
		}

		var inUse bool
		if err := tx.QueryRow(c.Request.Context(), `
			SELECT EXISTS(SELECT 1 FROM recipe_items WHERE ingredient_id = $1::uuid)
			    OR EXISTS(SELECT 1 FROM purchases WHERE ingredient_id = $1::uuid)
			    OR EXISTS(SELECT 1 FROM stock_movements WHERE ingredient_id = $1::uuid)
		`, id).Scan(&inUse); err != nil {
			return err
		}

		if inUse {
			archived = true
			if _, err := tx.Exec(c.Request.Context(), `UPDATE ingredients SET is_active = false WHERE id = $1::uuid`, id); err != nil {
				return err
			}
			return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "ingredients", id, map[string]any{
				"op":       "archive",
				"archived": true,
			})
		}

		if _, err := tx.Exec(c.Request.Context(), `DELETE FROM ingredients WHERE id = $1::uuid`, id); err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "ingredients", id, map[string]any{
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

