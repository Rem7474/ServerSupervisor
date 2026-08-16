package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

// AgentHub keeps at most one live WebSocket connection per host, used to
// nudge the agent to poll for pending commands immediately instead of
// waiting out its next ticker interval. It deliberately does not carry
// command content: the nudge only triggers the agent's existing report/poll
// cycle (which claims pending commands atomically, FOR UPDATE SKIP LOCKED)
// sooner, rather than opening a second delivery path with its own claim
// semantics that could race the first and double-execute a command. This is
// a latency optimization layered on top of the existing poll cycle:
// dispatch.Dispatcher always persists the command first, and Notify is
// best-effort — an agent with no connection (an older binary, or one behind
// a proxy that blocks WebSocket upgrades) keeps working exactly as before,
// picking the command up on its next regularly scheduled poll.
type AgentHub struct {
	mu           sync.RWMutex
	conns        map[string]*websocket.Conn // host_id -> latest connection
	onDisconnect func(hostID string)
}

func NewAgentHub() *AgentHub {
	return &AgentHub{conns: make(map[string]*websocket.Conn)}
}

// SetOnDisconnect registers a callback fired whenever a host's live
// connection is actually removed (not when it's superseded by a fresh
// reconnect — see Unregister). Used to react faster than the last-seen sweep
// when we have direct evidence the socket closed.
func (h *AgentHub) SetOnDisconnect(fn func(hostID string)) {
	h.mu.Lock()
	h.onDisconnect = fn
	h.mu.Unlock()
}

// Register stores hostID's connection, closing out any previous one for the
// same host — an agent that reconnects (e.g. after a restart) replaces its
// own stale connection rather than leaving two registered for one host.
func (h *AgentHub) Register(hostID string, conn *websocket.Conn) {
	h.mu.Lock()
	old := h.conns[hostID]
	h.conns[hostID] = conn
	h.mu.Unlock()
	if old != nil && old != conn {
		_ = old.Close()
	}
}

// Unregister removes conn if it is still the registered connection for
// hostID — a no-op if it was already replaced by a newer one (Register
// closes the superseded connection directly, so that connection's own
// eventual Unregister call here is correctly ignored rather than treated as
// a disconnect of the still-live replacement).
func (h *AgentHub) Unregister(hostID string, conn *websocket.Conn) {
	h.mu.Lock()
	removed := false
	if h.conns[hostID] == conn {
		delete(h.conns, hostID)
		removed = true
	}
	fn := h.onDisconnect
	h.mu.Unlock()
	if removed && fn != nil {
		fn(hostID)
	}
}

// Connected reports whether hostID currently has a live push connection.
func (h *AgentHub) Connected(hostID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.conns[hostID]
	return ok
}

type agentPollNowMessage struct {
	Type string `json:"type"`
}

// Notify best-effort tells hostID's live connection, if any, to poll for
// pending commands immediately. The returned bool is diagnostic only ("was a
// live connection nudged") — callers must never treat a false return as an
// error: the command is already durably queued in remote_commands and will
// be picked up on the agent's next regularly scheduled poll regardless.
func (h *AgentHub) Notify(hostID string) bool {
	h.mu.RLock()
	conn := h.conns[hostID]
	h.mu.RUnlock()
	if conn == nil {
		return false
	}

	if err := safeWriteJSON(conn, agentPollNowMessage{Type: "poll_now"}); err != nil {
		_ = conn.Close()
		h.Unregister(hostID, conn)
		return false
	}
	return true
}
