package database_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func TestLatestMetrics_BoundedFallbackAndFleet(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	// Register test hosts
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-metrics-fresh", Name: "fresh", Hostname: "fresh.local", Status: "online",
	}); err != nil {
		t.Fatalf("register fresh host: %v", err)
	}
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-metrics-old", Name: "old", Hostname: "old.local", Status: "offline",
	}); err != nil {
		t.Fatalf("register old host: %v", err)
	}

	// Insert fresh metrics (5m ago, inside 30m window)
	testutil.MustQuery(t, db,
		`INSERT INTO system_metrics (host_id, timestamp, cpu_usage_percent, memory_total, memory_used, memory_percent, hostname)
		 VALUES ($1, NOW() - INTERVAL '5 minutes', 25.5, 8000, 2000, 25.0, 'fresh.local')`,
		"host-metrics-fresh")

	// Insert old metrics (45m ago, outside 30m window)
	testutil.MustQuery(t, db,
		`INSERT INTO system_metrics (host_id, timestamp, cpu_usage_percent, memory_total, memory_used, memory_percent, hostname)
		 VALUES ($1, NOW() - INTERVAL '45 minutes', 75.0, 16000, 8000, 50.0, 'old.local')`,
		"host-metrics-old")

	// 1. GetLatestMetrics on fresh host hits bounded path
	fresh, err := db.GetLatestMetrics(ctx, "host-metrics-fresh")
	if err != nil {
		t.Fatalf("GetLatestMetrics fresh: %v", err)
	}
	if fresh.CPUUsagePercent != 25.5 {
		t.Errorf("expected CPU 25.5, got %f", fresh.CPUUsagePercent)
	}

	// 2. GetLatestMetrics on old host misses bounded path, falls back to unbounded
	old, err := db.GetLatestMetrics(ctx, "host-metrics-old")
	if err != nil {
		t.Fatalf("GetLatestMetrics old fallback: %v", err)
	}
	if old.CPUUsagePercent != 75.0 {
		t.Errorf("expected fallback CPU 75.0, got %f", old.CPUUsagePercent)
	}

	// 3. GetLatestMetrics on nonexistent host returns ErrNoRows
	_, err = db.GetLatestMetrics(ctx, "host-metrics-nonexistent")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected ErrNoRows for nonexistent host, got %v", err)
	}

	// 4. GetLatestMetricsAll returns only the fresh host (no unbounded fallback)
	all, err := db.GetLatestMetricsAll(ctx)
	if err != nil {
		t.Fatalf("GetLatestMetricsAll: %v", err)
	}
	if all["host-metrics-fresh"] == nil {
		t.Errorf("expected fresh host in GetLatestMetricsAll map")
	}
	if all["host-metrics-old"] != nil {
		t.Errorf("expected old host to be excluded from GetLatestMetricsAll map")
	}
}

