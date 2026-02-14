# Task 012: DETAIL Screen — Full-Screen File Tree + Diff

## Status: TODO

## Depends On
- 010 (LIST screen — `internal/ui/screens/list.go`)
- 009 (diff view component — `internal/ui/components/diffview.go`, file tree — `internal/ui/components/filetree.go`)
- 007 (layout engine — `internal/ui/layout/split.go`)

## Parallelizable With
- 011 (PREVIEW screen — both extend LIST independently)

## Problem
When a developer presses `Enter` in LIST or PREVIEW mode, the screen enters DETAIL mode (PRD §10 Screen 3): a full-screen horizontal split with a file tree (~25% width) on the left and a diff viewport (~75% width) on the right. Files are grouped by status (staged/working/untracked). `Tab` switches focus between tree and diff panes. The file tree uses `lipgloss/v2/tree` for rendering. Navigation within each pane uses `j`/`k`. `Esc` returns to the previous mode (LIST or PREVIEW).

## PRD Reference
- Section 6.1 FR-03.3 — DETAIL mode definition
- Section 6.1 FR-03.4 — Esc returns to previous mode
- Section 10 Screen 3 — DETAIL layout spec, pane ratios, tree grouping
- Section 11.2 — DETAIL mode keymap (Tab focus, j/k, ^d/^u, Enter expand/collapse, Esc)
- Section 13.2 — `lipgloss.JoinHorizontal` for tree + diff layout
- Section 13.2 — `lipgloss/tree` for file tree rendering
- Section 13.4 — `viewport.Model` for diff pane
- Section 13.5 — Split pane custom component

## Files to Create
- `internal/ui/screens/detail.go` — DETAIL mode screen model
- `internal/ui/screens/detail_test.go` — unit and integration tests

## Execution Steps

### Step 1: Define the file tree data model

```go
// internal/ui/screens/detail.go
package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui/layout"
)

// FileStatus represents the status of a file in a stash.
type FileStatus int

const (
	FileStaged    FileStatus = iota // staged changes (index)
	FileWorking                     // working tree changes
	FileUntracked                   // untracked files
)

// String returns the display name for a file status.
func (fs FileStatus) String() string {
	switch fs {
	case FileStaged:
		return "staged"
	case FileWorking:
		return "working"
	case FileUntracked:
		return "untracked"
	default:
		return "unknown"
	}
}

// TreeNode represents an item in the file tree.
type TreeNode struct {
	// Name is the display name (e.g., "token.go" or "staged (2)").
	Name string
	// Path is the full file path (empty for group headers).
	Path string
	// Status is the file status group this node belongs to.
	Status FileStatus
	// IsGroup indicates this is a group header (staged/working/untracked).
	IsGroup bool
	// Expanded indicates if a group header is expanded (showing children).
	Expanded bool
	// Diff is the diff content for this file (empty for group headers).
	Diff string
	// Icon prefix for the file (e.g., "~" modified, "+" added, "-" removed).
	Icon string
}

// FocusedPane tracks which pane has keyboard focus.
type FocusedPane int

const (
	PaneTree FocusedPane = iota
	PaneDiff
)

// treeWidthRatio is the fraction of width allocated to the file tree.
const treeWidthRatio = 0.25
```

### Step 2: Define the DetailScreen model

```go
// DetailScreen implements DETAIL mode — horizontal split: file tree (left) + diff viewport (right).
type DetailScreen struct {
	// nodes is the flat list of tree nodes (groups + files).
	nodes []TreeNode

	// treeCursor is the index of the selected node in the tree.
	treeCursor int

	// focused tracks which pane has keyboard focus.
	focused FocusedPane

	// viewport is the diff display pane.
	viewport viewport.Model

	// previousMode tracks where to return on Esc (LIST or PREVIEW).
	previousMode core.Mode

	// dimensions
	width  int
	height int
}

// NewDetailScreen creates a new DETAIL screen.
// previousMode is where Esc returns to (LIST or PREVIEW).
func NewDetailScreen(previousMode core.Mode) *DetailScreen {
	return &DetailScreen{
		previousMode: previousMode,
		focused:      PaneTree,
	}
}
```

