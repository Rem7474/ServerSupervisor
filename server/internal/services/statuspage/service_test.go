package statuspage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// fakeRepo is an in-memory Repository. probes holds every probe, published
// only the ones on the page: the tests check the others never show up.
type fakeRepo struct {
	cfg       *models.StatusPageConfig
	cfgErr    error
	probes    []models.UptimeProbe
	published []models.PublishedProbe
	daily     []models.UptimeDailyCount
	downs     []models.UptimeDownPeriod
	replaced  *models.StatusPageConfigRequest
	audits    []string

	dailyIDs   []string
	downIDs    []string
	buildCount int
}

func (f *fakeRepo) GetStatusPageConfig(context.Context) (*models.StatusPageConfig, error) {
	f.buildCount++
	if f.cfgErr != nil {
		return nil, f.cfgErr
	}
	cp := *f.cfg
	return &cp, nil
}
func (f *fakeRepo) ReplaceStatusPageConfig(_ context.Context, req models.StatusPageConfigRequest) error {
	f.replaced = &req
	f.cfg = &models.StatusPageConfig{Enabled: req.Enabled, Title: req.Title, Description: req.Description, Services: req.Services}
	return nil
}
func (f *fakeRepo) ListPublishedProbes(context.Context) ([]models.PublishedProbe, error) {
	return f.published, nil
}
func (f *fakeRepo) ListUptimeProbes(context.Context) ([]models.UptimeProbe, error) {
	return f.probes, nil
}
func (f *fakeRepo) GetUptimeDailyCounts(_ context.Context, ids []string, _ int) ([]models.UptimeDailyCount, error) {
	f.dailyIDs = ids
	return f.daily, nil
}
func (f *fakeRepo) ListUptimeDownPeriods(_ context.Context, ids []string, _ time.Time, _ int) ([]models.UptimeDownPeriod, error) {
	f.downIDs = ids
	return f.downs, nil
}
func (f *fakeRepo) CreateAuditLog(_ context.Context, user, action, _, _, details, _ string) (int64, error) {
	f.audits = append(f.audits, user+":"+action+":"+details)
	return 1, nil
}

var now = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func newSvc(repo *fakeRepo) *Service {
	s := NewService(repo)
	s.now = func() time.Time { return now }
	return s
}

// leakyRepo publishes one probe out of two, both with internal details
// (target, error text) that must never reach the public view.
func leakyRepo() *fakeRepo {
	resolved := now.Add(-2 * time.Hour)
	return &fakeRepo{
		cfg: &models.StatusPageConfig{Enabled: true, Title: "Acme status", Description: "Public services"},
		probes: []models.UptimeProbe{
			{ID: "pub", Name: "internal-api-01", Target: "http://10.0.0.12:8080/health", LastError: "dial tcp 10.0.0.12:8080: connection refused"},
			{ID: "secret", Name: "db-primary", Target: "10.0.0.50:5432"},
		},
		published: []models.PublishedProbe{{ProbeID: "pub", PublicName: "Website", Enabled: true, LastStatus: "down"}},
		daily: []models.UptimeDailyCount{
			{ProbeID: "pub", Day: now.AddDate(0, 0, -1), TotalChecks: 100, UpChecks: 90},
			{ProbeID: "pub", Day: now, TotalChecks: 100, UpChecks: 100},
		},
		downs: []models.UptimeDownPeriod{
			{ProbeID: "pub", StartedAt: now.Add(-10 * time.Minute)},
			{ProbeID: "pub", StartedAt: now.Add(-3 * time.Hour), ResolvedAt: &resolved},
		},
	}
}

func TestPublic_DisabledIsNotFound(t *testing.T) {
	repo := leakyRepo()
	repo.cfg.Enabled = false
	_, err := newSvc(repo).Public(context.Background())
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 404 {
		t.Fatalf("disabled page must be a 404, got %v", err)
	}
	if repo.dailyIDs != nil || repo.downIDs != nil {
		t.Fatal("a disabled page must not query any probe data")
	}
}

func TestPublic_MissingConfigRowIsNotFound(t *testing.T) {
	repo := leakyRepo()
	repo.cfgErr = sql.ErrNoRows
	enabled, err := newSvc(repo).Enabled(context.Background())
	if err != nil || enabled {
		t.Fatalf("no config row → disabled, got %v / %v", enabled, err)
	}
}

