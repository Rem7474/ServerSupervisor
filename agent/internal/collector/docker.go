package collector

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

type DockerContainer struct {
	ID          string            `json:"id"`
	ContainerID string            `json:"container_id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	ImageTag    string            `json:"image_tag"`
	ImageID     string            `json:"image_id"`
	ImageDigest string            `json:"image_digest,omitempty"` // manifest sha256 from RepoDigests
	State       string            `json:"state"`
	Status      string            `json:"status"`
	Created     time.Time         `json:"created"`
	Ports       string            `json:"ports"`
	Labels      map[string]string `json:"labels"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	Networks    []string          `json:"networks,omitempty"`
	NetRxBytes  uint64            `json:"net_rx_bytes,omitempty"`
	NetTxBytes  uint64            `json:"net_tx_bytes,omitempty"`
}

const containerShutdownTimeoutSecs uint = 10 // seconds to wait before SIGKILL

// readBufPool provides reusable 4 KiB read buffers for command/log streaming.
var readBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 4096)
		return &buf
	},
}

// imageDigestCache caches Docker image RepoDigests (sha256 manifest hashes).
// Image metadata is static until the image is replaced, so a 5-minute TTL is safe.
type imageDigestEntry struct {
	digest   string
	cachedAt time.Time
}

var (
	imageDigestCacheMu sync.RWMutex
	imageDigestCache   = make(map[string]imageDigestEntry)
	imageDigestTTL     = 5 * time.Minute
)

// dockerMu guards the shared Docker client singleton.
// A regular sync.Once cannot be used here because we need to retry when the
// initial creation fails (e.g. Docker daemon not yet started at agent boot).
var (
	dockerMu     sync.Mutex
	dockerSingle *docker.Client
	dockerInitOK bool
)

// newDockerClient returns a shared Docker client initialised from environment
// variables (DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH) or the local
// Unix socket. go-dockerclient does not open a persistent connection in the
// constructor, so the singleton is safe to share across goroutines.
//
// Unlike sync.Once, this implementation retries on failure so that a transient
// error at agent startup (daemon not yet ready) does not permanently disable
// Docker monitoring for the lifetime of the process.
func newDockerClient() (*docker.Client, error) {
	dockerMu.Lock()
	defer dockerMu.Unlock()

	if dockerInitOK {
		return dockerSingle, nil
	}

	c, err := docker.NewClientFromEnv()
	if err != nil {
		log.Printf("Docker client init failed (will retry on next collection): %v", err)
		return nil, err
	}
	dockerSingle = c
	dockerInitOK = true
	return dockerSingle, nil
}

