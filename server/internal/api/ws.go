package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type WSHandler struct {
	db  *database.DB
	cfg *config.Config
}

func NewWSHandler(db *database.DB, cfg *config.Config) *WSHandler {
	return &WSHandler{db: db, cfg: cfg}
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *WSHandler) Dashboard(c *gin.Context) {
	if !h.validateWSToken(c) {
		return
	}
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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
	if !h.validateWSToken(c) {
		return
	}
	hostID := c.Param("id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host id required"})
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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
	if !h.validateWSToken(c) {
		return
	}
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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

func (h *WSHandler) Apt(c *gin.Context) {
	if !h.validateWSToken(c) {
		return
	}
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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

func (h *WSHandler) readLoop(conn *websocket.Conn, done chan struct{}) {
	defer close(done)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *WSHandler) validateWSToken(c *gin.Context) bool {
	tokenString := c.Query("token")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return false
	}
	return true
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
	aptHistory, _ := h.db.GetAptCommandHistory(hostID, 50)
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
