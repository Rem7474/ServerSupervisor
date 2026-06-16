// Package agent is the application/service layer for the agent↔server protocol:
// report ingestion, command-result handling (with completion fan-out), live output
// streaming, agent-reported audit, and the host metric reads. It sits behind a
// Repository port + small StreamHub/NotificationPusher ports so it is unit-testable
// without a database or a live WebSocket hub.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
)

// CommandCompletionListener reacts to a remote command terminal state update.
type CommandCompletionListener interface {
	HandleCommandCompletion(commandID, status string)
}

// NotificationPusher broadcasts real-time events to connected frontend clients.
type NotificationPusher interface {
	Broadcast(payload interface{})
}

// StreamHub relays live command output/status to WebSocket clients.
// *ws.CommandStreamHub satisfies it.
type StreamHub interface {
	Broadcast(commandID, chunk string)
	BroadcastStatus(commandID, status, output string)
}

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetHostStatus(ctx context.Context, id string) string
	UpdateHostStatus(ctx context.Context, id, status string) error
	FailRunningCommandsOnAgentReconnect(ctx context.Context, hostID string) error
	CleanupHostStalledCommands(ctx context.Context, hostID string, timeoutMinutes int) error
	ClaimPendingRemoteCommands(ctx context.Context, hostID string) ([]models.PendingCommand, error)
	GetProxmoxGuestLinkByHost(ctx context.Context, hostID string) (*models.ProxmoxGuestLink, error)
	IsProxmoxGuestDataFresh(ctx context.Context, hostID string) (bool, error)
	IsHostUsedAsProxmoxCPUTempSource(ctx context.Context, hostID string) bool
	IsHostUsedAsProxmoxFanRPMSource(ctx context.Context, hostID string) bool
	UpdateHost(ctx context.Context, id string, update *models.HostUpdate) error
	InsertUptimeMetrics(ctx context.Context, hostID string, uptime uint64, hostname string) error
	InsertMetrics(ctx context.Context, m *models.SystemMetrics) (int64, error)
	UpsertDockerContainers(ctx context.Context, hostID string, containers []models.DockerContainer) error
	UpsertUUStatus(ctx context.Context, hostID string, s models.UnattendedUpgradesStatus) error
	InsertUURunIfNew(ctx context.Context, hostID string, run models.UURun) (bool, error)
	UpdateUULastRun(ctx context.Context, hostID string, runAt time.Time, pkgCount int) error
	TouchAptLastAction(ctx context.Context, hostID, command string) error
	TouchAptLastUpgradeAt(ctx context.Context, hostID string, t time.Time) error
	GetHost(ctx context.Context, id string) (*models.Host, error)
	UpsertDockerNetworks(ctx context.Context, hostID string, networks []models.DockerNetwork) error
	UpsertComposeProjects(ctx context.Context, hostID string, projects []models.ComposeProject) error
	InsertDiskMetrics(ctx context.Context, metrics []models.DiskMetrics) error
	InsertDiskHealth(ctx context.Context, healthData []models.DiskHealth) error
	UpdateHostCustomTasks(ctx context.Context, hostID, tasksJSON string) error
	UpdateHostTasksConfigYAML(ctx context.Context, hostID, yaml string) error
	UpdateHostCollectors(ctx context.Context, hostID, collectorsJSON string) error
	UpdateHostWebLogs(ctx context.Context, hostID string, report *models.WebLogReport) error
	InsertWebLogSnapshot(ctx context.Context, hostID string, report *models.WebLogReport) error
	GetRemoteCommandByID(ctx context.Context, id string) (*models.RemoteCommand, error)
	UpdateRemoteCommandStatus(ctx context.Context, id, status, output string) error
	UpdateAuditLogStatus(ctx context.Context, id int64, status, details string) error
	UpdateScheduledTaskStatus(ctx context.Context, id, status string) error
	UpsertAptStatus(ctx context.Context, status *models.AptStatus) error
	GetRecentCommandsByHost(ctx context.Context, hostID string, limit int) ([]models.RemoteCommand, error)
	GetMetricsHistory(ctx context.Context, hostID string, hours int) ([]models.SystemMetrics, error)
	GetMetricsAggregatesByType(ctx context.Context, hostID string, hours int, aggregationType string) ([]models.SystemMetrics, error)
	GetMetricsSummary(ctx context.Context, hours, bucketMinutes int) ([]models.SystemMetricsSummary, error)
	CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error)
	CreateCompletedRemoteCommand(ctx context.Context, hostID, module, action, target, output, triggeredBy string, status string, auditLogID *int64) error
}

