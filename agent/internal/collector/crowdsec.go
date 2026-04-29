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
	BlockedAt    time.Time // When the block started
	BlockedUntil time.Time // When the block expires
}

// crowdSecAPIDecision is the struct to parse CrowdSec Local API response
type crowdSecAPIDecision struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Value    string `json:"value"`    // IP address or range
	Type     string `json:"type"`     // remediation type: "ban", "captcha", etc.
	Scope    string `json:"scope"`    // "Ip", "Range", "Country", etc.
	Origin   string `json:"origin"`   // "crowdsec", "cscli", etc.
	Scenario string `json:"scenario"` // scenario name, e.g., "crowdsecurity/http-bad-user-agent"
	Duration string `json:"duration"` // remaining duration, e.g., "3h45m23s"
}

// CollectCrowdSecDecisions queries the CrowdSec Local API and returns a map of IP -> decision.
// If connectionString is empty or collectEnabled is false, it returns an empty map (graceful degradation).
func CollectCrowdSecDecisions(connectionString, apiKey string, verbose bool) (map[string]CrowdSecDecision, error) {
	result := make(map[string]CrowdSecDecision)

	if connectionString == "" {
		if verbose {
			log.Println("[crowdsec] connection string is empty, skipping collection")
		}
		return result, nil
	}

	// Construct the URL for the CrowdSec Local API
	// By default, query for active decisions: ?has_active_decision=true
	apiURL := fmt.Sprintf("%s/v1/decisions?has_active_decision=true", connectionString)

	// Create HTTP request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Printf("[crowdsec] failed to create request: %v", err)
		return result, nil // Graceful degradation
	}

	// Add API key if provided
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	// Execute request with 5 second timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[crowdsec] failed to query CrowdSec API: %v", err)
		return result, nil // Graceful degradation
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[crowdsec] API returned status %d: %s", resp.StatusCode, string(body))
		return result, nil // Graceful degradation
	}

	// Parse response
	var decisions []crowdSecAPIDecision
	if err := json.NewDecoder(resp.Body).Decode(&decisions); err != nil {
		log.Printf("[crowdsec] failed to decode response: %v", err)
		return result, nil // Graceful degradation
	}

	// Convert to our format
	now := time.Now()
	for _, d := range decisions {
		// Only process IP-scope decisions with a value
		if (d.Scope != "Ip" && d.Scope != "Range") || d.Value == "" {
			continue
		}

		// CrowdSec returns remaining duration (e.g. "3h45m23s"), compute absolute expiry
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

	return result, nil
}
