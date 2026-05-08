# Task 010: LIST Screen — Default View

## Status: TODO

## Depends On
- 006 (core model — `internal/core/app.go`, `mode.go`, `state.go`)
- 007 (layout engine — `internal/ui/layout/layout.go`, `split.go`, `responsive.go`)
- 008 (stash row component — `internal/ui/components/stashrow.go`)

## Parallelizable With
- None (core screen — 011/012 depend on this)

## Problem
nidhi needs its primary screen: the LIST mode (PRD §10 Screen 1). This is the default view on startup — a scrollable list of all stashes with cursor navigation, empty state handling, stash count in the status bar, and key dispatching for all CRUD operations. The list must be custom-built (NOT bubbles `list.Model` — too opinionated per PRD §13.5 and CLAUDE.md Key Decisions) with bare-metal row rendering using the stashrow component from task 008.

## PRD Reference
- Section 6.1 FR-01 — Stash list & navigation (FR-01.1 through FR-01.7)
- Section 6.1 FR-02 — CRUD key dispatching
- Section 6.1 FR-03.1 — LIST mode definition
- Section 10 Screen 1 — LIST layout spec, row anatomy, responsive behavior
- Section 11.2 — LIST mode keymap (j/k/g/G/a/p/d/n/r/e/i/Tab/Enter)
- Section 13.5 — Custom stash list rationale (~200 lines)
- Section 14.2 — Cursor move < 1ms budget

## Files to Create
- `internal/ui/screens/list.go` — LIST mode screen model
- `internal/ui/screens/list_test.go` — unit and integration tests

## Files to Modify
- `internal/core/app.go` — wire LIST screen into the top-level model (if not already routed)

## Execution Steps

### Step 1: Define the ListScreen model

```go
// internal/ui/screens/list.go
package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui/components"
	"github.com/indrasvat/nidhi/internal/ui/layout"
)

// ListScreen implements the LIST mode — the default view in nidhi.
// Custom scrollable list with cursor navigation. Does NOT use bubbles list.Model
// because it is too opinionated (built-in filtering, status bar) — we need
// bare-metal row rendering with conditional inline editing (rename).
type ListScreen struct {
	// cursor is the index of the selected stash in the visible list.
	cursor int
	// offset is the first visible row index for scrolling.
	offset int
	// width and height of the content area (excluding status bar and footer).
	width  int
	height int
}

// NewListScreen creates a new ListScreen with default state.
func NewListScreen() *ListScreen {
	return &ListScreen{}
}
```

### Step 2: Implement cursor navigation helpers

```go
// visibleRows returns how many stash rows fit in the content area.
// Each stash row occupies 2 lines (primary + secondary) plus 1 blank separator,
// except the last row which has no trailing blank.
func (l *ListScreen) visibleRows() int {
	if l.height <= 0 {
		return 0
	}
	// Each row = 3 lines (2 content + 1 blank), last row = 2 lines.
	// Solve: 3*n - 1 <= height → n = (height + 1) / 3
	return max((l.height+1)/3, 1)
}

// clampCursor ensures cursor stays within [0, len(stashes)-1] and adjusts
// the scroll offset so the cursor row is always visible.
func (l *ListScreen) clampCursor(stashCount int) {
	if stashCount == 0 {
		l.cursor = 0
		l.offset = 0
		return
	}
	l.cursor = max(0, min(l.cursor, stashCount-1))
	visible := l.visibleRows()
	// Scroll down if cursor is below the visible window.
	if l.cursor >= l.offset+visible {
		l.offset = l.cursor - visible + 1
	}
	// Scroll up if cursor is above the visible window.
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	l.offset = max(0, l.offset)
}

// moveCursor moves the cursor by delta and clamps.
func (l *ListScreen) moveCursor(delta int, stashCount int) {
	l.cursor += delta
	l.clampCursor(stashCount)
}

// jumpTop moves cursor to the first stash.
func (l *ListScreen) jumpTop(stashCount int) {
	l.cursor = 0
	l.clampCursor(stashCount)
}

// jumpBottom moves cursor to the last stash.
func (l *ListScreen) jumpBottom(stashCount int) {
	l.cursor = stashCount - 1
	l.clampCursor(stashCount)
}

// pageDown moves cursor by half the visible rows (Ctrl+D).
func (l *ListScreen) pageDown(stashCount int) {
	l.moveCursor(l.visibleRows()/2, stashCount)
}

// pageUp moves cursor by negative half the visible rows (Ctrl+U).
func (l *ListScreen) pageUp(stashCount int) {
	l.moveCursor(-l.visibleRows()/2, stashCount)
}
```