func TestLatestDiskMetrics_BoundedFallbackAndRootAll(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-disk-fresh", Name: "disk-fresh", Hostname: "disk-fresh.local", Status: "online",
	}); err != nil {
		t.Fatalf("register fresh host: %v", err)
	}
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-disk-old", Name: "disk-old", Hostname: "disk-old.local", Status: "offline",
	}); err != nil {
		t.Fatalf("register old host: %v", err)
	}

	// Fresh disk metrics
	testutil.MustQuery(t, db,
		`INSERT INTO disk_metrics (host_id, timestamp, mount_point, filesystem, size_gb, used_gb, avail_gb, used_percent)
		 VALUES ($1, NOW() - INTERVAL '5 minutes', '/', 'ext4', 100, 35, 65, 35.0)`,
		"host-disk-fresh")

	// Old disk metrics
	testutil.MustQuery(t, db,
		`INSERT INTO disk_metrics (host_id, timestamp, mount_point, filesystem, size_gb, used_gb, avail_gb, used_percent)
		 VALUES ($1, NOW() - INTERVAL '50 minutes', '/', 'ext4', 200, 160, 40, 80.0)`,
		"host-disk-old")

	// 1. GetLatestDiskMetrics on fresh host
	fresh, err := db.GetLatestDiskMetrics(ctx, "host-disk-fresh")
	if err != nil {
		t.Fatalf("GetLatestDiskMetrics fresh: %v", err)
	}
	if len(fresh) != 1 || fresh[0].UsedPercent != 35.0 {
		t.Errorf("expected 1 fresh disk with used_percent 35.0, got %+v", fresh)
	}

	// 2. GetLatestDiskMetrics on old host (fallback path)
	old, err := db.GetLatestDiskMetrics(ctx, "host-disk-old")
	if err != nil {
		t.Fatalf("GetLatestDiskMetrics old: %v", err)
	}
	if len(old) != 1 || old[0].UsedPercent != 80.0 {
		t.Errorf("expected 1 fallback disk with used_percent 80.0, got %+v", old)
	}

	// 3. GetLatestDiskMetrics on empty host
	empty, err := db.GetLatestDiskMetrics(ctx, "host-disk-empty")
	if err != nil {
		t.Fatalf("GetLatestDiskMetrics empty: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty slice, got %+v", empty)
	}

	// 4. GetRootDiskPercentAll returns only fresh root disk
	rootMap := db.GetRootDiskPercentAll(ctx)
	if pct, ok := rootMap["host-disk-fresh"]; !ok || pct != 35.0 {
		t.Errorf("expected fresh host with root pct 35.0, got %f (ok=%v)", pct, ok)
	}
	if _, ok := rootMap["host-disk-old"]; ok {
		t.Errorf("expected old host to be excluded from GetRootDiskPercentAll")
	}
}

func TestLatestDiskHealth_BoundedAndFallback(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-smart-fresh", Name: "smart-fresh", Hostname: "smart-fresh.local", Status: "online",
	}); err != nil {
		t.Fatalf("register fresh host: %v", err)
	}
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-smart-old", Name: "smart-old", Hostname: "smart-old.local", Status: "offline",
	}); err != nil {
		t.Fatalf("register old host: %v", err)
	}

	// Fresh disk health
	testutil.MustQuery(t, db,
		`INSERT INTO disk_health (
			host_id, timestamp, device, model, serial_number, smart_status,
			temperature, power_on_hours, power_cycles, realloc_sectors, pending_sectors,
			uncorrectable_sectors, percentage_used
		 ) VALUES ($1, NOW() - INTERVAL '5 minutes', '/dev/sda', 'ModelA', 'SN1', 'PASSED', 35, 100, 10, 0, 0, 0, 5)`,
		"host-smart-fresh")

	// Old disk health
	testutil.MustQuery(t, db,
		`INSERT INTO disk_health (
			host_id, timestamp, device, model, serial_number, smart_status,
			temperature, power_on_hours, power_cycles, realloc_sectors, pending_sectors,
			uncorrectable_sectors, percentage_used
		 ) VALUES ($1, NOW() - INTERVAL '50 minutes', '/dev/sdb', 'ModelB', 'SN2', 'PASSED', 42, 500, 50, 0, 0, 0, 20)`,
		"host-smart-old")

	// 1. GetLatestDiskHealth fresh host
	fresh, err := db.GetLatestDiskHealth(ctx, "host-smart-fresh")
	if err != nil {
		t.Fatalf("GetLatestDiskHealth fresh: %v", err)
	}
	if len(fresh) != 1 || fresh[0].Device != "/dev/sda" {
		t.Errorf("expected fresh device /dev/sda, got %+v", fresh)
	}

	// 2. GetLatestDiskHealth old host (fallback)
	old, err := db.GetLatestDiskHealth(ctx, "host-smart-old")
	if err != nil {
		t.Fatalf("GetLatestDiskHealth old: %v", err)
	}
	if len(old) != 1 || old[0].Device != "/dev/sdb" {
		t.Errorf("expected fallback device /dev/sdb, got %+v", old)
	}

	// 3. GetLatestDiskHealth empty host
	empty, err := db.GetLatestDiskHealth(ctx, "host-smart-empty")
	if err != nil {
		t.Fatalf("GetLatestDiskHealth empty: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("expected empty slice, got %+v", empty)
	}
}