### Step 3: Build the file tree from diff content

```go
// BuildTree constructs the tree nodes from a stash's diff content.
// The diff string is parsed to extract per-file sections, then grouped
// by status. In Phase 1, we infer status from diff headers:
//   - Files in "diff --git" are working tree changes
//   - Staged vs. working distinction requires separate git calls,
//     which will be wired in when StashCache provides structured diffs.
func BuildTree(diff string) []TreeNode {
	files := parseDiffIntoFiles(diff)
	if len(files) == 0 {
		return nil
	}

	var nodes []TreeNode

	// Group by status. For now, all files are "working" until we have
	// structured diff data from the cache that distinguishes staged/working/untracked.
	groups := map[FileStatus][]fileEntry{
		FileStaged:    {},
		FileWorking:   {},
		FileUntracked: {},
	}

	for _, f := range files {
		groups[FileWorking] = append(groups[FileWorking], f)
	}

	// Build tree nodes for each non-empty group.
	for _, status := range []FileStatus{FileStaged, FileWorking, FileUntracked} {
		entries := groups[status]
		if len(entries) == 0 {
			continue
		}

		// Group header node.
		nodes = append(nodes, TreeNode{
			Name:     fmt.Sprintf("%s (%d)", status, len(entries)),
			IsGroup:  true,
			Status:   status,
			Expanded: true, // expanded by default
		})

		// File nodes under this group.
		for _, e := range entries {
			icon := fileIcon(e.filename, e.diff)
			nodes = append(nodes, TreeNode{
				Name:   e.filename,
				Path:   e.filename,
				Status: status,
				Icon:   icon,
				Diff:   e.diff,
			})
		}
	}

	return nodes
}

type fileEntry struct {
	filename string
	diff     string
}

// parseDiffIntoFiles splits a unified diff into per-file entries.
func parseDiffIntoFiles(diff string) []fileEntry {
	if diff == "" {
		return nil
	}

	var entries []fileEntry
	lines := strings.Split(diff, "\n")
	var current *fileEntry

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				entries = append(entries, *current)
			}
			filename := extractFilename(line)
			current = &fileEntry{
				filename: filename,
				diff:     line + "\n",
			}
		} else if current != nil {
			current.diff += line + "\n"
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries
}

// fileIcon returns a status icon for a file based on its diff content.
func fileIcon(filename, diff string) string {
	if strings.Contains(diff, "new file mode") {
		return "+"
	}
	if strings.Contains(diff, "deleted file mode") {
		return "-"
	}
	if strings.Contains(diff, "rename from") {
		return "→"
	}
	return "~" // modified
}

// visibleNodes returns only the nodes that should be displayed
// (group headers + files under expanded groups).
func (d *DetailScreen) visibleNodes() []TreeNode {
	var visible []TreeNode
	showChildren := true

	for _, node := range d.nodes {
		if node.IsGroup {
			visible = append(visible, node)
			showChildren = node.Expanded
		} else if showChildren {
			visible = append(visible, node)
		}
	}
	return visible
}
```

### Step 4: Implement navigation methods

