package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func TestBatchPerf_AlertsAndMaintenance(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	host1 := "host-batch-1"
	host2 := "host-batch-2"
	for _, h := range []string{host1, host2} {
		if err := db.RegisterHost(ctx, &models.Host{
			ID: h, Name: h, Hostname: h + ".local", Status: "online",
		}); err != nil {
			t.Fatalf("register host %s: %v", h, err)
		}
	}

	// 1. Test GetHostsInMaintenance
	now := time.Now()
	// Create host-scoped window for host1
	w1 := models.MaintenanceWindow{
		HostID:    &host1,
		Reason:    "maintenance host1",
		StartsAt:  now.Add(-10 * time.Minute),
		EndsAt:    now.Add(10 * time.Minute),
		CreatedBy: "test",
	}
	if _, err := db.CreateMaintenanceWindow(ctx, w1); err != nil {
		t.Fatalf("create window1: %v", err)
	}

	maintSet, err := db.GetHostsInMaintenance(ctx)
	if err != nil {
		t.Fatalf("GetHostsInMaintenance: %v", err)
	}
	if !maintSet[host1] {
		t.Errorf("expected host1 in maintenance, got map: %v", maintSet)
	}
	if maintSet[host2] {
		t.Errorf("did not expect host2 in maintenance")
	}
	if maintSet["*"] {
		t.Errorf("did not expect global wildcard maintenance yet")
	}

	// Add global maintenance window (host_id IS NULL)
	wGlobal := models.MaintenanceWindow{
		HostID:    nil,
		Reason:    "global window",
		StartsAt:  now.Add(-5 * time.Minute),
		EndsAt:    now.Add(5 * time.Minute),
		CreatedBy: "admin",
	}
	if _, err := db.CreateMaintenanceWindow(ctx, wGlobal); err != nil {
		t.Fatalf("create global window: %v", err)
	}

	maintSetGlobal, err := db.GetHostsInMaintenance(ctx)
	if err != nil {
		t.Fatalf("GetHostsInMaintenance after global: %v", err)
	}
	if !maintSetGlobal["*"] {
		t.Errorf("expected '*' in maintenance, got %v", maintSetGlobal)
	}

	// 2. Test ListAllOpenAlertIncidents
	warnVal := 80.0
	rule := &models.AlertRule{
		SourceType:    models.AlertSourceAgent,
		HostID:        &host1,
		Metric:        "cpu_percent",
		Operator:      ">",
		ThresholdWarn: &warnVal,
		Enabled:       true,
		Actions: models.AlertActions{
			Channels: []string{"browser"},
		},
	}
	if err := db.CreateAlertRule(ctx, rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	incID1, err := db.CreateAlertIncident(ctx, rule.ID, host1, 85.0, "warn")
	if err != nil {
		t.Fatalf("create incident 1: %v", err)
	}

	incID2, err := db.CreateAlertIncident(ctx, rule.ID, host2, 90.0, "crit")
	if err != nil {
		t.Fatalf("create incident 2: %v", err)
	}

	openIncidents, err := db.ListAllOpenAlertIncidents(ctx)
	if err != nil {
		t.Fatalf("ListAllOpenAlertIncidents: %v", err)
	}

	foundInc1 := false
	foundInc2 := false
	for _, inc := range openIncidents {
		if inc.ID == incID1 {
			foundInc1 = true
		}
		if inc.ID == incID2 {
			foundInc2 = true
		}
	}
	if !foundInc1 {
		t.Errorf("expected incident 1 in open incidents map")
	}
	if !foundInc2 {
		t.Errorf("expected incident 2 in open incidents map")
	}

	// Resolve incident 1 and verify it disappears from open incidents
	if err := db.ResolveAlertIncident(ctx, incID1); err != nil {
		t.Fatalf("resolve incident 1: %v", err)
	}

	openAfterResolve, err := db.ListAllOpenAlertIncidents(ctx)
	if err != nil {
		t.Fatalf("ListAllOpenAlertIncidents after resolve: %v", err)
	}
	for _, inc := range openAfterResolve {
		if inc.ID == incID1 {
			t.Errorf("expected incident 1 resolved, but still found in open incidents")
		}
	}
}

func TestBatchPerf_AptAndMetrics(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	host1 := "host-apt-1"
	host2 := "host-apt-2"
	for _, h := range []string{host1, host2} {
		if err := db.RegisterHost(ctx, &models.Host{
			ID: h, Name: h, Hostname: h + ".local", Status: "online",
		}); err != nil {
			t.Fatalf("register host %s: %v", h, err)
		}
	}

	// 1. Test GetAptStatusAll
	if err := db.UpsertAptStatus(ctx, &models.AptStatus{
		HostID:          host1,
		PendingPackages: 5,
		SecurityUpdates: 2,
		PackageList:     `["pkg1","pkg2"]`,
		CVEList:         "[]",
	}); err != nil {
		t.Fatalf("update apt status host1: %v", err)
	}
	if err := db.UpsertAptStatus(ctx, &models.AptStatus{
		HostID:          host2,
		PendingPackages: 0,
		SecurityUpdates: 0,
		PackageList:     "[]",
		CVEList:         "[]",
	}); err != nil {
		t.Fatalf("update apt status host2: %v", err)
	}

	aptMap, err := db.GetAptStatusAll(ctx)
	if err != nil {
		t.Fatalf("GetAptStatusAll: %v", err)
	}
	if s, ok := aptMap[host1]; !ok || s.PendingPackages != 5 || s.SecurityUpdates != 2 {
		t.Errorf("unexpected apt status for host1: %+v", s)
	}
	if s, ok := aptMap[host2]; !ok || s.PendingPackages != 0 {
		t.Errorf("unexpected apt status for host2: %+v", s)
	}

	// 2. Test GetUUStatusAll
	now := time.Now().Truncate(time.Second)
	if err := db.UpsertUUStatus(ctx, host1, models.UnattendedUpgradesStatus{
		Installed:      true,
		Enabled:        true,
		RebootRequired: false,
		Config: models.UUConfig{
			SecurityOnly: true,
			AutoReboot:   false,
		},
	}); err != nil {
		t.Fatalf("upsert uu status: %v", err)
	}
	if err := db.UpdateUULastRun(ctx, host1, now, 3); err != nil {
		t.Fatalf("update uu last run: %v", err)
	}

	uuMap, err := db.GetUUStatusAll(ctx)
	if err != nil {
		t.Fatalf("GetUUStatusAll: %v", err)
	}
	if uu, ok := uuMap[host1]; !ok || !uu.Installed || !uu.Enabled || uu.LastRunPackages != 3 {
		t.Errorf("unexpected uu status for host1: %+v", uu)
	}

	// 3. Test GetAptHistoryAll
	if _, err := db.CreateRemoteCommand(ctx, host1, "apt", "update", "", "", "admin", nil); err != nil {
		t.Fatalf("create remote command 1: %v", err)
	}
	if _, err := db.CreateRemoteCommand(ctx, host1, "apt", "upgrade", "", "", "admin", nil); err != nil {
		t.Fatalf("create remote command 2: %v", err)
	}
	// Non-apt command should not be returned by GetAptHistoryAll
	if _, err := db.CreateRemoteCommand(ctx, host1, "docker", "restart", "", "", "admin", nil); err != nil {
		t.Fatalf("create docker command: %v", err)
	}

	historyMap, err := db.GetAptHistoryAll(ctx, 10)
	if err != nil {
		t.Fatalf("GetAptHistoryAll: %v", err)
	}
	cmds, ok := historyMap[host1]
	if !ok || len(cmds) != 2 {
		t.Errorf("expected 2 apt commands for host1, got %d", len(cmds))
	}
	for _, c := range cmds {
		if c.Module != "apt" {
			t.Errorf("expected apt module, got %s", c.Module)
		}
	}

	// 4. Test CountMetrics (hypertable_approximate_row_count fallback)
	count, err := db.CountMetrics(ctx)
	if err != nil {
		t.Fatalf("CountMetrics: %v", err)
	}
	if count < 0 {
		t.Errorf("expected count >= 0, got %d", count)
	}
}
