package handlers

import (
	"github.com/serversupervisor/server/internal/database"
	alertrulesvc "github.com/serversupervisor/server/internal/services/alertrule"
)

// AlertRulesHandler translates HTTP to the alert-rule service. CRUD, incidents,
// validation and capability discovery all go through the service; db is retained
// only for the engine-preview test endpoints (_testrun.go) which drive the alerts
// engine over the concrete *DB.
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
