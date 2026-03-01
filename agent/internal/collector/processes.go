package collector

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ProcessInfo represents a single running process.
type ProcessInfo struct {
	PID    int     `json:"pid"`
	PPID   int     `json:"ppid"`
	User   string  `json:"user"`
	CPUPct float64 `json:"cpu_pct"`
	MemPct float64 `json:"mem_pct"`
	MemRSS int64   `json:"mem_rss_kb"`
	State  string  `json:"state"`
	Name   string  `json:"name"`
}

// GetProcessList returns a snapshot of all running processes via ps.
// Fields: pid, ppid, user, %cpu, %mem, rss (KB), stat, comm.
func GetProcessList() ([]ProcessInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ps", "-eo", "pid,ppid,user,pcpu,pmem,rss,stat,comm",
		"--no-header", "--sort=-pcpu",
	)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("ps timed out")
		}
		return nil, fmt.Errorf("ps failed: %w", err)
	}

	var processes []ProcessInfo
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		cpu, _ := strconv.ParseFloat(fields[3], 64)
		mem, _ := strconv.ParseFloat(fields[4], 64)
		rss, _ := strconv.ParseInt(fields[5], 10, 64)

		processes = append(processes, ProcessInfo{
			PID:    pid,
			PPID:   ppid,
			User:   fields[2],
			CPUPct: cpu,
			MemPct: mem,
			MemRSS: rss,
			State:  fields[6],
			Name:   fields[7],
		})
	}
	return processes, nil
}
