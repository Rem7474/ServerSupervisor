package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
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

func sendAlertNotifications(n notify.Notifier, cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
	msg := buildAlertMessage(rule, host, value)
	payload := map[string]interface{}{
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
	}

	for _, channel := range rule.Actions.Channels {
		switch channel {
		case "smtp":
			to := rule.Actions.SMTPTo
			if to == "" {
				to = cfg.SMTPTo
			}
			if to == "" || cfg.SMTPFrom == "" {
				slog.Warn("alerts: SMTP to/from not configured", slog.Int64("rule_id", rule.ID))
				continue
			}
			if err := n.SendSMTP(cfg, cfg.SMTPFrom, to, "[ServerSupervisor] Alert triggered", msg); err != nil {
				slog.Error("alerts: SMTP send failed", slog.Int64("rule_id", rule.ID), slog.Any("err", err))
			}

		case "ntfy":
			topic := rule.Actions.NtfyTopic
			if topic == "" {
				slog.Warn("alerts: ntfy topic not configured", slog.Int64("rule_id", rule.ID))
				continue
			}
			ntfyURL := "https://ntfy.sh/" + topic
			if err := n.SendNtfy(cfg, ntfyURL, "ServerSupervisor Alert", msg); err != nil {
				slog.Error("alerts: ntfy failed", slog.Int64("rule_id", rule.ID), slog.Any("err", err))
			}

		case "notify":
			// Legacy webhook channel — send to configured notify URL
			notifyURL := cfg.NotifyURL
			if notifyURL == "" {
				slog.Warn("alerts: notify URL not configured")
				continue
			}
			data, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", notifyURL, bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 10 * time.Second}
			if resp, err := client.Do(req); err != nil {
				slog.Error("alerts: notify failed", slog.Any("err", err))
			} else {
				_ = resp.Body.Close()
			}

		case "browser":
			// Browser notifications are delivered via the WebSocket push mechanism; no backend action needed here.

		default:
			slog.Warn("alerts: unknown channel", slog.String("channel", channel), slog.Int64("rule_id", rule.ID))
		}
	}
}

// pushWebNotifications delivers a Web Push notification to every registered device subscription
// when a rule with the "browser" channel fires. Runs in a goroutine; VAPID keys are fetched
// from the settings table (generated on first alert if not yet present).
func pushWebNotifications(ctx context.Context, db *database.DB, cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
	hasBrowser := false
	for _, ch := range rule.Actions.Channels {
		if ch == "browser" {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return
	}

	privateKey, err := db.GetSetting(ctx, "vapid_private_key")
	if err != nil || privateKey == "" {
		return
	}
	publicKey, err := db.GetSetting(ctx, "vapid_public_key")
	if err != nil || publicKey == "" {
		return
	}

	ruleName := ""
	if rule.Name != nil {
		ruleName = *rule.Name
	} else if rule.ThresholdCrit != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (crit)", rule.Metric, rule.Operator, *rule.ThresholdCrit)
	} else if rule.ThresholdWarn != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (warn)", rule.Metric, rule.Operator, *rule.ThresholdWarn)
	}

	unit := ""
	switch rule.Metric {
	case "cpu", "memory", "disk":
		unit = "%"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"title": "Alerte : " + ruleName,
		"body":  fmt.Sprintf("%s — Valeur : %.2f%s", host.Name, value, unit),
		"tag":   fmt.Sprintf("alert-%d-%s", rule.ID, host.ID),
		"url":   "/alerts?tab=incidents",
	})

	subs, err := db.GetPushSubscriptionsByRole(ctx, "admin")
	if err != nil || len(subs) == 0 {
		return
	}
	for _, sub := range subs {
		wpSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256DHKey,
				Auth:   sub.AuthKey,
			},
		}
		resp, sendErr := webpush.SendNotification(payload, wpSub, &webpush.Options{
			Subscriber:      cfg.BaseURL,
			VAPIDPublicKey:  publicKey,
			VAPIDPrivateKey: privateKey,
			TTL:             120,
		})
		if sendErr != nil {
			slog.ErrorContext(ctx, "push: delivery failed", slog.String("endpoint", truncateStr(sub.Endpoint, 40)), slog.Any("err", sendErr))
			if resp != nil && resp.StatusCode == http.StatusGone {
				_ = db.DeletePushSubscription(ctx, sub.Endpoint)
			}
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// pushBrowserNotification sends a real-time WebSocket event to all connected frontend clients
// when a rule with the "browser" channel fires. Safe to call with a nil pusher.
func pushBrowserNotification(pusher NotificationPusher, rule models.AlertRule, host models.Host, value float64, incID int64) {
	if pusher == nil {
		return
	}
	hasBrowser := false
	for _, ch := range rule.Actions.Channels {
		if ch == "browser" {
			hasBrowser = true
			break
		}
	}
	if !hasBrowser {
		return
	}

	ruleName := ""
	if rule.Name != nil {
		ruleName = *rule.Name
	} else if rule.ThresholdCrit != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (crit)", rule.Metric, rule.Operator, *rule.ThresholdCrit)
	} else if rule.ThresholdWarn != nil {
		ruleName = fmt.Sprintf("%s %s %.2f (warn)", rule.Metric, rule.Operator, *rule.ThresholdWarn)
	}

	pusher.Broadcast(map[string]interface{}{
		"type": "new_alert",
		"notification": map[string]interface{}{
			"id":             fmt.Sprintf("alert:%d", incID),
			"type":           "alert_incident",
			"rule_id":        rule.ID,
			"host_id":        host.ID,
			"host_name":      host.Name,
			"rule_name":      ruleName,
			"metric":         rule.Metric,
			"value":          value,
			"triggered_at":   time.Now().UTC(),
			"resolved_at":    nil,
			"browser_notify": true,
		},
	})
}

// broadcastIncidentUpdate pushes a lightweight WS event so the frontend can refresh its incidents list
// without a polling interval. Fired for both new incidents and resolutions, regardless of channels.
func broadcastIncidentUpdate(pusher NotificationPusher, event string, rule models.AlertRule, hostID string) {
	if pusher == nil {
		return
	}
	pusher.Broadcast(map[string]interface{}{
		"type":    "alert_incident_update",
		"event":   event, // "fired" | "resolved"
		"rule_id": rule.ID,
		"host_id": hostID,
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
	if _, err := dispatcher.Create(ctx, dispatch.Request{
		HostID:      targetHostID,
		Module:      ct.Module,
		Action:      ct.Action,
		Target:      ct.Target,
		Payload:     payload,
		TriggeredBy: triggeredBy,
	}); err != nil {
		slog.ErrorContext(ctx, "alerts: failed to create command trigger", slog.Int64("rule_id", rule.ID), slog.String("host_id", targetHostID), slog.Any("err", err))
	} else {
		slog.InfoContext(ctx, "alerts: triggered command", slog.String("module", ct.Module), slog.String("action", ct.Action), slog.String("host", targetLabel), slog.Int64("rule_id", rule.ID))
	}
}
