package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serversupervisor/server/internal/alerts"
	"github.com/serversupervisor/server/internal/api"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/github"
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
	defer db.Close()

	// Cleanup stalled commands at startup (commands older than 10 minutes)
	if err := db.CleanupStalledCommands(10); err != nil {
		log.Printf("Warning: failed to cleanup stalled commands: %v", err)
	}

	// Create default admin user (sets must_change_password if using default "admin" password)
	hash, err := api.HashPassword(cfg.AdminPassword)
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

	// Start GitHub release tracker
	tracker := github.NewTracker(db, cfg)
	tracker.Start()
	defer tracker.Stop()

	// Start periodic cleanup of old audit logs (90-day retention for compliance).
	// Metrics retention is now managed by TimescaleDB retention policies; see
	// migration 010_timescaledb.sql.  CleanOldMetrics is available for manual use
	// on installations without TimescaleDB.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if deleted, err := db.CleanOldAuditLogs(90); err != nil {
				log.Printf("Audit cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("Cleaned up %d old audit log records", deleted)
			}
		}
	}()

	// Start periodic host status check (mark offline if no heartbeat for 2 minutes)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			// Update all hosts that haven't been seen in 2+ minutes
			if err := db.UpdateHostStatusBasedOnLastSeen(2); err != nil {
				log.Printf("Failed to update host status: %v", err)
			}
		}
	}()

	// Start periodic alert evaluation
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			alerts.EvaluateAlerts(db, cfg)
		}
	}()

	// Start periodic metrics downsampling (single batch SQL query for all hosts)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now().UTC()
			end := now.Truncate(5 * time.Minute)
			start5 := end.Add(-5 * time.Minute)

			if n, err := db.BatchAggregateMetrics(start5, end, "5min"); err != nil {
				log.Printf("5min downsampling error: %v", err)
			} else if n > 0 {
				log.Printf("Downsampled 5min metrics for %d hosts", n)
			}

			if end.Minute() == 0 {
				if _, err := db.BatchAggregateMetrics(end.Add(-time.Hour), end, "hour"); err != nil {
					log.Printf("Hourly downsampling error: %v", err)
				}
			}

			if end.Hour() == 0 && end.Minute() == 0 {
				if _, err := db.BatchAggregateMetrics(end.Add(-24*time.Hour), end, "day"); err != nil {
					log.Printf("Daily downsampling error: %v", err)
				}
			}
		}
	}()

	// Setup router
	router := api.SetupRouter(db, cfg)

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
