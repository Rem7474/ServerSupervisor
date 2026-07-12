package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
	"github.com/serversupervisor/server/internal/services/notifychannels"
	"github.com/serversupervisor/server/internal/services/push"
)

func buildAlertMessage(rule models.AlertRule, host models.Host, value float64) string {
	if rule.Metric == "heartbeat_timeout" {
		totalSecs := int(value)
		if totalSecs >= 60 {
			return fmt.Sprintf("Agent silencieux sur %s depuis %dm%ds (dernier contact : %s)",
				host.Name, totalSecs/60, totalSecs%60, host.LastSeen.Local().Format("15:04:05"))
		}
		return fmt.Sprintf("Agent silencieux sur %s depuis %ds (dernier contact : %s)",
			host.Name, totalSecs, host.LastSeen.Local().Format("15:04:05"))
	}

	// Format Proxmox metrics in French with scope information
	if isProxmoxMetric(rule.Metric) {
		metricLabel := rule.Metric
		switch rule.Metric {
		case "proxmox_storage_percent":
			metricLabel = "Stockage Proxmox"
		case "proxmox_node_cpu_percent":
			metricLabel = "CPU noeud Proxmox"
		case "proxmox_node_memory_percent":
			metricLabel = "RAM noeud Proxmox"
		case "proxmox_node_cpu_temperature":
			metricLabel = "Temp. CPU noeud Proxmox"
		case "proxmox_node_fan_rpm":
			metricLabel = "RPM ventilateurs noeud Proxmox"
		case "proxmox_guest_cpu_percent":
			metricLabel = "CPU VM/LXC Proxmox"
		case "proxmox_guest_memory_percent":
			metricLabel = "RAM VM/LXC Proxmox"
		case "proxmox_node_pending_updates":
			metricLabel = "Paquets APT en attente"
		case "proxmox_recent_failed_tasks_24h":
			metricLabel = "Tâches Proxmox échouées (24h)"
		case "proxmox_auth_failures_recent":
			metricLabel = "Echecs auth Proxmox (logs)"
		case "proxmox_disk_failed_count":
			metricLabel = "Disques physiques en échec"
		case "proxmox_disk_min_wearout_percent":
			metricLabel = "Usure disque min"
		}
		switch rule.Metric {
		case "proxmox_node_pending_updates", "proxmox_recent_failed_tasks_24h", "proxmox_auth_failures_recent", "proxmox_disk_failed_count":
			return fmt.Sprintf("Alerte %s %s %.0f sur %s", metricLabel, rule.Operator, value, host.Name)
		case "proxmox_node_cpu_temperature":
			return fmt.Sprintf("Alerte %s %s %.1f°C sur %s", metricLabel, rule.Operator, value, host.Name)
		case "proxmox_node_fan_rpm":
			return fmt.Sprintf("Alerte %s %s %.0f RPM sur %s", metricLabel, rule.Operator, value, host.Name)
		default:
			return fmt.Sprintf("Alerte %s %s %.1f%% sur %s", metricLabel, rule.Operator, value, host.Name)
		}
	}

	return fmt.Sprintf("Alert %s %s %.2f on host %s (%s)", rule.Metric, rule.Operator, value, host.Name, host.ID)
}

func alertMetricUnit(metric string) string {
	switch metric {
	case "cpu", "memory", "disk":
		return "%"
	default:
		return ""
	}
}

// firedEvent builds the notifychannels.Event for a newly-fired (or re-fired)
// incident: smtp/ntfy/legacy-webhook/browser(WS+push) fanned out across
// whatever channels the rule is configured with. OnBrowser is left nil — the
// caller sets it, since it needs the incident ID and the live pusher.
func firedEvent(cfg *config.Config, rule models.AlertRule, host models.Host, value float64) notifychannels.Event {
	msg := buildAlertMessage(rule, host, value)

	smtpTo := rule.Actions.SMTPTo
	if smtpTo == "" {
		smtpTo = cfg.SMTPTo
	}

	ntfyURL := ntfyTopicURL(cfg.NotifyURL, rule.Actions.NtfyTopic)

	// SMTP gets the styled HTML template when it renders cleanly; every other
	// channel keeps the plain-text message (ntfy/webhook bodies aren't HTML).
	smtpBody := msg
	if html, err := notify.RenderAlertEmail(alertEmailData(cfg, rule, host, value)); err != nil {
		slog.Warn("alerts: failed to render HTML alert email, falling back to plain text", slog.Any("err", err))
	} else {
		smtpBody = html
	}

	return notifychannels.Event{
		LogID:       fmt.Sprintf("rule:%d", rule.ID),
		Channels:    rule.Actions.Channels,
		SMTPSubject: "[ServerSupervisor] Alert triggered",
		SMTPBody:    smtpBody,
		SMTPTo:      smtpTo,
		NtfyTitle:   "ServerSupervisor Alert",
		NtfyBody:    msg,
		NtfyURL:     ntfyURL,
		LegacyWebhook: map[string]interface{}{
			"title":          "ServerSupervisor Alert",
			"message":        msg,
			"rule_id":        rule.ID,
			"host_id":        host.ID,
			"host_name":      host.Name,
			"metric":         rule.Metric,
			"operator":       rule.Operator,
			"threshold_warn": rule.ThresholdWarn,
			"threshold_crit": rule.ThresholdCrit,
			"value":          value,
			"triggered_at":   time.Now().UTC(),
		},
		Push: &push.Payload{
			Title:  "Alerte : " + rule.DisplayName(),
			Body:   fmt.Sprintf("%s — Valeur : %.2f%s", host.Name, value, alertMetricUnit(rule.Metric)),
			Tag:    fmt.Sprintf("alert-%d-%s", rule.ID, host.ID),
			URL:    "/alerts?tab=incidents",
			Status: "fired",
		},
	}
}

