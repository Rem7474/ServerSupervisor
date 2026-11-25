// QEMU VNC console support (vncproxy + vncwebsocket). The REST half follows
// PVE's API (POST /nodes/{node}/qemu/{vmid}/vncproxy); the RFB handshake that
// runs over the resulting socket lives in package rfb.
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
	"github.com/serversupervisor/server/internal/rfb"
)

// vncProxy is the part of PVE's vncproxy response the console needs.
type vncProxy struct {
	Ticket   string
	Port     int
	Password string
}

// createVNCProxy calls POST /nodes/{node}/qemu/{vmid}/vncproxy with
// websocket=1, authenticated as the same cookie identity that later presents
// the ticket to vncwebsocket (see login's doc comment for why this cannot be
// the API token). Requires the VM.Console privilege on that PVE user.
//
// PVE returns a one-time `password` to answer the VNC Authentication
// challenge inside the RFB protocol. Current PVE generates it whenever
// websocket=1; generate-password=1 is still sent for older releases that only
// generate it on request (the parameter is deprecated, not removed).
func (c *Client) createVNCProxy(node string, vmid int, auth pveLogin) (vncProxy, error) {
	reqURL := c.baseURL + fmt.Sprintf("/nodes/%s/qemu/%d/vncproxy", url.PathEscape(node), vmid)
	form := url.Values{"websocket": {"1"}, "generate-password": {"1"}}
	req, err := http.NewRequest(http.MethodPost, reqURL, strings.NewReader(form.Encode()))
	if err != nil {
		return vncProxy{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "PVEAuthCookie="+auth.Ticket)
	req.Header.Set("CSRFPreventionToken", auth.CSRFText)
	req.Header.Set("Accept", "application/json")

	// codeql[go/request-forgery]: baseURL is admin-configured, not attacker input — see client.go's identical, already-accepted pattern
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return vncProxy{}, fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return vncProxy{}, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return vncProxy{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	var envelope struct {
		Data struct {
			Ticket   string  `json:"ticket"`
			Port     FlexInt `json:"port"` // PVE sometimes quotes numeric fields; see FlexInt.
			Password string  `json:"password"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return vncProxy{}, fmt.Errorf("parse response: %w", err)
	}
	if envelope.Data.Ticket == "" || envelope.Data.Port == 0 {
		return vncProxy{}, fmt.Errorf("vncproxy returned no ticket or port")
	}
	return vncProxy{Ticket: envelope.Data.Ticket, Port: int(envelope.Data.Port), Password: envelope.Data.Password}, nil
}

// VNCSession is an open, authenticated RFB connection to a QEMU VM's
// display, positioned right before the ClientInit message. The caller relays
// raw bytes in both directions from there on.
type VNCSession struct {
	stream *rfb.WSStream
}

// OpenQEMUVNC opens a VNC session on a QEMU VM: logs in with the supplied PVE
// user credentials, asks PVE for a VNC proxy (createVNCProxy), dials the
// resulting vncwebsocket endpoint and performs the RFB client handshake,
// answering PVE's VNC Authentication challenge with the one-time password.
// Neither the PVE ticket nor the VNC password leaves this function.
func (c *Client) OpenQEMUVNC(ctx context.Context, node string, vmid int, pveUsername, pvePassword string) (*VNCSession, error) {
	auth, err := c.login(pveUsername, pvePassword)
	if err != nil {
		return nil, fmt.Errorf("pve login: %w", err)
	}

	proxy, err := c.createVNCProxy(node, vmid, auth)
	if err != nil {
		return nil, fmt.Errorf("create vncproxy: %w", err)
	}

	wsURL := fmt.Sprintf("%s/nodes/%s/qemu/%d/vncwebsocket?port=%d&vncticket=%s",
		c.wsBaseURL(), url.PathEscape(node), vmid, proxy.Port, url.QueryEscape(proxy.Ticket))

	// Same dialer setup as OpenLXCConsole: the "binary" subprotocol PVE's own
	// UI negotiates, and the connection's TLS settings (InsecureSkipVerify
	// follows the Proxmox connection's own configuration).
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
			return nil, fmt.Errorf("dial vnc websocket: %w (HTTP %d: %s)", err, resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("dial vnc websocket: %w", err)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	// Bound the handshake like OpenLXCConsole does: nothing else stops a PVE
	// that accepts the upgrade but never speaks RFB from hanging forever. The
	// deadlines are cleared afterwards: an interactive session can sit idle.
	deadline := time.Now().Add(handshakeTimeout)
	_ = conn.SetReadDeadline(deadline)
	_ = conn.SetWriteDeadline(deadline)
	stream := rfb.NewWSStream(conn)
	if err := rfb.ClientHandshake(stream, proxy.Password); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("vnc handshake: %w", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	_ = conn.SetWriteDeadline(time.Time{})

	return &VNCSession{stream: stream}, nil
}

// ReadMessage returns the next chunk of RFB bytes from the VM's display
// (including any bytes the handshake read ahead).
func (s *VNCSession) ReadMessage() ([]byte, error) {
	return s.stream.ReadMessage()
}

// Write sends RFB bytes from the browser to the VM's display.
func (s *VNCSession) Write(data []byte) error {
	return s.stream.WriteMessage(data)
}

// Close closes the underlying WebSocket connection to PVE.
func (s *VNCSession) Close() error {
	return s.stream.Conn().Close()
}
