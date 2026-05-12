package database

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

func (db *DB) UpdateHostWebLogs(hostID string, report *models.WebLogReport) error {
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

	_, err := db.conn.Exec(
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

func (db *DB) InsertWebLogSnapshot(hostID string, report *models.WebLogReport) error {
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

	tx, err := db.conn.Begin()
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
	err = tx.QueryRow(
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
		if _, err := tx.Exec(
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

func (db *DB) GetWebLogsSummary(since time.Time, hostID string, source string) (map[string]any, error) {
	args := []any{since}
	where := "captured_at >= $1"
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND host_id = $%d", len(args))
	}
	if source != "" {
		args = append(args, source)
		where += fmt.Sprintf(" AND source = $%d", len(args))
	}

	traffic := map[string]any{}
	var totalRequests int64
	var totalBytes int64
	var status2xx int64
	var status3xx int64
	var errors4xx int64
	var errors5xx int64
	if err := db.conn.QueryRow(
		fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(bytes),0),
		COALESCE(SUM(CASE WHEN status BETWEEN 200 AND 299 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status BETWEEN 300 AND 399 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0)
		FROM web_log_requests WHERE %s`, where),
		args...,
	).Scan(&totalRequests, &totalBytes, &status2xx, &status3xx, &errors4xx, &errors5xx); err != nil {
		return nil, err
	}
	traffic["total_requests"] = totalRequests
	traffic["total_bytes"] = totalBytes
	traffic["errors_4xx"] = errors4xx
	traffic["errors_5xx"] = errors5xx

	count := toInt64(traffic["total_requests"])
	if count > 0 {
		traffic["ratio_4xx"] = float64(toInt64(traffic["errors_4xx"])) / float64(count)
		traffic["ratio_5xx"] = float64(toInt64(traffic["errors_5xx"])) / float64(count)
	} else {
		traffic["ratio_4xx"] = float64(0)
		traffic["ratio_5xx"] = float64(0)
	}
	traffic["status_distribution"] = map[string]any{
		"2xx": status2xx,
		"3xx": status3xx,
		"4xx": errors4xx,
		"5xx": errors5xx,
	}

	// Blocked requests statistics
	var blockedRequests int64
	var blockedIPs int64
	if err := db.conn.QueryRow(
		fmt.Sprintf(`SELECT 
		COALESCE(COUNT(*),0),
		COALESCE(COUNT(DISTINCT ip),0)
		FROM web_log_requests WHERE %s AND blocked = TRUE`, where),
		args...,
	).Scan(&blockedRequests, &blockedIPs); err == nil {
		traffic["blocked_requests"] = blockedRequests
		traffic["blocked_ips"] = blockedIPs
		if totalRequests > 0 {
			traffic["blocked_ratio"] = float64(blockedRequests) / float64(totalRequests)
		} else {
			traffic["blocked_ratio"] = float64(0)
		}
	}

	// Pre-fetch method distribution for all domains in a single query to avoid N+1.
	domainMethods := map[string]map[string]int64{}
	methodBatchRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT COALESCE(NULLIF(domain,''), '(unknown)'), method, COUNT(*)
		FROM web_log_requests
		WHERE %s
		GROUP BY 1, 2`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	for methodBatchRows.Next() {
		var dom, method string
		var cnt int64
		if err := methodBatchRows.Scan(&dom, &method, &cnt); err != nil {
			_ = methodBatchRows.Close()
			return nil, err
		}
		if domainMethods[dom] == nil {
			domainMethods[dom] = map[string]int64{}
		}
		domainMethods[dom][method] = cnt
	}
	_ = methodBatchRows.Close()

	rows, err := db.conn.Query(
		fmt.Sprintf(`SELECT COALESCE(NULLIF(domain,''), '(unknown)') AS domain,
		COUNT(*) AS hits,
		COALESCE(SUM(bytes),0) AS bytes,
		SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END) AS errors_4xx,
		SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END) AS errors_5xx
		FROM web_log_requests
		WHERE %s
		GROUP BY domain
		ORDER BY hits DESC
		LIMIT 20`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	topDomains := make([]map[string]any, 0)
	for rows.Next() {
		var domain string
		var hits int64
		var bytes int64
		var errors4xx int64
		var errors5xx int64
		if err := rows.Scan(&domain, &hits, &bytes, &errors4xx, &errors5xx); err != nil {
			return nil, err
		}
		methods := domainMethods[domain]
		if methods == nil {
			methods = map[string]int64{}
		}
		topDomains = append(topDomains, map[string]any{
			"domain":     domain,
			"hits":       hits,
			"bytes":      bytes,
			"errors_4xx": errors4xx,
			"errors_5xx": errors5xx,
			"methods":    methods,
		})
	}
	traffic["top_domains"] = topDomains

	endpointRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT method, path, status, COUNT(*) AS hits, COALESCE(SUM(bytes),0) AS bytes
		FROM web_log_requests
		WHERE %s
		GROUP BY method, path, status
		ORDER BY hits DESC
		LIMIT 30`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = endpointRows.Close() }()
	topEndpoints := make([]map[string]any, 0)
	for endpointRows.Next() {
		var method string
		var path string
		var status int
		var hits int64
		var bytes int64
		if err := endpointRows.Scan(&method, &path, &status, &hits, &bytes); err != nil {
			return nil, err
		}
		topEndpoints = append(topEndpoints, map[string]any{
			"method": method,
			"path":   path,
			"status": status,
			"hits":   hits,
			"bytes":  bytes,
		})
	}
	traffic["top_endpoints"] = topEndpoints

	// top_proxy_hosts is the same data as top_domains; derive it without an extra query.
	topProxyHosts := make([]map[string]any, 0, len(topDomains))
	for _, d := range topDomains {
		topProxyHosts = append(topProxyHosts, map[string]any{
			"vhost":      d["domain"],
			"hits":       d["hits"],
			"bytes":      d["bytes"],
			"errors_4xx": d["errors_4xx"],
			"errors_5xx": d["errors_5xx"],
		})
	}
	traffic["top_proxy_hosts"] = topProxyHosts

	hostTrafficRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT h.id, h.name, COUNT(*) AS hits, COALESCE(SUM(r.bytes),0) AS bytes
		FROM web_log_requests r
		JOIN hosts h ON h.id = r.host_id
		WHERE %s
		GROUP BY h.id, h.name
		ORDER BY hits DESC
		LIMIT 20`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = hostTrafficRows.Close() }()
	topHosts := make([]map[string]any, 0)
	for hostTrafficRows.Next() {
		var id, name string
		var hits int64
		var bytes int64
		if err := hostTrafficRows.Scan(&id, &name, &hits, &bytes); err != nil {
			return nil, err
		}
		topHosts = append(topHosts, map[string]any{
			"host_id":   id,
			"host_name": name,
			"hits":      hits,
			"bytes":     bytes,
		})
	}
	traffic["top_hosts"] = topHosts

	threats := map[string]any{}
	var suspiciousRequests int64
	var suspiciousIPs int64
	var targetedHosts int64
	if err := db.conn.QueryRow(
		fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(COUNT(DISTINCT ip),0), COALESCE(COUNT(DISTINCT COALESCE(NULLIF(domain,''), '(unknown)')),0)
		FROM web_log_requests
		WHERE %s AND suspicious = TRUE`, where),
		args...,
	).Scan(&suspiciousRequests, &suspiciousIPs, &targetedHosts); err != nil {
		return nil, err
	}
	threats["suspicious_requests"] = suspiciousRequests
	threats["suspicious_ips"] = suspiciousIPs
	threats["targeted_hosts"] = targetedHosts

	ipRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT ip,
		COUNT(*) AS hits,
		COUNT(DISTINCT path) AS unique_paths,
		COUNT(DISTINCT COALESCE(NULLIF(domain,''), '(unknown)')) AS host_count,
		MIN(captured_at) AS first_seen,
		MAX(captured_at) AS last_seen,
		MAX(CASE WHEN blocked = TRUE THEN blocked_source END) AS blocked_source,
		MAX(CASE WHEN blocked = TRUE THEN blocked_reason END) AS blocked_reason,
		MAX(CASE WHEN blocked = TRUE THEN blocked_at END) AS blocked_at,
		MAX(CASE WHEN blocked = TRUE THEN blocked_until END) AS blocked_until,
		MAX(CASE WHEN blocked = TRUE THEN 1 ELSE 0 END) AS is_blocked
		FROM web_log_requests
		WHERE %s AND suspicious = TRUE
		GROUP BY ip
		ORDER BY hits DESC
		LIMIT 25`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ipRows.Close() }()
	topIPs := make([]map[string]any, 0)
	for ipRows.Next() {
		var ip string
		var hits int64
		var uniquePaths int64
		var hostCount int64
		var firstSeen time.Time
		var lastSeen time.Time
		var blockedSource sql.NullString
		var blockedReason sql.NullString
		var blockedAt sql.NullTime
		var blockedUntil sql.NullTime
		var isBlocked int
		if err := ipRows.Scan(&ip, &hits, &uniquePaths, &hostCount, &firstSeen, &lastSeen, &blockedSource, &blockedReason, &blockedAt, &blockedUntil, &isBlocked); err != nil {
			return nil, err
		}
		score := hits * uniquePaths
		level := "LOW"
		switch {
		case score >= 400:
			level = "CRITICAL"
		case score >= 200:
			level = "HIGH"
		case score >= 80:
			level = "MEDIUM"
		}
		ipData := map[string]any{
			"ip":           ip,
			"hits":         hits,
			"unique_paths": uniquePaths,
			"host_count":   hostCount,
			"first_seen":   firstSeen,
			"last_seen":    lastSeen,
			"threat_score": score,
			"level":        level,
		}
		// Add blocking info if IP is blocked
		if isBlocked == 1 {
			ipData["blocked"] = true
			if blockedSource.Valid {
				ipData["blocked_source"] = blockedSource.String
			}
			if blockedReason.Valid {
				ipData["blocked_reason"] = blockedReason.String
			}
			if blockedAt.Valid {
				ipData["blocked_at"] = blockedAt.Time
			}
			if blockedUntil.Valid {
				ipData["blocked_until"] = blockedUntil.Time
			}
		}
		topIPs = append(topIPs, ipData)
	}
	threats["top_ips"] = topIPs

	pathsRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT path, COALESCE(NULLIF(category,''), 'Unknown') AS category, COUNT(*) AS hits
		FROM web_log_requests
		WHERE %s AND suspicious = TRUE
		GROUP BY path, category
		ORDER BY hits DESC
		LIMIT 25`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = pathsRows.Close() }()
	topPaths := make([]map[string]any, 0)
	for pathsRows.Next() {
		var path, category string
		var hits int64
		if err := pathsRows.Scan(&path, &category, &hits); err != nil {
			return nil, err
		}
		topPaths = append(topPaths, map[string]any{"path": path, "category": category, "hits": hits})
	}
	threats["top_paths"] = topPaths

	hostRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT COALESCE(NULLIF(r.domain,''), '(unknown)') AS vhost, COUNT(*) AS hits
		FROM web_log_requests r
		WHERE %s AND r.suspicious = TRUE
		GROUP BY vhost
		ORDER BY hits DESC
		LIMIT 20`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = hostRows.Close() }()
	mostTargetedHosts := make([]map[string]any, 0)
	for hostRows.Next() {
		var vhost string
		var hits int64
		if err := hostRows.Scan(&vhost, &hits); err != nil {
			return nil, err
		}
		mostTargetedHosts = append(mostTargetedHosts, map[string]any{"host_id": vhost, "host_name": vhost, "hits": hits})
	}
	threats["most_targeted_hosts"] = mostTargetedHosts

	matrixRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT ip, COUNT(DISTINCT COALESCE(NULLIF(domain,''), '(unknown)')) AS host_count, COUNT(*) AS hits
		FROM web_log_requests
		WHERE %s AND suspicious = TRUE
		GROUP BY ip
		HAVING COUNT(DISTINCT COALESCE(NULLIF(domain,''), '(unknown)')) > 1
		ORDER BY host_count DESC, hits DESC
		LIMIT 30`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = matrixRows.Close() }()
	ipHostMatrix := make([]map[string]any, 0)
	for matrixRows.Next() {
		var ip string
		var hostCount int64
		var hits int64
		if err := matrixRows.Scan(&ip, &hostCount, &hits); err != nil {
			return nil, err
		}
		ipHostMatrix = append(ipHostMatrix, map[string]any{"ip": ip, "host_count": hostCount, "hits": hits})
	}
	threats["ip_host_matrix"] = ipHostMatrix

	// CrowdSec: return count + top blocked IPs from the most recent snapshot.
	var crowdSecBlocked int64
	var crowdSecHostCount int64
	csArgs := []any{since}
	csWhere := "captured_at >= $1"
	if hostID != "" {
		csArgs = append(csArgs, hostID)
		csWhere += fmt.Sprintf(" AND host_id = $%d", len(csArgs))
	}
	if source != "" {
		csArgs = append(csArgs, source)
		csWhere += fmt.Sprintf(" AND source = $%d", len(csArgs))
	}
	countQuery := fmt.Sprintf(`
		WITH snapshots AS (
			SELECT captured_at, host_id, COALESCE(crowdsec_top_blocked, '[]'::jsonb) AS top_blocked
			FROM web_log_snapshots
			WHERE %s
		),
		expanded AS (
			SELECT
				snapshots.captured_at,
				snapshots.host_id,
				elem->>'ip' AS ip
			FROM snapshots
			CROSS JOIN LATERAL jsonb_array_elements(snapshots.top_blocked) AS elem
		)
		SELECT COALESCE(COUNT(DISTINCT ip), 0), COALESCE(COUNT(DISTINCT host_id), 0)
		FROM expanded
		WHERE ip IS NOT NULL AND ip <> ''`, csWhere)
	if err := db.conn.QueryRow(countQuery, csArgs...).Scan(&crowdSecBlocked, &crowdSecHostCount); err != nil {
		return nil, err
	}
	threats["crowdsec_blocked_ips"] = crowdSecBlocked

	if crowdSecBlocked > 0 {
		const crowdSecTopBlockedLimit = 500
		listQuery := fmt.Sprintf(`
			WITH snapshots AS (
				SELECT captured_at, host_id, COALESCE(crowdsec_top_blocked, '[]'::jsonb) AS top_blocked
				FROM web_log_snapshots
				WHERE %s
			),
			expanded AS (
				SELECT
					snapshots.captured_at,
					snapshots.host_id,
					elem->>'ip' AS ip,
					COALESCE(NULLIF(elem->>'type', ''), 'ban') AS type,
					elem->>'reason' AS reason,
					elem->>'origin' AS origin,
					elem->>'country' AS country,
					elem->>'as_name' AS as_name,
					elem->>'blocked_until' AS blocked_until
				FROM snapshots
				CROSS JOIN LATERAL jsonb_array_elements(snapshots.top_blocked) AS elem
			),
			dedup AS (
				SELECT DISTINCT ON (ip)
					ip, type, reason, origin, country, as_name, blocked_until, captured_at, host_id
				FROM expanded
				WHERE ip IS NOT NULL AND ip <> ''
				ORDER BY ip, captured_at DESC
			)
			SELECT ip, type, reason, origin, country, as_name, blocked_until, captured_at, host_id
			FROM dedup
			ORDER BY captured_at DESC, ip
			LIMIT %d`, csWhere, crowdSecTopBlockedLimit)
		rows, err := db.conn.Query(listQuery, csArgs...)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()

		csEntries := make([]map[string]any, 0)
		for rows.Next() {
			var ip string
			var decisionType sql.NullString
			var reason sql.NullString
			var origin sql.NullString
			var country sql.NullString
			var asName sql.NullString
			var blockedUntil sql.NullString
			var capturedAt time.Time
			var rowHostID string
			if err := rows.Scan(&ip, &decisionType, &reason, &origin, &country, &asName, &blockedUntil, &capturedAt, &rowHostID); err != nil {
				return nil, err
			}
			entry := map[string]any{"ip": ip}
			if decisionType.Valid && decisionType.String != "" {
				entry["type"] = decisionType.String
			}
			if reason.Valid && reason.String != "" {
				entry["reason"] = reason.String
			}
			if origin.Valid && origin.String != "" {
				entry["origin"] = origin.String
			}
			if country.Valid && country.String != "" {
				entry["country"] = country.String
			}
			if asName.Valid && asName.String != "" {
				entry["as_name"] = asName.String
			}
			if blockedUntil.Valid && blockedUntil.String != "" {
				entry["blocked_until"] = blockedUntil.String
			}
			entry["last_seen"] = capturedAt
			entry["host_id"] = rowHostID
			csEntries = append(csEntries, entry)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		threats["crowdsec_top_blocked"] = csEntries
	}

	if hostID != "" {
		threats["crowdsec_host_id"] = hostID
	} else if crowdSecHostCount == 1 {
		var singleHostID string
		if err := db.conn.QueryRow(
			fmt.Sprintf(`SELECT MAX(host_id) FROM web_log_snapshots WHERE %s`, csWhere),
			csArgs...,
		).Scan(&singleHostID); err == nil && singleHostID != "" {
			threats["crowdsec_host_id"] = singleHostID
		}
	}

	return map[string]any{
		"traffic": traffic,
		"threats": threats,
	}, nil
}

func (db *DB) GetIPTimeline(ip string, since time.Time, hostID string, limit int) ([]models.WebLogIPTimelineRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	args := []any{ip, since}
	where := "r.ip = $1 AND r.captured_at >= $2"
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND r.host_id = $%d", len(args))
	}
	args = append(args, limit)

	rows, err := db.conn.Query(
		fmt.Sprintf(`SELECT r.captured_at, r.host_id, h.name, r.source, r.ip, r.method, r.path, r.status, r.bytes, COALESCE(r.user_agent,''), COALESCE(r.domain,''), COALESCE(r.category,''), r.blocked, COALESCE(r.blocked_source,''), COALESCE(r.blocked_reason,''), r.blocked_at, r.blocked_until
		FROM web_log_requests r
		JOIN hosts h ON h.id = r.host_id
		WHERE %s
		ORDER BY r.captured_at DESC
		LIMIT $%d`, where, len(args)),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.WebLogIPTimelineRow, 0)
	for rows.Next() {
		var row models.WebLogIPTimelineRow
		if err := rows.Scan(&row.Timestamp, &row.HostID, &row.HostName, &row.Source, &row.IP, &row.Method, &row.Path, &row.Status, &row.Bytes, &row.UserAgent, &row.Domain, &row.Category, &row.Blocked, &row.BlockedSource, &row.BlockedReason, &row.BlockedAt, &row.BlockedUntil); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (db *DB) GetDomainDetails(domain string, since time.Time, hostID string, source string, limit int) (map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 300
	}
	args := []any{since, domain}
	where := "captured_at >= $1 AND COALESCE(NULLIF(domain,''), '(unknown)') = $2"
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND host_id = $%d", len(args))
	}
	if source != "" {
		args = append(args, source)
		where += fmt.Sprintf(" AND source = $%d", len(args))
	}

	out := map[string]any{}
	var hits int64
	var bytes int64
	var status2xx int64
	var status3xx int64
	var status4xx int64
	var status5xx int64
	if err := db.conn.QueryRow(
		fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(bytes),0),
		COALESCE(SUM(CASE WHEN status BETWEEN 200 AND 299 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status BETWEEN 300 AND 399 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0)
		FROM web_log_requests WHERE %s`, where),
		args...,
	).Scan(&hits, &bytes, &status2xx, &status3xx, &status4xx, &status5xx); err != nil {
		return nil, err
	}
	out["hits"] = hits
	out["bytes"] = bytes
	out["status_2xx"] = status2xx
	out["status_3xx"] = status3xx
	out["status_4xx"] = status4xx
	out["status_5xx"] = status5xx

	pathsRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT path, COUNT(*) AS hits
		FROM web_log_requests
		WHERE %s
		GROUP BY path
		ORDER BY hits DESC
		LIMIT 30`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = pathsRows.Close() }()
	paths := make([]map[string]any, 0)
	for pathsRows.Next() {
		var path string
		var hits int64
		if err := pathsRows.Scan(&path, &hits); err != nil {
			return nil, err
		}
		paths = append(paths, map[string]any{"path": path, "hits": hits})
	}
	out["top_paths"] = paths

	ipRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT ip, COUNT(*) AS hits,
		MAX(CASE WHEN blocked = TRUE THEN blocked_source END) AS blocked_source,
		MAX(CASE WHEN blocked = TRUE THEN blocked_reason END) AS blocked_reason,
		MAX(CASE WHEN blocked = TRUE THEN blocked_at END) AS blocked_at,
		MAX(CASE WHEN blocked = TRUE THEN blocked_until END) AS blocked_until,
		MAX(CASE WHEN blocked = TRUE THEN 1 ELSE 0 END) AS is_blocked
		FROM web_log_requests
		WHERE %s
		GROUP BY ip
		ORDER BY hits DESC
		LIMIT 30`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ipRows.Close() }()
	ipClients := make([]map[string]any, 0)
	for ipRows.Next() {
		var ip string
		var hits int64
		var blockedSource sql.NullString
		var blockedReason sql.NullString
		var blockedAt sql.NullTime
		var blockedUntil sql.NullTime
		var isBlocked int
		if err := ipRows.Scan(&ip, &hits, &blockedSource, &blockedReason, &blockedAt, &blockedUntil, &isBlocked); err != nil {
			return nil, err
		}
		clientData := map[string]any{"ip": ip, "hits": hits}
		if isBlocked == 1 {
			clientData["blocked"] = true
			if blockedSource.Valid {
				clientData["blocked_source"] = blockedSource.String
			}
			if blockedReason.Valid {
				clientData["blocked_reason"] = blockedReason.String
			}
			if blockedAt.Valid {
				clientData["blocked_at"] = blockedAt.Time
			}
			if blockedUntil.Valid {
				clientData["blocked_until"] = blockedUntil.Time
			}
		}
		ipClients = append(ipClients, clientData)
	}
	out["top_clients"] = ipClients

	args = append(args, limit)
	reqRows, err := db.conn.Query(
		fmt.Sprintf(`SELECT captured_at, host_id, source, ip, method, path, status, bytes, COALESCE(user_agent,''), COALESCE(category,''), blocked, COALESCE(blocked_source,''), COALESCE(blocked_reason,''), blocked_at, blocked_until
		FROM web_log_requests
		WHERE %s
		ORDER BY captured_at DESC
		LIMIT $%d`, where, len(args)),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reqRows.Close() }()
	requests := make([]map[string]any, 0)
	for reqRows.Next() {
		var capturedAt time.Time
		var rowHostID, rowSource, ip, method, path, userAgent, category string
		var status int
		var bytes int64
		var blocked bool
		var blockedSource, blockedReason string
		var blockedAt, blockedUntil sql.NullTime
		if err := reqRows.Scan(&capturedAt, &rowHostID, &rowSource, &ip, &method, &path, &status, &bytes, &userAgent, &category, &blocked, &blockedSource, &blockedReason, &blockedAt, &blockedUntil); err != nil {
			return nil, err
		}
		reqData := map[string]any{
			"timestamp":  capturedAt,
			"host_id":    rowHostID,
			"source":     rowSource,
			"ip":         ip,
			"method":     method,
			"path":       path,
			"status":     status,
			"bytes":      bytes,
			"user_agent": userAgent,
			"category":   category,
		}
		if blocked {
			reqData["blocked"] = true
			if blockedSource != "" {
				reqData["blocked_source"] = blockedSource
			}
			if blockedReason != "" {
				reqData["blocked_reason"] = blockedReason
			}
			if blockedAt.Valid {
				reqData["blocked_at"] = blockedAt.Time
			}
			if blockedUntil.Valid {
				reqData["blocked_until"] = blockedUntil.Time
			}
		}
		requests = append(requests, reqData)
	}
	out["requests"] = requests

	return out, nil
}

func (db *DB) GetHostWebLogCache(hostID string) (requests int64, bytes int64, errors5xx int64, capturedAt *time.Time, err error) {
	var ts sql.NullTime
	err = db.conn.QueryRow(
		`SELECT web_log_total_requests, web_log_total_bytes, web_log_errors_5xx, web_log_collected_at FROM hosts WHERE id = $1`,
		hostID,
	).Scan(&requests, &bytes, &errors5xx, &ts)
	if ts.Valid {
		t := ts.Time
		capturedAt = &t
	}
	return
}

func (db *DB) GetWebLogsKPIWindow(since time.Time, until time.Time, hostID string, source string) (map[string]any, error) {
	args := []any{since}
	where := "captured_at >= $1"
	if !until.IsZero() {
		args = append(args, until)
		where += fmt.Sprintf(" AND captured_at < $%d", len(args))
	}
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND host_id = $%d", len(args))
	}
	if source != "" {
		args = append(args, source)
		where += fmt.Sprintf(" AND source = $%d", len(args))
	}

	var totalRequests int64
	var totalBytes int64
	var errors5xx int64
	if err := db.conn.QueryRow(
		fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(bytes),0),
		COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0)
		FROM web_log_requests
		WHERE %s`, where),
		args...,
	).Scan(&totalRequests, &totalBytes, &errors5xx); err != nil {
		return nil, err
	}

	var suspiciousIPs int64
	if err := db.conn.QueryRow(
		fmt.Sprintf(`SELECT COALESCE(COUNT(DISTINCT ip),0)
		FROM web_log_requests
		WHERE %s AND suspicious = TRUE`, where),
		args...,
	).Scan(&suspiciousIPs); err != nil {
		return nil, err
	}

	ratio5xx := float64(0)
	if totalRequests > 0 {
		ratio5xx = float64(errors5xx) / float64(totalRequests)
	}

	return map[string]any{
		"total_requests": totalRequests,
		"total_bytes":    totalBytes,
		"ratio_5xx":      ratio5xx,
		"suspicious_ips": suspiciousIPs,
	}, nil
}

func (db *DB) GetWebLogsTimeseries(since time.Time, hostID string, source string, bucket string) ([]map[string]any, error) {
	if bucket != "minute" && bucket != "hour" {
		bucket = "hour"
	}

	args := []any{since}
	where := "captured_at >= $1"
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND host_id = $%d", len(args))
	}
	if source != "" {
		args = append(args, source)
		where += fmt.Sprintf(" AND source = $%d", len(args))
	}

	query := fmt.Sprintf(`SELECT date_trunc('%s', captured_at) AS bucket_ts,
	COUNT(*) AS total,
	SUM(CASE WHEN suspicious = TRUE THEN 1 ELSE 0 END) AS bot,
	SUM(CASE WHEN suspicious = FALSE THEN 1 ELSE 0 END) AS human,
	SUM(CASE WHEN status BETWEEN 200 AND 299 THEN 1 ELSE 0 END) AS status_2xx,
	SUM(CASE WHEN status BETWEEN 300 AND 399 THEN 1 ELSE 0 END) AS status_3xx,
	SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END) AS status_4xx,
	SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END) AS status_5xx
	FROM web_log_requests
	WHERE %s
	GROUP BY bucket_ts
	ORDER BY bucket_ts ASC`, bucket, where)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var ts time.Time
		var total int64
		var bot int64
		var human int64
		var status2xx int64
		var status3xx int64
		var status4xx int64
		var status5xx int64
		if err := rows.Scan(&ts, &total, &bot, &human, &status2xx, &status3xx, &status4xx, &status5xx); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"timestamp":  ts,
			"total":      total,
			"bot":        bot,
			"human":      human,
			"status_2xx": status2xx,
			"status_3xx": status3xx,
			"status_4xx": status4xx,
			"status_5xx": status5xx,
		})
	}

	return out, nil
}

func (db *DB) GetWebLogsLive(hostID string, source string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	args := []any{}
	where := "TRUE"
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND r.host_id = $%d", len(args))
	}
	if source != "" {
		args = append(args, source)
		where += fmt.Sprintf(" AND r.source = $%d", len(args))
	}
	args = append(args, limit)

	rows, err := db.conn.Query(
		fmt.Sprintf(`SELECT r.captured_at, r.host_id, h.name, r.source, r.ip, r.method, r.path, r.status, r.bytes, COALESCE(r.user_agent,''), COALESCE(r.domain,''), COALESCE(r.category,''), r.suspicious
		FROM web_log_requests r
		JOIN hosts h ON h.id = r.host_id
		WHERE %s
		ORDER BY r.captured_at DESC
		LIMIT $%d`, where, len(args)),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var capturedAt time.Time
		var rowHostID, rowHostName, rowSource, ip, method, path, userAgent, domain, category string
		var status int
		var bytes int64
		var suspicious bool
		if err := rows.Scan(&capturedAt, &rowHostID, &rowHostName, &rowSource, &ip, &method, &path, &status, &bytes, &userAgent, &domain, &category, &suspicious); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"timestamp":  capturedAt,
			"host_id":    rowHostID,
			"host_name":  rowHostName,
			"source":     rowSource,
			"ip":         ip,
			"method":     method,
			"path":       path,
			"status":     status,
			"bytes":      bytes,
			"user_agent": userAgent,
			"domain":     domain,
			"category":   category,
			"suspicious": suspicious,
		})
	}

	return out, nil
}

func (db *DB) GetWebLogsTopClientIPs(since time.Time, hostID string, source string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	args := []any{since}
	where := "captured_at >= $1"
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND host_id = $%d", len(args))
	}
	if source != "" {
		args = append(args, source)
		where += fmt.Sprintf(" AND source = $%d", len(args))
	}
	args = append(args, limit)

	rows, err := db.conn.Query(
		fmt.Sprintf(`SELECT ip, COUNT(*) AS hits
		FROM web_log_requests
		WHERE %s
		GROUP BY ip
		ORDER BY hits DESC
		LIMIT $%d`, where, len(args)),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]map[string]any, 0)
	for rows.Next() {
		var ip string
		var hits int64
		if err := rows.Scan(&ip, &hits); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"ip": ip, "hits": hits})
	}

	return out, nil
}

func (db *DB) CleanOldWebLogs(days int) (int64, error) {
	if days <= 0 {
		days = 30
	}
	res, err := db.conn.Exec(`DELETE FROM web_log_snapshots WHERE captured_at < NOW() - ($1 || ' days')::INTERVAL`, days)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}
