package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/scheduler"
	scheduledtasksvc "github.com/serversupervisor/server/internal/services/scheduledtask"
	"github.com/serversupervisor/server/internal/testutil"
)

func newScheduledTasksRouter(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	disp := dispatch.New(db)
	sched := scheduler.New(db, disp)
	h := handlers.NewScheduledTaskHandler(scheduledtasksvc.NewService(db, sched, disp), db)

	r := gin.New()
	r.Use(withRole("admin"))
	r.GET("/scheduled-tasks", h.ListAllScheduledTasks)
	r.GET("/hosts/:id/scheduled-tasks", h.ListScheduledTasks)
	r.POST("/hosts/:id/scheduled-tasks", h.CreateScheduledTask)
	r.PUT("/scheduled-tasks/:id", h.UpdateScheduledTask)
	r.DELETE("/scheduled-tasks/:id", h.DeleteScheduledTask)
	return r, db
}

func validTaskPayload() map[string]any {
	return map[string]any{
		"name":            "nightly apt",
		"module":          "apt",
		"action":          "update",
		"cron_expression": "0 3 * * *",
		"enabled":         true,
	}
}

func TestScheduledTasksCRUD(t *testing.T) {
	r, db := newScheduledTasksRouter(t)
	const hostID = "sched-host-1"
	seedHost(t, db, hostID)

	// Create
	w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/scheduled-tasks", validTaskPayload())
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatalf("created task has no id: %s", w.Body.String())
	}

	// List for host
	wl := doJSON(t, r, http.MethodGet, "/hosts/"+hostID+"/scheduled-tasks", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list host tasks = %d", wl.Code)
	}
	var hostTasks []map[string]any
	_ = json.Unmarshal(wl.Body.Bytes(), &hostTasks)
	if len(hostTasks) != 1 {
		t.Fatalf("expected 1 host task, got %d", len(hostTasks))
	}

	// Global list
	wg := doJSON(t, r, http.MethodGet, "/scheduled-tasks", nil)
	if wg.Code != http.StatusOK {
		t.Fatalf("global list = %d", wg.Code)
	}
	var all []map[string]any
	_ = json.Unmarshal(wg.Body.Bytes(), &all)
	if len(all) != 1 {
		t.Fatalf("expected 1 global task, got %d", len(all))
	}

	// Update (change name + cron)
	upd := validTaskPayload()
	upd["name"] = "nightly apt renamed"
	upd["cron_expression"] = "0 4 * * *"
	u := doJSON(t, r, http.MethodPut, "/scheduled-tasks/"+id, upd)
	if u.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", u.Code, u.Body.String())
	}
	var updated map[string]any
	_ = json.Unmarshal(u.Body.Bytes(), &updated)
	if updated["name"] != "nightly apt renamed" {
		t.Errorf("name = %v, want renamed", updated["name"])
	}

	// Delete
	if d := doJSON(t, r, http.MethodDelete, "/scheduled-tasks/"+id, nil); d.Code != http.StatusOK {
		t.Fatalf("delete status = %d", d.Code)
	}
	// Deleting again -> 404 (no such task)
	if d := doJSON(t, r, http.MethodDelete, "/scheduled-tasks/"+id, nil); d.Code != http.StatusNotFound {
		t.Errorf("delete missing = %d, want 404", d.Code)
	}
}

// newScheduledTasksRouterWithRunCustomTask adds the RunCustomTask route,
// which the other router builders above don't register.
func newScheduledTasksRouterWithRunCustomTask(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	disp := dispatch.New(db)
	sched := scheduler.New(db, disp)
	h := handlers.NewScheduledTaskHandler(scheduledtasksvc.NewService(db, sched, disp), db)

	r := gin.New()
	r.Use(withRole(role))
	r.POST("/hosts/:id/custom-tasks/:taskId/run", h.RunCustomTask)
	return r, db
}

