package handlers

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/apperr"
)

// maxTimeRangeWindow caps how far apart from/to can be, to bound the cost of
// unindexed captured_at/timestamp range scans — see GetWebLogsSummary's
// ~10-way goroutine fan-out (root CLAUDE.md's "Web-logs dashboard
// performance" section): the only index on captured_at is a plain
// (captured_at DESC), not composite with host_id, so an unbounded custom
// range on a busy, unfiltered install would be expensive.
const maxTimeRangeWindow = 90 * 24 * time.Hour

// parseTimeRange resolves the effective query window for an endpoint that
// supports both a relative preset (?period=24h, the pre-existing behavior)
// and a precise custom range (?from=RFC3339&to=RFC3339, new). from/to take
// priority when both present; until is the zero value when no upper bound
// applies (period mode) — every call site must already treat a zero until as
// "no upper bound", mirroring GetWebLogsKPIWindow's existing
// `if !until.IsZero()` check, so this is purely additive: nothing that only
// ever passes a period-derived since/zero-until changes behavior.
func parseTimeRange(c *gin.Context, defaultPeriod string) (since, until time.Time, ok bool) {
	fromRaw := strings.TrimSpace(c.Query("from"))
	toRaw := strings.TrimSpace(c.Query("to"))
	if fromRaw != "" || toRaw != "" {
		if fromRaw == "" || toRaw == "" {
			respondError(c, apperr.Validation("from and to must both be provided"))
			return time.Time{}, time.Time{}, false
		}
		from, err := time.Parse(time.RFC3339, fromRaw)
		if err != nil {
			respondError(c, apperr.Validation("invalid from (expected RFC3339, e.g. 2026-08-08T13:25:00Z)"))
			return time.Time{}, time.Time{}, false
		}
		to, err := time.Parse(time.RFC3339, toRaw)
		if err != nil {
			respondError(c, apperr.Validation("invalid to (expected RFC3339, e.g. 2026-08-08T15:02:00Z)"))
			return time.Time{}, time.Time{}, false
		}
		if !to.After(from) {
			respondError(c, apperr.Validation("to must be after from"))
			return time.Time{}, time.Time{}, false
		}
		if to.Sub(from) > maxTimeRangeWindow {
			respondError(c, apperr.Validation("range too wide (max 90 days)"))
			return time.Time{}, time.Time{}, false
		}
		// A future `to` isn't rejected (client/server clock skew is common and
		// harmless here — the query would just return fewer/no rows past
		// "now"), but is clamped so callers never see an until that implies
		// data could exist past the present.
		if now := time.Now(); to.After(now) {
			to = now
		}
		return from, to, true
	}

	raw := strings.TrimSpace(c.DefaultQuery("period", defaultPeriod))
	period, err := time.ParseDuration(raw)
	if err != nil || period <= 0 {
		respondError(c, apperr.Validation("invalid period (example: 24h, 168h)"))
		return time.Time{}, time.Time{}, false
	}
	return time.Now().Add(-period), time.Time{}, true
}
