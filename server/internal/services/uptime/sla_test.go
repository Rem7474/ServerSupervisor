package uptime

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

var slaToday = time.Date(2026, 11, 25, 15, 30, 0, 0, time.UTC)

func day(offset int) time.Time {
	return time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC).AddDate(0, 0, offset)
}

func count(probe string, offset, total, up int) models.UptimeDailyCount {
	return models.UptimeDailyCount{ProbeID: probe, Day: day(offset), TotalChecks: total, UpChecks: up}
}

func approx(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: got nil, want %v", label, want)
	}
	if math.Abs(*got-want) > 1e-9 {
		t.Fatalf("%s: got %v, want %v", label, *got, want)
	}
}

func TestComputeSLAWindow_NoData(t *testing.T) {
	w := ComputeSLAWindow(30, 99.9, slaToday, nil)
	if w.Status != models.SLAStatusNoData || w.UptimePercent != nil || w.BudgetRemainingPercent != nil || w.CoveredDays != 0 {
		t.Fatalf("expected an empty no_data window, got %+v", w)
	}
	// 0.1 % of 30 days = 43.2 minutes, reported even without data.
	if math.Abs(w.AllowedDowntimeMinutes-43.2) > 1e-9 {
		t.Fatalf("allowed downtime = %v, want 43.2", w.AllowedDowntimeMinutes)
	}
}

func TestComputeSLAWindow_WindowBoundsAreInclusiveOfTodayAndFirstDay(t *testing.T) {
	counts := []models.UptimeDailyCount{
		count("p", -7, 1000, 0), // just outside a 7-day window (today + 6 days back)
		count("p", -6, 100, 100),
		count("p", 0, 100, 100),
		count("p", 1, 1000, 0), // future day, ignored
		count("p", -3, 0, 0),   // empty day, not covered
	}
	w := ComputeSLAWindow(7, 99, slaToday, counts)
	if w.TotalChecks != 200 || w.UpChecks != 200 || w.CoveredDays != 2 {
		t.Fatalf("window kept the wrong days: %+v", w)
	}
	approx(t, "uptime", w.UptimePercent, 100)
	approx(t, "budget", w.BudgetRemainingPercent, 100)
	if w.Status != models.SLAStatusMet {
		t.Fatalf("status = %s, want met", w.Status)
	}
}

func TestComputeSLAWindow_ExactlyOnTargetConsumesWholeBudget(t *testing.T) {
	// 1 failure in 1000 checks is exactly 99.9 %: the objective is met but
	// the budget is used up, which is at risk rather than breached.
	w := ComputeSLAWindow(30, 99.9, slaToday, []models.UptimeDailyCount{count("p", 0, 1000, 999)})
	approx(t, "uptime", w.UptimePercent, 99.9)
	approx(t, "budget", w.BudgetRemainingPercent, 0)
	if w.Status != models.SLAStatusAtRisk {
		t.Fatalf("status = %s, want at_risk", w.Status)
	}
}

func TestComputeSLAWindow_BudgetPartiallyUsed(t *testing.T) {
	// 99 % target over 1000 checks tolerates 10 failures; 2 used → 80 % left.
	w := ComputeSLAWindow(7, 99, slaToday, []models.UptimeDailyCount{count("p", -1, 600, 599), count("p", 0, 400, 399)})
	approx(t, "budget", w.BudgetRemainingPercent, 80)
	if w.Status != models.SLAStatusMet {
		t.Fatalf("status = %s, want met", w.Status)
	}
	// 9 failures → 10 % left, under the at-risk threshold.
	w = ComputeSLAWindow(7, 99, slaToday, []models.UptimeDailyCount{count("p", 0, 1000, 991)})
	approx(t, "budget", w.BudgetRemainingPercent, 10)
	if w.Status != models.SLAStatusAtRisk {
		t.Fatalf("status = %s, want at_risk", w.Status)
	}
}

func TestComputeSLAWindow_BreachedClampsBudgetAtZero(t *testing.T) {
	w := ComputeSLAWindow(7, 99.9, slaToday, []models.UptimeDailyCount{count("p", 0, 100, 90)})
	approx(t, "uptime", w.UptimePercent, 90)
	approx(t, "budget", w.BudgetRemainingPercent, 0)
	if w.Status != models.SLAStatusBreached {
		t.Fatalf("status = %s, want breached", w.Status)
	}
}

func TestComputeSLAWindow_HundredPercentTarget(t *testing.T) {
	clean := ComputeSLAWindow(7, 100, slaToday, []models.UptimeDailyCount{count("p", 0, 50, 50)})
	approx(t, "budget", clean.BudgetRemainingPercent, 100)
	if clean.Status != models.SLAStatusMet || clean.AllowedDowntimeMinutes != 0 {
		t.Fatalf("a flawless window meets a 100 %% objective: %+v", clean)
	}
	one := ComputeSLAWindow(7, 100, slaToday, []models.UptimeDailyCount{count("p", 0, 50, 49)})
	approx(t, "budget", one.BudgetRemainingPercent, 0)
	if one.Status != models.SLAStatusBreached {
		t.Fatalf("one failure breaches a 100 %% objective, got %s", one.Status)
	}
}

