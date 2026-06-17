package dispatcher

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/serversupervisor/agent/internal/sender"
)

func handleAgent(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	if cmd.Action != "update" {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("unsupported agent action: %s", cmd.Action))
		return
	}

	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    "running",
		Output:    "Launching detached update helper...",
	}); err != nil {
		slog.Warn("failed to report agent update running status", "err", err)
	}

	if err := d.updater(s, cmd, d.cfgPath); err != nil {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("ERROR: %v", err))
	}
}
