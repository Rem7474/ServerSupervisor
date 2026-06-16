package ws

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
)

func (h *WSHandler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	h.serveSnapshot(c, true, []string{events.TopicDashboard}, func(conn *websocket.Conn, lastHash *string) error {
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
	h.serveSnapshot(c, false, []string{events.HostTopic(hostID)}, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendHostSnapshot(ctx, conn, hostID, lastHash)
	})
}

func (h *WSHandler) Docker(c *gin.Context) {
	ctx := c.Request.Context()
	h.serveSnapshot(c, false, []string{events.TopicDocker}, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendDockerSnapshot(ctx, conn, lastHash)
	})
}

func (h *WSHandler) Network(c *gin.Context) {
	ctx := c.Request.Context()
	h.serveSnapshot(c, false, []string{events.TopicNetwork}, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendNetworkSnapshot(ctx, conn, lastHash)
	})
}

func (h *WSHandler) Apt(c *gin.Context) {
	ctx := c.Request.Context()
	h.serveSnapshot(c, false, []string{events.TopicApt}, func(conn *websocket.Conn, lastHash *string) error {
		return h.sendAptSnapshot(ctx, conn, lastHash)
	})
}

// serveSnapshot upgrades the connection then pushes the snapshot event-driven:
// it rebuilds and sends whenever a writer publishes one of `topics` (debounced to
// coalesce bursts), with a slow safety-net rebuild as a backstop. The diff-hash in
// each sendSnapshot still suppresses redundant writes when nothing actually changed.
func (h *WSHandler) serveSnapshot(c *gin.Context, enforceIPLimit bool, topics []string, sendSnapshot func(*websocket.Conn, *string) error) {
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

	signal, unsubscribe := h.events.Subscribe(topics...)
	defer unsubscribe()

	safetyTicker := time.NewTicker(snapshotSafetyInterval)
	pingTicker := time.NewTicker(wsPingInterval)
	defer safetyTicker.Stop()
	defer pingTicker.Stop()

	var lastHash string
	if err := sendSnapshot(conn, &lastHash); err != nil {
		return
	}

	done := make(chan struct{})
	go h.readLoop(conn, done)

	// debounce coalesces a burst of signals into one rebuild. It is armed on the
	// first signal and disarmed when it fires; debounceC is nil while disarmed so
	// the select arm stays dormant.
	var debounce *time.Timer
	var debounceC <-chan time.Time
	arm := func() {
		if debounce == nil {
			debounce = time.NewTimer(snapshotDebounce)
			debounceC = debounce.C
		}
	}
	defer func() {
		if debounce != nil {
			debounce.Stop()
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-signal:
			arm()
		case <-debounceC:
			debounce = nil
			debounceC = nil
			if err := sendSnapshot(conn, &lastHash); err != nil {
				return
			}
		case <-safetyTicker.C:
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
