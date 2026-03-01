package config

import (
	"fmt"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Server connection
	ServerURL string `yaml:"server_url"`
	APIKey    string `yaml:"api_key"`

	// Intervals
	ReportInterval int `yaml:"report_interval"` // seconds

	// Features
	CollectDocker bool `yaml:"collect_docker"`
	CollectAPT    bool `yaml:"collect_apt"`
	CollectSMART  bool `yaml:"collect_smart"`

	// TLS
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`

	// APT behaviour
	AptAutoUpdateOnStart bool `yaml:"apt_auto_update_on_start"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		ServerURL:      "http://localhost:8080",
		ReportInterval: 30,
		CollectDocker:  true,
		CollectAPT:     true,
		CollectSMART:   true,
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
	if env := os.Getenv("SUPERVISOR_COLLECT_DOCKER"); env != "" {
		cfg.CollectDocker = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_APT"); env != "" {
		cfg.CollectAPT = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_SMART"); env != "" {
		cfg.CollectSMART = env == "true" || env == "1"
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

# Enable Docker container monitoring
collect_docker: true

# Enable APT update monitoring
collect_apt: true

# Enable SMART disk health monitoring (requires smartmontools)
# Disable on VMs or systems without smartctl
collect_smart: true

# Skip TLS verification (for self-signed certs)
insecure_skip_verify: false

# Automatically run apt update at agent startup (opt-in, default false)
apt_auto_update_on_start: false
`
}
