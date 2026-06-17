package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/agent/internal/config"
	"github.com/serversupervisor/agent/internal/sender"
)

// handleCustom runs a task defined in the local tasks.yaml configuration.
// cmd.Target holds the task ID; the command argv is exec'd directly (no shell)
// so user-controlled arguments cannot expand into shell metacharacters.
func handleCustom(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	task := d.tasks.FindTask(cmd.Target)
	if task == nil {
		slog.Warn("custom task not found in local tasks config", "task", cmd.Target)
		reportTerminal(ctx, s, cmd, "failed",
			fmt.Sprintf("task %q not found in local tasks config (tasks.yaml)", cmd.Target))
		return
	}

	reportRunning(ctx, s, cmd)

	var payloadData struct {
		Env map[string]string `json:"env"`
	}
	_ = json.Unmarshal([]byte(cmd.Payload), &payloadData)

	stream := func(chunk string) { streamChunk(ctx, s, cmd.ID, chunk) }
	output, err := executeTask(ctx, task, payloadData.Env, stream)
	status := "completed"
	if err != nil {
		status = "failed"
		output += "\nERROR: " + err.Error()
		slog.Error("custom task failed", "task", task.ID, "err", err)
	} else {
		slog.Info("custom task completed", "task", task.ID)
	}
	reportTerminal(ctx, s, cmd, status, output)
}

// executeTask runs a tasks.yaml-declared task, streaming combined output via
// streamCB, and returns the captured output plus any execution error. It does
// NOT report command status — the caller owns reporting. Used by handleCustom
// and as pre/post hooks by the compose module. The argv is exec'd directly (no
// shell), so env values cannot expand into shell metacharacters.
func executeTask(ctx context.Context, task *config.CustomTask, env map[string]string, streamCB func(string)) (string, error) {
	timeout := time.Duration(task.Timeout) * time.Second
	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(taskCtx, task.Command[0], task.Command[1:]...)
	if len(env) > 0 {
		extraEnv := make([]string, 0, len(env))
		for k, v := range env {
			extraEnv = append(extraEnv, k+"="+v)
		}
		c.Env = append(os.Environ(), extraEnv...)
	}

	stdout, err := c.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := c.Start(); err != nil {
		return "", fmt.Errorf("failed to start: %w", err)
	}

	var builder strings.Builder
	streamPipe := func(r interface{ Read([]byte) (int, error) }) {
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				builder.WriteString(chunk)
				if streamCB != nil {
					streamCB(chunk)
				}
			}
			if readErr != nil {
				break
			}
		}
	}

	var pipeWg sync.WaitGroup
	pipeWg.Add(2)
	go func() { defer pipeWg.Done(); streamPipe(stdout) }()
	go func() { defer pipeWg.Done(); streamPipe(stderr) }()
	pipeWg.Wait()

	waitErr := c.Wait()
	if waitErr != nil && taskCtx.Err() == context.DeadlineExceeded {
		return builder.String(), fmt.Errorf("task timed out after %ds", task.Timeout)
	}
	return builder.String(), waitErr
}
