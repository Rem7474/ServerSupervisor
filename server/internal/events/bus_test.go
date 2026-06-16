package events

import (
	"testing"
	"time"
)

func recv(t *testing.T, ch <-chan struct{}) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(200 * time.Millisecond):
		return false
	}
}

func TestSubscribeReceivesPublishedTopic(t *testing.T) {
	b := NewBus()
	ch, unsub := b.Subscribe(TopicDashboard)
	defer unsub()

	b.Publish(TopicDashboard)
	if !recv(t, ch) {
		t.Fatal("expected a signal for the subscribed topic")
	}
}

func TestPublishIgnoresUnsubscribedTopic(t *testing.T) {
	b := NewBus()
	ch, unsub := b.Subscribe(TopicDashboard)
	defer unsub()

	b.Publish(TopicDocker)
	if recv(t, ch) {
		t.Fatal("did not expect a signal for a topic we are not subscribed to")
	}
}

func TestSignalsCoalesce(t *testing.T) {
	b := NewBus()
	ch, unsub := b.Subscribe(TopicApt)
	defer unsub()

	// Three publishes before any read must collapse to a single pending wake-up.
	b.Publish(TopicApt)
	b.Publish(TopicApt)
	b.Publish(TopicApt)

	if !recv(t, ch) {
		t.Fatal("expected one coalesced signal")
	}
	if recv(t, ch) {
		t.Fatal("expected only one signal after coalescing")
	}
}

func TestHostTopicIsolation(t *testing.T) {
	b := NewBus()
	ch1, unsub1 := b.Subscribe(HostTopic("h1"))
	defer unsub1()
	ch2, unsub2 := b.Subscribe(HostTopic("h2"))
	defer unsub2()

	b.Publish(HostTopic("h1"))
	if !recv(t, ch1) {
		t.Fatal("h1 subscriber should have been signalled")
	}
	if recv(t, ch2) {
		t.Fatal("h2 subscriber must not be signalled by an h1 publish")
	}
}

func TestUnsubscribeStopsSignals(t *testing.T) {
	b := NewBus()
	ch, unsub := b.Subscribe(TopicNetwork)
	unsub()

	b.Publish(TopicNetwork)
	if recv(t, ch) {
		t.Fatal("unsubscribed channel must not receive signals")
	}
}

func TestPublishAllFansOut(t *testing.T) {
	b := NewBus()
	dash, u1 := b.Subscribe(TopicDashboard)
	defer u1()
	apt, u2 := b.Subscribe(TopicApt)
	defer u2()

	b.PublishAll(TopicDashboard, TopicApt)
	if !recv(t, dash) || !recv(t, apt) {
		t.Fatal("PublishAll should signal every matching subscriber")
	}
}

func TestNilBusIsSafe(t *testing.T) {
	var b *Bus // nil
	ch, unsub := b.Subscribe(TopicDashboard)
	b.Publish(TopicDashboard) // must not panic
	b.PublishAll(TopicDocker) // must not panic
	unsub()                   // must not panic
	if recv(t, ch) {
		t.Fatal("nil-bus subscription must never fire")
	}
}
