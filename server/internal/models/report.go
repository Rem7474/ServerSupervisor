package models

import "time"

// ========== Agent Capabilities ==========

// AgentCapabilities tracks which collectors are active on the agent
type AgentCapabilities struct {
	Docker  bool `json:"docker"`   // Docker collector enabled
	APT     bool `json:"apt"`      // APT package collector enabled
	SMART   bool `json:"smart"`    // SMART disk health enabled
	CPUTemp bool `json:"cpu_temp"` // CPU temperature collector enabled
	WebLogs bool `json:"web_logs"` // Web access log parsing enabled
	Systemd bool `json:"systemd"`  // Systemd unit monitoring enabled
	Journal bool `json:"journal"`  // Journald log collection enabled
	Restic  bool `json:"restic"`   // Restic backup collector enabled
}

// ResticStatus mirrors agent/internal/collector.ResticStatus — the periodic,
// passive snapshot of Restic's state. Never contains resticconf content or
// resolved credential values.
type ResticStatus struct {
	Installed     bool       `json:"installed"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
	LastStatus    string     `json:"last_status,omitempty"`
	DurationSec   *int       `json:"duration_sec,omitempty"`
	FilesNew      *int       `json:"files_new,omitempty"`
	FilesChanged  *int       `json:"files_changed,omitempty"`
	BytesAdded    *int64     `json:"bytes_added,omitempty"`
	SnapshotID    string     `json:"snapshot_id,omitempty"`
	RepoSizeBytes *int64     `json:"repo_size_bytes,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	Source        string     `json:"source"`
}

// ========== Agent Heartbeat / Full Report ==========

// AgentReport is the full status report sent by the agent to the server
type AgentReport struct {
	HostID             string                    `json:"host_id"`
	AgentVersion       string                    `json:"agent_version"`
	Capabilities       *AgentCapabilities        `json:"capabilities,omitempty"` // Which collectors are enabled on this agent
	Metrics            *SystemMetrics            `json:"metrics,omitempty"`
	Docker             *DockerReport             `json:"docker,omitempty"`
	UnattendedUpgrades *UnattendedUpgradesStatus `json:"unattended_upgrades,omitempty"`
	WebLogs            *WebLogReport             `json:"web_logs,omitempty"`
	DockerNetworks     []DockerNetwork           `json:"docker_networks,omitempty"`
	ComposeProjects    []ComposeProject          `json:"compose_projects,omitempty"`
	DiskMetrics        []DiskMetrics             `json:"disk_metrics,omitempty"`
	DiskHealth         []DiskHealth              `json:"disk_health,omitempty"`
	CustomTasks        []CustomTaskSummary       `json:"custom_tasks,omitempty"`
	TasksConfigYAML    string                    `json:"tasks_config_yaml,omitempty"`
	Restic             *ResticStatus             `json:"restic,omitempty"`
	ResticProfiles     []string                  `json:"restic_profiles,omitempty"`
	Timestamp          time.Time                 `json:"timestamp"`
}
