package alertrule

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/models"
)

// i18nKey returns the catalog key an error carries, or "" if it carries none.
func i18nKey(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.I18nKey
	}
	return ""
}

// Every user-facing validation failure has to carry a catalog key, otherwise
// respondError falls back to the literal English message regardless of the
// caller's language. These assert the key, not the wording, so rephrasing a
// message doesn't break them — but dropping the .I18n() call does.

func TestValidateDockerScope_CarriesI18nCodes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		repo  *fakeRepo
		scope *models.DockerMetricScope
		want  string
	}{
		{
			name: "nil scope",
			repo: &fakeRepo{hostExists: true},
			want: apperr.CodeAlertDockerScopeRequired,
		},
		{
			name:  "unknown host",
			repo:  &fakeRepo{hostExists: false},
			scope: &models.DockerMetricScope{HostID: "h1"},
			want:  apperr.CodeAlertDockerHostNotFound,
		},
		{
			name:  "unknown container",
			repo:  &fakeRepo{hostExists: true, missingContainerID: "c1"},
			scope: &models.DockerMetricScope{HostID: "h1", ScopeMode: "container", ContainerIDs: []string{"c1"}},
			want:  apperr.CodeAlertDockerContainerNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := newSvc(tc.repo).ValidateDockerScope(ctx, tc.scope)
			if got := i18nKey(err); got != tc.want {
				t.Errorf("i18n key = %q, want %q (err: %v)", got, tc.want, err)
			}
			if status(err) != 400 {
				t.Errorf("status = %d, want 400", status(err))
			}
		})
	}
}

func TestValidateProxmoxScope_CarriesI18nCodes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		kind  string
		scope *models.ProxmoxMetricScope
		want  string
	}{
		{name: "nil scope", want: apperr.CodeAlertProxmoxScopeRequired},
		{name: "connection", kind: "connection", scope: &models.ProxmoxMetricScope{ScopeMode: "connection"}, want: apperr.CodeAlertProxmoxConnNotFound},
		{name: "node", kind: "node", scope: &models.ProxmoxMetricScope{ScopeMode: "node"}, want: apperr.CodeAlertProxmoxNodeNotFound},
		{name: "storage", kind: "storage", scope: &models.ProxmoxMetricScope{ScopeMode: "storage"}, want: apperr.CodeAlertProxmoxStorageNotFound},
		{name: "guest", kind: "guest", scope: &models.ProxmoxMetricScope{ScopeMode: "guest"}, want: apperr.CodeAlertProxmoxGuestNotFound},
		{name: "disk", kind: "disk", scope: &models.ProxmoxMetricScope{ScopeMode: "disk"}, want: apperr.CodeAlertProxmoxDiskNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newSvc(&fakeRepo{hostExists: true, missingProxmoxKind: tc.kind})
			err := svc.ValidateProxmoxScope(ctx, tc.scope)
			if got := i18nKey(err); got != tc.want {
				t.Errorf("i18n key = %q, want %q (err: %v)", got, tc.want, err)
			}
		})
	}
}

func TestValidateProxmoxScope_AcceptsAnExistingTarget(t *testing.T) {
	svc := newSvc(&fakeRepo{hostExists: true})
	for _, mode := range []string{"connection", "node", "storage", "guest", "disk"} {
		scope := &models.ProxmoxMetricScope{ScopeMode: mode}
		if err := svc.ValidateProxmoxScope(context.Background(), scope); err != nil {
			t.Errorf("mode %s: unexpected error %v", mode, err)
		}
	}
}

func TestValidateBaselineWindow(t *testing.T) {
	window := func(v int) *int { return &v }

	tests := []struct {
		name   string
		metric string
		value  *int
		want   string
	}{
		{name: "nil is always valid", metric: "cpu", value: nil, want: ""},
		{name: "valid preset on the right metric", metric: "bandwidth_vs_rolling_avg", value: window(3600), want: ""},
		{name: "off-preset value", metric: "bandwidth_vs_rolling_avg", value: window(1800), want: apperr.CodeAlertRollingWindowInvalid},
		{name: "set on a metric that ignores it", metric: "cpu", value: window(3600), want: apperr.CodeAlertRollingWindowNotAllowed},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBaselineWindow(tc.metric, tc.value)
			if got := i18nKey(err); got != tc.want {
				t.Errorf("i18n key = %q, want %q (err: %v)", got, tc.want, err)
			}
		})
	}
}

