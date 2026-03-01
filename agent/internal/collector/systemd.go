package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SystemdService represents a single systemd service unit.
type SystemdService struct {
	Name        string `json:"name"`
	LoadState   string `json:"load_state"`
	ActiveState string `json:"active_state"`
	SubState    string `json:"sub_state"`
	Description string `json:"description"`
}

// ListSystemdServices returns all systemd service units using --output=json
// (requires systemd >= v230, available on all modern distros).
func ListSystemdServices() ([]SystemdService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"systemctl", "list-units",
		"--type=service", "--all",
		"--no-pager", "--output=json",
	)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("systemctl list-units timed out")
		}
		return nil, fmt.Errorf("systemctl list-units: %w", err)
	}

	// JSON output: array of objects with lowercase field names.
	type rawUnit struct {
		Unit        string `json:"unit"`
		Load        string `json:"load"`
		Active      string `json:"active"`
		Sub         string `json:"sub"`
		Description string `json:"description"`
	}
	var raw []rawUnit
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse systemctl JSON output: %w", err)
	}

	services := make([]SystemdService, 0, len(raw))
	for _, u := range raw {
		services = append(services, SystemdService{
			Name:        u.Unit,
			LoadState:   u.Load,
			ActiveState: u.Active,
			SubState:    u.Sub,
			Description: u.Description,
		})
	}
	return services, nil
}

// ExecuteSystemdCommand runs a systemctl action on a service and streams its output.
// Valid actions: start, stop, restart, enable, disable, status.
// For status, --output=json formats embedded journal entries as JSON objects.
func ExecuteSystemdCommand(serviceName, action string, chunkCB func(string)) (string, error) {
	if !validServiceName.MatchString(serviceName) {
		return "", fmt.Errorf("invalid service name: %q", serviceName)
	}

	validActions := map[string]bool{
		"start": true, "stop": true, "restart": true,
		"enable": true, "disable": true, "status": true,
	}
	if !validActions[action] {
		return "", fmt.Errorf("invalid systemd action: %q", action)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{action, "--no-pager"}
	if action == "status" {
		args = append(args, "--output=json")
	}
	args = append(args, serviceName)
	cmd := exec.CommandContext(ctx, "systemctl", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start systemctl: %w", err)
	}

	var builder strings.Builder

	stdoutScanner := bufio.NewScanner(stdout)
	for stdoutScanner.Scan() {
		line := stdoutScanner.Text() + "\n"
		builder.WriteString(line)
		if chunkCB != nil {
			chunkCB(line)
		}
	}

	stderrScanner := bufio.NewScanner(stderr)
	for stderrScanner.Scan() {
		line := stderrScanner.Text() + "\n"
		builder.WriteString(line)
		if chunkCB != nil {
			chunkCB(line)
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return builder.String(), fmt.Errorf("systemctl timed out after 30s")
		}
		// systemctl status exits with non-zero for inactive/missing units — not an error for us.
		if action == "status" {
			return builder.String(), nil
		}
		return builder.String(), fmt.Errorf("systemctl %s %s: %w", action, serviceName, err)
	}

	return builder.String(), nil
}
