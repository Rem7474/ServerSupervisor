package ws

import (
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

func byImage(rows []models.VersionComparison) map[string]models.VersionComparison {
	out := make(map[string]models.VersionComparison, len(rows))
	for _, r := range rows {
		out[r.HostID+"|"+r.DockerImage] = r
	}
	return out
}

// The ambient engine must classify a container the same way a tracker would,
// from the docker_image_versions cache alone — that's what makes the badge show
// for containers nobody configured a tracker for.
func TestBuildAmbientComparisons_Statuses(t *testing.T) {
	containers := []models.DockerContainer{
		{HostID: "h1", Hostname: "srv", Image: "uptodate", ImageTag: "latest", ImageDigest: "sha256:same"},
		{HostID: "h1", Hostname: "srv", Image: "outdated", ImageTag: "latest", ImageDigest: "sha256:old"},
		{HostID: "h1", Hostname: "srv", Image: "pinned", ImageTag: "v1.2.3"},
		{HostID: "h1", Hostname: "srv", Image: "private", ImageTag: "latest", ImageDigest: "sha256:x"},
		{HostID: "h1", Hostname: "srv", Image: "uncached", ImageTag: "latest", ImageDigest: "sha256:y"},
	}
	cache := []models.DockerImageVersion{
		{Image: "uptodate", ImageTag: "latest", LatestDigest: "same", LatestTag: "v2.0.0"},
		{Image: "outdated", ImageTag: "latest", LatestDigest: "new", LatestTag: "v3.0.0"},
		{Image: "pinned", ImageTag: "v1.2.3", LatestDigest: "d", LatestTag: "v1.2.3"},
		{Image: "private", ImageTag: "latest", LastError: "registre privé : aucun identifiant enregistré ne correspond à cet hôte de registre"},
	}

	rows := byImage(buildAmbientComparisons(containers, cache, map[string]struct{}{}))
	if len(rows) != 5 {
		t.Fatalf("got %d ambient rows, want one per container group", len(rows))
	}

	if got := rows["h1|uptodate"]; got.Status != models.VersionStatusUpToDate {
		t.Errorf("matching digest → status %q, want %q", got.Status, models.VersionStatusUpToDate)
	}
	// The digest match also reveals the exact release behind a moving "latest".
	if got := rows["h1|uptodate"].RunningVersion; got != "v2.0.0" {
		t.Errorf("running_version = %q, want the resolved v2.0.0", got)
	}
	if got := rows["h1|outdated"]; got.Status != models.VersionStatusUpdateAvailable || !got.UpdateConfirmed {
		t.Errorf("differing digests → %+v, want a confirmed update_available", got)
	}
	if got := rows["h1|pinned"]; got.Status != models.VersionStatusUpToDate {
		t.Errorf("pinned tag equal to latest → status %q, want up_to_date", got.Status)
	}
	// A private registry with no usable credential is explicitly unknown, with
	// the reason attached — not silently "up to date" or "outdated".
	private := rows["h1|private"]
	if private.Status != models.VersionStatusUnknown || private.LastError == "" {
		t.Errorf("private image → %+v, want unknown with a reason", private)
	}
	// Same for an image the sweep has simply never reached yet.
	if got := rows["h1|uncached"]; got.Status != models.VersionStatusUnknown || got.LatestVersion != "" {
		t.Errorf("uncached image → %+v, want unknown with no latest version", got)
	}
}

// A container already explained by a tracker row must not get a second,
// competing ambient row — the frontend would have two answers for one badge.
func TestBuildAmbientComparisons_SkipsTrackerCoveredGroups(t *testing.T) {
	containers := []models.DockerContainer{
		{HostID: "h1", Image: "tracked", ImageTag: "latest"},
		{HostID: "h1", Image: "untracked", ImageTag: "latest"},
	}
	covered := map[string]struct{}{containerGroupKey("h1", "tracked", "latest"): {}}

	rows := buildAmbientComparisons(containers, nil, covered)
	if len(rows) != 1 || rows[0].DockerImage != "untracked" {
		t.Fatalf("got %+v, want only the untracked container", rows)
	}
}

func TestBuildAmbientComparisons_GroupsByHostImageTag(t *testing.T) {
	containers := []models.DockerContainer{
		{HostID: "h1", Image: "nginx", ImageTag: "latest"},
		{HostID: "h1", Image: "nginx", ImageTag: "latest"},
		{HostID: "h1", Image: "nginx", ImageTag: "1.25"},
		{HostID: "h2", Image: "nginx", ImageTag: "latest"},
	}
	rows := buildAmbientComparisons(containers, nil, map[string]struct{}{})
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3 (two tags on h1 + one on h2)", len(rows))
	}
	for _, r := range rows {
		if r.HostID == "h1" && r.ImageTag == "latest" && r.ContainerCount != 2 {
			t.Errorf("container_count = %d, want 2 for the duplicated group", r.ContainerCount)
		}
	}
	// Output must be deterministic: the WS layer suppresses unchanged pushes by
	// hashing the snapshot, so map-iteration order would create phantom diffs.
	for i := 0; i < 20; i++ {
		again := buildAmbientComparisons(containers, nil, map[string]struct{}{})
		for j := range again {
			if again[j].HostID != rows[j].HostID || again[j].ImageTag != rows[j].ImageTag {
				t.Fatal("ambient comparison order is not deterministic")
			}
		}
	}
}

