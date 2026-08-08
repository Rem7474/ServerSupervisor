package alerts

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/services/notifychannels"
	"github.com/serversupervisor/server/internal/services/push"
)

// NotificationPusher broadcasts a real-time alert event to connected frontend clients.
// The api.NotificationHub implements this interface; pass nil to skip push.
type NotificationPusher interface {
	Broadcast(payload interface{})
}

// CurrentIncidentValue returns the live metric value for an open incident,
// reconstructing the evaluation target the same way the engine does (real agent
// host, or synthetic Proxmox/synthetic target keyed by the incident's host_id).
func CurrentIncidentValue(ctx context.Context, db *database.DB, rule models.AlertRule, hostID string) (float64, bool) {
	var host models.Host
	if strings.HasPrefix(hostID, "proxmox:") || strings.HasPrefix(hostID, "synthetic:") ||
		strings.HasPrefix(hostID, "docker:") {
		host = models.Host{ID: hostID, Status: "online", LastSeen: time.Now()}
	} else {
		h, err := db.GetHost(ctx, hostID)
		if err != nil || h == nil {
			return 0, false
		}
		host = *h
	}
	return GetMetricValue(ctx, db, host, rule)
}

// ResolveStaleIncidentsForRule immediately resolves any open incidents for a rule
// that are no longer active under its current thresholds. Called after a rule
// update so stuck incidents don't wait for the next engine tick.
//
// Unlike the periodic engine, this function fetches the actual current metric
// value (not inc.Value, which is stale once the value drops below the trigger
// and the engine stops updating it).
func ResolveStaleIncidentsForRule(ctx context.Context, db *database.DB, cfg *config.Config, pusher NotificationPusher, pushSvc *push.Service, rule models.AlertRule) {
	incidents, err := db.ListOpenAlertIncidentsByRule(ctx, rule.ID)
	if err != nil || len(incidents) == 0 {
		return
	}

	chDispatch := notifychannels.NewDispatcher(cfg, pushSvc)

	// Build a host map so status_offline can read the real host status.
	hostMap := map[string]models.Host{}
	if hosts, err := db.GetAllHosts(ctx); err == nil {
		for _, h := range hosts {
			hostMap[h.ID] = h
		}
	}

	for _, inc := range incidents {
		host, ok := hostMap[inc.HostID]
		if !ok {
			// Synthetic host (Proxmox / synthetic metrics) — construct a minimal record.
			host = models.Host{ID: inc.HostID, Status: "online", LastSeen: time.Now()}
		}

		// Fetch the actual current metric value, same as the engine does.
		currentValue, ok := GetMetricValue(ctx, db, host, rule)
		if !ok {
			continue
		}

		// Still firing with the new thresholds → leave open.
		if DetermineSeverity(rule, host, currentValue) != SeverityNone {
			continue
		}
		// Resolution conditions met → close immediately.
		if ShouldResolveAlertSeverity(rule, host, currentValue, AlertSeverity(inc.Severity)) {
			if err := db.ResolveAlertIncident(ctx, inc.ID); err == nil {
				slog.InfoContext(ctx, "alerts: stale incident resolved after rule update",
					slog.Int64("incident_id", inc.ID), slog.Int64("rule_id", rule.ID))
				broadcastIncidentUpdate(pusher, "resolved", rule, host.ID)
				chDispatch.Send(ctx, resolvedEvent(rule, host, inc))
			}
		}
	}
}

