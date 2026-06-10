package ws

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/networkview"
)

func (h *WSHandler) sendDashboardSnapshot(ctx context.Context, conn *websocket.Conn, lastHash *string) error {
	payload, err := h.dashboardPayload(ctx)
	if err != nil {
		return err
	}
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return safeWriteJSON(conn, payload)
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
		hosts, hostsErr = h.db.GetAllHosts(ctx)
	}()

	go func() {
		defer wg.Done()
		hostMetrics, _ = h.db.GetLatestMetricsAll(ctx)
	}()

	go func() {
		defer wg.Done()
		c, err := h.buildVersionComparisons(ctx)
		if err == nil {
			comparisons = c
		}
	}()

	go func() {
		defer wg.Done()
		proxmoxNodes, _ = h.db.ListProxmoxNodes(ctx)
	}()

	go func() {
		defer wg.Done()
		proxmoxLinks, _ = h.db.ListProxmoxGuestLinks(ctx, "confirmed")
	}()

	go func() {
		defer wg.Done()
		aptPending = h.db.GetTotalAptPending(ctx)
	}()

	go func() {
		defer wg.Done()
		aptPendingHosts = h.db.GetAptPendingAll(ctx)
	}()

	go func() {
		defer wg.Done()
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

	allComparisons, _ := h.buildVersionComparisons(ctx)
	comparisons := make([]models.VersionComparison, 0)
	for _, vc := range allComparisons {
		if vc.HostID == hostID {
			comparisons = append(comparisons, vc)
		}
	}

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
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return safeWriteJSON(conn, payload)
}

func (h *WSHandler) sendDockerSnapshot(ctx context.Context, conn *websocket.Conn, lastHash *string) error {
	containers, err := h.db.GetAllDockerContainers(ctx)
	if err != nil {
		return err
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
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return safeWriteJSON(conn, payload)
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
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return safeWriteJSON(conn, payload)
}

func (h *WSHandler) sendAptSnapshot(ctx context.Context, conn *websocket.Conn, lastHash *string) error {
	hosts, err := h.db.GetAllHosts(ctx)
	if err != nil {
		return err
	}

	aptStatuses := map[string]*models.AptStatus{}
	aptHistories := map[string][]models.RemoteCommand{}

	for _, host := range hosts {
		status, err := h.db.GetAptStatus(ctx, host.ID)
		if err == nil {
			aptStatuses[host.ID] = status
		}
		hist, err := h.db.GetAptHistoryWithAgentUpdates(ctx, host.ID, 20)
		if err == nil {
			aptHistories[host.ID] = hist
		}
	}

	payload := &models.WSAptSnapshot{
		Type:         "apt",
		Hosts:        hosts,
		AptStatuses:  aptStatuses,
		AptHistories: aptHistories,
	}
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return safeWriteJSON(conn, payload)
}