### Step 3: Implement the Update method for key dispatch

```go
// Update handles messages for the LIST screen.
// Returns updated AppState and any commands to execute.
func (l *ListScreen) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		// Content area = total height - status bar (1) - footer (1)
		l.height = msg.Height - layout.ChromeHeight
		l.clampCursor(len(state.Stashes))
		return state, nil

	case tea.KeyPressMsg:
		return l.handleKey(msg, state)
	}
	return state, nil
}

// handleKey dispatches key presses for LIST mode.
func (l *ListScreen) handleKey(msg tea.KeyPressMsg, state core.AppState) (core.AppState, tea.Cmd) {
	n := len(state.Stashes)

	switch {
	// Cursor navigation
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

	// Page scroll
	case msg.Text == "d" && msg.Mod == tea.ModCtrl:
		l.pageDown(n)
		state.Cursor = l.cursor
	case msg.Text == "u" && msg.Mod == tea.ModCtrl:
		l.pageUp(n)
		state.Cursor = l.cursor

	// Mode switching
	case msg.Code == tea.KeyTab:
		state.Mode = core.ModePreview
		return state, nil
	case msg.Code == tea.KeyEnter:
		if n > 0 {
			state.Mode = core.ModeDetail
		}
		return state, nil

	// CRUD dispatch — these return commands that the core model executes.
	case msg.Text == "a" && n > 0:
		return state, core.ApplyStashCmd(state.Stashes[l.cursor])
	case msg.Text == "p" && n > 0:
		return state, core.PopStashCmd(state.Stashes[l.cursor])
	case msg.Text == "d" && msg.Mod == 0 && n > 0:
		return state, core.DropStashCmd(state.Stashes[l.cursor])
	case msg.Text == "n":
		state.Mode = core.ModeNewStash
		return state, nil
	case msg.Text == "r" && n > 0:
		return state, core.RenameStashCmd(state.Stashes[l.cursor])
	case msg.Text == "e":
		state.Mode = core.ModeExport
		return state, nil
	case msg.Text == "i":
		state.Mode = core.ModeImport
		return state, nil
	case msg.Text == "b" && n > 0:
		return state, core.BranchFromStashCmd(state.Stashes[l.cursor])
	}

	return state, nil
}
```

### Step 4: Implement the View method

```go
// View renders the LIST screen content area.
// width and height are the available content dimensions (after chrome subtracted).
func (l *ListScreen) View(state core.AppState, width, height int) string {
	l.width = width
	l.height = height
	l.clampCursor(len(state.Stashes))

	if len(state.Stashes) == 0 {
		return l.renderEmptyState(width, height)
	}

	return l.renderStashList(state, width, height)
}

// renderEmptyState shows a helpful message when no stashes exist (FR-01.6).
func (l *ListScreen) renderEmptyState(width, height int) string {
	msg := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")). // fg.secondary
		Align(lipgloss.Center).
		Width(width).
		Render("No stashes found.\n\nPress n to create a new stash.")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg)
}

// renderStashList renders the scrollable stash list with cursor.
func (l *ListScreen) renderStashList(state core.AppState, width, height int) string {
	var b strings.Builder
	visible := l.visibleRows()
	end := min(l.offset+visible, len(state.Stashes))

	for idx := l.offset; idx < end; idx++ {
		stash := state.Stashes[idx]
		selected := idx == l.cursor
		row := components.RenderStashRow(stash, selected, width)
		b.WriteString(row)
		if idx < end-1 {
			b.WriteString("\n") // blank separator between rows
		}
	}

	// Pad remaining height with empty lines to fill the content area.
	rendered := b.String()
	renderedLines := strings.Count(rendered, "\n") + 1
	for i := renderedLines; i < height; i++ {
		rendered += "\n"
	}

	return rendered
}

// Cursor returns the current cursor position (for status bar stash count display).
func (l *ListScreen) Cursor() int {
	return l.cursor
}

// Offset returns the current scroll offset.
func (l *ListScreen) Offset() int {
	return l.offset
}
```

