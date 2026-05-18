package cookies

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestReadAccessToken_PrefersCookieOverHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: AccessTokenName, Value: "from-cookie"})
	r.Header.Set("Authorization", "Bearer from-header")

	if got := ReadAccessToken(r); got != "from-cookie" {
		t.Fatalf("expected cookie to win, got %q", got)
	}
}

func TestReadAccessToken_FallsBackToBearer(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer abc.def.ghi")
	if got := ReadAccessToken(r); got != "abc.def.ghi" {
		t.Fatalf("expected bearer token to be returned, got %q", got)
	}
}

func TestReadAccessToken_IgnoresMalformedAuthorization(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Token abc")
	if got := ReadAccessToken(r); got != "" {
		t.Fatalf("expected empty for non-Bearer schemes, got %q", got)
	}
}

func TestReadAccessToken_EmptyWhenNothingPresent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := ReadAccessToken(r); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestSetAccess_WritesHttpOnlyJWTAndReadableCSRFCookies(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	cfg := &config.Config{BaseURL: "https://example.test", TLSEnabled: true}

	exp := time.Now().Add(time.Hour)
	SetAccess(c, cfg, "jwt-value", exp, "csrf-value")

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 Set-Cookie headers, got %d", len(cookies))
	}

	byName := map[string]*http.Cookie{}
	for _, ck := range cookies {
		byName[ck.Name] = ck
	}
	jwt, ok := byName[AccessTokenName]
	if !ok {
		t.Fatalf("missing %s cookie", AccessTokenName)
	}
	if !jwt.HttpOnly {
		t.Fatal("access token cookie must be HttpOnly")
	}
	if !jwt.Secure {
		t.Fatal("access token cookie must be Secure when TLS is enabled")
	}
	if jwt.SameSite != http.SameSiteLaxMode {
		t.Fatalf("access cookie SameSite=%v want Lax", jwt.SameSite)
	}
	if jwt.Value != "jwt-value" {
		t.Fatalf("unexpected JWT value %q", jwt.Value)
	}

	csrf, ok := byName[CSRFTokenName]
	if !ok {
		t.Fatalf("missing %s cookie", CSRFTokenName)
	}
	if csrf.HttpOnly {
		t.Fatal("CSRF cookie must be readable from JS — HttpOnly must be false")
	}
	if csrf.Value != "csrf-value" {
		t.Fatalf("unexpected CSRF value %q", csrf.Value)
	}
}

func TestClear_ExpiresAllAuthCookies(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	cfg := &config.Config{BaseURL: "http://localhost"}

	Clear(c, cfg)
	cookies := w.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected 3 Set-Cookie headers, got %d", len(cookies))
	}
	for _, ck := range cookies {
		if ck.MaxAge >= 0 {
			t.Fatalf("cookie %s should be expired (MaxAge<0), got %d", ck.Name, ck.MaxAge)
		}
	}
}

func TestCSRFMiddleware_SkipsGETandHEAD(t *testing.T) {
	mw := CSRFMiddleware()

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/api/v1/things", nil)
		c.Request.AddCookie(&http.Cookie{Name: AccessTokenName, Value: "x"})
		c.Request.AddCookie(&http.Cookie{Name: CSRFTokenName, Value: "y"})

		mw(c)
		if c.IsAborted() {
			t.Fatalf("%s should not trigger CSRF check", method)
		}
	}
}

func TestCSRFMiddleware_SkipsBearerClients(t *testing.T) {
	mw := CSRFMiddleware()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/things", nil)
	// No session cookie — pure Bearer client (curl, script). CSRF must not block.
	c.Request.Header.Set("Authorization", "Bearer abc")

	mw(c)
	if c.IsAborted() {
		t.Fatal("CSRF middleware must skip Bearer-only requests")
	}
}

func TestCSRFMiddleware_BlocksMissingHeader(t *testing.T) {
	mw := CSRFMiddleware()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/things", nil)
	c.Request.AddCookie(&http.Cookie{Name: AccessTokenName, Value: "x"})
	c.Request.AddCookie(&http.Cookie{Name: CSRFTokenName, Value: "csrf"})

	mw(c)
	if !c.IsAborted() {
		t.Fatal("missing X-CSRF-Token header must abort")
	}
	if w.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Result().StatusCode)
	}
}

func TestCSRFMiddleware_BlocksMismatchedTokens(t *testing.T) {
	mw := CSRFMiddleware()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/things", nil)
	c.Request.AddCookie(&http.Cookie{Name: AccessTokenName, Value: "x"})
	c.Request.AddCookie(&http.Cookie{Name: CSRFTokenName, Value: "cookie-token"})
	c.Request.Header.Set(CSRFHeaderName, "header-token")

	mw(c)
	if !c.IsAborted() {
		t.Fatal("mismatched CSRF cookie/header must abort")
	}
}

func TestCSRFMiddleware_AcceptsMatchingTokens(t *testing.T) {
	mw := CSRFMiddleware()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/things", nil)
	c.Request.AddCookie(&http.Cookie{Name: AccessTokenName, Value: "x"})
	c.Request.AddCookie(&http.Cookie{Name: CSRFTokenName, Value: "same"})
	c.Request.Header.Set(CSRFHeaderName, "same")

	mw(c)
	if c.IsAborted() {
		t.Fatal("matching tokens must let the request through")
	}
}

func TestGenerateCSRFToken_RandomAndLongEnough(t *testing.T) {
	a, err := GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken failed: %v", err)
	}
	b, err := GenerateCSRFToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two consecutive CSRF tokens must differ")
	}
	if len(a) < 32 {
		t.Fatalf("CSRF token too short: %d", len(a))
	}
}

func TestSecure_PicksUpHTTPSBaseURL(t *testing.T) {
	if secure(&config.Config{BaseURL: "https://x.test"}) != true {
		t.Fatal("https BaseURL must set Secure=true")
	}
	if secure(&config.Config{BaseURL: "http://x.test"}) != false {
		t.Fatal("http BaseURL must set Secure=false")
	}
	if secure(&config.Config{TLSEnabled: true, BaseURL: "http://x.test"}) != true {
		t.Fatal("TLSEnabled must force Secure=true")
	}
}

func TestRefreshPath_DoesNotLeakOnAPICalls(t *testing.T) {
	// Sanity check that the refresh cookie is not accidentally sent on every
	// API call — the path is /api/auth, not /.
	if !strings.HasPrefix(RefreshPath(), "/api/auth") {
		t.Fatalf("refresh cookie path should be scoped to /api/auth, got %q", RefreshPath())
	}
}
