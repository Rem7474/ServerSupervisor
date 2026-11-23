package system

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeDispatcher struct {
	got dispatch.Request
	err error
}

func (f *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	f.got = req
	if f.err != nil {
		return nil, f.err
	}
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd-1"}}, nil
}

var actor = Actor{Username: "alice", IPAddress: "10.0.0.1"}

func TestJournalLogs_BuildsCommandAndAudit(t *testing.T) {
	d := &fakeDispatcher{}
	id, err := NewService(d).JournalLogs(context.Background(), actor, "h1", "nginx.service")
	if err != nil || id != "cmd-1" {
		t.Fatalf("id=%q err=%v", id, err)
	}
	if d.got.Module != "journal" || d.got.Action != "logs" || d.got.Target != "nginx.service" || d.got.HostID != "h1" {
		t.Errorf("unexpected request: %+v", d.got)
	}
	if d.got.Audit == nil || d.got.Audit.Action != "journalctl" || d.got.Audit.IPAddress != "10.0.0.1" || d.got.Audit.Username != "alice" {
		t.Errorf("unexpected audit: %+v", d.got.Audit)
	}
}

func TestJournalLogs_RejectsInvalidServiceName(t *testing.T) {
	d := &fakeDispatcher{}
	_, err := NewService(d).JournalLogs(context.Background(), actor, "h1", "nginx; rm -rf /")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("want a 400 validation error, got %v", err)
	}
	if d.got.Module != "" {
		t.Error("nothing must be dispatched for an invalid service name")
	}
}

func TestProcessesList_DispatchesWithoutTarget(t *testing.T) {
	d := &fakeDispatcher{}
	if _, err := NewService(d).ProcessesList(context.Background(), actor, "h1"); err != nil {
		t.Fatal(err)
	}
	if d.got.Module != "processes" || d.got.Action != "list" || d.got.Target != "" || d.got.Audit.Action != "processes_list" {
		t.Errorf("unexpected request: %+v", d.got)
	}
}

func TestSystemd(t *testing.T) {
	tests := []struct {
		name, service, action string
		wantErr               bool
	}{
		{"list needs no service", "", "list", false},
		{"restart with service", "nginx", "restart", false},
		{"restart without service", "", "restart", true},
		{"restart with invalid service", "a b", "restart", true},
		{"unknown action", "nginx", "reboot", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &fakeDispatcher{}
			_, err := NewService(d).Systemd(context.Background(), actor, "h1", tc.service, tc.action)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && (d.got.Module != "systemd" || d.got.Action != tc.action || d.got.Audit.Action != "systemd_"+tc.action) {
				t.Errorf("unexpected request: %+v", d.got)
			}
		})
	}
}

func TestCreate_DispatcherFailureIsGeneric(t *testing.T) {
	d := &fakeDispatcher{err: errors.New("db password leaked in error")}
	_, err := NewService(d).ProcessesList(context.Background(), actor, "h1")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 500 || ae.Message != "failed to create command" {
		t.Fatalf("want a generic 500, got %v", err)
	}
}
