package screens

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/components"
	"github.com/indrasvat/nidhi/internal/ui/layout"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── Stash command messages ─────────────────────────────────
// Dispatched from LIST screen, handled by core model (task 013).

// StashApplyMsg requests applying a stash (git stash apply).
type StashApplyMsg struct{ Stash plugin.Stash }

// StashPopMsg requests popping a stash (git stash pop).
type StashPopMsg struct{ Stash plugin.Stash }

// StashDropMsg requests dropping a stash (git stash drop).
type StashDropMsg struct{ Stash plugin.Stash }

// StashRenameMsg requests renaming a stash message.
type StashRenameMsg struct{ Stash plugin.Stash }

// StashBranchMsg requests creating a branch from a stash.
type StashBranchMsg struct{ Stash plugin.Stash }

// ─── ListScreen ─────────────────────────────────────────────

// ListScreen implements the LIST mode — the default view in nidhi.
// Custom scrollable list with cursor navigation. Does NOT use bubbles
// list.Model because it is too opinionated (built-in filtering, status bar).
type ListScreen struct {
	cursor      int
	offset      int
	width       int
	height      int
	theme       theme.Theme
	rowRenderer components.StashRowRenderer
}

// NewListScreen creates a new ListScreen with the given theme.
func NewListScreen(th theme.Theme) *ListScreen {
	return &ListScreen{
		theme:       th,
		rowRenderer: components.NewStashRowRenderer(th),
	}
}

// Update handles messages for the LIST screen.
func (l *ListScreen) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height - layout.ChromeHeight
		l.clampCursor(len(state.Stashes))
		state.Cursor = l.cursor
		return state, nil

	case tea.KeyPressMsg:
		return l.handleKey(msg, state)
	}
	return state, nil
}

// View renders the LIST screen content area.
func (l *ListScreen) View(state core.AppState, width, height int) string {
	l.width = width
	l.height = height
	l.clampCursor(len(state.Stashes))

	if len(state.Stashes) == 0 {
		return l.renderEmptyState(width, height)
	}

	return l.renderStashList(state, width, height)
}

// Cursor returns the current cursor position.
func (l *ListScreen) Cursor() int {
	return l.cursor
}

// Offset returns the current scroll offset.
func (l *ListScreen) Offset() int {
	return l.offset
}

// SetSize updates the screen dimensions.
func (l *ListScreen) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// ─── Navigation ─────────────────────────────────────────────

// rowHeight returns the number of lines per stash row at the current width.
func (l *ListScreen) rowHeight() int {
	if l.width >= 100 {
		return 2 // primary + secondary line
	}
	return 1 // single line (narrow terminal)
}

// visibleRows returns how many stash rows fit in the content area.
func (l *ListScreen) visibleRows() int {
	if l.height <= 0 {
		return 0
	}
	rh := l.rowHeight()
	// Each row = rh content lines + 1 blank separator; last row has no trailing blank.
	// rh*n + (n-1) <= height  →  n*(rh+1) - 1 <= height  →  n <= (height+1) / (rh+1)
	return max((l.height+1)/(rh+1), 1)
}

// clampCursor ensures cursor stays within bounds and adjusts scroll offset.
func (l *ListScreen) clampCursor(stashCount int) {
	if stashCount == 0 {
		l.cursor = 0
		l.offset = 0
		return
	}
	l.cursor = max(0, min(l.cursor, stashCount-1))
	visible := l.visibleRows()
	if l.cursor >= l.offset+visible {
		l.offset = l.cursor - visible + 1
	}
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	l.offset = max(0, l.offset)
}

func (l *ListScreen) moveCursor(delta, stashCount int) {
	l.cursor += delta
	l.clampCursor(stashCount)
}

func (l *ListScreen) jumpTop(stashCount int) {
	l.cursor = 0
	l.clampCursor(stashCount)
}

