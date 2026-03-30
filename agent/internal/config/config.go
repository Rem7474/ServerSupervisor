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

	// TLS
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`

	// APT behaviour
	AptAutoUpdateOnStart bool `yaml:"apt_auto_update_on_start"`
}

// WebLogGlobs returns configured web access log globs.
func (c *Config) WebLogGlobs() []string {
	return c.WebLogsLogPaths
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		ServerURL:             "http://localhost:8080",
		ReportInterval:        30,
		MaxReportBodyBytes:    3 * 1024 * 1024,
		CollectDocker:         true,
		CollectAPT:            true,
		CollectSMART:          false,
		CollectCPUTemperature: true,
		CollectWebLogs:        false,
		WebLogsLogPaths: []string{
			"/var/log/nginx/access.log",
			"/var/log/apache2/access.log",
			"/var/log/httpd/access_log",
			"/data/logs/proxy-host-*_access.log",
		},
		WebLogsTailLines:     5000,
		WebLogsTopN:          10,
		WebLogsRequestsLimit: 200,
		WebLogsCursorFile:    "/var/lib/serversupervisor/web_logs_cursor.json",
	}

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
	if env := os.Getenv("SUPERVISOR_INSECURE_SKIP_VERIFY"); env != "" {
		cfg.InsecureSkipVerify = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_APT_AUTO_UPDATE_ON_START"); env != "" {
		cfg.AptAutoUpdateOnStart = env == "true" || env == "1"
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required (set in config or SUPERVISOR_API_KEY env var)")
	}

	return cfg, nil
}

func DefaultConfigFile() string {
	return `# ServerSupervisor Agent Configuration
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
collect_cpu_temperature: true

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

# Skip TLS verification (for self-signed certs)
insecure_skip_verify: false

# Automatically run apt update at agent startup (opt-in, default false)
apt_auto_update_on_start: false
`
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
