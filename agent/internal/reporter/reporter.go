// Package reporter builds and sends the periodic host report to the server.
package reporter

import (
	"context"
	"encoding/json"
	"log"
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
	verbose     bool
	skipMetrics *atomic.Bool
	version     string
}

// New returns a ready Reporter. skipMetrics is shared with the caller — the
// reporter updates it after each successful send based on the server directive.
func New(cfg *config.Config, tasks *config.TasksConfig, verbose bool, skipMetrics *atomic.Bool, version string) *Reporter {
	return &Reporter{
		cfg:         cfg,
		tasks:       tasks,
		verbose:     verbose,
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
		metricsPayload   interface{}
		collectedMetrics *collector.SystemMetrics
		dockerData       interface{}
		dockerNetworks   interface{}
		composeProjects  interface{}
		diskMetrics      []collector.DiskMetrics
		diskHealth       []collector.DiskHealth
		uuData           *collector.UnattendedUpgradesStatus
		webLogs          interface{}
	)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if r.skipMetrics.Load() {
			log.Printf("System metrics collection skipped (Proxmox is the designated metrics source)")
			m, err := collector.CollectMinimalMetrics()
			if err != nil {
				log.Printf("Failed to collect minimal metrics: %v", err)
				return
			}
			collectedMetrics = m
			metricsPayload = m
		} else {
			m, err := collector.CollectSystem(r.cfg.CollectCPUTemperature)
			if err != nil {
				log.Printf("Failed to collect system metrics: %v", err)
				return
			}
			collectedMetrics = m
			metricsPayload = m
		}
	}()

	if r.cfg.CollectDocker {
		wg.Add(1)
		go func() {
			defer wg.Done()
			containers, err := collector.CollectDocker()
			if err != nil {
				log.Printf("Docker collection skipped: %v", err)
				return
			}
			dockerData = struct {
				Containers []collector.DockerContainer `json:"containers"`
			}{Containers: containers}

			if networks, err := collector.CollectDockerNetworks(); err == nil {
				dockerNetworks = networks
			}

			if projects, err := collector.CollectComposeProjects(); err == nil {
				composeProjects = projects
			} else {
				log.Printf("Compose projects collection skipped: %v", err)
			}
		}()
	} else {
		dockerData = struct {
			Containers []interface{} `json:"containers"`
		}{Containers: []interface{}{}}
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
			log.Printf("Failed to collect disk metrics: %v", err)
		}
	}()

	if r.cfg.CollectSMART {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error
			diskHealth, err = collector.CollectDiskHealth()
			if err != nil {
				log.Printf("Failed to collect disk health (smartctl may not be installed): %v", err)
			}
		}()
	}

	if r.cfg.CollectWebLogs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			globs := r.cfg.WebLogGlobs()
			log.Printf("Web logs: scanning globs %v (crowdsec=%v)", globs, r.cfg.CollectCrowdSecCorrelation)
			report, err := collector.CollectWebLogs(
				globs,
				r.cfg.WebLogsTailLines,
				r.cfg.WebLogsTopN,
				r.cfg.WebLogsRequestsLimit,
				r.cfg.WebLogsCursorFile,
				r.verbose,
				r.cfg.CrowdSecConnectionString,
				r.cfg.CrowdSecAPIKey,
				r.cfg.CrowdSecAlertsMachineID,
				r.cfg.CrowdSecAlertsPassword,
				r.cfg.CollectCrowdSecCorrelation,
			)
			if err != nil {
				log.Printf("Web logs collection skipped: %v", err)
				return
			}
			suspicious := 0
			if report.Threats != nil {
				suspicious = report.Threats.SuspiciousRequests
			}
			log.Printf("Web logs: source=%s files=%d requests=%d suspicious=%d",
				report.Source, len(report.LogFilesScanned), report.TotalRequests, suspicious)
			webLogs = report
		}()
	}

	wg.Wait()

	if collectedMetrics == nil {
		return
	}

	var customTasksList interface{}
	if r.tasks != nil && len(r.tasks.Tasks) > 0 {
		customTasksList = r.tasks.Summaries()
	}

	capabilities := map[string]bool{
		"docker":   r.cfg.CollectDocker,
		"apt":      r.cfg.CollectAPT,
		"smart":    r.cfg.CollectSMART,
		"cpu_temp": r.cfg.CollectCPUTemperature,
		"web_logs": r.cfg.CollectWebLogs,
		"systemd":  true,
		"journal":  true,
	}

	report := &sender.Report{
		AgentVersion:       r.version,
		Capabilities:       capabilities,
		Metrics:            metricsPayload,
		Docker:             dockerData,
		AptStatus:          nil,
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
		log.Printf("Failed to send report: %v", err)
		return
	}

	if r.skipMetrics.Load() {
		log.Printf("Report sent successfully (uptime: %ds — Proxmox source)", collectedMetrics.Uptime)
	} else {
		log.Printf("Report sent successfully (CPU: %.1f%%, RAM: %.1f%%, Disks: %d)",
			collectedMetrics.CPUUsagePercent, collectedMetrics.MemoryPercent, len(collectedMetrics.Disks))
	}

	r.skipMetrics.Store(response.SkipMetrics)

	if len(response.Commands) == 0 {
		return
	}
	select {
	case cmdQueue <- response.Commands:
	default:
		log.Printf("Command queue full (%d batches pending), reporting %d commands as failed",
			len(cmdQueue), len(response.Commands))
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

// trimWebLogsForReportSize shrinks web.Requests until the marshaled report fits
// within maxBodyBytes. Uses a proportional estimate (2 marshals in the common
// case) instead of the previous O(log N) full-report marshal loop.
func trimWebLogsForReportSize(report *sender.Report, maxBodyBytes int) {
	if report == nil || maxBodyBytes <= 0 {
		return
	}
	web, ok := report.WebLogs.(*collector.WebLogReport)
	if !ok || web == nil || len(web.Requests) == 0 {
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
		target := int(float64(original)*ratio*0.9) // 10 % safety margin
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

	log.Printf("Web logs payload trimmed: requests %d -> %d to fit max_report_body_bytes=%d",
		original, len(web.Requests), maxBodyBytes)
}
