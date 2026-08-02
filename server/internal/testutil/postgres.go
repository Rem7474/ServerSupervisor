// Package testutil exposes shared helpers for integration tests. The headline
// helper is NewPostgresDB which hands each test a fresh, fully migrated
// Postgres database ready to use.
//
// A single Postgres container is started once per test binary (i.e. once per
// Go package, since `go test ./...` compiles and runs each package as its own
// process) and reused by every NewPostgresDB call in that process — each test
// still gets its own isolated database on that shared container, just without
// paying container-create + "wait for ready" (~10-13s) on every single test.
//
// Migrations themselves are not replayed per test either: the first call in a
// process migrates one template database (ss_test_template0) and every
// NewPostgresDB call after that clones it with `CREATE DATABASE ... TEMPLATE`,
// a filesystem-level copy that takes milliseconds instead of replaying ~80
// migration files. Each per-test clone is dropped in t.Cleanup — see
// ensureSharedContainer for why that used to be unsafe and isn't anymore.
//
// When Docker is not available locally (developer machine without Docker, CI
// without DinD…) the tests skip cleanly so the rest of the suite keeps
// running. Set TESTCONTAINERS_HOST_OVERRIDE or DOCKER_HOST in the environment
// to point at a remote Docker daemon if needed.
package testutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// TimescaleDB is a hard requirement (time_bucket, hypertables, retention policies).
	pgImage    = "timescale/timescaledb:2.27.2-pg16"
	pgUser     = "supervisor"
	pgPassword = "supervisor-test"
	pgDatabase = "serversupervisor_test"
	// pgTemplate holds one fully-migrated database, built once per process by
	// ensureSharedContainer. Every NewPostgresDB call clones it instead of
	// running migrations again.
	pgTemplate = "ss_test_template0"
)

// Shared container state, lazily initialized once per test binary via
// containerOnce and reused by every subsequent NewPostgresDB call in the same
// process. Never explicitly terminated: the container is ephemeral and reaped
// by testcontainers' Ryuk sidecar when the test process exits, same lifetime
// as before — only now shared across every test in the package instead of
// recreated per test.
var (
	containerOnce sync.Once
	containerErr  error
	sharedHost    string
	sharedPort    string
	adminDB       *sql.DB
	dbCounter     atomic.Int64
)

// ensureSharedContainer starts the shared Postgres container on the first
// call in this process and returns its connection coordinates on every call.
func ensureSharedContainer(ctx context.Context) (host, port string, err error) {
	containerOnce.Do(func() {
		startCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		pg, runErr := postgres.Run(startCtx, pgImage,
			postgres.WithDatabase(pgDatabase),
			postgres.WithUsername(pgUser),
			postgres.WithPassword(pgPassword),
			// Test-only container: durability doesn't matter. TimescaleDB's
			// background worker launcher is disabled outright (rather than
			// just raised) — see buildTemplateDatabase for why any nonzero
			// value here is a bug waiting to happen at this database count.
			testcontainers.WithCmd("postgres",
				"-c", "fsync=off",
				"-c", "full_page_writes=off",
				"-c", "timescaledb.max_background_workers=0",
			),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
			),
		)
		if runErr != nil {
			containerErr = runErr
			return
		}

		h, hErr := pg.Host(startCtx)
		if hErr != nil {
			containerErr = hErr
			return
		}
		p, pErr := pg.MappedPort(startCtx, "5432/tcp")
		if pErr != nil {
			containerErr = pErr
			return
		}

		conn, openErr := sql.Open("postgres", fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			h, p.Port(), pgUser, pgPassword, pgDatabase,
		))
		if openErr != nil {
			containerErr = openErr
			return
		}
		if pingErr := conn.Ping(); pingErr != nil {
			containerErr = pingErr
			return
		}

		if err := buildTemplateDatabase(conn, h, p.Port()); err != nil {
			containerErr = err
			return
		}

		sharedHost, sharedPort, adminDB = h, p.Port(), conn
	})
	return sharedHost, sharedPort, containerErr
}

