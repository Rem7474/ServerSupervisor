package uptime

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

// fakeRepo is an in-memory Repository for fast, DB-free unit tests.
type fakeRepo struct {
	created   *models.UptimeProbe
	updated   *models.UptimeProbe
	recorded  []models.UptimeProbeResult
	probe     *models.UptimeProbe
	getErr    error
	listProbe []models.UptimeProbe
}

func (f *fakeRepo) ListUptimeProbes(context.Context) ([]models.UptimeProbe, error) {
	return f.listProbe, nil
}
func (f *fakeRepo) GetUptimeProbe(_ context.Context, _ string) (*models.UptimeProbe, error) {
	return f.probe, f.getErr
}
func (f *fakeRepo) CreateUptimeProbe(_ context.Context, p models.UptimeProbe) (*models.UptimeProbe, error) {
	cp := p
	f.created = &cp
	return &cp, nil
}
func (f *fakeRepo) UpdateUptimeProbe(_ context.Context, p models.UptimeProbe) error {
	cp := p
	f.updated = &cp
	return nil
}
func (f *fakeRepo) DeleteUptimeProbe(context.Context, string) error { return nil }
func (f *fakeRepo) GetUptimeProbeResults(context.Context, string, int) ([]models.UptimeProbeResult, error) {
	return nil, nil
}
func (f *fakeRepo) GetUptimeStats(context.Context, string, int) (*models.UptimeStats, error) {
	return &models.UptimeStats{}, nil
}
func (f *fakeRepo) RecordUptimeProbeResult(_ context.Context, r models.UptimeProbeResult) error {
	f.recorded = append(f.recorded, r)
	return nil
}

func boolPtr(b bool) *bool { return &b }

// TestCreateProbe_AppliesDefaults verifies the business rules the service owns:
// interval/timeout floors, the http expected-status default, and pointer overrides.
func TestCreateProbe_AppliesDefaults(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.CreateProbe(context.Background(), models.UptimeProbeRequest{
		Name:   "  api  ",
		Type:   "http",
		Target: "  https://x  ",
		// IntervalSec/TimeoutSec/ExpectedStatus omitted -> defaults
	})
	if err != nil {
		t.Fatalf("CreateProbe: %v", err)
	}
	got := repo.created
	if got == nil {
		t.Fatal("expected a probe to be created")
	}
	if got.Name != "api" || got.Target != "https://x" {
		t.Errorf("expected trimmed name/target, got %q / %q", got.Name, got.Target)
	}
	if got.IntervalSec != 60 {
		t.Errorf("interval < 10 should default to 60, got %d", got.IntervalSec)
	}
	if got.TimeoutSec != 10 {
		t.Errorf("timeout <= 0 should default to 10, got %d", got.TimeoutSec)
	}
	if got.ExpectedStatus != 200 {
		t.Errorf("http expected_status 0 should default to 200, got %d", got.ExpectedStatus)
	}
	if !got.FollowRedirects || !got.VerifyTLS || !got.Enabled {
		t.Errorf("follow_redirects/verify_tls/enabled should default true")
	}
}

func TestCreateProbe_PointerOverrides(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, _ = svc.CreateProbe(context.Background(), models.UptimeProbeRequest{
		Name: "x", Type: "tcp", Target: "x:5432",
		FollowRedirects: boolPtr(false),
		VerifyTLS:       boolPtr(false),
		Enabled:         boolPtr(false),
	})
	got := repo.created
	if got.FollowRedirects || got.VerifyTLS || got.Enabled {
		t.Errorf("explicit false pointers must override the defaults, got %+v", got)
	}
	if got.ExpectedStatus != 0 {
		t.Errorf("tcp probe must not get the http status default, got %d", got.ExpectedStatus)
	}
}

func TestUpdateProbe_SetsID(t *testing.T) {
	repo := &fakeRepo{probe: &models.UptimeProbe{ID: "p1"}}
	svc := NewService(repo)
	if _, err := svc.UpdateProbe(context.Background(), "p1", models.UptimeProbeRequest{Name: "x", Type: "http", Target: "https://x"}); err != nil {
		t.Fatalf("UpdateProbe: %v", err)
	}
	if repo.updated == nil || repo.updated.ID != "p1" {
		t.Errorf("update should carry the path id onto the model, got %+v", repo.updated)
	}
}

// TestCheckNow_RunsAndRecords verifies orchestration with an injected fake runner
// (no real network).
func TestCheckNow_RunsAndRecords(t *testing.T) {
	repo := &fakeRepo{probe: &models.UptimeProbe{ID: "p1", Type: "http"}}
	svc := NewService(repo)
	svc.runOnce = func(_ context.Context, p models.UptimeProbe) models.UptimeProbeResult {
		return models.UptimeProbeResult{ProbeID: p.ID, Success: true, LatencyMs: 12}
	}

	res, err := svc.CheckNow(context.Background(), "p1")
	if err != nil {
		t.Fatalf("CheckNow: %v", err)
	}
	if res == nil || !res.Success || res.LatencyMs != 12 {
		t.Errorf("unexpected result: %+v", res)
	}
	if len(repo.recorded) != 1 || repo.recorded[0].ProbeID != "p1" {
		t.Errorf("CheckNow must persist exactly one result for the probe, got %+v", repo.recorded)
	}
}

func TestListProbes_NeverNil(t *testing.T) {
	svc := NewService(&fakeRepo{listProbe: nil})
	got, err := svc.ListProbes(context.Background())
	if err != nil {
		t.Fatalf("ListProbes: %v", err)
	}
	if got == nil {
		t.Error("ListProbes must return a non-nil slice")
	}
}
