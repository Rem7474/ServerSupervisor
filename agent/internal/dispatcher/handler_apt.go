package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

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

	// After every apt mutation we resnapshot the package list + CVEs so the
	// server can refresh its tile immediately — without waiting for the next
	// periodic report.
	var aptStatus interface{}
	slog.Debug("collecting apt status with CVE extraction", "action", cmd.Action)
	apt, aptErr := collector.CollectAPT(true)
	if aptErr != nil {
		slog.Warn("failed to collect apt status", "action", cmd.Action, "err", aptErr)
	} else {
		aptStatus = apt
		slog.Debug("apt status collected", "packages", apt.PendingPackages, "security", apt.SecurityUpdates)
	}

	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    status,
		Output:    output,
		AptStatus: aptStatus,
	}); err != nil {
		slog.Warn("failed to report apt command result", "err", err)
	}
}

func finaliseUUResult(err error, output string) (string, string) {
	if err == nil {
		return "completed", output
	}
	return "failed", decorateErrorOutput(err, output)
}
