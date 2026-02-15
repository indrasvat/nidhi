package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui/layout"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func makeStashes(n int) []core.Stash {
	now := time.Now()
	stashes := make([]core.Stash, n)
	for i := range n {
		stashes[i] = core.Stash{
			Index:      i,
			SHA:        fmt.Sprintf("abc%04d1234567890abcdef1234567890abcdef", i),
			ShortSHA:   fmt.Sprintf("abc%04d", i),
			Message:    fmt.Sprintf("Test stash %d", i),
			Branch:     "main",
			Date:       now.Add(-time.Duration(i) * time.Hour),
			FileCount:  i + 1,
			Insertions: (i + 1) * 10,
			Deletions:  (i + 1) * 3,
		}
	}
	return stashes
}

func makeState(stashCount int) core.AppState {
	return core.AppState{
		Stashes: makeStashes(stashCount),
		Cursor:  0,
		Mode:    core.ModeList,
		Width:   120,
		Height:  30,
	}
}

func TestListScreen_CursorMovement(t *testing.T) {
	tests := []struct {
		name       string
		stashCount int
		keys       []string
		wantCursor int
	}{
		{"j moves cursor down", 5, []string{"j"}, 1},
		{"k at top stays at 0", 5, []string{"k"}, 0},
		{"j j j moves to 3", 5, []string{"j", "j", "j"}, 3},
		{"j past end clamps to last", 3, []string{"j", "j", "j", "j", "j"}, 2},
		{"G jumps to last", 10, []string{"G"}, 9},
		{"g jumps to first", 10, []string{"G", "g"}, 0},
		{"j then k returns to 1", 5, []string{"j", "j", "k"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := NewListScreen(theme.NewAgni())
			ls.width = 120
			ls.height = 30
			state := makeState(tt.stashCount)

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
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 30
	state := makeState(0)

	// j on empty list does nothing.
	state, _ = ls.Update(tea.KeyPressMsg{Text: "j"}, state)
	if ls.cursor != 0 {
		t.Errorf("j on empty: cursor = %d, want 0", ls.cursor)
	}

	// k on empty list does nothing.
	state, _ = ls.Update(tea.KeyPressMsg{Text: "k"}, state)
	if ls.cursor != 0 {
		t.Errorf("k on empty: cursor = %d, want 0", ls.cursor)
	}

	// Enter on empty list does NOT switch to DETAIL.
	state, _ = ls.Update(tea.KeyPressMsg{Code: tea.KeyEnter}, state)
	if state.Mode != core.ModeList {
		t.Errorf("Enter on empty: mode = %v, want ModeList", state.Mode)
	}
}

func TestListScreen_EmptyStateRendering(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	view := ls.View(core.AppState{Stashes: nil}, 120, 30)

	if !strings.Contains(view, "No stashes found") {
		t.Error("empty state should contain 'No stashes found'")
	}
	if !strings.Contains(view, "Press n") {
		t.Error("empty state should mention 'Press n'")
	}
}

func TestListScreen_SingleItem(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 30
	state := makeState(1)

	// j should not move past the only item.
	state, _ = ls.Update(tea.KeyPressMsg{Text: "j"}, state)
	if ls.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (single item)", ls.cursor)
	}

	// G should stay at 0.
	_, _ = ls.Update(tea.KeyPressMsg{Text: "G"}, state)
	if ls.cursor != 0 {
		t.Errorf("G: cursor = %d, want 0", ls.cursor)
	}
}

func TestListScreen_ModeSwitch(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		code     int32
		wantMode core.Mode
	}{
		{"Tab to PREVIEW", "", tea.KeyTab, core.ModePreview},
		{"Enter to DETAIL", "", tea.KeyEnter, core.ModeDetail},
		{"n to NEW STASH", "n", 0, core.ModeNewStash},
		{"e to EXPORT", "e", 0, core.ModeExport},
		{"i to IMPORT", "i", 0, core.ModeImport},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := NewListScreen(theme.NewAgni())
			ls.width = 120
			ls.height = 30
			state := makeState(5)

			msg := tea.KeyPressMsg{Text: tt.key, Code: tt.code}
			newState, _ := ls.Update(msg, state)

			if newState.Mode != tt.wantMode {
				t.Errorf("mode = %v, want %v", newState.Mode, tt.wantMode)
			}
		})
	}
}

func TestListScreen_CRUDDispatch(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantCmd bool
	}{
		{"a dispatches apply", "a", true},
		{"p dispatches pop", "p", true},
		{"r dispatches rename", "r", true},
		{"b dispatches branch", "b", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ls := NewListScreen(theme.NewAgni())
			ls.width = 120
			ls.height = 30
			state := makeState(5)

			msg := tea.KeyPressMsg{Text: tt.key}
			_, cmd := ls.Update(msg, state)

			if tt.wantCmd && cmd == nil {
				t.Error("expected a tea.Cmd but got nil")
			}
		})
	}
}

