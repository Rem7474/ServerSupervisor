package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/serversupervisor/server/internal/models"
)

// pqForeignKeyViolation is PostgreSQL's foreign_key_violation SQLSTATE.
const pqForeignKeyViolation = "23503"

func scanHostGroup(row interface {
	Scan(dest ...interface{}) error
}) (models.HostGroup, error) {
	var g models.HostGroup
	var tagsJSON string
	if err := row.Scan(&g.ID, &g.Name, &g.Description, &g.MatchMode, &tagsJSON, &g.OSContains, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return g, err
	}
	g.Tags = parseTags(tagsJSON)
	return g, nil
}

// hostGroupUUID parses a group id. Any UUID spelling is accepted (upper or
// lower case); anything else cannot name a group.
func hostGroupUUID(id string) (uuid.UUID, bool) {
	u, err := uuid.Parse(id)
	return u, err == nil
}

// ListHostGroups returns every host group ordered by name (never nil).
// MemberIDs is left empty: membership is computed by the caller from the
// current hosts.
func (db *DB) ListHostGroups(ctx context.Context) ([]models.HostGroup, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id, name, description, match_mode, tags::text, os_contains, created_at, updated_at FROM host_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.HostGroup, 0)
	for rows.Next() {
		g, err := scanHostGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// GetHostGroupByID returns one host group, or sql.ErrNoRows when absent or
// when id is not a UUID.
func (db *DB) GetHostGroupByID(ctx context.Context, id string) (*models.HostGroup, error) {
	u, ok := hostGroupUUID(id)
	if !ok {
		return nil, sql.ErrNoRows
	}
	g, err := scanHostGroup(db.conn.QueryRowContext(ctx, `SELECT id, name, description, match_mode, tags::text, os_contains, created_at, updated_at FROM host_groups WHERE id = $1::uuid`, u.String()))
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// CreateHostGroup inserts a group and fills its id and timestamps.
func (db *DB) CreateHostGroup(ctx context.Context, g *models.HostGroup) error {
	return db.conn.QueryRowContext(ctx,
		`INSERT INTO host_groups (name, description, match_mode, tags, os_contains)
		 VALUES ($1, $2, $3, CAST($4 AS JSONB), $5)
		 RETURNING id, created_at, updated_at`,
		g.Name, g.Description, g.MatchMode, marshalTags(g.Tags), g.OSContains,
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
}

// UpdateHostGroup replaces a group's definition. Returns sql.ErrNoRows when
// the group does not exist.
func (db *DB) UpdateHostGroup(ctx context.Context, g *models.HostGroup) error {
	u, ok := hostGroupUUID(g.ID)
	if !ok {
		return sql.ErrNoRows
	}
	return db.conn.QueryRowContext(ctx,
		`UPDATE host_groups
		 SET name = $1, description = $2, match_mode = $3, tags = CAST($4 AS JSONB), os_contains = $5, updated_at = NOW()
		 WHERE id = $6::uuid
		 RETURNING id, created_at, updated_at`,
		g.Name, g.Description, g.MatchMode, marshalTags(g.Tags), g.OSContains, u.String(),
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
}

// DeleteHostGroup removes a group. The alert_rules foreign key refuses the
// delete while a rule still targets it: that refusal is returned as
// models.ErrHostGroupInUse.
func (db *DB) DeleteHostGroup(ctx context.Context, id string) error {
	u, ok := hostGroupUUID(id)
	if !ok {
		return nil
	}
	_, err := db.conn.ExecContext(ctx, `DELETE FROM host_groups WHERE id = $1::uuid`, u.String())
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == pqForeignKeyViolation {
		return models.ErrHostGroupInUse
	}
	return err
}

// CountAlertRulesForHostGroup returns how many alert rules target the group.
func (db *DB) CountAlertRulesForHostGroup(ctx context.Context, id string) (int, error) {
	u, ok := hostGroupUUID(id)
	if !ok {
		return 0, nil
	}
	var n int
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM alert_rules WHERE host_group_id = $1::uuid`, u.String()).Scan(&n)
	return n, err
}
