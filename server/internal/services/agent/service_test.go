package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
)

// fakeRepo is a no-op Repository whose behavior is tuned via the few fields the
// individual tests care about; everything else returns zero values.
type fakeRepo struct {
	cmd                *models.RemoteCommand
	guestLink          *models.ProxmoxGuestLink
	dataFresh          bool
	cpuTempSource      bool
	fanRPMSource       bool
	aggType            string // records the aggregation type requested
	updatedCmdStatus   string
	touchedAptActions  []string
	upsertedApt        *models.AptStatus
	updatedSchedStatus string
	createdCompleted   bool
	createdAuditAction string
}

func (f *fakeRepo) GetHostStatus(context.Context, string) string                      { return "online" }
func (f *fakeRepo) UpdateHostStatus(context.Context, string, string) error            { return nil }
func (f *fakeRepo) FailRunningCommandsOnAgentReconnect(context.Context, string) error { return nil }
func (f *fakeRepo) CleanupHostStalledCommands(context.Context, string, int) error     { return nil }
func (f *fakeRepo) ClaimPendingRemoteCommands(context.Context, string) ([]models.PendingCommand, error) {
	return nil, nil
}
func (f *fakeRepo) GetProxmoxGuestLinkByHost(context.Context, string) (*models.ProxmoxGuestLink, error) {
	return f.guestLink, nil
}
func (f *fakeRepo) IsProxmoxGuestDataFresh(context.Context, string) (bool, error) {
	return f.dataFresh, nil
}
func (f *fakeRepo) IsHostUsedAsProxmoxCPUTempSource(context.Context, string) bool {
	return f.cpuTempSource
}
func (f *fakeRepo) IsHostUsedAsProxmoxFanRPMSource(context.Context, string) bool {
	return f.fanRPMSource
}
func (f *fakeRepo) UpdateHost(context.Context, string, *models.HostUpdate) error      { return nil }
func (f *fakeRepo) InsertUptimeMetrics(context.Context, string, uint64, string) error { return nil }
func (f *fakeRepo) InsertMetrics(context.Context, *models.SystemMetrics) (int64, error) {
	return 1, nil
}
func (f *fakeRepo) UpsertDockerContainers(context.Context, string, []models.DockerContainer) error {
	return nil
}
func (f *fakeRepo) UpsertUUStatus(context.Context, string, models.UnattendedUpgradesStatus) error {
	return nil
}
func (f *fakeRepo) InsertUURunIfNew(context.Context, string, models.UURun) (bool, error) {
	return false, nil
}
func (f *fakeRepo) UpdateUULastRun(context.Context, string, time.Time, int) error { return nil }
func (f *fakeRepo) TouchAptLastAction(_ context.Context, _ string, command string) error {
	f.touchedAptActions = append(f.touchedAptActions, command)
	return nil
}
func (f *fakeRepo) TouchAptLastUpgradeAt(context.Context, string, time.Time) error { return nil }
func (f *fakeRepo) GetHost(context.Context, string) (*models.Host, error)          { return nil, nil }
func (f *fakeRepo) UpsertDockerNetworks(context.Context, string, []models.DockerNetwork) error {
	return nil
}
func (f *fakeRepo) UpsertComposeProjects(context.Context, string, []models.ComposeProject) error {
	return nil
}
func (f *fakeRepo) InsertDiskMetrics(context.Context, []models.DiskMetrics) error         { return nil }
func (f *fakeRepo) InsertDiskHealth(context.Context, []models.DiskHealth) error           { return nil }
func (f *fakeRepo) UpdateHostCustomTasks(context.Context, string, string) error           { return nil }
func (f *fakeRepo) UpdateHostTasksConfigYAML(context.Context, string, string) error       { return nil }
func (f *fakeRepo) UpdateHostCollectors(context.Context, string, string) error            { return nil }
func (f *fakeRepo) UpdateHostWebLogs(context.Context, string, *models.WebLogReport) error { return nil }
func (f *fakeRepo) InsertWebLogSnapshot(context.Context, string, *models.WebLogReport) error {
	return nil
}
func (f *fakeRepo) GetRemoteCommandByID(context.Context, string) (*models.RemoteCommand, error) {
	if f.cmd == nil {
		return nil, apperr.NotFound("not found")
	}
	return f.cmd, nil
}
func (f *fakeRepo) UpdateRemoteCommandStatus(_ context.Context, _, status, _ string) error {
	f.updatedCmdStatus = status
	return nil
}
func (f *fakeRepo) UpdateAuditLogStatus(context.Context, int64, string, string) error { return nil }
func (f *fakeRepo) UpdateScheduledTaskStatus(_ context.Context, _, status string) error {
	f.updatedSchedStatus = status
	return nil
}
func (f *fakeRepo) UpsertAptStatus(_ context.Context, status *models.AptStatus) error {
	f.upsertedApt = status
	return nil
}
func (f *fakeRepo) GetRecentCommandsByHost(context.Context, string, int) ([]models.RemoteCommand, error) {
	return nil, nil
}
func (f *fakeRepo) GetMetricsHistory(context.Context, string, int) ([]models.SystemMetrics, error) {
	f.aggType = "raw"
	return nil, nil
}
func (f *fakeRepo) GetMetricsAggregatesByType(_ context.Context, _ string, _ int, agg string) ([]models.SystemMetrics, error) {
	f.aggType = agg
	return nil, nil
}
func (f *fakeRepo) GetMetricsSummary(context.Context, int, int) ([]models.SystemMetricsSummary, error) {
	return nil, nil
}
func (f *fakeRepo) CreateAuditLog(_ context.Context, _, action, _, _, _, _ string) (int64, error) {
	f.createdAuditAction = action
	return 7, nil
}
func (f *fakeRepo) CreateCompletedRemoteCommand(context.Context, string, string, string, string, string, string, string, *int64) error {
	f.createdCompleted = true
	return nil
}

