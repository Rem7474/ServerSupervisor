package main

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/testutil"
)

// TestSeedProxmox covers the demo Proxmox connection/node/guest seeding path,
// including the find-or-create-by-name idempotency seedProxmox's doc comment
// promises (a re-seed must reuse the same connection ID rather than
// duplicating it, so FK-linked nodes/guests stay stable).
func TestSeedProxmox(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	if err := seedProxmox(ctx, db); err != nil {
		t.Fatalf("seedProxmox: %v", err)
	}

	conns, err := db.ListProxmoxConnections(ctx)
	if err != nil {
		t.Fatalf("list connections: %v", err)
	}
	var connID string
	for _, c := range conns {
		if c.Name == demoProxmoxConnName {
			connID = c.ID
			if c.Enabled {
				t.Error("demo connection must be created disabled (defense in depth, see seedProxmox's doc comment)")
			}
		}
	}
	if connID == "" {
		t.Fatalf("demo connection %q not found among %+v", demoProxmoxConnName, conns)
	}

	guests, err := db.ListProxmoxGuests(ctx, connID, "", "")
	if err != nil {
		t.Fatalf("list guests: %v", err)
	}
	if len(guests) != 5 {
		t.Errorf("expected 5 demo guests, got %d", len(guests))
	}

	// Re-seeding must find the existing connection by name rather than
	// creating a second one.
	if err := seedProxmox(ctx, db); err != nil {
		t.Fatalf("second seedProxmox call: %v", err)
	}
	connsAfter, err := db.ListProxmoxConnections(ctx)
	if err != nil {
		t.Fatalf("list connections after re-seed: %v", err)
	}
	count := 0
	for _, c := range connsAfter {
		if c.Name == demoProxmoxConnName {
			count++
			if c.ID != connID {
				t.Errorf("re-seed should reuse connection ID %q, got %q", connID, c.ID)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 demo connection after re-seed, found %d", count)
	}
}
