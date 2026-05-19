package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/serversupervisor/agent/internal/sender"
)

// handleCustom runs a task defined in the local tasks.yaml configuration.
// cmd.Target holds the task ID; the command argv is exec'd directly (no shell)
// so user-controlled arguments cannot expand into shell metacharacters.
func handleCustom(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	task := d.tasks.FindTask(cmd.Target)
	if task == nil {
		log.Printf("Custom task %q not found in local tasks config", cmd.Target)
		reportTerminal(ctx, s, cmd, "failed",
			fmt.Sprintf("task %q not found in local tasks config (tasks.yaml)", cmd.Target))
		return
	}

	reportRunning(ctx, s, cmd)

	timeout := time.Duration(task.Timeout) * time.Second
	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(taskCtx, task.Command[0], task.Command[1:]...)

	var payloadData struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal([]byte(cmd.Payload), &payloadData); err == nil && len(payloadData.Env) > 0 {
		extraEnv := make([]string, 0, len(payloadData.Env))
		for k, v := range payloadData.Env {
			extraEnv = append(extraEnv, k+"="+v)
		}
		c.Env = append(os.Environ(), extraEnv...)
	}

	stdout, err := c.StdoutPipe()
	if err != nil {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("ERROR: %v", err))
		return
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("ERROR: %v", err))
		return
	}

	if err := c.Start(); err != nil {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("ERROR: failed to start: %v", err))
		return
	}

	var builder strings.Builder
	streamPipe := func(r interface{ Read([]byte) (int, error) }) {
		buf := make([]byte, 4096)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				builder.WriteString(chunk)
				streamChunk(ctx, s, cmd.ID, chunk)
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
	status := "completed"
	output := builder.String()
	if waitErr != nil {
		status = "failed"
		if taskCtx.Err() == context.DeadlineExceeded {
			output += fmt.Sprintf("\nERROR: task timed out after %ds", task.Timeout)
		} else {
			output += fmt.Sprintf("\nERROR: %v", waitErr)
		}
		log.Printf("Custom task %q failed: %v", task.ID, waitErr)
	} else {
		log.Printf("Custom task %q completed", task.ID)
	}
	reportTerminal(ctx, s, cmd, status, output)
}
