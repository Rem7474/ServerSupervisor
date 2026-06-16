// Package events is a tiny in-process publish/subscribe bus used to drive the
// WebSocket snapshot endpoints event-driven instead of polling the database on a
// fixed timer. Writers publish a topic after a state change; each WebSocket
// connection subscribes to the topics its view depends on and rebuilds+pushes its
// snapshot when signalled.
//
// It has zero dependencies on other internal packages so any layer (services,
// background jobs, pollers) can publish without import cycles. All methods are
// nil-safe: a nil *Bus publishes nothing and hands out a never-firing
// subscription, so call sites (and tests) that don't wire a bus keep working.
package events

import "sync"

// Topics for the global (non per-host) snapshot views.
const (
	TopicDashboard = "dashboard"
	TopicDocker    = "docker"
	TopicNetwork   = "network"
	TopicApt       = "apt"
)

// HostTopic returns the per-host topic for the host-detail view of hostID.
func HostTopic(hostID string) string { return "host:" + hostID }

type subscription struct {
	topics map[string]struct{}
	ch     chan struct{}
}

// Bus is a topic-based signal broker. Signals carry no payload: a subscriber
// reacts by recomputing its own state. The subscriber channel is buffered to 1
// and sends are non-blocking, so a burst of publishes between two reads collapses
// into a single wake-up (coalescing).
type Bus struct {
	mu   sync.Mutex
	subs map[*subscription]struct{}
}

func NewBus() *Bus {
	return &Bus{subs: make(map[*subscription]struct{})}
}

// Subscribe returns a coalescing signal channel that fires whenever any of the
// given topics is published, plus an unsubscribe func that must be called when the
// subscriber goes away. A nil bus returns a channel that never fires.
func (b *Bus) Subscribe(topics ...string) (<-chan struct{}, func()) {
	if b == nil {
		return make(chan struct{}), func() {}
	}
	set := make(map[string]struct{}, len(topics))
	for _, t := range topics {
		set[t] = struct{}{}
	}
	s := &subscription{topics: set, ch: make(chan struct{}, 1)}

	b.mu.Lock()
	b.subs[s] = struct{}{}
	b.mu.Unlock()

	return s.ch, func() {
		b.mu.Lock()
		delete(b.subs, s)
		b.mu.Unlock()
	}
}

// Publish signals every subscriber that registered the given topic. It never
// blocks: a subscriber whose buffer is already full keeps its pending wake-up.
func (b *Bus) Publish(topic string) {
	if b == nil {
		return
	}
	b.mu.Lock()
	targets := make([]*subscription, 0, len(b.subs))
	for s := range b.subs {
		if _, ok := s.topics[topic]; ok {
			targets = append(targets, s)
		}
	}
	b.mu.Unlock()

	for _, s := range targets {
		select {
		case s.ch <- struct{}{}:
		default:
		}
	}
}

// PublishAll signals several topics at once (convenience for writers that affect
// multiple views, e.g. an agent report touching dashboard/docker/network/apt).
func (b *Bus) PublishAll(topics ...string) {
	for _, t := range topics {
		b.Publish(t)
	}
}
