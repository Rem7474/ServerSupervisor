package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

// Version is injected at build time via:
//
//	go build -ldflags="-X 'main.Version=<tag>'"
//
// Falls back to "dev" for local builds.
var Version = "dev"

// commandQueue delivers command batches to the worker goroutine.
// Capacity 10 lets up to 10 pending batches queue up before we start dropping.
var commandQueue = make(chan []sender.PendingCommand, 10)

// aptMu serialises APT operations — dpkg cannot run concurrently.
var aptMu sync.Mutex

// skipSystemMetrics is set to true when the server signals that Proxmox is the
// designated metrics source for this host. System metrics (CPU/RAM/load) are then
// omitted from reports to avoid redundant collection and storage.
var skipSystemMetrics atomic.Bool

// cmdSem limits concurrent non-APT commands to 4 at a time.
var cmdSem = make(chan struct{}, 4)

// tasksConfig holds custom tasks loaded from the local YAML file at startup.
var tasksConfig *config.TasksConfig

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	configPath := flag.String("config", "/etc/serversupervisor/agent.yaml", "Path to config file")
	initConfig := flag.Bool("init", false, "Generate a default config file")
	flag.Parse()

	// Generate default config
	if *initConfig {
		fmt.Print(config.DefaultConfigFile())
		return
	}

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("ServerSupervisor Agent starting (version: %s)", Version)
	log.Printf("Server: %s", cfg.ServerURL)
	log.Printf("Report interval: %ds", cfg.ReportInterval)
	log.Printf("Docker monitoring: %v", cfg.CollectDocker)
	log.Printf("APT monitoring: %v", cfg.CollectAPT)
	log.Printf("APT auto-update on start: %v", cfg.AptAutoUpdateOnStart)
	log.Printf("SMART monitoring: %v", cfg.CollectSMART)
	log.Printf("CPU temperature monitoring: %v", cfg.CollectCPUTemperature)
	log.Printf("Web logs analytics: %v (paths: %v)", cfg.CollectWebLogs, cfg.WebLogGlobs())
	if cfg.MaxReportBodyBytes <= 0 {
		cfg.MaxReportBodyBytes = 3 * 1024 * 1024
	}
	log.Printf("Max report body size: %d bytes", cfg.MaxReportBodyBytes)

	// Load custom tasks config (tasks.yaml)
	tc, err := config.LoadTasksConfig()
	if err != nil {
		log.Printf("Warning: failed to load tasks config: %v", err)
		tc = &config.TasksConfig{}
	} else {
		log.Printf("Loaded %d custom task(s) from tasks config", len(tc.Tasks))
	}
	tasksConfig = tc

	// Create sender
	s := sender.New(cfg)

	// ctx is cancelled on SIGINT/SIGTERM — stops the periodic report loop.
	// Command execution uses context.Background() so in-flight commands complete.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the sequential command worker — ensures commands never overlap and
	// are never silently dropped by a failed TryLock.
	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		for cmds := range commandQueue {
			processCommands(s, cmds)
		}
	}()

	// Run first report immediately
	sendReport(ctx, cfg, s)

	// Perform initial APT status collection with CVE extraction (only once at startup)
	if cfg.CollectAPT {
		go initialAptCollection(ctx, cfg, s)
	}

	// Start periodic reporting
	ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sendReport(ctx, cfg, s)
		case <-ctx.Done():
			log.Println("Agent shutting down...")
			// Stop accepting new commands; wait for the current batch to finish.
			close(commandQueue)
			workerWg.Wait()
			return
		}
	}
}

