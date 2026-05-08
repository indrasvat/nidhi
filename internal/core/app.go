package core

import (
	"context"
	"fmt"
	"image/color"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// UIRenderer is an optional interface for rendering real screen content.
// When nil, the model falls back to placeholder strings.
// This avoids circular imports between core and ui/screens.
type UIRenderer interface {
	RenderContent(state AppState) string
	RenderWelcome(width, height int, version, commit string) string
	HandleMessage(msg tea.Msg, state AppState) (AppState, tea.Cmd)
	OnModeChange(prev, next Mode, state AppState) tea.Cmd
}

// Model is the top-level BubbleTea model for nidhi.
type Model struct {
	state AppState
	modes *ModeManager

	pluginCtx plugin.PluginContext
	bus       *Bus
	logger    *slog.Logger

	keyHandlers     *plugin.Registry[plugin.KeyHandler]
	screenProviders *plugin.Registry[plugin.ScreenProvider]
	stashHooks      *plugin.Registry[plugin.StashHook]

	// UI is an optional renderer for real screen content.
	// Set by cmd/nidhi/main.go after construction.
	UI UIRenderer

	// BgColor sets the terminal's default background color via OSC 11.
	// Set by cmd/nidhi/main.go to the theme's bg.deep color.
	BgColor color.Color

	// Build metadata, set by cmd/nidhi/main.go.
	Version string // e.g., "dev", "v0.1.0"
	Commit  string // e.g., "abc1234"

	// Welcome shows the startup welcome screen until Enter is pressed.
	// Set to true by cmd/nidhi/main.go; defaults to false for tests.
	Welcome bool

	ready bool
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
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return m.loadStashes()
}

// Update implements tea.Model.
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
	case StashMutatedMsg:
		return m.handleStashMutated()
	}
	// Forward unhandled messages to UI renderer (DiffLoadedMsg, ToastTickMsg, etc).
	if m.UI != nil {
		newState, cmd := m.UI.HandleMessage(msg, m.state)
		m.state = newState
		return m, cmd
	}
	return m, nil
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	if !m.ready {
		banner := m.startupBanner()
		v := tea.NewView(banner)
		v.AltScreen = true
		v.BackgroundColor = m.BgColor
		return v
	}
	if m.Welcome {
		var content string
		if m.UI != nil {
			content = m.UI.RenderWelcome(m.state.Width, m.state.Height, m.Version, m.Commit)
		} else {
			content = m.startupBanner()
		}
		v := tea.NewView(content)
		v.AltScreen = true
		v.BackgroundColor = m.BgColor
		return v
	}
	content := m.renderContent()
	v := tea.NewView(content)
	v.AltScreen = true
	v.BackgroundColor = m.BgColor
	return v
}

// State returns a copy of the current application state.
func (m *Model) State() AppState {
	return m.state
}

// ─── Internal message types ─────────────────────────────────

type stashesLoadedMsg struct {
	stashes []plugin.Stash
}

type errMsg struct {
	operation string
	err       error
}

// ─── Message handlers ───────────────────────────────────────

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.state = WithSize(m.state, msg.Width, msg.Height)
	m.ready = true
	m.logger.Debug("window resized", "width", msg.Width, "height", msg.Height)
	// Forward to UI for screen resize handling.
	if m.UI != nil {
		newState, cmd := m.UI.HandleMessage(msg, m.state)
		m.state = newState
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Welcome screen: only Enter/q/Ctrl+C are active.
	if m.Welcome {
		switch {
		case msg.Code == tea.KeyEnter:
			m.Welcome = false
			return m, nil
		case msg.Text == "q":
			return m, tea.Quit
		case msg.Text == "c" && msg.Mod.Contains(tea.ModCtrl):
			return m, tea.Quit
		}
		return m, nil
	}

	// Global keys.
	switch {
	case msg.Text == "q":
		return m, tea.Quit
	case msg.Text == "c" && msg.Mod.Contains(tea.ModCtrl):
		return m, tea.Quit
	case msg.Text == "?":
		if m.state.Mode == ModeHelp {
			return m, m.popMode()
		} else {
			cmd := m.pushMode(ModeHelp)
			return m, cmd
		}
	case msg.Code == tea.KeyEscape && (m.state.Mode == m.modes.Current() || m.state.Mode == ModeHelp):
		return m, m.popMode()
	}

	// Mode-specific key handling.
	switch m.state.Mode {
	case ModeList:
		return m.handleListKeys(msg)
	case ModePreview:
		return m.handlePreviewKeys(msg)
	case ModeDetail:
		return m.handleDetailKeys(msg)
	}

	if m.UI != nil {
		newState, cmd := m.UI.HandleMessage(msg, m.state)
		m.state = newState
		if cmd != nil {
			return m, cmd
		}
	}

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

// handleStashMutated invalidates the cache and reloads the stash list after
// any mutation (apply/pop/drop/rename/branch/store/reorder).
func (m *Model) handleStashMutated() (tea.Model, tea.Cmd) {
	m.pluginCtx.Cache.Invalidate()
	m.bus.Publish(NewCacheInvalidatedEvent())
	return m, m.loadStashes()
}

// ─── Built-in key handlers ──────────────────────────────────

func (m *Model) handleListKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Mode switches use pushMode (must stay in core for mode stack).
	switch msg.Code {
	case tea.KeyTab:
		cmd := m.pushMode(ModePreview)
		return m, cmd
	case tea.KeyEnter:
		cmd := m.pushMode(ModeDetail)
		return m, cmd
	}

	// Delegate all other keys (j/k/g/G, CRUD actions) to UI screens.
	if m.UI != nil {
		newState, cmd := m.UI.HandleMessage(msg, m.state)
		m.state = newState
		if cmd != nil {
			return m, cmd
		}
	}

	return m.delegateToPluginKeyHandlers(msg)
}

