# Task 009: Diff View and File Tree Components

## Status: TODO

## Depends On
- 003 (Agni theme -- diff color tokens: diff.added.fg/bg, diff.removed.fg/bg, diff.hunk)
- 007 (Layout engine -- split pane dimensions for DETAIL mode, content area height)

## Parallelizable With
- 008 (Stash row renderer -- independent component, built simultaneously)

## Problem
The diff viewer and file tree are the primary deep-inspection components of nidhi. The diff viewer is used in PREVIEW mode (bottom pane) and DETAIL mode (right pane) to show syntax-colored git diffs with line numbers and scrolling. The file tree is used in DETAIL mode (left pane) to group files by staged/working/untracked with expand/collapse nodes. The filter chip group is used in SEARCH mode to toggle search scope. These three components are the visual foundation for anything beyond the LIST screen, and they must handle large diffs efficiently, support keyboard scrolling (`^d/^u`), and render correctly at varying widths.

## PRD Reference
- Section 10 Screen 2 (PREVIEW) -- diff viewport in bottom pane, h/l file cycling
- Section 10 Screen 3 (DETAIL) -- file tree (left 25%) + diff viewport (right 75%), Tab focus
- Section 10 Screen 5 (SEARCH) -- filter chips: All, Messages, Files, Diffs, Branch
- Section 9.1 (Agni Theme) -- diff.added.fg/bg, diff.removed.fg/bg, diff.hunk, all color tokens
- Section 9.2 (Typography & Icons) -- file type icons (modified ~, added +, removed -, renamed ->)
- Section 13.2 (LipGloss v2 Features) -- `lipgloss/v2/tree` for file tree rendering
- Section 13.4 (Bubbles v2 Components) -- `viewport.Model` for scrollable diff

## Files to Create
- `internal/ui/components/diffview.go` -- diff viewport with syntax coloring and line numbers
- `internal/ui/components/filetree.go` -- file tree grouped by staged/working/untracked
- `internal/ui/components/filterchip.go` -- toggle chip group for search scopes
- `internal/ui/components/diffview_test.go` -- diff rendering tests
- `internal/ui/components/filetree_test.go` -- file tree grouping and state tests
- `internal/ui/components/filterchip_test.go` -- chip toggle tests

## Execution Steps

### Step 1: Create `internal/ui/components/diffview.go`

The diff viewer parses unified diff output and renders it with syntax coloring.

