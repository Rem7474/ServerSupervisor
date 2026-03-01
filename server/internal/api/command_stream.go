package api

import (
	"sync"

	"github.com/gorilla/websocket"
)

// CommandStreamHub manages real-time streaming of remote command output.
// It is shared across all modules (apt, docker, systemd, journal, processes).
type CommandStreamHub struct {
	clients    map[string]map[*websocket.Conn]bool // commandID -> set of websocket connections
	broadcasts map[string]chan string              // commandID -> broadcast channel
	mu         sync.RWMutex
}

// NewCommandStreamHub creates a new streaming hub.
func NewCommandStreamHub() *CommandStreamHub {
	return &CommandStreamHub{
		clients:    make(map[string]map[*websocket.Conn]bool),
		broadcasts: make(map[string]chan string),
	}
}

// Register adds a websocket connection to receive output for a specific command.
func (h *CommandStreamHub) Register(commandID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[commandID] == nil {
		h.clients[commandID] = make(map[*websocket.Conn]bool)
		h.broadcasts[commandID] = make(chan string, 100)
		go h.runBroadcast(commandID)
	}
	h.clients[commandID][conn] = true
}

// Unregister removes a websocket connection.
func (h *CommandStreamHub) Unregister(commandID string, conn *websocket.Conn) {
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

// Broadcast sends an output chunk to all connected clients for a given command.
func (h *CommandStreamHub) Broadcast(commandID string, logChunk string) {
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

// BroadcastStatus sends a status update to all connected clients for a given command.
// output is included in the payload when non-empty (e.g. for completed commands).
func (h *CommandStreamHub) BroadcastStatus(commandID, status, output string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.clients[commandID]
	for conn := range clients {
		payload := map[string]interface{}{
			"type":       "cmd_status_update",
			"command_id": commandID,
			"status":     status,
		}
		if output != "" {
			payload["output"] = output
		}
		if err := conn.WriteJSON(payload); err != nil {
			conn.Close()
			go h.Unregister(commandID, conn)
		}
	}
}

// runBroadcast runs the broadcast loop for a specific command.
func (h *CommandStreamHub) runBroadcast(commandID string) {
	h.mu.RLock()
	broadcast := h.broadcasts[commandID]
	h.mu.RUnlock()

	for logChunk := range broadcast {
		h.mu.RLock()
		clients := h.clients[commandID]
		for conn := range clients {
			payload := map[string]interface{}{
				"type":       "cmd_stream",
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
