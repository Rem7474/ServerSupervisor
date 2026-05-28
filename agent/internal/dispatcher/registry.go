package dispatcher

import (
	"context"
	"fmt"

	"github.com/serversupervisor/agent/internal/sender"
)

// ModuleHandler executes one PendingCommand and is responsible for reporting
// both the intermediate "running" status and the terminal result via the
// supplied *sender.Sender. Each module (docker, apt, journal…) lives in its
// own file and registers itself in moduleRegistry below.
type ModuleHandler func(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand)

// moduleRegistry maps PendingCommand.Module to the function that handles it.
// Adding a new module is a single entry here plus the corresponding handler
// file — the dispatcher never needs to change.
var moduleRegistry = map[string]ModuleHandler{
	"docker":    handleDocker,
	"journal":   handleJournal,
	"apt":       handleApt,
	"agent":     handleAgent,
	"systemd":   handleSystemd,
	"processes": handleProcesses,
	"custom":    handleCustom,
	"crowdsec":  handleCrowdSec,
	"compose":   handleCompose,
}

// dispatch picks the handler for cmd.Module and reports a clear error for
// unknown modules so the server flags the command as failed quickly.
func dispatch(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	handler, ok := moduleRegistry[cmd.Module]
	if !ok {
		reportUnknownModule(ctx, s, cmd)
		return
	}
	handler(ctx, d, s, cmd)
}

func reportUnknownModule(ctx context.Context, s *sender.Sender, cmd sender.PendingCommand) {
	if err := s.ReportCommandResult(ctx, &sender.CommandResult{
		CommandID: cmd.ID,
		Status:    "failed",
		Output:    fmt.Sprintf("unknown command module: %s", cmd.Module),
	}); err != nil {
		// Result reporting itself failed — log but do not retry; the server
		// will reap the command as stalled.
		logUnknownModule(cmd, err)
	}
}
