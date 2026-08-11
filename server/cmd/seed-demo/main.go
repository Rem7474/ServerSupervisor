// Command seed-demo fills a DEMO_MODE database with a fixed, realistic fleet
// of hosts/VMs, Docker containers, APT packages, alerts, scheduled tasks,
// audit history and uptime/SSL probes — used for local demo mode and the
// release screenshot pipeline (.github/workflows/screenshots.yml). See
// CONTRIBUTING.md's "Mode démo" section.
package main

import (
	"context"
	"log"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
)

// anchor is captured once at process start and every seeded timestamp is an
// offset from it (anchor.Add(-2*time.Hour), etc.) instead of a hardcoded
// calendar date. This is deliberate: a hardcoded date would make "seeded N
// days ago" text drift further into the past on every release, producing a
// screenshot diff every single tag even with zero real changes. Anchoring to
// "now" at each seed run keeps every relative-time label (host last-seen,
// audit log entries, alert incidents, uptime history) rendering identically
// run over run — which is what the CI screenshot pipeline actually needs.
var anchor = time.Now().UTC()

func main() {
	cfg := config.Load()
	if !cfg.DemoMode {
		log.Fatal("seed-demo refuses to run without DEMO_MODE=true — refusing to write fixture data into what looks like a real database")
	}

	if err := database.EnsureDatabaseExists(cfg); err != nil {
		log.Printf("warning: could not ensure database exists: %v (will retry on connection)", err)
	}

	db, err := database.New(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	start := time.Now()

	steps := []struct {
		name string
		fn   func(context.Context, *database.DB) error
	}{
		{"hosts", seedHosts},
		{"metrics", seedMetrics},
		{"docker", seedDocker},
		{"apt", seedApt},
		{"alerts", seedAlerts},
		{"scheduled_tasks", seedScheduledTasks},
		{"audit", seedAudit},
		{"uptime_ssl", seedUptimeSSL},
		{"proxmox", seedProxmox},
	}

	for _, s := range steps {
		stepStart := time.Now()
		if err := s.fn(ctx, db); err != nil {
			log.Fatalf("seed step %q failed: %v", s.name, err)
		}
		log.Printf("seed step %q done in %s", s.name, time.Since(stepStart).Round(time.Millisecond))
	}

	elapsed := time.Since(start)
	log.Printf("seed-demo completed in %s", elapsed.Round(time.Millisecond))
	if elapsed > 30*time.Second {
		log.Printf("warning: seed-demo took longer than the 30s target")
	}
}
