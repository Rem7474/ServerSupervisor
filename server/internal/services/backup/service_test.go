package backup

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	created          *models.BackupRun
	updated          *models.BackupRun
	byCommandID      *models.BackupRun
	byCommandIDErr   error
	remoteCmd        *models.RemoteCommand
	remoteCmdErr     error
	getRunResult     *models.BackupRun
	getRunErr        error
	stalled          []models.BackupRun
	resticProfiles   string
	resticProfileErr error
}

func (f *fakeRepo) CreateBackupRun(_ context.Context, r models.BackupRun) (*models.BackupRun, error) {
	cp := r
	cp.ID = "run-1"
	f.created = &cp
	return &cp, nil
}
func (f *fakeRepo) GetBackupRun(context.Context, string) (*models.BackupRun, error) {
	return f.getRunResult, f.getRunErr
}
func (f *fakeRepo) ListBackupRuns(context.Context, string, int) ([]models.BackupRun, error) {
	return nil, nil
}
func (f *fakeRepo) GetLatestBackupRunByHost(context.Context, string) (*models.BackupRun, error) {
	return nil, sql.ErrNoRows
}
func (f *fakeRepo) GetBackupRunByCommandID(context.Context, string) (*models.BackupRun, error) {
	return f.byCommandID, f.byCommandIDErr
}
func (f *fakeRepo) UpdateBackupRun(_ context.Context, r models.BackupRun) error {
	cp := r
	f.updated = &cp
	return nil
}
func (f *fakeRepo) GetRemoteCommandByID(context.Context, string) (*models.RemoteCommand, error) {
	return f.remoteCmd, f.remoteCmdErr
}
func (f *fakeRepo) GetResticStatus(context.Context, string) (*models.ResticStatus, error) {
	return nil, sql.ErrNoRows
}
func (f *fakeRepo) ListStalledBackupRuns(context.Context, int) ([]models.BackupRun, error) {
	return f.stalled, nil
}
func (f *fakeRepo) GetHostResticProfiles(context.Context, string) (string, error) {
	return f.resticProfiles, f.resticProfileErr
}

type fakeDispatcher struct {
	lastReq  dispatch.Request
	commands int
}

func (f *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	f.commands++
	f.lastReq = req
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd-1", HostID: req.HostID, Module: req.Module, Action: req.Action}}, nil
}

func TestTriggerBackup_CreatesRunningRowAndDispatches(t *testing.T) {
	repo := &fakeRepo{}
	disp := &fakeDispatcher{}
	svc := NewService(repo, disp, nil, nil, nil)

	run, err := svc.TriggerBackup(context.Background(), "host-1", "files", "alice")
	if err != nil {
		t.Fatalf("TriggerBackup: %v", err)
	}
	if disp.commands != 1 {
		t.Fatalf("expected exactly one dispatched command, got %d", disp.commands)
	}
	if disp.lastReq.Module != "restic" || disp.lastReq.Action != "run_backup" {
		t.Errorf("expected module=restic action=run_backup, got module=%q action=%q", disp.lastReq.Module, disp.lastReq.Action)
	}
	if disp.lastReq.Target != "files" {
		t.Errorf("expected profile passed as Target, got %q", disp.lastReq.Target)
	}
	if run.CommandID == nil || *run.CommandID != "cmd-1" {
		t.Errorf("expected the created run to be linked to the dispatched command, got %+v", run.CommandID)
	}
	if repo.created.Status != "running" {
		t.Errorf("expected the initial row to be status=running, got %q", repo.created.Status)
	}
}

func TestTriggerBackup_RequiresHostID(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeDispatcher{}, nil, nil, nil)
	_, err := svc.TriggerBackup(context.Background(), "", "", "alice")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("expected apperr 400 validation error, got %v", err)
	}
}

