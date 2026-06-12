package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/models"
)

// ========== SSL/TLS Certificate Monitoring ==========

func (db *DB) CreateSSLCertificate(ctx context.Context, c models.SSLCertificate) (*models.SSLCertificate, error) {
	if c.Port == 0 {
		c.Port = 443
	}
	dns := c.DNSNames
	if dns == nil {
		dns = []string{}
	}
	var out models.SSLCertificate
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO ssl_certificates (name, host, port, server_name, enabled, dns_names)
		 VALUES ($1,$2,$3,$4,$5,$6)
		 RETURNING id, name, host, port, server_name, enabled, last_checked_at,
		           valid_from, valid_to, issuer, subject, serial_number, dns_names,
		           last_error, created_at, updated_at`,
		c.Name, c.Host, c.Port, c.ServerName, c.Enabled, pq.Array(dns),
	).Scan(
		&out.ID, &out.Name, &out.Host, &out.Port, &out.ServerName, &out.Enabled, &out.LastCheckedAt,
		&out.ValidFrom, &out.ValidTo, &out.Issuer, &out.Subject, &out.SerialNumber, pq.Array(&out.DNSNames),
		&out.LastError, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if out.DNSNames == nil {
		out.DNSNames = []string{}
	}
	out.DaysRemaining = computeDaysRemaining(out.ValidTo)
	return &out, nil
}

func (db *DB) ListSSLCertificates(ctx context.Context) ([]models.SSLCertificate, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, host, port, server_name, enabled, last_checked_at,
		        valid_from, valid_to, issuer, subject, serial_number, dns_names,
		        last_error, created_at, updated_at
		 FROM ssl_certificates
		 ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.SSLCertificate
	for rows.Next() {
		var c models.SSLCertificate
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Host, &c.Port, &c.ServerName, &c.Enabled, &c.LastCheckedAt,
			&c.ValidFrom, &c.ValidTo, &c.Issuer, &c.Subject, &c.SerialNumber, pq.Array(&c.DNSNames),
			&c.LastError, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if c.DNSNames == nil {
			c.DNSNames = []string{}
		}
		c.DaysRemaining = computeDaysRemaining(c.ValidTo)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (db *DB) GetSSLCertificate(ctx context.Context, id string) (*models.SSLCertificate, error) {
	var c models.SSLCertificate
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, name, host, port, server_name, enabled, last_checked_at,
		        valid_from, valid_to, issuer, subject, serial_number, dns_names,
		        last_error, created_at, updated_at
		 FROM ssl_certificates WHERE id = $1`, id,
	).Scan(
		&c.ID, &c.Name, &c.Host, &c.Port, &c.ServerName, &c.Enabled, &c.LastCheckedAt,
		&c.ValidFrom, &c.ValidTo, &c.Issuer, &c.Subject, &c.SerialNumber, pq.Array(&c.DNSNames),
		&c.LastError, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if c.DNSNames == nil {
		c.DNSNames = []string{}
	}
	c.DaysRemaining = computeDaysRemaining(c.ValidTo)
	return &c, nil
}

func (db *DB) UpdateSSLCertificate(ctx context.Context, c models.SSLCertificate) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE ssl_certificates
		 SET name=$1, host=$2, port=$3, server_name=$4, enabled=$5, updated_at=NOW()
		 WHERE id=$6`,
		c.Name, c.Host, c.Port, c.ServerName, c.Enabled, c.ID,
	)
	return err
}

func (db *DB) DeleteSSLCertificate(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM ssl_certificates WHERE id = $1`, id)
	return err
}

