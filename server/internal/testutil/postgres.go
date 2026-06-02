// Package testutil exposes shared helpers for integration tests. The headline
// helper is NewPostgresDB which spins up a real Postgres in a Docker
// container via testcontainers-go, runs the embedded migrations on it, and
// returns a *database.DB ready to use.
//
// When Docker is not available locally (developer machine without Docker, CI
// without DinD…) the tests skip cleanly so the rest of the suite keeps
// running. Set TESTCONTAINERS_HOST_OVERRIDE or DOCKER_HOST in the environment
// to point at a remote Docker daemon if needed.
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

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
)

// NewPostgresDB returns a *database.DB backed by a fresh Postgres container
// with all migrations applied. The container is destroyed via t.Cleanup so
// each test gets a clean state.
//
// If the Docker daemon is unreachable the test is skipped (rather than
// failed) so developers without Docker on their local machine still get a
// green build for the rest of the suite.
func NewPostgresDB(t *testing.T) *database.DB {
	t.Helper()

	if os.Getenv("SS_SKIP_INTEGRATION") != "" {
		t.Skip("SS_SKIP_INTEGRATION is set — skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	pg, err := postgres.Run(ctx, pgImage,
		postgres.WithDatabase(pgDatabase),
		postgres.WithUsername(pgUser),
		postgres.WithPassword(pgPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		if isDockerUnavailable(err) {
			t.Skipf("Docker not available — skipping integration test: %v", err)
		}
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		// Best-effort termination; if the container is already gone the
		// returned error is harmless.
		termCtx, termCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer termCancel()
		_ = pg.Terminate(termCtx)
	})

	host, err := pg.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("failed to get mapped port: %v", err)
	}

	cfg := &config.Config{
		DBHost:               host,
		DBPort:               port.Port(),
		DBUser:               pgUser,
		DBPassword:           pgPassword,
		DBName:               pgDatabase,
		DBSSLMode:            "disable",
		JWTSecret:            "test-jwt-secret-with-enough-length-1234",
		JWTExpiration:        24 * time.Hour,
		RefreshTokenExpiration: 7 * 24 * time.Hour,
		APIKeyHeader:         "X-API-Key",
		MetricsRetentionDays: 30,
		AuditRetentionDays:   90,
		WebLogsRetentionDays: 30,
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

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

// Unused returns a value to silence the unused-import linter when the file
// is included in a build but the tests inside are gated by Docker.
var _ = fmt.Sprintf
