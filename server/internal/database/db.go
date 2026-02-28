package database

import (
	"crypto/rand"
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
		// Migration: add Authelia/Internet topology node fields
		`ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS authelia_label VARCHAR(255) DEFAULT 'Authelia'`,
		`ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS authelia_ip VARCHAR(45) DEFAULT ''`,
		`ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS internet_label VARCHAR(255) DEFAULT 'Internet'`,
		`ALTER TABLE network_topology_config ADD COLUMN IF NOT EXISTS internet_ip VARCHAR(45) DEFAULT ''`,
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
		// Migration: Add working_dir to docker_commands (for compose project actions)
		`ALTER TABLE IF EXISTS docker_commands ADD COLUMN IF NOT EXISTS working_dir TEXT DEFAULT ''`,
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
		// Migration: Add must_change_password flag to users (for first-login enforcement)
		`ALTER TABLE IF EXISTS users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE`,
		// Migration: Login events table for security auditing and brute-force detection
		`CREATE TABLE IF NOT EXISTS login_events (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(255) NOT NULL,
			ip_address VARCHAR(45) NOT NULL DEFAULT '',
			success BOOLEAN NOT NULL,
			user_agent VARCHAR(500) DEFAULT '',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_login_events_ip_time ON login_events(ip_address, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_login_events_user_time ON login_events(username, created_at DESC)`,
		// Migration: Dynamic settings table (DB overrides env vars, no restart needed)
		`CREATE TABLE IF NOT EXISTS settings (
			key VARCHAR(100) PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		// Migration: Unified remote_commands table (replaces docker_commands + apt_commands)
		`CREATE TABLE IF NOT EXISTS remote_commands (
			id           VARCHAR(36)  PRIMARY KEY,
			host_id      VARCHAR(64)  REFERENCES hosts(id) ON DELETE CASCADE,
			module       VARCHAR(50)  NOT NULL,
			action       VARCHAR(100) NOT NULL,
			target       VARCHAR(255) NOT NULL DEFAULT '',
			payload      TEXT         NOT NULL DEFAULT '{}',
			status       VARCHAR(20)  NOT NULL DEFAULT 'pending',
			output       TEXT         NOT NULL DEFAULT '',
			triggered_by VARCHAR(255) NOT NULL DEFAULT 'system',
			audit_log_id BIGINT,
			created_at   TIMESTAMPTZ  DEFAULT NOW(),
			started_at   TIMESTAMPTZ,
			ended_at     TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_remote_commands_host_status ON remote_commands(host_id, status)`,
		`DROP TABLE IF EXISTS docker_commands`,
		`DROP TABLE IF EXISTS apt_commands`,
		// Migration: Consolidate alert_rules notification config into single actions JSONB column
		`ALTER TABLE IF EXISTS alert_rules ADD COLUMN IF NOT EXISTS actions JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`UPDATE alert_rules SET actions = jsonb_build_object(
			'channels', COALESCE(channels, '[]'::jsonb),
			'smtp_to', COALESCE(smtp_to, ''),
			'ntfy_topic', COALESCE(ntfy_topic, ''),
			'cooldown', COALESCE(cooldown, 0)
		) WHERE actions = '{}'::jsonb OR actions IS NULL`,
		`ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channel`,
		`ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channel_config`,
		`ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS channels`,
		`ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS smtp_to`,
		`ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS ntfy_topic`,
		`ALTER TABLE IF EXISTS alert_rules DROP COLUMN IF EXISTS cooldown`,
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

func (db *DB) CreateUser(username, passwordHash, role string, mustChangePassword ...bool) error {
	mcp := len(mustChangePassword) > 0 && mustChangePassword[0]
	_, err := db.conn.Exec(
		`INSERT INTO users (username, password_hash, role, must_change_password) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (username) DO NOTHING`,
		username, passwordHash, role, mcp,
	)
	return err
}

func (db *DB) SetUserMustChangePassword(username string, value bool) error {
	_, err := db.conn.Exec(
		`UPDATE users SET must_change_password = $1 WHERE username = $2`,
		value, username,
	)
	return err
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, role, totp_secret, backup_codes, mfa_enabled, must_change_password, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.MustChangePassword, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (db *DB) GetUserByID(id int64) (*models.User, error) {
	var u models.User
	err := db.conn.QueryRow(
		`SELECT id, username, password_hash, role, totp_secret, backup_codes, mfa_enabled, must_change_password, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.TOTPSecret, &u.BackupCodes, &u.MFAEnabled, &u.MustChangePassword, &u.CreatedAt)
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
		`UPDATE users SET password_hash = $1, must_change_password = FALSE WHERE username = $2`,
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

// ========== Login Events ==========

func (db *DB) CreateLoginEvent(username, ipAddress, userAgent string, success bool) error {
	_, err := db.conn.Exec(
		`INSERT INTO login_events (username, ip_address, user_agent, success) VALUES ($1, $2, $3, $4)`,
		username, ipAddress, userAgent, success,
	)
	return err
}

func (db *DB) GetLoginEventsByUser(username string, limit int) ([]models.LoginEvent, error) {
	rows, err := db.conn.Query(
		`SELECT id, username, ip_address, success, user_agent, created_at
		 FROM login_events WHERE username = $1 ORDER BY created_at DESC LIMIT $2`,
		username, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []models.LoginEvent
	for rows.Next() {
		var e models.LoginEvent
		if err := rows.Scan(&e.ID, &e.Username, &e.IPAddress, &e.Success, &e.UserAgent, &e.CreatedAt); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func (db *DB) CountRecentFailedLogins(ipAddress string, since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*) FROM login_events WHERE ip_address = $1 AND success = FALSE AND created_at >= $2`,
		ipAddress, since,
	).Scan(&count)
	return count, err
}

// GetLoginStats returns aggregate login counts for the given time window.
func (db *DB) GetLoginStats(since time.Time) (*models.LoginStats, error) {
	var stats models.LoginStats
	err := db.conn.QueryRow(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE NOT success) AS failures,
			COUNT(DISTINCT ip_address) AS unique_ips
		FROM login_events WHERE created_at > $1
	`, since).Scan(&stats.Total, &stats.Failures, &stats.UniqueIPs)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetTopFailedIPs returns the IPs with the most failed login attempts in the given window.
func (db *DB) GetTopFailedIPs(since time.Time, limit int) ([]models.IPFailCount, error) {
	rows, err := db.conn.Query(`
		SELECT ip_address, COUNT(*) AS fail_count
		FROM login_events
		WHERE success = false AND created_at > $1
		GROUP BY ip_address
		ORDER BY fail_count DESC
		LIMIT $2
	`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.IPFailCount
	for rows.Next() {
		var item models.IPFailCount
		if err := rows.Scan(&item.IPAddress, &item.FailCount); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
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
	// UPSERT each container, tracking IDs to prune removed ones afterwards
	ids := make([]string, 0, len(containers))
	for _, c := range containers {
		labelsJSON, _ := json.Marshal(c.Labels)
		envVarsJSON, _ := json.Marshal(c.EnvVars)
		volumesJSON, _ := json.Marshal(c.Volumes)
		networksJSON, _ := json.Marshal(c.Networks)
		_, err := db.conn.Exec(`
			INSERT INTO docker_containers (id, host_id, container_id, name, image, image_tag, image_id, state, status, created, ports, labels, env_vars, volumes, networks, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,NOW())
			ON CONFLICT (id) DO UPDATE SET
				name       = EXCLUDED.name,
				image      = EXCLUDED.image,
				image_tag  = EXCLUDED.image_tag,
				image_id   = EXCLUDED.image_id,
				state      = EXCLUDED.state,
				status     = EXCLUDED.status,
				created    = EXCLUDED.created,
				ports      = EXCLUDED.ports,
				labels     = EXCLUDED.labels,
				env_vars   = EXCLUDED.env_vars,
				volumes    = EXCLUDED.volumes,
				networks   = EXCLUDED.networks,
				updated_at = NOW()`,
			c.ID, hostID, c.ContainerID, c.Name, c.Image, c.ImageTag, c.ImageID, c.State, c.Status, c.Created, c.Ports,
			string(labelsJSON), string(envVarsJSON), string(volumesJSON), string(networksJSON),
		)
		if err != nil {
			return err
		}
		ids = append(ids, c.ID)
	}

	// Remove containers no longer reported by the agent
	if len(ids) > 0 {
		_, err := db.conn.Exec(
			`DELETE FROM docker_containers WHERE host_id = $1 AND NOT (id = ANY($2))`,
			hostID, pq.Array(ids),
		)
		return err
	}
	// Agent reported no containers — clear all for this host
	_, err := db.conn.Exec(`DELETE FROM docker_containers WHERE host_id = $1`, hostID)
	return err
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

// ========== Remote Commands ==========

// newUUID generates a UUID v4 using crypto/rand — no external dependency.
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (db *DB) CreateRemoteCommand(hostID, module, action, target, payload, triggeredBy string, auditLogID *int64) (*models.RemoteCommand, error) {
	if triggeredBy == "" {
		triggeredBy = "system"
	}
	if payload == "" {
		payload = "{}"
	}
	id := newUUID()
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	err := db.conn.QueryRow(
		`INSERT INTO remote_commands (id, host_id, module, action, target, payload, triggered_by, audit_log_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at`,
		id, hostID, module, action, target, payload, triggeredBy, auditLogID,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
		&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		cmd.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		cmd.EndedAt = &endedAt.Time
	}
	return &cmd, nil
}

func (db *DB) GetPendingRemoteCommands(hostID string) ([]models.PendingCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, module, action, target, payload
		 FROM remote_commands WHERE host_id = $1 AND status = 'pending' ORDER BY created_at ASC`, hostID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.PendingCommand
	for rows.Next() {
		var c models.PendingCommand
		if err := rows.Scan(&c.ID, &c.Module, &c.Action, &c.Target, &c.Payload); err != nil {
			continue
		}
		cmds = append(cmds, c)
	}
	return cmds, nil
}

func (db *DB) UpdateRemoteCommandStatus(id, status, output string) error {
	switch status {
	case "running":
		_, err := db.conn.Exec(
			`UPDATE remote_commands SET status = $1, started_at = NOW() WHERE id = $2`,
			status, id)
		return err
	default:
		_, err := db.conn.Exec(
			`UPDATE remote_commands SET status = $1, output = $2, ended_at = NOW() WHERE id = $3`,
			status, output, id)
		return err
	}
}

func (db *DB) GetRemoteCommandByID(id string) (*models.RemoteCommand, error) {
	var cmd models.RemoteCommand
	var startedAt, endedAt sql.NullTime
	err := db.conn.QueryRow(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE id = $1`, id,
	).Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
		&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	if startedAt.Valid {
		cmd.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		cmd.EndedAt = &endedAt.Time
	}
	return &cmd, nil
}

func (db *DB) GetRemoteCommandsByHostAndModule(hostID, module string, limit int) ([]models.RemoteCommand, error) {
	rows, err := db.conn.Query(
		`SELECT id, host_id, module, action, target, payload, status, output, triggered_by, audit_log_id, created_at, started_at, ended_at
		 FROM remote_commands WHERE host_id = $1 AND module = $2 ORDER BY created_at DESC LIMIT $3`,
		hostID, module, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cmds []models.RemoteCommand
	for rows.Next() {
		var cmd models.RemoteCommand
		var startedAt, endedAt sql.NullTime
		if err := rows.Scan(&cmd.ID, &cmd.HostID, &cmd.Module, &cmd.Action, &cmd.Target, &cmd.Payload,
			&cmd.Status, &cmd.Output, &cmd.TriggeredBy, &cmd.AuditLogID, &cmd.CreatedAt, &startedAt, &endedAt); err != nil {
			continue
		}
		if startedAt.Valid {
			cmd.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			cmd.EndedAt = &endedAt.Time
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

func (db *DB) GetRecentNotifications(limit int) ([]models.NotificationItem, error) {
	rows, err := db.conn.Query(
		`SELECT ai.id, ai.rule_id, ai.host_id,
		        COALESCE(h.name, ai.host_id) AS host_name,
		        COALESCE(ar.name, ar.metric || ' ' || ar.operator || ' ' || CAST(ar.threshold AS TEXT)) AS rule_name,
		        COALESCE(ar.metric, '') AS metric,
		        ai.value, ai.triggered_at, ai.resolved_at,
		        COALESCE(ar.channels @> '["browser"]'::jsonb, FALSE) AS browser_notify
		 FROM alert_incidents ai
		 LEFT JOIN alert_rules ar ON ai.rule_id = ar.id
		 LEFT JOIN hosts h ON ai.host_id = h.id
		 ORDER BY ai.triggered_at DESC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.NotificationItem
	for rows.Next() {
		var item models.NotificationItem
		if err := rows.Scan(
			&item.ID, &item.RuleID, &item.HostID,
			&item.HostName, &item.RuleName, &item.Metric,
			&item.Value, &item.TriggeredAt, &item.ResolvedAt,
			&item.BrowserNotify,
		); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// ========== Settings ==========

func (db *DB) GetAllSettings() (map[string]string, error) {
	rows, err := db.conn.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			result[k] = v
		}
	}
	return result, nil
}

func (db *DB) SetSetting(key, value string) error {
	_, err := db.conn.Exec(
		`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value,
	)
	return err
}

func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM settings WHERE key = $1`, key).Scan(&value)
	return value, err
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

// CleanupStalledCommands marks pending/running commands as failed if they're older than the timeout.
// Also closes any linked audit log entries to keep the audit trail consistent.
func (db *DB) CleanupStalledCommands(timeoutMinutes int) error {
	rows, err := db.conn.Query(`
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $1
		RETURNING audit_log_id`,
		timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup stalled commands: %w", err)
	}
	defer rows.Close()

	var auditIDs []int64
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		if err := rows.Scan(&auditLogID); err == nil && auditLogID.Valid {
			auditIDs = append(auditIDs, auditLogID.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		log.Printf("Cleaned up %d stalled remote commands", count)
		if len(auditIDs) > 0 {
			_, _ = db.conn.Exec(`
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs))
		}
	}
	return nil
}

// CleanupHostStalledCommands marks old pending/running commands for a specific host as failed.
// Also closes any linked audit log entries to keep the audit trail consistent.
func (db *DB) CleanupHostStalledCommands(hostID string, timeoutMinutes int) error {
	rows, err := db.conn.Query(`
		UPDATE remote_commands
		SET status = 'failed',
		    output = 'Command timed out - agent may have crashed or restarted',
		    ended_at = NOW()
		WHERE host_id = $1
		  AND status IN ('pending', 'running')
		  AND created_at < NOW() - INTERVAL '1 minute' * $2
		RETURNING audit_log_id`,
		hostID, timeoutMinutes)
	if err != nil {
		return fmt.Errorf("failed to cleanup host stalled commands: %w", err)
	}
	defer rows.Close()

	var auditIDs []int64
	count := 0
	for rows.Next() {
		count++
		var auditLogID sql.NullInt64
		if err := rows.Scan(&auditLogID); err == nil && auditLogID.Valid {
			auditIDs = append(auditIDs, auditLogID.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		log.Printf("Cleaned up %d stalled commands for host %s", count, hostID)
		if len(auditIDs) > 0 {
			_, _ = db.conn.Exec(`
				UPDATE audit_logs SET status = 'failed',
				    details = 'Command timed out - agent may have crashed or restarted'
				WHERE id = ANY($1)`,
				pq.Array(auditIDs))
		}
	}
	return nil
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
	actionsJSON, _ := json.Marshal(rule.Actions)
	return db.conn.QueryRow(
		`INSERT INTO alert_rules (host_id, metric, operator, threshold, duration_seconds, actions, enabled)
		 VALUES ($1,$2,$3,$4,$5,CAST($6 AS JSONB),$7)
		 RETURNING id`,
		rule.HostID, rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, string(actionsJSON), rule.Enabled,
	).Scan(&rule.ID)
}

func (db *DB) UpdateAlertRule(rule *models.AlertRule) error {
	actionsJSON, _ := json.Marshal(rule.Actions)
	_, err := db.conn.Exec(
		`UPDATE alert_rules SET
			host_id = $1,
			metric = $2,
			operator = $3,
			threshold = $4,
			duration_seconds = $5,
			actions = CAST($6 AS JSONB),
			enabled = $7,
			updated_at = NOW()
		 WHERE id = $8`,
		rule.HostID, rule.Metric, rule.Operator, rule.Threshold, rule.DurationSeconds, string(actionsJSON), rule.Enabled, rule.ID,
	)
	return err
}

func (db *DB) DeleteAlertRule(id int64) error {
	_, err := db.conn.Exec(`DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

func (db *DB) GetAlertRules() ([]models.AlertRule, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, host_id, metric, operator, threshold, duration_seconds,
		        actions, last_fired, enabled, created_at, updated_at
		 FROM alert_rules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.AlertRule
	for rows.Next() {
		var r models.AlertRule
		var name sql.NullString
		var hostID sql.NullString
		var threshold sql.NullFloat64
		var actionsJSON []byte
		var lastFired, updatedAt sql.NullTime

		if err := rows.Scan(
			&r.ID, &name, &hostID, &r.Metric, &r.Operator, &threshold, &r.DurationSeconds,
			&actionsJSON, &lastFired, &r.Enabled, &r.CreatedAt, &updatedAt,
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
		if lastFired.Valid {
			r.LastFired = &lastFired.Time
		}
		if updatedAt.Valid {
			r.UpdatedAt = &updatedAt.Time
		}
		if len(actionsJSON) > 0 {
			json.Unmarshal(actionsJSON, &r.Actions)
		}
		if r.Actions.Channels == nil {
			r.Actions.Channels = []string{}
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

// GetAptHistoryWithAgentUpdates returns APT remote commands for a host.
// Delegates to GetRemoteCommandsByHostAndModule for the "apt" module.
func (db *DB) GetAptHistoryWithAgentUpdates(hostID string, limit int) ([]models.RemoteCommand, error) {
	return db.GetRemoteCommandsByHostAndModule(hostID, "apt", limit)
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
		`SELECT id, root_label, root_ip, excluded_ports, service_map, host_overrides, manual_services,
		        COALESCE(authelia_label, 'Authelia'), COALESCE(authelia_ip, ''),
		        COALESCE(internet_label, 'Internet'), COALESCE(internet_ip, ''),
		        updated_at
		 FROM network_topology_config LIMIT 1`,
	).Scan(&cfg.ID, &cfg.RootLabel, &cfg.RootIP, &excludedPortsJSON, &cfg.ServiceMap,
		&cfg.HostOverrides, &cfg.ManualServices,
		&cfg.AutheliaLabel, &cfg.AutheliaIP, &cfg.InternetLabel, &cfg.InternetIP,
		&cfg.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default config if none exists
			return &models.NetworkTopologyConfig{
				RootLabel:      "Infrastructure",
				ExcludedPorts:  []int{},
				ServiceMap:     "{}",
				HostOverrides:  "{}",
				ManualServices: "[]",
				AutheliaLabel:  "Authelia",
				InternetLabel:  "Internet",
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
		`INSERT INTO network_topology_config (id, root_label, root_ip, excluded_ports, service_map, host_overrides, manual_services,
		        authelia_label, authelia_ip, internet_label, internet_ip, updated_at)
		 VALUES (1, $1, $2, $3::jsonb, $4, $5, $6, $7, $8, $9, $10, NOW())
		 ON CONFLICT(id) DO UPDATE SET
		   root_label = EXCLUDED.root_label,
		   root_ip = EXCLUDED.root_ip,
		   excluded_ports = EXCLUDED.excluded_ports,
		   service_map = EXCLUDED.service_map,
		   host_overrides = EXCLUDED.host_overrides,
		   manual_services = EXCLUDED.manual_services,
		   authelia_label = EXCLUDED.authelia_label,
		   authelia_ip = EXCLUDED.authelia_ip,
		   internet_label = EXCLUDED.internet_label,
		   internet_ip = EXCLUDED.internet_ip,
		   updated_at = NOW()`,
		cfg.RootLabel, cfg.RootIP, excludedPortsJSON,
		cfg.ServiceMap, cfg.HostOverrides, cfg.ManualServices,
		cfg.AutheliaLabel, cfg.AutheliaIP, cfg.InternetLabel, cfg.InternetIP,
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
