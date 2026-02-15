package screens

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui/components"
	"github.com/indrasvat/nidhi/internal/ui/layout"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func newTestDetail() *DetailScreen {
	th := theme.NewAgni()
	ds := NewDetailScreen(th)
	ds.width = 120
	ds.height = 30
	ds.recalcSplit()
	ds.SetDiff(testDiff())
	ds.SetPreviousMode(core.ModeList)
	return ds
}

func TestInferFileStatus(t *testing.T) {
	tests := []struct {
		name string
		diff string
		want components.FileStatus
	}{
		{"modified", "--- a/foo\n+++ b/foo\n@@ -1 +1,2 @@\n", components.FileModified},
		{"new file", "new file mode 100644\n--- /dev/null\n+++ b/bar\n", components.FileAdded},
		{"deleted", "deleted file mode 100644\n--- a/baz\n+++ /dev/null\n", components.FileRemoved},
		{"renamed", "rename from old.go\nrename to new.go\n", components.FileRenamed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferFileStatus(tt.diff)
			if got != tt.want {
				t.Errorf("inferFileStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetailScreen_SetDiff(t *testing.T) {
	ds := newTestDetail()

	if ds.tree.FileCount() != 3 {
		t.Errorf("file count = %d, want 3", ds.tree.FileCount())
	}

	if len(ds.fileDiffs) != 3 {
		t.Errorf("fileDiffs count = %d, want 3", len(ds.fileDiffs))
	}

	// After SetDiff, cursor should be on the first file (not a category header).
	if ds.tree.SelectedFile() == nil {
		t.Error("cursor should be on a file node after SetDiff, not a category header")
	}
}

func TestDetailScreen_SetDiff_Empty(t *testing.T) {
	th := theme.NewAgni()
	ds := NewDetailScreen(th)
	ds.SetDiff("")

	if ds.tree.FileCount() != 0 {
		t.Errorf("file count = %d, want 0", ds.tree.FileCount())
	}
	if len(ds.fileDiffs) != 0 {
		t.Errorf("fileDiffs count = %d, want 0", len(ds.fileDiffs))
	}
}

func TestDetailScreen_FocusSwitching(t *testing.T) {
	ds := newTestDetail()
	state := core.AppState{Mode: core.ModeDetail}

	// Initial focus should be on tree.
	if ds.focused != PaneTree {
		t.Errorf("initial focus = %v, want PaneTree", ds.focused)
	}

	// Tab switches to diff.
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	state, _ = ds.Update(msg, state)
	if ds.focused != PaneDiff {
		t.Errorf("after Tab: focus = %v, want PaneDiff", ds.focused)
	}

	// Tab again switches back to tree.
	_, _ = ds.Update(msg, state)
	if ds.focused != PaneTree {
		t.Errorf("after second Tab: focus = %v, want PaneTree", ds.focused)
	}
}

func TestDetailScreen_EscReturnsToList(t *testing.T) {
	ds := newTestDetail()
	ds.SetPreviousMode(core.ModeList)
	state := core.AppState{Mode: core.ModeDetail}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	newState, _ := ds.Update(msg, state)

	if newState.Mode != core.ModeList {
		t.Errorf("mode = %v, want ModeList", newState.Mode)
	}
}

func TestDetailScreen_EscReturnsToPreview(t *testing.T) {
	ds := newTestDetail()
	ds.SetPreviousMode(core.ModePreview)
	state := core.AppState{Mode: core.ModeDetail}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	newState, _ := ds.Update(msg, state)

	if newState.Mode != core.ModePreview {
		t.Errorf("mode = %v, want ModePreview", newState.Mode)
	}
}

func TestDetailScreen_TreeNavigation(t *testing.T) {
	ds := newTestDetail()
	ds.focused = PaneTree
	state := core.AppState{Mode: core.ModeDetail}

	initialCursor := ds.tree.Cursor()

	// j moves down.
	msg := tea.KeyPressMsg{Text: "j"}
	state, _ = ds.Update(msg, state)
	if ds.tree.Cursor() <= initialCursor {
		t.Errorf("j should move cursor down: got %d, was %d", ds.tree.Cursor(), initialCursor)
	}

	// k moves back up.
	msg = tea.KeyPressMsg{Text: "k"}
	_, _ = ds.Update(msg, state)
	if ds.tree.Cursor() != initialCursor {
		t.Errorf("k should move cursor back: got %d, want %d", ds.tree.Cursor(), initialCursor)
	}
}

func TestDetailScreen_DiffNavigation(t *testing.T) {
	ds := newTestDetail()
	ds.focused = PaneDiff
	state := core.AppState{Mode: core.ModeDetail}

	treeCursorBefore := ds.tree.Cursor()

	// j in diff pane should NOT move tree cursor.
	msg := tea.KeyPressMsg{Text: "j"}
	_, _ = ds.Update(msg, state)
	if ds.tree.Cursor() != treeCursorBefore {
		t.Error("j in diff pane should not move tree cursor")
	}

	// k in diff pane should NOT move tree cursor either.
	msg = tea.KeyPressMsg{Text: "k"}
	_, _ = ds.Update(msg, state)
	if ds.tree.Cursor() != treeCursorBefore {
		t.Error("k in diff pane should not move tree cursor")
	}
}

func TestDetailScreen_ExpandCollapse(t *testing.T) {
	ds := newTestDetail()
	ds.focused = PaneTree
	state := core.AppState{Mode: core.ModeDetail}

	// After SetDiff + selectFirstFile, cursor is on the first file.
	// Move up to the "working" category header.
	ds.tree.CursorUp()

	// Verify we're on a category header.
	if ds.tree.SelectedFile() != nil {
		t.Fatal("expected to be on a category header")
	}

	// View before collapse should contain file names.
	viewBefore := ds.tree.View()
	if !strings.Contains(viewBefore, "limiter.go") {
		t.Error("before collapse: view should contain file name 'limiter.go'")
	}

	// Enter collapses the group.
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	state, _ = ds.Update(msg, state)

	viewAfter := ds.tree.View()
	if strings.Contains(viewAfter, "limiter.go") {
		t.Error("after collapse: view should not contain file name 'limiter.go'")
	}

	// Enter again expands.
	_, _ = ds.Update(msg, state)

	viewExpanded := ds.tree.View()
	if !strings.Contains(viewExpanded, "limiter.go") {
		t.Error("after expand: view should contain file name 'limiter.go' again")
	}
}

func TestDetailScreen_FileSelectionUpdatesDiff(t *testing.T) {
	ds := newTestDetail()
	ds.focused = PaneTree
	state := core.AppState{Mode: core.ModeDetail}

	// Get diff view content for the current file.
	diffBefore := ds.diffView.View()

	// Move to next file.
	msg := tea.KeyPressMsg{Text: "j"}
	_, _ = ds.Update(msg, state)

	diffAfter := ds.diffView.View()

	// Diff content should change when selecting a different file.
	if diffBefore == diffAfter {
		t.Error("diff content should update when selecting a different file")
	}
}

func TestDetailScreen_SplitMath(t *testing.T) {
	tests := []struct {
		totalWidth int
		wantTreeW  int
		wantDiffW  int
	}{
		{120, 29, 90},  // ComputeSplit: usable=119, primary=int(119*0.25)=29, secondary=90
		{80, 19, 60},   // usable=79, primary=int(79*0.25)=19, secondary=60
		{200, 49, 150}, // usable=199, primary=int(199*0.25)=49, secondary=150
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("width=%d", tt.totalWidth), func(t *testing.T) {
			split := layout.ComputeSplit(tt.totalWidth, layout.DetailSplitRatio)
			if split.PrimarySize != tt.wantTreeW {
				t.Errorf("tree width = %d, want %d", split.PrimarySize, tt.wantTreeW)
			}
			if split.SecondarySize != tt.wantDiffW {
				t.Errorf("diff width = %d, want %d", split.SecondarySize, tt.wantDiffW)
			}
			if split.DividerSize != 1 {
				t.Errorf("divider size = %d, want 1", split.DividerSize)
			}
		})
	}
}

func TestDetailScreen_SplitCollapse(t *testing.T) {
	// Very narrow terminal — not enough for both panes.
	split := layout.ComputeSplit(30, layout.DetailSplitRatio)
	if split.SecondarySize != 0 {
		t.Errorf("secondary should be 0 at width 30, got %d", split.SecondarySize)
	}
	if split.DividerSize != 0 {
		t.Errorf("divider should be 0 when collapsed, got %d", split.DividerSize)
	}
}

func TestDetailScreen_WindowResize(t *testing.T) {
	ds := newTestDetail()
	state := core.AppState{Mode: core.ModeDetail}

	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, _ = ds.Update(msg, state)

	if ds.width != 80 {
		t.Errorf("width = %d, want 80", ds.width)
	}
	expectedHeight := 24 - layout.ChromeHeight
	if ds.height != expectedHeight {
		t.Errorf("height = %d, want %d", ds.height, expectedHeight)
	}
}

func TestDetailScreen_ViewRenders(t *testing.T) {
	ds := newTestDetail()
	state := core.AppState{
		Stashes: makeStashes(5),
		Mode:    core.ModeDetail,
	}

	view := ds.View(state, 120, 30)
	if view == "" {
		t.Error("View should not return empty string")
	}

	// Should contain the vertical divider character.
	if !strings.Contains(view, "\u2502") {
		t.Error("view should contain vertical divider")
	}
}

func TestDetailScreen_ViewCollapsedWidth(t *testing.T) {
	ds := newTestDetail()
	state := core.AppState{Mode: core.ModeDetail}

	// Very narrow width — should show tree only, no panic.
	view := ds.View(state, 30, 20)
	if view == "" {
		t.Error("collapsed view should not be empty")
	}
}

func TestDetailScreen_PageScroll(t *testing.T) {
	ds := newTestDetail()
	state := core.AppState{Mode: core.ModeDetail}

	// Ctrl+D should not panic.
	msg := tea.KeyPressMsg{Text: "d", Mod: tea.ModCtrl}
	_, _ = ds.Update(msg, state)

	// Ctrl+U should not panic.
	msg = tea.KeyPressMsg{Text: "u", Mod: tea.ModCtrl}
	_, _ = ds.Update(msg, state)
}

func TestDetailScreen_EnterOnFileNode(t *testing.T) {
	ds := newTestDetail()
	ds.focused = PaneTree
	state := core.AppState{Mode: core.ModeDetail}

	// Cursor is on a file node (after selectFirstFile).
	if ds.tree.SelectedFile() == nil {
		t.Fatal("expected cursor on a file node")
	}

	// Enter on a file node should be a no-op (ToggleCollapse only works on category headers).
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := ds.Update(msg, state)
	if newState.Mode != core.ModeDetail {
		t.Errorf("mode = %v, want ModeDetail (Enter on file should not change mode)", newState.Mode)
	}
}

func TestDetailScreen_EnterInDiffPane(t *testing.T) {
	ds := newTestDetail()
	ds.focused = PaneDiff
	state := core.AppState{Mode: core.ModeDetail}

	// Enter in diff pane should be a no-op.
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := ds.Update(msg, state)
	if newState.Mode != core.ModeDetail {
		t.Errorf("mode = %v, want ModeDetail", newState.Mode)
	}
}

func TestDetailScreen_EmptyDiffView(t *testing.T) {
	th := theme.NewAgni()
	ds := NewDetailScreen(th)
	ds.width = 120
	ds.height = 30
	ds.recalcSplit()
	// Don't set any diff.
	state := core.AppState{Mode: core.ModeDetail}

	// Should render without panic.
	view := ds.View(state, 120, 30)
	if view == "" {
		t.Error("view should not be empty even with no diff")
	}
}

func TestDetailScreen_FocusedGetter(t *testing.T) {
	ds := newTestDetail()

	if ds.Focused() != PaneTree {
		t.Errorf("initial Focused() = %v, want PaneTree", ds.Focused())
	}

	state := core.AppState{Mode: core.ModeDetail}
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	_, _ = ds.Update(msg, state)

	if ds.Focused() != PaneDiff {
		t.Errorf("after Tab: Focused() = %v, want PaneDiff", ds.Focused())
	}
}
