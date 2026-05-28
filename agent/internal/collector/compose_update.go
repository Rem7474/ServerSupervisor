package collector

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

// ResolveComposeProject finds a discovered compose project by name. Returns nil
// (no error) when the project is not present in the local Docker inventory.
// The agent refuses to act on a project the server merely names but that does
// not exist locally — the working directory comes from Docker's own labels, so
// the server can never inject an arbitrary path or target.
func ResolveComposeProject(name string) (*ComposeProject, error) {
	projects, err := CollectComposeProjects()
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i].Name == name {
			return &projects[i], nil
		}
	}
	return nil, nil
}

// streamExec runs a command streaming combined stdout+stderr to chunkCB and
// returns the full captured output. argv is exec'd directly (no shell).
func streamExec(ctx context.Context, chunkCB func(string), name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	var b strings.Builder
	var mu sync.Mutex
	pump := func(r io.Reader) {
		bufPtr := readBufPool.Get().(*[]byte)
		buf := *bufPtr
		defer readBufPool.Put(bufPtr)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				mu.Lock()
				b.WriteString(chunk)
				mu.Unlock()
				if chunkCB != nil {
					chunkCB(chunk)
				}
			}
			if rerr != nil {
				return
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); pump(stdout) }()
	go func() { defer wg.Done(); pump(stderr) }()
	wg.Wait()

	return b.String(), cmd.Wait()
}

// composeArgs builds the `docker compose [--project-directory dir] -p name`
// prefix. The working dir comes from the project's Docker labels (trusted),
// never from the server.
func composeArgs(projectName, workingDir string, rest ...string) []string {
	args := make([]string, 0, 6+len(rest))
	args = append(args, "compose")
	if workingDir != "" {
		args = append(args, "--project-directory", workingDir)
	}
	args = append(args, "-p", projectName)
	return append(args, rest...)
}

// ComposePull pulls the latest images for a project (or single service).
func ComposePull(ctx context.Context, projectName, workingDir, service string, chunkCB func(string)) (string, error) {
	rest := []string{"pull"}
	if service != "" {
		rest = append(rest, service)
	}
	return streamExec(ctx, chunkCB, "docker", composeArgs(projectName, workingDir, rest...)...)
}

// ComposeUp (re)creates and starts the project (or single service) detached.
func ComposeUp(ctx context.Context, projectName, workingDir, service string, chunkCB func(string)) (string, error) {
	rest := []string{"up", "-d"}
	if service != "" {
		rest = append(rest, service)
	}
	return streamExec(ctx, chunkCB, "docker", composeArgs(projectName, workingDir, rest...)...)
}

// PruneImages removes dangling images after a successful update.
func PruneImages(ctx context.Context, chunkCB func(string)) (string, error) {
	return streamExec(ctx, chunkCB, "docker", "image", "prune", "-f")
}

// ComposeServiceImage pairs a service's resolved image reference (name:tag) with
// the concrete image ID currently in use, captured before an update so a failed
// deployment can be rolled back by re-tagging the old image ID.
type ComposeServiceImage struct {
	Service  string // compose service name
	ImageRef string // config image, e.g. "nginx:1.25"
	ImageID  string // resolved local image ID, e.g. "sha256:..."
}

// CaptureComposeImages records the current image ID per service so a failed
// update can be rolled back. Only image-based services are captured (services
// built locally have no stable ref to retag and are skipped).
func CaptureComposeImages(ctx context.Context, projectName, service string) ([]ComposeServiceImage, error) {
	client, err := newDockerClient()
	if err != nil {
		return nil, err
	}
	labels := []string{"com.docker.compose.project=" + projectName}
	if service != "" {
		labels = append(labels, "com.docker.compose.service="+service)
	}
	apiContainers, err := client.ListContainers(docker.ListContainersOptions{
		All:     true,
		Filters: map[string][]string{"label": labels},
		Context: ctx,
	})
	if err != nil {
		return nil, err
	}

	var out []ComposeServiceImage
	for _, ac := range apiContainers {
		ins, err := client.InspectContainerWithOptions(docker.InspectContainerOptions{ID: ac.ID, Context: ctx})
		if err != nil {
			continue
		}
		svc := ins.Config.Labels["com.docker.compose.service"]
		ref := ins.Config.Image
		if svc == "" || ref == "" || ins.Image == "" {
			continue
		}
		out = append(out, ComposeServiceImage{Service: svc, ImageRef: ref, ImageID: ins.Image})
	}
	return out, nil
}