func sendReport(ctx context.Context, cfg *config.Config, s *sender.Sender) {
	// When the server designates Proxmox as the metrics source for this host,
	// skip CPU/RAM/load collection entirely — Proxmox polling already covers it.
	// Use interface{} so the JSON field is truly null (not a typed nil interface).
	var metricsPayload interface{}
	var collectedMetrics *collector.SystemMetrics
	if skipSystemMetrics.Load() {
		log.Printf("System metrics collection skipped (Proxmox is the designated metrics source)")
	} else {
		m, err := collector.CollectSystem(cfg.CollectCPUTemperature)
		if err != nil {
			log.Printf("Failed to collect system metrics: %v", err)
			return
		}
		collectedMetrics = m
		metricsPayload = m
	}

	// Collect Docker containers
	var dockerData interface{}
	var dockerNetworks interface{}
	var containerEnvs interface{}
	var composeProjects interface{}
	if cfg.CollectDocker {
		containers, err := collector.CollectDocker()
		if err != nil {
			log.Printf("Docker collection skipped: %v", err)
			// Keep payload nil on collector failure so server state is not wiped.
			dockerData = nil
		} else {
			dockerData = struct {
				Containers []collector.DockerContainer `json:"containers"`
			}{Containers: containers}

			// Collect Docker networks
			if networks, err := collector.CollectDockerNetworks(); err == nil {
				dockerNetworks = networks
			}

			// Build container env vars from already-collected container data (no extra Docker API calls)
			envs := make([]collector.ContainerEnv, 0, len(containers))
			for _, c := range containers {
				if len(c.EnvVars) > 0 {
					envs = append(envs, collector.ContainerEnv{
						ContainerName: c.Name,
						EnvVars:       c.EnvVars,
					})
				}
			}
			containerEnvs = envs

			// Collect docker-compose projects
			if projects, err := collector.CollectComposeProjects(); err == nil {
				composeProjects = projects
			} else {
				log.Printf("Compose projects collection skipped: %v", err)
			}
		}
	} else {
		dockerData = struct {
			Containers []interface{} `json:"containers"`
		}{Containers: []interface{}{}}
	}

	// Don't collect APT in periodic reports to avoid overwriting CVE history
	// APT status is only collected:
	// 1. At agent startup (with CVE)
	// 2. After manual apt update/upgrade commands (with CVE)
	var aptData interface{} = nil

	// Collect disk metrics and health
	diskMetrics, err := collector.CollectDiskMetrics()
	if err != nil {
		log.Printf("Failed to collect disk metrics: %v", err)
	}

	var diskHealth []collector.DiskHealth
	if cfg.CollectSMART {
		diskHealth, err = collector.CollectDiskHealth()
		if err != nil {
			log.Printf("Failed to collect disk health (smartctl may not be installed): %v", err)
		}
	}

	// Include custom task summaries so the server can display them in the UI.
	var customTasksList interface{}
	if tasksConfig != nil && len(tasksConfig.Tasks) > 0 {
		customTasksList = tasksConfig.Summaries()
	}

	// Collect and parse web access logs once to build both traffic and threat views.
	var webLogs interface{}
	if cfg.CollectWebLogs {
		globs := cfg.WebLogGlobs()
		log.Printf("Web logs: scanning globs %v", globs)
		report, err := collector.CollectWebLogs(globs, cfg.WebLogsTailLines, cfg.WebLogsTopN, cfg.WebLogsRequestsLimit, cfg.WebLogsCursorFile)
		if err != nil {
			log.Printf("Web logs collection skipped: %v", err)
		} else {
			suspicious := 0
			if report.Threats != nil {
				suspicious = report.Threats.SuspiciousRequests
			}
			log.Printf("Web logs: source=%s files=%d requests=%d suspicious=%d",
				report.Source, len(report.LogFilesScanned), report.TotalRequests, suspicious)
			webLogs = report
		}
	}

	// Send report (with retry on transient network errors)
	report := &sender.Report{
		AgentVersion:    Version,
		Metrics:         metricsPayload,
		Docker:          dockerData,
		AptStatus:       aptData,
		WebLogs:         webLogs,
		DockerNetworks:  dockerNetworks,
		ContainerEnvs:   containerEnvs,
		ComposeProjects: composeProjects,
		DiskMetrics:     diskMetrics,
		DiskHealth:      diskHealth,
		CustomTasks:     customTasksList,
		Timestamp:       time.Now(),
	}
	trimWebLogsForReportSize(report, cfg.MaxReportBodyBytes)

	response, err := s.SendReportWithRetry(ctx, report)
	if err != nil {
		log.Printf("Failed to send report: %v", err)
		return
	}

	if collectedMetrics != nil {
		log.Printf("Report sent successfully (CPU: %.1f%%, RAM: %.1f%%, Disks: %d)",
			collectedMetrics.CPUUsagePercent, collectedMetrics.MemoryPercent, len(collectedMetrics.Disks))
	} else {
		log.Printf("Report sent successfully (system metrics skipped — Proxmox source)")
	}

	// Update the skip flag for the next cycle based on server directive.
	skipSystemMetrics.Store(response.SkipMetrics)

	// Enqueue pending commands — the worker goroutine processes them sequentially.
	if len(response.Commands) > 0 {
		select {
		case commandQueue <- response.Commands:
		default:
			// Queue is full. Report each command as failed immediately so the server
			// doesn't leave them in "pending" state waiting for the stalled-command
			// cleanup timeout (10 minutes). The user will see a clear failure reason.
			log.Printf("Command queue full (%d batches pending), reporting %d commands as failed",
				len(commandQueue), len(response.Commands))
			for _, cmd := range response.Commands {
				if err := s.ReportCommandResult(ctx, &sender.CommandResult{
					CommandID: cmd.ID,
					Status:    "failed",
					Output:    "command dropped: agent command queue was full — try again",
				}); err != nil {
					log.Printf("Failed to report dropped command %s as failed: %v", cmd.ID, err)
				}
			}
		}
	}
}

