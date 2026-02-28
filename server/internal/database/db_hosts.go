package database

import (
	"encoding/json"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// ========== Hosts ==========

func marshalTags(tags []string) string {
	if tags == nil {
		return "[]"
	}
	data, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func parseTags(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return []string{}
	}
	return tags
}

func (db *DB) RegisterHost(host *models.Host) error {
	lastSeen := host.LastSeen
	if lastSeen.IsZero() {
		lastSeen = time.Now()
	}
	tagsJSON := marshalTags(host.Tags)
	_, err := db.conn.Exec(
		`INSERT INTO hosts (id, name, hostname, ip_address, os, api_key, tags, status, last_seen)
		 VALUES ($1, $2, $3, $4, $5, $6, CAST($7 AS JSONB), $8, $9)`,
		host.ID, host.Name, host.Hostname, host.IPAddress, host.OS, host.APIKey, tagsJSON, host.Status, lastSeen,
	)
	return err
}

func (db *DB) GetHost(id string) (*models.Host, error) {
	var h models.Host
	var tagsJSON string
	err := db.conn.QueryRow(
		`SELECT id, name, hostname, ip_address, os, agent_version, api_key, tags, status, last_seen, created_at, updated_at
		 FROM hosts WHERE id = $1`, id,
	).Scan(&h.ID, &h.Name, &h.Hostname, &h.IPAddress, &h.OS, &h.AgentVersion, &h.APIKey, &tagsJSON, &h.Status, &h.LastSeen, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	h.Tags = parseTags(tagsJSON)
	return &h, nil
}

func (db *DB) GetHostByAPIKey(apiKey string) (*models.Host, error) {
	var h models.Host
	var tagsJSON string
	apiKeyHash := HashAPIKey(apiKey)
	err := db.conn.QueryRow(
		`SELECT id, name, hostname, ip_address, os, agent_version, api_key, tags, status, last_seen, created_at, updated_at
		 FROM hosts WHERE api_key = $1`, apiKeyHash,
	).Scan(&h.ID, &h.Name, &h.Hostname, &h.IPAddress, &h.OS, &h.AgentVersion, &h.APIKey, &tagsJSON, &h.Status, &h.LastSeen, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	h.Tags = parseTags(tagsJSON)
	return &h, nil
}

func (db *DB) GetAllHosts() ([]models.Host, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, hostname, ip_address, os, agent_version, tags, status, last_seen, created_at, updated_at
		 FROM hosts ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.Host
	for rows.Next() {
		var h models.Host
		var tagsJSON string
		if err := rows.Scan(&h.ID, &h.Name, &h.Hostname, &h.IPAddress, &h.OS, &h.AgentVersion, &tagsJSON, &h.Status, &h.LastSeen, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		h.Tags = parseTags(tagsJSON)
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func (db *DB) UpdateHostStatus(id, status string) error {
	_, err := db.conn.Exec(
		`UPDATE hosts SET status = $1, last_seen = NOW(), updated_at = NOW() WHERE id = $2`,
		status, id,
	)
	return err
}

func (db *DB) UpdateHost(id string, update *models.HostUpdate) error {
	var tagsJSON *string
	if update.Tags != nil {
		value := marshalTags(*update.Tags)
		tagsJSON = &value
	}
	_, err := db.conn.Exec(
		`UPDATE hosts SET
			name = COALESCE($1, name),
			hostname = COALESCE($2, hostname),
			ip_address = COALESCE($3, ip_address),
			os = COALESCE($4, os),
			agent_version = COALESCE($5, agent_version),
			tags = COALESCE($6::jsonb, tags),
			updated_at = NOW()
		WHERE id = $7`,
		update.Name, update.Hostname, update.IPAddress, update.OS, update.AgentVersion, tagsJSON, id,
	)
	return err
}

func (db *DB) DeleteHost(id string) error {
	_, err := db.conn.Exec(`DELETE FROM hosts WHERE id = $1`, id)
	return err
}

func (db *DB) UpdateHostAPIKey(id string, apiKeyHash string) error {
	_, err := db.conn.Exec(
		`UPDATE hosts SET api_key = $1, updated_at = NOW() WHERE id = $2`,
		apiKeyHash, id,
	)
	return err
}

// UpdateHostStatusBasedOnLastSeen sets status to 'offline' for hosts not seen recently.
func (db *DB) UpdateHostStatusBasedOnLastSeen(maxOfflineMinutes int) error {
	cutoffTime := time.Now().Add(time.Duration(-maxOfflineMinutes) * time.Minute)
	_, err := db.conn.Exec(
		`UPDATE hosts SET status = 'offline' WHERE status != 'offline' AND last_seen < $1`,
		cutoffTime,
	)
	return err
}

// GetHostHealthStatus returns status and last_seen for a single host.
func (db *DB) GetHostHealthStatus(hostID string) (status string, lastSeen time.Time, err error) {
	err = db.conn.QueryRow(
		`SELECT status, last_seen FROM hosts WHERE id = $1`,
		hostID,
	).Scan(&status, &lastSeen)
	return
}

// CountHosts returns the total number of registered hosts.
func (db *DB) CountHosts() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&count)
	return count, err
}
