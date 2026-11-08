// Package users menangani manajemen akun pengguna internal (khusus peran admin).
package users

import (
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"kalkurasa/backend/internal/audit"
	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
)

// Handler menyediakan endpoint CRUD /api/v1/users (khusus admin).
type Handler struct {
	DB db.Conn
}

const userCols = `id::text, email, name, contact, role, is_active, created_at`

type createUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
	Contact  string `json:"contact"`
	Phone    string `json:"phone"`
	IsActive *bool  `json:"is_active"`
}

type updateUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Password string `json:"password"`
	Contact  string `json:"contact"`
	Phone    string `json:"phone"`
	IsActive *bool  `json:"is_active"`
}

// List mengembalikan daftar pengguna dengan paginasi.
func (h *Handler) List(c *gin.Context) {
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}
	page, limit, offset := httpx.Paginate(c)
	ctx := c.Request.Context()

	var total int
	if err := h.DB.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		httpx.HandleError(c, err)
		return
	}

	rows, err := h.DB.Query(ctx,
		`SELECT `+userCols+` FROM users ORDER BY created_at ASC, id ASC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}
	defer rows.Close()

	items := make([]auth.User, 0)
	for rows.Next() {
		var u auth.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Contact, &u.Role, &u.IsActive, &u.CreatedAt); err != nil {
			httpx.HandleError(c, err)
			return
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.List(c, items, httpx.Meta{Page: page, Limit: limit, Total: total})
}

// Get mengembalikan satu pengguna berdasarkan ID.
func (h *Handler) Get(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	var u auth.User
	err := h.DB.QueryRow(c.Request.Context(),
		`SELECT `+userCols+` FROM users WHERE id = $1::uuid`, id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Contact, &u.Role, &u.IsActive, &u.CreatedAt)
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, u)
}

// Create membuat pengguna baru dengan kata sandi di-hash bcrypt dan mencatat audit log.
func (h *Handler) Create(c *gin.Context) {
	var req createUserRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	v := httpx.NewValidation()
	email := v.Email("Email", req.Email)
	name := v.Str("Nama pengguna", req.Name, 120)
	role := v.OneOf("Peran", req.Role, auth.Roles()...)
	password := v.Password("Kata sandi", req.Password)
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	if !v.OK() {
		v.Respond(c)
		return
	}

	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	pwHash, err := auth.HashPassword(password)
	if err != nil {
		httpx.InternalError(c)
		return
	}

	actor := auth.CurrentUser(c)
	var out auth.User
	err = db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(c.Request.Context(), `
			INSERT INTO users (email, name, role, password_hash, is_active)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING `+userCols,
			email, name, role, pwHash, active,
		).Scan(&out.ID, &out.Email, &out.Name, &out.Contact, &out.Role, &out.IsActive, &out.CreatedAt)
		if err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "users", out.ID, map[string]any{
			"op":   "create",
			"user": out,
		})
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			httpx.Conflict(c, "Email sudah terdaftar pada pengguna lain.")
			return
		}
		httpx.HandleError(c, err)
		return
	}

	httpx.Created(c, out)
}

// Update memperbarui nama, peran, status aktif, email (opsional), dan kata sandi (opsional).
func (h *Handler) Update(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}

	var req updateUserRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	v := httpx.NewValidation()
	var email string
	if req.Email != "" {
		email = v.Email("Email", req.Email)
	}
	name := v.Str("Nama pengguna", req.Name, 120)
	role := v.OneOf("Peran", req.Role, auth.Roles()...)
	var password string
	if req.Password != "" {
		password = v.Password("Kata sandi baru", req.Password)
	}

	actor := auth.CurrentUser(c)
	if actor != nil && actor.ID == id {
		if req.IsActive != nil && !*req.IsActive {
			v.Add("Anda tidak dapat menonaktifkan akun Anda sendiri.")
		}
		if role != "" && role != auth.RoleAdmin {
			v.Add("Anda tidak dapat mengubah peran admin pada akun Anda sendiri.")
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

	var pwHash string
	if password != "" {
		var err error
		pwHash, err = auth.HashPassword(password)
		if err != nil {
			httpx.InternalError(c)
			return
		}
	}

	var out auth.User
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		var cur auth.User
		err := tx.QueryRow(c.Request.Context(),
			`SELECT `+userCols+` FROM users WHERE id = $1::uuid`, id,
		).Scan(&cur.ID, &cur.Email, &cur.Name, &cur.Contact, &cur.Role, &cur.IsActive, &cur.CreatedAt)
		if err != nil {
			return err
		}

		newEmail := cur.Email
		if email != "" {
			newEmail = email
		}
		newActive := cur.IsActive
		if req.IsActive != nil {
			newActive = *req.IsActive
		}

		if pwHash != "" {
			err = tx.QueryRow(c.Request.Context(), `
				UPDATE users
				SET email = $2, name = $3, role = $4, is_active = $5, password_hash = $6
				WHERE id = $1::uuid
				RETURNING `+userCols,
				id, newEmail, name, role, newActive, pwHash,
			).Scan(&out.ID, &out.Email, &out.Name, &out.Contact, &out.Role, &out.IsActive, &out.CreatedAt)
		} else {
			err = tx.QueryRow(c.Request.Context(), `
				UPDATE users
				SET email = $2, name = $3, role = $4, is_active = $5
				WHERE id = $1::uuid
				RETURNING `+userCols,
				id, newEmail, name, role, newActive,
			).Scan(&out.ID, &out.Email, &out.Name, &out.Contact, &out.Role, &out.IsActive, &out.CreatedAt)
		}
		if err != nil {
			return err
		}

		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "users", out.ID, map[string]any{
			"op":               "update",
			"user":             out,
			"password_changed": pwHash != "",
		})
	})
	if err != nil {
		if db.IsUniqueViolation(err) {
			httpx.Conflict(c, "Email sudah terdaftar pada pengguna lain.")
			return
		}
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, out)
}

// Delete mengarsipkan (menonaktifkan) pengguna (is_active=false) dan mencatat audit log.
func (h *Handler) Delete(c *gin.Context) {
	id, ok := httpx.ParamUUID(c, "id")
	if !ok {
		return
	}
	actor := auth.CurrentUser(c)
	if actor != nil && actor.ID == id {
		httpx.UnprocessableEntity(c, "validation_error", "Anda tidak dapat menonaktifkan akun Anda sendiri.")
		return
	}
	if h.DB == nil {
		httpx.DBUnavailable(c)
		return
	}

	var out auth.User
	err := db.WithTx(c.Request.Context(), h.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(c.Request.Context(), `
			UPDATE users SET is_active = false
			WHERE id = $1::uuid
			RETURNING `+userCols, id,
		).Scan(&out.ID, &out.Email, &out.Name, &out.Contact, &out.Role, &out.IsActive, &out.CreatedAt)
		if err != nil {
			return err
		}
		return audit.Log(c.Request.Context(), tx, actor.ID, audit.ActionMaster, "users", out.ID, map[string]any{
			"op":       "archive",
			"archived": true,
		})
	})
	if err != nil {
		httpx.HandleError(c, err)
		return
	}

	httpx.OK(c, gin.H{
		"id":        out.ID,
		"archived":  true,
		"is_active": false,
	})
}
