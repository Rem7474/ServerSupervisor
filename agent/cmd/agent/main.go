package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

const AgentVersion = "1.2.0"

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

	// Send report
	report := &sender.Report{
		AgentVersion: AgentVersion,
		Metrics:      metrics,
		Docker:       dockerData,
		AptStatus:    aptData,
		Timestamp:    time.Now(),
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

		var aptCmd string
		switch cmd.Type {
		case "update":
			aptCmd = "update"
		case "upgrade":
			aptCmd = "upgrade"
		case "dist-upgrade":
			aptCmd = "dist-upgrade"
		default:
			log.Printf("Unknown command type: %s", cmd.Type)
			s.ReportCommandResult(&sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "failed",
				Output:    fmt.Sprintf("unknown command type: %s", cmd.Type),
			})
			continue
		}

		// Notify server that command is starting
		if err := s.ReportCommandStatus(cmd.ID, "running"); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		// Execute the APT command with streaming
		output, err := collector.ExecuteAptCommandWithStreaming(aptCmd, func(chunk string) {
			// Stream each chunk to the server
			if err := s.StreamCommandChunk(cmd.ID, chunk); err != nil {
				log.Printf("Failed to stream chunk: %v", err)
			}
		})
		status := "completed"
		if err != nil {
			status = "failed"
			output = err.Error() + "\n" + output
		}

		// Collect APT status after successful update/upgrade (with CVE extraction)
		var aptStatus interface{}
		if status == "completed" {
			log.Printf("Collecting APT status with CVE extraction after %s...", aptCmd)
			apt, aptErr := collector.CollectAPT(true) // true = extract CVE
			if aptErr != nil {
				log.Printf("Failed to collect APT status: %v", aptErr)
				aptStatus = nil
			} else {
				aptStatus = apt
				log.Printf("APT status collected: %d packages, %d security",
					apt.PendingPackages, apt.SecurityUpdates)
			}
		}

		// Report command result with APT status
		s.ReportCommandResult(&sender.CommandResult{
			CommandID: cmd.ID,
			Status:    status,
			Output:    output,
			AptStatus: aptStatus,
		})
	}
}

// initialAptCollection performs a full APT status check with CVE extraction at startup
func initialAptCollection(cfg *config.Config, s *sender.Sender) {
	// Wait a bit to avoid overwhelming the system at startup
	time.Sleep(5 * time.Second)

	log.Println("🔍 Performing initial APT status collection with CVE extraction...")
	apt, err := collector.CollectAPT(true) // true = extract CVE
	if err != nil {
		log.Printf("❌ Initial APT collection failed: %v", err)
		return
	}

	log.Printf("✅ Initial APT status: %d packages, %d security updates",
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
		log.Printf("❌ Failed to send initial APT status: %v", err)
	} else {
		log.Println("✅ Initial APT status with CVE sent successfully")
	}
}
