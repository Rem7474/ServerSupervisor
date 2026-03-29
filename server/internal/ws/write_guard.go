package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

var connWriteGuards sync.Map // map[*websocket.Conn]*sync.Mutex

func writeGuardFor(conn *websocket.Conn) *sync.Mutex {
	guard, _ := connWriteGuards.LoadOrStore(conn, &sync.Mutex{})
	return guard.(*sync.Mutex)
}

func releaseWriteGuard(conn *websocket.Conn) {
	connWriteGuards.Delete(conn)
}

func safeWriteJSON(conn *websocket.Conn, payload interface{}) error {
	guard := writeGuardFor(conn)
	guard.Lock()
	defer guard.Unlock()
	return conn.WriteJSON(payload)
}

func safeWriteMessage(conn *websocket.Conn, messageType int, data []byte) error {
	guard := writeGuardFor(conn)
	guard.Lock()
	defer guard.Unlock()
	return conn.WriteMessage(messageType, data)
}
