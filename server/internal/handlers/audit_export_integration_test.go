package handlers_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	auditsvc "github.com/serversupervisor/server/internal/services/audit"
	"github.com/serversupervisor/server/internal/testutil"
)

func newAuditRouter(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewAuditHandler(auditsvc.NewService(db))

	r := gin.New()
	r.Use(withRole(role))
	r.GET("/audit/logs", h.GetAuditLogs)
	r.GET("/audit/logs/export", h.ExportAuditLogs)
	return r, db
}

func TestGetAuditLogs_CategoryFilter(t *testing.T) {
	r, db := newAuditRouter(t, "admin")
	if _, err := db.CreateAuditLog(context.Background(), "admin", "update_settings", "", "", "", "success"); err != nil {
		t.Fatalf("seed settings log: %v", err)
	}
	if _, err := db.CreateAuditLog(context.Background(), "alert-engine", "alert_fired", "", "", "", "success"); err != nil {
		t.Fatalf("seed alert log: %v", err)
	}

	w := doJSON(t, r, http.MethodGet, "/audit/logs?category=alert", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "alert_fired") {
		t.Errorf("expected alert_fired in filtered response, got %s", body)
	}
	if strings.Contains(body, "update_settings") {
		t.Errorf("did not expect update_settings in category=alert response, got %s", body)
	}
}

func TestGetAuditLogs_RequiresAdmin(t *testing.T) {
	r, _ := newAuditRouter(t, "operator")
	w := doJSON(t, r, http.MethodGet, "/audit/logs", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin = %d, want 403; body = %s", w.Code, w.Body.String())
	}
}

func TestExportAuditLogs_ReturnsCSV(t *testing.T) {
	r, db := newAuditRouter(t, "admin")
	if _, err := db.CreateAuditLog(context.Background(), "admin", "update_settings", "", "1.2.3.4", "Settings updated", "success"); err != nil {
		t.Fatalf("seed log: %v", err)
	}

	w := doJSON(t, r, http.MethodGet, "/audit/logs/export", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv prefix", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".csv") {
		t.Errorf("Content-Disposition = %q, want an attachment .csv filename", cd)
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "id,created_at,username,category,action,host_name,ip_address,status,details") {
		t.Fatalf("unexpected CSV header, got: %s", body)
	}
	if !strings.Contains(body, "update_settings") || !strings.Contains(body, "settings") {
		t.Errorf("expected the seeded row (action + category) in the CSV, got: %s", body)
	}
}

func TestExportAuditLogs_RequiresAdmin(t *testing.T) {
	r, _ := newAuditRouter(t, "viewer")
	w := doJSON(t, r, http.MethodGet, "/audit/logs/export", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin export = %d, want 403; body = %s", w.Code, w.Body.String())
	}
}
