package alerts

import "github.com/serversupervisor/server/internal/models"

// MatchRule evaluates whether a rule condition is currently met for the given value.
// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityNone AlertSeverity = ""
	SeverityWarn AlertSeverity = "warn"
	SeverityCrit AlertSeverity = "crit"
)

// DetermineSeverity returns the highest severity level (crit > warn > none) triggered by the rule
func DetermineSeverity(rule models.AlertRule, host models.Host, value float64) AlertSeverity {
	if rule.Metric == "status_offline" {
		if host.Status == "offline" {
			return SeverityCrit
		}
		return SeverityNone
	}

	if rule.ThresholdCrit == nil && rule.ThresholdWarn == nil {
		return SeverityNone
	}

	// Check critical threshold first
	if rule.ThresholdCrit != nil && matchThreshold(rule.Operator, value, *rule.ThresholdCrit) {
		return SeverityCrit
	}

	// Check warning threshold
	if rule.ThresholdWarn != nil && matchThreshold(rule.Operator, value, *rule.ThresholdWarn) {
		return SeverityWarn
	}

	return SeverityNone
}

// matchThreshold is a helper that checks if value matches operator condition against threshold
func matchThreshold(operator string, value float64, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	default:
		return false
	}
}

// MatchRule maintains backward compatibility - returns true if any severity is triggered
func MatchRule(rule models.AlertRule, host models.Host, value float64) bool {
	return DetermineSeverity(rule, host, value) != SeverityNone
}

// ShouldActivateAlert determines if an alert should be activated (incident created).
func ShouldActivateAlert(rule models.AlertRule, host models.Host, value float64) bool {
	return DetermineSeverity(rule, host, value) != SeverityNone
}

// ShouldResolveAlertSeverity determines if an open alert with given severity should be resolved.
// Uses hysteresis thresholds if available, otherwise uses lower severity as clearance condition.
func ShouldResolveAlertSeverity(rule models.AlertRule, host models.Host, value float64, currentSeverity AlertSeverity) bool {
	if rule.Metric == "status_offline" {
		return host.Status != "offline"
	}

	// Determine what severity level would be active at the current value
	activeSeverity := DetermineSeverity(rule, host, value)

	if currentSeverity == SeverityCrit {
		// For critical incidents:
		// If threshold_clear_crit is set, resolve when value crosses it
		if rule.ThresholdClearCrit != nil {
			return resolvesHysteresis(rule.Operator, value, *rule.ThresholdClearCrit)
		}
		// Otherwise resolve if we drop to warn or below
		return activeSeverity != SeverityCrit
	}

	if currentSeverity == SeverityWarn {
		// For warning incidents:
		// If threshold_clear_warn is set, resolve when value crosses it
		if rule.ThresholdClearWarn != nil {
			return resolvesHysteresis(rule.Operator, value, *rule.ThresholdClearWarn)
		}
		// Otherwise resolve if no severity is active
		return activeSeverity == SeverityNone
	}

	return false
}

// ResolveThresholdForSeverity returns the value the metric must cross for an
// open incident of the given severity to resolve: the hysteresis clear
// threshold when set, otherwise the trigger threshold. Returns nil when not
// applicable (e.g. status_offline, or missing thresholds).
func ResolveThresholdForSeverity(rule models.AlertRule, severity AlertSeverity) *float64 {
	switch severity {
	case SeverityCrit:
		if rule.ThresholdClearCrit != nil {
			return rule.ThresholdClearCrit
		}
		return rule.ThresholdCrit
	case SeverityWarn:
		if rule.ThresholdClearWarn != nil {
			return rule.ThresholdClearWarn
		}
		return rule.ThresholdWarn
	default:
		return nil
	}
}

// resolvesHysteresis checks if value has crossed the clear threshold based on operator
func resolvesHysteresis(operator string, value float64, clearThreshold float64) bool {
	switch operator {
	case ">", ">=":
		return value <= clearThreshold
	case "<", "<=":
		return value >= clearThreshold
	default:
		return false
	}
}
