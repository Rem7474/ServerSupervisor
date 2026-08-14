package collector

import (
	"testing"
	"time"
)

func TestBuildContainerIPIndex(t *testing.T) {
	tests := []struct {
		name       string
		containers []DockerContainer
		want       map[string]string
	}{
		{
			name:       "no containers",
			containers: nil,
			want:       map[string]string{},
		},
		{
			name: "one container on the default bridge",
			containers: []DockerContainer{
				{Name: "nginx", IPAddresses: []string{"172.17.0.4"}},
			},
			want: map[string]string{"172.17.0.4": "nginx"},
		},
		{
			name: "container attached to several networks",
			containers: []DockerContainer{
				{Name: "app", IPAddresses: []string{"172.18.0.2", "172.19.0.7"}},
			},
			want: map[string]string{"172.18.0.2": "app", "172.19.0.7": "app"},
		},
		{
			name: "IPv6 addresses are normalized to their canonical form",
			containers: []DockerContainer{
				{Name: "v6", IPAddresses: []string{"2001:0db8:0000:0000:0000:0000:0000:0001"}},
			},
			want: map[string]string{"2001:db8::1": "v6"},
		},
		{
			name: "unnamed containers and unparseable/empty addresses are skipped",
			containers: []DockerContainer{
				{Name: "", IPAddresses: []string{"172.17.0.9"}},
				{Name: "broken", IPAddresses: []string{"", "not-an-ip", "0.0.0.0"}},
				{Name: "ok", IPAddresses: []string{"172.17.0.5"}},
			},
			want: map[string]string{"172.17.0.5": "ok"},
		},
		{
			name: "a duplicate address keeps the first container rather than guessing",
			containers: []DockerContainer{
				{Name: "first", IPAddresses: []string{"10.5.0.2"}},
				{Name: "second", IPAddresses: []string{"10.5.0.2"}},
			},
			want: map[string]string{"10.5.0.2": "first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildContainerIPIndex(tt.containers)
			if len(got) != len(tt.want) {
				t.Fatalf("index size = %d (%v), want %d (%v)", len(got), got, len(tt.want), tt.want)
			}
			for ip, name := range tt.want {
				if got[ip] != name {
					t.Errorf("index[%q] = %q, want %q", ip, got[ip], name)
				}
			}
		})
	}
}

func TestContainerIPIndexPublishAndExpiry(t *testing.T) {
	now := time.Now()
	origClock := containerIPsNow
	t.Cleanup(func() {
		containerIPsNow = origClock
		containerIPMu.Lock()
		containerIPs, containerIPsAt = nil, time.Time{}
		containerIPMu.Unlock()
	})
	containerIPsNow = func() time.Time { return now }

	if got := ContainerIPIndex(); len(got) != 0 {
		t.Fatalf("expected an empty index before any publish, got %v", got)
	}

	publishContainerIPs([]DockerContainer{{Name: "redis", IPAddresses: []string{"172.20.0.3"}}})
	if got := ContainerIPIndex(); got["172.20.0.3"] != "redis" {
		t.Fatalf("after publish, index = %v, want redis at 172.20.0.3", got)
	}

	// Still fresh just inside the TTL.
	now = now.Add(containerIPTTL - time.Second)
	if got := ContainerIPIndex(); got["172.20.0.3"] != "redis" {
		t.Fatalf("index expired early: %v", got)
	}

	// Past the TTL the snapshot must stop being used, so a stopped Docker
	// daemon can't pin stale container IPs into the flow collector forever.
	now = now.Add(2 * time.Second)
	if got := ContainerIPIndex(); len(got) != 0 {
		t.Fatalf("expected the index to expire past the TTL, got %v", got)
	}
}

func TestContainerIPIndexReturnsACopy(t *testing.T) {
	t.Cleanup(func() {
		containerIPMu.Lock()
		containerIPs, containerIPsAt = nil, time.Time{}
		containerIPMu.Unlock()
	})

	publishContainerIPs([]DockerContainer{{Name: "api", IPAddresses: []string{"172.21.0.2"}}})
	got := ContainerIPIndex()
	got["172.21.0.2"] = "tampered"
	got["172.21.0.99"] = "injected"

	fresh := ContainerIPIndex()
	if fresh["172.21.0.2"] != "api" {
		t.Errorf("caller mutation leaked into the shared index: %v", fresh)
	}
	if _, ok := fresh["172.21.0.99"]; ok {
		t.Errorf("caller insertion leaked into the shared index: %v", fresh)
	}
}
