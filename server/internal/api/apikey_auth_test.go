package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
	apikeysvc "github.com/serversupervisor/server/internal/services/apikey"
)

const (
	readKey    = "ssk_readread_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	writeKey   = "ssk_writewri_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	viewerKey  = "ssk_viewervi_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	expiredKey = "ssk_expiredx_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
)

// fakeKeys stands in for apikey.Service.Authenticate.
type fakeKeys struct{ calls int }

func (f *fakeKeys) Authenticate(_ context.Context, raw string) (*apikeysvc.Principal, error) {
	f.calls++
	switch raw {
	case readKey:
		return &apikeysvc.Principal{KeyID: 1, Username: "alice", Role: models.RoleAdmin, Scope: models.APIKeyScopeRead}, nil
	case writeKey:
		return &apikeysvc.Principal{KeyID: 2, Username: "alice", Role: models.RoleAdmin, Scope: models.APIKeyScopeWrite}, nil
	case viewerKey:
		return &apikeysvc.Principal{KeyID: 3, Username: "bob", Role: models.RoleViewer, Scope: models.APIKeyScopeWrite}, nil
	case expiredKey:
		return nil, apperr.Unauthorized("API key expired").I18n(apperr.CodeAPIKeyExpired, nil)
	}
	return nil, apperr.Unauthorized("invalid API key").I18n(apperr.CodeAPIKeyInvalid, nil)
}

func newKeyRouter(keys APIKeyAuthenticator, rl *IPRateLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(SessionOrAPIKeyMiddleware(&config.Config{JWTSecret: "test-secret"}, keys, rl))
	echo := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"username":    c.GetString("username"),
			"role":        c.GetString("role"),
			"auth_method": c.GetString("auth_method"),
		})
	}
	v1.GET("/hosts", echo)
	v1.POST("/hosts", echo)
	v1.GET("/hosts/:id", echo)
	v1.POST("/hosts/:id/maintenance-windows", echo)
	v1.POST("/hosts/:id/rotate-key", echo)
	v1.GET("/auth/api-keys", echo)
	v1.GET("/auth/profile", echo)
	v1.GET("/users", echo)
	v1.PUT("/config/:key", echo)
	v1.GET("/settings", echo)
	v1.GET("/proxmox/instances", echo)
	v1.GET("/webhooks/git/:id", echo)
	admin := v1.Group("")
	admin.Use(AdminOnlyMiddleware())
	admin.GET("/api-keys", echo)
	admin.POST("/alerts/incidents/:id/ack", echo)
	return r
}

type keyResp struct {
	code int
	body map[string]any
}

func call(t *testing.T, r http.Handler, method, path, auth string) keyResp {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return keyResp{w.Code, body}
}

func TestSessionOrAPIKey_KeyActsAsItsOwner(t *testing.T) {
	r := newKeyRouter(&fakeKeys{}, nil)
	got := call(t, r, http.MethodGet, "/api/v1/hosts/h1", "Bearer "+readKey)
	if got.code != http.StatusOK {
		t.Fatalf("status = %d, body = %v", got.code, got.body)
	}
	if got.body["username"] != "alice" || got.body["role"] != models.RoleAdmin || got.body["auth_method"] != "api_key" {
		t.Fatalf("context not populated from the key: %v", got.body)
	}
}

func TestSessionOrAPIKey_BearerSchemeIsCaseInsensitive(t *testing.T) {
	r := newKeyRouter(&fakeKeys{}, nil)
	if got := call(t, r, http.MethodGet, "/api/v1/hosts", "bearer "+readKey); got.code != http.StatusOK {
		t.Fatalf("status = %d", got.code)
	}
}

func TestSessionOrAPIKey_RejectedKeys(t *testing.T) {
	r := newKeyRouter(&fakeKeys{}, nil)
	for name, tc := range map[string]struct {
		key, i18n string
	}{
		"expired": {expiredKey, apperr.CodeAPIKeyExpired},
		"unknown": {"ssk_unknownx_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", apperr.CodeAPIKeyInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			got := call(t, r, http.MethodGet, "/api/v1/hosts", "Bearer "+tc.key)
			if got.code != http.StatusUnauthorized || got.body["i18nKey"] != tc.i18n {
				t.Fatalf("got %d %v, want 401 %s", got.code, got.body, tc.i18n)
			}
		})
	}
}

func TestSessionOrAPIKey_ReadScopeOnlyReachesGET(t *testing.T) {
	r := newKeyRouter(&fakeKeys{}, nil)
	got := call(t, r, http.MethodPost, "/api/v1/hosts/h1/maintenance-windows", "Bearer "+readKey)
	if got.code != http.StatusForbidden || got.body["i18nKey"] != apperr.CodeAPIKeyReadOnly {
		t.Fatalf("read key on POST: got %d %v", got.code, got.body)
	}
	if got := call(t, r, http.MethodPost, "/api/v1/hosts/h1/maintenance-windows", "Bearer "+writeKey); got.code != http.StatusOK {
		t.Fatalf("write key on POST: got %d %v", got.code, got.body)
	}
}

