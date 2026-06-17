// Package reporter builds and sends the periodic host report to the server.
package reporter

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

// Reporter builds and sends periodic host reports.
type Reporter struct {
	cfg         *config.Config
	tasks       *config.TasksConfig
	skipMetrics *atomic.Bool
	version     string
}

// New returns a ready Reporter. skipMetrics is shared with the caller — the
// reporter updates it after each successful send based on the server directive.
func New(cfg *config.Config, tasks *config.TasksConfig, skipMetrics *atomic.Bool, version string) *Reporter {
	return &Reporter{
		cfg:         cfg,
		tasks:       tasks,
		skipMetrics: skipMetrics,
		version:     version,
	}
}

// Send collects host metrics, builds the report, sends it, then enqueues any
// commands returned by the server. If the command queue is full the commands
// are immediately reported as failed so the server does not wait out the
// stalled-command cleanup timeout.
func (r *Reporter) Send(ctx context.Context, s *sender.Sender, cmdQueue chan<- []sender.PendingCommand) {
	var (
		collectedMetrics *collector.SystemMetrics
		dockerData       *sender.DockerPayload
		dockerNetworks   []collector.DockerNetwork
		composeProjects  []collector.ComposeProject
		diskMetrics      []collector.DiskMetrics
		diskHealth       []collector.DiskHealth
		uuData           *collector.UnattendedUpgradesStatus
		webLogs          *collector.WebLogReport
	)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if r.skipMetrics.Load() {
			slog.Debug("system metrics collection skipped (Proxmox is the designated metrics source)")
			m, err := collector.CollectMinimalMetrics()
			if err != nil {
				slog.Error("failed to collect minimal metrics", "err", err)
				return
			}
			collectedMetrics = m
		} else {
			m, err := collector.CollectSystem(r.cfg.CollectCPUTemperature)
			if err != nil {
				slog.Error("failed to collect system metrics", "err", err)
				return
			}
			collectedMetrics = m
		}
	}()

	if r.cfg.CollectDocker {
		wg.Add(1)
		go func() {
			defer wg.Done()
			containers, err := collector.CollectDocker()
			if err != nil {
				slog.Warn("docker collection skipped", "err", err)
				return
			}
			dockerData = &sender.DockerPayload{Containers: containers}

			if networks, err := collector.CollectDockerNetworks(); err == nil {
				dockerNetworks = networks
			}

			if projects, err := collector.CollectComposeProjects(); err == nil {
				composeProjects = projects
			} else {
				slog.Warn("compose projects collection skipped", "err", err)
			}
		}()
	} else {
		dockerData = &sender.DockerPayload{Containers: []collector.DockerContainer{}}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		uuData = collector.CollectUnattendedUpgrades()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		diskMetrics, err = collector.CollectDiskMetrics()
		if err != nil {
			slog.Error("failed to collect disk metrics", "err", err)
		}
	}()

	if r.cfg.CollectSMART {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			diskHealth, err = collector.CollectDiskHealth()
			if err != nil {
				slog.Warn("failed to collect disk health (smartctl may not be installed)", "err", err)
			}
		}()
	}

	if r.cfg.CollectWebLogs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			globs := r.cfg.WebLogGlobs()
			slog.Debug("web logs scan starting", "globs", globs, "crowdsec", r.cfg.CollectCrowdSecCorrelation)
			report, err := collector.CollectWebLogs(
				globs,
				r.cfg.WebLogsTailLines,
				r.cfg.WebLogsTopN,
				r.cfg.WebLogsRequestsLimit,
				r.cfg.WebLogsCursorFile,
				r.cfg.CrowdSecConnectionString,
				r.cfg.CrowdSecAPIKey,
				r.cfg.CrowdSecAlertsMachineID,
				r.cfg.CrowdSecAlertsPassword,
				r.cfg.CollectCrowdSecCorrelation,
			)
			if err != nil {
				slog.Warn("web logs collection skipped", "err", err)
				return
			}
			suspicious := 0
			if report.Threats != nil {
				suspicious = report.Threats.SuspiciousRequests
			}
			slog.Debug("web logs scan complete",
				"source", report.Source, "files", len(report.LogFilesScanned),
				"requests", report.TotalRequests, "suspicious", suspicious)
			webLogs = report
		}()
	}

	wg.Wait()

	if collectedMetrics == nil {
		return
	}

	var customTasksList []config.TaskSummary
	if r.tasks != nil && len(r.tasks.Tasks) > 0 {
		customTasksList = r.tasks.Summaries()
	}

	capabilities := &sender.Capabilities{
		Docker:  r.cfg.CollectDocker,
		APT:     r.cfg.CollectAPT,
		SMART:   r.cfg.CollectSMART,
		CPUTemp: r.cfg.CollectCPUTemperature,
		WebLogs: r.cfg.CollectWebLogs,
		Systemd: true,
		Journal: true,
	}

	report := &sender.Report{
		AgentVersion:       r.version,
		Capabilities:       capabilities,
		Metrics:            collectedMetrics,
		Docker:             dockerData,
		UnattendedUpgrades: uuData,
		WebLogs:            webLogs,
		DockerNetworks:     dockerNetworks,
		ComposeProjects:    composeProjects,
		DiskMetrics:        diskMetrics,
		DiskHealth:         diskHealth,
		CustomTasks:        customTasksList,
		TasksConfigYAML:    config.LoadTasksConfigRaw(),
		Timestamp:          time.Now(),
	}
	trimWebLogsForReportSize(report, r.cfg.MaxReportBodyBytes)

	response, err := s.SendReportWithRetry(ctx, report)
	if err != nil {
		slog.Error("failed to send report", "err", err)
		return
	}

	if r.skipMetrics.Load() {
		slog.Info("report sent", "source", "proxmox", "uptime_s", collectedMetrics.Uptime)
	} else {
		slog.Info("report sent",
			"cpu_pct", collectedMetrics.CPUUsagePercent,
			"ram_pct", collectedMetrics.MemoryPercent,
			"disks", len(diskMetrics))
	}

	r.skipMetrics.Store(response.SkipMetrics)

	if len(response.Commands) == 0 {
		return
	}
	select {
	case cmdQueue <- response.Commands:
	default:
		slog.Warn("command queue full, reporting commands as failed",
			"pending_batches", len(cmdQueue), "dropped_commands", len(response.Commands))
		for _, cmd := range response.Commands {
			if err := s.ReportCommandResult(ctx, &sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "failed",
				Output:    "command dropped: agent command queue was full — try again",
			}); err != nil {
				slog.Error("failed to report dropped command as failed", "command_id", cmd.ID, "err", err)
			}
		}
	}
}

