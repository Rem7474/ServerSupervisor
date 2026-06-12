package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Uptime Probes ==========

func (db *DB) CreateUptimeProbe(ctx context.Context, p models.UptimeProbe) (*models.UptimeProbe, error) {
	var out models.UptimeProbe
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO uptime_probes
		 (name, type, target, interval_sec, timeout_sec, expected_status, expected_body_regex,
		  follow_redirects, verify_tls, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 RETURNING id, name, type, target, interval_sec, timeout_sec, expected_status, expected_body_regex,
		           follow_redirects, verify_tls, enabled, last_status, last_latency_ms, last_status_code,
		           last_error, last_checked_at, consecutive_failures, created_at, updated_at`,
		p.Name, p.Type, p.Target, p.IntervalSec, p.TimeoutSec, p.ExpectedStatus, p.ExpectedBodyRegex,
		p.FollowRedirects, p.VerifyTLS, p.Enabled,
	).Scan(
		&out.ID, &out.Name, &out.Type, &out.Target, &out.IntervalSec, &out.TimeoutSec,
		&out.ExpectedStatus, &out.ExpectedBodyRegex, &out.FollowRedirects, &out.VerifyTLS, &out.Enabled,
		&out.LastStatus, &out.LastLatencyMs, &out.LastStatusCode, &out.LastError, &out.LastCheckedAt,
		&out.ConsecutiveFailures, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (db *DB) ListUptimeProbes(ctx context.Context) ([]models.UptimeProbe, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, type, target, interval_sec, timeout_sec, expected_status, expected_body_regex,
		        follow_redirects, verify_tls, enabled, last_status, last_latency_ms, last_status_code,
		        last_error, last_checked_at, consecutive_failures, created_at, updated_at
		 FROM uptime_probes
		 ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.UptimeProbe
	for rows.Next() {
		var p models.UptimeProbe
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Type, &p.Target, &p.IntervalSec, &p.TimeoutSec,
			&p.ExpectedStatus, &p.ExpectedBodyRegex, &p.FollowRedirects, &p.VerifyTLS, &p.Enabled,
			&p.LastStatus, &p.LastLatencyMs, &p.LastStatusCode, &p.LastError, &p.LastCheckedAt,
			&p.ConsecutiveFailures, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (db *DB) GetUptimeProbe(ctx context.Context, id string) (*models.UptimeProbe, error) {
	var p models.UptimeProbe
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, name, type, target, interval_sec, timeout_sec, expected_status, expected_body_regex,
		        follow_redirects, verify_tls, enabled, last_status, last_latency_ms, last_status_code,
		        last_error, last_checked_at, consecutive_failures, created_at, updated_at
		 FROM uptime_probes WHERE id = $1`, id,
	).Scan(
		&p.ID, &p.Name, &p.Type, &p.Target, &p.IntervalSec, &p.TimeoutSec,
		&p.ExpectedStatus, &p.ExpectedBodyRegex, &p.FollowRedirects, &p.VerifyTLS, &p.Enabled,
		&p.LastStatus, &p.LastLatencyMs, &p.LastStatusCode, &p.LastError, &p.LastCheckedAt,
		&p.ConsecutiveFailures, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (db *DB) UpdateUptimeProbe(ctx context.Context, p models.UptimeProbe) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE uptime_probes
		 SET name=$1, type=$2, target=$3, interval_sec=$4, timeout_sec=$5,
		     expected_status=$6, expected_body_regex=$7, follow_redirects=$8, verify_tls=$9, enabled=$10,
		     updated_at=NOW()
		 WHERE id=$11`,
		p.Name, p.Type, p.Target, p.IntervalSec, p.TimeoutSec,
		p.ExpectedStatus, p.ExpectedBodyRegex, p.FollowRedirects, p.VerifyTLS, p.Enabled, p.ID,
	)
	return err
}

func (db *DB) DeleteUptimeProbe(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM uptime_probes WHERE id = $1`, id)
	return err
}

