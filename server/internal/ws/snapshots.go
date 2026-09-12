package ws

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/networkview"
	"github.com/serversupervisor/server/internal/safego"
)

func (h *WSHandler) sendDashboardSnapshot(ctx context.Context, conn *websocket.Conn, lastHash *string) error {
	payload, err := h.dashboardPayload(ctx)
	if err != nil {
		return err
	}
	raw, changed := snapshotChanged(payload, lastHash)
	if !changed {
		return nil
	}
	return safeWriteRaw(conn, raw)
}

// dashboardPayload returns the shared dashboard snapshot, rebuilding it only when
// the cached copy is older than dashboardCacheTTL. The build runs without holding
// the lock so a slow DB doesn't serialize all clients; a brief concurrent
// double-build during a cache miss is harmless (both produce the same result).
func (h *WSHandler) dashboardPayload(ctx context.Context) (*models.WSDashboardSnapshot, error) {
	h.dashCacheMu.Lock()
	if h.dashCache != nil && time.Since(h.dashCacheAt) < dashboardCacheTTL {
		cached := h.dashCache
		h.dashCacheMu.Unlock()
		return cached, nil
	}
	h.dashCacheMu.Unlock()

	payload, err := h.buildDashboardPayload(ctx)
	if err != nil {
		return nil, err
	}

	h.dashCacheMu.Lock()
	h.dashCache = payload
	h.dashCacheAt = time.Now()
	h.dashCacheMu.Unlock()
	return payload, nil
}

func (h *WSHandler) buildDashboardPayload(ctx context.Context) (*models.WSDashboardSnapshot, error) {
	var (
		hosts       []models.Host
		hostsErr    error
		hostMetrics map[string]*models.SystemMetrics

		comparisons []models.VersionComparison

		proxmoxNodes []models.ProxmoxNode
		proxmoxLinks []models.ProxmoxGuestLink

		aptPending      int
		aptPendingHosts map[string]int
		diskUsage       map[string]float64
	)

	var wg sync.WaitGroup
	wg.Add(8)

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.hosts")
		hosts, hostsErr = h.db.GetAllHosts(ctx)
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.hostMetrics")
		hostMetrics, _ = h.db.GetLatestMetricsAll(ctx)
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.versionComparisons")
		c, err := h.buildVersionComparisons(ctx)
		if err == nil {
			comparisons = c
		}
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.proxmoxNodes")
		proxmoxNodes, _ = h.db.ListProxmoxNodes(ctx)
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.proxmoxLinks")
		proxmoxLinks, _ = h.db.ListProxmoxGuestLinks(ctx, "confirmed")
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.aptPending")
		aptPending = h.db.GetTotalAptPending(ctx)
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.aptPendingHosts")
		aptPendingHosts = h.db.GetAptPendingAll(ctx)
	}()

	go func() {
		defer wg.Done()
		defer safego.Recover(ctx, "ws.buildDashboardPayload.diskUsage")
		diskUsage = h.db.GetRootDiskPercentAll(ctx)
	}()

	wg.Wait()

	if hostsErr != nil {
		return nil, hostsErr
	}
	// Project the full per-host metrics onto the lean dashboard subset
	// (CPU% / memory% / uptime) — the dashboard renders nothing else, so the rest
	// of SystemMetrics never goes on the 10s-per-client wire.
	leanMetrics := make(map[string]*models.DashboardHostMetrics, len(hostMetrics))
	for id, m := range hostMetrics {
		if m == nil {
			continue
		}
		leanMetrics[id] = &models.DashboardHostMetrics{
			CPUUsagePercent: m.CPUUsagePercent,
			MemoryPercent:   m.MemoryPercent,
			Uptime:          m.Uptime,
		}
	}
	if comparisons == nil {
		comparisons = []models.VersionComparison{}
	}
	if proxmoxNodes == nil {
		proxmoxNodes = []models.ProxmoxNode{}
	}
	if proxmoxLinks == nil {
		proxmoxLinks = []models.ProxmoxGuestLink{}
	}
	if aptPendingHosts == nil {
		aptPendingHosts = map[string]int{}
	}
	if diskUsage == nil {
		diskUsage = map[string]float64{}
	}

	// Keep dashboard snapshot assembly strictly set-based: per-host effective
	// sensor resolution triggers N+1 SQL queries and delays first paint.
	// Host detail keeps the richer per-host resolution path.
	payload := &models.WSDashboardSnapshot{
		Type:               "dashboard",
		Hosts:              hosts,
		HostMetrics:        leanMetrics,
		VersionComparisons: comparisons,
		AptPending:         aptPending,
		AptPendingHosts:    aptPendingHosts,
		DiskUsage:          diskUsage,
		ProxmoxNodes:       proxmoxNodes,
		ProxmoxLinks:       proxmoxLinks,
	}
	return payload, nil
}

