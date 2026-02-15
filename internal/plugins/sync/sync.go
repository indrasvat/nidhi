package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os/user"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

const (
	PluginID   = "sync"
	PluginName = "Export/Import & Remote Sync"

	MinGitVersionMajor = 2
	MinGitVersionMinor = 51
)

// Remote represents a git remote.
type Remote struct {
	Name string
	URL  string
}

// ─── tea.Msg types ──────────────────────────────────────────

type RemoteListMsg struct {
	Remotes []Remote
}

type ExportResultMsg struct {
	ExportedCount int
	Ref           string
	Remote        string
	Err           error
}

type ImportResultMsg struct {
	ImportedCount int
	Err           error
}

// ─── Plugin ─────────────────────────────────────────────────

// Plugin implements KeyHandler + ScreenProvider for export/import (PRD FR-12).
type Plugin struct {
	git           plugin.GitRunner
	cache         plugin.StashCache
	events        plugin.EventBus
	logger        *slog.Logger
	th            theme.Theme
	exportEnabled bool
	defaultRef    string
	defaultRemote string

	// Export screen state.
	exportActive  bool
	exportStashes []plugin.Stash
	exportSel     []bool
	exportCursor  int
	exportRef     string
	exportRefCur  int
	exportRemotes []Remote
	exportRemIdx  int
	exportFocus   int // 0=stash list, 1=ref input, 2=remote selector
	exporting     bool

	// Import screen state.
	importActive  bool
	importRef     string
	importRefCur  int
	importRemotes []Remote
	importRemIdx  int
	importFocus   int // 0=ref input, 1=remote selector
	importFetched bool
	importing     bool
}

var (
	_ plugin.KeyHandler     = (*Plugin)(nil)
	_ plugin.ScreenProvider = (*Plugin)(nil)
)

func New(th theme.Theme) *Plugin {
	return &Plugin{th: th}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.cache = ctx.Cache
	p.events = ctx.Events
	p.logger = ctx.Logger

	p.exportEnabled = ctx.GitVer.AtLeast(MinGitVersionMajor, MinGitVersionMinor)

	// Load config defaults.
	if ctx.Config != nil {
		if ref := ctx.Config.GetString("export_ref"); ref != "" {
			p.defaultRef = ref
		}
		if remote := ctx.Config.GetString("export_remote"); remote != "" {
			p.defaultRemote = remote
		}
	}
	if p.defaultRef == "" {
		p.defaultRef = "refs/stashes/user"
	}
	if p.defaultRemote == "" {
		p.defaultRemote = "origin"
	}

	// Expand $USER in ref.
	if u, err := user.Current(); err == nil {
		p.defaultRef = strings.ReplaceAll(p.defaultRef, "$USER", u.Username)
	}

	return nil
}

func (p *Plugin) Destroy() error { return nil }

// ─── KeyHandler ─────────────────────────────────────────────

func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "e", Desc: "Export stashes", Modes: []plugin.Mode{plugin.ModeList}},
		{Key: "i", Desc: "Import stashes", Modes: []plugin.Mode{plugin.ModeList}},
	}
}

func (p *Plugin) HandleKey(key plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch key.Key {
	case "e":
		if !p.exportEnabled {
			return state, func() tea.Msg {
				return core.InfoToastMsg{
					Text: fmt.Sprintf("Export requires Git >= 2.51 (current: %s)", state.GitVersion.Raw),
				}
			}
		}
		p.initExportScreen(state)
		state.Mode = plugin.ModeExport
		return state, p.fetchRemotesCmd()

	case "i":
		if !p.exportEnabled {
			return state, func() tea.Msg {
				return core.InfoToastMsg{
					Text: fmt.Sprintf("Import requires Git >= 2.51 (current: %s)", state.GitVersion.Raw),
				}
			}
		}
		p.initImportScreen()
		state.Mode = plugin.ModeImport
		return state, p.fetchRemotesCmd()
	}

	return state, nil
}

// ─── ScreenProvider ─────────────────────────────────────────

func (p *Plugin) Screens() []plugin.ScreenDef {
	return []plugin.ScreenDef{
		{Mode: plugin.ModeExport, Name: "EXPORT"},
		{Mode: plugin.ModeImport, Name: "IMPORT"},
	}
}

func (p *Plugin) Update(msg tea.Msg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch state.Mode {
	case plugin.ModeExport:
		return p.updateExport(msg, state)
	case plugin.ModeImport:
		return p.updateImport(msg, state)
	}
	return state, nil
}

