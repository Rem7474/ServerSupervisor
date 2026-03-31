package alerts

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
)

// NotificationPusher broadcasts a real-time alert event to connected frontend clients.
// The api.NotificationHub implements this interface; pass nil to skip push.
type NotificationPusher interface {
	Broadcast(payload interface{})
}

func EvaluateAlerts(db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, pusher NotificationPusher) {
	rules, err := db.GetAlertRules()
	if err != nil {
		log.Printf("Alerts: failed to fetch rules: %v", err)
		return
	}
	if len(rules) == 0 {
		return
	}

	hosts, err := db.GetAllHosts()
	if err != nil {
		log.Printf("Alerts: failed to fetch hosts: %v", err)
		return
	}

	n := notify.New()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		ruleName := fmt.Sprintf("rule#%d(%s %s)", rule.ID, rule.Metric, rule.Operator)
		if rule.Name != nil && *rule.Name != "" {
			ruleName = fmt.Sprintf("rule#%d(%s)", rule.ID, *rule.Name)
		}

		hostsForRule := hosts
		if rule.Metric == "proxmox_storage_percent" {
			hostsForRule = selectHostsForGlobalMetric(rule, hosts)
		}

		for _, host := range hostsForRule {
			if rule.HostID != nil && *rule.HostID != host.ID {
				continue
			}

			value, ok := GetMetricValue(db, host, rule)
			if !ok {
				continue
			}

			matched := MatchRule(rule, host, value)
			inc, err := db.GetOpenAlertIncident(rule.ID, host.ID)
			if err != nil && err != sql.ErrNoRows {
				log.Printf("Alerts: failed to check incidents: %v", err)
				continue
			}

			if matched {
				if err == sql.ErrNoRows || inc == nil {
					incID, err := db.CreateAlertIncident(rule.ID, host.ID, value)
					if err != nil {
						log.Printf("Alerts: failed to create incident: %v", err)
						continue
					}
					log.Printf("Alerts: FIRED %s host=%s value=%.2f → incident#%d created", ruleName, host.Name, value, incID)
					details := fmt.Sprintf(`{"rule_id":%d,"metric":"%s","operator":"%s","value":%.4f}`, rule.ID, rule.Metric, rule.Operator, value)
					_, _ = db.CreateAuditLog("alert-engine", "alert_fired", host.ID, "", details, "success")
					sendAlertNotifications(n, cfg, rule, host, value)
					triggerAlertCommand(dispatcher, rule, host)
					pushBrowserNotification(pusher, rule, host, value, incID)
					go pushWebNotifications(db, cfg, rule, host, value)
					broadcastIncidentUpdate(pusher, "fired", rule, host.ID)
				}
			} else if inc != nil {
				_ = db.ResolveAlertIncident(inc.ID)
				log.Printf("Alerts: %s host=%s — resolved incident#%d", ruleName, host.Name, inc.ID)
				details := fmt.Sprintf(`{"rule_id":%d,"incident_id":%d}`, rule.ID, inc.ID)
				_, _ = db.CreateAuditLog("alert-engine", "alert_resolved", host.ID, "", details, "success")
				broadcastIncidentUpdate(pusher, "resolved", rule, host.ID)
			}
		}
	}
}

// selectHostsForGlobalMetric returns a single host target for global metrics to avoid
// creating duplicated incidents for every host in the inventory.
func selectHostsForGlobalMetric(rule models.AlertRule, hosts []models.Host) []models.Host {
	if len(hosts) == 0 {
		return hosts
	}
	if rule.HostID != nil {
		for _, host := range hosts {
			if host.ID == *rule.HostID {
				return []models.Host{host}
			}
		}
		return []models.Host{}
	}
	return []models.Host{hosts[0]}
}

