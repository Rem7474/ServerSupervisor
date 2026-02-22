package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/serversupervisor/server/internal/models"
)

// UpsertComposeProjects replaces all compose projects for a host
func (db *DB) UpsertComposeProjects(hostID string, projects []models.ComposeProject) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing projects for this host
	if _, err := tx.Exec("DELETE FROM compose_projects WHERE host_id = $1", hostID); err != nil {
		return fmt.Errorf("failed to delete old compose projects: %w", err)
	}

	for _, p := range projects {
		id := fmt.Sprintf("%s-%s", hostID, p.Name)
		servicesJSON, _ := json.Marshal(p.Services)

		_, err := tx.Exec(`
			INSERT INTO compose_projects (id, host_id, name, working_dir, config_file, services, raw_config, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, id, hostID, p.Name, p.WorkingDir, p.ConfigFile, string(servicesJSON), p.RawConfig, time.Now())
		if err != nil {
			return fmt.Errorf("failed to insert compose project %s: %w", p.Name, err)
		}
	}

	return tx.Commit()
}

// GetComposeProjectsByHost returns compose projects for a specific host
func (db *DB) GetComposeProjectsByHost(hostID string) ([]models.ComposeProject, error) {
	rows, err := db.conn.Query(`
		SELECT cp.id, cp.host_id, COALESCE(h.hostname, ''), cp.name,
		       cp.working_dir, cp.config_file, cp.services, cp.raw_config, cp.updated_at
		FROM compose_projects cp
		LEFT JOIN hosts h ON h.id = cp.host_id
		WHERE cp.host_id = $1
		ORDER BY cp.name
	`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComposeProjects(rows)
}

// GetAllComposeProjects returns all compose projects across all hosts
func (db *DB) GetAllComposeProjects() ([]models.ComposeProject, error) {
	rows, err := db.conn.Query(`
		SELECT cp.id, cp.host_id, COALESCE(h.hostname, ''), cp.name,
		       cp.working_dir, cp.config_file, cp.services, cp.raw_config, cp.updated_at
		FROM compose_projects cp
		LEFT JOIN hosts h ON h.id = cp.host_id
		ORDER BY cp.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanComposeProjects(rows)
}

func scanComposeProjects(rows *sql.Rows) ([]models.ComposeProject, error) {
	var projects []models.ComposeProject
	for rows.Next() {
		var p models.ComposeProject
		var servicesJSON string
		err := rows.Scan(
			&p.ID, &p.HostID, &p.Hostname, &p.Name,
			&p.WorkingDir, &p.ConfigFile, &servicesJSON, &p.RawConfig, &p.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if servicesJSON != "" {
			_ = json.Unmarshal([]byte(servicesJSON), &p.Services)
		}
		if p.Services == nil {
			p.Services = []string{}
		}
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []models.ComposeProject{}
	}
	return projects, rows.Err()
}
