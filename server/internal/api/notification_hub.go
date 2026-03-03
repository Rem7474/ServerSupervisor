package api

import (
	"sync"

	"github.com/gorilla/websocket"
)

// NotificationHub broadcasts real-time alert notification events to all connected frontend clients.
// It is registered as the single global push channel for browser notifications.
type NotificationHub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func NewNotificationHub() *NotificationHub {
	return &NotificationHub{clients: make(map[*websocket.Conn]struct{})}
}

func (h *NotificationHub) Register(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()
}

func (h *NotificationHub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

// Broadcast sends a JSON payload to all registered clients.
// Failed connections are silently dropped.
func (h *NotificationHub) Broadcast(payload interface{}) {
	h.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.clients))
	for conn := range h.clients {
		conns = append(conns, conn)
	}
	h.mu.Unlock()

	for _, conn := range conns {
		if err := conn.WriteJSON(payload); err != nil {
			_ = conn.Close()
			h.Unregister(conn)
		}
	}
}
