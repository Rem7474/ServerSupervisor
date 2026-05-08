// Package dispatcher runs agent commands with concurrency controls.
// APT commands are serialized via a mutex (dpkg cannot run concurrently).
// All other modules share a 4-slot semaphore.
package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

// maxCmdDuration is an absolute guard timeout applied to every command execution.
// Prevents a permanently-stuck subprocess (e.g. blocked apt upgrade) from leaking
// the goroutine indefinitely.
const maxCmdDuration = 45 * time.Minute

// UpdaterFunc starts a detached self-update helper process. Injected from the
// main package so the dispatcher does not need the HTTP/binary-install logic.
type UpdaterFunc func(s *sender.Sender, cmd sender.PendingCommand, cfgPath string) error

// Dispatcher executes agent commands with concurrency controls.
type Dispatcher struct {
	aptMu   sync.Mutex
	cmdSem  chan struct{}
	tasks   *config.TasksConfig
	cfg     *config.Config
	cfgPath string
	updater UpdaterFunc
}

// New returns a ready Dispatcher. updater is called for module=agent action=update.
func New(cfg *config.Config, cfgPath string, tasks *config.TasksConfig, updater UpdaterFunc) *Dispatcher {
	return &Dispatcher{
		cmdSem:  make(chan struct{}, 4),
		tasks:   tasks,
		cfg:     cfg,
		cfgPath: cfgPath,
		updater: updater,
	}
}

// Process runs each command in its own goroutine and waits for all to complete.
func (d *Dispatcher) Process(s *sender.Sender, commands []sender.PendingCommand) {
	var wg sync.WaitGroup
	for _, cmd := range commands {
		wg.Add(1)
		go func(c sender.PendingCommand) {
			defer wg.Done()
			if c.Module == "apt" {
				d.aptMu.Lock()
				defer d.aptMu.Unlock()
			} else {
				d.cmdSem <- struct{}{}
				defer func() { <-d.cmdSem }()
			}
			d.execute(s, c)
		}(cmd)
	}
	wg.Wait()
}

