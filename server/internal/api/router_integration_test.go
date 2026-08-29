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
