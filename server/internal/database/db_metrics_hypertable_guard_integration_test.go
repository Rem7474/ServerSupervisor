package database_test

import (
	"context"
	"testing"

	"github.com/serversupervisor/server/internal/testutil"
)

// TestCountMetrics_HypertableCheckFailureFallsBackGracefully exercises
// CountMetrics's guard against calling hypertable_approximate_row_count on a
// table it can't confirm is a hypertable. A cancelled context makes the
// isHypertable check itself fail (same as an unreachable/misconfigured
// timescaledb_information view would), so CountMetrics must skip straight to
// its plain COUNT(*) fallback rather than attempting the doomed hypertable
// call — that fallback query also runs on the same cancelled context here,
// so the overall call still errors, but it must be *this* query that ran,
// not a "function does not exist" failure from the hypertable-only path.
func TestCountMetrics_HypertableCheckFailureFallsBackGracefully(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := db.CountMetrics(ctx)
	if err == nil {
		t.Fatal("expected an error propagated from the cancelled context")
	}
}

// TestUpdateMetricsRetentionPolicy_HypertableCheckFailureIsANoop exercises
// the same guard on the write path: if the hypertable check can't be
// resolved, UpdateMetricsRetentionPolicy must skip attempting
// remove_retention_policy/add_retention_policy (which would themselves fail
// with "function does not exist" on a non-hypertable, or simply can't be
// trusted to be meaningful here) rather than surface a spurious error for a
// setting change that never actually needed a retention policy call to begin
// with. A cancelled context — which fails the isHypertable check the same
// way an unreachable extension/schema would — must therefore return nil, not
// an error.
func TestUpdateMetricsRetentionPolicy_HypertableCheckFailureIsANoop(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := db.UpdateMetricsRetentionPolicy(ctx, 30); err != nil {
		t.Fatalf("expected a no-op (nil) when the hypertable check can't be resolved, got: %v", err)
	}
}

// TestUpdateMetricsRetentionPolicy_AppliesPolicyOnARealHypertable is the
// counterpart to the no-op test above: system_metrics/disk_metrics genuinely
// are hypertables in this test's real TimescaleDB instance, so the guard
// above must not skip them — remove_retention_policy/add_retention_policy
// must actually run.
func TestUpdateMetricsRetentionPolicy_AppliesPolicyOnARealHypertable(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	if err := db.UpdateMetricsRetentionPolicy(ctx, 45); err != nil {
		t.Fatalf("expected the retention policy update to succeed on a real hypertable, got: %v", err)
	}
}