func (d *Dispatcher) execute(s *sender.Sender, cmd sender.PendingCommand) {
	// Background parent so commands survive agent shutdown; maxCmdDuration guards
	// against stuck subprocesses that would otherwise hold the goroutine forever.
	ctx, cancel := context.WithTimeout(context.Background(), maxCmdDuration)
	defer cancel()

	log.Printf("Processing command %s: module=%s action=%s target=%s", cmd.ID, cmd.Module, cmd.Action, cmd.Target)

	switch cmd.Module {
	case "docker":
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
		stream := func(chunk string) {
			if streamErr := s.StreamCommandChunk(ctx, cmd.ID, chunk); streamErr != nil {
				log.Printf("Failed to stream apt chunk: %v", streamErr)
			}
		}

		switch cmd.Action {
		case "install_uu":
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: "running"})
			output, err := collector.InstallUnattendedUpgrades(stream)
			status := "completed"
			if err != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v\n%s", err, output)
			}
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: status, Output: output})
			return

		case "toggle_uu":
			enable := cmd.Target == "enable"
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: "running"})
			output, err := collector.ToggleUnattendedUpgrades(enable)
			status := "completed"
			if err != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v\n%s", err, output)
			}
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: status, Output: output})
			return

		case "configure_uu":
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: "running"})
			var cfg collector.UUConfig
			if jsonErr := json.Unmarshal([]byte(cmd.Payload), &cfg); jsonErr != nil {
				_ = s.ReportCommandResult(ctx, &sender.CommandResult{
					CommandID: cmd.ID, Status: "failed",
					Output: fmt.Sprintf("invalid payload: %v", jsonErr),
				})
				return
			}
			err := collector.ConfigureUnattendedUpgrades(cfg)
			status := "completed"
			output := "Configuration applied."
			if err != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v", err)
			}
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: status, Output: output})
			return

		case "run_uu":
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: "running"})
			output, err := collector.RunUnattendedUpgrades(stream)
			status := "completed"
			if err != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR: %v\n%s", err, output)
			}
			_ = s.ReportCommandResult(ctx, &sender.CommandResult{CommandID: cmd.ID, Status: status, Output: output})
			return
		}

		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
		}); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		output, err := collector.ExecuteAptCommandWithStreaming(cmd.Action, stream)
		status := "completed"
		if err != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v\n%s", err, output)
			log.Printf("APT %s failed: %v", cmd.Action, err)
		} else {
			log.Printf("APT %s completed", cmd.Action)
		}

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

	case "agent":
		if cmd.Action != "update" {
			if err := s.ReportCommandResult(ctx, &sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "failed",
				Output:    fmt.Sprintf("unsupported agent action: %s", cmd.Action),
			}); err != nil {
				log.Printf("Failed to report unsupported agent action: %v", err)
			}
			return
		}

		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
			Output:    "Launching detached update helper...",
		}); err != nil {
			log.Printf("Failed to report agent update running status: %v", err)
		}

		if err := d.updater(s, cmd, d.cfgPath); err != nil {
			output := fmt.Sprintf("ERROR: %v", err)
			if reportErr := s.ReportCommandResult(ctx, &sender.CommandResult{
				CommandID: cmd.ID,
				Status:    "failed",
				Output:    output,
			}); reportErr != nil {
				log.Printf("Failed to report agent update launch failure: %v", reportErr)
			}
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
		d.executeCustomTask(ctx, s, cmd)

	case "crowdsec":
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    "running",
		}); err != nil {
			log.Printf("Failed to report running status: %v", err)
		}

		var output string
		var execErr error

		switch cmd.Action {
		case "unban":
			if cmd.Target == "" {
				execErr = fmt.Errorf("no IP provided for unban action")
			} else {
				execErr = collector.DeleteCrowdSecDecision(
					d.cfg.CrowdSecConnectionString,
					d.cfg.CrowdSecAlertsMachineID,
					d.cfg.CrowdSecAlertsPassword,
					cmd.Target,
				)
			}
		case "ban":
			if cmd.Target == "" {
				execErr = fmt.Errorf("no IP provided for ban action")
			} else {
				var banPayload struct {
					Duration string `json:"duration"`
				}
				banPayload.Duration = "4h"
				_ = json.Unmarshal([]byte(cmd.Payload), &banPayload)
				if banPayload.Duration == "" {
					banPayload.Duration = "4h"
				}
				execErr = collector.CreateCrowdSecDecision(
					d.cfg.CrowdSecConnectionString,
					d.cfg.CrowdSecAlertsMachineID,
					d.cfg.CrowdSecAlertsPassword,
					cmd.Target,
					banPayload.Duration,
				)
			}
		default:
			execErr = fmt.Errorf("unknown crowdsec action: %s", cmd.Action)
		}

		status := "completed"
		if execErr != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v", execErr)
			log.Printf("crowdsec %s %s failed: %v", cmd.Action, cmd.Target, execErr)
		} else {
			switch cmd.Action {
			case "ban":
				output = fmt.Sprintf("Successfully banned IP: %s", cmd.Target)
			default:
				output = fmt.Sprintf("Successfully unbanned IP: %s", cmd.Target)
			}
			log.Printf("crowdsec %s %s completed", cmd.Action, cmd.Target)
		}
		if err := s.ReportCommandResult(ctx, &sender.CommandResult{
			CommandID: cmd.ID,
			Status:    status,
			Output:    output,
		}); err != nil {
			log.Printf("Failed to report crowdsec command result: %v", err)
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

// executeCustomTask runs a task defined in the local tasks.yaml configuration.
// cmd.Target holds the task ID; the command argv is exec'd directly (no shell).
func (d *Dispatcher) executeCustomTask(ctx context.Context, s *sender.Sender, cmd sender.PendingCommand) {
	task := d.tasks.FindTask(cmd.Target)
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

	c := exec.CommandContext(taskCtx, task.Command[0], task.Command[1:]...)

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