// The tracker path must keep producing exactly what it produced before the
// ambient engine existed, including the container-less "tracker with nothing
// running" row and the aggregation across several containers.
func TestBuildTrackerComparisons_PreservesExistingBehaviour(t *testing.T) {
	trackers := []models.ReleaseTracker{
		{ID: "t1", HostID: "h1", HostName: "srv", DockerImage: "app", LastReleaseTag: "v2", LatestImageDigest: "sha256:new", CustomTaskID: "task"},
		{ID: "t2", HostID: "h1", HostName: "srv", DockerImage: "ghost", LastReleaseTag: "v9"},
		{ID: "t3", HostID: "h1", DockerImage: "nopoll"}, // never polled → no row at all
	}
	containersByHost := map[string][]models.DockerContainer{
		"h1": {
			{HostID: "h1", Image: "app", ImageTag: "latest", ImageDigest: "sha256:new"},
			{HostID: "h1", Image: "app", ImageTag: "latest", ImageDigest: "sha256:old"},
		},
	}
	covered := map[string]struct{}{}
	rows := buildTrackerComparisons(trackers, containersByHost, map[string]string{}, covered)

	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (an unpolled tracker still produces none)", len(rows))
	}
	byID := map[string]models.VersionComparison{}
	for _, r := range rows {
		byID[r.TrackerID] = r
	}
	t1 := byID["t1"]
	if t1.ContainerCount != 2 {
		t.Errorf("container_count = %d, want both containers aggregated", t1.ContainerCount)
	}
	// Worst-case aggregation: one stale container makes the whole tracker stale.
	if t1.IsUpToDate || t1.Status != models.VersionStatusUpdateAvailable {
		t.Errorf("aggregate = %+v, want update_available (one container is behind)", t1)
	}
	if t2 := byID["t2"]; t2.ContainerCount != 0 || t2.Status != models.VersionStatusUnknown {
		t.Errorf("tracker with no running container = %+v, want an unknown row", t2)
	}
	if _, ok := covered[containerGroupKey("h1", "app", "latest")]; !ok {
		t.Error("tracker-matched container group should be marked covered")
	}
}

func TestComparisonStatus(t *testing.T) {
	cases := []struct {
		name            string
		upToDate        bool
		runningVersion  string
		updateConfirmed bool
		want            string
	}{
		{"up to date wins", true, "", false, models.VersionStatusUpToDate},
		{"known running version behind latest", false, "v1", false, models.VersionStatusUpdateAvailable},
		{"digest-confirmed update with unknown version", false, "", true, models.VersionStatusUpdateAvailable},
		{"nothing to go on", false, "", false, models.VersionStatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := comparisonStatus(tc.upToDate, tc.runningVersion, tc.updateConfirmed); got != tc.want {
				t.Errorf("comparisonStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}
