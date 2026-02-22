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
	cfg    *config.Config
	client *http.Client
}

type Report struct {
	HostID       string      `json:"host_id"`
	AgentVersion string      `json:"agent_version"`
	Metrics      interface{} `json:"metrics"`
	Docker       interface{} `json:"docker"`
	AptStatus    interface{} `json:"apt_status"`
	Timestamp    time.Time   `json:"timestamp"`
}

type ReportResponse struct {
	Status   string           `json:"status"`
	Commands []PendingCommand `json:"commands"`
}

type PendingCommand struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type CommandResult struct {
	CommandID int64       `json:"command_id"`
	Status    string      `json:"status"`
	Output    string      `json:"output"`
	AptStatus interface{} `json:"apt_status,omitempty"` // Full APT status after update/upgrade
}

func New(cfg *config.Config) *Sender {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}

	return &Sender{
		cfg: cfg,
		client: &http.Client{
			Timeout:   30 * time.Second,
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

	resp, err := s.client.Do(req)
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

// ReportCommandResult sends the result of a command execution back to the server
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

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send command result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Command result for #%d reported successfully (status: %s)", result.CommandID, result.Status)
	return nil
}

// ReportCommandStatus sends a status update for a running command
func (s *Sender) ReportCommandStatus(commandID int64, status string) error {
	result := &CommandResult{
		CommandID: commandID,
		Status:    status,
		Output:    "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	req, err := http.NewRequest("POST", s.cfg.ServerURL+"/api/agent/command/result", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", s.cfg.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send command status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Command #%d marked as %s", commandID, status)
	return nil
}

// StreamCommandChunk sends a chunk of command output to the server for real-time streaming
func (s *Sender) StreamCommandChunk(commandID int64, chunk string) error {
	payload := struct {
		CommandID string `json:"command_id"`
		Chunk     string `json:"chunk"`
	}{
		CommandID: fmt.Sprintf("%d", commandID),
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

	resp, err := s.client.Do(req)
	if err != nil {
		// Don't fail the command if streaming fails, just log it
		log.Printf("Failed to stream chunk for command #%d: %v", commandID, err)
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

	resp, err := s.client.Do(req)
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