### Step 5: Write comprehensive unit tests

```go
// internal/ui/screens/list_test.go
package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
)

// makeStashes creates n test stashes with predictable content.
func makeStashes(n int) []core.Stash {
	stashes := make([]core.Stash, n)
	for i := range n {
		stashes[i] = core.Stash{
			Index:      i,
			SHA:        fmt.Sprintf("abc%04d", i),
			ShortSHA:   fmt.Sprintf("abc%04d", i)[:7],
			Message:    fmt.Sprintf("Test stash %d", i),
			Branch:     "main",
			Date:       time.Now().Add(-time.Duration(i) * time.Hour),
			FileCount:  i + 1,
			Insertions: (i + 1) * 10,
			Deletions:  (i + 1) * 3,
		}
	}
	return stashes
}

func TestListScreen_CursorMovement(t *testing.T) {
	tests := []struct {
		name         string
		stashCount   int
		keys         []string // sequence of key texts to send
		wantCursor   int
		wantOffset   int
	}{
		{
			name:       "j moves cursor down",
			stashCount: 5,
			keys:       []string{"j"},
			wantCursor: 1,
		},
		{
			name:       "k at top stays at 0",
			stashCount: 5,
			keys:       []string{"k"},
			wantCursor: 0,
		},
		{
			name:       "j j j moves to 3",
			stashCount: 5,
			keys:       []string{"j", "j", "j"},
			wantCursor: 3,
		},
		{
			name:       "j past end clamps to last",
			stashCount: 3,
			keys:       []string{"j", "j", "j", "j", "j"},
			wantCursor: 2,
		},
		{
			name:       "G jumps to last",
			stashCount: 10,
			keys:       []string{"G"},
			wantCursor: 9,
		},
		{
			name:       "g jumps to first",
			stashCount: 10,
			keys:       []string{"G", "g"},
			wantCursor: 0,
		},
		{
			name:       "j then k returns to start",
			stashCount: 5,
			keys:       []string{"j", "j", "k"},
			wantCursor: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := NewListScreen()
			ls.width = 120
			ls.height = 30
			state := core.AppState{
				Stashes: makeStashes(tt.stashCount),
				Cursor:  0,
				Mode:    core.ModeList,
			}

			for _, key := range tt.keys {
				msg := tea.KeyPressMsg{Text: key}
				state, _ = ls.Update(msg, state)
			}

			if ls.cursor != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", ls.cursor, tt.wantCursor)
			}
		})
	}
}

func TestListScreen_EmptyList(t *testing.T) {
	tests := []struct {
		name       string
		stashCount int
		key        string
		wantMode   core.Mode
	}{
		{
			name:       "empty list j does nothing",
			stashCount: 0,
			key:        "j",
			wantMode:   core.ModeList,
		},
		{
			name:       "empty list Enter does not switch to detail",
			stashCount: 0,
			key:        "",
			wantMode:   core.ModeList,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := NewListScreen()
			ls.width = 120
			ls.height = 30
			state := core.AppState{
				Stashes: makeStashes(tt.stashCount),
				Mode:    core.ModeList,
			}

			if tt.key != "" {
				msg := tea.KeyPressMsg{Text: tt.key}
				state, _ = ls.Update(msg, state)
			}

			if state.Mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", state.Mode, tt.wantMode)
			}
		})
	}
}

func TestListScreen_EmptyStateRendering(t *testing.T) {
	ls := NewListScreen()
	view := ls.View(core.AppState{Stashes: nil}, 120, 30)

	if !strings.Contains(view, "No stashes found") {
		t.Error("empty state should contain 'No stashes found'")
	}
	if !strings.Contains(view, "Press n") {
		t.Error("empty state should mention 'Press n' to create stash")
	}
}

func TestListScreen_SingleItem(t *testing.T) {
	ls := NewListScreen()
	state := core.AppState{
		Stashes: makeStashes(1),
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// j should not move past the only item.
	msg := tea.KeyPressMsg{Text: "j"}
	state, _ = ls.Update(msg, state)
	if ls.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (single item)", ls.cursor)
	}

	// G and g should both stay at 0.
	msg = tea.KeyPressMsg{Text: "G"}
	state, _ = ls.Update(msg, state)
	if ls.cursor != 0 {
		t.Errorf("G: cursor = %d, want 0", ls.cursor)
	}
}

func TestListScreen_KeyDispatch(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		mod      tea.KeyMod
		code     int32
		wantMode core.Mode
		wantCmd  bool // whether a tea.Cmd is returned
	}{
		{
			name:     "Tab switches to PREVIEW",
			code:     tea.KeyTab,
			wantMode: core.ModePreview,
		},
		{
			name:     "Enter switches to DETAIL",
			code:     tea.KeyEnter,
			wantMode: core.ModeDetail,
		},
		{
			name:     "n switches to NEW STASH",
			key:      "n",
			wantMode: core.ModeNewStash,
		},
		{
			name:     "e switches to EXPORT",
			key:      "e",
			wantMode: core.ModeExport,
		},
		{
			name:     "i switches to IMPORT",
			key:      "i",
			wantMode: core.ModeImport,
		},
		{
			name:    "a dispatches apply command",
			key:     "a",
			wantCmd: true,
		},
		{
			name:    "p dispatches pop command",
			key:     "p",
			wantCmd: true,
		},
		{
			name:    "d dispatches drop command",
			key:     "d",
			wantCmd: true,
		},
		{
			name:    "r dispatches rename command",
			key:     "r",
			wantCmd: true,
		},
		{
			name:    "b dispatches branch command",
			key:     "b",
			wantCmd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := NewListScreen()
			ls.width = 120
			ls.height = 30
			state := core.AppState{
				Stashes: makeStashes(5),
				Cursor:  0,
				Mode:    core.ModeList,
			}

			msg := tea.KeyPressMsg{
				Text: tt.key,
				Mod:  tt.mod,
				Code: tt.code,
			}
			newState, cmd := ls.Update(msg, state)

			if tt.wantMode != 0 && newState.Mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", newState.Mode, tt.wantMode)
			}
			if tt.wantCmd && cmd == nil {
				t.Error("expected a tea.Cmd but got nil")
			}
		})
	}
}

func TestListScreen_ScrollWithManyItems(t *testing.T) {
	ls := NewListScreen()
	ls.width = 120
	ls.height = 15 // fits ~5 rows (3 lines each: 2 content + 1 blank)
	state := core.AppState{
		Stashes: makeStashes(100),
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Move to position 10 — should scroll.
	for i := 0; i < 10; i++ {
		msg := tea.KeyPressMsg{Text: "j"}
		state, _ = ls.Update(msg, state)
	}

	if ls.cursor != 10 {
		t.Errorf("cursor = %d, want 10", ls.cursor)
	}
	if ls.offset == 0 {
		t.Error("offset should have scrolled away from 0")
	}
	visible := ls.visibleRows()
	if ls.cursor >= ls.offset+visible {
		t.Errorf("cursor %d is not within visible window [%d, %d)",
			ls.cursor, ls.offset, ls.offset+visible)
	}
}

func TestListScreen_PageScroll(t *testing.T) {
	ls := NewListScreen()
	ls.width = 120
	ls.height = 15
	state := core.AppState{
		Stashes: makeStashes(50),
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Ctrl+D should page down.
	msg := tea.KeyPressMsg{Text: "d", Mod: tea.ModCtrl}
	state, _ = ls.Update(msg, state)

	halfPage := ls.visibleRows() / 2
	if ls.cursor != halfPage {
		t.Errorf("after Ctrl+D: cursor = %d, want %d", ls.cursor, halfPage)
	}

	// Ctrl+U should page up.
	msg = tea.KeyPressMsg{Text: "u", Mod: tea.ModCtrl}
	state, _ = ls.Update(msg, state)
	if ls.cursor != 0 {
		t.Errorf("after Ctrl+U: cursor = %d, want 0", ls.cursor)
	}
}

func TestListScreen_WindowResize(t *testing.T) {
	ls := NewListScreen()
	state := core.AppState{
		Stashes: makeStashes(20),
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Resize to a small terminal.
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	state, _ = ls.Update(msg, state)

	if ls.width != 80 {
		t.Errorf("width = %d, want 80", ls.width)
	}
	// height should subtract chrome (status bar + footer = 2 lines typically)
	expectedHeight := 24 - layout.ChromeHeight
	if ls.height != expectedHeight {
		t.Errorf("height = %d, want %d", ls.height, expectedHeight)
	}
}

func TestListScreen_ViewRendersAllVisibleStashes(t *testing.T) {
	ls := NewListScreen()
	stashes := makeStashes(5)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	view := ls.View(state, 120, 30)

	// All 5 stash messages should appear in the rendered output.
	for _, s := range stashes {
		if !strings.Contains(view, s.Message) {
			t.Errorf("view should contain stash message %q", s.Message)
		}
	}
}

func TestListScreen_VisibleRows(t *testing.T) {
	tests := []struct {
		height int
		want   int
	}{
		{height: 0, want: 0},
		{height: 1, want: 1},
		{height: 2, want: 1},
		{height: 3, want: 1},
		{height: 4, want: 1},
		{height: 5, want: 2},
		{height: 6, want: 2},
		{height: 8, want: 3},
		{height: 15, want: 5},
		{height: 30, want: 10},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("height=%d", tt.height), func(t *testing.T) {
			ls := &ListScreen{height: tt.height}
			got := ls.visibleRows()
			if got != tt.want {
				t.Errorf("visibleRows() = %d, want %d", got, tt.want)
			}
		})
	}
}
```

