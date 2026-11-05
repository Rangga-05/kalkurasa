package auth

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"kalkurasa/backend/internal/httpx"
)

// Handler berisi endpoint /auth/login, /auth/logout, dan /auth/me.
type Handler struct {
	Store        UserStore
	Secret       []byte
	CookieSecure bool
	// ExposeToken menyertakan token di body login (untuk pengujian dengan
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

// Me mengembalikan pengguna yang sedang login.
func (h *Handler) Me(c *gin.Context) {
	httpx.OK(c, CurrentUser(c))
}

