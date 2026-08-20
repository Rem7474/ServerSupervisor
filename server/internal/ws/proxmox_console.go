package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/safego"
)

// consolePingInterval is how often the PVE side of a console session is
// pinged. PVE's termproxy closes an idle session after a few minutes without
// one — see proxmoxclient.TermSession.Ping's doc comment.
const consolePingInterval = 60 * time.Second

// ProxmoxConsole proxies an interactive LXC shell (xterm.js on the browser
// side, PVE termproxy on the other) over a single WebSocket. Unlike
// CommandStream this is bidirectional: keystroke/paste bytes from the
// browser go straight to the PTY, and PVE's raw output goes straight back —
// see models.WSConsoleResize's doc comment for the one exception (resize).
//
// Admin-only, same posture as GuestAction (see the root CLAUDE.md's "Proxmox
// integration" note): a console session is a materially bigger blast radius
// than the whitelisted start/shutdown/reboot actions. Checked after the
// authenticateWSClaims call (not before the WS upgrade) to stay consistent
// with every other role check in this package (e.g. CommandStream) — claims
// aren't available pre-upgrade for a client using the in-band auth fallback,
// so there is no uniform way to check earlier.
func (h *WSHandler) ProxmoxConsole(c *gin.Context) {
	guestID := c.Param("guest_id")
	if guestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "guest_id required"})
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
		releaseWriteGuard(conn)
		_ = conn.Close()
	}()

	claims, ok := h.authenticateWSClaims(c, conn)
	if !ok {
		return
	}
	if role, _ := claims["role"].(string); role != models.RoleAdmin {
		_ = safeWriteJSON(conn, models.WSConsoleError{Type: "console_error", Error: "admin only"})
		return
	}
	username, _ := claims["sub"].(string)

	// c.Request.Context() is cancelled the moment this handler returns, but
	// the console session's lifetime is the WebSocket's, which outlives the
	// synchronous part of this handler — a background context is correct
	// here, same reasoning as CreateAuditLog's use of one below.
	ctx := context.Background()
	guest, session, err := h.proxmoxSvc.OpenGuestConsole(ctx, guestID)
	if err != nil {
		_ = safeWriteJSON(conn, models.WSConsoleError{Type: "console_error", Error: err.Error()})
		return
	}
	defer func() { _ = session.Close() }()

	details := fmt.Sprintf("node=%s vmid=%d guest_id=%s", guest.NodeName, guest.VMID, guestID)
	if _, err := h.db.CreateAuditLog(ctx, username, "proxmox_console_open", "", ip, details, "success"); err != nil {
		slog.Error("proxmox console: audit log failed", slog.Any("err", err))
	}

	done := make(chan struct{})
	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			close(done)
			_ = conn.Close()
			_ = session.Close()
		})
	}

	// PVE -> browser: raw PTY output, passed straight through as binary
	// frames. This must not be a text frame: PTY output is arbitrary bytes,
	// not guaranteed valid UTF-8 (binary command output, or a multi-byte
	// UTF-8 character split across two reads), and a browser is required by
	// the WebSocket spec to fail the connection on an invalid-UTF-8 text
	// frame — xterm.js's own guidance is binary frames for exactly this
	// reason. The frontend writes these bytes straight into xterm.js as a
	// Uint8Array, which has its own incremental UTF-8 decoder that handles a
	// split character correctly across writes (see useProxmoxConsole.ts).
	safego.Go(ctx, "ws.proxmoxConsole.pveToBrowser", func() {
		defer closeAll()
		for {
			out, err := session.ReadMessage()
			if err != nil {
				return
			}
			if err := safeWriteMessage(conn, websocket.BinaryMessage, out); err != nil {
				return
			}
		}
	})

	// browser -> PVE: keystroke/paste bytes forwarded as-is; a resize control
	// message (models.WSConsoleResize) is translated to PVE's own framing
	// instead of being forwarded verbatim.
	safego.Go(ctx, "ws.proxmoxConsole.browserToPVE", func() {
		defer closeAll()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var resize models.WSConsoleResize
			if json.Unmarshal(msg, &resize) == nil && resize.Type == "resize" {
				if err := session.Resize(resize.Cols, resize.Rows); err != nil {
					return
				}
				continue
			}
			if err := session.Write(msg); err != nil {
				return
			}
		}
	})

	pingTicker := time.NewTicker(consolePingInterval)
	defer pingTicker.Stop()
	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			if err := session.Ping(); err != nil {
				closeAll()
				return
			}
		}
	}
}
