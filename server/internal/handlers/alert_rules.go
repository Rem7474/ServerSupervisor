package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type AlertRulesHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewAlertRulesHandler(db *database.DB, cfg *config.Config) *AlertRulesHandler {
	return &AlertRulesHandler{db: db, cfg: cfg}
}

// alertRuleFieldLabel maps Go struct field names to human-readable French labels.
var alertRuleFieldLabel = map[string]string{
	"Name":      "Nom",
	"Metric":    "Metrique",
	"Operator":  "Operateur",
	"Threshold": "Seuil",
	"Duration":  "Duree",
	"Enabled":   "Active",
	"HostID":    "Hote",
}

var validAlertOperators = map[string]bool{">": true, "<": true, ">=": true, "<=": true}

var validAlertChannels = map[string]bool{
	"smtp":    true,
	"ntfy":    true,
	"browser": true,
	"notify":  true,
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
	"docker":  true,
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
	"proxmox_disk_failed_count": true, "proxmox_disk_min_wearout_percent": true,
	"docker_container_not_running": true, "docker_container_running_count": true, "docker_compose_degraded_services": true,
}

type alertMetricCapability struct {
	Metric             string `json:"metric"`
	Label              string `json:"label"`
	Unit               string `json:"unit"`
	Icon               string `json:"icon"`
	BadgeClass         string `json:"badge_class"`
	SupportsThreshold  bool   `json:"supports_threshold"`
	SupportsDuration   bool   `json:"supports_duration"`
	SupportsHostFilter bool   `json:"supports_host_filter"`
}

type alertScopeOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type alertHostCapabilitiesResponse struct {
	HostID   string                  `json:"host_id"`
	HostName string                  `json:"host_name"`
	Metrics  []alertMetricCapability `json:"metrics"`
}

type alertSplitCapabilitiesResponse struct {
	AgentMetrics   []alertMetricCapability `json:"agent_metrics"`
	ProxmoxMetrics []alertMetricCapability `json:"proxmox_metrics"`
	ProxmoxScope   struct {
		Modes       []string           `json:"modes"`
		Connections []alertScopeOption `json:"connections"`
		Nodes       []alertScopeOption `json:"nodes"`
		Storages    []alertScopeOption `json:"storages"`
		Guests      []alertScopeOption `json:"guests"`
		Disks       []alertScopeOption `json:"disks"`
	} `json:"proxmox_scope"`
}

func validateAlertRuleMetricOperator(metric, operator string) error {
	if !validAlertOperators[operator] {
		return errors.New("Operateur invalide.")
	}
	if !validAlertMetrics[metric] {
		return errors.New("Metrique invalide.")
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
		return errors.New("La periode de silence doit etre positive ou nulle.")
	}
	for _, channel := range actions.Channels {
		if !validAlertChannels[channel] {
			return fmt.Errorf("Canal de notification invalide: %s", channel)
		}
	}

	if actions.CommandTrigger != nil {
		ct := actions.CommandTrigger
		ct.Module = strings.TrimSpace(ct.Module)
		ct.Action = strings.TrimSpace(ct.Action)
		ct.Target = strings.TrimSpace(ct.Target)

		if ct.Module == "" || ct.Action == "" {
			return errors.New("Le declencheur de commande doit definir un module et une action.")
		}
		allowedActions, ok := commandModuleActions[ct.Module]
		if !ok {
			return fmt.Errorf("Module de commande invalide: %s", ct.Module)
		}
		if !containsString(allowedActions, ct.Action) {
			return fmt.Errorf("Action invalide pour le module %s: %s", ct.Module, ct.Action)
		}
		if commandModuleRequiresTarget[ct.Module] && ct.Target == "" {
			return fmt.Errorf("Le module %s requiert une cible.", ct.Module)
		}
		if !commandModuleRequiresTarget[ct.Module] {
			ct.Target = ""
		}
	}
	return nil
}

func validateDockerScopeExists(ctx context.Context, db *database.DB, scope *models.DockerMetricScope) error {
	if scope == nil {
		return errors.New("Le scope Docker est requis.")
	}
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM hosts WHERE id = $1)`, scope.HostID).Scan(&exists); err != nil || !exists {
		return errors.New("Hôte introuvable pour ce scope Docker.")
	}
	if scope.ScopeMode == "container" && scope.ContainerID != "" {
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM docker_containers WHERE id = $1 AND host_id = $2)`, scope.ContainerID, scope.HostID).Scan(&exists); err != nil || !exists {
			return errors.New("Container Docker introuvable pour ce scope.")
		}
	}
	if scope.ScopeMode == "compose_project" && scope.ProjectName != "" {
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM compose_projects WHERE name = $1 AND host_id = $2)`, scope.ProjectName, scope.HostID).Scan(&exists); err != nil || !exists {
			return errors.New("Projet Compose introuvable pour ce scope.")
		}
	}
	return nil
}

func validateProxmoxScopeExists(ctx context.Context, db *database.DB, scope *models.ProxmoxMetricScope) error {
	if scope == nil {
		return errors.New("Le scope Proxmox est requis.")
	}

	if scope.ScopeMode == "connection" {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_connections WHERE id = $1)`, scope.ConnectionID).Scan(&exists); err != nil || !exists {
			return errors.New("Connexion Proxmox introuvable pour ce scope.")
		}
	}

	if scope.ScopeMode == "node" {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_nodes WHERE id = $1)`, scope.NodeID).Scan(&exists); err != nil || !exists {
			return errors.New("Noeud Proxmox introuvable pour ce scope.")
		}
	}

	if scope.ScopeMode == "storage" {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_storages WHERE id = $1)`, scope.StorageID).Scan(&exists); err != nil || !exists {
			return errors.New("Stockage Proxmox introuvable pour ce scope.")
		}
	}

	if scope.ScopeMode == "guest" {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_guests WHERE id = $1)`, scope.GuestID).Scan(&exists); err != nil || !exists {
			return errors.New("VM/LXC Proxmox introuvable pour ce scope.")
		}
	}

	if scope.ScopeMode == "disk" {
		var exists bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM proxmox_disks WHERE id = $1)`, scope.DiskID).Scan(&exists); err != nil || !exists {
			return errors.New("Disque physique Proxmox introuvable pour ce scope.")
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
