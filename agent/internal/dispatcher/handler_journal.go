package dispatcher

import (
	"context"
	"log/slog"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

func handleJournal(ctx context.Context, _ *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	reportRunning(ctx, s, cmd)

	output, err := collector.ExecuteJournalctl(cmd.Target, func(chunk string) {
		streamChunk(ctx, s, cmd.ID, chunk)
	})
	status := "completed"
	if err != nil {
		status = "failed"
		output = decorateErrorOutput(err, output)
		slog.Error("journalctl failed", "target", cmd.Target, "err", err)
	} else {
		slog.Info("journalctl completed", "target", cmd.Target)
	}
	reportTerminal(ctx, s, cmd, status, output)
}