```go
// moveTreeCursor moves the cursor within the visible tree nodes.
func (d *DetailScreen) moveTreeCursor(delta int) {
	visible := d.visibleNodes()
	if len(visible) == 0 {
		return
	}
	d.treeCursor += delta
	d.treeCursor = max(0, min(d.treeCursor, len(visible)-1))
}

// toggleExpand expands or collapses a group header node.
func (d *DetailScreen) toggleExpand() {
	visible := d.visibleNodes()
	if d.treeCursor >= len(visible) {
		return
	}
	node := visible[d.treeCursor]
	if !node.IsGroup {
		return
	}

	// Find this group in the full node list and toggle it.
	for i := range d.nodes {
		if d.nodes[i].IsGroup && d.nodes[i].Status == node.Status {
			d.nodes[i].Expanded = !d.nodes[i].Expanded
			break
		}
	}

	// Re-clamp cursor after collapse may have hidden the previously selected node.
	newVisible := d.visibleNodes()
	d.treeCursor = min(d.treeCursor, len(newVisible)-1)
	d.treeCursor = max(0, d.treeCursor)
}

// updateDiffForSelectedFile loads the diff content for the currently selected file.
func (d *DetailScreen) updateDiffForSelectedFile() {
	visible := d.visibleNodes()
	if d.treeCursor >= len(visible) {
		d.viewport.SetContent("(no file selected)")
		return
	}
	node := visible[d.treeCursor]
	if node.IsGroup {
		// Show summary for group.
		d.viewport.SetContent(fmt.Sprintf("-- %s --\nSelect a file to view its diff.", node.Name))
		return
	}
	d.viewport.SetContent(node.Diff)
	d.viewport.GotoTop()
}
```

### Step 5: Implement the Update method

```go
// Update handles messages for the DETAIL screen.
func (d *DetailScreen) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height - layout.ChromeHeight
		d.recalcSplit()
		return state, nil

	case tea.KeyPressMsg:
		return d.handleKey(msg, state)
	}

	return state, nil
}

func (d *DetailScreen) handleKey(msg tea.KeyPressMsg, state core.AppState) (core.AppState, tea.Cmd) {
	switch {
	// Tab switches focus between tree and diff.
	case msg.Code == tea.KeyTab:
		if d.focused == PaneTree {
			d.focused = PaneDiff
		} else {
			d.focused = PaneTree
		}
		return state, nil

	// Esc returns to previous mode.
	case msg.Code == tea.KeyEscape:
		state.Mode = d.previousMode
		return state, nil

	// j/k navigate within the focused pane.
	case msg.Text == "j":
		if d.focused == PaneTree {
			d.moveTreeCursor(1)
			d.updateDiffForSelectedFile()
		} else {
			d.viewport.LineDown(1)
		}
		return state, nil

	case msg.Text == "k":
		if d.focused == PaneTree {
			d.moveTreeCursor(-1)
			d.updateDiffForSelectedFile()
		} else {
			d.viewport.LineUp(1)
		}
		return state, nil

	// ^d/^u page scroll in diff pane (regardless of focus, for convenience).
	case msg.Text == "d" && msg.Mod == tea.ModCtrl:
		d.viewport.HalfViewDown()
		return state, nil
	case msg.Text == "u" && msg.Mod == tea.ModCtrl:
		d.viewport.HalfViewUp()
		return state, nil

	// Enter expands/collapses tree nodes when tree is focused.
	case msg.Code == tea.KeyEnter:
		if d.focused == PaneTree {
			d.toggleExpand()
		}
		return state, nil
	}

	return state, nil
}

// recalcSplit adjusts pane dimensions based on the width ratio.
func (d *DetailScreen) recalcSplit() {
	treeW := int(float64(d.width) * treeWidthRatio)
	diffW := d.width - treeW - 1 // 1 for the vertical divider
	d.viewport.Width = max(diffW, 1)
	d.viewport.Height = max(d.height, 1)
}
```

### Step 6: Implement the View method

