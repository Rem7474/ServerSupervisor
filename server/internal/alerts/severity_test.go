package alerts

import (
	"testing"

	"github.com/serversupervisor/server/internal/models"
)

func fptr(f float64) *float64 { return &f }

// rule builds a minimal AlertRule for severity tests.
func rule(metric, op string, warn, crit, clearWarn, clearCrit *float64) models.AlertRule {
	return models.AlertRule{
		Metric:             metric,
		Operator:           op,
		ThresholdWarn:      warn,
		ThresholdCrit:      crit,
		ThresholdClearWarn: clearWarn,
		ThresholdClearCrit: clearCrit,
	}
}

func TestDetermineSeverity(t *testing.T) {
	tests := []struct {
		name  string
		rule  models.AlertRule
		host  models.Host
		value float64
		want  AlertSeverity
	}{
		{
			name: "status_offline crit when offline",
			rule: rule("status_offline", "", nil, nil, nil, nil),
			host: models.Host{Status: "offline"},
			want: SeverityCrit,
		},
		{
			name: "status_offline none when online",
			rule: rule("status_offline", "", nil, nil, nil, nil),
			host: models.Host{Status: "online"},
			want: SeverityNone,
		},
		{
			name:  "no thresholds -> none",
			rule:  rule("cpu", ">", nil, nil, nil, nil),
			value: 99,
			want:  SeverityNone,
		},
		{
			name:  "above crit -> crit (crit checked first)",
			rule:  rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value: 95,
			want:  SeverityCrit,
		},
		{
			name:  "between warn and crit -> warn",
			rule:  rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value: 85,
			want:  SeverityWarn,
		},
		{
			name:  "below warn -> none",
			rule:  rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value: 50,
			want:  SeverityNone,
		},
		{
			name:  "exactly at warn threshold with > operator -> none (strict)",
			rule:  rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value: 80,
			want:  SeverityNone,
		},
		{
			name:  "low-disk-free style with < operator -> crit",
			rule:  rule("disk_free", "<", fptr(20), fptr(10), nil, nil),
			value: 5,
			want:  SeverityCrit,
		},
		{
			name:  "only warn threshold set, above it -> warn",
			rule:  rule("cpu", ">=", fptr(80), nil, nil, nil),
			value: 80,
			want:  SeverityWarn,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineSeverity(tt.rule, tt.host, tt.value); got != tt.want {
				t.Errorf("DetermineSeverity() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMatchThreshold(t *testing.T) {
	tests := []struct {
		op        string
		value     float64
		threshold float64
		want      bool
	}{
		{">", 10, 5, true},
		{">", 5, 5, false},
		{"<", 3, 5, true},
		{"<", 5, 5, false},
		{">=", 5, 5, true},
		{"<=", 5, 5, true},
		{"==", 5, 5, false}, // unsupported operator
		{"", 5, 5, false},
	}
	for _, tt := range tests {
		if got := matchThreshold(tt.op, tt.value, tt.threshold); got != tt.want {
			t.Errorf("matchThreshold(%q, %v, %v) = %v, want %v", tt.op, tt.value, tt.threshold, got, tt.want)
		}
	}
}

func TestMatchRule(t *testing.T) {
	r := rule("cpu", ">", fptr(80), fptr(90), nil, nil)
	if !MatchRule(r, models.Host{}, 95) {
		t.Error("expected MatchRule true above crit")
	}
	if MatchRule(r, models.Host{}, 10) {
		t.Error("expected MatchRule false below warn")
	}
}

func TestShouldResolveAlertSeverity(t *testing.T) {
	tests := []struct {
		name            string
		rule            models.AlertRule
		host            models.Host
		value           float64
		currentSeverity AlertSeverity
		want            bool
	}{
		{
			name:            "status_offline resolves when back online",
			rule:            rule("status_offline", "", nil, nil, nil, nil),
			host:            models.Host{Status: "online"},
			currentSeverity: SeverityCrit,
			want:            true,
		},
		{
			name:            "status_offline stays open while offline",
			rule:            rule("status_offline", "", nil, nil, nil, nil),
			host:            models.Host{Status: "offline"},
			currentSeverity: SeverityCrit,
			want:            false,
		},
		{
			name:            "crit with hysteresis: value still above clear -> not resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), nil, fptr(85)),
			value:           88,
			currentSeverity: SeverityCrit,
			want:            false,
		},
		{
			name:            "crit with hysteresis: value dropped to/below clear -> resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), nil, fptr(85)),
			value:           85,
			currentSeverity: SeverityCrit,
			want:            true,
		},
		{
			name:            "crit without hysteresis: still crit -> not resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value:           95,
			currentSeverity: SeverityCrit,
			want:            false,
		},
		{
			name:            "crit without hysteresis: dropped to warn band -> resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value:           85,
			currentSeverity: SeverityCrit,
			want:            true,
		},
		{
			name:            "warn with hysteresis: above clear -> not resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), fptr(75), nil),
			value:           78,
			currentSeverity: SeverityWarn,
			want:            false,
		},
		{
			name:            "warn with hysteresis: at/below clear -> resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), fptr(75), nil),
			value:           75,
			currentSeverity: SeverityWarn,
			want:            true,
		},
		{
			name:            "warn without hysteresis: still warn -> not resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value:           85,
			currentSeverity: SeverityWarn,
			want:            false,
		},
		{
			name:            "warn without hysteresis: back to none -> resolved",
			rule:            rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value:           50,
			currentSeverity: SeverityWarn,
			want:            true,
		},
		{
			name:            "no current severity -> never resolves",
			rule:            rule("cpu", ">", fptr(80), fptr(90), nil, nil),
			value:           50,
			currentSeverity: SeverityNone,
			want:            false,
		},
		{
			name:            "low-disk crit hysteresis with < operator: recovered above clear -> resolved",
			rule:            rule("disk_free", "<", fptr(20), fptr(10), nil, fptr(15)),
			value:           15,
			currentSeverity: SeverityCrit,
			want:            true,
		},
		{
			name:            "low-disk crit hysteresis with < operator: still low -> not resolved",
			rule:            rule("disk_free", "<", fptr(20), fptr(10), nil, fptr(15)),
			value:           12,
			currentSeverity: SeverityCrit,
			want:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldResolveAlertSeverity(tt.rule, tt.host, tt.value, tt.currentSeverity)
			if got != tt.want {
				t.Errorf("ShouldResolveAlertSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvesHysteresis(t *testing.T) {
	tests := []struct {
		op    string
		value float64
		clear float64
		want  bool
	}{
		{">", 80, 85, true},  // value dropped to/below clear
		{">", 90, 85, false}, // still above clear
		{">=", 85, 85, true}, // at clear
		{"<", 20, 15, true},  // value rose to/above clear
		{"<", 12, 15, false}, // still below clear
		{"<=", 15, 15, true}, // at clear
		{"==", 1, 1, false},  // unsupported operator
	}
	for _, tt := range tests {
		if got := resolvesHysteresis(tt.op, tt.value, tt.clear); got != tt.want {
			t.Errorf("resolvesHysteresis(%q, %v, %v) = %v, want %v", tt.op, tt.value, tt.clear, got, tt.want)
		}
	}
}

func TestResolveThresholdForSeverity(t *testing.T) {
	r := rule("cpu", ">", fptr(80), fptr(90), fptr(75), fptr(85))

	if got := ResolveThresholdForSeverity(r, SeverityCrit); got == nil || *got != 85 {
		t.Errorf("crit with clear set: want 85, got %v", got)
	}
	if got := ResolveThresholdForSeverity(r, SeverityWarn); got == nil || *got != 75 {
		t.Errorf("warn with clear set: want 75, got %v", got)
	}

	// Without clear thresholds, falls back to the trigger thresholds.
	r2 := rule("cpu", ">", fptr(80), fptr(90), nil, nil)
	if got := ResolveThresholdForSeverity(r2, SeverityCrit); got == nil || *got != 90 {
		t.Errorf("crit without clear: want 90, got %v", got)
	}
	if got := ResolveThresholdForSeverity(r2, SeverityWarn); got == nil || *got != 80 {
		t.Errorf("warn without clear: want 80, got %v", got)
	}

	if got := ResolveThresholdForSeverity(r, SeverityNone); got != nil {
		t.Errorf("none severity: want nil, got %v", got)
	}
}
