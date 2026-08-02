package dispatcher

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"context"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

// handleRestic supports two actions:
//   - "status": an immediate CollectResticStatus snapshot, reported as the
//     terminal result.
//   - "run_backup": runs the configured run_backup.sh script for cmd.Target
//     (the resticprofile profile name, empty = script default), streaming
//     progress live and reporting a structured ResticBackupSummary as the
//     terminal Output.
func handleRestic(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	switch cmd.Action {
	case "status":
		reportRunning(ctx, s, cmd)
		status, err := collector.CollectResticStatus(ctx, d.cfg)
		if err != nil {
			reportTerminal(ctx, s, cmd, "failed", decorateErrorOutput(err, ""))
			return
		}
		output, marshalErr := json.Marshal(status)
		if marshalErr != nil {
			reportTerminal(ctx, s, cmd, "failed", decorateErrorOutput(marshalErr, ""))
			return
		}
		reportTerminal(ctx, s, cmd, "completed", string(output))
		return

	case "run_backup":
		reportRunning(ctx, s, cmd)
		stream := func(chunk string) { streamChunk(ctx, s, cmd.ID, chunk) }

		summary, err := collector.RunResticBackupWithProgress(ctx, d.cfg, cmd.Target, stream)
		if summary == nil {
			// Failed before a summary could be built at all (missing config/script).
			reportTerminal(ctx, s, cmd, "failed", decorateErrorOutput(err, ""))
			return
		}

		output, marshalErr := json.Marshal(summary)
		if marshalErr != nil {
			output = []byte(fmt.Sprintf(`{"status":"error","error_message":%q}`, marshalErr.Error()))
		}

		status := "completed"
		if err != nil {
			status = "failed"
			slog.Error("restic run_backup failed", "profile", cmd.Target, "err", err)
		} else {
			slog.Info("restic run_backup completed", "profile", cmd.Target)
		}
		reportTerminal(ctx, s, cmd, status, string(output))

		// Re-snapshot immediately so the server has fresh status without waiting
		// for the next periodic report.
		if freshStatus, statusErr := collector.CollectResticStatus(ctx, d.cfg); statusErr == nil {
			if pushErr := s.SendResticStatus(ctx, freshStatus); pushErr != nil {
				slog.Warn("failed to push restic status", "err", pushErr)
			}
		}
		return

	default:
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("unknown restic action: %s", cmd.Action))
	}
}
