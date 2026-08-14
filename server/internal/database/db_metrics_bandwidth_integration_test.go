package database_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func insertBandwidthSample(t *testing.T, db *database.DB, hostID string, rx, tx uint64, ts time.Time) {
	t.Helper()
	if _, err := db.InsertMetrics(context.Background(), &models.SystemMetrics{
		HostID:         hostID,
		Timestamp:      ts,
		NetworkRxBytes: rx,
		NetworkTxBytes: tx,
		Hostname:       "bandwidth-host",
	}); err != nil {
		t.Fatalf("insert metric: %v", err)
	}
}

func mustRegisterHost(t *testing.T, db *database.DB, hostID string) {
	t.Helper()
	if err := db.RegisterHost(context.Background(), &models.Host{ID: hostID, Name: hostID, Hostname: hostID, Status: "online"}); err != nil {
		t.Fatalf("register host %s: %v", hostID, err)
	}
}

// TestGetBandwidthRateBytesPerSec exercises the MAX-MIN counter-delta
// primitive behind the bandwidth_vs_rolling_avg alert metric directly against
// a real Postgres/TimescaleDB instance.
func TestGetBandwidthRateBytesPerSec(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()
	now := time.Now()

	t.Run("computes rate from two samples", func(t *testing.T) {
		h := "bandwidth-host-basic"
		mustRegisterHost(t, db, h)
		insertBandwidthSample(t, db, h, 1_000_000, 500_000, now.Add(-240*time.Second))
		insertBandwidthSample(t, db, h, 1_600_000, 800_000, now)

		rate, ok := db.GetBandwidthRateBytesPerSec(ctx, h, 300)
		if !ok {
			t.Fatal("expected ok=true with two samples in window")
		}
		// (600_000 + 300_000) / 240s = 3750 bytes/sec.
		want := 900_000.0 / 240.0
		if math.Abs(rate-want) > 1.0 {
			t.Errorf("rate = %v, want ~%v", rate, want)
		}
	})

	t.Run("insufficient samples returns not ok", func(t *testing.T) {
		h := "bandwidth-host-single"
		mustRegisterHost(t, db, h)
		insertBandwidthSample(t, db, h, 1_000_000, 500_000, now)

		if _, ok := db.GetBandwidthRateBytesPerSec(ctx, h, 300); ok {
			t.Error("expected ok=false with only one sample in window")
		}
	})

	t.Run("no samples returns not ok", func(t *testing.T) {
		h := "bandwidth-host-empty"
		mustRegisterHost(t, db, h)

		if _, ok := db.GetBandwidthRateBytesPerSec(ctx, h, 300); ok {
			t.Error("expected ok=false with no samples at all")
		}
	})

	t.Run("counter reset guards against a bogus rate", func(t *testing.T) {
		h := "bandwidth-host-reset"
		mustRegisterHost(t, db, h)
		// rx goes backward (agent restart zeroing its counters).
		insertBandwidthSample(t, db, h, 2_000_000, 500_000, now.Add(-240*time.Second))
		insertBandwidthSample(t, db, h, 100_000, 800_000, now)

		if _, ok := db.GetBandwidthRateBytesPerSec(ctx, h, 300); ok {
			t.Error("expected ok=false when the rx counter goes backward within the window")
		}
	})
}
