package core

import (
	"errors"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestModeChangedPayload(t *testing.T) {
	e := NewModeChangedEvent(ModeList, ModePreview)
	payload, ok := e.Payload.(ModeChangedPayload)
	if !ok {
		t.Fatalf("payload type = %T", e.Payload)
	}
	if payload.From != ModeList || payload.To != ModePreview {
		t.Errorf("From=%s To=%s", payload.From, payload.To)
	}
}

func TestErrorPayload(t *testing.T) {
	err := errors.New("git failed")
	e := NewErrorEvent("stash apply", err)
	payload, ok := e.Payload.(ErrorPayload)
	if !ok {
		t.Fatalf("payload type = %T", e.Payload)
	}
	if payload.Operation != "stash apply" || payload.Err.Error() != "git failed" {
		t.Errorf("payload = %+v", payload)
	}
}

func TestStashMutationPayload(t *testing.T) {
	p := StashMutationPayload{Stash: plugin.Stash{Index: 0, SHA: "abc123"}, SHA: "abc123"}
	if p.SHA != "abc123" {
		t.Errorf("SHA = %q", p.SHA)
	}
}

func TestEventTypeConstants(t *testing.T) {
	types := []string{
		EventStashesChanged, EventModeChanged, EventCacheInvalidated,
		EventFilterChanged, EventSearchQueryChanged, EventStashDropped,
		EventStashApplied, EventStashCreated, EventError,
	}
	seen := make(map[string]bool)
	for _, typ := range types {
		if typ == "" {
			t.Error("empty event type")
		}
		if seen[typ] {
			t.Errorf("duplicate: %q", typ)
		}
		seen[typ] = true
	}
}
