package ws

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/cookies"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/events"
	"github.com/serversupervisor/server/internal/models"
)

// snapshotChanged returns true (and updates *lastHash) when payload differs from the
// previously sent snapshot. It allows the caller to skip WriteJSON when nothing changed.
func snapshotChanged(payload any, lastHash *string) bool {
	raw, err := json.Marshal(payload)
	if err != nil {
		return true
	}
	sum := sha256.Sum256(raw)
	hash := fmt.Sprintf("%x", sum)
	if hash == *lastHash {
		return false
	}
	*lastHash = hash
	return true
}

const wsMaxConnsPerIP = 20

const (
	wsPingInterval = 30 * time.Second
	wsPongWait     = 60 * time.Second
	// dashboardCacheTTL is how long a freshly built dashboard snapshot is reused
	// across all connected clients before being recomputed. It exists to collapse
	// the N concurrent rebuilds that happen when many dashboards react to the same
	// write event (or connect at once) into a single ~8-query build. It is kept
	// short so an event-driven refresh stays effectively instant.
	dashboardCacheTTL = 1 * time.Second
	// snapshotDebounce coalesces a burst of write events (e.g. one agent report
	// fans out to several topics, or several agents report at once) into a single
	// snapshot rebuild per connection.
	snapshotDebounce = 750 * time.Millisecond
	// snapshotSafetyInterval is the slow backstop poll. Updates are normally pushed
	// the instant a writer publishes the relevant topic; this periodic rebuild only
	// exists to self-heal any write path that forgot to publish and to refresh
	// values that change without a write event. It is deliberately long because the
	// event bus carries the common case.
	snapshotSafetyInterval = 60 * time.Second
)

type WSHandler struct {
	db        *database.DB
	cfg       *config.Config
	streamHub *CommandStreamHub
	notifHub  *NotificationHub
	events    *events.Bus
	ipConns   map[string]int
	ipConnsMu sync.Mutex

	// Shared, short-lived cache of the dashboard snapshot payload. The payload is
	// identical for every client (no per-user filtering), so it is computed once
	// per TTL window regardless of how many dashboards are open.
	dashCacheMu sync.Mutex
	dashCache   *models.WSDashboardSnapshot
	dashCacheAt time.Time
}

type wsAuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

func NewWSHandler(db *database.DB, cfg *config.Config, notifHub *NotificationHub, bus *events.Bus) *WSHandler {
	return &WSHandler{
		db:        db,
		cfg:       cfg,
		streamHub: NewCommandStreamHub(),
		notifHub:  notifHub,
		events:    bus,
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

func (h *WSHandler) GetStreamHub() *CommandStreamHub {
	return h.streamHub
}

func (h *WSHandler) GetNotificationHub() *NotificationHub {
	return h.notifHub
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
		slog.Warn("[WS] rejected origin (parse error)", slog.String("origin", origin))
		return false
	}

	hostname := strings.ToLower(parsedOrigin.Hostname())
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return true
	}

	parsedBase, err := url.Parse(baseURL)
	if err == nil {
		if parsedOrigin.Host == parsedBase.Host && parsedOrigin.Scheme == parsedBase.Scheme {
			return true
		}
		if parsedOrigin.Host == parsedBase.Host {
			slog.Warn("[WS] allowed origin with scheme mismatch (update BASE_URL scheme if needed)", slog.String("origin", origin), slog.String("base_url", baseURL))
			return true
		}
	}

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
		if parsedOrigin.Host == parsedAllowed.Host {
			slog.Warn("[WS] allowed origin with scheme mismatch", slog.String("origin", origin), slog.String("allowed", allowed))
			return true
		}
	}

	slog.Warn("[WS] rejected origin (set BASE_URL or ALLOWED_ORIGINS correctly)", slog.String("origin", origin), slog.String("base_url", baseURL))
	return false
}

func (h *WSHandler) authenticateWS(c *gin.Context, conn *websocket.Conn) bool {
	ok, _ := h.authenticateWSWithRole(c, conn)
	return ok
}

func (h *WSHandler) authenticateWSWithRole(c *gin.Context, conn *websocket.Conn) (bool, string) {
	claims, ok := h.authenticateWSClaims(c, conn)
	if !ok {
		return false, ""
	}
	role, _ := claims["role"].(string)
	return true, role
}

// authenticateWSClaims authorises the connection. It prefers the session
// cookie (sent automatically by the browser on the upgrade request) and falls
// back to the legacy in-band {"type":"auth","token":"…"} handshake for older
// clients. Returns the JWT claims on success.
func (h *WSHandler) authenticateWSClaims(c *gin.Context, conn *websocket.Conn) (jwt.MapClaims, bool) {
	if c != nil && c.Request != nil {
		if tok := cookies.ReadAccessToken(c.Request); tok != "" {
			if claims, ok := h.parseTokenClaims(tok); ok {
				return claims, true
			}
		}
		if tok := c.Query("token"); tok != "" {
			if claims, ok := h.parseTokenClaims(tok); ok {
				return claims, true
			}
		}
	}

	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msg wsAuthMessage
	if err := conn.ReadJSON(&msg); err != nil {
		_ = conn.WriteJSON(gin.H{"type": "auth_error", "error": "missing auth"})
		return nil, false
	}
	if msg.Type != "auth" || msg.Token == "" {
		_ = conn.WriteJSON(gin.H{"type": "auth_error", "error": "invalid auth"})
		return nil, false
	}
	claims, ok := h.parseTokenClaims(msg.Token)
	if !ok {
		_ = conn.WriteJSON(gin.H{"type": "auth_error", "error": "unauthorized"})
		return nil, false
	}
	_ = conn.SetReadDeadline(time.Time{})
	return claims, true
}

func (h *WSHandler) validateToken(tokenString string) bool {
	_, ok := h.parseTokenClaims(tokenString)
	return ok
}

func (h *WSHandler) parseTokenClaims(tokenString string) (jwt.MapClaims, bool) {
	if tokenString == "" {
		return nil, false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(h.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

func (h *WSHandler) readLoop(conn *websocket.Conn, done chan struct{}) {
	defer close(done)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
