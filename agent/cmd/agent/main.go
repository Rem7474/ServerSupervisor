package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
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

const AgentVersion = "1.3.0"

// commandMutex ensures only one APT command runs at a time
var commandMutex sync.Mutex

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

	log.Printf("ServerSupervisor Agent starting...")
	log.Printf("Server: %s", cfg.ServerURL)
	log.Printf("Report interval: %ds", cfg.ReportInterval)
	log.Printf("Docker monitoring: %v", cfg.CollectDocker)
	log.Printf("APT monitoring: %v", cfg.CollectAPT)

	// Create sender
	s := sender.New(cfg)

	// Run first report immediately
	sendReport(cfg, s)

	// Perform initial APT status collection with CVE extraction (only once at startup)
	if cfg.CollectAPT {
		go initialAptCollection(cfg, s)
	}

	// Start periodic reporting
	ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			sendReport(cfg, s)
		case <-quit:
			log.Println("Agent shutting down...")
			return
		}
	}
}

func sendReport(cfg *config.Config, s *sender.Sender) {
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

	// Send report
	report := &sender.Report{
		HostID:          cfg.HostID,
		AgentVersion:    AgentVersion,
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

	response, err := s.SendReport(report)
	if err != nil {
		log.Printf("Failed to send report: %v", err)
		return
	}

	log.Printf("Report sent successfully (CPU: %.1f%%, RAM: %.1f%%, Disks: %d)",
		metrics.CPUUsagePercent, metrics.MemoryPercent, len(metrics.Disks))

	// Process pending commands
	if len(response.Commands) > 0 {
		go processCommands(s, response.Commands)
	}
}

func processCommands(s *sender.Sender, commands []sender.PendingCommand) {
	// Try to acquire lock; if another command is running, skip this batch
	if !commandMutex.TryLock() {
		log.Println("A command is already running, skipping new commands")
		return
	}
	defer commandMutex.Unlock()

	for _, cmd := range commands {
		log.Printf("Processing command #%d: %s", cmd.ID, cmd.Type)

		switch cmd.Type {
		case "docker":
			var payload struct {
				ContainerName string `json:"container_name"`
				Action        string `json:"action"`
				WorkingDir    string `json:"working_dir"`
			}
			if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
				log.Printf("Failed to parse docker command payload: %v", err)
				if err := s.ReportCommandResult(&sender.CommandResult{
					CommandID: cmd.ID,
					Status:    "failed",
					Output:    fmt.Sprintf("invalid payload: %v", err),
					Type:      "docker",
				}); err != nil {
					log.Printf("Failed to report command result: %v", err)
				}
				continue
			}

			// Report as running
			if err := s.ReportCommandResult(&sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "running",
				Type:      "docker",
			}); err != nil {
				log.Printf("Failed to report command result: %v", err)
			}

			if payload.Action == "journalctl" {
				output, err := collector.ExecuteJournalctl(payload.ContainerName, func(chunk string) {
					if streamErr := s.StreamCommandChunk(cmd.ID, chunk); streamErr != nil {
						log.Printf("Failed to stream journal chunk: %v", streamErr)
					}
				})
				status := "completed"
				if err != nil {
					status = "failed"
					output = fmt.Sprintf("ERROR: %v\n%s", err, output)
					log.Printf("journalctl %s failed: %v", payload.ContainerName, err)
				} else {
					log.Printf("journalctl %s completed successfully", payload.ContainerName)
				}
				if err := s.ReportCommandResult(&sender.CommandResult{
					CommandID: cmd.ID,
					Status:    status,
					Output:    output,
					Type:      "docker",
				}); err != nil {
					log.Printf("Failed to report command result: %v", err)
				}
				continue
			}

			isCompose := strings.HasPrefix(payload.Action, "compose_")
			var output string
			var err error
			if isCompose {
				output, err = collector.ExecuteComposeCommand(payload.Action, payload.ContainerName, payload.WorkingDir, func(chunk string) {
					if streamErr := s.StreamCommandChunk(cmd.ID, chunk); streamErr != nil {
						log.Printf("Failed to stream compose chunk: %v", streamErr)
					}
				})
			} else {
				output, err = collector.ExecuteDockerCommand(payload.Action, payload.ContainerName, func(chunk string) {
					if streamErr := s.StreamCommandChunk(cmd.ID, chunk); streamErr != nil {
						log.Printf("Failed to stream docker chunk: %v", streamErr)
					}
				})
			}

			status := "completed"
			if err != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v\n%s", err, output)
				log.Printf("Docker %s %s failed: %v", payload.Action, payload.ContainerName, err)
			} else {
				log.Printf("Docker %s %s completed successfully", payload.Action, payload.ContainerName)
			}

			if err := s.ReportCommandResult(&sender.CommandResult{
				CommandID: cmd.ID,
				Status:    status,
				Output:    output,
				Type:      "docker",
			}); err != nil {
				log.Printf("Failed to report command result: %v", err)
			}

		case "systemd":
			var payload struct {
				ContainerName string `json:"container_name"` // service name
				Action        string `json:"action"`         // systemd_list, systemd_start, etc.
			}
			if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
				log.Printf("Failed to parse systemd command payload: %v", err)
				if err := s.ReportCommandResult(&sender.CommandResult{
					CommandID: cmd.ID,
					Status:    "failed",
					Output:    fmt.Sprintf("invalid payload: %v", err),
					Type:      "systemd",
				}); err != nil {
					log.Printf("Failed to report command result: %v", err)
				}
				continue
			}

			// Report as running
			if err := s.ReportCommandResult(&sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "running",
				Type:      "systemd",
			}); err != nil {
				log.Printf("Failed to report command result: %v", err)
			}

			// Strip "systemd_" prefix from action
			action := strings.TrimPrefix(payload.Action, "systemd_")

			if action == "list" {
				services, listErr := collector.ListSystemdServices()
				var output string
				status := "completed"
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
				if err := s.ReportCommandResult(&sender.CommandResult{
					CommandID: cmd.ID,
					Status:    status,
					Output:    output,
					Type:      "systemd",
				}); err != nil {
					log.Printf("Failed to report systemd list result: %v", err)
				}
			} else {
				output, err := collector.ExecuteSystemdCommand(payload.ContainerName, action, func(chunk string) {
					if streamErr := s.StreamCommandChunk(cmd.ID, chunk); streamErr != nil {
						log.Printf("Failed to stream systemd chunk: %v", streamErr)
					}
				})
				status := "completed"
				if err != nil {
					status = "failed"
					output = fmt.Sprintf("ERROR: %v\n%s", err, output)
					log.Printf("systemctl %s %s failed: %v", action, payload.ContainerName, err)
				} else {
					log.Printf("systemctl %s %s completed", action, payload.ContainerName)
				}
				if err := s.ReportCommandResult(&sender.CommandResult{
					CommandID: cmd.ID,
					Status:    status,
					Output:    output,
					Type:      "systemd",
				}); err != nil {
					log.Printf("Failed to report systemd command result: %v", err)
				}
			}

		case "update", "upgrade", "dist-upgrade":
			aptCmd := cmd.Type

			// Notify server that command is starting (status running)
			_ = s.ReportCommandResult(&sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "running",
				Type:      "apt",
			})

			// Execute the APT command with streaming
			output, err := collector.ExecuteAptCommandWithStreaming(aptCmd, func(chunk string) {
				if err := s.StreamCommandChunk(cmd.ID, chunk); err != nil {
					log.Printf("Failed to stream chunk: %v", err)
				}
			})
			status := "completed"
			if err != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v\n%s", err, output)
				log.Printf("APT %s failed: %v", aptCmd, err)
			} else {
				log.Printf("APT %s completed successfully", aptCmd)
			}

			// Collect APT status after command (with CVE extraction)
			var aptStatus interface{}
			log.Printf("Collecting APT status with CVE extraction after %s...", aptCmd)
			apt, aptErr := collector.CollectAPT(true)
			if aptErr != nil {
				log.Printf("Failed to collect APT status after %s: %v", aptCmd, aptErr)
				aptStatus = nil
			} else {
				aptStatus = apt
				log.Printf("APT status collected: %d packages, %d security", apt.PendingPackages, apt.SecurityUpdates)
			}

			if err := s.ReportCommandResult(&sender.CommandResult{
				CommandID: cmd.ID,
				Status:    status,
				Output:    output,
				AptStatus: aptStatus,
			}); err != nil {
				log.Printf("Failed to report command result: %v", err)
			}

		default:
			log.Printf("Unknown command type: %s", cmd.Type)
			if err := s.ReportCommandResult(&sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "failed",
				Output:    fmt.Sprintf("unknown command type: %s", cmd.Type),
			}); err != nil {
				log.Printf("Failed to report command result: %v", err)
			}
		}
	}
}

