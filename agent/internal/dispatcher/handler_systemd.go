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
