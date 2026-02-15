package screens

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/components"
	"github.com/indrasvat/nidhi/internal/ui/layout"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── Diff loading messages ──────────────────────────────────

// DiffLoadedMsg is sent when a diff finishes loading from the cache.
type DiffLoadedMsg struct {
	SHA  string
	Diff string
	Err  error
}

// ─── Diff file parsing ──────────────────────────────────────

// diffFileSection represents one file's chunk of the diff output.
type diffFileSection struct {
	Filename string
	Content  string
}

// parseDiffFiles splits a unified diff into per-file sections.
func parseDiffFiles(diff string) []diffFileSection {
	if diff == "" {
		return nil
	}

	var sections []diffFileSection
	lines := strings.Split(diff, "\n")
	var current *diffFileSection

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				sections = append(sections, *current)
			}
			filename := extractFilename(line)
			current = &diffFileSection{
				Filename: filename,
				Content:  line + "\n",
			}
		} else if current != nil {
			current.Content += line + "\n"
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}

// extractFilename pulls the b/ path from "diff --git a/foo b/foo".
func extractFilename(diffLine string) string {
	parts := strings.SplitN(diffLine, " b/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return diffLine
}

// ─── PreviewScreen ──────────────────────────────────────────

const listHeightRatio = 0.4

// PreviewScreen implements PREVIEW mode — split layout with compressed list (top)
// and diff view (bottom). Uses our custom DiffViewModel (not bubbles viewport).
type PreviewScreen struct {
	list     *ListScreen
	diffView components.DiffViewModel

	diffFiles []diffFileSection
	fileIndex int

	loading    bool
	currentSHA string

	cache plugin.StashCache
	theme theme.Theme

	width  int
	height int
}

// NewPreviewScreen creates a new PREVIEW mode screen.
func NewPreviewScreen(list *ListScreen, cache plugin.StashCache, th theme.Theme) *PreviewScreen {
	return &PreviewScreen{
		list:     list,
		diffView: components.NewDiffViewModel(th, 80, 20),
		cache:    cache,
		theme:    th,
	}
}

// Update handles messages for the PREVIEW screen.
func (p *PreviewScreen) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height - layout.ChromeHeight
		p.recalcSplit()
		return state, nil

	case DiffLoadedMsg:
		return p.handleDiffLoaded(msg, state)

	case tea.KeyPressMsg:
		return p.handleKey(msg, state)
	}
	return state, nil
}

// View renders the PREVIEW mode: compressed list (top) + divider + diff view (bottom).
func (p *PreviewScreen) View(state core.AppState, width, height int) string {
	p.width = width
	p.height = height
	p.recalcSplit()

	th := p.theme

	// Top pane: compressed stash list.
	listH := int(float64(height) * listHeightRatio)
	listView := p.list.View(state, width, listH)

	// Divider with file progress indicator.
	dividerText := p.fileProgressIndicator()
	dividerStyle := lipgloss.NewStyle().
		Foreground(th.AccentGold()).
		Width(width)
	lineLen := max(width-lipgloss.Width(dividerText)-7, 0)
	divider := dividerStyle.Render(
		fmt.Sprintf("\u2500\u2500\u2500\u2500\u2500 %s %s", dividerText, strings.Repeat("\u2500", lineLen)))

	// Bottom pane: diff view (or loading indicator).
	var diffView string
	if p.loading {
		diffView = lipgloss.NewStyle().
			Foreground(th.FgSecondary()).
			Width(width).
			Render("Loading diff...")
	} else {
		diffView = p.diffView.View()
	}

	return lipgloss.JoinVertical(lipgloss.Top, listView, divider, diffView)
}

// EnsureDiffLoaded triggers a diff load for the currently selected stash if not already loaded.
func (p *PreviewScreen) EnsureDiffLoaded(state core.AppState) tea.Cmd {
	if len(state.Stashes) == 0 {
		return nil
	}
	stash := state.Stashes[p.list.cursor]
	if stash.SHA == p.currentSHA {
		return nil
	}
	p.loading = true
	p.currentSHA = stash.SHA
	return loadDiffCmd(p.cache, stash.SHA)
}

// ─── Internal ───────────────────────────────────────────────

