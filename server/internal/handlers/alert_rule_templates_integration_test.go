package handlers_test

import (
	"encoding/json"
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

func newAlertRuleTemplatesRouter(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, _ := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewAlertRulesHandler(alertrulesvc.NewService(db, func(models.AlertRule) {}, alertrulesvc.EngineFuncs{}), db)

	r := gin.New()
	r.Use(withRole(role))
	r.GET("/alert-rule-templates", h.ListAlertRuleTemplates)
	r.GET("/alert-rule-templates/:id", h.GetAlertRuleTemplate)
	r.POST("/alert-rule-templates", h.CreateAlertRuleTemplate)
	r.PATCH("/alert-rule-templates/:id", h.UpdateAlertRuleTemplate)
	r.DELETE("/alert-rule-templates/:id", h.DeleteAlertRuleTemplate)
	r.POST("/alert-rule-templates/:id/apply", h.ApplyAlertRuleTemplate)
	return r, db
}

func validTemplatePayload() map[string]any {
	return map[string]any{
		"name":           "High CPU",
		"metric":         "cpu",
		"operator":       ">",
		"threshold_warn": 70,
		"threshold_crit": 90,
		"actions":        map[string]any{"channels": []string{"browser"}},
	}
}

func TestAlertRuleTemplatesCRUD(t *testing.T) {
	admin, db := newAlertRuleTemplatesRouter(t, "admin")

	w := doJSON(t, admin, http.MethodPost, "/alert-rule-templates", validTemplatePayload())
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	id, _ := created["id"].(float64)
	if id == 0 {
		t.Fatalf("created template has no id: %s", w.Body.String())
	}
	idPath := "/alert-rule-templates/" + strconv.FormatFloat(id, 'f', 0, 64)

	// List includes it.
	wl := doJSON(t, admin, http.MethodGet, "/alert-rule-templates", nil)
	if wl.Code != http.StatusOK {
		t.Fatalf("list status = %d", wl.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(wl.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 template, got %d", len(list))
	}

	if g := doJSON(t, admin, http.MethodGet, idPath, nil); g.Code != http.StatusOK {
		t.Fatalf("get status = %d", g.Code)
	}

	// Update (rename + raise thresholds).
	upd := validTemplatePayload()
	upd["name"] = "High CPU (renamed)"
	upd["threshold_warn"] = 75
	if u := doJSON(t, admin, http.MethodPatch, idPath, upd); u.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", u.Code, u.Body.String())
	}

	// Apply to two hosts creates two independent rules.
	seedHost(t, db, "tmpl-host-1")
	seedHost(t, db, "tmpl-host-2")
	applyBody := map[string]any{"host_ids": []string{"tmpl-host-1", "tmpl-host-2"}, "enabled": true}
	ar := doJSON(t, admin, http.MethodPost, idPath+"/apply", applyBody)
	if ar.Code != http.StatusOK {
		t.Fatalf("apply status = %d, body = %s", ar.Code, ar.Body.String())
	}
	var applyResult struct {
		CreatedRuleIDs []int64           `json:"created_rule_ids"`
		Errors         map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(ar.Body.Bytes(), &applyResult); err != nil {
		t.Fatalf("decode apply result: %v", err)
	}
	if len(applyResult.CreatedRuleIDs) != 2 {
		t.Fatalf("expected 2 created rules, got %+v (errors: %+v)", applyResult.CreatedRuleIDs, applyResult.Errors)
	}

	if d := doJSON(t, admin, http.MethodDelete, idPath, nil); d.Code != http.StatusOK {
		t.Fatalf("delete status = %d", d.Code)
	}
	if g := doJSON(t, admin, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}

func TestAlertRuleTemplates_WriteRequiresAdmin(t *testing.T) {
	r, _ := newAlertRuleTemplatesRouter(t, "operator")

	if w := doJSON(t, r, http.MethodPost, "/alert-rule-templates", validTemplatePayload()); w.Code != http.StatusForbidden {
		t.Errorf("non-admin create = %d, want 403", w.Code)
	}
	if w := doJSON(t, r, http.MethodPatch, "/alert-rule-templates/1", validTemplatePayload()); w.Code != http.StatusForbidden {
		t.Errorf("non-admin update = %d, want 403", w.Code)
	}
	if w := doJSON(t, r, http.MethodDelete, "/alert-rule-templates/1", nil); w.Code != http.StatusForbidden {
		t.Errorf("non-admin delete = %d, want 403", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/alert-rule-templates/1/apply", map[string]any{"host_ids": []string{"h1"}}); w.Code != http.StatusForbidden {
		t.Errorf("non-admin apply = %d, want 403", w.Code)
	}
}

func TestAlertRuleTemplates_RejectsNonTemplatableMetric(t *testing.T) {
	admin, _ := newAlertRuleTemplatesRouter(t, "admin")
	bad := validTemplatePayload()
	bad["metric"] = "docker_container_state"
	w := doJSON(t, admin, http.MethodPost, "/alert-rule-templates", bad)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("docker metric template = %d, want 400; body = %s", w.Code, w.Body.String())
	}
}

func TestApplyAlertRuleTemplate_PartialFailureIsNotAllOrNothing(t *testing.T) {
	admin, db := newAlertRuleTemplatesRouter(t, "admin")
	w := doJSON(t, admin, http.MethodPost, "/alert-rule-templates", validTemplatePayload())
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id, _ := created["id"].(float64)
	idPath := "/alert-rule-templates/" + strconv.FormatFloat(id, 'f', 0, 64)

	seedHost(t, db, "tmpl-host-ok")
	// "tmpl-host-missing" is never registered — Create() doesn't validate host
	// existence for agent-sourced rules today (same as a single manual rule
	// creation), so this actually still succeeds; this test locks in that
	// applying to N hosts always returns a per-host result shape rather than
	// erroring the whole call, whatever the per-host outcome turns out to be.
	applyBody := map[string]any{"host_ids": []string{"tmpl-host-ok", "tmpl-host-missing"}}
	ar := doJSON(t, admin, http.MethodPost, idPath+"/apply", applyBody)
	if ar.Code != http.StatusOK {
		t.Fatalf("apply status = %d, body = %s", ar.Code, ar.Body.String())
	}
	var result struct {
		CreatedRuleIDs []int64           `json:"created_rule_ids"`
		Errors         map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(ar.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.CreatedRuleIDs) != 2 {
		t.Fatalf("expected 2 created rules (host existence isn't validated), got %+v errors=%+v", result.CreatedRuleIDs, result.Errors)
	}
}
