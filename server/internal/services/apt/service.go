// Package apt is the application/service layer for APT + unattended-upgrades. It
// owns the agent-command dispatch (build request + audit) and the status reads
// behind a Repository + Dispatcher port. Per-host access control stays in the
// handler (it needs the gin context), so the handler passes already-authorized
// host ids here.
package apt

import (
	"context"
	"encoding/json"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetAptCVESummary(ctx context.Context) (*models.AptCVESummary, error)
	GetAptStatus(ctx context.Context, hostID string) (*models.AptStatus, error)
	GetUUStatus(ctx context.Context, hostID string) (*models.UnattendedUpgradesDB, error)
	GetUURuns(ctx context.Context, hostID string, limit int) ([]models.UURun, error)
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Service holds the APT use-cases.
type Service struct {
	repo       Repository
	dispatcher Dispatcher
}

func NewService(repo Repository, dispatcher Dispatcher) *Service {
	return &Service{repo: repo, dispatcher: dispatcher}
}

// Command dispatches an apt command (update/upgrade/…) to a host and returns the
// queued command id.
func (s *Service) Command(ctx context.Context, hostID, command, username, clientIP string) (string, error) {
	r, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      command,
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "apt_" + command,
			HostID:    hostID,
			IPAddress: clientIP,
			Details:   "apt " + command,
		},
	})
	if err != nil {
		return "", err
	}
	return r.Command.ID, nil
}

// ConfigureUU dispatches configure_uu (with the encoded config) and a toggle_uu
// to enable/disable the service, returning the queued command ids.
func (s *Service) ConfigureUU(ctx context.Context, hostID string, req models.UnattendedUpgradesConfigureRequest, username, clientIP string) ([]string, error) {
	cfgPayload, err := json.Marshal(req.Config)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	var commandIDs []string
	r, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      "configure_uu",
		Payload:     string(cfgPayload),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "uu_configure",
			HostID:    hostID,
			IPAddress: clientIP,
			Details:   "configure unattended-upgrades",
		},
	})
	if err != nil {
		return nil, err
	}
	commandIDs = append(commandIDs, r.Command.ID)

	target := "disable"
	if req.Enabled {
		target = "enable"
	}
	if r2, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      "toggle_uu",
		Target:      target,
		Payload:     "{}",
		TriggeredBy: username,
	}); err == nil {
		commandIDs = append(commandIDs, r2.Command.ID)
	}
	return commandIDs, nil
}

// InstallUU dispatches an install of unattended-upgrades.
func (s *Service) InstallUU(ctx context.Context, hostID, username, clientIP string) (string, error) {
	return s.dispatchUU(ctx, hostID, "install_uu", "uu_install", "install unattended-upgrades", username, clientIP)
}

// RunUUNow dispatches a manual unattended-upgrade run.
func (s *Service) RunUUNow(ctx context.Context, hostID, username, clientIP string) (string, error) {
	return s.dispatchUU(ctx, hostID, "run_uu", "uu_run", "manual unattended-upgrade run", username, clientIP)
}

func (s *Service) dispatchUU(ctx context.Context, hostID, action, auditAction, details, username, clientIP string) (string, error) {
	r, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "apt",
		Action:      action,
		Payload:     "{}",
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    auditAction,
			HostID:    hostID,
			IPAddress: clientIP,
			Details:   details,
		},
	})
	if err != nil {
		return "", err
	}
	return r.Command.ID, nil
}

// CVESummary returns aggregated CVE severity counts across all hosts.
func (s *Service) CVESummary(ctx context.Context) (*models.AptCVESummary, error) {
	return s.repo.GetAptCVESummary(ctx)
}

// Status returns a host's APT status, or apperr.NotFound when absent.
func (s *Service) Status(ctx context.Context, hostID string) (*models.AptStatus, error) {
	status, err := s.repo.GetAptStatus(ctx, hostID)
	if err != nil {
		return nil, apperr.NotFound("apt status not found")
	}
	return status, nil
}

// UUStatus returns a host's unattended-upgrades status.
func (s *Service) UUStatus(ctx context.Context, hostID string) (*models.UnattendedUpgradesDB, error) {
	return s.repo.GetUUStatus(ctx, hostID)
}

// UURuns returns a host's upgrade run history (never nil).
func (s *Service) UURuns(ctx context.Context, hostID string, limit int) ([]models.UURun, error) {
	runs, err := s.repo.GetUURuns(ctx, hostID, limit)
	if err != nil {
		return nil, err
	}
	if runs == nil {
		runs = []models.UURun{}
	}
	return runs, nil
}
