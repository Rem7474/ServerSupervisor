package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/serversupervisor/agent/internal/collector"
	"github.com/serversupervisor/agent/internal/sender"
)

// validComposeName matches Docker Compose project and service names.
// Docker itself restricts these to lowercase alphanumerics plus -._ ; we apply
// the same allow-list as defense-in-depth so a server-supplied target can never
// be interpreted as a CLI flag or expand unexpectedly.
var validComposeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// composePayload is the typed payload for module=compose, action=update.
// All execution behaviour is data, never a command string: pre/post hooks are
// tasks.yaml IDs (host-declared), and booleans map to fixed flags agent-side.
type composePayload struct {
	Service          string            `json:"service"`            // optional: limit to one service
	PreTaskID        string            `json:"pre_task_id"`        // optional tasks.yaml hook
	PostTaskID       string            `json:"post_task_id"`       // optional tasks.yaml hook
	Cleanup          bool              `json:"cleanup"`            // prune dangling images after success
	HealthTimeoutSec int               `json:"healthcheck_timeout_sec"`
	Rollback         bool              `json:"rollback"`           // retag+redeploy old image on failure
	Env              map[string]string `json:"env"`
}

// handleCompose performs a native "Watchtower-like" update of a Docker Compose
// project: optional pre-hook → pull → up -d → healthcheck → optional rollback →
// optional post-hook → optional image prune. cmd.Target is the compose project
// name, accepted only if it exists in the agent's local inventory.
func handleCompose(ctx context.Context, d *Dispatcher, s *sender.Sender, cmd sender.PendingCommand) {
	if cmd.Action != "update" {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("unknown compose action: %s", cmd.Action))
		return
	}

	project := cmd.Target
	if !validComposeName.MatchString(project) {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("invalid compose project name: %q", project))
		return
	}

	var p composePayload
	if cmd.Payload != "" {
		if err := json.Unmarshal([]byte(cmd.Payload), &p); err != nil {
			reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("invalid payload: %v", err))
			return
		}
	}
	if p.Service != "" && !validComposeName.MatchString(p.Service) {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("invalid compose service name: %q", p.Service))
		return
	}

	// Refuse projects not present in the local inventory — the working dir comes
	// from Docker's own labels, so the server can never inject a path or target.
	proj, err := collector.ResolveComposeProject(project)
	if err != nil {
		reportTerminal(ctx, s, cmd, "failed", fmt.Sprintf("compose discovery failed: %v", err))
		return
	}
	if proj == nil {
		reportTerminal(ctx, s, cmd, "failed",
			fmt.Sprintf("compose project %q not found locally — deploy it once before tracking", project))
		return
	}

	// Serialize updates per project: two concurrent up/pull on the same project
	// can corrupt container state.
	unlock := d.lockCompose(project)
	defer unlock()

	reportRunning(ctx, s, cmd)
	var out strings.Builder
	stream := func(chunk string) {
		out.WriteString(chunk)
		streamChunk(ctx, s, cmd.ID, chunk)
	}
	section := func(title string) { stream("\n=== " + title + " ===\n") }

	// 1. Pre-update hook (host-declared task; aborts the update if it fails).
	if p.PreTaskID != "" {
		section("Pre-update hook: " + p.PreTaskID)
		task := d.tasks.FindTask(p.PreTaskID)
		if task == nil {
			reportTerminal(ctx, s, cmd, "failed",
				out.String()+fmt.Sprintf("\nERROR: pre-update task %q not found in tasks.yaml", p.PreTaskID))
			return
		}
		hookOut, herr := executeTask(ctx, task, p.Env, stream)
		_ = hookOut
		if herr != nil {
			reportTerminal(ctx, s, cmd, "failed",
				out.String()+fmt.Sprintf("\nERROR: pre-update hook failed: %v (update aborted)", herr))
			return
		}
	}

	// 2. Capture current images for rollback before mutating anything.
	var snapshot []collector.ComposeServiceImage
	if p.Rollback {
		if snap, serr := collector.CaptureComposeImages(ctx, project, p.Service); serr != nil {
			stream(fmt.Sprintf("warning: could not snapshot images for rollback: %v\n", serr))
		} else {
			snapshot = snap
		}
	}

	// 3. Pull new images.
	section("Pulling images")
	if _, perr := collector.ComposePull(ctx, project, proj.WorkingDir, p.Service, stream); perr != nil {
		reportTerminal(ctx, s, cmd, "failed", out.String()+fmt.Sprintf("\nERROR: pull failed: %v", perr))
		return
	}

	// 4. Recreate/start containers.
	section("Applying (up -d)")
	if _, uerr := collector.ComposeUp(ctx, project, proj.WorkingDir, p.Service, stream); uerr != nil {
		out.WriteString(fmt.Sprintf("\nERROR: up failed: %v", uerr))
		if rolledBack := tryRollback(ctx, project, proj.WorkingDir, p, snapshot, stream); rolledBack {
			out.WriteString("\nRolled back to previous images.")
		}
		reportTerminal(ctx, s, cmd, "failed", out.String())
		return
	}

	// 5. Healthcheck (optional).
	if p.HealthTimeoutSec > 0 {
		section(fmt.Sprintf("Waiting for health (timeout %ds)", p.HealthTimeoutSec))
		healthy, detail := collector.WaitComposeHealthy(ctx, project, p.Service,
			time.Duration(p.HealthTimeoutSec)*time.Second, stream)
		stream("\n" + detail + "\n")
		if !healthy {
			out.WriteString("\nERROR: containers did not become healthy")
			if rolledBack := tryRollback(ctx, project, proj.WorkingDir, p, snapshot, stream); rolledBack {
				out.WriteString("\nRolled back to previous images.")
			}
			reportTerminal(ctx, s, cmd, "failed", out.String())
			return
		}
	}

	// 6. Post-update hook (runs after a successful deploy; failure is surfaced
	//    but the update itself already succeeded).
	status := "completed"
	if p.PostTaskID != "" {
		section("Post-update hook: " + p.PostTaskID)
		if task := d.tasks.FindTask(p.PostTaskID); task != nil {
			if _, herr := executeTask(ctx, task, p.Env, stream); herr != nil {
				out.WriteString(fmt.Sprintf("\nWARNING: post-update hook failed: %v", herr))
				status = "failed"
			}
		} else {
			out.WriteString(fmt.Sprintf("\nWARNING: post-update task %q not found in tasks.yaml", p.PostTaskID))
			status = "failed"
		}
	}

	// 7. Cleanup dangling images (only after a healthy deploy; runs after any
	//    rollback path has already returned, so the old image is safe to prune).
	if p.Cleanup {
		section("Pruning dangling images")
		if _, cerr := collector.PruneImages(ctx, stream); cerr != nil {
			out.WriteString(fmt.Sprintf("\nWARNING: image prune failed: %v", cerr))
		}
	}

	log.Printf("Compose update %q completed (status=%s)", project, status)
	reportTerminal(ctx, s, cmd, status, out.String())
}

// tryRollback retags the snapshot images back and re-applies the project.
// Returns true if a rollback was attempted and the redeploy succeeded.
func tryRollback(ctx context.Context, project, workingDir string, p composePayload, snapshot []collector.ComposeServiceImage, stream func(string)) bool {
	if !p.Rollback || len(snapshot) == 0 {
		return false
	}
	stream("\n=== Rolling back ===\n")
	if err := collector.RollbackComposeImages(ctx, snapshot, stream); err != nil {
		stream(fmt.Sprintf("rollback retag failed: %v\n", err))
		return false
	}
	if _, err := collector.ComposeUp(ctx, project, workingDir, p.Service, stream); err != nil {
		stream(fmt.Sprintf("rollback redeploy failed: %v\n", err))
		return false
	}
	return true
}
