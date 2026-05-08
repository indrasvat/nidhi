package plugin

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
)

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
	Index        int
	SHA          string
	ShortSHA     string
	Message      string
	RawMessage   string
	Branch       string
	Date         time.Time
	FileCount    int
	Insertions   int
	Deletions    int
	IsStale      bool
	HasUntracked bool
}

// Filter represents an active filter applied to the stash list.
type Filter struct {
	ID    string
	Label string
	Value string
}

// AppState is the immutable snapshot of application state passed to plugins.
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
	Raw   string
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
	Key      string
	Desc     string
	Modes    []Mode
	Priority int
}

// KeyEvent is the event passed to KeyHandler.HandleKey.
type KeyEvent struct {
	Key  string
	Mod  int
	Mode Mode
}

// ScreenDef defines a screen that a ScreenProvider plugin contributes.
type ScreenDef struct {
	Mode Mode
	Name string
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
	Run(ctx context.Context, args ...string) (string, error)
	RunLines(ctx context.Context, args ...string) ([]string, error)
	RunExitCode(ctx context.Context, args ...string) (stdout string, exitCode int, err error)
}

// StashCache provides cached access to stash data.
type StashCache interface {
	List(ctx context.Context) ([]Stash, error)
	Diff(ctx context.Context, sha string) (string, error)
	Invalidate()
}

// EventBus for decoupled communication between core and plugins.
type EventBus interface {
	Publish(event Event)
	Subscribe(eventType string, handler func(Event))
}

// ConfigStore provides read access to configuration values.
type ConfigStore interface {
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
}

// Theme provides access to the current theme tokens.
type Theme interface {
	Color(token string) string
}

// ─── Plugin Interfaces ──────────────────────────────────────

// Plugin is the base interface all plugins must implement.
type Plugin interface {
	ID() string
	Name() string
	Init(ctx PluginContext) error
	Destroy() error
}

// KeyHandler plugins can register keybindings.
type KeyHandler interface {
	Plugin
	KeyBindings() []KeyBinding
	HandleKey(key KeyEvent, state AppState) (AppState, tea.Cmd)
}

// ScreenProvider plugins can register new screen modes.
type ScreenProvider interface {
	Plugin
	Screens() []ScreenDef
	Update(msg tea.Msg, state AppState) (AppState, tea.Cmd)
	View(state AppState, width, height int) string
}

// StashHook plugins can intercept stash operations.
type StashHook interface {
	Plugin
	BeforeApply(stash Stash) (proceed bool, cmd tea.Cmd)
	AfterDrop(stash Stash, sha string) tea.Cmd
	BeforePush(opts PushOptions) (PushOptions, error)
}
