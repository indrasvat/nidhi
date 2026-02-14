# Task 006: Core BubbleTea Model and Mode Management

## Status: TODO

## Depends On
- 005 (Plugin interfaces, registry, PluginContext -- defines Plugin, AppState, Mode, EventBus, all types this task consumes)
- 004 (StashCache -- needed to populate AppState.Stashes on Init)
- 003 (Theme -- needed for View rendering and PluginContext)

## Parallelizable With
- None (this is foundational -- Tasks 007, 008, 009 all depend on this)

## Problem
The core BubbleTea model is the central nervous system of nidhi. It owns the application lifecycle: initializing services, routing messages to the correct mode handler, managing mode transitions (LIST -> PREVIEW -> DETAIL, etc.), maintaining the canonical AppState, and composing the final `tea.View` struct. Without this, no screen can render, no key can be handled, and no plugin can be invoked. The mode manager must enforce valid transitions and support a push/pop stack so `Esc` always returns to the previous mode (PRD Section 6.1 FR-03.4).

## PRD Reference
- Section 8.1 (High-Level Architecture) -- Core Engine box
- Section 8.2 (Core Interfaces) -- PluginContext, AppState usage
- Section 8.3 (Core Types) -- AppState struct, EventBus interface
- Section 8.4 (Module Structure) -- `internal/core/` files
- Section 6.1 FR-03 (Mode Switching) -- List, Preview, Detail, Esc behavior
- Section 13.1 (BubbleTea v2 Features) -- tea.View struct, tea.KeyPressMsg, WindowSizeMsg
- Section 9.3 (Layout Contract) -- status bar + content + footer structure

## Files to Create
- `internal/core/mode.go` -- Mode enum (re-exports from plugin pkg), ModeManager with push/pop stack
- `internal/core/state.go` -- AppState factory, snapshot, state mutation helpers
- `internal/core/events.go` -- Internal event types (StashesChanged, ModeChanged, etc.)
- `internal/core/eventbus.go` -- EventBus implementation (satisfies plugin.EventBus interface)
- `internal/core/app.go` -- Top-level Model implementing tea.Model (Init, Update, View)
- `internal/core/mode_test.go` -- Mode transition tests
- `internal/core/state_test.go` -- AppState factory and snapshot tests
- `internal/core/events_test.go` -- Event type tests
- `internal/core/eventbus_test.go` -- EventBus publish/subscribe tests
- `internal/core/app_test.go` -- Model.Update handles WindowSizeMsg, key dispatch, mode transitions

## Execution Steps

### Step 1: Create `internal/core/mode.go`

The ModeManager maintains a stack of modes for push/pop navigation. The Mode type itself is re-exported from the plugin package to avoid circular imports.

```go
// internal/core/mode.go
package core

import (
	"fmt"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// Mode is an alias for plugin.Mode to avoid circular imports.
// All mode constants are re-exported from the plugin package.
type Mode = plugin.Mode

// Re-export mode constants for convenience within the core package.
const (
	ModeList     = plugin.ModeList
	ModePreview  = plugin.ModePreview
	ModeDetail   = plugin.ModeDetail
	ModeSearch   = plugin.ModeSearch
	ModeNewStash = plugin.ModeNewStash
	ModeExport   = plugin.ModeExport
	ModeImport   = plugin.ModeImport
	ModeConflict = plugin.ModeConflict
	ModeHelp     = plugin.ModeHelp
)

// maxModeStackDepth prevents unbounded stack growth from buggy transitions.
const maxModeStackDepth = 20

// ModeManager manages mode transitions with a push/pop stack.
// The stack enables "Esc always goes back" behavior (PRD FR-03.4).
type ModeManager struct {
	stack []Mode
}

// NewModeManager creates a ModeManager starting in the given initial mode.
func NewModeManager(initial Mode) *ModeManager {
	return &ModeManager{
		stack: []Mode{initial},
	}
}

// Current returns the current (top of stack) mode.
func (m *ModeManager) Current() Mode {
	if len(m.stack) == 0 {
		return ModeList // Safety fallback
	}
	return m.stack[len(m.stack)-1]
}

// Push transitions to a new mode by pushing it onto the stack.
// Returns an error if the transition is invalid or stack is full.
func (m *ModeManager) Push(mode Mode) error {
	if len(m.stack) >= maxModeStackDepth {
		return fmt.Errorf("mode stack overflow: depth %d", len(m.stack))
	}

	current := m.Current()
	if !isValidTransition(current, mode) {
		return fmt.Errorf("invalid mode transition: %s -> %s", current, mode)
	}

	m.stack = append(m.stack, mode)
	return nil
}

// Pop returns to the previous mode. If already at the root (LIST), it's a no-op.
// Returns the new current mode after popping.
func (m *ModeManager) Pop() Mode {
	if len(m.stack) <= 1 {
		// At root; can't go further back.
		return m.Current()
	}
	m.stack = m.stack[:len(m.stack)-1]
	return m.Current()
}

// Reset clears the stack and sets the mode to the given mode.
// Used for hard resets (e.g., after a critical error).
func (m *ModeManager) Reset(mode Mode) {
	m.stack = []Mode{mode}
}

// Depth returns the current stack depth.
func (m *ModeManager) Depth() int {
	return len(m.stack)
}

// History returns a copy of the mode stack (bottom to top).
func (m *ModeManager) History() []Mode {
	result := make([]Mode, len(m.stack))
	copy(result, m.stack)
	return result
}

// isValidTransition checks whether transitioning from `from` to `to` is allowed.
// Transition rules from PRD FR-03:
//   - LIST -> PREVIEW, DETAIL, SEARCH, NEWSTASH, EXPORT, IMPORT, HELP, CONFLICT
//   - PREVIEW -> LIST, DETAIL, SEARCH, HELP
//   - DETAIL -> LIST, PREVIEW, HELP
//   - SEARCH -> LIST (jump to result)
//   - NEWSTASH -> LIST (after create or cancel)
//   - EXPORT -> LIST (after export or cancel)
//   - IMPORT -> LIST (after import or cancel)
//   - CONFLICT -> LIST, DETAIL (after action or cancel)
//   - HELP -> any previous (pop only)
//   - Any mode -> HELP (push)
func isValidTransition(from, to Mode) bool {
	// Help is always accessible from any mode.
	if to == ModeHelp {
		return true
	}

	switch from {
	case ModeList:
		return to == ModePreview || to == ModeDetail || to == ModeSearch ||
			to == ModeNewStash || to == ModeExport || to == ModeImport ||
			to == ModeConflict || to == ModeHelp
	case ModePreview:
		return to == ModeList || to == ModeDetail || to == ModeSearch || to == ModeHelp
	case ModeDetail:
		return to == ModeList || to == ModePreview || to == ModeHelp
	case ModeSearch:
		return to == ModeList || to == ModePreview || to == ModeHelp
	case ModeNewStash:
		return to == ModeList || to == ModeHelp
	case ModeExport:
		return to == ModeList || to == ModeHelp
	case ModeImport:
		return to == ModeList || to == ModeHelp
	case ModeConflict:
		return to == ModeList || to == ModeDetail || to == ModeHelp
	case ModeHelp:
		// Help pops, doesn't push to new modes. But allow List as fallback.
		return to == ModeList
	default:
		return false
	}
}

// IsValidTransition is the exported version for testing.
func IsValidTransition(from, to Mode) bool {
	return isValidTransition(from, to)
}
```

