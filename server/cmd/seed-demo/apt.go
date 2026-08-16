package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

// seedApt upserts one apt_status row per demo host via UpsertAptStatus (ON
// CONFLICT (host_id) DO UPDATE — idempotent by construction).
func seedApt(ctx context.Context, db *database.DB) error {
	packages := []string{
		"openssl", "libssl3", "curl", "libcurl4", "python3.12", "python3.12-minimal",
		"linux-libc-dev", "systemd", "libsystemd0", "sudo", "bash", "coreutils",
		"tzdata", "ca-certificates", "gnupg",
	}
	packagesJSON, err := json.Marshal(packages)
	if err != nil {
		return err
	}

	cves := []models.CVEInfo{
		{ID: "CVE-2025-31337", Severity: "critical", UbuntuPriority: "critical", CVSSScore: 9.1, Package: "openssl"},
		{ID: "CVE-2025-22222", Severity: "high", UbuntuPriority: "high", CVSSScore: 7.5, Package: "libcurl4"},
		{ID: "CVE-2025-19999", Severity: "medium", UbuntuPriority: "medium", CVSSScore: 5.3, Package: "python3.12"},
	}
	cveJSON, err := json.Marshal(cves)
	if err != nil {
		return err
	}

	for _, h := range demoHosts {
		status := &models.AptStatus{
			HostID:          h.ID,
			LastUpdate:      anchor.Add(-4 * time.Hour),
			LastUpgrade:     anchor.Add(-72 * time.Hour),
			PendingPackages: len(packages),
			PackageList:     string(packagesJSON),
			SecurityUpdates: 2,
			CVEList:         string(cveJSON),
		}
		if err := db.UpsertAptStatus(ctx, status); err != nil {
			return err
		}
	}
	return nil
}
