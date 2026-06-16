package host

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	registered *models.Host
	host       *models.Host
	getErr     error
	agentCmds  []models.RemoteCommand
}

func (f *fakeRepo) RegisterHost(_ context.Context, h *models.Host) error {
	f.registered = h
	return nil
}
func (f *fakeRepo) GetAllHosts(context.Context) ([]models.Host, error) { return nil, nil }
func (f *fakeRepo) GetHost(context.Context, string) (*models.Host, error) {
	return f.host, f.getErr
}
func (f *fakeRepo) UpdateHost(context.Context, string, *models.HostUpdate) error { return nil }
func (f *fakeRepo) DeleteHost(context.Context, string) error                     { return nil }
func (f *fakeRepo) UpdateHostAPIKey(context.Context, string, string) error       { return nil }
func (f *fakeRepo) GetRemoteCommandsByHostAndModule(context.Context, string, string, int) ([]models.RemoteCommand, error) {
	return f.agentCmds, nil
}
func (f *fakeRepo) GetLatestMetrics(context.Context, string) (*models.SystemMetrics, error) {
	return nil, nil
}
func (f *fakeRepo) GetEffectiveHostCPUTemperature(context.Context, string, float64) (float64, bool) {
	return 0, false
}
func (f *fakeRepo) GetDockerContainers(context.Context, string) ([]models.DockerContainer, error) {
	return nil, nil
}
func (f *fakeRepo) GetAptStatus(context.Context, string) (*models.AptStatus, error) { return nil, nil }
func (f *fakeRepo) GetLatestDiskMetrics(context.Context, string) ([]models.DiskMetrics, error) {
	return nil, nil
}
func (f *fakeRepo) GetLatestDiskHealth(context.Context, string) ([]models.DiskHealth, error) {
	return nil, nil
}
func (f *fakeRepo) GetDiskMetricsHistory(context.Context, string, string, int) ([]models.DiskMetrics, error) {
	return nil, nil
}
func (f *fakeRepo) GetDiskMetricsAggregated(context.Context, string, string, int) ([]models.DiskMetrics, string, error) {
	return nil, "raw", nil
}
func (f *fakeRepo) GetRecentCommandsByHost(context.Context, string, int) ([]models.RemoteCommand, error) {
	return nil, nil
}

type fakeDispatcher struct{ gotReq dispatch.Request }

func (f *fakeDispatcher) Create(_ context.Context, req dispatch.Request) (*dispatch.Result, error) {
	f.gotReq = req
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd-1"}}, nil
}

func newSvc(repo Repository, disp Dispatcher) *Service {
	return NewService(repo, disp, func() string { return "v2.0.0" }, nil)
}

func TestRegister_InvalidIP(t *testing.T) {
	_, _, err := newSvc(&fakeRepo{}, &fakeDispatcher{}).Register(context.Background(), models.HostRegistration{Name: "x", IPAddress: "not-an-ip"})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("invalid IP should be apperr 400, got %v", err)
	}
}

func TestRegister_GeneratesKeyAndPersists(t *testing.T) {
	repo := &fakeRepo{}
	id, plainKey, err := newSvc(repo, &fakeDispatcher{}).Register(context.Background(), models.HostRegistration{Name: "web", IPAddress: "10.0.0.1", Tags: []string{"prod"}})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if repo.registered == nil || repo.registered.ID != id {
		t.Fatal("host not persisted with the returned id")
	}
	if !strings.HasPrefix(plainKey, id+".") {
		t.Errorf("plain key should be {id}.{secret}, got %q", plainKey)
	}
	if repo.registered.APIKey == "" || repo.registered.APIKey == plainKey {
		t.Error("stored key must be a hash, not the plain key")
	}
	if repo.registered.Status != "offline" {
		t.Errorf("new host should start offline, got %q", repo.registered.Status)
	}
}

func TestUpdate_NoFields(t *testing.T) {
	_, err := newSvc(&fakeRepo{}, &fakeDispatcher{}).Update(context.Background(), "h1", models.HostUpdate{})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("empty update should be apperr 400, got %v", err)
	}
}

func TestTriggerAgentUpdate_AlreadyUpToDate(t *testing.T) {
	repo := &fakeRepo{host: &models.Host{ID: "h1", AgentVersion: "v2.0.0"}}
	_, _, err := newSvc(repo, &fakeDispatcher{}).TriggerAgentUpdate(context.Background(), "h1", "alice", "1.2.3.4")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "conflict" {
		t.Fatalf("already-current agent should be apperr conflict, got %v", err)
	}
}

func TestTriggerAgentUpdate_InProgress(t *testing.T) {
	repo := &fakeRepo{
		host:      &models.Host{ID: "h1", AgentVersion: "v1.0.0"},
		agentCmds: []models.RemoteCommand{{Action: "update", Status: "running"}},
	}
	_, _, err := newSvc(repo, &fakeDispatcher{}).TriggerAgentUpdate(context.Background(), "h1", "alice", "1.2.3.4")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != "conflict" {
		t.Fatalf("in-flight update should be apperr conflict, got %v", err)
	}
}

func TestTriggerAgentUpdate_Dispatches(t *testing.T) {
	repo := &fakeRepo{host: &models.Host{ID: "h1", AgentVersion: "v1.0.0"}}
	disp := &fakeDispatcher{}
	cmdID, version, err := newSvc(repo, disp).TriggerAgentUpdate(context.Background(), "h1", "alice", "1.2.3.4")
	if err != nil {
		t.Fatalf("TriggerAgentUpdate: %v", err)
	}
	if cmdID != "cmd-1" || version != "v2.0.0" {
		t.Errorf("unexpected result: cmd=%q version=%q", cmdID, version)
	}
	if disp.gotReq.Module != "agent" || disp.gotReq.Action != "update" || disp.gotReq.Audit == nil {
		t.Errorf("dispatch request not built correctly: %+v", disp.gotReq)
	}
}
