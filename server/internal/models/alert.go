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
}

type AlertSourceType string

const (
	AlertSourceAgent   AlertSourceType = "agent"
	AlertSourceProxmox AlertSourceType = "proxmox"
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
	ID              int64               `json:"id" db:"id"`
	Name            *string             `json:"name,omitempty" db:"name"`
	SourceType      AlertSourceType     `json:"source_type,omitempty" db:"source_type"`
	HostID          *string             `json:"host_id" db:"host_id"`
	ProxmoxScope    *ProxmoxMetricScope `json:"proxmox_scope,omitempty" db:"proxmox_scope"`
	Metric          string              `json:"metric" db:"metric"`
	Operator        string              `json:"operator" db:"operator"`
	Threshold       *float64            `json:"threshold" db:"threshold"`
	DurationSeconds int                 `json:"duration_seconds" db:"duration_seconds"`
	Actions         AlertActions        `json:"actions" db:"-"` // stored as JSONB in DB
	LastFired       *time.Time          `json:"last_fired,omitempty" db:"last_fired"`
	Enabled         bool                `json:"enabled" db:"enabled"`
	CreatedAt       time.Time           `json:"created_at" db:"created_at"`
	UpdatedAt       *time.Time          `json:"updated_at,omitempty" db:"updated_at"`
}

type AlertIncident struct {
	ID          int64      `json:"id" db:"id"`
	RuleID      *int64     `json:"rule_id" db:"rule_id"`
	HostID      string     `json:"host_id" db:"host_id"`
	TriggeredAt time.Time  `json:"triggered_at" db:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at" db:"resolved_at"`
	Value       float64    `json:"value" db:"value"`
}

type NotificationItem struct {
	ID            int64      `json:"id"`
	RuleID        *int64     `json:"rule_id"`
	HostID        string     `json:"host_id"`
	HostName      string     `json:"host_name"`
	RuleName      string     `json:"rule_name"`
	Metric        string     `json:"metric"`
	Value         float64    `json:"value"`
	TriggeredAt   time.Time  `json:"triggered_at"`
	ResolvedAt    *time.Time `json:"resolved_at"`
	BrowserNotify bool       `json:"browser_notify"`
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
	Name         string              `json:"name" binding:"required"`
	Enabled      bool                `json:"enabled"`
	SourceType   AlertSourceType     `json:"source_type"`
	HostID       *string             `json:"host_id"`
	ProxmoxScope *ProxmoxMetricScope `json:"proxmox_scope"`
	Metric       string              `json:"metric" binding:"required"`
	Operator     string              `json:"operator" binding:"required"`
	Threshold    float64             `json:"threshold" binding:"required"`
	Duration     int                 `json:"duration"`
	Actions      AlertActions        `json:"actions"`
}

type AlertRuleUpdate struct {
	Name         *string             `json:"name"`
	Enabled      *bool               `json:"enabled"`
	SourceType   *AlertSourceType    `json:"source_type"`
	HostID       *string             `json:"host_id"`
	ProxmoxScope *ProxmoxMetricScope `json:"proxmox_scope"`
	Metric       *string             `json:"metric"`
	Operator     *string             `json:"operator"`
	Threshold    *float64            `json:"threshold"`
	Duration     *int                `json:"duration"`
	Actions      *AlertActions       `json:"actions"`
}

func IsProxmoxMetric(metric string) bool {
	switch metric {
	case "proxmox_storage_percent", "proxmox_node_cpu_percent", "proxmox_node_memory_percent",
		"proxmox_guest_cpu_percent", "proxmox_guest_memory_percent",
		"proxmox_node_pending_updates", "proxmox_node_security_updates",
		"proxmox_recent_failed_tasks_24h",
		"proxmox_disk_failed_count", "proxmox_disk_min_wearout_percent":
		return true
	default:
		return false
	}
}

func InferAlertSourceType(metric string) AlertSourceType {
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

	validModes := map[string]bool{"global": true, "connection": true, "node": true, "storage": true}
	if !validModes[ps.ScopeMode] {
		return fmt.Errorf("scope Proxmox invalide")
	}

	ps.ConnectionID = strings.TrimSpace(ps.ConnectionID)
	ps.NodeID = strings.TrimSpace(ps.NodeID)
	ps.StorageID = strings.TrimSpace(ps.StorageID)

	switch ps.ScopeMode {
	case "connection":
		if ps.ConnectionID == "" {
			return fmt.Errorf("le scope connexion requiert une connexion Proxmox")
		}
	case "node":
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
		ar.ProxmoxScope = nil
	case AlertSourceProxmox:
		if !IsProxmoxMetric(ar.Metric) {
			return fmt.Errorf("la metrique %s est reservee a la source agent", ar.Metric)
		}
		ar.HostID = nil
		if err := ar.ProxmoxScope.Validate(ar.Metric); err != nil {
			return err
		}
	default:
		return fmt.Errorf("source_type invalide")
	}

	return nil
}
