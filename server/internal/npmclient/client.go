// Package npmclient provides a minimal HTTP client for the Nginx Proxy Manager
// REST API. A fresh token is requested on every call; NPM tokens last 1 day so
// there is no need to cache them between syncs.
package npmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProxyHost is the subset of an NPM proxy-host record that ServerSupervisor uses.
type ProxyHost struct {
	ID            int      `json:"id"`
	DomainNames   []string `json:"domain_names"`
	ForwardScheme string   `json:"forward_scheme"` // "http" | "https"
	ForwardHost   string   `json:"forward_host"`
	ForwardPort   int      `json:"forward_port"`
	// CertificateID can be 0 (no cert), a positive int, or the string "0".
	// We decode it separately; SSLEnabled is derived below.
	SSLForced bool `json:"ssl_forced"`
	Enabled   bool `json:"enabled"`
	// certificate_id handled via raw map — see sslEnabled()
	Meta map[string]any `json:"meta"`
}

// SSLEnabled returns true when the proxy host has an active SSL certificate.
func (p *ProxyHost) SSLEnabled() bool {
	return p.SSLForced
}

type tokenResponse struct {
	Token   string `json:"token"`
	Expires string `json:"expires"`
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Authenticate obtains a Bearer token from NPM. The token is valid for 1 day
// by default and should be used immediately for the current sync cycle.
func Authenticate(ctx context.Context, apiURL, identity, secret string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"identity": identity,
		"secret":   secret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(apiURL, "/")+"/api/tokens", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("NPM auth failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("NPM auth: decode response: %w", err)
	}
	if tr.Token == "" {
		return "", fmt.Errorf("NPM auth: empty token in response")
	}
	return tr.Token, nil
}

// GetProxyHosts lists all proxy hosts from the NPM instance identified by apiURL
// using the given Bearer token.
func GetProxyHosts(ctx context.Context, apiURL, token string) ([]ProxyHost, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(apiURL, "/")+"/api/nginx/proxy-hosts?expand=certificate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("NPM proxy-hosts failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	// NPM returns a plain JSON array.
	var raw []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("NPM proxy-hosts: decode: %w", err)
	}

	hosts := make([]ProxyHost, 0, len(raw))
	for _, r := range raw {
		var h ProxyHost
		if err := json.Unmarshal(r, &h); err != nil {
			continue
		}
		// Derive SSLEnabled from certificate_id field which may be int 0 / positive int.
		h.SSLForced = h.SSLForced || hasCertificate(r)
		hosts = append(hosts, h)
	}
	return hosts, nil
}

// hasCertificate returns true when the raw proxy-host JSON has a non-zero certificate_id.
func hasCertificate(raw json.RawMessage) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	v, ok := m["certificate_id"]
	if !ok {
		return false
	}
	var n float64
	if err := json.Unmarshal(v, &n); err != nil {
		return false
	}
	return n > 0
}
