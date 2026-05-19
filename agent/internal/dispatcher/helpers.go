package dispatcher

import (
	"context"
	"fmt"
	"log"

	"github.com/serversupervisor/agent/internal/sender"
)

// reportRunning emits the intermediate "running" status so the UI can show
// the live spinner while output streams in. Returns nothing — failures are
// non-fatal (logged only) since the agent has nothing useful to fall back to.
func reportRunning(ctx context.Context, s *sender.Sender, cmd sender.PendingCommand) {
	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    "running",
	}); err != nil {
		log.Printf("Failed to report running status for %s: %v", cmd.ID, err)
	}
}

// reportTerminal emits the final completed/failed status with output.
// Designed to be the last line of a handler; the caller may pre-decorate
// output with "ERROR: ..." prefixes.
func reportTerminal(ctx context.Context, s *sender.Sender, cmd sender.PendingCommand, status, output string) {
	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    status,
		Output:    output,
	}); err != nil {
		log.Printf("Failed to report terminal status for %s: %v", cmd.ID, err)
	}
}

// streamChunk forwards a chunk of subprocess output to the server-side
// WebSocket buffer. Errors are logged and dropped — the chunk is best-effort.
func streamChunk(ctx context.Context, s *sender.Sender, commandID, chunk string) {
	if err := s.StreamCommandChunk(ctx, commandID, chunk); err != nil {
		log.Printf("Failed to stream chunk for %s: %v", commandID, err)
	}
}

// decorateErrorOutput wraps any execution error into the conventional
// "ERROR: ..." prefix on top of whatever subprocess output was produced.
func decorateErrorOutput(err error, existingOutput string) string {
	if err == nil {
		return existingOutput
	}
	return fmt.Sprintf("ERROR: %v\n%s", err, existingOutput)
}

// logUnknownModule is kept separate so registry.go does not need a log import.
func logUnknownModule(cmd sender.PendingCommand, err error) {
	log.Printf("Failed to report 'unknown module' result for %s (%s): %v", cmd.ID, cmd.Module, err)
}
