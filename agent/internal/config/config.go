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
	ReportInterval int `yaml:"report_interval"` // seconds

	// Features
	CollectDocker         bool     `yaml:"collect_docker"`
	CollectAPT            bool     `yaml:"collect_apt"`
	CollectSMART          bool     `yaml:"collect_smart"`
	CollectBotDetection   bool     `yaml:"collect_bot_detection"`
	BotDetectionLogPaths  []string `yaml:"bot_detection_log_paths"`
	BotDetectionTailLines int      `yaml:"bot_detection_tail_lines"`
	BotDetectionTopN      int      `yaml:"bot_detection_top_n"`
	CollectNPMAnalytics   bool     `yaml:"collect_npm_analytics"`
	// NPMAnalyticsLogDir : dossier contenant les logs NPM.
	// L'agent trouve automatiquement les fichiers proxy-host-*.log dedans.
	// Si vide, NPMAnalyticsLogPaths (liste de globs) est utilisé à la place (rétrocompat).
	NPMAnalyticsLogDir    string   `yaml:"npm_analytics_log_dir"`
	NPMAnalyticsLogPaths  []string `yaml:"npm_analytics_log_paths"`
	NPMAnalyticsTailLines int      `yaml:"npm_analytics_tail_lines"`
	NPMAnalyticsTopN      int      `yaml:"npm_analytics_top_n"`

	// TLS
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`

	// APT behaviour
	AptAutoUpdateOnStart bool `yaml:"apt_auto_update_on_start"`
}

// NPMLogGlobs retourne la liste de globs à utiliser pour les logs NPM.
// Priorité : npm_analytics_log_dir > npm_analytics_log_paths.
func (c *Config) NPMLogGlobs() []string {
	if c.NPMAnalyticsLogDir != "" {
		dir := strings.TrimRight(c.NPMAnalyticsLogDir, "/")
		return []string{dir + "/proxy-host-*.log"}
	}
	return c.NPMAnalyticsLogPaths
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		ServerURL:           "http://localhost:8080",
		ReportInterval:      30,
		CollectDocker:       true,
		CollectAPT:          true,
		CollectSMART:        true,
		CollectBotDetection: true,
		CollectNPMAnalytics: true,
		BotDetectionLogPaths: []string{
			"/var/log/nginx/access.log",
			"/var/log/apache2/access.log",
			"/var/log/httpd/access_log",
			"/data/logs/proxy-host-*.log",
		},
		BotDetectionTailLines: 5000,
		BotDetectionTopN:      10,
		NPMAnalyticsLogPaths: []string{
			"/var/log/nginx/access.log",
			"/var/log/apache2/access.log",
			"/var/log/httpd/access_log",
			"/data/logs/proxy-host-*.log",
		},
		NPMAnalyticsTailLines: 5000,
		NPMAnalyticsTopN:      10,
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
	if env := os.Getenv("SUPERVISOR_COLLECT_BOT_DETECTION"); env != "" {
		cfg.CollectBotDetection = env == "true" || env == "1"
	}
	if env := os.Getenv("SUPERVISOR_BOT_DETECTION_LOG_PATHS"); env != "" {
		parts := []string{}
		for _, p := range strings.Split(env, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				parts = append(parts, p)
			}
		}
		if len(parts) > 0 {
			cfg.BotDetectionLogPaths = parts
		}
	}
	if env := os.Getenv("SUPERVISOR_BOT_DETECTION_TAIL_LINES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.BotDetectionTailLines = n
		}
	}
	if env := os.Getenv("SUPERVISOR_BOT_DETECTION_TOP_N"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.BotDetectionTopN = n
		}
	}
	if env := os.Getenv("SUPERVISOR_COLLECT_NPM_ANALYTICS"); env != "" {
		cfg.CollectNPMAnalytics = env == "true" || env == "1"
	}
	// Priorité env : dossier > liste de paths
	if env := os.Getenv("SUPERVISOR_NPM_ANALYTICS_LOG_DIR"); env != "" {
		cfg.NPMAnalyticsLogDir = strings.TrimSpace(env)
	}
	if env := os.Getenv("SUPERVISOR_NPM_ANALYTICS_LOG_PATHS"); env != "" && cfg.NPMAnalyticsLogDir == "" {
		parts := []string{}
		for _, p := range strings.Split(env, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				parts = append(parts, p)
			}
		}
		if len(parts) > 0 {
			cfg.NPMAnalyticsLogPaths = parts
		}
	}
	if env := os.Getenv("SUPERVISOR_NPM_ANALYTICS_TAIL_LINES"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.NPMAnalyticsTailLines = n
		}
	}
	if env := os.Getenv("SUPERVISOR_NPM_ANALYTICS_TOP_N"); env != "" {
		if n, err := strconv.Atoi(env); err == nil && n > 0 {
			cfg.NPMAnalyticsTopN = n
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

# Enable Docker container monitoring
collect_docker: true

# Enable APT update monitoring
collect_apt: true

# Enable SMART disk health monitoring (requires smartmontools)
# Disable on VMs or systems without smartctl
collect_smart: true

# Detect automated scans/bots from web access logs (nginx/apache/NPM)
collect_bot_detection: true

# Glob paths to parse (supports wildcards)
bot_detection_log_paths:
  - "/var/log/nginx/access.log"
  - "/var/log/apache2/access.log"
  - "/var/log/httpd/access_log"
  - "/data/logs/proxy-host-*.log"

# Number of latest log lines to inspect per file
bot_detection_tail_lines: 5000

# Number of top suspicious IPs/paths returned
bot_detection_top_n: 10

# Analyze Nginx Proxy Manager request patterns from web access logs
collect_npm_analytics: true

# Dossier contenant les logs NPM (proxy-host-*.log détectés automatiquement).
# Exemple : "/usr/local/bin/docker-compose/data/logs"
# Si renseigné, npm_analytics_log_paths est ignoré.
npm_analytics_log_dir: ""

# (Avancé) Liste explicite de globs — ignoré si npm_analytics_log_dir est défini.
# npm_analytics_log_paths:
#   - "/var/log/nginx/access.log"
#   - "/data/logs/proxy-host-*.log"

# Number of latest log lines to inspect per file
npm_analytics_tail_lines: 5000

# Number of top domains/hosts returned in analytics
npm_analytics_top_n: 10

# Skip TLS verification (for self-signed certs)
insecure_skip_verify: false

# Automatically run apt update at agent startup (opt-in, default false)
apt_auto_update_on_start: false
`
}