// GetMetricValue retrieves the current value of a metric for a host according to a rule.
// Supports both legacy metric names (cpu_percent, ram_percent, disk_percent) and
// the current short names (cpu, memory, disk) used by the frontend.
func GetMetricValue(db *database.DB, host models.Host, rule models.AlertRule) (float64, bool) {
	now := time.Now()
	duration := time.Duration(rule.DurationSeconds) * time.Second

	switch rule.Metric {
	case "status_offline":
		if rule.DurationSeconds > 0 && now.Sub(host.LastSeen) < duration {
			return 0, false
		}
		if host.Status == "offline" {
			return 1, true
		}
		return 0, true
	case "heartbeat_timeout":
		// Returns seconds since the agent last reported. Threshold is the silence duration in seconds.
		return now.Sub(host.LastSeen).Seconds(), true
	case "cpu", "cpu_percent", "memory", "ram_percent", "disk", "disk_percent", "load":
		// When a host is explicitly linked to a Proxmox guest in confirmed+auto mode,
		// CPU/RAM alerts must use guest metrics collected from Proxmox (hypervisor view).
		if rule.Metric == "cpu" || rule.Metric == "cpu_percent" || rule.Metric == "memory" || rule.Metric == "ram_percent" {
			if link, err := db.GetProxmoxGuestLinkByHost(host.ID); err == nil && link != nil && link.Status == "confirmed" && link.MetricsSource == "auto" {
				cpuPct, memPct, ts, err := db.GetLatestProxmoxGuestMetricPercent(link.GuestID)
				if err == nil {
					if rule.DurationSeconds > 0 && now.Sub(ts) > duration {
						return 0, false
					}
					if rule.Metric == "cpu" || rule.Metric == "cpu_percent" {
						return cpuPct, true
					}
					return memPct, true
				}
			}
		}

		metrics, err := db.GetLatestMetrics(host.ID)
		if err != nil {
			return 0, false
		}
		if rule.DurationSeconds > 0 && now.Sub(metrics.Timestamp) > duration {
			return 0, false
		}
		switch rule.Metric {
		case "cpu", "cpu_percent":
			return metrics.CPUUsagePercent, true
		case "cpu_temperature":
			temp, ok := db.GetEffectiveHostCPUTemperature(host.ID, metrics.CPUTemperature)
			if !ok || temp <= 0 {
				return 0, false
			}
			return temp, true
		case "memory", "ram_percent":
			return metrics.MemoryPercent, true
		case "disk", "disk_percent":
			maxDisk := 0.0
			for _, d := range metrics.Disks {
				if d.UsedPercent > maxDisk {
					maxDisk = d.UsedPercent
				}
			}
			return maxDisk, true
		case "load":
			return metrics.LoadAvg1, true
		}
		return 0, false
	case "disk_smart_status":
		// Returns 1 if any disk has FAILED SMART status, 0 otherwise.
		healthData, err := db.GetLatestDiskHealth(host.ID)
		if err != nil || len(healthData) == 0 {
			return 0, false
		}
		for _, h := range healthData {
			if h.SmartStatus == "FAILED" {
				return 1, true
			}
		}
		return 0, true
	case "disk_temperature":
		// Returns the maximum temperature (°C) across all disks for this host.
		healthData, err := db.GetLatestDiskHealth(host.ID)
		if err != nil || len(healthData) == 0 {
			return 0, false
		}
		maxTemp := 0.0
		for _, h := range healthData {
			if float64(h.Temperature) > maxTemp {
				maxTemp = float64(h.Temperature)
			}
		}
		return maxTemp, true
	case "proxmox_storage_percent":
		pct := resolveProxmoxStoragePercent(db, rule)
		return pct, true
	case "npm_requests", "npm_traffic_bytes", "npm_5xx_errors":
		requests, bytes, errors5xx, capturedAt, err := db.GetHostWebLogCache(host.ID)
		if err != nil || capturedAt == nil {
			return 0, false
		}

		if rule.DurationSeconds > 0 {
			if now.Sub(*capturedAt) > duration {
				return 0, false
			}
		}

		switch rule.Metric {
		case "npm_requests":
			return float64(requests), true
		case "npm_traffic_bytes":
			return float64(bytes), true
		case "npm_5xx_errors":
			return float64(errors5xx), true
		}
	}
	return 0, false
}

func resolveProxmoxStoragePercent(db *database.DB, rule models.AlertRule) float64 {
	scope := rule.Actions.ProxmoxScope
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return db.GetMaxProxmoxStorageUsagePercent()
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return db.GetMaxProxmoxStorageUsagePercent()
		}
		return db.GetMaxProxmoxStorageUsagePercentByConnection(scope.ConnectionID)
	case "node":
		if scope.NodeID == "" {
			return db.GetMaxProxmoxStorageUsagePercent()
		}
		return db.GetMaxProxmoxStorageUsagePercentByNode(scope.NodeID)
	case "storage":
		if scope.StorageID == "" {
			return db.GetMaxProxmoxStorageUsagePercent()
		}
		return db.GetProxmoxStorageUsagePercentByStorage(scope.StorageID)
	default:
		return db.GetMaxProxmoxStorageUsagePercent()
	}
}

// MatchRule evaluates whether a rule condition is currently met for the given value.
// Supports both symbol operators (">", "<", ">=", "<=") and legacy string operators ("gt", "lt").
func MatchRule(rule models.AlertRule, host models.Host, value float64) bool {
	if rule.Metric == "status_offline" {
		return host.Status == "offline"
	}
	if rule.Threshold == nil {
		return false
	}

	switch rule.Operator {
	case "gt", ">":
		return value > *rule.Threshold
	case "lt", "<":
		return value < *rule.Threshold
	case "gte", ">=":
		return value >= *rule.Threshold
	case "lte", "<=":
		return value <= *rule.Threshold
	case "eq":
		return value == *rule.Threshold
	default:
		return false
	}
}

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
	return fmt.Sprintf("Alert %s %s %.2f on host %s (%s)", rule.Metric, rule.Operator, value, host.Name, host.ID)
}

