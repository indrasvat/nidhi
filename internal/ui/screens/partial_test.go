package screens

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func newTestPartial(t *testing.T) *PartialScreen {
	t.Helper()
	s := NewPartialScreen(theme.NewAgni())
	diff := strings.Join([]string{
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,2 +1,4 @@",
		" a",
		"+b",
		"+c",
		" d",
		"",
	}, "\n")
	ps, err := git.ParsePatch(diff)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	s.SetPatchForTest(ps)
	return s
}

func key(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: text}
}

func TestPartial_StartsOnFileRow(t *testing.T) {
	s := newTestPartial(t)
	if got := s.CursorRowForTest(); got != "file" {
		t.Errorf("cursor should start on file row, got %q", got)
	}
}

func TestPartial_HunkModeNavigationSkipsLines(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	// file → hunk (next landable in hunk mode is the hunk header, not a line).
	s.Update(key("j"), st)
	if got := s.CursorRowForTest(); got != "hunk" {
		t.Errorf("after j, expected hunk row, got %q", got)
	}
	// No more landable rows below the hunk in hunk-mode → cursor stays.
	s.Update(key("j"), st)
	if got := s.CursorRowForTest(); got != "hunk" {
		t.Errorf("hunk-mode should not land on lines, got %q", got)
	}
}

func TestPartial_LineModeLandsOnLines(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	s.Update(key("v"), st) // enter line-mode
	if !s.LineModeForTest() {
		t.Fatal("v should enable line mode")
	}
	// file → first changeable line.
	s.Update(key("j"), st)
	if got := s.CursorRowForTest(); got != "line" {
		t.Errorf("line-mode should land on a line, got %q", got)
	}
}

func TestPartial_SpaceTogglesHunk(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	s.Update(key("j"), st) // move to hunk row
	s.Update(key(" "), st) // toggle hunk
	stats := s.PatchForTest().SelectedStats()
	if stats.Added != 2 {
		t.Errorf("toggling hunk should select both added lines, got +%d", stats.Added)
	}
	s.Update(key(" "), st) // toggle off
	if s.PatchForTest().HasSelection() {
		t.Error("second toggle should clear selection")
	}
}

func TestPartial_LineModeTogglesSingleLine(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	s.Update(key("v"), st)
	s.Update(key("j"), st) // first changeable line (b)
	s.Update(key(" "), st)
	stats := s.PatchForTest().SelectedStats()
	if stats.Added != 1 {
		t.Errorf("line-mode toggle should select exactly 1 line, got +%d", stats.Added)
	}
}

func TestPartial_ToggleAll(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	s.Update(key("A"), st)
	if !s.PatchForTest().HasSelection() {
		t.Error("A should select everything")
	}
	s.Update(key("A"), st)
	if s.PatchForTest().HasSelection() {
		t.Error("A again should clear everything")
	}
}

func TestPartial_EnterOpensPromptOnlyWithSelection(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	// No selection → Enter must not open prompt.
	s.Update(key("enter"), st)
	if s.PromptingForTest() {
		t.Error("Enter with no selection should not open prompt")
	}
	// Select all, then Enter opens prompt.
	s.Update(key("A"), st)
	s.Update(key("enter"), st)
	if !s.PromptingForTest() {
		t.Error("Enter with selection should open the message prompt")
	}
}

func TestPartial_EscReturnsToList(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	newState, _ := s.Update(key("escape"), st)
	if newState.Mode != plugin.ModeList {
		t.Errorf("Esc should return to LIST, got %v", newState.Mode)
	}
}

func TestPartial_ViewRendersCheckboxesAndTally(t *testing.T) {
	s := newTestPartial(t)
	st := plugin.AppState{Mode: plugin.ModePartial}
	s.Update(key("A"), st) // select all
	out := s.View(st, 80, 20)
	if !strings.Contains(out, "Selected:") {
		t.Error("view should show the live tally")
	}
	if !strings.Contains(out, "▣") {
		t.Error("view should render a selected checkbox glyph")
	}
	if !strings.Contains(out, "foo.txt") {
		t.Error("view should render the file path")
	}
}

func TestPartial_ViewEmptyState(t *testing.T) {
	s := NewPartialScreen(theme.NewAgni())
	empty, _ := git.ParsePatch("")
	s.SetPatchForTest(empty)
	out := s.View(plugin.AppState{Mode: plugin.ModePartial}, 80, 20)
	if !strings.Contains(out, "No working-tree changes") {
		t.Errorf("expected empty-state message, got:\n%s", out)
	}
}
