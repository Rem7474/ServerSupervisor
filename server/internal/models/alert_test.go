package models

import "testing"

func TestAlertRuleDisplayName(t *testing.T) {
	name := "High CPU"
	crit := 90.0
	warn := 70.0

	tests := []struct {
		name string
		rule AlertRule
		want string
	}{
		{
			name: "custom name wins over thresholds",
			rule: AlertRule{Name: &name, Metric: "cpu", Operator: ">", ThresholdCrit: &crit},
			want: "High CPU",
		},
		{
			name: "falls back to crit threshold",
			rule: AlertRule{Metric: "cpu", Operator: ">", ThresholdCrit: &crit, ThresholdWarn: &warn},
			want: "cpu > 90.00 (crit)",
		},
		{
			name: "falls back to warn threshold when no crit",
			rule: AlertRule{Metric: "cpu", Operator: ">", ThresholdWarn: &warn},
			want: "cpu > 70.00 (warn)",
		},
		{
			name: "empty when the rule has no name and no threshold",
			rule: AlertRule{Metric: "cpu", Operator: ">"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rule.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
