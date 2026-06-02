package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/config"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// HashAPIKey returns a bcrypt hash of an API key secret.
func HashAPIKey(apiKey string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// EnsureDatabaseExists creates the database if it doesn't exist.
func EnsureDatabaseExists(cfg *config.Config) error {
	tempDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBSSLMode)

	tempConn, err := sql.Open("postgres", tempDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() { _ = tempConn.Close() }()

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

	slog.Info("Database is ready", slog.String("db", cfg.DBName))
	return nil
}

// DB wraps the underlying sql.DB connection and exposes domain-specific methods
// split across db_*.go files in this package.
type DB struct {
	conn           *sql.DB
	hasTimescaleDB bool
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

	// Detect TimescaleDB availability once so callers never attempt time_bucket()
	// on a plain PostgreSQL instance (avoids ERROR noise in the DB server log).
	var hasTS bool
	_ = conn.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'timescaledb')`).Scan(&hasTS)
	db.hasTimescaleDB = hasTS
	if hasTS {
		slog.Info("TimescaleDB extension detected - using time_bucket() for metric bucketing")
		// Continuous aggregates cannot be created inside a transaction/DO block,
		// so they live here (run as standalone statements) rather than in a SQL
		// migration. Idempotent and gated on Timescale availability; a failure is
		// logged but never blocks startup (GetMetricsSummary falls back to raw).
		if err := db.ensureTimescaleObjects(context.Background()); err != nil {
			slog.Error("failed to ensure TimescaleDB continuous aggregates", slog.Any("err", err))
		}
	} else {
		slog.Info("TimescaleDB not found - using plain PostgreSQL bucketing")
	}

	return db, nil
}

// ensureTimescaleObjects creates the continuous aggregate (and its refresh
// policy) that powers the dashboard metrics summary. It requires system_metrics
// to already be a hypertable (done by migration 064 / the V2 baseline). Each
// statement is idempotent so this is safe to run on every startup.
func (db *DB) ensureTimescaleObjects(ctx context.Context) error {
	if _, err := db.conn.ExecContext(ctx,
		`CREATE MATERIALIZED VIEW IF NOT EXISTS system_metrics_5min
		 WITH (timescaledb.continuous) AS
		 SELECT time_bucket(INTERVAL '5 minutes', timestamp) AS bucket,
		        host_id,
		        AVG(cpu_usage_percent) AS cpu_avg,
		        AVG(memory_percent)    AS mem_avg,
		        COUNT(*)               AS sample_count
		 FROM system_metrics
		 GROUP BY bucket, host_id
		 WITH NO DATA`); err != nil {
		return fmt.Errorf("create system_metrics_5min: %w", err)
	}

	if _, err := db.conn.ExecContext(ctx,
		`SELECT add_continuous_aggregate_policy('system_metrics_5min',
		    start_offset      => INTERVAL '30 days',
		    end_offset        => INTERVAL '5 minutes',
		    schedule_interval => INTERVAL '5 minutes',
		    if_not_exists     => true)`); err != nil {
		return fmt.Errorf("add continuous aggregate policy: %w", err)
	}

	return nil
}

func (db *DB) Close() error { return db.conn.Close() }
func (db *DB) Ping() error  { return db.conn.Ping() }

// Query executes a query that returns rows. The provided context controls
// cancellation and per-request timeouts: when ctx is cancelled the in-flight
// statement is aborted and Postgres returns an error rather than blocking.
func (db *DB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return db.conn.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
// The provided context propagates cancellation to the database driver.
func (db *DB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return db.conn.QueryRowContext(ctx, query, args...)
}

// Exec executes a query without returning any rows. The provided context
// propagates cancellation to the database driver.
func (db *DB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.conn.ExecContext(ctx, query, args...)
}

// migrate runs all embedded SQL migration files in alphabetical order.
// Applied migrations are tracked in the schema_migrations table so each file
// runs exactly once, even across server restarts.
func (db *DB) migrate() error {
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
filename   TEXT PRIMARY KEY,
applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	rows, err := db.conn.Query(`SELECT filename FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	applied := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[name] = struct{}{}
	}
	rows.Close()

	if len(applied) == 0 {
		var exists bool
		_ = db.conn.QueryRow(`SELECT EXISTS (
SELECT 1 FROM information_schema.tables
WHERE table_schema = 'public' AND table_name = 'hosts'
)`).Scan(&exists)
		if exists {
			entries, err := fs.ReadDir(migrationFS, "migrations")
			if err != nil {
				return fmt.Errorf("failed to read migrations dir: %w", err)
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".sql") {
					continue
				}
				if _, err := db.conn.Exec(`INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`, e.Name()); err != nil {
					return fmt.Errorf("backfill schema_migrations %s: %w", e.Name(), err)
				}
				applied[e.Name()] = struct{}{}
			}
			slog.Info("schema_migrations bootstrapped", slog.Int("existing_migrations", len(applied)))
		}
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if _, ok := applied[e.Name()]; ok {
			continue
		}

		if e.Name() == "000_full_baseline_breaking.sql" {
			var hostsExists bool
			if err := db.conn.QueryRowContext(context.Background(), `SELECT EXISTS (
SELECT 1 FROM information_schema.tables
WHERE table_schema = 'public' AND table_name = 'hosts'
)`).Scan(&hostsExists); err != nil {
				return fmt.Errorf("check existing schema before baseline migration: %w", err)
			}
			if hostsExists {
				if _, err := db.conn.ExecContext(context.Background(), `INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING`, e.Name()); err != nil {
					return fmt.Errorf("record migration %s: %w", e.Name(), err)
				}
				slog.Info("Migration skipped on existing schema", slog.String("file", e.Name()))
				continue
			}

			// Fresh install path: the baseline declares the legacy migrations it
			// subsumes via "-- ===== BEGIN <filename>.sql =====" markers. Only
			// those get pre-marked as applied — anything added AFTER the baseline
			// was generated must still execute, otherwise schema changes shipped
			// in later migrations would silently never run on a fresh DB.
			baselineSubsumed, readErr := readBaselineManifest(migrationFS, e.Name())
			if readErr != nil {
				return fmt.Errorf("parse baseline manifest: %w", readErr)
			}
			for fn := range baselineSubsumed {
				applied[fn] = struct{}{}
			}
		}

		data, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", e.Name(), err)
		}
		for _, stmt := range splitSQLStatements(string(data)) {
			if _, err := db.conn.Exec(stmt); err != nil {
				n := len(stmt)
				if n > 80 {
					n = 80
				}
				return fmt.Errorf("migration %s failed at [%s]: %w", e.Name(), stmt[:n], err)
			}
		}
		if _, err := db.conn.Exec(`INSERT INTO schema_migrations (filename) VALUES ($1)`, e.Name()); err != nil {
			return fmt.Errorf("record migration %s: %w", e.Name(), err)
		}
		slog.Info("Migration applied", slog.String("file", e.Name()))
	}

	slog.Info("Database migrations completed")
	return nil
}

