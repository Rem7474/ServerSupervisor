package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Network Flow Metrics ("top talkers") ==========

// InsertNetworkFlowMetrics persists one report cycle's top-N talkers plus its
// "others" rollup row, if present. No-op when the report is unavailable
// (degraded collection) or genuinely empty for this cycle. Up to ~51 rows per
// call (agent-side top-N cutoff bounds this regardless of the host's real
// connection count) — a simple per-row insert is enough at that volume; COPY
// is the natural next lever if this ever becomes a hot path.
func (db *DB) InsertNetworkFlowMetrics(ctx context.Context, hostID string, report *models.NetworkFlowsReport) error {
	if report == nil || !report.Available {
		return nil
	}
	if len(report.TopTalkers) == 0 && report.Others == nil {
		return nil
	}
	ts := report.CollectedAt
	if ts.IsZero() {
		ts = time.Now()
	}
	for _, t := range report.TopTalkers {
		_, err := db.conn.ExecContext(ctx,
			`INSERT INTO network_flow_metrics (
				host_id, timestamp, is_others, remote_ip, remote_port, protocol, direction,
				process_name, pid, rx_bytes, tx_bytes, packets, connections
			) VALUES ($1, $2, false, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			hostID, ts, t.RemoteIP, t.RemotePort, t.Protocol, t.Direction,
			t.ProcessName, t.PID, t.RxBytes, t.TxBytes, t.Packets, t.Connections,
		)
		if err != nil {
			return fmt.Errorf("failed to insert network flow talker: %w", err)
		}
	}
	if report.Others != nil {
		_, err := db.conn.ExecContext(ctx,
			`INSERT INTO network_flow_metrics (
				host_id, timestamp, is_others, rx_bytes, tx_bytes, connections
			) VALUES ($1, $2, true, $3, $4, $5)`,
			hostID, ts, report.Others.RxBytes, report.Others.TxBytes, report.Others.Connections,
		)
		if err != nil {
			return fmt.Errorf("failed to insert network flow others bucket: %w", err)
		}
	}
	return nil
}

// GetLatestNetworkFlowMetrics returns the most recent report cycle's talkers
// for a host (the "others" row, if any, sorted last), busiest first.
func (db *DB) GetLatestNetworkFlowMetrics(ctx context.Context, hostID string) ([]models.NetworkFlowMetric, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, host_id, timestamp, is_others, remote_ip, remote_port, protocol, direction,
			process_name, pid, rx_bytes, tx_bytes, packets, connections
		FROM network_flow_metrics
		WHERE host_id = $1
		  AND timestamp = (
			SELECT MAX(timestamp) FROM network_flow_metrics WHERE host_id = $1
		  )
		ORDER BY is_others ASC, (rx_bytes + tx_bytes) DESC`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanNetworkFlowMetrics(rows)
}

func scanNetworkFlowMetrics(rows *sql.Rows) ([]models.NetworkFlowMetric, error) {
	var metrics []models.NetworkFlowMetric
	for rows.Next() {
		var m models.NetworkFlowMetric
		if err := rows.Scan(
			&m.ID, &m.HostID, &m.Timestamp, &m.IsOthers, &m.RemoteIP, &m.RemotePort, &m.Protocol, &m.Direction,
			&m.ProcessName, &m.PID, &m.RxBytes, &m.TxBytes, &m.Packets, &m.Connections,
		); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// historyBucketInterval picks a coarser bucket for longer ranges, same
// adaptive-granularity intent as GetDiskMetricsAggregated (raw/hour/day).
// Takes the actual span (until-since) rather than a named preset so it works
// identically for a preset ("last 24h") and a precise custom range that has
// no preset name to key off of.
func historyBucketInterval(span time.Duration) string {
	switch {
	case span <= 6*time.Hour:
		return "1 minute"
	case span <= 48*time.Hour:
		return "15 minutes"
	default:
		return "1 hour"
	}
}

// GetNetworkFlowsHistory returns one talker's bandwidth over time, bucketed
// and SUMmed (not MAX-MIN: rx_bytes/tx_bytes are already per-cycle deltas —
// see models.NetworkFlowTalker). Used for the per-talker drill-down chart.
// until being zero means "open ended" (no upper bound), same convention as
// buildWebLogsWhere.
func (db *DB) GetNetworkFlowsHistory(ctx context.Context, hostID, remoteIP string, remotePort int, protocol string, since, until time.Time) ([]models.NetworkFlowSummaryPoint, error) {
	effectiveUntil := until
	if effectiveUntil.IsZero() {
		effectiveUntil = time.Now()
	}

	args := []any{historyBucketInterval(effectiveUntil.Sub(since)), hostID, remoteIP, remotePort, protocol, since}
	where := "host_id = $2 AND is_others = false AND remote_ip = $3 AND remote_port = $4 AND protocol = $5 AND timestamp > $6"
	if !until.IsZero() {
		args = append(args, until)
		where += fmt.Sprintf(" AND timestamp <= $%d", len(args))
	}

	rows, err := db.conn.QueryContext(ctx,
		fmt.Sprintf(`SELECT time_bucket($1::interval, timestamp) AS bucket,
			COALESCE(SUM(rx_bytes), 0), COALESCE(SUM(tx_bytes), 0)
		FROM network_flow_metrics
		WHERE %s
		GROUP BY bucket
		ORDER BY bucket ASC`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanNetworkFlowSummary(rows)
}

// GetNetworkFlowsSummary returns a host's total tracked bandwidth over time
// (every talker plus the "others" bucket summed together), for the overview
// chart. until being zero means "open ended".
func (db *DB) GetNetworkFlowsSummary(ctx context.Context, hostID string, since, until time.Time) ([]models.NetworkFlowSummaryPoint, error) {
	effectiveUntil := until
	if effectiveUntil.IsZero() {
		effectiveUntil = time.Now()
	}

	args := []any{historyBucketInterval(effectiveUntil.Sub(since)), hostID, since}
	where := "host_id = $2 AND timestamp > $3"
	if !until.IsZero() {
		args = append(args, until)
		where += fmt.Sprintf(" AND timestamp <= $%d", len(args))
	}

	rows, err := db.conn.QueryContext(ctx,
		fmt.Sprintf(`SELECT time_bucket($1::interval, timestamp) AS bucket,
			COALESCE(SUM(rx_bytes), 0), COALESCE(SUM(tx_bytes), 0)
		FROM network_flow_metrics
		WHERE %s
		GROUP BY bucket
		ORDER BY bucket ASC`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanNetworkFlowSummary(rows)
}

func scanNetworkFlowSummary(rows *sql.Rows) ([]models.NetworkFlowSummaryPoint, error) {
	var points []models.NetworkFlowSummaryPoint
	for rows.Next() {
		var p models.NetworkFlowSummaryPoint
		if err := rows.Scan(&p.Timestamp, &p.RxBytes, &p.TxBytes); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// CleanOldNetworkFlowMetrics deletes rows older than the given retention
// window. remote_ip is a potentially identifying value, so this is driven by
// an applicative job (internal/background/network_flows.go) on a short
// default, not a fixed TimescaleDB retention policy — see migration 092.
func (db *DB) CleanOldNetworkFlowMetrics(ctx context.Context, days int) (int64, error) {
	if days <= 0 {
		days = 14
	}
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM network_flow_metrics WHERE "timestamp" < NOW() - ($1 || ' days')::INTERVAL`, days)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
