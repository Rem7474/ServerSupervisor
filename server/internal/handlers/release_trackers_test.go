package handlers_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// newReleaseTrackerRouter wires the release-tracker CRUD routes behind a
// middleware that injects a fixed role, mirroring what JWTMiddleware sets in
// production (c.Set("role", ...)). The poller is never started.
func newReleaseTrackerRouter(t *testing.T, role string) *gin.Engine {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewReleaseTrackerHandler(db, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("role", role) })
	r.GET("/release-trackers", h.List)
	r.POST("/release-trackers", h.Create)
	r.GET("/release-trackers/:id", h.Get)
	r.PUT("/release-trackers/:id", h.Update)
	r.DELETE("/release-trackers/:id", h.Delete)
	return r
}

func TestReleaseTracker_CreateGitRequiresAdmin(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleOperator)

	w := doJSON(t, r, http.MethodPost, "/release-trackers", map[string]any{
		"name":         "tracker",
		"tracker_type": "git",
		"provider":     "github",
		"repo_owner":   "torvalds",
		"repo_name":    "linux",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("operator must be forbidden from creating trackers, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestReleaseTracker_CreateGitMonitorOnlyPersists(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleAdmin)

	w := doJSON(t, r, http.MethodPost, "/release-trackers", map[string]any{
		"name":         "linux-stable",
		"tracker_type": "git",
		"provider":     "github",
		"repo_owner":   "torvalds",
		"repo_name":    "linux",
		// monitor-only: no host_id / custom_task_id
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
	}

	// It must now appear in the list.
	list := doJSON(t, r, http.MethodGet, "/release-trackers", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list: got %d", list.Code)
	}
	if !strings.Contains(list.Body.String(), "linux-stable") {
		t.Fatalf("created tracker not present in list: %s", list.Body.String())
	}
}

func TestReleaseTracker_CreateGitRejectsHalfDispatchConfig(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleAdmin)

	// host_id without custom_task_id is invalid for git trackers.
	w := doJSON(t, r, http.MethodPost, "/release-trackers", map[string]any{
		"name":         "bad",
		"tracker_type": "git",
		"provider":     "github",
		"repo_owner":   "torvalds",
		"repo_name":    "linux",
		"host_id":      "host-1",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("host_id without custom_task_id must be 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestReleaseTracker_CreateGitRejectsUnknownProvider(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleAdmin)

	w := doJSON(t, r, http.MethodPost, "/release-trackers", map[string]any{
		"name":         "bad-provider",
		"tracker_type": "git",
		"provider":     "bitbucket",
		"repo_owner":   "x",
		"repo_name":    "y",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown provider must be 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestReleaseTracker_CreateDockerRequiresImage(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleAdmin)

	w := doJSON(t, r, http.MethodPost, "/release-trackers", map[string]any{
		"name":         "no-image",
		"tracker_type": "docker",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("docker tracker without image must be 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestReleaseTracker_CreateRejectsOutOfRangeCooldown(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleAdmin)

	w := doJSON(t, r, http.MethodPost, "/release-trackers", map[string]any{
		"name":           "cooldown",
		"tracker_type":   "git",
		"provider":       "github",
		"repo_owner":     "x",
		"repo_name":      "y",
		"cooldown_hours": 999,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("cooldown_hours=999 must be 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestReleaseTracker_GetUnknownReturns404(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleAdmin)

	w := doJSON(t, r, http.MethodGet, "/release-trackers/00000000-0000-0000-0000-000000000000", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown tracker must be 404, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestReleaseTracker_DeleteRequiresAdmin(t *testing.T) {
	r := newReleaseTrackerRouter(t, models.RoleViewer)

	w := doJSON(t, r, http.MethodDelete, "/release-trackers/some-id", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("viewer must not delete trackers, got %d (%s)", w.Code, w.Body.String())
	}
}
