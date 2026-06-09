package ws

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/models"
)

// CommandStreamHub manages real-time streaming of remote command output.
// It is shared across all modules (apt, docker, systemd, journal, processes).
type CommandStreamHub struct {
	clients    map[string]map[*websocket.Conn]bool // commandID -> set of websocket connections
	broadcasts map[string]chan string              // commandID -> broadcast channel
	mu         sync.RWMutex

	bufferMu sync.Mutex
	buffers  map[string]string // accumulated stream output per active command
}

// NewCommandStreamHub creates a new streaming hub.
func NewCommandStreamHub() *CommandStreamHub {
	return &CommandStreamHub{
		clients:    make(map[string]map[*websocket.Conn]bool),
		broadcasts: make(map[string]chan string),
		buffers:    make(map[string]string),
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
// The chunk is also appended to an in-memory buffer so late-joining clients
// can receive the full output history via cmd_stream_init.
func (h *CommandStreamHub) Broadcast(commandID string, logChunk string) {
	h.bufferMu.Lock()
	h.buffers[commandID] += logChunk
	h.bufferMu.Unlock()

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

// GetBufferedOutput returns all output chunks accumulated so far for a command.
// Returns an empty string if no chunks have been buffered (command not yet started
// or already completed and buffer cleaned up).
func (h *CommandStreamHub) GetBufferedOutput(commandID string) string {
	h.bufferMu.Lock()
	defer h.bufferMu.Unlock()
	return h.buffers[commandID]
}

// BroadcastStatus sends a status update to all connected clients for a given command.
// output is included in the payload when non-empty (e.g. for completed commands).
// On terminal statuses (completed/failed) the in-memory chunk buffer is released.
func (h *CommandStreamHub) BroadcastStatus(commandID, status, output string) {
	h.mu.RLock()
	conns := make([]*websocket.Conn, 0, len(h.clients[commandID]))
	for conn := range h.clients[commandID] {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	payload := models.WSCommandStatusUpdate{
		Type:      "cmd_status_update",
		CommandID: commandID,
		Status:    status,
		Output:    output, // omitempty drops it when empty
	}
	for _, conn := range conns {
		if err := safeWriteJSON(conn, payload); err != nil {
			_ = conn.Close()
			h.Unregister(commandID, conn)
		}
	}

	if status == "completed" || status == "failed" {
		h.bufferMu.Lock()
		delete(h.buffers, commandID)
		h.bufferMu.Unlock()
	}
}

// runBroadcast runs the broadcast loop for a specific command.
func (h *CommandStreamHub) runBroadcast(commandID string) {
	h.mu.RLock()
	broadcast := h.broadcasts[commandID]
	h.mu.RUnlock()

	for logChunk := range broadcast {
		// Snapshot client list under read lock so I/O doesn't block the hub.
		h.mu.RLock()
		conns := make([]*websocket.Conn, 0, len(h.clients[commandID]))
		for conn := range h.clients[commandID] {
			conns = append(conns, conn)
		}
		h.mu.RUnlock()

		payload := models.WSCommandStreamChunk{
			Type:      "cmd_stream",
			CommandID: commandID,
			Chunk:     logChunk,
		}
		for _, conn := range conns {
			if err := safeWriteJSON(conn, payload); err != nil {
				_ = conn.Close()
				h.Unregister(commandID, conn)
			}
		}
	}
}
