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
	created   bool
	getErr    error
	getResult *models.ReleaseTracker
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
	return nil, nil
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
func (f *fakeRepo) ListTrackerTagDigests(context.Context, string, int) ([]models.ReleaseVersionHistoryItem, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateReleaseTrackerExecutionByCommandID(context.Context, string, string) (string, bool, []string, error) {
	return "", false, nil, nil
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
