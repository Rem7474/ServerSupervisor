package models

import "time"

// ===== WebSocket payloads =====
// Typed messages pushed over the per-page WebSocket endpoints and the live
// command-stream hub. Each snapshot mirrors the gin.H payload its WSHandler used
// to build; the Type field is the discriminator the frontend switches on.

// WSDashboardSnapshot is broadcast on the dashboard endpoint (type "dashboard").
type WSDashboardSnapshot struct {
	Type               string                    `json:"type"`
	Hosts              []Host                    `json:"hosts"`
	HostMetrics        map[string]*SystemMetrics `json:"host_metrics"`
	VersionComparisons []VersionComparison       `json:"version_comparisons"`
	AptPending         int                       `json:"apt_pending"`
	AptPendingHosts    map[string]int            `json:"apt_pending_hosts"`
	DiskUsage          map[string]float64        `json:"disk_usage"`
	ProxmoxNodes       []ProxmoxNode             `json:"proxmox_nodes"`
	ProxmoxLinks       []ProxmoxGuestLink        `json:"proxmox_links"`
}

// WSHostSnapshot is broadcast on the host-detail endpoint (type "host_detail").
type WSHostSnapshot struct {
	Type               string                `json:"type"`
	Host               *Host                 `json:"host"`
	Metrics            *SystemMetrics        `json:"metrics"`
	Containers         []DockerContainer     `json:"containers"`
	AptStatus          *AptStatus            `json:"apt_status"`
	AptHistory         []RemoteCommand       `json:"apt_history"`
	UUStatus           *UnattendedUpgradesDB `json:"uu_status"`
	UURuns             []UURun               `json:"uu_runs"`
	AuditLogs          []AuditLog            `json:"audit_logs"`
	VersionComparisons []VersionComparison   `json:"version_comparisons"`
	ProxmoxLink        *ProxmoxGuestLink     `json:"proxmox_link"`
}

// WSDockerSnapshot is broadcast on the docker endpoint (type "docker").
type WSDockerSnapshot struct {
	Type               string              `json:"type"`
	Containers         []DockerContainer   `json:"containers"`
	ComposeProjects    []ComposeProject    `json:"compose_projects"`
	VersionComparisons []VersionComparison `json:"version_comparisons"`
}

// WSNetworkSnapshot is broadcast on the network endpoint (type "network").
type WSNetworkSnapshot struct {
	Type       string                 `json:"type"`
	Hosts      []NetworkHost          `json:"hosts"`
	Containers []NetworkContainer     `json:"containers"`
	Config     *NetworkTopologyConfig `json:"config"`
	UpdatedAt  time.Time              `json:"updated_at"`
}

// WSAptSnapshot is broadcast on the apt endpoint (type "apt").
type WSAptSnapshot struct {
	Type         string                     `json:"type"`
	Hosts        []Host                     `json:"hosts"`
	AptStatuses  map[string]*AptStatus      `json:"apt_statuses"`
	AptHistories map[string][]RemoteCommand `json:"apt_histories"`
}

// ===== Live command stream (GET /api/v1/ws/commands/stream/:id) =====

// WSCommandStreamInit is the first message sent on connect: the command's current
// status plus any output buffered so far (empty while still running).
type WSCommandStreamInit struct {
	Type      string `json:"type"` // "cmd_stream_init"
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Command   string `json:"command"`
	Output    string `json:"output"`
}

// WSCommandStreamChunk carries one chunk of live command output (type "cmd_stream").
type WSCommandStreamChunk struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
	Chunk     string `json:"chunk"`
}

// WSCommandStatusUpdate signals a command status transition (type
// "cmd_status_update"); Output is set on terminal statuses (completed/failed).
type WSCommandStatusUpdate struct {
	Type      string `json:"type"`
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Output    string `json:"output,omitempty"`
}
