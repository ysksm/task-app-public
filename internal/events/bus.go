// Package events is a small in-process pub/sub used to push changes to
// connected UI clients over SSE.
package events

import "sync"

// Event describes a change. Clients refetch the affected data.
type Event struct {
	Type       string `json:"type"` // e.g. issue.created, issue.updated, project.updated
	ProjectKey string `json:"project_key,omitempty"`
	IssueKey   string `json:"issue_key,omitempty"`
	ActorID    string `json:"actor_id,omitempty"`
}

// Bus fans events out to subscribers. Slow subscribers drop events rather
// than blocking writers.
type Bus struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// NewBus creates an empty bus.
func NewBus() *Bus {
	return &Bus{subs: map[chan Event]struct{}{}}
}

// Subscribe returns a channel of events and a function to unsubscribe.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}
}

// Publish sends e to all subscribers.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}
