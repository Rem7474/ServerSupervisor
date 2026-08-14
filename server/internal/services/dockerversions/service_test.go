package dockerversions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

// ===== fakes =====

type fakeRepo struct {
	running  []models.DockerImageRef
	trackers []models.DockerImageRef
	creds    []models.RegistryCredential
	auth     map[string][2]string
	rows     map[string]models.DockerImageVersion

	upserts []models.DockerImageVersion
	errors  []string
	pruned  []models.DockerImageRef
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{auth: map[string][2]string{}, rows: map[string]models.DockerImageVersion{}}
}

func (f *fakeRepo) ListDistinctDockerImages(context.Context) ([]models.DockerImageRef, error) {
	return f.running, nil
}
func (f *fakeRepo) ListDockerTrackerImageRefs(context.Context) ([]models.DockerImageRef, error) {
	return f.trackers, nil
}
func (f *fakeRepo) ListRegistryCredentials(context.Context) ([]models.RegistryCredential, error) {
	return f.creds, nil
}
func (f *fakeRepo) GetRegistryCredentialAuth(_ context.Context, id string) (string, string, error) {
	if a, ok := f.auth[id]; ok {
		return a[0], a[1], nil
	}
	return "", "", nil
}
func (f *fakeRepo) GetDockerImageVersion(_ context.Context, image, tag string) (*models.DockerImageVersion, error) {
	if v, ok := f.rows[image+":"+tag]; ok {
		return &v, nil
	}
	return nil, nil
}
func (f *fakeRepo) UpsertDockerImageVersion(_ context.Context, v models.DockerImageVersion) error {
	f.upserts = append(f.upserts, v)
	now := time.Now()
	v.CheckedAt = &now
	f.rows[v.Image+":"+v.ImageTag] = v
	return nil
}
func (f *fakeRepo) SetDockerImageVersionError(_ context.Context, image, tag, _, errMsg string) error {
	f.errors = append(f.errors, image+":"+tag+" -> "+errMsg)
	now := time.Now()
	row := f.rows[image+":"+tag]
	row.Image, row.ImageTag, row.LastError, row.CheckedAt = image, tag, errMsg, &now
	f.rows[image+":"+tag] = row
	return nil
}
func (f *fakeRepo) PruneDockerImageVersions(_ context.Context, keep []models.DockerImageRef) (int64, error) {
	f.pruned = keep
	return 0, nil
}

type fakeRegistry struct {
	digests  map[string]string
	versions map[string]string
	err      error
	calls    int
	authSeen [2]string
}

func (r *fakeRegistry) ManifestDigest(image, tag, _, user, pass string) (string, error) {
	r.calls++
	r.authSeen = [2]string{user, pass}
	if r.err != nil {
		return "", r.err
	}
	return r.digests[image+":"+tag], nil
}

func (r *fakeRegistry) VersionForDigest(image, digest, _, _, _ string) string {
	return r.versions[image+"@"+digest]
}

func newSvc(repo Repository, reg RegistryClient) *Service {
	s := NewService(repo, &config.Config{DockerImagePollInterval: time.Hour})
	s.reg = reg
	return s
}

// ===== tests =====

func TestRefreshAll_ChecksRunningAndTrackedImagesOnce(t *testing.T) {
	repo := newFakeRepo()
	repo.running = []models.DockerImageRef{
		{Image: "nginx", ImageTag: "latest"},
		{Image: "nginx", ImageTag: ""}, // same row as above once normalized
		{Image: "ghcr.io/org/app", ImageTag: "v1.2.3"},
	}
	// A tracker whose image isn't running right now must still be refreshed.
	repo.trackers = []models.DockerImageRef{{Image: "redis", ImageTag: "7"}}
	reg := &fakeRegistry{digests: map[string]string{
		"nginx:latest":           "d-nginx",
		"ghcr.io/org/app:v1.2.3": "d-app",
		"redis:7":                "d-redis",
	}, versions: map[string]string{"redis@d-redis": "7.2.4"}}

	newSvc(repo, reg).RefreshAll(context.Background())

	if reg.calls != 3 {
		t.Fatalf("registry called %d times, want 3 (deduplicated by normalized image:tag)", reg.calls)
	}
	if len(repo.upserts) != 3 {
		t.Fatalf("got %d cache writes, want 3", len(repo.upserts))
	}
	byRef := map[string]models.DockerImageVersion{}
	for _, u := range repo.upserts {
		byRef[u.Image+":"+u.ImageTag] = u
	}
	if got := byRef["nginx:latest"].LatestDigest; got != "d-nginx" {
		t.Errorf("nginx digest = %q, want d-nginx", got)
	}
	// A broad tag resolves to the exact release behind it...
	if got := byRef["redis:7"].LatestTag; got != "7.2.4" {
		t.Errorf("redis latest_tag = %q, want 7.2.4 (resolved from digest)", got)
	}
	// ...while an already-pinned one stays itself and is never resolved.
	if got := byRef["ghcr.io/org/app:v1.2.3"].LatestTag; got != "v1.2.3" {
		t.Errorf("pinned latest_tag = %q, want v1.2.3", got)
	}
	if len(repo.pruned) != 3 {
		t.Errorf("prune keep-list has %d refs, want the 3 deduplicated ones", len(repo.pruned))
	}
}

