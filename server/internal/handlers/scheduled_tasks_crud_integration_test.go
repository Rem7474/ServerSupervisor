package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/testutil"
)

func newScheduledTasksRouter(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	disp := dispatch.New(db)
	sched := scheduler.New(db, disp)
	h := handlers.NewScheduledTaskHandler(db, cfg, disp, sched)

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