// TestRunCustomTask dispatches an ad-hoc tasks.yaml task (module=custom,
// action=run, target=the task id) — the ad-hoc equivalent of RunScheduledTask
// for the one module that otherwise had no dedicated ad-hoc endpoint.
func TestRunCustomTask(t *testing.T) {
	r, db := newScheduledTasksRouterWithRunCustomTask(t, "admin")
	const hostID = "custom-task-host-1"
	seedHost(t, db, hostID)

	w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/custom-tasks/backup-db/run", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "pending" {
		t.Errorf("status field = %v, want pending", body["status"])
	}
	if cmdID, _ := body["command_id"].(string); cmdID == "" {
		t.Error("expected a non-empty command_id")
	}
}

// TestRunCustomTask_RequiresOperatorHostAccess covers the same
// requireHostAccess("operator") gate every other scheduled-task mutation has.
func TestRunCustomTask_RequiresOperatorHostAccess(t *testing.T) {
	r, db := newScheduledTasksRouterWithRunCustomTask(t, "operator")
	const hostID = "custom-task-host-2"
	seedHost(t, db, hostID)

	if err := db.SetHostPermission(context.Background(), "tester", hostID, "viewer"); err != nil {
		t.Fatalf("seed host permission: %v", err)
	}
	if w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/custom-tasks/backup-db/run", nil); w.Code != http.StatusForbidden {
		t.Fatalf("viewer-level access = %d, want 403; body = %s", w.Code, w.Body.String())
	}

	if err := db.SetHostPermission(context.Background(), "tester", hostID, "operator"); err != nil {
		t.Fatalf("upgrade host permission: %v", err)
	}
	if w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/custom-tasks/backup-db/run", nil); w.Code != http.StatusOK {
		t.Fatalf("operator-level access = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

func TestScheduledTaskCreateValidation(t *testing.T) {
	r, db := newScheduledTasksRouter(t)
	const hostID = "sched-host-2"
	seedHost(t, db, hostID)
	path := "/hosts/" + hostID + "/scheduled-tasks"

	cases := []struct {
		name   string
		mutate func(p map[string]any)
	}{
		{"missing action", func(p map[string]any) { delete(p, "action") }},
		{"missing cron", func(p map[string]any) { delete(p, "cron_expression") }},
		{"invalid module", func(p map[string]any) { p["module"] = "bogus" }},
		{"invalid cron", func(p map[string]any) { p["cron_expression"] = "not a cron" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validTaskPayload()
			tc.mutate(p)
			if w := doJSON(t, r, http.MethodPost, path, p); w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestScheduledTaskUpdateNotFound(t *testing.T) {
	r, _ := newScheduledTasksRouter(t)
	w := doJSON(t, r, http.MethodPut, "/scheduled-tasks/00000000-0000-0000-0000-000000000000", validTaskPayload())
	if w.Code != http.StatusNotFound {
		t.Errorf("update missing = %d, want 404; body = %s", w.Code, w.Body.String())
	}
}

// newScheduledTasksRouterAsOperator mirrors newScheduledTasksRouter but runs
// requests as a non-admin "operator" role/"tester" username, so per-host
// requireHostAccess restrictions (host_permissions rows) actually apply —
// role "admin" always short-circuits requireHostAccess, which would hide a
// regression on the create/update/delete checks added below.
func newScheduledTasksRouterAsOperator(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	disp := dispatch.New(db)
	sched := scheduler.New(db, disp)
	h := handlers.NewScheduledTaskHandler(scheduledtasksvc.NewService(db, sched, disp), db)

	r := gin.New()
	r.Use(withRole("operator"))
	r.POST("/hosts/:id/scheduled-tasks", h.CreateScheduledTask)
	r.PUT("/scheduled-tasks/:id", h.UpdateScheduledTask)
	r.DELETE("/scheduled-tasks/:id", h.DeleteScheduledTask)
	return r, db
}

// TestScheduledTaskCreateRequiresOperatorHostAccess is the regression test for
// the RBAC gap documented in wiki/Runbooks-and-Scheduled-Tasks.md §3: create was
// previously reachable by any authenticated caller regardless of per-host
// permissions, unlike RunScheduledTask which was already Operator+-gated.
func TestScheduledTaskCreateRequiresOperatorHostAccess(t *testing.T) {
	r, db := newScheduledTasksRouterAsOperator(t)
	const hostID = "sched-rbac-host-1"
	seedHost(t, db, hostID)

	// "tester" has a restricted host_permissions row (viewer level) on this
	// host -> create must be rejected even though their global role is
	// "operator".
	if err := db.SetHostPermission(context.Background(), "tester", hostID, "viewer"); err != nil {
		t.Fatalf("seed host permission: %v", err)
	}
	if w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/scheduled-tasks", validTaskPayload()); w.Code != http.StatusForbidden {
		t.Fatalf("create with viewer-level host access = %d, want 403; body = %s", w.Code, w.Body.String())
	}

	// Upgrading to operator-level access on that same host must unblock it.
	if err := db.SetHostPermission(context.Background(), "tester", hostID, "operator"); err != nil {
		t.Fatalf("upgrade host permission: %v", err)
	}
	if w := doJSON(t, r, http.MethodPost, "/hosts/"+hostID+"/scheduled-tasks", validTaskPayload()); w.Code != http.StatusCreated {
		t.Fatalf("create with operator-level host access = %d, want 201; body = %s", w.Code, w.Body.String())
	}
}

// TestScheduledTaskUpdateDeleteRequireOperatorHostAccess covers the same gap
// for update/delete, which must resolve the task's host before checking
// access (unlike create, :id here is the task id, not the host id).
func TestScheduledTaskUpdateDeleteRequireOperatorHostAccess(t *testing.T) {
	admin, db := newScheduledTasksRouter(t)
	const hostID = "sched-rbac-host-2"
	seedHost(t, db, hostID)

	created := doJSON(t, admin, http.MethodPost, "/hosts/"+hostID+"/scheduled-tasks", validTaskPayload())
	if created.Code != http.StatusCreated {
		t.Fatalf("seed task create = %d, body = %s", created.Code, created.Body.String())
	}
	var task map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode created task: %v", err)
	}
	id, _ := task["id"].(string)
	if id == "" {
		t.Fatalf("created task has no id: %s", created.Body.String())
	}

	op, _ := newScheduledTasksRouterAsOperatorOnDB(t, db)

	// "tester" restricted to viewer level on this host -> both blocked.
	if err := db.SetHostPermission(context.Background(), "tester", hostID, "viewer"); err != nil {
		t.Fatalf("seed host permission: %v", err)
	}
	if w := doJSON(t, op, http.MethodPut, "/scheduled-tasks/"+id, validTaskPayload()); w.Code != http.StatusForbidden {
		t.Fatalf("update with viewer-level host access = %d, want 403; body = %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, op, http.MethodDelete, "/scheduled-tasks/"+id, nil); w.Code != http.StatusForbidden {
		t.Fatalf("delete with viewer-level host access = %d, want 403; body = %s", w.Code, w.Body.String())
	}

	// Operator-level access on that host unblocks both.
	if err := db.SetHostPermission(context.Background(), "tester", hostID, "operator"); err != nil {
		t.Fatalf("upgrade host permission: %v", err)
	}
	if w := doJSON(t, op, http.MethodPut, "/scheduled-tasks/"+id, validTaskPayload()); w.Code != http.StatusOK {
		t.Fatalf("update with operator-level host access = %d, want 200; body = %s", w.Code, w.Body.String())
	}
	if w := doJSON(t, op, http.MethodDelete, "/scheduled-tasks/"+id, nil); w.Code != http.StatusOK {
		t.Fatalf("delete with operator-level host access = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

// newScheduledTasksRouterAsOperatorOnDB is newScheduledTasksRouterAsOperator
// but reuses an already-open db (needed when a test seeds data as admin
// first, then re-issues requests as a restricted operator against the same
// database).
func newScheduledTasksRouterAsOperatorOnDB(t *testing.T, db *database.DB) (*gin.Engine, *database.DB) {
	t.Helper()
	disp := dispatch.New(db)
	sched := scheduler.New(db, disp)
	h := handlers.NewScheduledTaskHandler(scheduledtasksvc.NewService(db, sched, disp), db)

	r := gin.New()
	r.Use(withRole("operator"))
	r.PUT("/scheduled-tasks/:id", h.UpdateScheduledTask)
	r.DELETE("/scheduled-tasks/:id", h.DeleteScheduledTask)
	return r, db
}