func TestListScreen_DropDispatch(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 30
	state := makeState(5)

	// d without Ctrl should dispatch drop.
	msg := tea.KeyPressMsg{Text: "d"}
	_, cmd := ls.Update(msg, state)
	if cmd == nil {
		t.Error("d should dispatch drop command")
	}
}

func TestListScreen_CRUDOnEmptyDoesNothing(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 30
	state := makeState(0)

	for _, key := range []string{"a", "p", "d", "r", "b"} {
		msg := tea.KeyPressMsg{Text: key}
		_, cmd := ls.Update(msg, state)
		if cmd != nil {
			t.Errorf("key %q on empty list should not produce a command", key)
		}
	}
}

func TestListScreen_ScrollWithManyItems(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 15
	state := makeState(100)

	for range 10 {
		state, _ = ls.Update(tea.KeyPressMsg{Text: "j"}, state)
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
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 15
	state := makeState(50)

	// Ctrl+D should page down.
	state, _ = ls.Update(tea.KeyPressMsg{Text: "d", Mod: tea.ModCtrl}, state)
	halfPage := ls.visibleRows() / 2
	if ls.cursor != halfPage {
		t.Errorf("after Ctrl+D: cursor = %d, want %d", ls.cursor, halfPage)
	}

	// Ctrl+U should page up.
	_, _ = ls.Update(tea.KeyPressMsg{Text: "u", Mod: tea.ModCtrl}, state)
	if ls.cursor != 0 {
		t.Errorf("after Ctrl+U: cursor = %d, want 0", ls.cursor)
	}
}

func TestListScreen_WindowResize(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	state := makeState(20)

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, _ = ls.Update(msg, state)

	if ls.width != 80 {
		t.Errorf("width = %d, want 80", ls.width)
	}
	expectedHeight := 24 - layout.ChromeHeight
	if ls.height != expectedHeight {
		t.Errorf("height = %d, want %d", ls.height, expectedHeight)
	}
}

func TestListScreen_VisibleRows(t *testing.T) {
	tests := []struct {
		width  int
		height int
		want   int
	}{
		// Narrow terminal (width < 100): 1-line rows, 2 lines per row (1 + blank).
		{80, 0, 0},
		{80, 1, 1},
		{80, 2, 1},
		{80, 3, 2},
		{80, 5, 3},
		{80, 10, 5},
		// Wide terminal (width >= 100): 2-line rows, 3 lines per row (2 + blank).
		{120, 0, 0},
		{120, 1, 1},
		{120, 2, 1},
		{120, 3, 1},
		{120, 4, 1},
		{120, 5, 2},
		{120, 8, 3},
		{120, 14, 5},
		{120, 29, 10},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("w=%d_h=%d", tt.width, tt.height), func(t *testing.T) {
			ls := &ListScreen{width: tt.width, height: tt.height, theme: theme.NewAgni()}
			got := ls.visibleRows()
			if got != tt.want {
				t.Errorf("visibleRows() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestListScreen_ViewRendersStashes(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	stashes := makeStashes(3)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	view := ls.View(state, 120, 30)

	for _, s := range stashes {
		if !strings.Contains(view, s.Message) {
			t.Errorf("view should contain stash message %q", s.Message)
		}
	}
}

func TestListScreen_ViewShowsSelectedCursor(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	state := core.AppState{
		Stashes: makeStashes(3),
		Cursor:  0,
		Mode:    core.ModeList,
	}

	view := ls.View(state, 120, 30)
	if !strings.Contains(view, "\u25b8") { // ▸ cursor indicator
		t.Error("view should contain cursor indicator for selected row")
	}
}

func TestListScreen_CursorSyncsWithState(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 30
	state := makeState(5)

	// Move cursor down twice.
	state, _ = ls.Update(tea.KeyPressMsg{Text: "j"}, state)
	state, _ = ls.Update(tea.KeyPressMsg{Text: "j"}, state)

	if state.Cursor != 2 {
		t.Errorf("state.Cursor = %d, want 2", state.Cursor)
	}
	if ls.Cursor() != 2 {
		t.Errorf("ls.Cursor() = %d, want 2", ls.Cursor())
	}
}

func TestListScreen_OffsetAfterJumpBottom(t *testing.T) {
	ls := NewListScreen(theme.NewAgni())
	ls.width = 120
	ls.height = 9 // fits 3 rows (2-line rows + blank = 3 lines each)
	state := makeState(20)

	// G should jump to last and scroll.
	_, _ = ls.Update(tea.KeyPressMsg{Text: "G"}, state)

	if ls.cursor != 19 {
		t.Errorf("cursor = %d, want 19", ls.cursor)
	}
	if ls.offset == 0 {
		t.Error("offset should not be 0 after jumping to bottom of 20 items")
	}
}
