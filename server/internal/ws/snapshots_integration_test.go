package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/testutil"
)

func TestSnapshots_Integration(t *testing.T) {
	db := testutil.NewPostgresDB(t)
	ctx := context.Background()

	h1 := &models.Host{ID: "host-snap-1", Name: "Host 1", Status: "online"}
	if err := db.RegisterHost(ctx, h1); err != nil {
		t.Fatalf("failed to register host: %v", err)
	}

	var serverConn *websocket.Conn
	var serverConnMu sync.Mutex
	connReady := make(chan struct{})

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConnMu.Lock()
		serverConn = c
		serverConnMu.Unlock()
		close(connReady)

		for {
			if _, _, err := c.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")
	clientConn, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial error: %v", err)
	}
	defer clientConn.Close()
	<-connReady

	serverConnMu.Lock()
	conn := serverConn
	serverConnMu.Unlock()
	defer conn.Close()
	defer releaseWriteGuard(conn)

	h := &WSHandler{
		db:                 db,
		latestAgentVersion: func() string { return "1.0.0" },
	}

	// 1. Dashboard snapshot
	var dashHash string
	if err := h.sendDashboardSnapshot(ctx, conn, &dashHash); err != nil {
		t.Fatalf("sendDashboardSnapshot error: %v", err)
	}
	if dashHash == "" {
		t.Fatal("expected dashHash to be set")
	}
	if _, _, err := clientConn.ReadMessage(); err != nil {
		t.Fatalf("client read dashboard error: %v", err)
	}
	if err := h.sendDashboardSnapshot(ctx, conn, &dashHash); err != nil {
		t.Fatalf("sendDashboardSnapshot unchanged error: %v", err)
	}

	// 2. Host snapshot
	var hostHash string
	if err := h.sendHostSnapshot(ctx, conn, h1.ID, &hostHash); err != nil {
		t.Fatalf("sendHostSnapshot error: %v", err)
	}
	if hostHash == "" {
		t.Fatal("expected hostHash to be set")
	}
	if _, _, err := clientConn.ReadMessage(); err != nil {
		t.Fatalf("client read host error: %v", err)
	}
	if err := h.sendHostSnapshot(ctx, conn, h1.ID, &hostHash); err != nil {
		t.Fatalf("sendHostSnapshot unchanged error: %v", err)
	}

	// 3. Apt snapshot
	var aptHash string
	if err := h.sendAptSnapshot(ctx, conn, &aptHash); err != nil {
		t.Fatalf("sendAptSnapshot error: %v", err)
	}
	if aptHash == "" {
		t.Fatal("expected aptHash to be set")
	}
	if _, _, err := clientConn.ReadMessage(); err != nil {
		t.Fatalf("client read apt error: %v", err)
	}
	if err := h.sendAptSnapshot(ctx, conn, &aptHash); err != nil {
		t.Fatalf("sendAptSnapshot unchanged error: %v", err)
	}

	// 4. Docker snapshot
	var dockerHash string
	if err := h.sendDockerSnapshot(ctx, conn, &dockerHash); err != nil {
		t.Fatalf("sendDockerSnapshot error: %v", err)
	}
	if dockerHash == "" {
		t.Fatal("expected dockerHash to be set")
	}
	if _, _, err := clientConn.ReadMessage(); err != nil {
		t.Fatalf("client read docker error: %v", err)
	}
	if err := h.sendDockerSnapshot(ctx, conn, &dockerHash); err != nil {
		t.Fatalf("sendDockerSnapshot unchanged error: %v", err)
	}

	// 5. Network snapshot
	var netHash string
	if err := h.sendNetworkSnapshot(ctx, conn, &netHash); err != nil {
		t.Fatalf("sendNetworkSnapshot error: %v", err)
	}
	if netHash == "" {
		t.Fatal("expected netHash to be set")
	}
	if _, _, err := clientConn.ReadMessage(); err != nil {
		t.Fatalf("client read net error: %v", err)
	}
	if err := h.sendNetworkSnapshot(ctx, conn, &netHash); err != nil {
		t.Fatalf("sendNetworkSnapshot unchanged error: %v", err)
	}
}