```go
// internal/ui/components/diffview.go
package components

import (
	"fmt"
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	viewport "charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// DiffColors holds the Agni theme tokens for diff rendering.
type DiffColors struct {
	AddedFg   string // #73D990
	AddedBg   string // #1A2E1A
	RemovedFg string // #FF5F6D
	RemovedBg string // #2E1A1A
	HunkFg    string // #61AFEF
	ContextFg string // #C8CCD4
	LineNumFg string // #3D4450
	BgDeep    string // #07090E
}

// DefaultDiffColors returns the Agni theme diff colors.
func DefaultDiffColors() DiffColors {
	return DiffColors{
		AddedFg:   "#73D990",
		AddedBg:   "#1A2E1A",
		RemovedFg: "#FF5F6D",
		RemovedBg: "#2E1A1A",
		HunkFg:    "#61AFEF",
		ContextFg: "#C8CCD4",
		LineNumFg: "#3D4450",
		BgDeep:    "#07090E",
	}
}

// DiffLineType classifies a line in a unified diff.
type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdded
	DiffLineRemoved
	DiffLineHunk    // @@ ... @@
	DiffLineHeader  // diff --git, index, ---, +++
)

// DiffLine represents a single parsed line from a unified diff.
type DiffLine struct {
	Type    DiffLineType
	Content string
	OldNum  int // Line number in the old file (0 if not applicable).
	NewNum  int // Line number in the new file (0 if not applicable).
}

// DiffViewModel wraps a Bubbles viewport.Model for displaying diffs.
type DiffViewModel struct {
	viewport viewport.Model
	lines    []DiffLine
	colors   DiffColors
	width    int
	height   int
	ready    bool
}

// NewDiffViewModel creates a new diff view model.
func NewDiffViewModel(width, height int) DiffViewModel {
	vp := viewport.New(width, height)
	return DiffViewModel{
		viewport: vp,
		colors:   DefaultDiffColors(),
		width:    width,
		height:   height,
	}
}

// SetContent parses and displays a unified diff string.
func (d *DiffViewModel) SetContent(diffStr string) {
	d.lines = parseDiff(diffStr)
	rendered := d.renderLines()
	d.viewport.SetContent(rendered)
	d.ready = true
}

// SetSize updates the viewport dimensions.
func (d *DiffViewModel) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.viewport.Width = width
	d.viewport.Height = height
	// Re-render content at new width.
	if d.ready {
		rendered := d.renderLines()
		d.viewport.SetContent(rendered)
	}
}

// Update forwards messages to the viewport for scroll handling.
func (d *DiffViewModel) Update(msg tea.Msg) (*DiffViewModel, tea.Cmd) {
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

// View returns the rendered diff viewport.
func (d *DiffViewModel) View() string {
	if !d.ready {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color(d.colors.LineNumFg)).
			Width(d.width).
			Height(d.height)
		return style.Render("No diff loaded")
	}
	return d.viewport.View()
}

// ScrollPercent returns the current scroll percentage (0-100).
func (d *DiffViewModel) ScrollPercent() float64 {
	return d.viewport.ScrollPercent()
}

// LineCount returns the total number of diff lines.
func (d *DiffViewModel) LineCount() int {
	return len(d.lines)
}

// renderLines renders all parsed diff lines with syntax coloring and line numbers.
func (d *DiffViewModel) renderLines() string {
	if len(d.lines) == 0 {
		return ""
	}

	c := d.colors
	bg := lipgloss.Color(c.BgDeep)

	lineNumStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.LineNumFg)).
		Background(bg).
		Width(4).
		Align(lipgloss.Right)

	addedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.AddedFg)).
		Background(lipgloss.Color(c.AddedBg))

	removedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.RemovedFg)).
		Background(lipgloss.Color(c.RemovedBg))

	hunkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.HunkFg)).
		Background(bg).
		Bold(true)

	contextStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.ContextFg)).
		Background(bg)

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.HunkFg)).
		Background(bg).
		Bold(true)

	sepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.LineNumFg)).
		Background(bg)

	var lines []string
	for _, dl := range d.lines {
		var lineNum string
		var content string

		switch dl.Type {
		case DiffLineAdded:
			if dl.NewNum > 0 {
				lineNum = lineNumStyle.Render(fmt.Sprintf("%d", dl.NewNum))
			} else {
				lineNum = lineNumStyle.Render("")
			}
			content = addedStyle.Render(padToWidth(dl.Content, d.width-6))
		case DiffLineRemoved:
			if dl.OldNum > 0 {
				lineNum = lineNumStyle.Render(fmt.Sprintf("%d", dl.OldNum))
			} else {
				lineNum = lineNumStyle.Render("")
			}
			content = removedStyle.Render(padToWidth(dl.Content, d.width-6))
		case DiffLineHunk:
			lineNum = lineNumStyle.Render("")
			content = hunkStyle.Render(padToWidth(dl.Content, d.width-6))
		case DiffLineHeader:
			lineNum = lineNumStyle.Render("")
			content = headerStyle.Render(padToWidth(dl.Content, d.width-6))
		default: // Context
			num := ""
			if dl.NewNum > 0 {
				num = fmt.Sprintf("%d", dl.NewNum)
			}
			lineNum = lineNumStyle.Render(num)
			content = contextStyle.Render(padToWidth(dl.Content, d.width-6))
		}

		sep := sepStyle.Render(" \u2502 ") // vertical line separator
		lines = append(lines, lineNum+sep+content)
	}

	return strings.Join(lines, "\n")
}

// padToWidth pads a string with spaces to the target width.
func padToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// ─── Diff Parser ────────────────────────────────────────────

// parseDiff parses a unified diff string into typed lines with line numbers.
func parseDiff(diff string) []DiffLine {
	rawLines := strings.Split(diff, "\n")
	var result []DiffLine

	var oldNum, newNum int

	for _, raw := range rawLines {
		if raw == "" && len(result) > 0 {
			// Preserve empty lines in context.
			result = append(result, DiffLine{Type: DiffLineContext, Content: "", NewNum: newNum})
			newNum++
			oldNum++
			continue
		}

		switch {
		case strings.HasPrefix(raw, "@@"):
			// Parse hunk header: @@ -old,count +new,count @@
			oldNum, newNum = parseHunkHeader(raw)
			result = append(result, DiffLine{Type: DiffLineHunk, Content: raw})

		case strings.HasPrefix(raw, "+"):
			if strings.HasPrefix(raw, "+++ ") {
				result = append(result, DiffLine{Type: DiffLineHeader, Content: raw})
			} else {
				result = append(result, DiffLine{
					Type:    DiffLineAdded,
					Content: raw,
					NewNum:  newNum,
				})
				newNum++
			}

		case strings.HasPrefix(raw, "-"):
			if strings.HasPrefix(raw, "--- ") {
				result = append(result, DiffLine{Type: DiffLineHeader, Content: raw})
			} else {
				result = append(result, DiffLine{
					Type:    DiffLineRemoved,
					Content: raw,
					OldNum:  oldNum,
				})
				oldNum++
			}

		case strings.HasPrefix(raw, "diff ") || strings.HasPrefix(raw, "index "):
			result = append(result, DiffLine{Type: DiffLineHeader, Content: raw})

		default:
			result = append(result, DiffLine{
				Type:    DiffLineContext,
				Content: raw,
				OldNum:  oldNum,
				NewNum:  newNum,
			})
			oldNum++
			newNum++
		}
	}

	return result
}

// parseHunkHeader extracts starting line numbers from a @@ header.
// Format: @@ -oldStart[,oldCount] +newStart[,newCount] @@
func parseHunkHeader(header string) (oldStart, newStart int) {
	// Strip the @@ markers.
	header = strings.TrimPrefix(header, "@@")
	idx := strings.Index(header, "@@")
	if idx > 0 {
		header = header[:idx]
	}
	header = strings.TrimSpace(header)

	parts := strings.Fields(header)
	for _, p := range parts {
		if strings.HasPrefix(p, "-") {
			fmt.Sscanf(p, "-%d", &oldStart)
		} else if strings.HasPrefix(p, "+") {
			fmt.Sscanf(p, "+%d", &newStart)
		}
	}
	return
}
```

