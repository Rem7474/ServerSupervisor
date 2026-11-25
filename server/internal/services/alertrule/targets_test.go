package alertrule

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

func ptr[T any](v T) *T { return &v }

func cpuCreate(target string) models.AlertRuleCreate {
	return models.AlertRuleCreate{
		Name: "cpu", Metric: "cpu", Operator: ">", ThresholdWarn: 70, ThresholdCrit: 90,
		SourceType: models.AlertSourceAgent, TargetType: target,
	}
}

func TestCreate_GroupTarget(t *testing.T) {
	t.Run("existing group is stored", func(t *testing.T) {
		repo := &fakeRepo{hostGroups: []models.HostGroup{{ID: "g1"}}}
		req := cpuCreate(models.AlertTargetGroup)
		req.HostGroupID = ptr("g1")
		req.HostID = ptr("stale-host")
		rule, err := newSvc(repo).Create(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if rule.TargetType != models.AlertTargetGroup || *rule.HostGroupID != "g1" || rule.HostID != nil {
			t.Errorf("rule target = %q group=%v host=%v", rule.TargetType, rule.HostGroupID, rule.HostID)
		}
	})
	t.Run("unknown group is a validation error", func(t *testing.T) {
		repo := &fakeRepo{}
		req := cpuCreate(models.AlertTargetGroup)
		req.HostGroupID = ptr("missing")
		_, err := newSvc(repo).Create(context.Background(), req)
		if i18nKey(err) != apperr.CodeAlertHostGroupNotFound {
			t.Fatalf("err = %v", err)
		}
		if repo.created != nil {
			t.Error("rule persisted with an unknown group")
		}
	})
	t.Run("group lookup failure is not reported as missing", func(t *testing.T) {
		boom := errors.New("db down")
		req := cpuCreate(models.AlertTargetGroup)
		req.HostGroupID = ptr("g1")
		_, err := newSvc(&fakeRepo{hostGroupErr: boom}).Create(context.Background(), req)
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want the repo error", err)
		}
	})
}

func TestCreate_TagAndAllTargets(t *testing.T) {
	repo := &fakeRepo{}
	req := cpuCreate(models.AlertTargetTag)
	req.TargetTags = []string{" db ", "DB", "web"}
	rule, err := newSvc(repo).Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rule.TargetTags, []string{"db", "web"}) {
		t.Errorf("tags = %v", rule.TargetTags)
	}

	// "All hosts" is a valid agent target: no host_id at all.
	rule, err = newSvc(&fakeRepo{}).Create(context.Background(), cpuCreate(""))
	if err != nil {
		t.Fatal(err)
	}
	if rule.TargetType != models.AlertTargetAll {
		t.Errorf("target without host_id = %q, want all", rule.TargetType)
	}

	_, err = newSvc(&fakeRepo{}).Create(context.Background(), cpuCreate(models.AlertTargetTag))
	if status(err) != 400 {
		t.Errorf("tag target without tags should be 400, got %v", err)
	}
}