// buildTemplateDatabase creates pgTemplate on the given admin connection, runs
// every migration into it via database.New, and marks it a template so every
// NewPostgresDB call can clone it instead of migrating from scratch.
//
// This only works because timescaledb.max_background_workers=0 on the
// container (see ensureSharedContainer). A fully migrated database registers
// ~15 TimescaleDB jobs (compression + retention policies on 6 hypertables
// from migration 064, plus a continuous-aggregate refresh policy on each of
// the 4 views ensureTimescaleObjects creates) — confirmed against a throwaway
// container that the instant the *first* job is registered on a database,
// TimescaleDB's launcher permanently attaches a "Background Worker Scheduler"
// backend to it (idle, but connected for that database's lifetime — removing
// the job afterwards does not disconnect it). With background workers
// disabled that scheduler never launches for any database, which fixes two
// things at once: `CREATE DATABASE ... TEMPLATE pgTemplate` no longer fails
// with "source database is being accessed by other users" (55006, since the
// scheduler counts as a connection to the template), and per-test databases
// can safely be DROP DATABASE'd in t.Cleanup instead of leaking forever (the
// previous version of this file never dropped them for exactly that reason —
// see git history). No test exercises these jobs actually firing —
// RefreshContinuousAggregate below exists precisely because tests refresh
// continuous aggregates manually — so losing the automatic schedule costs
// nothing.
func buildTemplateDatabase(conn *sql.DB, host, port string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := conn.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", pq.QuoteIdentifier(pgTemplate))); err != nil {
		return fmt.Errorf("failed to create template database: %w", err)
	}

	tmplDB, err := database.New(&config.Config{
		DBHost: host, DBPort: port, DBUser: pgUser, DBPassword: pgPassword,
		DBName: pgTemplate, DBSSLMode: "disable",
	})
	if err != nil {
		return fmt.Errorf("failed to migrate template database: %w", err)
	}
	if err := tmplDB.Close(); err != nil {
		return fmt.Errorf("failed to close template database connection: %w", err)
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(
		"ALTER DATABASE %s WITH is_template = true", pq.QuoteIdentifier(pgTemplate))); err != nil {
		return fmt.Errorf("failed to mark template database: %w", err)
	}
	// is_template alone doesn't block connections (only DROP/rename); belt and
	// braces against anything ever accidentally connecting to it. PG16 has no
	// ALTER DATABASE ... WITH ALLOW_CONNECTIONS (added in PG17), hence the
	// direct catalog update.
	if _, err := conn.ExecContext(ctx,
		"UPDATE pg_database SET datallowconn = false WHERE datname = $1", pgTemplate); err != nil {
		return fmt.Errorf("failed to lock down template database: %w", err)
	}

	return nil
}

// NewPostgresDB returns a *database.DB backed by a fresh, isolated database,
// cloned from the already-migrated pgTemplate on the shared per-process
// Postgres container.
//
// If the Docker daemon is unreachable the test is skipped (rather than
// failed) so developers without Docker on their local machine still get a
// green build for the rest of the suite.
func NewPostgresDB(t *testing.T) *database.DB {
	t.Helper()

	if os.Getenv("SS_SKIP_INTEGRATION") != "" {
		t.Skip("SS_SKIP_INTEGRATION is set — skipping integration test")
	}

	host, port, err := ensureSharedContainer(context.Background())
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("Docker not available — skipping integration test: %v", err)
		}
		t.Fatalf("failed to start shared postgres container: %v", err)
	}

	dbName := fmt.Sprintf("ss_test_%d", dbCounter.Add(1))
	if _, err := adminDB.Exec(fmt.Sprintf(
		"CREATE DATABASE %s TEMPLATE %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(pgTemplate))); err != nil {
		t.Fatalf("failed to create test database %s: %v", dbName, err)
	}

	cfg := &config.Config{
		DBHost:                 host,
		DBPort:                 port,
		DBUser:                 pgUser,
		DBPassword:             pgPassword,
		DBName:                 dbName,
		DBSSLMode:              "disable",
		JWTSecret:              "test-jwt-secret-with-enough-length-1234",
		JWTExpiration:          24 * time.Hour,
		RefreshTokenExpiration: 7 * 24 * time.Hour,
		APIKeyHeader:           "X-API-Key",
		MetricsRetentionDays:   30,
		AuditRetentionDays:     90,
		WebLogsRetentionDays:   30,
	}

	// Open, not New: dbName was cloned from pgTemplate, which is already fully
	// migrated. Calling New here would re-run ensureTimescaleObjects, which
	// would re-register the continuous-aggregate refresh policies
	// buildTemplateDatabase deliberately stripped — right back to the
	// per-database background worker that caused the original crash.
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		// Safe now: the clone carries no live TimescaleDB background job (see
		// buildTemplateDatabase), so nothing keeps a connection open against
		// it once db.Close() above returns.
		if _, err := adminDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", pq.QuoteIdentifier(dbName))); err != nil {
			t.Logf("failed to drop test database %s: %v", dbName, err)
		}
	})

	return db
}

