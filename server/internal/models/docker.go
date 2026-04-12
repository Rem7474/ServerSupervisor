package models

import "time"

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
	ImageDigest string            `json:"image_digest" db:"image_digest"` // manifest sha256 (RepoDigest)
	State       string            `json:"state" db:"state"`               // running, stopped, paused, etc.
	Status      string            `json:"status" db:"status"`
	Created     time.Time         `json:"created" db:"created"`
	Ports       string            `json:"ports" db:"ports"`
	Labels      map[string]string `json:"labels" db:"-"`
	EnvVars     map[string]string `json:"env_vars" db:"-"`
	Volumes     []string          `json:"volumes" db:"-"`
	Networks    []string          `json:"networks" db:"-"`
	NetRxBytes  uint64            `json:"net_rx_bytes" db:"net_rx_bytes"`
	NetTxBytes  uint64            `json:"net_tx_bytes" db:"net_tx_bytes"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
}

type DockerReport struct {
	HostID     string            `json:"host_id"`
	Containers []DockerContainer `json:"containers"`
	Timestamp  time.Time         `json:"timestamp"`
}

// ========== Docker Compose Projects ==========

type ComposeProject struct {
	ID         string    `json:"id" db:"id"`
	HostID     string    `json:"host_id" db:"host_id"`
	Hostname   string    `json:"hostname" db:"hostname"`
	Name       string    `json:"name" db:"name"`
	WorkingDir string    `json:"working_dir" db:"working_dir"`
	ConfigFile string    `json:"config_file" db:"config_file"`
	Services   []string  `json:"services" db:"-"`
	RawConfig  string    `json:"raw_config" db:"raw_config"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
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

// ========== Version Comparison ==========

type VersionComparison struct {
	TrackerID       string `json:"tracker_id"`
	DockerImage     string `json:"docker_image"`
	RunningVersion  string `json:"running_version"`
	LatestVersion   string `json:"latest_version"`
	IsUpToDate      bool   `json:"is_up_to_date"`
	UpdateConfirmed bool   `json:"update_confirmed"` // true when digest comparison confirms an update (even if running version is unknown)
	ContainerCount  int    `json:"container_count"`  // number of containers using this image on the host
	RepoOwner       string `json:"repo_owner"`
	RepoName        string `json:"repo_name"`
	ReleaseURL      string `json:"release_url"`
	HostID          string `json:"host_id"`
	Hostname        string `json:"hostname"`
}

// ========== Network Topology (Docker Networks) ==========

// DockerNetwork represents a Docker network and its connected containers
type DockerNetwork struct {
	ID           string    `json:"id" db:"id"`
	HostID       string    `json:"host_id" db:"host_id"`
	NetworkID    string    `json:"network_id" db:"network_id"`
	Name         string    `json:"name" db:"name"`
	Driver       string    `json:"driver" db:"driver"`   // bridge, overlay, host, none
	Scope        string    `json:"scope" db:"scope"`     // local, swarm
	ContainerIDs []string  `json:"container_ids" db:"-"` // Stored as JSONB, not queried directly
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// ContainerEnv represents container environment variables for topology inference
type ContainerEnv struct {
	ContainerName string            `json:"container_name"`
	EnvVars       map[string]string `json:"env_vars"`
}

// ========== APT Updates (belongs to host, included here for convenience) ==========

type CVEInfo struct {
	ID             string  `json:"id"`
	Severity       string  `json:"severity"`        // Mapped from UbuntuPriority
	UbuntuPriority string  `json:"ubuntu_priority"` // Raw Ubuntu priority (critical/high/medium/low/negligible)
	CVSSScore      float64 `json:"cvss_score"`      // CVSS v3 score (0 if unavailable)
	CVSSVector     string  `json:"cvss_vector,omitempty"`
	Package        string  `json:"package"`
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

// AptCVESummary aggregates CVE severity counts across all hosts.
type AptCVESummary struct {
	HostsWithCritical int `json:"hosts_with_critical"`
	HostsWithHigh     int `json:"hosts_with_high"`
	CriticalCount     int `json:"critical_count"`
	HighCount         int `json:"high_count"`
	MediumCount       int `json:"medium_count"`
	TotalCVECount     int `json:"total_cve_count"`
}

type AptCommandRequest struct {
	HostIDs []string `json:"host_ids" binding:"required"`
	Command string   `json:"command" binding:"required,oneof=update upgrade dist-upgrade"`
}
