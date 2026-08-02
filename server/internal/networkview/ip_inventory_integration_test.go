package networkview_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/networkview"
	"github.com/serversupervisor/server/internal/proxmoxclient"
	npmsvc "github.com/serversupervisor/server/internal/services/npm"
	"github.com/serversupervisor/server/internal/testutil"
)

// fakeGuestNetworks is a canned GuestNetworksProvider — BuildIPInventory's
// correlation logic is what this test exercises, not the live Proxmox HTTP
// call (already covered separately by proxmox.TestAllGuestNetworks).
type fakeGuestNetworks struct {
	nets map[string]map[int][]proxmoxclient.GuestNetworkIface
}

func (f fakeGuestNetworks) AllGuestNetworks(context.Context) (map[string]map[int][]proxmoxclient.GuestNetworkIface, error) {
	return f.nets, nil
}

// TestBuildIPInventory exercises the real SQL + correlation behind the
// Network page's IP inventory: a Proxmox guest is correlated with a Host via
// a confirmed proxmox_guest_links entry (or left "unlinked" without one),
// and an NPM proxy host is correlated by matching forward_host against
// either a Host's IP address or a Proxmox guest's live-fetched IP.
func TestBuildIPInventory(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	webHostIP := "10.0.0.10"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "web-host", Name: "web", Hostname: "web.local", IPAddress: webHostIP, Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}
	appHostID := "app-host"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: appHostID, Name: "app", Hostname: "app.local", IPAddress: "10.0.0.20", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	connID, err := db.CreateProxmoxConnection(ctx, "main", "https://pve.local:8006", "user@pve!token", "secret", false, true, 30)
	if err != nil {
		t.Fatalf("create proxmox connection: %v", err)
	}
	if err := db.UpsertProxmoxNode(ctx, connID, "pve1", "online", 8, 10, 32000, 16000, 1000, "8.2", "cluster1", "10.0.0.1"); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	nodes, err := db.ListProxmoxNodes(ctx)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("list nodes: %v (nodes=%+v)", err, nodes)
	}
	nodeID := nodes[0].ID

	// Guest A: linked (confirmed) to app-host.
	if err := db.UpsertProxmoxGuest(ctx, connID, "pve1", "vm", 100, "app-vm", "running", 2, 0.1, 4096, 1024, 20000, 3600, ""); err != nil {
		t.Fatalf("upsert guest A: %v", err)
	}
	// Guest B: no link.
	if err := db.UpsertProxmoxGuest(ctx, connID, "pve1", "lxc", 200, "db-vm", "running", 1, 0.05, 2048, 512, 10000, 3600, ""); err != nil {
		t.Fatalf("upsert guest B: %v", err)
	}
	guests, err := db.ListProxmoxGuests(ctx, connID, "", "")
	if err != nil || len(guests) != 2 {
		t.Fatalf("list guests: %v (guests=%+v)", err, guests)
	}
	var guestA, guestB models.ProxmoxGuest
	for _, g := range guests {
		switch g.VMID {
		case 100:
			guestA = g
		case 200:
			guestB = g
		}
	}
	if _, err := db.UpsertProxmoxGuestLink(ctx, guestA.ID, appHostID, "confirmed", "auto"); err != nil {
		t.Fatalf("link guest A: %v", err)
	}

	npmConn, err := db.CreateNPMConnection(ctx, models.NPMConnectionRequest{
		Name: "main npm", APIURL: "https://npm.local", Identity: "admin@example.com", Secret: "s3cr3t",
	})
	if err != nil {
		t.Fatalf("create npm connection: %v", err)
	}
	// Resolves to the Host by IP.
	if _, err := db.UpsertNPMProxyHost(ctx, models.NPMProxyHost{
		ConnectionID: npmConn.ID, NPMID: 1, DomainNames: []string{"web.example.com"},
		ForwardHost: webHostIP, ForwardPort: 8080, NPMEnabled: true,
	}); err != nil {
		t.Fatalf("upsert proxy host 1: %v", err)
	}
	// Resolves to guest B by its (fake, live-fetched) IP.
	if _, err := db.UpsertNPMProxyHost(ctx, models.NPMProxyHost{
		ConnectionID: npmConn.ID, NPMID: 2, DomainNames: []string{"db.example.com"},
		ForwardHost: "10.0.5.6", ForwardPort: 5432, NPMEnabled: true,
	}); err != nil {
		t.Fatalf("upsert proxy host 2: %v", err)
	}
	// Resolves to nothing known.
	if _, err := db.UpsertNPMProxyHost(ctx, models.NPMProxyHost{
		ConnectionID: npmConn.ID, NPMID: 3, DomainNames: []string{"external.example.com"},
		ForwardHost: "1.2.3.4", ForwardPort: 443, NPMEnabled: true,
	}); err != nil {
		t.Fatalf("upsert proxy host 3: %v", err)
	}

	guestNets := fakeGuestNetworks{nets: map[string]map[int][]proxmoxclient.GuestNetworkIface{
		nodeID: {
			// Guest A also reports a docker0 bridge IP, which must be
			// excluded — only "ethX" interfaces are correlation targets.
			100: {
				{Name: "eth0", IPs: []string{"10.0.5.5/24", "127.0.0.1/8"}},
				{Name: "docker0", IPs: []string{"172.17.0.1/16"}},
			},
			200: {{Name: "eth0", IPs: []string{"10.0.5.6/24"}}},
		},
	}}
	npmSvc := npmsvc.NewService(db)

	inventory, err := networkview.BuildIPInventory(ctx, db, guestNets, npmSvc)
	if err != nil {
		t.Fatalf("BuildIPInventory: %v", err)
	}

	if len(inventory.ProxmoxGuests) != 2 {
		t.Fatalf("expected 2 proxmox guests, got %d: %+v", len(inventory.ProxmoxGuests), inventory.ProxmoxGuests)
	}
	var gotA, gotB *models.NetworkProxmoxGuestIP
	for i := range inventory.ProxmoxGuests {
		g := &inventory.ProxmoxGuests[i]
		switch g.VMID {
		case 100:
			gotA = g
		case 200:
			gotB = g
		}
	}
	if gotA == nil || gotA.HostID != appHostID {
		t.Errorf("guest A should be linked to %s, got %+v", appHostID, gotA)
	}
	if gotA == nil || len(gotA.IPAddresses) != 1 || gotA.IPAddresses[0] != "10.0.5.5" {
		t.Errorf("guest A should have exactly one routable IP (loopback and docker0 filtered out, only eth0 kept), got %+v", gotA)
	}
	if gotB == nil || gotB.HostID != "" {
		t.Errorf("guest B has no confirmed link, expected empty host_id, got %+v", gotB)
	}

	if len(inventory.NPMHosts) != 3 {
		t.Fatalf("expected 3 npm entries, got %d: %+v", len(inventory.NPMHosts), inventory.NPMHosts)
	}
	byNPMID := map[int]models.NetworkNPMEntry{}
	for _, n := range inventory.NPMHosts {
		byNPMID[n.ProxyHostID] = n
	}
	if e := byNPMID[1]; e.MatchedType != "host" || e.MatchedID != "web-host" {
		t.Errorf("proxy host 1 should match the Host by IP, got %+v", e)
	}
	if e := byNPMID[2]; e.MatchedType != "proxmox_guest" || e.MatchedID != guestB.ID {
		t.Errorf("proxy host 2 should match guest B by IP, got %+v (guestB.ID=%s)", e, guestB.ID)
	}
	if e := byNPMID[3]; e.MatchedType != "" {
		t.Errorf("proxy host 3 should resolve to nothing, got %+v", e)
	}
}
