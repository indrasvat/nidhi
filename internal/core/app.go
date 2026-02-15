package core

import (
	"context"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// UIRenderer is an optional interface for rendering real screen content.
// When nil, the model falls back to placeholder strings.
// This avoids circular imports between core and ui/screens.
type UIRenderer interface {
	RenderContent(state AppState) string
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
		v := tea.NewView("Loading nidhi...")
		v.AltScreen = true
		return v
	}
	content := m.renderContent()
	v := tea.NewView(content)
	v.AltScreen = true
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
	// Global keys.
	switch {
	case msg.Text == "q":
		return m, tea.Quit
	case msg.Text == "c" && msg.Mod.Contains(tea.ModCtrl):
		return m, tea.Quit
	case msg.Text == "?":
		if m.modes.Current() == ModeHelp {
			m.popMode()
		} else {
			cmd := m.pushMode(ModeHelp)
			return m, cmd
		}
		return m, nil
	case msg.Code == tea.KeyEscape:
		m.popMode()
		return m, nil
	}

	// Mode-specific key handling.
	switch m.modes.Current() {
	case ModeList:
		return m.handleListKeys(msg)
	case ModePreview:
		return m.handlePreviewKeys(msg)
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

	switch msg.Code {
	case tea.KeyTab:
		cmd := m.pushMode(ModePreview)
		return m, cmd
	case tea.KeyEnter:
		cmd := m.pushMode(ModeDetail)
		return m, cmd
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
		m.popMode()
		return m, nil
	case tea.KeyEnter:
		cmd := m.pushMode(ModeDetail)
		return m, cmd
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