func (p *PreviewScreen) handleDiffLoaded(msg DiffLoadedMsg, state core.AppState) (core.AppState, tea.Cmd) {
	if msg.SHA != p.currentSHA {
		return state, nil // stale response
	}
	p.loading = false
	if msg.Err != nil {
		p.diffView.SetContent(fmt.Sprintf("Error loading diff: %v", msg.Err))
		p.diffFiles = nil
		return state, nil
	}
	p.diffFiles = parseDiffFiles(msg.Diff)
	p.fileIndex = 0
	if len(p.diffFiles) > 0 {
		p.diffView.SetContent(p.diffFiles[0].Content)
	} else {
		p.diffView.SetContent("(no changes)")
	}
	return state, nil
}

func (p *PreviewScreen) handleKey(msg tea.KeyPressMsg, state core.AppState) (core.AppState, tea.Cmd) {
	n := len(state.Stashes)

	switch {
	// File cycling in diff pane.
	case msg.Text == "h":
		p.cycleFile(-1)
		return state, nil
	case msg.Text == "l":
		p.cycleFile(1)
		return state, nil

	// Diff viewport scroll.
	case msg.Text == "d" && msg.Mod.Contains(tea.ModCtrl):
		p.diffView.ScrollDown(p.diffView.LineCount() / 2)
		return state, nil
	case msg.Text == "u" && msg.Mod.Contains(tea.ModCtrl):
		p.diffView.ScrollUp(p.diffView.LineCount() / 2)
		return state, nil

	// Tab toggles back to LIST.
	case msg.Code == tea.KeyTab:
		state.Mode = core.ModeList
		return state, nil

	// Enter goes to DETAIL.
	case msg.Code == tea.KeyEnter:
		if n > 0 {
			state.Mode = core.ModeDetail
		}
		return state, nil

	// Arrow keys mapped to j/k for stash navigation.
	case msg.Code == tea.KeyDown:
		prevCursor := p.list.cursor
		p.list.moveCursor(1, len(state.Stashes))
		state.Cursor = p.list.cursor
		return state, p.maybeReloadDiff(prevCursor, state)
	case msg.Code == tea.KeyUp:
		prevCursor := p.list.cursor
		p.list.moveCursor(-1, len(state.Stashes))
		state.Cursor = p.list.cursor
		return state, p.maybeReloadDiff(prevCursor, state)

	// j/k navigate the list and may reload diff.
	case msg.Text == "j", msg.Text == "k", msg.Text == "g", msg.Text == "G":
		prevCursor := p.list.cursor
		state, _ = p.list.Update(msg, state)
		return state, p.maybeReloadDiff(prevCursor, state)

	// CRUD actions — delegate to list.
	case msg.Text == "a" && n > 0,
		msg.Text == "p" && n > 0,
		msg.Text == "r" && n > 0,
		msg.Text == "b" && n > 0:
		return p.list.Update(msg, state)
	case msg.Text == "d" && !msg.Mod.Contains(tea.ModCtrl) && n > 0:
		return p.list.Update(msg, state)
	case msg.Text == "n":
		return p.list.Update(msg, state)
	case msg.Text == "e":
		return p.list.Update(msg, state)
	case msg.Text == "i":
		return p.list.Update(msg, state)
	}

	return state, nil
}

func (p *PreviewScreen) cycleFile(delta int) {
	if len(p.diffFiles) == 0 {
		return
	}
	p.fileIndex = (p.fileIndex + delta + len(p.diffFiles)) % len(p.diffFiles)
	p.diffView.SetContent(p.diffFiles[p.fileIndex].Content)
}

func (p *PreviewScreen) fileProgressIndicator() string {
	if len(p.diffFiles) == 0 {
		return ""
	}
	f := p.diffFiles[p.fileIndex]
	return fmt.Sprintf("%s (%d/%d)", f.Filename, p.fileIndex+1, len(p.diffFiles))
}

func (p *PreviewScreen) maybeReloadDiff(prevCursor int, state core.AppState) tea.Cmd {
	if p.list.cursor == prevCursor || len(state.Stashes) == 0 {
		return nil
	}
	stash := state.Stashes[p.list.cursor]
	if stash.SHA == p.currentSHA {
		return nil
	}
	p.loading = true
	p.currentSHA = stash.SHA
	return loadDiffCmd(p.cache, stash.SHA)
}

func (p *PreviewScreen) recalcSplit() {
	listH := int(float64(p.height) * listHeightRatio)
	viewH := p.height - listH - 1 // 1 for the divider line
	p.list.SetSize(p.width, listH)
	p.diffView.SetSize(p.width, max(viewH, 1))
}

func loadDiffCmd(cache plugin.StashCache, sha string) tea.Cmd {
	return func() tea.Msg {
		diff, err := cache.Diff(context.Background(), sha)
		return DiffLoadedMsg{SHA: sha, Diff: diff, Err: err}
	}
}