### Step 6: Write integration test with real git repo

```go
// At the bottom of internal/ui/screens/list_test.go (or in a separate
// internal/ui/screens/list_integration_test.go file)

func TestListScreen_Integration_RealRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()

	// Initialize a git repo with known stashes.
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Setup: init repo, create initial commit, create 5 stashes.
	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	writeFile("README.md", "# test repo")
	runGit("add", ".")
	runGit("commit", "-m", "initial commit")

	for i := range 5 {
		writeFile(fmt.Sprintf("file%d.go", i),
			fmt.Sprintf("package main\n// stash %d content\n", i))
		runGit("add", ".")
		runGit("stash", "push", "-m", fmt.Sprintf("stash number %d", i))
	}

	// Verify we have 5 stashes via git CLI.
	cmd := exec.Command("git", "stash", "list")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git stash list failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 stashes, got %d: %s", len(lines), out)
	}

	// Parse stashes into Stash structs (mimicking what the cache would do).
	stashes := make([]core.Stash, 5)
	for i := range 5 {
		stashes[i] = core.Stash{
			Index:   i,
			Message: fmt.Sprintf("stash number %d", 4-i), // git stash is LIFO
		}
	}

	// Create ListScreen, render, verify all stashes appear.
	ls := NewListScreen()
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}
	view := ls.View(state, 120, 30)

	for _, s := range stashes {
		if !strings.Contains(view, s.Message) {
			t.Errorf("rendered view missing stash %q", s.Message)
		}
	}

	// Verify cursor navigation across all 5 stashes.
	for i := range 4 {
		msg := tea.KeyPressMsg{Text: "j"}
		state, _ = ls.Update(msg, state)
		if ls.cursor != i+1 {
			t.Errorf("after %d j presses: cursor = %d, want %d", i+1, ls.cursor, i+1)
		}
	}

	// G should jump to last (index 4).
	msg := tea.KeyPressMsg{Text: "G"}
	state, _ = ls.Update(msg, state)
	if ls.cursor != 4 {
		t.Errorf("after G: cursor = %d, want 4", ls.cursor)
	}

	// g should jump to first (index 0).
	msg = tea.KeyPressMsg{Text: "g"}
	state, _ = ls.Update(msg, state)
	if ls.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", ls.cursor)
	}
}
```

