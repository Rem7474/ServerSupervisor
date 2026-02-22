package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// HashAPIKey returns the SHA-256 hash of an API key
func HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// EnsureDatabaseExists creates the database if it doesn't exist
func EnsureDatabaseExists(cfg *config.Config) error {
	// Connect to the default "postgres" database to create our database
	tempDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBSSLMode)

	tempConn, err := sql.Open("postgres", tempDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer tempConn.Close()

	if err := tempConn.Ping(); err != nil {
		return fmt.Errorf("failed to ping postgres database: %w", err)
	}

	// Create database if it doesn't exist
	var exists int
	row := tempConn.QueryRow("SELECT 1 FROM pg_database WHERE datname = $1", cfg.DBName)
	if err := row.Scan(&exists); err != nil {
		if err != sql.ErrNoRows {
			return fmt.Errorf("failed to check database existence: %w", err)
		}

		createDBSQL := fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(cfg.DBName))
		if _, err := tempConn.Exec(createDBSQL); err != nil {
			return fmt.Errorf("failed to create database: %w", err)
		}
	}

	log.Printf("Database %s is ready", cfg.DBName)
	return nil
}

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
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id BIGSERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
			token_hash VARCHAR(64) UNIQUE NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			revoked_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id)`,
		`CREATE TABLE IF NOT EXISTS hosts (
			id VARCHAR(64) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			hostname VARCHAR(255) NOT NULL DEFAULT '',
			ip_address VARCHAR(45) NOT NULL,
			os VARCHAR(255) NOT NULL DEFAULT '',
			api_key VARCHAR(255) NOT NULL,
			tags JSONB DEFAULT '[]',
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
			package_list JSONB DEFAULT '[]',
			security_updates INTEGER DEFAULT 0,
			cve_list JSONB DEFAULT '[]',
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS apt_commands (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			command VARCHAR(50) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			output TEXT DEFAULT '',
			audit_log_id BIGINT,
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
		// Migration: Add tags column to hosts
		`ALTER TABLE IF EXISTS hosts ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]'::jsonb`,
		// Migration: Convert package_list from TEXT to JSONB for existing databases
		`ALTER TABLE IF EXISTS apt_status ALTER COLUMN package_list DROP DEFAULT`,
		`ALTER TABLE IF EXISTS apt_status ALTER COLUMN package_list TYPE JSONB USING COALESCE(package_list::jsonb, '[]'::jsonb)`,
		`ALTER TABLE IF EXISTS apt_status ALTER COLUMN package_list SET DEFAULT '[]'::jsonb`,
		// Migration: Add TOTP & RBAC fields to users
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS totp_secret TEXT DEFAULT ''`,
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS backup_codes TEXT DEFAULT '[]'`,
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN DEFAULT FALSE`,
		// Migration: Add triggered_by to apt_commands (who launched it)
		`ALTER TABLE IF EXISTS apt_commands ADD COLUMN IF NOT EXISTS triggered_by VARCHAR(255) DEFAULT 'system'`,
		// Migration: Link apt_commands to audit_logs
		`ALTER TABLE IF EXISTS apt_commands ADD COLUMN IF NOT EXISTS audit_log_id BIGINT`,
		// Migration: Create audit_logs table for APT & admin action history
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			action VARCHAR(100) NOT NULL,
			host_id VARCHAR(64),
			ip_address VARCHAR(45),
			details TEXT,
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_action ON audit_logs(username, action, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_host ON audit_logs(host_id, created_at DESC)`,
		// Migration: Create alert rules and incidents
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id SERIAL PRIMARY KEY,
			host_id VARCHAR(64),
			metric VARCHAR(50) NOT NULL,
			operator VARCHAR(5) NOT NULL,
			threshold DOUBLE PRECISION,
			duration_seconds INTEGER DEFAULT 60,
			channel VARCHAR(50) NOT NULL,
			channel_config JSONB NOT NULL DEFAULT '{}'::jsonb,
			enabled BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS alert_incidents (
			id BIGSERIAL PRIMARY KEY,
			rule_id INTEGER REFERENCES alert_rules(id) ON DELETE CASCADE,
			host_id VARCHAR(64),
			triggered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			resolved_at TIMESTAMP WITH TIME ZONE,
			value DOUBLE PRECISION
		)`,
		`CREATE INDEX IF NOT EXISTS idx_alert_incidents_rule ON alert_incidents(rule_id, triggered_at DESC)`,
		// Migration: Create metrics_aggregates table for downsampling (5min, hourly, daily)
		`CREATE TABLE IF NOT EXISTS metrics_aggregates (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			aggregation_type VARCHAR(20) NOT NULL, -- '5min', 'hour', 'day'
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL, -- Start of the interval
			cpu_usage_avg DOUBLE PRECISION,
			cpu_usage_max DOUBLE PRECISION,
			memory_usage_avg BIGINT,
			memory_usage_max BIGINT,
			disk_usage_avg DOUBLE PRECISION,
			network_rx_bytes BIGINT,
			network_tx_bytes BIGINT,
			sample_count INTEGER DEFAULT 1,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_aggregates_host_time ON metrics_aggregates(host_id, aggregation_type, timestamp DESC)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_aggregates_unique ON metrics_aggregates(host_id, aggregation_type, timestamp)`,
		// Migration: Add agent_version to hosts
		`ALTER TABLE IF EXISTS hosts ADD COLUMN IF NOT EXISTS agent_version VARCHAR(20) DEFAULT ''`,
		// Migration: Add cve_list to apt_status for CVE tracking
		`ALTER TABLE IF EXISTS apt_status ADD COLUMN IF NOT EXISTS cve_list JSONB DEFAULT '[]'::jsonb`,
		`ALTER TABLE IF EXISTS apt_status ALTER COLUMN cve_list DROP DEFAULT`,
		`ALTER TABLE IF EXISTS apt_status ALTER COLUMN cve_list TYPE JSONB USING COALESCE(cve_list::jsonb, '[]'::jsonb)`,
		`ALTER TABLE IF EXISTS apt_status ALTER COLUMN cve_list SET DEFAULT '[]'::jsonb`,
		// Migration: Convert backup_codes from TEXT to JSONB for better validation
		`ALTER TABLE IF EXISTS users ALTER COLUMN backup_codes DROP DEFAULT`,
		`ALTER TABLE IF EXISTS users ALTER COLUMN backup_codes TYPE JSONB USING COALESCE(backup_codes::jsonb, '[]'::jsonb)`,
		`ALTER TABLE IF EXISTS users ALTER COLUMN backup_codes SET DEFAULT '[]'::jsonb`,
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
		`SELECT id, username, password_hash, role, totp_secret, backup_codes, mfa_enabled, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, role, totp_secret, backup_codes, mfa_enabled, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUsers() ([]models.User, error) {
	rows, err := db.conn.Query(
		`SELECT id, username, role, created_at FROM users ORDER BY username`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	return users, nil
}

func (db *DB) UpdateUserRole(id int64, role string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET role = $1 WHERE id = $2`,
		role, id,
	)
	return err
}

func (db *DB) UpdateUserPassword(username, passwordHash string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET password_hash = $1 WHERE username = $2`,
		passwordHash, username,
	)
	return err
}

type RefreshTokenRecord struct {
	UserID    int64
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (db *DB) CreateRefreshToken(userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := db.conn.Exec(
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	return err
}

func (db *DB) GetRefreshToken(tokenHash string) (*RefreshTokenRecord, error) {
	var rec RefreshTokenRecord
	var revoked sql.NullTime
	err := db.conn.QueryRow(
		`SELECT user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&rec.UserID, &rec.ExpiresAt, &revoked)
	if err != nil {
		return nil, err
	}
	if revoked.Valid {
		rec.RevokedAt = &revoked.Time
	}
	return &rec, nil
}

func (db *DB) RevokeRefreshToken(tokenHash string) error {
	_, err := db.conn.Exec(
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
	)
	return err
}

func (db *DB) DeleteUser(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM users WHERE id = $1`, id)
	return err
}

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

func (db *DB) GetMetricsSummary(hours int, bucketMinutes int) ([]models.SystemMetricsSummary, error) {
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}
	rows, err := db.conn.Query(
		`SELECT
			to_timestamp(floor(extract(epoch from timestamp) / ($2 * 60)) * ($2 * 60)) AS ts,
			AVG(cpu_usage_percent) AS cpu_avg,
			AVG(memory_percent) AS mem_avg,
			COUNT(*) AS sample_count
		 FROM system_metrics
		 WHERE timestamp > NOW() - INTERVAL '1 hour' * $1
		 GROUP BY ts
		 ORDER BY ts ASC`,
		hours, bucketMinutes,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summary []models.SystemMetricsSummary
	for rows.Next() {
		var s models.SystemMetricsSummary
		if err := rows.Scan(&s.Timestamp, &s.CPUAvg, &s.MemoryAvg, &s.SampleCount); err != nil {
			continue
		}
		summary = append(summary, s)
	}
	return summary, nil
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
		`INSERT INTO apt_status (host_id, last_update, last_upgrade, pending_packages, package_list, security_updates, cve_list, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,CAST($7 AS JSONB),NOW())
		 ON CONFLICT (host_id) DO UPDATE SET
			last_update = EXCLUDED.last_update,
			last_upgrade = EXCLUDED.last_upgrade,
			pending_packages = EXCLUDED.pending_packages,
			package_list = EXCLUDED.package_list,
			security_updates = EXCLUDED.security_updates,
			cve_list = EXCLUDED.cve_list,
			updated_at = NOW()`,
		status.HostID, status.LastUpdate, status.LastUpgrade, status.PendingPackages, status.PackageList, status.SecurityUpdates, status.CVEList,
	)
	return err
}

func (db *DB) GetAptStatus(hostID string) (*models.AptStatus, error) {
	var s models.AptStatus
	err := db.conn.QueryRow(
		`SELECT id, host_id, last_update, last_upgrade, pending_packages, package_list, security_updates, cve_list, updated_at
		 FROM apt_status WHERE host_id = $1`, hostID,
	).Scan(&s.ID, &s.HostID, &s.LastUpdate, &s.LastUpgrade, &s.PendingPackages, &s.PackageList, &s.SecurityUpdates, &s.CVEList, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (db *DB) CreateAptCommand(hostID, command, triggeredBy string, auditLogID *int64) (*models.AptCommand, error) {
	if triggeredBy == "" {
		triggeredBy = "system"
	}
	var cmd models.AptCommand
	err := db.conn.QueryRow(
		`INSERT INTO apt_commands (host_id, command, status, triggered_by, audit_log_id)
		 VALUES ($1, $2, 'pending', $3, $4)
		 RETURNING id, host_id, command, status, triggered_by, audit_log_id, created_at`,
		hostID, command, triggeredBy, auditLogID,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Command, &cmd.Status, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt)
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

// CleanupStalledCommands marks pending/running commands as failed if they're older than the timeout
func (db *DB) CleanupStalledCommands(timeoutMinutes int) error {
	query := `
		UPDATE apt_commands 
		SET status = 'failed', 
		    output = 'Command timed out - agent may have crashed or restarted', 
		    ended_at = NOW()
		WHERE status IN ('pending', 'running') 
		  AND created_at < NOW() - INTERVAL '1 minute' * $1
	`
	result, err := db.conn.Exec(query, timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup stalled commands: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		log.Printf("Cleaned up %d stalled APT commands", affected)
	}
	return nil
}

// CleanupHostStalledCommands marks old pending/running commands for a specific host as failed
func (db *DB) CleanupHostStalledCommands(hostID string, timeoutMinutes int) error {
	query := `
		UPDATE apt_commands 
		SET status = 'failed', 
			output = 'Command timed out - agent may have crashed or restarted', 
			ended_at = NOW()
		WHERE host_id = $1 
		  AND status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $2
	`
	result, err := db.conn.Exec(query, hostID, timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup host stalled commands: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected > 0 {
		log.Printf("Cleaned up %d stalled commands for host %s", affected, hostID)
	}
	return nil
}

func (db *DB) GetAptCommandByID(id int64) (*models.AptCommand, error) {
	var c models.AptCommand
	err := db.conn.QueryRow(
		`SELECT id, host_id, command, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM apt_commands WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.HostID, &c.Command, &c.Status, &c.Output, &c.TriggeredBy, &c.AuditLogID, &c.CreatedAt, &c.StartedAt, &c.EndedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) TouchAptLastAction(hostID, command string) error {
	now := time.Now()

	if command == "update" {
		_, err := db.conn.Exec(
			`INSERT INTO apt_status (host_id, last_update, pending_packages, package_list, security_updates, updated_at)
			 VALUES ($1, $2, 0, '[]', 0, NOW())
			 ON CONFLICT (host_id) DO UPDATE SET
				last_update = $2,
				updated_at = NOW()`,
			hostID, now,
		)
		return err
	}

	if command == "upgrade" || command == "dist-upgrade" {
		_, err := db.conn.Exec(
			`INSERT INTO apt_status (host_id, last_upgrade, pending_packages, package_list, security_updates, updated_at)
			 VALUES ($1, $2, 0, '[]', 0, NOW())
			 ON CONFLICT (host_id) DO UPDATE SET
				last_upgrade = $2,
				updated_at = NOW()`,
			hostID, now,
		)
		return err
	}

	return nil
}

func (db *DB) GetAptCommandHistory(hostID string, limit int) ([]models.AptCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, command, status, output, triggered_by, created_at, started_at, ended_at
		 FROM apt_commands WHERE host_id = $1 ORDER BY created_at DESC LIMIT $2`, hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.AptCommand
	for rows.Next() {
		var c models.AptCommand
		if err := rows.Scan(&c.ID, &c.HostID, &c.Command, &c.Status, &c.Output, &c.TriggeredBy, &c.CreatedAt, &c.StartedAt, &c.EndedAt); err != nil {
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

// ========== Audit Logs ==========

func (db *DB) CreateAuditLog(username, action, hostID, ipAddress, details, status string) (int64, error) {
	var id int64
	err := db.conn.QueryRow(
		`INSERT INTO audit_logs (username, action, host_id, ip_address, details, status)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		username, action, hostID, ipAddress, details, status,
	).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (db *DB) GetAuditLogs(limit, offset int) ([]models.AuditLog, error) {
	rows, err := db.conn.Query(
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 ORDER BY al.created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.Username, &log.Action, &log.HostID, &log.HostName, &log.IPAddress,
			&log.Details, &log.Status, &log.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (db *DB) GetAuditLogsByHost(hostID string, limit int) ([]models.AuditLog, error) {
	rows, err := db.conn.Query(
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 WHERE al.host_id = $1
		 ORDER BY al.created_at DESC LIMIT $2`,
		hostID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.Username, &log.Action, &log.HostID, &log.HostName, &log.IPAddress,
			&log.Details, &log.Status, &log.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (db *DB) GetAuditLogsByUser(username string, limit int) ([]models.AuditLog, error) {
	rows, err := db.conn.Query(
		`SELECT al.id, al.username, al.action, al.host_id,
			COALESCE(h.name, '') AS host_name,
			al.ip_address, al.details, al.status, al.created_at
		 FROM audit_logs al
		 LEFT JOIN hosts h ON al.host_id = h.id
		 WHERE al.username = $1
		 ORDER BY al.created_at DESC LIMIT $2`,
		username, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.Username, &log.Action, &log.HostID, &log.HostName, &log.IPAddress,
			&log.Details, &log.Status, &log.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, log)
	}
	return logs, nil
}

// CleanOldAuditLogs removes audit logs older than retentionDays (for compliance/storage reduction)
func (db *DB) CleanOldAuditLogs(retentionDays int) (int64, error) {
	result, err := db.conn.Exec(
		`DELETE FROM audit_logs WHERE created_at < NOW() - INTERVAL '1 day' * $1`,
		retentionDays,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (db *DB) UpdateAuditLogStatus(id int64, status, details string) error {
	_, err := db.conn.Exec(
		`UPDATE audit_logs
		 SET status = $1,
		     details = COALESCE(NULLIF($2, ''), details)
		 WHERE id = $3`,
		status, details, id,
	)
	return err
}

// ========== Alerts ==========

func (db *DB) CreateAlertRule(rule *models.AlertRule) error {
	return db.conn.QueryRow(
		`INSERT INTO alert_rules (host_id, metric, operator, threshold, duration_seconds, channel, channel_config, enabled)
		 VALUES ($1,$2,$3,$4,$5,$6,CAST($7 AS JSONB),$8)
		 RETURNING id`,
		rule.HostID, rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, rule.Channel, rule.ChannelConfig, rule.Enabled,
	).Scan(&rule.ID)
}

func (db *DB) UpdateAlertRule(rule *models.AlertRule) error {
	_, err := db.conn.Exec(
		`UPDATE alert_rules SET
			host_id = $1,
			metric = $2,
			operator = $3,
			threshold = $4,
			duration_seconds = $5,
			channel = $6,
			channel_config = CAST($7 AS JSONB),
			enabled = $8
		 WHERE id = $9`,
		rule.HostID, rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, rule.Channel, rule.ChannelConfig, rule.Enabled, rule.ID,
	)
	return err
}

func (db *DB) DeleteAlertRule(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

func (db *DB) GetAlertRules() ([]models.AlertRule, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, metric, operator, threshold, duration_seconds, channel, channel_config, enabled, created_at
		 FROM alert_rules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var hostID sql.NullString
		var channelConfig string
		if err := rows.Scan(&r.ID, &hostID, &r.Metric, &r.Operator, &r.Threshold, &r.DurationSeconds, &r.Channel, &channelConfig, &r.Enabled, &r.CreatedAt); err != nil {
			continue
		}
		if hostID.Valid {
			r.HostID = &hostID.String
		}
		r.ChannelConfig = channelConfig
		rules = append(rules, r)
	}
	return rules, nil
}

func (db *DB) GetOpenAlertIncident(ruleID int64, hostID string) (*models.AlertIncident, error) {
	var inc models.AlertIncident
	err := db.conn.QueryRow(
		`SELECT id, rule_id, host_id, triggered_at, resolved_at, value
		 FROM alert_incidents
		 WHERE rule_id = $1 AND host_id = $2 AND resolved_at IS NULL
		 ORDER BY triggered_at DESC LIMIT 1`,
		ruleID, hostID,
	).Scan(&inc.ID, &inc.RuleID, &inc.HostID, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value)
	if err != nil {
		return nil, err
	}
	return &inc, nil
}

// HasOpenIncident checks if there's an unresolved incident for a rule+host combination
func (db *DB) HasOpenIncident(ruleID int64, hostID string) bool {
	var id int64
	err := db.conn.QueryRow(
		`SELECT id FROM alert_incidents
		 WHERE rule_id = $1 AND host_id = $2 AND resolved_at IS NULL LIMIT 1`,
		ruleID, hostID,
	).Scan(&id)
	return err == nil
}

func (db *DB) CreateAlertIncident(ruleID int64, hostID string, value float64) error {
	_, err := db.conn.Exec(
		`INSERT INTO alert_incidents (rule_id, host_id, value) VALUES ($1, $2, $3)`,
		ruleID, hostID, value,
	)
	return err
}

func (db *DB) ResolveAlertIncident(id int64) error {
	_, err := db.conn.Exec(
		`UPDATE alert_incidents SET resolved_at = NOW() WHERE id = $1 AND resolved_at IS NULL`,
		id,
	)
	return err
}

func (db *DB) GetAlertIncidents(limit, offset int) ([]models.AlertIncident, error) {
	rows, err := db.conn.Query(
		`SELECT id, rule_id, host_id, triggered_at, resolved_at, value
		 FROM alert_incidents ORDER BY triggered_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []models.AlertIncident
	for rows.Next() {
		var inc models.AlertIncident
		if err := rows.Scan(&inc.ID, &inc.RuleID, &inc.HostID, &inc.TriggeredAt, &inc.ResolvedAt, &inc.Value); err != nil {
			continue
		}
		incidents = append(incidents, inc)
	}
	return incidents, nil
}

// ========== User TOTP ==========

func (db *DB) SetUserTOTPSecret(userID int64, secret, backupCodes string, enabled bool) error {
	_, err := db.conn.Exec(
		`UPDATE users SET totp_secret = $1, backup_codes = $2, mfa_enabled = $3 WHERE id = $4`,
		secret, backupCodes, enabled, userID,
	)
	return err
}

func (db *DB) GetUserTOTPSecret(username string) (string, error) {
	var secret string
	err := db.conn.QueryRow(
		`SELECT COALESCE(totp_secret, '') FROM users WHERE username = $1`,
		username,
	).Scan(&secret)
	return secret, err
}

func (db *DB) GetUserMFAStatus(username string) (bool, error) {
	var enabled bool
	err := db.conn.QueryRow(
		`SELECT mfa_enabled FROM users WHERE username = $1`,
		username,
	).Scan(&enabled)
	return enabled, err
}

func (db *DB) DisableUserMFA(username string) error {
	_, err := db.conn.Exec(
		`UPDATE users SET mfa_enabled = FALSE, totp_secret = '', backup_codes = '[]' WHERE username = $1`,
		username,
	)
	return err
}

func (db *DB) ConsumeMFABackupCode(username, usedCode string) error {
	var rawJSON string
	err := db.conn.QueryRow(
		`SELECT backup_codes FROM users WHERE username = $1`, username,
	).Scan(&rawJSON)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	var codes []string
	if err := json.Unmarshal([]byte(rawJSON), &codes); err != nil {
		return fmt.Errorf("invalid backup codes format: %w", err)
	}

	// Find and remove the used code (compare both plain and hashed versions)
	var remaining []string
	var found bool
	for _, hashed := range codes {
		// Try bcrypt comparison (codes are hashed)
		if bcrypt.CompareHashAndPassword([]byte(hashed), []byte(usedCode)) == nil {
			found = true
			continue // Skip this code (consumed)
		}
		remaining = append(remaining, hashed)
	}

	if !found {
		return fmt.Errorf("backup code not found or invalid")
	}

	newJSON, _ := json.Marshal(remaining)
	_, err = db.conn.Exec(
		`UPDATE users SET backup_codes = $1 WHERE username = $2`, string(newJSON), username,
	)
	return err
}

// ========== Metrics Aggregates (Downsampling) ==========

func (db *DB) InsertMetricsAggregate(agg *models.MetricsAggregate) error {
	_, err := db.conn.Exec(
		`INSERT INTO metrics_aggregates (host_id, aggregation_type, timestamp, cpu_usage_avg, cpu_usage_max,
		 memory_usage_avg, memory_usage_max, disk_usage_avg, network_rx_bytes, network_tx_bytes, sample_count)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 ON CONFLICT (host_id, aggregation_type, timestamp) DO NOTHING`,
		agg.HostID, agg.AggregationType, agg.Timestamp, agg.CPUUsageAvg, agg.CPUUsageMax,
		agg.MemoryUsageAvg, agg.MemoryUsageMax, agg.DiskUsageAvg, agg.NetworkRxBytes, agg.NetworkTxBytes, agg.SampleCount,
	)
	return err
}

func (db *DB) BuildMetricsAggregate(hostID string, start, end time.Time) (*models.MetricsAggregate, error) {
	var agg models.MetricsAggregate
	var sampleCount int
	var diskAvg sql.NullFloat64
	var rxDelta sql.NullInt64
	var txDelta sql.NullInt64
	var cpuAvg sql.NullFloat64
	var cpuMax sql.NullFloat64
	var memAvg sql.NullFloat64
	var memMax sql.NullFloat64

	err := db.conn.QueryRow(
		`SELECT
			AVG(cpu_usage_percent) AS cpu_avg,
			MAX(cpu_usage_percent) AS cpu_max,
			AVG(memory_used) AS mem_avg,
			MAX(memory_used) AS mem_max,
			COUNT(*) AS sample_count,
			MAX(network_rx_bytes) - MIN(network_rx_bytes) AS rx_delta,
			MAX(network_tx_bytes) - MIN(network_tx_bytes) AS tx_delta
		 FROM system_metrics
		 WHERE host_id = $1 AND timestamp >= $2 AND timestamp < $3`,
		hostID, start, end,
	).Scan(&cpuAvg, &cpuMax, &memAvg, &memMax, &sampleCount, &rxDelta, &txDelta)
	if err != nil {
		return nil, err
	}
	if sampleCount == 0 {
		return nil, nil
	}

	err = db.conn.QueryRow(
		`SELECT AVG(di.used_percent)
		 FROM system_metrics sm
		 JOIN disk_info di ON di.metrics_id = sm.id
		 WHERE sm.host_id = $1 AND sm.timestamp >= $2 AND sm.timestamp < $3`,
		hostID, start, end,
	).Scan(&diskAvg)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if diskAvg.Valid {
		agg.DiskUsageAvg = diskAvg.Float64
	}
	if cpuAvg.Valid {
		agg.CPUUsageAvg = cpuAvg.Float64
	}
	if cpuMax.Valid {
		agg.CPUUsageMax = cpuMax.Float64
	}
	if memAvg.Valid {
		agg.MemoryUsageAvg = uint64(memAvg.Float64)
	}
	if memMax.Valid {
		agg.MemoryUsageMax = uint64(memMax.Float64)
	}
	if rxDelta.Valid {
		agg.NetworkRxBytes = uint64(rxDelta.Int64)
	}
	if txDelta.Valid {
		agg.NetworkTxBytes = uint64(txDelta.Int64)
	}

	agg.HostID = hostID
	agg.SampleCount = sampleCount
	return &agg, nil
}

func (db *DB) GetMetricsAggregates(hostID string, aggregationType string, limit int) ([]models.MetricsAggregate, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, aggregation_type, timestamp, cpu_usage_avg, cpu_usage_max,
		 memory_usage_avg, memory_usage_max, disk_usage_avg, network_rx_bytes, network_tx_bytes, sample_count, created_at
		 FROM metrics_aggregates WHERE host_id = $1 AND aggregation_type = $2
		 ORDER BY timestamp DESC LIMIT $3`,
		hostID, aggregationType, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aggs []models.MetricsAggregate
	for rows.Next() {
		var agg models.MetricsAggregate
		if err := rows.Scan(&agg.ID, &agg.HostID, &agg.AggregationType, &agg.Timestamp, &agg.CPUUsageAvg, &agg.CPUUsageMax,
			&agg.MemoryUsageAvg, &agg.MemoryUsageMax, &agg.DiskUsageAvg, &agg.NetworkRxBytes, &agg.NetworkTxBytes, &agg.SampleCount, &agg.CreatedAt); err != nil {
			continue
		}
		aggs = append(aggs, agg)
	}
	return aggs, nil
}

// DeleteOldMetrics deletes raw metrics older than retentionDays and based on downsampling policy
func (db *DB) DeleteOldMetrics(hostID string, retentionDays int) error {
	// Keep raw metrics for only retentionDays
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	_, err := db.conn.Exec(
		`DELETE FROM system_metrics WHERE host_id = $1 AND timestamp < $2`,
		hostID, cutoffTime,
	)
	return err
}

// UpdateHostStatusBasedOnLastSeen updates host status to 'offline' if not seen for too long
func (db *DB) UpdateHostStatusBasedOnLastSeen(maxOfflineMinutes int) error {
	cutoffTime := time.Now().Add(time.Duration(-maxOfflineMinutes) * time.Minute)
	_, err := db.conn.Exec(
		`UPDATE hosts SET status = 'offline' WHERE status != 'offline' AND last_seen < $1`,
		cutoffTime,
	)
	return err
}

// GetHostHealthStatus returns health check info for a host
func (db *DB) GetHostHealthStatus(hostID string) (status string, lastSeen time.Time, err error) {
	err = db.conn.QueryRow(
		`SELECT status, last_seen FROM hosts WHERE id = $1`,
		hostID,
	).Scan(&status, &lastSeen)
	return
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
// CountAuditLogs returns the total number of audit log entries
func (db *DB) CountAuditLogs() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&count)
	return count, err
}

// CountMetrics returns the total number of metrics records
func (db *DB) CountMetrics() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM metrics`).Scan(&count)
	return count, err
}

// CountHosts returns the total number of registered hosts
func (db *DB) CountHosts() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&count)
	return count, err
}