func trimWebLogsForReportSize(report *sender.Report, maxBodyBytes int) {
	if report == nil || maxBodyBytes <= 0 {
		return
	}
	web, ok := report.WebLogs.(*collector.WebLogReport)
	if !ok || web == nil {
		return
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		return
	}
	if len(encoded) <= maxBodyBytes {
		return
	}

	original := len(web.Requests)
	if original == 0 {
		return
	}

	trimmed := web.Requests
	for len(trimmed) > 0 {
		nextLen := len(trimmed) / 2
		if nextLen == len(trimmed) {
			nextLen--
		}
		if nextLen < 0 {
			nextLen = 0
		}
		trimmed = trimmed[:nextLen]
		web.Requests = trimmed

		encoded, err = json.Marshal(report)
		if err != nil {
			break
		}
		if len(encoded) <= maxBodyBytes {
			break
		}
	}

	log.Printf("Web logs payload trimmed: requests %d -> %d to fit max_report_body_bytes=%d", original, len(web.Requests), maxBodyBytes)
}

// processCommands dispatches each command in its own goroutine.
// APT commands are serialised via aptMu (dpkg cannot run concurrently).
// All other modules share a semaphore limiting parallelism to 4.
// Commands use context.Background() so they complete even during agent shutdown.
func processCommands(s *sender.Sender, commands []sender.PendingCommand) {
	var wg sync.WaitGroup
	for _, cmd := range commands {
		wg.Add(1)
		go func(c sender.PendingCommand) {
			defer wg.Done()
			if c.Module == "apt" {
				aptMu.Lock()
				defer aptMu.Unlock()
			} else {
				cmdSem <- struct{}{}
				defer func() { <-cmdSem }()
			}
			executeCommand(s, c)
		}(cmd)
	}
	wg.Wait()
}