func (l *ListScreen) jumpBottom(stashCount int) {
	l.cursor = stashCount - 1
	l.clampCursor(stashCount)
}

func (l *ListScreen) pageDown(stashCount int) {
	l.moveCursor(l.visibleRows()/2, stashCount)
}

func (l *ListScreen) pageUp(stashCount int) {
	l.moveCursor(-l.visibleRows()/2, stashCount)
}

// ─── Key handling ───────────────────────────────────────────

func (l *ListScreen) handleKey(msg tea.KeyPressMsg, state core.AppState) (core.AppState, tea.Cmd) {
	n := len(state.Stashes)

	switch {
	// Cursor navigation.
	case msg.Text == "j":
		l.moveCursor(1, n)
		state.Cursor = l.cursor
	case msg.Text == "k":
		l.moveCursor(-1, n)
		state.Cursor = l.cursor
	case msg.Text == "g":
		l.jumpTop(n)
		state.Cursor = l.cursor
	case msg.Text == "G":
		l.jumpBottom(n)
		state.Cursor = l.cursor

	// Page scroll.
	case msg.Text == "d" && msg.Mod.Contains(tea.ModCtrl):
		l.pageDown(n)
		state.Cursor = l.cursor
	case msg.Text == "u" && msg.Mod.Contains(tea.ModCtrl):
		l.pageUp(n)
		state.Cursor = l.cursor

	// Mode switching.
	case msg.Code == tea.KeyTab:
		state.Mode = core.ModePreview
	case msg.Code == tea.KeyEnter:
		if n > 0 {
			state.Mode = core.ModeDetail
		}

	// CRUD dispatch.
	case msg.Text == "a" && n > 0:
		return state, stashCmd(StashApplyMsg{Stash: state.Stashes[l.cursor]})
	case msg.Text == "p" && n > 0:
		return state, stashCmd(StashPopMsg{Stash: state.Stashes[l.cursor]})
	case msg.Text == "d" && !msg.Mod.Contains(tea.ModCtrl) && n > 0:
		return state, stashCmd(StashDropMsg{Stash: state.Stashes[l.cursor]})
	case msg.Text == "n":
		state.Mode = core.ModeNewStash
	case msg.Text == "r" && n > 0:
		return state, stashCmd(StashRenameMsg{Stash: state.Stashes[l.cursor]})
	case msg.Text == "e":
		state.Mode = core.ModeExport
	case msg.Text == "i":
		state.Mode = core.ModeImport
	case msg.Text == "b" && n > 0:
		return state, stashCmd(StashBranchMsg{Stash: state.Stashes[l.cursor]})
	}

	return state, nil
}

// stashCmd wraps a stash message as a tea.Cmd.
func stashCmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// ─── Rendering ──────────────────────────────────────────────

func (l *ListScreen) renderEmptyState(width, height int) string {
	th := l.theme
	msg := lipgloss.NewStyle().
		Foreground(th.FgSecondary()).
		Align(lipgloss.Center).
		Width(width).
		Render("No stashes found.\n\nPress n to create a new stash.")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg)
}

func (l *ListScreen) renderStashList(state core.AppState, width, height int) string {
	var b strings.Builder
	visible := l.visibleRows()
	end := min(l.offset+visible, len(state.Stashes))
	now := time.Now()

	for idx := l.offset; idx < end; idx++ {
		row := l.rowRenderer.Render(components.StashRowParams{
			Stash:      state.Stashes[idx],
			Selected:   idx == l.cursor,
			Width:      width,
			UseNerd:    false,
			TotalCount: len(state.Stashes),
			Now:        now,
		})
		b.WriteString(row)
		if idx < end-1 {
			b.WriteString("\n") // blank separator between rows
		}
	}

	// Pad remaining height.
	rendered := b.String()
	renderedLines := strings.Count(rendered, "\n") + 1
	for i := renderedLines; i < height; i++ {
		rendered += "\n"
	}

	return rendered
}
