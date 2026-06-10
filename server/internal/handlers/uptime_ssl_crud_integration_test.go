package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	uptimesvc "github.com/serversupervisor/server/internal/services/uptime"
	"github.com/serversupervisor/server/internal/testutil"
)

// idOnly decodes the "id" field of a create response (probe/cert IDs are strings).
type idOnly struct {
	ID string `json:"id"`
}

func newUptimeRouter(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewUptimeHandler(uptimesvc.NewService(db))
	r := gin.New()
	r.GET("/uptime/probes", h.List)
	r.POST("/uptime/probes", h.Create)
	r.GET("/uptime/probes/:id", h.Get)
	r.PUT("/uptime/probes/:id", h.Update)
	r.DELETE("/uptime/probes/:id", h.Delete)
	return r, db
}

func TestUptimeProbesCRUD(t *testing.T) {
	r, _ := newUptimeRouter(t)

	w := doJSON(t, r, http.MethodPost, "/uptime/probes", map[string]any{
		"type": "http", "name": "site", "target": "https://example.com",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var created idOnly
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("decode created id: %v (%s)", err, w.Body.String())
	}
	idPath := "/uptime/probes/" + created.ID

	// List wraps results under "probes".
	wl := doJSON(t, r, http.MethodGet, "/uptime/probes", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	var list struct {
		Probes []map[string]any `json:"probes"`
	}
	_ = json.Unmarshal(wl.Body.Bytes(), &list)
	if len(list.Probes) != 1 {
		t.Fatalf("expected 1 probe, got %d", len(list.Probes))
	}

	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusOK {
		t.Fatalf("get status = %d", g.Code)
	}

	u := doJSON(t, r, http.MethodPut, idPath, map[string]any{
		"type": "http", "name": "site-renamed", "target": "https://example.com",
	})
	if u.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", u.Code, u.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(u.Body.Bytes(), &updated)
	if updated["name"] != "site-renamed" {
		t.Errorf("name = %v, want site-renamed", updated["name"])
	}

	if d := doJSON(t, r, http.MethodDelete, idPath, nil); d.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", d.Code)
	}
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}

func TestUptimeProbeCreateValidation(t *testing.T) {
	r, _ := newUptimeRouter(t)
	// Missing target -> 400
	if w := doJSON(t, r, http.MethodPost, "/uptime/probes", map[string]any{"type": "http", "name": "x"}); w.Code != http.StatusBadRequest {
		t.Errorf("missing target = %d, want 400", w.Code)
	}
	// Invalid type (not http/tcp) -> 400
	if w := doJSON(t, r, http.MethodPost, "/uptime/probes", map[string]any{"type": "ftp", "name": "x", "target": "y"}); w.Code != http.StatusBadRequest {
		t.Errorf("invalid type = %d, want 400", w.Code)
	}
}

func newSSLRouter(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewSSLHandler(db, cfg)
	r := gin.New()
	r.GET("/ssl/certificates", h.List)
	r.POST("/ssl/certificates", h.Create)
	r.GET("/ssl/certificates/:id", h.Get)
	r.PUT("/ssl/certificates/:id", h.Update)
	r.DELETE("/ssl/certificates/:id", h.Delete)
	return r, db
}

func TestSSLCertificatesCRUD(t *testing.T) {
	r, _ := newSSLRouter(t)

	w := doJSON(t, r, http.MethodPost, "/ssl/certificates", map[string]any{
		"name": "site", "host": "example.com",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var created idOnly
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("decode created id: %v (%s)", err, w.Body.String())
	}
	idPath := "/ssl/certificates/" + created.ID

	wl := doJSON(t, r, http.MethodGet, "/ssl/certificates", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	var list struct {
		Certificates []map[string]any `json:"certificates"`
	}
	_ = json.Unmarshal(wl.Body.Bytes(), &list)
	if len(list.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(list.Certificates))
	}

	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusOK {
		t.Fatalf("get status = %d", g.Code)
	}

	u := doJSON(t, r, http.MethodPut, idPath, map[string]any{"name": "site-renamed", "host": "example.com"})
	if u.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", u.Code, u.Body.String())
	}

	if d := doJSON(t, r, http.MethodDelete, idPath, nil); d.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", d.Code)
	}
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}

func TestSSLCertificateCreateValidation(t *testing.T) {
	r, _ := newSSLRouter(t)
	// Missing host -> 400
	if w := doJSON(t, r, http.MethodPost, "/ssl/certificates", map[string]any{"name": "x"}); w.Code != http.StatusBadRequest {
		t.Errorf("missing host = %d, want 400", w.Code)
	}
}