// Service holds the agent-protocol use-cases + the completion-listener registry.
type Service struct {
	repo                Repository
	cfg                 *config.Config
	streamHub           StreamHub
	notifPusher         NotificationPusher
	bus                 *events.Bus
	completionListeners []CommandCompletionListener
	completionMu        sync.RWMutex
}

func NewService(repo Repository, cfg *config.Config, streamHub StreamHub, notifPusher NotificationPusher, bus *events.Bus) *Service {
	return &Service{repo: repo, cfg: cfg, streamHub: streamHub, notifPusher: notifPusher, bus: bus}
}

func (s *Service) AddCompletionListener(listener CommandCompletionListener) {
	if listener == nil {
		return
	}
	s.completionMu.Lock()
	defer s.completionMu.Unlock()
	s.completionListeners = append(s.completionListeners, listener)
}

// ReportResult is the outcome of an agent report: the commands to run + whether
// the agent should stop collecting CPU/RAM (Proxmox is the metrics source).
type ReportResult struct {
	Commands    []models.PendingCommand
	SkipMetrics bool
}

// ReceiveReport ingests a full agent report and returns the pending commands.
// safeHostID is the log-sanitized host id (newlines stripped) for log lines.
func (s *Service) ReceiveReport(ctx context.Context, hostID, safeHostID string, report *models.AgentReport) (*ReportResult, error) {
	prevStatus := s.repo.GetHostStatus(ctx, hostID)

	if err := s.repo.UpdateHostStatus(ctx, hostID, "online"); err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to update host %s status to online: %v", safeHostID, err))
	}

	// On reconnect, fail any 'running' commands from the previous dead session.
	if prevStatus == "offline" {
		if err := s.repo.FailRunningCommandsOnAgentReconnect(ctx, hostID); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to cleanup running commands on reconnect for host %s: %v", safeHostID, err))
		}
	}

	if err := s.repo.CleanupHostStalledCommands(ctx, hostID, 10); err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to cleanup stalled commands for host %s: %v", safeHostID, err))
	}

	proxmoxIsMetricsSource := s.proxmoxIsMetricsSource(ctx, hostID)

	if err := s.storeMetrics(ctx, hostID, report, proxmoxIsMetricsSource); err != nil {
		return nil, apperr.Internal(err)
	}
	s.storeContainersAndPackages(ctx, hostID, safeHostID, report)
	s.storeDiskAndMetadata(ctx, hostID, safeHostID, report)

	commands, err := s.repo.ClaimPendingRemoteCommands(ctx, hostID)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to get pending commands for host %s: %v", safeHostID, err))
	}
	if commands == nil {
		commands = []models.PendingCommand{}
	}

	// A report refreshes everything the live views render for this host, so wake
	// the relevant snapshot subscribers (nil-safe when no bus is wired).
	s.bus.PublishAll(events.TopicDashboard, events.TopicDocker, events.TopicNetwork, events.TopicApt)
	s.bus.Publish(events.HostTopic(hostID))

	return &ReportResult{Commands: commands, SkipMetrics: proxmoxIsMetricsSource}, nil
}

// ReportCommandResult records a command's terminal result, updates linked records,
// streams the status and fans out to completion listeners.
func (s *Service) ReportCommandResult(ctx context.Context, hostID string, result models.CommandResult) error {
	cmd, err := s.repo.GetRemoteCommandByID(ctx, result.CommandID)
	if err != nil || cmd.HostID != hostID {
		return apperr.Forbidden("command does not belong to host")
	}
	if err := s.repo.UpdateRemoteCommandStatus(ctx, result.CommandID, result.Status, result.Output); err != nil {
		return apperr.Failed("failed to update command")
	}

	if cmd.AuditLogID != nil {
		details := ""
		if result.Status == "failed" {
			details = truncateOutput(result.Output, 2000)
		}
		if err := s.repo.UpdateAuditLogStatus(ctx, *cmd.AuditLogID, result.Status, details); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to update audit log %d for command %s: %v", *cmd.AuditLogID, result.CommandID, err))
		}
	}

	s.streamHub.BroadcastStatus(result.CommandID, result.Status, result.Output)

	if cmd.ScheduledTaskID != nil && (result.Status == "completed" || result.Status == "failed") {
		if err := s.repo.UpdateScheduledTaskStatus(ctx, *cmd.ScheduledTaskID, result.Status); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Failed to update scheduled task %s status: %v", *cmd.ScheduledTaskID, err))
		}
	}

	if result.Status == "completed" || result.Status == "failed" {
		s.completionMu.RLock()
		listeners := make([]CommandCompletionListener, len(s.completionListeners))
		copy(listeners, s.completionListeners)
		s.completionMu.RUnlock()
		for _, listener := range listeners {
			go listener.HandleCommandCompletion(result.CommandID, result.Status)
		}
	}

	if cmd.Module == "apt" && result.Status == "completed" {
		_ = s.repo.TouchAptLastAction(ctx, cmd.HostID, cmd.Action)
		if result.AptStatus != nil {
			result.AptStatus.HostID = cmd.HostID
			if err := s.repo.UpsertAptStatus(ctx, result.AptStatus); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Failed to update APT status: %v", err))
			}
		}
		s.bus.Publish(events.TopicApt)
		s.bus.Publish(events.HostTopic(cmd.HostID))
	}
	return nil
}

