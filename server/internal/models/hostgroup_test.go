package models

import (
	"reflect"
	"testing"
)

func hostIDs(hosts []Host) []string {
	ids := make([]string, 0, len(hosts))
	for _, h := range hosts {
		ids = append(ids, h.ID)
	}
	return ids
}

func strPtr(s string) *string { return &s }

var fleet = []Host{
	{ID: "web1", Tags: []string{"prod", "web"}, OS: "Debian GNU/Linux 12"},
	{ID: "web2", Tags: []string{"Prod", "web"}, OS: "Ubuntu 24.04"},
	{ID: "db1", Tags: []string{"prod", "db"}, OS: "Debian GNU/Linux 12"},
	{ID: "lab1", Tags: []string{"lab"}, OS: "Debian GNU/Linux 13"},
	{ID: "bare", Tags: nil, OS: "Alpine"},
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" prod ", "", "Prod", "web", "  "})
	if !reflect.DeepEqual(got, []string{"prod", "web"}) {
		t.Fatalf("NormalizeTags = %v, want [prod web]", got)
	}
	if got := NormalizeTags(nil); got == nil || len(got) != 0 {
		t.Fatalf("NormalizeTags(nil) = %#v, want empty non-nil slice", got)
	}
}

func TestHostGroupMatches(t *testing.T) {
	tests := []struct {
		name  string
		group HostGroup
		want  []string
	}{
		{"all tags required", HostGroup{MatchMode: HostGroupMatchAll, Tags: []string{"prod", "web"}}, []string{"web1", "web2"}},
		{"any tag is enough", HostGroup{MatchMode: HostGroupMatchAny, Tags: []string{"db", "lab"}}, []string{"db1", "lab1"}},
		{"empty match mode means all", HostGroup{Tags: []string{"prod", "db"}}, []string{"db1"}},
		{"tag match is case-insensitive", HostGroup{MatchMode: HostGroupMatchAll, Tags: []string{"PROD"}}, []string{"web1", "web2", "db1"}},
		{"OS filter alone", HostGroup{OSContains: "debian"}, []string{"web1", "db1", "lab1"}},
		{"OS filter narrows tags", HostGroup{MatchMode: HostGroupMatchAll, Tags: []string{"prod"}, OSContains: "Debian"}, []string{"web1", "db1"}},
		{"no criterion matches nothing", HostGroup{MatchMode: HostGroupMatchAny}, []string{}},
		{"blank tags count as no criterion", HostGroup{Tags: []string{" "}}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostIDs(tc.group.Members(fleet)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("members = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveAgentTargets(t *testing.T) {
	groups := map[string]HostGroup{
		"g-web": {ID: "g-web", MatchMode: HostGroupMatchAll, Tags: []string{"web"}},
	}
	tests := []struct {
		name string
		rule AlertRule
		want []string
	}{
		{"single host", AlertRule{TargetType: AlertTargetHost, HostID: strPtr("db1")}, []string{"db1"}},
		{"unknown host", AlertRule{TargetType: AlertTargetHost, HostID: strPtr("gone")}, []string{}},
		{"host target without id", AlertRule{TargetType: AlertTargetHost}, []string{}},
		{"every host", AlertRule{TargetType: AlertTargetAll}, []string{"web1", "web2", "db1", "lab1", "bare"}},
		{"group members", AlertRule{TargetType: AlertTargetGroup, HostGroupID: strPtr("g-web")}, []string{"web1", "web2"}},
		// A deleted/unknown group must never fall back to every host.
		{"unknown group covers nothing", AlertRule{TargetType: AlertTargetGroup, HostGroupID: strPtr("g-missing")}, []string{}},
		{"group target without id", AlertRule{TargetType: AlertTargetGroup}, []string{}},
		{"any of the tags", AlertRule{TargetType: AlertTargetTag, TargetTags: []string{"db", "LAB"}}, []string{"db1", "lab1"}},
		{"unknown target type covers nothing", AlertRule{TargetType: "bogus"}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostIDs(tc.rule.ResolveAgentTargets(fleet, groups)); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("targets = %v, want %v", got, tc.want)
			}
		})
	}
}

// A host tagged after the rule was saved is covered without editing the rule.
func TestResolveAgentTargets_NewlyTaggedHostIsCovered(t *testing.T) {
	rule := AlertRule{TargetType: AlertTargetTag, TargetTags: []string{"db"}}
	hosts := append([]Host{}, fleet...)
	if got := hostIDs(rule.ResolveAgentTargets(hosts, nil)); !reflect.DeepEqual(got, []string{"db1"}) {
		t.Fatalf("before tagging: %v", got)
	}
	hosts = append(hosts, Host{ID: "db2", Tags: []string{"db"}})
	if got := hostIDs(rule.ResolveAgentTargets(hosts, nil)); !reflect.DeepEqual(got, []string{"db1", "db2"}) {
		t.Fatalf("after tagging: %v", got)
	}
}

func TestNormalizeCompatibility_DerivesLegacyTarget(t *testing.T) {
	withHost := AlertRule{SourceType: AlertSourceAgent, HostID: strPtr("web1")}
	withHost.NormalizeCompatibility()
	if withHost.TargetType != AlertTargetHost {
		t.Errorf("rule with host_id: target_type = %q, want host", withHost.TargetType)
	}

	blankHost := AlertRule{SourceType: AlertSourceAgent, HostID: strPtr("  ")}
	blankHost.NormalizeCompatibility()
	if blankHost.TargetType != AlertTargetAll {
		t.Errorf("rule with blank host_id: target_type = %q, want all", blankHost.TargetType)
	}

	explicit := AlertRule{SourceType: AlertSourceAgent, TargetType: AlertTargetTag}
	explicit.NormalizeCompatibility()
	if explicit.TargetType != AlertTargetTag {
		t.Errorf("explicit target_type overwritten: %q", explicit.TargetType)
	}

	docker := AlertRule{Metric: "docker_container_state"}
	docker.NormalizeCompatibility()
	if docker.TargetType != "" {
		t.Errorf("docker rule got target_type %q, want empty", docker.TargetType)
	}
}

func TestAlertRuleValidate_AgentTargets(t *testing.T) {
	tests := []struct {
		name    string
		rule    AlertRule
		wantErr bool
		check   func(t *testing.T, r AlertRule)
	}{
		{
			name: "host target clears group and tags",
			rule: AlertRule{Metric: "cpu", TargetType: AlertTargetHost, HostID: strPtr("web1"), HostGroupID: strPtr("g"), TargetTags: []string{"x"}},
			check: func(t *testing.T, r AlertRule) {
				if r.HostGroupID != nil || r.TargetTags != nil {
					t.Errorf("stale group/tags kept: %v %v", r.HostGroupID, r.TargetTags)
				}
			},
		},
		{name: "host target needs a host", rule: AlertRule{Metric: "cpu", TargetType: AlertTargetHost, HostID: strPtr(" ")}, wantErr: true},
		{
			name: "legacy rule without host targets every host",
			rule: AlertRule{Metric: "cpu"},
			check: func(t *testing.T, r AlertRule) {
				if r.TargetType != AlertTargetAll || r.HostID != nil {
					t.Errorf("target = %q host=%v, want all/nil", r.TargetType, r.HostID)
				}
			},
		},
		{
			name: "group target clears host",
			rule: AlertRule{Metric: "cpu", TargetType: AlertTargetGroup, HostGroupID: strPtr("g"), HostID: strPtr("web1")},
			check: func(t *testing.T, r AlertRule) {
				if r.HostID != nil {
					t.Errorf("host_id kept on a group rule: %v", *r.HostID)
				}
			},
		},
		{name: "group target needs a group", rule: AlertRule{Metric: "cpu", TargetType: AlertTargetGroup}, wantErr: true},
		{
			name: "tag target normalizes tags",
			rule: AlertRule{Metric: "cpu", TargetType: AlertTargetTag, TargetTags: []string{" db ", "DB", ""}, HostGroupID: strPtr("g")},
			check: func(t *testing.T, r AlertRule) {
				if !reflect.DeepEqual(r.TargetTags, []string{"db"}) || r.HostGroupID != nil {
					t.Errorf("tags = %v group=%v", r.TargetTags, r.HostGroupID)
				}
			},
		},
		{name: "tag target needs a tag", rule: AlertRule{Metric: "cpu", TargetType: AlertTargetTag, TargetTags: []string{"  "}}, wantErr: true},
		{name: "unknown target type", rule: AlertRule{Metric: "cpu", TargetType: "everywhere"}, wantErr: true},
		{
			name: "docker rule drops agent target",
			rule: AlertRule{
				Metric: "docker_container_state", TargetType: AlertTargetTag, TargetTags: []string{"db"},
				DockerScope: &DockerMetricScope{HostID: "web1", ScopeMode: "host", CritStates: []string{"exited"}},
			},
			check: func(t *testing.T, r AlertRule) {
				if r.TargetType != "" || r.TargetTags != nil {
					t.Errorf("docker rule kept target %q %v", r.TargetType, r.TargetTags)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.rule
			err := r.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.check != nil {
				tc.check(t, r)
			}
		})
	}
}
