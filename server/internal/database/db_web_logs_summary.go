package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GetWebLogsSummary aggregates traffic + threats statistics across all
// captured web requests in the requested window. The shape of the returned
// map matches what the SecurityView/TrafficView frontends expect.
func (db *DB) GetWebLogsSummary(ctx context.Context, since time.Time, hostID string, source string) (map[string]any, error) {
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
	if err := db.conn.QueryRowContext(ctx,
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
	if err := db.conn.QueryRowContext(ctx,
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
	methodBatchRows, err := db.conn.QueryContext(ctx,
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

	rows, err := db.conn.QueryContext(ctx,
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

	endpointRows, err := db.conn.QueryContext(ctx,
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

	hostTrafficRows, err := db.conn.QueryContext(ctx,
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
	if err := db.conn.QueryRowContext(ctx,
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

	ipRows, err := db.conn.QueryContext(ctx,
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
		if isBlocked == 1 {
			ipData["blocked"] = true
			if blockedSource.Valid {
				ipData["blocked_source"] = blockedSource.String
				if blockedSource.String == "crowdsec" {
					ipData["blocked_type"] = "ban"
				}
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

	pathsRows, err := db.conn.QueryContext(ctx,
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

	hostRows, err := db.conn.QueryContext(ctx,
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

	matrixRows, err := db.conn.QueryContext(ctx,
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
	if err := db.conn.QueryRowContext(ctx, countQuery, csArgs...).Scan(&crowdSecBlocked, &crowdSecHostCount); err != nil {
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
		rows, err := db.conn.QueryContext(ctx, listQuery, csArgs...)
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
			typeStr := "ban"
			if decisionType.Valid && decisionType.String != "" {
				typeStr = decisionType.String
			}
			entry["type"] = typeStr
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
		if err := db.conn.QueryRowContext(ctx,
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

// GetWebLogsKPIWindow returns the small KPI tile values shown on the security
// dashboard (total requests, bytes, 5xx ratio, suspicious IPs) for a given
// window [since, until). until being zero means "open ended".
func (db *DB) GetWebLogsKPIWindow(ctx context.Context, since time.Time, until time.Time, hostID string, source string) (map[string]any, error) {
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
	if err := db.conn.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(bytes),0),
		COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0)
		FROM web_log_requests
		WHERE %s`, where),
		args...,
	).Scan(&totalRequests, &totalBytes, &errors5xx); err != nil {
		return nil, err
	}

	var suspiciousIPs int64
	if err := db.conn.QueryRowContext(ctx,
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

// GetWebLogsTimeseries returns request counts bucketed by minute or hour for
// stacked area / line charts. Bucket defaults to "hour" when unrecognised.
func (db *DB) GetWebLogsTimeseries(ctx context.Context, since time.Time, hostID string, source string, bucket string) ([]map[string]any, error) {
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

	rows, err := db.conn.QueryContext(ctx, query, args...)
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

// toInt64 is a tiny coercion helper used when collapsing map[string]any
// numeric values back to int64 for arithmetic.
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
