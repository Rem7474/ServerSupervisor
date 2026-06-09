// Package uptime is the application/service layer for synthetic uptime probes.
// It owns the probe business logic (validation defaults, check-now orchestration)
// behind a Repository port, so the logic is unit-testable without a database and
// the HTTP handler is reduced to request/response translation.
package uptime

import (
	"context"
	"strings"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/synthetic"
)

// Repository is the data-access port the service depends on. *database.DB
// satisfies it structurally; tests provide an in-memory fake.
type Repository interface {
	ListUptimeProbes(ctx context.Context) ([]models.UptimeProbe, error)
	GetUptimeProbe(ctx context.Context, id string) (*models.UptimeProbe, error)
	CreateUptimeProbe(ctx context.Context, p models.UptimeProbe) (*models.UptimeProbe, error)
	UpdateUptimeProbe(ctx context.Context, p models.UptimeProbe) error
	DeleteUptimeProbe(ctx context.Context, id string) error
	GetUptimeProbeResults(ctx context.Context, probeID string, limit int) ([]models.UptimeProbeResult, error)
	GetUptimeStats(ctx context.Context, probeID string, windowHours int) (*models.UptimeStats, error)
	RecordUptimeProbeResult(ctx context.Context, r models.UptimeProbeResult) error
}

// ProbeRunner executes a probe once and returns its result. Injected so tests can
// avoid real network I/O; defaults to synthetic.RunOnce.
type ProbeRunner func(ctx context.Context, p models.UptimeProbe) models.UptimeProbeResult

// Service holds the uptime use-cases.
type Service struct {
	repo    Repository
	runOnce ProbeRunner
}

// NewService wires the service with the production probe runner.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, runOnce: synthetic.RunOnce}
}

// probeFromRequest maps a create/update request onto a probe model, applying the
// server-side defaults (the business rules this layer owns).
func probeFromRequest(p models.UptimeProbeRequest) models.UptimeProbe {
	m := models.UptimeProbe{
		Name:              strings.TrimSpace(p.Name),
		Type:              p.Type,
		Target:            strings.TrimSpace(p.Target),
		IntervalSec:       p.IntervalSec,
		TimeoutSec:        p.TimeoutSec,
		ExpectedStatus:    p.ExpectedStatus,
		ExpectedBodyRegex: p.ExpectedBodyRegex,
		FollowRedirects:   true,
		VerifyTLS:         true,
		Enabled:           true,
	}
	if m.IntervalSec < 10 {
		m.IntervalSec = 60
	}
	if m.TimeoutSec <= 0 {
		m.TimeoutSec = 10
	}
	if m.Type == "http" && m.ExpectedStatus == 0 {
		m.ExpectedStatus = 200
	}
	if p.FollowRedirects != nil {
		m.FollowRedirects = *p.FollowRedirects
	}
	if p.VerifyTLS != nil {
		m.VerifyTLS = *p.VerifyTLS
	}
	if p.Enabled != nil {
		m.Enabled = *p.Enabled
	}
	return m
}

// ListProbes returns all probes (never nil).
func (s *Service) ListProbes(ctx context.Context) ([]models.UptimeProbe, error) {
	probes, err := s.repo.ListUptimeProbes(ctx)
	if err != nil {
		return nil, err
	}
	if probes == nil {
		probes = []models.UptimeProbe{}
	}
	return probes, nil
}

// GetProbe returns a single probe by id.
func (s *Service) GetProbe(ctx context.Context, id string) (*models.UptimeProbe, error) {
	return s.repo.GetUptimeProbe(ctx, id)
}

// CreateProbe validates+defaults the request and persists a new probe.
func (s *Service) CreateProbe(ctx context.Context, req models.UptimeProbeRequest) (*models.UptimeProbe, error) {
	return s.repo.CreateUptimeProbe(ctx, probeFromRequest(req))
}

// UpdateProbe applies the request to the probe identified by id and returns the
// stored result.
func (s *Service) UpdateProbe(ctx context.Context, id string, req models.UptimeProbeRequest) (*models.UptimeProbe, error) {
	m := probeFromRequest(req)
	m.ID = id
	if err := s.repo.UpdateUptimeProbe(ctx, m); err != nil {
		return nil, err
	}
	return s.repo.GetUptimeProbe(ctx, id)
}

// DeleteProbe removes a probe by id.
func (s *Service) DeleteProbe(ctx context.Context, id string) error {
	return s.repo.DeleteUptimeProbe(ctx, id)
}

// History returns recent result samples for a probe (never nil).
func (s *Service) History(ctx context.Context, id string, limit int) ([]models.UptimeProbeResult, error) {
	results, err := s.repo.GetUptimeProbeResults(ctx, id, limit)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []models.UptimeProbeResult{}
	}
	return results, nil
}

// Stats returns aggregated uptime/latency over a window.
func (s *Service) Stats(ctx context.Context, id string, hours int) (*models.UptimeStats, error) {
	return s.repo.GetUptimeStats(ctx, id, hours)
}

// CheckNow runs the probe immediately, records the result and returns it.
func (s *Service) CheckNow(ctx context.Context, id string) (*models.UptimeProbeResult, error) {
	probe, err := s.repo.GetUptimeProbe(ctx, id)
	if err != nil {
		return nil, err
	}
	result := s.runOnce(ctx, *probe)
	if err := s.repo.RecordUptimeProbeResult(ctx, result); err != nil {
		return nil, err
	}
	return &result, nil
}
