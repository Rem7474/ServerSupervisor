package models

import (
	"fmt"
	"strings"
	"time"
)

// ========== Alerts ==========

// CommandTrigger defines a remote command to execute automatically when an alert fires.
type CommandTrigger struct {
	Module  string `json:"module"`            // e.g. "processes", "journal", "docker", "systemd"
	Action  string `json:"action"`            // e.g. "list", "read", "restart"
	Target  string `json:"target,omitempty"`  // e.g. service name, container name
	Payload string `json:"payload,omitempty"` // optional JSON payload
}

// ProxmoxMetricScope defines how a Proxmox metric should be evaluated.
// ScopeMode can be one of: global, connection, node, storage.
type ProxmoxMetricScope struct {
	ScopeMode    string `json:"scope_mode,omitempty"`
	ConnectionID string `json:"connection_id,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	StorageID    string `json:"storage_id,omitempty"`
	GuestID      string `json:"guest_id,omitempty"`
	DiskID       string `json:"disk_id,omitempty"`
}

// DockerMetricScope defines how a Docker metric should be evaluated.
// ScopeMode can be one of: host, container, compose_project.
// HostID is always required. ContainerID or ProjectName are required for their respective modes.
// WarnStates and CritStates are used by docker_container_state to select which container states trigger each severity.
type DockerMetricScope struct {
	ScopeMode string `json:"scope_mode"`
	HostID    string `json:"host_id"`
	// ContainerID is the legacy single-container field, kept for rules saved
	// before multi-container selection existed. New rules are written to
	// ContainerIDs instead — see EffectiveContainerIDs for the read-time
	// migration that lets both shapes keep working unmodified.
	ContainerID  string   `json:"container_id,omitempty"`  // DB UUID of docker_containers row
	ContainerIDs []string `json:"container_ids,omitempty"` // DB UUIDs of docker_containers rows
	ProjectName  string   `json:"project_name,omitempty"`  // compose project name
	WarnStates   []string `json:"warn_states,omitempty"`   // container states triggering warn
	CritStates   []string `json:"crit_states,omitempty"`   // container states triggering crit
}

// EffectiveContainerIDs returns the containers a "container" scope_mode rule
// actually applies to: ContainerIDs when set (current shape), else a
// single-element slice built from the legacy ContainerID field so a rule
// saved before multi-select existed keeps matching exactly the one container
// it always did, with no migration/backfill needed. Nil for neither set.
func (s *DockerMetricScope) EffectiveContainerIDs() []string {
	if len(s.ContainerIDs) > 0 {
		return s.ContainerIDs
	}
	if s.ContainerID != "" {
		return []string{s.ContainerID}
	}
	return nil
}

type AlertSourceType string

const (
	AlertSourceAgent   AlertSourceType = "agent"
	AlertSourceProxmox AlertSourceType = "proxmox"
	AlertSourceDocker  AlertSourceType = "docker"
)

// ===== Alert capability discovery (metric catalogs + scope options) =====

// AlertMetricCapability describes a metric the UI can build a rule on.
type AlertMetricCapability struct {
	Metric             string `json:"metric"`
	Label              string `json:"label"`
	Unit               string `json:"unit"`
	Icon               string `json:"icon"`
	BadgeClass         string `json:"badge_class"`
	SupportsThreshold  bool   `json:"supports_threshold"`
	SupportsDuration   bool   `json:"supports_duration"`
	SupportsHostFilter bool   `json:"supports_host_filter"`
}

// AlertScopeOption is a selectable {id,label} scope entry (Proxmox connection,
// node, storage, guest, disk, or a Docker host).
type AlertScopeOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// AlertHostCapabilities is the per-host metric availability response.
type AlertHostCapabilities struct {
	HostID   string                  `json:"host_id"`
	HostName string                  `json:"host_name"`
	Metrics  []AlertMetricCapability `json:"metrics"`
}

// AlertProxmoxScope groups the Proxmox scope options by kind.
type AlertProxmoxScope struct {
	Modes       []string           `json:"modes"`
	Connections []AlertScopeOption `json:"connections"`
	Nodes       []AlertScopeOption `json:"nodes"`
	Storages    []AlertScopeOption `json:"storages"`
	Guests      []AlertScopeOption `json:"guests"`
	Disks       []AlertScopeOption `json:"disks"`
}

// AlertSplitCapabilities is the Proxmox capabilities response (metrics + scope).
type AlertSplitCapabilities struct {
	AgentMetrics   []AlertMetricCapability `json:"agent_metrics"`
	ProxmoxMetrics []AlertMetricCapability `json:"proxmox_metrics"`
	ProxmoxScope   AlertProxmoxScope       `json:"proxmox_scope"`
}

// AlertDockerScopeContainer is a container option for a Docker alert scope.
type AlertDockerScopeContainer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	State string `json:"state"`
}

// AlertDockerScopeProject is a compose-project option for a Docker alert scope.
type AlertDockerScopeProject struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

// AlertDockerHostScope groups a host's container + compose-project scope options.
type AlertDockerHostScope struct {
	HostID     string                      `json:"host_id"`
	HostName   string                      `json:"host_name"`
	Containers []AlertDockerScopeContainer `json:"containers"`
	Projects   []AlertDockerScopeProject   `json:"projects"`
}

// AlertDockerCapabilities is the Docker capabilities response (metrics + hosts).
type AlertDockerCapabilities struct {
	Metrics []AlertMetricCapability `json:"metrics"`
	Hosts   []AlertDockerHostScope  `json:"hosts"`
}

// AlertActions holds the consolidated notification configuration for an alert rule.
// Stored as a single JSONB column in the database.
type AlertActions struct {
	Channels       []string        `json:"channels"`                  // e.g. ["smtp", "ntfy", "browser"]
	SMTPTo         string          `json:"smtp_to,omitempty"`         // SMTP recipient address(es)
	NtfyTopic      string          `json:"ntfy_topic,omitempty"`      // ntfy push notification topic
	Cooldown       int             `json:"cooldown,omitempty"`        // seconds between re-notifications (0 = no cooldown)
	CommandTrigger *CommandTrigger `json:"command_trigger,omitempty"` // optional command to run on alert
	// EscalateAfterMinutes, when > 0, re-sends the fired notification for an
	// open incident that hasn't been acknowledged, every N minutes since it
	// triggered (or since the last escalation) — see internal/alerts/engine.go.
	// 0 (default) disables escalation entirely. Unlike Cooldown, this never
	// suppresses the *first* notification; it only repeats an unacknowledged
	// one.
	EscalateAfterMinutes int `json:"escalate_after_minutes,omitempty"`
}

type AlertRule struct {
	ID                  int64               `json:"id" db:"id"`
	Name                *string             `json:"name,omitempty" db:"name"`
	SourceType          AlertSourceType     `json:"source_type,omitempty" db:"source_type"`
	HostID              *string             `json:"host_id" db:"host_id"`
	ProxmoxScope        *ProxmoxMetricScope `json:"proxmox_scope,omitempty" db:"proxmox_scope"`
	DockerScope         *DockerMetricScope  `json:"docker_scope,omitempty" db:"docker_scope"`
	Metric              string              `json:"metric" db:"metric"`
	Operator            string              `json:"operator" db:"operator"`
	ThresholdWarn       *float64            `json:"threshold_warn" db:"threshold_warn"`
	ThresholdCrit       *float64            `json:"threshold_crit" db:"threshold_crit"`
	ThresholdClearWarn  *float64            `json:"threshold_clear_warn,omitempty" db:"threshold_clear_warn"` // hysteresis for warn
	ThresholdClearCrit  *float64            `json:"threshold_clear_crit,omitempty" db:"threshold_clear_crit"` // hysteresis for crit
	DurationSeconds     int                 `json:"duration_seconds" db:"duration_seconds"`
	Actions             AlertActions        `json:"actions" db:"-"` // stored as JSONB in DB
	LastFired           *time.Time          `json:"last_fired,omitempty" db:"last_fired"`
	Enabled             bool                `json:"enabled" db:"enabled"`
	CreatedAt           time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt           *time.Time          `json:"updated_at,omitempty" db:"updated_at"`
	ActiveIncidentCount int                 `json:"active_incident_count" db:"-"`
}

// DisplayName returns the human-readable label for a rule: its custom Name if
// set, otherwise "<metric> <operator> <threshold> (crit|warn)" built from
// whichever threshold is configured. This is the single formula behind every
// rendering of a rule's name — the live WS/push notifications and the REST
// notification feed both call it, so they can't drift apart.
func (r AlertRule) DisplayName() string {
	if r.Name != nil {
		return *r.Name
	}
	if r.ThresholdCrit != nil {
		return fmt.Sprintf("%s %s %.2f (crit)", r.Metric, r.Operator, *r.ThresholdCrit)
	}
	if r.ThresholdWarn != nil {
		return fmt.Sprintf("%s %s %.2f (warn)", r.Metric, r.Operator, *r.ThresholdWarn)
	}
	return ""
}

type AlertIncident struct {
	ID          int64      `json:"id" db:"id"`
	RuleID      *int64     `json:"rule_id" db:"rule_id"`
	HostID      string     `json:"host_id" db:"host_id"`
	Severity    string     `json:"severity" db:"severity"` // "warn" or "crit"
	TriggeredAt time.Time  `json:"triggered_at" db:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at" db:"resolved_at"`
	Value       float64    `json:"value" db:"value"`
	// CommandID is the remote_commands row a command_trigger dispatched when
	// this incident fired, if the rule has one configured. Nil otherwise.
	CommandID *string `json:"command_id,omitempty" db:"command_id"`
	// AcknowledgedAt/AcknowledgedBy mark that someone is handling this incident
	// — orthogonal to ResolvedAt (see migration 087's comment). Both nil until
	// AcknowledgeIncident is called; never cleared once resolved.
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	AcknowledgedBy *string    `json:"acknowledged_by,omitempty" db:"acknowledged_by"`
	// LastEscalatedAt is the last time the engine re-sent a notification for
	// this still-open, unacknowledged incident (see AlertActions.EscalateAfterMinutes).
	// Not exposed in the incidents list JSON — internal engine bookkeeping only.
	LastEscalatedAt *time.Time `json:"-" db:"last_escalated_at"`
	// CorrelatedWith is the id of the host's own open status_offline/
	// heartbeat_timeout incident this one was linked to at creation time — a
	// host-down cascade (e.g. every Docker container on that host firing its
	// own incident) is still recorded per-incident but doesn't independently
	// notify or escalate (see maybeCorrelateWithHostDown/maybeEscalateIncident
	// in internal/alerts/engine.go). Nil for an uncorrelated incident, or for
	// a host-down incident itself (it's never correlated with another one).
	CorrelatedWith *int64 `json:"correlated_with,omitempty" db:"correlated_with"`
	// Enriched post-fetch (not DB columns): Docker synthetic IDs resolution,
	// and the live status of CommandID's remote_commands row (joined at read
	// time so the frontend doesn't need a second round-trip per incident).
	ValueLabel    string `json:"value_label,omitempty" db:"-"`
	LinkHostID    string `json:"link_host_id,omitempty" db:"-"`
	CommandStatus string `json:"command_status,omitempty" db:"-"`
}

