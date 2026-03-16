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
	"strconv"
	"strings"
	"time"
)

// FlexInt unmarshals a JSON value that Proxmox sometimes returns as a quoted
// string ("100") and sometimes as a plain number (100). Absent values unmarshal
// to 0.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	// Unquote if it's a JSON string.
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// Non-numeric string (e.g. "N/A") — treat as 0.
		*f = 0
		return nil
	}
	*f = FlexInt(v)
	return nil
}

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

	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
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

// ─── Extended API types ───────────────────────────────────────────────────────

// PVETask is an element returned by GET /nodes/{node}/tasks.
// starttime/endtime are Unix epoch seconds.
type PVETask struct {
	UPID       string  `json:"upid"`
	Type       string  `json:"type"`
	Status     string  `json:"status"`              // running | stopped
	User       string  `json:"user"`
	StartTime  int64   `json:"starttime"`
	EndTime    int64   `json:"endtime,omitempty"`
	ID         string  `json:"id,omitempty"`        // vmid or other Proxmox object
	Node       string  `json:"node,omitempty"`
	ExitStatus string  `json:"exitstatus,omitempty"` // OK | error msg (only when stopped)
}

// PVEAptPackage is an element returned by GET /nodes/{node}/apt/update.
type PVEAptPackage struct {
	Package    string `json:"Package"`
	Version    string `json:"Version"`
	OldVersion string `json:"OldVersion"`
	Priority   string `json:"Priority"`
	Section    string `json:"Section"`
	Origin     string `json:"Origin"`
	Description string `json:"Description"`
}

// PVEDisk is an element returned by GET /nodes/{node}/disks/list.
type PVEDisk struct {
	DevPath string  `json:"devpath"`
	Model   string  `json:"model"`
	Serial  string  `json:"serial"`
	Size    int64   `json:"size"`
	Type    string  `json:"type"`    // ssd | hdd | nvme | unknown
	Health  string  `json:"health"`  // PASSED | FAILED | UNKNOWN
	Wearout FlexInt `json:"wearout"` // SSD wear % (100=new, absent for HDD) — may be quoted string in PVE API
}

// PVEBackupJob is an element returned by GET /cluster/backup.
type PVEBackupJob struct {
	ID       string `json:"id"`
	Enabled  int    `json:"enabled"` // 0 or 1
	Schedule string `json:"schedule,omitempty"`
	Storage  string `json:"storage,omitempty"`
	Mode     string `json:"mode,omitempty"`     // snapshot | suspend | stop
	Compress string `json:"compress,omitempty"`
	VMIDs    string `json:"vmid,omitempty"`     // comma-separated or "all"
	MailTo   string `json:"mailto,omitempty"`
}

// ─── Extended API methods ─────────────────────────────────────────────────────