// StreamCommandOutput relays a live output chunk after verifying host ownership.
func (s *Service) StreamCommandOutput(ctx context.Context, hostID, commandID, chunk string) error {
	cmd, err := s.repo.GetRemoteCommandByID(ctx, commandID)
	if err != nil || cmd.HostID != hostID {
		return apperr.Forbidden("command does not belong to host")
	}
	s.streamHub.Broadcast(commandID, chunk)
	return nil
}

// HostCommandHistory returns recent commands for a host (never nil).
func (s *Service) HostCommandHistory(ctx context.Context, hostID string, limit int) ([]models.RemoteCommand, error) {
	cmds, err := s.repo.GetRecentCommandsByHost(ctx, hostID, limit)
	if err != nil {
		return nil, err
	}
	if cmds == nil {
		cmds = []models.RemoteCommand{}
	}
	return cmds, nil
}

// MetricsHistory returns raw metric history (never nil).
func (s *Service) MetricsHistory(ctx context.Context, hostID string, hours int) ([]models.SystemMetrics, error) {
	metrics, err := s.repo.GetMetricsHistory(ctx, hostID, hours)
	if err != nil {
		return nil, err
	}
	if metrics == nil {
		metrics = []models.SystemMetrics{}
	}
	return metrics, nil
}

// MetricsAggregated returns metrics with an aggregation chosen by time range and
// the aggregation type used (raw / hour / day).
func (s *Service) MetricsAggregated(ctx context.Context, hostID string, hours int) ([]models.SystemMetrics, string, error) {
	var metrics []models.SystemMetrics
	var err error
	var aggType string
	switch {
	case hours <= 24:
		metrics, err = s.repo.GetMetricsHistory(ctx, hostID, hours)
		aggType = "raw"
	case hours <= 720:
		metrics, err = s.repo.GetMetricsAggregatesByType(ctx, hostID, hours, "hour")
		aggType = "hour"
	default:
		metrics, err = s.repo.GetMetricsAggregatesByType(ctx, hostID, hours, "day")
		aggType = "day"
	}
	if err != nil {
		return nil, "", err
	}
	if metrics == nil {
		metrics = []models.SystemMetrics{}
	}
	return metrics, aggType, nil
}

// MetricsSummary returns the global metric summary (never nil).
func (s *Service) MetricsSummary(ctx context.Context, hours, bucketMinutes int) ([]models.SystemMetricsSummary, error) {
	summary, err := s.repo.GetMetricsSummary(ctx, hours, bucketMinutes)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		summary = []models.SystemMetricsSummary{}
	}
	return summary, nil
}

// LogAuditAction records an agent-reported action (+ a completed remote_command
// when a module is given so it appears in the commands history).
func (s *Service) LogAuditAction(ctx context.Context, hostID, module, action, status, details, clientIP string) error {
	auditAction := action
	if module != "" {
		auditAction = module + "_" + action
	}
	auditLogID, err := s.repo.CreateAuditLog(ctx, "agent", auditAction, hostID, clientIP, details, status)
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Failed to log audit action: %v", err))
		return apperr.Failed("failed to record audit log")
	}
	if module != "" {
		var auditIDPtr *int64
		if auditLogID != 0 {
			auditIDPtr = &auditLogID
		}
		if cerr := s.repo.CreateCompletedRemoteCommand(ctx, hostID, module, action, "", details, "agent", status, auditIDPtr); cerr != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to create self-reported command record: %v", cerr))
		}
	}
	if action == "update" && status == "completed" {
		_ = s.repo.TouchAptLastAction(ctx, hostID, "update")
	}
	// A self-reported command shows up in the apt history + host views.
	if module != "" {
		s.bus.Publish(events.TopicApt)
		s.bus.Publish(events.HostTopic(hostID))
	}
	return nil
}