type NotificationItem struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	RuleID        *int64     `json:"rule_id"`
	HostID        string     `json:"host_id"`
	HostName      string     `json:"host_name"`
	SourceType    string     `json:"source_type,omitempty"`
	SourceLabel   string     `json:"source_label,omitempty"`
	RuleName      string     `json:"rule_name"`
	Metric        string     `json:"metric"`
	Severity      string     `json:"severity,omitempty"`
	Status        string     `json:"status,omitempty"`
	TrackerID     string     `json:"tracker_id,omitempty"`
	TrackerType   string     `json:"tracker_type,omitempty"`
	ReleaseURL    string     `json:"release_url,omitempty"`
	ReleaseName   string     `json:"release_name,omitempty"`
	Version       string     `json:"version,omitempty"`
	Value         float64    `json:"value"`
	TriggeredAt   time.Time  `json:"triggered_at"`
	ResolvedAt    *time.Time `json:"resolved_at"`
	BrowserNotify bool       `json:"browser_notify"`
	// CurrentValue / ClearThreshold are populated only for active alert
	// incidents: the live metric value and the threshold it must cross to
	// resolve (hysteresis clear threshold, or the trigger threshold otherwise).
	CurrentValue   *float64 `json:"current_value,omitempty"`
	ClearThreshold *float64 `json:"clear_threshold,omitempty"`
	Operator       string   `json:"operator,omitempty"`
	// Docker synthetic ID resolution: real host to navigate to, and human state label
	LinkHostID string `json:"link_host_id,omitempty"`
	ValueLabel string `json:"value_label,omitempty"`
	// CommandStatus is the remote_commands.status of the rule's command_trigger
	// dispatch for this incident (alert_incident type only), joined from
	// alert_incidents.command_id — empty when no command_trigger fired.
	CommandStatus string `json:"command_status,omitempty"`
	// AcknowledgedAt/AcknowledgedBy mirror AlertIncident's fields (alert_incident
	// type only; always nil for a release-tracker entry).
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AcknowledgedBy string     `json:"acknowledged_by,omitempty"`
	// CorrelatedWith mirrors AlertIncident.CorrelatedWith (alert_incident type
	// only) — lets the UI mark a host-down cascade child without a second call.
	CorrelatedWith *int64 `json:"correlated_with,omitempty"`
}

