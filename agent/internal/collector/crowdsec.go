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
	Reason       string    // e.g., "attack:web/cvi", "attack:brute-force"
	BlockedAt    time.Time // When the block started
	BlockedUntil time.Time // When the block expires
}

// crowdSecAPIDecision is the struct to parse CrowdSec Local API response
type crowdSecAPIDecision struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Value     string `json:"value"`    // IP address
	Type      string `json:"type"`     // "ip", "range", etc.
	Scope     string `json:"scope"`    // "Ip", "Range", etc.
	Origin    string `json:"origin"`   // "crowdsec", "cscli", etc.
	Until     string `json:"until"`    // Expiration time in RFC3339 format
	Reason    string `json:"reason"`   // Scenario/reason, e.g., "attack:web/cvi"
	Duration  string `json:"duration"` // e.g., "72h", "-1s" (permanent)
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
		if verbose {
			log.Printf("[crowdsec] failed to create request: %v", err)
		}
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
		if verbose {
			log.Printf("[crowdsec] failed to query CrowdSec API: %v", err)
		}
		return result, nil // Graceful degradation
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		if verbose {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[crowdsec] API returned status %d: %s", resp.StatusCode, string(body))
		}
		return result, nil // Graceful degradation
	}

	// Parse response
	var decisions []crowdSecAPIDecision
	if err := json.NewDecoder(resp.Body).Decode(&decisions); err != nil {
		if verbose {
			log.Printf("[crowdsec] failed to decode response: %v", err)
		}
		return result, nil // Graceful degradation
	}

	// Convert to our format
	now := time.Now()
	for _, d := range decisions {
		// Only process "ip" type decisions that are currently active
		if d.Type != "ip" || d.Scope == "" || d.Value == "" {
			continue
		}

		// Parse expiration time (CrowdSec provides RFC3339)
		blockedUntil := time.Time{}
		if d.Until != "" {
			if ut, err := time.Parse(time.RFC3339, d.Until); err == nil {
				blockedUntil = ut
			}
		}

		// Parse creation time to get blocked_at
		blockedAt := time.Time{}
		if d.CreatedAt != "" {
			if ct, err := time.Parse(time.RFC3339, d.CreatedAt); err == nil {
				blockedAt = ct
			}
		}

		// Determine if still active (has not expired yet)
		isActive := blockedUntil.IsZero() || blockedUntil.After(now)
		if !isActive {
			continue
		}

		result[d.Value] = CrowdSecDecision{
			IP:           d.Value,
			Blocked:      true,
			Reason:       d.Reason,
			BlockedAt:    blockedAt,
			BlockedUntil: blockedUntil,
		}
	}

	if verbose {
		log.Printf("[crowdsec] collected %d active IP decisions", len(result))
	}

	return result, nil
}
