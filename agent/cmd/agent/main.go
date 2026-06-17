package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
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
	"github.com/serversupervisor/agent/internal/logging"
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

func main() {
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

	if *showVersion {
		fmt.Println(Version)
		return
	}

	// Bootstrap logger for the pre-config code paths (--init / --internal-update).
	// Format defaults to text here; once the config is loaded it is re-initialised
	// with the operator-chosen level/format.
	bootstrapLevel := ""
	if *verbose {
		bootstrapLevel = "debug"
	}
	logging.Init(bootstrapLevel, "text")

	if *internalUpdate {
		if err := runInternalUpdate(*configPath, *updateCommandID, *updateVersion); err != nil {
			slog.Error("internal update failed", "err", err)
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
			slog.Error("config file already exists (use --init-force to overwrite)", "path", *configPath)
			os.Exit(1)
		} else if err != nil && !os.IsNotExist(err) {
			slog.Error("unable to check config file", "path", *configPath, "err", err)
			os.Exit(1)
		}

		parentDir := filepath.Dir(*configPath)
		if parentDir != "." && parentDir != "" {
			if err := os.MkdirAll(parentDir, 0o700); err != nil {
				slog.Error("failed to create config directory", "dir", parentDir, "err", err)
				os.Exit(1)
			}
		}

		if err := os.WriteFile(*configPath, []byte(content), 0o600); err != nil {
			slog.Error("failed to write config file", "path", *configPath, "err", err)
			os.Exit(1)
		}

		slog.Info("default config written", "path", *configPath)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// Re-initialise logging with the operator-chosen level/format now that config
	// is loaded. --verbose still forces debug regardless of config.
	logLevel := cfg.LogLevel
	if *verbose {
		logLevel = "debug"
	}
	logging.Init(logLevel, cfg.LogFormat)

	if cfg.MaxReportBodyBytes <= 0 {
		cfg.MaxReportBodyBytes = 3 * 1024 * 1024
	}

	smartAvailable, smartDetail := false, ""
	if cfg.CollectSMART {
		smartAvailable, smartDetail = collector.CheckSMARTAvailability()
	}
	slog.Info("agent starting",
		"version", Version,
		"server", cfg.ServerURL,
		"report_interval_s", cfg.ReportInterval,
		"collect_docker", cfg.CollectDocker,
		"collect_apt", cfg.CollectAPT,
		"collect_smart", cfg.CollectSMART,
		"collect_cpu_temperature", cfg.CollectCPUTemperature,
		"collect_web_logs", cfg.CollectWebLogs,
		"web_log_paths", cfg.WebLogGlobs(),
		"max_report_body_bytes", cfg.MaxReportBodyBytes)
	if cfg.CollectSMART {
		if smartAvailable {
			slog.Info("SMART monitoring available", "detail", smartDetail)
		} else {
			slog.Warn("SMART monitoring enabled but unavailable", "detail", smartDetail)
		}
	}

	tc, err := config.LoadTasksConfig()
	if err != nil {
		slog.Warn("failed to load tasks config", "err", err)
		tc = &config.TasksConfig{}
	} else {
		slog.Info("loaded custom tasks", "count", len(tc.Tasks))
	}

	s := sender.New(cfg)

	var skipMetrics atomic.Bool
	rep := reporter.New(cfg, tc, &skipMetrics, Version)
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
			slog.Info("agent shutting down")
			close(commandQueue)
			workerWg.Wait()
			return
		}
	}
}
