// Package alertrule is the application/service layer for alert-rule CRUD,
// incident listing/resolution and all the rule validation (metric/operator,
// notification actions, scope existence). It sits behind a Repository port; the
// stale-incident resolution (which needs the alerts package + concrete *DB) is
// injected as a func, like the other cross-package side effects.
package alertrule

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	ListAlertRulesAPI(ctx context.Context) ([]models.AlertRule, error)
	GetAlertRuleByID(ctx context.Context, id int64) (*models.AlertRule, error)
	CreateAlertRule(ctx context.Context, rule *models.AlertRule) error
	UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error
	DeleteAlertRule(ctx context.Context, id int64) error
	HostExists(ctx context.Context, id string) (bool, error)
	DockerContainerExists(ctx context.Context, id, hostID string) (bool, error)
	ComposeProjectExists(ctx context.Context, name, hostID string) (bool, error)
	ProxmoxConnectionExists(ctx context.Context, id string) (bool, error)
	ProxmoxNodeExists(ctx context.Context, id string) (bool, error)
	ProxmoxStorageExists(ctx context.Context, id string) (bool, error)
	ProxmoxGuestExists(ctx context.Context, id string) (bool, error)
	ProxmoxDiskExists(ctx context.Context, id string) (bool, error)
	ResolveOpenAlertIncidentsByRule(ctx context.Context, ruleID int64) (int64, error)
	ResolveAlertIncident(ctx context.Context, id int64) error
	GetAlertIncidents(ctx context.Context, limit, offset int) ([]models.AlertIncident, error)

	// capability discovery
	GetHost(ctx context.Context, id string) (*models.Host, error)
	GetDockerContainers(ctx context.Context, hostID string) ([]models.DockerContainer, error)
	GetComposeProjectsByHost(ctx context.Context, hostID string) ([]models.ComposeProject, error)
	ListAlertProxmoxConnections(ctx context.Context) ([]models.AlertScopeOption, error)
	ListAlertProxmoxNodes(ctx context.Context) ([]models.AlertScopeOption, error)
	ListAlertProxmoxStorages(ctx context.Context) ([]models.AlertScopeOption, error)
	ListAlertProxmoxGuests(ctx context.Context) ([]models.AlertScopeOption, error)
	ListAlertProxmoxDisks(ctx context.Context) ([]models.AlertScopeOption, error)
	ListAlertDockerScopeHosts(ctx context.Context) ([]models.AlertScopeOption, error)
	ProxmoxConnectionName(ctx context.Context, id string) (string, error)
	ProxmoxNodeLabelParts(ctx context.Context, id string) (connName, nodeName string, err error)
	ProxmoxStorageLabelParts(ctx context.Context, id string) (connName, nodeName, storageName string, err error)
	ProxmoxGuestLabelParts(ctx context.Context, id string) (connName, nodeName, guestName, guestType string, vmid int, err error)
	ProxmoxDiskLabelParts(ctx context.Context, id string) (connName, nodeName, devPath, model string, err error)
}

// Service holds the alert-rule use-cases.
type Service struct {
	repo Repository
	// resolveStale immediately resolves open incidents whose stored value no
	// longer meets the (new) firing condition. Wired to
	// alerts.ResolveStaleIncidentsForRule (launches its own goroutine).
	resolveStale func(rule models.AlertRule)
}

func NewService(repo Repository, resolveStale func(rule models.AlertRule)) *Service {
	return &Service{repo: repo, resolveStale: resolveStale}
}

// ===== reads =====

// List returns all alert rules (newest first, never nil).
func (s *Service) List(ctx context.Context) ([]models.AlertRule, error) {
	return s.repo.ListAlertRulesAPI(ctx)
}

// Get returns a rule by id, or apperr.NotFound.
func (s *Service) Get(ctx context.Context, id int64) (*models.AlertRule, error) {
	rule, err := s.repo.GetAlertRuleByID(ctx, id)
	if err == sql.ErrNoRows {
		return nil, apperr.NotFound("Alert rule not found")
	}
	if err != nil {
		return nil, err
	}
	return rule, nil
}

// ===== create =====

