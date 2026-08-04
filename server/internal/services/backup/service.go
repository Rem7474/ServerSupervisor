// Package backup is the application/service layer for Restic backup
// supervision: history of runs (backup_runs), the passive periodic status
// snapshot (restic_status), manual-trigger dispatch and completion
// notifications. It sits behind a Repository + Dispatcher port, following the
// same shape as internal/services/ssl. Recurring backups are scheduled
// through the existing internal/services/scheduledtask module (module
// "restic" was added to its validModules) — this package only owns the
// backup-domain history/status and the manual "run now" trigger.
package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/services/notifychannels"
	"github.com/serversupervisor/server/internal/services/push"
	"github.com/serversupervisor/server/internal/ws"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	CreateBackupRun(ctx context.Context, r models.BackupRun) (*models.BackupRun, error)
	GetBackupRun(ctx context.Context, id string) (*models.BackupRun, error)
	ListBackupRuns(ctx context.Context, hostID string, limit int) ([]models.BackupRun, error)
	GetLatestBackupRunByHost(ctx context.Context, hostID string) (*models.BackupRun, error)
	GetBackupRunByCommandID(ctx context.Context, commandID string) (*models.BackupRun, error)
	UpdateBackupRun(ctx context.Context, r models.BackupRun) error
	GetRemoteCommandByID(ctx context.Context, id string) (*models.RemoteCommand, error)
	GetResticStatus(ctx context.Context, hostID string) (*models.ResticStatus, error)
	ListStalledBackupRuns(ctx context.Context, olderThanMinutes int) ([]models.BackupRun, error)
	GetHostResticProfiles(ctx context.Context, hostID string) (string, error)
	GetHostResticGroups(ctx context.Context, hostID string) (string, error)
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Service holds the backup use-cases.
type Service struct {
	repo       Repository
	dispatcher Dispatcher
	cfg        *config.Config
	notifHub   *ws.NotificationHub
	dispatch   *notifychannels.Dispatcher
	bgCtx      context.Context
}

func NewService(repo Repository, dispatcher Dispatcher, cfg *config.Config, notifHub *ws.NotificationHub, pushSvc *push.Service) *Service {
	return &Service{
		repo: repo, dispatcher: dispatcher, cfg: cfg, notifHub: notifHub,
		dispatch: notifychannels.NewDispatcher(cfg, pushSvc),
		bgCtx:    context.Background(),
	}
}

// SetBackgroundContext threads a long-lived (SIGTERM-bound) ctx for the
// fire-and-forget completion callback.
func (s *Service) SetBackgroundContext(ctx context.Context) { s.bgCtx = ctx }

// TriggerBackup dispatches a manual run_backup command for hostID/profile
// (profile may be empty for the script's default profile) and creates a
// "running" history row linked to the resulting command.
func (s *Service) TriggerBackup(ctx context.Context, hostID, profile, triggeredBy string) (*models.BackupRun, error) {
	if hostID == "" {
		return nil, apperr.Validation("host_id is required")
	}
	created, err := s.repo.CreateBackupRun(ctx, models.BackupRun{
		HostID:      hostID,
		Profile:     profile,
		TriggeredBy: triggeredBy,
		Status:      "running",
		StartedAt:   time.Now(),
	})
	if err != nil {
		return nil, apperr.Failed("failed to create backup run")
	}

	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "restic",
		Action:      "run_backup",
		Target:      profile,
		TriggeredBy: triggeredBy,
	})
	if err != nil {
		return nil, apperr.Internal(err)
	}

	updated := *created
	commandID := result.Command.ID
	updated.CommandID = &commandID
	if err := s.repo.UpdateBackupRun(ctx, updated); err != nil {
		slog.WarnContext(ctx, "failed to link command to backup run",
			"backup_run_id", created.ID, "command_id", commandID, "err", err)
	}
	return &updated, nil
}

// GetStatus returns the aggregated backup status for a host: the latest
// dispatched run when one exists, plus the passive periodic snapshot (useful
// on its own for a host whose backups aren't dispatched by ServerSupervisor).
func (s *Service) GetStatus(ctx context.Context, hostID string) (*models.BackupStatus, error) {
	status := &models.BackupStatus{}
	if latest, err := s.repo.GetLatestBackupRunByHost(ctx, hostID); err == nil {
		status.LatestRun = latest
	}
	if passive, err := s.repo.GetResticStatus(ctx, hostID); err == nil {
		status.PassiveState = passive
	}
	return status, nil
}