// PushSubscription represents a Web Push (VAPID) subscription for a user's browser/device.
// Stored server-side so that alert notifications can be delivered even when the app is closed.
type PushSubscription struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Endpoint  string    `json:"endpoint"`
	P256DHKey string    `json:"p256dh"`
	AuthKey   string    `json:"auth"`
	UserAgent string    `json:"user_agent,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ========== Alert Rules - Create/Update Helpers ==========

type AlertRuleCreate struct {
	Name               string              `json:"name" binding:"required"`
	Enabled            bool                `json:"enabled"`
	SourceType         AlertSourceType     `json:"source_type"`
	HostID             *string             `json:"host_id"`
	ProxmoxScope       *ProxmoxMetricScope `json:"proxmox_scope"`
	DockerScope        *DockerMetricScope  `json:"docker_scope"`
	Metric             string              `json:"metric" binding:"required"`
	Operator           string              `json:"operator" binding:"required"`
	ThresholdWarn      float64             `json:"threshold_warn" binding:"required"`
	ThresholdCrit      float64             `json:"threshold_crit" binding:"required"`
	ThresholdClearWarn *float64            `json:"threshold_clear_warn"`
	ThresholdClearCrit *float64            `json:"threshold_clear_crit"`
	Duration           int                 `json:"duration"`
	Actions            AlertActions        `json:"actions"`
}

// AlertRuleTemplate is a reusable rule "recipe" for agent metrics — no host,
// so ApplyAlertRuleTemplateRequest can stamp out the same rule across many
// hosts at once instead of re-entering the same metric/thresholds/actions
// per host (ROADMAP.md item #9). Not a live link to the rules it spawns:
// editing or deleting a template never touches an already-created rule (same
// "definition + independent instances" shape as runbooks/runbook_steps).
// Docker/Proxmox-scoped rules aren't templatable in this MVP — Docker
// scope's host_id is required per rule and Proxmox scope is cluster-level
// already, so neither fits "apply the same recipe to N hosts."
type AlertRuleTemplate struct {
	ID                 int64        `json:"id" db:"id"`
	Name               string       `json:"name" db:"name"`
	Metric             string       `json:"metric" db:"metric"`
	Operator           string       `json:"operator" db:"operator"`
	ThresholdWarn      float64      `json:"threshold_warn" db:"threshold_warn"`
	ThresholdCrit      float64      `json:"threshold_crit" db:"threshold_crit"`
	ThresholdClearWarn *float64     `json:"threshold_clear_warn,omitempty" db:"threshold_clear_warn"`
	ThresholdClearCrit *float64     `json:"threshold_clear_crit,omitempty" db:"threshold_clear_crit"`
	DurationSeconds    int          `json:"duration_seconds" db:"duration_seconds"`
	Actions            AlertActions `json:"actions" db:"-"`
	CreatedAt          time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at" db:"updated_at"`
}

