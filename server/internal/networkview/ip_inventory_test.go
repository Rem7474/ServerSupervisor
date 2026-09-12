package networkview

import (
	"testing"

	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// TestExtractRoutableIPs guards the interface-name filter directly (no
// database needed, unlike TestBuildIPInventory) — this is where a guest
// whose primary NIC uses systemd's predictable naming (ens18, enp0s3, ...)
// used to be silently dropped entirely by a now-removed "ethX only" allowlist.
func TestExtractRoutableIPs(t *testing.T) {
	tests := []struct {
		name   string
		ifaces []proxmoxclient.GuestNetworkIface
		want   []string
	}{
		{
			name: "legacy ethN naming is kept",
			ifaces: []proxmoxclient.GuestNetworkIface{
				{Name: "eth0", IPs: []string{"10.0.5.5/24"}},
			},
			want: []string{"10.0.5.5"},
		},
		{
			name: "systemd predictable NIC names are kept, not treated as noise",
			ifaces: []proxmoxclient.GuestNetworkIface{
				{Name: "ens18", IPs: []string{"10.0.5.7/24"}},
				{Name: "enp0s3", IPs: []string{"10.0.5.8/24"}},
				{Name: "eno1", IPs: []string{"10.0.5.9/24"}},
			},
			want: []string{"10.0.5.7", "10.0.5.8", "10.0.5.9"},
		},
		{
			name: "known virtual/overlay interfaces are excluded as noise",
			ifaces: []proxmoxclient.GuestNetworkIface{
				{Name: "eth0", IPs: []string{"10.0.5.5/24"}},
				{Name: "lo", IPs: []string{"127.0.0.1/8"}},
				{Name: "docker0", IPs: []string{"172.17.0.1/16"}},
				{Name: "br-1234567890ab", IPs: []string{"172.18.0.1/16"}},
				{Name: "veth3a1b2c9", IPs: []string{"172.19.0.2/16"}},
				{Name: "tailscale0", IPs: []string{"100.64.0.1/32"}},
				{Name: "wg0", IPs: []string{"10.100.0.1/24"}},
				{Name: "virbr0", IPs: []string{"192.168.122.1/24"}},
			},
			want: []string{"10.0.5.5"},
		},
		{
			name: "loopback and link-local addresses are dropped even on a kept interface",
			ifaces: []proxmoxclient.GuestNetworkIface{
				{Name: "eth0", IPs: []string{"127.0.0.1/8", "169.254.1.5/16", "10.0.5.5/24"}},
			},
			want: []string{"10.0.5.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRoutableIPs(tt.ifaces)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					break
				}
			}
		})
	}
}
