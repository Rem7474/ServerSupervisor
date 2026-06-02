package ws

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
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
func (h *WSHandler) dashboardPayload(ctx context.Context) (gin.H, error) {
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

func (h *WSHandler) buildDashboardPayload(ctx context.Context) (gin.H, error) {
	hosts, err := h.db.GetAllHosts(ctx)
	if err != nil {
		return nil, err
	}

	hostMetrics, _ := h.db.GetLatestMetricsAll(ctx)
	if hostMetrics == nil {
		hostMetrics = map[string]*models.SystemMetrics{}
	}
	for hostID, m := range hostMetrics {
		if m == nil {
			continue
		}
		if temp, ok := h.db.GetEffectiveHostCPUTemperature(ctx, hostID, m.CPUTemperature); ok {
			m.CPUTemperature = temp
		}
	}

	comparisons, err := h.buildVersionComparisons(ctx)
	if err != nil {
		comparisons = []models.VersionComparison{}
	}

	proxmoxNodes, _ := h.db.ListProxmoxNodes(ctx)
	if proxmoxNodes == nil {
		proxmoxNodes = []models.ProxmoxNode{}
	}

	// Confirmed guest-host links with live guest metrics (cpu_usage, mem_alloc, mem_usage).
	// Used by the dashboard to override agent CPU/RAM when metrics_source=proxmox.
	proxmoxLinks, _ := h.db.ListProxmoxGuestLinks(ctx, "confirmed")
	if proxmoxLinks == nil {
		proxmoxLinks = []models.ProxmoxGuestLink{}
	}

	payload := gin.H{
		"type":                "dashboard",
		"hosts":               hosts,
		"host_metrics":        hostMetrics,
		"version_comparisons": comparisons,
		"apt_pending":         h.db.GetTotalAptPending(ctx),
		"apt_pending_hosts":   h.db.GetAptPendingAll(ctx),
		"disk_usage":          h.db.GetRootDiskPercentAll(ctx),
		"proxmox_nodes":       proxmoxNodes,
		"proxmox_links":       proxmoxLinks,
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
	aptHistory, _ := h.db.GetAptHistoryWithAgentUpdates(ctx, hostID, 50)
	uuStatus, _ := h.db.GetUUStatus(ctx, hostID)
	uuRuns, _ := h.db.GetUURuns(ctx, hostID, 20)
	auditLogs, _ := h.db.GetAuditLogsByHost(ctx, hostID, 50)

	allComparisons, _ := h.buildVersionComparisons(ctx)
	comparisons := make([]models.VersionComparison, 0)
	for _, vc := range allComparisons {
		if vc.HostID == hostID {
			comparisons = append(comparisons, vc)
		}
	}

	proxmoxLink, _ := h.db.GetProxmoxGuestLinkByHost(ctx, hostID)

	payload := gin.H{
		"type":                "host_detail",
		"host":                host,
		"metrics":             metrics,
		"containers":          containers,
		"apt_status":          aptStatus,
		"apt_history":         aptHistory,
		"uu_status":           uuStatus,
		"uu_runs":             uuRuns,
		"audit_logs":          auditLogs,
		"version_comparisons": comparisons,
		"proxmox_link":        proxmoxLink,
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

	payload := gin.H{
		"type":                "docker",
		"containers":          containers,
		"compose_projects":    composeProjects,
		"version_comparisons": comparisons,
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

	payload := gin.H{
		"type":       "network",
		"hosts":      snapshot.Hosts,
		"containers": snapshot.Containers,
		"config":     config,
		"updated_at": snapshot.UpdatedAt,
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

	payload := gin.H{
		"type":          "apt",
		"hosts":         hosts,
		"apt_statuses":  aptStatuses,
		"apt_histories": aptHistories,
	}
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return safeWriteJSON(conn, payload)
}