### Step 2: Create `internal/core/state.go`

AppState factory and immutable snapshot helpers. AppState is a value type (struct) so copying is automatic.

```go
// internal/core/state.go
package core

import "github.com/indrasvat/nidhi/internal/plugin"

// AppState is a type alias for plugin.AppState. All state lives in the plugin
// package to avoid circular imports. Core operates on the same type.
type AppState = plugin.AppState

// Stash is a type alias for plugin.Stash.
type Stash = plugin.Stash

// Filter is a type alias for plugin.Filter.
type Filter = plugin.Filter

// GitVersion is a type alias for plugin.GitVersion.
type GitVersion = plugin.GitVersion

// NewAppState creates an AppState with sensible defaults.
func NewAppState(repoPath, branch string, gitVer GitVersion) AppState {
	return AppState{
		Mode:       ModeList,
		Stashes:    nil,
		Cursor:     0,
		Filters:    nil,
		SearchQuery: "",
		Width:      80,
		Height:     24,
		GitVersion: gitVer,
		RepoPath:   repoPath,
		Branch:     branch,
	}
}

// WithStashes returns a copy of the state with updated stashes.
// Cursor is clamped to the valid range.
func WithStashes(s AppState, stashes []Stash) AppState {
	s.Stashes = stashes
	if s.Cursor >= len(stashes) {
		s.Cursor = max(0, len(stashes)-1)
	}
	return s
}

// WithCursor returns a copy with the cursor moved, clamped to [0, len(Stashes)-1].
func WithCursor(s AppState, cursor int) AppState {
	if len(s.Stashes) == 0 {
		s.Cursor = 0
		return s
	}
	s.Cursor = clamp(cursor, 0, len(s.Stashes)-1)
	return s
}

// WithSize returns a copy with updated terminal dimensions.
func WithSize(s AppState, width, height int) AppState {
	s.Width = width
	s.Height = height
	return s
}

// WithMode returns a copy with the mode changed.
func WithMode(s AppState, mode Mode) AppState {
	s.Mode = mode
	return s
}

// WithFilters returns a copy with updated filters.
func WithFilters(s AppState, filters []Filter) AppState {
	s.Filters = filters
	return s
}

// WithSearchQuery returns a copy with updated search query.
func WithSearchQuery(s AppState, query string) AppState {
	s.SearchQuery = query
	return s
}

// SelectedStash returns the currently selected stash, or nil if none.
func SelectedStash(s AppState) *Stash {
	if len(s.Stashes) == 0 || s.Cursor < 0 || s.Cursor >= len(s.Stashes) {
		return nil
	}
	stash := s.Stashes[s.Cursor]
	return &stash
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
```

### Step 3: Create `internal/core/events.go`

Define event type constants and typed event constructors for the EventBus.

```go
// internal/core/events.go
package core

import "github.com/indrasvat/nidhi/internal/plugin"

// Event type constants for the internal event bus.
const (
	EventStashesChanged    = "stashes.changed"
	EventModeChanged       = "mode.changed"
	EventCacheInvalidated  = "cache.invalidated"
	EventFilterChanged     = "filter.changed"
	EventSearchQueryChanged = "search.query.changed"
	EventStashDropped      = "stash.dropped"
	EventStashApplied      = "stash.applied"
	EventStashCreated      = "stash.created"
	EventError             = "error"
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

// NewStashesChangedEvent creates a stashes.changed event.
func NewStashesChangedEvent(count int) plugin.Event {
	return plugin.Event{Type: EventStashesChanged, Payload: count}
}

// NewModeChangedEvent creates a mode.changed event.
func NewModeChangedEvent(from, to Mode) plugin.Event {
	return plugin.Event{Type: EventModeChanged, Payload: ModeChangedPayload{From: from, To: to}}
}

// NewCacheInvalidatedEvent creates a cache.invalidated event.
func NewCacheInvalidatedEvent() plugin.Event {
	return plugin.Event{Type: EventCacheInvalidated, Payload: nil}
}

// NewFilterChangedEvent creates a filter.changed event.
func NewFilterChangedEvent(filters []plugin.Filter) plugin.Event {
	return plugin.Event{Type: EventFilterChanged, Payload: filters}
}

// NewErrorEvent creates an error event.
func NewErrorEvent(operation string, err error) plugin.Event {
	return plugin.Event{Type: EventError, Payload: ErrorPayload{Operation: operation, Err: err}}
}
```

### Step 4: Create `internal/core/eventbus.go`

Simple synchronous EventBus implementation that satisfies `plugin.EventBus`.

```go
// internal/core/eventbus.go
package core

import (
	"sync"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// Bus is a simple synchronous event bus implementing plugin.EventBus.
// Handlers are called synchronously in registration order.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]func(plugin.Event)
}

// Compile-time interface check.
var _ plugin.EventBus = (*Bus)(nil)

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]func(plugin.Event)),
	}
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
// Useful for testing.
func (b *Bus) SubscriberCount(eventType string) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType])
}
```

