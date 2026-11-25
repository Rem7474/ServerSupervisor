package alerts

import (
	"sort"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

func TestIsAgentTargetedRule(t *testing.T) {
	for metric, want := range map[string]bool{
		"cpu":                    true,
		"docker_container_state": false,
	} {
		if got := isAgentTargetedRule(models.AlertRule{Metric: metric}); got != want {
			t.Errorf("isAgentTargetedRule(%q) = %v, want %v", metric, got, want)
		}
	}
}

func TestOutOfScopeIncidents(t *testing.T) {
	rule, other := int64(1), int64(2)
	open := map[string]*models.AlertIncident{
		"a": {ID: 10, RuleID: &rule, HostID: "web1"}, // still targeted
		"b": {ID: 11, RuleID: &rule, HostID: "db1"},  // left the scope
		"c": {ID: 12, RuleID: &other, HostID: "db1"}, // another rule
		"d": {ID: 13, RuleID: nil, HostID: "db1"},    // no rule
		"e": nil,
	}
	got := outOfScopeIncidents(rule, open, map[string]struct{}{"web1": {}})
	ids := []int64{}
	for _, inc := range got {
		ids = append(ids, inc.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != 1 || ids[0] != 11 {
		t.Fatalf("out of scope = %v, want [11]", ids)
	}
	if len(outOfScopeIncidents(rule, open, map[string]struct{}{"web1": {}, "db1": {}})) != 0 {
		t.Error("incident on an evaluated host must stay open")
	}
}
