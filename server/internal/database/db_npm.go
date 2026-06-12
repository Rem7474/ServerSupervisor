package database

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// ========== NPM Connections ==========

// NPMConnectionFull includes the secret — only used by the background poller.
// It is never returned to HTTP clients.
type NPMConnectionFull struct {
	models.NPMConnection
	Secret string
}

func (db *DB) CreateNPMConnection(ctx context.Context, req models.NPMConnectionRequest) (*models.NPMConnection, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	interval := req.PollIntervalSec
	if interval <= 0 {
		interval = 3600
	}
	var id string
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO npm_connections (name, api_url, identity, secret, host_id, enabled, poll_interval_sec)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		req.Name, req.APIURL, req.Identity, req.Secret, req.HostID, enabled, interval,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return db.GetNPMConnectionByID(ctx, id)
}

func (db *DB) ListNPMConnections(ctx context.Context) ([]models.NPMConnection, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT c.id, c.name, c.api_url, c.identity, c.host_id, c.enabled,
		        c.poll_interval_sec, c.last_error, c.last_error_at, c.last_success_at,
		        c.created_at, c.updated_at,
		        COUNT(p.id) AS proxy_host_count
		 FROM npm_connections c
		 LEFT JOIN npm_proxy_hosts p ON p.connection_id = c.id
		 GROUP BY c.id
		 ORDER BY c.name ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []models.NPMConnection
	for rows.Next() {
		var c models.NPMConnection
		if err := rows.Scan(
			&c.ID, &c.Name, &c.APIURL, &c.Identity, &c.HostID, &c.Enabled,
			&c.PollIntervalSec, &c.LastError, &c.LastErrorAt, &c.LastSuccessAt,
			&c.CreatedAt, &c.UpdatedAt, &c.ProxyHostCount,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) GetNPMConnectionByID(ctx context.Context, id string) (*models.NPMConnection, error) {
	var c models.NPMConnection
	err := db.conn.QueryRowContext(ctx,
		`SELECT c.id, c.name, c.api_url, c.identity, c.host_id, c.enabled,
		        c.poll_interval_sec, c.last_error, c.last_error_at, c.last_success_at,
		        c.created_at, c.updated_at,
		        COUNT(p.id) AS proxy_host_count
		 FROM npm_connections c
		 LEFT JOIN npm_proxy_hosts p ON p.connection_id = c.id
		 WHERE c.id = $1
		 GROUP BY c.id`, id,
	).Scan(
		&c.ID, &c.Name, &c.APIURL, &c.Identity, &c.HostID, &c.Enabled,
		&c.PollIntervalSec, &c.LastError, &c.LastErrorAt, &c.LastSuccessAt,
		&c.CreatedAt, &c.UpdatedAt, &c.ProxyHostCount,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetEnabledNPMConnections returns all enabled connections including the secret,
// for use by the background poller only.
func (db *DB) GetEnabledNPMConnections(ctx context.Context) ([]NPMConnectionFull, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, api_url, identity, secret, host_id, enabled,
		        poll_interval_sec, last_error, last_error_at, last_success_at,
		        created_at, updated_at
		 FROM npm_connections WHERE enabled = true`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []NPMConnectionFull
	for rows.Next() {
		var f NPMConnectionFull
		c := &f.NPMConnection
		if err := rows.Scan(
			&c.ID, &c.Name, &c.APIURL, &c.Identity, &f.Secret, &c.HostID, &c.Enabled,
			&c.PollIntervalSec, &c.LastError, &c.LastErrorAt, &c.LastSuccessAt,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetNPMConnectionSecret retrieves only the secret for a connection. Used by
// the preview and import endpoints that need to authenticate against NPM.
func (db *DB) GetNPMConnectionSecret(ctx context.Context, id string) (string, error) {
	var secret string
	return secret, db.conn.QueryRowContext(ctx, `SELECT secret FROM npm_connections WHERE id = $1`, id).Scan(&secret)
}

func (db *DB) UpdateNPMConnection(ctx context.Context, id string, req models.NPMConnectionRequest) (*models.NPMConnection, error) {
	interval := req.PollIntervalSec
	if interval <= 0 {
		interval = 3600
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	// Preserve existing secret when not provided.
	if req.Secret != "" {
		_, err := db.conn.ExecContext(ctx,
			`UPDATE npm_connections SET name=$2, api_url=$3, identity=$4, secret=$5,
			 host_id=$6, enabled=$7, poll_interval_sec=$8, updated_at=NOW()
			 WHERE id=$1`,
			id, req.Name, req.APIURL, req.Identity, req.Secret, req.HostID, enabled, interval,
		)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := db.conn.ExecContext(ctx,
			`UPDATE npm_connections SET name=$2, api_url=$3, identity=$4,
			 host_id=$5, enabled=$6, poll_interval_sec=$7, updated_at=NOW()
			 WHERE id=$1`,
			id, req.Name, req.APIURL, req.Identity, req.HostID, enabled, interval,
		)
		if err != nil {
			return nil, err
		}
	}
	return db.GetNPMConnectionByID(ctx, id)
}

func (db *DB) DeleteNPMConnection(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM npm_connections WHERE id = $1`, id)
	return err
}

func (db *DB) UpdateNPMConnectionSuccess(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE npm_connections SET last_error='', last_error_at=NULL, last_success_at=NOW(), updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (db *DB) UpdateNPMConnectionError(ctx context.Context, id string, errMsg string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE npm_connections SET last_error=$2, last_error_at=NOW(), updated_at=NOW() WHERE id=$1`, id, errMsg)
	return err
}

// ========== NPM Proxy Hosts ==========

func scanNPMProxyHost(rows interface {
	Scan(...any) error
}) (models.NPMProxyHost, error) {
	var h models.NPMProxyHost
	return h, rows.Scan(
		&h.ID, &h.ConnectionID, &h.NPMID,
		pq.Array(&h.DomainNames),
		&h.ForwardHost, &h.ForwardPort, &h.SSLEnabled, &h.NPMEnabled,
		&h.MonitoringEnabled, &h.UptimeMonitoringEnabled, &h.SSLMonitoringEnabled,
		&h.UptimeProbeID, &h.SSLCertificateID,
		&h.LastSeenAt, &h.CreatedAt, &h.UpdatedAt,
	)
}

const npmProxyHostColumns = `id, connection_id, npm_id, domain_names,
	forward_host, forward_port, ssl_enabled, npm_enabled,
	monitoring_enabled, uptime_monitoring_enabled, ssl_monitoring_enabled,
	uptime_probe_id, ssl_certificate_id, last_seen_at, created_at, updated_at`

func (db *DB) ListNPMProxyHosts(ctx context.Context, connectionID string) ([]models.NPMProxyHost, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+npmProxyHostColumns+`
		 FROM npm_proxy_hosts WHERE connection_id = $1
		 ORDER BY domain_names[1] ASC`, connectionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []models.NPMProxyHost
	for rows.Next() {
		h, err := scanNPMProxyHost(rows)
		if err != nil {
			return nil, err
		}
		if h.DomainNames == nil {
			h.DomainNames = []string{}
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// UpsertNPMProxyHost inserts or updates a proxy host record (keyed on connection_id + npm_id).
func (db *DB) UpsertNPMProxyHost(ctx context.Context, h models.NPMProxyHost) (*models.NPMProxyHost, error) {
	domains := h.DomainNames
	if domains == nil {
		domains = []string{}
	}
	var id string
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO npm_proxy_hosts
		 (connection_id, npm_id, domain_names, forward_host, forward_port, ssl_enabled, npm_enabled, last_seen_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
		 ON CONFLICT (connection_id, npm_id) DO UPDATE
		   SET domain_names=$3, forward_host=$4, forward_port=$5, ssl_enabled=$6,
		       npm_enabled=$7, last_seen_at=NOW(), updated_at=NOW()
		 RETURNING id`,
		h.ConnectionID, h.NPMID, pq.Array(domains), h.ForwardHost, h.ForwardPort, h.SSLEnabled, h.NPMEnabled,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return db.GetNPMProxyHostByID(ctx, id)
}

func (db *DB) GetNPMProxyHostByID(ctx context.Context, id string) (*models.NPMProxyHost, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT `+npmProxyHostColumns+` FROM npm_proxy_hosts WHERE id = $1`, id)
	h, err := scanNPMProxyHost(row)
	if err != nil {
		return nil, err
	}
	if h.DomainNames == nil {
		h.DomainNames = []string{}
	}
	return &h, nil
}

// UpdateNPMProxyHostLinks sets the uptime probe and SSL cert links after import.
func (db *DB) UpdateNPMProxyHostLinks(ctx context.Context, id string, probeID, certID *string) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE npm_proxy_hosts SET uptime_probe_id=$2, ssl_certificate_id=$3, updated_at=NOW() WHERE id=$1`,
		id, probeID, certID,
	)
	return err
}

// GetNPMProxyHostsByConnectionNPMIDs returns existing proxy-host records keyed by npm_id
// for a connection. Used by the preview endpoint to determine which hosts are already imported.
func (db *DB) GetNPMProxyHostsByConnectionNPMIDs(ctx context.Context, connectionID string) (map[int]models.NPMProxyHost, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT `+npmProxyHostColumns+`
		 FROM npm_proxy_hosts WHERE connection_id = $1`, connectionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[int]models.NPMProxyHost)
	for rows.Next() {
		h, err := scanNPMProxyHost(rows)
		if err != nil {
			return nil, err
		}
		if h.DomainNames == nil {
			h.DomainNames = []string{}
		}
		out[h.NPMID] = h
	}
	return out, rows.Err()
}

// RefreshNPMProxyHostSeen updates last_seen_at and npm_enabled for an already-imported host.
func (db *DB) RefreshNPMProxyHostSeen(ctx context.Context, connectionID string, npmID int, npmEnabled bool, lastSeenAt time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE npm_proxy_hosts SET npm_enabled=$3, last_seen_at=$4, updated_at=NOW()
		 WHERE connection_id=$1 AND npm_id=$2`,
		connectionID, npmID, npmEnabled, lastSeenAt,
	)
	return err
}

// UpdateNPMProxyHostSettings persists the three monitoring toggle columns.
func (db *DB) UpdateNPMProxyHostSettings(ctx context.Context, id string, monitoring, uptime, ssl bool) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE npm_proxy_hosts
		 SET monitoring_enabled=$2, uptime_monitoring_enabled=$3, ssl_monitoring_enabled=$4, updated_at=NOW()
		 WHERE id=$1`,
		id, monitoring, uptime, ssl,
	)
	return err
}

// ListAllNPMProxyHostsEnriched returns every imported proxy host joined with its
// connection name and live status from the linked uptime probe and SSL cert.
func (db *DB) ListAllNPMProxyHostsEnriched(ctx context.Context) ([]models.NPMProxyHostEnriched, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT p.id, p.connection_id, p.npm_id, p.domain_names,
		       p.forward_host, p.forward_port, p.ssl_enabled, p.npm_enabled,
		       p.monitoring_enabled, p.uptime_monitoring_enabled, p.ssl_monitoring_enabled,
		       p.uptime_probe_id, p.ssl_certificate_id, p.last_seen_at, p.created_at, p.updated_at,
		       c.name AS connection_name,
		       COALESCE(u.last_status, '') AS uptime_status,
		       u.last_latency_ms,
		       s.days_remaining AS ssl_days_remaining
		FROM npm_proxy_hosts p
		JOIN npm_connections c ON c.id = p.connection_id
		LEFT JOIN uptime_probes u ON u.id = p.uptime_probe_id
		LEFT JOIN ssl_certificates s ON s.id = p.ssl_certificate_id
		ORDER BY c.name ASC, p.domain_names[1] ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []models.NPMProxyHostEnriched
	for rows.Next() {
		var e models.NPMProxyHostEnriched
		h := &e.NPMProxyHost
		if err := rows.Scan(
			&h.ID, &h.ConnectionID, &h.NPMID,
			pq.Array(&h.DomainNames),
			&h.ForwardHost, &h.ForwardPort, &h.SSLEnabled, &h.NPMEnabled,
			&h.MonitoringEnabled, &h.UptimeMonitoringEnabled, &h.SSLMonitoringEnabled,
			&h.UptimeProbeID, &h.SSLCertificateID, &h.LastSeenAt, &h.CreatedAt, &h.UpdatedAt,
			&e.ConnectionName, &e.UptimeStatus, &e.UptimeLastLatencyMs, &e.SSLDaysRemaining,
		); err != nil {
			return nil, err
		}
		if h.DomainNames == nil {
			h.DomainNames = []string{}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