### Step 5: Create `internal/core/app.go`

Top-level BubbleTea model. This is the heart of the application. It owns all state and coordinates everything.

```go
// internal/core/app.go
package core

import (
	"context"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// Model is the top-level BubbleTea model for nidhi.
// It implements tea.Model and owns all application state.
type Model struct {
	// State
	state AppState
	modes *ModeManager

	// Core services
	pluginCtx plugin.PluginContext
	bus       *Bus
	logger    *slog.Logger

	// Plugin registries
	keyHandlers     *plugin.Registry[plugin.KeyHandler]
	screenProviders *plugin.Registry[plugin.ScreenProvider]
	stashHooks      *plugin.Registry[plugin.StashHook]

	// Flags
	ready bool // Set after first WindowSizeMsg
}

// New creates a new Model with the provided dependencies.
func New(
	state AppState,
	pctx plugin.PluginContext,
	bus *Bus,
	logger *slog.Logger,
	keyHandlers *plugin.Registry[plugin.KeyHandler],
	screenProviders *plugin.Registry[plugin.ScreenProvider],
	stashHooks *plugin.Registry[plugin.StashHook],
) *Model {
	return &Model{
		state:           state,
		modes:           NewModeManager(state.Mode),
		pluginCtx:       pctx,
		bus:             bus,
		logger:          logger,
		keyHandlers:     keyHandlers,
		screenProviders: screenProviders,
		stashHooks:      stashHooks,
		ready:           false,
	}
}

// Init implements tea.Model. Called once on startup.
// Returns a Cmd to load initial stash data.
func (m *Model) Init() tea.Cmd {
	return m.loadStashes()
}

// Update implements tea.Model. Processes all incoming messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case stashesLoadedMsg:
		return m.handleStashesLoaded(msg)

	case errMsg:
		return m.handleError(msg)
	}

	return m, nil
}

// View implements tea.Model. Returns a tea.View struct.
// Content is a tea.Layer (interface), NOT a string.
func (m *Model) View() tea.View {
	if !m.ready {
		// Before first WindowSizeMsg, render a loading placeholder.
		return tea.View{
			AltScreen: true,
			Content:   tea.NewLayer("Loading nidhi..."),
		}
	}

	// Render content based on current mode.
	// For now, render a placeholder. Layout engine (Task 007) and
	// screen renderers (Tasks 008-009) will replace this.
	content := m.renderContent()

	return tea.View{
		AltScreen: true,
		Content:   tea.NewLayer(content),
	}
}

// State returns a copy of the current application state.
func (m *Model) State() AppState {
	return m.state
}

// ─── Internal message types ─────────────────────────────────

// stashesLoadedMsg is sent when stash data has been loaded from git.
type stashesLoadedMsg struct {
	stashes []plugin.Stash
}

// errMsg is sent when an async operation fails.
type errMsg struct {
	operation string
	err       error
}

// ─── Message handlers ───────────────────────────────────────

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.state = WithSize(m.state, msg.Width, msg.Height)
	m.ready = true
	m.logger.Debug("window resized", "width", msg.Width, "height", msg.Height)
	return m, nil
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Global keys first.
	switch {
	case msg.Text == "q" || (msg.Text == "c" && msg.Mod == tea.ModCtrl):
		return m, tea.Quit

	case msg.Text == "?" :
		if m.modes.Current() == ModeHelp {
			m.popMode()
		} else {
			m.pushMode(ModeHelp)
		}
		return m, nil

	case msg.Code == tea.KeyEscape:
		m.popMode()
		return m, nil
	}

	// Mode-specific key handling.
	// Delegate to the core key dispatcher for built-in keys.
	switch m.modes.Current() {
	case ModeList:
		return m.handleListKeys(msg)
	case ModePreview:
		return m.handlePreviewKeys(msg)
	}

	// Delegate to plugin key handlers.
	return m.delegateToPluginKeyHandlers(msg)
}

func (m *Model) handleStashesLoaded(msg stashesLoadedMsg) (tea.Model, tea.Cmd) {
	m.state = WithStashes(m.state, msg.stashes)
	m.bus.Publish(NewStashesChangedEvent(len(msg.stashes)))
	m.logger.Info("stashes loaded", "count", len(msg.stashes))
	return m, nil
}

func (m *Model) handleError(msg errMsg) (tea.Model, tea.Cmd) {
	m.logger.Error("operation failed", "op", msg.operation, "error", msg.err)
	m.bus.Publish(NewErrorEvent(msg.operation, msg.err))
	return m, nil
}

// ─── Built-in key handlers ──────────────────────────────────

func (m *Model) handleListKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Text {
	case "j":
		m.state = WithCursor(m.state, m.state.Cursor+1)
		return m, nil
	case "k":
		m.state = WithCursor(m.state, m.state.Cursor-1)
		return m, nil
	case "g":
		m.state = WithCursor(m.state, 0)
		return m, nil
	case "G":
		m.state = WithCursor(m.state, len(m.state.Stashes)-1)
		return m, nil
	}

	// Tab -> Preview, Enter -> Detail.
	switch msg.Code {
	case tea.KeyTab:
		m.pushMode(ModePreview)
		return m, nil
	case tea.KeyEnter:
		m.pushMode(ModeDetail)
		return m, nil
	}

	return m.delegateToPluginKeyHandlers(msg)
}

func (m *Model) handlePreviewKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Text {
	case "j":
		m.state = WithCursor(m.state, m.state.Cursor+1)
		return m, nil
	case "k":
		m.state = WithCursor(m.state, m.state.Cursor-1)
		return m, nil
	}

	switch msg.Code {
	case tea.KeyTab:
		// Toggle back to list.
		m.popMode()
		return m, nil
	case tea.KeyEnter:
		m.pushMode(ModeDetail)
		return m, nil
	}

	return m.delegateToPluginKeyHandlers(msg)
}

// ─── Plugin delegation ──────────────────────────────────────

func (m *Model) delegateToPluginKeyHandlers(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keyEvent := plugin.KeyEvent{
		Key:  msg.Text,
		Mod:  int(msg.Mod),
		Mode: m.modes.Current(),
	}

	for _, handler := range m.keyHandlers.All() {
		for _, kb := range handler.KeyBindings() {
			if kb.Key != msg.Text {
				continue
			}
			if len(kb.Modes) > 0 && !modeInSlice(m.modes.Current(), kb.Modes) {
				continue
			}
			newState, cmd := handler.HandleKey(keyEvent, m.state)
			m.state = newState
			return m, cmd
		}
	}

	return m, nil
}

// ─── Mode helpers ───────────────────────────────────────────

func (m *Model) pushMode(mode Mode) {
	prev := m.modes.Current()
	if err := m.modes.Push(mode); err != nil {
		m.logger.Warn("mode push failed", "from", prev, "to", mode, "error", err)
		return
	}
	m.state = WithMode(m.state, mode)
	m.bus.Publish(NewModeChangedEvent(prev, mode))
}

func (m *Model) popMode() {
	prev := m.modes.Current()
	newMode := m.modes.Pop()
	m.state = WithMode(m.state, newMode)
	if prev != newMode {
		m.bus.Publish(NewModeChangedEvent(prev, newMode))
	}
}

// ─── Async commands ─────────────────────────────────────────

func (m *Model) loadStashes() tea.Cmd {
	cache := m.pluginCtx.Cache
	return func() tea.Msg {
		stashes, err := cache.List(context.Background())
		if err != nil {
			return errMsg{operation: "load stashes", err: err}
		}
		return stashesLoadedMsg{stashes: stashes}
	}
}

// ─── Rendering ──────────────────────────────────────────────

func (m *Model) renderContent() string {
	// Placeholder content renderer. The layout engine (Task 007) will replace this
	// with proper status bar + content area + footer composition.
	// Screen renderers (Tasks 008-009) will provide mode-specific content.

	mode := m.modes.Current()
	stashCount := len(m.state.Stashes)

	switch mode {
	case ModeList:
		if stashCount == 0 {
			return "No stashes. Press 'n' to create one."
		}
		return "LIST mode — stash list placeholder"
	case ModePreview:
		return "PREVIEW mode — split view placeholder"
	case ModeDetail:
		return "DETAIL mode — file tree + diff placeholder"
	case ModeHelp:
		return "HELP — press ? or Esc to close"
	default:
		return mode.String() + " mode"
	}
}

// ─── Utility ────────────────────────────────────────────────

func modeInSlice(mode Mode, modes []Mode) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}
```

