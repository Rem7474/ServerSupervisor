package collector

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// validServiceName matches valid systemd service names.
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9._:@\-]{1,256}$`)

// ExecuteJournalctl streams systemd journal logs for a given service.
// Lines are passed to chunkCB as they arrive. Returns the full output and any error.
func ExecuteJournalctl(serviceName string, chunkCB func(string)) (string, error) {
	if serviceName == "" {
		return "", fmt.Errorf("service name is required")
	}
	if !validServiceName.MatchString(serviceName) {
		return "", fmt.Errorf("invalid service name: %q", serviceName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "journalctl",
		"-u", serviceName,
		"--no-pager",
		"-n", "200",
		"--output=short-iso",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start journalctl: %w", err)
	}

	var builder strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		builder.WriteString(line)
		if chunkCB != nil {
			chunkCB(line)
		}
	}

	// Capture stderr too
	errScanner := bufio.NewScanner(stderr)
	for errScanner.Scan() {
		line := errScanner.Text() + "\n"
		builder.WriteString(line)
		if chunkCB != nil {
			chunkCB(line)
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return builder.String(), fmt.Errorf("journalctl timed out after 30s")
		}
		return builder.String(), fmt.Errorf("journalctl exited: %w", err)
	}

	return builder.String(), nil
}
