package screens

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── Types ──────────────────────────────────────────────────

// ScopeType represents a category of changes that can be stashed.
type ScopeType int

const (
	ScopeStaged ScopeType = iota
	ScopeUnstaged
	ScopeUntracked
)

// Scope represents a toggleable file scope with a live file count.
type Scope struct {
	Type    ScopeType
	Label   string
	Count   int
	Enabled bool
}

// StashOption represents a toggleable option for stash creation.
type StashOption struct {
	Label   string
	Key     string
	Enabled bool
}

// FocusField tracks which field has keyboard focus.
type FocusField int

const (
	FocusMessage FocusField = iota
	FocusScopes
	FocusOptions
)

// ─── Messages ───────────────────────────────────────────────

// FileCountsMsg carries file counts from `git status`.
type FileCountsMsg struct {
	Staged    int
	Unstaged  int
	Untracked int
}

// StashCreatedMsg signals successful stash creation.
type StashCreatedMsg struct{}

// StashCreateErrorMsg signals a stash creation error.
type StashCreateErrorMsg struct {
	Err error
}

// PatchModeMsg signals that patch mode should be entered via tea.Exec.
type PatchModeMsg struct {
	Args []string
}

// ─── Screen ─────────────────────────────────────────────────

// NewStashScreen implements the new stash creation screen
// (PRD §10 Screen 6, FR-02.4).
type NewStashScreen struct {
	git    plugin.GitRunner
	cache  plugin.StashCache
	logger interface{ Info(string, ...any) }
	th     theme.Theme

	// UI state
	message  string
	cursor   int // Cursor position within message
	scopes   []Scope
	options  []StashOption
	focus    FocusField
	scopeIdx int
	optIdx   int
	errMsg   string
}

// NewNewStashScreen creates a new stash screen instance.
func NewNewStashScreen(th theme.Theme) *NewStashScreen {
	return &NewStashScreen{
		th: th,
		scopes: []Scope{
			{Type: ScopeStaged, Label: "Staged changes", Enabled: true},
			{Type: ScopeUnstaged, Label: "Unstaged changes", Enabled: true},
			{Type: ScopeUntracked, Label: "Untracked files", Enabled: false},
		},
		options: []StashOption{
			{Label: "Keep index (don't unstage staged files)", Key: "--keep-index", Enabled: true},
			{Label: "Patch mode (select hunks)", Key: "--patch", Enabled: false},
		},
		focus: FocusMessage,
	}
}

func (s *NewStashScreen) ID() string   { return "newstash" }
func (s *NewStashScreen) Name() string { return "New Stash" }

func (s *NewStashScreen) Init(ctx plugin.PluginContext) error {
	s.git = ctx.Git
	s.cache = ctx.Cache
	s.logger = ctx.Logger
	return nil
}

func (s *NewStashScreen) Destroy() error { return nil }

func (s *NewStashScreen) Screens() []plugin.ScreenDef {
	return []plugin.ScreenDef{
		{Mode: plugin.ModeNewStash, Name: "New Stash"},
	}
}

// Update handles messages when the new stash screen is active.
func (s *NewStashScreen) Update(msg tea.Msg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return s.handleKey(msg, state)

	case FileCountsMsg:
		s.scopes[0].Count = msg.Staged
		s.scopes[1].Count = msg.Unstaged
		s.scopes[2].Count = msg.Untracked
		return state, nil

	case StashCreatedMsg:
		state.Mode = plugin.ModeList
		s.reset()
		return state, nil

	case StashCreateErrorMsg:
		s.errMsg = msg.Err.Error()
		return state, nil
	}

	return state, nil
}

