package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

// aptStatusRefreshTimeout bounds the post-command CollectAPT(true) call below —
// on a fresh host with a large backlog, its per-package security/CVE lookups
// (agent/internal/collector/apt.go) are sequential subprocess + network calls
// that could otherwise run for tens of minutes.
const aptStatusRefreshTimeout = 5 * time.Minute

func handleApt(ctx context.Context, _ *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	stream := func(chunk string) { streamChunk(ctx, s, cmd.ID, chunk) }

	switch cmd.Action {
	case "install_uu":
		reportRunning(ctx, s, cmd)
		output, err := collector.InstallUnattendedUpgrades(stream)
		status, output := finaliseUUResult(err, output)
		reportTerminal(ctx, s, cmd, status, output)
		return

	case "toggle_uu":
		reportRunning(ctx, s, cmd)
		enable := cmd.Target == "enable"
		output, err := collector.ToggleUnattendedUpgrades(enable)
		status, output := finaliseUUResult(err, output)
		reportTerminal(ctx, s, cmd, status, output)
		return

	case "configure_uu":
		reportRunning(ctx, s, cmd)
		var cfg collector.UUConfig
		if jsonErr := json.Unmarshal([]byte(cmd.Payload), &cfg); jsonErr != nil {
			reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("invalid payload: %v", jsonErr))
			return
		}
		err := collector.ConfigureUnattendedUpgrades(cfg)
		status := "completed"
		output := "Configuration applied."
		if err != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v", err)
		}
		reportTerminal(ctx, s, cmd, status, output)
		return

	case "run_uu":
		reportRunning(ctx, s, cmd)
		output, err := collector.RunUnattendedUpgrades(stream)
		status, output := finaliseUUResult(err, output)
		reportTerminal(ctx, s, cmd, status, output)
		return
	}

	// Default: standard apt action (update/upgrade/full-upgrade/autoremove…)
	reportRunning(ctx, s, cmd)

	output, err := collector.ExecuteAptCommandWithStreaming(cmd.Action, stream)
	status := "completed"
	if err != nil {
		status = "failed"
		output = decorateErrorOutput(err, output)
		slog.Error("apt command failed", "action", cmd.Action, "err", err)
	} else {
		slog.Info("apt command completed", "action", cmd.Action)
	}

	// Bundle a fast, CVE-free package count into the terminal report itself —
	// collector.CollectAPTFast is a single bounded apt-get upgrade --simulate
	// call (no per-package apt-cache/CVE lookups), so it stays quick even on a
	// large backlog. The server applies CommandResult.AptStatus synchronously
	// (server/internal/services/agent/service.go), so this is what makes the
	// UI's pending-package count update the instant the command completes,
	// instead of only ever being set by the slower goroutine below.
	fastStatus, fastErr := collector.CollectAPTFast(ctx)
	if fastErr != nil {
		slog.Warn("fast apt status collection failed", "action", cmd.Action, "err", fastErr)
	}

	// Report the terminal status (with the fast package count above), before
	// the (slow, network-bound) CVE enrichment below. The CVE-enriched
	// snapshot is pushed separately via /api/agent/apt-status, so the
	// command's completion is never delayed by — nor falsely timed-out
	// because of — the Ubuntu CVE API round-trips on a fresh host.
	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    status,
		Output:    output,
		AptStatus: fastStatus,
	}); err != nil {
		slog.Warn("failed to report apt command result", "err", err)
	}

	// After every apt mutation we resnapshot the package list + CVEs so the server can
	// refresh its tile immediately — without waiting for the next periodic report.
	// This runs detached in its own goroutine, *after* handleApt has already
	// returned the command as completed: CollectAPT(true)'s security/CVE lookups
	// (apt-cache/apt-get changelog/Ubuntu CVE API, one round-trip per pending
	// package) are read-only w.r.t. dpkg and don't need aptMu at all, but until
	// this was detached they ran synchronously *inside* the aptMu-locked call —
	// so a second apt command would sit stuck "pending" for however long CVE
	// enrichment took (minutes, on a fresh host with a large backlog), even
	// though the first command had already finished. ctx (execute()'s bounded
	// per-command context) is cancelled the instant handleApt returns, so this
	// goroutine gets its own independent, bounded context instead.
	go func() {
		slog.Debug("collecting apt status with CVE extraction", "action", cmd.Action)
		collectCtx, cancel := context.WithTimeout(context.Background(), aptStatusRefreshTimeout)
		apt, aptErr := collector.CollectAPT(collectCtx, true)
		cancel()
		if aptErr != nil {
			slog.Warn("failed to collect apt status", "action", cmd.Action, "err", aptErr)
			return
		}
		slog.Debug("apt status collected", "packages", apt.PendingPackages, "security", apt.SecurityUpdates)
		sendCtx, sendCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer sendCancel()
		if err := s.SendAptStatus(sendCtx, apt); err != nil {
			slog.Warn("failed to push apt status", "action", cmd.Action, "err", err)
		}
	}()
}

func finaliseUUResult(err error, output string) (string, string) {
	if err == nil {
		return "completed", output
	}
	return "failed", decorateErrorOutput(err, output)
}
