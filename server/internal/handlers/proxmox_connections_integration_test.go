package handlers_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	proxmoxsvc "github.com/serversupervisor/server/internal/services/proxmox"
	"github.com/serversupervisor/server/internal/testutil"
)

const proxmoxSecret = "p2x-secret-DO-NOT-LEAK-123"

func newProxmoxRouter(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewProxmoxHandler(proxmoxsvc.NewService(db, cfg, nil))

	r := gin.New()
	r.GET("/proxmox/instances", h.ListConnections)
	r.POST("/proxmox/instances", h.CreateConnection)
	r.GET("/proxmox/instances/:id", h.GetConnection)
	r.PUT("/proxmox/instances/:id", h.UpdateConnection)
	r.DELETE("/proxmox/instances/:id", h.DeleteConnection)
	return r, db
}

func validProxmoxPayload() map[string]any {
	return map[string]any{
		"name":         "pve-1",
		"api_url":      "https://pve.example.com:8006",
		"token_id":     "root@pam!monitoring",
		"token_secret": proxmoxSecret,
		"enabled":      true,
	}
}

// assertNoSecret fails if the response body leaks the token secret in any form.
func assertNoSecret(t *testing.T, label, body string) {
	t.Helper()
	if strings.Contains(body, proxmoxSecret) {
		t.Errorf("%s response leaks the token secret value: %s", label, body)
	}
	if strings.Contains(body, "token_secret") {
		t.Errorf("%s response exposes a token_secret field: %s", label, body)
	}
}

func TestProxmoxConnectionsCRUD(t *testing.T) {
	r, _ := newProxmoxRouter(t)

	// Create
	w := doJSON(t, r, http.MethodPost, "/proxmox/instances", validProxmoxPayload())
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	assertNoSecret(t, "create", w.Body.String())
	var created idOnly
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("decode created id: %v (%s)", err, w.Body.String())
	}
	idPath := "/proxmox/instances/" + created.ID

	// List — must not leak the secret
	wl := doJSON(t, r, http.MethodGet, "/proxmox/instances", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	assertNoSecret(t, "list", wl.Body.String())

	// Get — must not leak the secret
	g := doJSON(t, r, http.MethodGet, idPath, nil)
	if g.Code != http.StatusOK {
		t.Fatalf("get status = %d", g.Code)
	}
	assertNoSecret(t, "get", g.Body.String())

	// Update (rename, empty token_secret keeps the existing one)
	upd := validProxmoxPayload()
	upd["name"] = "pve-renamed"
	upd["token_secret"] = ""
	u := doJSON(t, r, http.MethodPut, idPath, upd)
	if u.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", u.Code, u.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(u.Body.Bytes(), &updated)
	if updated["name"] != "pve-renamed" {
		t.Errorf("name = %v, want pve-renamed", updated["name"])
	}

	// Delete then 404
	if d := doJSON(t, r, http.MethodDelete, idPath, nil); d.Code != http.StatusOK {
		t.Fatalf("delete status = %d", d.Code)
	}
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}

func TestProxmoxConnectionCreateValidation(t *testing.T) {
	r, _ := newProxmoxRouter(t)

	cases := []struct {
		name   string
		mutate func(p map[string]any)
	}{
		{"missing name", func(p map[string]any) { delete(p, "name") }},
		{"missing api_url", func(p map[string]any) { delete(p, "api_url") }},
		{"missing token_id", func(p map[string]any) { delete(p, "token_id") }},
		{"missing token_secret", func(p map[string]any) { delete(p, "token_secret") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validProxmoxPayload()
			tc.mutate(p)
			if w := doJSON(t, r, http.MethodPost, "/proxmox/instances", p); w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestProxmoxConnectionNotFound(t *testing.T) {
	r, _ := newProxmoxRouter(t)
	missing := "/proxmox/instances/00000000-0000-0000-0000-000000000000"
	if w := doJSON(t, r, http.MethodGet, missing, nil); w.Code != http.StatusNotFound {
		t.Errorf("get missing = %d, want 404", w.Code)
	}
	if w := doJSON(t, r, http.MethodDelete, missing, nil); w.Code != http.StatusNotFound {
		t.Errorf("delete missing = %d, want 404", w.Code)
	}
}
