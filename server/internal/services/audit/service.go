// Package audit is the application/service layer for audit logs and command
// history reads. The handler keeps the HTTP concerns (role authz, query parsing /
// limit clamping, response envelopes); the service owns the data orchestration —
// non-nil guarantees and the not-found semantics — behind a Repository port so it
// is unit-testable without a database.
package audit

import (
	"context"

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

func nonNilLogs(logs []models.AuditLog) []models.AuditLog {
	if logs == nil {
		return []models.AuditLog{}
	}
	return logs
}
