package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
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

// cmdSem limits concurrent non-APT commands to 4 at a time.
var cmdSem = make(chan struct{}, 4)

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
	// Collect system metrics
	metrics, err := collector.CollectSystem()
	if err != nil {
		log.Printf("Failed to collect system metrics: %v", err)
		return
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
			dockerData = struct {
				Containers []interface{} `json:"containers"`
			}{Containers: []interface{}{}}
		} else {
			dockerData = struct {
				Containers []collector.DockerContainer `json:"containers"`
			}{Containers: containers}

			// Collect Docker networks
			if networks, err := collector.CollectDockerNetworks(); err == nil {
				dockerNetworks = networks
			}

			// Collect container environment variables
			containerNames := make([]string, len(containers))
			for i, c := range containers {
				containerNames[i] = c.Name
			}
			if envs, err := collector.CollectContainerEnvVars(containerNames); err == nil {
				containerEnvs = envs
			}

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

	diskHealth, err := collector.CollectDiskHealth()
	if err != nil {
		log.Printf("Failed to collect disk health (smartctl may not be installed): %v", err)
	}

	// Send report (with retry on transient network errors)
	report := &sender.Report{
		AgentVersion:    Version,
		Metrics:         metrics,
		Docker:          dockerData,
		AptStatus:       aptData,
		DockerNetworks:  dockerNetworks,
		ContainerEnvs:   containerEnvs,
		ComposeProjects: composeProjects,
		DiskMetrics:     diskMetrics,
		DiskHealth:      diskHealth,
		Timestamp:       time.Now(),
	}

	response, err := s.SendReportWithRetry(ctx, report)
	if err != nil {
		log.Printf("Failed to send report: %v", err)
		return
	}

	log.Printf("Report sent successfully (CPU: %.1f%%, RAM: %.1f%%, Disks: %d)",
		metrics.CPUUsagePercent, metrics.MemoryPercent, len(metrics.Disks))

	// Enqueue pending commands — the worker goroutine processes them sequentially.
	if len(response.Commands) > 0 {
		select {
		case commandQueue <- response.Commands:
		default:
			log.Printf("Command queue full (%d batches pending), dropping batch of %d commands",
				len(commandQueue), len(response.Commands))
		}
	}
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