// Create validates and stores a new alert rule.
func (s *Service) Create(ctx context.Context, req models.AlertRuleCreate) (*models.AlertRule, error) {
	req.SourceType = normalizeRuleSourceType(req.SourceType, req.Metric)
	if err := validateAlertRuleMetricOperator(req.Metric, req.Operator); err != nil {
		return nil, err
	}
	if err := validateAlertActions(&req.Actions); err != nil {
		return nil, err
	}
	if req.Actions.Channels == nil {
		req.Actions.Channels = []string{}
	}

	name := req.Name
	rule := models.AlertRule{
		Name:               &name,
		Enabled:            req.Enabled,
		SourceType:         req.SourceType,
		HostID:             req.HostID,
		ProxmoxScope:       req.ProxmoxScope,
		DockerScope:        req.DockerScope,
		Metric:             req.Metric,
		Operator:           req.Operator,
		ThresholdWarn:      &req.ThresholdWarn,
		ThresholdCrit:      &req.ThresholdCrit,
		ThresholdClearWarn: req.ThresholdClearWarn,
		ThresholdClearCrit: req.ThresholdClearCrit,
		DurationSeconds:    req.Duration,
		Actions:            req.Actions,
	}
	if err := rule.Validate(); err != nil {
		return nil, apperr.Validation(err.Error())
	}
	if err := s.validateScope(ctx, &rule); err != nil {
		return nil, err
	}
	if err := s.repo.CreateAlertRule(ctx, &rule); err != nil {
		return nil, apperr.Failed(alertRuleDBError(err))
	}
	return &rule, nil
}

// ===== update =====

// Update applies the (partial) request onto the existing rule and persists it,
// then reconciles open incidents with the new condition.
func (s *Service) Update(ctx context.Context, id int64, req models.AlertRuleUpdate) error {
	existing, err := s.repo.GetAlertRuleByID(ctx, id)
	if err == sql.ErrNoRows {
		return apperr.NotFound("Regle d'alerte introuvable.")
	}
	if err != nil {
		return err
	}

	if req.SourceType != nil && *req.SourceType != existing.SourceType {
		return apperr.Validation("Le changement de source_type n'est pas autorise.")
	}

	next := *existing
	if req.Name != nil {
		next.Name = req.Name
	}
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}
	if req.HostID != nil {
		next.HostID = req.HostID
	}
	if req.Metric != nil {
		next.Metric = *req.Metric
	}
	if req.Operator != nil {
		next.Operator = *req.Operator
	}
	if req.ThresholdWarn != nil {
		next.ThresholdWarn = req.ThresholdWarn
	}
	if req.ThresholdCrit != nil {
		next.ThresholdCrit = req.ThresholdCrit
	}
	// Hysteresis clear thresholds: nil means "clear" — always applied so the
	// frontend can remove them by sending null.
	next.ThresholdClearWarn = req.ThresholdClearWarn
	next.ThresholdClearCrit = req.ThresholdClearCrit
	if req.Duration != nil {
		next.DurationSeconds = *req.Duration
	}
	if req.Actions != nil {
		next.Actions = *req.Actions
	}
	if req.ProxmoxScope != nil {
		next.ProxmoxScope = req.ProxmoxScope
	}
	if req.DockerScope != nil {
		next.DockerScope = req.DockerScope
	}

	if err := validateAlertRuleMetricOperator(next.Metric, next.Operator); err != nil {
		return err
	}
	if err := validateAlertActions(&next.Actions); err != nil {
		return err
	}
	if err := next.Validate(); err != nil {
		return apperr.Validation(err.Error())
	}
	if err := s.validateScope(ctx, &next); err != nil {
		return err
	}

	if err := s.repo.UpdateAlertRule(ctx, &next); err != nil {
		return apperr.Failed(alertRuleDBError(err))
	}

	if req.Enabled != nil && !next.Enabled {
		if _, err := s.repo.ResolveOpenAlertIncidentsByRule(ctx, next.ID); err != nil {
			return apperr.Failed("Regle mise a jour, mais echec de resolution des incidents ouverts.")
		}
	} else if s.resolveStale != nil {
		// Thresholds/hysteresis may have changed: resolve any open incident whose
		// stored value no longer meets the (new) firing condition.
		s.resolveStale(next)
	}
	return nil
}

