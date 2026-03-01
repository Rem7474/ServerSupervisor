package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DockerContainer struct {
	ID          string            `json:"id"`
	ContainerID string            `json:"container_id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	ImageTag    string            `json:"image_tag"`
	ImageID     string            `json:"image_id"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Created     time.Time         `json:"created"`
	Ports       string            `json:"ports"`
	Labels      map[string]string `json:"labels"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Networks    []string          `json:"networks,omitempty"`
}

// CollectDocker gathers Docker container information using docker CLI
// This avoids requiring the Docker SDK and works with any Docker setup
func CollectDocker() ([]DockerContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker not found in PATH")
	}

	// Get all container IDs
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a", "-q").Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	containerIDs := strings.Fields(strings.TrimSpace(string(out)))
	if len(containerIDs) == 0 {
		log.Printf("No Docker containers found")
		return []DockerContainer{}, nil
	}

	// Inspect all containers at once to get JSON data
	args := append([]string{"inspect"}, containerIDs...)
	inspectOut, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect failed: %w", err)
	}

	// Parse JSON output
	var inspectData []struct {
		Id     string `json:"Id"`
		Name   string `json:"Name"`
		Config struct {
			Image  string            `json:"Image"`
			Labels map[string]string `json:"Labels"`
			Env    []string          `json:"Env"` // "KEY=VALUE" format
		} `json:"Config"`
		State struct {
			Status     string    `json:"Status"`
			Running    bool      `json:"Running"`
			Paused     bool      `json:"Paused"`
			StartedAt  time.Time `json:"StartedAt"`
			FinishedAt time.Time `json:"FinishedAt"`
		} `json:"State"`
		Created time.Time `json:"Created"`
		Image   string    `json:"Image"`
		Mounts  []struct {
			Destination string `json:"Destination"`
		} `json:"Mounts"`
		NetworkSettings struct {
			Ports    map[string][]struct {
				HostIp   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"Ports"`
			Networks map[string]json.RawMessage `json:"Networks"` // keys are network names
		} `json:"NetworkSettings"`
	}

	if err := json.Unmarshal(inspectOut, &inspectData); err != nil {
		return nil, fmt.Errorf("failed to parse docker inspect output: %w", err)
	}

	var containers []DockerContainer
	for _, data := range inspectData {
		name := strings.TrimPrefix(data.Name, "/")
		fullImage := data.Config.Image
		image, tag := parseImageTag(fullImage)

		// Build state string
		var state string
		if data.State.Running {
			state = "running"
		} else if data.State.Paused {
			state = "paused"
		} else {
			state = "exited"
		}

		// Build status string (similar to docker ps output)
		status := data.State.Status
		if data.State.Running {
			uptime := time.Since(data.State.StartedAt)
			status = fmt.Sprintf("Up %s", formatDuration(uptime))
		} else if !data.State.FinishedAt.IsZero() {
			downtime := time.Since(data.State.FinishedAt)
			status = fmt.Sprintf("Exited %s ago", formatDuration(downtime))
		}

		// Format ports
		ports := formatPorts(data.NetworkSettings.Ports)

		// Parse env vars (filter sensitive keys)
		envVars := make(map[string]string)
		for _, envLine := range data.Config.Env {
			if idx := strings.Index(envLine, "="); idx > 0 {
				k := envLine[:idx]
				v := envLine[idx+1:]
				if !isSensitiveEnvKey(k) {
					envVars[k] = v
				}
			}
		}

		// Parse volume mount destinations
		var volumes []string
		for _, m := range data.Mounts {
			if m.Destination != "" {
				volumes = append(volumes, m.Destination)
			}
		}

		// Parse network names
		var networks []string
		for netName := range data.NetworkSettings.Networks {
			networks = append(networks, netName)
		}

		containers = append(containers, DockerContainer{
			ID:          fmt.Sprintf("%s-%s", data.Id[:12], name),
			ContainerID: data.Id[:12],
			Name:        name,
			Image:       image,
			ImageTag:    tag,
			ImageID:     data.Image[:12],
			State:       state,
			Status:      status,
			Created:     data.Created,
			Ports:       ports,
			Labels:      data.Config.Labels,
			EnvVars:     envVars,
			Volumes:     volumes,
			Networks:    networks,
		})
	}

	log.Printf("Collected %d Docker containers", len(containers))
	return containers, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

func formatPorts(portsMap map[string][]struct {
	HostIp   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}) string {
	if len(portsMap) == 0 {
		return ""
	}

	var parts []string
	for containerPort, bindings := range portsMap {
		if len(bindings) == 0 {
			// No host binding, just exposed
			parts = append(parts, containerPort)
		} else {
			for _, binding := range bindings {
				if binding.HostPort != "" {
					hostIp := binding.HostIp
					if hostIp == "" || hostIp == "0.0.0.0" {
						hostIp = "0.0.0.0"
					}
					parts = append(parts, fmt.Sprintf("%s:%s->%s", hostIp, binding.HostPort, containerPort))
				}
			}
		}
	}

	return strings.Join(parts, ", ")
}

