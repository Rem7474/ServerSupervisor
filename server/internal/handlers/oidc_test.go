package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
	oidcsvc "github.com/serversupervisor/server/internal/services/oidc"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestOIDCHandler_GetStatus(t *testing.T) {
	cfg := &config.Config{
		OIDCEnabled:         true,
		OIDCDisplayName:     "Keycloak SSO",
		OIDCAllowLocalLogin: true,
	}
	svc := oidcsvc.NewService(nil, nil, cfg)
	handler := NewOIDCHandler(svc, cfg)

	r := gin.New()
	r.GET("/api/auth/oidc/status", handler.GetStatus)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res models.OIDCStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !res.Enabled || res.DisplayName != "Keycloak SSO" || !res.AllowLocalLogin {
		t.Fatalf("unexpected status payload: %+v", res)
	}
}

func TestOIDCHandler_Callback_IdPError(t *testing.T) {
	cfg := &config.Config{}
	svc := oidcsvc.NewService(nil, nil, cfg)
	handler := NewOIDCHandler(svc, cfg)

	r := gin.New()
	r.GET("/api/auth/oidc/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback?error=access_denied&error_description=User+cancelled", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc == "" || loc != "/login?error=sso_error%3A+User+cancelled" {
		t.Fatalf("unexpected redirect location: %s", loc)
	}
}

func TestOIDCHandler_Callback_MissingParams(t *testing.T) {
	cfg := &config.Config{}
	svc := oidcsvc.NewService(nil, nil, cfg)
	handler := NewOIDCHandler(svc, cfg)

	r := gin.New()
	r.GET("/api/auth/oidc/callback", handler.Callback)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/callback", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected status 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc == "" || loc != "/login?error=missing+authorization+code+or+state" {
		t.Fatalf("unexpected redirect location: %s", loc)
	}
}