func TestUpdate_Retargeting(t *testing.T) {
	existing := func() *models.AlertRule {
		return &models.AlertRule{
			ID: 1, SourceType: models.AlertSourceAgent, TargetType: models.AlertTargetHost, HostID: ptr("h1"),
			Metric: "cpu", Operator: ">", ThresholdWarn: ptr(70.0), ThresholdCrit: ptr(90.0),
		}
	}

	t.Run("target_type replaces the whole target", func(t *testing.T) {
		repo := &fakeRepo{rule: existing(), hostGroups: []models.HostGroup{{ID: "g1"}}}
		err := newSvc(repo).Update(context.Background(), 1, models.AlertRuleUpdate{
			TargetType: ptr(models.AlertTargetGroup), HostGroupID: ptr("g1"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if repo.updated.TargetType != models.AlertTargetGroup || repo.updated.HostID != nil || *repo.updated.HostGroupID != "g1" {
			t.Errorf("updated = %q host=%v group=%v", repo.updated.TargetType, repo.updated.HostID, repo.updated.HostGroupID)
		}
	})
	t.Run("host_id alone retargets a group rule to that host", func(t *testing.T) {
		rule := existing()
		rule.TargetType, rule.HostID, rule.HostGroupID = models.AlertTargetGroup, nil, ptr("g1")
		repo := &fakeRepo{rule: rule}
		if err := newSvc(repo).Update(context.Background(), 1, models.AlertRuleUpdate{HostID: ptr("h2")}); err != nil {
			t.Fatal(err)
		}
		if repo.updated.TargetType != models.AlertTargetHost || *repo.updated.HostID != "h2" || repo.updated.HostGroupID != nil {
			t.Errorf("updated = %q host=%v group=%v", repo.updated.TargetType, repo.updated.HostID, repo.updated.HostGroupID)
		}
	})
	t.Run("no target field keeps the target", func(t *testing.T) {
		repo := &fakeRepo{rule: existing()}
		if err := newSvc(repo).Update(context.Background(), 1, models.AlertRuleUpdate{Name: ptr("renamed")}); err != nil {
			t.Fatal(err)
		}
		if repo.updated.TargetType != models.AlertTargetHost || *repo.updated.HostID != "h1" {
			t.Errorf("target changed: %q %v", repo.updated.TargetType, repo.updated.HostID)
		}
	})
	t.Run("unknown group rejected", func(t *testing.T) {
		repo := &fakeRepo{rule: existing()}
		err := newSvc(repo).Update(context.Background(), 1, models.AlertRuleUpdate{
			TargetType: ptr(models.AlertTargetGroup), HostGroupID: ptr("missing"),
		})
		if i18nKey(err) != apperr.CodeAlertHostGroupNotFound || repo.updated != nil {
			t.Fatalf("err = %v updated=%v", err, repo.updated)
		}
	})
}

func TestRun_GroupAndTagTargets(t *testing.T) {
	hosts := []models.Host{
		{ID: "h1", Name: "alpha", Tags: []string{"db"}},
		{ID: "h2", Name: "beta", Tags: []string{"web"}},
		{ID: "h3", Name: "gamma", Tags: []string{"db", "web"}},
	}
	repo := &fakeRepo{
		allHosts:   hosts,
		hostGroups: []models.HostGroup{{ID: "g-web", MatchMode: models.HostGroupMatchAll, Tags: []string{"web"}}},
	}
	s := NewService(repo, nil, newEngineStub(10, true, false))
	ids := func(results []TestRunResult) []string {
		out := []string{}
		for _, r := range results {
			out = append(out, r.HostID)
		}
		return out
	}

	results, _, err := s.TestRun(context.Background(), TestRunInput{
		Metric: "cpu", Operator: ">", ThresholdWarn: 70, ThresholdCrit: 90,
		TargetType: models.AlertTargetGroup, HostGroupID: ptr("g-web"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(results); !reflect.DeepEqual(got, []string{"h2", "h3"}) {
		t.Errorf("group preview = %v", got)
	}

	results, _, err = s.TestRun(context.Background(), TestRunInput{
		Metric: "cpu", Operator: ">", ThresholdWarn: 70, ThresholdCrit: 90,
		TargetType: models.AlertTargetTag, TargetTags: []string{"db"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(results); !reflect.DeepEqual(got, []string{"h1", "h3"}) {
		t.Errorf("tag preview = %v", got)
	}

	_, _, err = s.TestRun(context.Background(), TestRunInput{
		Metric: "cpu", Operator: ">", ThresholdWarn: 70, ThresholdCrit: 90,
		TargetType: models.AlertTargetGroup, HostGroupID: ptr("missing"),
	})
	if i18nKey(err) != apperr.CodeAlertHostGroupNotFound {
		t.Errorf("missing group preview err = %v", err)
	}
}

// The rule form sends source_type "synthetic" for the uptime/SSL metrics.
func TestCreate_SyntheticSource(t *testing.T) {
	repo := &fakeRepo{}
	rule, err := newSvc(repo).Create(context.Background(), models.AlertRuleCreate{
		Name: "uptime", Metric: "uptime_down_count", Operator: ">", ThresholdWarn: 0, ThresholdCrit: 1,
		SourceType: models.AlertSourceSynthetic, TargetType: models.AlertTargetAll,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rule.SourceType != models.AlertSourceSynthetic || rule.TargetType != "" || repo.created == nil {
		t.Errorf("rule = source %q target %q persisted=%v", rule.SourceType, rule.TargetType, repo.created != nil)
	}

	_, err = newSvc(&fakeRepo{}).Create(context.Background(), models.AlertRuleCreate{
		Name: "x", Metric: "cpu", Operator: ">", ThresholdWarn: 70, ThresholdCrit: 90,
		SourceType: models.AlertSourceSynthetic,
	})
	if i18nKey(err) != apperr.CodeAlertSyntheticMetricRequired || status(err) != 400 {
		t.Fatalf("synthetic source with an agent metric: err = %v", err)
	}
}

// Catalogued model errors keep their i18n code through the service.
func TestCreate_TargetErrorKeepsI18nCode(t *testing.T) {
	_, err := newSvc(&fakeRepo{}).Create(context.Background(), cpuCreate(models.AlertTargetGroup))
	if i18nKey(err) != apperr.CodeAlertTargetGroupRequired || status(err) != 400 {
		t.Fatalf("err = %v", err)
	}
	repo := &fakeRepo{rule: &models.AlertRule{
		ID: 1, SourceType: models.AlertSourceAgent, TargetType: models.AlertTargetAll,
		Metric: "cpu", Operator: ">", ThresholdWarn: ptr(70.0), ThresholdCrit: ptr(90.0),
	}}
	err = newSvc(repo).Update(context.Background(), 1, models.AlertRuleUpdate{TargetType: ptr("everywhere")})
	if i18nKey(err) != apperr.CodeAlertTargetTypeInvalid {
		t.Fatalf("update err = %v", err)
	}
}

func TestRun_SyntheticSourceEvaluatesOneTarget(t *testing.T) {
	repo := &fakeRepo{allHosts: []models.Host{{ID: "h1"}, {ID: "h2"}}}
	results, _, err := NewService(repo, nil, newEngineStub(2, true, false)).TestRun(context.Background(), TestRunInput{
		SourceType: models.AlertSourceSynthetic, Metric: "uptime_down_count", Operator: ">", ThresholdWarn: 0, ThresholdCrit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].HostID != "synthetic:uptime_down_count" {
		t.Fatalf("results = %+v", results)
	}
	_, _, err = NewService(repo, nil, newEngineStub(2, true, false)).TestRun(context.Background(), TestRunInput{
		Metric: "cpu", Operator: ">", ThresholdWarn: 70, ThresholdCrit: 90, TargetType: models.AlertTargetTag,
	})
	if i18nKey(err) != apperr.CodeAlertTargetTagsRequired {
		t.Fatalf("preview err = %v", err)
	}
}
