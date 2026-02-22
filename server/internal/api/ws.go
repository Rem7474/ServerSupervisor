package api

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type WSHandler struct {
	db        *database.DB
	cfg       *config.Config
	streamHub *AptStreamHub
}

func NewWSHandler(db *database.DB, cfg *config.Config) *WSHandler {
	return &WSHandler{
		db:        db,
		cfg:       cfg,
		streamHub: NewAptStreamHub(),
	}
}

// GetStreamHub returns the APT stream hub for use by other handlers
func (h *WSHandler) GetStreamHub() *AptStreamHub {
	return h.streamHub
}

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
	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if !h.authenticateWS(conn) {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	if err := h.sendDashboardSnapshot(conn); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := h.sendDashboardSnapshot(conn); err != nil {
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
	defer conn.Close()

	if !h.authenticateWS(conn) {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	if err := h.sendHostSnapshot(conn, hostID); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := h.sendHostSnapshot(conn, hostID); err != nil {
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
	defer conn.Close()

	if !h.authenticateWS(conn) {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	if err := h.sendDockerSnapshot(conn); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := h.sendDockerSnapshot(conn); err != nil {
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
	defer conn.Close()

	if !h.authenticateWS(conn) {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	if err := h.sendNetworkSnapshot(conn); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := h.sendNetworkSnapshot(conn); err != nil {
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
	defer conn.Close()

	if !h.authenticateWS(conn) {
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	if err := h.sendAptSnapshot(conn); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := h.sendAptSnapshot(conn); err != nil {
				return
			}
		}
	}
}

// AptStream allows clients to subscribe to real-time APT command output
func (h *WSHandler) AptStream(c *gin.Context) {
	commandID := c.Param("command_id")
	if commandID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command_id required"})
		return
	}

	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() {
		h.streamHub.Unregister(commandID, conn)
		conn.Close()
	}()

	if !h.authenticateWS(conn) {
		return
	}

	// Register this connection to receive streaming logs
	h.streamHub.Register(commandID, conn)

	// Send initial status (parse commandID as int64 for DB query)
	cmdID, parseErr := strconv.ParseInt(commandID, 10, 64)
	if parseErr == nil {
		cmd, dbErr := h.db.GetAptCommandByID(cmdID)
		if dbErr == nil {
			conn.WriteJSON(gin.H{
				"type":       "apt_stream_init",
				"command_id": commandID,
				"status":     cmd.Status,
				"command":    cmd.Command,
				"output":     cmd.Output, // Send existing output if any
			})
		}
	}

	// Keep connection alive until client disconnects
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

func (h *WSHandler) sendDashboardSnapshot(conn *websocket.Conn) error {
	hosts, err := h.db.GetAllHosts()
	if err != nil {
		return err
	}

	hostMetrics := map[string]*models.SystemMetrics{}
	for _, host := range hosts {
		metrics, err := h.db.GetLatestMetrics(host.ID)
		if err == nil {
			hostMetrics[host.ID] = metrics
		}
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
	}
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendHostSnapshot(conn *websocket.Conn, hostID string) error {
	host, err := h.db.GetHost(hostID)
	if err != nil {
		return err
	}
	metrics, _ := h.db.GetLatestMetrics(hostID)
	containers, _ := h.db.GetDockerContainers(hostID)
	aptStatus, _ := h.db.GetAptStatus(hostID)
	aptHistory, _ := h.db.GetAptHistoryWithAgentUpdates(hostID, 50)
	auditLogs, _ := h.db.GetAuditLogsByHost(hostID, 50)

	payload := gin.H{
		"type":        "host_detail",
		"host":        host,
		"metrics":     metrics,
		"containers":  containers,
		"apt_status":  aptStatus,
		"apt_history": aptHistory,
		"audit_logs":  auditLogs,
	}
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendDockerSnapshot(conn *websocket.Conn) error {
	containers, err := h.db.GetAllDockerContainers()
	if err != nil {
		return err
	}
	payload := gin.H{
		"type":       "docker",
		"containers": containers,
	}
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendNetworkSnapshot(conn *websocket.Conn) error {
	snapshot, err := buildNetworkSnapshot(h.db)
	if err != nil {
		return err
	}

	// Get Docker networks
	networks, _ := h.db.GetAllDockerNetworks()

	// Get network topology config
	config, _ := h.db.GetNetworkTopologyConfig()

	// Infer topology links
	links := inferTopologyLinks(h.db, snapshot.Containers, networks)

	payload := gin.H{
		"type":       "network",
		"hosts":      snapshot.Hosts,
		"containers": snapshot.Containers,
		"networks":   networks,
		"links":      links,
		"config":     config,
		"updated_at": snapshot.UpdatedAt,
	}
	return conn.WriteJSON(payload)
}

func (h *WSHandler) sendAptSnapshot(conn *websocket.Conn) error {
	hosts, err := h.db.GetAllHosts()
	if err != nil {
		return err
	}

	aptStatuses := map[string]*models.AptStatus{}
	aptHistories := map[string][]models.AptCommand{}

	for _, host := range hosts {
		status, err := h.db.GetAptStatus(host.ID)
		if err == nil {
			aptStatuses[host.ID] = status
		}
		hist, err := h.db.GetAptCommandHistory(host.ID, 20)
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
	return conn.WriteJSON(payload)
}

func (h *WSHandler) buildVersionComparisons() ([]models.VersionComparison, error) {
	repos, err := h.db.GetTrackedRepos()
	if err != nil {
		return nil, err
	}

	containers, err := h.db.GetAllDockerContainers()
	if err != nil {
		return nil, err
	}

	hosts, err := h.db.GetAllHosts()
	if err != nil {
		return nil, err
	}

	hostMap := make(map[string]string)
	for _, host := range hosts {
		hostMap[host.ID] = host.Hostname
	}

	var comparisons []models.VersionComparison
	for _, repo := range repos {
		if repo.DockerImage == "" {
			continue
		}
		for _, container := range containers {
			if container.Image == repo.DockerImage || container.Image+":"+container.ImageTag == repo.DockerImage {
				comparisons = append(comparisons, models.VersionComparison{
					DockerImage:    container.Image,
					RunningVersion: container.ImageTag,
					LatestVersion:  repo.LatestVersion,
					IsUpToDate:     normalizeVersion(container.ImageTag) == normalizeVersion(repo.LatestVersion),
					RepoOwner:      repo.Owner,
					RepoName:       repo.Repo,
					ReleaseURL:     repo.ReleaseURL,
					HostID:         container.HostID,
					Hostname:       hostMap[container.HostID],
				})
			}
		}
	}

	return comparisons, nil
}