// NewPostgresDBWithConfig is identical to NewPostgresDB but also returns the
// resolved *config.Config so the caller can pass it to handlers / middlewares
// that take a config.
func NewPostgresDBWithConfig(t *testing.T) (*database.DB, *config.Config) {
	t.Helper()
	db := NewPostgresDB(t)
	// Pull the same secret + DSN bits back out — easier than threading them
	// through return values everywhere.
	cfg := &config.Config{
		JWTSecret:              "test-jwt-secret-with-enough-length-1234",
		JWTExpiration:          24 * time.Hour,
		RefreshTokenExpiration: 7 * 24 * time.Hour,
		APIKeyHeader:           "X-API-Key",
		AdminUser:              "admin",
		AdminPassword:          "admin",
		BaseURL:                "http://localhost",
	}
	return db, cfg
}

// isDockerUnavailable inspects the testcontainers error to decide whether the
// failure is "no Docker daemon" (skip) vs an actual container problem (fail).
// We err on the side of skipping: any error that mentions Docker provider /
// daemon / runtime missing trips the skip path so developers without Docker
// are not blocked by integration-test failures.
func isDockerUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, hint := range []string{
		"Cannot connect to the Docker daemon",
		"docker daemon",
		"failed to create Docker provider",
		"rootless Docker is not supported",
		"rootless Docker not found",
		"executable file not found",
		"docker: command not found",
		"open //./pipe/docker_engine",
		"open /var/run/docker.sock",
		"could not connect",
		"connect: connection refused",
		"no such host",
		"dockerd",
	} {
		if contains(msg, hint) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// MustQuery is a tiny convenience to fail fast on setup queries inside tests.
func MustQuery(t *testing.T, db *database.DB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("setup query failed: %v\nquery: %s", err, query)
	}
}

// pqLockNotAvailable is the SQLSTATE Postgres/TimescaleDB raises when a
// continuous aggregate refresh is already in progress.
const pqLockNotAvailable = "55P03"

// RefreshContinuousAggregate refreshes a TimescaleDB continuous aggregate,
// retrying on SQLSTATE 55P03 (lock_not_available). ensureTimescaleObjects
// registers an automatic refresh policy for every CAGG right after creating
// it, and TimescaleDB's background job scheduler can fire that policy's
// first run within moments of registration — independently of, and racing,
// this manual call. TimescaleDB doesn't queue concurrent refreshes on the
// same CAGG, it fails one of them immediately, so a short client-side retry
// is the correct fix here: the race is a property of a freshly-provisioned
// test database, not a bug in the app being tested.
func RefreshContinuousAggregate(t *testing.T, db *database.DB, name string) {
	t.Helper()
	const maxAttempts = 5
	query := fmt.Sprintf(`CALL refresh_continuous_aggregate('%s', NULL, NULL)`, name)
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, err := db.Exec(context.Background(), query)
		if err == nil {
			return
		}
		lastErr = err
		var pqErr *pq.Error
		if !errors.As(err, &pqErr) || string(pqErr.Code) != pqLockNotAvailable {
			t.Fatalf("refresh continuous aggregate %s failed: %v", name, err)
		}
		time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
	}
	t.Fatalf("refresh continuous aggregate %s: still failing with concurrent-refresh (%s) after %d attempts: %v",
		name, pqLockNotAvailable, maxAttempts, lastErr)
}