### Step 7: Verify build and tests

```bash
# Format
gofmt -w internal/ui/screens/list.go internal/ui/screens/list_test.go

# Vet
go vet ./internal/ui/screens/...

# Test
go test -v -race ./internal/ui/screens/...

# Full CI
make ci
```

## Verification

### Functional
```bash
# LIST screen compiles
go build ./internal/ui/screens/...

# All tests pass
go test -v -race -count=1 ./internal/ui/screens/...

# Integration test creates real stashes and verifies rendering
go test -v -run TestListScreen_Integration_RealRepo ./internal/ui/screens/...

# Cursor boundary tests pass
go test -v -run TestListScreen_CursorMovement ./internal/ui/screens/...
go test -v -run TestListScreen_SingleItem ./internal/ui/screens/...
go test -v -run TestListScreen_EmptyList ./internal/ui/screens/...

# Key dispatch tests pass
go test -v -run TestListScreen_KeyDispatch ./internal/ui/screens/...

# Scroll tests pass
go test -v -run TestListScreen_ScrollWithManyItems ./internal/ui/screens/...
go test -v -run TestListScreen_PageScroll ./internal/ui/screens/...

# Empty state renders correctly
go test -v -run TestListScreen_EmptyStateRendering ./internal/ui/screens/...

# View renders all visible stashes
go test -v -run TestListScreen_ViewRendersAllVisibleStashes ./internal/ui/screens/...
```

