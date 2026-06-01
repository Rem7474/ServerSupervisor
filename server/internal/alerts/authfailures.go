package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/proxmoxclient"
)

func resolveProxmoxAuthFailuresRecent(ctx context.Context, db *database.DB, rule models.AlertRule) float64 {
	window := time.Duration(rule.DurationSeconds) * time.Second
	if window <= 0 {
		window = 10 * time.Minute
	}
	since := time.Now().Add(-window)

	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
		}
		return float64(countAuthFailuresAcrossNodes(ctx, db, since, scope.ConnectionID))
	case "node":
		if scope.NodeID == "" {
			return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
		}
		node, err := db.GetProxmoxNode(ctx, scope.NodeID)
		if err != nil || node == nil {
			return 0
		}
		return float64(countAuthFailuresForNode(ctx, db, *node, since))
	default:
		return float64(countAuthFailuresAcrossNodes(ctx, db, since, ""))
	}
}

// FetchProxmoxAuthFailureLogs returns the syslog lines used for the proxmox_auth_failures_recent metric.
// The returned lines are already filtered by the requested duration window.
func FetchProxmoxAuthFailureLogs(ctx context.Context, db *database.DB, rule models.AlertRule) ([]string, time.Time) {
	window := time.Duration(rule.DurationSeconds) * time.Second
	if window <= 0 {
		window = 10 * time.Minute
	}
	since := time.Now().Add(-window)

	scope := proxmoxScopeFromRule(rule)
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
	}

	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID == "" {
			return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
		}
		return collectAuthFailureLogsAcrossNodes(ctx, db, since, scope.ConnectionID), since
	case "node":
		if scope.NodeID == "" {
			return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
		}
		node, err := db.GetProxmoxNode(ctx, scope.NodeID)
		if err != nil || node == nil {
			return []string{}, since
		}
		return collectAuthFailureLogsForNode(ctx, db, *node, since), since
	default:
		return collectAuthFailureLogsAcrossNodes(ctx, db, since, ""), since
	}
}

func countAuthFailuresAcrossNodes(ctx context.Context, db *database.DB, since time.Time, connectionID string) int {
	var nodes []models.ProxmoxNode
	var err error

	if strings.TrimSpace(connectionID) == "" {
		nodes, err = db.ListProxmoxNodes(ctx)
	} else {
		nodes, err = db.ListProxmoxNodesByConnection(ctx, connectionID)
	}
	if err != nil || len(nodes) == 0 {
		return 0
	}

	count := 0
	for _, node := range nodes {
		count += countAuthFailuresForNode(ctx, db, node, since)
	}
	return count
}

func collectAuthFailureLogsAcrossNodes(ctx context.Context, db *database.DB, since time.Time, connectionID string) []string {
	var nodes []models.ProxmoxNode
	var err error

	if strings.TrimSpace(connectionID) == "" {
		nodes, err = db.ListProxmoxNodes(ctx)
	} else {
		nodes, err = db.ListProxmoxNodesByConnection(ctx, connectionID)
	}
	if err != nil || len(nodes) == 0 {
		return []string{}
	}

	var out []string
	for _, node := range nodes {
		out = append(out, collectAuthFailureLogsForNode(ctx, db, node, since)...)
	}
	return out
}

