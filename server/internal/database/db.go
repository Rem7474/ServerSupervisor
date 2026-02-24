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

func (db *DB) Ping() error {
	return db.conn.Ping()
}

// Query executes a query that returns rows
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

// Exec executes a query without returning any rows
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
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
		// Migration: Docker networks and automatic topology detection
		`CREATE TABLE IF NOT EXISTS docker_networks (
			id VARCHAR(64) PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			network_id VARCHAR(64) NOT NULL,
			name VARCHAR(255) NOT NULL,
			driver VARCHAR(50) DEFAULT 'bridge',
			scope VARCHAR(20) DEFAULT 'local',
			container_ids JSONB DEFAULT '[]'::jsonb,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_networks_host ON docker_networks(host_id)`,
		// Migration: Container environment variables (for topology inference)
		`CREATE TABLE IF NOT EXISTS container_envs (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			container_name VARCHAR(255) NOT NULL,
			env_vars JSONB DEFAULT '{}'::jsonb,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_container_envs_host ON container_envs(host_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_container_envs_host_name ON container_envs(host_id, container_name)`,
		// Migration: Network topology configuration (persistent)
		`CREATE TABLE IF NOT EXISTS network_topology_config (
			id SERIAL PRIMARY KEY,
			root_label VARCHAR(255) DEFAULT 'Infrastructure',
			root_ip VARCHAR(45) DEFAULT '',
			excluded_ports JSONB DEFAULT '[]'::jsonb,
			service_map TEXT DEFAULT '{}',
			show_proxy_links BOOLEAN DEFAULT TRUE,
			host_overrides TEXT DEFAULT '{}',
			manual_services TEXT DEFAULT '[]',
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		// Insert default config if not exists
		`INSERT INTO network_topology_config (id, root_label) 
		 SELECT 1, 'Infrastructure' WHERE NOT EXISTS (SELECT 1 FROM network_topology_config)`,
		// Create partial unique index for singleton pattern (PostgreSQL doesn't support WHERE in UNIQUE constraints)
		`CREATE UNIQUE INDEX IF NOT EXISTS network_topology_config_singleton ON network_topology_config (id) WHERE id = 1`,
		// Migration: Docker Compose projects
		`CREATE TABLE IF NOT EXISTS compose_projects (
			id VARCHAR(255) PRIMARY KEY,
			host_id VARCHAR(64) NOT NULL REFERENCES hosts(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			working_dir TEXT NOT NULL DEFAULT '',
			config_file TEXT NOT NULL DEFAULT '',
			services TEXT NOT NULL DEFAULT '[]',
			raw_config TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_compose_projects_host_id ON compose_projects(host_id)`,
		// Migration: Add env_vars, volumes, networks to docker_containers
		`ALTER TABLE IF EXISTS docker_containers ADD COLUMN IF NOT EXISTS env_vars JSONB DEFAULT '{}'::jsonb`,
		`ALTER TABLE IF EXISTS docker_containers ADD COLUMN IF NOT EXISTS volumes JSONB DEFAULT '[]'::jsonb`,
		`ALTER TABLE IF EXISTS docker_containers ADD COLUMN IF NOT EXISTS networks JSONB DEFAULT '[]'::jsonb`,
		// Migration: Create docker_commands table for container management
		`CREATE TABLE IF NOT EXISTS docker_commands (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			container_name VARCHAR(255) NOT NULL,
			action VARCHAR(20) NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			output TEXT DEFAULT '',
			triggered_by VARCHAR(255) DEFAULT 'system',
			audit_log_id BIGINT,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			started_at TIMESTAMP WITH TIME ZONE,
			ended_at TIMESTAMP WITH TIME ZONE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_docker_commands_host_status ON docker_commands(host_id, status)`,
		// Migration: Disk metrics table (detailed usage per mount point with inodes)
		`CREATE TABLE IF NOT EXISTS disk_metrics (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			mount_point VARCHAR(255) NOT NULL,
			filesystem VARCHAR(255) NOT NULL DEFAULT '',
			size_gb DOUBLE PRECISION DEFAULT 0,
			used_gb DOUBLE PRECISION DEFAULT 0,
			avail_gb DOUBLE PRECISION DEFAULT 0,
			used_percent DOUBLE PRECISION DEFAULT 0,
			inodes_total BIGINT DEFAULT 0,
			inodes_used BIGINT DEFAULT 0,
			inodes_free BIGINT DEFAULT 0,
			inodes_percent DOUBLE PRECISION DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_disk_metrics_host_time ON disk_metrics(host_id, timestamp DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_disk_metrics_host_mount ON disk_metrics(host_id, mount_point, timestamp DESC)`,
		// Migration: Disk health table (SMART monitoring)
		`CREATE TABLE IF NOT EXISTS disk_health (
			id BIGSERIAL PRIMARY KEY,
			host_id VARCHAR(64) REFERENCES hosts(id) ON DELETE CASCADE,
			timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			device VARCHAR(255) NOT NULL,
			model VARCHAR(255) NOT NULL DEFAULT '',
			serial_number VARCHAR(255) NOT NULL DEFAULT '',
			smart_status VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN',
			temperature INTEGER DEFAULT 0,
			power_on_hours BIGINT DEFAULT 0,
			power_cycles BIGINT DEFAULT 0,
			realloc_sectors INTEGER DEFAULT 0,
			pending_sectors INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_disk_health_host_time ON disk_health(host_id, timestamp DESC)`,
		// Migration: Add extra columns to alert_rules
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS name VARCHAR(255)`,
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS channels JSONB DEFAULT '[]'::jsonb`,
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS smtp_to VARCHAR(255)`,
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS ntfy_topic VARCHAR(255)`,
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS cooldown INTEGER`,
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS last_fired TIMESTAMP WITH TIME ZONE`,
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()`,
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

// GetMetricsAggregatesByType returns aggregated metrics (hourly or daily) to reduce data points
func (db *DB) GetMetricsAggregatesByType(hostID string, hours int, aggregationType string) ([]models.SystemMetrics, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, timestamp, cpu_usage_avg as cpu_usage_percent, 0 as cpu_cores, 
		 0 as load_avg_1, 0 as load_avg_5, 0 as load_avg_15,
		 0 as memory_total, memory_usage_avg as memory_used, 0 as memory_free, 0 as memory_percent, 
		 0 as swap_total, 0 as swap_used,
		 network_rx_bytes, network_tx_bytes, 0 as uptime
		 FROM metrics_aggregates
		 WHERE host_id = $1 AND aggregation_type = $2 AND timestamp > NOW() - INTERVAL '1 hour' * $3
		 ORDER BY timestamp ASC`, hostID, aggregationType, hours,
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
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Delete raw metrics
	rawResult, err := tx.Exec(
		`DELETE FROM system_metrics WHERE timestamp < NOW() - INTERVAL '1 day' * $1`,
		retentionDays,
	)
	if err != nil {
		return 0, err
	}
	rawDeleted, _ := rawResult.RowsAffected()

	// Delete aggregates of the same period
	_, err = tx.Exec(
		`DELETE FROM metrics_aggregates WHERE timestamp < NOW() - INTERVAL '1 day' * $1`,
		retentionDays,
	)
	if err != nil {
		return rawDeleted, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return rawDeleted, nil
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
		envVarsJSON, _ := json.Marshal(c.EnvVars)
		volumesJSON, _ := json.Marshal(c.Volumes)
		networksJSON, _ := json.Marshal(c.Networks)
		_, err := db.conn.Exec(
			`INSERT INTO docker_containers (id, host_id, container_id, name, image, image_tag, image_id, state, status, created, ports, labels, env_vars, volumes, networks, updated_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NOW())`,
			c.ID, hostID, c.ContainerID, c.Name, c.Image, c.ImageTag, c.ImageID, c.State, c.Status, c.Created, c.Ports,
			string(labelsJSON), string(envVarsJSON), string(volumesJSON), string(networksJSON),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetDockerContainers(hostID string) ([]models.DockerContainer, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, container_id, name, image, image_tag, image_id, state, status, created, ports, labels,
		 COALESCE(env_vars::text, '{}'), COALESCE(volumes::text, '[]'), COALESCE(networks::text, '[]'), updated_at
		 FROM docker_containers WHERE host_id = $1 ORDER BY name`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []models.DockerContainer
	for rows.Next() {
		var c models.DockerContainer
		var labelsJSON, envVarsJSON, volumesJSON, networksJSON string
		if err := rows.Scan(&c.ID, &c.HostID, &c.ContainerID, &c.Name, &c.Image, &c.ImageTag, &c.ImageID,
			&c.State, &c.Status, &c.Created, &c.Ports, &labelsJSON, &envVarsJSON, &volumesJSON, &networksJSON, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(labelsJSON), &c.Labels)
		json.Unmarshal([]byte(envVarsJSON), &c.EnvVars)
		json.Unmarshal([]byte(volumesJSON), &c.Volumes)
		json.Unmarshal([]byte(networksJSON), &c.Networks)
		containers = append(containers, c)
	}
	return containers, nil
}

func (db *DB) GetAllDockerContainers() ([]models.DockerContainer, error) {
	rows, err := db.conn.Query(
		`SELECT dc.id, dc.host_id, h.hostname, dc.container_id, dc.name, dc.image, dc.image_tag, dc.image_id,
		 dc.state, dc.status, dc.created, dc.ports, dc.labels,
		 COALESCE(dc.env_vars::text, '{}'), COALESCE(dc.volumes::text, '[]'), COALESCE(dc.networks::text, '[]'), dc.updated_at
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
		var labelsJSON, envVarsJSON, volumesJSON, networksJSON string
		if err := rows.Scan(&c.ID, &c.HostID, &c.Hostname, &c.ContainerID, &c.Name, &c.Image, &c.ImageTag, &c.ImageID,
			&c.State, &c.Status, &c.Created, &c.Ports, &labelsJSON, &envVarsJSON, &volumesJSON, &networksJSON, &c.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal([]byte(labelsJSON), &c.Labels)
		json.Unmarshal([]byte(envVarsJSON), &c.EnvVars)
		json.Unmarshal([]byte(volumesJSON), &c.Volumes)
		json.Unmarshal([]byte(networksJSON), &c.Networks)
		containers = append(containers, c)
	}
	return containers, nil
}

// ========== Docker Commands ==========

func (db *DB) CreateDockerCommand(hostID, containerName, action, triggeredBy string, auditLogID *int64) (*models.DockerCommand, error) {
	if triggeredBy == "" {
		triggeredBy = "system"
	}
	var cmd models.DockerCommand
	err := db.conn.QueryRow(
		`INSERT INTO docker_commands (host_id, container_name, action, status, triggered_by, audit_log_id)
		 VALUES ($1, $2, $3, 'pending', $4, $5)
		 RETURNING id, host_id, container_name, action, status, triggered_by, audit_log_id, created_at`,
		hostID, containerName, action, triggeredBy, auditLogID,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.ContainerName, &cmd.Action, &cmd.Status, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (db *DB) GetDockerCommandByID(id int64) (*models.DockerCommand, error) {
	var c models.DockerCommand
	var startedAt, endedAt sql.NullTime
	err := db.conn.QueryRow(
		`SELECT id, host_id, container_name, action, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM docker_commands WHERE id = $1`, id,
	).Scan(&c.ID, &c.HostID, &c.ContainerName, &c.Action, &c.Status, &c.Output, &c.TriggeredBy, &c.AuditLogID, &c.CreatedAt, &startedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		c.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		c.EndedAt = &endedAt.Time
	}
	return &c, nil
}

func (db *DB) GetPendingDockerCommands(hostID string) ([]models.PendingCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, container_name, action FROM docker_commands WHERE host_id = $1 AND status = 'pending' ORDER BY created_at ASC`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.PendingCommand
	for rows.Next() {
		var id int64
		var containerName, action string
		if err := rows.Scan(&id, &containerName, &action); err != nil {
			continue
		}
		payload := fmt.Sprintf(`{"container_name":%q,"action":%q}`, containerName, action)
		cmds = append(cmds, models.PendingCommand{
			ID:      id,
			Type:    "docker",
			Payload: payload,
		})
	}
	return cmds, nil
}

func (db *DB) UpdateDockerCommandStatus(id int64, status, output string) error {
	switch status {
	case "running":
		_, err := db.conn.Exec(`UPDATE docker_commands SET status = $1, started_at = NOW() WHERE id = $2`, status, id)
		return err
	default:
		_, err := db.conn.Exec(`UPDATE docker_commands SET status = $1, output = $2, ended_at = NOW() WHERE id = $3`, status, output, id)
		return err
	}
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
		`SELECT id, name, host_id, metric, operator, threshold, duration_seconds, channel, channel_config, 
		        channels, smtp_to, ntfy_topic, cooldown, last_fired, enabled, created_at, updated_at
		 FROM alert_rules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var name, smtpTo, ntfyTopic sql.NullString
		var hostID sql.NullString
		var threshold sql.NullFloat64
		var channelsJSON []byte
		var cooldown sql.NullInt32
		var lastFired, updatedAt sql.NullTime
		var channelConfig string

		if err := rows.Scan(
			&r.ID, &name, &hostID, &r.Metric, &r.Operator, &threshold, &r.DurationSeconds,
			&r.Channel, &channelConfig, &channelsJSON, &smtpTo, &ntfyTopic, &cooldown,
			&lastFired, &r.Enabled, &r.CreatedAt, &updatedAt,
		); err != nil {
			continue
		}

		if name.Valid {
			r.Name = &name.String
		}
		if hostID.Valid {
			r.HostID = &hostID.String
		}
		if threshold.Valid {
			r.Threshold = &threshold.Float64
		}
		if smtpTo.Valid {
			r.SMTPTo = &smtpTo.String
		}
		if ntfyTopic.Valid {
			r.NtfyTopic = &ntfyTopic.String
		}
		if cooldown.Valid {
			cooldownInt := int(cooldown.Int32)
			r.Cooldown = &cooldownInt
		}
		if lastFired.Valid {
			r.LastFired = &lastFired.Time
		}
		if updatedAt.Valid {
			r.UpdatedAt = &updatedAt.Time
		}

		r.ChannelConfig = channelConfig
		if len(channelsJSON) > 0 {
			json.Unmarshal(channelsJSON, &r.Channels)
		} else {
			r.Channels = []string{}
		}

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
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM system_metrics`).Scan(&count)
	return count, err
}

// CountHosts returns the total number of registered hosts
func (db *DB) CountHosts() (int64, error) {
	var count int64
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&count)
	return count, err
}

// GetAptHistoryWithAgentUpdates returns combined APT command history and agent-initiated updates from audit logs
func (db *DB) GetAptHistoryWithAgentUpdates(hostID string, limit int) ([]models.AptCommand, error) {
	rows, err := db.conn.Query(`
		SELECT id, host_id, command, status, output, triggered_by, created_at, started_at, ended_at, audit_log_id FROM (
			-- APT commands from apt_commands table
			SELECT 
				id, host_id, command, status, output, triggered_by, 
				created_at, started_at, ended_at, NULL::BIGINT as audit_log_id
			FROM apt_commands 
			WHERE host_id = $1
			
			UNION ALL
			
			-- Agent-initiated updates from audit_logs table (only "update" actions by agent)
			SELECT 
				id, host_id, action as command, status, details as output, username as triggered_by,
				created_at, NULL::TIMESTAMP, NULL::TIMESTAMP, id as audit_log_id
			FROM audit_logs
			WHERE host_id = $1 AND action = 'update' AND username = 'agent'
		) combined
		ORDER BY created_at DESC
		LIMIT $2
	`, hostID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.AptCommand
	for rows.Next() {
		var c models.AptCommand
		if err := rows.Scan(&c.ID, &c.HostID, &c.Command, &c.Status, &c.Output, &c.TriggeredBy, &c.CreatedAt, &c.StartedAt, &c.EndedAt, &c.AuditLogID); err != nil {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

// ========== Network Topology ==========

func (db *DB) UpsertDockerNetworks(hostID string, networks []models.DockerNetwork) error {
	if len(networks) == 0 {
		return nil
	}
	for _, n := range networks {
		containerIDsJSON, _ := json.Marshal(n.ContainerIDs)
		_, err := db.conn.Exec(
			`INSERT INTO docker_networks (id, host_id, network_id, name, driver, scope, container_ids, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
			 ON CONFLICT(id) DO UPDATE SET
			 container_ids = $7, updated_at = NOW()`,
			n.ID, hostID, n.NetworkID, n.Name, n.Driver, n.Scope, containerIDsJSON,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetDockerNetworksByHost(hostID string) ([]models.DockerNetwork, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, network_id, name, driver, scope, container_ids, updated_at
		 FROM docker_networks WHERE host_id = $1`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var networks []models.DockerNetwork
	for rows.Next() {
		var n models.DockerNetwork
		var containerIDsJSON []byte
		if err := rows.Scan(&n.ID, &n.HostID, &n.NetworkID, &n.Name, &n.Driver, &n.Scope, &containerIDsJSON, &n.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal(containerIDsJSON, &n.ContainerIDs)
		networks = append(networks, n)
	}
	return networks, nil
}

func (db *DB) GetAllDockerNetworks() ([]models.DockerNetwork, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, network_id, name, driver, scope, container_ids, updated_at
		 FROM docker_networks ORDER BY host_id, name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var networks []models.DockerNetwork
	for rows.Next() {
		var n models.DockerNetwork
		var containerIDsJSON []byte
		if err := rows.Scan(&n.ID, &n.HostID, &n.NetworkID, &n.Name, &n.Driver, &n.Scope, &containerIDsJSON, &n.UpdatedAt); err != nil {
			continue
		}
		json.Unmarshal(containerIDsJSON, &n.ContainerIDs)
		networks = append(networks, n)
	}
	return networks, nil
}

func (db *DB) UpsertContainerEnvs(hostID string, envs []models.ContainerEnv) error {
	if len(envs) == 0 {
		return nil
	}
	for _, env := range envs {
		envVarsJSON, _ := json.Marshal(env.EnvVars)
		_, err := db.conn.Exec(
			`INSERT INTO container_envs (host_id, container_name, env_vars, updated_at)
			 VALUES ($1, $2, $3, NOW())
			 ON CONFLICT(host_id, container_name) DO UPDATE SET
			 env_vars = $3, updated_at = NOW()`,
			hostID, env.ContainerName, envVarsJSON,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetContainerEnvsByHost(hostID string) ([]models.ContainerEnv, error) {
	rows, err := db.conn.Query(
		`SELECT container_name, env_vars FROM container_envs WHERE host_id = $1`,
		hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []models.ContainerEnv
	for rows.Next() {
		var env models.ContainerEnv
		var envVarsJSON []byte
		if err := rows.Scan(&env.ContainerName, &envVarsJSON); err != nil {
			continue
		}
		json.Unmarshal(envVarsJSON, &env.EnvVars)
		envs = append(envs, env)
	}
	return envs, nil
}

func (db *DB) GetAllContainerEnvs() ([]models.ContainerEnv, error) {
	rows, err := db.conn.Query(
		`SELECT container_name, env_vars FROM container_envs ORDER BY host_id, container_name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []models.ContainerEnv
	for rows.Next() {
		var env models.ContainerEnv
		var envVarsJSON []byte
		if err := rows.Scan(&env.ContainerName, &envVarsJSON); err != nil {
			continue
		}
		json.Unmarshal(envVarsJSON, &env.EnvVars)
		envs = append(envs, env)
	}
	return envs, nil
}

func (db *DB) GetNetworkTopologyConfig() (*models.NetworkTopologyConfig, error) {
	var cfg models.NetworkTopologyConfig
	var excludedPortsJSON []byte
	err := db.conn.QueryRow(
		`SELECT id, root_label, root_ip, excluded_ports, service_map, show_proxy_links, host_overrides, manual_services, updated_at
		 FROM network_topology_config LIMIT 1`,
	).Scan(&cfg.ID, &cfg.RootLabel, &cfg.RootIP, &excludedPortsJSON, &cfg.ServiceMap, &cfg.ShowProxyLinks, &cfg.HostOverrides, &cfg.ManualServices, &cfg.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default config if none exists
			return &models.NetworkTopologyConfig{
				RootLabel:      "Infrastructure",
				ShowProxyLinks: true,
				ExcludedPorts:  []int{},
				ServiceMap:     "{}",
				HostOverrides:  "{}",
				ManualServices: "[]",
			}, nil
		}
		return nil, err
	}
	// Unmarshal JSONB excluded_ports
	if len(excludedPortsJSON) > 0 {
		json.Unmarshal(excludedPortsJSON, &cfg.ExcludedPorts)
	}
	return &cfg, nil
}

func (db *DB) SaveNetworkTopologyConfig(cfg *models.NetworkTopologyConfig) error {
	excludedPortsJSON, _ := json.Marshal(cfg.ExcludedPorts)
	_, err := db.conn.Exec(
		`INSERT INTO network_topology_config (id, root_label, root_ip, excluded_ports, service_map, show_proxy_links, host_overrides, manual_services, updated_at)
		 VALUES (1, $1, $2, $3::jsonb, $4, $5, $6, $7, NOW())
		 ON CONFLICT(id) DO UPDATE SET
		   root_label = EXCLUDED.root_label,
		   root_ip = EXCLUDED.root_ip,
		   excluded_ports = EXCLUDED.excluded_ports,
		   service_map = EXCLUDED.service_map,
		   show_proxy_links = EXCLUDED.show_proxy_links,
		   host_overrides = EXCLUDED.host_overrides,
		   manual_services = EXCLUDED.manual_services,
		   updated_at = NOW()`,
		cfg.RootLabel, cfg.RootIP, excludedPortsJSON,
		cfg.ServiceMap, cfg.ShowProxyLinks, cfg.HostOverrides,
		cfg.ManualServices,
	)
	return err
}

// ========== Disk Metrics ==========

func (db *DB) InsertDiskMetrics(metrics []models.DiskMetrics) error {
	if len(metrics) == 0 {
		return nil
	}

	query := `
		INSERT INTO disk_metrics (
			host_id, timestamp, mount_point, filesystem,
			size_gb, used_gb, avail_gb, used_percent,
			inodes_total, inodes_used, inodes_free, inodes_percent
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	for _, m := range metrics {
		_, err := db.conn.Exec(query,
			m.HostID, m.Timestamp, m.MountPoint, m.Filesystem,
			m.SizeGB, m.UsedGB, m.AvailGB, m.UsedPercent,
			m.InodesTotal, m.InodesUsed, m.InodesFree, m.InodesPercent,
		)
		if err != nil {
			return fmt.Errorf("failed to insert disk metrics: %w", err)
		}
	}

	return nil
}

func (db *DB) GetLatestDiskMetrics(hostID string) ([]models.DiskMetrics, error) {
	query := `
		SELECT DISTINCT ON (mount_point)
			id, host_id, timestamp, mount_point, filesystem,
			size_gb, used_gb, avail_gb, used_percent,
			inodes_total, inodes_used, inodes_free, inodes_percent
		FROM disk_metrics
		WHERE host_id = $1
		ORDER BY mount_point, timestamp DESC
	`

	rows, err := db.conn.Query(query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.DiskMetrics
	for rows.Next() {
		var m models.DiskMetrics
		err := rows.Scan(
			&m.ID, &m.HostID, &m.Timestamp, &m.MountPoint, &m.Filesystem,
			&m.SizeGB, &m.UsedGB, &m.AvailGB, &m.UsedPercent,
			&m.InodesTotal, &m.InodesUsed, &m.InodesFree, &m.InodesPercent,
		)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

func (db *DB) GetDiskMetricsHistory(hostID, mountPoint string, limit int) ([]models.DiskMetrics, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, host_id, timestamp, mount_point, filesystem,
			   size_gb, used_gb, avail_gb, used_percent,
			   inodes_total, inodes_used, inodes_free, inodes_percent
		FROM disk_metrics
		WHERE host_id = $1 AND mount_point = $2
		ORDER BY timestamp DESC
		LIMIT $3
	`

	rows, err := db.conn.Query(query, hostID, mountPoint, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.DiskMetrics
	for rows.Next() {
		var m models.DiskMetrics
		err := rows.Scan(
			&m.ID, &m.HostID, &m.Timestamp, &m.MountPoint, &m.Filesystem,
			&m.SizeGB, &m.UsedGB, &m.AvailGB, &m.UsedPercent,
			&m.InodesTotal, &m.InodesUsed, &m.InodesFree, &m.InodesPercent,
		)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, nil
}

// ========== Disk Health (SMART) ==========

func (db *DB) InsertDiskHealth(healthData []models.DiskHealth) error {
	if len(healthData) == 0 {
		return nil
	}

	query := `
		INSERT INTO disk_health (
			host_id, timestamp, device, model, serial_number,
			smart_status, temperature, power_on_hours, power_cycles,
			realloc_sectors, pending_sectors
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	for _, h := range healthData {
		_, err := db.conn.Exec(query,
			h.HostID, h.CollectedAt, h.Device, h.Model, h.SerialNumber,
			h.SmartStatus, h.Temperature, h.PowerOnHours, h.PowerCycles,
			h.ReallocSectors, h.PendingSectors,
		)
		if err != nil {
			return fmt.Errorf("failed to insert disk health: %w", err)
		}
	}

	return nil
}

func (db *DB) GetLatestDiskHealth(hostID string) ([]models.DiskHealth, error) {
	query := `
		SELECT DISTINCT ON (device)
			id, host_id, timestamp, device, model, serial_number,
			smart_status, temperature, power_on_hours, power_cycles,
			realloc_sectors, pending_sectors
		FROM disk_health
		WHERE host_id = $1
		ORDER BY device, timestamp DESC
	`

	rows, err := db.conn.Query(query, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var healthData []models.DiskHealth
	for rows.Next() {
		var h models.DiskHealth
		err := rows.Scan(
			&h.ID, &h.HostID, &h.CollectedAt, &h.Device, &h.Model, &h.SerialNumber,
			&h.SmartStatus, &h.Temperature, &h.PowerOnHours, &h.PowerCycles,
			&h.ReallocSectors, &h.PendingSectors,
		)
		if err != nil {
			return nil, err
		}
		healthData = append(healthData, h)
	}

	return healthData, nil
}
