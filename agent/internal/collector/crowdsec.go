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
}

// crowdSecAPIAlert is used to enrich decisions with country/ASN metadata.
type crowdSecAPIAlert struct {
	Source struct {
		IP          string `json:"ip"`
		Value       string `json:"value"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		CN          string `json:"cn"`      // ISO-2 country code
		ASName      string `json:"as_name"` // AS organisation name
		AS          string `json:"as"`
		ASN         string `json:"asn"`
		ASNumber    string `json:"as_number"` // AS number
	} `json:"source"`
	EventsCount int    `json:"events_count"`
	Scenario    string `json:"scenario"`
}

// CollectCrowdSecDecisions queries the CrowdSec Local API and returns a map of IP -> decision.
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
	apiURL := fmt.Sprintf("%s/v1/decisions?has_active_decision=true", baseURL)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("[crowdsec] failed to create request: %v", err)
		return result, nil
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[crowdsec] failed to query CrowdSec API: %v", err)
		return result, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[crowdsec] API returned status %d: %s", resp.StatusCode, string(body))
		return result, nil
	}

	var decisions []crowdSecAPIDecision
	if err := json.NewDecoder(resp.Body).Decode(&decisions); err != nil {
		log.Printf("[crowdsec] failed to decode decisions: %v", err)
		return result, nil
	}

	now := time.Now()
	for _, d := range decisions {
		if (d.Scope != "Ip" && d.Scope != "Range") || d.Value == "" {
			continue
		}
		blockedUntil := time.Time{}
		if d.Duration != "" {
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

	log.Printf("[crowdsec] collected %d active IP decisions", len(result))

	// Enrich decisions with country/ASN from the alerts endpoint.
	enrichDecisionsWithAlerts(result, baseURL, apiKey, alertsMachineID, alertsPassword, client)

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

// enrichDecisionsWithAlerts calls /v1/alerts and merges country + ASN into existing decisions.
// Failures are silently ignored — this is best-effort enrichment only.
func enrichDecisionsWithAlerts(decisions map[string]CrowdSecDecision, connectionString, apiKey, alertsMachineID, alertsPassword string, client *http.Client) {
	if len(decisions) == 0 {
		return
	}

	token, err := loginCrowdSecAlertsToken(connectionString, alertsMachineID, alertsPassword, client)
	if err != nil {
		log.Printf("[crowdsec] unable to obtain alerts token: %v", err)
		return
	}

	baseURL := strings.TrimRight(connectionString, "/")
	alertURL := fmt.Sprintf("%s/v1/alerts?has_active_decision=true&limit=5000", baseURL)
	req, err := http.NewRequest("GET", alertURL, nil)
	if err != nil {
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
		return
	}

	for _, a := range alerts {
		keys := []string{strings.TrimSpace(a.Source.Value), strings.TrimSpace(a.Source.IP)}
		var matchedKey string
		for _, key := range keys {
			if key == "" {
				continue
			}
			if _, ok := decisions[key]; ok {
				matchedKey = key
				break
			}
		}
		if matchedKey == "" {
			continue
		}

		dec := decisions[matchedKey]
		country := firstNonEmpty(a.Source.Country, a.Source.CountryCode, a.Source.CN)
		asName := firstNonEmpty(a.Source.ASName, a.Source.AS, a.Source.ASN)
		if asName == "" && a.Source.ASNumber != "" {
			asName = "AS" + strings.TrimSpace(a.Source.ASNumber)
		}
		if country == "" && asName == "" {
			continue
		}
		if country != "" {
			dec.Country = country
		}
		if asName != "" {
			dec.ASName = asName
		}
		decisions[matchedKey] = dec
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