func TestPublic_ConfigErrorPropagates(t *testing.T) {
	repo := leakyRepo()
	boom := errors.New("boom")
	repo.cfgErr = boom
	if _, err := newSvc(repo).Enabled(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("expected the repository error, got %v", err)
	}
}

func TestPublic_OnlyPublishedServicesAndPublicFields(t *testing.T) {
	repo := leakyRepo()
	page, err := newSvc(repo).Public(context.Background())
	if err != nil {
		t.Fatalf("Public: %v", err)
	}
	if strings.Join(repo.dailyIDs, ",") != "pub" || strings.Join(repo.downIDs, ",") != "pub" {
		t.Fatalf("only published probes may be queried, got %v / %v", repo.dailyIDs, repo.downIDs)
	}

	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	for _, secret := range []string{"10.0.0", "internal-api-01", "db-primary", "secret", "connection refused", "8080", "5432", "\"pub\""} {
		if strings.Contains(body, secret) {
			t.Errorf("public payload leaks %q: %s", secret, body)
		}
	}

	// Strict key whitelist at every level of the payload.
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, "page", generic, "title", "description", "status", "generated_at", "services", "incidents")
	svc := generic["services"].([]any)[0].(map[string]any)
	assertKeys(t, "service", svc, "name", "status", "uptime_percent_90d", "days")
	assertKeys(t, "day", svc["days"].([]any)[0].(map[string]any), "date", "uptime_percent")
	assertKeys(t, "incident", generic["incidents"].([]any)[0].(map[string]any), "service", "started_at", "resolved_at")

	if page.Status != models.PublicStatusMajorOutage || page.Services[0].Status != models.PublicStatusMajorOutage {
		t.Fatalf("a down probe is a major outage, got %s / %s", page.Status, page.Services[0].Status)
	}
	s := page.Services[0]
	if s.Name != "Website" || len(s.Days) != HistoryDays {
		t.Fatalf("unexpected service %q with %d days", s.Name, len(s.Days))
	}
	if s.Days[HistoryDays-1].Date != "2026-11-25" || s.Days[0].Date != "2026-08-28" {
		t.Fatalf("bar must end today and span 90 days, got %s..%s", s.Days[0].Date, s.Days[HistoryDays-1].Date)
	}
	if s.Days[0].UptimePercent != nil {
		t.Fatal("a day without checks must have a nil uptime")
	}
	if got := *s.Days[HistoryDays-2].UptimePercent; got != 90 {
		t.Fatalf("yesterday uptime = %v, want 90", got)
	}
	if got := *s.UptimePercent90d; got != 95 {
		t.Fatalf("90-day uptime = %v, want 95", got)
	}
	if len(page.Incidents) != 2 || page.Incidents[0].Service != "Website" || page.Incidents[0].ResolvedAt != nil || page.Incidents[1].ResolvedAt == nil {
		t.Fatalf("unexpected incidents %+v", page.Incidents)
	}
}

func assertKeys(t *testing.T, label string, m map[string]any, want ...string) {
	t.Helper()
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s keys = %v, want exactly %v", label, got, want)
	}
}

