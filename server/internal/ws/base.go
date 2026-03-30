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
)

// snapshotChanged returns true (and updates *lastHash) when payload differs from the
// previously sent snapshot. It allows the caller to skip WriteJSON when nothing changed.
func snapshotChanged(payload gin.H, lastHash *string) bool {
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
)

type WSHandler struct {
	db        *database.DB
	cfg       *config.Config
	streamHub *CommandStreamHub
	notifHub  *NotificationHub
	ipConns   map[string]int
	ipConnsMu sync.Mutex
}

type wsAuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
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
		log.Printf("[WS] rejected origin (parse error): %q", origin)
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
			log.Printf("[WS] allowed origin with scheme mismatch: origin=%q base=%q (update BASE_URL scheme if needed)", origin, baseURL)
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
			log.Printf("[WS] allowed origin with scheme mismatch: origin=%q allowed=%q", origin, allowed)
			return true
		}
	}

	log.Printf("[WS] rejected origin: %q (BASE_URL=%q) - set BASE_URL or ALLOWED_ORIGINS correctly", origin, baseURL)
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

func (h *WSHandler) readLoop(conn *websocket.Conn, done chan struct{}) {
	defer close(done)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