func TestGetRun_NotFoundMapsToAppErr(t *testing.T) {
	svc := NewService(&fakeRepo{getRunErr: sql.ErrNoRows}, &fakeDispatcher{}, nil, nil, nil)
	_, err := svc.GetRun(context.Background(), "missing")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("expected apperr 404, got %v", err)
	}
}

func TestListRuns_NeverNil(t *testing.T) {
	got, err := NewService(&fakeRepo{}, &fakeDispatcher{}, nil, nil, nil).ListRuns(context.Background(), "host-1", 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if got == nil {
		t.Error("ListRuns must return a non-nil slice")
	}
}

func TestGetProfiles_ParsesCachedJSON(t *testing.T) {
	svc := NewService(&fakeRepo{resticProfiles: `["db","files"]`}, &fakeDispatcher{}, nil, nil, nil)
	got, err := svc.GetProfiles(context.Background(), "host-1")
	if err != nil {
		t.Fatalf("GetProfiles: %v", err)
	}
	if len(got) != 2 || got[0] != "db" || got[1] != "files" {
		t.Errorf("expected [db files], got %v", got)
	}
}

func TestGetProfiles_NeverNilWhenUncached(t *testing.T) {
	svc := NewService(&fakeRepo{resticProfiles: ""}, &fakeDispatcher{}, nil, nil, nil)
	got, err := svc.GetProfiles(context.Background(), "host-1")
	if err != nil {
		t.Fatalf("GetProfiles: %v", err)
	}
	if got == nil {
		t.Error("GetProfiles must return a non-nil slice")
	}
}

func TestHandleCommandCompletion_IgnoresNonResticCommands(t *testing.T) {
	repo := &fakeRepo{remoteCmd: &models.RemoteCommand{ID: "cmd-1", Module: "apt", Action: "upgrade"}}
	svc := NewService(repo, &fakeDispatcher{}, nil, nil, nil)

	svc.HandleCommandCompletion("cmd-1", "completed")

	if repo.created != nil || repo.updated != nil {
		t.Error("a non-restic command must never create or update a backup run")
	}
}

func TestHandleCommandCompletion_IgnoresNonRunBackupAction(t *testing.T) {
	repo := &fakeRepo{remoteCmd: &models.RemoteCommand{ID: "cmd-1", Module: "restic", Action: "status"}}
	svc := NewService(repo, &fakeDispatcher{}, nil, nil, nil)

	svc.HandleCommandCompletion("cmd-1", "completed")

	if repo.created != nil || repo.updated != nil {
		t.Error("a restic 'status' command must never create or update a backup run (only run_backup does)")
	}
}

func TestHandleCommandCompletion_UpdatesExistingRun(t *testing.T) {
	now := time.Now()
	existing := &models.BackupRun{ID: "run-1", HostID: "host-1"}
	repo := &fakeRepo{
		remoteCmd: &models.RemoteCommand{
			ID: "cmd-1", HostID: "host-1", Module: "restic", Action: "run_backup",
			Target: "files", Output: `{"status":"ok","snapshot_id":"abc123","files_new":3}`,
			CreatedAt: now, StartedAt: &now, EndedAt: &now,
		},
		byCommandID: existing,
	}
	svc := NewService(repo, &fakeDispatcher{}, nil, nil, nil)

	svc.HandleCommandCompletion("cmd-1", "completed")

	if repo.created != nil {
		t.Error("an existing backup run (manual trigger case) must be updated, not re-created")
	}
	if repo.updated == nil {
		t.Fatal("expected the existing backup run to be updated")
	}
	if repo.updated.ID != "run-1" {
		t.Errorf("expected the existing run's ID to be preserved, got %q", repo.updated.ID)
	}
	if repo.updated.Status != "ok" {
		t.Errorf("expected status=ok from the summary, got %q", repo.updated.Status)
	}
	if repo.updated.SnapshotID != "abc123" {
		t.Errorf("expected snapshot_id from the summary, got %q", repo.updated.SnapshotID)
	}
}

func TestHandleCommandCompletion_CreatesRunFromScheduledTask(t *testing.T) {
	// No pre-existing backup_runs row for this command — this is the
	// scheduled-task path, which dispatches directly via scheduledtask.Service
	// and never calls TriggerBackup.
	now := time.Now()
	repo := &fakeRepo{
		remoteCmd: &models.RemoteCommand{
			ID: "cmd-2", HostID: "host-2", Module: "restic", Action: "run_backup",
			Target: "db", Output: `{"status":"ok","snapshot_id":"def456"}`,
			CreatedAt: now, StartedAt: &now, EndedAt: &now, TriggeredBy: "scheduled_task",
		},
		byCommandIDErr: sql.ErrNoRows,
	}
	svc := NewService(repo, &fakeDispatcher{}, nil, nil, nil)

	svc.HandleCommandCompletion("cmd-2", "completed")

	if repo.updated != nil {
		t.Error("a command with no existing backup run must be created, not updated")
	}
	if repo.created == nil {
		t.Fatal("expected a new backup run to be created")
	}
	if repo.created.HostID != "host-2" || repo.created.Profile != "db" {
		t.Errorf("expected the new run to carry the command's host/profile, got %+v", repo.created)
	}
	if repo.created.TriggeredBy != "scheduled_task" {
		t.Errorf("expected triggered_by from the command, got %q", repo.created.TriggeredBy)
	}
}

func TestHandleCommandCompletion_FailedCommandMapsToErrorAndNotifies(t *testing.T) {
	now := time.Now()
	repo := &fakeRepo{
		remoteCmd: &models.RemoteCommand{
			ID: "cmd-3", HostID: "host-3", Module: "restic", Action: "run_backup",
			Output: "boom: repository locked", CreatedAt: now, StartedAt: &now, EndedAt: &now,
		},
		byCommandIDErr: sql.ErrNoRows,
	}
	// A real (zero-value) *config.Config is required here: the failure path
	// reaches notifychannels.Dispatcher.Send, which dereferences cfg directly.
	svc := NewService(repo, &fakeDispatcher{}, &config.Config{}, nil, nil)

	svc.HandleCommandCompletion("cmd-3", "failed")

	if repo.created == nil {
		t.Fatal("expected a backup run to be created even on failure")
	}
	if repo.created.Status != "error" {
		t.Errorf("expected a failed command to map to status=error, got %q", repo.created.Status)
	}
	if repo.created.ErrorMessage == "" {
		t.Error("expected a non-empty error message for a failed run")
	}
}

func TestHandleCommandCompletion_IgnoresNonTerminalStatus(t *testing.T) {
	repo := &fakeRepo{remoteCmd: &models.RemoteCommand{ID: "cmd-4", Module: "restic", Action: "run_backup"}}
	svc := NewService(repo, &fakeDispatcher{}, nil, nil, nil)

	svc.HandleCommandCompletion("cmd-4", "running")

	if repo.created != nil || repo.updated != nil {
		t.Error("a non-terminal status (running/pending) must not touch backup run history")
	}
}

func TestCheckStalledRuns_MarksErrorAndPersists(t *testing.T) {
	stalledStart := time.Now().Add(-8 * time.Hour)
	repo := &fakeRepo{
		stalled: []models.BackupRun{
			{ID: "run-stalled", HostID: "host-4", Status: "running", StartedAt: stalledStart},
		},
	}
	svc := NewService(repo, &fakeDispatcher{}, &config.Config{}, nil, nil)

	svc.CheckStalledRuns(context.Background(), 360)

	if repo.updated == nil {
		t.Fatal("expected the stalled run to be updated")
	}
	if repo.updated.Status != "error" {
		t.Errorf("expected stalled run to be marked status=error, got %q", repo.updated.Status)
	}
	if repo.updated.FinishedAt == nil {
		t.Error("expected FinishedAt to be set on a stalled run")
	}
}
