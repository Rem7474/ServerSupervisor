package database

import (
	"context"
	"database/sql"

	"github.com/serversupervisor/server/internal/models"
)

// CreateProxmoxConnection inserts a new connection and returns its ID.
func (db *DB) CreateProxmoxConnection(ctx context.Context, name, apiURL, tokenID, tokenSecret string, insecureSkipVerify, enabled bool, pollIntervalSec int) (string, error) {
	if pollIntervalSec <= 0 {
		pollIntervalSec = 60
	}
	var id string
	err := db.conn.QueryRowContext(ctx, `
		INSERT INTO proxmox_connections (name, api_url, token_id, token_secret, insecure_skip_verify, enabled, poll_interval_sec)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		name, apiURL, tokenID, tokenSecret, insecureSkipVerify, enabled, pollIntervalSec,
	).Scan(&id)
	return id, err
}

// ListProxmoxConnections returns all connections without secrets.
// Node and guest counts are joined.
func (db *DB) ListProxmoxConnections(ctx context.Context) ([]models.ProxmoxConnection, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT
			c.id, c.name, c.api_url, c.token_id,
			c.insecure_skip_verify, c.enabled, c.poll_interval_sec,
			c.last_error, c.last_error_at, c.last_success_at,
			c.created_at, c.updated_at,
			COUNT(DISTINCT n.id) AS node_count,
			COUNT(DISTINCT g.id) AS guest_count
		FROM proxmox_connections c
		LEFT JOIN proxmox_nodes   n ON n.connection_id = c.id
		LEFT JOIN proxmox_guests  g ON g.connection_id = c.id
		GROUP BY c.id
		ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var conns []models.ProxmoxConnection
	for rows.Next() {
		var c models.ProxmoxConnection
		var lastErrAt, lastSuccAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.Name, &c.APIURL, &c.TokenID,
			&c.InsecureSkipVerify, &c.Enabled, &c.PollIntervalSec,
			&c.LastError, &lastErrAt, &lastSuccAt,
			&c.CreatedAt, &c.UpdatedAt,
			&c.NodeCount, &c.GuestCount,
		); err != nil {
			return nil, err
		}
		if lastErrAt.Valid {
			t := lastErrAt.Time
			c.LastErrorAt = &t
		}
		if lastSuccAt.Valid {
			t := lastSuccAt.Time
			c.LastSuccessAt = &t
		}
		conns = append(conns, c)
	}
	if conns == nil {
		conns = []models.ProxmoxConnection{}
	}
	return conns, rows.Err()
}

// GetProxmoxConnectionByID returns a connection without secret.
func (db *DB) GetProxmoxConnectionByID(ctx context.Context, id string) (*models.ProxmoxConnection, error) {
	var c models.ProxmoxConnection
	var lastErrAt, lastSuccAt sql.NullTime
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, name, api_url, token_id, insecure_skip_verify, enabled, poll_interval_sec,
		       last_error, last_error_at, last_success_at, created_at, updated_at
		FROM proxmox_connections WHERE id = $1`, id).Scan(
		&c.ID, &c.Name, &c.APIURL, &c.TokenID,
		&c.InsecureSkipVerify, &c.Enabled, &c.PollIntervalSec,
		&c.LastError, &lastErrAt, &lastSuccAt,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastErrAt.Valid {
		t := lastErrAt.Time
		c.LastErrorAt = &t
	}
	if lastSuccAt.Valid {
		t := lastSuccAt.Time
		c.LastSuccessAt = &t
	}
	return &c, nil
}

// ProxmoxConnectionFull is a connection record including its token secret.
// Reserved for the poller — never returned to API clients.
type ProxmoxConnectionFull struct {
	models.ProxmoxConnection
	TokenSecret string
}

// GetEnabledProxmoxConnections returns enabled connections WITH their secrets (for the poller).
func (db *DB) GetEnabledProxmoxConnections(ctx context.Context) ([]ProxmoxConnectionFull, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, name, api_url, token_id, token_secret,
		       insecure_skip_verify, enabled, poll_interval_sec,
		       last_error, last_error_at, last_success_at, created_at, updated_at
		FROM proxmox_connections WHERE enabled = TRUE ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var conns []ProxmoxConnectionFull
	for rows.Next() {
		var c ProxmoxConnectionFull
		var lastErrAt, lastSuccAt sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.Name, &c.APIURL, &c.TokenID, &c.TokenSecret,
			&c.InsecureSkipVerify, &c.Enabled, &c.PollIntervalSec,
			&c.LastError, &lastErrAt, &lastSuccAt,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if lastErrAt.Valid {
			t := lastErrAt.Time
			c.LastErrorAt = &t
		}
		if lastSuccAt.Valid {
			t := lastSuccAt.Time
			c.LastSuccessAt = &t
		}
		conns = append(conns, c)
	}
	return conns, rows.Err()
}

// UpdateProxmoxConnection updates mutable fields.
// If tokenSecret is empty the existing secret is preserved.
func (db *DB) UpdateProxmoxConnection(ctx context.Context, id, name, apiURL, tokenID, tokenSecret string, insecureSkipVerify, enabled bool, pollIntervalSec int) error {
	if pollIntervalSec <= 0 {
		pollIntervalSec = 60
	}
	if tokenSecret == "" {
		_, err := db.conn.ExecContext(ctx, `
			UPDATE proxmox_connections
			SET name=$2, api_url=$3, token_id=$4,
			    insecure_skip_verify=$5, enabled=$6, poll_interval_sec=$7, updated_at=NOW()
			WHERE id=$1`,
			id, name, apiURL, tokenID, insecureSkipVerify, enabled, pollIntervalSec)
		return err
	}
	_, err := db.conn.ExecContext(ctx, `
		UPDATE proxmox_connections
		SET name=$2, api_url=$3, token_id=$4, token_secret=$5,
		    insecure_skip_verify=$6, enabled=$7, poll_interval_sec=$8, updated_at=NOW()
		WHERE id=$1`,
		id, name, apiURL, tokenID, tokenSecret, insecureSkipVerify, enabled, pollIntervalSec)
	return err
}

// DeleteProxmoxConnection removes a connection (cascade deletes nodes/guests/storages).
func (db *DB) DeleteProxmoxConnection(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM proxmox_connections WHERE id = $1`, id)
	return err
}

// UpdateProxmoxConnectionSuccess records a successful poll.
func (db *DB) UpdateProxmoxConnectionSuccess(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `
		UPDATE proxmox_connections SET last_error='', last_error_at=NULL, last_success_at=NOW(), updated_at=NOW()
		WHERE id=$1`, id)
	return err
}

// UpdateProxmoxConnectionError records a poll error.
func (db *DB) UpdateProxmoxConnectionError(ctx context.Context, id, errMsg string) error {
	_, err := db.conn.ExecContext(ctx, `
		UPDATE proxmox_connections SET last_error=$2, last_error_at=NOW(), updated_at=NOW()
		WHERE id=$1`, id, errMsg)
	return err
}

// GetProxmoxTokenSecret returns only the token secret for a connection.
func (db *DB) GetProxmoxTokenSecret(ctx context.Context, id string) (string, error) {
	var secret string
	err := db.conn.QueryRowContext(ctx, `SELECT token_secret FROM proxmox_connections WHERE id=$1`, id).Scan(&secret)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return secret, err
}
