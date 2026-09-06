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
	AcknowledgeAlertIncident(ctx context.Context, id int64, username string) error
	GetAlertIncidents(ctx context.Context, limit, offset int) ([]models.AlertIncident, error)
	GetAllHosts(ctx context.Context) ([]models.Host, error)

	// rule templates (ROADMAP.md item #9)
	CreateAlertRuleTemplate(ctx context.Context, t *models.AlertRuleTemplate) error
	GetAlertRuleTemplates(ctx context.Context) ([]models.AlertRuleTemplate, error)
	GetAlertRuleTemplateByID(ctx context.Context, id int64) (*models.AlertRuleTemplate, error)
	UpdateAlertRuleTemplate(ctx context.Context, t *models.AlertRuleTemplate) error
	DeleteAlertRuleTemplate(ctx context.Context, id int64) error

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
	// engine holds the alert-engine entry points used by the preview endpoints.
	engine EngineFuncs
}

func NewService(repo Repository, resolveStale func(rule models.AlertRule), engine EngineFuncs) *Service {
	return &Service{repo: repo, resolveStale: resolveStale, engine: engine}
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
		return nil, apperr.NotFound("alert rule not found").I18n(apperr.CodeAlertRuleNotFound, nil)
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
	if err := validateBaselineWindow(req.Metric, req.BaselineWindowSeconds); err != nil {
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
		Name:                  &name,
		Enabled:               req.Enabled,
		SourceType:            req.SourceType,
		HostID:                req.HostID,
		ProxmoxScope:          req.ProxmoxScope,
		DockerScope:           req.DockerScope,
		Metric:                req.Metric,
		Operator:              req.Operator,
		ThresholdWarn:         &req.ThresholdWarn,
		ThresholdCrit:         &req.ThresholdCrit,
		ThresholdClearWarn:    req.ThresholdClearWarn,
		ThresholdClearCrit:    req.ThresholdClearCrit,
		DurationSeconds:       req.Duration,
		BaselineWindowSeconds: req.BaselineWindowSeconds,
		Actions:               req.Actions,
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
		return apperr.NotFound("alert rule not found").I18n(apperr.CodeAlertRuleNotFound, nil)
	}
	if err != nil {
		return err
	}

	if req.SourceType != nil && *req.SourceType != existing.SourceType {
		return apperr.Validation("changing source_type is not allowed").I18n(apperr.CodeAlertSourceTypeImmutable, nil)
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
	if req.BaselineWindowSeconds != nil {
		next.BaselineWindowSeconds = req.BaselineWindowSeconds
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
	if err := validateBaselineWindow(next.Metric, next.BaselineWindowSeconds); err != nil {
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
			return apperr.Failed("rule updated, but resolving its open incidents failed").I18n(apperr.CodeAlertIncidentResolveFailed, nil)
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
		return apperr.NotFound("alert rule not found").I18n(apperr.CodeAlertRuleNotFound, nil)
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

// AcknowledgeIncident marks an open incident as being handled, which also
// stops the engine's escalation re-notifications for it (see
// AlertActions.EscalateAfterMinutes). A no-op if already acknowledged or
// already resolved.
func (s *Service) AcknowledgeIncident(ctx context.Context, id int64, username string) error {
	return s.repo.AcknowledgeAlertIncident(ctx, id, username)
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
		return apperr.Validation("a Docker scope is required").I18n(apperr.CodeAlertDockerScopeRequired, nil)
	}
	if ok, _ := s.repo.HostExists(ctx, scope.HostID); !ok {
		return apperr.Validation("host not found for this Docker scope").I18n(apperr.CodeAlertDockerHostNotFound, nil)
	}
	if scope.ScopeMode == "container" {
		for _, id := range scope.EffectiveContainerIDs() {
			if ok, _ := s.repo.DockerContainerExists(ctx, id, scope.HostID); !ok {
				return apperr.Validation("Docker container not found for this scope").I18n(apperr.CodeAlertDockerContainerNotFound, nil)
			}
		}
	}
	if scope.ScopeMode == "compose_project" && scope.ProjectName != "" {
		if ok, _ := s.repo.ComposeProjectExists(ctx, scope.ProjectName, scope.HostID); !ok {
			return apperr.Validation("Compose project not found for this scope").I18n(apperr.CodeAlertComposeProjectNotFound, nil)
		}
	}
	return nil
}

func (s *Service) validateProxmoxScope(ctx context.Context, scope *models.ProxmoxMetricScope) error {
	if scope == nil {
		return apperr.Validation("a Proxmox scope is required").I18n(apperr.CodeAlertProxmoxScopeRequired, nil)
	}
	switch scope.ScopeMode {
	case "connection":
		if ok, _ := s.repo.ProxmoxConnectionExists(ctx, scope.ConnectionID); !ok {
			return apperr.Validation("Proxmox connection not found for this scope").I18n(apperr.CodeAlertProxmoxConnNotFound, nil)
		}
	case "node":
		if ok, _ := s.repo.ProxmoxNodeExists(ctx, scope.NodeID); !ok {
			return apperr.Validation("Proxmox node not found for this scope").I18n(apperr.CodeAlertProxmoxNodeNotFound, nil)
		}
	case "storage":
		if ok, _ := s.repo.ProxmoxStorageExists(ctx, scope.StorageID); !ok {
			return apperr.Validation("Proxmox storage not found for this scope").I18n(apperr.CodeAlertProxmoxStorageNotFound, nil)
		}
	case "guest":
		if ok, _ := s.repo.ProxmoxGuestExists(ctx, scope.GuestID); !ok {
			return apperr.Validation("Proxmox VM/LXC not found for this scope").I18n(apperr.CodeAlertProxmoxGuestNotFound, nil)
		}
	case "disk":
		if ok, _ := s.repo.ProxmoxDiskExists(ctx, scope.DiskID); !ok {
			return apperr.Validation("Proxmox physical disk not found for this scope").I18n(apperr.CodeAlertProxmoxDiskNotFound, nil)
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
	"restic_backup_age_hours": true, "restic_repo_size_bytes": true,
	"bandwidth_vs_rolling_avg": true,
}

func validateAlertRuleMetricOperator(metric, operator string) error {
	if !validAlertOperators[operator] {
		return apperr.Validation("invalid operator").I18n(apperr.CodeInvalidOperator, nil)
	}
	if !validAlertMetrics[metric] {
		return apperr.Validation("invalid metric").I18n(apperr.CodeInvalidMetric, nil)
	}
	return nil
}

// validBaselineWindowsSeconds are the only presets the frontend offers for
// bandwidth_vs_rolling_avg's rolling-baseline window: 1h/6h/24h.
var validBaselineWindowsSeconds = map[int]bool{3600: true, 21600: true, 86400: true}

// validateBaselineWindow checks AlertRule.BaselineWindowSeconds when set —
// only bandwidth_vs_rolling_avg uses it today. A nil value is always valid
// (the engine defaults it to 3600/1h), and any metric may safely leave it
// nil; only a non-nil value on a metric that doesn't understand it, or an
// off-preset value, is rejected.
func validateBaselineWindow(metric string, windowSeconds *int) error {
	if windowSeconds == nil {
		return nil
	}
	if metric != "bandwidth_vs_rolling_avg" {
		return apperr.Validation("the rolling-average window does not apply to this metric").I18n(apperr.CodeAlertRollingWindowNotAllowed, nil)
	}
	if !validBaselineWindowsSeconds[*windowSeconds] {
		return apperr.Validation("invalid rolling-average window (allowed: 1h, 6h, 24h)").I18n(apperr.CodeAlertRollingWindowInvalid, nil)
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
		return apperr.Validation("the silence period must be zero or positive").I18n(apperr.CodeAlertCooldownNegative, nil)
	}
	if actions.EscalateAfterMinutes < 0 {
		return apperr.Validation("the escalation delay must be zero or positive").I18n(apperr.CodeAlertEscalationNegative, nil)
	}
	for _, channel := range actions.Channels {
		if !validAlertChannels[channel] {
			return apperr.Validation(fmt.Sprintf("invalid notification channel: %s", channel)).I18n(apperr.CodeAlertChannelInvalid, map[string]string{"channel": channel})
		}
	}
	if actions.CommandTrigger != nil {
		ct := actions.CommandTrigger
		ct.Module = strings.TrimSpace(ct.Module)
		ct.Action = strings.TrimSpace(ct.Action)
		ct.Target = strings.TrimSpace(ct.Target)
		if ct.Module == "" || ct.Action == "" {
			return apperr.Validation("a command trigger must define both a module and an action").I18n(apperr.CodeAlertCommandTriggerIncomplete, nil)
		}
		allowedActions, ok := commandModuleActions[ct.Module]
		if !ok {
			return apperr.Validation(fmt.Sprintf("invalid command module: %s", ct.Module)).I18n(apperr.CodeAlertCommandModuleInvalid, map[string]string{"module": ct.Module})
		}
		if !containsString(allowedActions, ct.Action) {
			return apperr.Validation(fmt.Sprintf("invalid action for module %s: %s", ct.Module, ct.Action)).I18n(apperr.CodeAlertCommandActionInvalid, map[string]string{"module": ct.Module, "action": ct.Action})
		}
		if commandModuleRequiresTarget[ct.Module] && ct.Target == "" {
			return apperr.Validation(fmt.Sprintf("module %s requires a target", ct.Module)).I18n(apperr.CodeAlertCommandTargetRequired, map[string]string{"module": ct.Module})
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