func EvaluateAlerts(ctx context.Context, db *database.DB, cfg *config.Config, dispatcher *dispatch.Dispatcher, pusher NotificationPusher, pushSvc *push.Service) {
	rules, err := db.GetAlertRules(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "alerts: failed to fetch rules", slog.Any("err", err))
		return
	}
	if len(rules) == 0 {
		return
	}

	hosts, err := db.GetAllHosts(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "alerts: failed to fetch hosts", slog.Any("err", err))
		return
	}

	chDispatch := notifychannels.NewDispatcher(cfg, pushSvc)

	hostByID := make(map[string]models.Host, len(hosts))
	for _, h := range hosts {
		hostByID[h.ID] = h
	}

	for _, rule := range rules {
		if !rule.Enabled {
			staleIncidents, _ := db.ListOpenAlertIncidentsByRule(ctx, rule.ID)
			resolvedCount, err := db.ResolveOpenAlertIncidentsByRule(ctx, rule.ID)
			if err != nil {
				slog.ErrorContext(ctx, "alerts: failed to resolve open incidents for disabled rule", slog.Int64("rule_id", rule.ID), slog.Any("err", err))
			} else if resolvedCount > 0 {
				slog.InfoContext(ctx, "alerts: disabled rule resolved open incidents", slog.Int64("rule_id", rule.ID), slog.Int64("resolved", resolvedCount))
				broadcastIncidentUpdate(pusher, "resolved", rule, "")
				for _, inc := range staleIncidents {
					host, ok := hostByID[inc.HostID]
					if !ok {
						host = models.Host{ID: inc.HostID, Name: inc.HostID}
					}
					chDispatch.Send(ctx, resolvedEvent(rule, host, inc))
				}
			}
			continue
		}

		ruleName := fmt.Sprintf("rule#%d(%s %s)", rule.ID, rule.Metric, rule.Operator)
		if rule.Name != nil && *rule.Name != "" {
			ruleName = fmt.Sprintf("rule#%d(%s)", rule.ID, *rule.Name)
		}

		// Build evaluation targets:
		// - agent metrics: real hosts
		// - Proxmox metrics: synthetic targets (scope-based or per-entity for global rules)
		hostsForRule := buildAlertEvaluationTargets(ctx, db, rule, hosts)
		evaluatedTargets := make(map[string]struct{}, len(hostsForRule))

		for _, host := range hostsForRule {
			evaluatedTargets[host.ID] = struct{}{}
			if hasHostID(rule) && !isProxmoxMetric(rule.Metric) && *rule.HostID != host.ID {
				continue
			}

			if inMaintenance, err := db.IsHostInMaintenance(ctx, host.ID); err != nil {
				slog.ErrorContext(ctx, "alerts: failed to check maintenance window", slog.String("host", host.ID), slog.Any("err", err))
			} else if inMaintenance {
				// Same shape as the disabled-rule branch above: silently resolve
				// any already-open incident (UI refresh ping only, no loud
				// notification channel) and skip evaluation entirely, so a
				// planned intervention produces zero alert noise in either
				// direction — no new incidents, no "resolved" emails either.
				if inc, err := db.GetOpenAlertIncident(ctx, rule.ID, host.ID); err == nil && inc != nil {
					if err := db.ResolveAlertIncident(ctx, inc.ID); err != nil {
						slog.ErrorContext(ctx, "alerts: failed to resolve incident for host in maintenance", slog.Int64("incident_id", inc.ID), slog.Any("err", err))
					} else {
						slog.InfoContext(ctx, "alerts: incident silently resolved — host in maintenance", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Int64("incident_id", inc.ID))
						broadcastIncidentUpdate(pusher, "resolved", rule, host.ID)
					}
				}
				continue
			}

			value, ok := GetMetricValue(ctx, db, host, rule)
			if !ok {
				continue
			}

			// Determine current severity based on rule and value
			currentSeveration := DetermineSeverity(rule, host, value)

			// Get any open incident (regardless of severity)
			inc, err := db.GetOpenAlertIncident(ctx, rule.ID, host.ID)
			if err != nil && err != sql.ErrNoRows {
				slog.ErrorContext(ctx, "alerts: failed to check incidents", slog.Any("err", err))
				continue
			}

			if currentSeveration != SeverityNone {
				// Alert is triggered at current severity level
				if err == sql.ErrNoRows || inc == nil {
					// No existing incident - create new one with current severity
					incID, err := db.CreateAlertIncident(ctx, rule.ID, host.ID, value, string(currentSeveration))
					if err != nil {
						slog.ErrorContext(ctx, "alerts: failed to create incident", slog.Any("err", err))
						continue
					}
					slog.InfoContext(ctx, "alerts: incident FIRED", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Float64("value", value), slog.String("severity", string(currentSeveration)), slog.Int64("incident_id", incID))
					details := fmt.Sprintf(`{"rule_id":%d,"metric":"%s","operator":"%s","value":%.4f,"severity":"%s"}`, rule.ID, rule.Metric, rule.Operator, value, currentSeveration)
					if _, auditErr := db.CreateAuditLog(ctx, "alert-engine", "alert_fired", host.ID, "", details, "success"); auditErr != nil {
						slog.WarnContext(ctx, "alerts: failed to write alert_fired audit log", slog.Int64("incident_id", incID), slog.Any("err", auditErr))
					}
					// The incident list refresh ping always goes out — it's a lightweight
					// UI signal, not a notification channel — but the loud channels
					// (smtp/ntfy/push/browser toast) and any command_trigger are subject
					// to AlertActions.Cooldown, so a flapping rule can't spam either.
					broadcastIncidentUpdate(pusher, "fired", rule, host.ID)

					// A host-down cascade (e.g. every Docker container on that host
					// firing its own incident at once) shouldn't send an independent
					// notification per child — link it to the host's own open
					// status_offline/heartbeat_timeout incident and skip straight to
					// the next target. The incident itself is still recorded (and
					// still visible, grouped, in the UI) — only the loud notification
					// and command_trigger are suppressed, since the real cause is
					// "host is down," not this rule's own condition.
					if correlatedWith := correlationTargetIncidentID(ctx, db, rule, host.ID); correlatedWith != nil {
						if err := db.SetAlertIncidentCorrelation(ctx, incID, *correlatedWith); err != nil {
							slog.ErrorContext(ctx, "alerts: failed to set incident correlation", slog.Int64("incident_id", incID), slog.Any("err", err))
						} else {
							slog.InfoContext(ctx, "alerts: incident correlated with host-down, notification suppressed", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Int64("incident_id", incID), slog.Int64("correlated_with", *correlatedWith))
						}
						continue
					}

					now := time.Now()
					cooldown := time.Duration(rule.Actions.Cooldown) * time.Second
					if cooldown > 0 && rule.LastFired != nil && now.Sub(*rule.LastFired) < cooldown {
						slog.InfoContext(ctx, "alerts: notification/command_trigger suppressed by cooldown", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Int64("incident_id", incID), slog.Duration("cooldown", cooldown))
					} else {
						if err := db.UpdateAlertRuleLastFired(ctx, rule.ID, now); err != nil {
							slog.WarnContext(ctx, "alerts: failed to stamp rule last_fired", slog.Int64("rule_id", rule.ID), slog.Any("err", err))
						}
						if cmdID := triggerAlertCommand(ctx, dispatcher, db, rule, host); cmdID != nil {
							if err := db.UpdateAlertIncidentCommandID(ctx, incID, *cmdID); err != nil {
								slog.WarnContext(ctx, "alerts: failed to link command to incident", slog.Int64("incident_id", incID), slog.Any("err", err))
							}
						}
						ev := firedEvent(cfg, rule, host, value)
						ev.OnBrowser = newAlertBroadcast(pusher, rule, host, value, incID)
						chDispatch.Send(ctx, ev)
					}
				} else {
					// Keep incident context fresh so UI and resolution logic use current severity/value.
					severityChanged := AlertSeverity(inc.Severity) != currentSeveration
					valueChanged := inc.Value != value
					hostChanged := inc.HostID != host.ID
					if severityChanged || valueChanged || hostChanged {
						if err := db.UpdateAlertIncidentContext(ctx, inc.ID, host.ID, value, string(currentSeveration)); err != nil {
							slog.ErrorContext(ctx, "alerts: failed to update incident context", slog.Int64("incident_id", inc.ID), slog.Any("err", err))
						} else if severityChanged {
							slog.InfoContext(ctx, "alerts: incident UPDATED", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Float64("value", value), slog.String("severity_from", inc.Severity), slog.String("severity_to", string(currentSeveration)), slog.Int64("incident_id", inc.ID))
						}
					}
					maybeEscalateIncident(ctx, db, chDispatch, pusher, cfg, rule, host, value, ruleName, *inc)
				}
			} else if inc != nil {
				// No alert triggered - resolve if one exists
				if ShouldResolveAlertSeverity(rule, host, value, AlertSeverity(inc.Severity)) {
					if err := db.ResolveAlertIncident(ctx, inc.ID); err != nil {
						slog.ErrorContext(ctx, "alerts: failed to resolve incident", slog.Int64("incident_id", inc.ID), slog.Any("err", err))
						continue
					}
					slog.InfoContext(ctx, "alerts: incident resolved", slog.String("rule", ruleName), slog.String("host", host.Name), slog.String("severity", inc.Severity), slog.Int64("incident_id", inc.ID))
					details := fmt.Sprintf(`{"rule_id":%d,"incident_id":%d,"severity":"%s"}`, rule.ID, inc.ID, inc.Severity)
					if _, auditErr := db.CreateAuditLog(ctx, "alert-engine", "alert_resolved", host.ID, "", details, "success"); auditErr != nil {
						slog.WarnContext(ctx, "alerts: failed to write alert_resolved audit log", slog.Int64("incident_id", inc.ID), slog.Any("err", auditErr))
					}
					broadcastIncidentUpdate(pusher, "resolved", rule, host.ID)
					chDispatch.Send(ctx, resolvedEvent(rule, host, *inc))
				}
			}
		}

		if isProxmoxGlobalScope(rule) {
			resolveStaleGlobalProxmoxIncidents(ctx, db, chDispatch, pusher, rule, evaluatedTargets)
		}
	}
}

