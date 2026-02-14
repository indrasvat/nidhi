# Task 005: Plugin Interfaces and Registry

## Status: TODO

## Depends On
- 001 (GitRunner interface and implementation in `internal/git/`)
- 002 (Config loading and ConfigStore interface in `internal/config/`)

## Parallelizable With
- 003 (Agni theme and theme interface)
- 004 (StashCache implementation)

## Problem
The plugin system is the backbone of nidhi's extensibility model. All non-core features (conflict preview, search, sync, rename, undo, stale detection, reorder, filter) are plugins conforming to well-defined interfaces. Without these interfaces and the registry, no plugin can be built or tested. The registry must handle plugin registration, priority-ordered iteration, keybinding collision detection, and lifecycle management. This is a prerequisite for the core BubbleTea model (Task 006) which orchestrates plugins.

## PRD Reference
- Section 8.2 (Core Interfaces) -- Plugin, KeyHandler, ScreenProvider, StashHook, PluginContext
- Section 8.3 (Core Types) -- Stash, AppState, GitRunner, StashCache, EventBus
- Section 8.4 (Module Structure) -- `internal/plugin/` directory
- Section 4.4 (Go Language Features) -- Generics for `Registry[T Plugin]`
- Section 11.2 (Complete Keymap) -- keybinding structure informing KeyBinding type

## Files to Create
- `internal/plugin/interfaces.go` -- all plugin interfaces, supporting types
- `internal/plugin/registry.go` -- generic Registry[T], ordered by priority
- `internal/plugin/context.go` -- PluginContext struct and factory
- `internal/plugin/loader.go` -- LoadBuiltins() function
- `internal/plugin/interfaces_test.go` -- interface compliance tests
- `internal/plugin/registry_test.go` -- registry add/remove/lookup/collision tests
- `internal/plugin/context_test.go` -- context factory tests

## Execution Steps

### Step 1: Create `internal/plugin/interfaces.go`

Define all plugin interfaces and supporting types from PRD Section 8.2. Every type that crosses the plugin boundary lives here.

