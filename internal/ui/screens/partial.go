package screens

import (
	"context"
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// partial.go implements the PARTIAL screen — the interactive, visual hunk &
// line picker for partial stashing (Proposal 001). It renders a live diff of
// the working tree with tri-state checkboxes, lets the user toggle hunks (or
// individual lines in line-mode), shows a live +X/−Y tally, and on confirm
// builds a stash containing exactly the selected changes via git.CreatePartialStash.

// ─── Messages ───────────────────────────────────────────────

// PartialDiffLoadedMsg carries the parsed working-tree diff into the screen.
type PartialDiffLoadedMsg struct {
	Patch *git.PatchSet
	Err   error
}

// PartialStashCreatedMsg signals a successful partial stash creation.
type PartialStashCreatedMsg struct {
	Stashes []plugin.Stash
	Note    string
}

// PartialStashErrorMsg signals a partial stash error.
type PartialStashErrorMsg struct {
	Err error
}

// ─── Row model ──────────────────────────────────────────────

type pRowKind int

const (
	rowFile pRowKind = iota
	rowHunk
	rowLine
)

type pRow struct {
	kind pRowKind
	file int
	hunk int
	line int
}

// ─── Screen ─────────────────────────────────────────────────

// PartialScreen is the interactive partial-stash picker.
type PartialScreen struct {
	git   plugin.GitRunner
	cache plugin.StashCache
	th    theme.Theme

	patch  *git.PatchSet
	rows   []pRow
	cursor int // index into rows (always a landable row)
	offset int
	width  int
	height int

	lineMode bool // false = hunk granularity, true = line granularity

	// Message prompt sub-state.
	prompting bool
	message   string
	msgCursor int

	loading bool
	errMsg  string
}

// NewPartialScreen creates a new partial stash picker.
func NewPartialScreen(th theme.Theme) *PartialScreen {
	return &PartialScreen{th: th}
}

// Init wires the git runner and cache (called from main.go).
func (s *PartialScreen) Init(runner plugin.GitRunner, cache plugin.StashCache) {
	s.git = runner
	s.cache = cache
}

// EnsureLoaded returns a command that loads the working-tree diff when the
// screen is first entered.
func (s *PartialScreen) EnsureLoaded(_ plugin.AppState) tea.Cmd {
	s.reset()
	s.loading = true
	runner := s.git
	return func() tea.Msg {
		ctx := context.Background()
		out, err := runner.Run(ctx, "diff", "HEAD", "--no-color")
		if err != nil {
			return PartialDiffLoadedMsg{Err: fmt.Errorf("git diff HEAD: %w", err)}
		}
		ps, perr := git.ParsePatch(out + "\n")
		if perr != nil {
			return PartialDiffLoadedMsg{Err: perr}
		}
		return PartialDiffLoadedMsg{Patch: ps}
	}
}

// Update handles messages when the PARTIAL screen is active.
func (s *PartialScreen) Update(msg tea.Msg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return state, nil

	case PartialDiffLoadedMsg:
		s.loading = false
		if msg.Err != nil {
			s.errMsg = msg.Err.Error()
			return state, nil
		}
		s.patch = msg.Patch
		s.buildRows()
		s.snapCursor()
		return state, nil

	case PartialStashCreatedMsg:
		state.Stashes = msg.Stashes
		if state.Cursor >= len(state.Stashes) {
			state.Cursor = max(0, len(state.Stashes)-1)
		}
		state.Mode = plugin.ModeList
		s.reset()
		return state, nil

	case PartialStashErrorMsg:
		s.errMsg = msg.Err.Error()
		s.prompting = false
		return state, nil

	case tea.KeyPressMsg:
		return s.handleKey(msg, state)
	}
	return state, nil
}

func (s *PartialScreen) handleKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	if s.prompting {
		return s.handlePromptKey(msg, state)
	}

	switch {
	case msg.Text == "escape", msg.Code == tea.KeyEscape:
		state.Mode = plugin.ModeList
		s.reset()
		return state, nil

	case msg.Text == "j", msg.Code == tea.KeyDown:
		s.moveCursor(1)
	case msg.Text == "k", msg.Code == tea.KeyUp:
		s.moveCursor(-1)
	case msg.Text == "g":
		s.cursor = 0
		s.snapCursor()
	case msg.Text == "G":
		s.cursor = len(s.rows) - 1
		s.snapCursorBackward()

	case msg.Text == "v":
		s.lineMode = !s.lineMode
		s.snapCursor()

	case msg.Text == " ":
		s.toggleFocused()
	case msg.Text == "a":
		s.toggleFile()
	case msg.Text == "A":
		s.toggleAll()

	case msg.Text == "enter", msg.Code == tea.KeyEnter:
		if s.patch != nil && s.patch.HasSelection() {
			s.prompting = true
			s.errMsg = ""
		} else {
			s.errMsg = "nothing selected"
		}
		return state, nil
	}
	return state, nil
}

