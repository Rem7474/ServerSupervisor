package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

func handleProcesses(ctx context.Context, _ *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	reportRunning(ctx, s, cmd)

	procs, procErr := collector.GetProcessList()
	status := "completed"
	var output string
	if procErr != nil {
		status = "failed"
		output = fmt.Sprintf("ERROR: %v", procErr)
		slog.Error("ps failed", "err", procErr)
	} else {
		jsonBytes, jsonErr := json.Marshal(procs)
		if jsonErr != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR marshaling processes: %v", jsonErr)
		} else {
			output = string(jsonBytes)
			slog.Debug("processes listed", "count", len(procs))
		}
	}
	reportTerminal(ctx, s, cmd, status, output)
}
