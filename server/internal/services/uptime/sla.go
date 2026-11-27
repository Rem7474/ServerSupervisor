package uptime

import (
	"context"
	"math"
	"time"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// SLAWindowDays are the sliding windows (in UTC calendar days, today
// included) every probe's SLA is reported over.
var SLAWindowDays = []int{7, 30, 90}

// atRiskBudgetPercent is the remaining error budget under which a window that
// still meets its objective is flagged at risk.
const atRiskBudgetPercent = 25.0

// ValidateSLOTarget rejects an objective outside (0, 100].
func ValidateSLOTarget(v float64) error {
	if math.IsNaN(v) || v <= 0 || v > 100 {
		return apperr.Validation("slo_target must be greater than 0 and at most 100").I18n(apperr.CodeInvalidSLOTarget, nil)
	}
	return nil
}

// ComputeSLAWindow measures availability over the last `days` UTC days ending
// at `today` (included) from per-day check counts, against a target in
// percent. Counts outside the window are ignored.
//
// Availability is check-based (successful checks / checks), the same ratio
// the rest of the monitoring UI reports. The error budget is the number of
// failed checks the objective tolerates for the checks actually run:
// (1 - target) × total. A 100 % objective has no budget: any failure
// exhausts it.
func ComputeSLAWindow(days int, target float64, today time.Time, counts []models.UptimeDailyCount) models.SLAWindow {
	w := models.SLAWindow{
		Days:                   days,
		AllowedDowntimeMinutes: (100 - target) / 100 * float64(days) * 24 * 60,
		Status:                 models.SLAStatusNoData,
	}
	first := truncateDay(today).AddDate(0, 0, -(days - 1))
	for _, c := range counts {
		day := truncateDay(c.Day)
		if day.Before(first) || day.After(truncateDay(today)) || c.TotalChecks <= 0 {
			continue
		}
		w.CoveredDays++
		w.TotalChecks += c.TotalChecks
		w.UpChecks += c.UpChecks
	}
	if w.TotalChecks == 0 {
		return w
	}

	uptime := float64(w.UpChecks) * 100 / float64(w.TotalChecks)
	w.UptimePercent = &uptime

	failed := float64(w.TotalChecks - w.UpChecks)
	allowed := (100 - target) / 100 * float64(w.TotalChecks)
	var remaining float64
	switch {
	case failed == 0:
		remaining = 100
	case allowed <= 0:
		remaining = 0
	default:
		remaining = math.Max(0, 100*(1-failed/allowed))
	}
	w.BudgetRemainingPercent = &remaining

	switch {
	case uptime < target:
		w.Status = models.SLAStatusBreached
	case remaining < atRiskBudgetPercent:
		w.Status = models.SLAStatusAtRisk
	default:
		w.Status = models.SLAStatusMet
	}
	return w
}

func truncateDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// SLA reports every probe's availability over SLAWindowDays against its
// objective, from the daily rollup (which outlives the raw results).
func (s *Service) SLA(ctx context.Context) (*models.SLAReport, error) {
	probes, err := s.repo.ListUptimeProbes(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	maxDays := SLAWindowDays[len(SLAWindowDays)-1]
	report := &models.SLAReport{GeneratedAt: now, WindowDays: SLAWindowDays, Probes: []models.ProbeSLA{}}
	if len(probes) == 0 {
		return report, nil
	}

	ids := make([]string, len(probes))
	for i, p := range probes {
		ids[i] = p.ID
	}
	counts, err := s.repo.GetUptimeDailyCounts(ctx, ids, maxDays)
	if err != nil {
		return nil, err
	}
	byProbe := make(map[string][]models.UptimeDailyCount, len(probes))
	for _, c := range counts {
		byProbe[c.ProbeID] = append(byProbe[c.ProbeID], c)
	}

	for _, p := range probes {
		entry := models.ProbeSLA{ProbeID: p.ID, Name: p.Name, Enabled: p.Enabled, SLOTarget: p.SLOTarget}
		for _, c := range byProbe[p.ID] {
			if c.TotalChecks <= 0 {
				continue
			}
			day := truncateDay(c.Day)
			if entry.DataSince == nil || day.Before(*entry.DataSince) {
				entry.DataSince = &day
			}
		}
		for _, days := range SLAWindowDays {
			entry.Windows = append(entry.Windows, ComputeSLAWindow(days, p.SLOTarget, now, byProbe[p.ID]))
		}
		report.Probes = append(report.Probes, entry)
	}
	return report, nil
}
