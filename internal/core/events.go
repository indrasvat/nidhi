package core

import "github.com/indrasvat/nidhi/internal/plugin"

const (
	EventStashesChanged     = "stashes.changed"
	EventModeChanged        = "mode.changed"
	EventCacheInvalidated   = "cache.invalidated"
	EventFilterChanged      = "filter.changed"
	EventSearchQueryChanged = "search.query.changed"
	EventStashDropped       = "stash.dropped"
	EventStashApplied       = "stash.applied"
	EventStashCreated       = "stash.created"
	EventError              = "error"
)

// ModeChangedPayload is the payload for EventModeChanged.
type ModeChangedPayload struct {
	From Mode
	To   Mode
}

// StashMutationPayload is the payload for stash mutation events.
type StashMutationPayload struct {
	Stash plugin.Stash
	SHA   string
}

// ErrorPayload is the payload for EventError.
type ErrorPayload struct {
	Operation string
	Err       error
}

func NewStashesChangedEvent(count int) plugin.Event {
	return plugin.Event{Type: EventStashesChanged, Payload: count}
}

func NewModeChangedEvent(from, to Mode) plugin.Event {
	return plugin.Event{Type: EventModeChanged, Payload: ModeChangedPayload{From: from, To: to}}
}

func NewCacheInvalidatedEvent() plugin.Event {
	return plugin.Event{Type: EventCacheInvalidated}
}

func NewFilterChangedEvent(filters []plugin.Filter) plugin.Event {
	return plugin.Event{Type: EventFilterChanged, Payload: filters}
}

func NewErrorEvent(operation string, err error) plugin.Event {
	return plugin.Event{Type: EventError, Payload: ErrorPayload{Operation: operation, Err: err}}
}

// ─── tea.Msg types for plugin→core communication ────────────

// InfoToastMsg triggers an informational toast notification.
type InfoToastMsg struct {
	Text string
}

// ErrorMsg wraps an error as a tea.Msg.
type ErrorMsg struct {
	Err error
}

// StashMutatedMsg signals that the stash list changed and cache must be invalidated.
type StashMutatedMsg struct{}

// PromptBranchNameMsg signals that the user should be prompted for a branch name.
type PromptBranchNameMsg struct {
	Stash plugin.Stash
}