// ListEnabledUptimeProbesDue returns probes whose interval has elapsed since last_checked_at.
// Used by the worker to pick which probes to run on each tick.
func (db *DB) ListEnabledUptimeProbesDue(ctx context.Context) ([]models.UptimeProbe, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, type, target, interval_sec, timeout_sec, expected_status, expected_body_regex,
		        follow_redirects, verify_tls, enabled, last_status, last_latency_ms, last_status_code,
		        last_error, last_checked_at, consecutive_failures, created_at, updated_at
		 FROM uptime_probes
		 WHERE enabled = TRUE
		   AND (last_checked_at IS NULL
		        OR last_checked_at < NOW() - (interval_sec || ' seconds')::interval)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.UptimeProbe
	for rows.Next() {
		var p models.UptimeProbe
		if err := rows.Scan(
			&p.ID, &p.Name, &p.Type, &p.Target, &p.IntervalSec, &p.TimeoutSec,
			&p.ExpectedStatus, &p.ExpectedBodyRegex, &p.FollowRedirects, &p.VerifyTLS, &p.Enabled,
			&p.LastStatus, &p.LastLatencyMs, &p.LastStatusCode, &p.LastError, &p.LastCheckedAt,
			&p.ConsecutiveFailures, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecordUptimeProbeResult inserts a result row and updates the cached probe state.
func (db *DB) RecordUptimeProbeResult(ctx context.Context, r models.UptimeProbeResult) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO uptime_probe_results (probe_id, checked_at, success, status_code, latency_ms, error)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		r.ProbeID, r.CheckedAt, r.Success, r.StatusCode, r.LatencyMs, r.Error,
	); err != nil {
		return err
	}

	status := "down"
	if r.Success {
		status = "up"
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE uptime_probes
		 SET last_status = $1,
		     last_latency_ms = $2,
		     last_status_code = $3,
		     last_error = $4,
		     last_checked_at = $5,
		     consecutive_failures = CASE WHEN $6 THEN 0 ELSE consecutive_failures + 1 END,
		     updated_at = NOW()
		 WHERE id = $7`,
		status, r.LatencyMs, r.StatusCode, r.Error, r.CheckedAt, r.Success, r.ProbeID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// GetUptimeProbeResults returns recent results for a probe, newest first.
func (db *DB) GetUptimeProbeResults(ctx context.Context, probeID string, limit int) ([]models.UptimeProbeResult, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, probe_id, checked_at, success, status_code, latency_ms, error
		 FROM uptime_probe_results
		 WHERE probe_id = $1
		 ORDER BY checked_at DESC
		 LIMIT $2`, probeID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.UptimeProbeResult
	for rows.Next() {
		var r models.UptimeProbeResult
		var statusCode sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ProbeID, &r.CheckedAt, &r.Success, &statusCode, &r.LatencyMs, &r.Error); err != nil {
			return nil, err
		}
		if statusCode.Valid {
			v := int(statusCode.Int64)
			r.StatusCode = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetUptimeStats aggregates uptime over the given window (hours).
func (db *DB) GetUptimeStats(ctx context.Context, probeID string, windowHours int) (*models.UptimeStats, error) {
	if windowHours <= 0 {
		windowHours = 24
	}
	stats := &models.UptimeStats{WindowHours: windowHours}
	var avgLatency sql.NullFloat64
	var p95Latency sql.NullFloat64
	err := db.conn.QueryRowContext(ctx,
		`SELECT
		    COUNT(*) AS total,
		    COUNT(*) FILTER (WHERE success) AS ok,
		    AVG(latency_ms) FILTER (WHERE success) AS avg_lat,
		    percentile_disc(0.95) WITHIN GROUP (ORDER BY latency_ms) FILTER (WHERE success) AS p95
		 FROM uptime_probe_results
		 WHERE probe_id = $1
		   AND checked_at >= NOW() - ($2 || ' hours')::interval`,
		probeID, windowHours,
	).Scan(&stats.TotalChecks, &stats.SuccessfulChecks, &avgLatency, &p95Latency)
	if err != nil {
		return nil, err
	}
	if stats.TotalChecks > 0 {
		stats.UptimePercent = float64(stats.SuccessfulChecks) * 100 / float64(stats.TotalChecks)
	}
	if avgLatency.Valid {
		stats.AvgLatencyMs = avgLatency.Float64
	}
	if p95Latency.Valid {
		stats.P95LatencyMs = int(p95Latency.Float64)
	}
	return stats, nil
}

// CountDownProbes returns how many enabled probes are currently in the "down" state.
// Used by the alert engine for the global "uptime_down_count" metric.
func (db *DB) CountDownProbes(ctx context.Context) (int, error) {
	var n int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM uptime_probes WHERE enabled = TRUE AND last_status = 'down'`,
	).Scan(&n)
	return n, err
}

// CleanupOldUptimeResults removes result rows older than the given age. Returns rows deleted.
func (db *DB) CleanupOldUptimeResults(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM uptime_probe_results WHERE checked_at < NOW() - ($1 || ' seconds')::interval`,
		int(olderThan.Seconds()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// SetUptimeProbeEnabled toggles the enabled flag on a single uptime probe.
func (db *DB) SetUptimeProbeEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE uptime_probes SET enabled=$2, updated_at=NOW() WHERE id=$1`, id, enabled)
	return err
}
