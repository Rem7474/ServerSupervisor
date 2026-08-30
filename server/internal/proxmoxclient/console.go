// LXC console support (termproxy). PVE's termproxy/vncwebsocket wire
// protocol is not documented in the official API viewer — the handshake and
// framing implemented here were confirmed against Proxmox's own
// pve-xtermjs source (termproxy/src/main.rs) plus community reports, not
// guessed.
package proxmoxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// pveLogin is the result of a PVE ticket login: the PVEAuthCookie value plus
// the CSRF prevention token PVE requires on any cookie-authenticated
// POST/PUT/DELETE.
type pveLogin struct {
	Ticket   string
	CSRFText string
}

// login performs a PVE ticket login (POST /access/ticket).
//
// This is a separate auth mechanism from the PVEAPIToken header every other
// Client method uses: PVE's vncwebsocket upgrade endpoint (shared by the
// term console and VNC) does not accept API-token auth — confirmed by a
// Proxmox staff reply on the community forum, reproduced by multiple users
// hitting "does not look like a valid user name" with a token. It requires a
// real user ticket in a PVEAuthCookie cookie instead.
//
// Critically, PVE also requires the *entire* console-opening sequence to run
// as one consistent identity: the vncticket returned by termproxy is only
// accepted by vncwebsocket if it was generated for the same user as the
// PVEAuthCookie presented alongside it ("permission denied - invalid
// PVEVNC ticket" otherwise — confirmed by a Proxmox staff reply on the
// community forum, reproduced against a real cluster). So termproxy must
// also be called via this cookie/CSRF pair, not the API token — see
// createTermproxy. Credentials are used once per call and never cached: a
// console session logs in fresh each time it's opened, which is well inside
// a PVE ticket's ~2h lifetime for a human-initiated, short-lived
// interactive session.
func (c *Client) login(username, password string) (pveLogin, error) {
	form := url.Values{"username": {username}, "password": {password}}
	reqURL := c.baseURL + "/access/ticket"
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return pveLogin{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// reqURL is built from c.baseURL, the admin-configured Proxmox
	// connection URL (proxmox_connections.api_url) — not attacker-controlled
	// request data. Every other Client method in this package (see
	// client.go) makes the same trust-boundary call against this identical
	// baseURL and carries the same accepted, still-open alert (#130); this
	// mirrors that existing posture rather than introducing a new one.
	// codeql[go/request-forgery]: baseURL is admin-configured, not attacker input — see client.go's identical, already-accepted pattern
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return pveLogin{}, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return pveLogin{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return pveLogin{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	var envelope struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return pveLogin{}, fmt.Errorf("parse response: %w", err)
	}
	if envelope.Data.Ticket == "" {
		return pveLogin{}, fmt.Errorf("login succeeded but no ticket returned")
	}
	return pveLogin{Ticket: envelope.Data.Ticket, CSRFText: envelope.Data.CSRFPreventionToken}, nil
}

// TestLogin verifies PVE user credentials by performing the same login used
// to open a console (see login's doc comment), without opening a session.
// Used to validate console credentials from the connection settings UI,
// where no specific guest is available to open a real console against.
func (c *Client) TestLogin(username, password string) error {
	_, err := c.login(username, password)
	return err
}

// createTermproxy calls POST /nodes/{node}/lxc/{vmid}/termproxy, which asks
// PVE to spawn a shell and open a local TCP proxy for it, authenticated as
// the same cookie identity that will later present the resulting vncticket
// to vncwebsocket (see login's doc comment for why this can't be the API
// token). Requires the VM.Console privilege on that PVE user.
func (c *Client) createTermproxy(node string, vmid int, auth pveLogin) (ticket string, port int, user string, err error) {
	reqURL := c.baseURL + fmt.Sprintf("/nodes/%s/lxc/%d/termproxy", node, vmid)
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return "", 0, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Cookie", "PVEAuthCookie="+auth.Ticket)
	req.Header.Set("CSRFPreventionToken", auth.CSRFText)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return "", 0, "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	var envelope struct {
		Data struct {
			Ticket string  `json:"ticket"`
			Port   FlexInt `json:"port"` // PVE sometimes quotes numeric fields; see FlexInt.
			User   string  `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", 0, "", fmt.Errorf("parse response: %w", err)
	}
	return envelope.Data.Ticket, int(envelope.Data.Port), envelope.Data.User, nil
}

// wsBaseURL converts the client's http(s) baseURL to its ws(s) equivalent.
func (c *Client) wsBaseURL() string {
	switch {
	case strings.HasPrefix(c.baseURL, "https://"):
		return "wss://" + strings.TrimPrefix(c.baseURL, "https://")
	case strings.HasPrefix(c.baseURL, "http://"):
		return "ws://" + strings.TrimPrefix(c.baseURL, "http://")
	default:
		return c.baseURL
	}
}

// TermSession is an open PVE termproxy console for a running LXC container.
// PVE multiplexes keystroke/resize/ping control messages from the client
// over the same socket as "0:LEN:DATA" / "1:COLS:ROWS:" / "2" text frames,
// while writing raw, unframed PTY output straight back — TermSession hides
// all of that so callers only ever see plain bytes in (Write) and out
// (ReadMessage).
type TermSession struct {
	conn *websocket.Conn
	// writeMu serializes Write/Resize/Ping: gorilla/websocket requires at
	// most one concurrent writer per connection, but ws/proxmox_console.go
	// calls Write/Resize from its browser-to-PVE forwarding goroutine while
	// a separate keepalive ticker goroutine calls Ping concurrently.
	writeMu sync.Mutex
}

// handshakeTimeout bounds the termproxy login write/read after the
// WebSocket dial succeeds — see its use in OpenLXCConsole for why nothing
// else already covers this specific step. A var, not a const, so tests can
// shrink it rather than waiting out the real value.
var handshakeTimeout = 10 * time.Second

// OpenLXCConsole opens a live shell session on an LXC container: logs in
// with the supplied PVE user credentials (see login's doc comment for why
// this differs from the token auth used everywhere else), asks PVE to spawn
// a shell (createTermproxy), dials the resulting vncwebsocket endpoint, and
// performs the undocumented termproxy login handshake. The returned session
// is immediately ready for interactive use.
func (c *Client) OpenLXCConsole(ctx context.Context, node string, vmid int, pveUsername, pvePassword string) (*TermSession, error) {
	auth, err := c.login(pveUsername, pvePassword)
	if err != nil {
		return nil, fmt.Errorf("pve login: %w", err)
	}

	termTicket, port, user, err := c.createTermproxy(node, vmid, auth)
	if err != nil {
		return nil, fmt.Errorf("create termproxy: %w", err)
	}

	wsURL := fmt.Sprintf("%s/nodes/%s/lxc/%d/vncwebsocket?port=%d&vncticket=%s",
		c.wsBaseURL(), node, vmid, port, url.QueryEscape(termTicket))

	// Subprotocol "binary" mirrors what PVE's own web UI negotiates for this
	// endpoint; PVE rejects the upgrade (bad handshake) without it.
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second, Subprotocols: []string{"binary"}}
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok && transport != nil {
		dialer.TLSClientConfig = transport.TLSClientConfig
	}
	header := http.Header{}
	header.Set("Cookie", "PVEAuthCookie="+auth.Ticket)

	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
			_ = resp.Body.Close()
			return nil, fmt.Errorf("dial console websocket: %w (HTTP %d: %s)", err, resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("dial console websocket: %w", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	// Undocumented termproxy login handshake: "<user>:<ticket>\n" as the
	// first message, server replies with the literal bytes "OK". Unlike the
	// login()/createTermproxy() HTTP calls above (bounded by c.httpClient's
	// own Timeout) and the dial above (bounded by dialer.HandshakeTimeout),
	// nothing else bounds this write/read pair — a PVE that accepts the WS
	// upgrade but never completes the handshake would otherwise hang this
	// goroutine forever. The deadline is cleared before returning: the
	// interactive session that follows must not inherit it (TermSession's
	// own Ping-based keepalive is what keeps that connection alive instead).
	if err := conn.SetWriteDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set handshake write deadline: %w", err)
	}
	handshake := fmt.Sprintf("%s:%s\n", user, termTicket)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(handshake)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send login handshake: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("set handshake read deadline: %w", err)
	}
	_, loginResp, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read login response: %w", err)
	}
	if string(loginResp) != "OK" {
		_ = conn.Close()
		return nil, fmt.Errorf("termproxy login rejected: %s", loginResp)
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clear handshake read deadline: %w", err)
	}

	return &TermSession{conn: conn}, nil
}

// Write sends keystroke/paste bytes to the remote shell.
func (s *TermSession) Write(data []byte) error {
	frame := append([]byte(fmt.Sprintf("0:%d:", len(data))), data...)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, frame)
}

// Resize tells PVE to resize the PTY.
func (s *TermSession) Resize(cols, rows int) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("1:%d:%d:", cols, rows)))
}

// Ping sends a termproxy keepalive; PVE closes idle console sessions after a
// few minutes without one.
func (s *TermSession) Ping() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, []byte("2"))
}

// ReadMessage returns the next raw PTY output chunk from the remote shell.
func (s *TermSession) ReadMessage() ([]byte, error) {
	_, data, err := s.conn.ReadMessage()
	return data, err
}

// Close closes the underlying WebSocket connection to PVE.
func (s *TermSession) Close() error {
	return s.conn.Close()
}
