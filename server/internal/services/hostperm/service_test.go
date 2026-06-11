package hostperm

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	setLevel string
	deleted  bool
	listNil  bool
}

func (f *fakeRepo) ListHostPermissions(context.Context, string) ([]models.HostPermission, error) {
	if f.listNil {
		return nil, nil
	}
	return []models.HostPermission{{}}, nil
}
func (f *fakeRepo) ListUserHostPermissions(context.Context, string) ([]models.HostPermission, error) {
	return nil, nil
}
func (f *fakeRepo) SetHostPermission(_ context.Context, _, _, level string) error {
	f.setLevel = level
	return nil
}
func (f *fakeRepo) DeleteHostPermission(context.Context, string, string) error {
	f.deleted = true
	return nil
}

func TestSet_RejectsInvalidLevel(t *testing.T) {
	repo := &fakeRepo{}
	err := NewService(repo).Set(context.Background(), "alice", "h1", "admin")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("invalid level should be apperr 400, got %v", err)
	}
	if repo.setLevel != "" {
		t.Error("must not persist an invalid level")
	}
}

func TestSet_AcceptsValidLevels(t *testing.T) {
	for _, lvl := range []string{"viewer", "operator"} {
		repo := &fakeRepo{}
		if err := NewService(repo).Set(context.Background(), "alice", "h1", lvl); err != nil {
			t.Fatalf("Set(%q): %v", lvl, err)
		}
		if repo.setLevel != lvl {
			t.Errorf("level %q not persisted, got %q", lvl, repo.setLevel)
		}
	}
}

func TestList_NeverNil(t *testing.T) {
	got, err := NewService(&fakeRepo{listNil: true}).List(context.Background(), "h1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got == nil {
		t.Error("List must return a non-nil slice")
	}
}