// CollectDocker gathers Docker container information using the Docker API.
func CollectDocker() ([]DockerContainer, error) {
	client, err := newDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	ctx := context.Background()
	apiContainers, err := client.ListContainers(docker.ListContainersOptions{
		All:     true,
		Context: ctx,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(apiContainers) == 0 {
		log.Printf("No Docker containers found")
		return []DockerContainer{}, nil
	}

	var containers []DockerContainer
	for _, ac := range apiContainers {
		container, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{ID: ac.ID, Context: ctx})
		if err != nil {
			log.Printf("Failed to inspect container %s: %v", ac.ID[:12], err)
			continue
		}

		name := strings.TrimPrefix(container.Name, "/")
		fullImage := container.Config.Image
		image, tag := parseImageTag(fullImage)

		var state string
		if container.State.Running {
			state = "running"
		} else if container.State.Paused {
			state = "paused"
		} else {
			state = "exited"
		}

		var status string
		if container.State.Running {
			uptime := time.Since(container.State.StartedAt)
			status = fmt.Sprintf("Up %s", formatDuration(uptime))
		} else if !container.State.FinishedAt.IsZero() {
			downtime := time.Since(container.State.FinishedAt)
			status = fmt.Sprintf("Exited %s ago", formatDuration(downtime))
		} else {
			status = container.State.StateString()
		}

		ports := formatPortBindings(container.NetworkSettings.Ports)

		envVars := make(map[string]string)
		for _, envLine := range container.Config.Env {
			if idx := strings.Index(envLine, "="); idx > 0 {
				k := envLine[:idx]
				v := envLine[idx+1:]
				if !isSensitiveEnvKey(k) {
					envVars[k] = v
				}
			}
		}

		var volumes []string
		for _, m := range container.Mounts {
			if m.Destination != "" {
				volumes = append(volumes, m.Destination)
			}
		}

		var networks []string
		for netName := range container.NetworkSettings.Networks {
			networks = append(networks, netName)
		}

		// Fetch the manifest digest (RepoDigest) from the image metadata.
		// container.Image is the full sha256 image config ID; use it to inspect the image.
		// Results are cached for 5 minutes since image metadata is static until the image is replaced.
		var imageDigest string
		imageDigestCacheMu.RLock()
		cached, ok := imageDigestCache[container.Image]
		imageDigestCacheMu.RUnlock()
		if ok && time.Since(cached.cachedAt) < imageDigestTTL {
			imageDigest = cached.digest
		} else {
			if imgInfo, err := client.InspectImage(container.Image); err == nil {
				for _, rd := range imgInfo.RepoDigests {
					// RepoDigest format: "nginx@sha256:f88cbb90..."
					if at := strings.Index(rd, "@sha256:"); at >= 0 {
						imageDigest = rd[at+1:] // keep "sha256:..." prefix
						break
					}
				}
			}
			imageDigestCacheMu.Lock()
			imageDigestCache[container.Image] = imageDigestEntry{digest: imageDigest, cachedAt: time.Now()}
			imageDigestCacheMu.Unlock()
		}

		imageID := container.Image
		if len(imageID) > 12 {
			imageID = imageID[:12]
		}
		containerID := container.ID
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}

		containers = append(containers, DockerContainer{
			ID:          fmt.Sprintf("%s-%s", containerID, name),
			ContainerID: containerID,
			Name:        name,
			Image:       image,
			ImageTag:    tag,
			ImageID:     imageID,
			ImageDigest: imageDigest,
			State:       state,
			Status:      status,
			Created:     container.Created,
			Ports:       ports,
			Labels:      container.Config.Labels,
			EnvVars:     envVars,
			Volumes:     volumes,
			Networks:    networks,
		})
	}

	// Enrich running containers with network I/O stats in parallel.
	// A semaphore limits concurrent Docker Stats calls to avoid overwhelming
	// the daemon with many containers.
	const maxNetStatWorkers = 8
	sem := make(chan struct{}, maxNetStatWorkers)
	var wg sync.WaitGroup
	for i := range containers {
		if containers[i].State != "running" {
			continue
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// Each goroutine writes to a unique slice index — no mutex needed.
			containers[idx].NetRxBytes, containers[idx].NetTxBytes =
				collectContainerNetStats(client, containers[idx].ContainerID)
		}(i)
	}
	wg.Wait()

	log.Printf("Collected %d Docker containers", len(containers))
	return containers, nil
}

// collectContainerNetStats fetches a single stats snapshot for the given
// container and returns the total network rx/tx bytes.
func collectContainerNetStats(client *docker.Client, containerID string) (rx, tx uint64) {
	statsC := make(chan *docker.Stats, 1)
	done := make(chan bool)
	errC := make(chan error, 1)

	go func() {
		errC <- client.Stats(docker.StatsOptions{
			ID:     containerID,
			Stats:  statsC,
			Stream: false,
			Done:   done,
		})
	}()

	select {
	case stats, ok := <-statsC:
		if ok && stats != nil {
			for _, ns := range stats.Networks {
				rx += ns.RxBytes
				tx += ns.TxBytes
			}
		}
		// Drain the error channel so the goroutine can exit cleanly.
		select {
		case err := <-errC:
			if err != nil {
				log.Printf("Failed to get stats for container %s: %v", containerID, err)
			}
		default:
		}
	case err := <-errC:
		if err != nil {
			log.Printf("Failed to get stats for container %s: %v", containerID, err)
		}
	case <-time.After(5 * time.Second):
		close(done)
	}
	return rx, tx
}