func (s *NewStashScreen) handleKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	// Handle text input when message field is focused.
	if s.focus == FocusMessage {
		switch {
		case msg.Text == "tab":
			s.focus = FocusScopes
			return state, nil
		case msg.Text == "escape":
			state.Mode = plugin.ModeList
			s.reset()
			return state, nil
		case msg.Text == "enter":
			return state, s.createStash()
		case msg.Text == "backspace" || msg.Code == 8:
			if s.cursor > 0 {
				s.message = s.message[:s.cursor-1] + s.message[s.cursor:]
				s.cursor--
			}
			return state, nil
		case msg.Text == "left":
			if s.cursor > 0 {
				s.cursor--
			}
			return state, nil
		case msg.Text == "right":
			if s.cursor < len(s.message) {
				s.cursor++
			}
			return state, nil
		case msg.Text == "home" || (msg.Text == "a" && msg.Mod == 4): // ctrl+a
			s.cursor = 0
			return state, nil
		case msg.Text == "end" || (msg.Text == "e" && msg.Mod == 4): // ctrl+e
			s.cursor = len(s.message)
			return state, nil
		default:
			// Insert printable characters.
			if len(msg.Text) > 0 && msg.Mod == 0 && len(s.message) < 200 {
				s.message = s.message[:s.cursor] + msg.Text + s.message[s.cursor:]
				s.cursor += len(msg.Text)
			}
			return state, nil
		}
	}

	// Non-message focus handling.
	switch msg.Text {
	case "tab":
		s.focus = (s.focus + 1) % 3
		return state, nil

	case "shift+tab":
		s.focus = (s.focus + 2) % 3
		return state, nil

	case "escape":
		state.Mode = plugin.ModeList
		s.reset()
		return state, nil

	case "enter":
		return state, s.createStash()

	case " ":
		if s.focus == FocusScopes && s.scopeIdx < len(s.scopes) {
			s.scopes[s.scopeIdx].Enabled = !s.scopes[s.scopeIdx].Enabled
		} else if s.focus == FocusOptions && s.optIdx < len(s.options) {
			s.options[s.optIdx].Enabled = !s.options[s.optIdx].Enabled
		}
		return state, nil

	case "j", "down":
		if s.focus == FocusScopes && s.scopeIdx < len(s.scopes)-1 {
			s.scopeIdx++
		} else if s.focus == FocusOptions && s.optIdx < len(s.options)-1 {
			s.optIdx++
		}
		return state, nil

	case "k", "up":
		if s.focus == FocusScopes && s.scopeIdx > 0 {
			s.scopeIdx--
		} else if s.focus == FocusOptions && s.optIdx > 0 {
			s.optIdx--
		}
		return state, nil
	}

	return state, nil
}

// View renders the new stash screen.
func (s *NewStashScreen) View(state plugin.AppState, width, height int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(s.th.FgPrimary())
	dimStyle := lipgloss.NewStyle().Foreground(s.th.FgDimmed())
	greenStyle := lipgloss.NewStyle().Foreground(s.th.SemanticGreen())
	activeStyle := lipgloss.NewStyle().Foreground(s.th.AccentGold())
	errStyle := lipgloss.NewStyle().Foreground(s.th.SemanticRed())

	var b strings.Builder

	b.WriteString("  ")
	b.WriteString(headerStyle.Render("New Stash"))
	b.WriteString("\n\n")

	// Message input.
	label := "  Message: "
	if s.focus == FocusMessage {
		label = activeStyle.Render("  Message: ")
	}
	b.WriteString(label)

	// Render message with cursor.
	if s.focus == FocusMessage {
		before := s.message[:s.cursor]
		after := s.message[s.cursor:]
		cursorChar := " "
		if s.cursor < len(s.message) {
			cursorChar = string(s.message[s.cursor])
			after = after[1:]
		}
		cursorStyle := lipgloss.NewStyle().Reverse(true)
		b.WriteString(before)
		b.WriteString(cursorStyle.Render(cursorChar))
		b.WriteString(after)
	} else {
		if s.message == "" {
			b.WriteString(dimStyle.Render("Describe what you're stashing..."))
		} else {
			b.WriteString(s.message)
		}
	}
	b.WriteString("\n\n")

	// Scope toggles.
	scopeHeader := "  Scope:"
	if s.focus == FocusScopes {
		scopeHeader = activeStyle.Render("  Scope:")
	}
	b.WriteString(scopeHeader)
	b.WriteString("\n")

	for i, scope := range s.scopes {
		check := " "
		if scope.Enabled {
			check = greenStyle.Render("\u2713")
		}
		cursor := "  "
		if s.focus == FocusScopes && i == s.scopeIdx {
			cursor = activeStyle.Render("> ")
		}
		countStr := dimStyle.Render(fmt.Sprintf("(%d files)", scope.Count))
		b.WriteString(fmt.Sprintf("  %s[%s] %s %s\n", cursor, check, scope.Label, countStr))
	}

	b.WriteString("\n")

	// Options.
	optHeader := "  Options:"
	if s.focus == FocusOptions {
		optHeader = activeStyle.Render("  Options:")
	}
	b.WriteString(optHeader)
	b.WriteString("\n")

	for i, opt := range s.options {
		check := " "
		if opt.Enabled {
			check = greenStyle.Render("\u2713")
		}
		cursor := "  "
		if s.focus == FocusOptions && i == s.optIdx {
			cursor = activeStyle.Render("> ")
		}
		b.WriteString(fmt.Sprintf("  %s[%s] %s\n", cursor, check, opt.Label))
	}

	// Footer hints.
	b.WriteString("\n")
	hints := dimStyle.Render("  Tab: next field  Space: toggle  Enter: create  Esc: cancel")
	b.WriteString(hints)
	b.WriteString("\n")

	// Error message if any.
	if s.errMsg != "" {
		b.WriteString("\n")
		b.WriteString("  ")
		b.WriteString(errStyle.Render("Error: " + s.errMsg))
		b.WriteString("\n")
	}

	return b.String()
}