### Step 6: Create test files

#### `internal/core/mode_test.go`

```go
// internal/core/mode_test.go
package core

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestModeManager_InitialMode(t *testing.T) {
	mm := NewModeManager(ModeList)
	if mm.Current() != ModeList {
		t.Errorf("Current() = %s, want LIST", mm.Current())
	}
	if mm.Depth() != 1 {
		t.Errorf("Depth() = %d, want 1", mm.Depth())
	}
}

func TestModeManager_PushPop(t *testing.T) {
	mm := NewModeManager(ModeList)

	// LIST -> PREVIEW
	if err := mm.Push(ModePreview); err != nil {
		t.Fatalf("Push(PREVIEW) error: %v", err)
	}
	if mm.Current() != ModePreview {
		t.Errorf("after Push: Current() = %s, want PREVIEW", mm.Current())
	}
	if mm.Depth() != 2 {
		t.Errorf("after Push: Depth() = %d, want 2", mm.Depth())
	}

	// PREVIEW -> DETAIL
	if err := mm.Push(ModeDetail); err != nil {
		t.Fatalf("Push(DETAIL) error: %v", err)
	}
	if mm.Current() != ModeDetail {
		t.Errorf("after second Push: Current() = %s, want DETAIL", mm.Current())
	}

	// Pop back to PREVIEW
	got := mm.Pop()
	if got != ModePreview {
		t.Errorf("Pop() returned %s, want PREVIEW", got)
	}

	// Pop back to LIST
	got = mm.Pop()
	if got != ModeList {
		t.Errorf("Pop() returned %s, want LIST", got)
	}

	// Pop at root is a no-op
	got = mm.Pop()
	if got != ModeList {
		t.Errorf("Pop() at root returned %s, want LIST", got)
	}
	if mm.Depth() != 1 {
		t.Errorf("Depth() at root = %d, want 1", mm.Depth())
	}
}

func TestModeManager_InvalidTransition(t *testing.T) {
	mm := NewModeManager(ModeDetail)

	// DETAIL -> EXPORT is not valid.
	err := mm.Push(ModeExport)
	if err == nil {
		t.Error("Push(EXPORT) from DETAIL should fail")
	}
	// Mode should not have changed.
	if mm.Current() != ModeDetail {
		t.Errorf("after invalid Push: Current() = %s, want DETAIL", mm.Current())
	}
}

func TestModeManager_HelpFromAnyMode(t *testing.T) {
	modes := []Mode{
		ModeList, ModePreview, ModeDetail, ModeSearch,
		ModeNewStash, ModeExport, ModeImport, ModeConflict,
	}

	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			mm := NewModeManager(ModeList)
			// If not already in the target mode, push to it first.
			if mode != ModeList {
				// Force the stack to start at the right mode.
				mm.Reset(mode)
			}
			err := mm.Push(ModeHelp)
			if err != nil {
				t.Errorf("Push(HELP) from %s should succeed, got: %v", mode, err)
			}
		})
	}
}

func TestModeManager_Reset(t *testing.T) {
	mm := NewModeManager(ModeList)
	mm.Push(ModePreview)
	mm.Push(ModeDetail)

	mm.Reset(ModeList)

	if mm.Current() != ModeList {
		t.Errorf("after Reset: Current() = %s, want LIST", mm.Current())
	}
	if mm.Depth() != 1 {
		t.Errorf("after Reset: Depth() = %d, want 1", mm.Depth())
	}
}

func TestModeManager_History(t *testing.T) {
	mm := NewModeManager(ModeList)
	mm.Push(ModePreview)
	mm.Push(ModeHelp)

	history := mm.History()
	want := []Mode{ModeList, ModePreview, ModeHelp}
	if len(history) != len(want) {
		t.Fatalf("History() length = %d, want %d", len(history), len(want))
	}
	for i, m := range want {
		if history[i] != m {
			t.Errorf("History()[%d] = %s, want %s", i, history[i], m)
		}
	}
}

func TestModeManager_StackOverflow(t *testing.T) {
	mm := NewModeManager(ModeList)

	// Push Help -> pop -> push Help repeatedly to fill the stack.
	for i := 0; i < maxModeStackDepth-1; i++ {
		// Alternate between modes to avoid invalid transitions.
		if err := mm.Push(ModeHelp); err != nil {
			// Reset and try a valid path.
			mm.Reset(ModeList)
			break
		}
		mm.Pop()
	}

	// Fill the stack to the limit by pushing valid transitions.
	mm.Reset(ModeList)
	for mm.Depth() < maxModeStackDepth {
		if err := mm.Push(ModeHelp); err != nil {
			break
		}
	}

	err := mm.Push(ModeHelp)
	if err == nil && mm.Depth() > maxModeStackDepth {
		t.Error("Push() should fail when stack is at max depth")
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from plugin.Mode
		to   plugin.Mode
		want bool
	}{
		// LIST transitions
		{ModeList, ModePreview, true},
		{ModeList, ModeDetail, true},
		{ModeList, ModeSearch, true},
		{ModeList, ModeNewStash, true},
		{ModeList, ModeExport, true},
		{ModeList, ModeImport, true},
		{ModeList, ModeConflict, true},
		{ModeList, ModeHelp, true},

		// PREVIEW transitions
		{ModePreview, ModeList, true},
		{ModePreview, ModeDetail, true},
		{ModePreview, ModeSearch, true},
		{ModePreview, ModeExport, false},
		{ModePreview, ModeImport, false},

		// DETAIL transitions
		{ModeDetail, ModeList, true},
		{ModeDetail, ModePreview, true},
		{ModeDetail, ModeHelp, true},
		{ModeDetail, ModeExport, false},
		{ModeDetail, ModeNewStash, false},

		// SEARCH transitions
		{ModeSearch, ModeList, true},
		{ModeSearch, ModePreview, true},
		{ModeSearch, ModeDetail, false},

		// Modal screens -> LIST only
		{ModeNewStash, ModeList, true},
		{ModeNewStash, ModeDetail, false},
		{ModeExport, ModeList, true},
		{ModeImport, ModeList, true},

		// CONFLICT transitions
		{ModeConflict, ModeList, true},
		{ModeConflict, ModeDetail, true},
		{ModeConflict, ModePreview, false},

		// HELP -> only LIST (pop handles the rest)
		{ModeHelp, ModeList, true},
		{ModeHelp, ModeDetail, false},
	}

	for _, tt := range tests {
		name := tt.from.String() + "->" + tt.to.String()
		t.Run(name, func(t *testing.T) {
			got := IsValidTransition(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("IsValidTransition(%s, %s) = %v, want %v",
					tt.from, tt.to, got, tt.want)
			}
		})
	}
}
```

