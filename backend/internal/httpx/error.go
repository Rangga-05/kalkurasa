package httpx

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"kalkurasa/backend/internal/db"
)

// ErrorResponse is the standard JSON error envelope.
//
//	{ "error": { "code": "...", "message": "..." } }
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the error code and a human-readable message (Indonesian).
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AppError adalah error bisnis yang membawa status HTTP. Fungsi di dalam
// transaksi mengembalikannya agar handler dapat membalas dengan benar.
type AppError struct {
	Status  int
	Code    string
	Message string
}

func (e *AppError) Error() string { return e.Message }

// Err membuat AppError.
func Err(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

// RespondError sends a JSON error response with the given HTTP status.
func RespondError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}

// BadRequest sends a 400 error.
func BadRequest(c *gin.Context, code, message string) {
	RespondError(c, http.StatusBadRequest, code, message)
}

// Unauthorized sends a 401 error.
func Unauthorized(c *gin.Context, message string) {
	RespondError(c, http.StatusUnauthorized, "unauthorized", message)
}

// Forbidden sends a 403 error.
func Forbidden(c *gin.Context, message string) {
	RespondError(c, http.StatusForbidden, "forbidden", message)
}

// NotFound sends a 404 error.
func NotFound(c *gin.Context, message string) {
	RespondError(c, http.StatusNotFound, "not_found", message)
}

// Conflict sends a 409 error.
func Conflict(c *gin.Context, message string) {
	RespondError(c, http.StatusConflict, "conflict", message)
}

// UnprocessableEntity sends a 422 error.
func UnprocessableEntity(c *gin.Context, code, message string) {
	RespondError(c, http.StatusUnprocessableEntity, code, message)
}

// InternalError sends a 500 error with a generic message.
func InternalError(c *gin.Context) {
	RespondError(c, http.StatusInternalServerError, "internal_error", "Terjadi kesalahan pada server.")
}

// Unavailable sends a 503 error.
func Unavailable(c *gin.Context, message string) {
	RespondError(c, http.StatusServiceUnavailable, "unavailable", message)
}

// DBUnavailable membalas 503 saat database belum dikonfigurasi.
func DBUnavailable(c *gin.Context) {
	Unavailable(c, "Database belum dikonfigurasi atau tidak dapat dihubungi.")
}

// HandleError memetakan error dari lapisan data ke respons JSON seragam.
// Pesan error database mentah tidak pernah dikirim ke klien.
func HandleError(c *gin.Context, err error) {
	var appErr *AppError
	switch {
	case errors.As(err, &appErr):
		RespondError(c, appErr.Status, appErr.Code, appErr.Message)
	case errors.Is(err, pgx.ErrNoRows):
		NotFound(c, "Data tidak ditemukan.")
	case db.IsUniqueViolation(err):
		Conflict(c, "Data dengan nilai tersebut sudah ada.")
	case db.IsForeignKeyViolation(err):
		Conflict(c, "Data masih terhubung dengan data lain.")
	case db.IsCheckViolation(err):
		UnprocessableEntity(c, "validation_error", "Nilai tidak memenuhi batasan data.")
	default:
		log.Printf("ERROR %s %s: %v", c.Request.Method, c.FullPath(), err)
		InternalError(c)
	}
}