// alertEmailData formats a fired incident into the plain-string fields
// alert_email_template.html expects. Threshold prefers the crit value when
// set, same precedence as AlertRule.DisplayName().
func alertEmailData(cfg *config.Config, rule models.AlertRule, host models.Host, value float64) notify.AlertEmailData {
	threshold := rule.ThresholdWarn
	if rule.ThresholdCrit != nil {
		threshold = rule.ThresholdCrit
	}
	thresholdStr := ""
	if threshold != nil {
		thresholdStr = fmt.Sprintf("%.2f", *threshold)
	}

	cooldownMsg := ""
	if rule.Actions.Cooldown > 0 {
		cooldownMsg = formatCooldownDuration(rule.Actions.Cooldown)
	}

	return notify.AlertEmailData{
		RuleName:        rule.DisplayName(),
		RuleID:          rule.ID,
		HostName:        host.Name,
		Metric:          rule.Metric,
		Operator:        rule.Operator,
		Threshold:       thresholdStr,
		Value:           fmt.Sprintf("%.2f", value),
		Unit:            alertMetricUnit(rule.Metric),
		TriggeredAt:     time.Now().Local().Format("2006-01-02 15:04:05"),
		IncidentLink:    strings.TrimRight(cfg.BaseURL, "/") + "/alerts?tab=incidents",
		CooldownMessage: cooldownMsg,
	}
}