#### `internal/core/state_test.go`

```go
// internal/core/state_test.go
package core

import (
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestNewAppState(t *testing.T) {
	ver := GitVersion{Major: 2, Minor: 53, Patch: 0, Raw: "git version 2.53.0"}
	s := NewAppState("/path/to/repo", "main", ver)

	if s.Mode != ModeList {
		t.Errorf("Mode = %s, want LIST", s.Mode)
	}
	if s.Width != 80 || s.Height != 24 {
		t.Errorf("dimensions = %dx%d, want 80x24", s.Width, s.Height)
	}
	if s.RepoPath != "/path/to/repo" {
		t.Errorf("RepoPath = %q", s.RepoPath)
	}
	if s.Branch != "main" {
		t.Errorf("Branch = %q", s.Branch)
	}
	if s.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", s.Cursor)
	}
}

func TestWithStashes(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	stashes := []plugin.Stash{
		{Index: 0, SHA: "aaa", Message: "first"},
		{Index: 1, SHA: "bbb", Message: "second"},
		{Index: 2, SHA: "ccc", Message: "third"},
	}
	s = WithStashes(s, stashes)

	if len(s.Stashes) != 3 {
		t.Fatalf("len(Stashes) = %d, want 3", len(s.Stashes))
	}
	if s.Stashes[0].SHA != "aaa" {
		t.Errorf("Stashes[0].SHA = %q", s.Stashes[0].SHA)
	}
}

func TestWithStashes_ClampsCursor(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s.Cursor = 5 // Out of range

	stashes := []plugin.Stash{
		{Index: 0, SHA: "aaa"},
		{Index: 1, SHA: "bbb"},
	}
	s = WithStashes(s, stashes)

	if s.Cursor != 1 {
		t.Errorf("Cursor should be clamped to 1, got %d", s.Cursor)
	}
}

func TestWithStashes_EmptyClampsCursorToZero(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s.Cursor = 3

	s = WithStashes(s, nil)

	if s.Cursor != 0 {
		t.Errorf("Cursor with empty stashes should be 0, got %d", s.Cursor)
	}
}

func TestWithCursor(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		cursor int
		want   int
	}{
		{"normal", 5, 2, 2},
		{"clamp below", 5, -1, 0},
		{"clamp above", 5, 10, 4},
		{"first", 5, 0, 0},
		{"last", 5, 4, 4},
		{"empty stashes", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewAppState("/repo", "main", GitVersion{})
			stashes := make([]plugin.Stash, tt.count)
			for i := range stashes {
				stashes[i] = plugin.Stash{Index: i}
			}
			s = WithStashes(s, stashes)
			s = WithCursor(s, tt.cursor)

			if s.Cursor != tt.want {
				t.Errorf("WithCursor(%d) on %d stashes = %d, want %d",
					tt.cursor, tt.count, s.Cursor, tt.want)
			}
		})
	}
}

func TestWithSize(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s = WithSize(s, 200, 60)

	if s.Width != 200 || s.Height != 60 {
		t.Errorf("dimensions = %dx%d, want 200x60", s.Width, s.Height)
	}
}

func TestWithMode(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s = WithMode(s, ModePreview)

	if s.Mode != ModePreview {
		t.Errorf("Mode = %s, want PREVIEW", s.Mode)
	}
}

func TestSelectedStash(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	stashes := []plugin.Stash{
		{Index: 0, SHA: "aaa", Message: "first"},
		{Index: 1, SHA: "bbb", Message: "second"},
	}
	s = WithStashes(s, stashes)
	s = WithCursor(s, 1)

	got := SelectedStash(s)
	if got == nil {
		t.Fatal("SelectedStash() returned nil")
	}
	if got.SHA != "bbb" {
		t.Errorf("SelectedStash().SHA = %q, want %q", got.SHA, "bbb")
	}
}

func TestSelectedStash_Empty(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	got := SelectedStash(s)
	if got != nil {
		t.Errorf("SelectedStash() on empty should be nil, got %+v", got)
	}
}

func TestStateImmutability(t *testing.T) {
	s1 := NewAppState("/repo", "main", GitVersion{})
	stashes := []plugin.Stash{{Index: 0, SHA: "aaa"}}
	s1 = WithStashes(s1, stashes)

	// WithCursor returns a new copy; s1 should not be mutated.
	s2 := WithCursor(s1, 0)
	_ = WithMode(s2, ModePreview)

	// s1 should still be LIST mode.
	if s1.Mode != ModeList {
		t.Error("original state was mutated by With* functions")
	}
}

func TestWithFilters(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	filters := []plugin.Filter{
		{ID: "branch", Label: "main", Value: "main"},
	}
	s = WithFilters(s, filters)

	if len(s.Filters) != 1 {
		t.Fatalf("len(Filters) = %d, want 1", len(s.Filters))
	}
	if s.Filters[0].ID != "branch" {
		t.Errorf("Filters[0].ID = %q", s.Filters[0].ID)
	}
}

func TestWithSearchQuery(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s = WithSearchQuery(s, "token refresh")

	if s.SearchQuery != "token refresh" {
		t.Errorf("SearchQuery = %q, want %q", s.SearchQuery, "token refresh")
	}
}

// Ensure Stash fields work with real time values.
func TestStashDateField(t *testing.T) {
	now := time.Now()
	s := plugin.Stash{
		Index: 0,
		Date:  now,
	}
	if s.Date.IsZero() {
		t.Error("Stash.Date should not be zero")
	}
}
```

