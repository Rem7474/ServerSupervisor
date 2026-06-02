package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/serversupervisor/server/internal/api"
	"github.com/serversupervisor/server/internal/background"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/logging"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/ws"
)

func main() {
	// Root ctx — cancelled by SIGINT/SIGTERM. Propagated to background jobs,
	// scheduler, pollers, and any DB call made outside an HTTP request context.
	rootCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()

	// Load config (must precede logging.Init so LOG_LEVEL/LOG_FORMAT apply).
	cfg := config.Load()

	// Structured logging — also bridges the standard log package through slog.
	logging.Init(cfg.LogLevel, cfg.LogFormat)
	slog.Info("ServerSupervisor starting", slog.String("log_level", cfg.LogLevel), slog.String("log_format", cfg.LogFormat))
	log.Printf("Database Config: host=%s port=%s dbname=%s", cfg.DBHost, cfg.DBPort, cfg.DBName)

	// ⚠️  Validate configuration — log all warnings before connecting to the database
	for _, w := range cfg.Validate() {
		log.Printf("⚠️  WARNING: %s", w)
	}

	// In production (APP_ENV != "dev"/"development"), refuse to start when
	// insecure defaults are present (JWT_SECRET, ADMIN_PASSWORD, DB_PASSWORD).
	if err := cfg.ValidateStrict(); err != nil {
		log.Fatalf("Refusing to start: %v. Set APP_ENV=dev to bypass for local development.", err)
	}
	if config.IsDevEnv() {
		log.Printf("[dev] APP_ENV=%s — strict secret validation disabled. Do NOT use this mode in production.", config.AppEnv())
	}

	// Ensure database exists
	if err := database.EnsureDatabaseExists(cfg); err != nil {
		log.Printf("Warning: could not ensure database exists: %v (will retry on connection)", err)
	}

	// Connect to database
	db, err := database.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Cleanup stalled commands at startup (commands older than 10 minutes)
	if err := db.CleanupStalledCommands(rootCtx, 10); err != nil {
		log.Printf("Warning: failed to cleanup stalled commands: %v", err)
	}

	// Create default admin user (sets must_change_password if using default "admin" password)
	hash, err := handlers.HashPassword(cfg.AdminPassword)
	if err != nil {
		log.Fatalf("Failed to hash admin password: %v", err)
	}
	mustChangePassword := cfg.AdminPassword == "admin"
	if err := db.CreateUser(rootCtx, cfg.AdminUser, hash, "admin", mustChangePassword); err != nil {
		log.Printf("Admin user creation: %v (may already exist)", err)
	}
	// For existing installations still using default password, ensure flag is set
	if mustChangePassword {
		if err := db.SetUserMustChangePassword(rootCtx, cfg.AdminUser, true); err != nil {
			log.Printf("Warning: failed to set must_change_password for admin: %v", err)
		}
	}

	dispatcher := dispatch.New(db)

	// Start task scheduler
	sched := scheduler.New(db, dispatcher)
	sched.Start(rootCtx)
	defer sched.Stop()

	// Notification hub — shared between alert engine (push on fire) and WS handler
	notifHub := ws.NewNotificationHub()

	// Start background jobs (each runs in its own goroutine with panic recovery)
	bg := background.New()
	bg.Add(background.NewAuditCleanupJob(db, cfg))
	bg.Add(background.NewHostStatusJob(db))
	bg.Add(background.NewAlertEvalJob(db, cfg, dispatcher, notifHub))
	// Metric downsampling is handled by the TimescaleDB continuous aggregate
	// (system_metrics_5min); metric retention/compression by Timescale policies.
	// The remaining job only trims release-tracker tag digests.
	bg.Add(background.NewMetricsRetentionJob(db, cfg))
	bg.Add(background.NewWebLogsRetentionJob(db, cfg))
	bg.Add(background.NewUptimeWorkerJob(db))
	bg.Add(background.NewSSLWorkerJob(db))
	bg.Start(rootCtx)
	defer bg.Stop()

	// Setup router
	router, releaseTrackerH, proxmoxH, cleanupRouter := api.SetupRouter(db, cfg, notifHub, sched, dispatcher)
	defer cleanupRouter()
	releaseTrackerH.StartPoller(rootCtx)
	defer releaseTrackerH.StopPoller()
	proxmoxH.StartPoller(rootCtx)
	defer proxmoxH.StopPoller()

	// Start server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // Disabled for WebSocket streaming
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown — wait for SIGINT/SIGTERM (already wired via signal.NotifyContext).
	<-rootCtx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}
