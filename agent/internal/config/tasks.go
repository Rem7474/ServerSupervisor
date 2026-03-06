package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

const defaultTasksConfigPath = "/etc/serversupervisor/tasks.yaml"

// validTaskID matches safe task identifiers (alphanumeric, hyphens, underscores).
var validTaskID = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,64}$`)

// CustomTask defines a locally-declared automation task on the agent.
// Tasks are identified by ID and triggered remotely by the server.
// The command is executed directly (no shell) to prevent injection.
type CustomTask struct {
	ID      string   `yaml:"id"`
	Name    string   `yaml:"name"`
	Command []string `yaml:"command"` // argv — exec'd directly, not via shell
	Timeout int      `yaml:"timeout"` // seconds; default 60, max 3600
}

// TasksConfig holds all custom tasks declared in the local YAML file.
type TasksConfig struct {
	Tasks []CustomTask `yaml:"tasks"`
}

// LoadTasksConfig reads and validates the tasks YAML file.
// If the file does not exist, an empty config is returned (not an error).
// The path is read from TASKS_CONFIG_PATH env var or defaults to /etc/serversupervisor/tasks.yaml.
func LoadTasksConfig() (*TasksConfig, error) {
	path := os.Getenv("TASKS_CONFIG_PATH")
	if path == "" {
		path = defaultTasksConfigPath
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &TasksConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("tasks config: read %s: %w", path, err)
	}

	var cfg TasksConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("tasks config: parse %s: %w", path, err)
	}

	if err := validateTasksConfig(&cfg); err != nil {
		return nil, fmt.Errorf("tasks config: %w", err)
	}

	return &cfg, nil
}

func validateTasksConfig(cfg *TasksConfig) error {
	seen := make(map[string]bool)
	for i, t := range cfg.Tasks {
		if !validTaskID.MatchString(t.ID) {
			return fmt.Errorf("task[%d]: invalid id %q (alphanumeric, hyphens, underscores only)", i, t.ID)
		}
		if seen[t.ID] {
			return fmt.Errorf("task[%d]: duplicate id %q", i, t.ID)
		}
		seen[t.ID] = true
		if len(t.Command) == 0 || t.Command[0] == "" {
			return fmt.Errorf("task %q: command must not be empty", t.ID)
		}
		if t.Timeout < 0 || t.Timeout > 3600 {
			return fmt.Errorf("task %q: timeout must be between 0 and 3600", t.ID)
		}
		if t.Timeout == 0 {
			cfg.Tasks[i].Timeout = 60
		}
	}
	return nil
}

// TaskSummary is the lightweight representation sent in agent reports so the
// server can display available custom tasks in the UI.
type TaskSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Summaries returns a lightweight list of all tasks (ID + Name only).
func (c *TasksConfig) Summaries() []TaskSummary {
	out := make([]TaskSummary, len(c.Tasks))
	for i, t := range c.Tasks {
		out[i] = TaskSummary{ID: t.ID, Name: t.Name}
	}
	return out
}

// FindTask returns the CustomTask with the given ID, or nil if not found.
func (c *TasksConfig) FindTask(id string) *CustomTask {
	for i := range c.Tasks {
		if c.Tasks[i].ID == id {
			return &c.Tasks[i]
		}
	}
	return nil
}
