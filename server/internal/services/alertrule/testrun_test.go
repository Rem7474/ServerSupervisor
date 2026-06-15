package alertrule

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// engineStub records the calls and returns scripted metric values, proving the
// preview is exercisable without a database or the real alerts engine.
func newEngineStub(value float64, hasData, fires bool) EngineFuncs {
	return EngineFuncs{
		MetricValue: func(context.Context, models.Host, models.AlertRule) (float64, bool) {
			return value, hasData
		},
		MatchRule: func(models.AlertRule, models.Host, float64) bool { return fires },
		BuildDockerTargets: func(context.Context, models.AlertRule) []models.Host {
			return []models.Host{{ID: "c1", Name: "nginx"}}
		},
	}
}

func TestRun_AgentSource_EvaluatesEachHost(t *testing.T) {
	repo := &fakeRepo{allHosts: []models.Host{
		{ID: "h1", Name: "alpha"},
		{ID: "h2", Name: "beta"},
	}}
	s := NewService(repo, nil, newEngineStub(91, true, true))

	results, anyFires, err := s.TestRun(context.Background(), TestRunInput{
		Metric:        "cpu",
		Operator:      ">",
		ThresholdWarn: 70,
		ThresholdCrit: 90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2 (one per host)", len(results))
	}
	if !anyFires {
		t.Error("anyFires should be true when a host crosses the threshold")
	}
	if results[0].CurrentValue != 91 || !results[0].WouldFire || !results[0].HasData {
		t.Errorf("unexpected first result: %+v", results[0])
	}
}

func TestRun_AgentSource_FiltersByHostID(t *testing.T) {
	repo := &fakeRepo{allHosts: []models.Host{
		{ID: "h1", Name: "alpha"},
		{ID: "h2", Name: "beta"},
	}}
	s := NewService(repo, nil, newEngineStub(10, true, false))

	target := "h2"
	results, _, err := s.TestRun(context.Background(), TestRunInput{
		SourceType:    models.AlertSourceAgent,
		HostID:        &target,
		Metric:        "cpu",
		Operator:      ">",
		ThresholdWarn: 70,
		ThresholdCrit: 90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].HostID != "h2" {
		t.Fatalf("expected only h2, got %+v", results)
	}
}

func TestRun_InvalidMetricRejected(t *testing.T) {
	s := NewService(&fakeRepo{}, nil, EngineFuncs{})
	_, _, err := s.TestRun(context.Background(), TestRunInput{
		Metric:        "bogus",
		Operator:      ">",
		ThresholdWarn: 1,
		ThresholdCrit: 2,
	})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("want 400 validation, got %v", err)
	}
}

func TestRunLogs_RejectsNonAuthFailureMetric(t *testing.T) {
	s := NewService(&fakeRepo{}, nil, EngineFuncs{})
	_, _, err := s.TestRunLogs(context.Background(), TestRunInput{
		Metric:        "cpu",
		Operator:      ">",
		ThresholdWarn: 1,
		ThresholdCrit: 2,
	})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 400 {
		t.Fatalf("want 400 validation for non-auth-failure metric, got %v", err)
	}
}
