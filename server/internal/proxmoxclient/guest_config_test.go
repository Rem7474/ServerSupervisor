package proxmoxclient

import "testing"

func TestParseStaticConfigIPs(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]any
		want []GuestNetworkIface
	}{
		{
			name: "lxc static ip",
			cfg: map[string]any{
				"net0":   "name=eth0,bridge=vmbr0,hwaddr=AA:BB:CC:DD:EE:01,ip=192.168.1.10/24,gw=192.168.1.1,type=veth",
				"cores":  float64(2), // non-string config values must be ignored, not crash
				"memory": float64(512),
			},
			want: []GuestNetworkIface{{Name: "eth0", MAC: "AA:BB:CC:DD:EE:01", IPs: []string{"192.168.1.10/24"}}},
		},
		{
			name: "lxc dhcp is not a static ip",
			cfg: map[string]any{
				"net0": "name=eth0,bridge=vmbr0,ip=dhcp,type=veth",
			},
			want: nil,
		},
		{
			name: "lxc ipv6 alongside ipv4",
			cfg: map[string]any{
				"net0": "name=eth0,bridge=vmbr0,ip=10.0.0.5/24,ip6=fd00::5/64,type=veth",
			},
			want: []GuestNetworkIface{{Name: "eth0", IPs: []string{"10.0.0.5/24", "fd00::5/64"}}},
		},
		{
			name: "vm cloud-init ipconfig",
			cfg: map[string]any{
				"ipconfig0": "ip=10.0.0.8/24,gw=10.0.0.1",
			},
			want: []GuestNetworkIface{{Name: "eth0", IPs: []string{"10.0.0.8/24"}}},
		},
		{
			name: "vm without cloud-init has no recoverable ip",
			cfg: map[string]any{
				"net0": "virtio=AA:BB:CC:DD:EE:02,bridge=vmbr0",
			},
			want: nil,
		},
		{
			name: "vm cloud-init dhcp is not a static ip",
			cfg: map[string]any{
				"ipconfig0": "ip=dhcp",
			},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseStaticConfigIPs(tc.cfg)
			if len(got) != len(tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got {
				if got[i].Name != tc.want[i].Name {
					t.Errorf("iface %d name = %q, want %q", i, got[i].Name, tc.want[i].Name)
				}
				if got[i].MAC != tc.want[i].MAC {
					t.Errorf("iface %d mac = %q, want %q", i, got[i].MAC, tc.want[i].MAC)
				}
				if len(got[i].IPs) != len(tc.want[i].IPs) {
					t.Fatalf("iface %d ips = %+v, want %+v", i, got[i].IPs, tc.want[i].IPs)
				}
				for j := range got[i].IPs {
					if got[i].IPs[j] != tc.want[i].IPs[j] {
						t.Errorf("iface %d ip %d = %q, want %q", i, j, got[i].IPs[j], tc.want[i].IPs[j])
					}
				}
			}
		})
	}
}
