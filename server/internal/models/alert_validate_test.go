package models

import (
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
)

func i18nKeyOf(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.I18nKey
	}
	return ""
}

// The rule form sends source_type "synthetic" for the uptime and SSL metrics.
func TestAlertRuleValidate_SyntheticSource(t *testing.T) {
	for _, metric := range []string{"uptime_down_count", "ssl_min_days_remaining"} {
		t.Run(metric, func(t *testing.T) {
			r := AlertRule{
				SourceType: AlertSourceSynthetic, Metric: metric,
				TargetType: AlertTargetHost, HostID: strPtr("web1"), TargetTags: []string{"x"},
				ProxmoxScope: &ProxmoxMetricScope{}, DockerScope: &DockerMetricScope{},
			}
			if err := r.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if r.TargetType != "" || r.HostID != nil || r.TargetTags != nil || r.ProxmoxScope != nil || r.DockerScope != nil {
				t.Errorf("synthetic rule kept agent/proxmox/docker fields: %+v", r)
			}
		})
	}

	r := AlertRule{SourceType: AlertSourceSynthetic, Metric: "cpu"}
	if got := i18nKeyOf(r.Validate()); got != apperr.CodeAlertSyntheticMetricRequired {
		t.Fatalf("synthetic source with cpu metric: code = %q", got)
	}
}

func TestAlertRuleValidate_TargetErrorCodes(t *testing.T) {
	tests := []struct {
		name string
		rule AlertRule
		want string
	}{
		{"host without id", AlertRule{Metric: "cpu", TargetType: AlertTargetHost}, apperr.CodeAlertTargetHostRequired},
		{"group without id", AlertRule{Metric: "cpu", TargetType: AlertTargetGroup}, apperr.CodeAlertTargetGroupRequired},
		{"tags without tag", AlertRule{Metric: "cpu", TargetType: AlertTargetTag}, apperr.CodeAlertTargetTagsRequired},
		{"unknown type", AlertRule{Metric: "cpu", TargetType: "everywhere"}, apperr.CodeAlertTargetTypeInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := i18nKeyOf(tc.rule.Validate()); got != tc.want {
				t.Fatalf("code = %q, want %q", got, tc.want)
			}
		})
	}
}
