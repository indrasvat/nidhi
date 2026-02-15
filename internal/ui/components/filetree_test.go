package components

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func testFiles() []FileEntry {
	return []FileEntry{
		{Path: "src/auth/token.go", Status: FileModified, Category: CategoryStaged},
		{Path: "src/auth/refresh.go", Status: FileAdded, Category: CategoryStaged},
		{Path: "pkg/config/config.go", Status: FileModified, Category: CategoryWorking},
		{Path: "tmp/debug.log", Status: FileAdded, Category: CategoryUntracked},
	}
}

func TestFileTreeModel_SetFiles(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())

	if len(m.Staged) != 2 {
		t.Errorf("Staged count = %d, want 2", len(m.Staged))
	}
	if len(m.Working) != 1 {
		t.Errorf("Working count = %d, want 1", len(m.Working))
	}
	if len(m.Untracked) != 1 {
		t.Errorf("Untracked count = %d, want 1", len(m.Untracked))
	}
}

func TestFileTreeModel_FilesSortedByPath(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	files := []FileEntry{
		{Path: "z/file.go", Status: FileModified, Category: CategoryStaged},
		{Path: "a/file.go", Status: FileAdded, Category: CategoryStaged},
		{Path: "m/file.go", Status: FileModified, Category: CategoryStaged},
	}
	m.SetFiles(files)

	if m.Staged[0].Path != "a/file.go" {
		t.Errorf("first staged file = %q, want 'a/file.go'", m.Staged[0].Path)
	}
	if m.Staged[2].Path != "z/file.go" {
		t.Errorf("last staged file = %q, want 'z/file.go'", m.Staged[2].Path)
	}
}

func TestFileTreeModel_FileCount(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())

	if m.FileCount() != 4 {
		t.Errorf("FileCount() = %d, want 4", m.FileCount())
	}
}

func TestFileTreeModel_CursorNavigation(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())

	if m.Cursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.Cursor())
	}

	// Move down: staged header -> file1 -> file2 -> working header -> file -> untracked header -> file
	m.CursorDown() // staged file 1
	m.CursorDown() // staged file 2
	m.CursorDown() // working header
	m.CursorDown() // working file
	m.CursorDown() // untracked header
	m.CursorDown() // untracked file

	// Should not go beyond last item.
	m.CursorDown()
	m.CursorDown()

	// Move up.
	m.CursorUp()
	m.CursorUp()
}

func TestFileTreeModel_ToggleCollapse(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())

	// Cursor is at staged category header (index 0).
	m.ToggleCollapse()

	// Staged should now be collapsed.
	visible := m.visibleItems()
	// After collapsing staged, should have: staged header, working header, working file, untracked header, untracked file.
	if len(visible) != 5 {
		t.Errorf("after collapsing staged: visible items = %d, want 5", len(visible))
	}

	// Uncollapse.
	m.cursor = 0
	m.ToggleCollapse()

	visible = m.visibleItems()
	// All expanded: 3 headers + 4 files = 7.
	if len(visible) != 7 {
		t.Errorf("after expanding staged: visible items = %d, want 7", len(visible))
	}
}

func TestFileTreeModel_SelectedFile(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())

	// At category header, SelectedFile should be nil.
	if m.SelectedFile() != nil {
		t.Error("SelectedFile() at category header should be nil")
	}

	// Move to first file.
	m.CursorDown()
	f := m.SelectedFile()
	if f == nil {
		t.Fatal("SelectedFile() at first file should not be nil")
	}
	if f.Category != CategoryStaged {
		t.Errorf("first file category = %v, want Staged", f.Category)
	}
}

func TestFileTreeModel_ViewRenders(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())

	view := m.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "staged") {
		t.Error("view should contain 'staged'")
	}
	if !strings.Contains(plain, "working") {
		t.Error("view should contain 'working'")
	}
	if !strings.Contains(plain, "untracked") {
		t.Error("view should contain 'untracked'")
	}
}

func TestFileTreeModel_ViewShowsCounts(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())

	view := m.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "(2)") {
		t.Error("view should show count (2) for staged")
	}
	if !strings.Contains(plain, "(1)") {
		t.Error("view should show count (1)")
	}
}

func TestFileTreeModel_EmptyFiles(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(nil)

	view := m.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "No files") {
		t.Errorf("empty tree should show 'No files', got: %q", plain)
	}
}

func TestFileTreeModel_HidesEmptyCategories(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	// Only working files — staged and untracked should be hidden.
	m.SetFiles([]FileEntry{
		{Path: "pkg/config.go", Status: FileModified, Category: CategoryWorking},
	})

	view := m.View()
	plain := stripAnsi(view)

	if strings.Contains(plain, "staged") {
		t.Error("empty 'staged' category should be hidden")
	}
	if strings.Contains(plain, "untracked") {
		t.Error("empty 'untracked' category should be hidden")
	}
	if !strings.Contains(plain, "working") {
		t.Error("non-empty 'working' category should be visible")
	}
}

func TestFileTreeModel_FocusDimming(t *testing.T) {
	m := NewFileTreeModel(theme.NewAgni(), 40, false)
	m.SetFiles(testFiles())
	m.CursorDown() // Move to first file.

	// Focused view should use gold accent.
	m.SetFocused(true)
	focusedView := m.View()

	// Unfocused view should use dimmed styling.
	m.SetFocused(false)
	unfocusedView := m.View()

	// The ANSI escape codes differ between focused and unfocused.
	if focusedView == unfocusedView {
		t.Error("focused and unfocused views should differ in styling")
	}
}

func TestFileStatus_String(t *testing.T) {
	tests := []struct {
		status FileStatus
		want   string
	}{
		{FileModified, "~"},
		{FileAdded, "+"},
		{FileRemoved, "-"},
		{FileRenamed, "\u2192"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("FileStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestFileCategory_String(t *testing.T) {
	tests := []struct {
		cat  FileCategory
		want string
	}{
		{CategoryStaged, "staged"},
		{CategoryWorking, "working"},
		{CategoryUntracked, "untracked"},
	}

	for _, tt := range tests {
		if got := tt.cat.String(); got != tt.want {
			t.Errorf("FileCategory(%d).String() = %q, want %q", tt.cat, got, tt.want)
		}
	}
}