func parseImageTag(fullImage string) (image, tag string) {
	// Handle images with registry prefix (e.g., ghcr.io/org/image:tag)
	lastColon := strings.LastIndex(fullImage, ":")
	if lastColon == -1 || strings.Contains(fullImage[lastColon:], "/") {
		return fullImage, "latest"
	}
	return fullImage[:lastColon], fullImage[lastColon+1:]
}

// DockerNetwork represents a Docker network and connected containers
type DockerNetwork struct {
	NetworkID    string   `json:"network_id"`
	Name         string   `json:"name"`
	Driver       string   `json:"driver"`
	Scope        string   `json:"scope"`
	ContainerIDs []string `json:"container_ids"`
}

// ContainerEnv represents environment variables of a container (filtered for non-sensitive data)
type ContainerEnv struct {
	ContainerName string            `json:"container_name"`
	EnvVars       map[string]string `json:"env_vars"`
}

// CollectDockerNetworks retrieves Docker networks and their connected containers
func CollectDockerNetworks() ([]DockerNetwork, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// List all networks
	out, err := exec.CommandContext(ctx, "docker", "network", "ls",
		"--no-trunc", "--format", "{{.ID}}|{{.Name}}|{{.Driver}}|{{.Scope}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker network ls failed: %w", err)
	}

	var networks []DockerNetwork
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		n := DockerNetwork{
			NetworkID: parts[0][:12],
			Name:      parts[1],
			Driver:    parts[2],
			Scope:     parts[3],
		}
		// Skip system networks
		if n.Name == "bridge" || n.Name == "host" || n.Name == "none" {
			continue
		}

		// Get containers in this network
		inspOut, err := exec.CommandContext(ctx, "docker", "network", "inspect",
			parts[0], "--format", "{{range .Containers}}{{slice .Name 1}}|{{end}}").Output()
		if err == nil {
			for _, c := range strings.Split(strings.TrimSuffix(strings.TrimSpace(string(inspOut)), "|"), "|") {
				if c != "" {
					n.ContainerIDs = append(n.ContainerIDs, strings.TrimSpace(c))
				}
			}
		}
		networks = append(networks, n)
	}
	return networks, nil
}

// CollectContainerEnvVars retrieves environment variables from containers (filtered for sensitive data)
func CollectContainerEnvVars(containerNames []string) ([]ContainerEnv, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var result []ContainerEnv
	for _, name := range containerNames {
		out, err := exec.CommandContext(ctx, "docker", "inspect",
			"--format", "{{range .Config.Env}}{{.}}\n{{end}}", name).Output()
		if err != nil {
			continue
		}

		envMap := make(map[string]string)
		for _, line := range strings.Split(string(out), "\n") {
			if idx := strings.Index(line, "="); idx > 0 {
				k := line[:idx]
				v := line[idx+1:]
				// Filter: skip sensitive env vars
				if !isSensitiveEnvKey(k) {
					envMap[k] = v
				}
			}
		}
		if len(envMap) > 0 {
			result = append(result, ContainerEnv{ContainerName: name, EnvVars: envMap})
		}
	}
	return result, nil
}

// isSensitiveEnvKey checks if an env var key should be filtered out
func isSensitiveEnvKey(key string) bool {
	sensitivePatterns := []string{
		"password", "secret", "token", "key", "pass",
		"pwd", "credential", "auth", "private", "salt",
		"api_key", "apikey", "bearer", "jwt",
	}
	keyLower := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(keyLower, pattern) {
			return true
		}
	}
	return false
}

// ComposeProject represents a docker-compose project and its resolved config
type ComposeProject struct {
	Name       string   `json:"name"`
	WorkingDir string   `json:"working_dir"`
	ConfigFile string   `json:"config_file"`
	Services   []string `json:"services"`
	RawConfig  string   `json:"raw_config"`
}

