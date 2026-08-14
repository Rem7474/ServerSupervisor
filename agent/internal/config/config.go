package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Server connection
	ServerURL string `yaml:"server_url"`
	APIKey    string `yaml:"api_key"`

	// Intervals
	ReportInterval     int `yaml:"report_interval"` // seconds
	MaxReportBodyBytes int `yaml:"max_report_body_bytes"`

	// Features
	CollectDocker         bool     `yaml:"collect_docker"`
	CollectAPT            bool     `yaml:"collect_apt"`
	CollectSMART          bool     `yaml:"collect_smart"`
	CollectCPUTemperature bool     `yaml:"collect_cpu_temperature"`
	CollectWebLogs        bool     `yaml:"collect_web_logs"`
	WebLogsLogPaths       []string `yaml:"web_logs_log_paths"`
	WebLogsTailLines      int      `yaml:"web_logs_tail_lines"`
	WebLogsTopN           int      `yaml:"web_logs_top_n"`
	WebLogsRequestsLimit  int      `yaml:"web_logs_requests_limit"`
	WebLogsCursorFile     string   `yaml:"web_logs_cursor_file"`

	// CrowdSec correlation
	CollectCrowdSecCorrelation bool   `yaml:"collect_crowdsec_correlation"`
	CrowdSecConnectionString   string `yaml:"crowdsec_connection_string"`
	CrowdSecAPIKey             string `yaml:"crowdsec_api_key"`
	CrowdSecAlertsMachineID    string `yaml:"crowdsec_alerts_machine_id"`
	CrowdSecAlertsPassword     string `yaml:"crowdsec_alerts_password"`

	// Network flows ("top talkers"): per-host bandwidth by remote peer,
	// derived from conntrack accounting. CollectNetworkFlows only controls
	// whether the collector runs at all — kernel-level availability
	// (nf_conntrack_acct) is detected and reported separately every cycle
	// (see collector.CollectNetworkFlows), never configured from here.
	CollectNetworkFlows bool `yaml:"collect_network_flows"`
	NetworkFlowsTopN    int  `yaml:"network_flows_top_n"`
	// NetworkFlowsL7Capture opts into a bounded packet-sampling pass that reads
	// the TLS SNI hostname off outbound handshakes, so a talker can be labelled
	// with the domain it actually contacted instead of just an IP and a port.
	// Default false: it needs CAP_NET_RAW and reads packet headers off the
	// wire, which is a materially bigger ask than the rest of the agent's
	// /proc-and-netlink collection. Missing capability degrades to the UI's
	// well-known-port heuristic — it never fails a report cycle.
	NetworkFlowsL7Capture bool `yaml:"network_flows_l7_capture"`

	// Restic backup supervision. Only paths and feature flags live here —
	// credentials (repository password, storage backend keys) stay in the
	// resticconf file on disk (ResticConfPath) and are never read into this
	// struct or sent to the server.
	CollectRestic        bool   `yaml:"collect_restic"`
	ResticBin            string `yaml:"restic_bin"`
	ResticConfPath       string `yaml:"restic_conf_path"`
	ResticRunScriptPath  string `yaml:"restic_run_script_path"`
	ResticStatusFilePath string `yaml:"restic_status_file_path"`
	// ResticProfileConfigPath points at resticprofile.yaml (profile
	// definitions only — repository password/backend keys live in
	// resticconf, not this file). Read locally to report the list of
	// available profile names; never forwarded to the server as raw YAML.
	ResticProfileConfigPath        string  `yaml:"restic_profile_config_path"`
	ResticEnableProgress           bool    `yaml:"restic_enable_progress"`
	ResticProgressFPS              float64 `yaml:"restic_progress_fps"`
	ResticBackupIdleTimeoutMinutes int     `yaml:"restic_backup_idle_timeout_minutes"`

	// Low-latency command push: an optional, additive WebSocket connection the
	// agent opens to the server purely to be nudged to poll immediately when a
	// command is dispatched, instead of waiting out report_interval. Disabling
	// it only removes that latency win — command delivery still works exactly
	// as before via the regular poll cycle.
	DisableWSPush bool `yaml:"disable_ws_push"`

	// TLS
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`

	// Logging
	LogLevel  string `yaml:"log_level"`  // debug|info|warn|error (default info; --verbose forces debug)
	LogFormat string `yaml:"log_format"` // text|json (default text)
}

// WebLogGlobs returns configured web access log globs.
func (c *Config) WebLogGlobs() []string {
	return c.WebLogsLogPaths
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Environment variable overrides (for Docker/Kubernetes deployments)
	if env := os.Getenv("SUPERVISOR_SERVER_URL"); env != "" {
		cfg.ServerURL = env
	}
	if env := os.Getenv("SUPERVISOR_API_KEY"); env != "" {
		cfg.APIKey = env
	}
	if env := os.Getenv("SUPERVISOR_REPORT_INTERVAL"); env != "" {
		if interval, err := strconv.Atoi(env); err == nil {
			cfg.ReportInterval = interval
		}
	}
	if env := os.Getenv("SUPERVISOR_MAX_REPORT_BODY_BYTES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.MaxReportBodyBytes = n
		}
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_DOCKER"); env != "" {
		cfg.CollectDocker = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_APT"); env != "" {
		cfg.CollectAPT = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_SMART"); env != "" {
		cfg.CollectSMART = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_CPU_TEMPERATURE"); env != "" {
		cfg.CollectCPUTemperature = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_WEB_LOGS"); env != "" {
		cfg.CollectWebLogs = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_WEB_LOGS_LOG_PATHS"); env != "" {
		parts := []string{}
		for _, p := range strings.Split(env, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				parts = append(parts, p)
			}
		}
		if len(parts) > 0 {
			cfg.WebLogsLogPaths = parts
		}
	}
	if env := os.Getenv("SUPERVISOR_WEB_LOGS_TAIL_LINES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.WebLogsTailLines = n
		}
	}
	if env := os.Getenv("SUPERVISOR_WEB_LOGS_TOP_N"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.WebLogsTopN = n
		}
	}
	if env := os.Getenv("SUPERVISOR_WEB_LOGS_REQUESTS_LIMIT"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.WebLogsRequestsLimit = n
		}
	}
	if env := os.Getenv("SUPERVISOR_WEB_LOGS_CURSOR_FILE"); env != "" {
		cfg.WebLogsCursorFile = strings.TrimSpace(env)
	}

	// Backward-compatible env aliases.
	if env := os.Getenv("SUPERVISOR_COLLECT_BOT_DETECTION"); env != "" {
		cfg.CollectWebLogs = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_NPM_ANALYTICS"); env != "" {
		cfg.CollectWebLogs = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_BOT_DETECTION_LOG_PATHS"); env != "" {
		cfg.WebLogsLogPaths = splitCSV(env)
	}
	if env := os.Getenv("SUPERVISOR_NPM_ANALYTICS_LOG_PATHS"); env != "" && len(cfg.WebLogsLogPaths) == 0 {
		cfg.WebLogsLogPaths = splitCSV(env)
	}
	if env := os.Getenv("SUPERVISOR_BOT_DETECTION_TAIL_LINES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.WebLogsTailLines = n
		}
	}
	if env := os.Getenv("SUPERVISOR_NPM_ANALYTICS_TAIL_LINES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.WebLogsTailLines = n
		}
	}
	if env := os.Getenv("SUPERVISOR_BOT_DETECTION_TOP_N"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.WebLogsTopN = n
		}
	}
	if env := os.Getenv("SUPERVISOR_NPM_ANALYTICS_TOP_N"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.WebLogsTopN = n
		}
	}
	if env := os.Getenv("SUPERVISOR_DISABLE_WS_PUSH"); env != "" {
		cfg.DisableWSPush = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_INSECURE_SKIP_VERIFY"); env != "" {
		cfg.InsecureSkipVerify = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_LOG_LEVEL"); env != "" {
		cfg.LogLevel = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_LOG_FORMAT"); env != "" {
		cfg.LogFormat = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_CROWDSEC_CORRELATION"); env != "" {
		cfg.CollectCrowdSecCorrelation = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_CROWDSEC_CONNECTION_STRING"); env != "" {
		cfg.CrowdSecConnectionString = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_CROWDSEC_API_KEY"); env != "" {
		cfg.CrowdSecAPIKey = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_CROWDSEC_ALERTS_MACHINE_ID"); env != "" {
		cfg.CrowdSecAlertsMachineID = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_CROWDSEC_ALERTS_PASSWORD"); env != "" {
		cfg.CrowdSecAlertsPassword = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_NETWORK_FLOWS"); env != "" {
		cfg.CollectNetworkFlows = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_NETWORK_FLOWS_TOP_N"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.NetworkFlowsTopN = n
		}
	}
	if env := os.Getenv("SUPERVISOR_NETWORK_FLOWS_L7_CAPTURE"); env != "" {
		cfg.NetworkFlowsL7Capture = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_RESTIC"); env != "" {
		cfg.CollectRestic = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_BIN"); env != "" {
		cfg.ResticBin = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_CONF_PATH"); env != "" {
		cfg.ResticConfPath = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_RUN_SCRIPT_PATH"); env != "" {
		cfg.ResticRunScriptPath = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_STATUS_FILE_PATH"); env != "" {
		cfg.ResticStatusFilePath = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_PROFILE_CONFIG_PATH"); env != "" {
		cfg.ResticProfileConfigPath = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_ENABLE_PROGRESS"); env != "" {
		cfg.ResticEnableProgress = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_PROGRESS_FPS"); env != "" {
		if f, err := strconv.ParseFloat(env, 64); err == nil && f > 0 {
			cfg.ResticProgressFPS = f
		}
	}
	if env := os.Getenv("SUPERVISOR_RESTIC_BACKUP_IDLE_TIMEOUT_MINUTES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.ResticBackupIdleTimeoutMinutes = n
		}
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required (set in config or SUPERVISOR_API_KEY env var)")
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		ServerURL:             "http://localhost:8080",
		ReportInterval:        30,
		MaxReportBodyBytes:    3 * 1024 * 1024,
		CollectDocker:         true,
		CollectAPT:            true,
		CollectSMART:          false,
		CollectCPUTemperature: false,
		CollectWebLogs:        false,
		WebLogsLogPaths: []string{
			"/var/log/nginx/access.log",
			"/var/log/apache2/access.log",
			"/var/log/httpd/access_log",
			"/data/logs/proxy-host-*_access.log",
		},
		WebLogsTailLines:               5000,
		WebLogsTopN:                    10,
		WebLogsRequestsLimit:           200,
		WebLogsCursorFile:              "/var/lib/serversupervisor/web_logs_cursor.json",
		CollectCrowdSecCorrelation:     false,
		CrowdSecConnectionString:       "http://localhost:8080",
		CrowdSecAPIKey:                 "",
		CrowdSecAlertsMachineID:        "",
		CrowdSecAlertsPassword:         "",
		DisableWSPush:                  false,
		LogLevel:                       "info",
		LogFormat:                      "text",
		CollectNetworkFlows:            true,
		NetworkFlowsTopN:               50,
		NetworkFlowsL7Capture:          false,
		CollectRestic:                  false,
		ResticEnableProgress:           true,
		ResticProgressFPS:              0.1,
		ResticBackupIdleTimeoutMinutes: 20,
	}
}

func DefaultConfigFile() string {
	return DefaultConfigFileWithOverrides("", "")
}

func DefaultConfigFileWithOverrides(serverURL, apiKey string) string {
	raw := `# ServerSupervisor Agent Configuration