func (s *PartialScreen) handlePromptKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch {
	case msg.Text == "escape", msg.Code == tea.KeyEscape:
		s.prompting = false
		return state, nil
	case msg.Text == "enter", msg.Code == tea.KeyEnter:
		return state, s.createCmd()
	case msg.Text == "backspace" || msg.Code == 8:
		if s.msgCursor > 0 {
			s.message = s.message[:s.msgCursor-1] + s.message[s.msgCursor:]
			s.msgCursor--
		}
		return state, nil
	case msg.Text == "left":
		if s.msgCursor > 0 {
			s.msgCursor--
		}
		return state, nil
	case msg.Text == "right":
		if s.msgCursor < len(s.message) {
			s.msgCursor++
		}
		return state, nil
	default:
		if len(msg.Text) > 0 && msg.Mod == 0 && len(s.message) < 200 {
			s.message = s.message[:s.msgCursor] + msg.Text + s.message[s.msgCursor:]
			s.msgCursor += len(msg.Text)
		}
		return state, nil
	}
}

// createCmd builds the selected patch and creates the partial stash.
func (s *PartialScreen) createCmd() tea.Cmd {
	patch := s.patch.BuildSelectedPatch()
	message := strings.TrimSpace(s.message)
	if message == "" {
		message = "partial stash"
	}
	runner := s.git
	cache := s.cache
	return func() tea.Msg {
		ctx := context.Background()
		res, err := git.CreatePartialStash(ctx, runner, patch, message)
		if err != nil {
			return PartialStashErrorMsg{Err: err}
		}
		cache.Invalidate()
		stashes, err := cache.List(ctx)
		if err != nil {
			return PartialStashErrorMsg{Err: fmt.Errorf("reload after partial stash: %w", err)}
		}
		return PartialStashCreatedMsg{Stashes: stashes, Note: res.Note}
	}
}

// ─── Navigation ─────────────────────────────────────────────

func (s *PartialScreen) buildRows() {
	s.rows = s.rows[:0]
	if s.patch == nil {
		return
	}
	for fi := range s.patch.Files {
		s.rows = append(s.rows, pRow{kind: rowFile, file: fi})
		if s.patch.Files[fi].IsBinary {
			continue
		}
		for hi := range s.patch.Files[fi].Hunks {
			s.rows = append(s.rows, pRow{kind: rowHunk, file: fi, hunk: hi})
			for li := range s.patch.Files[fi].Hunks[hi].Lines {
				s.rows = append(s.rows, pRow{kind: rowLine, file: fi, hunk: hi, line: li})
			}
		}
	}
}

// landable reports whether the cursor may rest on a row given the granularity.
func (s *PartialScreen) landable(r pRow) bool {
	switch r.kind {
	case rowFile:
		return true
	case rowHunk:
		return !s.lineMode
	case rowLine:
		if !s.lineMode {
			return false
		}
		f := s.patch.Files[r.file]
		if f.IsBinary {
			return false
		}
		return f.Hunks[r.hunk].Lines[r.line].Kind != git.LineContext
	}
	return false
}

func (s *PartialScreen) moveCursor(delta int) {
	if len(s.rows) == 0 {
		return
	}
	i := s.cursor
	for {
		i += delta
		if i < 0 || i >= len(s.rows) {
			return // keep cursor at current landable row
		}
		if s.landable(s.rows[i]) {
			s.cursor = i
			s.ensureVisible()
			return
		}
	}
}

