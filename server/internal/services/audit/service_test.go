package audit

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	cmd       *models.RemoteCommand
	cmdErr    error
	total     int64
	logs      []models.AuditLog
	cmds      []models.RemoteCommand
	incidents []database.AlertIncidentWithRule
}

func (f *fakeRepo) GetAuditLogs(context.Context, int, int) ([]models.AuditLog, error) {
	return nil, nil
}
func (f *fakeRepo) GetAuditLogsByHost(context.Context, string, int) ([]models.AuditLog, error) {
	return f.logs, nil
}
func (f *fakeRepo) GetAuditLogsByUser(context.Context, string, int) ([]models.AuditLog, error) {
	return nil, nil
}
func (f *fakeRepo) GetAllRemoteCommands(context.Context, int, int, database.CommandFilter) ([]database.RemoteCommandWithHost, error) {
	return nil, nil
}
func (f *fakeRepo) CountAllRemoteCommands(context.Context, database.CommandFilter) (int64, error) {
	return f.total, nil
}
func (f *fakeRepo) GetRemoteCommandByID(context.Context, string) (*models.RemoteCommand, error) {
	return f.cmd, f.cmdErr
}
func (f *fakeRepo) CancelRemoteCommand(context.Context, string) (bool, error) { return true, nil }
func (f *fakeRepo) GetRecentCommandsByHost(context.Context, string, int) ([]models.RemoteCommand, error) {
	return f.cmds, nil
}
func (f *fakeRepo) GetAlertIncidentsByHost(context.Context, string, int) ([]database.AlertIncidentWithRule, error) {
	return f.incidents, nil
}

func TestCommand_NotFoundMapsToAppErr(t *testing.T) {
	svc := NewService(&fakeRepo{cmdErr: errors.New("no rows")})
	_, err := svc.Command(context.Background(), "missing")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("missing command should be apperr 404, got %v", err)
	}
}

func TestLogs_NeverNil(t *testing.T) {
	svc := NewService(&fakeRepo{})
	got, err := svc.Logs(context.Background(), 50, 0)
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if got == nil {
		t.Error("Logs must return a non-nil slice")
	}
}

func TestHostTimeline_IncludesTriggeringUser(t *testing.T) {
	repo := &fakeRepo{
		logs: []models.AuditLog{
			{ID: 1, Action: "run_apt_command", Username: "alice", Status: "success"},
		},
		cmds: []models.RemoteCommand{
			{ID: "cmd-1", Module: "apt", Action: "upgrade", Status: "completed", TriggeredBy: "bob"},
			{ID: "cmd-2", Module: "apt", Action: "update", Status: "completed", TriggeredBy: "system"},
		},
	}
	svc := NewService(repo)

	events, err := svc.HostTimeline(context.Background(), "h1", 50)
	if err != nil {
		t.Fatalf("HostTimeline: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3", len(events))
	}

	byID := map[string]models.HostTimelineEvent{}
	for _, e := range events {
		byID[e.ID] = e
	}
	if got := byID["1"].User; got != "alice" {
		t.Errorf("audit event User = %q, want alice", got)
	}
	if got := byID["cmd-1"].User; got != "bob" {
		t.Errorf("manually-triggered command event User = %q, want bob", got)
	}
	if got := byID["cmd-2"].User; got != "system" {
		t.Errorf("automated command event User = %q, want system", got)
	}
}

func TestCommands_NonNilAndTotal(t *testing.T) {
	svc := NewService(&fakeRepo{total: 7})
	cmds, total, err := svc.Commands(context.Background(), 50, 0, database.CommandFilter{})
	if err != nil {
		t.Fatalf("Commands: %v", err)
	}
	if cmds == nil {
		t.Error("Commands must return a non-nil slice")
	}
	if total != 7 {
		t.Errorf("total = %d, want 7", total)
	}
}
