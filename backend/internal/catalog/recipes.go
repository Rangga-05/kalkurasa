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

// RecipeItem merepresentasikan baris tabel recipe_items beserta nama dan satuan bahan.
type RecipeItem struct {
	ProductID      string          `json:"product_id"`
	IngredientID   string          `json:"ingredient_id"`
	IngredientName string          `json:"ingredient_name,omitempty"`
	UseUnit        string          `json:"use_unit,omitempty"`
	QtyPerUnit     decimal.Decimal `json:"qty_per_unit"`
}

type recipeItemInput struct {
	IngredientID string           `json:"ingredient_id"`
	QtyPerUnit   *decimal.Decimal `json:"qty_per_unit"`
}

// recipeUpsertRequest mendukung input satu item (ingredient_id + qty_per_unit)
// maupun daftar item (items) untuk satu product_id.
type recipeUpsertRequest struct {
	ProductID    string            `json:"product_id"`
	IngredientID string            `json:"ingredient_id"`
	QtyPerUnit   *decimal.Decimal  `json:"qty_per_unit"`
	Items        []recipeItemInput `json:"items"`
	ReplaceAll   bool              `json:"replace_all"`
}

// ListRecipes mengembalikan daftar recipe_items (opsional difilter ?product_id=...).
func (h *Handler) ListRecipes(c *gin.Context) {
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

	items, err := queryRecipeItems(c, h.DB, productID)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, items)
}

func queryRecipeItems(c *gin.Context, q db.DBTX, productID string) ([]RecipeItem, error) {
	sql := `
		SELECT r.product_id::text, r.ingredient_id::text, i.name, i.use_unit, r.qty_per_unit
		FROM recipe_items r
		JOIN ingredients i ON i.id = r.ingredient_id
	`
	var args []any
	if productID != "" {
		sql += ` WHERE r.product_id = $1::uuid`
		args = append(args, productID)
	}
	sql += ` ORDER BY r.product_id ASC, i.name ASC`

	rows, err := q.Query(c.Request.Context(), sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]RecipeItem, 0)
	for rows.Next() {
		var it RecipeItem
		if err := rows.Scan(&it.ProductID, &it.IngredientID, &it.IngredientName, &it.UseUnit, &it.QtyPerUnit); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpsertRecipe menambahkan atau memperbarui item resep untuk suatu produk
// (mendukung POST/PUT /api/v1/recipes maupun PUT /api/v1/products/:id/recipes).
func (h *Handler) UpsertRecipe(c *gin.Context) {
	var req recipeUpsertRequest
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
	if len(rawItems) == 0 && (req.IngredientID != "" || req.QtyPerUnit != nil) {
		rawItems = []recipeItemInput{
			{IngredientID: req.IngredientID, QtyPerUnit: req.QtyPerUnit},
		}
	}
	if len(rawItems) == 0 && !req.ReplaceAll {
		v.Add("Minimal satu item resep wajib diisi.")
	}

	qtyOpts := httpx.DecOpts{
		Min:          decimal.Zero,
		Max:          decimal.NewFromInt(999_999_999),
		MinExclusive: true,
		MaxScale:     6,
	}

	type cleanItem struct {
		IngredientID string          `json:"ingredient_id"`
		QtyPerUnit   decimal.Decimal `json:"qty_per_unit"`
	}
	seen := make(map[string]bool, len(rawItems))
	clean := make([]cleanItem, 0, len(rawItems))
	for idx, it := range rawItems {
		iid := strings.ToLower(strings.TrimSpace(it.IngredientID))
		if !httpx.ValidUUID(iid) {
			v.Add(fmt.Sprintf("Item resep #%d: ingredient_id harus berformat UUID yang valid.", idx+1))
		} else if seen[iid] {
			v.Add(fmt.Sprintf("Item resep #%d: ingredient_id duplikat dalam satu permintaan.", idx+1))
		} else {
			seen[iid] = true
		}
		qty := v.Dec(fmt.Sprintf("Takaran bahan #%d (qty_per_unit)", idx+1), it.QtyPerUnit, qtyOpts)
		clean = append(clean, cleanItem{IngredientID: iid, QtyPerUnit: qty})
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
	var out []RecipeItem
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		if req.ReplaceAll {
			if _, err := tx.Exec(c.Request.Context(), `DELETE FROM recipe_items WHERE product_id = $1::uuid`, productID); err != nil {
				return err
			}
		}
		for _, it := range clean {
			_, err := tx.Exec(c.Request.Context(), `
				INSERT INTO recipe_items (product_id, ingredient_id, qty_per_unit)
				VALUES ($1::uuid, $2::uuid, $3)
				ON CONFLICT (product_id, ingredient_id) DO UPDATE
				SET qty_per_unit = EXCLUDED.qty_per_unit
			`, productID, it.IngredientID, it.QtyPerUnit)
			if err != nil {
				return err
			}
		}

		if err := audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "recipe_items", productID, map[string]any{
			"op":          "upsert",
			"product_id":  productID,
			"replace_all": req.ReplaceAll,
			"items":       clean,
		}); err != nil {
			return err
		}

		var err error
		out, err = queryRecipeItems(c, tx, productID)
		return err
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, out)
}

// DeleteRecipeItem menghapus satu baris recipe_items berdasarkan product_id dan ingredient_id.
func (h *Handler) DeleteRecipeItem(c *gin.Context) {
	productID, ok := httpx.ParamUUID(c, "product_id")
	if !ok {
		return
	}
	ingredientID, ok := httpx.ParamUUID(c, "ingredient_id")
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
			DELETE FROM recipe_items
			WHERE product_id = $1::uuid AND ingredient_id = $2::uuid
		`, productID, ingredientID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return httpx.Err(http.StatusNotFound, "not_found", "Item resep tidak ditemukan.")
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "recipe_items", productID, map[string]any{
			"op":            "delete",
			"product_id":    productID,
			"ingredient_id": ingredientID,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, gin.H{
		"product_id":    productID,
		"ingredient_id": ingredientID,
		"deleted":       true,
	})
}