server_url: "http://your-server:8080"
api_key: "your-api-key-here"

# Report interval in seconds
report_interval: 30

# Max HTTP JSON report payload size sent by agent (bytes).
# If exceeded, agent trims web_logs.requests before send.
max_report_body_bytes: 3145728

# Enable Docker container monitoring
collect_docker: true

# Enable APT update monitoring
collect_apt: true

# Enable SMART disk health monitoring (requires smartmontools)
# Disable on VMs or systems without smartctl
collect_smart: false

# Enable CPU temperature collection from thermal sensors (/sys, hwmon, sensors)
collect_cpu_temperature: false

# Parse web access logs once and derive traffic + threat summaries.
collect_web_logs: false

# Glob paths to parse (supports wildcards)
web_logs_log_paths:
  - "/var/log/nginx/access.log"
  - "/var/log/apache2/access.log"
  - "/var/log/httpd/access_log"
  - "/data/logs/proxy-host-*_access.log"

# Number of latest log lines to inspect per file
web_logs_tail_lines: 5000

# Number of top domains / IPs / paths returned
web_logs_top_n: 10

# Max number of raw requests embedded in each report.
web_logs_requests_limit: 200

# Incremental cursor state file used to avoid re-reading already processed lines.
web_logs_cursor_file: "/var/lib/serversupervisor/web_logs_cursor.json"

