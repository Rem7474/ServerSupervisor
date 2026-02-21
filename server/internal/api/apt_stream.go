package api

import (
	"sync"

	"github.com/gorilla/websocket"
)

// AptStreamHub manages real-time streaming of APT command output
type AptStreamHub struct {
	clients    map[string]map[*websocket.Conn]bool // commandID -> set of websocket connections
	broadcasts map[string]chan string              // commandID -> broadcast channel
	mu         sync.RWMutex
}

// NewAptStreamHub creates a new streaming hub
func NewAptStreamHub() *AptStreamHub {
	return &AptStreamHub{
		clients:    make(map[string]map[*websocket.Conn]bool),
		broadcasts: make(map[string]chan string),
	}
}

// Register adds a websocket connection to receive logs for a specific command
func (h *AptStreamHub) Register(commandID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[commandID] == nil {
		h.clients[commandID] = make(map[*websocket.Conn]bool)
		h.broadcasts[commandID] = make(chan string, 100)
		go h.runBroadcast(commandID)
	}
	h.clients[commandID][conn] = true
}

// Unregister removes a websocket connection
func (h *AptStreamHub) Unregister(commandID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[commandID]; ok {
		delete(clients, conn)
		if len(clients) == 0 {
			close(h.broadcasts[commandID])
			delete(h.clients, commandID)
			delete(h.broadcasts, commandID)
		}
	}
}

// Broadcast sends a log chunk to all connected clients for a given command
func (h *AptStreamHub) Broadcast(commandID string, logChunk string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if broadcast, ok := h.broadcasts[commandID]; ok {
		select {
		case broadcast <- logChunk:
		default:
			// Channel full, skip this chunk
		}
	}
}

// runBroadcast runs the broadcast loop for a specific command
func (h *AptStreamHub) runBroadcast(commandID string) {
	h.mu.RLock()
	broadcast := h.broadcasts[commandID]
	h.mu.RUnlock()

	for logChunk := range broadcast {
		h.mu.RLock()
		clients := h.clients[commandID]
		for conn := range clients {
			payload := map[string]interface{}{
				"type":       "apt_stream",
				"command_id": commandID,
				"chunk":      logChunk,
			}
			if err := conn.WriteJSON(payload); err != nil {
				conn.Close()
				go h.Unregister(commandID, conn)
			}
		}
		h.mu.RUnlock()
	}
}
