package components

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/ui/theme"
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
		return "\uf06d"
	case FileAdded:
		return "\uf06b"
	case FileRemoved:
		return "\uf06c"
	case FileRenamed:
		return "\uf06e"
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
	Staged    []FileEntry
	Working   []FileEntry
	Untracked []FileEntry

	collapsed map[FileCategory]bool
	cursor    int

	theme   theme.Theme
	width   int
	useNerd bool
}

// NewFileTreeModel creates a new file tree model with the given theme.
func NewFileTreeModel(th theme.Theme, width int, useNerd bool) FileTreeModel {
	return FileTreeModel{
		collapsed: make(map[FileCategory]bool),
		theme:     th,
		width:     width,
		useNerd:   useNerd,
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

// ToggleCollapse toggles the collapsed state of the category at the cursor.
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
}

// visibleItems returns the flattened list of visible items.
func (m *FileTreeModel) visibleItems() []treeItem {
	var items []treeItem

	addCategory := func(cat FileCategory, files []FileEntry) {
		items = append(items, treeItem{isCategory: true, category: cat})
		if !m.collapsed[cat] {
			for i := range files {
				items = append(items, treeItem{file: &files[i]})
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

// categoryColor returns the theme color for a file category (per mockup).
func (m *FileTreeModel) categoryColor(cat FileCategory) color.Color {
	switch cat {
	case CategoryStaged:
		return m.theme.SemanticGreen()
	case CategoryWorking:
		return m.theme.SemanticYellow()
	case CategoryUntracked:
		return m.theme.SemanticCoral()
	default:
		return m.theme.FgSecondary()
	}
}

// iconColor returns the color for a file status icon.
func (m *FileTreeModel) iconColor(f FileEntry) color.Color {
	switch f.Status {
	case FileAdded:
		return m.theme.SemanticGreen()
	case FileRemoved:
		return m.theme.SemanticRed()
	case FileModified:
		return m.theme.SemanticBlue()
	case FileRenamed:
		return m.theme.SemanticBlue()
	default:
		return m.theme.FgSecondary()
	}
}

// View renders the file tree.
func (m *FileTreeModel) View() string {
	visible := m.visibleItems()
	if len(visible) == 0 {
		return lipgloss.NewStyle().
			Foreground(m.theme.FgDimmed()).
			Render("No files")
	}

	bg := m.theme.BgDeep()
	var lines []string

	for i, item := range visible {
		selected := i == m.cursor
		var rowBg color.Color
		if selected {
			rowBg = m.theme.BgElevated()
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
func (m *FileTreeModel) renderCategory(cat FileCategory, bg color.Color, selected bool) string {
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

	fg := m.categoryColor(cat)
	if selected {
		fg = m.theme.AccentGold()
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
func (m *FileTreeModel) renderFile(f FileEntry, bg color.Color, selected bool) string {
	var icon string
	if m.useNerd {
		icon = f.Status.NerdIcon()
	} else {
		icon = f.Status.String()
	}

	iconStyle := lipgloss.NewStyle().
		Foreground(m.iconColor(f)).
		Background(bg)

	nameFg := m.theme.FgPrimary()
	if selected {
		nameFg = m.theme.AccentGold()
	}

	nameStyle := lipgloss.NewStyle().
		Foreground(nameFg).
		Background(bg)

	// Truncate path for narrow panes.
	displayPath := f.Path
	if m.width < 40 {
		parts := strings.Split(f.Path, "/")
		displayPath = parts[len(parts)-1]
	}

	rowStyle := lipgloss.NewStyle().
		Background(bg).
		Width(m.width).
		MaxWidth(m.width)

	return rowStyle.Render("   " + iconStyle.Render(icon) + " " + nameStyle.Render(displayPath))
}