// RollbackComposeImages re-tags each captured image ID back to its original
// reference so a subsequent `compose up -d` recreates containers with the
// previous image. Must run before any image prune.
func RollbackComposeImages(ctx context.Context, images []ComposeServiceImage, chunkCB func(string)) error {
	client, err := newDockerClient()
	if err != nil {
		return err
	}
	for _, img := range images {
		repo, tag := splitImageRef(img.ImageRef)
		if err := client.TagImage(img.ImageID, docker.TagImageOptions{Repo: repo, Tag: tag, Force: true, Context: ctx}); err != nil {
			if chunkCB != nil {
				chunkCB(fmt.Sprintf("rollback: failed to retag %s -> %s: %v\n", img.ImageID, img.ImageRef, err))
			}
			return fmt.Errorf("retag %s: %w", img.ImageRef, err)
		}
		if chunkCB != nil {
			chunkCB(fmt.Sprintf("rollback: restored %s to %s\n", img.ImageRef, img.ImageID))
		}
	}
	return nil
}

// splitImageRef splits "repo/name:tag" into ("repo/name", "tag"). A digest ref
// or missing tag yields tag "latest".
func splitImageRef(ref string) (repo, tag string) {
	lastColon := strings.LastIndex(ref, ":")
	if lastColon == -1 || strings.Contains(ref[lastColon:], "/") {
		return ref, "latest"
	}
	return ref[:lastColon], ref[lastColon+1:]
}

// WaitComposeHealthy polls every container of the project until all are healthy
// or the timeout elapses. A container with no healthcheck defined is considered
// healthy once it is running. Returns (healthy, human-readable detail).
func WaitComposeHealthy(ctx context.Context, projectName, service string, timeout time.Duration, chunkCB func(string)) (bool, string) {
	client, err := newDockerClient()
	if err != nil {
		return false, "docker client: " + err.Error()
	}
	labels := []string{"com.docker.compose.project=" + projectName}
	if service != "" {
		labels = append(labels, "com.docker.compose.service="+service)
	}
	deadline := time.Now().Add(timeout)

	for {
		apiContainers, lerr := client.ListContainers(docker.ListContainersOptions{
			All:     true,
			Filters: map[string][]string{"label": labels},
			Context: ctx,
		})
		if lerr != nil {
			return false, "list containers: " + lerr.Error()
		}

		allHealthy := true
		var detail strings.Builder
		for _, ac := range apiContainers {
			ins, ierr := client.InspectContainerWithOptions(docker.InspectContainerOptions{ID: ac.ID, Context: ctx})
			if ierr != nil {
				allHealthy = false
				continue
			}
			name := strings.TrimPrefix(ins.Name, "/")
			st := ins.State
			switch {
			case !st.Running:
				allHealthy = false
				fmt.Fprintf(&detail, "%s: not running (%s)\n", name, st.Status)
			case st.Health.Status != "" && st.Health.Status != "healthy":
				allHealthy = false
				fmt.Fprintf(&detail, "%s: health=%s\n", name, st.Health.Status)
			}
		}

		if allHealthy {
			return true, "all containers healthy"
		}
		if time.Now().After(deadline) {
			return false, strings.TrimSpace(detail.String())
		}
		select {
		case <-ctx.Done():
			return false, "cancelled while waiting for health"
		case <-time.After(3 * time.Second):
			if chunkCB != nil {
				chunkCB(".")
			}
		}
	}
}
