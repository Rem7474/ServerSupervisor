package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// demoHost is the seed definition for one fleet host. Real column names/IDs
// are fixed strings (not random UUIDs) so every seed run targets the exact
// same rows — the idempotency strategy for every table in this package.
type demoHost struct {
	ID         string
	Name       string
	Hostname   string
	IPAddress  string
	OS         string
	AgentVer   string
	Tags       []string
	Status     string
	LastSeen   time.Time
	Collectors map[string]bool
}

// demoHosts is the fixed fleet. demo-worker-01's LastSeen is far in the past
// on purpose — the "offline host" edge case the demo is meant to exercise.
var demoHosts = []demoHost{
	{
		ID: "demo-web-01", Name: "web-01", Hostname: "web-01.demo.local",
		IPAddress: "10.0.10.11", OS: "Ubuntu 24.04 LTS", AgentVer: "1.8.2",
		Tags: []string{"production", "nginx"}, Status: "online", LastSeen: anchor,
		Collectors: map[string]bool{"docker": true, "apt": true, "smart": true},
	},
	{
		ID: "demo-db-01", Name: "db-01", Hostname: "db-01.demo.local",
		IPAddress: "10.0.10.12", OS: "Debian 12", AgentVer: "1.8.2",
		Tags: []string{"production", "database"}, Status: "online", LastSeen: anchor,
		Collectors: map[string]bool{"docker": true, "apt": true, "smart": true},
	},
	{
		ID: "demo-app-02", Name: "app-02", Hostname: "app-02.demo.local",
		IPAddress: "10.0.10.21", OS: "Ubuntu 24.04 LTS", AgentVer: "1.8.2",
		Tags: []string{"production", "app"}, Status: "online", LastSeen: anchor,
		Collectors: map[string]bool{"docker": true, "apt": true},
	},
	{
		ID: "demo-worker-01", Name: "worker-01", Hostname: "worker-01.demo.local",
		IPAddress: "10.0.10.31", OS: "Ubuntu 22.04 LTS", AgentVer: "1.7.0",
		Tags: []string{"staging", "worker"}, Status: "offline", LastSeen: anchor.Add(-6 * time.Hour),
		Collectors: map[string]bool{"docker": true},
	},
}

// demoHostIDs is demoHosts' ID column, used to scope deletes in the other
// seed steps (metrics/docker/apt/audit all belong to one of these hosts).
var demoHostIDs = func() []string {
	ids := make([]string, len(demoHosts))
	for i, h := range demoHosts {
		ids[i] = h.ID
	}
	return ids
}()

func seedHosts(ctx context.Context, db *database.DB) error {
	for _, h := range demoHosts {
		_, err := db.GetHost(ctx, h.ID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if err := db.RegisterHost(ctx, &models.Host{
				ID: h.ID, Name: h.Name, Hostname: h.Hostname, IPAddress: h.IPAddress,
				OS: h.OS, Status: h.Status, LastSeen: h.LastSeen, Tags: h.Tags,
			}); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			name, hostname, ip, os := h.Name, h.Hostname, h.IPAddress, h.OS
			if err := db.UpdateHost(ctx, h.ID, &models.HostUpdate{
				Name: &name, Hostname: &hostname, IPAddress: &ip, OS: &os, Tags: &h.Tags,
			}); err != nil {
				return err
			}
		}
		if err := db.UpdateHostStatus(ctx, h.ID, h.Status); err != nil {
			return err
		}
		collectorsJSON, err := json.Marshal(h.Collectors)
		if err != nil {
			return err
		}
		if err := db.UpdateHostCollectors(ctx, h.ID, string(collectorsJSON)); err != nil {
			return err
		}
	}

	// demo-worker-01's LastSeen must stay in the past even on a re-seed of an
	// already-existing row — UpdateHost above doesn't touch last_seen (agents
	// own that column in the real flow), so set it explicitly here.
	for _, h := range demoHosts {
		if _, err := db.Exec(ctx, `UPDATE hosts SET last_seen = $2 WHERE id = $1`, h.ID, h.LastSeen); err != nil {
			return err
		}
	}
	return nil
}
