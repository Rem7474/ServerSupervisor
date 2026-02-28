package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Metrics Aggregates (Downsampling) ==========

func (db *DB) InsertMetricsAggregate(agg *models.MetricsAggregate) error {
	_, err := db.conn.Exec(
		`INSERT INTO metrics_aggregates (host_id, aggregation_type, timestamp, cpu_usage_avg, cpu_usage_max,
		 memory_usage_avg, memory_usage_max, disk_usage_avg, network_rx_bytes, network_tx_bytes, sample_count)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 ON CONFLICT (host_id, aggregation_type, timestamp) DO NOTHING`,
		agg.HostID, agg.AggregationType, agg.Timestamp, agg.CPUUsageAvg, agg.CPUUsageMax,
		agg.MemoryUsageAvg, agg.MemoryUsageMax, agg.DiskUsageAvg, agg.NetworkRxBytes, agg.NetworkTxBytes, agg.SampleCount,
	)
	return err
}

func (db *DB) BuildMetricsAggregate(hostID string, start, end time.Time) (*models.MetricsAggregate, error) {
	var agg models.MetricsAggregate
	var sampleCount int
	var diskAvg sql.NullFloat64
	var rxDelta, txDelta sql.NullInt64
	var cpuAvg, cpuMax, memAvg, memMax sql.NullFloat64

	err := db.conn.QueryRow(
		`SELECT
			AVG(cpu_usage_percent) AS cpu_avg,
			MAX(cpu_usage_percent) AS cpu_max,
			AVG(memory_used) AS mem_avg,
			MAX(memory_used) AS mem_max,
			COUNT(*) AS sample_count,
			MAX(network_rx_bytes) - MIN(network_rx_bytes) AS rx_delta,
			MAX(network_tx_bytes) - MIN(network_tx_bytes) AS tx_delta
		 FROM system_metrics
		 WHERE host_id = $1 AND timestamp >= $2 AND timestamp < $3`,
		hostID, start, end,
	).Scan(&cpuAvg, &cpuMax, &memAvg, &memMax, &sampleCount, &rxDelta, &txDelta)
	if err != nil {
		return nil, err
	}
	if sampleCount == 0 {
		return nil, nil
	}

	err = db.conn.QueryRow(
		`SELECT AVG(di.used_percent)
		 FROM system_metrics sm
		 JOIN disk_info di ON di.metrics_id = sm.id
		 WHERE sm.host_id = $1 AND sm.timestamp >= $2 AND sm.timestamp < $3`,
		hostID, start, end,
	).Scan(&diskAvg)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if diskAvg.Valid {
		agg.DiskUsageAvg = diskAvg.Float64
	}
	if cpuAvg.Valid {
		agg.CPUUsageAvg = cpuAvg.Float64
	}
	if cpuMax.Valid {
		agg.CPUUsageMax = cpuMax.Float64
	}
	if memAvg.Valid {
		agg.MemoryUsageAvg = uint64(memAvg.Float64)
	}
	if memMax.Valid {
		agg.MemoryUsageMax = uint64(memMax.Float64)
	}
	if rxDelta.Valid {
		agg.NetworkRxBytes = uint64(rxDelta.Int64)
	}
	if txDelta.Valid {
		agg.NetworkTxBytes = uint64(txDelta.Int64)
	}

	agg.HostID = hostID
	agg.SampleCount = sampleCount
	return &agg, nil
}

func (db *DB) GetMetricsAggregates(hostID string, aggregationType string, limit int) ([]models.MetricsAggregate, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, aggregation_type, timestamp, cpu_usage_avg, cpu_usage_max,
		 memory_usage_avg, memory_usage_max, disk_usage_avg, network_rx_bytes, network_tx_bytes, sample_count, created_at
		 FROM metrics_aggregates WHERE host_id = $1 AND aggregation_type = $2
		 ORDER BY timestamp DESC LIMIT $3`,
		hostID, aggregationType, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggs []models.MetricsAggregate
	for rows.Next() {
		var agg models.MetricsAggregate
		if err := rows.Scan(&agg.ID, &agg.HostID, &agg.AggregationType, &agg.Timestamp, &agg.CPUUsageAvg, &agg.CPUUsageMax,
			&agg.MemoryUsageAvg, &agg.MemoryUsageMax, &agg.DiskUsageAvg, &agg.NetworkRxBytes, &agg.NetworkTxBytes, &agg.SampleCount, &agg.CreatedAt); err != nil {
			continue
		}
		aggs = append(aggs, agg)
	}
	return aggs, nil
}

// DeleteOldMetrics deletes raw metrics older than retentionDays for a specific host.
func (db *DB) DeleteOldMetrics(hostID string, retentionDays int) error {
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	_, err := db.conn.Exec(
		`DELETE FROM system_metrics WHERE host_id = $1 AND timestamp < $2`,
		hostID, cutoffTime,
	)
	return err
}

// BatchAggregateMetrics inserts aggregate rows for ALL hosts in a single SQL statement,
// replacing the per-host N+1 loop that called BuildMetricsAggregate + InsertMetricsAggregate.
// Returns the number of hosts for which aggregates were written.
func (db *DB) BatchAggregateMetrics(start, end time.Time, aggType string) (int, error) {
	result, err := db.conn.Exec(`
		WITH disk_avgs AS (
			SELECT sm.host_id, AVG(di.used_percent) AS avg_disk
			FROM system_metrics sm
			JOIN disk_info di ON di.metrics_id = sm.id
			WHERE sm.timestamp >= $1 AND sm.timestamp < $2
			GROUP BY sm.host_id
		)
		INSERT INTO metrics_aggregates (
			host_id, aggregation_type, timestamp,
			cpu_usage_avg, cpu_usage_max,
			memory_usage_avg, memory_usage_max,
			disk_usage_avg,
			network_rx_bytes, network_tx_bytes,
			sample_count
		)
		SELECT
			sm.host_id,
			$3 AS aggregation_type,
			$1 AS timestamp,
			COALESCE(AVG(sm.cpu_usage_percent), 0),
			COALESCE(MAX(sm.cpu_usage_percent), 0),
			COALESCE(AVG(sm.memory_used)::BIGINT, 0),
			COALESCE(MAX(sm.memory_used)::BIGINT, 0),
			COALESCE(da.avg_disk, 0),
			COALESCE(MAX(sm.network_rx_bytes) - MIN(sm.network_rx_bytes), 0),
			COALESCE(MAX(sm.network_tx_bytes) - MIN(sm.network_tx_bytes), 0),
			COUNT(*)
		FROM system_metrics sm
		LEFT JOIN disk_avgs da ON da.host_id = sm.host_id
		WHERE sm.timestamp >= $1 AND sm.timestamp < $2
		GROUP BY sm.host_id, da.avg_disk
		HAVING COUNT(*) > 0
		ON CONFLICT (host_id, aggregation_type, timestamp) DO NOTHING`,
		start, end, aggType,
	)
	if err != nil {
		return 0, fmt.Errorf("batch aggregate %s: %w", aggType, err)
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}
