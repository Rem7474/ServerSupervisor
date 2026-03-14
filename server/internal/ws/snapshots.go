package ws

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/networkview"
)

func (h *WSHandler) sendDashboardSnapshot(conn *websocket.Conn, lastHash *string) error {
	hosts, err := h.db.GetAllHosts()
	if err != nil {
		return err
	}

	hostMetrics, _ := h.db.GetLatestMetricsAll()
	if hostMetrics == nil {
		hostMetrics = map[string]*models.SystemMetrics{}
	}

	comparisons, err := h.buildVersionComparisons()
	if err != nil {
		comparisons = []models.VersionComparison{}
	}

	payload := gin.H{
		"type":                "dashboard",
		"hosts":               hosts,
		"host_metrics":        hostMetrics,
		"version_comparisons": comparisons,
		"apt_pending":         h.db.GetTotalAptPending(),
		"apt_pending_hosts":   h.db.GetAptPendingAll(),
		"disk_usage":          h.db.GetRootDiskPercentAll(),
	}
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendHostSnapshot(conn *websocket.Conn, hostID string, lastHash *string) error {
	host, err := h.db.GetHost(hostID)
	if err != nil {
		return err
	}
	metrics, _ := h.db.GetLatestMetrics(hostID)
	containers, _ := h.db.GetDockerContainers(hostID)
	aptStatus, _ := h.db.GetAptStatus(hostID)
	aptHistory, _ := h.db.GetAptHistoryWithAgentUpdates(hostID, 50)
	auditLogs, _ := h.db.GetAuditLogsByHost(hostID, 50)

	allComparisons, _ := h.buildVersionComparisons()
	comparisons := make([]models.VersionComparison, 0)
	for _, vc := range allComparisons {
		if vc.HostID == hostID {
			comparisons = append(comparisons, vc)
		}
	}

	proxmoxLink, _ := h.db.GetProxmoxGuestLinkByHost(hostID)

	payload := gin.H{
		"type":                "host_detail",
		"host":                host,
		"metrics":             metrics,
		"containers":          containers,
		"apt_status":          aptStatus,
		"apt_history":         aptHistory,
		"audit_logs":          auditLogs,
		"version_comparisons": comparisons,
		"proxmox_link":        proxmoxLink,
	}
	if !snapshotChanged(payload, lastHash) {
		return nil
	}
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendDockerSnapshot(conn *websocket.Conn, lastHash *string) error {
	containers, err := h.db.GetAllDockerContainers()
	if err != nil {
		return err
	}

	composeProjects, _ := h.db.GetAllComposeProjects()
	if composeProjects == nil {
		composeProjects = []models.ComposeProject{}
	}

	comparisons, err := h.buildVersionComparisons()
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
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendNetworkSnapshot(conn *websocket.Conn, lastHash *string) error {
	snapshot, err := networkview.BuildSnapshot(h.db)
	if err != nil {
		return err
	}

	config, _ := h.db.GetNetworkTopologyConfig()

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
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendAptSnapshot(conn *websocket.Conn, lastHash *string) error {
	hosts, err := h.db.GetAllHosts()
	if err != nil {
		return err
	}

	aptStatuses := map[string]*models.AptStatus{}
	aptHistories := map[string][]models.RemoteCommand{}

	for _, host := range hosts {
		status, err := h.db.GetAptStatus(host.ID)
		if err == nil {
			aptStatuses[host.ID] = status
		}
		hist, err := h.db.GetAptHistoryWithAgentUpdates(host.ID, 20)
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
	return conn.WriteJSON(payload)
}
