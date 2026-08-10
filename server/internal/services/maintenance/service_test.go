package maintenance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	created *models.MaintenanceWindow
	stored  *models.MaintenanceWindow
	getErr  error
	deleted string
}

func (f *fakeRepo) ListMaintenanceWindows(context.Context) ([]models.MaintenanceWindow, error) {
	return nil, nil
}
func (f *fakeRepo) ListMaintenanceWindowsForHost(context.Context, string) ([]models.MaintenanceWindow, error) {
	return nil, nil
}
func (f *fakeRepo) GetMaintenanceWindow(context.Context, string) (*models.MaintenanceWindow, error) {
	return f.stored, f.getErr
}
func (f *fakeRepo) CreateMaintenanceWindow(_ context.Context, w models.MaintenanceWindow) (*models.MaintenanceWindow, error) {
	cp := w
	f.created = &cp
	return &cp, nil
}
func (f *fakeRepo) DeleteMaintenanceWindow(_ context.Context, id string) error {
	f.deleted = id
	return nil
}

func validReq() models.MaintenanceWindowRequest {
	now := time.Now()
	return models.MaintenanceWindowRequest{
		Reason:   "planned upgrade",
		StartsAt: now,
		EndsAt:   now.Add(time.Hour),
	}
}

func TestListAll_NeverReturnsNil(t *testing.T) {
	svc := NewService(&fakeRepo{})
	windows, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if windows == nil {
		t.Error("ListAll should return an empty slice, not nil, when the repo returns nil")
	}
}

func TestCreateForHost_RejectsMissingReason(t *testing.T) {
	svc := NewService(&fakeRepo{})
	req := validReq()
	req.Reason = ""
	_, err := svc.CreateForHost(context.Background(), "host-1", "tester", req)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("CreateForHost with empty reason: err = %v, want apperr 400", err)
	}
}

func TestCreateForHost_RejectsEndBeforeStart(t *testing.T) {
	svc := NewService(&fakeRepo{})
	req := validReq()
	req.EndsAt = req.StartsAt.Add(-time.Minute)
	_, err := svc.CreateForHost(context.Background(), "host-1", "tester", req)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("CreateForHost with ends_at before starts_at: err = %v, want apperr 400", err)
	}
}

func TestCreateForHost_SetsHostID(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if _, err := svc.CreateForHost(context.Background(), "host-1", "tester", validReq()); err != nil {
		t.Fatalf("CreateForHost: %v", err)
	}
	if repo.created.HostID == nil || *repo.created.HostID != "host-1" {
		t.Errorf("created.HostID = %v, want *\"host-1\"", repo.created.HostID)
	}
	if repo.created.CreatedBy != "tester" {
		t.Errorf("created.CreatedBy = %q, want tester", repo.created.CreatedBy)
	}
}

func TestCreateGlobal_LeavesHostIDNil(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if _, err := svc.CreateGlobal(context.Background(), "admin", validReq()); err != nil {
		t.Fatalf("CreateGlobal: %v", err)
	}
	if repo.created.HostID != nil {
		t.Errorf("created.HostID = %v, want nil for a global window", *repo.created.HostID)
	}
}

func TestDelete_NotFoundWhenRepoErrors(t *testing.T) {
	repo := &fakeRepo{getErr: errors.New("no rows")}
	svc := NewService(repo)
	err := svc.Delete(context.Background(), "missing")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("Delete on missing window: err = %v, want apperr 404", err)
	}
	if repo.deleted != "" {
		t.Error("DeleteMaintenanceWindow should not be called when Get fails")
	}
}