### CI Pipeline
```bash
make ci
```

### TUI Visual Verification (iterm2-driver)
```bash
# Build binary
make build

# Create a test repo with stashes
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
git init && git commit --allow-empty -m "init"
for i in 1 2 3 4 5; do
  echo "content $i" > "file$i.txt"
  git add . && git stash push -m "stash $i"
done

# Launch nidhi and take screenshot of LIST mode
# (using iterm2-driver for automated TUI screenshot capture)
bin/nidhi -C "$TMPDIR"
# Verify: all 5 stashes visible, cursor on first row, empty state if no stashes
```

## Completion Criteria
1. `internal/ui/screens/list.go` implements `ListScreen` with `Update`, `View`, and all navigation methods
2. Custom scrollable list — does NOT use `bubbles/list.Model`
3. Cursor navigation: `j`/`k` (single step), `g`/`G` (jump), `^d`/`^u` (page scroll)
4. Empty state renders helpful message with "Press n" hint (FR-01.6)
5. Key dispatching: `a`/`p`/`d`/`n`/`r`/`e`/`i`/`b` dispatch correct commands or mode switches
6. `Tab` switches to PREVIEW mode, `Enter` switches to DETAIL mode
7. `Enter` on empty list does NOT switch to DETAIL mode
8. Scroll offset adjusts to keep cursor visible in viewport
9. `WindowSizeMsg` updates dimensions and re-clamps cursor
10. All unit tests pass: cursor movement (7 cases), empty list (2 cases), key dispatch (10 cases), scroll, page scroll, resize, visible rows, view rendering
11. Integration test: creates 5 stashes in temp repo, verifies all rendered, cursor navigation works
12. `make ci` passes (lint + test)

## Commit
```
feat: implement LIST screen with cursor navigation and key dispatch

Add internal/ui/screens/list.go — the default view for nidhi (PRD §10
Screen 1). Custom scrollable list with j/k/g/G navigation, ^d/^u page
scroll, empty state message (FR-01.6), and key dispatch for all CRUD
operations. Comprehensive table-driven tests including integration test
with real git stashes in a temp repo.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.1, 10 (Screen 1), 11.2 (LIST keymap), 13.5, 14.2
4. Read tasks 006, 007, 008 to understand dependencies
5. Implement `list.go` following execution steps 1-4
6. Implement `list_test.go` following execution steps 5-6
7. Run `go vet`, `go test -v -race`, `make ci`
8. If iterm2-driver is available, take screenshot of LIST mode and verify against mockup
9. Update this file (Status: DONE) + `docs/PROGRESS.md`
10. Commit with the message above
