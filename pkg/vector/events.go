package vector

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// NodeNotFoundEvent is emitted when no static node meets the similarity threshold.
type NodeNotFoundEvent struct {
	EventID        string
	TrackingULID   string
	StudentID      string
	QueryEmbedding []float32
	BestSimilarity float32
	Threshold      float32
	Timestamp      time.Time
}

// EventHandler consumes NodeNotFound events (in-process bus for this phase).
type EventHandler func(NodeNotFoundEvent)

// EventBus is a simple fan-out bus for live-station fallback events.
type EventBus struct {
	mu       sync.RWMutex
	handlers []EventHandler
}

// NewEventBus creates an empty in-process event bus.
func NewEventBus() *EventBus {
	return &EventBus{}
}

// Subscribe registers a handler for NodeNotFound events.
func (b *EventBus) Subscribe(h EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, h)
}

// EmitNodeNotFound notifies all subscribers.
func (b *EventBus) EmitNodeNotFound(evt NodeNotFoundEvent) {
	b.mu.RLock()
	handlers := append([]EventHandler(nil), b.handlers...)
	b.mu.RUnlock()
	for _, h := range handlers {
		h(evt)
	}
}

// NewTrackingULID generates a chronologically sortable tracking token.
func NewTrackingULID() (string, error) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), entropy)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// NewEventID generates a unique event identifier.
func NewEventID() (string, error) {
	return NewTrackingULID()
}