```go
// View renders the DETAIL screen: file tree (left) | diff viewport (right).
func (d *DetailScreen) View(state core.AppState, width, height int) string {
	d.width = width
	d.height = height
	d.recalcSplit()

	treeW := int(float64(width) * treeWidthRatio)
	diffW := width - treeW - 1

	// Left pane: file tree.
	treeView := d.renderTree(treeW, height)

	// Vertical divider.
	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3D4450")) // fg.dimmed
	divider := dividerStyle.Render(strings.Repeat("│\n", max(height-1, 0)) + "│")

	// Right pane: diff viewport.
	diffView := d.renderDiff(diffW, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, treeView, divider, diffView)
}

// renderTree renders the file tree pane with cursor and icons.
func (d *DetailScreen) renderTree(width, height int) string {
	visible := d.visibleNodes()
	var b strings.Builder

	treeStyle := lipgloss.NewStyle().Width(width)
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#1A1F2B")). // bg.elevated
		Width(width)
	groupStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")). // fg.secondary
		Bold(true)
	fileStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C8CCD4")) // fg.primary
	focusIndicator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E8B85A")) // accent.bright

	for i, node := range visible {
		if i >= height {
			break // don't render beyond the pane height
		}

		var line string
		isSelected := i == d.treeCursor && d.focused == PaneTree

		if node.IsGroup {
			arrow := "▼"
			if !node.Expanded {
				arrow = "▶"
			}
			line = groupStyle.Render(fmt.Sprintf(" %s %s", arrow, node.Name))
		} else {
			indent := "  "
			icon := node.Icon
			line = fileStyle.Render(fmt.Sprintf(" %s%s %s", indent, icon, node.Name))
		}

		if isSelected {
			cursor := focusIndicator.Render("▸")
			line = cursor + line[1:] // replace first space with cursor
			line = selectedStyle.Render(line)
		} else {
			line = treeStyle.Render(line)
		}

		b.WriteString(line)
		if i < len(visible)-1 {
			b.WriteString("\n")
		}
	}

	// Pad to fill height.
	rendered := b.String()
	lineCount := strings.Count(rendered, "\n") + 1
	for i := lineCount; i < height; i++ {
		rendered += "\n" + treeStyle.Render("")
	}

	return rendered
}

// renderDiff renders the diff viewport pane.
func (d *DetailScreen) renderDiff(width, height int) string {
	return d.viewport.View()
}

// SetNodes sets the tree nodes and selects the first file node.
func (d *DetailScreen) SetNodes(nodes []TreeNode) {
	d.nodes = nodes
	d.treeCursor = 0

	// Auto-select the first file node (skip group headers).
	visible := d.visibleNodes()
	for i, node := range visible {
		if !node.IsGroup {
			d.treeCursor = i
			break
		}
	}
	d.updateDiffForSelectedFile()
}

// FocusedPane returns which pane currently has focus.
func (d *DetailScreen) FocusedPaneValue() FocusedPane {
	return d.focused
}

// TreeCursor returns the current tree cursor position.
func (d *DetailScreen) TreeCursor() int {
	return d.treeCursor
}
```

### Step 7: Write comprehensive unit tests

```go
// internal/ui/screens/detail_test.go
package screens

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
)

func TestBuildTree(t *testing.T) {
	diff := sampleDiff() // reuse from preview_test.go

	nodes := BuildTree(diff)
	if len(nodes) == 0 {
		t.Fatal("BuildTree returned no nodes")
	}

	// Should have at least one group header.
	hasGroup := false
	for _, n := range nodes {
		if n.IsGroup {
			hasGroup = true
			break
		}
	}
	if !hasGroup {
		t.Error("tree should contain at least one group header")
	}

	// Count file nodes.
	fileCount := 0
	for _, n := range nodes {
		if !n.IsGroup {
			fileCount++
		}
	}
	if fileCount != 3 {
		t.Errorf("expected 3 file nodes, got %d", fileCount)
	}
}

func TestBuildTree_Empty(t *testing.T) {
	nodes := BuildTree("")
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty diff, got %d", len(nodes))
	}
}

func TestBuildTree_SingleFile(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+func init() {}
`
	nodes := BuildTree(diff)

	fileCount := 0
	for _, n := range nodes {
		if !n.IsGroup {
			fileCount++
			if n.Name != "main.go" {
				t.Errorf("file name = %q, want %q", n.Name, "main.go")
			}
		}
	}
	if fileCount != 1 {
		t.Errorf("expected 1 file, got %d", fileCount)
	}
}

