package database

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/config"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// HashAPIKey returns the SHA-256 hash of an API key.
func HashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

// EnsureDatabaseExists creates the database if it doesn't exist.
func EnsureDatabaseExists(cfg *config.Config) error {
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

// DB wraps the underlying sql.DB connection and exposes domain-specific methods
// split across db_*.go files in this package.
type DB struct {
	conn *sql.DB
}

// New opens a connection to the database, runs migrations, and returns a DB.
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

func (db *DB) Close() error { return db.conn.Close() }
func (db *DB) Ping() error  { return db.conn.Ping() }

// Query executes a query that returns rows.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRow(query, args...)
}

// Exec executes a query without returning any rows.
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.conn.Exec(query, args...)
}

// migrate runs all embedded SQL migration files in alphabetical order.
// Each file is split on ";" — statements are trimmed and empty ones skipped.
// All CREATE / ALTER / INSERT statements use IF NOT EXISTS / IF EXISTS, so
// the runner is idempotent against both fresh and existing databases.
func (db *DB) migrate() error {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", e.Name(), err)
		}
		for _, stmt := range strings.Split(string(data), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.conn.Exec(stmt); err != nil {
				n := len(stmt)
				if n > 80 {
					n = 80
				}
				return fmt.Errorf("migration %s failed at [%s]: %w", e.Name(), stmt[:n], err)
			}
		}
	}

	log.Println("Database migrations completed")
	return nil
}
