package collector

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// CrowdSecDecision represents a single CrowdSec decision for an IP
type CrowdSecDecision struct {
	IP           string
	Blocked      bool
	Reason       string    // scenario, e.g., "crowdsecurity/http-bad-user-agent"
	Origin       string    // "CAPI", "crowdsec", "cscli", etc.
	Country      string    // ISO country code from alerts enrichment
	ASName       string    // AS organisation name from alerts enrichment
	BlockedAt    time.Time // When the block started
	BlockedUntil time.Time // When the block expires
}

type crowdSecWatcherLoginResponse struct {
	Code    int    `json:"code"`
	Expire  string `json:"expire"`
	Token   string `json:"token"`
	Message string `json:"message,omitempty"`
}

// crowdSecAPIDecision is the struct to parse CrowdSec Local API response
type crowdSecAPIDecision struct {
	Value    string `json:"value"`    // IP address or range
	Type     string `json:"type"`     // remediation type: "ban", "captcha", etc.
	Scope    string `json:"scope"`    // "Ip", "Range", "Country", etc.
	Origin   string `json:"origin"`   // "crowdsec", "cscli", "CAPI", etc.
	Scenario string `json:"scenario"` // scenario name, e.g., "crowdsecurity/http-bad-user-agent"
	Duration string `json:"duration"` // remaining duration, e.g., "3h45m23s"
	Until    string `json:"until"`    // expiry timestamp when provided
}

// crowdSecAPIAlert is used to enrich decisions with country/ASN metadata.
type crowdSecAPIAlert struct {
	Source struct {
		IP          string `json:"ip"`
		Value       string `json:"value"`
		CN          string `json:"cn"`        // ISO-2 country code
		ASName      string `json:"as_name"`   // AS organisation name
		ASNumber    string `json:"as_number"` // AS number
	} `json:"source"`
	EventsCount int    `json:"events_count"`
	Scenario    string `json:"scenario"`
	Decisions   []struct {
		Value    string `json:"value"`
		Scope    string `json:"scope"`
		Type     string `json:"type"`
		Origin   string `json:"origin"`
		Scenario string `json:"scenario"`
		Duration string `json:"duration"`
		Until    string `json:"until"`
	} `json:"decisions"`
}

// CollectCrowdSecDecisions queries the CrowdSec Local API and returns a map of IP -> decision.
// Auth priority: bouncer X-API-Key → watcher JWT (fallback when no bouncer key configured).
// Also enriches decisions with country/ASN data from the alerts endpoint.
func CollectCrowdSecDecisions(connectionString, apiKey, alertsMachineID, alertsPassword string, verbose bool) (map[string]CrowdSecDecision, error) {
	result := make(map[string]CrowdSecDecision)

	if connectionString == "" {
		if verbose {
			log.Println("[crowdsec] connection string is empty, skipping collection")
		}
		return result, nil
	}

	client := &http.Client{Timeout: 5 * time.Second}
	baseURL := strings.TrimRight(connectionString, "/")

	// Build auth header: prefer bouncer key; fall back to watcher JWT so that
	// setups with only machine credentials (ban/unban) still report decisions.
	authHeader := ""
	if apiKey != "" {
		authHeader = "X-API-Key " + apiKey
	} else if alertsMachineID != "" && alertsPassword != "" {
		token, err := loginCrowdSecAlertsToken(connectionString, alertsMachineID, alertsPassword, client)
		if err != nil {
			log.Printf("[crowdsec] watcher fallback auth failed: %v", err)
		} else {
			authHeader = "Bearer " + token
		}
	}

	apiURL := fmt.Sprintf("%s/v1/decisions", baseURL)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("[crowdsec] failed to create request: %v", err)
		return result, nil
	}
	if authHeader != "" {
		if strings.HasPrefix(authHeader, "X-API-Key ") {
			req.Header.Set("X-Api-Key", strings.TrimPrefix(authHeader, "X-API-Key "))
		} else {
			req.Header.Set("Authorization", authHeader)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[crowdsec] failed to query CrowdSec API: %v", err)
	} else {
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[crowdsec] API returned status %d: %s", resp.StatusCode, string(body))
		} else {
			var decisions []crowdSecAPIDecision
			if err := json.NewDecoder(resp.Body).Decode(&decisions); err != nil {
				log.Printf("[crowdsec] failed to decode decisions: %v", err)
			} else {
				now := time.Now()
				for _, d := range decisions {
					if !isCrowdSecIPScope(d.Scope) || strings.TrimSpace(d.Value) == "" {
						continue
					}
					blockedUntil := time.Time{}
					if d.Until != "" {
						if t, err := parseCrowdSecTime(d.Until); err == nil {
							blockedUntil = t
						}
					}
					if blockedUntil.IsZero() && d.Duration != "" {
						if dur, err := time.ParseDuration(d.Duration); err == nil && dur > 0 {
							blockedUntil = now.Add(dur)
						}
					}
					result[d.Value] = CrowdSecDecision{
						IP:           d.Value,
						Blocked:      true,
						Reason:       d.Scenario,
						Origin:       d.Origin,
						BlockedAt:    time.Time{},
						BlockedUntil: blockedUntil,
					}
				}
			}
		}
	}

	log.Printf("[crowdsec] collected %d active IP decisions", len(result))

	// Enrich decisions (or populate fallback) with data from the alerts endpoint.
	enrichDecisionsWithAlerts(result, baseURL, alertsMachineID, alertsPassword, client)

	return result, nil
}

