package alerts_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// A tag-targeted rule follows host tags dynamically: tagging a host makes it
// evaluated (incident opened), removing the tag resolves the incident even
// though the metric still breaches.
func TestEvaluateAlerts_TagTargetFollowsHostTags(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	for _, id := range []string{"tag-host-db", "tag-host-web"} {
		if err := db.RegisterHost(ctx, &models.Host{ID: id, Name: id, Hostname: id, Status: "online", LastSeen: time.Now()}); err != nil {
			t.Fatalf("register host: %v", err)
		}
		insertCPUMetric(t, db, id, 95, time.Now())
	}

	warn := 50.0
	rule := &models.AlertRule{
		SourceType: "agent", TargetType: models.AlertTargetTag, TargetTags: []string{"db"},
		Metric: "cpu", Operator: ">", ThresholdWarn: &warn, Enabled: true,
		Actions: models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	cfg, disp, pusher := &config.Config{}, dispatch.New(db), &stubPusher{}

	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)
	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, "tag-host-db"); err == nil {
		t.Fatal("untagged host must not be evaluated yet")
	}

	tags := []string{"DB"}
	if err := db.UpdateHost(ctx, "tag-host-db", &models.HostUpdate{Tags: &tags}); err != nil {
		t.Fatalf("tag host: %v", err)
	}
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)
	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, "tag-host-db"); err != nil {
		t.Fatalf("tagged host should have an open incident: %v", err)
	}
	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, "tag-host-web"); err == nil {
		t.Error("host without the tag got an incident")
	}

	none := []string{}
	if err := db.UpdateHost(ctx, "tag-host-db", &models.HostUpdate{Tags: &none}); err != nil {
		t.Fatalf("untag host: %v", err)
	}
	alerts.EvaluateAlerts(ctx, db, cfg, disp, pusher, nil)
	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, "tag-host-db"); err == nil {
		t.Error("incident must be resolved once the host leaves the rule's scope")
	}
}

// A group-targeted rule evaluates the group's current members.
func TestEvaluateAlerts_GroupTarget(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hosts := map[string][]string{"grp-prod": {"prod"}, "grp-lab": {"lab"}}
	for id, tags := range hosts {
		if err := db.RegisterHost(ctx, &models.Host{ID: id, Name: id, Hostname: id, Status: "online", LastSeen: time.Now()}); err != nil {
			t.Fatalf("register host: %v", err)
		}
		tags := tags
		if err := db.UpdateHost(ctx, id, &models.HostUpdate{Tags: &tags}); err != nil {
			t.Fatalf("tag host: %v", err)
		}
		insertCPUMetric(t, db, id, 95, time.Now())
	}
	group := &models.HostGroup{Name: "Prod", MatchMode: models.HostGroupMatchAll, Tags: []string{"prod"}}
	if err := db.CreateHostGroup(ctx, group); err != nil {
		t.Fatalf("create group: %v", err)
	}

	warn := 50.0
	rule := &models.AlertRule{
		SourceType: "agent", TargetType: models.AlertTargetGroup, HostGroupID: &group.ID,
		Metric: "cpu", Operator: ">", ThresholdWarn: &warn, Enabled: true,
		Actions: models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	alerts.EvaluateAlerts(ctx, db, &config.Config{}, dispatch.New(db), &stubPusher{}, nil)
	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, "grp-prod"); err != nil {
		t.Errorf("group member should have an incident: %v", err)
	}
	if _, err := db.GetOpenAlertIncident(ctx, rule.ID, "grp-lab"); err == nil {
		t.Error("non-member got an incident")
	}
}
