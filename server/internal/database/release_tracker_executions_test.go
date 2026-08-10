package database_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestListReleaseTrackerExecutions_IncludesHost is the regression test for the
// "Historique des exécutions" host column on the release tracker detail page
// coming back empty: ReleaseTrackerExecution never carried the tracker's
// target host, so the frontend's `execution.host_name || execution.host_id`
// fallback always rendered "—". The host isn't stored per-execution — it's
// joined in from release_trackers.host_id at read time (see
// ListReleaseTrackerExecutions), same "computed on read" shape as
// TrackerDriftDetected.
func TestListReleaseTrackerExecutions_IncludesHost(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "host-tracker-exec-test"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "exec-test-host", Hostname: "exec-test.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	tracker, err := db.CreateReleaseTracker(ctx, models.ReleaseTracker{
		Name: "exec tracker", TrackerType: "docker", DockerImage: "nginx", DockerTag: "latest",
		HostID: hostID, UpdateAction: "custom", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create tracker: %v", err)
	}

	if _, err := db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
		TrackerID: tracker.ID, TagName: "v1.2.3", Status: "pending",
	}); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	execs, err := db.ListReleaseTrackerExecutions(ctx, tracker.ID, 10)
	if err != nil {
		t.Fatalf("ListReleaseTrackerExecutions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].HostID != hostID {
		t.Errorf("HostID = %q, want %q", execs[0].HostID, hostID)
	}
	if execs[0].HostName != "exec-test-host" {
		t.Errorf("HostName = %q, want %q", execs[0].HostName, "exec-test-host")
	}
}

// TestListReleaseTrackerExecutions_MonitorOnlyHasNoHost covers the other
// side: a monitor-only tracker (no host_id) must not error and must report
// an empty host, not a spurious one from a JOIN gone wrong.
func TestListReleaseTrackerExecutions_MonitorOnlyHasNoHost(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	tracker, err := db.CreateReleaseTracker(ctx, models.ReleaseTracker{
		Name: "monitor-only tracker", TrackerType: "git", Provider: "github",
		RepoOwner: "acme", RepoName: "app", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create tracker: %v", err)
	}

	if _, err := db.CreateReleaseTrackerExecution(ctx, models.ReleaseTrackerExecution{
		TrackerID: tracker.ID, TagName: "v2.0.0", Status: "pending",
	}); err != nil {
		t.Fatalf("create execution: %v", err)
	}

	execs, err := db.ListReleaseTrackerExecutions(ctx, tracker.ID, 10)
	if err != nil {
		t.Fatalf("ListReleaseTrackerExecutions: %v", err)
	}
	if len(execs) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(execs))
	}
	if execs[0].HostID != "" || execs[0].HostName != "" {
		t.Errorf("expected empty host for a monitor-only tracker, got HostID=%q HostName=%q", execs[0].HostID, execs[0].HostName)
	}
}