// isHostDownMetric identifies the two "is this host reachable at all" metrics
// — the root-cause signal correlationTargetIncidentID looks for. A rule using
// one of these never gets correlated with another incident: it IS the root
// cause a cascade correlates against, not a symptom of one.
func isHostDownMetric(metric string) bool {
	return metric == "status_offline" || metric == "heartbeat_timeout"
}

// correlationTargetIncidentID checks whether the target this incident just
// fired on belongs to a host that's currently down, and if so returns that
// host's own open status_offline/heartbeat_timeout incident id to correlate
// against (see the comment at its call site in EvaluateAlerts). Returns nil
// when the rule is itself a host-down rule, when the target can't be resolved
// to a real host (Proxmox non-guest scopes, synthetic probes — same
// unresolvable-target trade-off as resolvableHostID in
// internal/handlers/host_authz.go), or when that host isn't currently down.
func correlationTargetIncidentID(ctx context.Context, db *database.DB, rule models.AlertRule, targetID string) *int64 {
	if isHostDownMetric(rule.Metric) {
		return nil
	}
	realHostID, ok := correlationHostID(ctx, db, targetID)
	if !ok {
		return nil
	}
	incID, err := db.GetOpenHostDownIncidentID(ctx, realHostID)
	if err != nil {
		slog.ErrorContext(ctx, "alerts: failed to check host-down correlation", slog.String("host", realHostID), slog.Any("err", err))
		return nil
	}
	return incID
}