type AlertRuleTemplateRequest struct {
	Name               string       `json:"name" binding:"required"`
	Metric             string       `json:"metric" binding:"required"`
	Operator           string       `json:"operator" binding:"required"`
	ThresholdWarn      float64      `json:"threshold_warn" binding:"required"`
	ThresholdCrit      float64      `json:"threshold_crit" binding:"required"`
	ThresholdClearWarn *float64     `json:"threshold_clear_warn"`
	ThresholdClearCrit *float64     `json:"threshold_clear_crit"`
	Duration           int          `json:"duration"`
	Actions            AlertActions `json:"actions"`
}

// ApplyAlertRuleTemplateRequest is the body of POST /alert-rule-templates/:id/apply.
// Enabled defaults to false (zero value) — applying a template to a fleet
// shouldn't immediately start firing everywhere before the admin has
// reviewed the resulting per-host rules.
type ApplyAlertRuleTemplateRequest struct {
	HostIDs []string `json:"host_ids" binding:"required,min=1"`
	Enabled bool     `json:"enabled"`
}

// ApplyAlertRuleTemplateResult reports what happened per host — applying to
// N hosts is not all-or-nothing, one host's failure shouldn't block the rest.
type ApplyAlertRuleTemplateResult struct {
	CreatedRuleIDs []int64           `json:"created_rule_ids"`
	Errors         map[string]string `json:"errors,omitempty"` // host_id -> error message
}

