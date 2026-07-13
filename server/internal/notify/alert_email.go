package notify

import (
	"bytes"
	_ "embed"
	"html/template"
)

//go:embed alert_email_template.html
var alertEmailTemplateSrc string

var alertEmailTemplate = template.Must(template.New("alert_email").Parse(alertEmailTemplateSrc))

// AlertEmailData is the data set for alert_email_template.html. Kept as plain
// strings (rather than taking models.AlertRule/models.Host directly) so this
// package doesn't need to depend on internal/models — the caller (alerts
// package) formats domain values into display strings first.
type AlertEmailData struct {
	RuleName        string
	RuleID          int64
	HostName        string
	Metric          string
	Operator        string
	Threshold       string
	Value           string
	Unit            string
	TriggeredAt     string
	IncidentLink    string
	CooldownMessage string
}

// RenderAlertEmail renders alert_email_template.html with data. Callers
// should fall back to a plain-text body on error rather than block the
// notification on a template bug.
func RenderAlertEmail(data AlertEmailData) (string, error) {
	var buf bytes.Buffer
	if err := alertEmailTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