func (m *Model) handlePreviewKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Mode switches use pushMode/popMode (must stay in core for mode stack).
	switch msg.Code {
	case tea.KeyTab:
		return m, m.popMode()
	case tea.KeyEnter:
		cmd := m.pushMode(ModeDetail)
		return m, cmd
	}

	// Delegate all other keys (j/k/h/l, diff scroll, CRUD) to UI screens.
	if m.UI != nil {
		newState, cmd := m.UI.HandleMessage(msg, m.state)
		m.state = newState
		if cmd != nil {
			return m, cmd
		}
	}

	return m.delegateToPluginKeyHandlers(msg)
}

func (m *Model) handleDetailKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Detail mode delegates ALL keys to UI (Tab for tree/diff focus toggle,
	// j/k/arrows for file navigation, Enter for tree expand/collapse).
	// No mode-switch interception here — Esc and ? are handled globally.
	if m.UI != nil {
		newState, cmd := m.UI.HandleMessage(msg, m.state)
		m.state = newState
		if cmd != nil {
			return m, cmd
		}
	}

	return m.delegateToPluginKeyHandlers(msg)
}

// ─── Plugin delegation ──────────────────────────────────────

func (m *Model) delegateToPluginKeyHandlers(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	keyEvent := plugin.KeyEvent{
		Key:  msg.Text,
		Mod:  int(msg.Mod),
		Mode: m.state.Mode,
	}

	for _, handler := range m.keyHandlers.All() {
		for _, kb := range handler.KeyBindings() {
			if kb.Key != msg.Text {
				continue
			}
			if len(kb.Modes) > 0 && !modeInSlice(m.state.Mode, kb.Modes) {
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

func (m *Model) pushMode(mode Mode) tea.Cmd {
	prev := m.modes.Current()
	if err := m.modes.Push(mode); err != nil {
		m.logger.Warn("mode push failed", "from", prev, "to", mode, "error", err)
		return nil
	}
	m.state = WithMode(m.state, mode)
	m.bus.Publish(NewModeChangedEvent(prev, mode))
	if m.UI != nil {
		return m.UI.OnModeChange(prev, mode, m.state)
	}
	return nil
}

func (m *Model) popMode() tea.Cmd {
	prev := m.modes.Current()
	newMode := m.modes.Pop()
	m.state = WithMode(m.state, newMode)
	if prev != newMode {
		m.bus.Publish(NewModeChangedEvent(prev, newMode))
		if m.UI != nil {
			return m.UI.OnModeChange(prev, newMode, m.state)
		}
	}
	return nil
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
	if m.UI != nil {
		return m.UI.RenderContent(m.state)
	}
	// Fallback for tests without UI wiring.
	mode := m.modes.Current()
	switch mode {
	case ModeList:
		if len(m.state.Stashes) == 0 {
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

func modeInSlice(mode Mode, modes []Mode) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

func (m *Model) startupBanner() string {
	ver := m.Version
	if ver == "" {
		ver = "dev"
	}
	commitInfo := ""
	if m.Commit != "" && m.Commit != "unknown" && len(m.Commit) >= 7 {
		commitInfo = " (" + m.Commit[:7] + ")"
	}
	return fmt.Sprintf("\n\n   \u25c6 nidhi\n   %s%s\n\n   treasure your stashes\n", ver, commitInfo)
}