func TestRefreshAll_OneFailingImageDoesNotAbortTheSweep(t *testing.T) {
	repo := newFakeRepo()
	repo.running = []models.DockerImageRef{{Image: "broken", ImageTag: "latest"}, {Image: "nginx", ImageTag: "latest"}}
	// digests map has no entry for "broken" → empty digest → recorded error.
	reg := &fakeRegistry{digests: map[string]string{"nginx:latest": "d-nginx"}}

	newSvc(repo, reg).RefreshAll(context.Background())

	if len(repo.errors) != 1 {
		t.Fatalf("got %d recorded errors, want 1", len(repo.errors))
	}
	if len(repo.upserts) != 1 || repo.upserts[0].Image != "nginx" {
		t.Fatalf("the healthy image was not refreshed after the failing one: %+v", repo.upserts)
	}
}

func TestRefresh_MatchesRegistryCredentialByHost(t *testing.T) {
	repo := newFakeRepo()
	repo.running = []models.DockerImageRef{{Image: "ghcr.io/org/app", ImageTag: "latest"}}
	repo.creds = []models.RegistryCredential{
		{ID: "hub", RegistryHost: "docker.io"},
		{ID: "ghcr", RegistryHost: "https://ghcr.io/"},
	}
	repo.auth["ghcr"] = [2]string{"user", "secret"}
	reg := &fakeRegistry{digests: map[string]string{"ghcr.io/org/app:latest": "d"}}

	newSvc(repo, reg).RefreshAll(context.Background())

	if reg.authSeen != [2]string{"user", "secret"} {
		t.Fatalf("credential not applied, registry saw %v", reg.authSeen)
	}
	if len(repo.upserts) != 1 || repo.upserts[0].RegistryCredentialsID != "ghcr" {
		t.Fatalf("credential id not recorded on the cache row: %+v", repo.upserts)
	}
}

func TestRefresh_NoMatchingCredentialIsUnknownNotFatal(t *testing.T) {
	repo := newFakeRepo()
	repo.running = []models.DockerImageRef{{Image: "registry.internal/app", ImageTag: "latest"}}
	repo.creds = []models.RegistryCredential{{ID: "ghcr", RegistryHost: "ghcr.io"}}
	reg := &fakeRegistry{err: errors.New("registry registry.internal returned status 401 for app:latest")}

	newSvc(repo, reg).RefreshAll(context.Background())

	if reg.authSeen != [2]string{"", ""} {
		t.Fatalf("a non-matching credential was applied: %v", reg.authSeen)
	}
	if len(repo.errors) != 1 {
		t.Fatalf("got %d recorded errors, want 1 (skipped, not fatal)", len(repo.errors))
	}
	if want := "identifiant"; !contains(repo.errors[0], want) {
		t.Errorf("error %q should explain the missing credential", repo.errors[0])
	}
}

