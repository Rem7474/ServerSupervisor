package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/serversupervisor/server/internal/config"
)

func (n *notifier) SendNtfy(cfg *config.Config, url, title, msg string) error {
	if url == "" {
		return fmt.Errorf("ntfy: URL is empty")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	payload, _ := json.Marshal(map[string]string{
		"message": msg,
	})

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if title != "" {
		req.Header.Set("Title", title)
	}

	// Add Authorization header if token is configured (e.g., "Bearer <token>")
	if cfg.NtfyAuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.NtfyAuthToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("notify: ntfy failed: %v", err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	return nil
}