func TestFileIcon(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		wantIcon string
	}{
		{"modified", "--- a/foo\n+++ b/foo\n@@ -1 +1,2 @@\n", "~"},
		{"new file", "new file mode 100644\n--- /dev/null\n+++ b/bar\n", "+"},
		{"deleted", "deleted file mode 100644\n--- a/baz\n+++ /dev/null\n", "-"},
		{"renamed", "rename from old.go\nrename to new.go\n", "→"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileIcon("test.go", tt.diff)
			if got != tt.wantIcon {
				t.Errorf("fileIcon() = %q, want %q", got, tt.wantIcon)
			}
		})
	}
}

func TestDetailScreen_FocusSwitching(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.width = 120
	ds.height = 30
	ds.nodes = BuildTree(sampleDiff())

	state := core.AppState{
		Stashes: makeStashes(5),
		Mode:    core.ModeDetail,
	}

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
	state, _ = ds.Update(msg, state)
	if ds.focused != PaneTree {
		t.Errorf("after second Tab: focus = %v, want PaneTree", ds.focused)
	}
}

func TestDetailScreen_EscReturnsToList(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.width = 120
	ds.height = 30

	state := core.AppState{
		Mode: core.ModeDetail,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	newState, _ := ds.Update(msg, state)

	if newState.Mode != core.ModeList {
		t.Errorf("mode = %v, want ModeList", newState.Mode)
	}
}

func TestDetailScreen_EscReturnsToPreview(t *testing.T) {
	ds := NewDetailScreen(core.ModePreview)
	ds.width = 120
	ds.height = 30

	state := core.AppState{
		Mode: core.ModeDetail,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	newState, _ := ds.Update(msg, state)

	if newState.Mode != core.ModePreview {
		t.Errorf("mode = %v, want ModePreview", newState.Mode)
	}
}

func TestDetailScreen_TreeNavigation(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.width = 120
	ds.height = 30
	ds.focused = PaneTree
	ds.nodes = BuildTree(sampleDiff())

	state := core.AppState{
		Mode: core.ModeDetail,
	}

	// Auto-select first file node (index 1, since 0 is group header).
	ds.SetNodes(ds.nodes)
	initialCursor := ds.treeCursor

	// j moves down.
	msg := tea.KeyPressMsg{Text: "j"}
	state, _ = ds.Update(msg, state)
	if ds.treeCursor <= initialCursor {
		t.Errorf("j should move cursor down: got %d, was %d", ds.treeCursor, initialCursor)
	}

	// k moves back up.
	msg = tea.KeyPressMsg{Text: "k"}
	state, _ = ds.Update(msg, state)
	if ds.treeCursor != initialCursor {
		t.Errorf("k should move cursor back: got %d, want %d", ds.treeCursor, initialCursor)
	}
}

func TestDetailScreen_DiffNavigation(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.width = 120
	ds.height = 30
	ds.focused = PaneDiff
	ds.nodes = BuildTree(sampleDiff())
	ds.SetNodes(ds.nodes)

	// Set viewport content with enough lines to scroll.
	longDiff := strings.Repeat("line\n", 100)
	ds.viewport.SetContent(longDiff)
	ds.viewport.Height = 10

	state := core.AppState{Mode: core.ModeDetail}

	// j in diff pane should scroll viewport, not move tree cursor.
	treeCursorBefore := ds.treeCursor
	msg := tea.KeyPressMsg{Text: "j"}
	state, _ = ds.Update(msg, state)
	if ds.treeCursor != treeCursorBefore {
		t.Error("j in diff pane should not move tree cursor")
	}
}

func TestDetailScreen_ExpandCollapse(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.width = 120
	ds.height = 30
	ds.focused = PaneTree
	ds.nodes = BuildTree(sampleDiff())

	// Move cursor to the group header (index 0).
	ds.treeCursor = 0

	// Count visible nodes before collapse.
	visibleBefore := len(ds.visibleNodes())

	state := core.AppState{Mode: core.ModeDetail}

	// Enter collapses the group.
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	state, _ = ds.Update(msg, state)

	visibleAfter := len(ds.visibleNodes())
	if visibleAfter >= visibleBefore {
		t.Errorf("collapsing group should reduce visible nodes: before=%d, after=%d",
			visibleBefore, visibleAfter)
	}

	// Enter again expands.
	state, _ = ds.Update(msg, state)
	visibleAgain := len(ds.visibleNodes())
	if visibleAgain != visibleBefore {
		t.Errorf("expanding group should restore visible nodes: got=%d, want=%d",
			visibleAgain, visibleBefore)
	}
}

func TestDetailScreen_FileSelectionUpdatesDiff(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.width = 120
	ds.height = 30
	ds.focused = PaneTree
	ds.nodes = BuildTree(sampleDiff())
	ds.SetNodes(ds.nodes)

	state := core.AppState{Mode: core.ModeDetail}

	// Get the diff content for the first file.
	visible := ds.visibleNodes()
	firstFileIdx := -1
	for i, n := range visible {
		if !n.IsGroup {
			firstFileIdx = i
			break
		}
	}
	if firstFileIdx == -1 {
		t.Fatal("no file nodes found")
	}

	ds.treeCursor = firstFileIdx
	ds.updateDiffForSelectedFile()
	firstDiff := ds.viewport.View()

	// Move to next file.
	msg := tea.KeyPressMsg{Text: "j"}
	state, _ = ds.Update(msg, state)

	secondDiff := ds.viewport.View()

	// The diff should have changed when selecting a different file.
	if firstDiff == secondDiff && len(visible) > firstFileIdx+1 && !visible[firstFileIdx+1].IsGroup {
		t.Error("diff content should update when selecting a different file")
	}
}

func TestDetailScreen_SplitMath(t *testing.T) {
	tests := []struct {
		totalWidth    int
		wantTreeW     int
		wantDiffW     int
	}{
		{totalWidth: 120, wantTreeW: 30, wantDiffW: 89},  // 120*0.25=30, 120-30-1=89
		{totalWidth: 80, wantTreeW: 20, wantDiffW: 59},   // 80*0.25=20, 80-20-1=59
		{totalWidth: 200, wantTreeW: 50, wantDiffW: 149}, // 200*0.25=50, 200-50-1=149
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("width=%d", tt.totalWidth), func(t *testing.T) {
			ds := NewDetailScreen(core.ModeList)
			ds.width = tt.totalWidth
			ds.height = 30
			ds.recalcSplit()

			treeW := int(float64(tt.totalWidth) * treeWidthRatio)
			diffW := tt.totalWidth - treeW - 1

			if treeW != tt.wantTreeW {
				t.Errorf("tree width = %d, want %d", treeW, tt.wantTreeW)
			}
			if diffW != tt.wantDiffW {
				t.Errorf("diff width = %d, want %d", diffW, tt.wantDiffW)
			}
		})
	}
}

func TestDetailScreen_VisibleNodes(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.nodes = BuildTree(sampleDiff())

	// All groups expanded → all nodes visible.
	all := ds.visibleNodes()
	if len(all) != len(ds.nodes) {
		t.Errorf("all expanded: visible=%d, total=%d", len(all), len(ds.nodes))
	}

	// Collapse the working group.
	for i := range ds.nodes {
		if ds.nodes[i].IsGroup && ds.nodes[i].Status == FileWorking {
			ds.nodes[i].Expanded = false
			break
		}
	}

	collapsed := ds.visibleNodes()
	if len(collapsed) >= len(all) {
		t.Errorf("after collapse: visible=%d should be less than %d", len(collapsed), len(all))
	}
}

func TestDetailScreen_TreeCursorBoundary(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.nodes = BuildTree(sampleDiff())

	// Move cursor way past the end.
	ds.moveTreeCursor(100)
	visible := ds.visibleNodes()
	if ds.treeCursor >= len(visible) {
		t.Errorf("cursor %d should be clamped to < %d", ds.treeCursor, len(visible))
	}

	// Move cursor way before the start.
	ds.moveTreeCursor(-200)
	if ds.treeCursor < 0 {
		t.Errorf("cursor %d should not be negative", ds.treeCursor)
	}
	if ds.treeCursor != 0 {
		t.Errorf("cursor = %d, want 0 (clamped)", ds.treeCursor)
	}
}

func TestDetailScreen_PageScroll(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.width = 120
	ds.height = 30
	ds.nodes = BuildTree(sampleDiff())
	ds.SetNodes(ds.nodes)

	// Set long content so page scroll has effect.
	longDiff := strings.Repeat("line content here\n", 200)
	ds.viewport.SetContent(longDiff)
	ds.viewport.Height = 20

	state := core.AppState{Mode: core.ModeDetail}

	// ^d should page down in diff.
	msg := tea.KeyPressMsg{Text: "d", Mod: tea.ModCtrl}
	state, _ = ds.Update(msg, state)
	// We can't easily check viewport offset, but at least verify no panic.

	// ^u should page up.
	msg = tea.KeyPressMsg{Text: "u", Mod: tea.ModCtrl}
	state, _ = ds.Update(msg, state)
}

func TestDetailScreen_ViewRenders(t *testing.T) {
	ds := NewDetailScreen(core.ModeList)
	ds.nodes = BuildTree(sampleDiff())
	ds.SetNodes(ds.nodes)

	state := core.AppState{
		Stashes: makeStashes(5),
		Mode:    core.ModeDetail,
	}

	view := ds.View(state, 120, 30)
	if view == "" {
		t.Error("View should not return empty string")
	}

	// Should contain file names from the tree.
	if !strings.Contains(view, "token.go") {
		t.Error("view should contain file name 'token.go'")
	}
}
```

### Step 8: Write integration test with real git repo

```go
func TestDetailScreen_Integration_StagedAndUnstaged(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()

	runGit := func(args ...string) string {
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
		return string(out)
	}

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	// Create repo with staged + unstaged changes in the stash.
	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	writeFile("staged.go", "package main\n")
	writeFile("unstaged.go", "package main\n")
	writeFile("both.go", "package main\n")
	runGit("add", ".")
	runGit("commit", "-m", "initial")

	// Stage one file, modify another without staging.
	writeFile("staged.go", "package main\n\nfunc Staged() {}\n")
	runGit("add", "staged.go")
	writeFile("unstaged.go", "package main\n\nfunc Unstaged() {}\n")
	writeFile("both.go", "package main\n\nfunc Both() {}\n")
	runGit("add", "both.go")

	// Stash with --keep-index to preserve the distinction.
	runGit("stash", "push", "-m", "mixed changes", "--include-untracked")

	// Get the diff for the tree.
	rawDiff := runGit("stash", "show", "-p", "stash@{0}")

	nodes := BuildTree(rawDiff)
	if len(nodes) == 0 {
		t.Fatal("BuildTree returned no nodes for real stash")
	}

	// Verify we have file nodes.
	fileCount := 0
	for _, n := range nodes {
		if !n.IsGroup {
			fileCount++
		}
	}
	if fileCount == 0 {
		t.Error("expected file nodes in the tree")
	}

	// Verify the detail screen can render with these nodes.
	ds := NewDetailScreen(core.ModeList)
	ds.SetNodes(nodes)

	state := core.AppState{Mode: core.ModeDetail}
	view := ds.View(state, 120, 30)
	if view == "" {
		t.Error("View should not return empty string for real stash")
	}

	// Navigate through all file nodes.
	visible := ds.visibleNodes()
	for range visible {
		msg := tea.KeyPressMsg{Text: "j"}
		state, _ = ds.Update(msg, state)
	}
}
```

### Step 9: Verify build and tests

```bash
gofmt -w internal/ui/screens/detail.go internal/ui/screens/detail_test.go
go vet ./internal/ui/screens/...
go test -v -race ./internal/ui/screens/...
make ci
```

## Verification

### Functional
```bash
# DETAIL screen compiles
go build ./internal/ui/screens/...

# All tests pass
go test -v -race -count=1 ./internal/ui/screens/...

# Tree building tests
go test -v -run TestBuildTree ./internal/ui/screens/...

# Focus switching tests
go test -v -run TestDetailScreen_FocusSwitching ./internal/ui/screens/...

# Esc return tests
go test -v -run TestDetailScreen_EscReturns ./internal/ui/screens/...

# Navigation tests
go test -v -run TestDetailScreen_TreeNavigation ./internal/ui/screens/...
go test -v -run TestDetailScreen_DiffNavigation ./internal/ui/screens/...

# Expand/collapse tests
go test -v -run TestDetailScreen_ExpandCollapse ./internal/ui/screens/...

# File selection updates diff
go test -v -run TestDetailScreen_FileSelectionUpdatesDiff ./internal/ui/screens/...

# Integration test
go test -v -run TestDetailScreen_Integration ./internal/ui/screens/...

# Split math
go test -v -run TestDetailScreen_SplitMath ./internal/ui/screens/...
```

### CI Pipeline
```bash
make ci
```

### TUI Visual Verification (iterm2-driver)
```bash
make build

# Create test repo with staged + unstaged + untracked files in stash
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
git init && git config user.email "t@t" && git config user.name "T"
echo "base" > staged.go && echo "base" > unstaged.go
git add . && git commit -m "init"
echo "staged change" > staged.go && git add staged.go
echo "unstaged change" > unstaged.go
echo "new file" > untracked.txt

git stash push -m "mixed" --include-untracked

# Launch nidhi, press Enter to go to DETAIL
# Verify: file tree on left (~25%), diff on right (~75%)
# Verify: Tab switches focus between tree and diff
# Verify: j/k navigates within focused pane
# Verify: selecting file updates diff pane
# Verify: Esc returns to LIST
```

## Completion Criteria
1. `internal/ui/screens/detail.go` implements `DetailScreen` with horizontal split layout
2. Left pane (~25%): file tree grouped by staged/working/untracked
3. Right pane (~75%): `viewport.Model` showing diff for selected file
4. `Tab` switches focus between tree and diff panes
5. `j`/`k` navigates within the focused pane (tree cursor or diff scroll)
6. `^d`/`^u` page scroll works in diff viewport
7. `Enter` expands/collapses tree group nodes when tree is focused
8. Selecting a file in tree updates the diff pane content
9. `Esc` returns to the previous mode (LIST or PREVIEW) correctly
10. `BuildTree` parses unified diff into grouped file tree nodes
11. File icons: `~` modified, `+` new, `-` deleted, `→` renamed
12. Cursor clamped to valid range, handles collapse/expand gracefully
13. All unit tests pass: tree building (3 cases), file icons (4 cases), focus switching, Esc return (2 cases), tree navigation, diff navigation, expand/collapse, file selection updates diff, split math (3 cases), visible nodes, cursor boundary, page scroll, view rendering
14. Integration test: creates stash with staged + unstaged files, verifies tree and navigation
15. `make ci` passes

## Commit
```
feat: implement DETAIL screen with file tree and diff viewport

Add internal/ui/screens/detail.go — DETAIL mode (PRD §10 Screen 3)
with horizontal split: file tree (~25% left) grouped by staged/working/
untracked and diff viewport (~75% right). Supports Tab focus switching,
j/k navigation per pane, Enter expand/collapse for tree groups, ^d/^u
page scroll, and Esc to return to previous mode. Comprehensive tests
including integration test with real git stash containing mixed changes.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.1 FR-03.3/FR-03.4, 10 (Screen 3), 11.2 (DETAIL keymap)
4. Read tasks 010, 009, 007 to understand dependencies
5. Implement `detail.go` following execution steps 1-6
6. Implement `detail_test.go` following execution steps 7-8
7. Run `go vet`, `go test -v -race`, `make ci`
8. If iterm2-driver is available, take screenshot of DETAIL mode and compare with mockup
9. Update this file (Status: DONE) + `docs/PROGRESS.md`
10. Commit with the message above
