package ws

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/models"
)

func (h *WSHandler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	h.servePollingSnapshot(c, true, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendDashboardSnapshot(ctx, conn, lastHash)
	})
}

func (h *WSHandler) HostDetail(c *gin.Context) {
	hostID := c.Param("id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host id required"})
		return
	}

	ctx := c.Request.Context()
	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendHostSnapshot(ctx, conn, hostID, lastHash)
	})
}

func (h *WSHandler) Docker(c *gin.Context) {
	ctx := c.Request.Context()
	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendDockerSnapshot(ctx, conn, lastHash)
	})
}

func (h *WSHandler) Network(c *gin.Context) {
	ctx := c.Request.Context()
	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendNetworkSnapshot(ctx, conn, lastHash)
	})
}

func (h *WSHandler) Apt(c *gin.Context) {
	ctx := c.Request.Context()
	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendAptSnapshot(ctx, conn, lastHash)
	})
}

func (h *WSHandler) servePollingSnapshot(c *gin.Context, enforceIPLimit bool, sendSnapshot func(*websocket.Conn, *string) error) {
	if enforceIPLimit {
		ip := c.ClientIP()
		if !h.acquireConn(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many WebSocket connections from this IP"})
			return
		}
		defer h.releaseConn(ip)
	}

	conn, err := h.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() {
		releaseWriteGuard(conn)
		_ = conn.Close()
	}()

	if !h.authenticateWS(c, conn) {
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
	if err := sendSnapshot(conn, &lastHash); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-dataTicker.C:
			if err := sendSnapshot(conn, &lastHash); err != nil {
				return
			}
		case <-pingTicker.C:
			if err := safeWriteMessage(conn, websocket.PingMessage, nil); err != nil {
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
		releaseWriteGuard(conn)
		_ = conn.Close()
	}()

	claims, ok := h.authenticateWSClaims(c, conn)
	if !ok {
		return
	}

	// Fetch the command to verify existence and, for non-admins, host ownership.
	cmd, err := h.db.GetRemoteCommandByID(c.Request.Context(), commandID)
	if err != nil {
		_ = safeWriteJSON(conn, gin.H{"type": "error", "error": "command not found"})
		return
	}

	role, _ := claims["role"].(string)
	if role != "admin" {
		username, _ := claims["sub"].(string)
		restricted, level, accessErr := h.db.GetHostAccess(c.Request.Context(), username, cmd.HostID)
		if accessErr != nil || (restricted && level == "") {
			_ = safeWriteJSON(conn, gin.H{"type": "auth_error", "error": "access denied"})
			return
		}
	}

	h.streamHub.Register(commandID, conn)

	// For active commands, prefer the in-memory buffer which contains all chunks
	// broadcast since the command started — the DB output column is only written
	// on completion, so it is empty while the command is running.
	initOutput := cmd.Output
	if cmd.Status == "running" || cmd.Status == "pending" {
		if buffered := h.streamHub.GetBufferedOutput(commandID); buffered != "" {
			initOutput = buffered
		}
	}

	_ = safeWriteJSON(conn, models.WSCommandStreamInit{
		Type:      "cmd_stream_init",
		CommandID: commandID,
		Status:    cmd.Status,
		Command:   cmd.Action,
		Output:    initOutput,
	})

	done := make(chan struct{})
	go h.readLoop(conn, done)
	<-done
}

// NotificationStream is a persistent WebSocket connection that receives real-time
// alert notification events pushed by the alert engine when a new incident fires.
// It includes ping/pong heartbeat to detect stale connections.
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
		releaseWriteGuard(conn)
		_ = conn.Close()
	}()

	ok, role := h.authenticateWSWithRole(c, conn)
	if !ok {
		return
	}
	if role != "admin" {
		_ = safeWriteJSON(conn, gin.H{"type": "auth_error", "error": "forbidden"})
		return
	}

	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	if err := safeWriteJSON(conn, gin.H{"type": "auth_ok"}); err != nil {
		return
	}

	h.notifHub.Register(conn)

	pingTicker := time.NewTicker(wsPingInterval)
	defer pingTicker.Stop()

	done := make(chan struct{})
	go h.readLoop(conn, done)

	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			if err := safeWriteMessage(conn, websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