// createStash builds and executes the `git stash push` command.
func (s *NewStashScreen) createStash() tea.Cmd {
	args := s.BuildArgs()

	return func() tea.Msg {
		ctx := context.Background()

		if !s.hasChanges() {
			return StashCreateErrorMsg{Err: fmt.Errorf("no changes to stash")}
		}

		// Patch mode: signal for tea.Exec.
		if s.options[1].Enabled {
			return PatchModeMsg{Args: args}
		}

		_, err := s.git.Run(ctx, args...)
		if err != nil {
			return StashCreateErrorMsg{Err: fmt.Errorf("git stash push: %w", err)}
		}

		s.cache.Invalidate()
		return StashCreatedMsg{}
	}
}

// BuildArgs constructs the `git stash push` argument list from the
// current scope toggles and options.
func (s *NewStashScreen) BuildArgs() []string {
	args := []string{"stash", "push"}

	msg := strings.TrimSpace(s.message)
	if msg != "" {
		args = append(args, "-m", msg)
	}

	stagedEnabled := s.scopes[0].Enabled
	unstagedEnabled := s.scopes[1].Enabled
	untrackedEnabled := s.scopes[2].Enabled

	if stagedEnabled && !unstagedEnabled {
		args = append(args, "--staged")
	}

	if untrackedEnabled {
		args = append(args, "--include-untracked")
	}

	if s.options[0].Enabled {
		args = append(args, "--keep-index")
	}
	if s.options[1].Enabled {
		args = append(args, "--patch")
	}

	return args
}

// hasChanges checks if there are any changes to stash by running git status.
func (s *NewStashScreen) hasChanges() bool {
	ctx := context.Background()
	out, err := s.git.Run(ctx, "status", "--porcelain")
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// LoadFileCounts returns a tea.Cmd that runs `git status` and returns file counts.
func LoadFileCounts(runner plugin.GitRunner) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		out, err := runner.Run(ctx, "status", "--porcelain")
		if err != nil {
			return FileCountsMsg{}
		}

		var staged, unstaged, untracked int
		for line := range strings.SplitSeq(out, "\n") {
			if len(line) < 2 {
				continue
			}
			x := line[0]
			y := line[1]

			switch {
			case x == '?' && y == '?':
				untracked++
			case x != ' ' && x != '?':
				staged++
				if y != ' ' && y != '?' {
					unstaged++
				}
			case y != ' ' && y != '?':
				unstaged++
			}
		}

		return FileCountsMsg{
			Staged:    staged,
			Unstaged:  unstaged,
			Untracked: untracked,
		}
	}
}

// reset clears the screen state for reuse.
func (s *NewStashScreen) reset() {
	s.message = ""
	s.cursor = 0
	s.focus = FocusMessage
	s.scopeIdx = 0
	s.optIdx = 0
	s.errMsg = ""
	s.scopes[0].Enabled = true
	s.scopes[1].Enabled = true
	s.scopes[2].Enabled = false
	s.options[0].Enabled = true
	s.options[1].Enabled = false
}

// ─── Test helpers ───────────────────────────────────────────

// SetMessageForTest sets the message for testing.
func (s *NewStashScreen) SetMessageForTest(msg string) {
	s.message = msg
	s.cursor = len(msg)
}

// SetScopesForTest sets scope toggle states for testing.
func (s *NewStashScreen) SetScopesForTest(staged, unstaged, untracked bool) {
	s.scopes[0].Enabled = staged
	s.scopes[1].Enabled = unstaged
	s.scopes[2].Enabled = untracked
}

// SetOptionsForTest sets option toggle states for testing.
func (s *NewStashScreen) SetOptionsForTest(keepIndex, patchMode bool) {
	s.options[0].Enabled = keepIndex
	s.options[1].Enabled = patchMode
}

// SetFileCountsForTest sets file counts for testing.
func (s *NewStashScreen) SetFileCountsForTest(staged, unstaged, untracked int) {
	s.scopes[0].Count = staged
	s.scopes[1].Count = unstaged
	s.scopes[2].Count = untracked
}

// GetFocusForTest returns the current focus field for testing.
func (s *NewStashScreen) GetFocusForTest() FocusField {
	return s.focus
}

// CycleFocusForTest cycles the focus to the next field.
func (s *NewStashScreen) CycleFocusForTest() {
	s.focus = (s.focus + 1) % 3
}

// MessageForTest returns the current message for testing.
func (s *NewStashScreen) MessageForTest() string {
	return s.message
}
