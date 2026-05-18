package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
		log.Printf("ps failed: %v", procErr)
	} else {
		jsonBytes, jsonErr := json.Marshal(procs)
		if jsonErr != nil {
			status = "failed"
			output = fmt.Sprintf("ERROR marshaling processes: %v", jsonErr)
		} else {
			output = string(jsonBytes)
			log.Printf("processes list: %d processes returned", len(procs))
		}
	}
	reportTerminal(ctx, s, cmd, status, output)
}