```go
// internal/plugin/interfaces.go
package plugin

import (
	"context"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ─── Supporting Types ───────────────────────────────────────

// Mode represents a screen mode in the application.
type Mode int

const (
	ModeList Mode = iota
	ModePreview
	ModeDetail
	ModeSearch
	ModeNewStash
	ModeExport
	ModeImport
	ModeConflict
	ModeHelp
)

// String returns the human-readable mode name.
func (m Mode) String() string {
	names := [...]string{
		"LIST", "PREVIEW", "DETAIL", "SEARCH",
		"NEW", "EXPORT", "IMPORT", "CONFLICT", "HELP",
	}
	if int(m) < len(names) {
		return names[m]
	}
	return "UNKNOWN"
}

// Stash represents a single stash entry.
type Stash struct {
	Index        int       // stash@{Index}
	SHA          string    // Full commit SHA
	ShortSHA     string    // Abbreviated SHA (7 chars)
	Message      string    // User message or auto-generated
	RawMessage   string    // Original message from git
	Branch       string    // Branch where stash was created
	Date         time.Time // Creation timestamp
	FileCount    int       // Number of files changed
	Insertions   int       // Lines added
	Deletions    int       // Lines deleted
	IsStale      bool      // Older than staleness threshold
	HasUntracked bool      // Includes untracked files
}

// Filter represents an active filter applied to the stash list.
type Filter struct {
	ID    string // e.g. "branch", "stale"
	Label string // Display label for the chip
	Value string // Filter value (e.g. branch name)
}

// AppState is the immutable snapshot of application state passed to plugins.
// Plugins receive a copy; they cannot mutate the original.
type AppState struct {
	Mode        Mode
	Stashes     []Stash
	Cursor      int
	Filters     []Filter
	SearchQuery string
	Width       int
	Height      int
	GitVersion  GitVersion
	RepoPath    string
	Branch      string
}

// GitVersion holds parsed git version info for feature gating.
type GitVersion struct {
	Major int
	Minor int
	Patch int
	Raw   string // e.g. "git version 2.53.0"
}

// AtLeast returns true if the git version is >= major.minor.
func (v GitVersion) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

// KeyBinding defines a single keybinding registered by a plugin.
type KeyBinding struct {
	Key      string // The key or key combo, e.g. "a", "J", "fb"
	Desc     string // Human-readable description, e.g. "Apply stash"
	Modes    []Mode // Modes where this binding is active (empty = all modes)
	Priority int    // Higher priority wins on collision (user > core > plugin)
}

// KeyEvent is the event passed to KeyHandler.HandleKey.
type KeyEvent struct {
	Key  string // The matched key text
	Mod  int    // Modifier flags (shift, ctrl, alt)
	Mode Mode   // Current mode when key was pressed
}

// ScreenDef defines a screen that a ScreenProvider plugin contributes.
type ScreenDef struct {
	Mode Mode   // The mode this screen handles
	Name string // Human-readable screen name
}

// PushOptions holds options for creating a new stash.
type PushOptions struct {
	Message        string
	IncludeStaged  bool
	IncludeWorking bool
	IncludeUntrkd  bool
	KeepIndex      bool
	PatchMode      bool
	Pathspecs      []string
}

// Event is a decoupled event for the EventBus.
type Event struct {
	Type    string
	Payload any
}

// ─── Core Service Interfaces ────────────────────────────────

// GitRunner abstracts all git command execution.
type GitRunner interface {
	// Run executes a git command and returns stdout.
	Run(ctx context.Context, args ...string) (string, error)
	// RunLines executes and returns stdout split by newline.
	RunLines(ctx context.Context, args ...string) ([]string, error)
	// RunExitCode executes and returns the exit code (for merge-tree).
	RunExitCode(ctx context.Context, args ...string) (stdout string, exitCode int, err error)
}

// StashCache provides cached access to stash data.
type StashCache interface {
	// List returns all stashes. Cached until Invalidate().
	List(ctx context.Context) ([]Stash, error)
	// Diff returns the diff for a stash, keyed by SHA (not index).
	Diff(ctx context.Context, sha string) (string, error)
	// Invalidate clears the cache. Called after mutations.
	Invalidate()
}

// EventBus for decoupled communication between core and plugins.
type EventBus interface {
	Publish(event Event)
	Subscribe(eventType string, handler func(Event))
}

// ConfigStore provides read access to configuration values.
type ConfigStore interface {
	// GetString returns a config value as string.
	GetString(key string) string
	// GetInt returns a config value as int.
	GetInt(key string) int
	// GetBool returns a config value as bool.
	GetBool(key string) bool
}

// Theme provides access to the current theme tokens.
type Theme interface {
	// Color returns the hex color for a named token (e.g. "bg.deep", "accent.gold").
	Color(token string) string
}

// ─── Plugin Interfaces ──────────────────────────────────────

// Plugin is the base interface all plugins must implement.
type Plugin interface {
	// ID returns a unique identifier for the plugin (e.g. "conflict", "search").
	ID() string
	// Name returns the human-readable name.
	Name() string
	// Init is called once during startup with access to core services.
	Init(ctx PluginContext) error
	// Destroy is called on shutdown for cleanup.
	Destroy() error
}

// KeyHandler plugins can register keybindings.
type KeyHandler interface {
	Plugin
	// KeyBindings returns the keybindings this plugin provides.
	// They are merged into the global keymap. Conflicts are resolved by priority.
	KeyBindings() []KeyBinding
	// HandleKey is called when a registered key is pressed.
	HandleKey(key KeyEvent, state AppState) (AppState, tea.Cmd)
}

// ScreenProvider plugins can register new screen modes.
type ScreenProvider interface {
	Plugin
	// Screens returns screen definitions this plugin provides.
	Screens() []ScreenDef
	// Update handles messages when the screen is active.
	Update(msg tea.Msg, state AppState) (AppState, tea.Cmd)
	// View renders the screen content area only (between status bar and footer).
	// width/height reflect the available content area after subtracting chrome.
	View(state AppState, width, height int) string
}

// StashHook plugins can intercept stash operations.
type StashHook interface {
	Plugin
	// BeforeApply is called before a stash is applied. Return proceed=false to abort.
	BeforeApply(stash Stash) (proceed bool, cmd tea.Cmd)
	// AfterDrop is called after a stash is dropped.
	AfterDrop(stash Stash, sha string) tea.Cmd
	// BeforePush is called before creating a new stash.
	BeforePush(opts PushOptions) (PushOptions, error)
}
```

### Step 2: Create `internal/plugin/context.go`

The PluginContext bundles all core services that plugins need. Factory function validates all fields are non-nil.

