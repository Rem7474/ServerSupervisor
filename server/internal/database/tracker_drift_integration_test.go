package database_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// TestTrackerDriftDetected exercises the real SQL behind GitOps-style compose
// drift reconciliation: it must compare the agent-reported deployed digest
// against the tracker's own recorded digest, and stay a strict no-op for
// anything that isn't a compose tracker with both sides populated.
func TestTrackerDriftDetected(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	hostID := "host-drift-test"
	if err := db.RegisterHost(ctx, &models.Host{
		ID: hostID, Name: "drift-test", Hostname: "drift.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	tracker, err := db.CreateReleaseTracker(ctx, models.ReleaseTracker{
		Name: "drift tracker", TrackerType: "docker", DockerImage: "nginx", DockerTag: "latest",
		HostID: hostID, UpdateAction: "compose", ComposeProject: "web", Enabled: true,
	})
	if err != nil {
		t.Fatalf("create tracker: %v", err)
	}

	t.Run("no digest recorded yet means no drift", func(t *testing.T) {
		drifted, err := db.TrackerDriftDetected(ctx, *tracker)
		if err != nil {
			t.Fatalf("TrackerDriftDetected: %v", err)
		}
		if drifted {
			t.Error("expected no drift before any digest is recorded")
		}
	})

	if err := db.UpsertDockerContainers(ctx, hostID, []models.DockerContainer{
		{ID: "drift-c1", HostID: hostID, ContainerID: "drift-c1", Name: "web", Image: "nginx", ImageTag: "latest", ImageDigest: "sha256:deployed"},
	}); err != nil {
		t.Fatalf("upsert containers: %v", err)
	}

	t.Run("deployed digest differs from the tracked digest means drift", func(t *testing.T) {
		tracker.LatestImageDigest = "sha256:expected"
		drifted, err := db.TrackerDriftDetected(ctx, *tracker)
		if err != nil {
			t.Fatalf("TrackerDriftDetected: %v", err)
		}
		if !drifted {
			t.Error("expected drift: deployed=sha256:deployed, tracked=sha256:expected")
		}
	})

	t.Run("deployed digest matching the tracked digest means no drift", func(t *testing.T) {
		tracker.LatestImageDigest = "sha256:deployed"
		drifted, err := db.TrackerDriftDetected(ctx, *tracker)
		if err != nil {
			t.Fatalf("TrackerDriftDetected: %v", err)
		}
		if drifted {
			t.Error("expected no drift when the deployed digest matches the tracked digest")
		}
	})

	t.Run("non-compose trackers never report drift", func(t *testing.T) {
		custom := *tracker
		custom.UpdateAction = "custom"
		custom.LatestImageDigest = "sha256:expected"
		drifted, err := db.TrackerDriftDetected(ctx, custom)
		if err != nil {
			t.Fatalf("TrackerDriftDetected: %v", err)
		}
		if drifted {
			t.Error("expected non-compose trackers to never report drift, regardless of digest state")
		}
	})
}
