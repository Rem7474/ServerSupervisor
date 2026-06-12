package apt

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	status    *models.AptStatus
	statusErr error
}

func (f *fakeRepo) GetAptCVESummary(context.Context) (*models.AptCVESummary, error) {
	return &models.AptCVESummary{}, nil
}
func (f *fakeRepo) GetAptStatus(context.Context, string) (*models.AptStatus, error) {
	return f.status, f.statusErr
}
func (f *fakeRepo) GetUUStatus(context.Context, string) (*models.UnattendedUpgradesDB, error) {
	return nil, nil
}
func (f *fakeRepo) GetUURuns(context.Context, string, int) ([]models.UURun, error) {
	return nil, nil
}

type fakeDispatcher struct {
	reqs []dispatch.Request
	n    int
}

func (f *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	f.reqs = append(f.reqs, req)
	f.n++
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd"}}, nil
}

func TestCommand_DispatchesWithAudit(t *testing.T) {
	disp := &fakeDispatcher{}
	id, err := NewService(&fakeRepo{}, disp).Command(context.Background(), "h1", "update", "alice", "1.2.3.4")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if id != "cmd" {
		t.Errorf("command id = %q", id)
	}
	r := disp.reqs[0]
	if r.Module != "apt" || r.Action != "update" || r.Audit == nil || r.Audit.Action != "apt_update" {
		t.Errorf("unexpected dispatch request: %+v", r)
	}
}

func TestConfigureUU_DispatchesConfigureAndToggle(t *testing.T) {
	disp := &fakeDispatcher{}
	ids, err := NewService(&fakeRepo{}, disp).ConfigureUU(context.Background(), "h1",
		models.UnattendedUpgradesConfigureRequest{Enabled: true}, "alice", "1.2.3.4")
	if err != nil {
		t.Fatalf("ConfigureUU: %v", err)
	}
	if len(ids) != 2 || disp.n != 2 {
		t.Fatalf("expected 2 dispatches, got %d (ids=%v)", disp.n, ids)
	}
	if disp.reqs[0].Action != "configure_uu" {
		t.Errorf("first dispatch should be configure_uu, got %q", disp.reqs[0].Action)
	}
	if disp.reqs[1].Action != "toggle_uu" || disp.reqs[1].Target != "enable" {
		t.Errorf("second dispatch should be toggle_uu/enable, got %q/%q", disp.reqs[1].Action, disp.reqs[1].Target)
	}
}

func TestStatus_NotFound(t *testing.T) {
	_, err := NewService(&fakeRepo{statusErr: errors.New("no rows")}, &fakeDispatcher{}).Status(context.Background(), "h1")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("missing status should be apperr 404, got %v", err)
	}
}