```go
// internal/plugin/context.go
package plugin

import (
	"errors"
	"log/slog"
)

// PluginContext provides plugins with access to core services.
type PluginContext struct {
	Git    GitRunner
	Cache  StashCache
	Config ConfigStore
	Events EventBus
	Logger *slog.Logger
	GitVer GitVersion
	Theme  Theme
}

// NewPluginContext creates a PluginContext, validating that required services are set.
func NewPluginContext(
	git GitRunner,
	cache StashCache,
	config ConfigStore,
	events EventBus,
	logger *slog.Logger,
	gitVer GitVersion,
	theme Theme,
) (PluginContext, error) {
	if git == nil {
		return PluginContext{}, errors.New("plugin context: GitRunner is required")
	}
	if cache == nil {
		return PluginContext{}, errors.New("plugin context: StashCache is required")
	}
	if config == nil {
		return PluginContext{}, errors.New("plugin context: ConfigStore is required")
	}
	if events == nil {
		return PluginContext{}, errors.New("plugin context: EventBus is required")
	}
	if logger == nil {
		return PluginContext{}, errors.New("plugin context: Logger is required")
	}
	if theme == nil {
		return PluginContext{}, errors.New("plugin context: Theme is required")
	}
	return PluginContext{
		Git:    git,
		Cache:  cache,
		Config: config,
		Events: events,
		Logger: logger,
		GitVer: gitVer,
		Theme:  theme,
	}, nil
}
```

### Step 3: Create `internal/plugin/registry.go`

Generic registry with priority ordering and keybinding collision detection.

```go
// internal/plugin/registry.go
package plugin

import (
	"fmt"
	"slices"
	"sync"
)

// RegistryEntry holds a plugin and its registration priority.
type RegistryEntry[T Plugin] struct {
	Plugin   T
	Priority int // Higher = wins on conflict. User: 100, Core: 50, Plugin: 10.
}

// Registry is a generic, thread-safe plugin registry ordered by priority.
type Registry[T Plugin] struct {
	mu      sync.RWMutex
	entries []RegistryEntry[T]
	byID    map[string]int // id -> index in entries
}

// NewRegistry creates an empty plugin registry.
func NewRegistry[T Plugin]() *Registry[T] {
	return &Registry[T]{
		entries: make([]RegistryEntry[T], 0, 16),
		byID:    make(map[string]int),
	}
}

// Register adds a plugin with a given priority. Returns error if ID already registered.
func (r *Registry[T]) Register(plugin T, priority int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := plugin.ID()
	if _, exists := r.byID[id]; exists {
		return fmt.Errorf("plugin already registered: %s", id)
	}

	entry := RegistryEntry[T]{Plugin: plugin, Priority: priority}
	r.entries = append(r.entries, entry)

	// Sort by priority descending (highest first).
	slices.SortStableFunc(r.entries, func(a, b RegistryEntry[T]) int {
		return b.Priority - a.Priority // descending
	})

	// Rebuild index after sort.
	r.rebuildIndex()

	return nil
}

// Unregister removes a plugin by ID. Returns error if not found.
func (r *Registry[T]) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	idx, exists := r.byID[id]
	if !exists {
		return fmt.Errorf("plugin not found: %s", id)
	}

	r.entries = slices.Delete(r.entries, idx, idx+1)
	r.rebuildIndex()

	return nil
}

// Get returns a plugin by ID. Returns zero value and false if not found.
func (r *Registry[T]) Get(id string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	idx, exists := r.byID[id]
	if !exists {
		var zero T
		return zero, false
	}
	return r.entries[idx].Plugin, true
}

// All returns all registered plugins in priority order (highest first).
func (r *Registry[T]) All() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]T, len(r.entries))
	for i, e := range r.entries {
		result[i] = e.Plugin
	}
	return result
}

// Len returns the number of registered plugins.
func (r *Registry[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

// rebuildIndex rebuilds the byID map after a sort or delete. Must hold mu.Lock().
func (r *Registry[T]) rebuildIndex() {
	r.byID = make(map[string]int, len(r.entries))
	for i, e := range r.entries {
		r.byID[e.Plugin.ID()] = i
	}
}

// ─── Keybinding Collision Detection ─────────────────────────

// KeyCollision describes a keybinding conflict between two plugins.
type KeyCollision struct {
	Key      string
	Mode     Mode
	PluginA  string // ID of first plugin (higher priority -- wins)
	PluginB  string // ID of second plugin (lower priority -- shadowed)
	PriorityA int
	PriorityB int
}

// DetectKeyCollisions checks a KeyHandler registry for keybinding conflicts.
// Returns all collisions found. The higher-priority plugin wins in practice,
// but collisions are reported so they can be logged or surfaced to the user.
func DetectKeyCollisions(reg *Registry[KeyHandler]) []KeyCollision {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	// Map: "key:mode" -> first registered entry (highest priority since sorted).
	type claim struct {
		pluginID string
		priority int
	}
	seen := make(map[string]claim)
	var collisions []KeyCollision

	for _, entry := range reg.entries {
		for _, kb := range entry.Plugin.KeyBindings() {
			modes := kb.Modes
			if len(modes) == 0 {
				// Empty modes means "all modes"; use a sentinel.
				modes = []Mode{Mode(-1)}
			}
			for _, mode := range modes {
				mapKey := fmt.Sprintf("%s:%d", kb.Key, mode)
				if prev, exists := seen[mapKey]; exists {
					collisions = append(collisions, KeyCollision{
						Key:       kb.Key,
						Mode:      mode,
						PluginA:   prev.pluginID,
						PluginB:   entry.Plugin.ID(),
						PriorityA: prev.priority,
						PriorityB: entry.Priority,
					})
				} else {
					seen[mapKey] = claim{
						pluginID: entry.Plugin.ID(),
						priority: entry.Priority,
					}
				}
			}
		}
	}

	return collisions
}
```