// formatPortBindings converts the Docker API port map to a human-readable string.
func formatPortBindings(ports map[docker.Port][]docker.PortBinding) string {
	if len(ports) == 0 {
		return ""
	}
	var parts []string
	for port, bindings := range ports {
		if len(bindings) == 0 {
			parts = append(parts, string(port))
		} else {
			for _, b := range bindings {
				if b.HostPort != "" {
					hostIP := b.HostIP
					if hostIP == "" {
						hostIP = "0.0.0.0"
					}
					parts = append(parts, fmt.Sprintf("%s:%s->%s", hostIP, b.HostPort, port))
				}
			}
		}
	}
	return strings.Join(parts, ", ")
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

func parseImageTag(fullImage string) (image, tag string) {
	// Handle images with registry prefix (e.g., ghcr.io/org/image:tag)
	lastColon := strings.LastIndex(fullImage, ":")
	if lastColon == -1 || strings.Contains(fullImage[lastColon:], "/") {
		return fullImage, "latest"
	}
	return fullImage[:lastColon], fullImage[lastColon+1:]
}

// DockerNetwork represents a Docker network and connected containers.
type DockerNetwork struct {
	NetworkID    string   `json:"network_id"`
	Name         string   `json:"name"`
	Driver       string   `json:"driver"`
	Scope        string   `json:"scope"`
	ContainerIDs []string `json:"container_ids"`
}

// ContainerEnv represents environment variables of a container (filtered for non-sensitive data).
type ContainerEnv struct {
	ContainerName string            `json:"container_name"`
	EnvVars       map[string]string `json:"env_vars"`
}

// CollectDockerNetworks retrieves Docker networks and their connected containers via the Docker API.
func CollectDockerNetworks() ([]DockerNetwork, error) {
	client, err := newDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	networks, err := client.ListNetworks()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var result []DockerNetwork
	for _, net := range networks {
		// Skip system networks
		if net.Name == "bridge" || net.Name == "host" || net.Name == "none" {
			continue
		}

		networkID := net.ID
		if len(networkID) > 12 {
			networkID = networkID[:12]
		}

		n := DockerNetwork{
			NetworkID: networkID,
			Name:      net.Name,
			Driver:    net.Driver,
			Scope:     net.Scope,
		}
		for _, ep := range net.Containers {
			n.ContainerIDs = append(n.ContainerIDs, ep.Name)
		}
		result = append(result, n)
	}
	return result, nil
}

// CollectContainerEnvVars retrieves environment variables from containers via the Docker API
// (sensitive values are filtered out).
func CollectContainerEnvVars(containerNames []string) ([]ContainerEnv, error) {
	client, err := newDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	var result []ContainerEnv
	for _, name := range containerNames {
		container, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{ID: name})
		if err != nil {
			continue
		}

		envMap := make(map[string]string)
		for _, envLine := range container.Config.Env {
			if idx := strings.Index(envLine, "="); idx > 0 {
				k := envLine[:idx]
				v := envLine[idx+1:]
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

// isSensitiveEnvKey checks if an env var key should be filtered out.
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

// composeConfigCache caches the output of `docker compose config` per working directory.
// The resolved config is expensive (subprocess) and only changes when docker-compose.yml is edited.
type composeCacheEntry struct {
	rawConfig string
	cachedAt  time.Time
}

var (
	composeCacheMu sync.RWMutex
	composeCache   = make(map[string]composeCacheEntry)
	composeTTL     = 10 * time.Minute
)

// ComposeProject represents a docker-compose project and its resolved config.
type ComposeProject struct {
	Name       string   `json:"name"`
	WorkingDir string   `json:"working_dir"`
	ConfigFile string   `json:"config_file"`
	Services   []string `json:"services"`
	RawConfig  string   `json:"raw_config"`
}

// CollectComposeProjects discovers docker-compose projects via container labels using the
// Docker API and retrieves their resolved configuration via `docker compose config`.
func CollectComposeProjects() ([]ComposeProject, error) {
	client, err := newDockerClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	ctx := context.Background()
	apiContainers, err := client.ListContainers(docker.ListContainersOptions{
		All:     true,
		Filters: map[string][]string{"label": {"com.docker.compose.project"}},
		Context: ctx,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list compose containers: %w", err)
	}

	if len(apiContainers) == 0 {
		log.Printf("No Docker Compose containers found")
		return []ComposeProject{}, nil
	}

	projects := make(map[string]*ComposeProject)
	for _, ac := range apiContainers {
		labels := ac.Labels
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

	// Retrieve the resolved compose config via CLI (no Docker API equivalent).
	// Results are cached for 10 minutes to avoid repeated subprocess invocations.
	var result []ComposeProject
	for _, p := range projects {
		if p.WorkingDir != "" {
			composeCacheMu.RLock()
			ce, hit := composeCache[p.WorkingDir]
			composeCacheMu.RUnlock()
			if hit && time.Since(ce.cachedAt) < composeTTL {
				p.RawConfig = ce.rawConfig
			} else {
				cfgCtx, cfgCancel := context.WithTimeout(context.Background(), 10*time.Second)
				cfgOut, err := exec.CommandContext(cfgCtx, "docker", "compose",
					"--project-directory", p.WorkingDir, "config").Output()
				cfgCancel()
				if err == nil {
					raw := filterSensitiveYAML(string(cfgOut))
					p.RawConfig = raw
					composeCacheMu.Lock()
					composeCache[p.WorkingDir] = composeCacheEntry{rawConfig: raw, cachedAt: time.Now()}
					composeCacheMu.Unlock()
				}
			}
		}
		result = append(result, *p)
	}

	log.Printf("Collected %d Docker Compose projects", len(result))
	return result, nil
}

// ExecuteDockerCommand runs a docker action (start/stop/restart/logs) on a container
// via the Docker API and streams output chunks to chunkCB.
func ExecuteDockerCommand(action, containerName string, chunkCB func(string)) (string, error) {
	client, err := newDockerClient()
	if err != nil {
		return "", fmt.Errorf("failed to connect to Docker: %w", err)
	}

	switch action {
	case "start":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := client.StartContainerWithContext(containerName, nil, ctx); err != nil {
			return "", fmt.Errorf("failed to start container %s: %w", containerName, err)
		}
		msg := fmt.Sprintf("Container %s started", containerName)
		if chunkCB != nil {
			chunkCB(msg)
		}
		return msg, nil

	case "stop":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := client.StopContainerWithContext(containerName, containerShutdownTimeoutSecs, ctx); err != nil {
			return "", fmt.Errorf("failed to stop container %s: %w", containerName, err)
		}
		msg := fmt.Sprintf("Container %s stopped", containerName)
		if chunkCB != nil {
			chunkCB(msg)
		}
		return msg, nil

	case "restart":
		// go-dockerclient has no RestartContainerWithContext, so enforce the timeout manually.
		// 60s = stop grace period (10s) + start + safety margin.
		restartDone := make(chan error, 1)
		go func() {
			restartDone <- client.RestartContainer(containerName, containerShutdownTimeoutSecs)
		}()
		select {
		case err := <-restartDone:
			if err != nil {
				return "", fmt.Errorf("failed to restart container %s: %w", containerName, err)
			}
		case <-time.After(60 * time.Second):
			return "", fmt.Errorf("restart of container %s timed out after 60s", containerName)
		}
		msg := fmt.Sprintf("Container %s restarted", containerName)
		if chunkCB != nil {
			chunkCB(msg)
		}
		return msg, nil

	case "logs":
		return streamContainerLogs(client, containerName, chunkCB)

	default:
		return "", fmt.Errorf("unknown docker action: %s", action)
	}
}

// streamContainerLogs retrieves the last 100 log lines from a container via the Docker API
// and streams them in chunks to chunkCB.
func streamContainerLogs(client *docker.Client, containerName string, chunkCB func(string)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pr, pw := io.Pipe()
	var fullOutput strings.Builder

	go func() {
		err := client.Logs(docker.LogsOptions{
			Context:      ctx,
			Container:    containerName,
			OutputStream: pw,
			ErrorStream:  pw,
			Stdout:       true,
			Stderr:       true,
			Tail:         "100",
			Timestamps:   true,
		})
		pw.CloseWithError(err)
	}()

	bufPtr := readBufPool.Get().(*[]byte)
	buf := *bufPtr
	defer readBufPool.Put(bufPtr)
	for {
		n, readErr := pr.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			fullOutput.WriteString(chunk)
			if chunkCB != nil {
				chunkCB(chunk)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fullOutput.String(), readErr
		}
	}
	return fullOutput.String(), nil
}

// ExecuteComposeCommand runs a docker compose action on a project and streams output.
// action must be one of: compose_up, compose_down, compose_restart, compose_logs.
// Docker Compose operations have no Docker API equivalent so the CLI is used.
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
	var outMu sync.Mutex
	streamPipe := func(r io.Reader) {
		bufPtr := readBufPool.Get().(*[]byte)
		buf := *bufPtr
		defer readBufPool.Put(bufPtr)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				outMu.Lock()
				fullOutput.WriteString(chunk)
				outMu.Unlock()
				if chunkCB != nil {
					chunkCB(chunk)
				}
			}
			if readErr != nil {
				return
			}
		}
	}

	var pipeWg sync.WaitGroup
	pipeWg.Add(2)
	go func() { defer pipeWg.Done(); streamPipe(stdoutPipe) }()
	go func() { defer pipeWg.Done(); streamPipe(stderrPipe) }()
	pipeWg.Wait()

	cmdErr := cmd.Wait()
	return fullOutput.String(), cmdErr
}

// filterSensitiveYAML redacts lines containing sensitive keys in YAML output.
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
