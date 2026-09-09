package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	configsvc "github.com/serversupervisor/server/internal/services/config"
)

type mockConfigRepo struct {
	settings  map[string]string
	auditLogs []string
}

func (m *mockConfigRepo) GetAllSettings(_ context.Context) (map[string]string, error) {
	res := make(map[string]string, len(m.settings))
	for k, v := range m.settings {
		res[k] = v
	}
	return res, nil
}

func (m *mockConfigRepo) SetSetting(_ context.Context, key, value string) error {
	m.settings[key] = value
	return nil
}

func (m *mockConfigRepo) DeleteSetting(_ context.Context, key string) error {
	delete(m.settings, key)
	return nil
}

func (m *mockConfigRepo) CreateAuditLog(_ context.Context, username, action, _, _, details, _ string) (int64, error) {
	m.auditLogs = append(m.auditLogs, username+":"+action+":"+details)
	return int64(len(m.auditLogs)), nil
}

func setupConfigTestRouter(role string) (*gin.Engine, *ConfigHandler, *mockConfigRepo) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	repo := &mockConfigRepo{
		settings:  make(map[string]string),
		auditLogs: make([]string, 0),
	}
	cfg := config.Load()
	svc := configsvc.NewService(repo, cfg)
	h := NewConfigHandler(svc)

	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("role", role)
			c.Set("username", "testadmin")
		}
		c.Next()
	})

	r.GET("/api/v1/config", h.GetConfig)
	r.PUT("/api/v1/config/:key", h.UpdateConfigKey)
	r.DELETE("/api/v1/config/:key", h.ResetConfigKey)
	r.PUT("/api/v1/config", h.UpdateConfigBulk)

	return r, h, repo
}

func TestConfigHandler_Permissions(t *testing.T) {
	// Viewer role should be forbidden
	r, _, _ := setupConfigTestRouter("viewer")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for viewer, got %d", w.Code)
	}
}

func TestConfigHandler_GetConfig(t *testing.T) {
	r, _, repo := setupConfigTestRouter("admin")
	repo.settings["smtp_host"] = "mail.mycorp.internal"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var summary config.ConfigSummary
	if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if summary.TotalParams == 0 {
		t.Errorf("expected params > 0")
	}
}

func TestConfigHandler_UpdateConfigKey(t *testing.T) {
	r, _, repo := setupConfigTestRouter("admin")

	body, _ := json.Marshal(map[string]string{"value": "https://new-supervisor.local"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/BASE_URL", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", w.Code, w.Body.String())
	}

	if repo.settings["base_url"] != "https://new-supervisor.local" {
		t.Errorf("expected setting to be saved in repo")
	}

	// Invalid value test
	bodyInvalid, _ := json.Marshal(map[string]string{"value": "not-a-valid-port"})
	reqInvalid := httptest.NewRequest(http.MethodPut, "/api/v1/config/SERVER_PORT", bytes.NewReader(bodyInvalid))
	reqInvalid.Header.Set("Content-Type", "application/json")
	wInvalid := httptest.NewRecorder()
	r.ServeHTTP(wInvalid, reqInvalid)

	if wInvalid.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid port, got %d", wInvalid.Code)
	}
}

func TestConfigHandler_ResetConfigKey(t *testing.T) {
	r, _, repo := setupConfigTestRouter("admin")
	repo.settings["smtp_host"] = "mail.temp.org"

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/config/SMTP_HOST", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if _, exists := repo.settings["smtp_host"]; exists {
		t.Errorf("expected setting to be removed from repo")
	}
}

func TestConfigHandler_UpdateConfigBulk(t *testing.T) {
	r, _, repo := setupConfigTestRouter("admin")

	bulk := map[string]any{
		"settings": map[string]string{
			"SMTP_HOST": "mail.bulk.local",
			"SMTP_PORT": "2525",
		},
	}
	body, _ := json.Marshal(bulk)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if repo.settings["smtp_host"] != "mail.bulk.local" {
		t.Errorf("expected smtp_host in repo")
	}
	if repo.settings["smtp_port"] != "2525" {
		t.Errorf("expected smtp_port in repo")
	}

	// Bad json bulk
	reqBad := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader([]byte("{invalid-json")))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON, got %d", wBad.Code)
	}

	// Bad json single update
	reqBadSingle := httptest.NewRequest(http.MethodPut, "/api/v1/config/SERVER_PORT", bytes.NewReader([]byte("{invalid-json")))
	reqBadSingle.Header.Set("Content-Type", "application/json")
	wBadSingle := httptest.NewRecorder()
	r.ServeHTTP(wBadSingle, reqBadSingle)
	if wBadSingle.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request on malformed JSON, got %d", wBadSingle.Code)
	}

	// Reset unknown param
	reqResetUnknown := httptest.NewRequest(http.MethodDelete, "/api/v1/config/NONEXISTENT_XYZ", nil)
	wResetUnknown := httptest.NewRecorder()
	r.ServeHTTP(wResetUnknown, reqResetUnknown)
	if wResetUnknown.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown param in reset, got %d", wResetUnknown.Code)
	}

	// GetConfig with reveal_secrets
	reqReveal := httptest.NewRequest(http.MethodGet, "/api/v1/config?reveal_secrets=true", nil)
	wReveal := httptest.NewRecorder()
	r.ServeHTTP(wReveal, reqReveal)
	if wReveal.Code != http.StatusOK {
		t.Errorf("expected 200 with reveal_secrets, got %d", wReveal.Code)
	}
}
