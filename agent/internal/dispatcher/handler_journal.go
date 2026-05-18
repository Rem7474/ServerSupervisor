package dispatcher

import (
	"context"
	"log"

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
		log.Printf("journalctl %s failed: %v", cmd.Target, err)
	} else {
		log.Printf("journalctl %s completed", cmd.Target)
	}
	reportTerminal(ctx, s, cmd, status, output)
}