func loginCrowdSecAlertsToken(connectionString, machineID, password string, client *http.Client) (string, error) {
	if machineID == "" || password == "" {
		return "", errors.New("missing CrowdSec alerts credentials")
	}

	baseURL := strings.TrimRight(connectionString, "/")
	loginURL := fmt.Sprintf("%s/v1/watchers/login", baseURL)
	payload := map[string]string{
		"machine_id": machineID,
		"password":   password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal CrowdSec alerts login payload: %w", err)
	}

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("failed to create CrowdSec alerts login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to login to CrowdSec alerts API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("CrowdSec alerts login failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var loginResp crowdSecWatcherLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode CrowdSec alerts login response: %w", err)
	}
	if loginResp.Token == "" {
		return "", errors.New("CrowdSec alerts login returned an empty token")
	}

	return loginResp.Token, nil
}

// crowdSecWriteClient returns an http.Client and a Bearer token for write operations
// (DELETE /v1/decisions, POST /v1/alerts). These endpoints require watcher JWT auth,
// not the bouncer X-API-Key used for reads.
func crowdSecWriteClient(connectionString, machineID, password string) (*http.Client, string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	token, err := loginCrowdSecAlertsToken(connectionString, machineID, password, client)
	if err != nil {
		return nil, "", fmt.Errorf("watcher login failed: %w", err)
	}
	return client, token, nil
}