// ListEnabledSSLCertificates returns all enabled certificates for the worker to check.
func (db *DB) ListEnabledSSLCertificates(ctx context.Context) ([]models.SSLCertificate, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, host, port, server_name, enabled, last_checked_at,
		        valid_from, valid_to, issuer, subject, serial_number, dns_names,
		        last_error, created_at, updated_at
		 FROM ssl_certificates
		 WHERE enabled = TRUE`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.SSLCertificate
	for rows.Next() {
		var c models.SSLCertificate
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Host, &c.Port, &c.ServerName, &c.Enabled, &c.LastCheckedAt,
			&c.ValidFrom, &c.ValidTo, &c.Issuer, &c.Subject, &c.SerialNumber, pq.Array(&c.DNSNames),
			&c.LastError, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateSSLCertificateCheckResult records the result of a TLS handshake check.
func (db *DB) UpdateSSLCertificateCheckResult(ctx context.Context, c models.SSLCertificate) error {
	dns := c.DNSNames
	if dns == nil {
		dns = []string{}
	}
	_, err := db.conn.ExecContext(ctx,
		`UPDATE ssl_certificates
		 SET last_checked_at=$1, valid_from=$2, valid_to=$3, issuer=$4, subject=$5,
		     serial_number=$6, dns_names=$7, last_error=$8, updated_at=NOW()
		 WHERE id=$9`,
		c.LastCheckedAt, c.ValidFrom, c.ValidTo, c.Issuer, c.Subject,
		c.SerialNumber, pq.Array(dns), c.LastError, c.ID,
	)
	return err
}

// GetMinSSLDaysRemaining returns the smallest "days until expiration" across all
// enabled certificates with a known valid_to. Returns +Inf-equivalent (math.MaxInt32) when no certs.
// Used by the alert engine for the global "ssl_min_days_remaining" metric.
func (db *DB) GetMinSSLDaysRemaining(ctx context.Context) (int, bool, error) {
	var validTo sql.NullTime
	err := db.conn.QueryRowContext(ctx,
		`SELECT MIN(valid_to) FROM ssl_certificates
		 WHERE enabled = TRUE AND valid_to IS NOT NULL`,
	).Scan(&validTo)
	if err != nil {
		return 0, false, err
	}
	if !validTo.Valid {
		return 0, false, nil
	}
	days := int(time.Until(validTo.Time) / (24 * time.Hour))
	return days, true, nil
}

// InsertSSLCertificateEventIfNew records a new certificate version when the
// serial_number hasn't been seen before for this endpoint. Duplicate serials are
// silently ignored (ON CONFLICT DO NOTHING) so the caller can always call this
// after every successful check without extra checks.
func (db *DB) InsertSSLCertificateEventIfNew(ctx context.Context, ev models.SSLCertificateEvent) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO ssl_certificate_events
		     (certificate_id, serial_number, valid_from, valid_to, issuer, subject, detected_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NOW())
		 ON CONFLICT (certificate_id, serial_number) DO NOTHING`,
		ev.CertificateID, ev.SerialNumber, ev.ValidFrom, ev.ValidTo, ev.Issuer, ev.Subject,
	)
	return err
}

// GetSSLCertificateEvents returns the renewal history for a certificate, newest
// first. limit is capped at 100.
func (db *DB) GetSSLCertificateEvents(ctx context.Context, certID string, limit int) ([]models.SSLCertificateEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, certificate_id, serial_number, valid_from, valid_to, issuer, subject, detected_at
		 FROM ssl_certificate_events
		 WHERE certificate_id = $1
		 ORDER BY detected_at DESC
		 LIMIT $2`, certID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []models.SSLCertificateEvent
	for rows.Next() {
		var e models.SSLCertificateEvent
		if err := rows.Scan(&e.ID, &e.CertificateID, &e.SerialNumber, &e.ValidFrom, &e.ValidTo, &e.Issuer, &e.Subject, &e.DetectedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []models.SSLCertificateEvent{}
	}
	return out, rows.Err()
}

// computeDaysRemaining returns the number of whole days until validTo, or nil if unknown.
func computeDaysRemaining(validTo *time.Time) *int {
	if validTo == nil || validTo.IsZero() {
		return nil
	}
	days := int(time.Until(*validTo) / (24 * time.Hour))
	return &days
}
