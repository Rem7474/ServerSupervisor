package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
		log.Printf("APT %s failed: %v", cmd.Action, err)
	} else {
		log.Printf("APT %s completed", cmd.Action)
	}

	// After every apt mutation we resnapshot the package list + CVEs so the
	// server can refresh its tile immediately — without waiting for the next
	// periodic report.
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
		log.Printf("Failed to report apt command result: %v", err)
	}
}

func finaliseUUResult(err error, output string) (string, string) {
	if err == nil {
		return "completed", output
	}
	return "failed", decorateErrorOutput(err, output)
}
