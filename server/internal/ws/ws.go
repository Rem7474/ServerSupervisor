package ws

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/networkview"
)

// snapshotChanged returns true (and updates *lastHash) when payload differs from the
// previously sent snapshot. It allows the caller to skip WriteJSON when nothing changed.
func snapshotChanged(payload gin.H, lastHash *string) bool {
	raw, err := json.Marshal(payload)
	if err != nil {
		return true // marshal failure → always send to be safe
	}
	sum := sha256.Sum256(raw)
	h := fmt.Sprintf("%x", sum)
	if h == *lastHash {
		return false
	}
	*lastHash = h
	return true
}

// wsMaxConnsPerIP is the maximum number of concurrent WebSocket connections allowed per client IP.
const wsMaxConnsPerIP = 20

type WSHandler struct {
	db        *database.DB
	cfg       *config.Config
	streamHub *CommandStreamHub
	notifHub  *NotificationHub
	ipConns   map[string]int
	ipConnsMu sync.Mutex
}

func NewWSHandler(db *database.DB, cfg *config.Config, notifHub *NotificationHub) *WSHandler {
	return &WSHandler{
		db:        db,
		cfg:       cfg,
		streamHub: NewCommandStreamHub(),
		notifHub:  notifHub,
		ipConns:   make(map[string]int),
	}
}

func (h *WSHandler) acquireConn(ip string) bool {
	h.ipConnsMu.Lock()
	defer h.ipConnsMu.Unlock()
	if h.ipConns[ip] >= wsMaxConnsPerIP {
		return false
	}
	h.ipConns[ip]++
	return true
}

func (h *WSHandler) releaseConn(ip string) {
	h.ipConnsMu.Lock()
	defer h.ipConnsMu.Unlock()
	if h.ipConns[ip] > 0 {
		h.ipConns[ip]--
		if h.ipConns[ip] == 0 {
			delete(h.ipConns, ip)
		}
	}
}

// GetStreamHub returns the command stream hub for use by other handlers.
func (h *WSHandler) GetStreamHub() *CommandStreamHub {
	return h.streamHub
}

// GetNotificationHub returns the notification broadcast hub.
func (h *WSHandler) GetNotificationHub() *NotificationHub {
	return h.notifHub
}

const (
	wsPingInterval = 30 * time.Second
	wsPongWait     = 60 * time.Second
)

type wsAuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

func (h *WSHandler) upgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return isAllowedOrigin(r.Header.Get("Origin"), h.cfg.BaseURL, h.cfg.AllowedOrigins)
		},
	}
}

func isAllowedOrigin(origin string, baseURL string, extraOrigins []string) bool {
	if origin == "" {
		return true
	}

	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		log.Printf("[WS] rejected origin (parse error): %q", origin)
		return false
	}

	// Always allow localhost for development
	if strings.Contains(parsedOrigin.Host, "localhost") ||
		strings.Contains(parsedOrigin.Host, "127.0.0.1") ||
		strings.Contains(parsedOrigin.Host, "[::1]") {
		return true
	}

	// Check against BASE_URL
	parsedBase, err := url.Parse(baseURL)
	if err == nil {
		// Exact match (scheme + host)
		if parsedOrigin.Host == parsedBase.Host && parsedOrigin.Scheme == parsedBase.Scheme {
			return true
		}
		// Allow host-only match — handles http/https mismatch during NPM setup
		if parsedOrigin.Host == parsedBase.Host {
			log.Printf("[WS] allowed origin with scheme mismatch: origin=%q base=%q (update BASE_URL scheme if needed)", origin, baseURL)
			return true
		}
	}

	// Check against ALLOWED_ORIGINS list (extra origins from config)
	for _, allowed := range extraOrigins {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		parsedAllowed, err := url.Parse(allowed)
		if err != nil {
			continue
		}
		if parsedOrigin.Host == parsedAllowed.Host && parsedOrigin.Scheme == parsedAllowed.Scheme {
			return true
		}
	}

	log.Printf("[WS] rejected origin: %q (BASE_URL=%q) — set BASE_URL or ALLOWED_ORIGINS correctly", origin, baseURL)
	return false
}

func (h *WSHandler) authenticateWS(conn *websocket.Conn) bool {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg wsAuthMessage
	if err := conn.ReadJSON(&msg); err != nil {
		_ = conn.WriteJSON(gin.H{"type": "auth_error", "error": "missing auth"})
		return false
	}
	if msg.Type != "auth" || msg.Token == "" {
		_ = conn.WriteJSON(gin.H{"type": "auth_error", "error": "invalid auth"})
		return false
	}
	if !h.validateToken(msg.Token) {
		_ = conn.WriteJSON(gin.H{"type": "auth_error", "error": "unauthorized"})
		return false
	}
	_ = conn.SetReadDeadline(time.Time{})
	return true
}

