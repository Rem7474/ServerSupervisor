package database_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func createCPURule(t *testing.T, db *database.DB, rule *models.AlertRule) {
	t.Helper()
	warn := 80.0
	rule.Metric, rule.Operator, rule.ThresholdWarn, rule.Enabled = "cpu", ">", &warn, true
	if rule.SourceType == "" {
		rule.SourceType = "agent"
	}
	if err := db.CreateAlertRule(context.Background(), rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}
}

func targetOf(t *testing.T, db *database.DB, id int64) string {
	t.Helper()
	var target string
	if err := db.QueryRow(context.Background(), `SELECT target_type FROM alert_rules WHERE id = $1`, id).Scan(&target); err != nil {
		t.Fatalf("read target: %v", err)
	}
	return target
}

// Migration 099 keeps the exact coverage of pre-existing agent rules: a rule
// with a host_id targets that host, one without targets every host; Docker
// and other non-agent rules are left untargeted.
func TestMigration099_BackfillsLegacyTargets(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "legacy-host"
	withHost := &models.AlertRule{HostID: &hostID}
	allHosts := &models.AlertRule{}
	docker := &models.AlertRule{SourceType: "docker"}
	for _, r := range []*models.AlertRule{withHost, allHosts, docker} {
		createCPURule(t, db, r)
	}
	// Simulate rows written before the migration existed.
	if _, err := db.Exec(ctx, `UPDATE alert_rules SET target_type = ''`); err != nil {
		t.Fatal(err)
	}

	migration, err := os.ReadFile("migrations/099_host_groups.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("re-running migration 099 must be idempotent: %v", err)
	}

	for _, tc := range []struct {
		rule *models.AlertRule
		want string
	}{{withHost, "host"}, {allHosts, "all"}, {docker, ""}} {
		if got := targetOf(t, db, tc.rule.ID); got != tc.want {
			t.Errorf("rule %d target_type = %q, want %q", tc.rule.ID, got, tc.want)
		}
	}
}

