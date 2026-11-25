package netdiscovery

import (
	"net/netip"
	"strings"
	"testing"
)

func TestIsPrivateIPv4(t *testing.T) {
	cases := map[string]bool{
		"10.1.2.3":        true,
		"172.16.0.1":      true,
		"172.31.255.254":  true,
		"192.168.1.10":    true,
		"172.32.0.1":      false, // just outside 172.16/12
		"8.8.8.8":         false,
		"127.0.0.1":       false,
		"169.254.1.1":     false,
		"100.64.0.1":      false, // CGNAT is not RFC 1918
		"::ffff:10.0.0.1": true,  // IPv4-mapped form is unmapped first
		"fd00::1":         false,
	}
	for in, want := range cases {
		if got := IsPrivateIPv4(netip.MustParseAddr(in)); got != want {
			t.Errorf("IsPrivateIPv4(%s) = %v, want %v", in, got, want)
		}
	}
}

func TestParseTarget(t *testing.T) {
	if a, err := ParseTarget(" 192.168.1.5 "); err != nil || a.String() != "192.168.1.5" {
		t.Fatalf("ParseTarget valid: %v %v", a, err)
	}
	for _, bad := range []string{"", "nope", "1.1.1.1", "192.168.1.0/24"} {
		if _, err := ParseTarget(bad); err == nil {
			t.Errorf("ParseTarget(%q) should fail", bad)
		}
	}
}

func TestExpandCIDR(t *testing.T) {
	tests := []struct {
		cidr  string
		count int
		first string
		last  string
	}{
		{"192.168.1.0/24", 254, "192.168.1.1", "192.168.1.254"},
		{"192.168.1.77/24", 254, "192.168.1.1", "192.168.1.254"}, // host bits masked
		{"10.0.0.0/30", 2, "10.0.0.1", "10.0.0.2"},
		{"10.0.0.0/31", 2, "10.0.0.0", "10.0.0.1"},
		{"10.0.0.9/32", 1, "10.0.0.9", "10.0.0.9"},
	}
	for _, tt := range tests {
		got, err := ExpandCIDR(tt.cidr)
		if err != nil {
			t.Fatalf("ExpandCIDR(%s): %v", tt.cidr, err)
		}
		if len(got) != tt.count || got[0].String() != tt.first || got[len(got)-1].String() != tt.last {
			t.Errorf("ExpandCIDR(%s) = %d addrs [%s..%s], want %d [%s..%s]", tt.cidr, len(got), got[0], got[len(got)-1], tt.count, tt.first, tt.last)
		}
	}
}

func TestExpandCIDRRefusals(t *testing.T) {
	tests := map[string]string{
		"garbage":        "invalid CIDR",
		"fd00::/120":     "only IPv4",
		"192.168.0.0/23": "too wide",
		"10.0.0.0/8":     "too wide",
		"8.8.8.0/24":     "not a private",
		"127.0.0.0/30":   "not a private",
	}
	for cidr, want := range tests {
		_, err := ExpandCIDR(cidr)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ExpandCIDR(%s) error = %v, want %q", cidr, err, want)
		}
	}
}

func TestNormalizePorts(t *testing.T) {
	got, err := NormalizePorts([]int{443, 22, 443, 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != 22 || got[1] != 80 || got[2] != 443 {
		t.Fatalf("NormalizePorts = %v", got)
	}
	if _, err := NormalizePorts(nil); err == nil {
		t.Error("empty list should fail")
	}
	for _, bad := range []int{0, -1, 65536} {
		if _, err := NormalizePorts([]int{bad}); err == nil {
			t.Errorf("port %d should fail", bad)
		}
	}
	tooMany := make([]int, MaxPorts+1)
	for i := range tooMany {
		tooMany[i] = i + 1
	}
	if _, err := NormalizePorts(tooMany); err == nil || !strings.Contains(err.Error(), "too many ports") {
		t.Errorf("too many ports error = %v", err)
	}
	// Duplicates do not count towards the cap.
	dup := make([]int, MaxPorts+10)
	for i := range dup {
		dup[i] = 22
	}
	if _, err := NormalizePorts(dup); err != nil {
		t.Errorf("duplicates should collapse below the cap: %v", err)
	}
}