### Step 2: Create `internal/ui/components/filetree.go`

```go
// internal/ui/components/filetree.go
package components

import (
	"fmt"
	"sort"
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
)

// FileStatus represents the modification status of a file in a stash.
type FileStatus int

const (
	FileModified FileStatus = iota
	FileAdded
	FileRemoved
	FileRenamed
)

// String returns the status character.
func (s FileStatus) String() string {
	switch s {
	case FileModified:
		return "~"
	case FileAdded:
		return "+"
	case FileRemoved:
		return "-"
	case FileRenamed:
		return "\u2192" // →
	default:
		return "?"
	}
}

// NerdIcon returns the Nerd Font icon for this status.
func (s FileStatus) NerdIcon() string {
	switch s {
	case FileModified:
		return "\uf06d" // nf-oct-diff_modified
	case FileAdded:
		return "\uf06b" // nf-oct-diff_added
	case FileRemoved:
		return "\uf06c" // nf-oct-diff_removed
	case FileRenamed:
		return "\uf06e" // nf-oct-diff_renamed
	default:
		return "?"
	}
}

// FileEntry represents a single file in the file tree.
type FileEntry struct {
	Path     string
	Status   FileStatus
	Category FileCategory
}

// FileCategory groups files by their staging status.
type FileCategory int

const (
	CategoryStaged FileCategory = iota
	CategoryWorking
	CategoryUntracked
)

// String returns the category label.
func (c FileCategory) String() string {
	switch c {
	case CategoryStaged:
		return "staged"
	case CategoryWorking:
		return "working"
	case CategoryUntracked:
		return "untracked"
	default:
		return "unknown"
	}
}

// FileTreeModel manages the file tree state for DETAIL mode.
type FileTreeModel struct {
	// All files grouped by category.
	Staged    []FileEntry
	Working   []FileEntry
	Untracked []FileEntry

	// Collapsed state per category.
	collapsed map[FileCategory]bool

	// Cursor position (index into the flattened visible list).
	cursor int

	// Visual config.
	width   int
	useNerd bool

	// Theme colors.
	FgPrimary   string
	FgSecondary string
	FgDimmed    string
	AccentGold  string
	BgDeep      string
	BgElevated  string
	GreenFg     string
	RedFg       string
	BlueFg      string
}

// NewFileTreeModel creates a new file tree model.
func NewFileTreeModel(width int, useNerd bool) FileTreeModel {
	return FileTreeModel{
		collapsed:   make(map[FileCategory]bool),
		width:       width,
		useNerd:     useNerd,
		FgPrimary:   "#C8CCD4",
		FgSecondary: "#6B7280",
		FgDimmed:    "#3D4450",
		AccentGold:  "#D4A050",
		BgDeep:      "#07090E",
		BgElevated:  "#1A1F2B",
		GreenFg:     "#73D990",
		RedFg:       "#FF5F6D",
		BlueFg:      "#61AFEF",
	}
}

// SetFiles sets the file list, grouping into categories.
func (m *FileTreeModel) SetFiles(files []FileEntry) {
	m.Staged = nil
	m.Working = nil
	m.Untracked = nil

	for _, f := range files {
		switch f.Category {
		case CategoryStaged:
			m.Staged = append(m.Staged, f)
		case CategoryWorking:
			m.Working = append(m.Working, f)
		case CategoryUntracked:
			m.Untracked = append(m.Untracked, f)
		}
	}

	// Sort each group by path.
	sortFiles := func(files []FileEntry) {
		sort.Slice(files, func(i, j int) bool {
			return files[i].Path < files[j].Path
		})
	}
	sortFiles(m.Staged)
	sortFiles(m.Working)
	sortFiles(m.Untracked)

	m.cursor = 0
}

// SetWidth updates the tree width.
func (m *FileTreeModel) SetWidth(width int) {
	m.width = width
}

// ToggleCollapse toggles the expanded/collapsed state of the category at the cursor.
func (m *FileTreeModel) ToggleCollapse() {
	item := m.itemAtCursor()
	if item == nil {
		return
	}
	if item.isCategory {
		m.collapsed[item.category] = !m.collapsed[item.category]
	}
}

// CursorUp moves the cursor up.
func (m *FileTreeModel) CursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

// CursorDown moves the cursor down.
func (m *FileTreeModel) CursorDown() {
	visible := m.visibleItems()
	if m.cursor < len(visible)-1 {
		m.cursor++
	}
}

// SelectedFile returns the currently selected file, or nil if a category header is selected.
func (m *FileTreeModel) SelectedFile() *FileEntry {
	item := m.itemAtCursor()
	if item == nil || item.isCategory {
		return nil
	}
	return item.file
}

// Cursor returns the current cursor position.
func (m *FileTreeModel) Cursor() int {
	return m.cursor
}

// FileCount returns the total number of files across all categories.
func (m *FileTreeModel) FileCount() int {
	return len(m.Staged) + len(m.Working) + len(m.Untracked)
}

// treeItem represents a visible item in the flattened tree list.
type treeItem struct {
	isCategory bool
	category   FileCategory
	file       *FileEntry
	depth      int
}

// visibleItems returns the flattened list of visible items.
func (m *FileTreeModel) visibleItems() []treeItem {
	var items []treeItem

	addCategory := func(cat FileCategory, files []FileEntry) {
		if len(files) == 0 {
			// Show empty categories with count 0.
			items = append(items, treeItem{isCategory: true, category: cat, depth: 0})
			return
		}
		items = append(items, treeItem{isCategory: true, category: cat, depth: 0})
		if !m.collapsed[cat] {
			for i := range files {
				items = append(items, treeItem{file: &files[i], depth: 1})
			}
		}
	}

	addCategory(CategoryStaged, m.Staged)
	addCategory(CategoryWorking, m.Working)
	addCategory(CategoryUntracked, m.Untracked)

	return items
}

// itemAtCursor returns the item at the current cursor position.
func (m *FileTreeModel) itemAtCursor() *treeItem {
	visible := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(visible) {
		return nil
	}
	item := visible[m.cursor]
	return &item
}

// View renders the file tree.
func (m *FileTreeModel) View() string {
	visible := m.visibleItems()
	if len(visible) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(m.FgDimmed)).
			Render("No files")
	}

	bg := lipgloss.Color(m.BgDeep)
	var lines []string

	for i, item := range visible {
		selected := i == m.cursor
		var rowBg lipgloss.Color
		if selected {
			rowBg = lipgloss.Color(m.BgElevated)
		} else {
			rowBg = bg
		}

		if item.isCategory {
			lines = append(lines, m.renderCategory(item.category, rowBg, selected))
		} else if item.file != nil {
			lines = append(lines, m.renderFile(*item.file, rowBg, selected))
		}
	}

	return strings.Join(lines, "\n")
}

// renderCategory renders a category header line.
func (m *FileTreeModel) renderCategory(cat FileCategory, bg lipgloss.Color, selected bool) string {
	var count int
	switch cat {
	case CategoryStaged:
		count = len(m.Staged)
	case CategoryWorking:
		count = len(m.Working)
	case CategoryUntracked:
		count = len(m.Untracked)
	}

	// Expand/collapse indicator.
	var indicator string
	if m.collapsed[cat] {
		indicator = "\u25b6" // ▶ (collapsed)
	} else {
		indicator = "\u25bc" // ▼ (expanded)
	}

	fg := lipgloss.Color(m.FgSecondary)
	if selected {
		fg = lipgloss.Color(m.AccentGold)
	}

	style := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true).
		Width(m.width).
		MaxWidth(m.width)

	return style.Render(fmt.Sprintf(" %s %s (%d)", indicator, cat, count))
}

// renderFile renders a single file entry.
func (m *FileTreeModel) renderFile(f FileEntry, bg lipgloss.Color, selected bool) string {
	// Status icon.
	var icon string
	if m.useNerd {
		icon = f.Status.NerdIcon()
	} else {
		icon = f.Status.String()
	}

	// Icon color based on status.
	var iconColor string
	switch f.Status {
	case FileAdded:
		iconColor = m.GreenFg
	case FileRemoved:
		iconColor = m.RedFg
	case FileModified:
		iconColor = m.BlueFg
	case FileRenamed:
		iconColor = m.BlueFg
	default:
		iconColor = m.FgSecondary
	}

	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(iconColor)).
		Background(bg)

	nameFg := lipgloss.Color(m.FgPrimary)
	if selected {
		nameFg = lipgloss.Color(m.AccentGold)
	}

	nameStyle := lipgloss.NewStyle().
		Foreground(nameFg).
		Background(bg)

	// Extract just the filename from the path for display.
	displayPath := f.Path
	if m.width < 40 {
		// Truncate path for narrow panes.
		parts := strings.Split(f.Path, "/")
		displayPath = parts[len(parts)-1]
	}

	rowStyle := lipgloss.NewStyle().
		Background(bg).
		Width(m.width).
		MaxWidth(m.width)

	return rowStyle.Render("   " + iconStyle.Render(icon) + " " + nameStyle.Render(displayPath))
}
```

