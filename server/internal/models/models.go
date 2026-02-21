package models

import (
	"time"
)

// ========== Host (VM) ==========

type Host struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`         // User-defined name (e.g., "Prod Web Server")
	Hostname     string    `json:"hostname" db:"hostname"` // System hostname (auto-populated by agent)
	IPAddress    string    `json:"ip_address" db:"ip_address"`
	OS           string    `json:"os" db:"os"`                       // Auto-populated by agent
	AgentVersion string    `json:"agent_version" db:"agent_version"` // Agent version
	APIKey       string    `json:"-" db:"api_key"`
	Status       string    `json:"status" db:"status"` // online, offline, warning
	LastSeen     time.Time `json:"last_seen" db:"last_seen"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

type HostRegistration struct {
	Name      string `json:"name" binding:"required"`
	IPAddress string `json:"ip_address" binding:"required"`
}

type HostUpdate struct {
	Name         *string `json:"name"`
	Hostname     *string `json:"hostname"`
	IPAddress    *string `json:"ip_address"`
	OS           *string `json:"os"`
	AgentVersion *string `json:"agent_version"`
}

// ========== System Metrics ==========

type SystemMetrics struct {
	ID        int64     `json:"id" db:"id"`
	HostID    string    `json:"host_id" db:"host_id"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`

	// CPU
	CPUUsagePercent float64 `json:"cpu_usage_percent" db:"cpu_usage_percent"`
	CPUCores        int     `json:"cpu_cores" db:"cpu_cores"`
	CPUModel        string  `json:"cpu_model" db:"cpu_model"`
	LoadAvg1        float64 `json:"load_avg_1" db:"load_avg_1"`
	LoadAvg5        float64 `json:"load_avg_5" db:"load_avg_5"`
	LoadAvg15       float64 `json:"load_avg_15" db:"load_avg_15"`

	// Memory
	MemoryTotal   uint64  `json:"memory_total" db:"memory_total"`
	MemoryUsed    uint64  `json:"memory_used" db:"memory_used"`
	MemoryFree    uint64  `json:"memory_free" db:"memory_free"`
	MemoryPercent float64 `json:"memory_percent" db:"memory_percent"`
	SwapTotal     uint64  `json:"swap_total" db:"swap_total"`
	SwapUsed      uint64  `json:"swap_used" db:"swap_used"`

	// Disk
	Disks []DiskInfo `json:"disks" db:"-"`

	// Network
	NetworkRxBytes uint64 `json:"network_rx_bytes" db:"network_rx_bytes"`
	NetworkTxBytes uint64 `json:"network_tx_bytes" db:"network_tx_bytes"`

	// System
	Uptime   uint64 `json:"uptime" db:"uptime"`
	OS       string `json:"os" db:"-"`
	Hostname string `json:"hostname" db:"hostname"`
}

type DiskInfo struct {
	ID          int64   `json:"id" db:"id"`
	MetricsID   int64   `json:"-" db:"metrics_id"`
	MountPoint  string  `json:"mount_point" db:"mount_point"`
	Device      string  `json:"device" db:"device"`
	FSType      string  `json:"fs_type" db:"fs_type"`
	TotalBytes  uint64  `json:"total_bytes" db:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes" db:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes" db:"free_bytes"`
	UsedPercent float64 `json:"used_percent" db:"used_percent"`
}

// SystemMetricsSummary is a global aggregated view used for dashboard charts.
type SystemMetricsSummary struct {
	Timestamp   time.Time `json:"timestamp" db:"timestamp"`
	CPUAvg      float64   `json:"cpu_avg" db:"cpu_avg"`
	MemoryAvg   float64   `json:"memory_avg" db:"memory_avg"`
	SampleCount int       `json:"sample_count" db:"sample_count"`
}

// ========== Docker Containers ==========

