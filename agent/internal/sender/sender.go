package sender

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/serversupervisor/agent/internal/config"
)

type Sender struct {
	cfg           *config.Config
	reportClient  *http.Client // 30s — periodic reports
	commandClient *http.Client // 30min — long-running command results/streaming
}

type Report struct {
	HostID          string      `json:"host_id"`
	AgentVersion    string      `json:"agent_version"`
	Metrics         interface{} `json:"metrics"`
	Docker          interface{} `json:"docker"`
	AptStatus       interface{} `json:"apt_status"`
	DockerNetworks  interface{} `json:"docker_networks,omitempty"`  // Network topology data
	ContainerEnvs   interface{} `json:"container_envs,omitempty"`   // Container environment variables
	ComposeProjects interface{} `json:"compose_projects,omitempty"` // Docker Compose projects
	DiskMetrics     interface{} `json:"disk_metrics,omitempty"`     // Detailed disk usage with inodes
	DiskHealth      interface{} `json:"disk_health,omitempty"`      // SMART disk health data
	Timestamp       time.Time   `json:"timestamp"`
}

type ReportResponse struct {
	Status   string           `json:"status"`
	Commands []PendingCommand `json:"commands"`
}

type PendingCommand struct {
	ID      string `json:"id"`      // UUID
	Module  string `json:"module"`  // docker | apt | systemd | journal
	Action  string `json:"action"`  // start, stop, upgrade, logs, list, …
	Target  string `json:"target"`  // container / service name; empty for apt
	Payload string `json:"payload"` // JSON extra args
}

type CommandResult struct {
	CommandID string      `json:"command_id"` // UUID
	Status    string      `json:"status"`
	Output    string      `json:"output"`
	AptStatus interface{} `json:"apt_status,omitempty"` // Full APT status after update/upgrade
}

func New(cfg *config.Config) *Sender {
	if cfg.InsecureSkipVerify {
		log.Println("WARNING: TLS certificate verification is disabled. Not suitable for production.")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}

	return &Sender{
		cfg: cfg,
		reportClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		commandClient: &http.Client{
			Timeout:   30 * time.Minute,
			Transport: transport,
		},
	}
}

// SendReport sends a full report to the server and returns any pending commands
func (s *Sender) SendReport(report *Report) (*ReportResponse, error) {
	data, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal report: %w", err)
	}

	req, err := http.NewRequest("POST", s.cfg.ServerURL+"/api/agent/report", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.cfg.APIKey)

	resp, err := s.reportClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send report: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var response ReportResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// SendReportWithRetry wraps SendReport with exponential backoff (3 attempts).
// Only reports should be retried — command results must not be retried to avoid
// double-execution side effects.
func (s *Sender) SendReportWithRetry(report *Report) (*ReportResponse, error) {
	var lastErr error
	delays := []time.Duration{5 * time.Second, 15 * time.Second}
	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			log.Printf("Retrying report (attempt %d/%d) after %v: %v",
				attempt+1, len(delays)+1, delays[attempt-1], lastErr)
			time.Sleep(delays[attempt-1])
		}
		resp, err := s.SendReport(report)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("all %d report attempts failed: %w", len(delays)+1, lastErr)
}

// ReportCommandResult sends the result of a command execution back to the server.
// Uses the long-timeout commandClient to support lengthy operations (apt upgrade, etc.).
func (s *Sender) ReportCommandResult(result *CommandResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	req, err := http.NewRequest("POST", s.cfg.ServerURL+"/api/agent/command/result", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.cfg.APIKey)

	resp, err := s.commandClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send command result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Command result for #%s reported successfully (status: %s)", result.CommandID, result.Status)
	return nil
}

// StreamCommandChunk sends a chunk of command output to the server for real-time streaming.
// Uses commandClient (30min timeout) since streaming can span long operations.
func (s *Sender) StreamCommandChunk(commandID string, chunk string) error {
	payload := struct {
		CommandID string `json:"command_id"`
		Chunk     string `json:"chunk"`
	}{
		CommandID: commandID,
		Chunk:     chunk,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal chunk: %w", err)
	}

	req, err := http.NewRequest("POST", s.cfg.ServerURL+"/api/agent/command/stream", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.cfg.APIKey)

	resp, err := s.commandClient.Do(req)
	if err != nil {
		// Don't fail the command if streaming fails, just log it
		log.Printf("Failed to stream chunk for command #%s: %v", commandID, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Server returned status %d when streaming chunk", resp.StatusCode)
	}

	return nil
}

// SendAuditLog sends an audit log entry for agent actions
func (s *Sender) SendAuditLog(action, status, details string) error {
	auditLog := map[string]string{
		"action":  action,
		"status":  status,
		"details": details,
	}

	data, err := json.Marshal(auditLog)
	if err != nil {
		return fmt.Errorf("failed to marshal audit log: %w", err)
	}

	req, err := http.NewRequest("POST", s.cfg.ServerURL+"/api/agent/audit", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.cfg.APIKey)

	resp, err := s.reportClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send audit log: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Log but don't fail - audit logging is best-effort
		log.Printf("Warning: Server returned status %d when recording audit log", resp.StatusCode)
	}

	return nil
}
