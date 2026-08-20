package main

import (
	"context"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

const demoProxmoxConnName = "Demo - Cluster PVE"

// seedProxmox creates one disabled (never polled — see main.go's DEMO_MODE
// gate and this file's defense-in-depth) proxmox_connections row, find-or-
// create by Name so its ID (and therefore FK-linked nodes/guests) stays
// stable across re-seeds, then upserts 2 nodes and 5 guests plus a short
// metric history for each so /proxmox and its node/guest detail pages render
// charts.
func seedProxmox(ctx context.Context, db *database.DB) error {
	connections, err := db.ListProxmoxConnections(ctx)
	if err != nil {
		return err
	}
	var connID string
	for _, c := range connections {
		if c.Name == demoProxmoxConnName {
			connID = c.ID
			break
		}
	}
	if connID == "" {
		// enabled=false: dummy connection, never polled even if the DEMO_MODE
		// poller gate in main.go is ever removed (defense in depth).
		connID, err = db.CreateProxmoxConnection(ctx, models.ProxmoxConnectionRequest{
			Name: demoProxmoxConnName, APIURL: "https://pve-demo.example.com:8006",
			TokenID: "demo@pve!seed", TokenSecret: "demo-token-secret-not-real",
			InsecureSkipVerify: true, Enabled: false, PollIntervalSec: 60,
		})
		if err != nil {
			return err
		}
	}

	nodes := []struct {
		Name     string
		CPUCores int
		CPUUsage float64
		MemGB    float64
		MemUsage float64
	}{
		{Name: "pve-node-01", CPUCores: 16, CPUUsage: 0.28, MemGB: 64, MemUsage: 0.55},
		{Name: "pve-node-02", CPUCores: 16, CPUUsage: 0.41, MemGB: 64, MemUsage: 0.62},
	}
	for _, n := range nodes {
		memTotal := int64(n.MemGB * gb)
		memUsed := int64(n.MemGB * gb * n.MemUsage)
		if err := db.UpsertProxmoxNode(ctx, connID, n.Name, "online", n.CPUCores, n.CPUUsage,
			memTotal, memUsed, 1_296_000, "8.3.1", "demo-cluster", "10.0.20."+n.Name[len(n.Name)-1:]); err != nil {
			return err
		}
		nodeID, err := db.GetProxmoxNodeID(ctx, connID, n.Name)
		if err != nil {
			return err
		}
		if err := seedProxmoxNodeMetrics(ctx, db, nodeID, connID, n.Name, n.CPUUsage, memTotal, memUsed); err != nil {
			return err
		}
	}

	guests := []struct {
		Node      string
		Type      string
		VMID      int
		Name      string
		Status    string
		CPUAlloc  float64
		CPUUsage  float64
		MemAllocG float64
		MemUsageG float64
	}{
		{Node: "pve-node-01", Type: "vm", VMID: 101, Name: "demo-web-01", Status: "running", CPUAlloc: 4, CPUUsage: 0.18, MemAllocG: 8, MemUsageG: 3.4},
		{Node: "pve-node-01", Type: "vm", VMID: 102, Name: "demo-db-01", Status: "running", CPUAlloc: 8, CPUUsage: 0.35, MemAllocG: 32, MemUsageG: 21.8},
		{Node: "pve-node-02", Type: "vm", VMID: 201, Name: "demo-app-02", Status: "running", CPUAlloc: 4, CPUUsage: 0.32, MemAllocG: 8, MemUsageG: 4.6},
		{Node: "pve-node-02", Type: "lxc", VMID: 301, Name: "demo-worker-01", Status: "stopped", CPUAlloc: 2, CPUUsage: 0, MemAllocG: 4, MemUsageG: 0},
		{Node: "pve-node-02", Type: "lxc", VMID: 302, Name: "demo-monitoring", Status: "running", CPUAlloc: 2, CPUUsage: 0.12, MemAllocG: 4, MemUsageG: 1.9},
	}
	for _, g := range guests {
		memAlloc := int64(g.MemAllocG * gb)
		memUsage := int64(g.MemUsageG * gb)
		if err := db.UpsertProxmoxGuest(ctx, connID, g.Node, g.Type, g.VMID, g.Name, g.Status,
			g.CPUAlloc, g.CPUUsage, memAlloc, memUsage, 100*gb, 40*gb, 648_000, ""); err != nil {
			return err
		}
	}

	nodeGuests, err := db.ListProxmoxGuests(ctx, connID, "", "")
	if err != nil {
		return err
	}
	for _, g := range nodeGuests {
		if err := seedProxmoxGuestMetrics(ctx, db, g.ID, g.CPUUsage, g.MemAlloc, g.MemUsage); err != nil {
			return err
		}
	}
	return nil
}

// seedProxmoxNodeMetrics writes a 2h/5min history via raw SQL — unlike
// InsertMetrics for hosts, InsertProxmoxNodeMetric always stamps NOW(), so a
// controllable-timestamp history needs a direct insert here.
func seedProxmoxNodeMetrics(ctx context.Context, db *database.DB, nodeID, connID, nodeName string, cpuUsage float64, memTotal, memUsed int64) error {
	if _, err := db.Exec(ctx, `DELETE FROM proxmox_node_metrics WHERE node_id = $1`, nodeID); err != nil {
		return err
	}
	const window = 2 * time.Hour
	const step = 5 * time.Minute
	for t := anchor.Add(-window); !t.After(anchor); t = t.Add(step) {
		if _, err := db.Exec(ctx,
			`INSERT INTO proxmox_node_metrics (node_id, connection_id, node_name, cpu_usage, mem_total, mem_used, timestamp)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			nodeID, connID, nodeName, cpuUsage, memTotal, memUsed, t,
		); err != nil {
			return err
		}
	}
	return nil
}

func seedProxmoxGuestMetrics(ctx context.Context, db *database.DB, guestID string, cpuUsage float64, memTotal, memUsed int64) error {
	if _, err := db.Exec(ctx, `DELETE FROM proxmox_guest_metrics WHERE guest_id = $1`, guestID); err != nil {
		return err
	}
	const window = 2 * time.Hour
	const step = 5 * time.Minute
	for t := anchor.Add(-window); !t.After(anchor); t = t.Add(step) {
		if _, err := db.Exec(ctx,
			`INSERT INTO proxmox_guest_metrics (guest_id, cpu_usage, mem_total, mem_used, timestamp)
			 VALUES ($1,$2,$3,$4,$5)`,
			guestID, cpuUsage, memTotal, memUsed, t,
		); err != nil {
			return err
		}
	}
	return nil
}