func executeCommand(s *sender.Sender, cmd sender.PendingCommand) {
	// Commands use a background context so they complete even if the agent is shutting down.
	ctx := context.Background()
	log.Printf("Processing command %s: module=%s action=%s target=%s", cmd.ID, cmd.Module, cmd.Action, cmd.Target)

	switch cmd.Module {
	case "docker":
		// Parse extra args (working_dir for compose operations)
		var extra struct {
			WorkingDir string `json:"working_dir"`
		}
		_ = json.Unmarshal([]byte(cmd.Payload), &extra)

		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
		}); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		isCompose := strings.HasPrefix(cmd.Action, "compose_")
		var output string
		var execErr error
		if isCompose {
			output, execErr = collector.ExecuteComposeCommand(cmd.Action, cmd.Target, extra.WorkingDir, func(chunk string) {
				if streamErr := s.StreamCommandChunk(ctx, cmd.ID, chunk); streamErr != nil {
					log.Printf("Failed to stream compose chunk: %v", streamErr)
				}
			})
		} else {
			output, execErr = collector.ExecuteDockerCommand(cmd.Action, cmd.Target, func(chunk string) {
				if streamErr := s.StreamCommandChunk(ctx, cmd.ID, chunk); streamErr != nil {
					log.Printf("Failed to stream docker chunk: %v", streamErr)
				}
			})
		}

		status := "completed"
		if execErr != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v\n%s", execErr, output)
			log.Printf("Docker %s %s failed: %v", cmd.Action, cmd.Target, execErr)
		} else {
			log.Printf("Docker %s %s completed", cmd.Action, cmd.Target)
		}
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    status,
			Output:    output,
		}); err != nil {
			log.Printf("Failed to report command result: %v", err)
		}

	case "journal":
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
		}); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		output, err := collector.ExecuteJournalctl(cmd.Target, func(chunk string) {
			if streamErr := s.StreamCommandChunk(ctx, cmd.ID, chunk); streamErr != nil {
				log.Printf("Failed to stream journal chunk: %v", streamErr)
			}
		})
		status := "completed"
		if err != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v\n%s", err, output)
			log.Printf("journalctl %s failed: %v", cmd.Target, err)
		} else {
			log.Printf("journalctl %s completed", cmd.Target)
		}
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    status,
			Output:    output,
		}); err != nil {
			log.Printf("Failed to report command result: %v", err)
		}

	case "apt":
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
		}); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		output, err := collector.ExecuteAptCommandWithStreaming(cmd.Action, func(chunk string) {
			if streamErr := s.StreamCommandChunk(ctx, cmd.ID, chunk); streamErr != nil {
				log.Printf("Failed to stream apt chunk: %v", streamErr)
			}
		})
		status := "completed"
		if err != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v\n%s", err, output)
			log.Printf("APT %s failed: %v", cmd.Action, err)
		} else {
			log.Printf("APT %s completed", cmd.Action)
		}

		// Collect APT status after the command (with CVE extraction)
		var aptStatus interface{}
		log.Printf("Collecting APT status with CVE extraction after %s...", cmd.Action)
		apt, aptErr := collector.CollectAPT(true)
		if aptErr != nil {
			log.Printf("Failed to collect APT status after %s: %v", cmd.Action, aptErr)
		} else {
			aptStatus = apt
			log.Printf("APT status collected: %d packages, %d security", apt.PendingPackages, apt.SecurityUpdates)
		}

		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    status,
			Output:    output,
			AptStatus: aptStatus,
		}); err != nil {
			log.Printf("Failed to report command result: %v", err)
		}

	case "systemd":
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
		}); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		if cmd.Action == "list" {
			services, listErr := collector.ListSystemdServices()
			status := "completed"
			var output string
			if listErr != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v", listErr)
				log.Printf("systemctl list-units failed: %v", listErr)
			} else {
				jsonBytes, jsonErr := json.Marshal(services)
				if jsonErr != nil {
					status = "failed"
					output = fmt.Sprintf("ERROR marshaling services: %v", jsonErr)
				} else {
					output = string(jsonBytes)
					log.Printf("systemctl list: %d services returned", len(services))
				}
			}
			if err := s.ReportCommandResult(ctx, &sender.CommandResult{
				CommandID: cmd.ID,
				Status:    status,
				Output:    output,
			}); err != nil {
				log.Printf("Failed to report systemd list result: %v", err)
			}
		} else {
			output, err := collector.ExecuteSystemdCommand(cmd.Target, cmd.Action, func(chunk string) {
				if streamErr := s.StreamCommandChunk(ctx, cmd.ID, chunk); streamErr != nil {
					log.Printf("Failed to stream systemd chunk: %v", streamErr)
				}
			})
			status := "completed"
			if err != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v\n%s", err, output)
				log.Printf("systemctl %s %s failed: %v", cmd.Action, cmd.Target, err)
			} else {
				log.Printf("systemctl %s %s completed", cmd.Action, cmd.Target)
			}
			if err := s.ReportCommandResult(ctx, &sender.CommandResult{
				CommandID: cmd.ID,
				Status:    status,
				Output:    output,
			}); err != nil {
				log.Printf("Failed to report systemd command result: %v", err)
			}
		}

	case "processes":
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
		}); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		procs, procErr := collector.GetProcessList()
		status := "completed"
		var output string
		if procErr != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v", procErr)
			log.Printf("ps failed: %v", procErr)
		} else {
			jsonBytes, jsonErr := json.Marshal(procs)
			if jsonErr != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR marshaling processes: %v", jsonErr)
			} else {
				output = string(jsonBytes)
				log.Printf("processes list: %d processes returned", len(procs))
			}
		}
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    status,
			Output:    output,
		}); err != nil {
			log.Printf("Failed to report processes result: %v", err)
		}

	case "custom":
		executeCustomTask(ctx, s, cmd)

	default:
		log.Printf("Unknown command module: %s", cmd.Module)
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "failed",
			Output:    fmt.Sprintf("unknown command module: %s", cmd.Module),
		}); err != nil {
			log.Printf("Failed to report command result: %v", err)
		}
	}
}

