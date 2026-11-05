package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"kalkurasa/backend/internal/auth"
	"kalkurasa/backend/internal/config"
	"kalkurasa/backend/internal/server"
)

type fakeUserStore struct {
	byEmail map[string]*auth.UserWithHash
	byID    map[string]*auth.User
}

func newFakeUserStore(t *testing.T, password string) *fakeUserStore {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	users := []auth.User{
		{ID: "11111111-1111-1111-1111-111111111111", Email: "owner@demo.test", Name: "Bu Sari (Pemilik)", Role: auth.RoleOwner, IsActive: true, CreatedAt: now},
		{ID: "22222222-2222-2222-2222-222222222222", Email: "manager@demo.test", Name: "Manajer Operasional", Role: auth.RoleManager, IsActive: true, CreatedAt: now},
		{ID: "33333333-3333-3333-3333-333333333333", Email: "staff@demo.test", Name: "Karyawan Produksi", Role: auth.RoleStaff, IsActive: true, CreatedAt: now},
		{ID: "44444444-4444-4444-4444-444444444444", Email: "admin@demo.test", Name: "Admin Sistem", Role: auth.RoleAdmin, IsActive: true, CreatedAt: now},
		{ID: "55555555-5555-5555-5555-555555555555", Email: "inactive@demo.test", Name: "Nonaktif", Role: auth.RoleStaff, IsActive: false, CreatedAt: now},
	}
	s := &fakeUserStore{
		byEmail: make(map[string]*auth.UserWithHash, len(users)),
		byID:    make(map[string]*auth.User, len(users)),
	}
	for _, u := range users {
		uCopy := u
		s.byEmail[u.Email] = &auth.UserWithHash{User: uCopy, PasswordHash: hash}
		s.byID[u.ID] = &uCopy
	}
	return s
}

