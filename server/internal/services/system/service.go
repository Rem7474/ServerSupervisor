// Package system turns the host-level "system" actions (journal logs, process
// snapshot, systemd management) into dispatched agent commands. The handler
// keeps request binding and authorization; command shape and audit entries
// live here.
package system

import (
	"context"
	"fmt"
	"regexp"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/dispatch"
)

// validServiceName matches valid systemd service names: alphanumeric plus ._:@-
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9._:@\-]{1,256}$`)

// SystemdActions are the actions SendSystemd accepts.
var SystemdActions = map[string]bool{
	"list": true, "start": true, "stop": true, "restart": true,
	"enable": true, "disable": true, "status": true,
}

// Dispatcher is the part of *dispatch.Dispatcher the service needs.
type Dispatcher interface {
	Create(ctx context.Context, req dispatch.Request) (*dispatch.Result, error)
}

// Actor identifies who triggers a command, for the command row and audit log.
type Actor struct {
	Username  string
	IPAddress string
}

type Service struct {
	dispatcher Dispatcher
}

func NewService(dispatcher Dispatcher) *Service {
	return &Service{dispatcher: dispatcher}
}

// JournalLogs enqueues a journalctl log fetch for one service on a host.
func (s *Service) JournalLogs(ctx context.Context, actor Actor, hostID, serviceName string) (string, error) {
	if !validServiceName.MatchString(serviceName) {
		return "", apperr.Validation("invalid service name")
	}
	return s.create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "journal",
		Action:      "logs",
		Target:      serviceName,
		Payload:     "{}",
		TriggeredBy: actor.Username,
		Audit: &dispatch.AuditLogRequest{
			Username:  actor.Username,
			Action:    "journalctl",
			HostID:    hostID,
			IPAddress: actor.IPAddress,
			Details:   fmt.Sprintf(`{"service":"%s"}`, serviceName),
		},
	})
}

// ProcessesList enqueues a process list snapshot request.
func (s *Service) ProcessesList(ctx context.Context, actor Actor, hostID string) (string, error) {
	return s.create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "processes",
		Action:      "list",
		Payload:     "{}",
		TriggeredBy: actor.Username,
		Audit: &dispatch.AuditLogRequest{
			Username:  actor.Username,
			Action:    "processes_list",
			HostID:    hostID,
			IPAddress: actor.IPAddress,
			Details:   "{}",
		},
	})
}

// Systemd enqueues a systemd service management command. A service name is
// required for every action except "list".
func (s *Service) Systemd(ctx context.Context, actor Actor, hostID, serviceName, action string) (string, error) {
	if !SystemdActions[action] {
		return "", apperr.Validation("invalid systemd action")
	}
	if action != "list" && !validServiceName.MatchString(serviceName) {
		return "", apperr.Validation("invalid service name")
	}
	return s.create(ctx, dispatch.Request{
		HostID:      hostID,
		Module:      "systemd",
		Action:      action,
		Target:      serviceName,
		Payload:     "{}",
		TriggeredBy: actor.Username,
		Audit: &dispatch.AuditLogRequest{
			Username:  actor.Username,
			Action:    "systemd_" + action,
			HostID:    hostID,
			IPAddress: actor.IPAddress,
			Details:   fmt.Sprintf(`{"service":"%s","action":"%s"}`, serviceName, action),
		},
	})
}

func (s *Service) create(ctx context.Context, req dispatch.Request) (string, error) {
	result, err := s.dispatcher.Create(ctx, req)
	if err != nil {
		return "", apperr.Failed("failed to create command")
	}
	return result.Command.ID, nil
}
