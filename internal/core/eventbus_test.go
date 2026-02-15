package core

import (
	"sync"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus()
	var received plugin.Event
	bus.Subscribe("test.event", func(e plugin.Event) { received = e })
	bus.Publish(plugin.Event{Type: "test.event", Payload: "hello"})
	if received.Type != "test.event" || received.Payload.(string) != "hello" {
		t.Errorf("received = %+v", received)
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()
	count := 0
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })
	bus.Publish(plugin.Event{Type: "multi"})
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestBus_NoSubscribers(t *testing.T) {
	bus := NewBus()
	bus.Publish(plugin.Event{Type: "nobody"}) // should not panic
}

func TestBus_DifferentEventTypes(t *testing.T) {
	bus := NewBus()
	var aCount, bCount int
	bus.Subscribe("a", func(_ plugin.Event) { aCount++ })
	bus.Subscribe("b", func(_ plugin.Event) { bCount++ })
	bus.Publish(plugin.Event{Type: "a"})
	bus.Publish(plugin.Event{Type: "a"})
	bus.Publish(plugin.Event{Type: "b"})
	if aCount != 2 || bCount != 1 {
		t.Errorf("a=%d b=%d", aCount, bCount)
	}
}

func TestBus_SubscriberCount(t *testing.T) {
	bus := NewBus()
	if bus.SubscriberCount("x") != 0 {
		t.Error("should be 0")
	}
	bus.Subscribe("x", func(_ plugin.Event) {})
	bus.Subscribe("x", func(_ plugin.Event) {})
	if bus.SubscriberCount("x") != 2 {
		t.Errorf("count = %d", bus.SubscriberCount("x"))
	}
}

func TestBus_ConcurrentPublish(t *testing.T) {
	bus := NewBus()
	var mu sync.Mutex
	count := 0
	bus.Subscribe("concurrent", func(_ plugin.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(plugin.Event{Type: "concurrent"})
		}()
	}
	wg.Wait()
	if count != 100 {
		t.Errorf("count = %d, want 100", count)
	}
}

func TestBus_EventConstructors(t *testing.T) {
	tests := []struct {
		name     string
		event    plugin.Event
		wantType string
	}{
		{"stashes changed", NewStashesChangedEvent(5), EventStashesChanged},
		{"mode changed", NewModeChangedEvent(ModeList, ModePreview), EventModeChanged},
		{"cache invalidated", NewCacheInvalidatedEvent(), EventCacheInvalidated},
		{"filter changed", NewFilterChangedEvent(nil), EventFilterChanged},
		{"error", NewErrorEvent("test", nil), EventError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.event.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", tt.event.Type, tt.wantType)
			}
		})
	}
}