func countAuthFailuresForNode(ctx context.Context, db *database.DB, node models.ProxmoxNode, since time.Time) int {
	conn, err := db.GetProxmoxConnectionByID(ctx, node.ConnectionID)
	if err != nil || conn == nil {
		return 0
	}
	secret, err := db.GetProxmoxTokenSecret(ctx, node.ConnectionID)
	if err != nil || secret == "" {
		return 0
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	services := []string{"pvedaemon", "pveproxy", "sshd"}
	limit := estimateAuthFailureLimit(since)

	count := 0
	for _, service := range services {
		lines, err := client.GetNodeSyslog(node.NodeName, limit, service)
		if err != nil {
			slog.ErrorContext(ctx, "alerts: syslog fetch failed", slog.String("connection", conn.Name), slog.String("node", node.NodeName), slog.String("service", service), slog.Any("err", err))
			continue
		}
		count += countAuthFailuresInLines(lines, since)
	}
	return count
}

func collectAuthFailureLogsForNode(ctx context.Context, db *database.DB, node models.ProxmoxNode, since time.Time) []string {
	conn, err := db.GetProxmoxConnectionByID(ctx, node.ConnectionID)
	if err != nil || conn == nil {
		return []string{}
	}
	secret, err := db.GetProxmoxTokenSecret(ctx, node.ConnectionID)
	if err != nil || secret == "" {
		return []string{}
	}

	client := proxmoxclient.New(conn.APIURL, conn.TokenID, secret, conn.InsecureSkipVerify)
	services := []string{"pvedaemon", "pveproxy", "sshd"}
	limit := estimateAuthFailureLimit(since)

	var out []string
	for _, service := range services {
		lines, err := client.GetNodeSyslog(node.NodeName, limit, service)
		if err != nil {
			slog.ErrorContext(ctx, "alerts: syslog fetch failed", slog.String("connection", conn.Name), slog.String("node", node.NodeName), slog.String("service", service), slog.Any("err", err))
			continue
		}
		out = append(out, authFailureLogLines(lines, since, node.NodeName)...)
	}
	return out
}

func authFailureLogLines(lines []proxmoxclient.PVESyslogLine, since time.Time, nodeName string) []string {
	var out []string
	for _, line := range lines {
		if !isAuthFailureSyslogLine(line) {
			continue
		}
		ts, ok := syslogLineTime(line)
		if !ok || ts.Before(since) {
			continue
		}
		text := formatSyslogLineText(line)
		if text == "" {
			continue
		}
		if nodeName != "" {
			text = fmt.Sprintf("[%s] %s", nodeName, text)
		}
		out = append(out, text)
	}
	return out
}

func formatSyslogLineText(line proxmoxclient.PVESyslogLine) string {
	if strings.TrimSpace(line.T) != "" {
		return strings.TrimSpace(line.T)
	}
	if strings.TrimSpace(line.Msg) != "" {
		return strings.TrimSpace(line.Msg)
	}
	parts := []string{}
	if strings.TrimSpace(line.Tag) != "" {
		parts = append(parts, strings.TrimSpace(line.Tag))
	}
	if strings.TrimSpace(line.Level) != "" {
		parts = append(parts, strings.TrimSpace(line.Level))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func countAuthFailuresInLines(lines []proxmoxclient.PVESyslogLine, since time.Time) int {
	count := 0
	for _, line := range lines {
		if !isAuthFailureSyslogLine(line) {
			continue
		}
		ts, ok := syslogLineTime(line)
		if !ok {
			// Skip lines without a usable timestamp so duration filters remain meaningful.
			continue
		}
		if ts.Before(since) {
			continue
		}
		count++
	}
	return count
}

func isAuthFailureSyslogLine(line proxmoxclient.PVESyslogLine) bool {
	text := strings.ToLower(strings.TrimSpace(strings.Join([]string{line.T, line.Msg, line.Tag, line.Level}, " ")))
	if text == "" {
		return false
	}
	return strings.Contains(text, "authentication failure") ||
		strings.Contains(text, "failed password") ||
		strings.Contains(text, "invalid user") ||
		strings.Contains(text, "too many authentication failures") ||
		strings.Contains(text, "maximum authentication attempts exceeded")
}

func estimateAuthFailureLimit(since time.Time) int {
	window := time.Since(since)
	if window <= 0 {
		return 300
	}
	// Heuristic: assume up to ~60 lines/minute for auth-related services.
	limit := int(window.Minutes()*60) + 50
	if limit < 300 {
		return 300
	}
	if limit > 5000 {
		return 5000
	}
	return limit
}

func syslogLineTime(line proxmoxclient.PVESyslogLine) (time.Time, bool) {
	if line.Time > 0 {
		sec := line.Time
		if sec > 946_684_800 {
			ms := sec
			if ms < 1_000_000_000_000 {
				ms *= 1000
			}
			return time.Unix(0, ms*int64(time.Millisecond)).UTC(), true
		}
	}

	if strings.TrimSpace(line.T) == "" {
		return time.Time{}, false
	}

	stamp, ok := extractSyslogTimestamp(line.T)
	if !ok {
		return time.Time{}, false
	}

	// Parse syslog timestamps as local time (logs use system timezone, which is local).
	parsed, err := time.ParseInLocation("Jan 2 15:04:05", stamp, time.Local)
	if err != nil {
		return time.Time{}, false
	}

	now := time.Now()
	// Reconstruct with current year in local time.
	parsed = time.Date(now.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.Local)
	if parsed.After(now.Add(24 * time.Hour)) {
		parsed = parsed.AddDate(-1, 0, 0)
	}
	return parsed, true
}

func extractSyslogTimestamp(text string) (string, bool) {
	// Expected prefix: "May 6 12:34:56"
	re := regexp.MustCompile(`^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+`)
	m := re.FindStringSubmatch(strings.TrimSpace(text))
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}
