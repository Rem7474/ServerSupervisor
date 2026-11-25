package database

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// APIKeyAuthRecord is what the API-key authentication path needs about a key:
// its stored hash, scope and validity, plus its owner's current username and
// role (read at every request, so a key follows a role change immediately).
type APIKeyAuthRecord struct {
	ID        int64
	KeyHash   string
	Scope     string
	ExpiresAt *time.Time
	RevokedAt *time.Time
	Username  string
	Role      string
}

const apiKeySelect = `SELECT k.id, k.name, k.prefix, k.scope, u.username, k.expires_at, k.last_used_at, k.revoked_at, k.created_at
	FROM api_keys k JOIN users u ON u.id = k.user_id`

func scanAPIKey(row interface{ Scan(...any) error }) (models.APIKey, error) {
	var k models.APIKey
	err := row.Scan(&k.ID, &k.Name, &k.Prefix, &k.Scope, &k.Username, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	return k, err
}

func (db *DB) queryAPIKeys(ctx context.Context, query string, args ...any) ([]models.APIKey, error) {
	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []models.APIKey{}
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// CreateAPIKey stores a new key (hash only) and returns it as listed.
func (db *DB) CreateAPIKey(ctx context.Context, userID int64, name, prefix, keyHash, scope string, expiresAt *time.Time) (*models.APIKey, error) {
	var id int64
	err := db.conn.QueryRowContext(ctx,
		`INSERT INTO api_keys (user_id, name, prefix, key_hash, scope, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		userID, name, prefix, keyHash, scope, expiresAt,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return db.GetAPIKeyByID(ctx, id)
}

// GetAPIKeyByID returns one key, sql.ErrNoRows when it does not exist.
func (db *DB) GetAPIKeyByID(ctx context.Context, id int64) (*models.APIKey, error) {
	k, err := scanAPIKey(db.conn.QueryRowContext(ctx, apiKeySelect+` WHERE k.id = $1`, id))
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListAPIKeysByUser returns a user's keys, revoked ones included, newest first.
func (db *DB) ListAPIKeysByUser(ctx context.Context, userID int64) ([]models.APIKey, error) {
	return db.queryAPIKeys(ctx, apiKeySelect+` WHERE k.user_id = $1 ORDER BY k.created_at DESC, k.id DESC`, userID)
}

// ListAllAPIKeys returns every user's keys, newest first (admin view).
func (db *DB) ListAllAPIKeys(ctx context.Context) ([]models.APIKey, error) {
	return db.queryAPIKeys(ctx, apiKeySelect+` ORDER BY k.created_at DESC, k.id DESC`)
}

// CountActiveAPIKeys counts a user's keys that are neither revoked nor expired.
func (db *DB) CountActiveAPIKeys(ctx context.Context, userID int64) (int, error) {
	var n int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM api_keys
		 WHERE user_id = $1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now())`,
		userID,
	).Scan(&n)
	return n, err
}

// RevokeAPIKey marks a key revoked. It reports false when the key was already
// revoked (the first revocation time is kept).
func (db *DB) RevokeAPIKey(ctx context.Context, id int64) (bool, error) {
	res, err := db.conn.ExecContext(ctx,
		`UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// GetAPIKeyForAuth looks a key up by its public prefix for authentication.
// It returns sql.ErrNoRows when no key has that prefix.
func (db *DB) GetAPIKeyForAuth(ctx context.Context, prefix string) (*APIKeyAuthRecord, error) {
	var r APIKeyAuthRecord
	err := db.conn.QueryRowContext(ctx,
		`SELECT k.id, k.key_hash, k.scope, k.expires_at, k.revoked_at, u.username, u.role
		 FROM api_keys k JOIN users u ON u.id = k.user_id
		 WHERE k.prefix = $1`,
		prefix,
	).Scan(&r.ID, &r.KeyHash, &r.Scope, &r.ExpiresAt, &r.RevokedAt, &r.Username, &r.Role)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// TouchAPIKeyLastUsed records a use of the key. The write is skipped when the
// stored value is less than a minute old, so a busy client costs at most one
// UPDATE per minute.
func (db *DB) TouchAPIKeyLastUsed(ctx context.Context, id int64) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = now()
		 WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`, id)
	return err
}
