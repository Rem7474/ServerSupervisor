package config

import (
	"fmt"
	"os"

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

	// TLS
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		ServerURL:      "http://localhost:8080",
		ReportInterval: 30,
		CollectDocker:  true,
		CollectAPT:     true,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required in config")
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

# Skip TLS verification (for self-signed certs)
insecure_skip_verify: false
`
}