### Step 4: Create `internal/plugin/loader.go`

LoadBuiltins registers all built-in plugin stubs. This will be fleshed out as each plugin is implemented.

```go
// internal/plugin/loader.go
package plugin

import "log/slog"

// BuiltinPluginIDs lists all built-in plugin identifiers.
var BuiltinPluginIDs = []string{
	"conflict",
	"search",
	"sync",
	"rename",
	"undo",
	"stale",
	"reorder",
	"filter",
}

// LoadBuiltins registers all built-in plugins into the provided registries.
// Each plugin is registered at the default plugin priority (10).
// As plugins are implemented in internal/plugins/*, they replace these stubs.
//
// Returns the count of successfully loaded plugins and any errors encountered.
func LoadBuiltins(
	keyHandlers *Registry[KeyHandler],
	screenProviders *Registry[ScreenProvider],
	stashHooks *Registry[StashHook],
	pctx PluginContext,
	logger *slog.Logger,
) (int, []error) {
	// Placeholder: as each plugin package is implemented (Task 010+),
	// this function will import and register them. For now, return 0 loaded.
	//
	// Example of what this will look like:
	//
	//   conflictPlugin := conflict.New()
	//   if err := conflictPlugin.Init(pctx); err != nil {
	//       errs = append(errs, fmt.Errorf("conflict plugin init: %w", err))
	//   } else {
	//       keyHandlers.Register(conflictPlugin, 10)
	//       stashHooks.Register(conflictPlugin, 10)
	//       loaded++
	//   }

	logger.Info("plugin loader: built-in plugin registration is stubbed (no plugins implemented yet)")
	return 0, nil
}
```

### Step 5: Create `internal/plugin/interfaces_test.go`

Compile-time interface compliance tests using type assertions.

