package handlers

import (
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	alertrulesvc "github.com/serversupervisor/server/internal/services/alertrule"
)

// AlertRulesHandler translates HTTP to the alert-rule service. The rule CRUD +
// incidents go through the service; the capability endpoints still read directly
// from db (db is kept for that until they migrate too).
type AlertRulesHandler struct {
	svc *alertrulesvc.Service
	db  *database.DB
	cfg *config.Config
}

func NewAlertRulesHandler(svc *alertrulesvc.Service, db *database.DB, cfg *config.Config) *AlertRulesHandler {
	return &AlertRulesHandler{svc: svc, db: db, cfg: cfg}
}

// alertRuleFieldLabel maps Go struct field names to human-readable French labels
// for binding-error messages (see humanizeValidationError).
var alertRuleFieldLabel = map[string]string{
	"Name":      "Nom",
	"Metric":    "Metrique",
	"Operator":  "Operateur",
	"Threshold": "Seuil",
	"Duration":  "Duree",
	"Enabled":   "Active",
	"HostID":    "Hote",
}

// ===== capability response shapes (consumed by alert_rules_capabilities.go) =====

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