// GetNodeTasks returns up to limit recent tasks for the given node.
// limit ≤ 0 defaults to 50.
func (c *Client) GetNodeTasks(node string, limit int) ([]PVETask, error) {
	if limit <= 0 {
		limit = 50
	}
	var tasks []PVETask
	if err := c.get(fmt.Sprintf("/nodes/%s/tasks?limit=%d", node, limit), &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetNodeAptUpdate returns the list of pending apt packages on the given node.
// This endpoint reads the cached package list; it never triggers an update.
// Returns the actual error so callers can log it; use graceful handling at call site.
func (c *Client) GetNodeAptUpdate(node string) ([]PVEAptPackage, error) {
	var pkgs []PVEAptPackage
	if err := c.get(fmt.Sprintf("/nodes/%s/apt/update", node), &pkgs); err != nil {
		return []PVEAptPackage{}, err
	}
	return pkgs, nil
}

// GetNodeDisksList returns physical disks on the given node.
// Returns the actual error so callers can log it; use graceful handling at call site.
func (c *Client) GetNodeDisksList(node string) ([]PVEDisk, error) {
	var disks []PVEDisk
	if err := c.get(fmt.Sprintf("/nodes/%s/disks/list", node), &disks); err != nil {
		return []PVEDisk{}, err
	}
	return disks, nil
}

// PVENodeStatus is the response from GET /nodes/{node}/status.
type PVENodeStatus struct {
	CPU    float64 `json:"cpu"`
	Wait   float64 `json:"wait"` // IO wait fraction (0–1)
	Memory struct {
		Free  int64 `json:"free"`
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
	} `json:"memory"`
	Swap struct {
		Free  int64 `json:"free"`
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
	} `json:"swap"`
	RootFS struct {
		Avail int64 `json:"avail"`
		Free  int64 `json:"free"`
		Total int64 `json:"total"`
		Used  int64 `json:"used"`
	} `json:"rootfs"`
	Uptime int64           `json:"uptime"`
	KSM    json.RawMessage `json:"ksm,omitempty"` // PVE returns an object {shared:int64}, not a scalar
}

// GetNodeStatus returns detailed real-time status for a node.
func (c *Client) GetNodeStatus(node string) (*PVENodeStatus, error) {
	var status PVENodeStatus
	if err := c.get(fmt.Sprintf("/nodes/%s/status", node), &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// PVETaskLogLine is one line from GET /nodes/{node}/tasks/{upid}/log.
type PVETaskLogLine struct {
	N int    `json:"n"` // line number (1-based)
	T string `json:"t"` // text
}

// GetNodeTaskLog returns the log lines for a given task UPID.
func (c *Client) GetNodeTaskLog(node, upid string) ([]PVETaskLogLine, error) {
	var lines []PVETaskLogLine
	if err := c.get(fmt.Sprintf("/nodes/%s/tasks/%s/log", node, upid), &lines); err != nil {
		return nil, err
	}
	return lines, nil
}

// TriggerNodeAptUpdate triggers an `apt-get update` on the given node via
// POST /nodes/{node}/apt/update. Requires Sys.Modify privilege.
// Returns the task UPID on success.
func (c *Client) TriggerNodeAptUpdate(node string) (string, error) {
	url := c.baseURL + fmt.Sprintf("/nodes/%s/apt/update", node)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	var envelope struct {
		Data string `json:"data"` // UPID of the created task
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return envelope.Data, nil
}

// GetClusterBackup returns backup job configurations from /cluster/backup.
// Returns an empty slice (no error) if the cluster backup endpoint is unavailable.
func (c *Client) GetClusterBackup() ([]PVEBackupJob, error) {
	var jobs []PVEBackupJob
	if err := c.get("/cluster/backup", &jobs); err != nil {
		return []PVEBackupJob{}, nil
	}
	return jobs, nil
}

// ─── RRD metrics ──────────────────────────────────────────────────────────────

// PVERRDPoint is one data point from GET /nodes/{node}/rrddata.
// Numeric fields use *float64 to handle JSON null (absent data in PVE round-robin DB).
type PVERRDPoint struct {
	Time      int64    `json:"time"`
	CPU       *float64 `json:"cpu"`
	Mem       *float64 `json:"mem"`
	MaxMem    *float64 `json:"maxmem"`
	NetIn     *float64 `json:"netin"`
	NetOut    *float64 `json:"netout"`
	DiskRead  *float64 `json:"diskread"`
	DiskWrite *float64 `json:"diskwrite"`
	IOWait    *float64 `json:"iowait"`
}

// GetNodeRRDData returns time-series metrics for a node.
// timeframe must be one of: hour | day | week | month | year.
func (c *Client) GetNodeRRDData(node, timeframe string) ([]PVERRDPoint, error) {
	var points []PVERRDPoint
	if err := c.get(fmt.Sprintf("/nodes/%s/rrddata?timeframe=%s&cf=AVERAGE", node, timeframe), &points); err != nil {
		return nil, err
	}
	return points, nil
}

// ─── Services ─────────────────────────────────────────────────────────────────

// PVEService is an element returned by GET /nodes/{node}/services.
type PVEService struct {
	Name        string `json:"name"`
	State       string `json:"state"`       // running | stopped
	ActiveState string `json:"active-state"` // active | inactive | failed
	SubState    string `json:"sub-state"`
	Description string `json:"desc"`
	UnitState   string `json:"unit-state,omitempty"` // enabled | disabled | static
}

// GetNodeServices returns all services on the given node. Requires Sys.Audit.
func (c *Client) GetNodeServices(node string) ([]PVEService, error) {
	var services []PVEService
	if err := c.get(fmt.Sprintf("/nodes/%s/services", node), &services); err != nil {
		return nil, err
	}
	return services, nil
}

// ─── Guest network interfaces ─────────────────────────────────────────────────

// GuestNetworkIface is a normalized network interface for a guest (VM or LXC).
type GuestNetworkIface struct {
	Name string   `json:"name"` // e.g. eth0, ens18
	MAC  string   `json:"mac"`
	IPs  []string `json:"ips"` // CIDR notation, e.g. ["192.168.1.10/24"]
}

// GetVMNetworkInterfaces fetches guest network interfaces via the QEMU guest agent.
// Returns an error if the agent is not running; callers should handle this gracefully.
func (c *Client) GetVMNetworkInterfaces(node string, vmid int) ([]GuestNetworkIface, error) {
	type pveIPAddr struct {
		Type    string `json:"ip-address-type"` // ipv4 | ipv6
		Address string `json:"ip-address"`
		Prefix  int    `json:"prefix"`
	}
	type pveIface struct {
		Name     string      `json:"name"`
		MAC      string      `json:"hardware-address"`
		IPAddrs  []pveIPAddr `json:"ip-addresses"`
	}
	var raw []pveIface
	if err := c.get(fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, vmid), &raw); err != nil {
		return nil, err
	}
	var result []GuestNetworkIface
	for _, iface := range raw {
		if iface.Name == "lo" {
			continue
		}
		g := GuestNetworkIface{Name: iface.Name, MAC: iface.MAC}
		for _, ip := range iface.IPAddrs {
			if ip.Address == "" {
				continue
			}
			g.IPs = append(g.IPs, fmt.Sprintf("%s/%d", ip.Address, ip.Prefix))
		}
		result = append(result, g)
	}
	return result, nil
}

// GetLXCInterfaces fetches the network interfaces of an LXC container.
func (c *Client) GetLXCInterfaces(node string, vmid int) ([]GuestNetworkIface, error) {
	type pveIface struct {
		Name  string `json:"name"`
		MAC   string `json:"hwaddr"`
		Inet  string `json:"inet,omitempty"`  // IPv4 CIDR
		Inet6 string `json:"inet6,omitempty"` // IPv6 CIDR
	}
	var raw []pveIface
	if err := c.get(fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", node, vmid), &raw); err != nil {
		return nil, err
	}
	var result []GuestNetworkIface
	for _, iface := range raw {
		if iface.Name == "lo" {
			continue
		}
		g := GuestNetworkIface{Name: iface.Name, MAC: iface.MAC}
		if iface.Inet != "" {
			g.IPs = append(g.IPs, iface.Inet)
		}
		if iface.Inet6 != "" {
			g.IPs = append(g.IPs, iface.Inet6)
		}
		result = append(result, g)
	}
	return result, nil
}

// NodeServiceAction performs an action on a service. Requires Sys.Modify.
// action must be one of: start | stop | restart | reload.
func (c *Client) NodeServiceAction(node, service, action string) (string, error) {
	url := c.baseURL + fmt.Sprintf("/nodes/%s/services/%s/%s", node, service, action)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s=%s", c.tokenID, c.tokenSecret))
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, snippet)
	}

	var envelope struct {
		Data string `json:"data"` // UPID of the created task
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	return envelope.Data, nil
}
