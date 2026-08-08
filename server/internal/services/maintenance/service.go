// Package maintenance is the application/service layer for maintenance
// windows — time ranges during which the alert engine (internal/alerts)
// suppresses notifications for a host, or every host for a global window,
// so a planned intervention doesn't generate alert noise.
package maintenance

import (
	"context"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	ListMaintenanceWindows(ctx context.Context) ([]models.MaintenanceWindow, error)
	ListMaintenanceWindowsForHost(ctx context.Context, hostID string) ([]models.MaintenanceWindow, error)
	GetMaintenanceWindow(ctx context.Context, id string) (*models.MaintenanceWindow, error)
	CreateMaintenanceWindow(ctx context.Context, w models.MaintenanceWindow) (*models.MaintenanceWindow, error)
	DeleteMaintenanceWindow(ctx context.Context, id string) error
}

// Service holds the maintenance-window use-cases.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListAll returns every maintenance window (never nil).
func (s *Service) ListAll(ctx context.Context) ([]models.MaintenanceWindow, error) {
	windows, err := s.repo.ListMaintenanceWindows(ctx)
	if err != nil {
		return nil, err
	}
	return nonNil(windows), nil
}

// ListForHost returns the windows applicable to a host — its own plus any
// global one (never nil).
func (s *Service) ListForHost(ctx context.Context, hostID string) ([]models.MaintenanceWindow, error) {
	windows, err := s.repo.ListMaintenanceWindowsForHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	return nonNil(windows), nil
}

// Get returns a window by id, or apperr.NotFound when it is absent.
func (s *Service) Get(ctx context.Context, id string) (*models.MaintenanceWindow, error) {
	w, err := s.repo.GetMaintenanceWindow(ctx, id)
	if err != nil {
		return nil, apperr.NotFound("maintenance window not found")
	}
	return w, nil
}

// CreateForHost validates and creates a window scoped to a single host. Host
// authorization is the caller's (HTTP) responsibility.
func (s *Service) CreateForHost(ctx context.Context, hostID, username string, req models.MaintenanceWindowRequest) (*models.MaintenanceWindow, error) {
	if err := validate(req); err != nil {
		return nil, err
	}
	return s.repo.CreateMaintenanceWindow(ctx, models.MaintenanceWindow{
		HostID:    &hostID,
		Reason:    req.Reason,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
		CreatedBy: username,
	})
}

// CreateGlobal validates and creates a window applying to every host.
func (s *Service) CreateGlobal(ctx context.Context, username string, req models.MaintenanceWindowRequest) (*models.MaintenanceWindow, error) {
	if err := validate(req); err != nil {
		return nil, err
	}
	return s.repo.CreateMaintenanceWindow(ctx, models.MaintenanceWindow{
		HostID:    nil,
		Reason:    req.Reason,
		StartsAt:  req.StartsAt,
		EndsAt:    req.EndsAt,
		CreatedBy: username,
	})
}

// Delete removes a window. Host/admin authorization is the caller's (HTTP)
// responsibility.
func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteMaintenanceWindow(ctx, id)
}

func validate(req models.MaintenanceWindowRequest) error {
	if req.Reason == "" {
		return apperr.Validation("reason is required")
	}
	if !req.EndsAt.After(req.StartsAt) {
		return apperr.Validation("ends_at must be after starts_at")
	}
	return nil
}

func nonNil(windows []models.MaintenanceWindow) []models.MaintenanceWindow {
	if windows == nil {
		return []models.MaintenanceWindow{}
	}
	return windows
}