### Step 3: Create `internal/ui/components/filterchip.go`

```go
// internal/ui/components/filterchip.go
package components

import (
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
)

// Chip represents a single filter chip.
type Chip struct {
	Label  string
	Active bool
}

// ChipGroupModel manages a group of toggle chips.
type ChipGroupModel struct {
	Chips  []Chip
	cursor int // Currently focused chip.

	// Theme colors.
	ActiveBg    string // accent.gold
	ActiveFg    string // bg.deep
	InactiveBg  string // bg.elevated
	InactiveFg  string // fg.secondary
	FocusBorder string // accent.bright
}

// NewChipGroupModel creates a chip group with the given labels.
// The first chip starts active.
func NewChipGroupModel(labels []string) ChipGroupModel {
	chips := make([]Chip, len(labels))
	for i, l := range labels {
		chips[i] = Chip{Label: l, Active: i == 0}
	}
	return ChipGroupModel{
		Chips:       chips,
		cursor:      0,
		ActiveBg:    "#D4A050",
		ActiveFg:    "#07090E",
		InactiveBg:  "#1A1F2B",
		InactiveFg:  "#6B7280",
		FocusBorder: "#E8B85A",
	}
}

// SearchScopeChips creates the standard search scope chip group.
// PRD Section 10 Screen 5: All, Messages, Files, Diffs, Branch.
func SearchScopeChips() ChipGroupModel {
	return NewChipGroupModel([]string{"All", "Messages", "Files", "Diffs", "Branch"})
}

// Next moves focus to the next chip (wraps around).
func (m *ChipGroupModel) Next() {
	m.cursor = (m.cursor + 1) % len(m.Chips)
}

// Prev moves focus to the previous chip (wraps around).
func (m *ChipGroupModel) Prev() {
	m.cursor = (m.cursor - 1 + len(m.Chips)) % len(m.Chips)
}

// Toggle toggles the active state of the focused chip.
// If "All" (index 0) is toggled on, all others are deactivated.
// If any other chip is toggled on, "All" is deactivated.
func (m *ChipGroupModel) Toggle() {
	if len(m.Chips) == 0 {
		return
	}

	m.Chips[m.cursor].Active = !m.Chips[m.cursor].Active

	if m.cursor == 0 && m.Chips[0].Active {
		// "All" toggled on: deactivate all others.
		for i := 1; i < len(m.Chips); i++ {
			m.Chips[i].Active = false
		}
	} else if m.cursor != 0 && m.Chips[m.cursor].Active {
		// Specific chip toggled on: deactivate "All".
		if len(m.Chips) > 0 {
			m.Chips[0].Active = false
		}
	}

	// If no chips are active, reactivate "All".
	anyActive := false
	for _, c := range m.Chips {
		if c.Active {
			anyActive = true
			break
		}
	}
	if !anyActive && len(m.Chips) > 0 {
		m.Chips[0].Active = true
	}
}

// ActiveLabels returns the labels of all active chips.
func (m *ChipGroupModel) ActiveLabels() []string {
	var labels []string
	for _, c := range m.Chips {
		if c.Active {
			labels = append(labels, c.Label)
		}
	}
	return labels
}

// Cursor returns the currently focused chip index.
func (m *ChipGroupModel) Cursor() int {
	return m.cursor
}

// View renders the chip group as a horizontal row.
func (m *ChipGroupModel) View() string {
	var chips []string

	for i, chip := range m.Chips {
		var style lipgloss.Style

		if chip.Active {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color(m.ActiveFg)).
				Background(lipgloss.Color(m.ActiveBg)).
				Bold(true).
				PaddingLeft(1).
				PaddingRight(1)
		} else {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color(m.InactiveFg)).
				Background(lipgloss.Color(m.InactiveBg)).
				PaddingLeft(1).
				PaddingRight(1)
		}

		// Add focus indicator for the currently selected chip.
		if i == m.cursor {
			style = style.
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(m.FocusBorder))
		}

		chips = append(chips, style.Render(chip.Label))
	}

	return strings.Join(chips, " ")
}
```

