// Package proxmoxclient provides a minimal HTTP client for the Proxmox VE REST API.
// Authentication is done via API token (PVEAPIToken header).
// All responses are unwrapped from the {"data": ...} envelope.
package proxmoxclient

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to one Proxmox VE instance.
type Client struct {
	baseURL     string
	tokenID     string
	tokenSecret string
	httpClient  *http.Client
}

// New creates a Client.
// insecureSkipVerify should only be true for self-signed certificates in dev/lab environments.
func New(baseURL, tokenID, tokenSecret string, insecureSkipVerify bool) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec
	}
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		tokenID:     tokenID,
		tokenSecret: tokenSecret,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
		},
	}
}

// get performs a GET request and unmarshals the Proxmox {"data": ...} envelope into result.
func (c *Client) get(path string, result interface{}) error {
	url := c.baseURL + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response from %s: %w", path, err)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("authentication failed (HTTP %d) — check token_id and token_secret", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return fmt.Errorf("API %s returned HTTP %d: %s", path, resp.StatusCode, snippet)
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("parse envelope from %s: %w", path, err)
	}
	if err := json.Unmarshal(envelope.Data, result); err != nil {
		return fmt.Errorf("parse data from %s: %w", path, err)
	}
	return nil
}

// ─── Proxmox API response structs ────────────────────────────────────────────

// PVENode is the element returned by GET /nodes.
type PVENode struct {
	Node       string  `json:"node"`
	Status     string  `json:"status"` // online | offline
	CPU        float64 `json:"cpu"`    // 0–1 fraction
	MaxCPU     int     `json:"maxcpu"`
	Mem        int64   `json:"mem"`
	MaxMem     int64   `json:"maxmem"`
	Uptime     int64   `json:"uptime"`
	IP         string  `json:"ip,omitempty"`
	PVEVersion string  `json:"pveversion,omitempty"`
}

// PVEGuest is used both for QEMU VMs and LXC containers.
type PVEGuest struct {
	VMID    int     `json:"vmid"`
	Name    string  `json:"name"`
	Status  string  `json:"status"` // running | stopped | paused
	CPU     float64 `json:"cpu"`    // current usage fraction
	CPUs    float64 `json:"cpus"`   // allocated vCPUs (float for LXC fractions)
	Mem     int64   `json:"mem"`    // current used bytes
	MaxMem  int64   `json:"maxmem"` // allocated bytes
	MaxDisk int64   `json:"maxdisk,omitempty"`
	Tags    string  `json:"tags,omitempty"`
	Uptime  int64   `json:"uptime,omitempty"`
	// Present only when fetched via /cluster/resources
	Node string `json:"node,omitempty"`
	Type string `json:"type,omitempty"` // qemu | lxc
}

// PVEStorage is an element from GET /nodes/{node}/storage.
type PVEStorage struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Total   int64  `json:"total"`
	Used    int64  `json:"used"`
	Avail   int64  `json:"avail"`
	Enabled int    `json:"enabled"` // 0 or 1
	Active  int    `json:"active"`  // 0 or 1
	Shared  int    `json:"shared"`  // 0 or 1
}

// PVEClusterStatus is an element from GET /cluster/status.
type PVEClusterStatus struct {
	Name string `json:"name"`
	Type string `json:"type"` // cluster | node
	ID   string `json:"id"`
}

// PVEVersion is from GET /nodes/{node}/version.
type PVEVersion struct {
	Version string `json:"version"`
	Release string `json:"release"`
}

// ─── API methods ─────────────────────────────────────────────────────────────

// TestConnection verifies that the API is reachable and the token is valid.
func (c *Client) TestConnection() error {
	var nodes []PVENode
	return c.get("/nodes", &nodes)
}

// GetNodes returns all nodes visible to this connection.
func (c *Client) GetNodes() ([]PVENode, error) {
	var nodes []PVENode
	if err := c.get("/nodes", &nodes); err != nil {
		return nil, err
	}
	return nodes, nil
}

// GetClusterStatus returns the cluster status (name, members).
// Returns an empty slice without error if the Proxmox instance is not clustered.
func (c *Client) GetClusterStatus() ([]PVEClusterStatus, error) {
	var statuses []PVEClusterStatus
	if err := c.get("/cluster/status", &statuses); err != nil {
		// Non-clustered nodes may return 500 on this endpoint; treat as empty.
		return []PVEClusterStatus{}, nil
	}
	return statuses, nil
}

// GetNodeQemu returns all QEMU VMs on the given node.
func (c *Client) GetNodeQemu(node string) ([]PVEGuest, error) {
	var guests []PVEGuest
	if err := c.get(fmt.Sprintf("/nodes/%s/qemu", node), &guests); err != nil {
		return nil, err
	}
	return guests, nil
}

// GetNodeLXC returns all LXC containers on the given node.
func (c *Client) GetNodeLXC(node string) ([]PVEGuest, error) {
	var guests []PVEGuest
	if err := c.get(fmt.Sprintf("/nodes/%s/lxc", node), &guests); err != nil {
		return nil, err
	}
	return guests, nil
}

// GetNodeStorage returns storage pools visible on the given node.
func (c *Client) GetNodeStorage(node string) ([]PVEStorage, error) {
	var storages []PVEStorage
	if err := c.get(fmt.Sprintf("/nodes/%s/storage", node), &storages); err != nil {
		return nil, err
	}
	return storages, nil
}

// GetNodeVersion returns the PVE version string for the given node.
func (c *Client) GetNodeVersion(node string) (string, error) {
	var v PVEVersion
	if err := c.get(fmt.Sprintf("/nodes/%s/version", node), &v); err != nil {
		return "", err
	}
	if v.Release != "" {
		return v.Version + "-" + v.Release, nil
	}
	return v.Version, nil
}

// ClusterName extracts the cluster name from a /cluster/status response.
// Returns an empty string if the instance is standalone (not clustered).
func ClusterName(statuses []PVEClusterStatus) string {
	for _, s := range statuses {
		if s.Type == "cluster" {
			return s.Name
		}
	}
	return ""
}
