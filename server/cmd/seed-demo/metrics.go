package main

import (
	"context"
	"time"

	"github.com/lib/pq"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type hostProfile struct {
	CPUCores   int
	CPUModel   string
	MemTotalGB float64
	BaseCPU    float64 // steady-state CPU usage %
	BaseMem    float64 // steady-state memory usage %
	// Spike, when > 0, ramps CPU linearly from BaseCPU up to Spike over the
	// last 2h of the window — demo-app-02's "about to fire an alert" story.
	Spike float64
}

var hostProfiles = map[string]hostProfile{
	"demo-web-01":    {CPUCores: 4, CPUModel: "Intel Xeon E5-2670 v3", MemTotalGB: 8, BaseCPU: 18, BaseMem: 42},
	"demo-db-01":     {CPUCores: 8, CPUModel: "AMD EPYC 7302P", MemTotalGB: 32, BaseCPU: 35, BaseMem: 68},
	"demo-app-02":    {CPUCores: 4, CPUModel: "Intel Xeon E5-2670 v3", MemTotalGB: 8, BaseCPU: 32, BaseMem: 58, Spike: 92},
	"demo-worker-01": {CPUCores: 2, CPUModel: "Intel Core i5-8250U", MemTotalGB: 4, BaseCPU: 22, BaseMem: 51},
}

const gb = 1024 * 1024 * 1024

// seedMetrics inserts a 6h, 5-minute-step system_metrics history per demo
// host (stopping at that host's LastSeen, so demo-worker-01's chart visibly
// stops 6h in the past) plus one disk_metrics snapshot per mount point,
// including one near-full mount for the "disk full" edge case. Idempotent via
// a delete-then-reinsert scoped to demoHostIDs — these tables have no natural
// business key to upsert on. Real-time continuous-aggregate reads (enabled in
// ensureTimescaleObjects, called by database.New before this runs) already
// union freshly-inserted raw rows, so no explicit cagg refresh is needed.
func seedMetrics(ctx context.Context, db *database.DB) error {
	if _, err := db.Exec(ctx, `DELETE FROM system_metrics WHERE host_id = ANY($1)`, pq.Array(demoHostIDs)); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `DELETE FROM disk_metrics WHERE host_id = ANY($1)`, pq.Array(demoHostIDs)); err != nil {
		return err
	}

	const window = 6 * time.Hour
	const step = 5 * time.Minute

	for _, h := range demoHosts {
		p := hostProfiles[h.ID]
		start := h.LastSeen.Add(-window)
		for t := start; !t.After(h.LastSeen); t = t.Add(step) {
			cpu := p.BaseCPU
			if p.Spike > 0 {
				rampStart := h.LastSeen.Add(-2 * time.Hour)
				if !t.Before(rampStart) {
					progress := t.Sub(rampStart).Seconds() / (2 * time.Hour).Seconds()
					cpu = p.BaseCPU + (p.Spike-p.BaseCPU)*progress
				}
			}
			memTotal := uint64(p.MemTotalGB * gb)
			memUsed := uint64(p.MemTotalGB * gb * p.BaseMem / 100)
			m := &models.SystemMetrics{
				HostID:          h.ID,
				Timestamp:       t,
				CPUUsagePercent: cpu,
				CPUCores:        p.CPUCores,
				CPUModel:        p.CPUModel,
				LoadAvg1:        cpu / 100 * float64(p.CPUCores),
				LoadAvg5:        p.BaseCPU / 100 * float64(p.CPUCores),
				LoadAvg15:       p.BaseCPU / 100 * float64(p.CPUCores),
				MemoryTotal:     memTotal,
				MemoryUsed:      memUsed,
				MemoryFree:      memTotal - memUsed,
				MemoryPercent:   p.BaseMem,
				NetworkRxBytes:  uint64(1_500_000 + t.Unix()%50_000),
				NetworkTxBytes:  uint64(900_000 + t.Unix()%30_000),
				Uptime:          uint64(t.Sub(start).Seconds()) + 86_400*14,
				Hostname:        h.Hostname,
			}
			if _, err := db.InsertMetrics(ctx, m); err != nil {
				return err
			}
		}
	}

	diskRows := []models.DiskMetrics{
		{HostID: "demo-web-01", Timestamp: anchor, MountPoint: "/", Filesystem: "ext4", SizeGB: 80, UsedGB: 34, AvailGB: 46, UsedPercent: 42.5},
		{HostID: "demo-db-01", Timestamp: anchor, MountPoint: "/", Filesystem: "ext4", SizeGB: 80, UsedGB: 28, AvailGB: 52, UsedPercent: 35},
		// The "disk full" edge case: a data volume at 94% usage.
		{HostID: "demo-db-01", Timestamp: anchor, MountPoint: "/var/lib/postgresql", Filesystem: "ext4", SizeGB: 500, UsedGB: 470, AvailGB: 30, UsedPercent: 94},
		{HostID: "demo-app-02", Timestamp: anchor, MountPoint: "/", Filesystem: "ext4", SizeGB: 80, UsedGB: 51, AvailGB: 29, UsedPercent: 63.75},
		{HostID: "demo-worker-01", Timestamp: demoHosts[3].LastSeen, MountPoint: "/", Filesystem: "ext4", SizeGB: 40, UsedGB: 19, AvailGB: 21, UsedPercent: 47.5},
	}
	return db.InsertDiskMetrics(ctx, diskRows)
}
