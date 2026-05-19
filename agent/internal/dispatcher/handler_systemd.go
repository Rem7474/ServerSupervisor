package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
			log.Printf("systemctl list-units failed: %v", listErr)
		} else {
			jsonBytes, jsonErr := json.Marshal(services)
			if jsonErr != nil {
				status = "failed"
				output = fmt.Sprintf("ERROR marshaling services: %v", jsonErr)
			} else {
				output = string(jsonBytes)
				log.Printf("systemctl list: %d services returned", len(services))
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
		log.Printf("systemctl %s %s failed: %v", cmd.Action, cmd.Target, err)
	} else {
		log.Printf("systemctl %s %s completed", cmd.Action, cmd.Target)
	}
	reportTerminal(ctx, s, cmd, status, output)
}
