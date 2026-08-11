package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

func handleSystemd(ctx context.Context, _ *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	reportRunning(ctx, s, cmd)

	if cmd.Action == "list" {
		services, listErr := collector.ListSystemdServices()
		status := "completed"
		var output string
		if listErr != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR: %v", listErr)
			slog.Error("systemctl list-units failed", "err", listErr)
		} else {
			jsonBytes, jsonErr := json.Marshal(services)
			if jsonErr != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR marshaling services: %v", jsonErr)
			} else {
				output = string(jsonBytes)
				slog.Debug("systemd services listed", "count", len(services))
			}
		}
		reportTerminal(ctx, s, cmd, status, output)
		return
	}

	if (cmd.Action == "restart" || cmd.Action == "stop") && collector.IsSelfServiceUnit(cmd.Target) {
		handleSelfServiceAction(ctx, s, cmd)
		return
	}

	output, err := collector.ExecuteSystemdCommand(cmd.Target, cmd.Action, func(chunk string) {
		streamChunk(ctx, s, cmd.ID, chunk)
	})
	status := "completed"
	if err != nil {
		status = "failed"
		output = decorateErrorOutput(err, output)
		slog.Error("systemctl command failed", "action", cmd.Action, "target", cmd.Target, "err", err)
	} else {
		slog.Info("systemctl command completed", "action", cmd.Action, "target", cmd.Target)
	}
	reportTerminal(ctx, s, cmd, status, output)
}

// handleSelfServiceAction handles a restart/stop targeting the agent's own
// systemd unit — see collector.ExecuteSelfRestart for why this can't go
// through the normal ExecuteSystemdCommand path.
func handleSelfServiceAction(ctx context.Context, s *sender.Sender, cmd sender.PendingCommand) {
	if err := collector.ExecuteSelfRestart(cmd.Action); err != nil {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("ERROR: %v", err))
		slog.Error("self systemd action failed to launch", "action", cmd.Action, "target", cmd.Target, "err", err)
		return
	}
	slog.Info("self systemd action launched via detached unit", "action", cmd.Action, "target", cmd.Target)
	reportTerminal(ctx, s, cmd, "completed",
		fmt.Sprintf("Detached systemctl %s launched for %s. The agent is about to restart and cannot confirm the final outcome itself — check its status again in a few seconds.", cmd.Action, collector.SelfServiceUnit))
}