// CollectComposeProjects discovers docker-compose projects via container labels
// and retrieves their resolved configuration via `docker compose config`
func CollectComposeProjects() ([]ComposeProject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker not found in PATH")
	}

	// Get all container IDs with compose labels
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a", "-q",
		"--filter", "label=com.docker.compose.project").Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps compose failed: %w", err)
	}

	containerIDs := strings.Fields(strings.TrimSpace(string(out)))
	if len(containerIDs) == 0 {
		log.Printf("No Docker Compose containers found")
		return []ComposeProject{}, nil
	}

	// Inspect all containers to get labels
	args := append([]string{"inspect"}, containerIDs...)
	inspectOut, err := exec.CommandContext(ctx, "docker", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect failed: %w", err)
	}

	// Parse JSON output
	var inspectData []struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(inspectOut, &inspectData); err != nil {
		return nil, fmt.Errorf("failed to parse inspect output: %w", err)
	}

	// Build projects map from labels
	projects := make(map[string]*ComposeProject)
	for _, data := range inspectData {
		labels := data.Config.Labels
		name := labels["com.docker.compose.project"]
		if name == "" {
			continue
		}

		if _, exists := projects[name]; !exists {
			projects[name] = &ComposeProject{
				Name:       name,
				WorkingDir: labels["com.docker.compose.project.working_dir"],
				ConfigFile: labels["com.docker.compose.project.config_files"],
				Services:   []string{},
			}
		}

		service := labels["com.docker.compose.service"]
		if service != "" {
			// avoid duplicates
			found := false
			for _, s := range projects[name].Services {
				if s == service {
					found = true
					break
				}
			}
			if !found {
				projects[name].Services = append(projects[name].Services, service)
			}
		}
	}

	// Get raw config for each project
	var result []ComposeProject
	for _, p := range projects {
		if p.WorkingDir != "" {
			cfgCtx, cfgCancel := context.WithTimeout(context.Background(), 10*time.Second)
			cfgOut, err := exec.CommandContext(cfgCtx, "docker", "compose",
				"--project-directory", p.WorkingDir, "config").Output()
			cfgCancel()
			if err == nil {
				p.RawConfig = filterSensitiveYAML(string(cfgOut))
			}
		}
		result = append(result, *p)
	}

	log.Printf("Collected %d Docker Compose projects", len(result))
	return result, nil
}

// ExecuteDockerCommand runs a docker action (start/stop/restart/logs) on a container
// and streams output chunks to chunkCB.
func ExecuteDockerCommand(action, containerName string, chunkCB func(string)) (string, error) {
	var args []string
	var timeout time.Duration

	switch action {
	case "start", "stop", "restart":
		args = []string{action, containerName}
		timeout = 30 * time.Second
	case "logs":
		args = []string{"logs", "--tail", "100", "--timestamps", containerName}
		timeout = 60 * time.Second
	default:
		return "", fmt.Errorf("unknown docker action: %s", action)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start docker %s %s: %w", action, containerName, err)
	}

	var fullOutput strings.Builder
	combined := io.MultiReader(stdoutPipe, stderrPipe)
	buf := make([]byte, 4096)
	for {
		n, readErr := combined.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			fullOutput.WriteString(chunk)
			if chunkCB != nil {
				chunkCB(chunk)
			}
		}
		if readErr != nil {
			break
		}
	}

	cmdErr := cmd.Wait()
	return fullOutput.String(), cmdErr
}

// ExecuteComposeCommand runs a docker compose action on a project and streams output.
// action must be one of: compose_up, compose_down, compose_restart, compose_logs.
func ExecuteComposeCommand(action, projectName, workingDir string, chunkCB func(string)) (string, error) {
	var args []string
	var timeout time.Duration

	switch action {
	case "compose_up":
		args = []string{"compose", "-p", projectName, "up", "-d"}
		timeout = 120 * time.Second
	case "compose_down":
		args = []string{"compose", "-p", projectName, "down"}
		timeout = 60 * time.Second
	case "compose_restart":
		args = []string{"compose", "-p", projectName, "restart"}
		timeout = 60 * time.Second
	case "compose_logs":
		args = []string{"compose", "-p", projectName, "logs", "--tail", "100", "--timestamps"}
		timeout = 60 * time.Second
	default:
		return "", fmt.Errorf("unknown compose action: %s", action)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	if workingDir != "" {
		if !filepath.IsAbs(workingDir) {
			return "", fmt.Errorf("invalid working directory: must be an absolute path")
		}
		cmd.Dir = workingDir
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start docker %s %s: %w", action, projectName, err)
	}

	var fullOutput strings.Builder
	combined := io.MultiReader(stdoutPipe, stderrPipe)
	buf := make([]byte, 4096)
	for {
		n, readErr := combined.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			fullOutput.WriteString(chunk)
			if chunkCB != nil {
				chunkCB(chunk)
			}
		}
		if readErr != nil {
			break
		}
	}

	cmdErr := cmd.Wait()
	return fullOutput.String(), cmdErr
}

// filterSensitiveYAML redacts lines containing sensitive keys in YAML output
func filterSensitiveYAML(yaml string) string {
	sensitiveKeys := []string{
		"password", "secret", "token", "key", "pass",
		"pwd", "credential", "auth", "private", "salt",
		"apikey", "bearer", "jwt",
	}
	var filtered []string
	for _, line := range strings.Split(yaml, "\n") {
		lineLower := strings.ToLower(line)
		sensitive := false
		for _, k := range sensitiveKeys {
			if strings.Contains(lineLower, k+"=") || strings.Contains(lineLower, k+":") {
				sensitive = true
				break
			}
		}
		if sensitive {
			if idx := strings.Index(line, ":"); idx >= 0 {
				filtered = append(filtered, line[:idx+1]+" [REDACTED]")
			} else {
				filtered = append(filtered, line)
			}
		} else {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}
