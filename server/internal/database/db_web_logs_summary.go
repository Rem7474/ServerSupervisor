package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/serversupervisor/server/internal/safego"
	"github.com/serversupervisor/server/internal/threatdetect"
)

// buildWebLogsWhere builds the shared WHERE clause + positional args used by
// every web-logs aggregate query (filtering by window, optional host, optional
// source). The same clause applies to both web_log_requests and
// web_log_snapshots since both carry captured_at / host_id / source columns.
// until being zero means "open ended" (no upper bound) — the pre-existing
// behavior for every caller before custom time ranges existed; see
// GetWebLogsKPIWindow, the original until-aware query this pattern is
// generalized from.
func buildWebLogsWhere(since, until time.Time, hostID, source string) (string, []any) {
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
	return where, args
}

// errCollector lets a WaitGroup-joined fan-out of independent queries report
// failures without a data race: each goroutine calls set(err) at most once,
// and the caller reads first() after Wait() to get the first one reported.
type errCollector struct {
	mu  sync.Mutex
	err error
}

func (c *errCollector) set(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.mu.Unlock()
}

func (c *errCollector) first() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// GetWebLogsSummary aggregates traffic + threats statistics across all
// captured web requests in the requested window. The shape of the returned
// map matches what the SecurityView/TrafficView frontends expect.
//
// The traffic/threats/CrowdSec sections below are independent aggregate
// queries over the same window — none reads another's result — so they run
// concurrently instead of as one long sequential chain. On a large table with
// no host filter (the default "all hosts" view), that sequential chain of
// ~15 queries was the direct cause of the dashboard's 30s request timeout;
// running them in parallel bounds the wall-clock cost by the single slowest
// query instead of their sum. Each still returns a real error on failure
// (via errCollector) except the best-effort "blocked" stats, which
// preserves the pre-existing "absent key on error" behavior other code
// depends on (see promoteBlockedIntoThreats in the weblogs service).
func (db *DB) GetWebLogsSummary(ctx context.Context, since, until time.Time, hostID string, source string) (map[string]any, error) {
	where, args := buildWebLogsWhere(since, until, hostID, source)

	var errs errCollector
	var wg sync.WaitGroup

	var totalRequests, totalBytes, status2xx, status3xx, errors4xx, errors5xx int64
	var blockedRequests, blockedIPs int64
	var blockedOK bool
	var topDomains, topEndpoints, topHosts []map[string]any
	threatsLocal := map[string]any{}
	crowdSecLocal := map[string]any{}

	wg.Add(7)

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.summary.counts")
		errs.set(db.conn.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(bytes),0),
			COALESCE(SUM(CASE WHEN status BETWEEN 200 AND 299 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status BETWEEN 300 AND 399 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0)
			FROM web_log_requests WHERE %s`, where),
			args...,
		).Scan(&totalRequests, &totalBytes, &status2xx, &status3xx, &errors4xx, &errors5xx))
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.summary.blocked")
		// Best-effort, matching the pre-existing behavior: a failure here just
		// leaves the blocked_* keys absent instead of failing the whole summary.
		if err := db.conn.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT
			COALESCE(COUNT(*),0),
			COALESCE(COUNT(DISTINCT ip),0)
			FROM web_log_requests WHERE %s AND blocked = TRUE`, where),
			args...,
		).Scan(&blockedRequests, &blockedIPs); err == nil {
			blockedOK = true
		}
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.summary.topDomains")
		td, err := db.topDomainsWithMethods(ctx, where, args)
		if err != nil {
			errs.set(err)
			return
		}
		topDomains = td
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.summary.topEndpoints")
		te, err := db.topEndpointTraffic(ctx, where, args)
		if err != nil {
			errs.set(err)
			return
		}
		topEndpoints = te
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.summary.topHosts")
		th, err := db.topHostTraffic(ctx, where, args)
		if err != nil {
			errs.set(err)
			return
		}
		topHosts = th
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.summary.threats")
		errs.set(db.fillWebLogsThreats(ctx, threatsLocal, where, args))
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.summary.crowdsec")
		errs.set(db.fillWebLogsCrowdSec(ctx, crowdSecLocal, where, args, hostID))
	}()

	wg.Wait()
	if err := errs.first(); err != nil {
		return nil, err
	}

	traffic := map[string]any{}
	traffic["total_requests"] = totalRequests
	traffic["total_bytes"] = totalBytes
	traffic["errors_4xx"] = errors4xx
	traffic["errors_5xx"] = errors5xx
	if totalRequests > 0 {
		traffic["ratio_4xx"] = float64(errors4xx) / float64(totalRequests)
		traffic["ratio_5xx"] = float64(errors5xx) / float64(totalRequests)
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
	if blockedOK {
		traffic["blocked_requests"] = blockedRequests
		traffic["blocked_ips"] = blockedIPs
		if totalRequests > 0 {
			traffic["blocked_ratio"] = float64(blockedRequests) / float64(totalRequests)
		} else {
			traffic["blocked_ratio"] = float64(0)
		}
	}
	traffic["top_domains"] = topDomains
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
	traffic["top_hosts"] = topHosts

	for k, v := range crowdSecLocal {
		threatsLocal[k] = v
	}

	return map[string]any{
		"traffic": traffic,
		"threats": threatsLocal,
	}, nil
}

