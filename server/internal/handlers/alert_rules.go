package handlers

import (
	"github.com/serversupervisor/server/internal/database"
	alertrulesvc "github.com/serversupervisor/server/internal/services/alertrule"
)

// AlertRulesHandler translates HTTP to the alert-rule service. CRUD,
// validation, capability discovery and the engine-preview test endpoints all
// go through the service. Like the docker/apt/system handlers, it also holds
// db directly — here solely to resolve the caller's hostperm scope
// (resolveAlertHostScope in host_authz.go) for the two hostperm-filtered list
// endpoints, ListAlertRules and ListIncidents (alert_rules_incidents.go).
type AlertRulesHandler struct {
	svc *alertrulesvc.Service
	db  *database.DB
}

func NewAlertRulesHandler(svc *alertrulesvc.Service, db *database.DB) *AlertRulesHandler {
	return &AlertRulesHandler{svc: svc, db: db}
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
