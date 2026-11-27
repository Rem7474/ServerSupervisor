// Package statuspage is the service layer of the opt-in public status page:
// the admin configuration (which uptime probes are published, under which
// public name) and the read-only public view built from it.
//
// The public view is built field by field from the public name and check
// counts only: nothing of the probe itself (target, type, host, status code,
// error text) is read into it, so nothing can leak by being forgotten in a
// filter.
package statuspage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// Field limits, mirrored by the status_page_services CHECK constraint for the
// public name.
const (
	MaxPublicNameLen  = 80
	MaxTitleLen       = 120
	MaxDescriptionLen = 500
	MaxServices       = 50

	// HistoryDays is the length of each service's uptime bar.
	HistoryDays = 90
	// IncidentDays is how far back the recent incidents list goes. Raw
	// results are kept 30 days, so this stays well inside that.
	IncidentDays = 14
	// MaxIncidents caps the recent incidents list.
	MaxIncidents = 20
	// CacheTTL is how long a built public view is served before being
	// rebuilt: the endpoint is unauthenticated, so a burst of visitors costs
	// one set of queries per TTL, not one per request.
	CacheTTL = 30 * time.Second
)

// Repository is the data-access port. *database.DB satisfies it structurally.
type Repository interface {
	GetStatusPageConfig(ctx context.Context) (*models.StatusPageConfig, error)
	ReplaceStatusPageConfig(ctx context.Context, req models.StatusPageConfigRequest) error
	ListPublishedProbes(ctx context.Context) ([]models.PublishedProbe, error)
	ListUptimeProbes(ctx context.Context) ([]models.UptimeProbe, error)
	GetUptimeDailyCounts(ctx context.Context, probeIDs []string, days int) ([]models.UptimeDailyCount, error)
	ListUptimeDownPeriods(ctx context.Context, probeIDs []string, since time.Time, limit int) ([]models.UptimeDownPeriod, error)
	CreateAuditLog(ctx context.Context, username, action, hostID, ipAddress, details, status string) (int64, error)
}

// Service holds the status page use-cases.
type Service struct {
	repo Repository
	now  func() time.Time

	mu        sync.Mutex
	cached    *models.PublicStatusPage // nil + cachedOK = disabled
	cachedOK  bool
	cachedExp time.Time
}

// NewService wires the service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// GetConfig returns the admin configuration.
func (s *Service) GetConfig(ctx context.Context) (*models.StatusPageConfig, error) {
	return s.repo.GetStatusPageConfig(ctx)
}

// UpdateConfig validates and stores the configuration, records who changed
// it, and drops the cached public view so the change shows at once.
func (s *Service) UpdateConfig(ctx context.Context, req models.StatusPageConfigRequest, actor, clientIP string) (*models.StatusPageConfig, error) {
	clean, err := s.validate(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceStatusPageConfig(ctx, clean); err != nil {
		return nil, err
	}
	s.invalidate()
	details := fmt.Sprintf("enabled=%t services=%d", clean.Enabled, len(clean.Services))
	_, _ = s.repo.CreateAuditLog(ctx, actor, "status_page_update", "", clientIP, details, "completed")
	return s.repo.GetStatusPageConfig(ctx)
}

func (s *Service) validate(ctx context.Context, req models.StatusPageConfigRequest) (models.StatusPageConfigRequest, error) {
	out := models.StatusPageConfigRequest{
		Enabled:     req.Enabled,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		Services:    []models.StatusPageService{},
	}
	if utf8.RuneCountInString(out.Title) > MaxTitleLen || utf8.RuneCountInString(out.Description) > MaxDescriptionLen {
		return out, apperr.Validation("title or description too long").I18n(apperr.CodeStatusPageTextTooLong, nil)
	}
	if len(req.Services) > MaxServices {
		return out, apperr.Validation(fmt.Sprintf("at most %d services can be published", MaxServices)).I18n(apperr.CodeInvalidInput, nil)
	}
	if len(req.Services) == 0 {
		return out, nil
	}

	probes, err := s.repo.ListUptimeProbes(ctx)
	if err != nil {
		return out, err
	}
	known := make(map[string]bool, len(probes))
	for _, p := range probes {
		known[p.ID] = true
	}
	seen := make(map[string]bool, len(req.Services))
	for _, svc := range req.Services {
		name := strings.TrimSpace(svc.PublicName)
		if name == "" || utf8.RuneCountInString(name) > MaxPublicNameLen {
			return out, apperr.Validation("public name must be 1 to 80 characters").I18n(apperr.CodeStatusPagePublicName, nil)
		}
		if !known[svc.ProbeID] {
			return out, apperr.Validation("unknown probe").I18n(apperr.CodeStatusPageUnknownProbe, nil)
		}
		if seen[svc.ProbeID] {
			return out, apperr.Validation("probe published twice").I18n(apperr.CodeStatusPageDuplicateProbe, nil)
		}
		seen[svc.ProbeID] = true
		out.Services = append(out.Services, models.StatusPageService{ProbeID: svc.ProbeID, PublicName: name})
	}
	return out, nil
}

func (s *Service) invalidate() {
	s.mu.Lock()
	s.cachedOK = false
	s.cached = nil
	s.mu.Unlock()
}

// errNotFound is what the public endpoint answers while the page is disabled:
// indistinguishable from a route that does not exist.
func errNotFound() error {
	return apperr.NotFound("status page not found").I18n(apperr.CodeNotFound, nil)
}

// Enabled reports whether the public page is currently enabled (cached like
// the public view).
func (s *Service) Enabled(ctx context.Context) (bool, error) {
	page, err := s.Public(ctx)
	if err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) && ae.HTTPStatus == 404 {
			return false, nil
		}
		return false, err
	}
	return page != nil, nil
}

