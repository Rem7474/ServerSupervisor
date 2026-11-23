package notifychannels

import (
	"context"
	"errors"
	"testing"

	"github.com/serversupervisor/server/internal/config"
	"github.com/serversupervisor/server/internal/notify"
)

type chatCall struct {
	provider, url string
	msg           notify.ChatMessage
}

type fakeNotifier struct {
	chat    []chatCall
	chatErr error
}

func (f *fakeNotifier) SendSMTP(*config.Config, string, string, string, string) error { return nil }
func (f *fakeNotifier) SendNtfy(*config.Config, string, string, string) error         { return nil }
func (f *fakeNotifier) SendChat(provider, url string, msg notify.ChatMessage) error {
	f.chat = append(f.chat, chatCall{provider, url, msg})
	return f.chatErr
}

func newTestDispatcher(cfg *config.Config, fn *fakeNotifier) *Dispatcher {
	return &Dispatcher{cfg: cfg, notifier: fn}
}

func TestSend_ChatChannelsUseTheirOwnWebhook(t *testing.T) {
	fn := &fakeNotifier{}
	d := newTestDispatcher(&config.Config{
		SlackWebhookURL:   "https://hooks.slack.com/services/a",
		TeamsWebhookURL:   "https://example.webhook.office.com/b",
		DiscordWebhookURL: "https://discord.com/api/webhooks/c",
	}, fn)
	msg := &notify.ChatMessage{Title: "T", Text: "body"}

	d.Send(context.Background(), Event{Channels: []string{"slack", "teams", "discord"}, Chat: msg})

	want := []chatCall{
		{"slack", "https://hooks.slack.com/services/a", *msg},
		{"teams", "https://example.webhook.office.com/b", *msg},
		{"discord", "https://discord.com/api/webhooks/c", *msg},
	}
	if len(fn.chat) != len(want) {
		t.Fatalf("got %d chat sends, want %d", len(fn.chat), len(want))
	}
	for i := range want {
		if fn.chat[i] != want[i] {
			t.Errorf("send %d = %+v, want %+v", i, fn.chat[i], want[i])
		}
	}
}

func TestSend_ChatChannelSkippedWhenNotConfiguredOrNoMessage(t *testing.T) {
	fn := &fakeNotifier{}
	d := newTestDispatcher(&config.Config{SlackWebhookURL: "https://hooks.slack.com/services/a"}, fn)

	// teams has no URL configured; slack is configured but the event has no Chat payload.
	d.Send(context.Background(), Event{Channels: []string{"teams"}, Chat: &notify.ChatMessage{Text: "x"}})
	d.Send(context.Background(), Event{Channels: []string{"slack"}})

	if len(fn.chat) != 0 {
		t.Errorf("expected no sends, got %+v", fn.chat)
	}
}

func TestSend_ChatFailureDoesNotStopOtherChannels(t *testing.T) {
	fn := &fakeNotifier{chatErr: errors.New("boom")}
	d := newTestDispatcher(&config.Config{
		SlackWebhookURL:   "https://hooks.slack.com/services/a",
		DiscordWebhookURL: "https://discord.com/api/webhooks/c",
	}, fn)

	d.Send(context.Background(), Event{Channels: []string{"slack", "discord"}, Chat: &notify.ChatMessage{Text: "x"}})

	if len(fn.chat) != 2 {
		t.Errorf("a failing channel must not abort the fan-out, got %d sends", len(fn.chat))
	}
}