func TestValidateSLOTarget(t *testing.T) {
	for _, ok := range []float64{0.001, 50, 99.999, 100} {
		if err := ValidateSLOTarget(ok); err != nil {
			t.Errorf("%v should be valid: %v", ok, err)
		}
	}
	for _, bad := range []float64{0, -1, 100.001, math.NaN()} {
		err := ValidateSLOTarget(bad)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.I18nKey != apperr.CodeInvalidSLOTarget {
			t.Errorf("%v should be rejected with INVALID_SLO_TARGET, got %v", bad, err)
		}
	}
}

func TestSLA_ReportsEveryProbeWithCoverage(t *testing.T) {
	repo := &fakeRepo{
		listProbe: []models.UptimeProbe{
			{ID: "a", Name: "api", Enabled: true, SLOTarget: 99},
			{ID: "b", Name: "new", Enabled: true, SLOTarget: 99.9},
		},
		daily: []models.UptimeDailyCount{
			count("a", -40, 100, 100), // only in the 90-day window
			count("a", -2, 100, 98),
			count("a", 0, 0, 0), // empty day must not move data_since
		},
	}
	svc := NewService(repo)
	svc.now = func() time.Time { return slaToday }

	report, err := svc.SLA(context.Background())
	if err != nil {
		t.Fatalf("SLA: %v", err)
	}
	if repo.dailyDays != 90 || len(repo.dailyIDs) != 2 {
		t.Fatalf("expected one 90-day query for both probes, got %d days / %v", repo.dailyDays, repo.dailyIDs)
	}
	if len(report.Probes) != 2 {
		t.Fatalf("expected 2 probes, got %d", len(report.Probes))
	}
	a := report.Probes[0]
	if a.DataSince == nil || !a.DataSince.Equal(day(-40)) {
		t.Fatalf("data_since = %v, want %v", a.DataSince, day(-40))
	}
	if len(a.Windows) != 3 || a.Windows[0].Days != 7 || a.Windows[2].Days != 90 {
		t.Fatalf("unexpected windows %+v", a.Windows)
	}
	if a.Windows[0].CoveredDays != 1 || a.Windows[2].CoveredDays != 2 {
		t.Fatalf("coverage 7d=%d 90d=%d, want 1 and 2", a.Windows[0].CoveredDays, a.Windows[2].CoveredDays)
	}
	approx(t, "7d uptime", a.Windows[0].UptimePercent, 98)
	approx(t, "90d uptime", a.Windows[2].UptimePercent, 99)

	b := report.Probes[1]
	if b.DataSince != nil || b.Windows[0].Status != models.SLAStatusNoData {
		t.Fatalf("a probe without results reports no data, got %+v", b)
	}
}

func TestSLA_EmptyAndErrors(t *testing.T) {
	svc := NewService(&fakeRepo{})
	report, err := svc.SLA(context.Background())
	if err != nil || report == nil || len(report.Probes) != 0 {
		t.Fatalf("no probe → empty report, got %+v / %v", report, err)
	}

	boom := errors.New("boom")
	if _, err := NewService(&fakeRepo{listErr: boom}).SLA(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("list error must propagate, got %v", err)
	}
	repo := &fakeRepo{listProbe: []models.UptimeProbe{{ID: "a"}}, dailyErr: boom}
	if _, err := NewService(repo).SLA(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("daily error must propagate, got %v", err)
	}
}

func TestCreateAndUpdateProbe_SLOTarget(t *testing.T) {
	repo := &fakeRepo{probe: &models.UptimeProbe{ID: "x"}}
	svc := NewService(repo)
	target := 99.5
	if _, err := svc.CreateProbe(context.Background(), models.UptimeProbeRequest{Name: "n", Type: "tcp", Target: "h:1", SLOTarget: &target}); err != nil {
		t.Fatalf("CreateProbe: %v", err)
	}
	if repo.created.SLOTarget != 99.5 {
		t.Fatalf("slo_target not passed through: %v", repo.created.SLOTarget)
	}
	// Omitted on update → zero, which the repository treats as "keep".
	if _, err := svc.UpdateProbe(context.Background(), "x", models.UptimeProbeRequest{Name: "n", Type: "tcp", Target: "h:1"}); err != nil {
		t.Fatalf("UpdateProbe: %v", err)
	}
	if repo.updated.SLOTarget != 0 {
		t.Fatalf("omitted slo_target should stay unset, got %v", repo.updated.SLOTarget)
	}

	bad := 150.0
	repo.created, repo.updated = nil, nil
	if _, err := svc.CreateProbe(context.Background(), models.UptimeProbeRequest{Name: "n", Type: "tcp", Target: "h:1", SLOTarget: &bad}); err == nil || repo.created != nil {
		t.Fatalf("invalid objective must be rejected before storing")
	}
	if _, err := svc.UpdateProbe(context.Background(), "x", models.UptimeProbeRequest{Name: "n", Type: "tcp", Target: "h:1", SLOTarget: &bad}); err == nil || repo.updated != nil {
		t.Fatalf("invalid objective must be rejected before updating")
	}
}