// Delete removes a rule, returning apperr.NotFound when it does not exist.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetAlertRuleByID(ctx, id); err == sql.ErrNoRows {
		return apperr.NotFound("Regle d'alerte introuvable.")
	} else if err != nil {
		return err
	}
	return s.repo.DeleteAlertRule(ctx, id)
}

// ===== incidents =====

// ResolveIncident manually closes an open incident.
func (s *Service) ResolveIncident(ctx context.Context, id int64) error {
	return s.repo.ResolveAlertIncident(ctx, id)
}

// ListIncidents returns a page of alert incidents (never nil).
func (s *Service) ListIncidents(ctx context.Context, limit, offset int) ([]models.AlertIncident, error) {
	incidents, err := s.repo.GetAlertIncidents(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	if incidents == nil {
		incidents = []models.AlertIncident{}
	}
	return incidents, nil
}

// ===== exported validation (shared with the engine-preview test endpoints) =====

// ValidateMetricOperator validates a metric/operator pair.
func (s *Service) ValidateMetricOperator(metric, operator string) error {
	return validateAlertRuleMetricOperator(metric, operator)
}

// ValidateActions validates a rule's notification actions.
func (s *Service) ValidateActions(actions *models.AlertActions) error {
	return validateAlertActions(actions)
}

// ValidateProxmoxScope checks a Proxmox scope's referenced entities exist.
func (s *Service) ValidateProxmoxScope(ctx context.Context, scope *models.ProxmoxMetricScope) error {
	return s.validateProxmoxScope(ctx, scope)
}

// ValidateDockerScope checks a Docker scope's referenced entities exist.
func (s *Service) ValidateDockerScope(ctx context.Context, scope *models.DockerMetricScope) error {
	return s.validateDockerScope(ctx, scope)
}

// ===== scope validation =====

func (s *Service) validateScope(ctx context.Context, rule *models.AlertRule) error {
	switch rule.SourceType {
	case models.AlertSourceProxmox:
		return s.validateProxmoxScope(ctx, rule.ProxmoxScope)
	case models.AlertSourceDocker:
		return s.validateDockerScope(ctx, rule.DockerScope)
	}
	return nil
}

func (s *Service) validateDockerScope(ctx context.Context, scope *models.DockerMetricScope) error {
	if scope == nil {
		return apperr.Validation("Le scope Docker est requis.")
	}
	if ok, _ := s.repo.HostExists(ctx, scope.HostID); !ok {
		return apperr.Validation("Hôte introuvable pour ce scope Docker.")
	}
	if scope.ScopeMode == "container" && scope.ContainerID != "" {
		if ok, _ := s.repo.DockerContainerExists(ctx, scope.ContainerID, scope.HostID); !ok {
			return apperr.Validation("Container Docker introuvable pour ce scope.")
		}
	}
	if scope.ScopeMode == "compose_project" && scope.ProjectName != "" {
		if ok, _ := s.repo.ComposeProjectExists(ctx, scope.ProjectName, scope.HostID); !ok {
			return apperr.Validation("Projet Compose introuvable pour ce scope.")
		}
	}
	return nil
}

func (s *Service) validateProxmoxScope(ctx context.Context, scope *models.ProxmoxMetricScope) error {
	if scope == nil {
		return apperr.Validation("Le scope Proxmox est requis.")
	}
	switch scope.ScopeMode {
	case "connection":
		if ok, _ := s.repo.ProxmoxConnectionExists(ctx, scope.ConnectionID); !ok {
			return apperr.Validation("Connexion Proxmox introuvable pour ce scope.")
		}
	case "node":
		if ok, _ := s.repo.ProxmoxNodeExists(ctx, scope.NodeID); !ok {
			return apperr.Validation("Noeud Proxmox introuvable pour ce scope.")
		}
	case "storage":
		if ok, _ := s.repo.ProxmoxStorageExists(ctx, scope.StorageID); !ok {
			return apperr.Validation("Stockage Proxmox introuvable pour ce scope.")
		}
	case "guest":
		if ok, _ := s.repo.ProxmoxGuestExists(ctx, scope.GuestID); !ok {
			return apperr.Validation("VM/LXC Proxmox introuvable pour ce scope.")
		}
	case "disk":
		if ok, _ := s.repo.ProxmoxDiskExists(ctx, scope.DiskID); !ok {
			return apperr.Validation("Disque physique Proxmox introuvable pour ce scope.")
		}
	}
	return nil
}

// ===== validation rules + maps =====

var validAlertOperators = map[string]bool{">": true, "<": true, ">=": true, "<=": true}

var validAlertChannels = map[string]bool{
	"smtp": true, "ntfy": true, "browser": true, "notify": true,
}

var commandModuleActions = map[string][]string{
	"docker":    {"logs", "restart", "start", "stop", "compose_up", "compose_down", "compose_pull", "compose_logs", "compose_restart"},
	"journal":   {"read"},
	"apt":       {"update", "upgrade", "full-upgrade", "autoremove"},
	"systemd":   {"status", "start", "stop", "restart", "list"},
	"processes": {"list"},
	"custom":    {"run"},
}

var commandModuleRequiresTarget = map[string]bool{
	"journal": true,
	"systemd": true,
	"custom":  true,
}

var validAlertMetrics = map[string]bool{
	"cpu": true, "memory": true, "disk": true, "load": true, "heartbeat_timeout": true,
	"status_offline":  true,
	"cpu_temperature": true, "disk_smart_status": true, "disk_temperature": true, "proxmox_storage_percent": true,
	"proxmox_node_cpu_percent": true, "proxmox_node_memory_percent": true,
	"proxmox_node_cpu_temperature": true, "proxmox_node_fan_rpm": true,
	"proxmox_guest_cpu_percent": true, "proxmox_guest_memory_percent": true,
	"proxmox_node_pending_updates":    true,
	"proxmox_recent_failed_tasks_24h": true,
	"proxmox_auth_failures_recent":    true,
	"proxmox_disk_failed_count":       true, "proxmox_disk_min_wearout_percent": true,
	"docker_container_state": true, "docker_compose_degraded_services": true,
}

func validateAlertRuleMetricOperator(metric, operator string) error {
	if !validAlertOperators[operator] {
		return apperr.Validation("Operateur invalide.")
	}
	if !validAlertMetrics[metric] {
		return apperr.Validation("Metrique invalide.")
	}
	return nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func validateAlertActions(actions *models.AlertActions) error {
	if actions == nil {
		return nil
	}
	if actions.Cooldown < 0 {
		return apperr.Validation("La periode de silence doit etre positive ou nulle.")
	}
	for _, channel := range actions.Channels {
		if !validAlertChannels[channel] {
			return apperr.Validation(fmt.Sprintf("Canal de notification invalide: %s", channel))
		}
	}
	if actions.CommandTrigger != nil {
		ct := actions.CommandTrigger
		ct.Module = strings.TrimSpace(ct.Module)
		ct.Action = strings.TrimSpace(ct.Action)
		ct.Target = strings.TrimSpace(ct.Target)
		if ct.Module == "" || ct.Action == "" {
			return apperr.Validation("Le declencheur de commande doit definir un module et une action.")
		}
		allowedActions, ok := commandModuleActions[ct.Module]
		if !ok {
			return apperr.Validation(fmt.Sprintf("Module de commande invalide: %s", ct.Module))
		}
		if !containsString(allowedActions, ct.Action) {
			return apperr.Validation(fmt.Sprintf("Action invalide pour le module %s: %s", ct.Module, ct.Action))
		}
		if commandModuleRequiresTarget[ct.Module] && ct.Target == "" {
			return apperr.Validation(fmt.Sprintf("Le module %s requiert une cible.", ct.Module))
		}
		if !commandModuleRequiresTarget[ct.Module] {
			ct.Target = ""
		}
	}
	return nil
}

func normalizeRuleSourceType(source models.AlertSourceType, metric string) models.AlertSourceType {
	if source == "" {
		return models.InferAlertSourceType(metric)
	}
	return source
}

// alertRuleDBError translates known PostgreSQL constraint violations on
// alert_rules into user-readable messages (the last safety net before the wire).
func alertRuleDBError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "chk_alert_rules_source_type") {
		return "Le type de source de cette règle n'est pas encore supporté par la base de données. Relancez le serveur pour appliquer les migrations en attente."
	}
	if strings.Contains(msg, "alert_rules_rebuilt_pkey") || strings.Contains(msg, "duplicate key") {
		return "Une erreur de base de données s'est produite lors de la création. Veuillez réessayer."
	}
	return msg
}
