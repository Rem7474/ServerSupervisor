package alertrule

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

func validTemplateReq() models.AlertRuleTemplateRequest {
	return models.AlertRuleTemplateRequest{
		Name: "High CPU", Metric: "cpu", Operator: ">",
		ThresholdWarn: 70, ThresholdCrit: 90,
		Actions: models.AlertActions{Channels: []string{"browser"}},
	}
}

func TestIsTemplatableMetric(t *testing.T) {
	cases := []struct {
		metric string
		want   bool
	}{
		{"cpu", true},
		{"memory", true},
		{"status_offline", true},
		{"docker_container_state", false},
		{"proxmox_node_cpu_percent", false},
		{"uptime_down_count", false},
		{"ssl_min_days_remaining", false},
	}
	for _, c := range cases {
		if got := isTemplatableMetric(c.metric); got != c.want {
			t.Errorf("isTemplatableMetric(%q) = %v, want %v", c.metric, got, c.want)
		}
	}
}

func TestCreateTemplate_RejectsNonTemplatableMetric(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo)
	req := validTemplateReq()
	req.Metric = "docker_container_state"
	_, err := svc.CreateTemplate(context.Background(), req)
	if status(err) != 400 {
		t.Fatalf("docker metric should be rejected with 400, got %v", err)
	}
}

func TestCreateTemplate_RejectsBadOperator(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo)
	req := validTemplateReq()
	req.Operator = "!!"
	_, err := svc.CreateTemplate(context.Background(), req)
	if status(err) != 400 {
		t.Fatalf("bad operator should be rejected with 400, got %v", err)
	}
}

func TestCreateTemplate_Success(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo)
	tmpl, err := svc.CreateTemplate(context.Background(), validTemplateReq())
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if tmpl.ID == 0 {
		t.Error("expected the created template to have an id")
	}
}

func TestApplyTemplate_CreatesOneRulePerHost(t *testing.T) {
	repo := &fakeRepo{
		template: &models.AlertRuleTemplate{
			ID: 1, Name: "High CPU", Metric: "cpu", Operator: ">",
			ThresholdWarn: 70, ThresholdCrit: 90,
			Actions: models.AlertActions{Channels: []string{"browser"}},
		},
	}
	svc := newSvc(repo)
	result, err := svc.ApplyTemplate(context.Background(), 1, models.ApplyAlertRuleTemplateRequest{
		HostIDs: []string{"host-a", "host-b"},
	})
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if len(result.CreatedRuleIDs) != 2 {
		t.Fatalf("expected 2 created rules, got %d (errors: %+v)", len(result.CreatedRuleIDs), result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %+v", result.Errors)
	}
	// The fakeRepo's CreateAlertRule stub only records the *last* call, but we
	// can still assert the last-applied rule carries the template's recipe
	// and the target host, not the template's own (nonexistent) host.
	if repo.created == nil || repo.created.HostID == nil {
		t.Fatal("expected the last created rule to have a host_id")
	}
	if repo.created.Metric != "cpu" || repo.created.Operator != ">" {
		t.Errorf("created rule = %+v, want the template's metric/operator", repo.created)
	}
}

func TestApplyTemplate_NotFound(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo)
	_, err := svc.ApplyTemplate(context.Background(), 999, models.ApplyAlertRuleTemplateRequest{HostIDs: []string{"host-a"}})
	if status(err) != 404 {
		t.Fatalf("applying a missing template should be 404, got %v", err)
	}
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo)
	if status(svc.DeleteTemplate(context.Background(), 999)) != 404 {
		t.Error("deleting a missing template should be 404")
	}
}

func TestListTemplates_NeverNil(t *testing.T) {
	svc := newSvc(&fakeRepo{})
	templates, err := svc.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if templates == nil {
		t.Error("ListTemplates should return an empty slice, not nil")
	}
}
