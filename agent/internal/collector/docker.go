package collector

import (
	"context"
	"fmt"
	"log"
	"os/exec"
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

	// List all containers (running and stopped)
	// Format: ID|Name|Image|State|Status|Ports|CreatedAt|ImageID|Labels
	format := "{{.ID}}|{{.Names}}|{{.Image}}|{{.State}}|{{.Status}}|{{.Ports}}|{{.CreatedAt}}|{{.ID}}|{{.Labels}}"
	out, err := exec.CommandContext(ctx, "docker", "ps", "-a", "--no-trunc", "--format", format).Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	var containers []DockerContainer
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 9)
		if len(parts) < 8 {
			continue
		}

		containerID := parts[0]
		name := strings.TrimPrefix(parts[1], "/")
		fullImage := parts[2]
		state := parts[3]
		status := parts[4]
		ports := parts[5]

		// Parse image name and tag
		image, tag := parseImageTag(fullImage)

		// Get the actual image ID
		imageID := getImageID(ctx, containerID)

		// Parse labels
		labels := make(map[string]string)
		if len(parts) > 8 && parts[8] != "" {
			labelStr := strings.Trim(parts[8], "map[]")
			for _, pair := range strings.Split(labelStr, " ") {
				kv := strings.SplitN(pair, ":", 2)
				if len(kv) == 2 {
					labels[kv[0]] = kv[1]
				}
			}
		}

		containers = append(containers, DockerContainer{
			ID:          fmt.Sprintf("%s-%s", containerID[:12], name),
			ContainerID: containerID[:12],
			Name:        name,
			Image:       image,
			ImageTag:    tag,
			ImageID:     imageID,
			State:       state,
			Status:      status,
			Ports:       ports,
			Labels:      labels,
		})
	}

	log.Printf("Collected %d Docker containers", len(containers))
	return containers, nil
}

func parseImageTag(fullImage string) (image, tag string) {
	// Handle images with registry prefix (e.g., ghcr.io/org/image:tag)
	lastColon := strings.LastIndex(fullImage, ":")
	if lastColon == -1 || strings.Contains(fullImage[lastColon:], "/") {
		return fullImage, "latest"
	}
	return fullImage[:lastColon], fullImage[lastColon+1:]
}

func getImageID(ctx context.Context, containerID string) string {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Image}}", containerID).Output()
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(string(out))
	// Shorten sha256:xxx to first 12 chars
	if strings.HasPrefix(id, "sha256:") {
		id = id[7:]
		if len(id) > 12 {
			id = id[:12]
		}
	}
	return id
}
