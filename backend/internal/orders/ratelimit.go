package orders

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
)

// Batas permintaan per menit per IP untuk endpoint publik (Bagian 6).
const (
	PublicOrderRateLimitPerMin = 30
	PublicAvailRateLimitPerMin = 60
)

// ClientIP mengambil alamat IP klien dari elemen pertama X-Forwarded-For
// (sesuai Bagian 6 brief) atau fallback ke Gin ClientIP.
func ClientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		if ip := strings.TrimSpace(first); ip != "" {
			return ip
		}
	}
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	return "unknown"
}

// AllowRate mencatat dan memeriksa kuota permintaan pada tabel rate_limits
// secara atomik di database (stateless untuk lingkungan serverless Vercel).
// Sekaligus membersihkan baris lama secara malas (lazy cleanup).
func AllowRate(ctx context.Context, q db.DBTX, key string, limitPerMin int) (bool, error) {
	// Lazy cleanup baris yang sudah lewat lebih dari 10 menit
	_, _ = q.Exec(ctx, `
		DELETE FROM rate_limits
		WHERE window_start < date_trunc('minute', now()) - interval '10 minutes'
	`)

	var count int
	err := q.QueryRow(ctx, `
		INSERT INTO rate_limits (key, window_start, count)
		VALUES ($1, date_trunc('minute', now()), 1)
		ON CONFLICT (key, window_start)
		DO UPDATE SET count = rate_limits.count + 1
		RETURNING count
	`, key).Scan(&count)
	if err != nil {
		return false, err
	}
	return count <= limitPerMin, nil
}

// EnforceRateLimit memeriksa batas laju untuk prefix tertentu dan mengirim
// 429 Too Many Requests bila batas terlampaui.
func EnforceRateLimit(c *gin.Context, q db.DBTX, prefix string, limitPerMin int) bool {
	key := prefix + ":" + ClientIP(c)
	allowed, err := AllowRate(c.Request.Context(), q, key, limitPerMin)
	if err != nil {
		httpx.HandleError(c, err)
		return false
	}
	if !allowed {
		httpx.RespondError(
			c,
			http.StatusTooManyRequests,
			"rate_limited",
			"Terlalu banyak permintaan dari alamat Anda. Silakan tunggu satu menit lalu coba lagi.",
		)
		return false
	}
	return true
}

