package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
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

// SelfServiceUnit is the systemd unit name install.sh registers the agent
// under. A restart/stop command targeting it needs different handling than
// any other unit — see ExecuteSelfRestart.
const SelfServiceUnit = "serversupervisor-agent"

// IsSelfServiceUnit reports whether name refers to the agent's own systemd
// unit, tolerating the optional ".service" suffix systemd accepts either way.
func IsSelfServiceUnit(name string) bool {
	return strings.TrimSuffix(name, ".service") == SelfServiceUnit
}

// ExecuteSelfRestart runs a restart or stop of the agent's own systemd unit
// via a detached systemd-run transient unit rather than a direct child
// process. systemd's default KillMode=control-group sends SIGTERM to every
// process in the service's cgroup when the unit stops (as part of a
// restart or a plain stop) — including a direct `systemctl restart` child
// and the agent process itself — so running it the same way
// ExecuteSystemdCommand does would race the command's own report against
// the agent being killed mid-flight, which is exactly why this action used
// to come back as a failure to the server. systemd-run launches the actual
// systemctl call in a new transient unit outside that cgroup, so it
// survives the agent going down. Since the agent cannot wait for or observe
// the eventual outcome (it's about to be torn down), this only reports
// whether the detached job was launched, not whether the service actually
// came back up.
func ExecuteSelfRestart(action string) error {
	if action != "restart" && action != "stop" {
		return fmt.Errorf("unsupported self-service action: %q", action)
	}
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return fmt.Errorf("systemd-run not available: %w", err)
	}
	unitName := fmt.Sprintf("%s-%s-%d", SelfServiceUnit, action, time.Now().UnixNano())
	cmd := exec.Command("systemd-run", "--unit="+unitName, "--collect", "systemctl", action, SelfServiceUnit)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch detached systemctl %s: %w", action, err)
	}
	return nil
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

	args := []string{action, "--no-pager", serviceName}
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
	var outMu sync.Mutex
	emit := func(line string) {
		outMu.Lock()
		builder.WriteString(line)
		outMu.Unlock()
		if chunkCB != nil {
			chunkCB(line)
		}
	}

	var streamWg sync.WaitGroup
	streamWg.Add(2)
	go func() {
		defer streamWg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			emit(scanner.Text() + "\n")
		}
	}()
	go func() {
		defer streamWg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			emit(scanner.Text() + "\n")
		}
	}()
	streamWg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return builder.String(), fmt.Errorf("systemctl timed out after 30s")
		}
		// systemctl status exits with non-zero for inactive/missing units - not an error for us.
		if action == "status" {
			return builder.String(), nil
		}
		return builder.String(), fmt.Errorf("systemctl %s %s: %w", action, serviceName, err)
	}

	return builder.String(), nil
}