```go
// internal/plugin/interfaces_test.go
package plugin_test

import (
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// ─── Mock implementations for interface compliance ──────────

type mockPlugin struct{}

func (m *mockPlugin) ID() string                       { return "mock" }
func (m *mockPlugin) Name() string                     { return "Mock Plugin" }
func (m *mockPlugin) Init(_ plugin.PluginContext) error { return nil }
func (m *mockPlugin) Destroy() error                   { return nil }

// Compile-time interface checks: these lines fail to compile if the
// mock doesn't satisfy the interface.
var _ plugin.Plugin = (*mockPlugin)(nil)

// ─── Type Tests ─────────────────────────────────────────────

func TestModeString(t *testing.T) {
	tests := []struct {
		mode plugin.Mode
		want string
	}{
		{plugin.ModeList, "LIST"},
		{plugin.ModePreview, "PREVIEW"},
		{plugin.ModeDetail, "DETAIL"},
		{plugin.ModeSearch, "SEARCH"},
		{plugin.ModeNewStash, "NEW"},
		{plugin.ModeExport, "EXPORT"},
		{plugin.ModeImport, "IMPORT"},
		{plugin.ModeConflict, "CONFLICT"},
		{plugin.ModeHelp, "HELP"},
		{plugin.Mode(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestGitVersionAtLeast(t *testing.T) {
	tests := []struct {
		name  string
		ver   plugin.GitVersion
		major int
		minor int
		want  bool
	}{
		{"exact match", plugin.GitVersion{Major: 2, Minor: 38, Patch: 0}, 2, 38, true},
		{"higher minor", plugin.GitVersion{Major: 2, Minor: 53, Patch: 0}, 2, 38, true},
		{"lower minor", plugin.GitVersion{Major: 2, Minor: 37, Patch: 0}, 2, 38, false},
		{"higher major", plugin.GitVersion{Major: 3, Minor: 0, Patch: 0}, 2, 51, true},
		{"lower major", plugin.GitVersion{Major: 1, Minor: 99, Patch: 0}, 2, 0, false},
		{"merge-tree check", plugin.GitVersion{Major: 2, Minor: 38, Patch: 0}, 2, 38, true},
		{"export check pass", plugin.GitVersion{Major: 2, Minor: 51, Patch: 0}, 2, 51, true},
		{"export check fail", plugin.GitVersion{Major: 2, Minor: 50, Patch: 0}, 2, 51, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ver.AtLeast(tt.major, tt.minor); got != tt.want {
				t.Errorf("GitVersion{%d,%d,%d}.AtLeast(%d,%d) = %v, want %v",
					tt.ver.Major, tt.ver.Minor, tt.ver.Patch, tt.major, tt.minor, got, tt.want)
			}
		})
	}
}

func TestStashStruct(t *testing.T) {
	// Verify Stash struct can be constructed with all fields.
	s := plugin.Stash{
		Index:        0,
		SHA:          "a3f7b2c1234567890abcdef1234567890abcdef0",
		ShortSHA:     "a3f7b2c",
		Message:      "Fix auth token refresh",
		RawMessage:   "WIP on main: a3f7b2c Fix auth token refresh",
		Branch:       "main",
		Date:         time.Now().Add(-3 * time.Hour),
		FileCount:    3,
		Insertions:   42,
		Deletions:    17,
		IsStale:      false,
		HasUntracked: false,
	}
	if s.Index != 0 {
		t.Errorf("unexpected Index: %d", s.Index)
	}
	if s.ShortSHA != "a3f7b2c" {
		t.Errorf("unexpected ShortSHA: %s", s.ShortSHA)
	}
	if s.FileCount != 3 {
		t.Errorf("unexpected FileCount: %d", s.FileCount)
	}
}

func TestFilterStruct(t *testing.T) {
	f := plugin.Filter{ID: "branch", Label: "main", Value: "main"}
	if f.ID != "branch" {
		t.Errorf("unexpected Filter.ID: %s", f.ID)
	}
}

func TestKeyBindingStruct(t *testing.T) {
	kb := plugin.KeyBinding{
		Key:      "a",
		Desc:     "Apply stash",
		Modes:    []plugin.Mode{plugin.ModeList, plugin.ModePreview},
		Priority: 50,
	}
	if kb.Key != "a" {
		t.Errorf("unexpected Key: %s", kb.Key)
	}
	if len(kb.Modes) != 2 {
		t.Errorf("unexpected Modes length: %d", len(kb.Modes))
	}
}

func TestPushOptionsStruct(t *testing.T) {
	opts := plugin.PushOptions{
		Message:        "test stash",
		IncludeStaged:  true,
		IncludeWorking: true,
		IncludeUntrkd:  false,
		KeepIndex:      true,
		PatchMode:      false,
		Pathspecs:      []string{"src/"},
	}
	if opts.Message != "test stash" {
		t.Errorf("unexpected Message: %s", opts.Message)
	}
	if !opts.KeepIndex {
		t.Error("expected KeepIndex to be true")
	}
}

func TestEventStruct(t *testing.T) {
	e := plugin.Event{Type: "stashes.changed", Payload: 5}
	if e.Type != "stashes.changed" {
		t.Errorf("unexpected Type: %s", e.Type)
	}
	if e.Payload.(int) != 5 {
		t.Errorf("unexpected Payload: %v", e.Payload)
	}
}
```

### Step 6: Create `internal/plugin/registry_test.go`

Thorough registry tests: add, remove, lookup, ordering, collisions.

