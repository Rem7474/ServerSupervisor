// Package docker is the application/service layer for Docker container/compose
// reads and agent-command dispatch. The working-dir path-traversal validation and
// the command/audit building live here behind a Repository + Dispatcher port.
// Per-host access control stays in the handler (it needs the gin context).
package docker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetDockerContainers(ctx context.Context, hostID string) ([]models.DockerContainer, error)
	GetAllDockerContainers(ctx context.Context) ([]models.DockerContainer, error)
	GetAllComposeProjects(ctx context.Context) ([]models.ComposeProject, error)
	GetComposeProjectsByHost(ctx context.Context, hostID string) ([]models.ComposeProject, error)
}

// Dispatcher is the agent-command port. *dispatch.Dispatcher satisfies it.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Service holds the Docker use-cases.
type Service struct {
	repo       Repository
	dispatcher Dispatcher
}

func NewService(repo Repository, dispatcher Dispatcher) *Service {
	return &Service{repo: repo, dispatcher: dispatcher}
}

// isValidWorkingDir returns true when p is empty or an absolute path that does
// not escape its root via ".." components.
func isValidWorkingDir(p string) bool {
	if p == "" {
		return true
	}
	if !filepath.IsAbs(p) {
		return false
	}
	return !strings.Contains(filepath.Clean(p), "..")
}

// Containers returns a host's containers (never nil).
func (s *Service) Containers(ctx context.Context, hostID string) ([]models.DockerContainer, error) {
	containers, err := s.repo.GetDockerContainers(ctx, hostID)
	if err != nil {
		return nil, err
	}
	return nonNilContainers(containers), nil
}

// AllContainers returns a page of containers across all hosts plus the total.
func (s *Service) AllContainers(ctx context.Context, limit, offset int) ([]models.DockerContainer, int, error) {
	all, err := s.repo.GetAllDockerContainers(ctx)
	if err != nil {
		return nil, 0, err
	}
	total := len(all)
	if offset >= total {
		return []models.DockerContainer{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total, nil
}

// SendCommand validates the working dir and dispatches a docker command, returning
// the queued command id.
func (s *Service) SendCommand(ctx context.Context, req models.DockerCommandRequest, username, clientIP string) (string, error) {
	if !isValidWorkingDir(req.WorkingDir) {
		return "", apperr.Validation("invalid working_dir: must be an absolute path")
	}
	result, err := s.dispatcher.Create(ctx, dispatch.Request{
		HostID:      req.HostID,
		Module:      "docker",
		Action:      req.Action,
		Target:      req.ContainerName,
		Payload:     fmt.Sprintf(`{"working_dir":%q}`, req.WorkingDir),
		TriggeredBy: username,
		Audit: &dispatch.AuditLogRequest{
			Username:  username,
			Action:    "docker_" + req.Action,
			HostID:    req.HostID,
			IPAddress: clientIP,
			Details:   fmt.Sprintf(`{"container":"%s","action":"%s","working_dir":"%s"}`, req.ContainerName, req.Action, req.WorkingDir),
		},
	})
	if err != nil {
		return "", err
	}
	return result.Command.ID, nil
}

// ComposeProjects returns all compose projects across hosts (never nil).
func (s *Service) ComposeProjects(ctx context.Context) ([]models.ComposeProject, error) {
	projects, err := s.repo.GetAllComposeProjects(ctx)
	if err != nil {
		return nil, err
	}
	return nonNilProjects(projects), nil
}

// HostComposeProjects returns a host's compose projects (never nil).
func (s *Service) HostComposeProjects(ctx context.Context, hostID string) ([]models.ComposeProject, error) {
	projects, err := s.repo.GetComposeProjectsByHost(ctx, hostID)
	if err != nil {
		return nil, err
	}
	return nonNilProjects(projects), nil
}

func nonNilContainers(v []models.DockerContainer) []models.DockerContainer {
	if v == nil {
		return []models.DockerContainer{}
	}
	return v
}
func nonNilProjects(v []models.ComposeProject) []models.ComposeProject {
	if v == nil {
		return []models.ComposeProject{}
	}
	return v
}