func TestPublic_NoPublishedService(t *testing.T) {
	repo := leakyRepo()
	repo.published = nil
	page, err := newSvc(repo).Public(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if page.Status != models.PublicStatusUnknown || len(page.Services) != 0 || page.Services == nil {
		t.Fatalf("empty page expected, got %+v", page)
	}
}

func TestPublic_CachedThenInvalidatedOnUpdate(t *testing.T) {
	repo := leakyRepo()
	svc := newSvc(repo)
	for range 3 {
		if _, err := svc.Public(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if repo.buildCount != 1 {
		t.Fatalf("expected one build within the TTL, got %d", repo.buildCount)
	}

	// Disabling through the admin endpoint takes effect immediately.
	if _, err := svc.UpdateConfig(context.Background(), models.StatusPageConfigRequest{Enabled: false}, "admin", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Public(context.Background()); err == nil {
		t.Fatal("page must be gone right after being disabled")
	}

	// The disabled state is cached too, until the TTL runs out.
	repo.cfg.Enabled = true
	if _, err := svc.Public(context.Background()); err == nil {
		t.Fatal("disabled state should still be cached")
	}
	svc.now = func() time.Time { return now.Add(CacheTTL + time.Second) }
	if _, err := svc.Public(context.Background()); err != nil {
		t.Fatalf("after the TTL the page is rebuilt: %v", err)
	}
}

func TestUpdateConfig_ValidatesAndAudits(t *testing.T) {
	repo := leakyRepo()
	svc := newSvc(repo)
	cfg, err := svc.UpdateConfig(context.Background(), models.StatusPageConfigRequest{
		Enabled:  true,
		Title:    "  Status  ",
		Services: []models.StatusPageService{{ProbeID: "secret", PublicName: "  Database  "}},
	}, "alice", "10.1.1.1")
	if err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if repo.replaced.Title != "Status" || repo.replaced.Services[0].PublicName != "Database" {
		t.Fatalf("values must be trimmed, got %+v", repo.replaced)
	}
	if cfg == nil || !cfg.Enabled {
		t.Fatalf("expected the stored config back, got %+v", cfg)
	}
	if len(repo.audits) != 1 || repo.audits[0] != "alice:status_page_update:enabled=true services=1" {
		t.Fatalf("unexpected audit %v", repo.audits)
	}

	cases := map[string]struct {
		req  models.StatusPageConfigRequest
		code string
	}{
		"empty name":    {models.StatusPageConfigRequest{Services: []models.StatusPageService{{ProbeID: "pub", PublicName: "   "}}}, apperr.CodeStatusPagePublicName},
		"long name":     {models.StatusPageConfigRequest{Services: []models.StatusPageService{{ProbeID: "pub", PublicName: strings.Repeat("é", 81)}}}, apperr.CodeStatusPagePublicName},
		"unknown probe": {models.StatusPageConfigRequest{Services: []models.StatusPageService{{ProbeID: "nope", PublicName: "x"}}}, apperr.CodeStatusPageUnknownProbe},
		"duplicate":     {models.StatusPageConfigRequest{Services: []models.StatusPageService{{ProbeID: "pub", PublicName: "a"}, {ProbeID: "pub", PublicName: "b"}}}, apperr.CodeStatusPageDuplicateProbe},
		"long title":    {models.StatusPageConfigRequest{Title: strings.Repeat("t", MaxTitleLen+1)}, apperr.CodeStatusPageTextTooLong},
		"long desc":     {models.StatusPageConfigRequest{Description: strings.Repeat("d", MaxDescriptionLen+1)}, apperr.CodeStatusPageTextTooLong},
		"too many":      {models.StatusPageConfigRequest{Services: make([]models.StatusPageService, MaxServices+1)}, apperr.CodeInvalidInput},
	}
	for name, tc := range cases {
		repo.replaced = nil
		_, err := svc.UpdateConfig(context.Background(), tc.req, "alice", "")
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.I18nKey != tc.code {
			t.Errorf("%s: want %s, got %v", name, tc.code, err)
		}
		if repo.replaced != nil {
			t.Errorf("%s: invalid config must not be stored", name)
		}
	}
}

func TestServiceAndOverallStatus(t *testing.T) {
	if ServiceStatus(false, "up") != models.PublicStatusUnknown {
		t.Error("a disabled probe is unknown, whatever its last status")
	}
	if ServiceStatus(true, "up") != models.PublicStatusOperational || ServiceStatus(true, "down") != models.PublicStatusMajorOutage || ServiceStatus(true, "unknown") != models.PublicStatusUnknown {
		t.Error("unexpected status mapping")
	}
	op, down, unk := models.PublicStatusOperational, models.PublicStatusMajorOutage, models.PublicStatusUnknown
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{op, op}, op},
		{[]string{op, down}, models.PublicStatusPartialOutage},
		{[]string{down, unk}, down},
		{[]string{unk}, unk},
		{nil, unk},
		{[]string{op, unk}, op},
	}
	for _, c := range cases {
		if got := OverallStatus(c.in); got != c.want {
			t.Errorf("OverallStatus(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}
