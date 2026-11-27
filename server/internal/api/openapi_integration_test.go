package api

import (
	"testing"

	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/testutil"
	"github.com/serversupervisor/server/internal/ws"
)

// TestOpenAPISpec_RoutesExistInRouter checks every documented operation
// against the routes SetupRouter actually registers.
func TestOpenAPISpec_RoutesExistInRouter(t *testing.T) {
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	disp := dispatch.New(db)
	r, _, _, _, cleanup := SetupRouter(db, cfg, ws.NewNotificationHub(), events.NewBus(), scheduler.New(db, disp), disp)
	t.Cleanup(cleanup)

	registered := map[string]bool{}
	for _, ri := range r.Routes() {
		registered[ri.Method+" "+ri.Path] = true
	}
	documented, _ := documentedRoutes(t)
	for route := range documented {
		if !registered[route] {
			t.Errorf("documented operation %s is not registered in the router", route)
		}
	}
}
