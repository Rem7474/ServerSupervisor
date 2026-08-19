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
	"time"

	"github.com/gorilla/websocket"
)

// login performs a PVE ticket login (POST /access/ticket) and returns the
// PVEAuthCookie ticket value.
//
// This is a separate auth mechanism from the PVEAPIToken header every other
// Client method uses: PVE's vncwebsocket upgrade endpoint (shared by the
// term console and VNC) does not accept API-token auth — confirmed by a
// Proxmox staff reply on the community forum, reproduced by multiple users
// hitting "does not look like a valid user name" with a token. It requires a
// real user ticket in a PVEAuthCookie cookie instead. Credentials are used
// once per call and never cached: a console session logs in fresh each time
// it's opened, which is well inside a PVE ticket's ~2h lifetime for a
// human-initiated, short-lived interactive session.
func (c *Client) login(username, password string) (ticket string, err error) {
	form := url.Values{"username": {username}, "password": {password}}
	reqURL := c.baseURL + "/access/ticket"
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	var envelope struct {
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if envelope.Data.Ticket == "" {
		return "", fmt.Errorf("login succeeded but no ticket returned")
	}
	return envelope.Data.Ticket, nil
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
// PVE to spawn a shell and open a local TCP proxy for it. Requires the
// VM.Console privilege on the API token.
func (c *Client) createTermproxy(node string, vmid int) (ticket string, port int, user string, err error) {
	reqURL := c.baseURL + fmt.Sprintf("/nodes/%s/lxc/%d/termproxy", node, vmid)
	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		return "", 0, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
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
}

// OpenLXCConsole opens a live shell session on an LXC container: logs in
// with the supplied PVE user credentials (see login's doc comment for why
// this differs from the token auth used everywhere else), asks PVE to spawn
// a shell (createTermproxy), dials the resulting vncwebsocket endpoint, and
// performs the undocumented termproxy login handshake. The returned session
// is immediately ready for interactive use.
func (c *Client) OpenLXCConsole(ctx context.Context, node string, vmid int, pveUsername, pvePassword string) (*TermSession, error) {
	authTicket, err := c.login(pveUsername, pvePassword)
	if err != nil {
		return nil, fmt.Errorf("pve login: %w", err)
	}

	termTicket, port, user, err := c.createTermproxy(node, vmid)
	if err != nil {
		return nil, fmt.Errorf("create termproxy: %w", err)
	}

	wsURL := fmt.Sprintf("%s/nodes/%s/lxc/%d/vncwebsocket?port=%d&vncticket=%s",
		c.wsBaseURL(), node, vmid, port, url.QueryEscape(termTicket))

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok && transport != nil {
		dialer.TLSClientConfig = transport.TLSClientConfig
	}
	header := http.Header{}
	header.Set("Cookie", "PVEAuthCookie="+authTicket)

	conn, resp, err := dialer.DialContext(ctx, wsURL, header)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("dial console websocket: %w", err)
	}

	// Undocumented termproxy login handshake: "<user>:<ticket>\n" as the
	// first message, server replies with the literal bytes "OK".
	handshake := fmt.Sprintf("%s:%s\n", user, termTicket)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(handshake)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send login handshake: %w", err)
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

	return &TermSession{conn: conn}, nil
}

// Write sends keystroke/paste bytes to the remote shell.
func (s *TermSession) Write(data []byte) error {
	frame := append([]byte(fmt.Sprintf("0:%d:", len(data))), data...)
	return s.conn.WriteMessage(websocket.TextMessage, frame)
}

// Resize tells PVE to resize the PTY.
func (s *TermSession) Resize(cols, rows int) error {
	return s.conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("1:%d:%d:", cols, rows)))
}

// Ping sends a termproxy keepalive; PVE closes idle console sessions after a
// few minutes without one.
func (s *TermSession) Ping() error {
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
