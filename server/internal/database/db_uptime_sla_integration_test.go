package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func newSLAProbe(t *testing.T, db *database.DB, name string, slo float64) *models.UptimeProbe {
	t.Helper()
	p, err := db.CreateUptimeProbe(context.Background(), models.UptimeProbe{
		Name: name, Type: "tcp", Target: name + ".internal:443", IntervalSec: 60, TimeoutSec: 10, Enabled: true, SLOTarget: slo,
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	return p
}

func recordCheck(t *testing.T, db *database.DB, probeID string, at time.Time, ok bool) {
	t.Helper()
	if err := db.RecordUptimeProbeResult(context.Background(), models.UptimeProbeResult{
		ProbeID: probeID, CheckedAt: at, Success: ok, LatencyMs: 12,
	}); err != nil {
		t.Fatalf("record result: %v", err)
	}
}

func TestUptimeProbe_SLOTargetPersistence(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	p := newSLAProbe(t, db, "unset", 0)
	if p.SLOTarget != models.DefaultSLOTarget {
		t.Fatalf("unset objective must default to %v, got %v", models.DefaultSLOTarget, p.SLOTarget)
	}
	p.SLOTarget = 99.95
	if err := db.UpdateUptimeProbe(ctx, *p); err != nil {
		t.Fatal(err)
	}
	p.SLOTarget = 0
	p.Name = "renamed"
	if err := db.UpdateUptimeProbe(ctx, *p); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetUptimeProbe(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SLOTarget != 99.95 || got.Name != "renamed" {
		t.Fatalf("a zero objective on update must keep the stored one, got %v (%s)", got.SLOTarget, got.Name)
	}
}

func TestUptimeDailyRollupAndCounts(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	id := newSLAProbe(t, db, "api", 99.9).ID

	today := time.Now().UTC().Truncate(24 * time.Hour)
	threeDaysAgo := today.AddDate(0, 0, -3).Add(6 * time.Hour)
	recordCheck(t, db, id, threeDaysAgo, true)
	recordCheck(t, db, id, threeDaysAgo.Add(time.Minute), false)
	recordCheck(t, db, id, threeDaysAgo.Add(2*time.Minute), false)
	recordCheck(t, db, id, threeDaysAgo.Add(3*time.Minute), true)
	recordCheck(t, db, id, today.Add(time.Minute), false)

	// Before the rollup, days older than yesterday are not visible.
	counts, err := db.GetUptimeDailyCounts(ctx, []string{id}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1 || counts[0].TotalChecks != 1 || counts[0].UpChecks != 0 {
		t.Fatalf("before rollup only today's raw counts are expected, got %+v", counts)
	}

	if n, err := db.RollupUptimeDaily(ctx, 29); err != nil || n != 1 {
		t.Fatalf("rollup wrote %d rows (%v), want 1", n, err)
	}
	if _, err := db.RollupUptimeDaily(ctx, 29); err != nil {
		t.Fatalf("rollup must be idempotent: %v", err)
	}
	// The rolled-up day survives the raw retention purge.
	if _, err := db.Exec(ctx, `DELETE FROM uptime_probe_results WHERE checked_at < $1`, today); err != nil {
		t.Fatal(err)
	}
	counts, err = db.GetUptimeDailyCounts(ctx, []string{id}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 || counts[0].TotalChecks != 4 || counts[0].UpChecks != 2 || !counts[0].Day.Equal(today.AddDate(0, 0, -3)) {
		t.Fatalf("unexpected counts after rollup: %+v", counts)
	}
	if short, _ := db.GetUptimeDailyCounts(ctx, []string{id}, 2); len(short) != 1 {
		t.Fatalf("a 2-day window only holds today, got %+v", short)
	}
	for _, tc := range []struct {
		ids  []string
		days int
	}{{nil, 7}, {[]string{id}, 0}} {
		if c, err := db.GetUptimeDailyCounts(ctx, tc.ids, tc.days); err != nil || len(c) != 0 {
			t.Fatalf("empty input must return no counts, got %+v (%v)", c, err)
		}
	}

	if _, err := db.Exec(ctx, `INSERT INTO uptime_probe_daily (probe_id, day, total_checks, up_checks) VALUES ($1, $2, 1, 1)`, id, today.AddDate(0, 0, -500)); err != nil {
		t.Fatal(err)
	}
	if n, err := db.CleanupOldUptimeDaily(ctx, 400); err != nil || n != 1 {
		t.Fatalf("cleanup removed %d rows (%v), want 1", n, err)
	}
}

func TestListUptimeDownPeriods(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	id := newSLAProbe(t, db, "web", 99.9).ID
	base := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)

	recordCheck(t, db, id, base, true)
	recordCheck(t, db, id, base.Add(time.Minute), false)
	recordCheck(t, db, id, base.Add(2*time.Minute), false)
	recordCheck(t, db, id, base.Add(3*time.Minute), true)
	recordCheck(t, db, id, base.Add(time.Hour), false)

	periods, err := db.ListUptimeDownPeriods(ctx, []string{id}, base.Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(periods) != 2 {
		t.Fatalf("expected 2 down periods, got %+v", periods)
	}
	if periods[0].ResolvedAt != nil || !periods[0].StartedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("newest period must be the ongoing one, got %+v", periods[0])
	}
	if periods[1].ResolvedAt == nil || !periods[1].ResolvedAt.Equal(base.Add(3*time.Minute)) || !periods[1].StartedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("older period must end on the next success, got %+v", periods[1])
	}
	if limited, _ := db.ListUptimeDownPeriods(ctx, []string{id}, base.Add(-time.Hour), 1); len(limited) != 1 {
		t.Fatalf("limit must apply, got %d periods", len(limited))
	}
	if none, err := db.ListUptimeDownPeriods(ctx, nil, base, 10); err != nil || len(none) != 0 {
		t.Fatalf("no probe ids must return no periods, got %+v (%v)", none, err)
	}
}

func TestStatusPageConfigStorage(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	cfg, err := db.GetStatusPageConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled || len(cfg.Services) != 0 {
		t.Fatalf("status page must start disabled and empty, got %+v", cfg)
	}

	a := newSLAProbe(t, db, "a", 99.9)
	b := newSLAProbe(t, db, "b", 99.9)
	req := models.StatusPageConfigRequest{
		Enabled: true, Title: "Acme", Description: "Services",
		Services: []models.StatusPageService{{ProbeID: b.ID, PublicName: "Zeta"}, {ProbeID: a.ID, PublicName: "Alpha"}},
	}
	if err := db.ReplaceStatusPageConfig(ctx, req); err != nil {
		t.Fatal(err)
	}
	cfg, err = db.GetStatusPageConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.Title != "Acme" || len(cfg.Services) != 2 || cfg.Services[0].PublicName != "Zeta" {
		t.Fatalf("config must keep the admin's order, got %+v", cfg)
	}

	published, err := db.ListPublishedProbes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(published) != 2 || published[0].ProbeID != b.ID || !published[0].Enabled || published[1].PublicName != "Alpha" {
		t.Fatalf("unexpected published probes %+v", published)
	}

	// A blank public name violates the CHECK and rolls the whole replace back.
	bad := req
	bad.Title = "Changed"
	bad.Services = []models.StatusPageService{{ProbeID: a.ID, PublicName: "  "}}
	if err := db.ReplaceStatusPageConfig(ctx, bad); err == nil {
		t.Fatal("blank public name must be rejected")
	}
	if cfg, _ = db.GetStatusPageConfig(ctx); cfg.Title != "Acme" || len(cfg.Services) != 2 {
		t.Fatalf("failed replace must leave the previous config, got %+v", cfg)
	}

	// Deleting a probe unpublishes it.
	if err := db.DeleteUptimeProbe(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if published, _ = db.ListPublishedProbes(ctx); len(published) != 1 || published[0].ProbeID != a.ID {
		t.Fatalf("deleted probe must leave the page, got %+v", published)
	}
}
