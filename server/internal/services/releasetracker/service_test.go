package releasetracker

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	created     bool
	getErr      error
	getResult   *models.ReleaseTracker
	listResult  []models.ReleaseTracker
	driftResult bool
	pickable    []models.TrackableContainer
}

func (f *fakeRepo) ListRegistryCredentials(context.Context) ([]models.RegistryCredential, error) {
	return nil, nil
}
func (f *fakeRepo) CreateRegistryCredential(_ context.Context, rc models.RegistryCredential) (*models.RegistryCredential, error) {
	f.created = true
	return &rc, nil
}
func (f *fakeRepo) UpdateRegistryCredential(context.Context, string, models.RegistryCredential) error {
	return nil
}
func (f *fakeRepo) DeleteRegistryCredential(context.Context, string) error { return nil }
func (f *fakeRepo) ListReleaseTrackers(context.Context) ([]models.ReleaseTracker, error) {
	return f.listResult, nil
}
func (f *fakeRepo) CreateReleaseTracker(_ context.Context, t models.ReleaseTracker) (*models.ReleaseTracker, error) {
	f.created = true
	return &t, nil
}
func (f *fakeRepo) GetReleaseTrackerByID(context.Context, string) (*models.ReleaseTracker, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) UpdateReleaseTracker(context.Context, string, models.ReleaseTracker) error {
	return nil
}
func (f *fakeRepo) DeleteReleaseTracker(context.Context, string) error { return nil }
func (f *fakeRepo) ListReleaseTrackerExecutions(context.Context, string, int) ([]models.ReleaseTrackerExecution, error) {
	return nil, nil
}
func (f *fakeRepo) ListTrackableContainers(context.Context) ([]models.TrackableContainer, error) {
	return nil, nil
}
func (f *fakeRepo) ListPickableContainers(context.Context) ([]models.TrackableContainer, error) {
	return f.pickable, nil
}
func (f *fakeRepo) ListTrackerTagDigests(context.Context, string, int) ([]models.ReleaseVersionHistoryItem, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateReleaseTrackerExecutionByCommandID(context.Context, string, string) (string, bool, []string, error) {
	return "", false, nil, nil
}
func (f *fakeRepo) TrackerDriftDetected(context.Context, models.ReleaseTracker) (bool, error) {
	return f.driftResult, nil
}

func newSvc(repo Repository) *Service {
	return &Service{repo: repo, cfg: &config.Config{}, notifHub: nil, poller: nil}
}

func status(err error) int {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.HTTPStatus
	}
	return 0
}

func TestCreate_Validation(t *testing.T) {
	cases := []struct {
		name string
		req  models.ReleaseTrackerRequest
	}{
		{"bad type", models.ReleaseTrackerRequest{Name: "x", TrackerType: "svn"}},
		{"missing name", models.ReleaseTrackerRequest{TrackerType: "git", Provider: "github", RepoOwner: "o", RepoName: "r"}},
		{"git missing repo", models.ReleaseTrackerRequest{Name: "x", TrackerType: "git", Provider: "github"}},
		{"git bad provider", models.ReleaseTrackerRequest{Name: "x", TrackerType: "git", Provider: "bitbucket", RepoOwner: "o", RepoName: "r"}},
		{"git half-dispatch", models.ReleaseTrackerRequest{Name: "x", TrackerType: "git", Provider: "github", RepoOwner: "o", RepoName: "r", HostID: "h"}},
		{"docker no image", models.ReleaseTrackerRequest{Name: "x", TrackerType: "docker"}},
		// The optional git link a docker tracker may carry for release notes is
		// all-or-nothing, and its provider is still validated.
		{"docker half git link", models.ReleaseTrackerRequest{Name: "x", TrackerType: "docker", DockerImage: "nginx", RepoOwner: "o"}},
		{"docker git link bad provider", models.ReleaseTrackerRequest{Name: "x", TrackerType: "docker", DockerImage: "nginx", Provider: "bitbucket", RepoOwner: "o", RepoName: "r"}},
		{"cooldown out of range", models.ReleaseTrackerRequest{Name: "x", TrackerType: "git", Provider: "github", RepoOwner: "o", RepoName: "r", CooldownHours: 999}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := newSvc(repo).Create(context.Background(), tc.req)
			if status(err) != 400 {
				t.Fatalf("expected 400, got %v", err)
			}
			if repo.created {
				t.Error("must not persist an invalid tracker")
			}
		})
	}
}

