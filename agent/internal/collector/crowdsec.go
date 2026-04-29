package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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
		IP       string `json:"ip"`
		CN       string `json:"cn"`        // ISO-2 country code
		ASName   string `json:"as_name"`   // AS organisation name
		ASNumber string `json:"as_number"` // AS number
	} `json:"source"`
	EventsCount int    `json:"events_count"`
	Scenario    string `json:"scenario"`
}

// CollectCrowdSecDecisions queries the CrowdSec Local API and returns a map of IP -> decision.
// Also enriches decisions with country/ASN data from the alerts endpoint.
func CollectCrowdSecDecisions(connectionString, apiKey string, verbose bool) (map[string]CrowdSecDecision, error) {
	result := make(map[string]CrowdSecDecision)

	if connectionString == "" {
		if verbose {
			log.Println("[crowdsec] connection string is empty, skipping collection")
		}
		return result, nil
	}

	client := &http.Client{Timeout: 5 * time.Second}

	apiURL := fmt.Sprintf("%s/v1/decisions?has_active_decision=true", connectionString)
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
	enrichDecisionsWithAlerts(result, connectionString, apiKey, client)

	return result, nil
}

// enrichDecisionsWithAlerts calls /v1/alerts and merges country + ASN into existing decisions.
// Failures are silently ignored — this is best-effort enrichment only.
func enrichDecisionsWithAlerts(decisions map[string]CrowdSecDecision, connectionString, apiKey string, client *http.Client) {
	if len(decisions) == 0 {
		return
	}

	alertURL := fmt.Sprintf("%s/v1/alerts?has_active_decision=true&limit=5000", connectionString)
	req, err := http.NewRequest("GET", alertURL, nil)
	if err != nil {
		return
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var alerts []crowdSecAPIAlert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return
	}

	for _, a := range alerts {
		ip := a.Source.IP
		if ip == "" {
			continue
		}
		dec, ok := decisions[ip]
		if !ok || (dec.Country != "" && dec.ASName != "") {
			continue
		}
		country := a.Source.CN
		asName := a.Source.ASName
		if asName == "" && a.Source.ASNumber != "" {
			asName = "AS" + a.Source.ASNumber
		}
		dec.Country = country
		dec.ASName = asName
		decisions[ip] = dec
	}
}