// correlationHostID resolves an evaluation target ID to the real agent host
// it should be correlated against. Mirrors triggerAlertCommand's synthetic-ID
// resolution (docker:container:/docker:compose:/proxmox:guest:) since a
// host-down cascade shows up on exactly those target shapes; a bare agent
// host ID resolves to itself. Proxmox non-guest scopes and synthetic probes
// have no single owning host and return ok=false, same as triggerAlertCommand.
func correlationHostID(ctx context.Context, db *database.DB, targetID string) (hostID string, ok bool) {
	switch {
	case strings.HasPrefix(targetID, "docker:container:"):
		uuid := strings.TrimPrefix(targetID, "docker:container:")
		c, err := db.GetDockerContainerByID(ctx, uuid)
		if err != nil || c == nil {
			return "", false
		}
		return c.HostID, true
	case strings.HasPrefix(targetID, "docker:compose:"):
		composeHostID, _, composeOK := parseDockerComposeScopeID(targetID)
		return composeHostID, composeOK
	case strings.HasPrefix(targetID, "proxmox:"):
		parts := strings.SplitN(targetID, ":", 3)
		if len(parts) != 3 || parts[1] != "guest" || parts[2] == "" {
			return "", false
		}
		link, err := db.GetProxmoxGuestLinkByGuest(ctx, parts[2])
		if err != nil || link == nil || link.Status != "confirmed" {
			return "", false
		}
		return link.HostID, true
	case strings.HasPrefix(targetID, "synthetic:"):
		return "", false
	default:
		return targetID, true
	}
}