# Logging
# log_level: debug|info|warn|error (default info). The --verbose flag forces debug.
# log_format: text|json (default text — switch to json for centralized ingestion).
log_level: "info"
log_format: "text"

# Low-latency command push: an optional WebSocket connection the agent opens
# to the server purely to be nudged to poll immediately when a command is
# dispatched, instead of waiting out report_interval. Purely additive — set
# to true only if your network blocks WebSocket upgrades to the server;
# command delivery still works exactly as before via the regular poll cycle.
disable_ws_push: false

# Skip TLS verification (for self-signed certs)
insecure_skip_verify: false

# CrowdSec integration: correlate web logs with active CrowdSec decisions.
# Requires collect_web_logs: true. The agent queries the CrowdSec Local API
# and marks blocked IPs in the web threats dashboard.
collect_crowdsec_correlation: false
crowdsec_connection_string: "http://localhost:8080"
crowdsec_api_key: ""

# CrowdSec alerts auth (used only for /v1/alerts via watcher login)
# Fill with machine_id/password from /etc/crowdsec/local_api_credentials.yaml
crowdsec_alerts_machine_id: ""
crowdsec_alerts_password: ""

# Network flows ("top talkers"): per-host bandwidth by remote peer, derived
# from conntrack accounting (requires nf_conntrack_acct=1, checked at runtime
# — see the host's diagnostics banner if this stays empty). Only bounds
# collection and payload size; never pushed from the server.
collect_network_flows: true
network_flows_top_n: 50

