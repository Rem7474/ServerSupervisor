package handlers

import (
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

func TestAlertHostScope_AllowsHost(t *testing.T) {
	t.Run("unrestricted allows anything, including empty host", func(t *testing.T) {
		scope := alertHostScope{unrestricted: true}
		if !scope.allowsHost("host-1") {
			t.Error("expected unrestricted scope to allow host-1")
		}
		if !scope.allowsHost("") {
			t.Error("expected unrestricted scope to allow empty host id")
		}
	})

	t.Run("restricted allows only granted hosts", func(t *testing.T) {
		scope := alertHostScope{hosts: map[string]bool{"host-1": true}}
		if !scope.allowsHost("host-1") {
			t.Error("expected host-1 to be allowed")
		}
		if scope.allowsHost("host-2") {
			t.Error("expected host-2 to be denied")
		}
		if scope.allowsHost("") {
			t.Error("expected empty (unresolvable) host id to be denied when restricted")
		}
	})
}

func TestResolvableHostID(t *testing.T) {
	cases := []struct {
		name       string
		hostID     string
		linkHostID string
		want       string
	}{
		{"link host id wins over a synthetic host id", "docker:container:abc", "real-host-1", "real-host-1"},
		{"plain host id passes through", "real-host-1", "", "real-host-1"},
		{"unresolved docker prefix hides", "docker:compose:x", "", ""},
		{"unresolved proxmox prefix hides", "proxmox:node:x", "", ""},
		{"unresolved synthetic prefix hides", "synthetic:probe-1", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolvableHostID(tc.hostID, tc.linkHostID); got != tc.want {
				t.Errorf("resolvableHostID(%q, %q) = %q, want %q", tc.hostID, tc.linkHostID, got, tc.want)
			}
		})
	}
}

func TestRuleHostID(t *testing.T) {
	hostID := "host-1"

	t.Run("docker rule resolves via DockerScope.HostID", func(t *testing.T) {
		rule := models.AlertRule{SourceType: models.AlertSourceDocker, DockerScope: &models.DockerMetricScope{HostID: hostID}}
		if got := ruleHostID(rule); got != hostID {
			t.Errorf("got %q, want %q", got, hostID)
		}
	})

	t.Run("docker rule with nil scope resolves to empty", func(t *testing.T) {
		rule := models.AlertRule{SourceType: models.AlertSourceDocker}
		if got := ruleHostID(rule); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("agent rule resolves via HostID", func(t *testing.T) {
		rule := models.AlertRule{SourceType: models.AlertSourceAgent, HostID: &hostID}
		if got := ruleHostID(rule); got != hostID {
			t.Errorf("got %q, want %q", got, hostID)
		}
	})

	t.Run("fleet-wide agent rule (nil HostID) resolves to empty", func(t *testing.T) {
		rule := models.AlertRule{SourceType: models.AlertSourceAgent}
		if got := ruleHostID(rule); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("proxmox rule has no resolvable host", func(t *testing.T) {
		rule := models.AlertRule{SourceType: models.AlertSourceProxmox}
		if got := ruleHostID(rule); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestFilterAlertRulesByScope(t *testing.T) {
	host1, host2 := "host-1", "host-2"
	rules := []models.AlertRule{
		{ID: 1, SourceType: models.AlertSourceAgent, HostID: &host1},
		{ID: 2, SourceType: models.AlertSourceAgent, HostID: &host2},
		{ID: 3, SourceType: models.AlertSourceDocker, DockerScope: &models.DockerMetricScope{HostID: host1}},
		{ID: 4, SourceType: models.AlertSourceProxmox},
	}

	t.Run("unrestricted returns everything unchanged", func(t *testing.T) {
		out := filterAlertRulesByScope(rules, alertHostScope{unrestricted: true})
		if len(out) != len(rules) {
			t.Fatalf("got %d rules, want %d", len(out), len(rules))
		}
	})

	t.Run("restricted keeps only rules scoped to a granted host", func(t *testing.T) {
		scope := alertHostScope{hosts: map[string]bool{host1: true}}
		out := filterAlertRulesByScope(rules, scope)
		if len(out) != 2 {
			t.Fatalf("got %d rules, want 2 (ids 1 and 3)", len(out))
		}
		for _, r := range out {
			if r.ID != 1 && r.ID != 3 {
				t.Errorf("unexpected rule id %d leaked through the scope filter", r.ID)
			}
		}
	})

	t.Run("restricted with no matching rules returns an empty, non-nil slice", func(t *testing.T) {
		scope := alertHostScope{hosts: map[string]bool{"host-nowhere": true}}
		out := filterAlertRulesByScope(rules, scope)
		if out == nil {
			t.Error("expected a non-nil empty slice")
		}
		if len(out) != 0 {
			t.Errorf("got %d rules, want 0", len(out))
		}
	})
}

func TestFilterAlertIncidentsByScope(t *testing.T) {
	incidents := []models.AlertIncident{
		{ID: 1, HostID: "host-1"},
		{ID: 2, HostID: "host-2"},
		{ID: 3, HostID: "docker:container:abc", LinkHostID: "host-1"},
		{ID: 4, HostID: "proxmox:node:xyz"},
	}

	t.Run("unrestricted returns everything unchanged", func(t *testing.T) {
		out := filterAlertIncidentsByScope(incidents, alertHostScope{unrestricted: true})
		if len(out) != len(incidents) {
			t.Fatalf("got %d incidents, want %d", len(out), len(incidents))
		}
	})

	t.Run("restricted keeps direct and docker-resolved host matches only", func(t *testing.T) {
		scope := alertHostScope{hosts: map[string]bool{"host-1": true}}
		out := filterAlertIncidentsByScope(incidents, scope)
		if len(out) != 2 {
			t.Fatalf("got %d incidents, want 2 (ids 1 and 3)", len(out))
		}
		for _, inc := range out {
			if inc.ID != 1 && inc.ID != 3 {
				t.Errorf("unexpected incident id %d leaked through the scope filter", inc.ID)
			}
		}
	})
}

func TestFilterNotificationsByScope(t *testing.T) {
	items := []models.NotificationItem{
		{ID: "1", HostID: "host-1"},
		{ID: "2", HostID: "host-2"},
		{ID: "3", HostID: "docker:compose:host-1:proj", LinkHostID: "host-1"},
		{ID: "4", HostID: ""}, // global release-tracker notification
	}

	t.Run("unrestricted returns everything unchanged", func(t *testing.T) {
		out := filterNotificationsByScope(items, alertHostScope{unrestricted: true})
		if len(out) != len(items) {
			t.Fatalf("got %d items, want %d", len(out), len(items))
		}
	})

	t.Run("restricted hides other hosts and unscoped items", func(t *testing.T) {
		scope := alertHostScope{hosts: map[string]bool{"host-1": true}}
		out := filterNotificationsByScope(items, scope)
		if len(out) != 2 {
			t.Fatalf("got %d items, want 2 (ids 1 and 3)", len(out))
		}
		for _, it := range out {
			if it.ID != "1" && it.ID != "3" {
				t.Errorf("unexpected notification id %v leaked through the scope filter", it.ID)
			}
		}
	})
}