func (p *Plugin) View(state plugin.AppState, width, height int) string {
	switch state.Mode {
	case plugin.ModeExport:
		return p.viewExport(width, height)
	case plugin.ModeImport:
		return p.viewImport(width, height)
	}
	return ""
}

// ─── Export Screen ───────────────────────────────────────────

func (p *Plugin) initExportScreen(state plugin.AppState) {
	p.exportActive = true
	p.exportStashes = make([]plugin.Stash, len(state.Stashes))
	copy(p.exportStashes, state.Stashes)
	p.exportSel = make([]bool, len(state.Stashes))
	for i := range p.exportSel {
		p.exportSel[i] = true
	}
	p.exportCursor = 0
	p.exportRef = p.defaultRef
	p.exportRefCur = len(p.defaultRef)
	p.exportRemotes = nil
	p.exportRemIdx = 0
	p.exportFocus = 0
	p.exporting = false
}

func (p *Plugin) updateExport(msg tea.Msg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case RemoteListMsg:
		p.exportRemotes = msg.Remotes
		// Select the default remote if it exists.
		for i, r := range msg.Remotes {
			if r.Name == p.defaultRemote {
				p.exportRemIdx = i
				break
			}
		}
		return state, nil

	case ExportResultMsg:
		p.exporting = false
		p.exportActive = false
		state.Mode = plugin.ModeList
		if msg.Err != nil {
			return state, func() tea.Msg { return core.ErrorMsg{Err: msg.Err} }
		}
		p.cache.Invalidate()
		text := fmt.Sprintf("Exported %d stash(es) to %s → %s", msg.ExportedCount, msg.Remote, msg.Ref)
		return state, func() tea.Msg { return core.InfoToastMsg{Text: text} }

	case tea.KeyPressMsg:
		return p.handleExportKey(msg, state)
	}
	return state, nil
}

func (p *Plugin) handleExportKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	if p.exporting {
		return state, nil
	}

	switch msg.Code {
	case tea.KeyEscape:
		p.exportActive = false
		state.Mode = plugin.ModeList
		return state, nil

	case tea.KeyTab:
		p.exportFocus = (p.exportFocus + 1) % 3
		return state, nil

	case tea.KeyEnter:
		if p.exportFocus == 0 || p.exportFocus == 2 {
			return state, p.executeExportCmd()
		}
		return state, nil
	}

	switch p.exportFocus {
	case 0: // Stash list
		return p.handleExportListKey(msg, state)
	case 1: // Ref input
		return p.handleExportRefKey(msg, state)
	case 2: // Remote selector
		return p.handleExportRemoteKey(msg, state)
	}

	return state, nil
}

func (p *Plugin) handleExportListKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch {
	case msg.Text == "j" || msg.Code == tea.KeyDown:
		if p.exportCursor < len(p.exportStashes)-1 {
			p.exportCursor++
		}
	case msg.Text == "k" || msg.Code == tea.KeyUp:
		if p.exportCursor > 0 {
			p.exportCursor--
		}
	case msg.Text == " ":
		if p.exportCursor < len(p.exportSel) {
			p.exportSel[p.exportCursor] = !p.exportSel[p.exportCursor]
		}
	case msg.Text == "a":
		allSelected := true
		for _, sel := range p.exportSel {
			if !sel {
				allSelected = false
				break
			}
		}
		for i := range p.exportSel {
			p.exportSel[i] = !allSelected
		}
	}
	return state, nil
}

func (p *Plugin) handleExportRefKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg.Code {
	case tea.KeyBackspace:
		if p.exportRefCur > 0 {
			p.exportRef = p.exportRef[:p.exportRefCur-1] + p.exportRef[p.exportRefCur:]
			p.exportRefCur--
		}
	case tea.KeyLeft:
		if p.exportRefCur > 0 {
			p.exportRefCur--
		}
	case tea.KeyRight:
		if p.exportRefCur < len(p.exportRef) {
			p.exportRefCur++
		}
	case tea.KeyHome:
		p.exportRefCur = 0
	case tea.KeyEnd:
		p.exportRefCur = len(p.exportRef)
	default:
		if msg.Text != "" && len(msg.Text) == 1 && msg.Text[0] >= 32 {
			p.exportRef = p.exportRef[:p.exportRefCur] + msg.Text + p.exportRef[p.exportRefCur:]
			p.exportRefCur += len(msg.Text)
		}
	}
	return state, nil
}