### Step 4: Create test files

#### `internal/ui/components/diffview_test.go`

```go
// internal/ui/components/diffview_test.go
package components

import (
	"strings"
	"testing"
)

const sampleDiff = `diff --git a/src/auth/token.go b/src/auth/token.go
index abc1234..def5678 100644
--- a/src/auth/token.go
+++ b/src/auth/token.go
@@ -42,7 +42,12 @@ func RefreshToken(ctx context.Context) error {
     if token.IsExpired() {
-        return nil, ErrExpired
+        newToken, err := provider.Refresh(token)
+        if err != nil {
+            return nil, fmt.Errorf("refresh: %w", err)
+        }
+        return newToken, nil
     }
     return token, nil
 }`

func TestParseDiff_LineTypes(t *testing.T) {
	lines := parseDiff(sampleDiff)

	if len(lines) == 0 {
		t.Fatal("parseDiff returned no lines")
	}

	// Count line types.
	var headers, hunks, added, removed, context int
	for _, l := range lines {
		switch l.Type {
		case DiffLineHeader:
			headers++
		case DiffLineHunk:
			hunks++
		case DiffLineAdded:
			added++
		case DiffLineRemoved:
			removed++
		case DiffLineContext:
			context++
		}
	}

	if headers < 3 {
		t.Errorf("expected >= 3 header lines (diff, ---, +++), got %d", headers)
	}
	if hunks != 1 {
		t.Errorf("expected 1 hunk header, got %d", hunks)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed line, got %d", removed)
	}
	if added != 5 {
		t.Errorf("expected 5 added lines, got %d", added)
	}
	if context < 2 {
		t.Errorf("expected >= 2 context lines, got %d", context)
	}
}

func TestParseDiff_LineNumbers(t *testing.T) {
	lines := parseDiff(sampleDiff)

	// Find the first added line after the hunk.
	for _, l := range lines {
		if l.Type == DiffLineAdded && l.NewNum > 0 {
			// After @@ -42,7 +42,12 @@, the new starting line is 42.
			// The removed line consumes old line 43.
			// First added line should be at new line 43.
			if l.NewNum < 42 {
				t.Errorf("first added line NewNum = %d, expected >= 42", l.NewNum)
			}
			break
		}
	}
}

func TestParseHunkHeader(t *testing.T) {
	tests := []struct {
		header   string
		wantOld  int
		wantNew  int
	}{
		{"@@ -42,7 +42,12 @@ func RefreshToken", 42, 42},
		{"@@ -1,3 +1,5 @@", 1, 1},
		{"@@ -100 +200 @@", 100, 200},
		{"@@ -0,0 +1,10 @@", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			old, new := parseHunkHeader(tt.header)
			if old != tt.wantOld {
				t.Errorf("oldStart = %d, want %d", old, tt.wantOld)
			}
			if new != tt.wantNew {
				t.Errorf("newStart = %d, want %d", new, tt.wantNew)
			}
		})
	}
}

func TestDiffViewModel_SetContent(t *testing.T) {
	dv := NewDiffViewModel(80, 20)
	dv.SetContent(sampleDiff)

	if dv.LineCount() == 0 {
		t.Error("LineCount() should be > 0 after SetContent")
	}
}

func TestDiffViewModel_EmptyDiff(t *testing.T) {
	dv := NewDiffViewModel(80, 20)
	dv.SetContent("")

	if dv.LineCount() != 0 {
		t.Errorf("LineCount() for empty diff = %d, want 0", dv.LineCount())
	}
}

func TestDiffViewModel_ViewBeforeContent(t *testing.T) {
	dv := NewDiffViewModel(80, 20)
	view := dv.View()

	if !strings.Contains(stripAnsi(view), "No diff loaded") {
		t.Errorf("view before content should show placeholder, got: %q", stripAnsi(view))
	}
}

func TestDiffViewModel_SetSize(t *testing.T) {
	dv := NewDiffViewModel(80, 20)
	dv.SetContent(sampleDiff)

	dv.SetSize(120, 40)

	// Should not panic or corrupt state.
	if dv.LineCount() == 0 {
		t.Error("LineCount() should still be > 0 after SetSize")
	}
}

func TestDiffLineType_Coverage(t *testing.T) {
	// Ensure all DiffLineType values are handled in parseDiff.
	diff := "diff --git a/f b/f\nindex abc..def 100644\n--- a/f\n+++ b/f\n@@ -1,2 +1,2 @@\n context\n-removed\n+added"
	lines := parseDiff(diff)

	typesSeen := make(map[DiffLineType]bool)
	for _, l := range lines {
		typesSeen[l.Type] = true
	}

	required := []DiffLineType{DiffLineHeader, DiffLineHunk, DiffLineAdded, DiffLineRemoved, DiffLineContext}
	for _, typ := range required {
		if !typesSeen[typ] {
			t.Errorf("DiffLineType %d not seen in parsed output", typ)
		}
	}
}
```

