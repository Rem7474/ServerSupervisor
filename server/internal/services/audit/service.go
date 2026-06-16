// Package audit is the application/service layer for audit logs and command
// history reads. The handler keeps the HTTP concerns (role authz, query parsing /
// limit clamping, response envelopes); the service owns the data orchestration —
// non-nil guarantees and the not-found semantics — behind a Repository port so it
// is unit-testable without a database.
package audit

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally; the
// command-history methods use the database package's query types (CommandFilter,
// RemoteCommandWithHost), so they appear here too.
type Repository interface {
	GetAuditLogs(ctx context.Context, limit, offset int) ([]models.AuditLog, error)
	GetAuditLogsByHost(ctx context.Context, hostID string, limit int) ([]models.AuditLog, error)
	GetAuditLogsByUser(ctx context.Context, username string, limit int) ([]models.AuditLog, error)
	GetAllRemoteCommands(ctx context.Context, limit, offset int, f database.CommandFilter) ([]database.RemoteCommandWithHost, error)
	CountAllRemoteCommands(ctx context.Context, f database.CommandFilter) (int64, error)
	GetRemoteCommandByID(ctx context.Context, id string) (*models.RemoteCommand, error)
	CancelRemoteCommand(ctx context.Context, id string) (bool, error)
	GetRecentCommandsByHost(ctx context.Context, hostID string, limit int) ([]models.RemoteCommand, error)
	GetAlertIncidentsByHost(ctx context.Context, hostID string, limit int) ([]database.AlertIncidentWithRule, error)
}

// Service holds the audit/command-history read use-cases.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Logs returns a page of audit logs (never nil).
func (s *Service) Logs(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	logs, err := s.repo.GetAuditLogs(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	return nonNilLogs(logs), nil
}

// LogsByHost returns a host's audit logs (never nil).
func (s *Service) LogsByHost(ctx context.Context, hostID string, limit int) ([]models.AuditLog, error) {
	logs, err := s.repo.GetAuditLogsByHost(ctx, hostID, limit)
	if err != nil {
		return nil, err
	}
	return nonNilLogs(logs), nil
}

// LogsByUser returns a user's audit logs (never nil).
func (s *Service) LogsByUser(ctx context.Context, username string, limit int) ([]models.AuditLog, error) {
	logs, err := s.repo.GetAuditLogsByUser(ctx, username, limit)
	if err != nil {
		return nil, err
	}
	return nonNilLogs(logs), nil
}

// Commands returns a page of remote-command history plus the total matching the
// filter (commands never nil).
func (s *Service) Commands(ctx context.Context, limit, offset int, f database.CommandFilter) ([]database.RemoteCommandWithHost, int64, error) {
	cmds, err := s.repo.GetAllRemoteCommands(ctx, limit, offset, f)
	if err != nil {
		return nil, 0, err
	}
	total, _ := s.repo.CountAllRemoteCommands(ctx, f)
	if cmds == nil {
		cmds = []database.RemoteCommandWithHost{}
	}
	return cmds, total, nil
}

// Command returns one remote command by id, or apperr.NotFound when absent.
func (s *Service) Command(ctx context.Context, id string) (*models.RemoteCommand, error) {
	cmd, err := s.repo.GetRemoteCommandByID(ctx, id)
	if err != nil {
		return nil, apperr.NotFound("command not found")
	}
	return cmd, nil
}

// Cancel cancels a pending or running command. Returns apperr.NotFound when the
// command does not exist or is already in a terminal state.
func (s *Service) Cancel(ctx context.Context, id string) error {
	cancelled, err := s.repo.CancelRemoteCommand(ctx, id)
	if err != nil {
		return err
	}
	if !cancelled {
		return apperr.NotFound("command not found or already in a terminal state")
	}
	return nil
}

// HostTimeline returns an aggregate activity feed for a host, merging audit logs,
// remote commands and alert incidents sorted newest-first.
func (s *Service) HostTimeline(ctx context.Context, hostID string, limit int) ([]models.HostTimelineEvent, error) {
	type result struct {
		logs      []models.AuditLog
		cmds      []models.RemoteCommand
		incidents []database.AlertIncidentWithRule
		err       error
	}

	logsCh := make(chan []models.AuditLog, 1)
	cmdsCh := make(chan []models.RemoteCommand, 1)
	incCh := make(chan []database.AlertIncidentWithRule, 1)
	errCh := make(chan error, 3)

	go func() {
		v, e := s.repo.GetAuditLogsByHost(ctx, hostID, limit)
		if e != nil {
			errCh <- e
		} else {
			logsCh <- v
		}
	}()
	go func() {
		v, e := s.repo.GetRecentCommandsByHost(ctx, hostID, limit)
		if e != nil {
			errCh <- e
		} else {
			cmdsCh <- v
		}
	}()
	go func() {
		v, e := s.repo.GetAlertIncidentsByHost(ctx, hostID, limit)
		if e != nil {
			errCh <- e
		} else {
			incCh <- v
		}
	}()

	var logs []models.AuditLog
	var cmds []models.RemoteCommand
	var incidents []database.AlertIncidentWithRule

	for range 3 {
		select {
		case v := <-logsCh:
			logs = v
		case v := <-cmdsCh:
			cmds = v
		case v := <-incCh:
			incidents = v
		case e := <-errCh:
			return nil, e
		}
	}

	events := make([]models.HostTimelineEvent, 0, len(logs)+len(cmds)+len(incidents))

	for _, l := range logs {
		events = append(events, models.HostTimelineEvent{
			ID:        strconv.FormatInt(l.ID, 10),
			Type:      "audit",
			Timestamp: l.CreatedAt,
			Title:     humaniseAuditAction(l.Action),
			Detail:    truncate(l.Details, 120),
			Status:    l.Status,
		})
	}
	for _, c := range cmds {
		title := strings.TrimSpace(c.Module + " " + c.Action)
		if c.Target != "" {
			title += " " + c.Target
		}
		events = append(events, models.HostTimelineEvent{
			ID:        c.ID,
			Type:      "command",
			Timestamp: c.CreatedAt,
			Title:     title,
			Detail:    truncate(c.Output, 120),
			Status:    c.Status,
			Module:    c.Module,
		})
	}
	for _, inc := range incidents {
		status := "active"
		if inc.ResolvedAt != nil {
			status = "resolved"
		}
		title := inc.RuleName
		if title == "" {
			title = inc.Metric
		}
		events = append(events, models.HostTimelineEvent{
			ID:        strconv.FormatInt(inc.ID, 10),
			Type:      "incident",
			Timestamp: inc.TriggeredAt,
			Title:     title,
			Detail:    fmt.Sprintf("%.2f (%s)", inc.Value, inc.Metric),
			Status:    status,
			Severity:  inc.Severity,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func humaniseAuditAction(action string) string {
	s := strings.ReplaceAll(action, "_", " ")
	if len(s) == 0 {
		return action
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func nonNilLogs(logs []models.AuditLog) []models.AuditLog {
	if logs == nil {
		return []models.AuditLog{}
	}
	return logs
}
