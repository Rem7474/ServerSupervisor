package alerts

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
)

type notifier struct {
	client *http.Client
}

func newNotifier() *notifier {
	return &notifier{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// NotificationPusher broadcasts a real-time alert event to connected frontend clients.
// The api.NotificationHub implements this interface; pass nil to skip push.
type NotificationPusher interface {
	Broadcast(payload interface{})
}

func EvaluateAlerts(db *database.DB, cfg *config.Config, pusher NotificationPusher) {
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

	n := newNotifier()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		ruleName := fmt.Sprintf("rule#%d(%s %s)", rule.ID, rule.Metric, rule.Operator)
		if rule.Name != nil && *rule.Name != "" {
			ruleName = fmt.Sprintf("rule#%d(%s)", rule.ID, *rule.Name)
		}

		for _, host := range hosts {
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
					n.notify(cfg, rule, host, value)
					triggerAlertCommand(db, rule, host)
					pushBrowserNotification(pusher, rule, host, value, incID)
				}
			} else if inc != nil {
				_ = db.ResolveAlertIncident(inc.ID)
				log.Printf("Alerts: %s host=%s — resolved incident#%d", ruleName, host.Name, inc.ID)
				details := fmt.Sprintf(`{"rule_id":%d,"incident_id":%d}`, rule.ID, inc.ID)
				_, _ = db.CreateAuditLog("alert-engine", "alert_resolved", host.ID, "", details, "success")
			}
		}
	}
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
	case "cpu", "cpu_percent", "memory", "ram_percent", "disk", "disk_percent", "load":
		metrics, err := db.GetLatestMetrics(host.ID)
		if err != nil || metrics == nil {
			return 0, false
		}
		if rule.DurationSeconds > 0 && now.Sub(metrics.Timestamp) > duration {
			return 0, false
		}
		switch rule.Metric {
		case "cpu", "cpu_percent":
			return metrics.CPUUsagePercent, true
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
	}
	return 0, false
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

func (n *notifier) notify(cfg *config.Config, rule models.AlertRule, host models.Host, value float64) {
	msg := fmt.Sprintf("Alert %s %s %.2f on host %s (%s)", rule.Metric, rule.Operator, value, host.Name, host.ID)
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
			n.sendSMTP(cfg, cfg.SMTPFrom, to, "[ServerSupervisor] Alert triggered", msg)

		case "ntfy":
			topic := rule.Actions.NtfyTopic
			if topic == "" {
				log.Printf("Alerts: ntfy topic not configured for rule %d", rule.ID)
				continue
			}
			data, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", "https://ntfy.sh/"+topic, bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Title", "ServerSupervisor Alert")
			if resp, err := n.client.Do(req); err != nil {
				log.Printf("Alerts: ntfy failed: %v", err)
			} else {
				_ = resp.Body.Close()
			}

		case "notify":
			// Legacy webhook channel — send to configured notify URL
			url := cfg.NotifyURL
			if url == "" {
				log.Printf("Alerts: notify URL not configured")
				continue
			}
			data, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
			req.Header.Set("Content-Type", "application/json")
			if resp, err := n.client.Do(req); err != nil {
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
			"id":            incID,
			"rule_id":       rule.ID,
			"host_id":       host.ID,
			"host_name":     host.Name,
			"rule_name":     ruleName,
			"metric":        rule.Metric,
			"value":         value,
			"triggered_at":  time.Now().UTC(),
			"resolved_at":   nil,
			"browser_notify": true,
		},
	})
}

// triggerAlertCommand creates a remote command on the host when an alert fires,
// if the rule's actions include a CommandTrigger.
func triggerAlertCommand(db *database.DB, rule models.AlertRule, host models.Host) {
	ct := rule.Actions.CommandTrigger
	if ct == nil || ct.Module == "" || ct.Action == "" {
		return
	}
	payload := ct.Payload
	if payload == "" {
		payload = "{}"
	}
	triggeredBy := fmt.Sprintf("alert:%d", rule.ID)
	if _, err := db.CreateRemoteCommand(host.ID, ct.Module, ct.Action, ct.Target, payload, triggeredBy, nil); err != nil {
		log.Printf("Alerts: failed to create command trigger for rule %d on host %s: %v", rule.ID, host.ID, err)
	} else {
		log.Printf("Alerts: triggered command %s/%s on host %s (rule %d)", ct.Module, ct.Action, host.Name, rule.ID)
	}
}

func (n *notifier) sendSMTP(cfg *config.Config, from, to, subject, body string) {
	if cfg.SMTPHost == "" || cfg.SMTPPort == 0 {
		log.Printf("Alerts: SMTP host/port not configured")
		return
	}

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		body,
	}, "\r\n")

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	c, err := smtp.Dial(addr)
	if err != nil {
		log.Printf("Alerts: SMTP dial failed: %v", err)
		return
	}
	defer c.Close()

	if cfg.SMTPTLS {
		_ = c.StartTLS(&tls.Config{ServerName: cfg.SMTPHost})
	}
	if cfg.SMTPUser != "" {
		_ = c.Auth(auth)
	}
	if err := c.Mail(from); err != nil {
		log.Printf("Alerts: SMTP mail failed: %v", err)
		return
	}
	if err := c.Rcpt(to); err != nil {
		log.Printf("Alerts: SMTP rcpt failed: %v", err)
		return
	}
	w, err := c.Data()
	if err != nil {
		log.Printf("Alerts: SMTP data failed: %v", err)
		return
	}
	_, _ = w.Write([]byte(msg))
	_ = w.Close()
}