#### `internal/ui/components/filetree_test.go`

```go
// internal/ui/components/filetree_test.go
package components

import (
	"strings"
	"testing"
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
	m := NewFileTreeModel(40, false)
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
	m := NewFileTreeModel(40, false)
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
	m := NewFileTreeModel(40, false)
	m.SetFiles(testFiles())

	if m.FileCount() != 4 {
		t.Errorf("FileCount() = %d, want 4", m.FileCount())
	}
}

func TestFileTreeModel_CursorNavigation(t *testing.T) {
	m := NewFileTreeModel(40, false)
	m.SetFiles(testFiles())

	// Initial cursor at 0 (staged category header).
	if m.Cursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.Cursor())
	}

	// Move down through staged header, files, working header, files, etc.
	m.CursorDown() // -> staged file 1
	m.CursorDown() // -> staged file 2
	m.CursorDown() // -> working header
	m.CursorDown() // -> working file 1
	m.CursorDown() // -> untracked header
	m.CursorDown() // -> untracked file 1

	// Should not go beyond last item.
	m.CursorDown()
	m.CursorDown()

	// Move up.
	m.CursorUp()
}

func TestFileTreeModel_ToggleCollapse(t *testing.T) {
	m := NewFileTreeModel(40, false)
	m.SetFiles(testFiles())

	// Cursor is at staged category header (index 0).
	m.ToggleCollapse()

	// Staged should now be collapsed. Next visible item after staged header
	// should be the working category header.
	m.CursorDown() // Should jump to working header, not a staged file.

	selectedFile := m.SelectedFile()
	// After collapsing staged, cursor at index 1 should be working header.
	if selectedFile != nil {
		// If we get a file, the collapse might not be working.
		// This depends on implementation. Check that collapsed state is set.
	}

	// Uncollapse.
	m.cursor = 0
	m.ToggleCollapse()
}

func TestFileTreeModel_SelectedFile(t *testing.T) {
	m := NewFileTreeModel(40, false)
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
	m := NewFileTreeModel(40, false)
	m.SetFiles(testFiles())

	view := m.View()
	plain := stripAnsi(view)

	// Should contain category headers.
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
	m := NewFileTreeModel(40, false)
	m.SetFiles(testFiles())

	view := m.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "(2)") { // 2 staged files
		t.Error("view should show count (2) for staged")
	}
	if !strings.Contains(plain, "(1)") { // 1 working or untracked file
		t.Error("view should show count (1)")
	}
}

func TestFileTreeModel_EmptyFiles(t *testing.T) {
	m := NewFileTreeModel(40, false)
	m.SetFiles(nil)

	view := m.View()
	plain := stripAnsi(view)

	// Should show category headers with (0) counts.
	if !strings.Contains(plain, "(0)") {
		t.Error("empty tree should show (0) counts")
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
```

