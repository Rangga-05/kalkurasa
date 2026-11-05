// Package server provides the shared Gin router used by both cmd/api (local)
// and Vercel's serverless entrypoint.
package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"kalkurasa/backend/internal/config"
)

// New creates and configures the Gin engine with all routes.
// The pool may be nil during early startup; /api/healthz handles this gracefully.
func New(cfg *config.Config, pool *pgxpool.Pool) *gin.Engine {
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// --- Public health check (no DB dependency) ---
	r.GET("/api/healthz", func(c *gin.Context) {
		resp := gin.H{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		}

		// Optional DB ping with short timeout
		if pool != nil {
			dbCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()

			if err := pool.Ping(dbCtx); err != nil {
				resp["db"] = "error"
				resp["db_error"] = sanitizeDBError(err.Error())
			} else {
				resp["db"] = "ok"
			}
		} else {
			resp["db"] = "not_configured"
		}

		c.JSON(http.StatusOK, resp)
	})

	return r
}

// sanitizeDBError removes any connection string, hostname, or password that
// might leak through a pgx error message. Returns a safe, short description.
func sanitizeDBError(raw string) string {
	// pgx errors may embed the DSN or hostname. Strip anything that looks
	// like a connection URI or contains sensitive keywords.
	lower := strings.ToLower(raw)
	for _, keyword := range []string{"password", "postgres://", "postgresql://", "@"} {
		if strings.Contains(lower, keyword) {
			return "koneksi database gagal"
		}
	}
	// Truncate to a reasonable length to avoid leaking internal details.
	if len(raw) > 120 {
		return raw[:120] + "…"
	}
	return raw
}