type DockerContainer struct {
	ID          string            `json:"id" db:"id"`
	HostID      string            `json:"host_id" db:"host_id"`
	Hostname    string            `json:"hostname" db:"hostname"` // Host's hostname for display
	ContainerID string            `json:"container_id" db:"container_id"`
	Name        string            `json:"name" db:"name"`
	Image       string            `json:"image" db:"image"`
	ImageTag    string            `json:"image_tag" db:"image_tag"`
	ImageID     string            `json:"image_id" db:"image_id"`
	State       string            `json:"state" db:"state"` // running, stopped, paused, etc.
	Status      string            `json:"status" db:"status"`
	Created     time.Time         `json:"created" db:"created"`
	Ports       string            `json:"ports" db:"ports"`
	Labels      map[string]string `json:"labels" db:"-"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
}

type DockerReport struct {
	HostID     string            `json:"host_id"`
	Containers []DockerContainer `json:"containers"`
	Timestamp  time.Time         `json:"timestamp"`
}

// ========== APT Updates ==========

type CVEInfo struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Package  string `json:"package"`
}

type AptStatus struct {
	ID              int64     `json:"id" db:"id"`
	HostID          string    `json:"host_id" db:"host_id"`
	LastUpdate      time.Time `json:"last_update" db:"last_update"`
	LastUpgrade     time.Time `json:"last_upgrade" db:"last_upgrade"`
	PendingPackages int       `json:"pending_packages" db:"pending_packages"`
	PackageList     string    `json:"package_list" db:"package_list"` // JSON array of package names
	SecurityUpdates int       `json:"security_updates" db:"security_updates"`
	CVEList         string    `json:"cve_list" db:"cve_list"` // JSON array of CVEInfo
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

type AptCommand struct {
	ID          int64      `json:"id" db:"id"`
	HostID      string     `json:"host_id" db:"host_id"`
	Command     string     `json:"command" db:"command"` // update, upgrade, dist-upgrade
	Status      string     `json:"status" db:"status"`   // pending, running, completed, failed
	Output      string     `json:"output" db:"output"`
	TriggeredBy string     `json:"triggered_by" db:"triggered_by"` // Username who triggered this
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	StartedAt   *time.Time `json:"started_at" db:"started_at"`
	EndedAt     *time.Time `json:"ended_at" db:"ended_at"`
}

type AptCommandRequest struct {
	HostIDs []string `json:"host_ids" binding:"required"`
	Command string   `json:"command" binding:"required,oneof=update upgrade dist-upgrade"`
}

// ========== GitHub Release Tracking ==========

type TrackedRepo struct {
	ID            int64     `json:"id" db:"id"`
	Owner         string    `json:"owner" db:"owner"`
	Repo          string    `json:"repo" db:"repo"`
	DisplayName   string    `json:"display_name" db:"display_name"`
	LatestVersion string    `json:"latest_version" db:"latest_version"`
	LatestDate    time.Time `json:"latest_date" db:"latest_date"`
	ReleaseURL    string    `json:"release_url" db:"release_url"`
	DockerImage   string    `json:"docker_image" db:"docker_image"` // associated docker image name for comparison
	CheckedAt     time.Time `json:"checked_at" db:"checked_at"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type TrackedRepoCreate struct {
	Owner       string `json:"owner" binding:"required"`
	Repo        string `json:"repo" binding:"required"`
	DisplayName string `json:"display_name"`
	DockerImage string `json:"docker_image"` // e.g. "homeassistant/home-assistant"
}

type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
}

// ========== Version Comparison ==========

type VersionComparison struct {
	DockerImage    string `json:"docker_image"`
	RunningVersion string `json:"running_version"`
	LatestVersion  string `json:"latest_version"`
	IsUpToDate     bool   `json:"is_up_to_date"`
	RepoOwner      string `json:"repo_owner"`
	RepoName       string `json:"repo_name"`
	ReleaseURL     string `json:"release_url"`
	HostID         string `json:"host_id"`
	Hostname       string `json:"hostname"`
}

// ========== Agent Heartbeat / Full Report ==========