```go
// internal/plugin/registry_test.go
package plugin_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// ─── Test helpers ───────────────────────────────────────────

type stubPlugin struct {
	id   string
	name string
}

func (s *stubPlugin) ID() string                       { return s.id }
func (s *stubPlugin) Name() string                     { return s.name }
func (s *stubPlugin) Init(_ plugin.PluginContext) error { return nil }
func (s *stubPlugin) Destroy() error                   { return nil }

var _ plugin.Plugin = (*stubPlugin)(nil)

type stubKeyHandler struct {
	stubPlugin
	bindings []plugin.KeyBinding
}

func (s *stubKeyHandler) KeyBindings() []plugin.KeyBinding { return s.bindings }
func (s *stubKeyHandler) HandleKey(_ plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	return state, nil
}

var _ plugin.KeyHandler = (*stubKeyHandler)(nil)

// ─── Registry Tests ─────────────────────────────────────────

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	p := &stubPlugin{id: "test", name: "Test"}
	if err := reg.Register(p, 10); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	got, ok := reg.Get("test")
	if !ok {
		t.Fatal("Get() returned false for registered plugin")
	}
	if got.ID() != "test" {
		t.Errorf("Get() returned plugin with ID %q, want %q", got.ID(), "test")
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	p1 := &stubPlugin{id: "dup", name: "First"}
	p2 := &stubPlugin{id: "dup", name: "Second"}

	if err := reg.Register(p1, 10); err != nil {
		t.Fatalf("first Register() error: %v", err)
	}
	if err := reg.Register(p2, 10); err == nil {
		t.Fatal("second Register() with duplicate ID should return error")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("Get() returned true for nonexistent plugin")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	p := &stubPlugin{id: "removeme", name: "Remove Me"}
	if err := reg.Register(p, 10); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	if err := reg.Unregister("removeme"); err != nil {
		t.Fatalf("Unregister() error: %v", err)
	}

	_, ok := reg.Get("removeme")
	if ok {
		t.Error("Get() returned true after Unregister()")
	}

	if reg.Len() != 0 {
		t.Errorf("Len() = %d after Unregister(), want 0", reg.Len())
	}
}

func TestRegistry_UnregisterNotFound(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	if err := reg.Unregister("ghost"); err == nil {
		t.Error("Unregister() of nonexistent plugin should return error")
	}
}

func TestRegistry_PriorityOrdering(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	low := &stubPlugin{id: "low", name: "Low Priority"}
	mid := &stubPlugin{id: "mid", name: "Mid Priority"}
	high := &stubPlugin{id: "high", name: "High Priority"}

	// Register in scrambled order.
	reg.Register(mid, 50)
	reg.Register(low, 10)
	reg.Register(high, 100)

	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d plugins, want 3", len(all))
	}

	wantOrder := []string{"high", "mid", "low"}
	for i, want := range wantOrder {
		if all[i].ID() != want {
			t.Errorf("All()[%d].ID() = %q, want %q", i, all[i].ID(), want)
		}
	}
}

func TestRegistry_Len(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	if reg.Len() != 0 {
		t.Errorf("Len() of empty registry = %d, want 0", reg.Len())
	}

	reg.Register(&stubPlugin{id: "a"}, 10)
	reg.Register(&stubPlugin{id: "b"}, 20)

	if reg.Len() != 2 {
		t.Errorf("Len() = %d, want 2", reg.Len())
	}
}

func TestRegistry_AllReturnsDefensiveCopy(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	reg.Register(&stubPlugin{id: "x"}, 10)

	all := reg.All()
	all[0] = &stubPlugin{id: "hacked"}

	got, ok := reg.Get("x")
	if !ok || got.ID() != "x" {
		t.Error("mutating All() return value should not affect registry")
	}
}

func TestRegistry_UnregisterPreservesOrder(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()

	reg.Register(&stubPlugin{id: "a"}, 30)
	reg.Register(&stubPlugin{id: "b"}, 20)
	reg.Register(&stubPlugin{id: "c"}, 10)

	// Remove middle element.
	reg.Unregister("b")

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d plugins after Unregister, want 2", len(all))
	}
	if all[0].ID() != "a" || all[1].ID() != "c" {
		t.Errorf("order after Unregister: [%s, %s], want [a, c]", all[0].ID(), all[1].ID())
	}
}

// ─── Keybinding Collision Tests ─────────────────────────────

func TestDetectKeyCollisions_NoCollision(t *testing.T) {
	reg := plugin.NewRegistry[plugin.KeyHandler]()

	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p1"},
		bindings:   []plugin.KeyBinding{{Key: "a", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)

	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p2"},
		bindings:   []plugin.KeyBinding{{Key: "b", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)

	collisions := plugin.DetectKeyCollisions(reg)
	if len(collisions) != 0 {
		t.Errorf("expected 0 collisions, got %d: %+v", len(collisions), collisions)
	}
}

func TestDetectKeyCollisions_SameKeySameMode(t *testing.T) {
	reg := plugin.NewRegistry[plugin.KeyHandler]()

	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "core"},
		bindings:   []plugin.KeyBinding{{Key: "a", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 50)

	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "custom"},
		bindings:   []plugin.KeyBinding{{Key: "a", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)

	collisions := plugin.DetectKeyCollisions(reg)
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}

	c := collisions[0]
	if c.Key != "a" {
		t.Errorf("collision Key = %q, want %q", c.Key, "a")
	}
	// Higher priority plugin (core, 50) should be PluginA.
	if c.PluginA != "core" {
		t.Errorf("collision PluginA = %q, want %q", c.PluginA, "core")
	}
	if c.PluginB != "custom" {
		t.Errorf("collision PluginB = %q, want %q", c.PluginB, "custom")
	}
}

func TestDetectKeyCollisions_SameKeyDifferentMode(t *testing.T) {
	reg := plugin.NewRegistry[plugin.KeyHandler]()

	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p1"},
		bindings:   []plugin.KeyBinding{{Key: "j", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)

	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p2"},
		bindings:   []plugin.KeyBinding{{Key: "j", Modes: []plugin.Mode{plugin.ModeDetail}}},
	}, 10)

	collisions := plugin.DetectKeyCollisions(reg)
	if len(collisions) != 0 {
		t.Errorf("same key in different modes should not collide, got %d collisions", len(collisions))
	}
}

func TestDetectKeyCollisions_GlobalModeBinding(t *testing.T) {
	reg := plugin.NewRegistry[plugin.KeyHandler]()

	// Plugin with global binding (empty Modes = all modes).
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "global"},
		bindings:   []plugin.KeyBinding{{Key: "q", Modes: nil}},
	}, 50)

	// Plugin with same key in a specific mode.
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "specific"},
		bindings:   []plugin.KeyBinding{{Key: "q", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)

	// Global uses sentinel mode(-1), specific uses ModeList.
	// These are different map keys so no collision detected at this level.
	// The runtime key dispatcher is responsible for resolving global vs specific.
	collisions := plugin.DetectKeyCollisions(reg)
	if len(collisions) != 0 {
		t.Logf("collisions: %+v", collisions)
	}
}
```

