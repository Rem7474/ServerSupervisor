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
type DockerMetricScope struct {
	ScopeMode   string `json:"scope_mode"`
	HostID      string `json:"host_id"`
	ContainerID string `json:"container_id,omitempty"`  // DB UUID of docker_containers row
	ProjectName string `json:"project_name,omitempty"` // compose project name
}

type AlertSourceType string

const (
	AlertSourceAgent   AlertSourceType = "agent"
	AlertSourceProxmox AlertSourceType = "proxmox"
	AlertSourceDocker  AlertSourceType = "docker"
)

// AlertActions holds the consolidated notification configuration for an alert rule.
// Stored as a single JSONB column in the database.
type AlertActions struct {
	Channels       []string        `json:"channels"`                  // e.g. ["smtp", "ntfy", "browser"]
	SMTPTo         string          `json:"smtp_to,omitempty"`         // SMTP recipient address(es)
	NtfyTopic      string          `json:"ntfy_topic,omitempty"`      // ntfy push notification topic
	Cooldown       int             `json:"cooldown,omitempty"`        // seconds between re-notifications (0 = no cooldown)
	CommandTrigger *CommandTrigger `json:"command_trigger,omitempty"` // optional command to run on alert
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

type AlertIncident struct {
	ID          int64      `json:"id" db:"id"`
	RuleID      *int64     `json:"rule_id" db:"rule_id"`
	HostID      string     `json:"host_id" db:"host_id"`
	Severity    string     `json:"severity" db:"severity"` // "warn" or "crit"
	TriggeredAt time.Time  `json:"triggered_at" db:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at" db:"resolved_at"`
	Value       float64    `json:"value" db:"value"`
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
	case "docker_container_not_running", "docker_container_running_count":
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

	switch ds.ScopeMode {
	case "container":
		if ds.ContainerID == "" {
			return fmt.Errorf("le scope container requiert un container Docker")
		}
		if metric == "docker_container_running_count" {
			return fmt.Errorf("la métrique docker_container_running_count ne supporte pas le scope container")
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