func TestHostGroups_RoundTripAndRuleReference(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	g := &models.HostGroup{Name: "Prod DB", MatchMode: models.HostGroupMatchAny, Tags: []string{"db", "prod"}, OSContains: "debian"}
	if err := db.CreateHostGroup(ctx, g); err != nil {
		t.Fatalf("create: %v", err)
	}
	if g.ID == "" || g.CreatedAt.IsZero() {
		t.Fatalf("id/timestamps not filled: %+v", g)
	}
	got, err := db.GetHostGroupByID(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Prod DB" || got.MatchMode != "any" || !reflect.DeepEqual(got.Tags, []string{"db", "prod"}) || got.OSContains != "debian" {
		t.Errorf("round trip = %+v", got)
	}

	g.Name, g.Tags = "Prod", []string{"prod"}
	if err := db.UpdateHostGroup(ctx, g); err != nil {
		t.Fatalf("update: %v", err)
	}
	list, err := db.ListHostGroups(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "Prod" {
		t.Fatalf("list = %+v, %v", list, err)
	}

	if _, err := db.GetHostGroupByID(ctx, "not-a-uuid"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("non-uuid id: err = %v, want ErrNoRows", err)
	}
	if err := db.UpdateHostGroup(ctx, &models.HostGroup{ID: "00000000-0000-0000-0000-000000000000", Name: "x", MatchMode: "all"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("update unknown: err = %v, want ErrNoRows", err)
	}

	// A group-targeted rule round-trips its target and blocks deletion.
	rule := &models.AlertRule{TargetType: models.AlertTargetGroup, HostGroupID: &g.ID}
	createCPURule(t, db, rule)
	n, err := db.CountAlertRulesForHostGroup(ctx, g.ID)
	if err != nil || n != 1 {
		t.Fatalf("count = %d, %v", n, err)
	}
	rules, err := db.GetAlertRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range rules {
		if r.ID == rule.ID {
			found = true
			if r.TargetType != "group" || r.HostGroupID == nil || *r.HostGroupID != g.ID || r.TargetTags != nil {
				t.Errorf("rule target = %q %v %v", r.TargetType, r.HostGroupID, r.TargetTags)
			}
		}
	}
	if !found {
		t.Fatal("rule not listed")
	}
	if err := db.DeleteHostGroup(ctx, g.ID); err == nil {
		t.Error("deleting a group still targeted by a rule must fail (FK restrict)")
	}

	tagRule := &models.AlertRule{TargetType: models.AlertTargetTag, TargetTags: []string{"web", "edge"}}
	createCPURule(t, db, tagRule)
	rules, _ = db.GetAlertRules(ctx)
	for _, r := range rules {
		if r.ID == tagRule.ID && !reflect.DeepEqual(r.TargetTags, []string{"web", "edge"}) {
			t.Errorf("tag rule tags = %v", r.TargetTags)
		}
	}

	if _, err := db.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, rule.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteHostGroup(ctx, g.ID); err != nil {
		t.Fatalf("delete unused group: %v", err)
	}
	if _, err := db.GetHostGroupByID(ctx, g.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("group still present after delete: %v", err)
	}
}

// Group ids are compared as UUIDs: any spelling of the id names the group,
// and the foreign key refusal comes back as models.ErrHostGroupInUse.
func TestHostGroups_UUIDLookupAndInUseError(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	g := &models.HostGroup{Name: "Web", MatchMode: models.HostGroupMatchAll, Tags: []string{"web"}}
	if err := db.CreateHostGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	upper := strings.ToUpper(g.ID)
	got, err := db.GetHostGroupByID(ctx, upper)
	if err != nil || got.ID != g.ID {
		t.Fatalf("upper-case lookup = %+v, %v", got, err)
	}
	if err := db.UpdateHostGroup(ctx, &models.HostGroup{ID: upper, Name: "Web 2", MatchMode: "all", Tags: []string{"web"}}); err != nil {
		t.Fatalf("upper-case update: %v", err)
	}
	if err := db.UpdateHostGroup(ctx, &models.HostGroup{ID: "nope", Name: "x", MatchMode: "all"}); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("non-uuid update: err = %v, want ErrNoRows", err)
	}

	rule := &models.AlertRule{TargetType: models.AlertTargetGroup, HostGroupID: &g.ID}
	createCPURule(t, db, rule)
	if n, err := db.CountAlertRulesForHostGroup(ctx, upper); err != nil || n != 1 {
		t.Fatalf("upper-case count = %d, %v", n, err)
	}
	if n, err := db.CountAlertRulesForHostGroup(ctx, "nope"); err != nil || n != 0 {
		t.Fatalf("non-uuid count = %d, %v", n, err)
	}
	if err := db.DeleteHostGroup(ctx, upper); !errors.Is(err, models.ErrHostGroupInUse) {
		t.Fatalf("delete of a targeted group: err = %v, want ErrHostGroupInUse", err)
	}
	if err := db.DeleteHostGroup(ctx, "nope"); err != nil {
		t.Fatalf("non-uuid delete: %v", err)
	}
}

// Migration 103: target_type only takes the values the engine handles, and a
// group target always names its group.
func TestAlertRules_TargetTypeCheck(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	rule := &models.AlertRule{TargetType: models.AlertTargetAll}
	createCPURule(t, db, rule)
	if _, err := db.Exec(ctx, `UPDATE alert_rules SET target_type = 'everywhere' WHERE id = $1`, rule.ID); err == nil {
		t.Error("unknown target_type accepted")
	}
	if _, err := db.Exec(ctx, `UPDATE alert_rules SET target_type = 'group', host_group_id = NULL WHERE id = $1`, rule.ID); err == nil {
		t.Error("group target without host_group_id accepted")
	}
	if _, err := db.Exec(ctx, `UPDATE alert_rules SET target_type = 'tag' WHERE id = $1`, rule.ID); err != nil {
		t.Errorf("valid target_type refused: %v", err)
	}
}