// maybeEscalateIncident re-sends the fired notification for an already-open
// incident that hasn't been acknowledged, once AlertActions.EscalateAfterMinutes
// have elapsed since it last notified (its trigger time, or its last
// escalation) — an unacknowledged critical incident staying silent between
// the initial fire and eventual resolution is the gap this closes (ROADMAP.md
// item #3). Acknowledging the incident (AcknowledgeIncident) stops it, same
// as resolving it does. Unlike the initial fire, this never re-dispatches
// CommandTrigger — repeating a remediation command every N minutes on a
// timer is a materially different (and riskier) action than repeating a
// notification, and isn't what "escalation" here means.
func maybeEscalateIncident(ctx context.Context, db *database.DB, chDispatch *notifychannels.Dispatcher, pusher NotificationPusher, cfg *config.Config, rule models.AlertRule, host models.Host, value float64, ruleName string, inc models.AlertIncident) {
	escalateAfter := rule.Actions.EscalateAfterMinutes
	// A correlated incident (host-down cascade child, see
	// correlationTargetIncidentID) never independently escalates either —
	// same reasoning as suppressing its initial notification: the real cause
	// is the host-down incident, which handles its own escalation.
	if escalateAfter <= 0 || inc.AcknowledgedAt != nil || inc.CorrelatedWith != nil {
		return
	}
	since := inc.TriggeredAt
	if inc.LastEscalatedAt != nil {
		since = *inc.LastEscalatedAt
	}
	now := time.Now()
	if now.Sub(since) < time.Duration(escalateAfter)*time.Minute {
		return
	}
	if err := db.UpdateAlertIncidentLastEscalated(ctx, inc.ID, now); err != nil {
		slog.ErrorContext(ctx, "alerts: failed to stamp incident escalation", slog.Int64("incident_id", inc.ID), slog.Any("err", err))
		return
	}
	slog.InfoContext(ctx, "alerts: incident ESCALATED", slog.String("rule", ruleName), slog.String("host", host.Name), slog.Int64("incident_id", inc.ID), slog.Int("escalate_after_minutes", escalateAfter))
	details := fmt.Sprintf(`{"rule_id":%d,"incident_id":%d,"severity":"%s"}`, rule.ID, inc.ID, inc.Severity)
	if _, auditErr := db.CreateAuditLog(ctx, "alert-engine", "alert_escalated", host.ID, "", details, "success"); auditErr != nil {
		slog.WarnContext(ctx, "alerts: failed to write alert_escalated audit log", slog.Int64("incident_id", inc.ID), slog.Any("err", auditErr))
	}
	broadcastIncidentUpdate(pusher, "fired", rule, host.ID)
	ev := firedEvent(cfg, rule, host, value)
	ev.OnBrowser = newAlertBroadcast(pusher, rule, host, value, inc.ID)
	chDispatch.Send(ctx, ev)
}

func isProxmoxGlobalScope(rule models.AlertRule) bool {
	if !isProxmoxMetric(rule.Metric) {
		return false
	}
	scope := proxmoxScopeFromRule(rule)
	return scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global"
}

