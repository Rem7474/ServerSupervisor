package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

const (
	agentUpdateServiceName = "serversupervisor-agent"
	agentUpdateBinaryPath  = "/usr/local/bin/serversupervisor-agent"
	agentReleaseRepo       = "serversupervisor/server"
)

type agentUpdatePayload struct {
	Version string `json:"version"`
}

func startDetachedAgentUpdate(_ *sender.Sender, cmd sender.PendingCommand, configPath string) error {
	var payload agentUpdatePayload
	if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
		return fmt.Errorf("invalid update payload: %w", err)
	}
	if payload.Version == "" {
		return fmt.Errorf("missing target version in update payload")
	}

	executablePath, err := os.Executable()
	if err != nil || executablePath == "" {
		executablePath = agentUpdateBinaryPath
	}

	if _, err := exec.LookPath("systemd-run"); err != nil {
		return fmt.Errorf("systemd-run not available: %w", err)
	}

	unitName := "serversupervisor-agent-update-" + strings.ReplaceAll(cmd.ID, "_", "-")
	args := []string{
		"--unit=" + unitName,
		"--collect",
		executablePath,
		"--internal-update",
		"--config", configPath,
		"--update-command-id", cmd.ID,
		"--update-version", payload.Version,
	}

	launcher := exec.Command("systemd-run", args...)
	launcher.Stdout = io.Discard
	launcher.Stderr = io.Discard
	if err := launcher.Start(); err != nil {
		return fmt.Errorf("failed to launch detached updater: %w", err)
	}
	return nil
}

func runInternalUpdate(cfgPath, commandID, targetVersion string) error {
	if commandID == "" {
		return fmt.Errorf("missing command id")
	}
	if targetVersion == "" {
		return fmt.Errorf("missing target version")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	s := sender.New(cfg)
	ctx := context.Background()
	progress := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		logUpdate(msg)
		if err := s.StreamCommandChunk(ctx, commandID, msg+"\n"); err != nil {
			_ = err
		}
	}

	progress("Preparing agent update to v%s", targetVersion)

	archSuffix, err := detectAgentAssetSuffix()
	if err != nil {
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}

	releaseBase := fmt.Sprintf("https://github.com/%s/releases/download/v%s", agentReleaseRepo, targetVersion)
	gzURL := fmt.Sprintf("%s/agent-%s.gz", releaseBase, archSuffix)
	shaURL := fmt.Sprintf("%s/agent-%s.gz.sha256", releaseBase, archSuffix)

	progress("Downloading release asset")
	gzBytes, err := downloadBytes(gzURL)
	if err != nil {
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}
	progress("Downloading checksum")
	checksumBytes, err := downloadBytes(shaURL)
	if err != nil {
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}

	expectedSHA, err := parseSHA256Sidecar(checksumBytes)
	if err != nil {
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}
	actualSHA := sha256.Sum256(gzBytes)
	if hex.EncodeToString(actualSHA[:]) != expectedSHA {
		return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("checksum mismatch for downloaded release asset"))
	}

	targetBinary := agentUpdateBinaryPath
	if exe, err := os.Executable(); err == nil && exe != "" {
		targetBinary = exe
	}
	targetDir := filepath.Dir(targetBinary)
	backupBinary := targetBinary + ".bak"
	tempFile, err := os.CreateTemp(targetDir, "serversupervisor-agent-*.new")
	if err != nil {
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()

	gr, err := gzip.NewReader(bytes.NewReader(gzBytes))
	if err != nil {
		_ = tempFile.Close()
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}
	if _, err := io.Copy(tempFile, gr); err != nil {
		_ = tempFile.Close()
		_ = gr.Close()
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}
	_ = gr.Close()
	if err := tempFile.Close(); err != nil {
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}
	if err := os.Chmod(tempPath, 0o755); err != nil {
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}

	progress("Installing new binary")
	if _, err := os.Stat(targetBinary); err == nil {
		_ = os.Remove(backupBinary)
		if err := os.Rename(targetBinary, backupBinary); err != nil {
			return finalizeUpdateFailure(s, ctx, commandID, progress, err)
		}
	}
	if err := os.Rename(tempPath, targetBinary); err != nil {
		if _, bakErr := os.Stat(backupBinary); bakErr == nil {
			_ = os.Rename(backupBinary, targetBinary)
		}
		return finalizeUpdateFailure(s, ctx, commandID, progress, err)
	}

	installedVersion, err := readBinaryVersion(targetBinary)
	if err != nil {
		if rollbackErr := restorePreviousBinary(targetBinary, backupBinary); rollbackErr != nil {
			return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("install verification failed (%v) and rollback failed: %w", err, rollbackErr))
		}
		return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("install verification failed: %w", err))
	}
	if installedVersion != targetVersion {
		if rollbackErr := restorePreviousBinary(targetBinary, backupBinary); rollbackErr != nil {
			return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("installed version %q does not match target %q and rollback failed: %w", installedVersion, targetVersion, rollbackErr))
		}
		return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("installed version %q does not match target %q", installedVersion, targetVersion))
	}

	progress("Restarting %s", agentUpdateServiceName)
	if err := exec.Command("systemctl", "restart", agentUpdateServiceName).Run(); err != nil {
		if rollbackErr := restorePreviousBinary(targetBinary, backupBinary); rollbackErr != nil {
			return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("service restart failed (%v) and rollback failed: %w", err, rollbackErr))
		}
		_ = exec.Command("systemctl", "restart", agentUpdateServiceName).Run()
		return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("service restart failed: %w", err))
	}

	if err := waitForServiceActive(agentUpdateServiceName, 30*time.Second); err != nil {
		if rollbackErr := restorePreviousBinary(targetBinary, backupBinary); rollbackErr != nil {
			return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("service did not become active (%v) and rollback failed: %w", err, rollbackErr))
		}
		_ = exec.Command("systemctl", "restart", agentUpdateServiceName).Run()
		return finalizeUpdateFailure(s, ctx, commandID, progress, fmt.Errorf("service did not become active: %w", err))
	}

	progress("Agent updated successfully to v%s", targetVersion)
	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: commandID,
		Status:    "completed",
		Output:    fmt.Sprintf("agent updated successfully to v%s\n", targetVersion),
	}); err != nil {
		return fmt.Errorf("failed to report update result: %w", err)
	}
	return nil
}