// topDomainsWithMethods returns the top 20 domains by hit count, each
// enriched with its method distribution (pre-fetched in one batched query to
// avoid an N+1 over the per-domain loop).
func (db *DB) topDomainsWithMethods(ctx context.Context, where string, args []any) ([]map[string]any, error) {
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
	if err := methodBatchRows.Err(); err != nil {
		return nil, err
	}

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
	return topDomains, rows.Err()
}

// topEndpointTraffic returns the top 30 (method, path, status) combinations
// by hit count.
func (db *DB) topEndpointTraffic(ctx context.Context, where string, args []any) ([]map[string]any, error) {
	rows, err := db.conn.QueryContext(ctx,
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
	defer func() { _ = rows.Close() }()
	topEndpoints := make([]map[string]any, 0)
	for rows.Next() {
		var method string
		var path string
		var status int
		var hits int64
		var bytes int64
		if err := rows.Scan(&method, &path, &status, &hits, &bytes); err != nil {
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
	return topEndpoints, rows.Err()
}

// topHostTraffic returns the top 20 hosts by hit count, joined against
// `hosts` for their display name. Unlike the threats-side "most targeted
// hosts" (see fillWebLogsThreats), host_id here is a real hosts.id from the
// JOIN, safe to link to /hosts/:id.
func (db *DB) topHostTraffic(ctx context.Context, where string, args []any) ([]map[string]any, error) {
	rows, err := db.conn.QueryContext(ctx,
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
	defer func() { _ = rows.Close() }()
	topHosts := make([]map[string]any, 0)
	for rows.Next() {
		var id, name string
		var hits int64
		var bytes int64
		if err := rows.Scan(&id, &name, &hits, &bytes); err != nil {
			return nil, err
		}
		topHosts = append(topHosts, map[string]any{
			"host_id":   id,
			"host_name": name,
			"hits":      hits,
			"bytes":     bytes,
		})
	}
	return topHosts, rows.Err()
}

// GetWebLogsThreats computes only the threats portion of the summary (suspicious
// activity + CrowdSec decisions + a blocked-IP count). It deliberately skips the
// traffic aggregates — which are unindexed full-table scans over the window —
// and the geolocation/compare enrichment, so the threats-only BotView stays fast
// even on large windows. Every query here is served by an index leading with
// `suspicious`, `blocked` or the snapshot indexes.
func (db *DB) GetWebLogsThreats(ctx context.Context, since, until time.Time, hostID string, source string) (map[string]any, error) {
	where, args := buildWebLogsWhere(since, until, hostID, source)

	var errs errCollector
	var wg sync.WaitGroup
	wg.Add(3)

	var blockedIPs int64
	var blockedOK bool
	threatsLocal := map[string]any{}
	crowdSecLocal := map[string]any{}

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.threats.blocked")
		// blocked_ips: the only blocked statistic the BotView displays. Indexed by
		// idx_web_log_requests_blocked_captured / idx_web_log_requests_ip_blocked.
		if err := db.conn.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT COALESCE(COUNT(DISTINCT ip),0)
			FROM web_log_requests WHERE %s AND blocked = TRUE`, where),
			args...,
		).Scan(&blockedIPs); err == nil {
			blockedOK = true
		}
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.threats.suspicious")
		errs.set(db.fillWebLogsThreats(ctx, threatsLocal, where, args))
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "weblogs.threats.crowdsec")
		errs.set(db.fillWebLogsCrowdSec(ctx, crowdSecLocal, where, args, hostID))
	}()

	wg.Wait()
	if err := errs.first(); err != nil {
		return nil, err
	}

	if blockedOK {
		threatsLocal["blocked_ips"] = blockedIPs
	}
	for k, v := range crowdSecLocal {
		threatsLocal[k] = v
	}
	return threatsLocal, nil
}

// fillWebLogsThreats populates the suspicious-activity threat signals (counts,
// top IPs, top scanned paths, most-targeted domains, IP×domain scan matrix)
// into the provided threats map. All queries filter on `suspicious = TRUE` and
// are served by idx_web_log_requests_suspicious_captured.
func (db *DB) fillWebLogsThreats(ctx context.Context, threats map[string]any, where string, args []any) error {
	var suspiciousRequests int64
	var suspiciousIPs int64
	var targetedHosts int64
	if err := db.conn.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT COALESCE(COUNT(*),0), COALESCE(COUNT(DISTINCT ip),0), COALESCE(COUNT(DISTINCT COALESCE(NULLIF(domain,''), '(unknown)')),0)
		FROM web_log_requests
		WHERE %s AND suspicious = TRUE`, where),
		args...,
	).Scan(&suspiciousRequests, &suspiciousIPs, &targetedHosts); err != nil {
		return err
	}
	threats["suspicious_requests"] = suspiciousRequests
	threats["suspicious_ips"] = suspiciousIPs
	threats["targeted_hosts"] = targetedHosts

	// The per-category/per-status breakdown below feeds threatdetect.Score,
	// which the weblogs service applies once it also has the admin's
	// configurable Weights (the database layer has no config dependency).
	// LIMIT 500 here is only a coarse safety cap on a pathological window —
	// the service re-ranks by computed score and keeps the top 25, so this
	// must stay well above 25 or a low-hit-but-high-severity IP could be cut
	// before it ever gets scored. Category literals must match the
	// threatdetect.Category* constants.
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
		MAX(CASE WHEN blocked = TRUE THEN 1 ELSE 0 END) AS is_blocked,
		COALESCE(SUM(CASE WHEN category = 'WordPress' THEN 1 ELSE 0 END),0) AS cat_wordpress,
		COALESCE(SUM(CASE WHEN category = 'AdminPanel' THEN 1 ELSE 0 END),0) AS cat_adminpanel,
		COALESCE(SUM(CASE WHEN category = 'PathTraversal' THEN 1 ELSE 0 END),0) AS cat_pathtraversal,
		COALESCE(SUM(CASE WHEN category = 'KnownScanner' THEN 1 ELSE 0 END),0) AS cat_knownscanner,
		COALESCE(SUM(CASE WHEN category = 'SuspiciousMethod' THEN 1 ELSE 0 END),0) AS cat_suspiciousmethod,
		COALESCE(SUM(CASE WHEN status BETWEEN 200 AND 299 THEN 1 ELSE 0 END),0) AS status_2xx,
		COALESCE(SUM(CASE WHEN status BETWEEN 300 AND 399 THEN 1 ELSE 0 END),0) AS status_3xx,
		COALESCE(SUM(CASE WHEN status = 404 THEN 1 ELSE 0 END),0) AS status_404,
		COALESCE(SUM(CASE WHEN status BETWEEN 400 AND 499 AND status <> 404 THEN 1 ELSE 0 END),0) AS status_4xx_other,
		COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0) AS status_5xx
		FROM web_log_requests
		WHERE %s AND suspicious = TRUE
		GROUP BY ip
		ORDER BY hits DESC
		LIMIT 500`, where),
		args...,
	)
	if err != nil {
		return err
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
		var cat threatdetect.CategoryCounts
		var st threatdetect.StatusCounts
		if err := ipRows.Scan(&ip, &hits, &uniquePaths, &hostCount, &firstSeen, &lastSeen, &blockedSource, &blockedReason, &blockedAt, &blockedUntil, &isBlocked,
			&cat.WordPress, &cat.AdminPanel, &cat.PathTraversal, &cat.KnownScanner, &cat.SuspiciousMethod,
			&st.Status2xx, &st.Status3xx, &st.Status404, &st.Status4xxOther, &st.Status5xx,
		); err != nil {
			return err
		}
		ipData := map[string]any{
			"ip":               ip,
			"hits":             hits,
			"unique_paths":     uniquePaths,
			"host_count":       hostCount,
			"first_seen":       firstSeen,
			"last_seen":        lastSeen,
			"_category_counts": cat,
			"_status_counts":   st,
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
		return err
	}
	defer func() { _ = pathsRows.Close() }()
	topPaths := make([]map[string]any, 0)
	for pathsRows.Next() {
		var path, category string
		var hits int64
		if err := pathsRows.Scan(&path, &category, &hits); err != nil {
			return err
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
		return err
	}
	defer func() { _ = hostRows.Close() }()
	mostTargetedHosts := make([]map[string]any, 0)
	for hostRows.Next() {
		var vhost string
		var hits int64
		if err := hostRows.Scan(&vhost, &hits); err != nil {
			return err
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
		return err
	}
	defer func() { _ = matrixRows.Close() }()
	ipHostMatrix := make([]map[string]any, 0)
	for matrixRows.Next() {
		var ip string
		var hostCount int64
		var hits int64
		if err := matrixRows.Scan(&ip, &hostCount, &hits); err != nil {
			return err
		}
		ipHostMatrix = append(ipHostMatrix, map[string]any{"ip": ip, "host_count": hostCount, "hits": hits})
	}
	threats["ip_host_matrix"] = ipHostMatrix

	return nil
}

// fillWebLogsCrowdSec populates the CrowdSec decision signals (active blocked-IP
// count + the most recent per-IP decisions) into the provided threats map from
// the latest web_log_snapshots. The same window/host/source WHERE clause applies
// to web_log_snapshots, which carries captured_at / host_id / source columns.
func (db *DB) fillWebLogsCrowdSec(ctx context.Context, threats map[string]any, where string, args []any, hostID string) error {
	var crowdSecBlocked int64
	var crowdSecHostCount int64
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
		WHERE ip IS NOT NULL AND ip <> ''`, where)
	if err := db.conn.QueryRowContext(ctx, countQuery, args...).Scan(&crowdSecBlocked, &crowdSecHostCount); err != nil {
		return err
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
			LIMIT %d`, where, crowdSecTopBlockedLimit)
		rows, err := db.conn.QueryContext(ctx, listQuery, args...)
		if err != nil {
			return err
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
				return err
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
			return err
		}
		threats["crowdsec_top_blocked"] = csEntries
	}

	if hostID != "" {
		threats["crowdsec_host_id"] = hostID
	} else if crowdSecHostCount == 1 {
		var singleHostID string
		if err := db.conn.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT MAX(host_id) FROM web_log_snapshots WHERE %s`, where),
			args...,
		).Scan(&singleHostID); err == nil && singleHostID != "" {
			threats["crowdsec_host_id"] = singleHostID
		}
	}

	return nil
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
func (db *DB) GetWebLogsTimeseries(ctx context.Context, since, until time.Time, hostID string, source string, bucket string) ([]map[string]any, error) {
	if bucket != "minute" && bucket != "hour" {
		bucket = "hour"
	}

	where, args := buildWebLogsWhere(since, until, hostID, source)

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