# OPTIONAL, off by default. Sample outbound TLS handshakes to label a talker
# with the domain it actually contacted (SNI) instead of just an IP + port.
# Requires CAP_NET_RAW on the agent binary:
#   setcap cap_net_raw+ep /usr/local/bin/serversupervisor-agent
# Without it the agent logs once, raises a diagnostic, and falls back to the
# interface's port-based guess — it never fails a report cycle. Only TLS SNI is
# extracted (no DNS, no HTTP); capture is bounded to a few seconds and a few
# thousand packets per cycle.
network_flows_l7_capture: false

# Restic backup supervision. Only paths/flags — never put restic/Swift/SMTP
# credentials here; they stay in resticconf on disk (restic_conf_path).
collect_restic: false
restic_bin: "/usr/local/bin/restic"
restic_conf_path: "/home/user/restic-backups/resticconf"
restic_run_script_path: "/home/user/restic-backups/run_backup.sh"
restic_status_file_path: "/home/user/restic-backups/backup-status.json"

# Path to resticprofile.yaml. Read locally to report the list of available
# profile names (e.g. for the "Lancer un backup" / scheduled-task pickers) —
# only profile names are ever sent to the server, never file content.
restic_profile_config_path: "/home/user/restic-backups/resticprofile.yaml"

restic_enable_progress: true
restic_progress_fps: 0.1

# How long a run_backup command may stay silent (no progress chunk) before
# the agent kills it. Not an absolute run duration cap — a backup that keeps
# progressing can run far longer than this.
restic_backup_idle_timeout_minutes: 20
`

	if strings.TrimSpace(serverURL) != "" {
		raw = strings.Replace(raw, "server_url: \"http://your-server:8080\"", fmt.Sprintf("server_url: %q", serverURL), 1)
	}
	if strings.TrimSpace(apiKey) != "" {
		raw = strings.Replace(raw, "api_key: \"your-api-key-here\"", fmt.Sprintf("api_key: %q", apiKey), 1)
	}

	return raw
}

func splitCSV(raw string) []string {
	parts := []string{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
