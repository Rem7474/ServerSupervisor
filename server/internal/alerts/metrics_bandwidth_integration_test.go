package alerts_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

// bandwidthAlertRule builds a bandwidth_vs_rolling_avg rule with a 1h
// baseline window and the given warn/crit thresholds (percent of baseline).
func bandwidthAlertRule(hostID string, warn, crit float64) models.AlertRule {
	window := 3600
	return models.AlertRule{
		SourceType:            models.AlertSourceAgent,
		HostID:                &hostID,
		Metric:                "bandwidth_vs_rolling_avg",
		Operator:              ">",
		ThresholdWarn:         &warn,
		ThresholdCrit:         &crit,
		BaselineWindowSeconds: &window,
		Enabled:               true,
	}
}

func insertBW(t *testing.T, db *database.DB, hostID string, rx, tx uint64, ts time.Time) {
	t.Helper()
	if _, err := db.InsertMetrics(context.Background(), &models.SystemMetrics{
		HostID: hostID, Timestamp: ts, NetworkRxBytes: rx, NetworkTxBytes: tx, Hostname: "bw",
	}); err != nil {
		t.Fatalf("insert metric: %v", err)
	}
}

// TestGetMetricValue_BandwidthVsRollingAvg covers the
// bandwidth_vs_rolling_avg case end-to-end (real DB, real GetMetricValue +
// DetermineSeverity), per the task's required coverage: a healthy ratio below
// threshold, ratio above threshold at each severity, the no-baseline-data
// case, and the division-by-zero guard.
//
// Three samples are enough to control both windows deterministically: t0
// (far in the past, inside the 1h baseline window but outside the 5-minute
// "current rate" window), t2 (inside the current-rate window), and t3="now"
// (shared endpoint of both windows). See resolveBandwidthVsRollingAvg's doc
// comment in metrics.go for the window shapes.
func TestGetMetricValue_BandwidthVsRollingAvg(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	now := time.Now()

	t0 := now.Add(-3550 * time.Second) // baseline window start
	t2 := now.Add(-250 * time.Second)  // current-rate window start
	t3 := now                          // shared endpoint

	newHost := func(t *testing.T, id string) models.Host {
		t.Helper()
		h := models.Host{ID: id, Name: id, Hostname: id, Status: "online", LastSeen: now}
		if err := db.RegisterHost(ctx, &h); err != nil {
			t.Fatalf("register host: %v", err)
		}
		return h
	}

	t.Run("healthy ratio below threshold", func(t *testing.T) {
		hostID := "bw-alert-healthy"
		host := newHost(t, hostID)
		// baseline: (3_550_000-0)/3550s = 1000 B/s; current: (3_550_000-3_300_000)/250s = 1000 B/s -> ratio 100%.
		insertBW(t, db, hostID, 0, 0, t0)
		insertBW(t, db, hostID, 3_300_000, 0, t2)
		insertBW(t, db, hostID, 3_550_000, 0, t3)

		rule := bandwidthAlertRule(hostID, 150, 200)
		value, ok := alerts.GetMetricValue(ctx, db, host, rule)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if math.Abs(value-100) > 1 {
			t.Errorf("value = %v, want ~100", value)
		}
		if sev := alerts.DetermineSeverity(rule, host, value); sev != alerts.SeverityNone {
			t.Errorf("severity = %v, want none", sev)
		}
	})

	t.Run("ratio above warn threshold", func(t *testing.T) {
		hostID := "bw-alert-warn"
		host := newHost(t, hostID)
		// baseline: 3_550_000/3550s = 1000 B/s; current: (3_550_000-3_150_000)/250s = 1600 B/s -> ratio 160%.
		insertBW(t, db, hostID, 0, 0, t0)
		insertBW(t, db, hostID, 3_150_000, 0, t2)
		insertBW(t, db, hostID, 3_550_000, 0, t3)

		rule := bandwidthAlertRule(hostID, 150, 200)
		value, ok := alerts.GetMetricValue(ctx, db, host, rule)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if math.Abs(value-160) > 1 {
			t.Errorf("value = %v, want ~160", value)
		}
		if sev := alerts.DetermineSeverity(rule, host, value); sev != alerts.SeverityWarn {
			t.Errorf("severity = %v, want warn", sev)
		}
	})

	t.Run("ratio above crit threshold", func(t *testing.T) {
		hostID := "bw-alert-crit"
		host := newHost(t, hostID)
		// baseline: 3_550_000/3550s = 1000 B/s; current: (3_550_000-2_925_000)/250s = 2500 B/s -> ratio 250%.
		insertBW(t, db, hostID, 0, 0, t0)
		insertBW(t, db, hostID, 2_925_000, 0, t2)
		insertBW(t, db, hostID, 3_550_000, 0, t3)

		rule := bandwidthAlertRule(hostID, 150, 200)
		value, ok := alerts.GetMetricValue(ctx, db, host, rule)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if math.Abs(value-250) > 1 {
			t.Errorf("value = %v, want ~250", value)
		}
		if sev := alerts.DetermineSeverity(rule, host, value); sev != alerts.SeverityCrit {
			t.Errorf("severity = %v, want crit", sev)
		}
	})

	t.Run("no data yet returns not ok", func(t *testing.T) {
		hostID := "bw-alert-nodata"
		host := newHost(t, hostID)
		// Only a single sample exists — not enough to compute any rate.
		insertBW(t, db, hostID, 1_000_000, 0, t3)

		rule := bandwidthAlertRule(hostID, 150, 200)
		if _, ok := alerts.GetMetricValue(ctx, db, host, rule); ok {
			t.Error("expected ok=false with only one sample total")
		}
	})

	t.Run("zero baseline rate is guarded against division by zero", func(t *testing.T) {
		hostID := "bw-alert-zerobaseline"
		host := newHost(t, hostID)
		// Flat counters the whole way: baseline delta is 0 -> baseline rate 0.
		insertBW(t, db, hostID, 1_000, 0, t0)
		insertBW(t, db, hostID, 1_000, 0, t2)
		insertBW(t, db, hostID, 1_000, 0, t3)

		rule := bandwidthAlertRule(hostID, 150, 200)
		if _, ok := alerts.GetMetricValue(ctx, db, host, rule); ok {
			t.Error("expected ok=false when the baseline rate is zero")
		}
	})
}