// CreateCrowdSecDecision adds a ban decision for a specific IP via the CrowdSec Local API.
// duration must be a Go duration string, e.g. "4h", "24h", "168h".
// machineID and password are the watcher credentials from /etc/crowdsec/local_api_credentials.yaml.
func CreateCrowdSecDecision(connectionString, machineID, password, ip, duration string) error {
	if connectionString == "" {
		return errors.New("crowdsec connection string is empty")
	}
	if ip == "" {
		return errors.New("IP is empty")
	}
	if duration == "" {
		duration = "4h"
	}
	dur, err := time.ParseDuration(duration)
	if err != nil || dur <= 0 {
		return fmt.Errorf("invalid duration %q: %w", duration, err)
	}

	client, token, err := crowdSecWriteClient(connectionString, machineID, password)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	until := now.Add(dur)
	startAt := now.Format(time.RFC3339Nano)
	stopAt := until.Format(time.RFC3339Nano)

	type crowdSecBanDecision struct {
		Origin          string `json:"origin"`
		Type            string `json:"type"`
		Scope           string `json:"scope"`
		Value           string `json:"value"`
		Duration        string `json:"duration"`
		Until           string `json:"until"`
		Scenario        string `json:"scenario"`
		ScenarioHash    string `json:"scenario_hash"`
		ScenarioVersion string `json:"scenario_version"`
	}
	type crowdSecAlertSource struct {
		Scope string `json:"scope"`
		Value string `json:"value"`
		IP    string `json:"ip"`
	}
	type crowdSecBanAlert struct {
		Scenario        string                `json:"scenario"`
		ScenarioHash    string                `json:"scenario_hash"`
		ScenarioVersion string                `json:"scenario_version"`
		Message         string                `json:"message"`
		EventsCount     int                   `json:"events_count"`
		StartAt         string                `json:"start_at"`
		StopAt          string                `json:"stop_at"`
		Capacity        int                   `json:"capacity"`
		LeakSpeed       string                `json:"leakspeed"`
		Simulated       bool                  `json:"simulated"`
		Events          []any                 `json:"events"`
		Labels          []string              `json:"labels,omitempty"`
		Source          crowdSecAlertSource   `json:"source"`
		Decisions       []crowdSecBanDecision `json:"decisions"`
	}

	alert := crowdSecBanAlert{
		Scenario:        "manual",
		ScenarioHash:    "manual",
		ScenarioVersion: "v0.0.0",
		Message:         "manual ban via ServerSupervisor",
		EventsCount:     1,
		StartAt:         startAt,
		StopAt:          stopAt,
		Capacity:        -1,
		LeakSpeed:       "0",
		Events:          []any{},
		Source:          crowdSecAlertSource{Scope: "Ip", Value: ip, IP: ip},
		Decisions: []crowdSecBanDecision{{
			Origin:          "cscli",
			Type:            "ban",
			Scope:           "Ip",
			Value:           ip,
			Duration:        duration,
			Until:           stopAt,
			Scenario:        "manual",
			ScenarioHash:    "manual",
			ScenarioVersion: "v0.0.0",
		}},
	}

	payload, err := json.Marshal([]crowdSecBanAlert{alert})
	if err != nil {
		return fmt.Errorf("failed to marshal ban payload: %w", err)
	}

	baseURL := strings.TrimRight(connectionString, "/")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/alerts", baseURL), strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("failed to create ban request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to query CrowdSec ban API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("CrowdSec ban API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// DeleteCrowdSecDecision removes a ban decision for a specific IP via the CrowdSec Local API.
// machineID and password are the watcher credentials from /etc/crowdsec/local_api_credentials.yaml.
func DeleteCrowdSecDecision(connectionString, machineID, password, ip string) error {
	if connectionString == "" {
		return errors.New("crowdsec connection string is empty")
	}
	if ip == "" {
		return errors.New("IP is empty")
	}

	client, token, err := crowdSecWriteClient(connectionString, machineID, password)
	if err != nil {
		return err
	}

	baseURL := strings.TrimRight(connectionString, "/")
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/v1/decisions?ip=%s", baseURL, ip), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to query CrowdSec delete API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("CrowdSec delete API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return nil
}

// enrichDecisionsWithAlerts calls /v1/alerts and merges country + ASN into existing decisions.
// If decisions is empty (or missing entries), it backfills from alerts so manual bans show up.
// Failures are silently ignored — this is best-effort enrichment only.
func enrichDecisionsWithAlerts(decisions map[string]CrowdSecDecision, connectionString, alertsMachineID, alertsPassword string, client *http.Client) {
	token, err := loginCrowdSecAlertsToken(connectionString, alertsMachineID, alertsPassword, client)
	if err != nil {
		log.Printf("[crowdsec] unable to obtain alerts token: %v", err)
		return
	}

	baseURL := strings.TrimRight(connectionString, "/")
	alertURL := fmt.Sprintf("%s/v1/alerts?has_active_decision=true&limit=5000", baseURL)
	req, err := http.NewRequest("GET", alertURL, nil)
	if err != nil {
		log.Printf("[crowdsec] failed to create alerts request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[crowdsec] failed to query CrowdSec alerts API: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[crowdsec] alerts API returned status %d: %s", resp.StatusCode, string(body))
		return
	}

	var alerts []crowdSecAPIAlert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		log.Printf("[crowdsec] failed to decode alerts: %v", err)
		return
	}

	now := time.Now().UTC()
	applyDecision := func(ip string, origin string, scenario string, untilRaw string, durationRaw string, a crowdSecAPIAlert) {
		if ip == "" {
			return
		}
		dec := decisions[ip]
		dec.IP = ip
		dec.Blocked = true
		if dec.Reason == "" {
			dec.Reason = scenario
		}
		if dec.Origin == "" {
			dec.Origin = origin
		}
		country := firstNonEmpty(a.Source.CN)
		asName := firstNonEmpty(a.Source.ASName)
		if asName == "" && a.Source.ASNumber != "" {
			asName = "AS" + strings.TrimSpace(a.Source.ASNumber)
		}
		if country != "" {
			dec.Country = country
		}
		if asName != "" {
			dec.ASName = asName
		}
		if dec.BlockedUntil.IsZero() && untilRaw != "" {
			if t, err := parseCrowdSecTime(untilRaw); err == nil {
				dec.BlockedUntil = t
			}
		}
		if dec.BlockedUntil.IsZero() && durationRaw != "" {
			if dur, err := time.ParseDuration(durationRaw); err == nil && dur > 0 {
				dec.BlockedUntil = now.Add(dur)
			}
		}
		decisions[ip] = dec
	}

	for _, a := range alerts {
		alertIP := strings.TrimSpace(a.Source.IP)
		if alertIP == "" {
			alertIP = strings.TrimSpace(a.Source.Value)
		}

		if len(a.Decisions) == 0 {
			applyDecision(alertIP, "", a.Scenario, "", "", a)
			continue
		}

		for _, d := range a.Decisions {
			if !isCrowdSecIPScope(d.Scope) {
				continue
			}
			applyDecision(strings.TrimSpace(d.Value), d.Origin, firstNonEmpty(d.Scenario, a.Scenario), d.Until, d.Duration, a)
		}
	}
}

func isCrowdSecIPScope(scope string) bool {
	return strings.EqualFold(scope, "Ip") || strings.EqualFold(scope, "Range")
}

func parseCrowdSecTime(value string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
