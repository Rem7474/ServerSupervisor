package handlers

import (
	"fmt"
	"log"
	"time"

	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
)

func (h *ReleaseTrackerHandler) notifyReleaseTrackerDetected(t models.ReleaseTracker, version, releaseURL, releaseName string) {
	if h.notifHub == nil || len(t.NotifyChannels) == 0 {
		return
	}

	hasBrowser := false
	for _, ch := range t.NotifyChannels {
		if ch == "browser" {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return
	}

	label := "Git"
	if t.TrackerType == "docker" {
		label = "Docker"
	}

	h.notifHub.Broadcast(map[string]interface{}{
		"type": "release_tracker_detected",
		"notification": map[string]interface{}{
			"tracker_id":   t.ID,
			"tracker_name": t.Name,
			"tracker_type": t.TrackerType,
			"version":      version,
			"release_url":  releaseURL,
			"release_name": releaseName,
			"status":       "detected",
			"label":        label,
			"triggered_at": time.Now().UTC(),
		},
	})
}

// NotifyComplete is called by the agent handler when a command completes.
// It is fire-and-forget from an HTTP goroutine, so it uses h.pollerCtx
// (cancelled at shutdown) rather than the request ctx (which is already done).
func (h *ReleaseTrackerHandler) NotifyComplete(commandID, status string) {
	ctx := h.pollerCtx
	trackerID, notifyOnRelease, channels, err := h.db.UpdateReleaseTrackerExecutionByCommandID(ctx, commandID, status)
	if err != nil {
		return // not a tracker command
	}

	if !notifyOnRelease || len(channels) == 0 {
		return
	}

	tracker, err := h.db.GetReleaseTrackerByID(ctx, trackerID)
	if err != nil {
		return
	}

	emoji := "✅"
	if status == "failed" {
		emoji = "❌"
	}

	var subject, msg string
	if tracker.TrackerType == "docker" {
		imageFull := tracker.DockerImage + ":" + tracker.DockerTag
		if tracker.DockerTag == "" {
			imageFull = tracker.DockerImage + ":latest"
		}
		subject = fmt.Sprintf("[ServerSupervisor] Docker tracker %s %s %s", tracker.Name, emoji, status)
		msg = fmt.Sprintf("Docker tracker '%s' (%s) execution %s on host %s (task: %s)",
			tracker.Name, imageFull, status, tracker.HostID, tracker.CustomTaskID)
	} else {
		subject = fmt.Sprintf("[ServerSupervisor] Release tracker %s %s %s", tracker.Name, emoji, status)
		msg = fmt.Sprintf("Release tracker '%s' (%s/%s) execution %s on host %s (task: %s)",
			tracker.Name, tracker.RepoOwner, tracker.RepoName, status, tracker.HostID, tracker.CustomTaskID)
	}

	notifier := notify.New()
	for _, ch := range channels {
		switch ch {
		case "smtp":
			to := h.cfg.SMTPTo
			if to == "" || h.cfg.SMTPFrom == "" {
				continue
			}
			if err := notifier.SendSMTP(h.cfg, h.cfg.SMTPFrom, to, subject, msg); err != nil {
				log.Printf("Release tracker SMTP send: %v", err)
			}

		case "ntfy":
			ntfyURL := h.cfg.NotifyURL
			if ntfyURL == "" {
				continue
			}
			if err := notifier.SendNtfy(h.cfg, ntfyURL, subject, msg); err != nil {
				log.Printf("Release tracker notify ntfy: %v", err)
			}

		case "browser":
			if h.notifHub == nil {
				continue
			}
			h.notifHub.Broadcast(map[string]interface{}{
				"type": "release_tracker_execution",
				"notification": map[string]interface{}{
					"tracker_id":   tracker.ID,
					"tracker_name": tracker.Name,
					"tracker_type": tracker.TrackerType,
					"status":       status,
					"triggered_at": time.Now().UTC(),
				},
			})
		}
	}
}

func (h *ReleaseTrackerHandler) HandleCommandCompletion(commandID, status string) {
	h.NotifyComplete(commandID, status)
}