// formatCooldownDuration renders AlertActions.Cooldown (seconds) as a short
// human string for the email footer ("no further notifications for X").
func formatCooldownDuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// ntfyTopicURL resolves the ntfy target for an alert rule. A rule only ever
// overrides the topic (rule.Actions.NtfyTopic); the server (scheme+host) still
// comes from the admin's configured ntfy URL (cfg.NotifyURL) — which may be a
// self-hosted instance, not just ntfy.sh. Falls back to the public ntfy.sh
// service when no server is configured at all, matching prior zero-config
// behavior for anyone who never set the "ntfy_url" setting.
func ntfyTopicURL(base, topic string) string {
	if topic == "" {
		return base
	}
	if base == "" {
		return "https://ntfy.sh/" + topic
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "https://ntfy.sh/" + topic
	}
	u.Path = "/" + topic
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// resolvedEvent builds the notifychannels.Event for an incident that just
// resolved. Resolution only ever goes out over "browser" — smtp/ntfy/legacy
// webhook don't fire on resolve, matching the pre-existing (fired-only)
// behavior of those channels. The Web Push payload reuses the exact tag the
// fired notification used, so the service worker updates it in place instead
// of leaving the Android/PWA notification stuck in the alerting state.
func resolvedEvent(rule models.AlertRule, host models.Host) notifychannels.Event {
	return notifychannels.Event{
		LogID:    fmt.Sprintf("rule:%d", rule.ID),
		Channels: []string{"browser"},
		Push: &push.Payload{
			Title:  "Résolu : " + rule.DisplayName(),
			Body:   fmt.Sprintf("%s — revenu à la normale", host.Name),
			Tag:    fmt.Sprintf("alert-%d-%s", rule.ID, host.ID),
			URL:    "/alerts?tab=incidents",
			Status: "resolved",
		},
	}
}

// newAlertBroadcast returns the OnBrowser callback for a freshly-fired
// incident: a WebSocket "new_alert" event carrying enough detail for the
// frontend's notification bell/list, independent of the Web Push payload
// above (which is sized for a system notification instead).
func newAlertBroadcast(pusher NotificationPusher, rule models.AlertRule, host models.Host, value float64, incID int64) func() {
	return func() {
		if pusher == nil {
			return
		}
		pusher.Broadcast(models.WSNewAlertMessage{
			Type: "new_alert",
			Notification: models.WSAlertIncidentNotification{
				ID:            fmt.Sprintf("alert:%d", incID),
				Type:          "alert_incident",
				RuleID:        rule.ID,
				HostID:        host.ID,
				HostName:      host.Name,
				RuleName:      rule.DisplayName(),
				Metric:        rule.Metric,
				Value:         value,
				TriggeredAt:   time.Now().UTC(),
				ResolvedAt:    nil,
				BrowserNotify: true,
			},
		})
	}
}

// broadcastIncidentUpdate pushes a lightweight WS event so the frontend can refresh its incidents list
// without a polling interval. Fired for both new incidents and resolutions, regardless of channels.
func broadcastIncidentUpdate(pusher NotificationPusher, event string, rule models.AlertRule, hostID string) {
	if pusher == nil {
		return
	}
	pusher.Broadcast(models.WSAlertIncidentUpdate{
		Type:   "alert_incident_update",
		Event:  event, // "fired" | "resolved"
		RuleID: rule.ID,
		HostID: hostID,
	})
}

// triggerAlertCommand creates a remote command on the host when an alert fires,
// if the rule's actions include a CommandTrigger.
//
// For Proxmox-scoped alerts the engine builds a synthetic host (ID prefixed by
// "proxmox:") that does not exist in the hosts table. We resolve the target to
// the agent host linked to the Proxmox guest (when scope_mode=guest and a
// confirmed link exists); other Proxmox scopes have no unique linked host, so
// the trigger is skipped.
func triggerAlertCommand(ctx context.Context, dispatcher *dispatch.Dispatcher, db *database.DB, rule models.AlertRule, host models.Host) {
	if dispatcher == nil {
		return
	}
	ct := rule.Actions.CommandTrigger
	if ct == nil || ct.Module == "" || ct.Action == "" {
		return
	}

	targetHostID := host.ID
	targetLabel := host.Name
	ctTarget := ct.Target

	if strings.HasPrefix(host.ID, "docker:container:") {
		// Resolve the real host and, if no explicit target, use the container name.
		containerUUID := strings.TrimPrefix(host.ID, "docker:container:")
		c, err := db.GetDockerContainerByID(ctx, containerUUID)
		if err != nil || c == nil {
			slog.WarnContext(ctx, "alerts: command_trigger skipped — docker container not found", slog.Int64("rule_id", rule.ID), slog.String("container_uuid", containerUUID))
			return
		}
		targetHostID = c.HostID
		targetLabel = c.Name
		if ctTarget == "" {
			ctTarget = c.Name
		}
	} else if strings.HasPrefix(host.ID, "docker:compose:") {
		// Compose-level alerts: dispatch to the host embedded in the ID.
		rest := strings.TrimPrefix(host.ID, "docker:compose:")
		if idx := strings.Index(rest, ":"); idx >= 0 {
			targetHostID = rest[:idx]
		} else {
			slog.WarnContext(ctx, "alerts: command_trigger skipped — malformed docker:compose host ID", slog.Int64("rule_id", rule.ID), slog.String("host_id", host.ID))
			return
		}
	}

	if strings.HasPrefix(host.ID, "proxmox:") {
		parts := strings.SplitN(host.ID, ":", 3)
		if len(parts) != 3 || parts[1] != "guest" || parts[2] == "" {
			slog.WarnContext(ctx, "alerts: command_trigger skipped — no linked host for Proxmox scope", slog.Int64("rule_id", rule.ID), slog.String("scope", host.ID))
			return
		}
		link, err := db.GetProxmoxGuestLinkByGuest(ctx, parts[2])
		if err != nil || link == nil {
			slog.WarnContext(ctx, "alerts: command_trigger skipped — no host link for Proxmox guest", slog.Int64("rule_id", rule.ID), slog.String("guest", parts[2]))
			return
		}
		if link.Status != "confirmed" {
			slog.WarnContext(ctx, "alerts: command_trigger skipped — Proxmox guest link not confirmed", slog.Int64("rule_id", rule.ID), slog.String("guest", parts[2]), slog.String("status", link.Status))
			return
		}
		targetHostID = link.HostID
		targetLabel = link.HostName
	}

	payload := ct.Payload
	if payload == "" {
		payload = "{}"
	}
	triggeredBy := fmt.Sprintf("alert:%d", rule.ID)
	auditDetails := fmt.Sprintf(`{"rule_id":%d,"module":%q,"action":%q,"target":%q}`, rule.ID, ct.Module, ct.Action, ctTarget)
	if _, err := dispatcher.Create(ctx, dispatch.Request{
		HostID:      targetHostID,
		Module:      ct.Module,
		Action:      ct.Action,
		Target:      ctTarget,
		Payload:     payload,
		TriggeredBy: triggeredBy,
		Audit: &dispatch.AuditLogRequest{
			Username:  "alert-engine",
			Action:    "alert_command_trigger",
			HostID:    targetHostID,
			IPAddress: "",
			Details:   auditDetails,
		},
	}); err != nil {
		slog.ErrorContext(ctx, "alerts: failed to create command trigger", slog.Int64("rule_id", rule.ID), slog.String("host_id", targetHostID), slog.Any("err", err))
	} else {
		slog.InfoContext(ctx, "alerts: triggered command", slog.String("module", ct.Module), slog.String("action", ct.Action), slog.String("host", targetLabel), slog.Int64("rule_id", rule.ID))
	}
}
