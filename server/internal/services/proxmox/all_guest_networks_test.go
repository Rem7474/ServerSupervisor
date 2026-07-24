package proxmox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// TestAllGuestNetworks exercises the multi-node fan-out: two nodes on an
// enabled connection must both be queried and merged into the result keyed
// by node ID, while a third node on a disabled/unresolvable connection is
// silently skipped rather than failing the whole call — mirroring how a
// single guest agent being down doesn't fail NodeGuestNetworks today.
func TestAllGuestNetworks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes/pve1/qemu/100/agent/network-get-interfaces":
			writeData(t, w, []map[string]any{
				{
					"name":             "eth0",
					"hardware-address": "aa:bb:cc:dd:ee:01",
					"ip-addresses": []map[string]any{
						{"ip-address-type": "ipv4", "ip-address": "10.0.0.5", "prefix": 24},
					},
				},
			})
		case "/nodes/pve2/lxc/200/interfaces":
			writeData(t, w, []map[string]any{
				{"name": "eth0", "hwaddr": "aa:bb:cc:dd:ee:02", "inet": "10.0.0.6/24"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	repo := &fakeRepo{
		nodes: []models.ProxmoxNode{
			{ID: "node-1", ConnectionID: "conn-ok", NodeName: "pve1"},
			{ID: "node-2", ConnectionID: "conn-ok", NodeName: "pve2"},
			{ID: "node-3", ConnectionID: "conn-disabled", NodeName: "pve3"},
		},
		enabledConns: []database.ProxmoxConnectionFull{
			{ProxmoxConnection: models.ProxmoxConnection{ID: "conn-ok"}, TokenSecret: "secret"},
		},
		connByID: map[string]*models.ProxmoxConnection{
			"conn-ok": {ID: "conn-ok", APIURL: srv.URL, TokenID: "user@pve!token"},
		},
		guestsByNode: map[string][]models.ProxmoxGuest{
			"conn-ok|pve1":       {{VMID: 100, GuestType: "vm", Status: "running"}},
			"conn-ok|pve2":       {{VMID: 200, GuestType: "lxc", Status: "running"}},
			"conn-disabled|pve3": {{VMID: 300, GuestType: "vm", Status: "running"}},
		},
	}

	got, err := newSvc(repo).AllGuestNetworks(context.Background())
	if err != nil {
		t.Fatalf("AllGuestNetworks: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 nodes with data (pve3's disabled connection must be skipped), got %d: %+v", len(got), got)
	}
	if ifaces, ok := got["node-1"][100]; !ok || len(ifaces) != 1 || ifaces[0].IPs[0] != "10.0.0.5/24" {
		t.Errorf("node-1 vmid 100: expected one iface with 10.0.0.5/24, got %+v", got["node-1"])
	}
	if ifaces, ok := got["node-2"][200]; !ok || len(ifaces) != 1 || ifaces[0].IPs[0] != "10.0.0.6/24" {
		t.Errorf("node-2 vmid 200: expected one iface with 10.0.0.6/24, got %+v", got["node-2"])
	}
	if _, ok := got["node-3"]; ok {
		t.Errorf("node-3 (disabled connection) must be absent from the result, got %+v", got["node-3"])
	}
}

func writeData(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"data": data}); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
