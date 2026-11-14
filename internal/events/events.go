// Package events defines the domain event contract and the synchronous
// publish/subscribe bus. Subscribers are isolated: a panic or error in one
// subscriber is logged and never affects other subscribers or the business
// transaction that published the event.
package events

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Topics of the P0 event set. Once published these are a public contract:
// fields may be added but their meaning never changes.
const (
	TopicTicketCreated        = "ticket.created"
	TopicTicketAssigned       = "ticket.assigned"
	TopicTicketStateChanged   = "ticket.state_changed"
	TopicTicketCommented      = "ticket.commented"
	TopicTicketAttachmentAdded = "ticket.attachment_added"
)

// Event is the envelope published after the business transaction commits.
// EventID lets consumers deduplicate deliveries.
type Event struct {
	Topic      string         `json:"topic"`
	EventID    string         `json:"event_id"`
	TenantID   int64          `json:"tenant_id"`
	HappenedAt time.Time      `json:"happened_at"`
	Payload    map[string]any `json:"payload"`
}

// New builds an event with a fresh id and the current timestamp.
func New(topic string, tenantID int64, payload map[string]any) *Event {
	return &Event{
		Topic:      topic,
		EventID:    uuid.NewString(),
		TenantID:   tenantID,
		HappenedAt: time.Now().UTC(),
		Payload:    payload,
	}
}

// Subscriber consumes domain events. Name identifies it in logs.
type Subscriber interface {
	Name() string
	Handle(ctx context.Context, ev *Event) error
}

// SubscriberFunc adapts a function to Subscriber.
type SubscriberFunc func(ctx context.Context, ev *Event) error

// Name implements Subscriber.
func (f SubscriberFunc) Name() string { return "subscriber_func" }

// Handle implements Subscriber.
func (f SubscriberFunc) Handle(ctx context.Context, ev *Event) error { return f(ctx, ev) }

// Bus is the synchronous in-process event bus.
type Bus struct {
	subscribers []Subscriber
}

// NewBus creates a bus with the given subscribers.
func NewBus(subs ...Subscriber) *Bus {
	return &Bus{subscribers: subs}
}

// Register adds a subscriber after construction.
func (b *Bus) Register(sub Subscriber) {
	b.subscribers = append(b.subscribers, sub)
}

// Publish delivers the event to every subscriber synchronously. Subscriber
// panics and errors are recovered and logged; delivery to remaining
// subscribers always continues.
func (b *Bus) Publish(ctx context.Context, ev *Event) {
	if ev == nil {
		return
	}
	slog.Debug("event published", "topic", ev.Topic, "event_id", ev.EventID, "tenant_id", ev.TenantID)
	for _, sub := range b.subscribers {
		b.deliver(ctx, sub, ev)
	}
}

func (b *Bus) deliver(ctx context.Context, sub Subscriber, ev *Event) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("event subscriber panicked",
				"subscriber", sub.Name(),
				"topic", ev.Topic,
				"event_id", ev.EventID,
				"panic", r,
			)
		}
	}()
	if err := sub.Handle(ctx, ev); err != nil {
		slog.Error("event subscriber failed",
			"subscriber", sub.Name(),
			"topic", ev.Topic,
			"event_id", ev.EventID,
			"error", err,
		)
	}
}