#### `internal/core/eventbus_test.go`

```go
// internal/core/eventbus_test.go
package core

import (
	"sync"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestBus_PublishSubscribe(t *testing.T) {
	bus := NewBus()
	var received plugin.Event

	bus.Subscribe("test.event", func(e plugin.Event) {
		received = e
	})

	bus.Publish(plugin.Event{Type: "test.event", Payload: "hello"})

	if received.Type != "test.event" {
		t.Errorf("received Type = %q, want %q", received.Type, "test.event")
	}
	if received.Payload.(string) != "hello" {
		t.Errorf("received Payload = %v, want %q", received.Payload, "hello")
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()
	var count int

	bus.Subscribe("multi", func(_ plugin.Event) { count++ })
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })

	bus.Publish(plugin.Event{Type: "multi"})

	if count != 3 {
		t.Errorf("expected 3 handlers called, got %d", count)
	}
}

func TestBus_NoSubscribers(t *testing.T) {
	bus := NewBus()
	// Should not panic.
	bus.Publish(plugin.Event{Type: "nobody.listening"})
}

func TestBus_DifferentEventTypes(t *testing.T) {
	bus := NewBus()
	var aCount, bCount int

	bus.Subscribe("type.a", func(_ plugin.Event) { aCount++ })
	bus.Subscribe("type.b", func(_ plugin.Event) { bCount++ })

	bus.Publish(plugin.Event{Type: "type.a"})
	bus.Publish(plugin.Event{Type: "type.a"})
	bus.Publish(plugin.Event{Type: "type.b"})

	if aCount != 2 {
		t.Errorf("type.a handler called %d times, want 2", aCount)
	}
	if bCount != 1 {
		t.Errorf("type.b handler called %d times, want 1", bCount)
	}
}

func TestBus_SubscriberCount(t *testing.T) {
	bus := NewBus()

	if bus.SubscriberCount("x") != 0 {
		t.Error("empty bus should have 0 subscribers")
	}

	bus.Subscribe("x", func(_ plugin.Event) {})
	bus.Subscribe("x", func(_ plugin.Event) {})

	if bus.SubscriberCount("x") != 2 {
		t.Errorf("SubscriberCount = %d, want 2", bus.SubscriberCount("x"))
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
		t.Errorf("concurrent publish: count = %d, want 100", count)
	}
}

func TestBus_EventConstructors(t *testing.T) {
	tests := []struct {
		name string
		event plugin.Event
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
				t.Errorf("event.Type = %q, want %q", tt.event.Type, tt.wantType)
			}
		})
	}
}
```

#### `internal/core/events_test.go`

```go
// internal/core/events_test.go
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
		t.Fatalf("payload type = %T, want ModeChangedPayload", e.Payload)
	}
	if payload.From != ModeList {
		t.Errorf("From = %s, want LIST", payload.From)
	}
	if payload.To != ModePreview {
		t.Errorf("To = %s, want PREVIEW", payload.To)
	}
}

func TestErrorPayload(t *testing.T) {
	err := errors.New("git command failed")
	e := NewErrorEvent("stash apply", err)
	payload, ok := e.Payload.(ErrorPayload)
	if !ok {
		t.Fatalf("payload type = %T, want ErrorPayload", e.Payload)
	}
	if payload.Operation != "stash apply" {
		t.Errorf("Operation = %q", payload.Operation)
	}
	if payload.Err.Error() != "git command failed" {
		t.Errorf("Err = %v", payload.Err)
	}
}

func TestStashMutationPayload(t *testing.T) {
	p := StashMutationPayload{
		Stash: plugin.Stash{Index: 0, SHA: "abc123"},
		SHA:   "abc123",
	}
	if p.Stash.Index != 0 {
		t.Errorf("Stash.Index = %d", p.Stash.Index)
	}
	if p.SHA != "abc123" {
		t.Errorf("SHA = %q", p.SHA)
	}
}

func TestEventTypeConstants(t *testing.T) {
	// Verify event type constants are unique and non-empty.
	types := []string{
		EventStashesChanged,
		EventModeChanged,
		EventCacheInvalidated,
		EventFilterChanged,
		EventSearchQueryChanged,
		EventStashDropped,
		EventStashApplied,
		EventStashCreated,
		EventError,
	}

	seen := make(map[string]bool)
	for _, typ := range types {
		if typ == "" {
			t.Error("event type constant is empty")
		}
		if seen[typ] {
			t.Errorf("duplicate event type: %q", typ)
		}
		seen[typ] = true
	}
}
```

