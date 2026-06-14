package handlers_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/serversupervisor/server/internal/database"
	"github.com/serversupervisor/server/internal/dispatch"
	"github.com/serversupervisor/server/internal/handlers"
	gitwebhooksvc "github.com/serversupervisor/server/internal/services/gitwebhook"
	"github.com/serversupervisor/server/internal/testutil"
	"github.com/serversupervisor/server/internal/ws"
)

func newGitWebhookRouter(t *testing.T, role string) (*gin.Engine, *database.DB) {
	t.Helper()
	db, cfg := testutil.NewPostgresDBWithConfig(t)
	h := handlers.NewGitWebhookHandler(gitwebhooksvc.NewService(db, cfg, dispatch.New(db), ws.NewNotificationHub()))

	r := gin.New()
	r.Use(withRole(role))
	r.GET("/webhooks/git", h.ListWebhooks)
	r.POST("/webhooks/git", h.CreateWebhook)
	r.GET("/webhooks/git/:id", h.GetWebhook)
	r.PUT("/webhooks/git/:id", h.UpdateWebhook)
	r.DELETE("/webhooks/git/:id", h.DeleteWebhook)
	r.POST("/webhooks/git/:id/regenerate-secret", h.RegenerateSecret)
	r.POST("/webhooks/git/:id/receive", h.ReceiveWebhook)
	return r, db
}

func validWebhookPayload(hostID string) map[string]any {
	return map[string]any{
		"name":           "deploy hook",
		"provider":       "github",
		"host_id":        hostID,
		"custom_task_id": "deploy",
		"enabled":        true,
	}
}

// createWebhook posts a valid webhook and returns its id.
func createWebhook(t *testing.T, r http.Handler, hostID string) string {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/webhooks/git", validWebhookPayload(hostID))
	if w.Code != http.StatusCreated {
		t.Fatalf("create webhook = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Webhook struct {
			ID string `json:"id"`
		} `json:"webhook"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Webhook.ID == "" {
		t.Fatalf("decode webhook id: %v (%s)", err, w.Body.String())
	}
	return resp.Webhook.ID
}

func ghSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func postRaw(t *testing.T, r http.Handler, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGitWebhooksCRUD(t *testing.T) {
	r, db := newGitWebhookRouter(t, "admin")
	const hostID = "wh-host-1"
	seedHost(t, db, hostID)

	id := createWebhook(t, r, hostID)
	idPath := "/webhooks/git/" + id

	if wl := doJSON(t, r, http.MethodGet, "/webhooks/git", nil); wl.Code != http.StatusOK {
		t.Fatalf("list = %d", wl.Code)
	}
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusOK {
		t.Fatalf("get = %d", g.Code)
	}

	upd := validWebhookPayload(hostID)
	upd["name"] = "renamed hook"
	if u := doJSON(t, r, http.MethodPut, idPath, upd); u.Code != http.StatusOK {
		t.Fatalf("update = %d, body = %s", u.Code, u.Body.String())
	}

	if d := doJSON(t, r, http.MethodDelete, idPath, nil); d.Code != http.StatusOK {
		t.Fatalf("delete = %d", d.Code)
	}
	if g := doJSON(t, r, http.MethodGet, idPath, nil); g.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", g.Code)
	}
}

func TestGitWebhookCreateValidation(t *testing.T) {
	r, db := newGitWebhookRouter(t, "admin")
	const hostID = "wh-host-2"
	seedHost(t, db, hostID)

	cases := []struct {
		name   string
		mutate func(p map[string]any)
	}{
		{"missing name", func(p map[string]any) { delete(p, "name") }},
		{"missing host_id", func(p map[string]any) { delete(p, "host_id") }},
		{"missing custom_task_id", func(p map[string]any) { delete(p, "custom_task_id") }},
		{"invalid provider", func(p map[string]any) { p["provider"] = "bitbucket" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validWebhookPayload(hostID)
			tc.mutate(p)
			if w := doJSON(t, r, http.MethodPost, "/webhooks/git", p); w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body = %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestGitWebhookCreateForbiddenForNonAdmin(t *testing.T) {
	r, db := newGitWebhookRouter(t, "viewer")
	const hostID = "wh-host-3"
	seedHost(t, db, hostID)
	if w := doJSON(t, r, http.MethodPost, "/webhooks/git", validWebhookPayload(hostID)); w.Code != http.StatusForbidden {
		t.Errorf("viewer create = %d, want 403", w.Code)
	}
}

// TestGitWebhookReceiveHMAC is the security-critical path: the public receiver
// must only accept requests carrying a valid HMAC-SHA256 signature of the body.
func TestGitWebhookReceiveHMAC(t *testing.T) {
	r, db := newGitWebhookRouter(t, "admin")
	const hostID = "wh-host-4"
	seedHost(t, db, hostID)

	id := createWebhook(t, r, hostID)

	// Obtain a known secret deterministically.
	sw := doJSON(t, r, http.MethodPost, "/webhooks/git/"+id+"/regenerate-secret", nil)
	if sw.Code != http.StatusOK {
		t.Fatalf("regenerate secret = %d", sw.Code)
	}
	var secretResp struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(sw.Body.Bytes(), &secretResp); err != nil || secretResp.Secret == "" {
		t.Fatalf("decode secret: %v (%s)", err, sw.Body.String())
	}

	body := []byte(`{"ref":"refs/heads/main","after":"deadbeef","repository":{"full_name":"acme/app"},"pusher":{"name":"alice"}}`)
	recvPath := "/webhooks/git/" + id + "/receive"

	// Valid signature -> dispatched (200).
	good := postRaw(t, r, recvPath, body, map[string]string{
		"Content-Type":        "application/json",
		"X-GitHub-Event":      "push",
		"X-Hub-Signature-256": ghSignature(secretResp.Secret, body),
	})
	if good.Code != http.StatusOK {
		t.Fatalf("valid signature = %d, want 200; body = %s", good.Code, good.Body.String())
	}

	// Wrong signature -> rejected (401).
	bad := postRaw(t, r, recvPath, body, map[string]string{
		"Content-Type":        "application/json",
		"X-GitHub-Event":      "push",
		"X-Hub-Signature-256": ghSignature("the-wrong-secret", body),
	})
	if bad.Code != http.StatusUnauthorized {
		t.Errorf("wrong signature = %d, want 401", bad.Code)
	}

	// Missing signature header -> rejected (401).
	missing := postRaw(t, r, recvPath, body, map[string]string{
		"Content-Type":   "application/json",
		"X-GitHub-Event": "push",
	})
	if missing.Code != http.StatusUnauthorized {
		t.Errorf("missing signature = %d, want 401", missing.Code)
	}

	// Unknown webhook id -> 404.
	unknown := postRaw(t, r, "/webhooks/git/00000000-0000-0000-0000-000000000000/receive", body, map[string]string{
		"Content-Type":        "application/json",
		"X-GitHub-Event":      "push",
		"X-Hub-Signature-256": ghSignature(secretResp.Secret, body),
	})
	if unknown.Code != http.StatusNotFound {
		t.Errorf("unknown id = %d, want 404", unknown.Code)
	}
}
