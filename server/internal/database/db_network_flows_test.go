package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

const testNetworkFlowsHostID = "host-network-flows-test"

func registerNetworkFlowsHost(t *testing.T, db *database.DB, ctx context.Context) {
	t.Helper()
	if err := db.RegisterHost(ctx, &models.Host{
		ID: testNetworkFlowsHostID, Name: "flows-test", Hostname: "flows.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}
}

// TestInsertNetworkFlowMetrics_RoundTrip guards the core write/read path: a
// report cycle's top talkers plus its "others" rollup must come back out of
// GetLatestNetworkFlowMetrics with the others row sorted last (is_others ASC)
// and talkers ordered busiest-first.
func TestInsertNetworkFlowMetrics_RoundTrip(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	registerNetworkFlowsHost(t, db, ctx)

	report := &models.NetworkFlowsReport{
		Available: true,
		TopTalkers: []models.NetworkFlowTalker{
			{RemoteIP: "1.2.3.4", RemotePort: 443, Protocol: "tcp", Direction: "outbound", ProcessName: "nginx", PID: 42, RxBytes: 100, TxBytes: 5000, Packets: 12, Connections: 2},
			{RemoteIP: "5.6.7.8", RemotePort: 22, Protocol: "tcp", Direction: "inbound", RxBytes: 900, TxBytes: 900, Packets: 30, Connections: 1},
		},
		Others:      &models.NetworkFlowBucket{Connections: 40, RxBytes: 1000, TxBytes: 2000},
		TotalFlows:  42,
		CollectedAt: time.Now().Truncate(time.Second),
	}

	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, report); err != nil {
		t.Fatalf("InsertNetworkFlowMetrics: %v", err)
	}

	got, err := db.GetLatestNetworkFlowMetrics(ctx, testNetworkFlowsHostID)
	if err != nil {
		t.Fatalf("GetLatestNetworkFlowMetrics: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rows (2 talkers + others), got %d", len(got))
	}
	// Busiest talker (rx+tx=5100) first, then the second (rx+tx=1800), others last.
	if got[0].RemoteIP != "1.2.3.4" || got[0].ProcessName != "nginx" || got[0].PID != 42 {
		t.Errorf("expected busiest talker first with process attribution, got %+v", got[0])
	}
	if got[1].RemoteIP != "5.6.7.8" {
		t.Errorf("expected second talker second, got %+v", got[1])
	}
	if !got[2].IsOthers || got[2].Connections != 40 || got[2].RxBytes != 1000 {
		t.Errorf("expected others rollup last, got %+v", got[2])
	}
}

// TestInsertNetworkFlowMetrics_UnavailableIsNoop guards the degraded-collection
// path (conntrack absent/disabled): the agent still sends a report each cycle
// with Available=false, and it must never write rows.
func TestInsertNetworkFlowMetrics_UnavailableIsNoop(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	registerNetworkFlowsHost(t, db, ctx)

	report := &models.NetworkFlowsReport{Available: false, Reason: "nf_conntrack not loaded"}
	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, report); err != nil {
		t.Fatalf("InsertNetworkFlowMetrics: %v", err)
	}

	got, err := db.GetLatestNetworkFlowMetrics(ctx, testNetworkFlowsHostID)
	if err != nil {
		t.Fatalf("GetLatestNetworkFlowMetrics: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no rows written for an unavailable report, got %d", len(got))
	}
}

// TestGetNetworkFlowsSummary_SumsAcrossTalkers guards the SUM()-per-bucket
// aggregation (not MAX-MIN, since rx_bytes/tx_bytes are already per-cycle
// deltas): two talkers plus an others bucket in the same cycle must add up.
func TestGetNetworkFlowsSummary_SumsAcrossTalkers(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	registerNetworkFlowsHost(t, db, ctx)

	report := &models.NetworkFlowsReport{
		Available: true,
		TopTalkers: []models.NetworkFlowTalker{
			{RemoteIP: "1.2.3.4", RemotePort: 443, Protocol: "tcp", Direction: "outbound", RxBytes: 100, TxBytes: 200},
			{RemoteIP: "5.6.7.8", RemotePort: 22, Protocol: "tcp", Direction: "inbound", RxBytes: 300, TxBytes: 400},
		},
		Others:      &models.NetworkFlowBucket{RxBytes: 50, TxBytes: 75},
		CollectedAt: time.Now(),
	}
	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, report); err != nil {
		t.Fatalf("InsertNetworkFlowMetrics: %v", err)
	}

	points, err := db.GetNetworkFlowsSummary(ctx, testNetworkFlowsHostID, time.Now().Add(-1*time.Hour), time.Time{})
	if err != nil {
		t.Fatalf("GetNetworkFlowsSummary: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 bucket, got %d: %+v", len(points), points)
	}
	if points[0].RxBytes != 450 || points[0].TxBytes != 675 {
		t.Errorf("expected summed rx=450/tx=675, got rx=%d/tx=%d", points[0].RxBytes, points[0].TxBytes)
	}
}

