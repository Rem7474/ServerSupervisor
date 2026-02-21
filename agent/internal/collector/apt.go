package collector

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type AptStatus struct {
	LastUpdate      time.Time `json:"last_update"`
	LastUpgrade     time.Time `json:"last_upgrade"`
	PendingPackages int       `json:"pending_packages"`
	PackageList     string    `json:"package_list"` // JSON array
	SecurityUpdates int       `json:"security_updates"`
}

// CollectAPT checks for available APT updates
func CollectAPT() (*AptStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	status := &AptStatus{}

	// Check if apt is available
	if _, err := exec.LookPath("apt"); err != nil {
		return nil, fmt.Errorf("apt not found in PATH")
	}

	// Get last update time from apt history log
	status.LastUpdate = getLastAptAction("Start-Date", "/var/log/apt/history.log")
	status.LastUpgrade = getLastAptUpgrade()

	// List upgradable packages
	out, err := exec.CommandContext(ctx, "apt", "list", "--upgradable").Output()
	if err != nil {
		log.Printf("apt list --upgradable failed: %v", err)
		return status, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var packages []string
	secCount := 0

	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}
		// Format: package/suite version arch [upgradable from: version]
		parts := strings.SplitN(line, "/", 2)
		if len(parts) >= 1 {
			packages = append(packages, parts[0])
		}
		if strings.Contains(line, "-security") {
			secCount++
		}
	}

	status.PendingPackages = len(packages)
	status.SecurityUpdates = secCount

	// Build JSON array of package names
	if len(packages) > 0 {
		quoted := make([]string, len(packages))
		for i, p := range packages {
			quoted[i] = fmt.Sprintf("%q", p)
		}
		status.PackageList = "[" + strings.Join(quoted, ",") + "]"
	} else {
		status.PackageList = "[]"
	}

	log.Printf("APT: %d upgradable packages (%d security)", status.PendingPackages, status.SecurityUpdates)
	return status, nil
}

// ExecuteAptCommand runs an apt command (update, upgrade, dist-upgrade)
func ExecuteAptCommand(command string) (string, error) {
	return ExecuteAptCommandWithStreaming(command, nil)
}

// ExecuteAptCommandWithStreaming runs an apt command with real-time output streaming
// streamCallback is called for each chunk of output (can be nil)
func ExecuteAptCommandWithStreaming(command string, streamCallback func(chunk string)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	switch command {
	case "update":
		cmd = exec.CommandContext(ctx, "sudo", "apt", "update", "-y")
	case "upgrade":
		cmd = exec.CommandContext(ctx, "sudo", "apt", "upgrade", "-y")
	case "dist-upgrade":
		cmd = exec.CommandContext(ctx, "sudo", "apt", "dist-upgrade", "-y")
	default:
		return "", fmt.Errorf("unknown apt command: %s", command)
	}

	log.Printf("Executing: sudo apt %s -y", command)

	// If streaming callback provided, capture output in real-time
	if streamCallback != nil {
		output, err := runCommandWithStreaming(cmd, streamCallback)
		if err != nil {
			return output, fmt.Errorf("apt %s failed: %w\nOutput: %s", command, err, output)
		}
		log.Printf("apt %s completed successfully", command)
		return output, nil
	}

	// Fallback to simple execution
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err != nil {
		return output, fmt.Errorf("apt %s failed: %w\nOutput: %s", command, err, output)
	}

	log.Printf("apt %s completed successfully", command)
	return output, nil
}

// runCommandWithStreaming executes a command and streams output via callback
func runCommandWithStreaming(cmd *exec.Cmd, streamCallback func(chunk string)) (string, error) {
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

	var fullOutput strings.Builder
	buf := make([]byte, 1024)

	// Read stdout and stderr concurrently
	done := make(chan bool)
	go func() {
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullOutput.WriteString(chunk)
				streamCallback(chunk)
			}
			if err != nil {
				break
			}
		}
		done <- true
	}()

	go func() {
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				fullOutput.WriteString(chunk)
				streamCallback(chunk)
			}
			if err != nil {
				break
			}
		}
		done <- true
	}()

	// Wait for both streams to finish
	<-done
	<-done

	err = cmd.Wait()
	return fullOutput.String(), err
}

func getLastAptAction(prefix, logFile string) time.Time {
	out, err := exec.Command("grep", prefix, logFile).Output()
	if err != nil {
		return time.Time{}
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return time.Time{}
	}
	// Get last entry
	lastLine := lines[len(lines)-1]
	parts := strings.SplitN(lastLine, ":", 2)
	if len(parts) != 2 {
		return time.Time{}
	}
	ts := strings.TrimSpace(parts[1])
	t, _ := time.Parse("2006-01-02  15:04:05", ts)
	return t
}

func getLastAptUpgrade() time.Time {
	out, err := exec.Command("stat", "-c", "%Y", "/var/lib/dpkg/info").Output()
	if err != nil {
		return time.Time{}
	}
	ts := strings.TrimSpace(string(out))
	epoch, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(epoch, 0)
}