// isProxmoxMetric detects if a metric belongs to the Proxmox subsystem.
func isProxmoxMetric(metric string) bool {
	switch metric {
	case "proxmox_storage_percent",
		"proxmox_node_cpu_percent",
		"proxmox_node_memory_percent",
		"proxmox_node_cpu_temperature",
		"proxmox_node_fan_rpm",
		"proxmox_guest_cpu_percent",
		"proxmox_guest_memory_percent",
		"proxmox_node_pending_updates",
		"proxmox_recent_failed_tasks_24h",
		"proxmox_auth_failures_recent",
		"proxmox_disk_failed_count",
		"proxmox_disk_min_wearout_percent":
		return true
	default:
		return false
	}
}

// isSyntheticMetric detects if a metric belongs to the synthetic monitoring
// subsystem (uptime probes, SSL certificates). These metrics are global and
// evaluated once per rule, not per host.
func isSyntheticMetric(metric string) bool {
	switch metric {
	case "uptime_down_count", "ssl_min_days_remaining":
		return true
	default:
		return false
	}
}

// hasHostID checks if a rule explicitly filters by host ID.
func hasHostID(rule models.AlertRule) bool {
	return rule.HostID != nil && *rule.HostID != ""
}

func proxmoxScopeFromRule(rule models.AlertRule) *models.ProxmoxMetricScope {
	return rule.ProxmoxScope
}

// proxmoxScopeKey generates a unique identifier for a Proxmox alert incident
// based on scope mode to avoid duplicate incidents per scope.
func proxmoxScopeKey(scope *models.ProxmoxMetricScope) string {
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return "proxmox:global"
	}
	switch scope.ScopeMode {
	case "connection":
		if scope.ConnectionID != "" {
			return fmt.Sprintf("proxmox:connection:%s", scope.ConnectionID)
		}
	case "node":
		if scope.NodeID != "" {
			return fmt.Sprintf("proxmox:node:%s", scope.NodeID)
		}
	case "storage":
		if scope.StorageID != "" {
			return fmt.Sprintf("proxmox:storage:%s", scope.StorageID)
		}
	case "guest":
		if scope.GuestID != "" {
			return fmt.Sprintf("proxmox:guest:%s", scope.GuestID)
		}
	case "disk":
		if scope.DiskID != "" {
			return fmt.Sprintf("proxmox:disk:%s", scope.DiskID)
		}
	}
	return "proxmox:global"
}

// proxmoxScopeLabel formats a human-readable description of a Proxmox scope for incident messages.
func proxmoxScopeLabel(scope *models.ProxmoxMetricScope) string {
	if scope == nil || scope.ScopeMode == "" || scope.ScopeMode == "global" {
		return "Proxmox global"
	}
	switch scope.ScopeMode {
	case "connection":
		return fmt.Sprintf("Proxmox connexion %s", scope.ConnectionID)
	case "node":
		return fmt.Sprintf("Proxmox noeud %s", scope.NodeID)
	case "storage":
		return fmt.Sprintf("Proxmox stockage %s", scope.StorageID)
	case "guest":
		return fmt.Sprintf("Proxmox VM/LXC %s", scope.GuestID)
	case "disk":
		return fmt.Sprintf("Proxmox disque %s", scope.DiskID)
	}
	return "Proxmox global"
}

// buildAlertEvaluationTargets creates the list of hosts/targets to evaluate for a rule.
// For agent metrics, returns the provided hosts. For Proxmox metrics, returns a single
// synthetic host record with ID from proxmoxScopeKey() to deduplicate incidents per scope.
// For Docker metrics, returns synthetic targets per container or per host aggregate.
func buildAlertEvaluationTargets(ctx context.Context, db *database.DB, rule models.AlertRule, hosts []models.Host) []models.Host {
	if isSyntheticMetric(rule.Metric) {
		// Synthetic metrics are global — evaluate once with a single synthetic target so
		// the engine creates exactly one incident per rule on fire.
		return []models.Host{{
			ID:       "synthetic:" + rule.Metric,
			Name:     "Monitoring synthétique",
			Status:   "online",
			LastSeen: time.Now(),
		}}
	}
	if isDockerMetric(rule.Metric) {
		return buildDockerEvaluationTargets(ctx, db, rule)
	}
	if !isProxmoxMetric(rule.Metric) {
		// For agent metrics, filter by HostID if set
		if hasHostID(rule) {
			for _, h := range hosts {
				if h.ID == *rule.HostID {
					return []models.Host{h}
				}
			}
			return []models.Host{}
		}
		return hosts
	}

	if isProxmoxGlobalScope(rule) {
		if targets := buildGlobalProxmoxEntityTargets(ctx, db, rule); len(targets) > 0 {
			return targets
		}
	}

	// For Proxmox metrics, create a synthetic host record with scope-based ID
	// This ensures one incident per Proxmox scope, not per agent host
	syntheticID := proxmoxScopeKey(proxmoxScopeFromRule(rule))
	syntheticLabel := proxmoxScopeLabel(proxmoxScopeFromRule(rule))
	return []models.Host{
		{
			ID:       syntheticID,
			Name:     syntheticLabel,
			Status:   "online",
			LastSeen: time.Now(),
		},
	}
}

