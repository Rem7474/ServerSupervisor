package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func newAlertRulesRouter(t *testing.T) (*gin.Engine, *database.DB) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewAlertRulesHandler(db, cfg)

	r := gin.New()
	r.GET("/alert-rules", h.ListAlertRules)
	r.POST("/alert-rules", h.CreateAlertRule)
	r.GET("/alert-rules/:id", h.GetAlertRule)
	r.DELETE("/alert-rules/:id", h.DeleteAlertRule)
	return r, db
}

// validAgentRulePayload returns a well-formed agent CPU rule body targeting hostID.
func validAgentRulePayload(hostID string) map[string]any {
	return map[string]any{
		"name":           "High CPU",
		"enabled":        true,
		"source_type":    "agent",
		"host_id":        hostID,
		"metric":         "cpu",
		"operator":       ">",
		"threshold_warn": 80,
		"threshold_crit": 90,
		"duration":       0,
		"actions":        map[string]any{"channels": []string{"browser"}},
	}
}

func TestAlertRulesCRUD(t *testing.T) {
	r, db := newAlertRulesRouter(t)

	hostID := "rule-host-1"
	if err := db.RegisterHost(context.Background(), &models.Host{
		ID: hostID, Name: "h", Hostname: "h", Status: "online", LastSeen: time.Now(),
	}); err != nil {
		t.Fatalf("register host: %v", err)
	}

	// Create
	w := doJSON(t, r, http.MethodPost, "/alert-rules", validAgentRulePayload(hostID))
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var created models.AlertRule
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("created rule has no ID")
	}
	idPath := fmt.Sprintf("/alert-rules/%d", created.ID)

	// Get
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", g.Code, g.Body.String())
	}

	// List contains the new rule
	wl := doJSON(t, r, http.MethodGet, "/alert-rules", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	var list []models.AlertRule
	if err := json.Unmarshal(wl.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, rule := range list {
		if rule.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("created rule %d not present in list (%d rules)", created.ID, len(list))
	}

	// Delete
	if d := doJSON(t, r, http.MethodDelete, idPath, nil); d.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", d.Code, d.Body.String())
	}

	// Get after delete -> 404
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}

func TestAlertRulesCreateValidation(t *testing.T) {
	r, _ := newAlertRulesRouter(t)

	cases := []struct {
		name   string
		mutate func(p map[string]any)
	}{
		{"missing name", func(p map[string]any) { delete(p, "name") }},
		{"invalid metric", func(p map[string]any) { p["metric"] = "bogus_metric" }},
		{"invalid operator", func(p map[string]any) { p["operator"] = "!!" }},
		{"invalid channel", func(p map[string]any) {
			p["actions"] = map[string]any{"channels": []string{"pager"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validAgentRulePayload("rule-host-x")
			tc.mutate(p)
			w := doJSON(t, r, http.MethodPost, "/alert-rules", p)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAlertRulesNotFound(t *testing.T) {
	r, _ := newAlertRulesRouter(t)
	if w := doJSON(t, r, http.MethodGet, "/alert-rules/999999", nil); w.Code != http.StatusNotFound {
		t.Errorf("get missing = %d, want 404", w.Code)
	}
	if w := doJSON(t, r, http.MethodDelete, "/alert-rules/999999", nil); w.Code != http.StatusNotFound {
		t.Errorf("delete missing = %d, want 404", w.Code)
	}
}