func (h *WSHandler) Dashboard(c *gin.Context) {
	ip := c.ClientIP()
	if !h.acquireConn(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many WebSocket connections from this IP"})
		return
	}
	defer h.releaseConn(ip)

	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if !h.authenticateWS(conn) {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	dataTicker := time.NewTicker(10 * time.Second)
	pingTicker := time.NewTicker(wsPingInterval)
	defer dataTicker.Stop()
	defer pingTicker.Stop()

	var lastHash string
	if err := h.sendDashboardSnapshot(conn, &lastHash); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-dataTicker.C:
			if err := h.sendDashboardSnapshot(conn, &lastHash); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *WSHandler) HostDetail(c *gin.Context) {
	hostID := c.Param("id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host id required"})
		return
	}

	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if !h.authenticateWS(conn) {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	dataTicker := time.NewTicker(10 * time.Second)
	pingTicker := time.NewTicker(wsPingInterval)
	defer dataTicker.Stop()
	defer pingTicker.Stop()

	var lastHash string
	if err := h.sendHostSnapshot(conn, hostID, &lastHash); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-dataTicker.C:
			if err := h.sendHostSnapshot(conn, hostID, &lastHash); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *WSHandler) Docker(c *gin.Context) {
	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if !h.authenticateWS(conn) {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	dataTicker := time.NewTicker(10 * time.Second)
	pingTicker := time.NewTicker(wsPingInterval)
	defer dataTicker.Stop()
	defer pingTicker.Stop()

	var lastHash string
	if err := h.sendDockerSnapshot(conn, &lastHash); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-dataTicker.C:
			if err := h.sendDockerSnapshot(conn, &lastHash); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *WSHandler) Network(c *gin.Context) {
	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if !h.authenticateWS(conn) {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	dataTicker := time.NewTicker(10 * time.Second)
	pingTicker := time.NewTicker(wsPingInterval)
	defer dataTicker.Stop()
	defer pingTicker.Stop()

	var lastHash string
	if err := h.sendNetworkSnapshot(conn, &lastHash); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-dataTicker.C:
			if err := h.sendNetworkSnapshot(conn, &lastHash); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *WSHandler) Apt(c *gin.Context) {
	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	if !h.authenticateWS(conn) {
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	dataTicker := time.NewTicker(10 * time.Second)
	pingTicker := time.NewTicker(wsPingInterval)
	defer dataTicker.Stop()
	defer pingTicker.Stop()

	var lastHash string
	if err := h.sendAptSnapshot(conn, &lastHash); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-dataTicker.C:
			if err := h.sendAptSnapshot(conn, &lastHash); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// CommandStream allows clients to subscribe to real-time command output (all modules).
func (h *WSHandler) CommandStream(c *gin.Context) {
	commandID := c.Param("command_id")
	if commandID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command_id required"})
		return
	}

	ip := c.ClientIP()
	if !h.acquireConn(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many WebSocket connections from this IP"})
		return
	}
	defer h.releaseConn(ip)

	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() {
		h.streamHub.Unregister(commandID, conn)
		_ = conn.Close()
	}()

	if !h.authenticateWS(conn) {
		return
	}

	// Register this connection to receive streaming output
	h.streamHub.Register(commandID, conn)

	// Send initial status from unified remote_commands table (UUID — no ParseInt needed)
	if cmd, err := h.db.GetRemoteCommandByID(commandID); err == nil {
		_ = conn.WriteJSON(gin.H{
			"type":       "cmd_stream_init",
			"command_id": commandID,
			"status":     cmd.Status,
			"command":    cmd.Action,
			"output":     cmd.Output,
		})
	}

	// Keep connection alive until client disconnects
	done := make(chan struct{})
	go h.readLoop(conn, done)
	<-done
}

// NotificationStream is a persistent WebSocket connection that receives real-time
// alert notification events pushed by the alert engine when a new incident fires.
func (h *WSHandler) NotificationStream(c *gin.Context) {
	ip := c.ClientIP()
	if !h.acquireConn(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many WebSocket connections from this IP"})
		return
	}
	defer h.releaseConn(ip)

	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() {
		h.notifHub.Unregister(conn)
		_ = conn.Close()
	}()

	if !h.authenticateWS(conn) {
		return
	}

	// Acknowledge auth so the client transitions from 'connecting' to 'connected'
	// immediately, without waiting for the first real notification.
	if err := conn.WriteJSON(gin.H{"type": "auth_ok"}); err != nil {
		return
	}

	h.notifHub.Register(conn)

	// Keep alive until client disconnects.
	// NOTE: do NOT add a ping ticker here — Broadcast() already writes to this conn
	// from the alert-engine goroutine; a second goroutine writing here would race.
	done := make(chan struct{})
	go h.readLoop(conn, done)
	<-done
}

func (h *WSHandler) readLoop(conn *websocket.Conn, done chan struct{}) {
	defer close(done)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *WSHandler) validateToken(tokenString string) bool {
	if tokenString == "" {
		return false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.cfg.JWTSecret), nil
	})
	return err == nil && token.Valid
}

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

	payload := gin.H{
		"type":                "host_detail",
		"host":                host,
		"metrics":             metrics,
		"containers":          containers,
		"apt_status":          aptStatus,
		"apt_history":         aptHistory,
		"audit_logs":          auditLogs,
		"version_comparisons": comparisons,
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

	// Get network topology config
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

func (h *WSHandler) buildVersionComparisons() ([]models.VersionComparison, error) {
	trackers, err := h.db.ListReleaseTrackers()
	if err != nil {
		return nil, err
	}

	containers, err := h.db.GetAllDockerContainers()
	if err != nil {
		return nil, err
	}

	// Load historical digest→tag map: key = "trackerID|digest", value = tag
	digestTagMap, _ := h.db.GetAllTrackerTagDigests()
	if digestTagMap == nil {
		digestTagMap = make(map[string]string)
	}

	var comparisons []models.VersionComparison
	for _, tracker := range trackers {
		if tracker.DockerImage == "" || tracker.LastReleaseTag == "" {
			continue
		}
		releaseURL := ""
		if tracker.LastExecution != nil {
			releaseURL = tracker.LastExecution.ReleaseURL
		}

		matched := false
		for _, container := range containers {
			if container.HostID != tracker.HostID {
				continue
			}
			if container.Image != tracker.DockerImage && container.Image+":"+container.ImageTag != tracker.DockerImage {
				continue
			}
			nd := normalizeDigest(container.ImageDigest)
			ld := normalizeDigest(tracker.LatestImageDigest)

			// isVersionUpToDate uses the raw image tag so the "latest" fallback still works
			isUpToDate := isVersionUpToDate(container.ImageTag, container.ImageDigest, tracker.LastReleaseTag, tracker.LatestImageDigest)

			// UpdateConfirmed = digest comparison was performed and confirms the image is outdated
			updateConfirmed := !isUpToDate && nd != "" && ld != ""

			// Display version: try OCI labels first, then exact digest match, then historical lookup
			runningVersion := resolveContainerVersion(container.ImageTag, container.Labels)
			if runningVersion == "latest" && nd != "" {
				if nd == ld {
					// Running image IS the latest tracked release
					runningVersion = tracker.LastReleaseTag
				} else if historicTag, ok := digestTagMap[tracker.ID+"|"+nd]; ok {
					// Running image matches a previously tracked release
					runningVersion = historicTag
				}
			}
			// Still "latest" = version inconnue pour l'affichage
			if runningVersion == "latest" {
				runningVersion = ""
			}

			comparisons = append(comparisons, models.VersionComparison{
				DockerImage:     tracker.DockerImage,
				RunningVersion:  runningVersion,
				LatestVersion:   tracker.LastReleaseTag,
				IsUpToDate:      isUpToDate,
				UpdateConfirmed: updateConfirmed,
				RepoOwner:       tracker.RepoOwner,
				RepoName:        tracker.RepoName,
				ReleaseURL:      releaseURL,
				HostID:          tracker.HostID,
				Hostname:        tracker.HostName,
			})
			matched = true
		}

		// Show tracker even when no running container matches (image name mismatch or container stopped)
		if !matched {
			comparisons = append(comparisons, models.VersionComparison{
				DockerImage:   tracker.DockerImage,
				LatestVersion: tracker.LastReleaseTag,
				IsUpToDate:    false,
				RepoOwner:     tracker.RepoOwner,
				RepoName:      tracker.RepoName,
				ReleaseURL:    releaseURL,
				HostID:        tracker.HostID,
				Hostname:      tracker.HostName,
			})
		}
	}

	return comparisons, nil
}

// normalizeDigest strips the "sha256:" prefix for consistent comparison.
// The agent reports "sha256:abc..." while the tracker stores "abc..." (after CutPrefix).
func normalizeDigest(d string) string {
	return strings.TrimPrefix(d, "sha256:")
}

// isVersionUpToDate compares versions using digest when available, falling back to tag comparison.
func isVersionUpToDate(runningTag, runningDigest, latestTag, latestDigest string) bool {
	// Digest comparison: most reliable (works even with :latest tag)
	nd := normalizeDigest(runningDigest)
	ld := normalizeDigest(latestDigest)
	if nd != "" && ld != "" {
		return nd == ld
	}
	// Tag fallback: skip if running tag is "latest" (unreliable without digest)
	if runningTag == "latest" {
		return false
	}
	return normalizeVersion(runningTag) == normalizeVersion(latestTag)
}

func normalizeVersion(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}

// resolveContainerVersion returns the best available version string for display.
// When the image tag is "latest", checks OCI/schema.org labels for the real version.
func resolveContainerVersion(imageTag string, labels map[string]string) string {
	if imageTag != "latest" {
		return imageTag
	}
	for _, key := range []string{
		"org.opencontainers.image.version",
		"org.label-schema.version",
		"version",
	} {
		if v := labels[key]; v != "" {
			return v
		}
	}
	return imageTag
}
