package weblogs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/threatdetect"
)

type fakeRepo struct{}

func (fakeRepo) GetWebLogsSummary(context.Context, time.Time, time.Time, string, string) (map[string]any, error) {
	return nil, nil
}
func (fakeRepo) GetWebLogsThreats(context.Context, time.Time, time.Time, string, string) (map[string]any, error) {
	return nil, nil
}
func (fakeRepo) GetWebLogsTopClientIPs(context.Context, time.Time, time.Time, string, string, int) ([]map[string]any, error) {
	return nil, nil
}
func (fakeRepo) GetWebLogsKPIWindow(context.Context, time.Time, time.Time, string, string) (map[string]any, error) {
	return nil, nil
}
func (fakeRepo) GetIPTimeline(context.Context, string, time.Time, time.Time, string, int) ([]models.WebLogIPTimelineRow, error) {
	return nil, nil
}
func (fakeRepo) GetDomainDetails(context.Context, string, time.Time, time.Time, string, string, database.DomainDetailsFilter, int, int) (map[string]any, error) {
	return nil, nil
}
func (fakeRepo) GetWebLogsTimeseries(context.Context, time.Time, time.Time, string, string, string) ([]map[string]any, error) {
	return nil, nil
}
func (fakeRepo) GetWebLogsLive(context.Context, string, string, int) ([]map[string]any, error) {
	return nil, nil
}

type fakeDispatcher struct{ called bool }

func (f *fakeDispatcher) Create(context.Context, dispatch.Request) (*dispatch.Result, error) {
	f.called = true
	return &dispatch.Result{Command: &models.RemoteCommand{ID: "cmd"}}, nil
}

func TestBlockIP_Validation(t *testing.T) {
	disp := &fakeDispatcher{}
	svc := NewService(fakeRepo{}, disp, nil)

	if _, err := svc.BlockIP(context.Background(), "h1", "not-an-ip", "4h", "u", "c"); !isValidationErr(err) {
		t.Errorf("invalid IP should be apperr 400, got %v", err)
	}
	if _, err := svc.BlockIP(context.Background(), "h1", "1.2.3.4", "banana", "u", "c"); !isValidationErr(err) {
		t.Errorf("invalid duration should be apperr 400, got %v", err)
	}
	if disp.called {
		t.Error("must not dispatch when validation fails")
	}
	id, err := svc.BlockIP(context.Background(), "h1", "1.2.3.4", "4h", "u", "c")
	if err != nil || id != "cmd" || !disp.called {
		t.Errorf("valid ban should dispatch: id=%q err=%v", id, err)
	}
}

func TestUnblockIP_InvalidIP(t *testing.T) {
	if _, err := NewService(fakeRepo{}, &fakeDispatcher{}, nil).UnblockIP(context.Background(), "h1", "bad", "u", "c"); !isValidationErr(err) {
		t.Errorf("invalid IP should be apperr 400, got %v", err)
	}
}

func TestDeltaPercent(t *testing.T) {
	if v := deltaPercent(0, 0); v != float64(0) {
		t.Errorf("0/0 delta = %v, want 0", v)
	}
	if v := deltaPercent(5, 0); v != nil {
		t.Errorf("growth from zero = %v, want nil", v)
	}
	if v := deltaPercent(150, 100); v != float64(50) {
		t.Errorf("100->150 delta = %v, want 50", v)
	}
}

func TestPromoteBlockedIntoThreats(t *testing.T) {
	summary := map[string]any{
		"traffic": map[string]any{"blocked_ips": int64(3), "blocked_requests": int64(9)},
		"threats": map[string]any{"crowdsec_blocked_ips": int64(7)},
	}
	threats := summary["threats"]
	promoteBlockedIntoThreats(summary, threats)
	tm := threats.(map[string]any)
	if tm["blocked_requests"] != int64(9) {
		t.Errorf("blocked_requests not promoted: %v", tm["blocked_requests"])
	}
	// crowdsec count (7) > web count (3) -> blocked_ips promoted to 7
	if tm["blocked_ips"] != int64(7) {
		t.Errorf("blocked_ips = %v, want 7 (crowdsec wins)", tm["blocked_ips"])
	}
}

// TestApplyThreatScoring_RanksBySeverityNotHits guards the reordering this
// method exists to do: a low-hit IP whose requests mostly 5xx must outrank a
// high-hit IP whose requests are all 2xx, even though the repository's
// pre-filter query orders candidates by raw hits.
func TestApplyThreatScoring_RanksBySeverityNotHits(t *testing.T) {
	svc := NewService(fakeRepo{}, &fakeDispatcher{}, nil)
	threats := map[string]any{
		"top_ips": []map[string]any{
			{
				"ip": "1.1.1.1", "hits": int64(5000), "unique_paths": int64(1),
				"_category_counts": threatdetect.CategoryCounts{WordPress: 5000},
				"_status_counts":   threatdetect.StatusCounts{Status2xx: 5000},
			},
			{
				"ip": "2.2.2.2", "hits": int64(3), "unique_paths": int64(1),
				"_category_counts": threatdetect.CategoryCounts{PathTraversal: 3},
				"_status_counts":   threatdetect.StatusCounts{Status5xx: 3},
			},
		},
	}
	svc.applyThreatScoring(threats)

	topIPs, ok := threats["top_ips"].([]map[string]any)
	if !ok || len(topIPs) != 2 {
		t.Fatalf("expected 2 top_ips entries, got %+v", threats["top_ips"])
	}
	if topIPs[0]["ip"] != "2.2.2.2" {
		t.Errorf("expected the low-hit/high-severity IP ranked first, got %+v", topIPs)
	}
	if _, ok := topIPs[0]["_category_counts"]; ok {
		t.Error("_category_counts should be stripped from the response")
	}
	if _, ok := topIPs[0]["_status_counts"]; ok {
		t.Error("_status_counts should be stripped from the response")
	}
	if _, ok := topIPs[0]["level"].(string); !ok {
		t.Error("expected a level string to be set")
	}
}

func TestCountryDistribution_PrivateIPsSortedNoHTTP(t *testing.T) {
	// Private IPs short-circuit geolocation (no external call), so this stays offline.
	top := []map[string]any{
		{"ip": "10.0.0.1", "hits": int64(5)},
		{"ip": "192.168.1.1", "hits": int64(2)},
	}
	dist := countryDistribution(top)
	if len(dist) != 1 || dist[0]["country"] != "Local / Private" {
		t.Fatalf("expected one Local/Private bucket, got %v", dist)
	}
	if anyToInt64(dist[0]["hits"]) != 7 {
		t.Errorf("hits aggregated = %v, want 7", dist[0]["hits"])
	}
}

func isValidationErr(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.HTTPStatus == 400
}
