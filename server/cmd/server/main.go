package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serversupervisor/server/internal/api"
	"github.com/serversupervisor/server/internal/background"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/github"
	"github.com/serversupervisor/server/internal/handlers"
	"github.com/serversupervisor/server/internal/scheduler"
	"github.com/serversupervisor/server/internal/ws"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("ServerSupervisor - Starting server...")

	// Load config
	cfg := config.Load()
	log.Printf("Database Config: host=%s port=%s user=%s dbname=%s", cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBName)

	// ⚠️  Security warnings for default configuration
	if cfg.JWTSecret == config.DefaultJWTSecret {
		log.Println("⚠️  WARNING: JWT_SECRET is using the default insecure value. Change it in production!")
	}
	if cfg.AdminPassword == "admin" {
		log.Println("⚠️  WARNING: ADMIN_PASSWORD is 'admin'. Change it immediately!")
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
	if err := db.CleanupStalledCommands(10); err != nil {
		log.Printf("Warning: failed to cleanup stalled commands: %v", err)
	}

	// Create default admin user (sets must_change_password if using default "admin" password)
	hash, err := handlers.HashPassword(cfg.AdminPassword)
	if err != nil {
		log.Fatalf("Failed to hash admin password: %v", err)
	}
	mustChangePassword := cfg.AdminPassword == "admin"
	if err := db.CreateUser(cfg.AdminUser, hash, "admin", mustChangePassword); err != nil {
		log.Printf("Admin user creation: %v (may already exist)", err)
	}
	// For existing installations still using default password, ensure flag is set
	if mustChangePassword {
		if err := db.SetUserMustChangePassword(cfg.AdminUser, true); err != nil {
			log.Printf("Warning: failed to set must_change_password for admin: %v", err)
		}
	}

	dispatcher := dispatch.New(db)

	// Start task scheduler
	sched := scheduler.New(db, dispatcher)
	sched.Start()
	defer sched.Stop()

	// Start GitHub release tracker (TrackedRepo / Docker version compare)
	tracker := github.NewTracker(db, cfg)
	tracker.Start()
	defer tracker.Stop()

	// Notification hub — shared between alert engine (push on fire) and WS handler
	notifHub := ws.NewNotificationHub()

	// Start background jobs (each runs in its own goroutine with panic recovery)
	bg := background.New()
	bg.Add(background.NewAuditCleanupJob(db))
	bg.Add(background.NewHostStatusJob(db))
	bg.Add(background.NewAlertEvalJob(db, cfg, dispatcher, notifHub))
	bg.Add(background.NewMetricsDownsampleJob(db))
	bg.Start()
	defer bg.Stop()

	// Setup router
	router, releaseTrackerH, proxmoxH := api.SetupRouter(db, cfg, notifHub, sched, dispatcher)
	releaseTrackerH.StartPoller()
	defer releaseTrackerH.StopPoller()
	proxmoxH.StartPoller()
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

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}
