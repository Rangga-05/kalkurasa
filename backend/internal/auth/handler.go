package auth

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kalkurasa/backend/internal/db"
	"kalkurasa/backend/internal/httpx"
)

// RegisterRateLimitPerMin adalah batas pendaftaran akun per menit per IP.
const RegisterRateLimitPerMin = 20

// Handler berisi endpoint /auth/register, /auth/login, /auth/logout, dan /auth/me.
type Handler struct {
	Store        UserStore
	DB           db.DBTX
	Secret       []byte
	CookieSecure bool
	// ExposeToken menyertakan token di body login/register (untuk pengujian dengan
	// header Authorization). Dimatikan di produksi.
	ExposeToken bool
	Now         func() time.Time
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

const msgInvalidCredentials = "Email atau kata sandi salah."

// Login memverifikasi email dan kata sandi (bcrypt), lalu menerbitkan JWT dan
// mengatur cookie sesi httpOnly, SameSite=Lax, dan Secure bila COOKIE_SECURE.
// Pesan gagal sama untuk email tidak ada, kata sandi salah, dan akun nonaktif.
func (h *Handler) Login(c *gin.Context) {
	if !SecretOK(h.Secret) {
		httpx.Unavailable(c, "Autentikasi belum dikonfigurasi di server.")
		return
	}

	var req loginRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	v := httpx.NewValidation()
	email := v.Email("Email", req.Email)
	if req.Password == "" {
		v.Add("Kata sandi wajib diisi.")
	}
	if !v.OK() {
		v.Respond(c)
		return
	}

	rec, err := h.Store.FindByEmail(c.Request.Context(), email)
	switch {
	case errors.Is(err, ErrNotFound):
		burnPasswordCheck(req.Password)
		httpx.Unauthorized(c, msgInvalidCredentials)
		return
	case errors.Is(err, ErrUnavailable):
		httpx.DBUnavailable(c)
		return
	case err != nil:
		log.Printf("ERROR login lookup: %v", err)
		httpx.InternalError(c)
		return
	}

	if !CheckPassword(rec.PasswordHash, req.Password) || !rec.IsActive {
		httpx.Unauthorized(c, msgInvalidCredentials)
		return
	}

	token, exp, err := IssueToken(h.Secret, rec.ID, rec.Role, h.now(), TokenTTL)
	if err != nil {
		log.Printf("ERROR issue token: %v", err)
		httpx.InternalError(c)
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, token, int(TokenTTL.Seconds()), "/", "", h.CookieSecure, true)

	data := gin.H{"user": rec.User, "expires_at": exp}
	if h.ExposeToken {
		data["token"] = token
	}
	httpx.OK(c, data)
}

// Logout menghapus cookie sesi. Token bersifat stateless sehingga tidak ada
// daftar cabut; token yang sudah bocor tetap berlaku sampai kedaluwarsa.
func (h *Handler) Logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, "", -1, "/", "", h.CookieSecure, true)
	httpx.OK(c, gin.H{"message": "Anda telah keluar."})
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Contact  string `json:"contact"`
	Password string `json:"password"`
	// Role diterima pada struct semata-mata agar injeksi field "role" pada body JSON
	// diabaikan dan selalu dipaksa menjadi RoleCustomer ("customer").
	Role string `json:"role"`
}

func clientIP(c *gin.Context) string {
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

func enforceRegisterRateLimit(c *gin.Context, q db.DBTX) bool {
	if q == nil {
		return true
	}
	ctx := c.Request.Context()
	_, _ = q.Exec(ctx, `
		DELETE FROM rate_limits
		WHERE window_start < date_trunc('minute', now()) - interval '10 minutes'
	`)
	key := "register:" + clientIP(c)
	var count int
	err := q.QueryRow(ctx, `
		INSERT INTO rate_limits (key, window_start, count)
		VALUES ($1, date_trunc('minute', now()), 1)
		ON CONFLICT (key, window_start)
		DO UPDATE SET count = rate_limits.count + 1
		RETURNING count
	`, key).Scan(&count)
	if err != nil {
		httpx.HandleError(c, err)
		return false
	}
	if count > RegisterRateLimitPerMin {
		httpx.RespondError(
			c,
			http.StatusTooManyRequests,
			"rate_limited",
			"Terlalu banyak permintaan pendaftaran dari alamat Anda. Silakan tunggu satu menit lalu coba lagi.",
		)
		return false
	}
	return true
}

// Register menangani POST /api/v1/auth/register (publik, dibatasi laju via rate_limits).
// Membuat akun baru dengan peran customer dan langsung menerbitkan sesi login (cookie yang sama).
// Field role dari body diabaikan dan selalu dipaksa menjadi 'customer'.
func (h *Handler) Register(c *gin.Context) {
	if !SecretOK(h.Secret) {
		httpx.Unavailable(c, "Autentikasi belum dikonfigurasi di server.")
		return
	}

	if !enforceRegisterRateLimit(c, h.DB) {
		return
	}

	var req registerRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	v := httpx.NewValidation()
	name := v.Str("Nama lengkap", req.Name, 120)
	email := v.Email("Email", req.Email)
	contact := v.OptStr("Nomor kontak", req.Contact, 80)
	password := v.Password("Kata sandi", req.Password)
	if !v.OK() {
		v.Respond(c)
		return
	}

	pwHash, err := HashPassword(password)
	if err != nil {
		httpx.UnprocessableEntity(c, "validation_error", "Kata sandi tidak valid.")
		return
	}

	// Paksa role selalu menjadi RoleCustomer ('customer'), abaikan req.Role bila dikirim.
	user, err := h.Store.CreateCustomer(c.Request.Context(), email, name, contact, pwHash)
	switch {
	case errors.Is(err, ErrEmailTaken):
		httpx.Conflict(c, "Email sudah terdaftar. Silakan gunakan email lain atau masuk ke akun Anda.")
		return
	case errors.Is(err, ErrUnavailable):
		httpx.DBUnavailable(c)
		return
	case err != nil:
		log.Printf("ERROR register customer: %v", err)
		httpx.InternalError(c)
		return
	}

	token, exp, err := IssueToken(h.Secret, user.ID, user.Role, h.now(), TokenTTL)
	if err != nil {
		log.Printf("ERROR issue token on register: %v", err)
		httpx.InternalError(c)
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, token, int(TokenTTL.Seconds()), "/", "", h.CookieSecure, true)

	data := gin.H{"user": *user, "expires_at": exp}
	if h.ExposeToken {
		data["token"] = token
	}
	httpx.Created(c, data)
}

// Me mengembalikan pengguna yang sedang login.
func (h *Handler) Me(c *gin.Context) {
	httpx.OK(c, CurrentUser(c))
}