func TestSessionOrAPIKey_NeverExceedsOwnerRole(t *testing.T) {
	r := newKeyRouter(&fakeKeys{}, nil)
	// A write key owned by a viewer still hits the admin-only gate.
	if got := call(t, r, http.MethodPost, "/api/v1/alerts/incidents/1/ack", "Bearer "+viewerKey); got.code != http.StatusForbidden {
		t.Fatalf("viewer key on admin route: got %d", got.code)
	}
	if got := call(t, r, http.MethodPost, "/api/v1/alerts/incidents/1/ack", "Bearer "+writeKey); got.code != http.StatusOK {
		t.Fatalf("admin write key on admin route: got %d", got.code)
	}
}

func TestSessionOrAPIKey_SensitiveRoutesClosedToKeys(t *testing.T) {
	r := newKeyRouter(&fakeKeys{}, nil)
	for _, rt := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/auth/api-keys"},
		{http.MethodGet, "/api/v1/auth/profile"},
		{http.MethodGet, "/api/v1/api-keys"},
		{http.MethodGet, "/api/v1/users"},
		{http.MethodPut, "/api/v1/config/RATE_LIMIT_RPS"},
		{http.MethodGet, "/api/v1/settings"},
		{http.MethodGet, "/api/v1/proxmox/instances"},
		{http.MethodGet, "/api/v1/webhooks/git/1"},
		{http.MethodPost, "/api/v1/hosts"},
		{http.MethodPost, "/api/v1/hosts/h1/rotate-key"},
	} {
		got := call(t, r, rt.method, rt.path, "Bearer "+writeKey)
		if got.code != http.StatusForbidden || got.body["i18nKey"] != apperr.CodeAPIKeyRouteForbidden {
			t.Errorf("%s %s: got %d %v, want 403 %s", rt.method, rt.path, got.code, got.body, apperr.CodeAPIKeyRouteForbidden)
		}
	}
	// The host list itself stays readable.
	if got := call(t, r, http.MethodGet, "/api/v1/hosts", "Bearer "+writeKey); got.code != http.StatusOK {
		t.Fatalf("GET /hosts: got %d", got.code)
	}
}

func TestSessionOrAPIKey_PerKeyRateLimit(t *testing.T) {
	rl := NewIPRateLimiter(1, 1, nil)
	t.Cleanup(rl.Stop)
	r := newKeyRouter(&fakeKeys{}, rl)
	if got := call(t, r, http.MethodGet, "/api/v1/hosts", "Bearer "+readKey); got.code != http.StatusOK {
		t.Fatalf("first request: %d", got.code)
	}
	got := call(t, r, http.MethodGet, "/api/v1/hosts", "Bearer "+readKey)
	if got.code != http.StatusTooManyRequests || got.body["i18nKey"] != apperr.CodeAPIKeyRateLimited {
		t.Fatalf("second request: got %d %v", got.code, got.body)
	}
	// Another key has its own bucket.
	if got := call(t, r, http.MethodGet, "/api/v1/hosts", "Bearer "+writeKey); got.code != http.StatusOK {
		t.Fatalf("other key: %d", got.code)
	}
}

func TestSessionOrAPIKey_NonKeyRequestsUseJWT(t *testing.T) {
	keys := &fakeKeys{}
	r := newKeyRouter(keys, nil)
	if got := call(t, r, http.MethodGet, "/api/v1/hosts", ""); got.code != http.StatusUnauthorized {
		t.Fatalf("no credentials: got %d", got.code)
	}
	if got := call(t, r, http.MethodGet, "/api/v1/hosts", "Bearer not.a.jwt"); got.code != http.StatusUnauthorized {
		t.Fatalf("bad JWT: got %d", got.code)
	}
	if got := call(t, r, http.MethodGet, "/api/v1/hosts", "Basic "+readKey); got.code != http.StatusUnauthorized {
		t.Fatalf("non-bearer scheme: got %d", got.code)
	}
	if keys.calls != 0 {
		t.Fatalf("the API-key path must not run for non-ssk credentials (%d calls)", keys.calls)
	}
}

func TestAPIKeyForbiddenRoute(t *testing.T) {
	cases := []struct {
		method, path string
		want         bool
	}{
		{http.MethodGet, "/api/v1/hosts", false},
		{http.MethodHead, "/api/v1/hosts", false},
		{http.MethodPost, "/api/v1/hosts", true},
		{http.MethodPost, "/api/v1/hosts/bulk", true},
		{http.MethodGet, "/api/v1/hosts/:id/permissions", true},
		{http.MethodGet, "/api/v1/auth/host-permissions", true},
		{http.MethodGet, "/api/v1/alerts/incidents", false},
		{http.MethodGet, "/api/v1/npm/proxy-hosts", false},
		{http.MethodGet, "/api/v1/npm/connections/:id/proxy-hosts", true},
		{http.MethodPost, "/api/v1/push/subscribe", true},
		{http.MethodGet, "/api/v1/users", true},
	}
	for _, tc := range cases {
		if got := apiKeyForbiddenRoute(tc.method, tc.path); got != tc.want {
			t.Errorf("apiKeyForbiddenRoute(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
