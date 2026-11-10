package httpx

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// matchWildcard memeriksa apakah pattern (misal "https://*.vercel.app") cocok dengan s.
func matchWildcard(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return strings.EqualFold(pattern, s)
	}
	sLower := strings.ToLower(s)
	if !strings.HasPrefix(sLower, strings.ToLower(parts[0])) {
		return false
	}
	rem := sLower[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		pLower := strings.ToLower(parts[i])
		idx := strings.Index(rem, pLower)
		if idx < 0 {
			return false
		}
		rem = rem[idx+len(pLower):]
	}
	return strings.HasSuffix(rem, strings.ToLower(parts[len(parts)-1]))
}

// OriginAllowed melaporkan apakah origin ada di daftar asal yang diizinkan.
// Mendukung:
// 1. Wildcard penuh "*" (mengizinkan semua origin).
// 2. Wildcard subdomain/domain seperti "https://*.vercel.app".
// 3. Exact match dengan normalisasi trailing slash dan case-insensitive.
func OriginAllowed(origin string, allowed []string) bool {
	normOrigin := strings.TrimRight(strings.TrimSpace(origin), "/")
	if normOrigin == "" {
		return false
	}
	for _, a := range allowed {
		cleanA := strings.TrimRight(strings.TrimSpace(a), "/")
		if cleanA == "" {
			continue
		}
		if cleanA == "*" {
			return true
		}
		if strings.EqualFold(cleanA, normOrigin) {
			return true
		}
		if strings.Contains(cleanA, "*") && matchWildcard(cleanA, normOrigin) {
			return true
		}
	}
	return false
}

// CORS hanya mengizinkan origin yang ada di ALLOWED_ORIGINS (Bagian 6).
// Origin lain tidak mendapat header CORS sehingga browser menolaknya.
func CORS(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && OriginAllowed(origin, allowed) {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")
			if c.Request.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				h.Set("Access-Control-Max-Age", "600")
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

