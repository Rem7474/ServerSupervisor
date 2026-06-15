package handlers

import (
	alertrulesvc "github.com/serversupervisor/server/internal/services/alertrule"
)

// AlertRulesHandler translates HTTP to the alert-rule service. CRUD, incidents,
// validation, capability discovery and the engine-preview test endpoints all go
// through the service — the handler holds no database reference.
type AlertRulesHandler struct {
	svc *alertrulesvc.Service
}

func NewAlertRulesHandler(svc *alertrulesvc.Service) *AlertRulesHandler {
	return &AlertRulesHandler{svc: svc}
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