func isDockerMetric(metric string) bool {
	return models.IsDockerMetric(metric)
}

// BuildDockerTestTargets is the exported entry point for the test-run handler.
func BuildDockerTestTargets(ctx context.Context, db *database.DB, rule models.AlertRule) []models.Host {
	return buildDockerEvaluationTargets(ctx, db, rule)
}

// buildDockerEvaluationTargets returns synthetic targets for Docker metrics.
// For docker_container_not_running with scope=host: one target per container on the host.
// For docker_container_not_running with scope=container: one target for the specific container.
// For docker_container_running_count: one aggregate target for the host.
func buildDockerEvaluationTargets(ctx context.Context, db *database.DB, rule models.AlertRule) []models.Host {
	scope := rule.DockerScope
	if scope == nil || scope.HostID == "" {
		return nil
	}

	switch rule.Metric {
	case "docker_container_state":
		switch scope.ScopeMode {
		case "host":
			containers, err := db.ListDockerContainersForAlerts(ctx, scope.HostID)
			if err != nil {
				return nil
			}
			targets := make([]models.Host, 0, len(containers))
			for _, c := range containers {
				targets = append(targets, models.Host{
					ID:       "docker:container:" + c.ID,
					Name:     c.Name + " (" + c.Image + ":" + c.ImageTag + ")",
					Status:   "online",
					LastSeen: time.Now(),
				})
			}
			return targets
		case "container":
			ids := scope.EffectiveContainerIDs()
			targets := make([]models.Host, 0, len(ids))
			for _, id := range ids {
				c, err := db.GetDockerContainerByID(ctx, id)
				if err != nil || c == nil {
					continue
				}
				targets = append(targets, models.Host{
					ID:       "docker:container:" + c.ID,
					Name:     c.Name + " (" + c.Image + ":" + c.ImageTag + ")",
					Status:   "online",
					LastSeen: time.Now(),
				})
			}
			return targets
		}
	case "docker_compose_degraded_services":
		return []models.Host{{
			ID:       "docker:compose:" + scope.HostID + ":" + scope.ProjectName,
			Name:     "Compose " + scope.ProjectName,
			Status:   "online",
			LastSeen: time.Now(),
		}}
	}
	return nil
}