type AlertRuleUpdate struct {
	Name               *string             `json:"name"`
	Enabled            *bool               `json:"enabled"`
	SourceType         *AlertSourceType    `json:"source_type"`
	HostID             *string             `json:"host_id"`
	ProxmoxScope       *ProxmoxMetricScope `json:"proxmox_scope"`
	DockerScope        *DockerMetricScope  `json:"docker_scope"`
	Metric             *string             `json:"metric"`
	Operator           *string             `json:"operator"`
	ThresholdWarn      *float64            `json:"threshold_warn"`
	ThresholdCrit      *float64            `json:"threshold_crit"`
	ThresholdClearWarn *float64            `json:"threshold_clear_warn"`
	ThresholdClearCrit *float64            `json:"threshold_clear_crit"`
	Duration           *int                `json:"duration"`
	Actions            *AlertActions       `json:"actions"`
}

func IsDockerMetric(metric string) bool {
	switch metric {
	case "docker_container_state", "docker_compose_degraded_services":
		return true
	default:
		return false
	}
}

func IsProxmoxMetric(metric string) bool {
	switch metric {
	case "proxmox_storage_percent", "proxmox_node_cpu_percent", "proxmox_node_memory_percent",
		"proxmox_node_cpu_temperature", "proxmox_node_fan_rpm",
		"proxmox_guest_cpu_percent", "proxmox_guest_memory_percent",
		"proxmox_node_pending_updates",
		"proxmox_recent_failed_tasks_24h",
		"proxmox_auth_failures_recent",
		"proxmox_disk_failed_count", "proxmox_disk_min_wearout_percent":
		return true
	default:
		return false
	}
}

func InferAlertSourceType(metric string) AlertSourceType {
	if IsDockerMetric(metric) {
		return AlertSourceDocker
	}
	if IsProxmoxMetric(metric) {
		return AlertSourceProxmox
	}
	return AlertSourceAgent
}

func (ps *ProxmoxMetricScope) Validate(metric string) error {
	if ps == nil {
		return fmt.Errorf("le scope Proxmox est requis")
	}

	ps.ScopeMode = strings.TrimSpace(ps.ScopeMode)
	if ps.ScopeMode == "" {
		ps.ScopeMode = "global"
	}

	validModes := map[string]bool{"global": true, "connection": true, "node": true, "storage": true, "guest": true, "disk": true}
	if !validModes[ps.ScopeMode] {
		return fmt.Errorf("scope Proxmox invalide")
	}

	ps.ConnectionID = strings.TrimSpace(ps.ConnectionID)
	ps.NodeID = strings.TrimSpace(ps.NodeID)
	ps.StorageID = strings.TrimSpace(ps.StorageID)
	ps.GuestID = strings.TrimSpace(ps.GuestID)
	ps.DiskID = strings.TrimSpace(ps.DiskID)

	switch ps.ScopeMode {
	case "connection":
		if metric == "proxmox_guest_cpu_percent" || metric == "proxmox_guest_memory_percent" {
			return fmt.Errorf("les metriques VM/LXC Proxmox ne supportent pas le scope connexion")
		}
		if ps.ConnectionID == "" {
			return fmt.Errorf("le scope connexion requiert une connexion Proxmox")
		}
	case "node":
		if metric == "proxmox_guest_cpu_percent" || metric == "proxmox_guest_memory_percent" {
			return fmt.Errorf("les metriques VM/LXC Proxmox ne supportent pas le scope noeud")
		}
		if ps.NodeID == "" {
			return fmt.Errorf("le scope noeud requiert un noeud Proxmox")
		}
	case "storage":
		if metric != "proxmox_storage_percent" {
			return fmt.Errorf("le scope stockage n'est disponible que pour la metrique de stockage Proxmox")
		}
		if ps.StorageID == "" {
			return fmt.Errorf("le scope stockage requiert un stockage Proxmox")
		}
	case "guest":
		if metric != "proxmox_guest_cpu_percent" && metric != "proxmox_guest_memory_percent" {
			return fmt.Errorf("le scope guest n'est disponible que pour les metriques VM/LXC Proxmox")
		}
		if ps.GuestID == "" {
			return fmt.Errorf("le scope guest requiert une VM/LXC Proxmox")
		}
	case "disk":
		if metric != "proxmox_disk_failed_count" && metric != "proxmox_disk_min_wearout_percent" {
			return fmt.Errorf("le scope disque n'est disponible que pour les metriques de disques physiques Proxmox")
		}
		if ps.DiskID == "" {
			return fmt.Errorf("le scope disque requiert un disque physique Proxmox")
		}
	}

	return nil
}

