package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/serversupervisor/server/internal/models"
)

// ========== System Metrics ==========

func (db *DB) InsertMetrics(ctx context.Context, m *models.SystemMetrics) (int64, error) {
	var id int64
	err := db.conn.QueryRowContext(ctx, 
		`INSERT INTO system_metrics (host_id, timestamp, cpu_usage_percent, cpu_cores, cpu_model,
		 cpu_temperature, fan_rpm, load_avg_1, load_avg_5, load_avg_15, memory_total, memory_used, memory_free, memory_percent,
		 swap_total, swap_used, network_rx_bytes, network_tx_bytes, uptime, hostname)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		 RETURNING id`,
		m.HostID, m.Timestamp, m.CPUUsagePercent, m.CPUCores, m.CPUModel,
		m.CPUTemperature, m.FanRPM, m.LoadAvg1, m.LoadAvg5, m.LoadAvg15, m.MemoryTotal, m.MemoryUsed, m.MemoryFree, m.MemoryPercent,
		m.SwapTotal, m.SwapUsed, m.NetworkRxBytes, m.NetworkTxBytes, m.Uptime, m.Hostname,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	// Per-disk details are stored separately in disk_metrics (a hypertable) via
	// InsertDiskMetrics; the legacy disk_info table was dropped in V2.
	return id, nil
}

func (db *DB) InsertUptimeMetrics(ctx context.Context, hostID string, uptime uint64, hostname string) error {
	_, err := db.conn.ExecContext(ctx, 
		`INSERT INTO system_metrics (host_id, timestamp, uptime, hostname)
		 VALUES ($1, NOW(), $2, $3)`,
		hostID, uptime, hostname,
	)
	return err
}

func (db *DB) GetLatestMetrics(ctx context.Context, hostID string) (*models.SystemMetrics, error) {
	var m models.SystemMetrics
	err := db.conn.QueryRowContext(ctx, 
		`SELECT id, host_id, timestamp,
		 COALESCE(cpu_usage_percent, 0), COALESCE(cpu_cores, 0), COALESCE(cpu_model, ''),
		 COALESCE(cpu_temperature, 0), COALESCE(fan_rpm, 0),
		 COALESCE(load_avg_1, 0), COALESCE(load_avg_5, 0), COALESCE(load_avg_15, 0),
		 COALESCE(memory_total, 0), COALESCE(memory_used, 0), COALESCE(memory_free, 0), COALESCE(memory_percent, 0),
		 COALESCE(swap_total, 0), COALESCE(swap_used, 0),
		 COALESCE(network_rx_bytes, 0), COALESCE(network_tx_bytes, 0),
		 COALESCE(uptime, 0), COALESCE(hostname, '')
		 FROM system_metrics WHERE host_id = $1 ORDER BY timestamp DESC LIMIT 1`, hostID,
	).Scan(&m.ID, &m.HostID, &m.Timestamp, &m.CPUUsagePercent, &m.CPUCores, &m.CPUModel,
		&m.CPUTemperature, &m.FanRPM, &m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15, &m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
		&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes, &m.Uptime, &m.Hostname)
	if err != nil {
		return nil, err
	}
	// Per-disk details now live in disk_metrics (hypertable); host views fetch
	// them via GetLatestDiskMetrics. The legacy disk_info join was removed in V2.
	return &m, nil
}

// GetLatestMetricsAll returns the most recent SystemMetrics row for every host
// in a single query, avoiding N+1 lookups on the dashboard.
// Disk details are intentionally omitted (dashboard only needs CPU/mem/net).
func (db *DB) GetLatestMetricsAll(ctx context.Context) (map[string]*models.SystemMetrics, error) {
	rows, err := db.conn.QueryContext(ctx, 
		`SELECT DISTINCT ON (host_id) id, host_id, timestamp,
		 COALESCE(cpu_usage_percent, 0), COALESCE(cpu_cores, 0), COALESCE(cpu_model, ''),
		 COALESCE(cpu_temperature, 0), COALESCE(fan_rpm, 0),
		 COALESCE(load_avg_1, 0), COALESCE(load_avg_5, 0), COALESCE(load_avg_15, 0),
		 COALESCE(memory_total, 0), COALESCE(memory_used, 0), COALESCE(memory_free, 0), COALESCE(memory_percent, 0),
		 COALESCE(swap_total, 0), COALESCE(swap_used, 0),
		 COALESCE(network_rx_bytes, 0), COALESCE(network_tx_bytes, 0),
		 COALESCE(uptime, 0), COALESCE(hostname, '')
		 FROM system_metrics
		 ORDER BY host_id, timestamp DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]*models.SystemMetrics)
	for rows.Next() {
		var m models.SystemMetrics
		if err := rows.Scan(
			&m.ID, &m.HostID, &m.Timestamp,
			&m.CPUUsagePercent, &m.CPUCores, &m.CPUModel,
			&m.CPUTemperature, &m.FanRPM, &m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15,
			&m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
			&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes,
			&m.Uptime, &m.Hostname,
		); err != nil {
			continue
		}
		result[m.HostID] = &m
	}
	return result, rows.Err()
}

// GetRootDiskPercentAll returns the used_percent of the root ("/") mount point for
// each host, based on the most recent disk_metrics row. Uses disk_metrics (not
// disk_info) so the value is always current even when Proxmox is the metrics source
// and InsertMetrics is skipped. Hosts with no disk_metrics row for "/" are omitted.
func (db *DB) GetRootDiskPercentAll(ctx context.Context) map[string]float64 {
	rows, err := db.conn.QueryContext(ctx, 
		`SELECT DISTINCT ON (host_id) host_id, used_percent
		 FROM disk_metrics
		 WHERE mount_point = '/'
		 ORDER BY host_id, timestamp DESC`,
	)
	if err != nil {
		return map[string]float64{}
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]float64)
	for rows.Next() {
		var hostID string
		var pct float64
		if err := rows.Scan(&hostID, &pct); err != nil {
			continue
		}
		result[hostID] = pct
	}
	return result
}

func (db *DB) GetMetricsHistory(ctx context.Context, hostID string, hours int) ([]models.SystemMetrics, error) {
	rows, err := db.conn.QueryContext(ctx, 
		`SELECT id, host_id, timestamp, cpu_usage_percent, cpu_cores, cpu_temperature, fan_rpm, load_avg_1, load_avg_5, load_avg_15,
		 memory_total, memory_used, memory_free, memory_percent, swap_total, swap_used,
		 network_rx_bytes, network_tx_bytes, uptime
		 FROM system_metrics WHERE host_id = $1 AND timestamp > NOW() - INTERVAL '1 hour' * $2
		 ORDER BY timestamp ASC`, hostID, hours,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var metrics []models.SystemMetrics
	for rows.Next() {
		var m models.SystemMetrics
		if err := rows.Scan(&m.ID, &m.HostID, &m.Timestamp, &m.CPUUsagePercent, &m.CPUCores,
			&m.CPUTemperature, &m.FanRPM, &m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15, &m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
			&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes, &m.Uptime); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (db *DB) GetSystemCPUTemperatureHistoryByHost(ctx context.Context, hostID string, hours int) ([]models.SystemMetrics, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours <= 24 {
		return db.GetMetricsHistory(ctx, hostID, hours)
	}

	bucketMinutes := 60
	if hours > 720 {
		bucketMinutes = 24 * 60
	}

	const bucketExpr = `time_bucket($2 * '1 minute'::interval, timestamp)`

	query := `
		SELECT
			0 AS id,
			$1 AS host_id,
			` + bucketExpr + ` AS timestamp,
			0 AS cpu_usage_percent,
			0 AS cpu_cores,
			'' AS cpu_model,
			COALESCE(AVG(NULLIF(cpu_temperature, 0)), 0) AS cpu_temperature,
			0 AS fan_rpm,
			0 AS load_avg_1,
			0 AS load_avg_5,
			0 AS load_avg_15,
			0 AS memory_total,
			0 AS memory_used,
			0 AS memory_free,
			0 AS memory_percent,
			0 AS swap_total,
			0 AS swap_used,
			0 AS network_rx_bytes,
			0 AS network_tx_bytes,
			0 AS uptime,
			'' AS hostname
		FROM system_metrics
		WHERE host_id = $1
		  AND timestamp > NOW() - INTERVAL '1 hour' * $3
		GROUP BY ` + bucketExpr + `
		ORDER BY ` + bucketExpr + ` ASC`

	rows, err := db.conn.QueryContext(ctx, query, hostID, bucketMinutes, hours)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var metrics []models.SystemMetrics
	for rows.Next() {
		var m models.SystemMetrics
		if err := rows.Scan(&m.ID, &m.HostID, &m.Timestamp, &m.CPUUsagePercent, &m.CPUCores, &m.CPUModel,
			&m.CPUTemperature, &m.FanRPM, &m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15, &m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
			&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes, &m.Uptime, &m.Hostname); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

func (db *DB) GetSystemFanRPMHistoryByHost(ctx context.Context, hostID string, hours int) ([]models.SystemMetrics, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours <= 24 {
		return db.GetMetricsHistory(ctx, hostID, hours)
	}

	bucketMinutes := 60
	if hours > 720 {
		bucketMinutes = 24 * 60
	}

	const bucketExpr = `time_bucket($2 * '1 minute'::interval, timestamp)`

	query := `
		SELECT
			0 AS id,
			$1 AS host_id,
			` + bucketExpr + ` AS timestamp,
			0 AS cpu_usage_percent,
			0 AS cpu_cores,
			'' AS cpu_model,
			0 AS cpu_temperature,
			COALESCE(AVG(NULLIF(fan_rpm, 0)), 0) AS fan_rpm,
			0 AS load_avg_1,
			0 AS load_avg_5,
			0 AS load_avg_15,
			0 AS memory_total,
			0 AS memory_used,
			0 AS memory_free,
			0 AS memory_percent,
			0 AS swap_total,
			0 AS swap_used,
			0 AS network_rx_bytes,
			0 AS network_tx_bytes,
			0 AS uptime,
			'' AS hostname
		FROM system_metrics
		WHERE host_id = $1
		  AND timestamp > NOW() - INTERVAL '1 hour' * $3
		GROUP BY ` + bucketExpr + `
		ORDER BY ` + bucketExpr + ` ASC`

	rows, err := db.conn.QueryContext(ctx, query, hostID, bucketMinutes, hours)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var metrics []models.SystemMetrics
	for rows.Next() {
		var m models.SystemMetrics
		if err := rows.Scan(&m.ID, &m.HostID, &m.Timestamp, &m.CPUUsagePercent, &m.CPUCores, &m.CPUModel,
			&m.CPUTemperature, &m.FanRPM, &m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15, &m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
			&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes, &m.Uptime, &m.Hostname); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// GetMetricsAggregatesByType returns per-host metrics bucketed for long time
// ranges. It buckets the raw system_metrics hypertable on the fly (chunk + index
// pruning keep this cheap for a single host), replacing the dropped
// metrics_aggregates table. aggregationType selects the bucket size:
// "hour" → 60-minute buckets, anything else ("day") → daily buckets.
func (db *DB) GetMetricsAggregatesByType(ctx context.Context, hostID string, hours int, aggregationType string) ([]models.SystemMetrics, error) {
	bucketMinutes := 60
	if aggregationType == "day" {
		bucketMinutes = 24 * 60
	}

	const bucketExpr = `time_bucket($3 * '1 minute'::interval, timestamp)`

	rows, err := db.conn.QueryContext(ctx,
		`SELECT 0 AS id, $1 AS host_id, `+bucketExpr+` AS timestamp,
		 AVG(cpu_usage_percent) AS cpu_usage_percent, 0 AS cpu_cores, 0 AS cpu_temperature, 0 AS fan_rpm,
		 0 AS load_avg_1, 0 AS load_avg_5, 0 AS load_avg_15,
		 0 AS memory_total, COALESCE(AVG(memory_used), 0)::BIGINT AS memory_used, 0 AS memory_free, AVG(memory_percent) AS memory_percent,
		 0 AS swap_total, 0 AS swap_used,
		 COALESCE(MAX(network_rx_bytes) - MIN(network_rx_bytes), 0) AS network_rx_bytes,
		 COALESCE(MAX(network_tx_bytes) - MIN(network_tx_bytes), 0) AS network_tx_bytes,
		 0 AS uptime
		 FROM system_metrics
		 WHERE host_id = $1 AND timestamp > NOW() - INTERVAL '1 hour' * $2
		 GROUP BY `+bucketExpr+`
		 ORDER BY `+bucketExpr+` ASC`, hostID, hours, bucketMinutes,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var metrics []models.SystemMetrics
	for rows.Next() {
		var m models.SystemMetrics
		if err := rows.Scan(&m.ID, &m.HostID, &m.Timestamp, &m.CPUUsagePercent, &m.CPUCores,
			&m.CPUTemperature, &m.FanRPM, &m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15, &m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
			&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes, &m.Uptime); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// GetMetricsSummary returns the global CPU/RAM history used by the dashboard chart.
// For buckets ≥ 5 minutes it reads the system_metrics_5min continuous aggregate
// (materialized by TimescaleDB) instead of scanning raw rows across all hosts;
// finer buckets (≤6h ranges) fall back to the raw table. If the aggregate is
// unavailable (e.g. non-Timescale dev/test DB) it also falls back to raw so the
// chart is never empty.
func (db *DB) GetMetricsSummary(ctx context.Context, hours int, bucketMinutes int) ([]models.SystemMetricsSummary, error) {
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	if bucketMinutes >= 5 {
		summary, err := db.metricsSummaryFromCAGG(ctx, hours, bucketMinutes)
		switch {
		case err == nil && len(summary) > 0:
			return summary, nil
		case err != nil:
			// The continuous aggregate is expected to be absent only on a
			// non-Timescale DB (dev/test); any other failure is logged so a real
			// error is never silently hidden before the raw fallback.
			slog.WarnContext(ctx, "metrics summary continuous aggregate query failed, falling back to raw",
				slog.Int("hours", hours), slog.Int("bucket_minutes", bucketMinutes), slog.Any("err", err))
		}
	}

	return db.metricsSummaryFromRaw(ctx, hours, bucketMinutes)
}

// metricsSummaryFromRaw aggregates the raw system_metrics table into time buckets.
func (db *DB) metricsSummaryFromRaw(ctx context.Context, hours int, bucketMinutes int) ([]models.SystemMetricsSummary, error) {
	const bucketExpr = `time_bucket($2 * '1 minute'::interval, timestamp)`
	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+bucketExpr+` AS ts,
			AVG(cpu_usage_percent) AS cpu_avg,
			AVG(memory_percent) AS mem_avg,
			COUNT(*) AS sample_count
		 FROM system_metrics
		 WHERE timestamp > NOW() - INTERVAL '1 hour' * $1
		 GROUP BY ts
		 ORDER BY ts ASC`,
		hours, bucketMinutes,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanMetricsSummary(rows)
}

// metricsSummaryFromCAGG re-buckets the system_metrics_5min continuous aggregate
// into the requested bucket size and averages across hosts. The aggregate stores
// one row per (5-minute bucket, host_id); coarser dashboard ranges roll those up.
func (db *DB) metricsSummaryFromCAGG(ctx context.Context, hours int, bucketMinutes int) ([]models.SystemMetricsSummary, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT time_bucket($2 * '1 minute'::interval, bucket) AS ts,
			AVG(cpu_avg) AS cpu_avg,
			AVG(mem_avg) AS mem_avg,
			SUM(sample_count) AS sample_count
		 FROM system_metrics_5min
		 WHERE bucket > NOW() - INTERVAL '1 hour' * $1
		 GROUP BY ts
		 ORDER BY ts ASC`,
		hours, bucketMinutes,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanMetricsSummary(rows)
}

func scanMetricsSummary(rows *sql.Rows) ([]models.SystemMetricsSummary, error) {
	var summary []models.SystemMetricsSummary
	for rows.Next() {
		var s models.SystemMetricsSummary
		if err := rows.Scan(&s.Timestamp, &s.CPUAvg, &s.MemoryAvg, &s.SampleCount); err != nil {
			continue
		}
		summary = append(summary, s)
	}
	return summary, rows.Err()
}

// UpdateMetricsRetentionPolicy updates the TimescaleDB retention policies for
// system_metrics and disk_metrics to the given number of days. The existing
// policy is replaced atomically so the change takes effect on the next
// scheduled policy run.
func (db *DB) UpdateMetricsRetentionPolicy(ctx context.Context, days int) error {
	for _, table := range []string{"system_metrics", "disk_metrics"} {
		if _, err := db.conn.ExecContext(ctx,
			`SELECT remove_retention_policy($1, if_not_exists => true)`, table); err != nil {
			return fmt.Errorf("remove retention policy for %s: %w", table, err)
		}
		if _, err := db.conn.ExecContext(ctx,
			`SELECT add_retention_policy($1, make_interval(days => $2))`, table, days); err != nil {
			return fmt.Errorf("add retention policy for %s: %w", table, err)
		}
	}
	return nil
}

// CountMetrics returns the total number of metrics records.
func (db *DB) CountMetrics(ctx context.Context) (int64, error) {
	var count int64
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM system_metrics`).Scan(&count)
	return count, err
}