func (p *Plugin) handleExportRemoteKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch {
	case msg.Text == "j" || msg.Code == tea.KeyDown:
		if p.exportRemIdx < len(p.exportRemotes)-1 {
			p.exportRemIdx++
		}
	case msg.Text == "k" || msg.Code == tea.KeyUp:
		if p.exportRemIdx > 0 {
			p.exportRemIdx--
		}
	}
	return state, nil
}

func (p *Plugin) executeExportCmd() tea.Cmd {
	indices := p.SelectedExportIndices()
	if len(indices) == 0 || p.exportRef == "" {
		return nil
	}
	remote := p.defaultRemote
	if p.exportRemIdx < len(p.exportRemotes) {
		remote = p.exportRemotes[p.exportRemIdx].Name
	}
	p.exporting = true
	return ExportCmd(p.git, p.exportRef, remote, indices)
}

// SelectedExportIndices returns the stash indices selected for export.
func (p *Plugin) SelectedExportIndices() []int {
	var indices []int
	for i, sel := range p.exportSel {
		if sel && i < len(p.exportStashes) {
			indices = append(indices, p.exportStashes[i].Index)
		}
	}
	return indices
}

func (p *Plugin) viewExport(_, _ int) string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle()
	selStyle := lipgloss.NewStyle()
	activeStyle := lipgloss.NewStyle()
	if p.th != nil {
		headerStyle = headerStyle.Foreground(p.th.FgPrimary())
		dimStyle = dimStyle.Foreground(p.th.FgDimmed())
		selStyle = selStyle.Foreground(p.th.SemanticGreen())
		activeStyle = activeStyle.Foreground(p.th.FgPrimary()).Reverse(true)
	}

	b.WriteString("\n  ")
	b.WriteString(headerStyle.Render("Export Stashes"))
	b.WriteString("\n\n")

	// Stash list with multi-select.
	focusLabel := func(idx int) string {
		if p.exportFocus == idx {
			return " *"
		}
		return "  "
	}

	b.WriteString(dimStyle.Render("  Select stashes:") + focusLabel(0) + "\n")
	for i, stash := range p.exportStashes {
		check := "[ ]"
		if p.exportSel[i] {
			check = selStyle.Render("[x]")
		}
		line := fmt.Sprintf("    %s  %d  %s", check, stash.Index, stash.Message)
		if p.exportFocus == 0 && i == p.exportCursor {
			line = activeStyle.Render(line)
		}
		b.WriteString(line + "\n")
	}

	// Ref input.
	b.WriteString("\n" + dimStyle.Render("  Ref path:") + focusLabel(1) + "\n")
	b.WriteString("    " + p.renderRefInput(p.exportRef, p.exportRefCur, p.exportFocus == 1) + "\n")

	// Remote selector.
	b.WriteString("\n" + dimStyle.Render("  Remote:") + focusLabel(2) + "\n")
	if len(p.exportRemotes) > 0 {
		for i, r := range p.exportRemotes {
			marker := "  "
			if i == p.exportRemIdx {
				marker = "> "
			}
			line := fmt.Sprintf("    %s%s (%s)", marker, r.Name, r.URL)
			b.WriteString(line + "\n")
		}
	} else {
		b.WriteString("    " + dimStyle.Render("(no remotes configured)") + "\n")
	}

	// Command preview.
	b.WriteString("\n" + dimStyle.Render("  Command preview:") + "\n")
	preview := p.ExportCommandPreview()
	for line := range strings.SplitSeq(preview, "\n") {
		b.WriteString("  " + dimStyle.Render(line) + "\n")
	}

	b.WriteString("\n  " + dimStyle.Render("Tab: switch fields  Space: toggle  Enter: execute  Esc: cancel") + "\n")

	return b.String()
}

// ExportCommandPreview returns the commands that will be executed.
func (p *Plugin) ExportCommandPreview() string {
	indices := p.SelectedExportIndices()
	if len(indices) == 0 || p.exportRef == "" {
		return "(select stashes and enter a ref path)"
	}

	var stashArgs []string
	for _, idx := range indices {
		stashArgs = append(stashArgs, fmt.Sprintf("stash@{%d}", idx))
	}

	remote := p.defaultRemote
	if p.exportRemIdx < len(p.exportRemotes) {
		remote = p.exportRemotes[p.exportRemIdx].Name
	}

	line1 := fmt.Sprintf("$ git stash export --to-ref %s %s", p.exportRef, strings.Join(stashArgs, " "))
	line2 := fmt.Sprintf("$ git push --no-verify --force %s %s", remote, p.exportRef)
	return line1 + "\n" + line2
}

