package database_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

const testImageVersionsHostID = "host-image-versions-test"

// Covers migration 093 end to end: the ambient image-version cache's work list
// (distinct running image:tag), its upsert/error/read paths and the prune that
// keeps the table following the fleet. The unit tests in
// internal/services/dockerversions fake this repository, so this is the only
// place the actual SQL (unique key, tag normalization, UNNEST-based prune) runs.
func TestDockerImageVersions_CachePaths(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	if err := db.RegisterHost(ctx, &models.Host{
		ID: testImageVersionsHostID, Name: "img-test", Hostname: "img.local", Status: "online",
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	// Two containers share nginx:latest; one runs an untagged image, which the
	// query must normalize to :latest so both map onto a single cache row.
	if err := db.UpsertDockerContainers(ctx, testImageVersionsHostID, []models.DockerContainer{
		{ID: "img-test-1", ContainerID: "c1", Name: "web-1", Image: "nginx", ImageTag: "latest", State: "running"},
		{ID: "img-test-2", ContainerID: "c2", Name: "web-2", Image: "nginx", ImageTag: "latest", State: "running"},
		{ID: "img-test-3", ContainerID: "c3", Name: "bare", Image: "redis", ImageTag: "", State: "running"},
		{ID: "img-test-4", ContainerID: "c4", Name: "app", Image: "ghcr.io/org/app", ImageTag: "v1.2.3", State: "running"},
	}); err != nil {
		t.Fatalf("upsert containers: %v", err)
	}

	refs, err := db.ListDistinctDockerImages(ctx)
	if err != nil {
		t.Fatalf("ListDistinctDockerImages: %v", err)
	}
	got := map[string]string{}
	for _, r := range refs {
		got[r.Image] = r.ImageTag
	}
	if len(refs) != 3 {
		t.Fatalf("got %d distinct image refs (%v), want 3", len(refs), got)
	}
	if got["redis"] != "latest" {
		t.Errorf("empty image_tag = %q, want it normalized to latest", got["redis"])
	}

	// Upsert twice: the unique (image, image_tag) key must update in place.
	for _, digest := range []string{"first", "second"} {
		if err := db.UpsertDockerImageVersion(ctx, models.DockerImageVersion{
			Image: "nginx", ImageTag: "latest", Registry: "registry-1.docker.io",
			LatestDigest: digest, LatestTag: "1.27.4",
		}); err != nil {
			t.Fatalf("upsert %s: %v", digest, err)
		}
	}
	row, err := db.GetDockerImageVersion(ctx, "nginx", "")
	if err != nil {
		t.Fatalf("GetDockerImageVersion: %v", err)
	}
	if row == nil {
		t.Fatal("expected a cache row for nginx:latest (empty tag must resolve to latest)")
	}
	if row.LatestDigest != "second" || row.LatestTag != "1.27.4" {
		t.Errorf("row = %+v, want the second upsert to have replaced the first", row)
	}
	if row.CheckedAt == nil {
		t.Error("checked_at should be set by the upsert")
	}

	// A failed check records the reason without discarding the known digest.
	if err := db.SetDockerImageVersionError(ctx, "nginx", "latest", "registry-1.docker.io", "boom"); err != nil {
		t.Fatalf("SetDockerImageVersionError: %v", err)
	}
	row, err = db.GetDockerImageVersion(ctx, "nginx", "latest")
	if err != nil {
		t.Fatalf("GetDockerImageVersion after error: %v", err)
	}
	if row.LastError != "boom" {
		t.Errorf("last_error = %q, want boom", row.LastError)
	}
	if row.LatestDigest != "second" {
		t.Errorf("a failed check must not blank the last known digest, got %q", row.LatestDigest)
	}

	// A never-checked image simply has no row — not an error.
	missing, err := db.GetDockerImageVersion(ctx, "never-checked", "latest")
	if err != nil {
		t.Fatalf("GetDockerImageVersion (missing): %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for an unchecked image, got %+v", missing)
	}

	// Prune drops everything outside the keep-list.
	if err := db.UpsertDockerImageVersion(ctx, models.DockerImageVersion{
		Image: "gone", ImageTag: "latest", LatestDigest: "d",
	}); err != nil {
		t.Fatalf("upsert gone: %v", err)
	}
	removed, err := db.PruneDockerImageVersions(ctx, []models.DockerImageRef{{Image: "nginx", ImageTag: ""}})
	if err != nil {
		t.Fatalf("PruneDockerImageVersions: %v", err)
	}
	if removed != 1 {
		t.Errorf("pruned %d rows, want 1", removed)
	}
	all, err := db.ListDockerImageVersions(ctx)
	if err != nil {
		t.Fatalf("ListDockerImageVersions: %v", err)
	}
	if len(all) != 1 || all[0].Image != "nginx" {
		t.Errorf("remaining rows = %+v, want only nginx (kept via its normalized tag)", all)
	}

	// An empty keep-list is a no-op guard, never a "delete everything".
	if removed, err = db.PruneDockerImageVersions(ctx, nil); err != nil || removed != 0 {
		t.Errorf("prune with an empty keep-list = (%d, %v), want (0, nil)", removed, err)
	}
}
