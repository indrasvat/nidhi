package core

import (
	"sync"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// Bus is a synchronous event bus implementing plugin.EventBus.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]func(plugin.Event)
}

var _ plugin.EventBus = (*Bus)(nil)

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{handlers: make(map[string][]func(plugin.Event))}
}

// Publish sends an event to all subscribers of its type.
func (b *Bus) Publish(event plugin.Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

// Subscribe registers a handler for events of the given type.
func (b *Bus) Subscribe(eventType string, handler func(plugin.Event)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// SubscriberCount returns the number of subscribers for a given event type.
func (b *Bus) SubscriberCount(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType])
}