func (p *Plugin) renderRefInput(ref string, cursor int, focused bool) string {
	if !focused {
		return ref
	}
	before := ref[:cursor]
	after := ref[cursor:]
	cursorChar := " "
	if cursor < len(ref) {
		cursorChar = string(ref[cursor])
		after = ref[cursor+1:]
	}
	cursorStyle := lipgloss.NewStyle().Reverse(true)
	return before + cursorStyle.Render(cursorChar) + after
}

// ─── Import Screen ──────────────────────────────────────────

func (p *Plugin) initImportScreen() {
	p.importActive = true
	p.importRef = p.defaultRef
	p.importRefCur = len(p.defaultRef)
	p.importRemotes = nil
	p.importRemIdx = 0
	p.importFocus = 0
	p.importFetched = false
	p.importing = false
}

func (p *Plugin) updateImport(msg tea.Msg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case RemoteListMsg:
		p.importRemotes = msg.Remotes
		for i, r := range msg.Remotes {
			if r.Name == p.defaultRemote {
				p.importRemIdx = i
				break
			}
		}
		return state, nil

	case ImportResultMsg:
		p.importing = false
		p.importActive = false
		state.Mode = plugin.ModeList
		if msg.Err != nil {
			return state, func() tea.Msg { return core.ErrorMsg{Err: msg.Err} }
		}
		p.cache.Invalidate()
		return state, func() tea.Msg {
			return core.InfoToastMsg{Text: "Stashes imported successfully"}
		}

	case tea.KeyPressMsg:
		return p.handleImportKey(msg, state)
	}
	return state, nil
}

func (p *Plugin) handleImportKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	if p.importing {
		return state, nil
	}

	switch msg.Code {
	case tea.KeyEscape:
		p.importActive = false
		state.Mode = plugin.ModeList
		return state, nil

	case tea.KeyTab:
		p.importFocus = (p.importFocus + 1) % 2
		return state, nil

	case tea.KeyEnter:
		return state, p.executeImportCmd()
	}

	switch p.importFocus {
	case 0: // Ref input
		return p.handleImportRefKey(msg, state)
	case 1: // Remote selector
		return p.handleImportRemoteKey(msg, state)
	}

	return state, nil
}

func (p *Plugin) handleImportRefKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg.Code {
	case tea.KeyBackspace:
		if p.importRefCur > 0 {
			p.importRef = p.importRef[:p.importRefCur-1] + p.importRef[p.importRefCur:]
			p.importRefCur--
		}
	case tea.KeyLeft:
		if p.importRefCur > 0 {
			p.importRefCur--
		}
	case tea.KeyRight:
		if p.importRefCur < len(p.importRef) {
			p.importRefCur++
		}
	default:
		if msg.Text != "" && len(msg.Text) == 1 && msg.Text[0] >= 32 {
			p.importRef = p.importRef[:p.importRefCur] + msg.Text + p.importRef[p.importRefCur:]
			p.importRefCur += len(msg.Text)
		}
	}
	return state, nil
}

func (p *Plugin) handleImportRemoteKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch {
	case msg.Text == "j" || msg.Code == tea.KeyDown:
		if p.importRemIdx < len(p.importRemotes)-1 {
			p.importRemIdx++
		}
	case msg.Text == "k" || msg.Code == tea.KeyUp:
		if p.importRemIdx > 0 {
			p.importRemIdx--
		}
	}
	return state, nil
}

func (p *Plugin) executeImportCmd() tea.Cmd {
	if p.importRef == "" {
		return nil
	}
	remote := p.defaultRemote
	if p.importRemIdx < len(p.importRemotes) {
		remote = p.importRemotes[p.importRemIdx].Name
	}
	p.importing = true
	return ImportCmd(p.git, p.importRef, remote)
}

