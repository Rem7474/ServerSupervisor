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

func EvaluateAlerts(db *database.DB, cfg *config.Config) {
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

		for _, host := range hosts {
			if rule.HostID != nil && *rule.HostID != host.ID {
				continue
			}

			value, ok := getMetricValue(db, host, rule)
			if !ok {
				continue
			}

			matched := matchRule(rule, host, value)
			inc, err := db.GetOpenAlertIncident(rule.ID, host.ID)
			if err != nil && err != sql.ErrNoRows {
				log.Printf("Alerts: failed to check incidents: %v", err)
				continue
			}

			if matched {
				if err == sql.ErrNoRows || inc == nil {
					if err := db.CreateAlertIncident(rule.ID, host.ID, value); err != nil {
						log.Printf("Alerts: failed to create incident: %v", err)
						continue
					}
					n.notify(cfg, rule, host, value)
				}
			} else if inc != nil {
				_ = db.ResolveAlertIncident(inc.ID)
			}
		}
	}
}

func getMetricValue(db *database.DB, host models.Host, rule models.AlertRule) (float64, bool) {
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
	case "cpu_percent", "ram_percent", "disk_percent":
		metrics, err := db.GetLatestMetrics(host.ID)
		if err != nil || metrics == nil {
			return 0, false
		}
		if rule.DurationSeconds > 0 && now.Sub(metrics.Timestamp) > duration {
			return 0, false
		}
		switch rule.Metric {
		case "cpu_percent":
			return metrics.CPUUsagePercent, true
		case "ram_percent":
			return metrics.MemoryPercent, true
		case "disk_percent":
			maxDisk := 0.0
			for _, d := range metrics.Disks {
				if d.UsedPercent > maxDisk {
					maxDisk = d.UsedPercent
				}
			}
			return maxDisk, true
		}
	}
	return 0, false
}

func matchRule(rule models.AlertRule, host models.Host, value float64) bool {
	if rule.Metric == "status_offline" {
		return host.Status == "offline"
	}
	if rule.Threshold == nil {
		return false
	}

	switch rule.Operator {
	case "gt":
		return value > *rule.Threshold
	case "lt":
		return value < *rule.Threshold
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

	config := map[string]interface{}{}
	if rule.ChannelConfig != "" {
		_ = json.Unmarshal([]byte(rule.ChannelConfig), &config)
	}

	switch rule.Channel {
	case "notify":
		url := cfg.NotifyURL
		if v, ok := config["url"].(string); ok && v != "" {
			url = v
		}
		if url == "" {
			log.Printf("Alerts: notify URL not configured")
			return
		}
		data, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", url, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if resp, err := n.client.Do(req); err != nil {
			log.Printf("Alerts: notify failed: %v", err)
		} else {
			_ = resp.Body.Close()
		}
	case "smtp":
		to := cfg.SMTPTo
		from := cfg.SMTPFrom
		if v, ok := config["to"].(string); ok && v != "" {
			to = v
		}
		if v, ok := config["from"].(string); ok && v != "" {
			from = v
		}
		if to == "" || from == "" {
			log.Printf("Alerts: SMTP to/from not configured")
			return
		}
		subject := "[ServerSupervisor] Alert triggered"
		if v, ok := config["subject"].(string); ok && v != "" {
			subject = v
		}
		n.sendSMTP(cfg, from, to, subject, msg)
	default:
		log.Printf("Alerts: unknown channel %s", rule.Channel)
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