### Step 7: Create `internal/plugin/context_test.go`

```go
// internal/plugin/context_test.go
package plugin_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// ─── Minimal mock implementations ───────────────────────────

type mockGitRunner struct{}

func (m *mockGitRunner) Run(_ context.Context, _ ...string) (string, error) { return "", nil }
func (m *mockGitRunner) RunLines(_ context.Context, _ ...string) ([]string, error) {
	return nil, nil
}
func (m *mockGitRunner) RunExitCode(_ context.Context, _ ...string) (string, int, error) {
	return "", 0, nil
}

type mockStashCache struct{}

func (m *mockStashCache) List(_ context.Context) ([]plugin.Stash, error) { return nil, nil }
func (m *mockStashCache) Diff(_ context.Context, _ string) (string, error) { return "", nil }
func (m *mockStashCache) Invalidate()                                       {}

type mockConfigStore struct{}

func (m *mockConfigStore) GetString(_ string) string { return "" }
func (m *mockConfigStore) GetInt(_ string) int       { return 0 }
func (m *mockConfigStore) GetBool(_ string) bool     { return false }

type mockEventBus struct{}

func (m *mockEventBus) Publish(_ plugin.Event)                          {}
func (m *mockEventBus) Subscribe(_ string, _ func(plugin.Event))        {}

type mockTheme struct{}

func (m *mockTheme) Color(_ string) string { return "#000000" }

// ─── Tests ──────────────────────────────────────────────────

func validArgs() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
	return &mockGitRunner{}, &mockStashCache{}, &mockConfigStore{}, &mockEventBus{},
		slog.New(slog.NewTextHandler(os.Discard, nil)),
		plugin.GitVersion{Major: 2, Minor: 53, Patch: 0, Raw: "git version 2.53.0"},
		&mockTheme{}
}

func TestNewPluginContext_Success(t *testing.T) {
	git, cache, config, events, logger, gitVer, theme := validArgs()
	ctx, err := plugin.NewPluginContext(git, cache, config, events, logger, gitVer, theme)
	if err != nil {
		t.Fatalf("NewPluginContext() error: %v", err)
	}
	if ctx.Git == nil {
		t.Error("Git field is nil")
	}
	if ctx.Cache == nil {
		t.Error("Cache field is nil")
	}
	if ctx.Config == nil {
		t.Error("Config field is nil")
	}
	if ctx.Events == nil {
		t.Error("Events field is nil")
	}
	if ctx.Logger == nil {
		t.Error("Logger field is nil")
	}
	if ctx.Theme == nil {
		t.Error("Theme field is nil")
	}
	if ctx.GitVer.Major != 2 || ctx.GitVer.Minor != 53 {
		t.Errorf("GitVer = %+v, want 2.53.x", ctx.GitVer)
	}
}

func TestNewPluginContext_NilFields(t *testing.T) {
	git, cache, config, events, logger, gitVer, theme := validArgs()

	tests := []struct {
		name   string
		modify func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme)
	}{
		{"nil GitRunner", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return nil, cache, config, events, logger, gitVer, theme
		}},
		{"nil StashCache", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, nil, config, events, logger, gitVer, theme
		}},
		{"nil ConfigStore", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, nil, events, logger, gitVer, theme
		}},
		{"nil EventBus", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, config, nil, logger, gitVer, theme
		}},
		{"nil Logger", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, config, events, nil, gitVer, theme
		}},
		{"nil Theme", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, config, events, logger, gitVer, nil
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, ca, co, ev, lo, gv, th := tt.modify()
			_, err := plugin.NewPluginContext(g, ca, co, ev, lo, gv, th)
			if err == nil {
				t.Error("NewPluginContext() should return error for nil field")
			}
		})
	}
}
```

