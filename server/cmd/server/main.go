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
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/logging"
	"github.com/serversupervisor/server/internal/poller"
	"github.com/serversupervisor/server/internal/scheduler"
	backupsvc "github.com/serversupervisor/server/internal/services/backup"
	pushsvc "github.com/serversupervisor/server/internal/services/push"
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

	// Apply DB-persisted settings on top of env vars so admin changes made via the
	// settings UI take effect immediately at boot (previously they were only loaded
	// lazily when the settings page was opened). All downstream consumers (pollers,
	// alert engine, notify, retention jobs) read the resulting config.
	cfg.OverrideFromDB(db)

	// If JWT_SECRET was not provided in env or DB, generate a strong random secret
	// and persist it in the settings table so it remains stable across restarts.
	if cfg.JWTSecret == "" {
		generatedSecret := config.GenerateRandomSecret()
		if err := db.SetSetting(rootCtx, "jwt_secret", generatedSecret); err != nil {
			log.Printf("Warning: failed to persist generated JWT secret to database: %v", err)
		}
		cfg.JWTSecret = generatedSecret
		slog.Info("JWT secret auto-generated and persisted in settings")
	}

	// Reconcile the TimescaleDB metric retention policy with the effective config
	// so a retention value set via env or the settings table is honoured without a
	// manual "cleanup metrics" click. Non-fatal: the migration baseline already
	// installs a default policy.
	if err := db.UpdateMetricsRetentionPolicy(rootCtx, cfg.MetricsRetentionDays); err != nil {
		slog.WarnContext(rootCtx, "failed to reconcile metrics retention policy at startup", slog.Any("err", err), slog.Int("days", cfg.MetricsRetentionDays))
	}

	// Cleanup stalled commands at startup (commands older than 10 minutes)
	if err := db.CleanupStalledCommands(rootCtx, 10); err != nil {
		log.Printf("Warning: failed to cleanup stalled commands: %v", err)
	}

	// First run / Account bootstrap:
	// If no admin user exists in the database, initialize the admin account.
	hasAdmin, err := db.HasAdminUser(rootCtx)
	if err != nil {
		log.Fatalf("Failed to check existing admin user: %v", err)
	}

	if !hasAdmin {
		adminUser := cfg.AdminUser
		if adminUser == "" {
			adminUser = "admin"
		}

		adminPassword := cfg.AdminPassword
		mustChangePassword := false
		isGenerated := false

		if adminPassword == "" {
			adminPassword = config.GenerateRandomPassword(16)
			mustChangePassword = true
			isGenerated = true
		}

		hash, err := handlers.HashPassword(adminPassword)
		if err != nil {
			log.Fatalf("Failed to hash admin password: %v", err)
		}

		if err := db.CreateUser(rootCtx, adminUser, hash, "admin", mustChangePassword); err != nil {
			log.Fatalf("Failed to create initial admin user: %v", err)
		}

		if isGenerated {
			log.Printf("\n" +
				"========================================================================\n" +
				"🚀 SERVERSUPERVISOR — COMPTE ADMINISTRATEUR INITIALISÉ\n" +
				"========================================================================\n" +
				"  Identifiant  : %s\n" +
				"  Mot de passe : %s\n\n" +
				"  ⚠️  IMPORTANT : Conservez ce mot de passe temporaire !\n" +
				"  ⚠️  Vous serez invité à le modifier dès votre première connexion.\n" +
				"========================================================================\n",
				adminUser, adminPassword)
			slog.Info("First run: initial admin account created with generated password", slog.String("username", adminUser))
		} else {
			slog.Info("First run: initial admin account created with configured password", slog.String("username", adminUser))
		}
	} else {
		slog.Info("Admin account already initialized")
	}

	dispatcher := dispatch.New(db)

	// Start task scheduler
	sched := scheduler.New(db, dispatcher)
	sched.Start(rootCtx)
	defer sched.Stop()

	// Notification hub — shared between alert engine (push on fire) and WS handler
	notifHub := ws.NewNotificationHub()
	pushSvc := pushsvc.NewService(db)

	// Event bus — writers publish topics; WS snapshot endpoints subscribe and push
	// on change instead of polling the DB on a fixed timer.
	eventBus := events.NewBus()

	// Start background jobs (each runs in its own goroutine with panic recovery)
	bg := background.New()
	bg.Add(background.NewAuditCleanupJob(db, cfg))
	if cfg.DemoMode {
		// Demo hosts never send real heartbeats, so this job (offline after a
		// 2-minute-stale last_seen, every 30s) would flip every seeded "online"
		// host offline within minutes — fighting the seed's fixed fleet state
		// instead of real network isolation, so it's gated here rather than in
		// the network-call group above.
		slog.Info("demo mode: skipping host-status job (seeded hosts have no real heartbeat)")
	} else {
		bg.Add(background.NewHostStatusJob(db, eventBus))
	}
	bg.Add(background.NewAlertEvalJob(db, cfg, dispatcher, notifHub, pushSvc))
	// Metric downsampling is handled by the TimescaleDB continuous aggregate
	// (system_metrics_5min); metric retention/compression by Timescale policies.
	// The remaining job only trims release-tracker tag digests.
	bg.Add(background.NewMetricsRetentionJob(db, cfg))
	bg.Add(background.NewWebLogsRetentionJob(db, cfg))
	bg.Add(background.NewNetworkFlowsRetentionJob(db, cfg))
	if cfg.DemoMode {
		slog.Info("demo mode: skipping uptime/SSL probe workers (no outbound network calls)")
	} else {
		bg.Add(background.NewUptimeWorkerJob(db))
		bg.Add(background.NewSSLWorkerJob(db))
	}
	// Separate Service instance from the one wired into the router/completion
	// listener — CheckStalledRuns only needs repo+notify, no HTTP-facing state.
	backupStallSvc := backupsvc.NewService(db, dispatcher, cfg, notifHub, pushSvc)
	bg.Add(background.NewBackupStallJob(backupStallSvc, 360))
	bg.Start(rootCtx)
	defer bg.Stop()

	// Setup router
	router, releaseTrackerH, proxmoxH, npmH, cleanupRouter := api.SetupRouter(db, cfg, notifHub, eventBus, sched, dispatcher)
	defer cleanupRouter()
	// Background pollers: the handlers expose the unit of work + a fire-and-forget
	// ctx; the poller package owns the scheduling loop. rootCtx cancellation
	// (SIGINT/SIGTERM) stops both loops, so no explicit Stop is needed.
	if cfg.DemoMode {
		slog.Info("demo mode: skipping release-tracker/docker-image-versions/proxmox/npm pollers (no outbound network calls)")
	} else {
		releaseTrackerH.SetBackgroundContext(rootCtx)
		poller.Every(rootCtx, releaseTrackerH.PollInterval(), true, "release-tracker", releaseTrackerH.CheckAll)
		// Ambient Docker image-version engine: one registry check per distinct
		// image:tag running anywhere, so every container gets an up-to-date/outdated
		// badge — not just the ones with a release tracker. Much slower cadence than
		// the tracker poller above (it scans the whole fleet, and registries rate
		// limit per source IP); the tracker poller reads its cache instead of
		// calling a registry itself.
		poller.Every(rootCtx, releaseTrackerH.DockerImagePollInterval(), true, "docker-image-versions", releaseTrackerH.RefreshDockerImageVersions)
		proxmoxH.SetBackgroundContext(rootCtx)
		poller.Every(rootCtx, handlers.ProxmoxPollInterval, true, "proxmox", proxmoxH.PollOnce)
		npmH.SetBackgroundContext(rootCtx)
		poller.Every(rootCtx, handlers.NPMPollInterval, false, "npm-sync", npmH.PollOnce)
	}

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
