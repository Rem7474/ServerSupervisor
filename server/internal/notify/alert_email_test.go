package notify

import (
	"strings"
	"testing"
)

func TestRenderAlertEmail(t *testing.T) {
	html, err := RenderAlertEmail(AlertEmailData{
		RuleName:        "CPU eleve",
		RuleID:          42,
		HostName:        "web-01",
		Metric:          "cpu",
		Operator:        ">",
		Threshold:       "85.00",
		Value:           "93.50",
		Unit:            "%",
		TriggeredAt:     "2026-07-12 10:00:00",
		IncidentLink:    "https://supervisor.example.lan/alerts?tab=incidents",
		CooldownMessage: "5 min",
	})
	if err != nil {
		t.Fatalf("RenderAlertEmail() error = %v", err)
	}

	for _, want := range []string{
		"CPU eleve",
		"web-01",
		"93.50",
		"85.00",
		"https://supervisor.example.lan/alerts?tab=incidents",
		"5 min",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered email missing %q\n--- output ---\n%s", want, html)
		}
	}

	if !isHTMLContent(html) {
		t.Error("rendered email should be detected as HTML by isHTMLContent (missing <!DOCTYPE/<html>/<body>)")
	}
}

func TestRenderAlertEmail_OmitsCooldownLineWhenNotSet(t *testing.T) {
	html, err := RenderAlertEmail(AlertEmailData{RuleName: "x", HostName: "y"})
	if err != nil {
		t.Fatalf("RenderAlertEmail() error = %v", err)
	}
	if strings.Contains(html, "Aucune nouvelle notification") {
		t.Error("cooldown footer line should be omitted when CooldownMessage is empty")
	}
}
