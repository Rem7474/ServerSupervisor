package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

// ProxmoxHandler manages Proxmox connections, exposes read-only data,
// and runs the background polling loop.
type ProxmoxHandler struct {
	db   *database.DB
	cfg  *config.Config
	svc  *proxmoxService
	stop chan struct{}
}

func NewProxmoxHandler(db *database.DB, cfg *config.Config) *ProxmoxHandler {
	return &ProxmoxHandler{
		db:   db,
		cfg:  cfg,
		svc:  newProxmoxService(db, cfg),
		stop: make(chan struct{}),
	}
}

// ─── Poller ───────────────────────────────────────────────────────────────────

// StartPoller begins periodic collection for all enabled Proxmox connections.
// It runs an immediate first pass, then repeats at the minimum configured interval.
func (h *ProxmoxHandler) StartPoller() {
	go h.pollAll() // immediate first pass

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				h.pollAll()
			case <-h.stop:
				ticker.Stop()
				return
			}
		}
	}()
	log.Println("Proxmox poller started (tick: 30s, respects per-connection poll_interval_sec)")
}

func (h *ProxmoxHandler) StopPoller() {
	close(h.stop)
}

// pollAll iterates all enabled connections and polls each one.
func (h *ProxmoxHandler) pollAll() {
	h.svc.PollAll()
}

func (h *ProxmoxHandler) pollOne(conn database.ProxmoxConnectionFull) {
	h.svc.PollOne(conn)
}

// ─── CRUD: Connections ────────────────────────────────────────────────────────

// ListConnections returns all connections (no secrets).
func (h *ProxmoxHandler) ListConnections(c *gin.Context) {
	conns, err := h.db.ListProxmoxConnections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, conns)
}

// CreateConnection adds a new Proxmox connection.
func (h *ProxmoxHandler) CreateConnection(c *gin.Context) {
	var req models.ProxmoxConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TokenSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token_secret is required when creating a connection"})
		return
	}

	id, err := h.db.CreateProxmoxConnection(
		req.Name, req.APIURL, req.TokenID, req.TokenSecret,
		req.InsecureSkipVerify, req.Enabled, req.PollIntervalSec,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	conn, _ := h.db.GetProxmoxConnectionByID(id)
	c.JSON(http.StatusCreated, conn)
}

// GetConnection returns one connection (no secret).
func (h *ProxmoxHandler) GetConnection(c *gin.Context) {
	conn, err := h.db.GetProxmoxConnectionByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	c.JSON(http.StatusOK, conn)
}

// UpdateConnection updates a connection. Empty token_secret keeps the existing one.
func (h *ProxmoxHandler) UpdateConnection(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxConnectionByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}

	var req models.ProxmoxConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.db.UpdateProxmoxConnection(
		id, req.Name, req.APIURL, req.TokenID, req.TokenSecret,
		req.InsecureSkipVerify, req.Enabled, req.PollIntervalSec,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	conn, _ := h.db.GetProxmoxConnectionByID(id)
	c.JSON(http.StatusOK, conn)
}

// DeleteConnection removes a connection (and cascade-deletes its nodes/guests/storages).
func (h *ProxmoxHandler) DeleteConnection(c *gin.Context) {
	id := c.Param("id")
	existing, err := h.db.GetProxmoxConnectionByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	if err := h.db.DeleteProxmoxConnection(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "connection deleted"})
}

// TestConnection tests connectivity and token validity without saving anything.
func (h *ProxmoxHandler) TestConnection(c *gin.Context) {
	var req struct {
		APIURL             string `json:"api_url" binding:"required"`
		TokenID            string `json:"token_id" binding:"required"`
		TokenSecret        string `json:"token_secret" binding:"required"`
		InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client := proxmoxclient.New(req.APIURL, req.TokenID, req.TokenSecret, req.InsecureSkipVerify)
	if err := client.TestConnection(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestConnectionByID tests an existing saved connection (uses stored secret).
func (h *ProxmoxHandler) TestConnectionByID(c *gin.Context) {
	id := c.Param("id")
	conn, err := h.db.GetProxmoxConnectionByID(id)
	if err != nil || conn == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}
	secret, err := h.db.GetProxmoxTokenSecret(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	if err := client.TestConnection(); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// PollNow triggers an immediate poll for one connection.
func (h *ProxmoxHandler) PollNow(c *gin.Context) {
	id := c.Param("id")
	conns, err := h.db.GetEnabledProxmoxConnections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, conn := range conns {
		if conn.ID == id {
			go h.pollOne(conn)
			c.JSON(http.StatusOK, gin.H{"message": "poll triggered"})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "enabled connection not found"})
}

// ─── Read-only data endpoints ─────────────────────────────────────────────────

// GetSummary returns aggregate stats (connection/node/guest/storage counts).
func (h *ProxmoxHandler) GetSummary(c *gin.Context) {
	summary, err := h.db.GetProxmoxSummary()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}