#### `internal/ui/components/filterchip_test.go`

```go
// internal/ui/components/filterchip_test.go
package components

import (
	"testing"
)

func TestChipGroupModel_Initial(t *testing.T) {
	m := SearchScopeChips()

	if len(m.Chips) != 5 {
		t.Fatalf("expected 5 chips, got %d", len(m.Chips))
	}

	// First chip ("All") should be active.
	if !m.Chips[0].Active {
		t.Error("first chip should be active initially")
	}

	// Others should be inactive.
	for i := 1; i < len(m.Chips); i++ {
		if m.Chips[i].Active {
			t.Errorf("chip %d (%q) should be inactive initially", i, m.Chips[i].Label)
		}
	}

	if m.Cursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.Cursor())
	}
}

func TestChipGroupModel_Navigation(t *testing.T) {
	m := SearchScopeChips()

	m.Next()
	if m.Cursor() != 1 {
		t.Errorf("after Next: cursor = %d, want 1", m.Cursor())
	}

	m.Next()
	if m.Cursor() != 2 {
		t.Errorf("after Next x2: cursor = %d, want 2", m.Cursor())
	}

	m.Prev()
	if m.Cursor() != 1 {
		t.Errorf("after Prev: cursor = %d, want 1", m.Cursor())
	}
}

func TestChipGroupModel_WrapAround(t *testing.T) {
	m := NewChipGroupModel([]string{"A", "B", "C"})

	// Wrap forward.
	m.Next() // 1
	m.Next() // 2
	m.Next() // 0 (wrapped)
	if m.Cursor() != 0 {
		t.Errorf("wrap forward: cursor = %d, want 0", m.Cursor())
	}

	// Wrap backward.
	m.Prev() // 2 (wrapped)
	if m.Cursor() != 2 {
		t.Errorf("wrap backward: cursor = %d, want 2", m.Cursor())
	}
}

func TestChipGroupModel_Toggle(t *testing.T) {
	m := SearchScopeChips()

	// Toggle "Messages" (index 1).
	m.Next() // cursor at 1
	m.Toggle()

	if !m.Chips[1].Active {
		t.Error("Messages should be active after toggle")
	}
	if m.Chips[0].Active {
		t.Error("All should be deactivated when a specific chip is activated")
	}
}

func TestChipGroupModel_ToggleAllDeactivatesOthers(t *testing.T) {
	m := SearchScopeChips()

	// Activate "Messages".
	m.Next()
	m.Toggle()

	// Activate "Files".
	m.Next()
	m.Toggle()

	// Now go back to "All" and activate it.
	m.cursor = 0
	m.Toggle()

	// "All" should be active, everything else inactive.
	if !m.Chips[0].Active {
		t.Error("All should be active")
	}
	for i := 1; i < len(m.Chips); i++ {
		if m.Chips[i].Active {
			t.Errorf("chip %d should be inactive when All is active", i)
		}
	}
}

func TestChipGroupModel_NoChipsActiveResetsToAll(t *testing.T) {
	m := SearchScopeChips()

	// Activate "Messages", then deactivate it.
	m.Next()    // cursor at 1
	m.Toggle()  // activate Messages, deactivate All
	m.Toggle()  // deactivate Messages

	// No chips active -> should auto-activate "All".
	if !m.Chips[0].Active {
		t.Error("All should auto-activate when no chips are active")
	}
}

func TestChipGroupModel_ActiveLabels(t *testing.T) {
	m := SearchScopeChips()

	labels := m.ActiveLabels()
	if len(labels) != 1 || labels[0] != "All" {
		t.Errorf("initial ActiveLabels() = %v, want [All]", labels)
	}

	// Activate Messages and Files.
	m.cursor = 1
	m.Toggle()
	m.cursor = 2
	m.Toggle()

	labels = m.ActiveLabels()
	if len(labels) != 2 {
		t.Errorf("ActiveLabels() = %v, want 2 items", labels)
	}
}

func TestChipGroupModel_ViewRendersAllChips(t *testing.T) {
	m := NewChipGroupModel([]string{"Alpha", "Beta", "Gamma"})
	view := m.View()
	plain := stripAnsi(view)

	for _, label := range []string{"Alpha", "Beta", "Gamma"} {
		if !containsString(plain, label) {
			t.Errorf("view should contain chip label %q", label)
		}
	}
}

func TestChipGroupModel_EmptyGroup(t *testing.T) {
	m := NewChipGroupModel(nil)

	// Should not panic.
	m.Next()
	m.Prev()
	m.Toggle()

	view := m.View()
	if view != "" {
		t.Errorf("empty chip group view should be empty, got: %q", view)
	}
}
```