// executeCustomTask runs a task defined in the local tasks.yaml configuration.
// cmd.Target holds the task ID; the command argv is exec'd directly (no shell).
func executeCustomTask(ctx context.Context, s *sender.Sender, cmd sender.PendingCommand) {
	task := tasksConfig.FindTask(cmd.Target)
	if task == nil {
		log.Printf("Custom task %q not found in local tasks config", cmd.Target)
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "failed",
			Output:    fmt.Sprintf("task %q not found in local tasks config (tasks.yaml)", cmd.Target),
		}); err != nil {
			log.Printf("Failed to report custom task result: %v", err)
		}
		return
	}

	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    "running",
	}); err != nil {
		log.Printf("Failed to report running status: %v", err)
	}

	timeout := time.Duration(task.Timeout) * time.Second
	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// exec.CommandContext with argv — no shell, no injection possible.
	c := exec.CommandContext(taskCtx, task.Command[0], task.Command[1:]...)

	// Inject env vars from payload (e.g. SS_REPO_NAME, SS_BRANCH set by webhook triggers)
	var payloadData struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(cmd.Payload), &payloadData); err == nil && len(payloadData.Env) > 0 {
		extraEnv := make([]string, 0, len(payloadData.Env))
		for k, v := range payloadData.Env {
			extraEnv = append(extraEnv, k+"="+v)
		}
		c.Env = append(os.Environ(), extraEnv...)
	}

	stdout, err := c.StdoutPipe()
	if err != nil {
		_ = s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID, Status: "failed",
			Output: fmt.Sprintf("ERROR: %v", err),
		})
		return
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		_ = s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID, Status: "failed",
			Output: fmt.Sprintf("ERROR: %v", err),
		})
		return
	}

	if err := c.Start(); err != nil {
		_ = s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID, Status: "failed",
			Output: fmt.Sprintf("ERROR: failed to start: %v", err),
		})
		return
	}

	var builder strings.Builder
	streamPipe := func(r interface{ Read([]byte) (int, error) }) {
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				builder.WriteString(chunk)
				if streamErr := s.StreamCommandChunk(ctx, cmd.ID, chunk); streamErr != nil {
					log.Printf("Failed to stream custom task chunk: %v", streamErr)
				}
			}
			if readErr != nil {
				break
			}
		}
	}

	var pipeWg sync.WaitGroup
	pipeWg.Add(2)
	go func() { defer pipeWg.Done(); streamPipe(stdout) }()
	go func() { defer pipeWg.Done(); streamPipe(stderr) }()
	pipeWg.Wait()

	waitErr := c.Wait()
	status := "completed"
	output := builder.String()
	if waitErr != nil {
		status = "failed"
		if taskCtx.Err() == context.DeadlineExceeded {
			output += fmt.Sprintf("\nERROR: task timed out after %ds", task.Timeout)
		} else {
			output += fmt.Sprintf("\nERROR: %v", waitErr)
		}
		log.Printf("Custom task %q failed: %v", task.ID, waitErr)
	} else {
		log.Printf("Custom task %q completed", task.ID)
	}

	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    status,
		Output:    output,
	}); err != nil {
		log.Printf("Failed to report custom task result: %v", err)
	}
}