func (h *WSHandler) sendHostSnapshot(ctx context.Context, conn *websocket.Conn, hostID string, lastHash *string) error {
	host, err := h.db.GetHost(ctx, hostID)
	if err != nil {
		return err
	}
	metrics, _ := h.db.GetLatestMetrics(ctx, hostID)
	if metrics != nil {
		if temp, ok := h.db.GetEffectiveHostCPUTemperature(ctx, hostID, metrics.CPUTemperature); ok {
			metrics.CPUTemperature = temp
		}
	}
	containers, _ := h.db.GetDockerContainers(ctx, hostID)
	aptStatus, _ := h.db.GetAptStatus(ctx, hostID)
	uuStatus, _ := h.db.GetUUStatus(ctx, hostID)
	uuRuns, _ := h.db.GetUURuns(ctx, hostID, 20)

	comparisons, _ := h.buildVersionComparisonsForHost(ctx, hostID)

	proxmoxLink, _ := h.db.GetProxmoxGuestLinkByHost(ctx, hostID)

	payload := &models.WSHostSnapshot{
		Type:               "host_detail",
		Host:               host,
		Metrics:            metrics,
		Containers:         containers,
		AptStatus:          aptStatus,
		UUStatus:           uuStatus,
		UURuns:             uuRuns,
		VersionComparisons: comparisons,
		ProxmoxLink:        proxmoxLink,
	}
	raw, changed := snapshotChanged(payload, lastHash)
	if !changed {
		return nil
	}
	return safeWriteRaw(conn, raw)
}

func (h *WSHandler) sendDockerSnapshot(ctx context.Context, conn *websocket.Conn, lastHash *string) error {
	containers, err := h.db.GetAllDockerContainers(ctx)
	if err != nil {
		return err
	}
	if containers == nil {
		containers = []models.DockerContainer{}
	}

	composeProjects, _ := h.db.GetAllComposeProjects(ctx)
	if composeProjects == nil {
		composeProjects = []models.ComposeProject{}
	}

	comparisons, err := h.buildVersionComparisons(ctx)
	if err != nil {
		comparisons = []models.VersionComparison{}
	}

	payload := &models.WSDockerSnapshot{
		Type:               "docker",
		Containers:         containers,
		ComposeProjects:    composeProjects,
		VersionComparisons: comparisons,
	}
	raw, changed := snapshotChanged(payload, lastHash)
	if !changed {
		return nil
	}
	return safeWriteRaw(conn, raw)
}

func (h *WSHandler) sendNetworkSnapshot(ctx context.Context, conn *websocket.Conn, lastHash *string) error {
	snapshot, err := networkview.BuildSnapshot(ctx, h.db)
	if err != nil {
		return err
	}

	config, _ := h.db.GetNetworkTopologyConfig(ctx)

	payload := &models.WSNetworkSnapshot{
		Type:       "network",
		Hosts:      snapshot.Hosts,
		Containers: snapshot.Containers,
		Config:     config,
		UpdatedAt:  snapshot.UpdatedAt,
	}
	raw, changed := snapshotChanged(payload, lastHash)
	if !changed {
		return nil
	}
	return safeWriteRaw(conn, raw)
}

func (h *WSHandler) sendAptSnapshot(ctx context.Context, conn *websocket.Conn, lastHash *string) error {
	hosts, err := h.db.GetAllHosts(ctx)
	if err != nil {
		return err
	}

	aptStatuses, _ := h.db.GetAptStatusAll(ctx)
	aptHistories, _ := h.db.GetAptHistoryAll(ctx, 20)
	uuStatuses, _ := h.db.GetUUStatusAll(ctx)

	payload := &models.WSAptSnapshot{
		Type:               "apt",
		Hosts:              hosts,
		AptStatuses:        aptStatuses,
		AptHistories:       aptHistories,
		UUStatuses:         uuStatuses,
		LatestAgentVersion: h.latestAgentVersion(),
	}
	raw, changed := snapshotChanged(payload, lastHash)
	if !changed {
		return nil
	}
	return safeWriteRaw(conn, raw)
}
