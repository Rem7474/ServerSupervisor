package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// GetIPTimeline returns the per-request timeline for a single source IP,
// used by the IP-detail drawer (block/unblock + per-host activity).
func (db *DB) GetIPTimeline(ctx context.Context, ip string, since, until time.Time, hostID string, limit int) ([]models.WebLogIPTimelineRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	args := []any{ip, since}
	where := "r.ip = $1 AND r.captured_at >= $2"
	if !until.IsZero() {
		args = append(args, until)
		where += fmt.Sprintf(" AND r.captured_at < $%d", len(args))
	}
	if hostID != "" {
		args = append(args, hostID)
		where += fmt.Sprintf(" AND r.host_id = $%d", len(args))
	}
	args = append(args, limit)

	rows, err := db.conn.QueryContext(ctx,
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

// DomainDetailsFilter narrows GetDomainDetails' scope. Status/Method/Path/IP
// apply to every query it runs (the KPI aggregate, top paths, top clients and
// the recent-requests page all share one WHERE) — clicking a "top path" row
// to scope the whole drawer down to just that path, for example, is meant to
// recompute the KPIs/top-clients for that path too, not just filter the
// requests table under it. Sort/Dir only affect the recent-requests page; the
// top-N lists stay fixed at their own hits-ranked LIMIT 30.
type DomainDetailsFilter struct {
	Status string // "" | "2xx" | "3xx" | "4xx" | "5xx" | "blocked" | "suspicious"
	Method string
	Path   string
	IP     string
	Sort   string // "time" (default) | "status" | "bytes"
	Dir    string // "desc" (default) | "asc"
}

// domainDetailsSortColumns whitelists Sort → real column, so it's never
// interpolated into SQL from unvalidated user input.
var domainDetailsSortColumns = map[string]string{
	"time":   "captured_at",
	"status": "status",
	"bytes":  "bytes",
}

// buildDomainDetailsWhere builds the shared WHERE clause + args for
// GetDomainDetails' four queries. until being zero means "open ended".
func buildDomainDetailsWhere(domain string, since, until time.Time, hostID, source string, filter DomainDetailsFilter) (string, []any) {
	args := []any{since, domain}
	where := "captured_at >= $1 AND COALESCE(NULLIF(domain,''), '(unknown)') = $2"
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
	if filter.Method != "" {
		args = append(args, filter.Method)
		where += fmt.Sprintf(" AND method = $%d", len(args))
	}
	if filter.Path != "" {
		args = append(args, filter.Path)
		where += fmt.Sprintf(" AND path = $%d", len(args))
	}
	if filter.IP != "" {
		args = append(args, filter.IP)
		where += fmt.Sprintf(" AND ip = $%d", len(args))
	}
	switch filter.Status {
	case "2xx":
		where += " AND status BETWEEN 200 AND 299"
	case "3xx":
		where += " AND status BETWEEN 300 AND 399"
	case "4xx":
		where += " AND status BETWEEN 400 AND 499"
	case "5xx":
		where += " AND status >= 500"
	case "blocked":
		where += " AND blocked = TRUE"
	case "suspicious":
		where += " AND suspicious = TRUE"
	}
	return where, args
}

// GetDomainDetails aggregates per-domain statistics (top paths, top clients,
// recent requests) for the domain drawer. hits (always computed from the same
// filtered WHERE as the requests page) doubles as that page's pagination
// total — filter/sort/limit/offset are consistent with each other, so there's
// no separate COUNT query to keep in sync.
func (db *DB) GetDomainDetails(ctx context.Context, domain string, since, until time.Time, hostID string, source string, filter DomainDetailsFilter, limit, offset int) (map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 300
	}
	if offset < 0 {
		offset = 0
	}
	where, args := buildDomainDetailsWhere(domain, since, until, hostID, source, filter)

	out := map[string]any{}
	var hits int64
	var bytes int64
	var status2xx int64
	var status3xx int64
	var status4xx int64
	var status5xx int64
	if err := db.conn.QueryRowContext(ctx,
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
	out["total"] = hits // pagination total for the requests page — same filtered WHERE
	out["bytes"] = bytes
	out["status_2xx"] = status2xx
	out["status_3xx"] = status3xx
	out["status_4xx"] = status4xx
	out["status_5xx"] = status5xx

	pathsRows, err := db.conn.QueryContext(ctx,
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

	// MAX(host_id) picks an arbitrary-but-stable collector host when a
	// (rare) multi-host domain window mixes IDs — good enough to seed a
	// ban dispatch target for the "Bloquer" action, which the caller can
	// still override via its own host filter same as everywhere else.
	ipRows, err := db.conn.QueryContext(ctx,
		fmt.Sprintf(`SELECT ip, COUNT(*) AS hits, MAX(host_id) AS host_id,
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
		var ip, hostID string
		var hits int64
		var blockedSource sql.NullString
		var blockedReason sql.NullString
		var blockedAt sql.NullTime
		var blockedUntil sql.NullTime
		var isBlocked int
		if err := ipRows.Scan(&ip, &hits, &hostID, &blockedSource, &blockedReason, &blockedAt, &blockedUntil, &isBlocked); err != nil {
			return nil, err
		}
		clientData := map[string]any{"ip": ip, "hits": hits, "host_id": hostID}
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

	sortColumn := domainDetailsSortColumns[filter.Sort]
	if sortColumn == "" {
		sortColumn = "captured_at"
	}
	sortDir := "DESC"
	if filter.Dir == "asc" {
		sortDir = "ASC"
	}

	reqArgs := append(append([]any{}, args...), limit, offset)
	reqRows, err := db.conn.QueryContext(ctx,
		fmt.Sprintf(`SELECT captured_at, host_id, source, ip, method, path, status, bytes, COALESCE(user_agent,''), COALESCE(category,''), blocked, COALESCE(blocked_source,''), COALESCE(blocked_reason,''), blocked_at, blocked_until, suspicious
		FROM web_log_requests
		WHERE %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d`, where, sortColumn, sortDir, len(reqArgs)-1, len(reqArgs)),
		reqArgs...,
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
		var blocked, suspicious bool
		var blockedSource, blockedReason string
		var blockedAt, blockedUntil sql.NullTime
		if err := reqRows.Scan(&capturedAt, &rowHostID, &rowSource, &ip, &method, &path, &status, &bytes, &userAgent, &category, &blocked, &blockedSource, &blockedReason, &blockedAt, &blockedUntil, &suspicious); err != nil {
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
			"suspicious": suspicious,
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

// GetWebLogsLive returns the most recent N web log requests across all hosts,
// used by the live tail panel.
func (db *DB) GetWebLogsLive(ctx context.Context, hostID string, source string, limit int) ([]map[string]any, error) {
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

	rows, err := db.conn.QueryContext(ctx,
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

// GetWebLogsTopClientIPs returns the N most active source IPs over the
// requested window.
func (db *DB) GetWebLogsTopClientIPs(ctx context.Context, since, until time.Time, hostID string, source string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	where, args := buildWebLogsWhere(since, until, hostID, source)
	args = append(args, limit)

	rows, err := db.conn.QueryContext(ctx,
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