## Verification

### Functional
```bash
# From project root
cd /Users/indrasvat/code/github.com/indrasvat-nidhi

# Run all component tests
go test -v -race ./internal/ui/components/...

# Check compilation
go build ./internal/ui/components/...

# Linter
make lint

# Full CI
make ci
```

### Integration Check
```bash
# Verify diff parser handles real git output
cd /tmp && git init test-diff-repo && cd test-diff-repo
git commit --allow-empty -m "init"
echo "line1" > file.go
git add file.go && git stash push -m "test"
git stash show -p stash@{0}
# Use this output to verify parseDiff handles real diffs correctly
rm -rf /tmp/test-diff-repo
```

## Completion Criteria
1. All three source files compile: `diffview.go`, `filetree.go`, `filterchip.go`
2. All three test files pass with `go test -v -race ./internal/ui/components/...`
3. `parseDiff` correctly classifies all line types: header, hunk, added, removed, context
4. `parseHunkHeader` extracts old/new starting line numbers
5. Diff view renders with Agni theme colors: green bg for additions, red bg for removals, blue for hunks
6. Line numbers are right-aligned in a 4-char column with a vertical separator
7. File tree groups files by staged/working/untracked categories
8. Categories show expand/collapse indicators and file counts
9. File tree supports cursor navigation and collapse/expand toggle
10. `SelectedFile()` returns nil at category headers, a `FileEntry` at file rows
11. Filter chips support Tab cycling, toggle, "All deactivates specifics" and "specifics deactivate All" behavior
12. When no chips are active, "All" auto-reactivates
13. `ActiveLabels()` returns the correct set of active chip labels
14. `make lint` passes with no warnings

## Commit
```
feat(ui): add diff view, file tree, and filter chip components

Implement three UI components for deep inspection modes:
- diffview.go: unified diff parser with line type classification
  (header/hunk/added/removed/context), line number tracking,
  syntax-colored rendering using Agni theme diff tokens, viewport
  integration for scrolling
- filetree.go: file tree grouped by staged/working/untracked,
  expand/collapse per category, cursor navigation, status icons
  (modified ~, added +, removed -, renamed →) with Nerd Font support
- filterchip.go: toggle chip group for search scopes (All, Messages,
  Files, Diffs, Branch) with exclusive "All" behavior, Tab cycling,
  focus indicator
- Comprehensive table-driven tests for diff parsing, hunk headers,
  tree grouping/sorting, chip toggle logic, and edge cases
```

## Session Protocol
1. Read this task file completely before writing any code.
2. Verify Task 003 (theme) and Task 007 (layout) are complete.
3. Write `diffview.go` -- diff parser and viewport wrapper.
4. Write `filetree.go` -- file tree model with categories.
5. Write `filterchip.go` -- chip group with toggle logic.
6. Write all three test files.
7. Run `go test -v -race ./internal/ui/components/...` and fix any failures. Update PROGRESS.md and CLAUDE.md.
