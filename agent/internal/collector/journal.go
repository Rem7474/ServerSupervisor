package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// validServiceName matches valid systemd service names.
var validServiceName = regexp.MustCompile(`^[a-zA-Z0-9._:@\-]{1,256}$`)

// journalEntry represents a single journald log entry in --output=json format.
// journalctl emits one JSON object per line (NDJSON).
type journalEntry struct {
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"` // microseconds since epoch
	Message           string `json:"MESSAGE"`
	Priority          string `json:"PRIORITY"` // syslog priority 0-7
	SyslogIdentifier  string `json:"SYSLOG_IDENTIFIER"`
}

var priorityLabel = [8]string{"EMERG", "ALERT", "CRIT", "ERR", "WARN", "NOTICE", "INFO", "DEBUG"}

func formatJournalLine(e journalEntry) string {
	ts := time.Now().UTC()
	if usec, err := strconv.ParseInt(e.RealtimeTimestamp, 10, 64); err == nil {
		ts = time.UnixMicro(usec).UTC()
	}

	prio := "INFO"
	if p, err := strconv.Atoi(e.Priority); err == nil && p >= 0 && p < len(priorityLabel) {
		prio = priorityLabel[p]
	}

	if e.SyslogIdentifier != "" {
		return fmt.Sprintf("[%s] %s %s: %s", ts.Format(time.RFC3339), prio, e.SyslogIdentifier, e.Message)
	}
	return fmt.Sprintf("[%s] %s %s", ts.Format(time.RFC3339), prio, e.Message)
}

// ExecuteJournalctl streams systemd journal logs for a given service.
// Lines are passed to chunkCB as they arrive. Returns the full output and any error.
// journalctl --output=json is used for structured parsing; each JSON line is formatted
// as "[TIMESTAMP] PRIORITY IDENTIFIER: MESSAGE" before being forwarded.
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
		"--output=json",
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
		var entry journalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Fallback: forward raw line as-is
			line := scanner.Text() + "\n"
			builder.WriteString(line)
			if chunkCB != nil {
				chunkCB(line)
			}
			continue
		}
		line := formatJournalLine(entry) + "\n"
		builder.WriteString(line)
		if chunkCB != nil {
			chunkCB(line)
		}
	}

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
