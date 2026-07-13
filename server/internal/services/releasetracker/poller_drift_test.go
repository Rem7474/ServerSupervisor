package releasetracker

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

// TestReconcileComposeDrift_ShortCircuits exercises reconcileComposeDrift's
// guard conditions, all of which must return before ever touching s.db — a
// Poller with a nil db proves that: a real DB call here would nil-pointer
// panic instead of silently doing nothing, so these cases are only "safe by
// construction" (git trackers, custom-mode trackers, etc. never dispatch
// anything from drift) if the guards actually short-circuit correctly.
func TestReconcileComposeDrift_ShortCircuits(t *testing.T) {
	p := &Poller{db: nil}
	hostID := "h1"

	cases := []struct {
		name string
		t    models.ReleaseTracker
	}{
		{"reconcile_drift disabled (the default)", models.ReleaseTracker{ReconcileDrift: false, UpdateAction: "compose", HostID: hostID, ComposeProject: "proj"}},
		{"custom update_action (not compose)", models.ReleaseTracker{ReconcileDrift: true, UpdateAction: "custom", HostID: hostID, CustomTaskID: "task-1"}},
		{"no dispatch target (monitor-only)", models.ReleaseTracker{ReconcileDrift: true, UpdateAction: "compose", HostID: "", ComposeProject: ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("reconcileComposeDrift touched a nil db instead of short-circuiting: %v", r)
				}
			}()
			p.reconcileComposeDrift(context.Background(), tc.t, "latest", "latest", "sha256:abc")
		})
	}
}
