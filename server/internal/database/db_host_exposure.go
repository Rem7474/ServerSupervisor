package database

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// domainWebLogStats is the aggregated web_log_requests row for one domain,
// used only to build HostExposedDomain entries.
type domainWebLogStats struct {
	requests, bytes, errors4xx, errors5xx, suspicious, blocked int64
}

// GetHostExposure resolves every NPM proxy host whose forward_host matches
// the given IP address, then aggregates web_log_requests traffic for their
// domain names since the given time. ip is looked up by the caller (usually
// the host's own hosts.ip_address); an empty ip returns an empty result
// without querying, since npm_proxy_hosts.forward_host defaults to ” and a
// blind match would false-positive against any not-yet-synced proxy host.
func (db *DB) GetHostExposure(ctx context.Context, ip string, since time.Time) (*models.HostExposure, error) {
	result := &models.HostExposure{IPAddress: ip, Since: since, Domains: []models.HostExposedDomain{}}
	if ip == "" {
		return result, nil
	}
	byIP, err := db.GetExposureByIPs(ctx, []string{ip}, since)
	if err != nil {
		return nil, err
	}
	if exp, ok := byIP[ip]; ok {
		return exp, nil
	}
	return result, nil
}

// GetExposureByIPs is the batched form of GetHostExposure: it resolves every
// NPM proxy host whose forward_host matches any of the given IPs in a single
// query (used for Proxmox guests, which — unlike a ServerSupervisor host —
// have no persisted IP column and may report several interfaces), grouping
// results by forward_host. IPs with no matching proxy host are simply absent
// from the returned map. An empty ips slice short-circuits before querying,
// same reasoning as GetHostExposure's empty-ip guard.
func (db *DB) GetExposureByIPs(ctx context.Context, ips []string, since time.Time) (map[string]*models.HostExposure, error) {
	result := map[string]*models.HostExposure{}
	if len(ips) == 0 {
		return result, nil
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT p.id, p.connection_id, c.name, p.domain_names, p.forward_port, p.ssl_enabled, p.npm_enabled, p.forward_host
		FROM npm_proxy_hosts p
		JOIN npm_connections c ON c.id = p.connection_id
		WHERE p.forward_host = ANY($1)
		ORDER BY p.forward_host, p.domain_names[1] ASC`, pq.Array(ips))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	type proxyHostRow struct {
		ip                               string
		id, connectionID, connectionName string
		domains                          []string
		forwardPort                      int
		sslEnabled, npmEnabled           bool
	}
	var proxyHosts []proxyHostRow
	var allDomains []string
	for rows.Next() {
		var r proxyHostRow
		if err := rows.Scan(&r.id, &r.connectionID, &r.connectionName, pq.Array(&r.domains), &r.forwardPort, &r.sslEnabled, &r.npmEnabled, &r.ip); err != nil {
			return nil, err
		}
		proxyHosts = append(proxyHosts, r)
		allDomains = append(allDomains, r.domains...)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(proxyHosts) == 0 {
		return result, nil
	}

	stats, err := db.getWebLogStatsByDomains(ctx, allDomains, since)
	if err != nil {
		return nil, err
	}

	for _, ph := range proxyHosts {
		agg := models.HostExposedDomain{
			ProxyHostID:    ph.id,
			ConnectionID:   ph.connectionID,
			ConnectionName: ph.connectionName,
			DomainNames:    ph.domains,
			ForwardPort:    ph.forwardPort,
			SSLEnabled:     ph.sslEnabled,
			NPMEnabled:     ph.npmEnabled,
		}
		for _, d := range ph.domains {
			if s, ok := stats[d]; ok {
				agg.Requests += s.requests
				agg.Bytes += s.bytes
				agg.Errors4xx += s.errors4xx
				agg.Errors5xx += s.errors5xx
				agg.SuspiciousRequests += s.suspicious
				agg.BlockedRequests += s.blocked
			}
		}
		exp, ok := result[ph.ip]
		if !ok {
			exp = &models.HostExposure{IPAddress: ph.ip, Since: since, Domains: []models.HostExposedDomain{}}
			result[ph.ip] = exp
		}
		exp.Domains = append(exp.Domains, agg)
		exp.TotalRequests += agg.Requests
		exp.TotalSuspicious += agg.SuspiciousRequests
		exp.TotalBlocked += agg.BlockedRequests
	}
	return result, nil
}

// getWebLogStatsByDomains aggregates web_log_requests by domain for the given
// domain list since the given time. Domains with no matching rows are simply
// absent from the returned map.
func (db *DB) getWebLogStatsByDomains(ctx context.Context, domains []string, since time.Time) (map[string]domainWebLogStats, error) {
	out := map[string]domainWebLogStats{}
	if len(domains) == 0 {
		return out, nil
	}
	rows, err := db.conn.QueryContext(ctx, `
		SELECT domain,
		       COUNT(*),
		       COALESCE(SUM(bytes),0),
		       COALESCE(SUM(CASE WHEN status BETWEEN 400 AND 499 THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN suspicious THEN 1 ELSE 0 END),0),
		       COALESCE(SUM(CASE WHEN blocked THEN 1 ELSE 0 END),0)
		FROM web_log_requests
		WHERE domain = ANY($1) AND captured_at >= $2
		GROUP BY domain`, pq.Array(domains), since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var domain string
		var s domainWebLogStats
		if err := rows.Scan(&domain, &s.requests, &s.bytes, &s.errors4xx, &s.errors5xx, &s.suspicious, &s.blocked); err != nil {
			return nil, err
		}
		out[domain] = s
	}
	return out, rows.Err()
}
