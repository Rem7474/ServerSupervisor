package collector

import (
	"bufio"
	"context"
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

// ListSystemdServices returns all systemd service units by parsing
// the output of `systemctl list-units --type=service --all`.
func ListSystemdServices() ([]SystemdService, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"systemctl", "list-units",
		"--type=service", "--all",
		"--no-pager", "--plain", "--no-legend",
	)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("systemctl list-units timed out")
		}
		return nil, fmt.Errorf("systemctl list-units: %w", err)
	}

	var services []SystemdService
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		svc := SystemdService{
			Name:        fields[0],
			LoadState:   fields[1],
			ActiveState: fields[2],
			SubState:    fields[3],
		}
		if len(fields) > 4 {
			svc.Description = strings.Join(fields[4:], " ")
		}
		services = append(services, svc)
	}
	return services, nil
}

// ExecuteSystemdCommand runs a systemctl action on a service and streams its output.
// Valid actions: start, stop, restart, enable, disable, status.
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

	cmd := exec.CommandContext(ctx, "systemctl", action, "--no-pager", serviceName)

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
