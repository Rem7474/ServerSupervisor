package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func captureServer(t *testing.T, status int) (*httptest.Server, *map[string]any, *http.Header) {
	t.Helper()
	var got map[string]any
	var hdr http.Header
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		hdr = r.Header.Clone()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &got, &hdr
}

func TestChatPayload_Slack(t *testing.T) {
	p, err := chatPayload(ProviderSlack, ChatMessage{Title: "Alert", Text: "cpu 95%"})
	if err != nil {
		t.Fatal(err)
	}
	m := p.(map[string]any)
	if m["text"] != "*Alert*\ncpu 95%" {
		t.Errorf("text = %v", m["text"])
	}
	blocks := m["blocks"].([]any)
	section := blocks[0].(map[string]any)
	if section["type"] != "section" || section["text"].(map[string]string)["text"] != m["text"] {
		t.Errorf("unexpected block %v", section)
	}
}

func TestChatPayload_SlackWithoutTitle(t *testing.T) {
	p, _ := chatPayload(ProviderSlack, ChatMessage{Text: "plain"})
	if p.(map[string]any)["text"] != "plain" {
		t.Errorf("text = %v", p.(map[string]any)["text"])
	}
}

func TestChatPayload_Teams(t *testing.T) {
	p, err := chatPayload(ProviderTeams, ChatMessage{Title: "Alert", Text: "cpu 95%"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(p)
	var m struct {
		Type        string `json:"type"`
		Attachments []struct {
			ContentType string `json:"contentType"`
			Content     struct {
				Type    string           `json:"type"`
				Version string           `json:"version"`
				Body    []map[string]any `json:"body"`
			} `json:"content"`
		} `json:"attachments"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Type != "message" || len(m.Attachments) != 1 {
		t.Fatalf("unexpected envelope: %s", raw)
	}
	a := m.Attachments[0]
	if a.ContentType != "application/vnd.microsoft.card.adaptive" || a.Content.Type != "AdaptiveCard" {
		t.Errorf("unexpected card: %s", raw)
	}
	if len(a.Content.Body) != 2 || a.Content.Body[0]["text"] != "Alert" || a.Content.Body[1]["text"] != "cpu 95%" {
		t.Errorf("unexpected body: %v", a.Content.Body)
	}
}

func TestChatPayload_TeamsWithoutTitle(t *testing.T) {
	p, _ := chatPayload(ProviderTeams, ChatMessage{Text: "x"})
	raw, _ := json.Marshal(p)
	if strings.Count(string(raw), "TextBlock") != 1 {
		t.Errorf("expected a single TextBlock: %s", raw)
	}
}

func TestChatPayload_Discord(t *testing.T) {
	p, err := chatPayload(ProviderDiscord, ChatMessage{Title: "Alert", Text: "cpu 95%"})
	if err != nil {
		t.Fatal(err)
	}
	embed := p.(map[string]any)["embeds"].([]any)[0].(map[string]any)
	if embed["title"] != "Alert" || embed["description"] != "cpu 95%" || embed["color"] != defaultColor {
		t.Errorf("unexpected embed: %v", embed)
	}
	p, _ = chatPayload(ProviderDiscord, ChatMessage{Text: "x", Color: 0x2fb344})
	embed = p.(map[string]any)["embeds"].([]any)[0].(map[string]any)
	if embed["color"] != 0x2fb344 {
		t.Errorf("custom color lost: %v", embed["color"])
	}
	if _, ok := embed["title"]; ok {
		t.Error("empty title must be omitted")
	}
}

func TestChatPayload_DiscordTruncates(t *testing.T) {
	long := strings.Repeat("é", discordTextMax+50)
	p, _ := chatPayload(ProviderDiscord, ChatMessage{Title: strings.Repeat("t", 400), Text: long})
	embed := p.(map[string]any)["embeds"].([]any)[0].(map[string]any)
	if n := len([]rune(embed["description"].(string))); n != discordTextMax {
		t.Errorf("description length = %d, want %d", n, discordTextMax)
	}
	if n := len([]rune(embed["title"].(string))); n != discordTitleMax {
		t.Errorf("title length = %d, want %d", n, discordTitleMax)
	}
}

func TestChatPayload_UnknownProvider(t *testing.T) {
	if _, err := chatPayload("mattermost", ChatMessage{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsChatProvider(t *testing.T) {
	for _, p := range []string{"slack", "teams", "discord"} {
		if !IsChatProvider(p) {
			t.Errorf("%s should be a chat provider", p)
		}
	}
	if IsChatProvider("ntfy") || IsChatProvider("") {
		t.Error("ntfy/empty are not chat providers")
	}
}

func TestSendChat_RejectsInvalidURLs(t *testing.T) {
	n := &notifier{}
	for name, u := range map[string]string{
		"empty":    "",
		"http":     "http://hooks.slack.com/x",
		"loopback": "https://127.0.0.1/x",
		"private":  "https://192.168.1.10/x",
	} {
		if err := n.SendChat(ProviderSlack, u, ChatMessage{Text: "x"}); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSendChat_UnknownProvider(t *testing.T) {
	if err := (&notifier{}).SendChat("mattermost", "https://example.com/x", ChatMessage{Text: "x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSendChat_EndToEnd(t *testing.T) {
	for _, provider := range []string{ProviderSlack, ProviderTeams, ProviderDiscord} {
		t.Run(provider, func(t *testing.T) {
			srv, got, hdr := captureServer(t, http.StatusOK)
			n := &notifier{chatClient: tlsClientFor(srv)}
			// A public-looking hostname passes the static URL validator; the
			// client transport redirects the connection to the test server.
			if err := n.SendChat(provider, "https://example.com/hook", ChatMessage{Title: "T", Text: "body"}); err != nil {
				t.Fatalf("SendChat: %v", err)
			}
			if (*hdr).Get("Content-Type") != "application/json" {
				t.Errorf("content-type = %q", (*hdr).Get("Content-Type"))
			}
			if len(*got) == 0 {
				t.Fatal("server received no JSON body")
			}
		})
	}
}

// tlsClientFor returns a client that sends every request to srv, trusting its
// certificate, whatever host the URL names.
func tlsClientFor(srv *httptest.Server) *http.Client {
	return &http.Client{Transport: rewriteTransport{target: srv.Listener.Addr().String(), inner: srv.Client().Transport.(*http.Transport)}}
}

type rewriteTransport struct {
	target string
	inner  *http.Transport
}

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Host = r.target
	return r.inner.RoundTrip(req)
}

func TestSendChat_NonSuccessStatusAndSecretNotLeaked(t *testing.T) {
	srv, _, _ := captureServer(t, http.StatusNotFound)
	n := &notifier{chatClient: tlsClientFor(srv)}
	err := n.SendChat(ProviderDiscord, "https://example.com/api/webhooks/123/SECRET-TOKEN", ChatMessage{Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected a 404 error, got %v", err)
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("error leaks the webhook URL: %v", err)
	}
}

func TestSendChat_TransportErrorDoesNotLeakURL(t *testing.T) {
	srv, _, _ := captureServer(t, http.StatusOK)
	srv.Close()
	n := &notifier{chatClient: &http.Client{Transport: rewriteTransport{target: srv.Listener.Addr().String(), inner: &http.Transport{}}}}
	err := n.SendChat(ProviderSlack, "https://example.com/services/SECRET-TOKEN", ChatMessage{Text: "x"})
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("error leaks the webhook URL: %v", err)
	}
}

func TestChatClient_BlocksInternalDestinations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer srv.Close()
	// Direct request to a loopback listener through the guarded client.
	resp, err := newChatClient(false).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("guarded client reached a loopback server")
	}
	if !strings.Contains(err.Error(), errInternalDestination.Error()) {
		t.Errorf("unexpected error: %v", err)
	}
	// The unguarded variant (tests) does reach it.
	resp, err = newChatClient(true).Get(srv.URL)
	if err != nil {
		t.Fatalf("unguarded client failed: %v", err)
	}
	_ = resp.Body.Close()
}

func TestChatClient_DoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer srv.Close()
	resp, err := newChatClient(true).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want 302 (redirect not followed)", resp.StatusCode)
	}
}