// initialAptCollection performs a full APT status check with CVE extraction at startup
func initialAptCollection(cfg *config.Config, s *sender.Sender) {
	// Wait a bit to avoid overwhelming the system at startup
	time.Sleep(5 * time.Second)

	log.Println("Performing initial APT update...")

	// Execute apt update at startup to ensure latest package list
	aptUpdateOutput, aptUpdateErr := executeAptUpdate()
	if aptUpdateErr != nil {
		log.Printf("Warning: Initial apt update failed: %v", aptUpdateErr)
	} else {
		log.Println("Initial apt update completed successfully")
	}

	log.Println("Performing APT status collection with CVE extraction...")
	apt, err := collector.CollectAPT(true) // true = extract CVE
	if err != nil {
		log.Printf("Initial APT collection failed: %v", err)
		return
	}

	log.Printf("Initial APT status: %d packages, %d security updates",
		apt.PendingPackages, apt.SecurityUpdates)

	// Send updated APT status to server
	report := &sender.Report{
		AgentVersion: AgentVersion,
		Metrics:      nil, // Skip metrics in this report
		Docker:       nil, // Skip docker in this report
		AptStatus:    apt,
		Timestamp:    time.Now(),
	}

	if _, err := s.SendReport(report); err != nil {
		log.Printf("Failed to send initial APT status: %v", err)
	} else {
		log.Println("Initial APT status with CVE sent successfully")
		// Log the apt update action with the real command output
		status := "completed"
		if aptUpdateErr != nil {
			status = "failed"
		}
		logAptAction(cfg, s, "update", status, aptUpdateOutput)
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

// logAptAction sends an audit log entry for APT actions to the server
func logAptAction(cfg *config.Config, s *sender.Sender, action, status, message string) {
	// Create a simple audit log entry (we'll send it in the next report or via API)
	log.Printf("APT Action: %s [%s] - %s", action, status, message)

	// Send it to the server via the audit endpoint if available
	// This ensures the action is logged in the dashboard
	if err := s.SendAuditLog(action, status, message); err != nil {
		log.Printf("Warning: Failed to send audit log: %v", err)
	}
}
