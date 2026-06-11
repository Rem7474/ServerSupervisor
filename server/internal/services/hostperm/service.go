// Package hostperm is the application/service layer for per-host user
// permissions. It owns the access-level validation behind a Repository port, so
// the logic is unit-testable without a database and the handler only translates
// HTTP.
package hostperm

import (
	"context"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	ListHostPermissions(ctx context.Context, hostID string) ([]models.HostPermission, error)
	ListUserHostPermissions(ctx context.Context, username string) ([]models.HostPermission, error)
	SetHostPermission(ctx context.Context, username, hostID, level string) error
	DeleteHostPermission(ctx context.Context, username, hostID string) error
}

// Service holds the host-permission use-cases.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func validLevel(level string) bool {
	return level == "viewer" || level == "operator"
}

// List returns the explicit permissions on a host (never nil).
func (s *Service) List(ctx context.Context, hostID string) ([]models.HostPermission, error) {
	perms, err := s.repo.ListHostPermissions(ctx, hostID)
	if err != nil {
		return nil, err
	}
	return nonNil(perms), nil
}

// ListForUser returns the calling user's permission entries (never nil).
func (s *Service) ListForUser(ctx context.Context, username string) ([]models.HostPermission, error) {
	perms, err := s.repo.ListUserHostPermissions(ctx, username)
	if err != nil {
		return nil, err
	}
	return nonNil(perms), nil
}

// Set grants or updates a user's access level to a host. The level must be
// "viewer" or "operator".
func (s *Service) Set(ctx context.Context, username, hostID, level string) error {
	if !validLevel(level) {
		return apperr.Validation("level doit être 'viewer' ou 'operator'")
	}
	return s.repo.SetHostPermission(ctx, username, hostID, level)
}

// Delete revokes a user's access to a host.
func (s *Service) Delete(ctx context.Context, username, hostID string) error {
	return s.repo.DeleteHostPermission(ctx, username, hostID)
}

func nonNil(perms []models.HostPermission) []models.HostPermission {
	if perms == nil {
		return []models.HostPermission{}
	}
	return perms
}