// recordingStreamHub captures status/chunk broadcasts.
type recordingStreamHub struct {
	statusBroadcasts int
	chunkBroadcasts  int
}

func (r *recordingStreamHub) Broadcast(string, string)               { r.chunkBroadcasts++ }
func (r *recordingStreamHub) BroadcastStatus(string, string, string) { r.statusBroadcasts++ }

func newSvc(repo Repository, hub StreamHub) *Service {
	return NewService(repo, &config.Config{}, hub, nil, nil)
}

func TestProxmoxIsMetricsSource(t *testing.T) {
	tests := []struct {
		name string
		repo *fakeRepo
		want bool
	}{
		{"no link", &fakeRepo{}, false},
		{"proxmox source", &fakeRepo{guestLink: &models.ProxmoxGuestLink{MetricsSource: "proxmox"}}, true},
		{"auto + fresh", &fakeRepo{guestLink: &models.ProxmoxGuestLink{MetricsSource: "auto"}, dataFresh: true}, true},
		{"auto + stale", &fakeRepo{guestLink: &models.ProxmoxGuestLink{MetricsSource: "auto"}, dataFresh: false}, false},
		{"agent source", &fakeRepo{guestLink: &models.ProxmoxGuestLink{MetricsSource: "agent"}}, false},
		{"proxmox but CPU-temp source overrides", &fakeRepo{guestLink: &models.ProxmoxGuestLink{MetricsSource: "proxmox"}, cpuTempSource: true}, false},
		{"proxmox but fan-RPM source overrides", &fakeRepo{guestLink: &models.ProxmoxGuestLink{MetricsSource: "proxmox"}, fanRPMSource: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSvc(tt.repo, &recordingStreamHub{})
			if got := s.proxmoxIsMetricsSource(context.Background(), "h1"); got != tt.want {
				t.Fatalf("proxmoxIsMetricsSource = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricsAggregated_SelectsAggregationType(t *testing.T) {
	cases := []struct {
		hours int
		want  string
	}{
		{1, "raw"}, {24, "raw"}, {25, "hour"}, {720, "hour"}, {721, "day"}, {8760, "day"},
	}
	for _, tc := range cases {
		repo := &fakeRepo{}
		s := newSvc(repo, &recordingStreamHub{})
		_, agg, err := s.MetricsAggregated(context.Background(), "h1", tc.hours)
		if err != nil {
			t.Fatalf("hours=%d: %v", tc.hours, err)
		}
		if agg != tc.want {
			t.Errorf("hours=%d agg=%q want %q", tc.hours, agg, tc.want)
		}
	}
}

func TestReceiveReport_PublishesSnapshotTopics(t *testing.T) {
	bus := events.NewBus()
	dash, unsubDash := bus.Subscribe(events.TopicDashboard)
	defer unsubDash()
	host, unsubHost := bus.Subscribe(events.HostTopic("h1"))
	defer unsubHost()

	s := NewService(&fakeRepo{}, &config.Config{}, &recordingStreamHub{}, nil, bus)
	if _, err := s.ReceiveReport(context.Background(), "h1", "h1", &models.AgentReport{}); err != nil {
		t.Fatalf("ReceiveReport: %v", err)
	}

	select {
	case <-dash:
	case <-time.After(time.Second):
		t.Fatal("an agent report must wake dashboard subscribers")
	}
	select {
	case <-host:
	case <-time.After(time.Second):
		t.Fatal("an agent report must wake the reporting host's subscribers")
	}
}

func TestReportCommandResult_ForbiddenWhenNotOwned(t *testing.T) {
	repo := &fakeRepo{cmd: &models.RemoteCommand{HostID: "other-host"}}
	s := newSvc(repo, &recordingStreamHub{})
	err := s.ReportCommandResult(context.Background(), "my-host", models.CommandResult{CommandID: "c1", Status: "completed"})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 403 {
		t.Fatalf("want 403 forbidden, got %v", err)
	}
}

func TestReportCommandResult_AptPostProcessingAndStream(t *testing.T) {
	hub := &recordingStreamHub{}
	repo := &fakeRepo{cmd: &models.RemoteCommand{HostID: "h1", Module: "apt", Action: "upgrade"}}
	s := newSvc(repo, hub)

	err := s.ReportCommandResult(context.Background(), "h1", models.CommandResult{
		CommandID: "c1",
		Status:    "completed",
		AptStatus: &models.AptStatus{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hub.statusBroadcasts != 1 {
		t.Errorf("status broadcasts = %d, want 1", hub.statusBroadcasts)
	}
	if len(repo.touchedAptActions) != 1 || repo.touchedAptActions[0] != "upgrade" {
		t.Errorf("touched apt actions = %v, want [upgrade]", repo.touchedAptActions)
	}
	if repo.upsertedApt == nil || repo.upsertedApt.HostID != "h1" {
		t.Errorf("apt status not upserted with host id, got %+v", repo.upsertedApt)
	}
}

func TestReportCommandResult_ScheduledTaskUpdated(t *testing.T) {
	taskID := "task-1"
	repo := &fakeRepo{cmd: &models.RemoteCommand{HostID: "h1", Module: "custom", ScheduledTaskID: &taskID}}
	s := newSvc(repo, &recordingStreamHub{})
	if err := s.ReportCommandResult(context.Background(), "h1", models.CommandResult{CommandID: "c1", Status: "failed"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.updatedSchedStatus != "failed" {
		t.Errorf("scheduled task status = %q, want failed", repo.updatedSchedStatus)
	}
}

func TestReportCommandResult_CompletionFanOut(t *testing.T) {
	repo := &fakeRepo{cmd: &models.RemoteCommand{HostID: "h1", Module: "docker"}}
	s := newSvc(repo, &recordingStreamHub{})

	var mu sync.Mutex
	var gotID, gotStatus string
	done := make(chan struct{})
	s.AddCompletionListener(listenerFunc(func(id, status string) {
		mu.Lock()
		gotID, gotStatus = id, status
		mu.Unlock()
		close(done)
	}))

	if err := s.ReportCommandResult(context.Background(), "h1", models.CommandResult{CommandID: "c1", Status: "completed"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("completion listener was not called")
	}
	mu.Lock()
	defer mu.Unlock()
	if gotID != "c1" || gotStatus != "completed" {
		t.Errorf("listener got (%q,%q), want (c1,completed)", gotID, gotStatus)
	}
}

func TestStreamCommandOutput_ForbiddenWhenNotOwned(t *testing.T) {
	repo := &fakeRepo{cmd: &models.RemoteCommand{HostID: "other"}}
	hub := &recordingStreamHub{}
	s := newSvc(repo, hub)
	err := s.StreamCommandOutput(context.Background(), "h1", "c1", "chunk")
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 403 {
		t.Fatalf("want 403, got %v", err)
	}
	if hub.chunkBroadcasts != 0 {
		t.Errorf("chunk must not be broadcast when ownership fails")
	}
}

func TestLogAuditAction_WithModuleCreatesCompletedCommand(t *testing.T) {
	repo := &fakeRepo{}
	s := newSvc(repo, &recordingStreamHub{})
	if err := s.LogAuditAction(context.Background(), "h1", "apt", "update", "completed", "details", "1.2.3.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createdAuditAction != "apt_update" {
		t.Errorf("audit action = %q, want apt_update", repo.createdAuditAction)
	}
	if !repo.createdCompleted {
		t.Error("expected a completed remote_command to be created when module is set")
	}
	// apt "update completed" must touch the last-action timestamp.
	found := false
	for _, a := range repo.touchedAptActions {
		if a == "update" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected apt update to be touched, got %v", repo.touchedAptActions)
	}
}

func TestLogAuditAction_WithoutModuleNoCommand(t *testing.T) {
	repo := &fakeRepo{}
	s := newSvc(repo, &recordingStreamHub{})
	if err := s.LogAuditAction(context.Background(), "h1", "", "login", "completed", "", "1.2.3.4"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.createdAuditAction != "login" {
		t.Errorf("audit action = %q, want login", repo.createdAuditAction)
	}
	if repo.createdCompleted {
		t.Error("no remote_command should be created without a module")
	}
}

type listenerFunc func(commandID, status string)

func (f listenerFunc) HandleCommandCompletion(commandID, status string) { f(commandID, status) }