// ListRuns returns a host's backup run history (never nil).
func (s *Service) ListRuns(ctx context.Context, hostID string, limit int) ([]models.BackupRun, error) {
	if limit <= 0 {
		limit = 20
	}
	runs, err := s.repo.ListBackupRuns(ctx, hostID, limit)
	if err != nil {
		return nil, err
	}
	if runs == nil {
		runs = []models.BackupRun{}
	}
	return runs, nil
}

// GetProfiles returns the resticprofile.yaml profile names last reported by
// the host's agent (never nil, empty slice when none reported yet).
func (s *Service) GetProfiles(ctx context.Context, hostID string) ([]string, error) {
	raw, err := s.repo.GetHostResticProfiles(ctx, hostID)
	if err != nil {
		return nil, err
	}
	profiles := []string{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &profiles) // best-effort; empty on malformed cache
	}
	if profiles == nil {
		profiles = []string{}
	}
	return profiles, nil
}

// GetGroups returns the resticprofile.yaml "groups" section group names last
// reported by the host's agent (never nil, empty slice when none reported
// yet). A group is resolved by resticprofile the same way as a profile when
// passed to run_backup.sh as --name, so it's exposed as a second, parallel
// pickable list rather than merged into GetProfiles's result.
func (s *Service) GetGroups(ctx context.Context, hostID string) ([]string, error) {
	raw, err := s.repo.GetHostResticGroups(ctx, hostID)
	if err != nil {
		return nil, err
	}
	groups := []string{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &groups) // best-effort; empty on malformed cache
	}
	if groups == nil {
		groups = []string{}
	}
	return groups, nil
}

// GetRun returns a single backup run by id, or apperr.NotFound.
func (s *Service) GetRun(ctx context.Context, id string) (*models.BackupRun, error) {
	run, err := s.repo.GetBackupRun(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("backup run not found")
	}
	if err != nil {
		return nil, err
	}
	return run, nil
}