func finalizeUpdateFailure(s *sender.Sender, ctx context.Context, commandID string, progress func(string, ...any), err error) error {
	progress("Update failed: %v", err)
	if reportErr := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: commandID,
		Status:    "failed",
		Output:    err.Error() + "\n",
	}); reportErr != nil {
		return fmt.Errorf("%v (also failed to report result: %w)", err, reportErr)
	}
	return err
}

func detectAgentAssetSuffix() (string, error) {
	output, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return "", fmt.Errorf("failed to detect architecture: %w", err)
	}
	arch := strings.TrimSpace(string(output))
	switch arch {
	case "x86_64", "amd64":
		return "linux-amd64", nil
	case "aarch64", "arm64":
		return "linux-arm64", nil
	case "armv7l", "armv7":
		return "linux-armv7", nil
	case "armv6l", "armv6":
		return "linux-armv6", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
}

func downloadBytes(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download failed (%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func parseSHA256Sidecar(data []byte) (string, error) {
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "", fmt.Errorf("invalid sha256 sidecar")
	}
	if len(fields[0]) != 64 {
		return "", fmt.Errorf("invalid sha256 checksum format")
	}
	return fields[0], nil
}

func readBinaryVersion(binaryPath string) (string, error) {
	cmd := exec.Command(binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func restorePreviousBinary(targetBinary, backupBinary string) error {
	if _, err := os.Stat(backupBinary); err != nil {
		return err
	}
	_ = os.Remove(targetBinary)
	return os.Rename(backupBinary, targetBinary)
}

func waitForServiceActive(serviceName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := exec.Command("systemctl", "is-active", "--quiet", serviceName).Run(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("service %s did not become active within %s", serviceName, timeout)
}

func logUpdate(message string) {
	fmt.Println(message)
}
