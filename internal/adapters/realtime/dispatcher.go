package realtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	deduplicationTTL     = 10 * time.Minute
	deduplicationMaxSize = 10000
)

// Dispatcher is the single entry point for realtime delivery. It publishes
// through the shared bus and applies a deduplicated local fast path.
type Dispatcher struct {
	hub      *Hub
	eventBus EventBus
	seen     *eventIDSet
}

type eventDispatcher interface {
	Publish(context.Context, string, string, int, []byte) error
}

func NewDispatcher(hub *Hub, eventBus EventBus) *Dispatcher {
	return &Dispatcher{
		hub:      hub,
		eventBus: eventBus,
		seen:     newEventIDSet(deduplicationTTL, deduplicationMaxSize),
	}
}

// Run consumes events published by every API instance until ctx is canceled.
func (dispatcher *Dispatcher) Run(ctx context.Context) error {
	if dispatcher == nil || dispatcher.eventBus == nil {
		return fmt.Errorf("running realtime dispatcher: event bus is required")
	}
	return dispatcher.eventBus.Listen(ctx, dispatcher.receive)
}

// Publish persists and distributes an event, then applies the local fast path.
// Callers must invoke it only after their business transaction has committed.
func (dispatcher *Dispatcher) Publish(ctx context.Context, authID, role string, profileID int, payload []byte) error {
	if dispatcher == nil || dispatcher.eventBus == nil || dispatcher.hub == nil {
		return fmt.Errorf("publishing realtime event: dispatcher is not configured")
	}
	event := EventEnvelope{
		ID:              uuid.NewString(),
		TargetAuthID:    authID,
		TargetRole:      role,
		TargetProfileID: profileID,
		Payload:         payload,
	}
	if err := validateEventEnvelope(event); err != nil {
		return fmt.Errorf("publishing realtime event: %w", err)
	}

	if err := dispatcher.eventBus.Publish(ctx, event); err != nil {
		return fmt.Errorf("distributing realtime event: %w", err)
	}
	dispatcher.deliverOnce(event)
	return nil
}

func (dispatcher *Dispatcher) receive(event EventEnvelope) {
	dispatcher.deliverOnce(event)
}

func (dispatcher *Dispatcher) deliverOnce(event EventEnvelope) {
	if dispatcher == nil || dispatcher.hub == nil || !dispatcher.seen.Add(event.ID) {
		return
	}
	dispatcher.hub.Deliver(event)
}

type eventIDSet struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	order   []eventIDEntry
	ttl     time.Duration
	maxSize int
}

type eventIDEntry struct {
	id       string
	received time.Time
}

func newEventIDSet(ttl time.Duration, maxSize int) *eventIDSet {
	return &eventIDSet{
		seen:    make(map[string]time.Time),
		order:   make([]eventIDEntry, 0, 1024),
		ttl:     ttl,
		maxSize: maxSize,
	}
}

func (set *eventIDSet) Add(id string) bool {
	if set == nil || id == "" {
		return false
	}

	now := time.Now()
	set.mu.Lock()
	defer set.mu.Unlock()
	set.removeExpired(now)
	if _, exists := set.seen[id]; exists {
		return false
	}
	set.seen[id] = now
	set.order = append(set.order, eventIDEntry{id: id, received: now})
	for len(set.order) > set.maxSize {
		oldest := set.order[0]
		set.order = set.order[1:]
		delete(set.seen, oldest.id)
	}
	return true
}

func (set *eventIDSet) removeExpired(now time.Time) {
	cutoff := now.Add(-set.ttl)
	firstActive := 0
	for firstActive < len(set.order) && set.order[firstActive].received.Before(cutoff) {
		delete(set.seen, set.order[firstActive].id)
		firstActive++
	}
	set.order = set.order[firstActive:]
}