func TestLatestProxmoxGuestMetrics_BoundedFallbackAndMax(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	connID, nodeID := seedProxmoxConnNode(t, db)

	var guestFreshID, guestOldID string
	if err := db.QueryRow(ctx,
		`INSERT INTO proxmox_guests (connection_id, node_name, guest_type, vmid, name, status)
		 VALUES ($1, 'pve1', 'qemu', 201, 'vm201', 'running') RETURNING id`, connID).Scan(&guestFreshID); err != nil {
		t.Fatalf("seed guest fresh: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO proxmox_guests (connection_id, node_name, guest_type, vmid, name, status)
		 VALUES ($1, 'pve1', 'qemu', 202, 'vm202', 'running') RETURNING id`, connID).Scan(&guestOldID); err != nil {
		t.Fatalf("seed guest old: %v", err)
	}

	// Fresh sample: cpu 0.4 (40%), mem 1000/2000 (50%)
	testutil.MustQuery(t, db,
		`INSERT INTO proxmox_guest_metrics (guest_id, cpu_usage, mem_total, mem_used, timestamp)
		 VALUES ($1, 0.40, 2000, 1000, NOW() - INTERVAL '5 minutes')`, guestFreshID)

	// Old sample: cpu 0.9 (90%), mem 1800/2000 (90%)
	testutil.MustQuery(t, db,
		`INSERT INTO proxmox_guest_metrics (guest_id, cpu_usage, mem_total, mem_used, timestamp)
		 VALUES ($1, 0.90, 2000, 1800, NOW() - INTERVAL '45 minutes')`, guestOldID)

	// 1. GetLatestProxmoxGuestMetricPercent fresh (bounded)
	cpu, mem, _, err := db.GetLatestProxmoxGuestMetricPercent(ctx, guestFreshID)
	if err != nil {
		t.Fatalf("GetLatestProxmoxGuestMetricPercent fresh: %v", err)
	}
	if cpu != 40.0 || mem != 50.0 {
		t.Errorf("expected cpu=40 mem=50, got cpu=%f mem=%f", cpu, mem)
	}

	// 2. GetLatestProxmoxGuestMetricPercent old (fallback)
	cpu, mem, _, err = db.GetLatestProxmoxGuestMetricPercent(ctx, guestOldID)
	if err != nil {
		t.Fatalf("GetLatestProxmoxGuestMetricPercent old: %v", err)
	}
	if cpu != 90.0 || mem != 90.0 {
		t.Errorf("expected cpu=90 mem=90, got cpu=%f mem=%f", cpu, mem)
	}

	// 3. GetLatestProxmoxGuestMetricPercent nonexistent
	_, _, _, err = db.GetLatestProxmoxGuestMetricPercent(ctx, "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected ErrNoRows, got %v", err)
	}

	// 4. Fleet max queries (must exclude old guest past 30m, only take fresh guest)
	maxCPU := db.GetMaxProxmoxGuestCPUUsagePercent(ctx)
	if maxCPU != 40.0 {
		t.Errorf("expected maxCPU 40.0, got %f", maxCPU)
	}
	maxMem := db.GetMaxProxmoxGuestMemoryUsagePercent(ctx)
	if maxMem != 50.0 {
		t.Errorf("expected maxMem 50.0, got %f", maxMem)
	}
	maxConnCPU := db.GetMaxProxmoxGuestCPUUsagePercentByConnection(ctx, connID)
	if maxConnCPU != 40.0 {
		t.Errorf("expected maxConnCPU 40.0, got %f", maxConnCPU)
	}
	maxNodeCPU := db.GetMaxProxmoxGuestCPUUsagePercentByNode(ctx, nodeID)
	if maxNodeCPU != 40.0 {
		t.Errorf("expected maxNodeCPU 40.0, got %f", maxNodeCPU)
	}
	maxConnMem := db.GetMaxProxmoxGuestMemoryUsagePercentByConnection(ctx, connID)
	if maxConnMem != 50.0 {
		t.Errorf("expected maxConnMem 50.0, got %f", maxConnMem)
	}
	maxNodeMem := db.GetMaxProxmoxGuestMemoryUsagePercentByNode(ctx, nodeID)
	if maxNodeMem != 50.0 {
		t.Errorf("expected maxNodeMem 50.0, got %f", maxNodeMem)
	}
}

func TestEffectiveHostSensors_ProxmoxMapping(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	// Register sensor host and guest host
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-sensor", Name: "sensor-host", Hostname: "sensor.local", Status: "online",
	}); err != nil {
		t.Fatalf("register sensor host: %v", err)
	}
	if err := db.RegisterHost(ctx, &models.Host{
		ID: "host-guest", Name: "guest-host", Hostname: "guest.local", Status: "online",
	}); err != nil {
		t.Fatalf("register guest host: %v", err)
	}

	connID, nodeID := seedProxmoxConnNode(t, db)

	// Map node sensors to host-sensor
	testutil.MustQuery(t, db,
		`UPDATE proxmox_nodes SET cpu_temp_source_host_id = $1, fan_rpm_source_host_id = $1 WHERE id = $2`,
		"host-sensor", nodeID)

	var guestID string
	if err := db.QueryRow(ctx,
		`INSERT INTO proxmox_guests (connection_id, node_name, guest_type, vmid, name, status)
		 VALUES ($1, 'pve1', 'qemu', 301, 'vm301', 'running') RETURNING id`, connID).Scan(&guestID); err != nil {
		t.Fatalf("seed guest: %v", err)
	}

	// Link host-guest to guestID with metrics_source = 'proxmox'
	testutil.MustQuery(t, db,
		`INSERT INTO proxmox_guest_links (host_id, guest_id, status, metrics_source)
		 VALUES ($1, $2, 'confirmed', 'proxmox')`,
		"host-guest", guestID)

	// Case 1: Fresh sensor readings (3m ago <= 10 min window)
	testutil.MustQuery(t, db,
		`INSERT INTO system_metrics (host_id, timestamp, cpu_temperature, fan_rpm)
		 VALUES ($1, NOW() - INTERVAL '3 minutes', 55.5, 1600)`,
		"host-sensor")

	temp, ok := db.GetEffectiveHostCPUTemperature(ctx, "host-guest", 30.0)
	if !ok || temp != 55.5 {
		t.Errorf("expected temp=55.5 ok=true, got temp=%f ok=%v", temp, ok)
	}

	fan, ok := db.GetEffectiveHostFanRPM(ctx, "host-guest", 700.0)
	if !ok || fan != 1600.0 {
		t.Errorf("expected fan=1600 ok=true, got fan=%f ok=%v", fan, ok)
	}

	// Case 2: Old sensor readings (15m ago > 10 min window)
	testutil.MustQuery(t, db, `DELETE FROM system_metrics WHERE host_id = $1`, "host-sensor")
	testutil.MustQuery(t, db,
		`INSERT INTO system_metrics (host_id, timestamp, cpu_temperature, fan_rpm)
		 VALUES ($1, NOW() - INTERVAL '15 minutes', 60.0, 2000)`,
		"host-sensor")

	// Must fall back to local since mapped sensor sample is > 10m old
	temp, ok = db.GetEffectiveHostCPUTemperature(ctx, "host-guest", 32.0)
	if !ok || temp != 32.0 {
		t.Errorf("expected fallback temp=32.0 ok=true, got temp=%f ok=%v", temp, ok)
	}
	temp, ok = db.GetEffectiveHostCPUTemperature(ctx, "host-guest", 0)
	if ok || temp != 0 {
		t.Errorf("expected ok=false temp=0 with no fallback, got temp=%f ok=%v", temp, ok)
	}

	fan, ok = db.GetEffectiveHostFanRPM(ctx, "host-guest", 750.0)
	if !ok || fan != 750.0 {
		t.Errorf("expected fallback fan=750.0 ok=true, got fan=%f ok=%v", fan, ok)
	}
	fan, ok = db.GetEffectiveHostFanRPM(ctx, "host-guest", 0)
	if ok || fan != 0 {
		t.Errorf("expected ok=false fan=0 with no fallback, got fan=%f ok=%v", fan, ok)
	}
}