// splitSQLStatements splits a SQL file into individual statements on ";",
// ignoring semicolons that appear inside:
//   - dollar-quoted strings ($$...$$), used by PL/pgSQL DO blocks
//   - single-quoted string literals ('...')
//   - line comments ("-- ..." until end of line)
//   - block comments ("/* ... */")
//
// Comment-aware splitting matters because a single ';' inside a "--" comment
// would otherwise be treated as a statement terminator and break parsing.
func splitSQLStatements(sql string) []string {
	var statements []string
	var cur strings.Builder
	inDollarQuote := false
	inSingleQuote := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(sql); i++ {
		ch := sql[i]

		// Line comment: skip from "--" until newline. Comment characters are
		// kept in the output so the executed SQL remains identical, but the
		// ';' inside no longer triggers a split.
		if inLineComment {
			cur.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			cur.WriteByte(ch)
			if ch == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				cur.WriteByte('/')
				i++
				inBlockComment = false
			}
			continue
		}

		// Detect comment starts only outside of strings.
		if !inSingleQuote && !inDollarQuote {
			if ch == '-' && i+1 < len(sql) && sql[i+1] == '-' {
				cur.WriteByte('-')
				cur.WriteByte('-')
				i++
				inLineComment = true
				continue
			}
			if ch == '/' && i+1 < len(sql) && sql[i+1] == '*' {
				cur.WriteByte('/')
				cur.WriteByte('*')
				i++
				inBlockComment = true
				continue
			}
		}

		if !inDollarQuote && ch == '\'' {
			inSingleQuote = !inSingleQuote
			cur.WriteByte(ch)
			if !inSingleQuote && i+1 < len(sql) && sql[i+1] == '\'' {
				inSingleQuote = true
			}
			continue
		}
		if !inSingleQuote && ch == '$' && i+1 < len(sql) && sql[i+1] == '$' {
			inDollarQuote = !inDollarQuote
			cur.WriteByte(ch)
			cur.WriteByte(sql[i+1])
			i++
			continue
		}
		if ch == ';' && !inDollarQuote && !inSingleQuote {
			if stmt := strings.TrimSpace(cur.String()); stmt != "" {
				statements = append(statements, stmt)
			}
			cur.Reset()
			continue
		}
		cur.WriteByte(ch)
	}
	if stmt := strings.TrimSpace(cur.String()); stmt != "" {
		statements = append(statements, stmt)
	}
	return statements
}