#### `internal/core/app_test.go`

```go
// internal/core/app_test.go
package core

import (
	"context"
	"log/slog"
	"os"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// ─── Mocks ──────────────────────────────────────────────────

type mockCache struct {
	stashes []plugin.Stash
}

func (m *mockCache) List(_ context.Context) ([]plugin.Stash, error) { return m.stashes, nil }
func (m *mockCache) Diff(_ context.Context, _ string) (string, error) { return "", nil }
func (m *mockCache) Invalidate()                                       {}

type mockGit struct{}

func (m *mockGit) Run(_ context.Context, _ ...string) (string, error) { return "", nil }
func (m *mockGit) RunLines(_ context.Context, _ ...string) ([]string, error) { return nil, nil }
func (m *mockGit) RunExitCode(_ context.Context, _ ...string) (string, int, error) {
	return "", 0, nil
}

type mockConfig struct{}

func (m *mockConfig) GetString(_ string) string { return "" }
func (m *mockConfig) GetInt(_ string) int       { return 0 }
func (m *mockConfig) GetBool(_ string) bool     { return false }

type mockTheme struct{}

func (m *mockTheme) Color(_ string) string { return "#000000" }

func newTestModel(stashes []plugin.Stash) *Model {
	logger := slog.New(slog.NewTextHandler(os.Discard, nil))
	bus := NewBus()
	cache := &mockCache{stashes: stashes}
	gitVer := plugin.GitVersion{Major: 2, Minor: 53, Patch: 0}

	pctx := plugin.PluginContext{
		Git:    &mockGit{},
		Cache:  cache,
		Config: &mockConfig{},
		Events: bus,
		Logger: logger,
		GitVer: gitVer,
		Theme:  &mockTheme{},
	}

	state := NewAppState("/test/repo", "main", gitVer)
	state = WithStashes(state, stashes)

	keyHandlers := plugin.NewRegistry[plugin.KeyHandler]()
	screenProviders := plugin.NewRegistry[plugin.ScreenProvider]()
	stashHooks := plugin.NewRegistry[plugin.StashHook]()

	return New(state, pctx, bus, logger, keyHandlers, screenProviders, stashHooks)
}

func testStashes() []plugin.Stash {
	return []plugin.Stash{
		{Index: 0, SHA: "aaa", ShortSHA: "aaa", Message: "First stash"},
		{Index: 1, SHA: "bbb", ShortSHA: "bbb", Message: "Second stash"},
		{Index: 2, SHA: "ccc", ShortSHA: "ccc", Message: "Third stash"},
	}
}

// ─── Tests ──────────────────────────────────────────────────

func TestModel_InitialState(t *testing.T) {
	m := newTestModel(testStashes())

	state := m.State()
	if state.Mode != ModeList {
		t.Errorf("initial Mode = %s, want LIST", state.Mode)
	}
	if state.Branch != "main" {
		t.Errorf("Branch = %q, want %q", state.Branch, "main")
	}
	if len(state.Stashes) != 3 {
		t.Errorf("len(Stashes) = %d, want 3", len(state.Stashes))
	}
}

func TestModel_WindowSizeMsg(t *testing.T) {
	m := newTestModel(testStashes())

	msg := tea.WindowSizeMsg{Width: 200, Height: 60}
	updated, _ := m.Update(msg)
	model := updated.(*Model)

	state := model.State()
	if state.Width != 200 || state.Height != 60 {
		t.Errorf("after WindowSizeMsg: %dx%d, want 200x60", state.Width, state.Height)
	}
}

func TestModel_CursorNavigation(t *testing.T) {
	m := newTestModel(testStashes())
	// Mark as ready so keys are handled.
	m.ready = true

	// j moves down.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "j"})
	model := updated.(*Model)
	if model.State().Cursor != 1 {
		t.Errorf("after 'j': Cursor = %d, want 1", model.State().Cursor)
	}

	// j again.
	updated, _ = model.Update(tea.KeyPressMsg{Text: "j"})
	model = updated.(*Model)
	if model.State().Cursor != 2 {
		t.Errorf("after 'j' x2: Cursor = %d, want 2", model.State().Cursor)
	}

	// j at end should clamp.
	updated, _ = model.Update(tea.KeyPressMsg{Text: "j"})
	model = updated.(*Model)
	if model.State().Cursor != 2 {
		t.Errorf("after 'j' at end: Cursor = %d, want 2", model.State().Cursor)
	}

	// k moves up.
	updated, _ = model.Update(tea.KeyPressMsg{Text: "k"})
	model = updated.(*Model)
	if model.State().Cursor != 1 {
		t.Errorf("after 'k': Cursor = %d, want 1", model.State().Cursor)
	}
}

func TestModel_JumpToFirstLast(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true
	m.state = WithCursor(m.state, 1)

	// 'g' jumps to first.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "g"})
	model := updated.(*Model)
	if model.State().Cursor != 0 {
		t.Errorf("after 'g': Cursor = %d, want 0", model.State().Cursor)
	}

	// 'G' jumps to last.
	updated, _ = model.Update(tea.KeyPressMsg{Text: "G"})
	model = updated.(*Model)
	if model.State().Cursor != 2 {
		t.Errorf("after 'G': Cursor = %d, want 2", model.State().Cursor)
	}
}

func TestModel_ModeTransitionViaKeys(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	// Tab -> PREVIEW.
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	model := updated.(*Model)
	if model.State().Mode != ModePreview {
		t.Errorf("after Tab: Mode = %s, want PREVIEW", model.State().Mode)
	}

	// Esc -> back to LIST.
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(*Model)
	if model.State().Mode != ModeList {
		t.Errorf("after Esc: Mode = %s, want LIST", model.State().Mode)
	}

	// Enter -> DETAIL.
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*Model)
	if model.State().Mode != ModeDetail {
		t.Errorf("after Enter: Mode = %s, want DETAIL", model.State().Mode)
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	// '?' opens help.
	updated, _ := m.Update(tea.KeyPressMsg{Text: "?"})
	model := updated.(*Model)
	if model.State().Mode != ModeHelp {
		t.Errorf("after '?': Mode = %s, want HELP", model.State().Mode)
	}

	// '?' again closes help (back to LIST).
	updated, _ = model.Update(tea.KeyPressMsg{Text: "?"})
	model = updated.(*Model)
	if model.State().Mode != ModeList {
		t.Errorf("after '?' again: Mode = %s, want LIST", model.State().Mode)
	}
}

func TestModel_ViewBeforeReady(t *testing.T) {
	m := newTestModel(testStashes())
	// m.ready is false by default.

	v := m.View()
	if !v.AltScreen {
		t.Error("View should use AltScreen")
	}
	if v.Content == nil {
		t.Error("View.Content should not be nil")
	}
}

func TestModel_ViewAfterReady(t *testing.T) {
	m := newTestModel(testStashes())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	v := m.View()
	if !v.AltScreen {
		t.Error("View should use AltScreen")
	}
	if v.Content == nil {
		t.Error("View.Content should not be nil even after ready")
	}
}

func TestModel_StashesLoadedMsg(t *testing.T) {
	m := newTestModel(nil) // Start with no stashes.
	m.ready = true

	newStashes := []plugin.Stash{
		{Index: 0, SHA: "xxx", Message: "loaded"},
	}

	updated, _ := m.Update(stashesLoadedMsg{stashes: newStashes})
	model := updated.(*Model)

	state := model.State()
	if len(state.Stashes) != 1 {
		t.Fatalf("len(Stashes) = %d, want 1", len(state.Stashes))
	}
	if state.Stashes[0].SHA != "xxx" {
		t.Errorf("Stashes[0].SHA = %q, want %q", state.Stashes[0].SHA, "xxx")
	}
}

func TestModel_EscAtListIsNoop(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	// Esc at LIST should be a no-op (can't go further back).
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model := updated.(*Model)
	if model.State().Mode != ModeList {
		t.Errorf("Esc at LIST: Mode = %s, want LIST", model.State().Mode)
	}
}

func TestModel_DeepModeStack(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	// LIST -> PREVIEW
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.State().Mode != ModePreview {
		t.Fatalf("expected PREVIEW, got %s", m.State().Mode)
	}

	// PREVIEW -> DETAIL
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.State().Mode != ModeDetail {
		t.Fatalf("expected DETAIL, got %s", m.State().Mode)
	}

	// DETAIL -> HELP
	m.Update(tea.KeyPressMsg{Text: "?"})
	if m.State().Mode != ModeHelp {
		t.Fatalf("expected HELP, got %s", m.State().Mode)
	}

	// Esc from HELP -> DETAIL
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State().Mode != ModeDetail {
		t.Errorf("expected DETAIL after Esc from HELP, got %s", m.State().Mode)
	}

	// Esc from DETAIL -> PREVIEW
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State().Mode != ModePreview {
		t.Errorf("expected PREVIEW after Esc from DETAIL, got %s", m.State().Mode)
	}

	// Esc from PREVIEW -> LIST
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State().Mode != ModeList {
		t.Errorf("expected LIST after Esc from PREVIEW, got %s", m.State().Mode)
	}
}
```

