package handlers_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	alertrulesvc "github.com/serversupervisor/server/internal/services/alertrule"
	"github.com/serversupervisor/server/internal/testutil"
)

func newAlertIncidentsRouter(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewAlertRulesHandler(alertrulesvc.NewService(db, func(models.AlertRule) {}, alertrulesvc.EngineFuncs{}), db)

	r := gin.New()
	r.Use(withRole(role))
	r.POST("/alerts/incidents/:id/resolve", h.ResolveIncident)
	r.POST("/alerts/incidents/:id/ack", h.AcknowledgeIncident)
	return r, db
}

// seedAlertIncident creates a minimal rule + open incident on hostID, and
// returns the incident id. A real rule is required — alert_incidents.rule_id
// has an FK to alert_rules(id), so a made-up id would violate it.
func seedAlertIncident(t *testing.T, db *database.DB, hostID string) int64 {
	t.Helper()
	warn := 80.0
	rule := &models.AlertRule{
		SourceType: "agent", HostID: &hostID, Metric: "cpu", Operator: ">",
		ThresholdWarn: &warn, Enabled: true,
		Actions: models.AlertActions{Channels: []string{"browser"}},
	}
	if err := db.CreateAlertRule(context.Background(), rule); err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	incID, err := db.CreateAlertIncident(context.Background(), rule.ID, hostID, 95, "warn")
	if err != nil {
		t.Fatalf("seed incident: %v", err)
	}
	return incID
}

// TestAcknowledgeIncident_RequiresAdmin covers the same admin-only bar as
// ResolveIncident (see AcknowledgeIncident's doc comment for why ack isn't
// scoped to Operator+ like the host-scoped domains).
func TestAcknowledgeIncident_RequiresAdmin(t *testing.T) {
	r, db := newAlertIncidentsRouter(t, "operator")
	seedHost(t, db, "ack-host-1")
	incID := seedAlertIncident(t, db, "ack-host-1")

	w := doJSON(t, r, http.MethodPost, "/alerts/incidents/"+strconv.FormatInt(incID, 10)+"/ack", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin ack = %d, want 403; body = %s", w.Code, w.Body.String())
	}
}

// TestAcknowledgeIncident_Success covers the admin happy path end to end:
// the HTTP call actually persists acknowledged_at/acknowledged_by.
func TestAcknowledgeIncident_Success(t *testing.T) {
	admin, db := newAlertIncidentsRouter(t, "admin")
	seedHost(t, db, "ack-host-2")
	incID := seedAlertIncident(t, db, "ack-host-2")

	w := doJSON(t, admin, http.MethodPost, "/alerts/incidents/"+strconv.FormatInt(incID, 10)+"/ack", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin ack = %d, want 200; body = %s", w.Code, w.Body.String())
	}

	incidents, err := db.GetAlertIncidents(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list incidents: %v", err)
	}
	var found *models.AlertIncident
	for i := range incidents {
		if incidents[i].ID == incID {
			found = &incidents[i]
		}
	}
	if found == nil {
		t.Fatalf("seeded incident %d not found in list", incID)
	}
	if found.AcknowledgedAt == nil {
		t.Fatal("expected acknowledged_at to be set after ack")
	}
	if found.AcknowledgedBy == nil || *found.AcknowledgedBy != "tester" {
		t.Errorf("acknowledged_by = %v, want \"tester\" (see withRole helper)", found.AcknowledgedBy)
	}

	// Acknowledging again is a harmless no-op, not an error.
	w = doJSON(t, admin, http.MethodPost, "/alerts/incidents/"+strconv.FormatInt(incID, 10)+"/ack", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("re-ack = %d, want 200; body = %s", w.Code, w.Body.String())
	}
}

// TestResolveIncident_RequiresAdmin is a light regression check that the
// existing resolve gate wasn't disturbed while wiring the new ack route
// alongside it.
func TestResolveIncident_RequiresAdmin(t *testing.T) {
	r, db := newAlertIncidentsRouter(t, "viewer")
	seedHost(t, db, "ack-host-3")
	incID := seedAlertIncident(t, db, "ack-host-3")
	w := doJSON(t, r, http.MethodPost, "/alerts/incidents/"+strconv.FormatInt(incID, 10)+"/resolve", nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("non-admin resolve = %d, want 403; body = %s", w.Code, w.Body.String())
	}
}