func TestCreate_GitMonitorOnlyValid(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newSvc(repo).Create(context.Background(), models.ReleaseTrackerRequest{
		Name: "linux", TrackerType: "git", Provider: "github", RepoOwner: "torvalds", RepoName: "linux",
	})
	if err != nil {
		t.Fatalf("monitor-only git tracker should be valid, got %v", err)
	}
	if !repo.created {
		t.Error("valid tracker should be persisted")
	}
}

// A docker tracker's git link is optional: no repo at all stays valid, and a
// complete one is accepted (and defaults its provider) without turning the
// tracker into a git tracker.
func TestCreate_DockerOptionalGitLink(t *testing.T) {
	cases := []struct {
		name         string
		req          models.ReleaseTrackerRequest
		wantProvider string
	}{
		{
			"no git link",
			models.ReleaseTrackerRequest{Name: "x", TrackerType: "docker", DockerImage: "nginx", DockerTag: "latest"},
			"",
		},
		{
			"full link defaults the provider to github",
			models.ReleaseTrackerRequest{Name: "x", TrackerType: "docker", DockerImage: "nginx", DockerTag: "latest", RepoOwner: "o", RepoName: "r"},
			"github",
		},
		{
			"explicit provider is kept",
			models.ReleaseTrackerRequest{Name: "x", TrackerType: "docker", DockerImage: "nginx", DockerTag: "latest", Provider: "gitea", RepoOwner: "o", RepoName: "r"},
			"gitea",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			created, err := newSvc(&fakeRepo{}).Create(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("expected a valid docker tracker, got %v", err)
			}
			if created.TrackerType != "docker" {
				t.Errorf("tracker_type = %q, want docker", created.TrackerType)
			}
			if created.Provider != tc.wantProvider {
				t.Errorf("provider = %q, want %q", created.Provider, tc.wantProvider)
			}
		})
	}
}

func TestPickableContainers_NotRestrictedToCompose(t *testing.T) {
	repo := &fakeRepo{pickable: []models.TrackableContainer{
		{HostID: "h1", ContainerName: "standalone", Image: "nginx", ImageTag: "latest"},
		{HostID: "h1", ContainerName: "svc", Image: "redis", ImageTag: "7", ComposeProject: "proj", Tracked: true},
	}}
	got, err := newSvc(repo).PickableContainers(context.Background())
	if err != nil {
		t.Fatalf("PickableContainers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d containers, want both the standalone and the already-tracked compose one", len(got))
	}
}

func TestCreateRegistryCredential_RequiresAllFields(t *testing.T) {
	repo := &fakeRepo{}
	_, err := newSvc(repo).CreateRegistryCredential(context.Background(), models.RegistryCredentialRequest{Name: "x"})
	if status(err) != 400 {
		t.Fatalf("missing fields should be 400, got %v", err)
	}
	if repo.created {
		t.Error("must not create an incomplete credential")
	}
}

func TestCreateRegistryCredential_ClearsPassword(t *testing.T) {
	created, err := newSvc(&fakeRepo{}).CreateRegistryCredential(context.Background(), models.RegistryCredentialRequest{
		Name: "x", RegistryHost: "ghcr.io", Username: "u", Password: "secret",
	})
	if err != nil {
		t.Fatalf("valid credential: %v", err)
	}
	if created.Password != "" {
		t.Error("password must never be echoed back")
	}
}

func TestGet_NotFound(t *testing.T) {
	_, _, err := newSvc(&fakeRepo{getErr: sql.ErrNoRows}).Get(context.Background(), "x")
	if status(err) != 404 {
		t.Fatalf("missing tracker should be 404, got %v", err)
	}
}

func TestGet_EnrichesDriftDetected(t *testing.T) {
	repo := &fakeRepo{getResult: &models.ReleaseTracker{ID: "t1", UpdateAction: "compose"}, driftResult: true}
	tracker, _, err := newSvc(repo).Get(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !tracker.DriftDetected {
		t.Error("expected DriftDetected to be enriched from the repository")
	}
}

func TestList_EnrichesDriftDetectedPerTracker(t *testing.T) {
	repo := &fakeRepo{
		listResult:  []models.ReleaseTracker{{ID: "t1"}, {ID: "t2"}},
		driftResult: true,
	}
	trackers, err := newSvc(repo).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, tr := range trackers {
		if !tr.DriftDetected {
			t.Errorf("tracker %s: expected DriftDetected to be enriched", tr.ID)
		}
	}
}
