package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
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

// RespondError sends a JSON error response with the given HTTP status.
func RespondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{
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