// ===== ingest helpers =====

func (s *Service) proxmoxIsMetricsSource(ctx context.Context, hostID string) bool {
	proxmoxIsMetricsSource := false
	if link, err := s.repo.GetProxmoxGuestLinkByHost(ctx, hostID); err == nil && link != nil {
		switch link.MetricsSource {
		case "proxmox":
			proxmoxIsMetricsSource = true
		case "auto":
			if fresh, err := s.repo.IsProxmoxGuestDataFresh(ctx, hostID); err == nil {
				proxmoxIsMetricsSource = fresh
			}
		}
	}
	if proxmoxIsMetricsSource && s.repo.IsHostUsedAsProxmoxCPUTempSource(ctx, hostID) {
		proxmoxIsMetricsSource = false
	}
	if proxmoxIsMetricsSource && s.repo.IsHostUsedAsProxmoxFanRPMSource(ctx, hostID) {
		proxmoxIsMetricsSource = false
	}
	return proxmoxIsMetricsSource
}

// storeMetrics persists host info + metrics. Returns a non-nil error only for the
// fatal cases (uptime/metrics insert) that must surface as HTTP 500.
func (s *Service) storeMetrics(ctx context.Context, hostID string, report *models.AgentReport, proxmoxIsMetricsSource bool) error {
	if report.Metrics != nil {
		update := models.HostUpdate{
			Hostname:     stringPtrIfNotEmpty(report.Metrics.Hostname),
			OS:           stringPtrIfNotEmpty(report.Metrics.OS),
			AgentVersion: stringPtrIfNotEmpty(report.AgentVersion),
		}
		if update.Hostname != nil || update.OS != nil || update.AgentVersion != nil {
			if err := s.repo.UpdateHost(ctx, hostID, &update); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to update host %s: %v", hostID, err))
			}
		}
		if proxmoxIsMetricsSource {
			if err := s.repo.InsertUptimeMetrics(ctx, hostID, report.Metrics.Uptime, report.Metrics.Hostname); err != nil {
				return fmt.Errorf("failed to store uptime")
			}
		} else {
			report.Metrics.HostID = hostID
			report.Metrics.Timestamp = time.Now()
			if _, err := s.repo.InsertMetrics(ctx, report.Metrics); err != nil {
				return fmt.Errorf("failed to store metrics")
			}
		}
		return nil
	}
	if report.AgentVersion != "" {
		update := models.HostUpdate{AgentVersion: stringPtrIfNotEmpty(report.AgentVersion)}
		if err := s.repo.UpdateHost(ctx, hostID, &update); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to update host %s: %v", hostID, err))
		}
	}
	return nil
}

func (s *Service) storeContainersAndPackages(ctx context.Context, hostID, safeHostID string, report *models.AgentReport) {
	if report.Docker != nil {
		for i := range report.Docker.Containers {
			report.Docker.Containers[i].HostID = hostID
		}
		if err := s.repo.UpsertDockerContainers(ctx, hostID, report.Docker.Containers); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store docker containers for host %s: %v", safeHostID, err))
		}
	}

	if report.UnattendedUpgrades != nil {
		if err := s.repo.UpsertUUStatus(ctx, hostID, *report.UnattendedUpgrades); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store UU status for host %s: %v", safeHostID, err))
		}
		for _, run := range report.UnattendedUpgrades.NewRuns {
			isNew, err := s.repo.InsertUURunIfNew(ctx, hostID, run)
			if err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to insert UU run for host %s: %v", safeHostID, err))
				continue
			}
			if isNew {
				_ = s.repo.UpdateUULastRun(ctx, hostID, run.RunAt, len(run.Packages))
				_ = s.repo.TouchAptLastAction(ctx, hostID, "update")
				if len(run.Packages) > 0 {
					_ = s.repo.TouchAptLastUpgradeAt(ctx, hostID, run.RunAt)
					hostname := hostID
					if host, err := s.repo.GetHost(ctx, hostID); err == nil && host != nil {
						hostname = host.Hostname
					}
					s.pushUUNotification(hostname, hostID, run)
				}
			}
		}
	}

	if report.DockerNetworks != nil {
		dbNetworks := make([]models.DockerNetwork, 0, len(report.DockerNetworks))
		for _, n := range report.DockerNetworks {
			dbNetworks = append(dbNetworks, models.DockerNetwork{
				ID:           fmt.Sprintf("%s-%s", hostID, n.NetworkID),
				HostID:       hostID,
				NetworkID:    n.NetworkID,
				Name:         n.Name,
				Driver:       n.Driver,
				Scope:        n.Scope,
				ContainerIDs: n.ContainerIDs,
				UpdatedAt:    time.Now(),
			})
		}
		if err := s.repo.UpsertDockerNetworks(ctx, hostID, dbNetworks); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store docker networks for host %s: %v", safeHostID, err))
		}
	}

	if report.ComposeProjects != nil {
		if err := s.repo.UpsertComposeProjects(ctx, hostID, report.ComposeProjects); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store compose projects for host %s: %v", safeHostID, err))
		}
	}
}