func (ds *DockerMetricScope) Validate(metric string) error {
	if ds == nil {
		return fmt.Errorf("le scope Docker est requis")
	}

	ds.ScopeMode = strings.TrimSpace(ds.ScopeMode)
	ds.HostID = strings.TrimSpace(ds.HostID)
	ds.ContainerID = strings.TrimSpace(ds.ContainerID)
	ds.ProjectName = strings.TrimSpace(ds.ProjectName)

	if ds.HostID == "" {
		return fmt.Errorf("le scope Docker requiert un hôte")
	}

	validModes := map[string]bool{"host": true, "container": true, "compose_project": true}
	if ds.ScopeMode == "" {
		ds.ScopeMode = "host"
	}
	if !validModes[ds.ScopeMode] {
		return fmt.Errorf("scope Docker invalide: %s", ds.ScopeMode)
	}

	switch metric {
	case "docker_container_state":
		if ds.ScopeMode == "compose_project" {
			return fmt.Errorf("docker_container_state ne supporte pas le scope compose_project")
		}
		if len(ds.WarnStates) == 0 && len(ds.CritStates) == 0 {
			return fmt.Errorf("docker_container_state requiert au moins un état à surveiller")
		}
	case "docker_compose_degraded_services":
		if ds.ScopeMode != "compose_project" {
			return fmt.Errorf("docker_compose_degraded_services requiert le scope compose_project")
		}
	}

	switch ds.ScopeMode {
	case "container":
		if ds.ContainerID == "" {
			return fmt.Errorf("le scope container requiert un container Docker")
		}
		if metric == "docker_compose_degraded_services" {
			return fmt.Errorf("docker_compose_degraded_services ne supporte pas le scope container")
		}
	case "compose_project":
		if ds.ProjectName == "" {
			return fmt.Errorf("le scope compose_project requiert un nom de projet")
		}
	}

	return nil
}

func (ar *AlertRule) NormalizeCompatibility() {
	if ar.SourceType == "" {
		ar.SourceType = InferAlertSourceType(ar.Metric)
	}
}

func (ar *AlertRule) Validate() error {
	ar.NormalizeCompatibility()

	switch ar.SourceType {
	case AlertSourceAgent:
		if ar.HostID == nil || strings.TrimSpace(*ar.HostID) == "" {
			return fmt.Errorf("une alerte agent doit cibler un hote")
		}
		if IsProxmoxMetric(ar.Metric) {
			return fmt.Errorf("la metrique %s est reservee a la source Proxmox", ar.Metric)
		}
		if IsDockerMetric(ar.Metric) {
			return fmt.Errorf("la metrique %s est reservee a la source Docker", ar.Metric)
		}
		ar.ProxmoxScope = nil
		ar.DockerScope = nil
	case AlertSourceProxmox:
		if !IsProxmoxMetric(ar.Metric) {
			return fmt.Errorf("la metrique %s est reservee a la source agent", ar.Metric)
		}
		ar.HostID = nil
		ar.DockerScope = nil
		if err := ar.ProxmoxScope.Validate(ar.Metric); err != nil {
			return err
		}
	case AlertSourceDocker:
		if !IsDockerMetric(ar.Metric) {
			return fmt.Errorf("la metrique %s n'est pas une metrique Docker", ar.Metric)
		}
		ar.HostID = nil
		ar.ProxmoxScope = nil
		if err := ar.DockerScope.Validate(ar.Metric); err != nil {
			return err
		}
	default:
		return fmt.Errorf("source_type invalide")
	}

	return nil
}