func (s *fakeUserStore) FindByEmail(_ context.Context, email string) (*auth.UserWithHash, error) {
	u, ok := s.byEmail[strings.ToLower(email)]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func (s *fakeUserStore) FindByID(_ context.Context, id string) (*auth.User, error) {
	u, ok := s.byID[id]
	if !ok {
		return nil, auth.ErrNotFound
	}
	return u, nil
}

func setupTestServer(t *testing.T) (*gin.Engine, *fakeUserStore, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	pw := "DemoKalkurasa2026!"
	store := newFakeUserStore(t, pw)
	cfg := &config.Config{
		Port:           "8080",
		JWTSecret:      "0123456789abcdef0123456789abcdef-kalkurasa-test-secret",
		AppEnv:         "development",
		AllowedOrigins: "http://localhost:3000",
		CookieSecure:   false,
		TZ:             "Asia/Jakarta",
	}
	return server.NewWithStore(cfg, nil, store), store, pw
}

type apiErrorResp struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func doJSON(r http.Handler, method, path string, body any, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		switch v := body.(type) {
		case string:
			buf.WriteString(v)
		default:
			_ = json.NewEncoder(&buf).Encode(body)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func loginAs(t *testing.T, r http.Handler, email, password string) (token string, sessionCookie *http.Cookie) {
	t.Helper()
	w := doJSON(r, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on login for %s, got %d: %s", email, w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Token string    `json:"token"`
			User  auth.User `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	for _, ck := range w.Result().Cookies() {
		if ck.Name == auth.CookieName {
			sessionCookie = ck
			break
		}
	}
	return resp.Data.Token, sessionCookie
}

func TestAuthLoginSuccessAndMeAndLogout(t *testing.T) {
	r, _, pw := setupTestServer(t)

	token, ck := loginAs(t, r, "owner@demo.test", pw)
	if token == "" {
		t.Fatalf("expected non-empty token in development mode")
	}
	if ck == nil {
		t.Fatalf("expected session cookie to be set")
	}
	if !ck.HttpOnly {
		t.Errorf("expected session cookie to be HttpOnly")
	}
	if ck.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSiteLaxMode, got %v", ck.SameSite)
	}

	// GET /api/v1/auth/me using Cookie
	wCookie := doJSON(r, http.MethodGet, "/api/v1/auth/me", nil, nil, ck)
	if wCookie.Code != http.StatusOK {
		t.Fatalf("GET /auth/me with cookie: got %d (%s)", wCookie.Code, wCookie.Body.String())
	}

	// GET /api/v1/auth/me using Authorization: Bearer
	wBearer := doJSON(r, http.MethodGet, "/api/v1/auth/me", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})
	if wBearer.Code != http.StatusOK {
		t.Fatalf("GET /auth/me with bearer: got %d (%s)", wBearer.Code, wBearer.Body.String())
	}
	var meResp struct {
		Data auth.User `json:"data"`
	}
	if err := json.Unmarshal(wBearer.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("unmarshal /auth/me: %v", err)
	}
	if meResp.Data.Email != "owner@demo.test" || meResp.Data.Role != auth.RoleOwner {
		t.Errorf("unexpected user in /auth/me: %+v", meResp.Data)
	}

	// POST /api/v1/auth/logout clears cookie
	wLogout := doJSON(r, http.MethodPost, "/api/v1/auth/logout", nil, nil, ck)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("POST /auth/logout: got %d", wLogout.Code)
	}
	var cleared *http.Cookie
	for _, c := range wLogout.Result().Cookies() {
		if c.Name == auth.CookieName {
			cleared = c
			break
		}
	}
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("expected session cookie to be cleared with MaxAge < 0, got %+v", cleared)
	}
}

func TestAuthLoginFailureCases(t *testing.T) {
	r, _, pw := setupTestServer(t)

	cases := []struct {
		name     string
		email    string
		password string
	}{
		{name: "wrong_password", email: "owner@demo.test", password: "WrongPassword123"},
		{name: "unknown_email", email: "nobody@demo.test", password: pw},
		{name: "inactive_user", email: "inactive@demo.test", password: pw},
	}

	var firstMsg string
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(r, http.MethodPost, "/api/v1/auth/login", map[string]string{
				"email":    tc.email,
				"password": tc.password,
			}, nil)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
			}
			var errResp apiErrorResp
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if errResp.Error.Code != "unauthorized" || errResp.Error.Message == "" {
				t.Errorf("unexpected error payload: %+v", errResp)
			}
			if firstMsg == "" {
				firstMsg = errResp.Error.Message
			} else if errResp.Error.Message != firstMsg {
				t.Errorf("expected identical failure message %q, got %q", firstMsg, errResp.Error.Message)
			}
		})
	}
}

func TestTC09_RoleAccessDeniedForStaff(t *testing.T) {
	r, store, pw := setupTestServer(t)

	staffToken, staffCookie := loginAs(t, r, "staff@demo.test", pw)
	authHdr := map[string]string{"Authorization": "Bearer " + staffToken}

	// TC-09 (bagian karyawan): Karyawan membuka endpoint kas, settings, produk, atau users -> 403 Forbidden dengan pesan jelas
	forbiddenRoutes := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/cash/projection"},
		{method: http.MethodPost, path: "/api/v1/cash/entries"},
		{method: http.MethodGet, path: "/api/v1/settings"},
		{method: http.MethodPut, path: "/api/v1/settings"},
		{method: http.MethodGet, path: "/api/v1/products"},
		{method: http.MethodPost, path: "/api/v1/products"},
		{method: http.MethodPost, path: "/api/v1/ingredients"},
		{method: http.MethodGet, path: "/api/v1/users"},
	}

	for _, rt := range forbiddenRoutes {
		w := doJSON(r, rt.method, rt.path, nil, authHdr)
		if w.Code != http.StatusForbidden {
			t.Fatalf("TC-09 %s %s for staff: expected 403, got %d (%s)", rt.method, rt.path, w.Code, w.Body.String())
		}
		var errResp apiErrorResp
		if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("unmarshal 403 response: %v", err)
		}
		if errResp.Error.Code != "forbidden" || !strings.Contains(strings.ToLower(errResp.Error.Message), "izin") {
			t.Errorf("TC-09 expected clear Indonesian forbidden message, got: %+v", errResp.Error)
		}
	}

	// Staff IS allowed to reach GET /api/v1/ingredients (passes RequireRole; returns 503 when DB is nil in unit test)
	wStaffIng := doJSON(r, http.MethodGet, "/api/v1/ingredients", nil, authHdr)
	if wStaffIng.Code != http.StatusServiceUnavailable {
		t.Errorf("expected staff GET /ingredients to pass RBAC (503 without DB), got %d", wStaffIng.Code)
	}

	// Manager denied on PUT /settings and GET /users -> 403
	mgrToken, _ := loginAs(t, r, "manager@demo.test", pw)
	mgrHdr := map[string]string{"Authorization": "Bearer " + mgrToken}
	if w := doJSON(r, http.MethodPut, "/api/v1/settings", map[string]any{"margin_min": 0.25}, mgrHdr); w.Code != http.StatusForbidden {
		t.Errorf("expected manager PUT /settings to be 403, got %d", w.Code)
	}
	if w := doJSON(r, http.MethodGet, "/api/v1/users", nil, mgrHdr); w.Code != http.StatusForbidden {
		t.Errorf("expected manager GET /users to be 403, got %d", w.Code)
	}

	// Unauthenticated request -> 401
	wUnauth := doJSON(r, http.MethodGet, "/api/v1/cash/projection", nil, nil)
	if wUnauth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", wUnauth.Code)
	}

	// Cookie-based POST from disallowed Origin -> 403 anti-CSRF
	wCSRF := doJSON(r, http.MethodPost, "/api/v1/cash/entries", nil, map[string]string{
		"Origin": "https://evil.example.com",
	}, staffCookie)
	if wCSRF.Code != http.StatusForbidden {
		t.Errorf("expected 403 for disallowed Origin on cookie mutation, got %d", wCSRF.Code)
	}

	// Deactivated user after token issued -> 401 immediately (stateless DB re-check)
	store.byID["33333333-3333-3333-3333-333333333333"].IsActive = false
	wDeact := doJSON(r, http.MethodGet, "/api/v1/auth/me", nil, authHdr)
	if wDeact.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after account deactivated, got %d", wDeact.Code)
	}
}

func TestAuthLoginValidation(t *testing.T) {
	r, _, _ := setupTestServer(t)

	cases := []struct {
		name       string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid_email_and_empty_password",
			body:       map[string]string{"email": "bukan-email", "password": ""},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "validation_error",
		},
		{
			name:       "empty_body",
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
		{
			name:       "unknown_json_field",
			body:       map[string]string{"email": "owner@demo.test", "password": "secret", "extra": "1"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_json",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(r, http.MethodPost, "/api/v1/auth/login", tc.body, nil)
			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.wantStatus, w.Code, w.Body.String())
			}
			var errResp apiErrorResp
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if errResp.Error.Code != tc.wantCode {
				t.Errorf("expected code %q, got %q (msg: %s)", tc.wantCode, errResp.Error.Code, errResp.Error.Message)
			}
		})
	}
}

func TestMasterAndSettingsInputValidation(t *testing.T) {
	r, _, pw := setupTestServer(t)

	ownerToken, _ := loginAs(t, r, "owner@demo.test", pw)
	mgrToken, _ := loginAs(t, r, "manager@demo.test", pw)
	adminToken, _ := loginAs(t, r, "admin@demo.test", pw)

	ownerHdr := map[string]string{"Authorization": "Bearer " + ownerToken}
	mgrHdr := map[string]string{"Authorization": "Bearer " + mgrToken}
	adminHdr := map[string]string{"Authorization": "Bearer " + adminToken}

	cases := []struct {
		name   string
		method string
		path   string
		hdr    map[string]string
		body   any
	}{
		{
			name:   "settings_unknown_key_and_out_of_range",
			method: http.MethodPut,
			path:   "/api/v1/settings",
			hdr:    ownerHdr,
			body:   map[string]any{"capacity_limit": 1.5, "unknown_key": 123},
		},
		{
			name:   "product_negative_price_and_empty_name",
			method: http.MethodPost,
			path:   "/api/v1/products",
			hdr:    mgrHdr,
			body:   map[string]any{"name": "", "price": -5000},
		},
		{
			name:   "ingredient_invalid_rounding_and_zero_size",
			method: http.MethodPost,
			path:   "/api/v1/ingredients",
			hdr:    mgrHdr,
			body:   map[string]any{"name": "Tepung", "use_unit": "kg", "buy_unit": "sak", "buy_unit_size": 0, "buy_price": 10000, "rounding": "invalid"},
		},
		{
			name:   "resource_invalid_code_and_efficiency",
			method: http.MethodPost,
			path:   "/api/v1/resources",
			hdr:    mgrHdr,
			body:   map[string]any{"code": "mixer", "name": "Mixer", "unit": "hour", "count": 0, "amount_per_unit_day": 8, "efficiency": 1.2},
		},
		{
			name:   "user_short_password_and_invalid_role",
			method: http.MethodPost,
			path:   "/api/v1/users",
			hdr:    adminHdr,
			body:   map[string]any{"email": "bad-email", "name": "Uji", "role": "superadmin", "password": "123"},
		},
		{
			name:   "admin_cannot_deactivate_self",
			method: http.MethodPut,
			path:   "/api/v1/users/44444444-4444-4444-4444-444444444444",
			hdr:    adminHdr,
			body:   map[string]any{"name": "Admin", "role": "admin", "is_active": false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(r, tc.method, tc.path, tc.body, tc.hdr)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got %d: %s", w.Code, w.Body.String())
			}
			var errResp apiErrorResp
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("unmarshal 422 error: %v", err)
			}
			if errResp.Error.Code != "validation_error" || errResp.Error.Message == "" {
				t.Errorf("unexpected validation error: %+v", errResp.Error)
			}
		})
	}
}