## Verification

### Functional
```bash
# From project root
cd /Users/indrasvat/code/github.com/indrasvat-nidhi

# Ensure directories exist
ls internal/core/

# Run tests for the core package
go test -v -race ./internal/core/...

# Run tests for both core and plugin (they're coupled)
go test -v -race ./internal/core/... ./internal/plugin/...

# Check compilation
go build ./internal/core/...

# Run linter
make lint

# Full CI
make ci
```

## Completion Criteria
1. All source files compile: `app.go`, `mode.go`, `state.go`, `events.go`, `eventbus.go`
2. All test files pass with `go test -v -race ./internal/core/...`
3. ModeManager supports push/pop with a stack; Esc always returns to previous mode
4. All valid mode transitions from PRD FR-03 are enforced by `isValidTransition`
5. Invalid transitions return errors and do not change state
6. Help mode is accessible from every other mode
7. AppState is a value type; With* functions return copies without mutating the original
8. `WithCursor` clamps to `[0, len(Stashes)-1]`; empty stash list clamps to 0
9. EventBus correctly dispatches to all subscribers of a given event type
10. Model.View returns `tea.View{AltScreen: true, Content: tea.NewLayer(...)}` -- Content is `tea.Layer`, not string
11. Model.Update handles `tea.WindowSizeMsg`, `tea.KeyPressMsg`, internal messages
12. `j/k` navigate cursor, `g/G` jump to first/last, `Tab` toggles preview, `Enter` enters detail
13. `make lint` passes with no warnings

## Commit
```
feat(core): add BubbleTea model, mode manager, state, and event bus

Implement the core application framework:
- app.go: top-level tea.Model with Init/Update/View, key dispatch to
  built-in handlers and plugin KeyHandlers, async stash loading
- mode.go: ModeManager with push/pop stack for Esc-always-goes-back
  behavior, valid transition rules from PRD FR-03
- state.go: AppState factory and immutable With* helpers for cursor,
  size, mode, filters, search query
- events.go: event type constants and typed constructors
- eventbus.go: synchronous EventBus implementation (plugin.EventBus)
- Comprehensive tests: mode transitions, state immutability, cursor
  clamping, event dispatch, model key handling, deep mode stack
```

## Session Protocol
1. Read this task file completely before writing any code.
2. Verify Task 005 (plugin interfaces) is complete -- `go build ./internal/plugin/...` must succeed.
3. Create `internal/core/` directory if it does not exist.
4. Write files in order: `mode.go`, `state.go`, `events.go`, `eventbus.go`, `app.go`.
5. Write test files: `mode_test.go`, `state_test.go`, `events_test.go`, `eventbus_test.go`, `app_test.go`.
6. Run `go test -v -race ./internal/core/...` and fix all failures.
7. Update `docs/PROGRESS.md` and `CLAUDE.md` Learnings section with any discoveries.