type AgentReport struct {
	HostID       string         `json:"host_id"`
	AgentVersion string         `json:"agent_version"`
	Metrics      *SystemMetrics `json:"metrics,omitempty"`
	Docker       *DockerReport  `json:"docker,omitempty"`
	AptStatus    *AptStatus     `json:"apt_status,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}

// ========== Commands (server → agent) ==========

type PendingCommand struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"`    // apt_update, apt_upgrade, apt_dist_upgrade
	Payload string `json:"payload"` // JSON payload if needed
}

type CommandResult struct {
	CommandID int64      `json:"command_id"`
	Status    string     `json:"status"` // completed, failed
	Output    string     `json:"output"`
	AptStatus *AptStatus `json:"apt_status,omitempty"` // Full APT status after update/upgrade
}

// ========== Auth ==========

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"` // Optional: TOTP code if user has MFA enabled
}

type LoginResponse struct {
	Token      string    `json:"token"`
	ExpiresAt  time.Time `json:"expires_at"`
	Role       string    `json:"role"`
	RequireMFA bool      `json:"require_mfa"` // True if needs TOTP step
}

type TOTPSecretResponse struct {
	Secret      string   `json:"secret"`       // Base32 encoded secret
	QRCode      string   `json:"qr_code"`      // Data URL for QR code
	BackupCodes []string `json:"backup_codes"` // 10 single-use backup codes
}

type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`      // admin, operator, viewer
	TOTPSecret   string    `json:"-" db:"totp_secret"`  // Encrypted TOTP secret (empty if MFA disabled)
	BackupCodes  string    `json:"-" db:"backup_codes"` // JSON array of backup codes (hashed)
	MFAEnabled   bool      `json:"mfa_enabled" db:"mfa_enabled"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// ========== RBAC & Permissions ==========

const (
	RoleAdmin    = "admin"    // Full access
	RoleOperator = "operator" // Can launch APT commands + read all
	RoleViewer   = "viewer"   // Read-only
)

// ========== Audit Log (APT & Admin Actions) ==========

type AuditLog struct {
	ID        int64     `json:"id" db:"id"`
	Username  string    `json:"username" db:"username"`     // Who
	Action    string    `json:"action" db:"action"`         // What (apt_update, apt_upgrade, user_created, etc.)
	HostID    string    `json:"host_id" db:"host_id"`       // On which host (nullable)
	IPAddress string    `json:"ip_address" db:"ip_address"` // Client IP
	Details   string    `json:"details" db:"details"`       // JSON payload (command output, new privileges, etc.)
	Status    string    `json:"status" db:"status"`         // pending, completed, failed
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ========== Metrics Aggregation (for downsampling) ==========

// MetricsAggregate stores downsampled metrics (5-min, hourly, daily)
type MetricsAggregate struct {
	ID              int64     `json:"id" db:"id"`
	HostID          string    `json:"host_id" db:"host_id"`
	AggregationType string    `json:"aggregation_type" db:"aggregation_type"` // 5min, hour, day
	Timestamp       time.Time `json:"timestamp" db:"timestamp"`               // Start of the interval

	// Metrics (averages for the period)
	CPUUsageAvg    float64 `json:"cpu_usage_avg" db:"cpu_usage_avg"`
	CPUUsageMax    float64 `json:"cpu_usage_max" db:"cpu_usage_max"`
	MemoryUsageAvg uint64  `json:"memory_usage_avg" db:"memory_usage_avg"`
	MemoryUsageMax uint64  `json:"memory_usage_max" db:"memory_usage_max"`
	DiskUsageAvg   float64 `json:"disk_usage_avg" db:"disk_usage_avg"`
	NetworkRxBytes uint64  `json:"network_rx_bytes" db:"network_rx_bytes"`
	NetworkTxBytes uint64  `json:"network_tx_bytes" db:"network_tx_bytes"`

	SampleCount int       `json:"sample_count" db:"sample_count"` // How many raw samples in period
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