func (s *Service) storeDiskAndMetadata(ctx context.Context, hostID, safeHostID string, report *models.AgentReport) {
	if len(report.DiskMetrics) > 0 {
		batchTime := time.Now()
		for i := range report.DiskMetrics {
			report.DiskMetrics[i].HostID = hostID
			report.DiskMetrics[i].Timestamp = batchTime
		}
		if err := s.repo.InsertDiskMetrics(ctx, report.DiskMetrics); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store disk metrics for host %s: %v", safeHostID, err))
		}
	}

	if len(report.DiskHealth) > 0 {
		for i := range report.DiskHealth {
			report.DiskHealth[i].HostID = hostID
			report.DiskHealth[i].CollectedAt = time.Now()
		}
		if err := s.repo.InsertDiskHealth(ctx, report.DiskHealth); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store disk health for host %s: %v", safeHostID, err))
		}
	}

	if report.CustomTasks != nil {
		if b, err := json.Marshal(report.CustomTasks); err == nil {
			if err := s.repo.UpdateHostCustomTasks(ctx, hostID, string(b)); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store custom tasks for host %s: %v", safeHostID, err))
			}
		}
	}

	if report.TasksConfigYAML != "" {
		if err := s.repo.UpdateHostTasksConfigYAML(ctx, hostID, report.TasksConfigYAML); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store tasks config YAML for host %s: %v", safeHostID, err))
		}
	}

	if report.Capabilities != nil {
		if b, err := json.Marshal(report.Capabilities); err == nil {
			if err := s.repo.UpdateHostCollectors(ctx, hostID, string(b)); err != nil {
				slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to store collectors for host %s: %v", safeHostID, err))
			}
		}
	}

	if report.WebLogs != nil {
		if err := s.repo.UpdateHostWebLogs(ctx, hostID, report.WebLogs); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to update web logs cache for host %s: %v", safeHostID, err))
		}
		if err := s.repo.InsertWebLogSnapshot(ctx, hostID, report.WebLogs); err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Warning: failed to insert web logs snapshot for host %s: %v", safeHostID, err))
		}
	}
}

// pushUUNotification sends a browser + ntfy notification when unattended-upgrades
// installs packages.
func (s *Service) pushUUNotification(hostname, hostID string, run models.UURun) {
	pkgCount := len(run.Packages)
	title := fmt.Sprintf("Mises à jour auto — %s", hostname)
	msg := fmt.Sprintf("%d paquet(s) installé(s) : %s", pkgCount, strings.Join(run.Packages, ", "))
	if len(msg) > 200 {
		msg = msg[:197] + "..."
	}
	if s.notifPusher != nil {
		s.notifPusher.Broadcast(models.WSUnattendedUpgradeMessage{
			Type: "unattended_upgrade",
			Notification: models.WSUUNotification{
				ID:            fmt.Sprintf("uu:%s:%d", hostID, run.RunAt.UnixNano()),
				Type:          "unattended_upgrade",
				HostID:        hostID,
				HostName:      hostname,
				Packages:      run.Packages,
				PkgCount:      pkgCount,
				RunAt:         run.RunAt,
				BrowserNotify: true,
				Title:         title,
				Message:       msg,
			},
		})
	}
	if s.cfg.NotifyURL != "" {
		n := notify.New()
		if err := n.SendNtfy(s.cfg, s.cfg.NotifyURL, title, msg); err != nil {
			slog.Error("UU ntfy notification failed", slog.String("host_id", hostID), slog.Any("err", err))
		}
	}
}

func truncateOutput(str string, max int) string {
	if max <= 0 || len(str) <= max {
		return str
	}
	return str[:max] + "..."
}

func stringPtrIfNotEmpty(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
