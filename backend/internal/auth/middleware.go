package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kalkurasa/backend/internal/httpx"
)

const userContextKey = "auth.user"

// Guard menyediakan middleware autentikasi.
type Guard struct {
	Store          UserStore
	Secret         []byte
	Now            func() time.Time
	AllowedOrigins []string
}

func (g *Guard) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

// CurrentUser mengambil pengguna yang sudah terautentikasi dari konteks.
func CurrentUser(c *gin.Context) *User {
	if v, ok := c.Get(userContextKey); ok {
		if u, ok := v.(*User); ok {
			return u
		}
	}
	return nil
}

func isUnsafeMethod(m string) bool {
	return m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions
}

// tokenFromRequest mengambil token dari header Authorization: Bearer
// (untuk pengujian) atau cookie sesi. fromCookie true bila dari cookie.
func tokenFromRequest(c *gin.Context) (token string, fromCookie bool) {
	if h := c.GetHeader("Authorization"); h != "" {
		if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
			return strings.TrimSpace(h[7:]), false
		}
		return "", false
	}
	if cookie, err := c.Cookie(CookieName); err == nil {
		return cookie, true
	}
	return "", false
}

// Authenticate memverifikasi token lalu memuat pengguna dari database pada
// setiap permintaan, sehingga penonaktifan akun dan perubahan peran langsung
// berlaku. Peran yang dipakai adalah peran di database, bukan klaim token.
//
// Untuk permintaan yang mengubah data dengan cookie sesi, header Origin (bila
// ada) harus termasuk ALLOWED_ORIGINS sebagai lapisan tambahan anti-CSRF di
// atas SameSite=Lax.
func (g *Guard) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !SecretOK(g.Secret) {
			httpx.Unavailable(c, "Autentikasi belum dikonfigurasi di server.")
			return
		}

		token, fromCookie := tokenFromRequest(c)
		if token == "" {
			httpx.Unauthorized(c, "Anda belum masuk. Silakan login terlebih dahulu.")
			return
		}

		claims, err := ParseToken(g.Secret, token, g.now())
		if err != nil {
			httpx.Unauthorized(c, "Sesi tidak valid atau sudah berakhir. Silakan login kembali.")
			return
		}

		if fromCookie && isUnsafeMethod(c.Request.Method) {
			if origin := c.GetHeader("Origin"); origin != "" && !httpx.OriginAllowed(origin, g.AllowedOrigins) {
				log.Printf("WARN CSRF origin mismatch: received Origin %q, allowed %v", origin, g.AllowedOrigins)
				httpx.Forbidden(c, "Permintaan ditolak: asal permintaan tidak diizinkan.")
				return
			}
		}

		user, err := g.Store.FindByID(c.Request.Context(), claims.Subject)
		switch {
		case errors.Is(err, ErrNotFound):
			httpx.Unauthorized(c, "Akun tidak ditemukan. Silakan login kembali.")
			return
		case errors.Is(err, ErrUnavailable):
			httpx.DBUnavailable(c)
			return
		case err != nil:
			log.Printf("ERROR auth lookup: %v", err)
			httpx.InternalError(c)
			return
		}
		if !user.IsActive {
			httpx.Unauthorized(c, "Akun Anda dinonaktifkan. Hubungi admin.")
			return
		}

		c.Set(userContextKey, user)
		c.Next()
	}
}

// RequireRole hanya meneruskan permintaan dari pengguna dengan salah satu
// peran yang diizinkan. Harus dipasang setelah Authenticate. Penegakan hak
// akses selalu di server (FR-AUTH-02).
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			httpx.Unauthorized(c, "Anda belum masuk. Silakan login terlebih dahulu.")
			return
		}
		for _, r := range roles {
			if u.Role == r {
				c.Next()
				return
			}
		}
		httpx.Forbidden(c, "Anda tidak memiliki izin untuk mengakses fitur ini.")
	}
}