// snapCursor moves the cursor forward to the nearest landable row.
func (s *PartialScreen) snapCursor() {
	if len(s.rows) == 0 {
		s.cursor = 0
		return
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= len(s.rows) {
		s.cursor = len(s.rows) - 1
	}
	for i := s.cursor; i < len(s.rows); i++ {
		if s.landable(s.rows[i]) {
			s.cursor = i
			s.ensureVisible()
			return
		}
	}
	s.snapCursorBackward()
}

func (s *PartialScreen) snapCursorBackward() {
	for i := min(s.cursor, len(s.rows)-1); i >= 0; i-- {
		if s.landable(s.rows[i]) {
			s.cursor = i
			s.ensureVisible()
			return
		}
	}
}

func (s *PartialScreen) ensureVisible() {
	vis := s.visibleRows()
	if vis <= 0 {
		return
	}
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+vis {
		s.offset = s.cursor - vis + 1
	}
	if s.offset < 0 {
		s.offset = 0
	}
}

func (s *PartialScreen) visibleRows() int {
	// Reserve 1 line for the tally/prompt header.
	return max(s.height-1, 1)
}

// ─── Selection toggles ──────────────────────────────────────

func (s *PartialScreen) toggleFocused() {
	if s.patch == nil || s.cursor >= len(s.rows) {
		return
	}
	r := s.rows[s.cursor]
	switch r.kind {
	case rowFile:
		f := &s.patch.Files[r.file]
		f.SetSelected(f.SelectionState() != git.SelAll)
	case rowHunk:
		h := &s.patch.Files[r.file].Hunks[r.hunk]
		h.SetSelected(h.SelectionState() != git.SelAll)
	case rowLine:
		ln := &s.patch.Files[r.file].Hunks[r.hunk].Lines[r.line]
		if ln.Kind != git.LineContext {
			ln.Selected = !ln.Selected
		}
	}
}

func (s *PartialScreen) toggleFile() {
	if s.patch == nil || s.cursor >= len(s.rows) {
		return
	}
	fi := s.rows[s.cursor].file
	f := &s.patch.Files[fi]
	f.SetSelected(f.SelectionState() != git.SelAll)
}

func (s *PartialScreen) toggleAll() {
	if s.patch == nil {
		return
	}
	s.patch.SelectAll(!s.patch.HasSelection())
}

// ─── Rendering ──────────────────────────────────────────────

// View renders the PARTIAL screen content area.
func (s *PartialScreen) View(_ plugin.AppState, width, height int) string {
	s.width = width
	s.height = height
	th := s.th

	if s.loading {
		return s.center(width, height, lipgloss.NewStyle().Foreground(th.FgSecondary()).Render("Loading changes…"))
	}
	if s.errMsg != "" && (s.patch == nil || len(s.rows) == 0) {
		return s.center(width, height, lipgloss.NewStyle().Foreground(th.SemanticRed()).Render("Error: "+s.errMsg))
	}
	if s.patch == nil || len(s.patch.Files) == 0 {
		return s.center(width, height, lipgloss.NewStyle().Foreground(th.FgSecondary()).
			Render("No working-tree changes to stash.\n\nPress Esc to go back."))
	}

	var b strings.Builder
	b.WriteString(s.renderHeader(width))
	b.WriteString("\n")
	b.WriteString(s.renderRows(width, s.visibleRows()))
	return b.String()
}

func (s *PartialScreen) renderHeader(width int) string {
	th := s.th
	if s.prompting {
		label := lipgloss.NewStyle().Foreground(th.AccentGold()).Bold(true).Render("  Message: ")
		before := s.message[:s.msgCursor]
		after := s.message[s.msgCursor:]
		cursorChar := " "
		if s.msgCursor < len(s.message) {
			cursorChar = string(s.message[s.msgCursor])
			after = after[1:]
		}
		cur := lipgloss.NewStyle().Reverse(true).Render(cursorChar)
		line := label + before + cur + after
		return padLine(line, width, th.BgDeep())
	}

	st := s.patch.SelectedStats()
	gran := "hunk"
	if s.lineMode {
		gran = "line"
	}
	added := lipgloss.NewStyle().Foreground(th.SemanticGreen()).Render(fmt.Sprintf("+%d", st.Added))
	removed := lipgloss.NewStyle().Foreground(th.SemanticRed()).Render(fmt.Sprintf("−%d", st.Removed))
	meta := lipgloss.NewStyle().Foreground(th.FgSecondary()).
		Render(fmt.Sprintf(" · %d hunks · %d files   [%s-mode]", st.Hunks, st.Files, gran))
	line := "  Selected: " + added + " " + removed + meta
	if s.errMsg != "" {
		line += lipgloss.NewStyle().Foreground(th.SemanticRed()).Render("   " + s.errMsg)
	}
	return padLine(line, width, th.BgDeep())
}

func (s *PartialScreen) renderRows(width, height int) string {
	th := s.th
	end := min(s.offset+height, len(s.rows))
	var out []string
	for i := s.offset; i < end; i++ {
		out = append(out, s.renderRow(s.rows[i], i == s.cursor, width))
	}
	// Pad to full height with themed blank lines.
	blank := padLine("", width, th.BgDeep())
	for len(out) < height {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

func (s *PartialScreen) renderRow(r pRow, focused bool, width int) string {
	th := s.th
	cursor := "  "
	if focused {
		cursor = lipgloss.NewStyle().Foreground(th.AccentBright()).Bold(true).Render("▸ ")
	}

	switch r.kind {
	case rowFile:
		f := &s.patch.Files[r.file]
		box := s.checkbox(f.SelectionState())
		name := f.NewPath
		if name == "" {
			name = f.OldPath
		}
		tag := ""
		switch {
		case f.IsBinary:
			tag = "  binary (not selectable)"
		case f.IsNew:
			tag = "  new file"
		case f.IsDelete:
			tag = "  deleted"
		case f.IsRename:
			tag = "  renamed"
		}
		nameStyle := lipgloss.NewStyle().Foreground(th.FgPrimary()).Bold(true)
		tagStyle := lipgloss.NewStyle().Foreground(th.FgDimmed())
		line := cursor + box + " " + nameStyle.Render(name) + tagStyle.Render(tag)
		return padLine(line, width, th.BgDeep())

	case rowHunk:
		h := &s.patch.Files[r.file].Hunks[r.hunk]
		box := s.checkbox(h.SelectionState())
		hdr := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		if h.Section != "" {
			hdr += " " + h.Section
		}
		hunkStyle := lipgloss.NewStyle().Foreground(th.DiffHunk())
		line := cursor + "  " + box + " " + hunkStyle.Render(hdr)
		return padLine(line, width, th.BgDeep())

	default: // rowLine
		ln := s.patch.Files[r.file].Hunks[r.hunk].Lines[r.line]
		return s.renderLine(ln, focused, width)
	}
}

func (s *PartialScreen) renderLine(ln git.PatchLine, focused bool, width int) string {
	th := s.th
	cursor := "    "
	if focused {
		cursor = lipgloss.NewStyle().Foreground(th.AccentBright()).Bold(true).Render("  ▸ ")
	}

	// Per-line checkbox only shown in line-mode for changeable lines.
	box := "  "
	if s.lineMode && ln.Kind != git.LineContext {
		if ln.Selected {
			box = lipgloss.NewStyle().Foreground(th.SemanticGreen()).Render("▣ ")
		} else {
			box = lipgloss.NewStyle().Foreground(th.FgDimmed()).Render("▢ ")
		}
	}

	var style lipgloss.Style
	var prefix string
	switch ln.Kind {
	case git.LineAdded:
		prefix = "+"
		if ln.Selected {
			style = lipgloss.NewStyle().Foreground(th.DiffAddedFg()).Background(th.DiffAddedBg())
		} else {
			style = lipgloss.NewStyle().Foreground(th.FgDimmed())
		}
	case git.LineRemoved:
		prefix = "-"
		if ln.Selected {
			style = lipgloss.NewStyle().Foreground(th.DiffRemovedFg()).Background(th.DiffRemovedBg())
		} else {
			style = lipgloss.NewStyle().Foreground(th.FgDimmed())
		}
	default:
		prefix = " "
		style = lipgloss.NewStyle().Foreground(th.FgSecondary())
	}
	content := style.Render(prefix + ln.Text)
	line := cursor + box + content
	return padLine(line, width, th.BgDeep())
}

// checkbox renders a tri-state selection glyph.
func (s *PartialScreen) checkbox(st git.SelState) string {
	th := s.th
	switch st {
	case git.SelAll:
		return lipgloss.NewStyle().Foreground(th.SemanticGreen()).Render("▣")
	case git.SelPartial:
		return lipgloss.NewStyle().Foreground(th.SemanticYellow()).Render("◪")
	default:
		return lipgloss.NewStyle().Foreground(th.FgDimmed()).Render("▢")
	}
}

func (s *PartialScreen) center(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func (s *PartialScreen) reset() {
	s.patch = nil
	s.rows = nil
	s.cursor = 0
	s.offset = 0
	s.lineMode = false
	s.prompting = false
	s.message = ""
	s.msgCursor = 0
	s.errMsg = ""
	s.loading = false
}

// padLine renders a string padded with themed background to the given width.
func padLine(s string, width int, bg color.Color) string {
	w := lipgloss.Width(s)
	if w < width {
		s += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", width-w))
	}
	return lipgloss.NewStyle().Background(bg).MaxWidth(width).Render(s)
}

// ─── Test accessors ─────────────────────────────────────────

// SetPatchForTest installs a parsed patch directly (testing only).
func (s *PartialScreen) SetPatchForTest(ps *git.PatchSet) {
	s.patch = ps
	s.loading = false
	s.buildRows()
	s.snapCursor()
}

// PatchForTest returns the current patch (testing only).
func (s *PartialScreen) PatchForTest() *git.PatchSet { return s.patch }

// CursorRowForTest returns the kind of the focused row (testing only):
// "file", "hunk", or "line".
func (s *PartialScreen) CursorRowForTest() string {
	if s.cursor >= len(s.rows) {
		return ""
	}
	switch s.rows[s.cursor].kind {
	case rowFile:
		return "file"
	case rowHunk:
		return "hunk"
	default:
		return "line"
	}
}

// LineModeForTest reports the current granularity (testing only).
func (s *PartialScreen) LineModeForTest() bool { return s.lineMode }

// PromptingForTest reports whether the message prompt is active (testing only).
func (s *PartialScreen) PromptingForTest() bool { return s.prompting }