func sendAlertNotifications(n notify.Notifier, cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
	msg := buildAlertMessage(rule, host, value)
	payload := map[string]interface{}{
		"title":        "ServerSupervisor Alert",
		"message":      msg,
		"rule_id":      rule.ID,
		"host_id":      host.ID,
		"host_name":    host.Name,
		"metric":       rule.Metric,
		"operator":     rule.Operator,
		"threshold":    rule.Threshold,
		"value":        value,
		"triggered_at": time.Now().UTC(),
	}

	for _, channel := range rule.Actions.Channels {
		switch channel {
		case "smtp":
			to := rule.Actions.SMTPTo
			if to == "" {
				to = cfg.SMTPTo
			}
			if to == "" || cfg.SMTPFrom == "" {
				log.Printf("Alerts: SMTP to/from not configured for rule %d", rule.ID)
				continue
			}
			if err := n.SendSMTP(cfg, cfg.SMTPFrom, to, "[ServerSupervisor] Alert triggered", msg); err != nil {
				log.Printf("Alerts: SMTP send failed for rule %d: %v", rule.ID, err)
			}

		case "ntfy":
			topic := rule.Actions.NtfyTopic
			if topic == "" {
				log.Printf("Alerts: ntfy topic not configured for rule %d", rule.ID)
				continue
			}
			ntfyURL := "https://ntfy.sh/" + topic
			if err := n.SendNtfy(cfg, ntfyURL, "ServerSupervisor Alert", msg); err != nil {
				log.Printf("Alerts: ntfy failed for rule %d: %v", rule.ID, err)
			}

		case "notify":
			// Legacy webhook channel — send to configured notify URL
			notifyURL := cfg.NotifyURL
			if notifyURL == "" {
				log.Printf("Alerts: notify URL not configured")
				continue
			}
			data, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", notifyURL, bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{Timeout: 10 * time.Second}
			if resp, err := client.Do(req); err != nil {
				log.Printf("Alerts: notify failed: %v", err)
			} else {
				_ = resp.Body.Close()
			}

		case "browser":
			// Browser notifications are delivered via the WebSocket push mechanism; no backend action needed here.

		default:
			log.Printf("Alerts: unknown channel %q for rule %d", channel, rule.ID)
		}
	}
}

// pushWebNotifications delivers a Web Push notification to every registered device subscription
// when a rule with the "browser" channel fires. Runs in a goroutine; VAPID keys are fetched
// from the settings table (generated on first alert if not yet present).
func pushWebNotifications(db *database.DB, cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
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

	privateKey, err := db.GetSetting("vapid_private_key")
	if err != nil || privateKey == "" {
		return
	}
	publicKey, err := db.GetSetting("vapid_public_key")
	if err != nil || publicKey == "" {
		return
	}

	ruleName := ""
	if rule.Name != nil {
		ruleName = *rule.Name
	} else if rule.Threshold != nil {
		ruleName = fmt.Sprintf("%s %s %.2f", rule.Metric, rule.Operator, *rule.Threshold)
	}

	unit := ""
	switch rule.Metric {
	case "cpu", "cpu_percent", "memory", "ram_percent", "disk", "disk_percent":
		unit = "%"
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"title": "Alerte : " + ruleName,
		"body":  fmt.Sprintf("%s — Valeur : %.2f%s", host.Name, value, unit),
		"tag":   fmt.Sprintf("alert-%d-%s", rule.ID, host.ID),
		"url":   "/alerts?tab=incidents",
	})

	subs, err := db.GetPushSubscriptionsByRole("admin")
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
			log.Printf("Push: delivery failed (%s…): %v", truncateStr(sub.Endpoint, 40), sendErr)
			if resp != nil && resp.StatusCode == http.StatusGone {
				_ = db.DeletePushSubscription(sub.Endpoint)
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
	} else if rule.Threshold != nil {
		ruleName = fmt.Sprintf("%s %s %.2f", rule.Metric, rule.Operator, *rule.Threshold)
	}

	pusher.Broadcast(map[string]interface{}{
		"type": "new_alert",
		"notification": map[string]interface{}{
			"id":             incID,
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
func triggerAlertCommand(dispatcher *dispatch.Dispatcher, rule models.AlertRule, host models.Host) {
	if dispatcher == nil {
		return
	}
	ct := rule.Actions.CommandTrigger
	if ct == nil || ct.Module == "" || ct.Action == "" {
		return
	}
	payload := ct.Payload
	if payload == "" {
		payload = "{}"
	}
	triggeredBy := fmt.Sprintf("alert:%d", rule.ID)
	if _, err := dispatcher.Create(dispatch.Request{
		HostID:      host.ID,
		Module:      ct.Module,
		Action:      ct.Action,
		Target:      ct.Target,
		Payload:     payload,
		TriggeredBy: triggeredBy,
	}); err != nil {
		log.Printf("Alerts: failed to create command trigger for rule %d on host %s: %v", rule.ID, host.ID, err)
	} else {
		log.Printf("Alerts: triggered command %s/%s on host %s (rule %d)", ct.Module, ct.Action, host.Name, rule.ID)
	}
}
