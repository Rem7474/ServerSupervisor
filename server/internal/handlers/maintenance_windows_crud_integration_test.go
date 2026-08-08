package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	maintenancesvc "github.com/serversupervisor/server/internal/services/maintenance"
	"github.com/serversupervisor/server/internal/testutil"
)

// newMaintenanceWindowsRouter wires the maintenance-window handler the same
// way router.go does (registerMaintenanceRoutes): plain routes for
// list/create/delete (RBAC enforced inside the handler via
// requireHostAccess), AdminOnlyMiddleware only in front of the global-create
// route.
func newMaintenanceWindowsRouter(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	r, _ := newMaintenanceWindowsRouterOnDB(t, db, role)
	return r, db
}

func newMaintenanceWindowsRouterOnDB(t *testing.T, db *database.DB, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	h := handlers.NewMaintenanceWindowHandler(maintenancesvc.NewService(db), db)

	r := gin.New()
	r.Use(withRole(role))
	r.GET("/maintenance-windows", h.ListAllMaintenanceWindows)
	r.GET("/hosts/:id/maintenance-windows", h.ListMaintenanceWindowsForHost)
	r.POST("/hosts/:id/maintenance-windows", h.CreateMaintenanceWindow)
	r.DELETE("/maintenance-windows/:id", h.DeleteMaintenanceWindow)

	admin := r.Group("")
	admin.Use(withRoleGate("admin"))
	admin.POST("/maintenance-windows/global", h.CreateGlobalMaintenanceWindow)
	return r, db
}

// withRoleGate mirrors AdminOnlyMiddleware's behavior (403 unless the role
// injected by withRole matches) without importing internal/api, avoiding a
// handlers_test -> internal/api -> internal/handlers round trip.
func withRoleGate(required string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != required {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func validWindowPayload() map[string]any {
	now := time.Now().UTC()
	return map[string]any{
		"reason":    "kernel upgrade",
		"starts_at": now.Format(time.RFC3339),
		"ends_at":   now.Add(time.Hour).Format(time.RFC3339),
	}
}

func TestMaintenanceWindowsCRUD(t *testing.T) {
	r, db := newMaintenanceWindowsRouter(t, "admin")
	const hostID = "mw-host-1"
	seedHost(t, db, hostID)

	w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/maintenance-windows", validWindowPayload())
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("created window has no id: %s", w.Body.String())
	}
	if created["host_id"] != hostID {
		t.Fatalf("created window host_id = %v, want %s", created["host_id"], hostID)
	}

	// List for host includes it.
	w = doJSON(t, r, http.MethodGet, "/hosts/"+hostID+"/maintenance-windows", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list for host status = %d, body = %s", w.Code, w.Body.String())
	}
	var forHost []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &forHost); err != nil {
		t.Fatalf("decode list for host: %v", err)
	}
	if len(forHost) != 1 {
		t.Fatalf("list for host = %d windows, want 1", len(forHost))
	}

	// Global list includes it too.
	w = doJSON(t, r, http.MethodGet, "/maintenance-windows", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list all status = %d, body = %s", w.Code, w.Body.String())
	}
	var all []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil {
		t.Fatalf("decode list all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("list all = %d windows, want 1", len(all))
	}

	// Delete.
	w = doJSON(t, r, http.MethodDelete, "/maintenance-windows/"+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, "/maintenance-windows", nil)
	if err := json.Unmarshal(w.Body.Bytes(), &all); err != nil {
		t.Fatalf("decode list all after delete: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("list all after delete = %d windows, want 0", len(all))
	}
}

func TestMaintenanceWindowCreateValidation(t *testing.T) {
	r, db := newMaintenanceWindowsRouter(t, "admin")
	const hostID = "mw-host-2"
	seedHost(t, db, hostID)

	// Missing reason.
	now := time.Now().UTC()
	bad := map[string]any{
		"starts_at": now.Format(time.RFC3339),
		"ends_at":   now.Add(time.Hour).Format(time.RFC3339),
	}
	w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/maintenance-windows", bad)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing reason = %d, want 400; body = %s", w.Code, w.Body.String())
	}

	// ends_at before starts_at.
	bad = map[string]any{
		"reason":    "bad range",
		"starts_at": now.Format(time.RFC3339),
		"ends_at":   now.Add(-time.Hour).Format(time.RFC3339),
	}
	w = doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/maintenance-windows", bad)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("ends_at before starts_at = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

// TestMaintenanceWindowGlobalRequiresAdmin covers the deliberately stricter
// gate on the all-hosts window: it silences every alert in the system at
// once, so only admin can create or delete one (see the handler's doc
// comment), unlike a host-scoped window which follows the Operator+
// per-host model shared with scheduled tasks.
func TestMaintenanceWindowGlobalRequiresAdmin(t *testing.T) {
	r, _ := newMaintenanceWindowsRouter(t, "operator")
	w := doJSON(t, r, http.MethodPost, "/maintenance-windows/global", validWindowPayload())
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin global create = %d, want 403; body = %s", w.Code, w.Body.String())
	}

	admin, adminDB := newMaintenanceWindowsRouter(t, "admin")
	w = doJSON(t, admin, http.MethodPost, "/maintenance-windows/global", validWindowPayload())
	if w.Code != http.StatusCreated {
		t.Fatalf("admin global create = %d, want 201; body = %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created["host_id"] != nil {
		t.Fatalf("global window host_id = %v, want null", created["host_id"])
	}
	id, _ := created["id"].(string)

	// Deleting the global window as a non-admin must also be rejected.
	op, _ := newMaintenanceWindowsRouterOnDB(t, adminDB, "operator")
	w = doJSON(t, op, http.MethodDelete, "/maintenance-windows/"+id, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin global delete = %d, want 403; body = %s", w.Code, w.Body.String())
	}
	w = doJSON(t, admin, http.MethodDelete, "/maintenance-windows/"+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin global delete = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

// TestMaintenanceWindowCreateRequiresOperatorHostAccess is the host-scoped
// RBAC regression test, mirroring the scheduled-tasks one: a restricted
// "operator"-role user with only viewer-level host_permissions must be
// rejected, and unblocked once upgraded to operator-level on that host.
func TestMaintenanceWindowCreateRequiresOperatorHostAccess(t *testing.T) {
	r, db := newMaintenanceWindowsRouter(t, "operator")
	const hostID = "mw-rbac-host-1"
	seedHost(t, db, hostID)

	if err := db.SetHostPermission(context.Background(), "tester", hostID, "viewer"); err != nil {
		t.Fatalf("seed host permission: %v", err)
	}
	w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/maintenance-windows", validWindowPayload())
	if w.Code != http.StatusForbidden {
		t.Fatalf("create with viewer-level host access = %d, want 403; body = %s", w.Code, w.Body.String())
	}

	if err := db.SetHostPermission(context.Background(), "tester", hostID, "operator"); err != nil {
		t.Fatalf("upgrade host permission: %v", err)
	}
	w = doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/maintenance-windows", validWindowPayload())
	if w.Code != http.StatusCreated {
		t.Fatalf("create with operator-level host access = %d, want 201; body = %s", w.Code, w.Body.String())
	}
}
