package database

import (
	"context"
	"database/sql"

	"github.com/serversupervisor/server/internal/models"
)

// ListRegistryCredentials returns all credentials without their passwords.
func (db *DB) ListRegistryCredentials(ctx context.Context) ([]models.RegistryCredential, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT id, name, registry_host, username, created_at, updated_at
		 FROM registry_credentials ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.RegistryCredential, 0)
	for rows.Next() {
		var rc models.RegistryCredential
		if err := rows.Scan(&rc.ID, &rc.Name, &rc.RegistryHost, &rc.Username, &rc.CreatedAt, &rc.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

// GetRegistryCredentialByID returns one credential without its password.
func (db *DB) GetRegistryCredentialByID(ctx context.Context, id string) (*models.RegistryCredential, error) {
	var rc models.RegistryCredential
	err := db.conn.QueryRowContext(ctx,
		`SELECT id, name, registry_host, username, created_at, updated_at
		 FROM registry_credentials WHERE id=$1`, id).
		Scan(&rc.ID, &rc.Name, &rc.RegistryHost, &rc.Username, &rc.CreatedAt, &rc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rc, nil
}

// GetRegistryCredentialAuth returns the username/password for a credential.
// Reserved for the release-tracker poller — never exposed to API clients.
func (db *DB) GetRegistryCredentialAuth(ctx context.Context, id string) (username, password string, err error) {
	err = db.conn.QueryRowContext(ctx,
		`SELECT username, password FROM registry_credentials WHERE id=$1`, id).
		Scan(&username, &password)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return username, password, err
}

func (db *DB) CreateRegistryCredential(ctx context.Context, rc models.RegistryCredential) (*models.RegistryCredential, error) {
	var result models.RegistryCredential
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO registry_credentials (name, registry_host, username, password)
		 VALUES ($1,$2,$3,$4)
		 RETURNING id, name, registry_host, username, created_at, updated_at`,
		rc.Name, rc.RegistryHost, rc.Username, rc.Password).
		Scan(&result.ID, &result.Name, &result.RegistryHost, &result.Username, &result.CreatedAt, &result.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateRegistryCredential updates a credential. The password is only changed
// when a non-empty value is supplied, so the UI can omit it to keep the existing one.
func (db *DB) UpdateRegistryCredential(ctx context.Context, id string, rc models.RegistryCredential) error {
	if rc.Password != "" {
		_, err := db.conn.ExecContext(ctx,
			`UPDATE registry_credentials
			 SET name=$2, registry_host=$3, username=$4, password=$5, updated_at=NOW()
			 WHERE id=$1`,
			id, rc.Name, rc.RegistryHost, rc.Username, rc.Password)
		return err
	}
	_, err := db.conn.ExecContext(ctx,
		`UPDATE registry_credentials
		 SET name=$2, registry_host=$3, username=$4, updated_at=NOW()
		 WHERE id=$1`,
		id, rc.Name, rc.RegistryHost, rc.Username)
	return err
}

func (db *DB) DeleteRegistryCredential(ctx context.Context, id string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM registry_credentials WHERE id=$1`, id)
	return err
}
