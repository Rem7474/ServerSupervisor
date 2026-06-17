package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// seedProxmoxConnNode inserts a minimal connection + node and returns their UUIDs.
func seedProxmoxConnNode(t *testing.T, db *database.DB) (connID, nodeID string) {
	t.Helper()
	ctx := context.Background()
	if err := db.QueryRow(ctx,
		`INSERT INTO proxmox_connections (name, api_url, token_id, token_secret)
		 VALUES ('test', 'https://pve.local', 'tok', 'sec') RETURNING id`).Scan(&connID); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO proxmox_nodes (connection_id, node_name, status)
		 VALUES ($1, 'pve1', 'online') RETURNING id`, connID).Scan(&nodeID); err != nil {
		t.Fatalf("seed node: %v", err)
	}
	return connID, nodeID
}

// TestProxmoxNodeMetricsSummary_CAGG verifies the node summary reads the
// continuous aggregate (≥5min bucket) and that the raw path serves finer buckets.
func TestProxmoxNodeMetricsSummary_CAGG(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	connID, nodeID := seedProxmoxConnNode(t, db)

	// Insert samples 20 minutes in the past so they fall outside the CAGG
	// end_offset (5 min) window and get materialized on refresh.
	// cpu_usage is a 0‒1 ratio; 0.5 → 50%. mem 500/1000 → 50%.
	for i := 0; i < 3; i++ {
		testutil.MustQuery(t, db,
			`INSERT INTO proxmox_node_metrics (node_id, connection_id, node_name, cpu_usage, mem_total, mem_used, timestamp)
			 VALUES ($1, $2, 'pve1', 0.5, 1000, 500, NOW() - INTERVAL '20 minutes')`,
			nodeID, connID)
	}
	testutil.MustQuery(t, db, `CALL refresh_continuous_aggregate('proxmox_node_metrics_5min', NULL, NULL)`)

	// CAGG path (bucket 5min).
	summary, err := db.GetProxmoxNodeMetricsSummary(ctx, 1, 5)
	if err != nil {
		t.Fatalf("node summary: %v", err)
	}
	assertProxmoxPct(t, "node CAGG", summary)

	byNode, err := db.GetProxmoxNodeMetricsSummaryByNode(ctx, nodeID, 1, 5)
	if err != nil {
		t.Fatalf("node-by-node summary: %v", err)
	}
	assertProxmoxPct(t, "node-by-node CAGG", byNode)

	// Raw path (bucket < 5min): must still return the same samples.
	raw, err := db.GetProxmoxNodeMetricsSummary(ctx, 1, 1)
	if err != nil {
		t.Fatalf("node summary raw: %v", err)
	}
	assertProxmoxPct(t, "node raw", raw)
}

// TestProxmoxGuestMetricsSummary_CAGG verifies the per-guest summary reads the CAGG.
func TestProxmoxGuestMetricsSummary_CAGG(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	connID, _ := seedProxmoxConnNode(t, db)

	var guestID string
	if err := db.QueryRow(ctx,
		`INSERT INTO proxmox_guests (connection_id, node_name, guest_type, vmid, name, status)
		 VALUES ($1, 'pve1', 'qemu', 100, 'vm100', 'running') RETURNING id`, connID).Scan(&guestID); err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	for i := 0; i < 3; i++ {
		testutil.MustQuery(t, db,
			`INSERT INTO proxmox_guest_metrics (guest_id, cpu_usage, mem_total, mem_used, timestamp)
			 VALUES ($1, 0.25, 2000, 500, NOW() - INTERVAL '20 minutes')`, guestID)
	}
	testutil.MustQuery(t, db, `CALL refresh_continuous_aggregate('proxmox_guest_metrics_5min', NULL, NULL)`)

	summary, err := db.GetProxmoxGuestMetricsSummary(ctx, guestID, 1, 5)
	if err != nil {
		t.Fatalf("guest summary: %v", err)
	}
	if len(summary) == 0 {
		t.Fatal("guest CAGG summary is empty")
	}
	for _, s := range summary {
		if s.CPUAvg < 24 || s.CPUAvg > 26 { // 0.25 ratio → 25%
			t.Errorf("guest cpu_avg = %.2f, want ~25", s.CPUAvg)
		}
		if s.MemoryAvg < 24 || s.MemoryAvg > 26 { // 500/2000 → 25%
			t.Errorf("guest mem_avg = %.2f, want ~25", s.MemoryAvg)
		}
	}
}

// TestMetricsSummary_RealTimeFreshness guards against the dashboard charts
// lagging behind the host-detail panel. A sample inserted < 5 min ago falls
// inside the CAGG end_offset window and is NOT materialized by a refresh, so it
// only surfaces in the summary if real-time aggregation is enabled on
// system_metrics_5min (materialized_only = false). Without it the dashboard
// would stop ~10-18 min short of "now" while the raw host panel stays current.
func TestMetricsSummary_RealTimeFreshness(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	const hostID = "host-fresh-cagg"
	if err := db.RegisterHost(ctx, &models.Host{ID: hostID, Name: "fresh", Hostname: "fresh.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	// Old sample (materialized on refresh) + a fresh one inside the end_offset.
	testutil.MustQuery(t, db,
		`INSERT INTO system_metrics (host_id, timestamp, cpu_usage_percent, memory_percent)
		 VALUES ($1, NOW() - INTERVAL '20 minutes', 50, 50)`, hostID)
	testutil.MustQuery(t, db, `CALL refresh_continuous_aggregate('system_metrics_5min', NULL, NULL)`)
	testutil.MustQuery(t, db,
		`INSERT INTO system_metrics (host_id, timestamp, cpu_usage_percent, memory_percent)
		 VALUES ($1, NOW() - INTERVAL '1 minute', 50, 50)`, hostID)

	// CAGG path (bucket 5min). The fresh, non-materialized sample must appear.
	summary, err := db.GetMetricsSummary(ctx, 1, 5)
	if err != nil {
		t.Fatalf("metrics summary: %v", err)
	}
	if len(summary) == 0 {
		t.Fatal("metrics summary is empty")
	}

	var latest time.Time
	for _, s := range summary {
		if s.Timestamp.After(latest) {
			latest = s.Timestamp
		}
	}
	if age := time.Since(latest); age > 6*time.Minute {
		t.Errorf("latest summary bucket is %s old, want < 6m — real-time aggregation not active", age)
	}
}

// TestDiskMetricsAggregated_CAGG verifies the hourly rollup reads disk_metrics_1h.
func TestDiskMetricsAggregated_CAGG(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	const hostID = "host-disk-cagg"
	if err := db.RegisterHost(ctx, &models.Host{ID: hostID, Name: "disk", Hostname: "disk.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	// Samples 2 hours old → outside the 1h end_offset → materialized on refresh.
	for i := 0; i < 3; i++ {
		testutil.MustQuery(t, db,
			`INSERT INTO disk_metrics (host_id, timestamp, mount_point, filesystem, size_gb, used_gb, avail_gb, used_percent)
			 VALUES ($1, NOW() - INTERVAL '2 hours', '/', 'ext4', 100, 60, 40, 60)`, hostID)
	}
	testutil.MustQuery(t, db, `CALL refresh_continuous_aggregate('disk_metrics_1h', NULL, NULL)`)

	// hours=48 → hourly path, served from the CAGG.
	metrics, aggType, err := db.GetDiskMetricsAggregated(ctx, hostID, "/", 48)
	if err != nil {
		t.Fatalf("disk aggregated: %v", err)
	}
	if aggType != "hour" {
		t.Fatalf("aggType = %q, want hour", aggType)
	}
	if len(metrics) == 0 {
		t.Fatal("disk hourly CAGG result is empty")
	}
	for _, m := range metrics {
		if m.UsedPercent < 59 || m.UsedPercent > 61 {
			t.Errorf("used_percent = %.2f, want ~60", m.UsedPercent)
		}
	}
}

// TestDiskHealth_SMARTRoundTrip verifies the newly-wired SMART fields persist and read back.
func TestDiskHealth_SMARTRoundTrip(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	const hostID = "host-smart"
	if err := db.RegisterHost(ctx, &models.Host{ID: hostID, Name: "smart", Hostname: "smart.local", Status: "online"}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	in := []models.DiskHealth{{
		HostID: hostID, Device: "/dev/nvme0n1", Model: "TestSSD", SerialNumber: "SN1",
		SmartStatus: "PASSED", Temperature: 35, PowerOnHours: 1000, PowerCycles: 42,
		ReallocSectors: 0, PendingSectors: 0, UncorrectableSectors: 7, PercentageUsed: 12,
	}}
	if err := db.InsertDiskHealth(ctx, in); err != nil {
		t.Fatalf("insert disk health: %v", err)
	}

	got, err := db.GetLatestDiskHealth(ctx, hostID)
	if err != nil {
		t.Fatalf("get disk health: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 disk health row, got %d", len(got))
	}
	h := got[0]
	if h.UncorrectableSectors != 7 {
		t.Errorf("uncorrectable_sectors = %d, want 7", h.UncorrectableSectors)
	}
	if h.PercentageUsed != 12 {
		t.Errorf("percentage_used = %d, want 12", h.PercentageUsed)
	}
	if h.PowerCycles != 42 {
		t.Errorf("power_cycles = %d, want 42", h.PowerCycles)
	}
}

func assertProxmoxPct(t *testing.T, label string, summary []models.ProxmoxNodeMetricsSummary) {
	t.Helper()
	if len(summary) == 0 {
		t.Fatalf("%s: summary is empty", label)
	}
	for _, s := range summary {
		if s.CPUAvg < 49 || s.CPUAvg > 51 {
			t.Errorf("%s: cpu_avg = %.2f, want ~50", label, s.CPUAvg)
		}
		if s.MemoryAvg < 49 || s.MemoryAvg > 51 {
			t.Errorf("%s: mem_avg = %.2f, want ~50", label, s.MemoryAvg)
		}
		if s.SampleCount <= 0 {
			t.Errorf("%s: sample_count = %d, want > 0", label, s.SampleCount)
		}
	}
}
