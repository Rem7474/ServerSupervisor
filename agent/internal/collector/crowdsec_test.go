package collector

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCollectCrowdSecDecisionsUsesWatcherLoginForAlerts(t *testing.T) {
	decisionsKey := "decisions-key"
	alertsMachineID := "login_user"
	alertsPassword := "login_password"
	alertsToken := "very-long-alerts-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/decisions":
			if got := r.Header.Get("X-API-Key"); got != decisionsKey {
				t.Fatalf("unexpected decisions api key: %s", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"value":"172.212.217.10","scope":"Ip","origin":"crowdsec","scenario":"crowdsecurity/http-probing","duration":"1h"}]`))
		case "/v1/watchers/login":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("login payload decode failed: %v", err)
			}
			if payload["machine_id"] != alertsMachineID || payload["password"] != alertsPassword {
				t.Fatalf("unexpected login payload: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":200,"expire":"2030-01-02T15:04:05Z","token":"` + alertsToken + `"}`))
		case "/v1/alerts":
			if got := r.Header.Get("Authorization"); got != "Bearer "+alertsToken {
				t.Fatalf("unexpected authorization header: %s", got)
			}
			if got := r.URL.Query().Get("has_active_decision"); got != "true" {
				t.Fatalf("unexpected has_active_decision: %s", got)
			}
			if got := r.URL.Query().Get("limit"); got != "5000" {
				t.Fatalf("unexpected limit: %s", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"source":{"value":"172.212.217.10","country":"US","as_name":"MICROSOFT-CORP-MSN-AS-BLOCK","ip":"172.212.217.10"},"events_count":11,"scenario":"crowdsecurity/http-probing"}]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	decisions, err := CollectCrowdSecDecisions(server.URL, decisionsKey, alertsMachineID, alertsPassword, false)
	if err != nil {
		t.Fatalf("collect crowdsec decisions failed: %v", err)
	}

	decision, ok := decisions["172.212.217.10"]
	if !ok {
		t.Fatalf("expected decision for IP")
	}
	if decision.Country != "US" {
		t.Fatalf("expected country US, got %q", decision.Country)
	}
	if decision.ASName != "MICROSOFT-CORP-MSN-AS-BLOCK" {
		t.Fatalf("expected ASName MICROSOFT-CORP-MSN-AS-BLOCK, got %q", decision.ASName)
	}
}
