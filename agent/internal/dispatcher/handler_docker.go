package dispatcher

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

func handleDocker(ctx context.Context, _ *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	var extra struct {
		WorkingDir string `json:"working_dir"`
	}
	_ = json.Unmarshal([]byte(cmd.Payload), &extra)

	reportRunning(ctx, s, cmd)

	isCompose := strings.HasPrefix(cmd.Action, "compose_")
	var output string
	var execErr error
	stream := func(chunk string) { streamChunk(ctx, s, cmd.ID, chunk) }
	if isCompose {
		output, execErr = collector.ExecuteComposeCommand(cmd.Action, cmd.Target, extra.WorkingDir, stream)
	} else {
		output, execErr = collector.ExecuteDockerCommand(cmd.Action, cmd.Target, stream)
	}

	status := "completed"
	if execErr != nil {
		status = "failed"
		output = decorateErrorOutput(execErr, output)
		log.Printf("Docker %s %s failed: %v", cmd.Action, cmd.Target, execErr)
	} else {
		log.Printf("Docker %s %s completed", cmd.Action, cmd.Target)
	}
	reportTerminal(ctx, s, cmd, status, output)
}
