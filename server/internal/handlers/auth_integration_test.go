package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/testutil"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// seedUser creates a user row directly via the DB so we don't need the
// CreateUser admin endpoint just to set up an auth test.
func seedUser(t *testing.T, db interface {
	CreateUser(ctx context.Context, username, passwordHash, role string, mustChangePassword ...bool) error
}, username, password, role string) {
	t.Helper()
	hash, err := handlers.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.CreateUser(context.Background(), username, hash, role); err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func newAuthRouter(t *testing.T) (*gin.Engine, *handlers.AuthHandler) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewAuthHandler(db, cfg, nil)

	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/refresh", h.RefreshToken)
	r.POST("/api/auth/logout", h.Logout)

	seedUser(t, db, "alice", "correct-horse-battery-staple", "admin")
	return r, h
}

func doJSON(t *testing.T, r http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func cookieByName(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestAuthLogin_SetsHttpOnlyCookiesAndCSRFToken(t *testing.T) {
	r, _ := newAuthRouter(t)

	w := doJSON(t, r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice",
		"password": "correct-horse-battery-staple",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login: got %d (%s)", w.Code, w.Body.String())
	}

	resp := w.Result()
	access := cookieByName(resp, cookies.AccessTokenName)
	refresh := cookieByName(resp, cookies.RefreshTokenName)
	csrf := cookieByName(resp, cookies.CSRFTokenName)

	if access == nil || refresh == nil || csrf == nil {
		t.Fatalf("missing cookies: access=%v refresh=%v csrf=%v", access, refresh, csrf)
	}
	if !access.HttpOnly {
		t.Error("access cookie must be HttpOnly")
	}
	if !refresh.HttpOnly {
		t.Error("refresh cookie must be HttpOnly")
	}
	if csrf.HttpOnly {
		t.Error("csrf cookie must NOT be HttpOnly — JS must read it")
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["role"] != "admin" {
		t.Errorf("expected role admin, got %v", body["role"])
	}
	if body["csrf_token"] != csrf.Value {
		t.Errorf("csrf_token in body must match cookie")
	}
	if _, hasToken := body["token"]; hasToken {
		t.Error("response body must NOT contain a JWT token field (cookies only)")
	}
}

func TestAuthLogin_RejectsBadPassword(t *testing.T) {
	r, _ := newAuthRouter(t)

	w := doJSON(t, r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice",
		"password": "wrong",
	})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if len(w.Result().Cookies()) > 0 {
		t.Error("no cookies must be set on failed login")
	}
}

func TestAuthLogin_BruteForceBlocksAfterFiveFailures(t *testing.T) {
	r, _ := newAuthRouter(t)

	for i := 0; i < 5; i++ {
		_ = doJSON(t, r, http.MethodPost, "/api/auth/login", map[string]string{
			"username": "alice",
			"password": "wrong",
		})
	}
	// The 6th call (even with the correct password) must be rate-limited.
	w := doJSON(t, r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice",
		"password": "correct-horse-battery-staple",
	})
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after brute-force, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestAuthRefresh_RotatesTokenAndIssuesNewCookies(t *testing.T) {
	r, _ := newAuthRouter(t)

	// Login first to obtain the refresh cookie.
	loginResp := doJSON(t, r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice",
		"password": "correct-horse-battery-staple",
	}).Result()
	refresh := cookieByName(loginResp, cookies.RefreshTokenName)
	if refresh == nil {
		t.Fatal("login did not return a refresh cookie")
	}

	// Refresh.
	w := doJSON(t, r, http.MethodPost, "/api/auth/refresh", nil, refresh)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: got %d (%s)", w.Code, w.Body.String())
	}
	newRefresh := cookieByName(w.Result(), cookies.RefreshTokenName)
	if newRefresh == nil {
		t.Fatal("refresh did not return a new refresh cookie")
	}
	if newRefresh.Value == refresh.Value {
		t.Fatal("refresh token must be rotated, got identical value")
	}

	// Using the OLD refresh token again must fail (revoked by rotation).
	w2 := doJSON(t, r, http.MethodPost, "/api/auth/refresh", nil, refresh)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("old refresh token must be rejected after rotation, got %d", w2.Code)
	}
}

func TestAuthLogout_ClearsCookiesAndRevokesRefresh(t *testing.T) {
	r, _ := newAuthRouter(t)

	loginResp := doJSON(t, r, http.MethodPost, "/api/auth/login", map[string]string{
		"username": "alice",
		"password": "correct-horse-battery-staple",
	}).Result()
	refresh := cookieByName(loginResp, cookies.RefreshTokenName)
	if refresh == nil {
		t.Fatal("login did not return a refresh cookie")
	}

	w := doJSON(t, r, http.MethodPost, "/api/auth/logout", nil, refresh)
	if w.Code != http.StatusOK {
		t.Fatalf("logout: got %d", w.Code)
	}

	for _, ck := range w.Result().Cookies() {
		if ck.MaxAge >= 0 {
			t.Errorf("logout must expire cookie %s (MaxAge=%d)", ck.Name, ck.MaxAge)
		}
	}

	// Re-attempting refresh with the (now revoked) token must fail.
	w2 := doJSON(t, r, http.MethodPost, "/api/auth/refresh", nil, refresh)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("revoked refresh token must be rejected, got %d", w2.Code)
	}
}

func TestAuthRefresh_MissingCookieReturns401(t *testing.T) {
	r, _ := newAuthRouter(t)

	w := doJSON(t, r, http.MethodPost, "/api/auth/refresh", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without refresh cookie, got %d", w.Code)
	}
}
