package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serversupervisor/server/internal/api"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/testutil"
	"github.com/serversupervisor/server/internal/ws"
)

// TestSetupRouter_WiresProxmoxConsoleRoute is a wiring smoke test for
// SetupRouter: it must return non-nil handlers and register the interactive
// LXC console WS route (backed by the same proxmoxsvc.Service instance
// wsH.ProxmoxConsole uses) alongside every other route group.
func TestSetupRouter_WiresProxmoxConsoleRoute(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	disp := dispatch.New(db)
	sched := scheduler.New(db, disp)
	bus := events.NewBus()
	notifHub := ws.NewNotificationHub()

	r, releaseTrackerH, proxmoxH, npmH, cleanup := api.SetupRouter(db, cfg, notifHub, bus, sched, disp)
	t.Cleanup(cleanup)

	if r == nil || releaseTrackerH == nil || proxmoxH == nil || npmH == nil {
		t.Fatalf("SetupRouter returned a nil handler: router=%v releaseTracker=%v proxmox=%v npm=%v",
			r, releaseTrackerH, proxmoxH, npmH)
	}

	// A plain (non-upgrade) GET against the console route must reach the
	// handler (proving the route is registered) rather than 404 — the
	// handler itself then fails the WebSocket upgrade since this isn't a
	// real WS handshake, which is expected and not what this test checks.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ws/proxmox/console/some-guest-id", nil)
	r.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Errorf("proxmox console route not registered, got 404")
	}
}

func TestStaticFiles_HEADAndCacheHeaders(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	cfg.RateLimitRPS = 1000
	cfg.RateLimitBurst = 1000
	disp := dispatch.New(db)
	sched := scheduler.New(db, disp)
	bus := events.NewBus()
	notifHub := ws.NewNotificationHub()

	r, _, _, _, cleanup := api.SetupRouter(db, cfg, notifHub, bus, sched, disp)
	t.Cleanup(cleanup)

	// 1. HEAD / should return 200 or 404 depending on index.html presence, but must NOT return "method not allowed"
	wHead := httptest.NewRecorder()
	reqHead := httptest.NewRequest(http.MethodHead, "/", nil)
	r.ServeHTTP(wHead, reqHead)
	if wHead.Code == http.StatusMethodNotAllowed {
		t.Errorf("HEAD / returned 405 Method Not Allowed")
	}

	// 2. 404 under /assets/* must NOT have "immutable" cache header
	wAsset404 := httptest.NewRecorder()
	reqAsset404 := httptest.NewRequest(http.MethodGet, "/assets/missing-chunk-123.js", nil)
	r.ServeHTTP(wAsset404, reqAsset404)
	if wAsset404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing asset, got %d", wAsset404.Code)
	}
	cacheControl := wAsset404.Header().Get("Cache-Control")
	if cacheControl == "public, max-age=31536000, immutable" {
		t.Errorf("missing asset 404 must not have immutable cache header: got %q", cacheControl)
	}

	// 3. Unmatched /api/* routes must return 404 JSON, not HTML SPA fallback
	wAPI404 := httptest.NewRecorder()
	reqAPI404 := httptest.NewRequest(http.MethodGet, "/api/v1/unknown-endpoint", nil)
	r.ServeHTTP(wAPI404, reqAPI404)
	if wAPI404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown api endpoint, got %d", wAPI404.Code)
	}
	contentType := wAPI404.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" && contentType != "application/json" {
		t.Errorf("expected JSON Content-Type for unknown api endpoint, got %q", contentType)
	}
}