// resticBackupSummary mirrors agent/internal/collector.ResticBackupSummary —
// the JSON the agent puts in CommandResult.Output for a run_backup command.
// Never contains restic/Swift/SMTP credentials (agent-side contract).
type resticBackupSummary struct {
	Status        string `json:"status"`
	Profile       string `json:"profile,omitempty"`
	DurationSec   int    `json:"duration_sec"`
	FilesNew      *int   `json:"files_new,omitempty"`
	FilesChanged  *int   `json:"files_changed,omitempty"`
	BytesAdded    *int64 `json:"bytes_added,omitempty"`
	SnapshotID    string `json:"snapshot_id,omitempty"`
	RepoSizeBytes *int64 `json:"repo_size_bytes,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
}

// HandleCommandCompletion implements agentsvc.CommandCompletionListener. It is
// invoked for every command reaching a terminal state, regardless of module —
// this no-ops for anything that isn't a restic run_backup command (whether
// dispatched manually via TriggerBackup or by a cron-scheduled task through
// scheduledtask.Service, which calls the dispatcher directly and never goes
// through this package).
func (s *Service) HandleCommandCompletion(commandID, status string) {
	if status != "completed" && status != "failed" {
		return
	}
	ctx := s.bgCtx

	cmd, err := s.repo.GetRemoteCommandByID(ctx, commandID)
	if err != nil || cmd.Module != "restic" || cmd.Action != "run_backup" {
		return // not a restic run_backup command
	}

	var summary resticBackupSummary
	_ = json.Unmarshal([]byte(cmd.Output), &summary) // best-effort; zero value is fine on failure

	run := buildBackupRun(cmd, status, summary)

	if existing, existErr := s.repo.GetBackupRunByCommandID(ctx, commandID); existErr == nil && existing != nil {
		run.ID = existing.ID
		if err := s.repo.UpdateBackupRun(ctx, run); err != nil {
			slog.ErrorContext(ctx, "failed to update backup run", "command_id", commandID, "err", err)
		}
	} else if _, err := s.repo.CreateBackupRun(ctx, run); err != nil {
		slog.ErrorContext(ctx, "failed to create backup run", "command_id", commandID, "err", err)
	}

	if run.Status != "error" {
		return
	}
	s.notifyFailure(ctx, cmd.HostID, commandID)
}

func buildBackupRun(cmd *models.RemoteCommand, cmdStatus string, summary resticBackupSummary) models.BackupRun {
	run := models.BackupRun{
		HostID:      cmd.HostID,
		Profile:     cmd.Target,
		CommandID:   &cmd.ID,
		TriggeredBy: cmd.TriggeredBy,
		StartedAt:   cmd.CreatedAt,
	}
	if cmd.StartedAt != nil {
		run.StartedAt = *cmd.StartedAt
	}
	if cmd.EndedAt != nil {
		run.FinishedAt = cmd.EndedAt
	} else {
		now := time.Now()
		run.FinishedAt = &now
	}
	if run.FinishedAt != nil {
		d := int(run.FinishedAt.Sub(run.StartedAt).Seconds())
		if summary.DurationSec > 0 {
			d = summary.DurationSec
		}
		if d >= 0 {
			run.DurationSec = &d
		}
	}

	switch {
	case cmdStatus == "failed":
		run.Status = "error"
	case summary.Status != "":
		run.Status = summary.Status
	default:
		run.Status = "ok"
	}

	run.FilesDone = int64PtrFromIntPtr(summary.FilesNew)
	run.BytesDone = summary.BytesAdded
	run.SnapshotID = summary.SnapshotID
	run.RepoSizeBytes = summary.RepoSizeBytes
	run.ErrorMessage = summary.ErrorMessage
	if run.ErrorMessage == "" && cmdStatus == "failed" {
		run.ErrorMessage = truncate(cmd.Output, 500)
	}
	if cmd.Output != "" {
		out := truncate(cmd.Output, 8192)
		run.RawSummary = &out
	}
	return run
}

func int64PtrFromIntPtr(p *int) *int64 {
	if p == nil {
		return nil
	}
	v := int64(*p)
	return &v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// CheckStalledRuns marks backup_runs still "running" past olderThanMinutes as
// "error" and notifies (same fan-out as a failed run). A safety net for the
// case where the agent dies or loses connectivity before reporting a
// run_backup command's terminal status — which would otherwise leave the row
// "running" forever, since there's deliberately no fixed absolute duration
// cap on a restic backup (see dispatcher.go's idle-timeout exception).
func (s *Service) CheckStalledRuns(ctx context.Context, olderThanMinutes int) {
	stalled, err := s.repo.ListStalledBackupRuns(ctx, olderThanMinutes)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list stalled backup runs", "err", err)
		return
	}
	for _, run := range stalled {
		run.Status = "error"
		if run.ErrorMessage == "" {
			run.ErrorMessage = fmt.Sprintf("backup marked stalled after %d minutes with no terminal status", olderThanMinutes)
		}
		now := time.Now()
		run.FinishedAt = &now
		if err := s.repo.UpdateBackupRun(ctx, run); err != nil {
			slog.ErrorContext(ctx, "failed to mark backup run stalled", "run_id", run.ID, "err", err)
			continue
		}
		commandID := ""
		if run.CommandID != nil {
			commandID = *run.CommandID
		}
		s.notifyFailure(ctx, run.HostID, commandID)
	}
}

func (s *Service) notifyFailure(ctx context.Context, hostID, commandID string) {
	subject := fmt.Sprintf("[ServerSupervisor] Backup Restic en échec sur %s", hostID)
	msg := fmt.Sprintf("Le backup Restic sur l'hôte %s a échoué (commande %s).", hostID, commandID)

	s.dispatch.Send(ctx, notifychannels.Event{
		LogID:       "backup:" + commandID,
		Channels:    []string{"smtp", "ntfy", "browser"},
		SMTPSubject: subject,
		SMTPBody:    msg,
		SMTPTo:      s.cfg.SMTPTo,
		NtfyTitle:   subject,
		NtfyBody:    msg,
		NtfyURL:     s.cfg.NotifyURL,
		OnBrowser: func() {
			if s.notifHub == nil {
				return
			}
			s.notifHub.Broadcast(models.WSBackupRunMessage{
				Type: "backup_run",
				Notification: models.WSBackupNotification{
					HostID:      hostID,
					Status:      "failed",
					TriggeredAt: time.Now().UTC(),
				},
			})
		},
		Push: &push.Payload{
			Title:  "Backup Restic en échec",
			Body:   fmt.Sprintf("Hôte : %s", hostID),
			Tag:    fmt.Sprintf("backup-run-%s", commandID),
			URL:    fmt.Sprintf("/hosts/%s?tab=backup", hostID),
			Status: "failed",
		},
	})
}
