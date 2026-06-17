package dispatcher

import (
	"context"
	"encoding/json"
	"log/slog"
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
		slog.Error("docker command failed", "action", cmd.Action, "target", cmd.Target, "err", execErr)
	} else {
		slog.Info("docker command completed", "action", cmd.Action, "target", cmd.Target)
	}
	reportTerminal(ctx, s, cmd, status, output)
}