func buildGlobalProxmoxEntityTargets(ctx context.Context, db *database.DB, rule models.AlertRule) []models.Host {
	switch rule.Metric {
	case "proxmox_node_cpu_percent", "proxmox_node_memory_percent", "proxmox_node_cpu_temperature", "proxmox_node_fan_rpm", "proxmox_node_pending_updates", "proxmox_recent_failed_tasks_24h", "proxmox_auth_failures_recent":
		nodes, err := db.ListProxmoxNodes(ctx)
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0, len(nodes))
		for _, n := range nodes {
			targets = append(targets, models.Host{
				ID:       "proxmox:node:" + n.ID,
				Name:     fmt.Sprintf("Proxmox noeud %s", n.NodeName),
				Status:   "online",
				LastSeen: time.Now(),
			})
		}
		return targets
	case "proxmox_guest_cpu_percent", "proxmox_guest_memory_percent":
		guests, err := db.ListProxmoxGuests(ctx, "", "", "")
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0, len(guests))
		for _, g := range guests {
			name := g.Name
			if strings.TrimSpace(name) == "" {
				name = fmt.Sprintf("VM/LXC %d", g.VMID)
			}
			targets = append(targets, models.Host{
				ID:       "proxmox:guest:" + g.ID,
				Name:     fmt.Sprintf("Proxmox VM/LXC %s", name),
				Status:   "online",
				LastSeen: time.Now(),
			})
		}
		return targets
	case "proxmox_storage_percent":
		nodes, err := db.ListProxmoxNodes(ctx)
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0)
		for _, n := range nodes {
			storages, err := db.ListProxmoxStoragesByNode(ctx, n.ConnectionID, n.NodeName)
			if err != nil {
				continue
			}
			for _, s := range storages {
				targets = append(targets, models.Host{
					ID:       "proxmox:storage:" + s.ID,
					Name:     fmt.Sprintf("Proxmox stockage %s", s.StorageName),
					Status:   "online",
					LastSeen: time.Now(),
				})
			}
		}
		return targets
	case "proxmox_disk_failed_count", "proxmox_disk_min_wearout_percent":
		nodes, err := db.ListProxmoxNodes(ctx)
		if err != nil {
			return nil
		}
		targets := make([]models.Host, 0)
		for _, n := range nodes {
			disks, err := db.ListProxmoxDisksByNode(ctx, n.ConnectionID, n.NodeName)
			if err != nil {
				continue
			}
			for _, d := range disks {
				targets = append(targets, models.Host{
					ID:       "proxmox:disk:" + d.ID,
					Name:     fmt.Sprintf("Proxmox disque %s", d.DevPath),
					Status:   "online",
					LastSeen: time.Now(),
				})
			}
		}
		return targets
	default:
		return nil
	}
}

func proxmoxScopedRuleForSyntheticTarget(rule models.AlertRule, targetID string) (models.AlertRule, bool) {
	if !isProxmoxGlobalScope(rule) {
		return rule, false
	}

	parts := strings.SplitN(targetID, ":", 3)
	if len(parts) != 3 || parts[0] != "proxmox" || parts[2] == "" {
		return rule, false
	}

	scoped := rule
	scope := &models.ProxmoxMetricScope{}
	entityType := parts[1]
	entityID := parts[2]

	switch entityType {
	case "node":
		scope.ScopeMode = "node"
		scope.NodeID = entityID
	case "guest":
		scope.ScopeMode = "guest"
		scope.GuestID = entityID
	case "storage":
		scope.ScopeMode = "storage"
		scope.StorageID = entityID
	case "disk":
		scope.ScopeMode = "disk"
		scope.DiskID = entityID
	default:
		return rule, false
	}

	scoped.ProxmoxScope = scope
	return scoped, true
}

func resolveStaleGlobalProxmoxIncidents(ctx context.Context, db *database.DB, chDispatch *notifychannels.Dispatcher, pusher NotificationPusher, rule models.AlertRule, evaluatedTargets map[string]struct{}) {
	openIncidents, err := db.ListOpenAlertIncidentsByRule(ctx, rule.ID)
	if err != nil {
		slog.ErrorContext(ctx, "alerts: failed to list open incidents for stale cleanup", slog.Int64("rule_id", rule.ID), slog.Any("err", err))
		return
	}

	for _, inc := range openIncidents {
		if !strings.HasPrefix(inc.HostID, "proxmox:") {
			continue
		}
		if _, ok := evaluatedTargets[inc.HostID]; ok {
			continue
		}
		if err := db.ResolveAlertIncident(ctx, inc.ID); err != nil {
			slog.ErrorContext(ctx, "alerts: failed to resolve stale incident", slog.Int64("incident_id", inc.ID), slog.Int64("rule_id", rule.ID), slog.Any("err", err))
			continue
		}
		broadcastIncidentUpdate(pusher, "resolved", rule, inc.HostID)
		chDispatch.Send(ctx, resolvedEvent(rule, models.Host{ID: inc.HostID, Name: inc.HostID}, inc))
	}
}