func TestLatest_UsesCacheThenRefreshesWhenStale(t *testing.T) {
	repo := newFakeRepo()
	fresh := time.Now()
	repo.rows["nginx:latest"] = models.DockerImageVersion{
		Image: "nginx", ImageTag: "latest", LatestDigest: "cached", CheckedAt: &fresh,
	}
	reg := &fakeRegistry{digests: map[string]string{"nginx:latest": "refreshed"}}
	svc := newSvc(repo, reg)

	got, err := svc.Latest(context.Background(), "nginx", "")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.LatestDigest != "cached" || reg.calls != 0 {
		t.Fatalf("a fresh row must be served from cache without a registry call (digest=%q calls=%d)", got.LatestDigest, reg.calls)
	}

	stale := time.Now().Add(-2 * time.Hour)
	repo.rows["nginx:latest"] = models.DockerImageVersion{
		Image: "nginx", ImageTag: "latest", LatestDigest: "cached", CheckedAt: &stale,
	}
	got, err = svc.Latest(context.Background(), "nginx", "latest")
	if err != nil {
		t.Fatalf("Latest (stale): %v", err)
	}
	if got.LatestDigest != "refreshed" || reg.calls != 1 {
		t.Fatalf("a stale row must trigger exactly one on-demand refresh (digest=%q calls=%d)", got.LatestDigest, reg.calls)
	}
}

func TestLatest_MissingRowRefreshesOnDemand(t *testing.T) {
	repo := newFakeRepo()
	reg := &fakeRegistry{digests: map[string]string{"nginx:latest": "d"}}

	got, err := newSvc(repo, reg).Latest(context.Background(), "nginx", "latest")
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got.LatestDigest != "d" || reg.calls != 1 {
		t.Fatalf("cold cache should refresh once, got digest=%q calls=%d", got.LatestDigest, reg.calls)
	}
}

// A registry outage must not blank out a digest we already know: the tracker
// poller would otherwise treat "no answer" as "nothing to compare against".
func TestLatest_FallsBackToStaleRowOnRegistryFailure(t *testing.T) {
	repo := newFakeRepo()
	old := time.Now().Add(-48 * time.Hour)
	repo.rows["nginx:latest"] = models.DockerImageVersion{
		Image: "nginx", ImageTag: "latest", LatestDigest: "known", CheckedAt: &old,
	}
	reg := &fakeRegistry{err: errors.New("dial tcp: connection refused")}

	got, err := newSvc(repo, reg).Latest(context.Background(), "nginx", "latest")
	if err != nil {
		t.Fatalf("Latest should degrade gracefully, got error %v", err)
	}
	if got.LatestDigest != "known" {
		t.Errorf("digest = %q, want the last known good one", got.LatestDigest)
	}
	if got.LastError == "" {
		t.Error("the stale fallback should still report why it is stale")
	}
}

func TestFriendlyRegistryError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		tag      string
		hadCreds bool
		want     string
	}{
		{"latest 404 keeps the pinned-tag hint", errors.New("registry x returned status 404 for a:latest"), "latest", false, "tag latest introuvable"},
		{"401 without credentials explains the gap", errors.New("auth: status 401"), "v1", false, "identifiant"},
		{"401 with credentials is passed through", errors.New("auth: status 401"), "v1", true, "status 401"},
		{"anything else is passed through", errors.New("boom"), "v1", false, "boom"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := friendlyRegistryError(tc.err, tc.tag, tc.hadCreds); !contains(got, tc.want) {
				t.Errorf("friendlyRegistryError() = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

func TestSameRegistryHost(t *testing.T) {
	cases := []struct {
		configured, parsed string
		want               bool
	}{
		{"ghcr.io", "ghcr.io", true},
		{"https://ghcr.io/", "ghcr.io", true},
		{"GHCR.IO", "ghcr.io", true},
		{"docker.io", "registry-1.docker.io", true},
		{"index.docker.io", "registry-1.docker.io", true},
		{"ghcr.io", "registry-1.docker.io", false},
		{"", "ghcr.io", false},
	}
	for _, tc := range cases {
		if got := sameRegistryHost(tc.configured, tc.parsed); got != tc.want {
			t.Errorf("sameRegistryHost(%q, %q) = %v, want %v", tc.configured, tc.parsed, got, tc.want)
		}
	}
}

func TestShouldResolveTag(t *testing.T) {
	truthy := []string{"", "latest", "LATEST", "v4", "4", "v4.4", "4.4"}
	for _, tag := range truthy {
		if !ShouldResolveTag(tag) {
			t.Errorf("ShouldResolveTag(%q) = false, want true (mutable/broad tag)", tag)
		}
	}
	falsy := []string{"v4.4.1", "4.4.1", "v1.2.3", "stable", "bookworm", "v4-rc1"}
	for _, tag := range falsy {
		if ShouldResolveTag(tag) {
			t.Errorf("ShouldResolveTag(%q) = true, want false (already-pinned tag)", tag)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
