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
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/github"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("ServerSupervisor - Starting server...")

	// Load config
	cfg := config.Load()

	// ⚠️  Security warnings for default configuration
	if cfg.JWTSecret == "change-me-in-production-please" {
		log.Println("⚠️  WARNING: JWT_SECRET is using the default insecure value. Change it in production!")
	}
	if cfg.AdminPassword == "admin" {
		log.Println("⚠️  WARNING: ADMIN_PASSWORD is 'admin'. Change it immediately!")
	}

	// Connect to database
	db, err := database.New(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create default admin user
	hash, err := api.HashPassword(cfg.AdminPassword)
	if err != nil {
		log.Fatalf("Failed to hash admin password: %v", err)
	}
	if err := db.CreateUser(cfg.AdminUser, hash, "admin"); err != nil {
		log.Printf("Admin user creation: %v (may already exist)", err)
	}

	// Start GitHub release tracker
	tracker := github.NewTracker(db, cfg)
	tracker.Start()
	defer tracker.Stop()

	// Start periodic cleanup of old metrics
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			deleted, err := db.CleanOldMetrics(cfg.MetricsRetentionDays)
			if err != nil {
				log.Printf("Metrics cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("Cleaned up %d old metrics records", deleted)
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

	// Start periodic metrics downsampling (aggregate raw metrics)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			// Get all hosts and downsample their metrics
			hosts, err := db.GetAllHosts()
			if err != nil {
				log.Printf("Failed to get hosts for downsampling: %v", err)
				continue
			}
			for _, h := range hosts {
				_ = h // TODO: Implement metrics downsampling for each host
				// Downsample 5-minute aggregates (keep for 30 days)
				// Downsampling logic would go here
				// For now, this is a placeholder for future enhancement
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
		WriteTimeout: 15 * time.Second,
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