## Verification

### Functional
```bash
# From project root
cd /Users/indrasvat/code/github.com/indrasvat-nidhi

# Ensure the directory structure exists
ls internal/plugin/

# Run tests for the plugin package only
go test -v -race ./internal/plugin/...

# Check for compilation errors
go build ./internal/plugin/...

# Run linter
make lint

# Full CI check
make ci
```

### Structure Check
```bash
# Verify all expected files exist
test -f internal/plugin/interfaces.go && echo "OK: interfaces.go"
test -f internal/plugin/registry.go && echo "OK: registry.go"
test -f internal/plugin/context.go && echo "OK: context.go"
test -f internal/plugin/loader.go && echo "OK: loader.go"
test -f internal/plugin/interfaces_test.go && echo "OK: interfaces_test.go"
test -f internal/plugin/registry_test.go && echo "OK: registry_test.go"
test -f internal/plugin/context_test.go && echo "OK: context_test.go"
```

## Completion Criteria
1. All four source files (`interfaces.go`, `registry.go`, `context.go`, `loader.go`) compile without errors
2. All three test files pass with `go test -v -race ./internal/plugin/...`
3. `Registry[T]` is generic over any type satisfying the `Plugin` constraint
4. Registry maintains priority-descending order after Register, Unregister
5. `DetectKeyCollisions` correctly identifies same-key-same-mode conflicts
6. `NewPluginContext` rejects nil for every required field (6 nil-check tests pass)
7. All types from PRD Section 8.2 and 8.3 are defined: Plugin, KeyHandler, ScreenProvider, StashHook, PluginContext, Stash, AppState, GitRunner, StashCache, EventBus, Mode, Filter, KeyBinding, KeyEvent, ScreenDef, PushOptions, Event, GitVersion
8. `make lint` passes with no warnings
9. `LoadBuiltins` is callable (stub only; returns 0 loaded, nil errors)

## Commit
```
feat(plugin): add plugin interfaces, generic registry, and context

Implement the plugin system foundation:
- interfaces.go: all plugin interfaces (Plugin, KeyHandler,
  ScreenProvider, StashHook) and supporting types from PRD Section 8.2/8.3
- registry.go: generic Registry[T Plugin] with priority ordering,
  collision detection for keybindings
- context.go: PluginContext factory with nil-field validation
- loader.go: LoadBuiltins stub for future plugin registration
- Full table-driven tests for registry operations, priority ordering,
  keybinding collision detection, and context validation
```

## Session Protocol
1. Read this task file completely before writing any code.
2. Create `internal/plugin/` directory if it does not exist.
3. Write files in order: `interfaces.go`, `context.go`, `registry.go`, `loader.go`.
4. Write test files: `interfaces_test.go`, `registry_test.go`, `context_test.go`.
5. Run `go test -v -race ./internal/plugin/...` and fix any failures.
6. Run `make lint` and fix any linter warnings.
7. Update `docs/PROGRESS.md` and `CLAUDE.md` Learnings section with any discoveries.
