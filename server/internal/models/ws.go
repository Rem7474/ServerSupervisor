package models

import "time"

// ===== WebSocket payloads =====
// Typed messages pushed over the per-page WebSocket endpoints and the live
// command-stream hub. Each snapshot mirrors the gin.H payload its WSHandler used
// to build; the Type field is the discriminator the frontend switches on.

// DashboardHostMetrics is the lean per-host metric subset the dashboard actually
// renders (CPU %, memory %, uptime). The full SystemMetrics carries ~20 more
// fields (temperatures, load, swap, network, cpu model, …) consumed only by the
// host detail view, so the dashboard snapshot — re-pushed every 10s per client —
// ships this instead of the whole struct per host.
type DashboardHostMetrics struct {
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	MemoryPercent   float64 `json:"memory_percent"`
	Uptime          uint64  `json:"uptime"`
}

// WSDashboardSnapshot is broadcast on the dashboard endpoint (type "dashboard").
type WSDashboardSnapshot struct {
	Type               string                           `json:"type"`
	Hosts              []Host                           `json:"hosts"`
	HostMetrics        map[string]*DashboardHostMetrics `json:"host_metrics"`
	VersionComparisons []VersionComparison              `json:"version_comparisons"`
	AptPending         int                              `json:"apt_pending"`
	AptPendingHosts    map[string]int                   `json:"apt_pending_hosts"`
	DiskUsage          map[string]float64               `json:"disk_usage"`
	ProxmoxNodes       []ProxmoxNode                    `json:"proxmox_nodes"`
	ProxmoxLinks       []ProxmoxGuestLink               `json:"proxmox_links"`
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

// ===== Browser notifications (GET /api/v1/ws/notifications) =====
// Each push is one of the message structs below. They replace the previous
// untyped map[string]interface{} payloads; the frontend switches on `type`.

// WSAlertIncidentNotification is the nested payload of a "new_alert" message.
type WSAlertIncidentNotification struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"` // "alert_incident"
	RuleID        int64      `json:"rule_id"`
	HostID        string     `json:"host_id"`
	HostName      string     `json:"host_name"`
	RuleName      string     `json:"rule_name"`
	Metric        string     `json:"metric"`
	Value         float64    `json:"value"`
	TriggeredAt   time.Time  `json:"triggered_at"`
	ResolvedAt    *time.Time `json:"resolved_at"`
	BrowserNotify bool       `json:"browser_notify"`
}

// WSNewAlertMessage is pushed when an alert incident fires (type "new_alert").
type WSNewAlertMessage struct {
	Type         string                      `json:"type"`
	Notification WSAlertIncidentNotification `json:"notification"`
}

// WSAlertIncidentUpdate is a lightweight flat event (no nested notification) that
// lets the frontend refresh its incidents list without polling (type
// "alert_incident_update").
type WSAlertIncidentUpdate struct {
	Type   string `json:"type"`
	Event  string `json:"event"` // "fired" | "resolved"
	RuleID int64  `json:"rule_id"`
	HostID string `json:"host_id"`
}

// WSWebhookNotification is the nested payload of a "webhook_execution" message.
type WSWebhookNotification struct {
	WebhookID   string    `json:"webhook_id"`
	WebhookName string    `json:"webhook_name"`
	Status      string    `json:"status"`
	TriggeredAt time.Time `json:"triggered_at"`
}

// WSWebhookExecutionMessage is pushed when a git-webhook command completes
// (type "webhook_execution").
type WSWebhookExecutionMessage struct {
	Type         string                `json:"type"`
	Notification WSWebhookNotification `json:"notification"`
}

// WSReleaseTrackerNotification is the nested payload of release-tracker messages.
// version/release_url/release_name/label are populated only for
// "release_tracker_detected".
type WSReleaseTrackerNotification struct {
	TrackerID   string    `json:"tracker_id"`
	TrackerName string    `json:"tracker_name"`
	TrackerType string    `json:"tracker_type"`
	Version     string    `json:"version,omitempty"`
	ReleaseURL  string    `json:"release_url,omitempty"`
	ReleaseName string    `json:"release_name,omitempty"`
	Status      string    `json:"status"`
	Label       string    `json:"label,omitempty"`
	TriggeredAt time.Time `json:"triggered_at"`
}

// WSReleaseTrackerMessage is pushed on release detection / execution completion
// (type "release_tracker_detected" | "release_tracker_execution").
type WSReleaseTrackerMessage struct {
	Type         string                       `json:"type"`
	Notification WSReleaseTrackerNotification `json:"notification"`
}

// WSUUNotification is the nested payload of an "unattended_upgrade" message.
type WSUUNotification struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"` // "unattended_upgrade"
	HostID        string    `json:"host_id"`
	HostName      string    `json:"host_name"`
	Packages      []string  `json:"packages"`
	PkgCount      int       `json:"pkg_count"`
	RunAt         time.Time `json:"run_at"`
	BrowserNotify bool      `json:"browser_notify"`
	Title         string    `json:"title"`
	Message       string    `json:"message"`
}

// WSUnattendedUpgradeMessage is pushed after an unattended-upgrades run
// (type "unattended_upgrade").
type WSUnattendedUpgradeMessage struct {
	Type         string           `json:"type"`
	Notification WSUUNotification `json:"notification"`
}