// TestGetNetworkFlowsHistory_FiltersToOneTalker guards the per-talker
// drill-down: it must only sum the matching (remote_ip, remote_port, protocol)
// rows, not every talker for the host, and must exclude the "others" bucket
// (which has no single identity to filter on).
func TestGetNetworkFlowsHistory_FiltersToOneTalker(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	registerNetworkFlowsHost(t, db, ctx)

	report := &models.NetworkFlowsReport{
		Available: true,
		TopTalkers: []models.NetworkFlowTalker{
			{RemoteIP: "1.2.3.4", RemotePort: 443, Protocol: "tcp", Direction: "outbound", RxBytes: 111, TxBytes: 222},
			{RemoteIP: "9.9.9.9", RemotePort: 443, Protocol: "tcp", Direction: "outbound", RxBytes: 999, TxBytes: 999},
		},
		Others:      &models.NetworkFlowBucket{RxBytes: 50, TxBytes: 75},
		CollectedAt: time.Now(),
	}
	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, report); err != nil {
		t.Fatalf("InsertNetworkFlowMetrics: %v", err)
	}

	points, err := db.GetNetworkFlowsHistory(ctx, testNetworkFlowsHostID, "1.2.3.4", 443, "tcp", time.Now().Add(-1*time.Hour), time.Time{})
	if err != nil {
		t.Fatalf("GetNetworkFlowsHistory: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 bucket, got %d: %+v", len(points), points)
	}
	if points[0].RxBytes != 111 || points[0].TxBytes != 222 {
		t.Errorf("expected only the matching talker's bytes (rx=111/tx=222), got rx=%d/tx=%d", points[0].RxBytes, points[0].TxBytes)
	}
}

// TestCleanOldNetworkFlowMetrics_DeletesOnlyOldRows guards the RGPD-motivated
// retention job: a row past the cutoff must be deleted, a fresh one kept.
func TestCleanOldNetworkFlowMetrics_DeletesOnlyOldRows(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	registerNetworkFlowsHost(t, db, ctx)

	old := &models.NetworkFlowsReport{
		Available:   true,
		TopTalkers:  []models.NetworkFlowTalker{{RemoteIP: "1.1.1.1", RemotePort: 80, Protocol: "tcp", Direction: "outbound", RxBytes: 1, TxBytes: 1}},
		CollectedAt: time.Now().Add(-30 * 24 * time.Hour),
	}
	fresh := &models.NetworkFlowsReport{
		Available:   true,
		TopTalkers:  []models.NetworkFlowTalker{{RemoteIP: "2.2.2.2", RemotePort: 80, Protocol: "tcp", Direction: "outbound", RxBytes: 1, TxBytes: 1}},
		CollectedAt: time.Now(),
	}
	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, old); err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, fresh); err != nil {
		t.Fatalf("insert fresh: %v", err)
	}

	deleted, err := db.CleanOldNetworkFlowMetrics(ctx, 14)
	if err != nil {
		t.Fatalf("CleanOldNetworkFlowMetrics: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted row, got %d", deleted)
	}

	got, err := db.GetLatestNetworkFlowMetrics(ctx, testNetworkFlowsHostID)
	if err != nil {
		t.Fatalf("GetLatestNetworkFlowMetrics: %v", err)
	}
	if len(got) != 1 || got[0].RemoteIP != "2.2.2.2" {
		t.Errorf("expected only the fresh row to remain, got %+v", got)
	}
}

// TestGetNetworkFlowsSummary_UntilExcludesPointsPastTheUpperBound guards the
// custom-range support: a non-zero until must exclude cycles reported after
// it, not just before since — the pre-existing preset-only path only ever
// exercised the lower bound (until always zero/open-ended).
func TestGetNetworkFlowsSummary_UntilExcludesPointsPastTheUpperBound(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	registerNetworkFlowsHost(t, db, ctx)

	now := time.Now()
	since := now.Add(-3 * time.Hour)
	until := now.Add(-1 * time.Hour)

	inWindow := &models.NetworkFlowsReport{
		Available:   true,
		TopTalkers:  []models.NetworkFlowTalker{{RemoteIP: "1.1.1.1", RemotePort: 80, Protocol: "tcp", Direction: "outbound", RxBytes: 10, TxBytes: 20}},
		CollectedAt: now.Add(-2 * time.Hour),
	}
	afterWindow := &models.NetworkFlowsReport{
		Available:   true,
		TopTalkers:  []models.NetworkFlowTalker{{RemoteIP: "2.2.2.2", RemotePort: 80, Protocol: "tcp", Direction: "outbound", RxBytes: 1000, TxBytes: 1000}},
		CollectedAt: now.Add(-30 * time.Minute),
	}
	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, inWindow); err != nil {
		t.Fatalf("insert in-window: %v", err)
	}
	if err := db.InsertNetworkFlowMetrics(ctx, testNetworkFlowsHostID, afterWindow); err != nil {
		t.Fatalf("insert after-window: %v", err)
	}

	points, err := db.GetNetworkFlowsSummary(ctx, testNetworkFlowsHostID, since, until)
	if err != nil {
		t.Fatalf("GetNetworkFlowsSummary: %v", err)
	}
	var totalRx uint64
	for _, p := range points {
		totalRx += p.RxBytes
	}
	if totalRx != 10 {
		t.Errorf("expected only the in-window cycle's rx=10, got total rx=%d across %+v", totalRx, points)
	}

	openEnded, err := db.GetNetworkFlowsSummary(ctx, testNetworkFlowsHostID, since, time.Time{})
	if err != nil {
		t.Fatalf("GetNetworkFlowsSummary (open-ended): %v", err)
	}
	var openTotalRx uint64
	for _, p := range openEnded {
		openTotalRx += p.RxBytes
	}
	if openTotalRx != 1010 {
		t.Errorf("expected both cycles with an open-ended until, got total rx=%d across %+v", openTotalRx, openEnded)
	}
}
