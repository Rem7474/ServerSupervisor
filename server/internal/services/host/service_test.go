package host

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

type fakeRepo struct {
	registered    *models.Host
	registeredAll []*models.Host
	registerErrIP string // if set, RegisterHost fails for this IP only
	host          *models.Host
	getErr        error
	agentCmds     []models.RemoteCommand

	exposureResult *models.HostExposure
	gotExposureIP  string
	gotExposureAt  time.Time
}

func (f *fakeRepo) RegisterHost(_ context.Context, h *models.Host) error {
	if f.registerErrIP != "" && h.IPAddress == f.registerErrIP {
		return errors.New("db: duplicate ip")
	}
	f.registered = h
	f.registeredAll = append(f.registeredAll, h)
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
func (f *fakeRepo) GetLatestNetworkFlowMetrics(context.Context, string) ([]models.NetworkFlowMetric, error) {
	return nil, nil
}
func (f *fakeRepo) GetNetworkFlowsHistory(context.Context, string, string, int, string, int) ([]models.NetworkFlowSummaryPoint, error) {
	return nil, nil
}
func (f *fakeRepo) GetNetworkFlowsSummary(context.Context, string, int) ([]models.NetworkFlowSummaryPoint, error) {
	return nil, nil
}
func (f *fakeRepo) GetRecentCommandsByHost(context.Context, string, int) ([]models.RemoteCommand, error) {
	return nil, nil
}
func (f *fakeRepo) GetHostExposure(_ context.Context, ip string, since time.Time) (*models.HostExposure, error) {
	f.gotExposureIP = ip
	f.gotExposureAt = since
	if f.exposureResult != nil {
		return f.exposureResult, nil
	}
	return &models.HostExposure{IPAddress: ip, Since: since, Domains: []models.HostExposedDomain{}}, nil
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

func TestRegisterBulk_EmptyRejected(t *testing.T) {
	_, _, err := newSvc(&fakeRepo{}, &fakeDispatcher{}).RegisterBulk(context.Background(), nil)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("empty batch should be apperr 400, got %v", err)
	}
}

func TestRegisterBulk_TooManyRejected(t *testing.T) {
	reqs := make([]models.HostRegistration, 255)
	for i := range reqs {
		reqs[i] = models.HostRegistration{Name: "h", IPAddress: "10.0.0.1"}
	}
	_, _, err := newSvc(&fakeRepo{}, &fakeDispatcher{}).RegisterBulk(context.Background(), reqs)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("oversized batch should be apperr 400, got %v", err)
	}
}

func TestRegisterBulk_PartialFailureDoesNotBlockOthers(t *testing.T) {
	repo := &fakeRepo{registerErrIP: "10.0.0.2"}
	reqs := []models.HostRegistration{
		{Name: "a", IPAddress: "10.0.0.1"},
		{Name: "bad-ip", IPAddress: "not-an-ip"},
		{Name: "b", IPAddress: "10.0.0.2"}, // rejected by the fake repo
		{Name: "c", IPAddress: "10.0.0.3"},
	}
	created, results, err := newSvc(repo, &fakeDispatcher{}).RegisterBulk(context.Background(), reqs)
	if err != nil {
		t.Fatalf("RegisterBulk: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected 2 created, got %d", created)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	if !results[0].Created || results[0].HostID == "" || results[0].APIKey == "" {
		t.Errorf("entry a should have been created with an id and key, got %+v", results[0])
	}
	if results[1].Created || results[1].Error == "" {
		t.Errorf("bad-ip entry should have failed with an error, got %+v", results[1])
	}
	if results[2].Created || results[2].Error == "" {
		t.Errorf("duplicate-ip entry should have failed with an error, got %+v", results[2])
	}
	if !results[3].Created {
		t.Errorf("entry c should have been created, got %+v", results[3])
	}
	if len(repo.registeredAll) != 2 {
		t.Fatalf("expected 2 hosts persisted, got %d", len(repo.registeredAll))
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

func TestExposure_HostNotFound(t *testing.T) {
	repo := &fakeRepo{getErr: errors.New("no rows")}
	_, err := newSvc(repo, &fakeDispatcher{}).Exposure(context.Background(), "missing", time.Hour)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("missing host should be apperr 404, got %v", err)
	}
}

func TestExposure_QueriesRepoByHostIPAndSetsHostID(t *testing.T) {
	repo := &fakeRepo{host: &models.Host{ID: "h1", IPAddress: "10.0.0.5"}}
	before := time.Now()
	result, err := newSvc(repo, &fakeDispatcher{}).Exposure(context.Background(), "h1", 24*time.Hour)
	if err != nil {
		t.Fatalf("Exposure: %v", err)
	}
	if repo.gotExposureIP != "10.0.0.5" {
		t.Errorf("expected repo to be queried with the host's own IP, got %q", repo.gotExposureIP)
	}
	if d := before.Add(-24 * time.Hour).Sub(repo.gotExposureAt); d > time.Second || d < -time.Second {
		t.Errorf("expected since ~= now-period, got %v (now was %v)", repo.gotExposureAt, before)
	}
	if result.HostID != "h1" {
		t.Errorf("expected HostID to be set to the requested host id, got %q", result.HostID)
	}
}

func TestExposure_PropagatesRepoResult(t *testing.T) {
	repo := &fakeRepo{
		host: &models.Host{ID: "h1", IPAddress: "10.0.0.5"},
		exposureResult: &models.HostExposure{
			IPAddress:     "10.0.0.5",
			Domains:       []models.HostExposedDomain{{DomainName: "app.example.com", Requests: 42}},
			TotalRequests: 42,
		},
	}
	result, err := newSvc(repo, &fakeDispatcher{}).Exposure(context.Background(), "h1", time.Hour)
	if err != nil {
		t.Fatalf("Exposure: %v", err)
	}
	if len(result.Domains) != 1 || result.Domains[0].Requests != 42 || result.TotalRequests != 42 {
		t.Errorf("expected repo result to be propagated through, got %+v", result)
	}
	if result.HostID != "h1" {
		t.Errorf("expected HostID to still be overwritten to the requested id, got %q", result.HostID)
	}
}