// trimWebLogsForReportSize shrinks web.Requests until the marshaled report fits
// within maxBodyBytes. Uses a proportional estimate (2 marshals in the common
// case) instead of the previous O(log N) full-report marshal loop.
func trimWebLogsForReportSize(report *sender.Report, maxBodyBytes int) {
	if report == nil || maxBodyBytes <= 0 {
		return
	}
	web := report.WebLogs
	if web == nil || len(web.Requests) == 0 {
		return
	}

	fullEncoded, err := json.Marshal(report)
	if err != nil || len(fullEncoded) <= maxBodyBytes {
		return
	}

	original := len(web.Requests)

	// Measure just the requests slice to estimate the budget available for it.
	reqEncoded, err := json.Marshal(web.Requests)
	if err != nil {
		return
	}
	overhead := len(fullEncoded) - len(reqEncoded)
	budget := maxBodyBytes - overhead

	if budget <= 0 {
		web.Requests = nil
	} else {
		ratio := float64(budget) / float64(len(reqEncoded))
		target := int(float64(original) * ratio * 0.9) // 10 % safety margin
		if target < 0 {
			target = 0
		}
		web.Requests = web.Requests[:target]
	}

	// One verification pass — halve further if the proportional estimate was off.
	for len(web.Requests) > 0 {
		encoded, encErr := json.Marshal(report)
		if encErr != nil || len(encoded) <= maxBodyBytes {
			break
		}
		web.Requests = web.Requests[:len(web.Requests)/2]
	}

	slog.Debug("web logs payload trimmed to fit report size",
		"requests_from", original, "requests_to", len(web.Requests), "max_report_body_bytes", maxBodyBytes)
}
