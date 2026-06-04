package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/testutil"
	"github.com/serversupervisor/server/internal/ws"
)

// newReleaseTrackerCRUDRouter wires the List/Update routes that the existing
// release_trackers_test.go router omits, so the full CRUD lifecycle (and the
// List + Update handlers specifically) can be exercised end to end.
func newReleaseTrackerCRUDRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewReleaseTrackerHandler(db, cfg, dispatch.New(db), ws.NewNotificationHub())

	r := gin.New()
	r.Use(withRole("admin"))
	r.GET("/release-trackers", h.List)
	r.POST("/release-trackers", h.Create)
	r.GET("/release-trackers/:id", h.Get)
	r.PUT("/release-trackers/:id", h.Update)
	r.DELETE("/release-trackers/:id", h.Delete)
	return r
}

// TestReleaseTrackerLifecycle complements release_trackers_test.go (which covers
// create validation / auth / 404) by exercising the List and Update handlers and
// the full create -> list -> get -> update -> delete -> 404 round-trip.
func TestReleaseTrackerLifecycle(t *testing.T) {
	r := newReleaseTrackerCRUDRouter(t)

	payload := map[string]any{
		"name": "track-app", "tracker_type": "git", "provider": "github",
		"repo_owner": "acme", "repo_name": "app",
	}
	w := doJSON(t, r, http.MethodPost, "/release-trackers", payload)
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var cr struct {
		Tracker struct {
			ID string `json:"id"`
		} `json:"tracker"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cr); err != nil || cr.Tracker.ID == "" {
		t.Fatalf("decode created tracker: %v (%s)", err, w.Body.String())
	}
	idPath := "/release-trackers/" + cr.Tracker.ID

	// List wraps results under "trackers".
	wl := doJSON(t, r, http.MethodGet, "/release-trackers", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	var list struct {
		Trackers []map[string]any `json:"trackers"`
	}
	_ = json.Unmarshal(wl.Body.Bytes(), &list)
	if len(list.Trackers) != 1 {
		t.Fatalf("expected 1 tracker, got %d", len(list.Trackers))
	}

	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusOK {
		t.Fatalf("get status = %d", g.Code)
	}

	// Update (rename).
	upd := map[string]any{
		"name": "track-app-renamed", "tracker_type": "git", "provider": "github",
		"repo_owner": "acme", "repo_name": "app",
	}
	if u := doJSON(t, r, http.MethodPut, idPath, upd); u.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", u.Code, u.Body.String())
	}

	if d := doJSON(t, r, http.MethodDelete, idPath, nil); d.Code != http.StatusOK {
		t.Fatalf("delete status = %d", d.Code)
	}
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}
