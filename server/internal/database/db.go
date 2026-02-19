package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
)

type DB struct {
	conn *sql.DB
}

func New(cfg *config.Config) (*DB, error) {
	conn, err := sql.Open("postgres", cfg.DBDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role VARCHAR(50) NOT NULL DEFAULT 'viewer',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS hosts (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			hostname VARCHAR(255) NOT NULL DEFAULT '',
			ip_address VARCHAR(45) NOT NULL,
			os VARCHAR(255) NOT NULL DEFAULT '',
			api_key VARCHAR(255) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'offline',
			last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS system_metrics (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			cpu_usage_percent DOUBLE PRECISION,
			cpu_cores INTEGER,
			cpu_model VARCHAR(255),
			load_avg_1 DOUBLE PRECISION,
			load_avg_5 DOUBLE PRECISION,
			load_avg_15 DOUBLE PRECISION,
			memory_total BIGINT,
			memory_used BIGINT,
			memory_free BIGINT,
			memory_percent DOUBLE PRECISION,
			swap_total BIGINT,
			swap_used BIGINT,
			network_rx_bytes BIGINT,
			network_tx_bytes BIGINT,
			uptime BIGINT,
			hostname VARCHAR(255)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_system_metrics_host_time ON system_metrics(host_id, timestamp DESC)`,
		`CREATE TABLE IF NOT EXISTS disk_info (
			id BIGSERIAL PRIMARY KEY,
			metrics_id BIGINT REFERENCES system_metrics(id) ON DELETE CASCADE,
			mount_point VARCHAR(255),
			device VARCHAR(255),
			fs_type VARCHAR(50),
			total_bytes BIGINT,
			used_bytes BIGINT,
			free_bytes BIGINT,
			used_percent DOUBLE PRECISION
		)`,
		`CREATE TABLE IF NOT EXISTS docker_containers (
			id VARCHAR(64) PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			container_id VARCHAR(64),
			name VARCHAR(255),
			image VARCHAR(512),
			image_tag VARCHAR(255),
			image_id VARCHAR(255),
			state VARCHAR(50),
			status VARCHAR(255),
			created TIMESTAMP WITH TIME ZONE,
			ports TEXT,
			labels JSONB DEFAULT '{}',
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_containers_host ON docker_containers(host_id)`,
		`CREATE TABLE IF NOT EXISTS apt_status (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) UNIQUE REFERENCES hosts(id) ON DELETE CASCADE,
			last_update TIMESTAMP WITH TIME ZONE,
			last_upgrade TIMESTAMP WITH TIME ZONE,
			pending_packages INTEGER DEFAULT 0,
			package_list TEXT DEFAULT '[]',
			security_updates INTEGER DEFAULT 0,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS apt_commands (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			command VARCHAR(50) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			output TEXT DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			started_at TIMESTAMP WITH TIME ZONE,
			ended_at TIMESTAMP WITH TIME ZONE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_apt_commands_host_status ON apt_commands(host_id, status)`,
		`CREATE TABLE IF NOT EXISTS tracked_repos (
			id SERIAL PRIMARY KEY,
			owner VARCHAR(255) NOT NULL,
			repo VARCHAR(255) NOT NULL,
			display_name VARCHAR(255),
			latest_version VARCHAR(255) DEFAULT '',
			latest_date TIMESTAMP WITH TIME ZONE,
			release_url TEXT DEFAULT '',
			docker_image VARCHAR(512) DEFAULT '',
			checked_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE(owner, repo)
		)`,
		// Migration: Add name column to hosts if it doesn't exist
		`ALTER TABLE IF EXISTS hosts ADD COLUMN IF NOT EXISTS name VARCHAR(255) NOT NULL DEFAULT ''`,
	}

	for _, m := range migrations {
		if _, err := db.conn.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %s: %w", m[:60], err)
		}
	}
	log.Println("Database migrations completed")
	return nil
}

// ========== Users ==========

func (db *DB) CreateUser(username, passwordHash, role string) error {
	_, err := db.conn.Exec(
		`INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3)
		 ON CONFLICT (username) DO NOTHING`,
		username, passwordHash, role,
	)
	return err
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) UpdateUserPassword(username, passwordHash string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET password_hash = $1 WHERE username = $2`,
		passwordHash, username,
	)
	return err
}

// ========== Hosts ==========

func (db *DB) RegisterHost(host *models.Host) error {
	lastSeen := host.LastSeen
	if lastSeen.IsZero() {
		lastSeen = time.Now()
	}
	_, err := db.conn.Exec(
		`INSERT INTO hosts (id, name, hostname, ip_address, os, api_key, status, last_seen)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		host.ID, host.Name, host.Hostname, host.IPAddress, host.OS, host.APIKey, host.Status, lastSeen,
	)
	return err
}

func (db *DB) GetHost(id string) (*models.Host, error) {
	var h models.Host
	err := db.conn.QueryRow(
		`SELECT id, name, hostname, ip_address, os, api_key, status, last_seen, created_at, updated_at
		 FROM hosts WHERE id = $1`, id,
	).Scan(&h.ID, &h.Name, &h.Hostname, &h.IPAddress, &h.OS, &h.APIKey, &h.Status, &h.LastSeen, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (db *DB) GetHostByAPIKey(apiKey string) (*models.Host, error) {
	var h models.Host
	err := db.conn.QueryRow(
		`SELECT id, name, hostname, ip_address, os, api_key, status, last_seen, created_at, updated_at
		 FROM hosts WHERE api_key = $1`, apiKey,
	).Scan(&h.ID, &h.Name, &h.Hostname, &h.IPAddress, &h.OS, &h.APIKey, &h.Status, &h.LastSeen, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (db *DB) GetAllHosts() ([]models.Host, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, hostname, ip_address, os, status, last_seen, created_at, updated_at
		 FROM hosts ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.Host
	for rows.Next() {
		var h models.Host
		if err := rows.Scan(&h.ID, &h.Name, &h.Hostname, &h.IPAddress, &h.OS, &h.Status, &h.LastSeen, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
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
	_, err := db.conn.Exec(
		`UPDATE hosts SET
			name = COALESCE($1, name),
			hostname = COALESCE($2, hostname),
			ip_address = COALESCE($3, ip_address),
			os = COALESCE($4, os),
			updated_at = NOW()
		WHERE id = $5`,
		update.Name, update.Hostname, update.IPAddress, update.OS, id,
	)
	return err
}

func (db *DB) DeleteHost(id string) error {
	_, err := db.conn.Exec(`DELETE FROM hosts WHERE id = $1`, id)
	return err
}

// ========== System Metrics ==========

func (db *DB) InsertMetrics(m *models.SystemMetrics) (int64, error) {
	var id int64
	err := db.conn.QueryRow(
		`INSERT INTO system_metrics (host_id, timestamp, cpu_usage_percent, cpu_cores, cpu_model,
		 load_avg_1, load_avg_5, load_avg_15, memory_total, memory_used, memory_free, memory_percent,
		 swap_total, swap_used, network_rx_bytes, network_tx_bytes, uptime, hostname)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		 RETURNING id`,
		m.HostID, m.Timestamp, m.CPUUsagePercent, m.CPUCores, m.CPUModel,
		m.LoadAvg1, m.LoadAvg5, m.LoadAvg15, m.MemoryTotal, m.MemoryUsed, m.MemoryFree, m.MemoryPercent,
		m.SwapTotal, m.SwapUsed, m.NetworkRxBytes, m.NetworkTxBytes, m.Uptime, m.Hostname,
	).Scan(&id)
	if err != nil {
		return 0, err
	}

	for _, d := range m.Disks {
		_, err := db.conn.Exec(
			`INSERT INTO disk_info (metrics_id, mount_point, device, fs_type, total_bytes, used_bytes, free_bytes, used_percent)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			id, d.MountPoint, d.Device, d.FSType, d.TotalBytes, d.UsedBytes, d.FreeBytes, d.UsedPercent,
		)
		if err != nil {
			return id, err
		}
	}
	return id, nil
}

func (db *DB) GetLatestMetrics(hostID string) (*models.SystemMetrics, error) {
	var m models.SystemMetrics
	err := db.conn.QueryRow(
		`SELECT id, host_id, timestamp, cpu_usage_percent, cpu_cores, cpu_model,
		 load_avg_1, load_avg_5, load_avg_15, memory_total, memory_used, memory_free, memory_percent,
		 swap_total, swap_used, network_rx_bytes, network_tx_bytes, uptime, hostname
		 FROM system_metrics WHERE host_id = $1 ORDER BY timestamp DESC LIMIT 1`, hostID,
	).Scan(&m.ID, &m.HostID, &m.Timestamp, &m.CPUUsagePercent, &m.CPUCores, &m.CPUModel,
		&m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15, &m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
		&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes, &m.Uptime, &m.Hostname)
	if err != nil {
		return nil, err
	}

	// Load disk info
	rows, err := db.conn.Query(
		`SELECT id, mount_point, device, fs_type, total_bytes, used_bytes, free_bytes, used_percent
		 FROM disk_info WHERE metrics_id = $1`, m.ID,
	)
	if err != nil {
		return &m, nil // Return metrics without disks
	}
	defer rows.Close()

	for rows.Next() {
		var d models.DiskInfo
		if err := rows.Scan(&d.ID, &d.MountPoint, &d.Device, &d.FSType, &d.TotalBytes, &d.UsedBytes, &d.FreeBytes, &d.UsedPercent); err != nil {
			continue
		}
		m.Disks = append(m.Disks, d)
	}
	return &m, nil
}

func (db *DB) GetMetricsHistory(hostID string, hours int) ([]models.SystemMetrics, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, timestamp, cpu_usage_percent, cpu_cores, load_avg_1, load_avg_5, load_avg_15,
		 memory_total, memory_used, memory_free, memory_percent, swap_total, swap_used,
		 network_rx_bytes, network_tx_bytes, uptime
		 FROM system_metrics WHERE host_id = $1 AND timestamp > NOW() - INTERVAL '1 hour' * $2
		 ORDER BY timestamp ASC`, hostID, hours,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.SystemMetrics
	for rows.Next() {
		var m models.SystemMetrics
		if err := rows.Scan(&m.ID, &m.HostID, &m.Timestamp, &m.CPUUsagePercent, &m.CPUCores,
			&m.LoadAvg1, &m.LoadAvg5, &m.LoadAvg15, &m.MemoryTotal, &m.MemoryUsed, &m.MemoryFree, &m.MemoryPercent,
			&m.SwapTotal, &m.SwapUsed, &m.NetworkRxBytes, &m.NetworkTxBytes, &m.Uptime); err != nil {
			continue
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

func (db *DB) CleanOldMetrics(retentionDays int) (int64, error) {
	result, err := db.conn.Exec(
		`DELETE FROM system_metrics WHERE timestamp < NOW() - INTERVAL '1 day' * $1`, retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ========== Docker ==========

func (db *DB) UpsertDockerContainers(hostID string, containers []models.DockerContainer) error {
	// Delete removed containers
	_, err := db.conn.Exec(`DELETE FROM docker_containers WHERE host_id = $1`, hostID)
	if err != nil {
		return err
	}

	for _, c := range containers {
		labelsJSON, _ := json.Marshal(c.Labels)
		_, err := db.conn.Exec(
			`INSERT INTO docker_containers (id, host_id, container_id, name, image, image_tag, image_id, state, status, created, ports, labels, updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW())`,
			c.ID, hostID, c.ContainerID, c.Name, c.Image, c.ImageTag, c.ImageID, c.State, c.Status, c.Created, c.Ports, string(labelsJSON),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetDockerContainers(hostID string) ([]models.DockerContainer, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, container_id, name, image, image_tag, image_id, state, status, created, ports, labels, updated_at
		 FROM docker_containers WHERE host_id = $1 ORDER BY name`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []models.DockerContainer
	for rows.Next() {
		var c models.DockerContainer
		var labelsJSON string
		if err := rows.Scan(&c.ID, &c.HostID, &c.ContainerID, &c.Name, &c.Image, &c.ImageTag, &c.ImageID,
			&c.State, &c.Status, &c.Created, &c.Ports, &labelsJSON, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(labelsJSON), &c.Labels)
		containers = append(containers, c)
	}
	return containers, nil
}

func (db *DB) GetAllDockerContainers() ([]models.DockerContainer, error) {
	rows, err := db.conn.Query(
		`SELECT dc.id, dc.host_id, h.hostname, dc.container_id, dc.name, dc.image, dc.image_tag, dc.image_id,
		 dc.state, dc.status, dc.created, dc.ports, dc.labels, dc.updated_at
		 FROM docker_containers dc
		 JOIN hosts h ON dc.host_id = h.id
		 ORDER BY h.hostname, dc.name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []models.DockerContainer
	for rows.Next() {
		var c models.DockerContainer
		var labelsJSON string
		if err := rows.Scan(&c.ID, &c.HostID, &c.Hostname, &c.ContainerID, &c.Name, &c.Image, &c.ImageTag, &c.ImageID,
			&c.State, &c.Status, &c.Created, &c.Ports, &labelsJSON, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(labelsJSON), &c.Labels)
		containers = append(containers, c)
	}
	return containers, nil
}

// ========== APT ==========

func (db *DB) UpsertAptStatus(status *models.AptStatus) error {
	_, err := db.conn.Exec(
		`INSERT INTO apt_status (host_id, last_update, last_upgrade, pending_packages, package_list, security_updates, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,NOW())
		 ON CONFLICT (host_id) DO UPDATE SET
			last_update = EXCLUDED.last_update,
			last_upgrade = EXCLUDED.last_upgrade,
			pending_packages = EXCLUDED.pending_packages,
			package_list = EXCLUDED.package_list,
			security_updates = EXCLUDED.security_updates,
			updated_at = NOW()`,
		status.HostID, status.LastUpdate, status.LastUpgrade, status.PendingPackages, status.PackageList, status.SecurityUpdates,
	)
	return err
}

func (db *DB) GetAptStatus(hostID string) (*models.AptStatus, error) {
	var s models.AptStatus
	err := db.conn.QueryRow(
		`SELECT id, host_id, last_update, last_upgrade, pending_packages, package_list, security_updates, updated_at
		 FROM apt_status WHERE host_id = $1`, hostID,
	).Scan(&s.ID, &s.HostID, &s.LastUpdate, &s.LastUpgrade, &s.PendingPackages, &s.PackageList, &s.SecurityUpdates, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) CreateAptCommand(hostID, command string) (*models.AptCommand, error) {
	var cmd models.AptCommand
	err := db.conn.QueryRow(
		`INSERT INTO apt_commands (host_id, command, status) VALUES ($1, $2, 'pending') RETURNING id, host_id, command, status, created_at`,
		hostID, command,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Command, &cmd.Status, &cmd.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (db *DB) GetPendingCommands(hostID string) ([]models.PendingCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, command FROM apt_commands WHERE host_id = $1 AND status = 'pending' ORDER BY created_at ASC`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.PendingCommand
	for rows.Next() {
		var c models.PendingCommand
		if err := rows.Scan(&c.ID, &c.Type); err != nil {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

func (db *DB) UpdateCommandStatus(id int64, status, output string) error {
	var query string
	switch status {
	case "running":
		query = `UPDATE apt_commands SET status = $1, started_at = NOW() WHERE id = $2`
		_, err := db.conn.Exec(query, status, id)
		return err
	default:
		query = `UPDATE apt_commands SET status = $1, output = $2, ended_at = NOW() WHERE id = $3`
		_, err := db.conn.Exec(query, status, output, id)
		return err
	}
}

func (db *DB) GetAptCommandHistory(hostID string, limit int) ([]models.AptCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, command, status, output, created_at, started_at, ended_at
		 FROM apt_commands WHERE host_id = $1 ORDER BY created_at DESC LIMIT $2`, hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.AptCommand
	for rows.Next() {
		var c models.AptCommand
		if err := rows.Scan(&c.ID, &c.HostID, &c.Command, &c.Status, &c.Output, &c.CreatedAt, &c.StartedAt, &c.EndedAt); err != nil {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

// ========== Tracked Repos ==========

func (db *DB) CreateTrackedRepo(repo *models.TrackedRepo) error {
	return db.conn.QueryRow(
		`INSERT INTO tracked_repos (owner, repo, display_name, docker_image)
		 VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
		repo.Owner, repo.Repo, repo.DisplayName, repo.DockerImage,
	).Scan(&repo.ID, &repo.CreatedAt)
}

func (db *DB) GetTrackedRepos() ([]models.TrackedRepo, error) {
	rows, err := db.conn.Query(
		`SELECT id, owner, repo, display_name, latest_version, COALESCE(latest_date, NOW()),
		 release_url, docker_image, checked_at, created_at
		 FROM tracked_repos ORDER BY display_name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var repos []models.TrackedRepo
	for rows.Next() {
		var r models.TrackedRepo
		if err := rows.Scan(&r.ID, &r.Owner, &r.Repo, &r.DisplayName, &r.LatestVersion,
			&r.LatestDate, &r.ReleaseURL, &r.DockerImage, &r.CheckedAt, &r.CreatedAt); err != nil {
			continue
		}
		repos = append(repos, r)
	}
	return repos, nil
}

func (db *DB) UpdateTrackedRepo(id int64, version, url string, date time.Time) error {
	_, err := db.conn.Exec(
		`UPDATE tracked_repos SET latest_version = $1, release_url = $2, latest_date = $3, checked_at = NOW() WHERE id = $4`,
		version, url, date, id,
	)
	return err
}

func (db *DB) DeleteTrackedRepo(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM tracked_repos WHERE id = $1`, id)
	return err
}
