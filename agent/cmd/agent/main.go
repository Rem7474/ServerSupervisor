package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/dispatcher"
	"github.com/serversupervisor/agent/internal/reporter"
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

// agentConfigPath stores the active configuration path for helper commands.
var agentConfigPath = "/etc/serversupervisor/agent.yaml"

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	configPath := flag.String("config", "/etc/serversupervisor/agent.yaml", "Path to config file")
	initConfig := flag.Bool("init", false, "Generate and write a default config file")
	initForce := flag.Bool("init-force", false, "Allow overwriting existing config file when used with --init")
	initServerURL := flag.String("server-url", "", "Server URL override used with --init")
	initAPIKey := flag.String("api-key", "", "API key override used with --init")
	showVersion := flag.Bool("version", false, "Print the agent version and exit")
	internalUpdate := flag.Bool("internal-update", false, "Run the detached self-update helper and exit")
	updateCommandID := flag.String("update-command-id", "", "Command ID for internal update helper")
	updateVersion := flag.String("update-version", "", "Target version for internal update helper")
	verbose := flag.Bool("verbose", false, "Enable verbose/debug logging output")
	flag.Parse()

	agentConfigPath = *configPath

	if *showVersion {
		fmt.Println(Version)
		return
	}

	if *internalUpdate {
		if err := runInternalUpdate(*configPath, *updateCommandID, *updateVersion); err != nil {
			log.Printf("Internal update failed: %v", err)
			os.Exit(1)
		}
		return
	}

	if *initConfig {
		content := config.DefaultConfigFileWithOverrides(*initServerURL, *initAPIKey)

		if *configPath == "-" {
			fmt.Print(content)
			return
		}

		if _, err := os.Stat(*configPath); err == nil && !*initForce {
			log.Fatalf("Config file already exists at %s (use --init-force to overwrite)", *configPath)
		} else if err != nil && !os.IsNotExist(err) {
			log.Fatalf("Unable to check config file %s: %v", *configPath, err)
		}

		parentDir := filepath.Dir(*configPath)
		if parentDir != "." && parentDir != "" {
			if err := os.MkdirAll(parentDir, 0o700); err != nil {
				log.Fatalf("Failed to create config directory %s: %v", parentDir, err)
			}
		}

		if err := os.WriteFile(*configPath, []byte(content), 0o600); err != nil {
			log.Fatalf("Failed to write config file %s: %v", *configPath, err)
		}

		log.Printf("Default config written to %s", *configPath)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("ServerSupervisor Agent starting (version: %s)", Version)
	log.Printf("Server: %s", cfg.ServerURL)
	log.Printf("Report interval: %ds", cfg.ReportInterval)
	log.Printf("Docker monitoring: %v", cfg.CollectDocker)
	log.Printf("APT monitoring: %v", cfg.CollectAPT)
	log.Printf("SMART monitoring: %v", cfg.CollectSMART)
	if cfg.CollectSMART {
		if ok, detail := collector.CheckSMARTAvailability(); ok {
			log.Printf("SMART check: OK (%s)", detail)
		} else {
			log.Printf("WARNING: SMART monitoring is enabled but unavailable: %s", detail)
		}
	}
	log.Printf("CPU temperature monitoring: %v", cfg.CollectCPUTemperature)
	log.Printf("Web logs analytics: %v (paths: %v)", cfg.CollectWebLogs, cfg.WebLogGlobs())
	if cfg.MaxReportBodyBytes <= 0 {
		cfg.MaxReportBodyBytes = 3 * 1024 * 1024
	}
	log.Printf("Max report body size: %d bytes", cfg.MaxReportBodyBytes)

	tc, err := config.LoadTasksConfig()
	if err != nil {
		log.Printf("Warning: failed to load tasks config: %v", err)
		tc = &config.TasksConfig{}
	} else {
		log.Printf("Loaded %d custom task(s) from tasks config", len(tc.Tasks))
	}

	s := sender.New(cfg)

	var skipMetrics atomic.Bool
	rep := reporter.New(cfg, tc, *verbose, &skipMetrics, Version)
	disp := dispatcher.New(cfg, *configPath, tc, startDetachedAgentUpdate)

	// ctx is cancelled on SIGINT/SIGTERM — stops the periodic report loop.
	// Command execution uses context.Background() so in-flight commands complete.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Sequential command worker — ensures command batches never overlap.
	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		for cmds := range commandQueue {
			disp.Process(s, cmds)
		}
	}()

	rep.Send(ctx, s, commandQueue)

	ticker := time.NewTicker(time.Duration(cfg.ReportInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rep.Send(ctx, s, commandQueue)
		case <-ctx.Done():
			log.Println("Agent shutting down...")
			close(commandQueue)
			workerWg.Wait()
			return
		}
	}
}