// Public returns the public view, or a NotFound error while the page is
// disabled. Results (including "disabled") are cached for CacheTTL.
func (s *Service) Public(ctx context.Context) (*models.PublicStatusPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.cachedOK && now.Before(s.cachedExp) {
		if s.cached == nil {
			return nil, errNotFound()
		}
		return s.cached, nil
	}

	page, err := s.build(ctx, now)
	if err != nil {
		return nil, err
	}
	s.cached, s.cachedOK, s.cachedExp = page, true, now.Add(CacheTTL)
	if page == nil {
		return nil, errNotFound()
	}
	return page, nil
}

// build assembles the public view; nil when the page is disabled.
func (s *Service) build(ctx context.Context, now time.Time) (*models.PublicStatusPage, error) {
	cfg, err := s.repo.GetStatusPageConfig(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, nil
	}
	published, err := s.repo.ListPublishedProbes(ctx)
	if err != nil {
		return nil, err
	}

	page := &models.PublicStatusPage{
		Title:       cfg.Title,
		Description: cfg.Description,
		GeneratedAt: now.UTC(),
		Services:    []models.PublicStatusService{},
		Incidents:   []models.PublicStatusIncident{},
	}
	if len(published) == 0 {
		page.Status = models.PublicStatusUnknown
		return page, nil
	}

	ids := make([]string, len(published))
	names := make(map[string]string, len(published))
	for i, p := range published {
		ids[i] = p.ProbeID
		names[p.ProbeID] = p.PublicName
	}
	counts, err := s.repo.GetUptimeDailyCounts(ctx, ids, HistoryDays)
	if err != nil {
		return nil, err
	}
	byProbeDay := make(map[string]map[string]models.UptimeDailyCount, len(published))
	for _, c := range counts {
		if byProbeDay[c.ProbeID] == nil {
			byProbeDay[c.ProbeID] = map[string]models.UptimeDailyCount{}
		}
		byProbeDay[c.ProbeID][c.Day.UTC().Format(time.DateOnly)] = c
	}

	statuses := make([]string, 0, len(published))
	for _, p := range published {
		svc := buildService(p, byProbeDay[p.ProbeID], now)
		statuses = append(statuses, svc.Status)
		page.Services = append(page.Services, svc)
	}
	page.Status = OverallStatus(statuses)

	periods, err := s.repo.ListUptimeDownPeriods(ctx, ids, now.AddDate(0, 0, -IncidentDays), MaxIncidents)
	if err != nil {
		return nil, err
	}
	for _, d := range periods {
		page.Incidents = append(page.Incidents, models.PublicStatusIncident{
			Service:    names[d.ProbeID],
			StartedAt:  d.StartedAt.UTC(),
			ResolvedAt: utcPtr(d.ResolvedAt),
		})
	}
	return page, nil
}

func buildService(p models.PublishedProbe, days map[string]models.UptimeDailyCount, now time.Time) models.PublicStatusService {
	svc := models.PublicStatusService{
		Name:   p.PublicName,
		Status: ServiceStatus(p.Enabled, p.LastStatus),
		Days:   make([]models.PublicStatusDay, 0, HistoryDays),
	}
	today := now.UTC()
	var total, up int
	for i := HistoryDays - 1; i >= 0; i-- {
		key := today.AddDate(0, 0, -i).Format(time.DateOnly)
		day := models.PublicStatusDay{Date: key}
		if c, ok := days[key]; ok && c.TotalChecks > 0 {
			pct := float64(c.UpChecks) * 100 / float64(c.TotalChecks)
			day.UptimePercent = &pct
			total += c.TotalChecks
			up += c.UpChecks
		}
		svc.Days = append(svc.Days, day)
	}
	if total > 0 {
		pct := float64(up) * 100 / float64(total)
		svc.UptimePercent90d = &pct
	}
	return svc
}

// ServiceStatus maps a probe's live state to a public status. A disabled
// probe is no longer checked, so its last state is not current: unknown.
func ServiceStatus(enabled bool, lastStatus string) string {
	if !enabled {
		return models.PublicStatusUnknown
	}
	switch lastStatus {
	case "up":
		return models.PublicStatusOperational
	case "down":
		return models.PublicStatusMajorOutage
	default:
		return models.PublicStatusUnknown
	}
}

// OverallStatus summarises service statuses: every known service down is a
// major outage, some down a partial outage, all up operational. Unknown
// services do not count unless nothing is known.
func OverallStatus(statuses []string) string {
	var up, down int
	for _, st := range statuses {
		switch st {
		case models.PublicStatusOperational:
			up++
		case models.PublicStatusMajorOutage:
			down++
		}
	}
	switch {
	case down > 0 && up == 0:
		return models.PublicStatusMajorOutage
	case down > 0:
		return models.PublicStatusPartialOutage
	case up > 0:
		return models.PublicStatusOperational
	default:
		return models.PublicStatusUnknown
	}
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
