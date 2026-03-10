package ws

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func (h *WSHandler) Dashboard(c *gin.Context) {
	h.servePollingSnapshot(c, true, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendDashboardSnapshot(conn, lastHash)
	})
}

func (h *WSHandler) HostDetail(c *gin.Context) {
	hostID := c.Param("id")
	if hostID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host id required"})
		return
	}

	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendHostSnapshot(conn, hostID, lastHash)
	})
}

func (h *WSHandler) Docker(c *gin.Context) {
	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendDockerSnapshot(conn, lastHash)
	})
}

func (h *WSHandler) Network(c *gin.Context) {
	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendNetworkSnapshot(conn, lastHash)
	})
}

func (h *WSHandler) Apt(c *gin.Context) {
	h.servePollingSnapshot(c, false, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendAptSnapshot(conn, lastHash)
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

	h.streamHub.Register(commandID, conn)

	if cmd, err := h.db.GetRemoteCommandByID(commandID); err == nil {
		_ = conn.WriteJSON(gin.H{
			"type":       "cmd_stream_init",
			"command_id": commandID,
			"status":     cmd.Status,
			"command":    cmd.Action,
			"output":     cmd.Output,
		})
	}

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

	if err := conn.WriteJSON(gin.H{"type": "auth_ok"}); err != nil {
		return
	}

	h.notifHub.Register(conn)

	done := make(chan struct{})
	go h.readLoop(conn, done)
	<-done
}
