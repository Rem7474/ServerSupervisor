package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Uptime daily rollup / SLA ==========

// sloTargetOrDefault maps an unset (zero or negative) objective to
// models.DefaultSLOTarget, so callers that build a probe without one (the NPM
// monitoring toggle, the demo seeder) still satisfy the column's CHECK.
func sloTargetOrDefault(v float64) float64 {
	if v <= 0 {
		return models.DefaultSLOTarget
	}
	return v
}

// RollupUptimeDaily recomputes uptime_probe_daily for the complete UTC days
// in [today - sinceDays, today) from the raw results. Idempotent: a day is
// rewritten with its full counts, so it is safe to re-run over days already
// rolled up, as long as they are still fully covered by the raw retention.
func (db *DB) RollupUptimeDaily(ctx context.Context, sinceDays int) (int64, error) {
	res, err := db.conn.ExecContext(ctx,
		`INSERT INTO uptime_probe_daily (probe_id, day, total_checks, up_checks)
		 SELECT probe_id,
		        (checked_at AT TIME ZONE 'UTC')::date AS day,
		        COUNT(*),
		        COUNT(*) FILTER (WHERE success)
		 FROM uptime_probe_results
		 WHERE checked_at >= (date_trunc('day', NOW() AT TIME ZONE 'UTC') - make_interval(days => $1)) AT TIME ZONE 'UTC'
		   AND checked_at <  date_trunc('day', NOW() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'
		 GROUP BY probe_id, day
		 ON CONFLICT (probe_id, day) DO UPDATE
		   SET total_checks = EXCLUDED.total_checks,
		       up_checks    = EXCLUDED.up_checks`,
		sinceDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// CleanupOldUptimeDaily removes daily rollup rows older than keepDays.
func (db *DB) CleanupOldUptimeDaily(ctx context.Context, keepDays int) (int64, error) {
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM uptime_probe_daily WHERE day < (NOW() AT TIME ZONE 'UTC')::date - $1::int`,
		keepDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetUptimeDailyCounts returns per-day check counts of the given probes over
// the last `days` UTC days, today included. Today and yesterday come from the
// raw results (the hourly rollup may not have closed yesterday yet), older
// days from uptime_probe_daily, which outlives the raw retention. Days
// without any check are absent.
func (db *DB) GetUptimeDailyCounts(ctx context.Context, probeIDs []string, days int) ([]models.UptimeDailyCount, error) {
	if len(probeIDs) == 0 || days <= 0 {
		return []models.UptimeDailyCount{}, nil
	}
	rows, err := db.conn.QueryContext(ctx,
		`SELECT probe_id, day, total_checks, up_checks
		 FROM uptime_probe_daily
		 WHERE probe_id = ANY($1::uuid[])
		   AND day >= (NOW() AT TIME ZONE 'UTC')::date - ($2::int - 1)
		   AND day <  (NOW() AT TIME ZONE 'UTC')::date - 1
		 UNION ALL
		 SELECT probe_id, (checked_at AT TIME ZONE 'UTC')::date AS day,
		        COUNT(*)::int, (COUNT(*) FILTER (WHERE success))::int
		 FROM uptime_probe_results
		 WHERE probe_id = ANY($1::uuid[])
		   AND checked_at >= (date_trunc('day', NOW() AT TIME ZONE 'UTC') - make_interval(days => LEAST($2::int - 1, 1))) AT TIME ZONE 'UTC'
		 GROUP BY probe_id, day
		 ORDER BY 1, 2`,
		pq.Array(probeIDs), days)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []models.UptimeDailyCount{}
	for rows.Next() {
		var c models.UptimeDailyCount
		if err := rows.Scan(&c.ProbeID, &c.Day, &c.TotalChecks, &c.UpChecks); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListUptimeDownPeriods returns the runs of consecutive failed checks of the
// given probes that started after `since`, newest first, with the first
// successful check that ended each one (nil while still down).
func (db *DB) ListUptimeDownPeriods(ctx context.Context, probeIDs []string, since time.Time, limit int) ([]models.UptimeDownPeriod, error) {
	if len(probeIDs) == 0 {
		return []models.UptimeDownPeriod{}, nil
	}
	rows, err := db.conn.QueryContext(ctx,
		`WITH r AS (
		     SELECT probe_id, checked_at, success,
		            CASE WHEN success IS DISTINCT FROM LAG(success) OVER w THEN 1 ELSE 0 END AS edge
		     FROM uptime_probe_results
		     WHERE probe_id = ANY($1::uuid[]) AND checked_at >= $2
		     WINDOW w AS (PARTITION BY probe_id ORDER BY checked_at)
		 ), g AS (
		     SELECT probe_id, checked_at, success,
		            SUM(edge) OVER (PARTITION BY probe_id ORDER BY checked_at) AS grp
		     FROM r
		 ), downs AS (
		     SELECT probe_id, MIN(checked_at) AS started_at, MAX(checked_at) AS last_failed_at
		     FROM g WHERE NOT success
		     GROUP BY probe_id, grp
		 )
		 SELECT d.probe_id, d.started_at,
		        (SELECT MIN(x.checked_at) FROM uptime_probe_results x
		          WHERE x.probe_id = d.probe_id AND x.checked_at > d.last_failed_at AND x.success)
		 FROM downs d
		 ORDER BY d.started_at DESC
		 LIMIT $3`,
		pq.Array(probeIDs), since, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []models.UptimeDownPeriod{}
	for rows.Next() {
		var p models.UptimeDownPeriod
		var resolved sql.NullTime
		if err := rows.Scan(&p.ProbeID, &p.StartedAt, &resolved); err != nil {
			return nil, err
		}
		if resolved.Valid {
			t := resolved.Time
			p.ResolvedAt = &t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ========== Status page configuration ==========

// GetStatusPageConfig returns the status page configuration and its
// published services in display order.
func (db *DB) GetStatusPageConfig(ctx context.Context) (*models.StatusPageConfig, error) {
	cfg := &models.StatusPageConfig{Services: []models.StatusPageService{}}
	err := db.conn.QueryRowContext(ctx,
		`SELECT enabled, title, description, updated_at FROM status_page_config WHERE id`,
	).Scan(&cfg.Enabled, &cfg.Title, &cfg.Description, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	rows, err := db.conn.QueryContext(ctx,
		`SELECT probe_id, public_name FROM status_page_services ORDER BY position, public_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s models.StatusPageService
		if err := rows.Scan(&s.ProbeID, &s.PublicName); err != nil {
			return nil, err
		}
		cfg.Services = append(cfg.Services, s)
	}
	return cfg, rows.Err()
}

// ReplaceStatusPageConfig stores the configuration and replaces the whole set
// of published services in one transaction.
func (db *DB) ReplaceStatusPageConfig(ctx context.Context, req models.StatusPageConfigRequest) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE status_page_config SET enabled = $1, title = $2, description = $3, updated_at = NOW() WHERE id`,
		req.Enabled, req.Title, req.Description); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM status_page_services`); err != nil {
		return err
	}
	for i, s := range req.Services {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO status_page_services (probe_id, public_name, position) VALUES ($1, $2, $3)`,
			s.ProbeID, s.PublicName, i); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListPublishedProbes returns the published services joined with the live
// state of their probe, in display order.
func (db *DB) ListPublishedProbes(ctx context.Context) ([]models.PublishedProbe, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT s.probe_id, s.public_name, p.enabled, p.last_status
		 FROM status_page_services s
		 JOIN uptime_probes p ON p.id = s.probe_id
		 ORDER BY s.position, s.public_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.PublishedProbe{}
	for rows.Next() {
		var p models.PublishedProbe
		if err := rows.Scan(&p.ProbeID, &p.PublicName, &p.Enabled, &p.LastStatus); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