func (p *Plugin) viewImport(_, _ int) string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle()
	if p.th != nil {
		headerStyle = headerStyle.Foreground(p.th.FgPrimary())
		dimStyle = dimStyle.Foreground(p.th.FgDimmed())
	}

	b.WriteString("\n  ")
	b.WriteString(headerStyle.Render("Import Stashes from Remote"))
	b.WriteString("\n\n")

	focusLabel := func(idx int) string {
		if p.importFocus == idx {
			return " *"
		}
		return "  "
	}

	// Ref input.
	b.WriteString(dimStyle.Render("  Ref path:") + focusLabel(0) + "\n")
	b.WriteString("    " + p.renderRefInput(p.importRef, p.importRefCur, p.importFocus == 0) + "\n")

	// Remote selector.
	b.WriteString("\n" + dimStyle.Render("  Remote:") + focusLabel(1) + "\n")
	if len(p.importRemotes) > 0 {
		for i, r := range p.importRemotes {
			marker := "  "
			if i == p.importRemIdx {
				marker = "> "
			}
			fmt.Fprintf(&b, "    %s%s (%s)\n", marker, r.Name, r.URL)
		}
	} else {
		b.WriteString("    " + dimStyle.Render("(no remotes configured)") + "\n")
	}

	// Command preview.
	remote := p.defaultRemote
	if p.importRemIdx < len(p.importRemotes) {
		remote = p.importRemotes[p.importRemIdx].Name
	}
	b.WriteString("\n" + dimStyle.Render("  Commands:") + "\n")
	if p.importRef != "" {
		b.WriteString("  " + dimStyle.Render(fmt.Sprintf("$ git fetch %s %s:%s", remote, p.importRef, p.importRef)) + "\n")
		b.WriteString("  " + dimStyle.Render(fmt.Sprintf("$ git stash import %s", p.importRef)) + "\n")
	}

	if p.importing {
		b.WriteString("\n  Importing...\n")
	} else {
		b.WriteString("\n  " + dimStyle.Render("Tab: switch fields  Enter: fetch & import  Esc: cancel") + "\n")
	}

	return b.String()
}

// ─── Commands ───────────────────────────────────────────────

func (p *Plugin) fetchRemotesCmd() tea.Cmd {
	git := p.git
	return func() tea.Msg {
		ctx := context.Background()
		lines, err := git.RunLines(ctx, "remote", "-v")
		if err != nil {
			return RemoteListMsg{Remotes: nil}
		}
		return RemoteListMsg{Remotes: ParseRemotes(lines)}
	}
}

// ParseRemotes parses `git remote -v` output into Remote structs.
// Deduplicates by name (git remote -v shows fetch and push URLs).
func ParseRemotes(lines []string) []Remote {
	seen := make(map[string]bool)
	var remotes []Remote
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		url := parts[1]
		if !seen[name] {
			seen[name] = true
			remotes = append(remotes, Remote{Name: name, URL: url})
		}
	}
	return remotes
}

// ValidateRef checks if a ref name is valid using `git check-ref-format`.
func ValidateRef(ctx context.Context, git plugin.GitRunner, ref string) error {
	_, err := git.Run(ctx, "check-ref-format", ref)
	if err != nil {
		return fmt.Errorf("invalid ref format %q: %w", ref, err)
	}
	return nil
}

// ExportCmd executes the export workflow:
// 1. git stash export --to-ref <ref> [stash indices...]
// 2. git push --no-verify --force <remote> <ref>
func ExportCmd(git plugin.GitRunner, ref, remote string, stashIndices []int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		if err := ValidateRef(ctx, git, ref); err != nil {
			return ExportResultMsg{Err: err}
		}

		args := []string{"stash", "export", "--to-ref", ref}
		for _, idx := range stashIndices {
			args = append(args, "stash@{"+strconv.Itoa(idx)+"}")
		}
		_, err := git.Run(ctx, args...)
		if err != nil {
			return ExportResultMsg{Err: fmt.Errorf("export: %w", err)}
		}

		_, err = git.Run(ctx, "push", "--no-verify", "--force", remote, ref)
		if err != nil {
			return ExportResultMsg{Err: fmt.Errorf("push: %w", err)}
		}

		return ExportResultMsg{
			ExportedCount: len(stashIndices),
			Ref:           ref,
			Remote:        remote,
		}
	}
}

// ImportCmd executes the import workflow:
// 1. git fetch <remote> <ref>:<ref>
// 2. git stash import <ref>
func ImportCmd(git plugin.GitRunner, ref, remote string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		refSpec := ref + ":" + ref
		_, err := git.Run(ctx, "fetch", remote, refSpec)
		if err != nil {
			return ImportResultMsg{Err: fmt.Errorf("fetch: %w", err)}
		}

		_, err = git.Run(ctx, "stash", "import", ref)
		if err != nil {
			return ImportResultMsg{Err: fmt.Errorf("import: %w", err)}
		}

		return ImportResultMsg{ImportedCount: -1}
	}
}

// IsExportEnabled returns whether export/import is available (Git >= 2.51).
func (p *Plugin) IsExportEnabled() bool {
	return p.exportEnabled
}
