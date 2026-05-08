package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui/components"
	"github.com/indrasvat/nidhi/internal/ui/layout"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── Focus tracking ─────────────────────────────────────────

// FocusedPane tracks which pane has keyboard focus in DETAIL mode.
type FocusedPane int

const (
	PaneTree FocusedPane = iota
	PaneDiff
)

// ─── DetailScreen ───────────────────────────────────────────

// DetailScreen implements DETAIL mode — horizontal split: file tree (left) + diff viewport (right).
// Uses FileTreeModel for the tree pane and DiffViewModel for the diff pane.
type DetailScreen struct {
	tree     components.FileTreeModel
	diffView components.DiffViewModel

	fileDiffs map[string]string // filename → diff content
	focused   FocusedPane

	theme theme.Theme

	width  int
	height int
}

// NewDetailScreen creates a new DETAIL mode screen.
func NewDetailScreen(th theme.Theme) *DetailScreen {
	return &DetailScreen{
		tree:      components.NewFileTreeModel(th, 30, false),
		diffView:  components.NewDiffViewModel(th, 80, 20),
		fileDiffs: make(map[string]string),
		theme:     th,
		focused:   PaneTree,
	}
}

// SetDiff parses a unified diff and populates the file tree and per-file diff content.
// In Phase 1, all files are categorized as "working" tree changes.
func (d *DetailScreen) SetDiff(diff string) {
	sections := parseDiffFiles(diff)

	entries := make([]components.FileEntry, 0, len(sections))
	d.fileDiffs = make(map[string]string, len(sections))

	for _, s := range sections {
		status := inferFileStatus(s.Content)
		entries = append(entries, components.FileEntry{
			Path:     s.Filename,
			Status:   status,
			Category: components.CategoryWorking,
		})
		d.fileDiffs[s.Filename] = s.Content
	}

	d.tree.SetFiles(entries)
	d.selectFirstFile()
	d.updateDiffForSelected()
}

// Focused returns which pane currently has focus.
func (d *DetailScreen) Focused() FocusedPane {
	return d.focused
}

// ResetFocus resets keyboard focus to the tree pane and tree cursor to the first file.
// Called when entering DETAIL mode to ensure a clean start.
func (d *DetailScreen) ResetFocus() {
	d.focused = PaneTree
	d.tree.ResetCursor()
	d.selectFirstFile()
	d.updateDiffForSelected()
}

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

// View renders the DETAIL screen: file tree (left) | divider | diff viewport (right).
func (d *DetailScreen) View(state core.AppState, width, height int) string {
	d.width = width
	d.height = height
	d.recalcSplit()

	// Wire focus state into sub-components before rendering.
	d.tree.SetFocused(d.focused == PaneTree)
	d.diffView.SetFocused(d.focused == PaneDiff)

	split := layout.ComputeSplit(width, layout.DetailSplitRatio)

	// Left pane: file tree.
	treeView := d.tree.View()
	treeStyle := lipgloss.NewStyle().
		Width(split.PrimarySize).
		Height(height)
	treeRendered := treeStyle.Render(treeView)

	// If collapsed (not enough room for two panes), show tree full-width.
	if split.SecondarySize == 0 {
		return treeRendered
	}

	// Vertical divider — accent color on the focused side.
	dividerFg := d.theme.FgDimmed()
	if d.focused == PaneDiff {
		dividerFg = d.theme.SemanticBlue()
	}
	dividerStyle := lipgloss.NewStyle().
		Foreground(dividerFg)
	divider := dividerStyle.Render(strings.Repeat("\u2502\n", max(height-1, 0)) + "\u2502")

	// Right pane: diff view.
	diffView := d.diffView.View()
	diffStyle := lipgloss.NewStyle().
		Width(split.SecondarySize).
		Height(height)
	diffRendered := diffStyle.Render(diffView)

	return lipgloss.JoinHorizontal(lipgloss.Top, treeRendered, divider, diffRendered)
}

// ─── Internal ───────────────────────────────────────────────

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

	// j/down navigate within the focused pane.
	case msg.Text == "j", msg.Code == tea.KeyDown:
		if d.focused == PaneTree {
			d.tree.CursorDown()
			d.updateDiffForSelected()
		} else {
			d.diffView.ScrollDown(1)
		}
		return state, nil

	// k/up navigate within the focused pane.
	case msg.Text == "k", msg.Code == tea.KeyUp:
		if d.focused == PaneTree {
			d.tree.CursorUp()
			d.updateDiffForSelected()
		} else {
			d.diffView.ScrollUp(1)
		}
		return state, nil

	// Ctrl+D/U page scroll in diff pane.
	case msg.Text == "d" && msg.Mod.Contains(tea.ModCtrl):
		d.diffView.ScrollDown(d.diffView.LineCount() / 2)
		return state, nil
	case msg.Text == "u" && msg.Mod.Contains(tea.ModCtrl):
		d.diffView.ScrollUp(d.diffView.LineCount() / 2)
		return state, nil

	// Enter expands/collapses tree groups when tree is focused.
	case msg.Code == tea.KeyEnter:
		if d.focused == PaneTree {
			d.tree.ToggleCollapse()
		}
		return state, nil
	}

	return state, nil
}

func (d *DetailScreen) updateDiffForSelected() {
	file := d.tree.SelectedFile()
	if file == nil {
		d.diffView.SetFileName("")
		d.diffView.SetContent("Select a file to view its diff.")
		return
	}
	d.diffView.SetFileName(file.Path)
	if content, ok := d.fileDiffs[file.Path]; ok {
		d.diffView.SetContent(content)
	} else {
		d.diffView.SetContent("(no diff available)")
	}
}

// selectFirstFile advances the tree cursor past category headers to the first file node.
func (d *DetailScreen) selectFirstFile() {
	for d.tree.SelectedFile() == nil {
		before := d.tree.Cursor()
		d.tree.CursorDown()
		if d.tree.Cursor() == before {
			break // can't advance further
		}
	}
}

func (d *DetailScreen) recalcSplit() {
	split := layout.ComputeSplit(d.width, layout.DetailSplitRatio)
	d.tree.SetWidth(split.PrimarySize)
	d.diffView.SetSize(max(split.SecondarySize, 1), max(d.height, 1))
}

// inferFileStatus determines the file modification status from diff content.
func inferFileStatus(diffContent string) components.FileStatus {
	if strings.Contains(diffContent, "new file mode") {
		return components.FileAdded
	}
	if strings.Contains(diffContent, "deleted file mode") {
		return components.FileRemoved
	}
	if strings.Contains(diffContent, "rename from") {
		return components.FileRenamed
	}
	return components.FileModified
}
