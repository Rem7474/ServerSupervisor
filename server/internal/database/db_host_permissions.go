package database

import "github.com/serversupervisor/server/internal/models"

// HostPermission stores a per-host access entry.
type HostPermission struct {
	Username  string `json:"username"`
	HostID    string `json:"host_id"`
	Level     string `json:"level"`
	CreatedAt string `json:"created_at"`
}

// GetHostAccess returns whether the user has explicit host restrictions and,
// if so, the level granted for hostID ("" = host not in their allow-list).
// Returns (restricted=false, ...) when the user has no host_permissions rows at all.
func (db *DB) GetHostAccess(username, hostID string) (restricted bool, level string, err error) {
	// Single query: check total count + level for this specific host.
	var totalCount int
	var hostLevel *string
	err = db.conn.QueryRow(`
		SELECT
			COUNT(*),
			MAX(CASE WHEN host_id = $2 THEN level END)
		FROM host_permissions
		WHERE username = $1
	`, username, hostID).Scan(&totalCount, &hostLevel)
	if err != nil {
		return false, "", err
	}
	if totalCount == 0 {
		return false, "", nil // no restrictions at all
	}
	if hostLevel == nil {
		return true, "", nil // restricted but this host not in their list
	}
	return true, *hostLevel, nil
}

// ListHostPermissions returns all entries for a given host.
func (db *DB) ListHostPermissions(hostID string) ([]models.HostPermission, error) {
	rows, err := db.conn.Query(`
		SELECT username, host_id, level, created_at
		FROM host_permissions
		WHERE host_id = $1
		ORDER BY username
	`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.HostPermission
	for rows.Next() {
		var p models.HostPermission
		if err := rows.Scan(&p.Username, &p.HostID, &p.Level, &p.CreatedAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// ListUserHostPermissions returns all host permission entries for a given username.
func (db *DB) ListUserHostPermissions(username string) ([]models.HostPermission, error) {
	rows, err := db.conn.Query(`
		SELECT username, host_id, level, created_at
		FROM host_permissions
		WHERE username = $1
		ORDER BY host_id
	`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.HostPermission
	for rows.Next() {
		var p models.HostPermission
		if err := rows.Scan(&p.Username, &p.HostID, &p.Level, &p.CreatedAt); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// SetHostPermission upserts an access entry for username+hostID.
func (db *DB) SetHostPermission(username, hostID, level string) error {
	_, err := db.conn.Exec(`
		INSERT INTO host_permissions (username, host_id, level)
		VALUES ($1, $2, $3)
		ON CONFLICT (username, host_id) DO UPDATE SET level = EXCLUDED.level
	`, username, hostID, level)
	return err
}

// DeleteHostPermission removes an access entry for username+hostID.
func (db *DB) DeleteHostPermission(username, hostID string) error {
	_, err := db.conn.Exec(`
		DELETE FROM host_permissions WHERE username = $1 AND host_id = $2
	`, username, hostID)
	return err
}
