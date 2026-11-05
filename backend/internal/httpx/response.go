package httpx

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Meta adalah informasi paginasi pada respons daftar.
type Meta struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

// OK membalas 200 dengan {"data": ...}.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Created membalas 201 dengan {"data": ...}.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

// List membalas 200 dengan {"data": [...], "meta": {...}}.
func List(c *gin.Context, data any, meta Meta) {
	c.JSON(http.StatusOK, gin.H{"data": data, "meta": meta})
}

// Batas paginasi.
const (
	defaultLimit = 50
	maxLimit     = 200
)

// Paginate membaca ?page= dan ?limit= (default 1 dan 50, maksimal 200).
// Nilai yang tidak valid diganti dengan nilai default.
func Paginate(c *gin.Context) (page, limit, offset int) {
	page, _ = strconv.Atoi(c.Query("page"))
	if page < 1 {
		page = 1
	}
	limit, _ = strconv.Atoi(c.Query("limit"))
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	return page, limit, (page - 1) * limit
}

