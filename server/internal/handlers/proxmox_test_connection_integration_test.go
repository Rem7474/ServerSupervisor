package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

// fakePVEHandlerServer answers the two REST calls TestConnection can make:
// the plain reachability check (GET /nodes) and, when console credentials
// are supplied, the PVE ticket login used to validate them.
func fakePVEHandlerServer(loginOK bool) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	mux.HandleFunc("/access/ticket", func(w http.ResponseWriter, r *http.Request) {
		if !loginOK {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"data":null}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"ticket":"PVE:root@pam:X=="}}`))
	})
	return httptest.NewServer(mux)
}

// TestProxmoxTestConnection covers the ad-hoc credentials test endpoint,
// including the console-credentials envelope (models.ProxmoxTestResult)
// introduced alongside the interactive LXC console feature.
func TestProxmoxTestConnection(t *testing.T) {
	r, _ := newProxmoxRouter(t)
	srv := fakePVEHandlerServer(true)
	defer srv.Close()

	w := doJSON(t, r, http.MethodPost, "/proxmox/instances/test", map[string]any{
		"api_url":      srv.URL,
		"token_id":     "user@pve!token",
		"token_secret": "secret",
		"pve_username": "root@pam",
		"pve_password": "hunter2",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var result models.ProxmoxTestResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result.Success || !result.ConsoleConfigured || !result.ConsoleOK {
		t.Errorf("expected a fully successful test, got %+v", result)
	}
}

// TestProxmoxTestConnection_ValidationError covers the ShouldBindJSON
// failure path (missing required fields).
func TestProxmoxTestConnection_ValidationError(t *testing.T) {
	r, _ := newProxmoxRouter(t)
	w := doJSON(t, r, http.MethodPost, "/proxmox/instances/test", map[string]any{"api_url": "https://pve.invalid"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

// TestProxmoxTestConnectionByID covers testing a stored connection using its
// persisted token secret and console credentials.
func TestProxmoxTestConnectionByID(t *testing.T) {
	r, _ := newProxmoxRouter(t)
	srv := fakePVEHandlerServer(true)
	defer srv.Close()

	created := doJSON(t, r, http.MethodPost, "/proxmox/instances", map[string]any{
		"name":         "pve-test-by-id",
		"api_url":      srv.URL,
		"token_id":     "user@pve!token",
		"token_secret": proxmoxSecret,
		"enabled":      true,
		"pve_username": "root@pam",
		"pve_password": "hunter2",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var idOut idOnly
	if err := json.Unmarshal(created.Body.Bytes(), &idOut); err != nil {
		t.Fatalf("decode created id: %v", err)
	}

	w := doJSON(t, r, http.MethodPost, "/proxmox/instances/"+idOut.ID+"/test", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var result models.ProxmoxTestResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !result.Success || !result.ConsoleConfigured || !result.ConsoleOK {
		t.Errorf("expected a fully successful test, got %+v", result)
	}
}

// TestProxmoxTestConnectionByID_NotFound covers the apperr.NotFound branch.
func TestProxmoxTestConnectionByID_NotFound(t *testing.T) {
	r, _ := newProxmoxRouter(t)
	w := doJSON(t, r, http.MethodPost, "/proxmox/instances/00000000-0000-0000-0000-000000000000/test", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", w.Code, w.Body.String())
	}
}
