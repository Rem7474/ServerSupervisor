package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/serversupervisor/server/internal/apperr"
	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/models"
	"github.com/serversupervisor/server/internal/notify"
)

type fakeNotifier struct {
	provider, url string
	msg           notify.ChatMessage
	err           error
}

func (f *fakeNotifier) SendSMTP(*config.Config, string, string, string, string) error { return nil }
func (f *fakeNotifier) SendNtfy(*config.Config, string, string, string) error         { return nil }
func (f *fakeNotifier) SendChat(provider, url string, msg notify.ChatMessage) error {
	f.provider, f.url, f.msg = provider, url, msg
	return f.err
}

const slackURL = "https://hooks.slack.com/services/T000/B000/SECRET"

func TestSnapshot_ChatWebhooksNeverExposeTheURL(t *testing.T) {
	cfg := &config.Config{SlackWebhookURL: slackURL, DiscordWebhookURL: "https://discord.com/api/webhooks/1/SECRET"}
	snap := newSvc(&fakeRepo{}, cfg).Snapshot(context.Background())["settings"].(map[string]any)
	if snap["slackWebhookConfigured"] != true || snap["discordWebhookConfigured"] != true || snap["teamsWebhookConfigured"] != false {
		t.Errorf("unexpected configured flags: %v", snap)
	}
	for k, v := range snap {
		if s, ok := v.(string); ok && strings.Contains(s, "SECRET") {
			t.Errorf("snapshot field %q leaks a webhook URL", k)
		}
	}
}

func TestUpdate_SavesChatWebhooksAndKeepsMaskedOnes(t *testing.T) {
	repo := &fakeRepo{}
	svc := newSvc(repo, &config.Config{})
	svc.Update(context.Background(), models.SettingsUpdateRequest{
		SlackWebhookURL:   "  " + slackURL + " ",
		TeamsWebhookURL:   config.MaskedSecretSentinel,
		DiscordWebhookURL: "",
	}, "alice", "1.2.3.4")

	if repo.setCalls["slack_webhook_url"] != slackURL {
		t.Errorf("slack_webhook_url = %q, want trimmed URL", repo.setCalls["slack_webhook_url"])
	}
	if _, ok := repo.setCalls["teams_webhook_url"]; ok {
		t.Error("masked sentinel must not overwrite the stored teams URL")
	}
	if _, ok := repo.setCalls["discord_webhook_url"]; ok {
		t.Error("empty value must not overwrite the stored discord URL")
	}
}

func TestValidateUpdate_ChatWebhooks(t *testing.T) {
	svc := newSvc(&fakeRepo{}, &config.Config{})
	if err := svc.ValidateUpdate(models.SettingsUpdateRequest{SlackWebhookURL: slackURL, TeamsWebhookURL: config.MaskedSecretSentinel}); err != nil {
		t.Errorf("valid request rejected: %v", err)
	}
	for name, req := range map[string]models.SettingsUpdateRequest{
		"slack http":       {SlackWebhookURL: "http://hooks.slack.com/x"},
		"teams private":    {TeamsWebhookURL: "https://10.0.0.5/hook"},
		"discord loopback": {DiscordWebhookURL: "https://localhost/hook"},
	} {
		err := svc.ValidateUpdate(req)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.HTTPStatus != 400 || ae.I18nKey != apperr.CodeSettingsWebhookURLInvalid {
			t.Errorf("%s: expected a 400 with the invalid-URL code, got %v", name, err)
		}
	}
}

func TestTestChatWebhook(t *testing.T) {
	fn := &fakeNotifier{}
	svc := newSvc(&fakeRepo{}, &config.Config{SlackWebhookURL: slackURL})
	svc.notifier = fn

	if _, err := svc.TestChatWebhook(notify.ProviderSlack); err != nil {
		t.Fatalf("TestChatWebhook: %v", err)
	}
	if fn.provider != notify.ProviderSlack || fn.url != slackURL || fn.msg.Text == "" {
		t.Errorf("notifier called with %q %q %+v", fn.provider, fn.url, fn.msg)
	}
}

func TestTestChatWebhook_Errors(t *testing.T) {
	svc := newSvc(&fakeRepo{}, &config.Config{SlackWebhookURL: slackURL})
	svc.notifier = &fakeNotifier{err: errors.New("slack: webhook returned status 404")}

	if _, err := svc.TestChatWebhook("mattermost"); !isValidation(err) {
		t.Errorf("unknown provider should be a 400, got %v", err)
	}
	if _, err := svc.TestChatWebhook(notify.ProviderTeams); !isValidation(err) {
		t.Errorf("unconfigured provider should be a 400, got %v", err)
	}
	_, err := svc.TestChatWebhook(notify.ProviderSlack)
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.HTTPStatus != 500 || ae.I18nKey != apperr.CodeSettingsWebhookSendFailed {
		t.Fatalf("send failure should be a 500 with the send-failed code, got %v", err)
	}
}
