package ws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/rfb"
	"github.com/serversupervisor/server/internal/safego"
)

// Close codes the VNC console uses to report why a session could not be
// opened. The browser leg speaks raw RFB, so unlike ProxmoxConsole there is
// no room for a JSON error frame: the reason travels in the WebSocket close
// frame instead (4000-4999 is the range RFC 6455 leaves to applications).
const (
	vncCloseBadRequest = 4400
	vncCloseForbidden  = 4403
	vncCloseNotFound   = 4404
	vncCloseInternal   = 4500
	vncCloseBadGateway = 4502
)

// maxCloseReasonLen is RFC 6455's limit on a close frame's reason (125-byte
// control payload minus the 2-byte code).
const maxCloseReasonLen = 123

// vncHandshakeTimeout bounds the browser-side RFB handshake.
const vncHandshakeTimeout = 10 * time.Second

// ProxmoxVNC proxies a QEMU VM's display (noVNC in the browser, PVE's
// vncwebsocket on the other side) over a single WebSocket. Same security
// model as ProxmoxConsole: admin only, audit row on open, PVE ticket obtained
// server-side right before the session and never sent to the browser.
//
// The proxy terminates RFB authentication: it answers PVE's VNC challenge
// with the one-time password from vncproxy and offers the browser the "None"
// security type (see package rfb). After both handshakes the RFB stream is
// relayed byte for byte.
func (h *WSHandler) ProxmoxVNC(c *gin.Context) {
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
	username, _ := claims["sub"].(string)
	if role, _ := claims["role"].(string); role != models.RoleAdmin {
		closeVNCWithError(conn, apperr.Forbidden("admin only").I18n(apperr.CodeProxmoxConsoleAdminOnly, nil))
		return
	}

	// Background context: the session outlives the synchronous part of this
	// handler (see ProxmoxConsole).
	ctx := context.Background()
	guest, session, err := h.proxmoxSvc.OpenGuestVNC(ctx, guestID)
	if err != nil {
		if guest != nil {
			h.auditVNC(ctx, username, ip, guest, guestID, err)
		}
		// guest_id comes from the URL: strip line breaks before logging it.
		safeGuestID := strings.NewReplacer("\n", "", "\r", "").Replace(guestID)
		slog.Warn("proxmox vnc: open failed", slog.String("guest_id", safeGuestID), slog.Any("err", err))
		closeVNCWithError(conn, err)
		return
	}
	defer func() { _ = session.Close() }()
	h.auditVNC(ctx, username, ip, guest, guestID, nil)

	browser := rfb.NewWSStream(conn)
	_ = conn.SetReadDeadline(time.Now().Add(vncHandshakeTimeout))
	if err := rfb.ServerHandshakeNoAuth(browser); err != nil {
		closeVNCWithError(conn, apperr.Validation("vnc handshake: "+err.Error()))
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	done := make(chan struct{})
	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			close(done)
			_ = conn.Close()
			_ = session.Close()
		})
	}

	// PVE -> browser: framebuffer updates and server messages, binary frames.
	safego.Go(ctx, "ws.proxmoxVNC.pveToBrowser", func() {
		defer closeAll()
		for {
			out, err := session.ReadMessage()
			if err != nil {
				return
			}
			if err := browser.WriteMessage(out); err != nil {
				return
			}
		}
	})

	// browser -> PVE: ClientInit then input events, starting with any bytes
	// the handshake read ahead (WSStream.ReadMessage returns those first).
	safego.Go(ctx, "ws.proxmoxVNC.browserToPVE", func() {
		defer closeAll()
		for {
			msg, err := browser.ReadMessage()
			if err != nil {
				return
			}
			if err := session.Write(msg); err != nil {
				return
			}
		}
	})

	<-done
}

// auditVNC records a VNC console open attempt. A failure on a resolved guest
// is recorded too: an admin repeatedly trying to reach a VM's display is
// worth seeing even when PVE refuses it.
func (h *WSHandler) auditVNC(ctx context.Context, username, ip string, guest *models.ProxmoxGuest, guestID string, openErr error) {
	details := fmt.Sprintf("node=%s vmid=%d guest_id=%s", guest.NodeName, guest.VMID, guestID)
	status := "success"
	if openErr != nil {
		status = "failure"
	}
	if _, err := h.db.CreateAuditLog(ctx, username, "proxmox_vnc_open", "", ip, details, status); err != nil {
		slog.Error("proxmox vnc: audit log failed", slog.Any("err", err))
	}
}

// closeVNCWithError ends the WebSocket with a close code derived from err's
// HTTP status and, as reason, its catalog key when it has one (the frontend
// translates it through errors.json) or its message, truncated to fit.
func closeVNCWithError(conn *websocket.Conn, err error) {
	code, reason := vncCloseFor(err)
	msg := websocket.FormatCloseMessage(code, reason)
	_ = conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
}

func vncCloseFor(err error) (int, string) {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		return vncCloseInternal, truncateCloseReason(err.Error())
	}
	code := vncCloseInternal
	switch ae.HTTPStatus {
	case http.StatusBadRequest:
		code = vncCloseBadRequest
	case http.StatusForbidden:
		code = vncCloseForbidden
	case http.StatusNotFound:
		code = vncCloseNotFound
	case http.StatusBadGateway:
		code = vncCloseBadGateway
	}
	if ae.I18nKey != "" {
		return code, ae.I18nKey
	}
	return code, truncateCloseReason(ae.Message)
}

// truncateCloseReason cuts s to maxCloseReasonLen bytes without splitting a
// UTF-8 character (a close reason must be valid UTF-8).
func truncateCloseReason(s string) string {
	if len(s) <= maxCloseReasonLen {
		return s
	}
	s = s[:maxCloseReasonLen]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