// initialAptCollection performs a full APT status check with CVE extraction at startup
func initialAptCollection(ctx context.Context, cfg *config.Config, s *sender.Sender) {
	// Wait a bit to avoid overwhelming the system at startup; exit early on shutdown.
	select {
	case <-time.After(5 * time.Second):
	case <-ctx.Done():
		return
	}

	var aptUpdateOutput string
	var aptUpdateErr error

	if cfg.AptAutoUpdateOnStart {
		log.Println("Performing initial APT update (apt_auto_update_on_start=true)...")
		aptUpdateOutput, aptUpdateErr = executeAptUpdate()
		status := "completed"
		if aptUpdateErr != nil {
			status = "failed"
			log.Printf("Warning: Initial apt update failed: %v", aptUpdateErr)
		} else {
			log.Println("Initial apt update completed successfully")
		}
		// Log the command immediately — independently of the APT status report below.
		logAptAction(ctx, s, "update", status, aptUpdateOutput)
	}

	log.Println("Performing APT status collection with CVE extraction...")
	apt, err := collector.CollectAPT(true) // true = extract CVE
	if err != nil {
		log.Printf("Initial APT collection failed: %v", err)
		return
	}

	log.Printf("Initial APT status: %d packages, %d security updates",
		apt.PendingPackages, apt.SecurityUpdates)

	// Send updated APT status to server (best-effort)
	report := &sender.Report{
		AgentVersion: Version,
		Metrics:      nil, // Skip metrics in this report
		Docker:       nil, // Skip docker in this report
		AptStatus:    apt,
		Timestamp:    time.Now(),
	}

	if _, err := s.SendReportWithRetry(ctx, report); err != nil {
		log.Printf("Failed to send initial APT status: %v", err)
	} else {
		log.Println("Initial APT status with CVE sent successfully")
	}
}

// executeAptUpdate runs apt update command, logs real output and returns it
func executeAptUpdate() (string, error) {
	cmd := exec.Command("apt", "update")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	if len(output) > 0 {
		log.Printf("[apt update]\n%s", outputStr)
	}
	if err != nil {
		return outputStr, fmt.Errorf("apt update failed: %w", err)
	}
	return outputStr, nil
}

// logAptAction sends an audit log entry for APT actions to the server.
// module="apt" tells the server to also create a remote_command record so the
// action appears in the unified commands history (Audit → Commandes tab).
func logAptAction(ctx context.Context, s *sender.Sender, action, status, message string) {
	log.Printf("APT Action: %s [%s] - %s", action, status, message)

	if err := s.SendAuditLog(ctx, "apt", action, status, message); err != nil {
		log.Printf("Warning: Failed to send audit log: %v", err)
	}
}
