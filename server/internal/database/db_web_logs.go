package database

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// UpdateHostWebLogs refreshes the small per-host counters on the hosts table
// from the latest agent report so the dashboard tile is up to date without
// having to scan the per-request rows.
func (db *DB) UpdateHostWebLogs(ctx context.Context, hostID string, report *models.WebLogReport) error {
	if report == nil {
		return nil
	}
	traffic := report.Traffic
	threats := report.Threats
	if traffic == nil {
		traffic = &models.TrafficSummary{}
	}
	if threats == nil {
		threats = &models.ThreatSummary{}
	}

	_, err := db.conn.ExecContext(ctx,
		`UPDATE hosts
		 SET web_log_source = $1,
		     web_log_collected_at = $2,
		     web_log_total_requests = $3,
		     web_log_total_bytes = $4,
		     web_log_errors_4xx = $5,
		     web_log_errors_5xx = $6,
		     web_log_suspicious_requests = $7,
		     web_log_suspicious_ips = $8,
		     updated_at = NOW()
		 WHERE id = $9`,
		report.Source,
		report.CollectedAt,
		traffic.TotalRequests,
		traffic.TotalBytes,
		traffic.Errors4xx,
		traffic.Errors5xx,
		threats.SuspiciousRequests,
		threats.UniqueSuspiciousIPs,
		hostID,
	)
	return err
}

// InsertWebLogSnapshot stores the full per-request batch from an agent report
// in `web_log_snapshots` + `web_log_requests`. Each request row carries an
// idempotency fingerprint so re-ingesting the same payload (retry, replay)
// only updates blocking metadata, never duplicates.
func (db *DB) InsertWebLogSnapshot(ctx context.Context, hostID string, report *models.WebLogReport) error {
	if report == nil {
		return nil
	}
	traffic := report.Traffic
	threats := report.Threats
	if traffic == nil {
		traffic = &models.TrafficSummary{}
	}
	if threats == nil {
		threats = &models.ThreatSummary{}
	}

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	crowdSecTopJSON := []byte("[]")
	if len(threats.CrowdSecTopBlocked) > 0 {
		if b, err := json.Marshal(threats.CrowdSecTopBlocked); err == nil {
			crowdSecTopJSON = b
		}
	}

	var snapshotID int64
	err = tx.QueryRowContext(ctx,
		`INSERT INTO web_log_snapshots (host_id, captured_at, source, total_requests, total_bytes, errors_4xx, errors_5xx, suspicious_requests, suspicious_ips, crowdsec_blocked_ips, crowdsec_top_blocked)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id`,
		hostID,
		report.CollectedAt,
		report.Source,
		traffic.TotalRequests,
		traffic.TotalBytes,
		traffic.Errors4xx,
		traffic.Errors5xx,
		threats.SuspiciousRequests,
		threats.UniqueSuspiciousIPs,
		threats.CrowdSecTotalBlocked,
		crowdSecTopJSON,
	).Scan(&snapshotID)
	if err != nil {
		return err
	}

	for _, req := range report.Requests {
		ts, parseErr := time.Parse(time.RFC3339, req.Timestamp)
		if parseErr != nil {
			ts = report.CollectedAt
		}
		suspicious := req.Category != ""
		fingerprint := webLogFingerprint(hostID, report.Source, ts, req, suspicious)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO web_log_requests (snapshot_id, host_id, captured_at, source, ip, method, path, status, bytes, user_agent, domain, category, suspicious, fingerprint, blocked, blocked_source, blocked_reason, blocked_at, blocked_until)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
			 ON CONFLICT (host_id, source, fingerprint) DO UPDATE
			 SET blocked = EXCLUDED.blocked,
			     blocked_source = EXCLUDED.blocked_source,
			     blocked_reason = EXCLUDED.blocked_reason,
			     blocked_at = EXCLUDED.blocked_at,
			     blocked_until = EXCLUDED.blocked_until
			 WHERE EXCLUDED.blocked = TRUE`,
			snapshotID,
			hostID,
			ts,
			report.Source,
			req.IP,
			req.Method,
			req.Path,
			req.Status,
			req.Bytes,
			req.UserAgent,
			req.Domain,
			req.Category,
			suspicious,
			fingerprint,
			req.Blocked,
			req.BlockedSource,
			req.BlockedReason,
			req.BlockedAt,
			req.BlockedUntil,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// webLogFingerprint produces a stable hash for de-duplicating identical
// requests on replay. Fields included are all the dimensions the UI may show.
func webLogFingerprint(hostID string, source string, capturedAt time.Time, req models.WebRequest, suspicious bool) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%d|%s|%s|%s|%t",
		hostID,
		source,
		capturedAt.UTC().Format(time.RFC3339Nano),
		req.IP,
		req.Method,
		req.Path,
		req.Status,
		req.Bytes,
		req.UserAgent,
		req.Domain,
		req.Category,
		suspicious,
	)
	digest := md5.Sum([]byte(payload))
	return hex.EncodeToString(digest[:])
}

// GetHostWebLogCache returns the small denormalised counters from the hosts
// table (no scan of per-request rows). Used by host detail tiles.
func (db *DB) GetHostWebLogCache(ctx context.Context, hostID string) (requests int64, bytes int64, errors5xx int64, capturedAt *time.Time, err error) {
	var ts sql.NullTime
	err = db.conn.QueryRowContext(ctx,
		`SELECT web_log_total_requests, web_log_total_bytes, web_log_errors_5xx, web_log_collected_at FROM hosts WHERE id = $1`,
		hostID,
	).Scan(&requests, &bytes, &errors5xx, &ts)
	if ts.Valid {
		t := ts.Time
		capturedAt = &t
	}
	return
}

// CleanOldWebLogs deletes web log snapshots older than the configured
// retention window. The per-request rows are removed via ON DELETE CASCADE.
func (db *DB) CleanOldWebLogs(ctx context.Context, days int) (int64, error) {
	if days <= 0 {
		days = 30
	}
	res, err := db.conn.ExecContext(ctx, `DELETE FROM web_log_snapshots WHERE captured_at < NOW() - ($1 || ' days')::INTERVAL`, days)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