func TestValidateAlertActions_CarriesI18nCodes(t *testing.T) {
	tests := []struct {
		name    string
		actions *models.AlertActions
		want    string
	}{
		{name: "nil is valid", actions: nil, want: ""},
		{name: "negative cooldown", actions: &models.AlertActions{Cooldown: -1}, want: apperr.CodeAlertCooldownNegative},
		{name: "negative escalation", actions: &models.AlertActions{EscalateAfterMinutes: -1}, want: apperr.CodeAlertEscalationNegative},
		{name: "unknown channel", actions: &models.AlertActions{Channels: []string{"carrier-pigeon"}}, want: apperr.CodeAlertChannelInvalid},
		{
			name:    "command trigger missing its action",
			actions: &models.AlertActions{CommandTrigger: &models.CommandTrigger{Module: "docker"}},
			want:    apperr.CodeAlertCommandTriggerIncomplete,
		},
		{
			name:    "command trigger with an unknown module",
			actions: &models.AlertActions{CommandTrigger: &models.CommandTrigger{Module: "nope", Action: "restart"}},
			want:    apperr.CodeAlertCommandModuleInvalid,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAlertActions(tc.actions)
			if got := i18nKey(err); got != tc.want {
				t.Errorf("i18n key = %q, want %q (err: %v)", got, tc.want, err)
			}
		})
	}
}

// A parameterized message is only useful if its params actually reach the
// catalog — otherwise the rendered text keeps a literal "{module}".
func TestValidateAlertActions_PassesInterpolationParams(t *testing.T) {
	err := validateAlertActions(&models.AlertActions{
		CommandTrigger: &models.CommandTrigger{Module: "nope", Action: "restart"},
	})

	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperr.Error, got %v", err)
	}
	if ae.Params["module"] != "nope" {
		t.Errorf("params[module] = %q, want %q", ae.Params["module"], "nope")
	}
	rendered := apperr.GetMessage(ae.I18nKey, "fr", ae.Params)
	if rendered == "" || rendered == "error: "+ae.I18nKey {
		t.Fatalf("catalog did not render %q: %q", ae.I18nKey, rendered)
	}
	if got := apperr.GetMessage(ae.I18nKey, "en", ae.Params); got == rendered {
		t.Errorf("EN and FR rendered identically (%q) — one of them is missing", got)
	}
}

func TestUpdate_UnknownRuleCarriesI18nCode(t *testing.T) {
	svc := newSvc(&fakeRepo{getErr: sql.ErrNoRows})

	err := svc.Update(context.Background(), 1, models.AlertRuleUpdate{})

	if got := i18nKey(err); got != apperr.CodeAlertRuleNotFound {
		t.Errorf("i18n key = %q, want %q (err: %v)", got, apperr.CodeAlertRuleNotFound, err)
	}
	if status(err) != 404 {
		t.Errorf("status = %d, want 404", status(err))
	}
}

func TestUpdate_ReportsAFailedIncidentResolution(t *testing.T) {
	// Disabling a rule resolves its open incidents. The rule itself is already
	// saved at that point, so this reports a partial success, not a rollback.
	disabled := false
	hostID := "h1"
	repo := &fakeRepo{
		rule:                &models.AlertRule{ID: 1, HostID: &hostID, SourceType: models.AlertSourceAgent, Metric: "cpu", Operator: ">", Enabled: true},
		hostExists:          true,
		resolveIncidentsErr: errors.New("db down"),
	}

	err := newSvc(repo).Update(context.Background(), 1, models.AlertRuleUpdate{Enabled: &disabled})

	if got := i18nKey(err); got != apperr.CodeAlertIncidentResolveFailed {
		t.Errorf("i18n key = %q, want %q (err: %v)", got, apperr.CodeAlertIncidentResolveFailed, err)
	}
	if repo.updated == nil {
		t.Error("the rule update itself should still have been persisted")
	}
}

func TestValidateDockerScope_UnknownComposeProject(t *testing.T) {
	repo := &fakeRepo{hostExists: true, composeProjectMissing: true}
	scope := &models.DockerMetricScope{HostID: "h1", ScopeMode: "compose_project", ProjectName: "stack"}

	err := newSvc(repo).ValidateDockerScope(context.Background(), scope)

	if got := i18nKey(err); got != apperr.CodeAlertComposeProjectNotFound {
		t.Errorf("i18n key = %q, want %q (err: %v)", got, apperr.CodeAlertComposeProjectNotFound, err)
	}
}

func TestValidateAlertActions_CommandTriggerActionAndTarget(t *testing.T) {
	tests := []struct {
		name    string
		trigger *models.CommandTrigger
		want    string
	}{
		{
			name:    "action not allowed for the module",
			trigger: &models.CommandTrigger{Module: "docker", Action: "delete"},
			want:    apperr.CodeAlertCommandActionInvalid,
		},
		{
			name:    "target-requiring module with no target",
			trigger: &models.CommandTrigger{Module: "systemd", Action: "restart"},
			want:    apperr.CodeAlertCommandTargetRequired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAlertActions(&models.AlertActions{CommandTrigger: tc.trigger})
			if got := i18nKey(err); got != tc.want {
				t.Errorf("i18n key = %q, want %q (err: %v)", got, tc.want, err)
			}
		})
	}
}
