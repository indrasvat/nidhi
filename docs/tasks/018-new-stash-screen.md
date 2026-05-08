# Task 018: New Stash Creation Screen

## Status: TODO

## Depends On
- 013 (Stash CRUD operations -- `git stash push` primitives)
- 006 (Core model -- AppState, Mode, Stash)

## Parallelizable With
- 015 (Conflict preview plugin)
- 016 (Undo plugin)
- 017 (Rename plugin)

## Problem
Creating a stash with `git stash push` requires users to remember multiple flags (`-m`, `-k`, `-u`, `-S`, `-p`) and type them in a single command line. There is no preview of what will be stashed, no file counts, and no way to toggle options interactively. The result: most users create stashes with useless default messages ("WIP on main: abc1234") because the CLI friction is too high to do better.

nidhi must provide a dedicated New Stash screen with a message-first design (cursor starts in the message field), scope toggles showing live file counts from `git status`, and options for keep-index, patch mode, and auto-export.

## PRD Reference
- Section 6.1, FR-02.4 (New stash -- open New Stash screen)
- Section 10, Screen 6 (New Stash layout and behavior)
- Section 8.2 (Core Interfaces) -- `ScreenProvider` interface
- Section 11 (Keyboard Navigation -- Tab between fields, Enter create, Esc cancel)

## Files to Create
- `internal/ui/screens/newstash.go` -- ScreenProvider for the New Stash screen
- `internal/ui/screens/newstash_test.go` -- unit and integration tests

## Files to Modify
- `internal/core/mode.go` -- ensure `ModeNewStash` exists
- `internal/core/events.go` -- add `StashCreatedMsg`

## Execution Steps

### Step 1: Define the scope model types

```go
package screens

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// ScopeType represents a category of changes that can be stashed.
type ScopeType int

const (
	ScopeStaged    ScopeType = iota
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
	Key     string // The git flag this maps to
	Enabled bool
}

// FocusField tracks which field has keyboard focus.
type FocusField int

const (
	FocusMessage   FocusField = iota
	FocusScopes
	FocusOptions
)
```

### Step 2: Create the New Stash screen in `internal/ui/screens/newstash.go`

```go
// NewStashScreen implements plugin.ScreenProvider for the new stash
// creation screen (PRD §10 Screen 6, FR-02.4).
type NewStashScreen struct {
	git      git.GitRunner
	cache    git.StashCache
	logger   *slog.Logger

	// UI state
	input    textinput.Model
	scopes   []Scope
	options  []StashOption
	focus    FocusField
	scopeIdx int // Which scope is highlighted when focus is on scopes
	optIdx   int // Which option is highlighted when focus is on options
	errMsg   string
}

// NewNewStashScreen creates a new stash screen instance.
func NewNewStashScreen() *NewStashScreen {
	ti := textinput.New()
	ti.Placeholder = "Describe what you're stashing..."
	ti.Focus()
	ti.CharLimit = 200

	return &NewStashScreen{
		input: ti,
		scopes: []Scope{
			{Type: ScopeStaged, Label: "Staged changes", Enabled: true},
			{Type: ScopeUnstaged, Label: "Unstaged changes", Enabled: true},
			{Type: ScopeUntracked, Label: "Untracked files", Enabled: false},
		},
		options: []StashOption{
			{Label: "Keep index (don't unstage staged files)", Key: "--keep-index", Enabled: true},
			{Label: "Patch mode (select hunks)", Key: "--patch", Enabled: false},
			{Label: "Auto-export after creation", Key: "", Enabled: false}, // Not a git flag
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
		{
			ID:   "newstash",
			Name: "New Stash",
			Mode: core.ModeNewStash,
		},
	}
}

// Update handles messages when the new stash screen is active.
func (s *NewStashScreen) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return s.handleKey(msg, state)

	case FileCountsMsg:
		s.scopes[0].Count = msg.Staged
		s.scopes[1].Count = msg.Unstaged
		s.scopes[2].Count = msg.Untracked
		return state, nil

	case StashCreatedMsg:
		state.Mode = core.ModeList
		s.reset()
		return state, nil

	case StashCreateErrorMsg:
		s.errMsg = msg.Err.Error()
		return state, nil
	}

	// Pass to text input if focused.
	if s.focus == FocusMessage {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return state, cmd
	}

	return state, nil
}

func (s *NewStashScreen) handleKey(msg tea.KeyPressMsg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg.Text {
	case "tab":
		// Cycle focus: Message -> Scopes -> Options -> Message.
		s.focus = (s.focus + 1) % 3
		if s.focus == FocusMessage {
			s.input.Focus()
		} else {
			s.input.Blur()
		}
		return state, nil

	case "shift+tab":
		// Reverse cycle.
		s.focus = (s.focus + 2) % 3
		if s.focus == FocusMessage {
			s.input.Focus()
		} else {
			s.input.Blur()
		}
		return state, nil

	case "escape":
		state.Mode = core.ModeList
		s.reset()
		return state, nil

	case "enter":
		if s.focus == FocusMessage || s.focus == FocusOptions {
			return state, s.createStash()
		}
		return state, nil

	case " ":
		// Toggle when focus is on scopes or options.
		if s.focus == FocusScopes && s.scopeIdx < len(s.scopes) {
			s.scopes[s.scopeIdx].Enabled = !s.scopes[s.scopeIdx].Enabled
		} else if s.focus == FocusOptions && s.optIdx < len(s.options) {
			s.options[s.optIdx].Enabled = !s.options[s.optIdx].Enabled
		}
		return state, nil

	case "j", "down":
		if s.focus == FocusScopes {
			if s.scopeIdx < len(s.scopes)-1 {
				s.scopeIdx++
			}
		} else if s.focus == FocusOptions {
			if s.optIdx < len(s.options)-1 {
				s.optIdx++
			}
		}
		return state, nil

	case "k", "up":
		if s.focus == FocusScopes {
			if s.scopeIdx > 0 {
				s.scopeIdx--
			}
		} else if s.focus == FocusOptions {
			if s.optIdx > 0 {
				s.optIdx--
			}
		}
		return state, nil
	}

	return state, nil
}

// View renders the new stash screen.
func (s *NewStashScreen) View(state core.AppState, width, height int) string {
	var b strings.Builder

	b.WriteString("  New Stash\n\n")

	// Message input.
	b.WriteString("  Message: ")
	b.WriteString(s.input.View())
	b.WriteString("\n\n")

	// Scope toggles.
	b.WriteString("  Scope:\n")
	for i, scope := range s.scopes {
		check := " "
		if scope.Enabled {
			check = "\u2713" // checkmark
		}
		cursor := "  "
		if s.focus == FocusScopes && i == s.scopeIdx {
			cursor = "> "
		}
		line := fmt.Sprintf("  %s[%s] %s (%d files)", cursor, check, scope.Label, scope.Count)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Options.
	b.WriteString("  Options:\n")
	for i, opt := range s.options {
		check := " "
		if opt.Enabled {
			check = "\u2713"
		}
		cursor := "  "
		if s.focus == FocusOptions && i == s.optIdx {
			cursor = "> "
		}
		line := fmt.Sprintf("  %s[%s] %s", cursor, check, opt.Label)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Error message if any.
	if s.errMsg != "" {
		b.WriteString("\n")
		b.WriteString("  Error: " + s.errMsg + "\n")
	}

	return b.String()
}

// createStash builds and executes the `git stash push` command.
func (s *NewStashScreen) createStash() tea.Cmd {
	args := s.BuildArgs()

	return func() tea.Msg {
		ctx := context.Background()

		// Check if there are any changes to stash.
		if !s.hasChanges() {
			return StashCreateErrorMsg{Err: fmt.Errorf("no changes to stash")}
		}

		// Patch mode: use tea.Exec to open interactive hunk picker.
		if s.options[1].Enabled { // Patch mode
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

	// Message.
	msg := strings.TrimSpace(s.input.Value())
	if msg != "" {
		args = append(args, "-m", msg)
	}

	// Scope flags.
	stagedEnabled := s.scopes[0].Enabled
	unstagedEnabled := s.scopes[1].Enabled
	untrackedEnabled := s.scopes[2].Enabled

	// --staged: only stash staged changes.
	if stagedEnabled && !unstagedEnabled {
		args = append(args, "--staged")
	}

	// --include-untracked: include untracked files.
	if untrackedEnabled {
		args = append(args, "--include-untracked")
	}

	// Options.
	if s.options[0].Enabled { // Keep index
		args = append(args, "--keep-index")
	}
	if s.options[1].Enabled { // Patch mode
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

// LoadFileCounts runs `git status` and returns file counts per scope.
func LoadFileCounts(ctx context.Context, runner git.GitRunner) tea.Cmd {
	return func() tea.Msg {
		out, err := runner.Run(ctx, "status", "--porcelain")
		if err != nil {
			return FileCountsMsg{} // Zeros on error
		}

		var staged, unstaged, untracked int
		for _, line := range strings.Split(out, "\n") {
			if len(line) < 2 {
				continue
			}
			x := line[0] // Index status
			y := line[1] // Working tree status

			switch {
			case x == '?' && y == '?':
				untracked++
			case x != ' ' && x != '?':
				staged++
				if y != ' ' && y != '?' {
					unstaged++ // Same file has both staged and unstaged changes
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
	s.input.SetValue("")
	s.input.Focus()
	s.focus = FocusMessage
	s.scopeIdx = 0
	s.optIdx = 0
	s.errMsg = ""
	s.scopes[0].Enabled = true
	s.scopes[1].Enabled = true
	s.scopes[2].Enabled = false
	s.options[0].Enabled = true
	s.options[1].Enabled = false
	s.options[2].Enabled = false
}

// Message types for the new stash screen.

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
```

### Step 3: Create tests in `internal/ui/screens/newstash_test.go`

```go
package screens_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/ui/screens"
)

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\noutput: %s", name, args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.name", "test")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	writeFile(t, dir, "README.md", "# test\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")
	return dir
}

func stashCount(t *testing.T, dir string) int {
	t.Helper()
	out := strings.TrimSpace(run(t, dir, "git", "stash", "list"))
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func TestBuildArgs_DefaultState(t *testing.T) {
	s := screens.NewNewStashScreen()
	args := s.BuildArgs()

	// Default: staged + unstaged enabled, keep-index enabled.
	// With both staged and unstaged, no --staged flag needed (it's the default).
	expected := []string{"stash", "push", "--keep-index"}

	if len(args) != len(expected) {
		t.Fatalf("args = %v, want %v", args, expected)
	}

	for i, a := range args {
		if a != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, a, expected[i])
		}
	}
}

func TestBuildArgs_AllFlags(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		staged      bool
		unstaged    bool
		untracked   bool
		keepIndex   bool
		patchMode   bool
		wantContains []string
		wantNotContain []string
	}{
		{
			name:      "message only",
			message:   "my stash",
			staged:    true,
			unstaged:  true,
			keepIndex: false,
			wantContains: []string{"-m", "my stash"},
		},
		{
			name:      "staged only",
			message:   "staged stuff",
			staged:    true,
			unstaged:  false,
			keepIndex: false,
			wantContains: []string{"--staged", "-m", "staged stuff"},
		},
		{
			name:      "include untracked",
			staged:    true,
			unstaged:  true,
			untracked: true,
			keepIndex: false,
			wantContains: []string{"--include-untracked"},
		},
		{
			name:      "keep index enabled",
			staged:    true,
			unstaged:  true,
			keepIndex: true,
			wantContains: []string{"--keep-index"},
		},
		{
			name:      "patch mode",
			staged:    true,
			unstaged:  true,
			patchMode: true,
			wantContains: []string{"--patch"},
		},
		{
			name:      "all disabled except staged",
			staged:    true,
			unstaged:  false,
			keepIndex: false,
			wantContains:    []string{"--staged"},
			wantNotContain: []string{"--keep-index", "--include-untracked", "--patch"},
		},
		{
			name:      "no message",
			staged:    true,
			unstaged:  true,
			keepIndex: false,
			wantNotContain: []string{"-m"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := screens.NewNewStashScreen()

			// Set message.
			if tt.message != "" {
				s.SetMessageForTest(tt.message)
			}

			// Set scopes.
			s.SetScopesForTest(tt.staged, tt.unstaged, tt.untracked)

			// Set options.
			s.SetOptionsForTest(tt.keepIndex, tt.patchMode)

			args := s.BuildArgs()
			argsStr := strings.Join(args, " ")

			for _, want := range tt.wantContains {
				found := false
				for _, a := range args {
					if a == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("args %q missing %q", argsStr, want)
				}
			}

			for _, notWant := range tt.wantNotContain {
				for _, a := range args {
					if a == notWant {
						t.Errorf("args %q should not contain %q", argsStr, notWant)
					}
				}
			}
		})
	}
}

func TestBuildArgs_TabNavigation(t *testing.T) {
	s := screens.NewNewStashScreen()

	// Initial focus should be on message.
	if s.GetFocusForTest() != screens.FocusMessage {
		t.Errorf("initial focus = %d, want FocusMessage", s.GetFocusForTest())
	}

	// Tab cycles: Message -> Scopes -> Options -> Message.
	s.CycleFocusForTest() // -> Scopes
	if s.GetFocusForTest() != screens.FocusScopes {
		t.Errorf("after 1 tab: focus = %d, want FocusScopes", s.GetFocusForTest())
	}

	s.CycleFocusForTest() // -> Options
	if s.GetFocusForTest() != screens.FocusOptions {
		t.Errorf("after 2 tabs: focus = %d, want FocusOptions", s.GetFocusForTest())
	}

	s.CycleFocusForTest() // -> Message (wraps)
	if s.GetFocusForTest() != screens.FocusMessage {
		t.Errorf("after 3 tabs: focus = %d, want FocusMessage", s.GetFocusForTest())
	}
}

func TestNewStashScreen_CreateStash_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create some changes to stash.
	writeFile(t, dir, "feature.go", "package main\n\nfunc feature() {}\n")
	run(t, dir, "git", "add", ".")

	// Run git stash push directly with the args we would build.
	s := screens.NewNewStashScreen()
	s.SetMessageForTest("test stash from screen")
	s.SetScopesForTest(true, true, false)
	s.SetOptionsForTest(false, false) // No keep-index

	args := s.BuildArgs()

	// Execute in the test repo.
	gitArgs := append([]string{}, args...)
	run(t, dir, "git", gitArgs...)

	// Verify stash created.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	// Verify message.
	out := run(t, dir, "git", "stash", "list")
	if !strings.Contains(out, "test stash from screen") {
		t.Errorf("stash list = %q, want to contain message", out)
	}
}

func TestNewStashScreen_ScopeTogglesShowCorrectCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create staged, unstaged, and untracked files.
	writeFile(t, dir, "staged1.go", "package main\n")
	writeFile(t, dir, "staged2.go", "package main\n")
	run(t, dir, "git", "add", "staged1.go", "staged2.go")

	writeFile(t, dir, "README.md", "# modified\n") // Already tracked, unstaged

	writeFile(t, dir, "untracked1.txt", "untracked\n")
	writeFile(t, dir, "untracked2.txt", "untracked\n")
	writeFile(t, dir, "untracked3.txt", "untracked\n")

	// Parse git status --porcelain to count files.
	runner := git.NewRunner(dir)
	out, err := runner.Run(context.Background(), "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status: %v", err)
	}

	var staged, unstaged, untracked int
	for _, line := range strings.Split(out, "\n") {
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

	if staged != 2 {
		t.Errorf("staged = %d, want 2", staged)
	}
	if unstaged != 1 {
		t.Errorf("unstaged = %d, want 1 (modified README)", unstaged)
	}
	if untracked != 3 {
		t.Errorf("untracked = %d, want 3", untracked)
	}
}

func TestNewStashScreen_NoChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Clean working tree -- no changes to stash.
	runner := git.NewRunner(dir)
	out, err := runner.Run(context.Background(), "status", "--porcelain")
	if err != nil {
		t.Fatalf("git status: %v", err)
	}

	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected clean working tree, got: %q", out)
	}

	// Attempting to stash should fail.
	_, err = runner.Run(context.Background(), "stash", "push", "-m", "empty")
	if err == nil {
		t.Error("expected error stashing with no changes")
	}
}

func TestNewStashScreen_KeepIndex_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Stage a file.
	writeFile(t, dir, "keep.go", "package main\n")
	run(t, dir, "git", "add", "keep.go")

	// Stash with --keep-index.
	run(t, dir, "git", "stash", "push", "-m", "keep index test", "--keep-index")

	// Verify: staged file should still be in the index after stash.
	statusOut := run(t, dir, "git", "status", "--porcelain")
	if !strings.Contains(statusOut, "keep.go") {
		t.Errorf("expected keep.go to remain in index with --keep-index, got: %q", statusOut)
	}

	// Verify: stash was created.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}
}

func TestNewStashScreen_StagedOnly_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create staged and unstaged changes.
	writeFile(t, dir, "staged.go", "package main // staged\n")
	run(t, dir, "git", "add", "staged.go")
	writeFile(t, dir, "README.md", "# unstaged modification\n")

	// Stash with --staged (only staged changes).
	run(t, dir, "git", "stash", "push", "-m", "staged only", "--staged")

	// Verify: unstaged changes should still be present.
	statusOut := run(t, dir, "git", "status", "--porcelain")
	if !strings.Contains(statusOut, "README.md") {
		t.Errorf("expected unstaged README.md to remain, got: %q", statusOut)
	}

	// Verify: stash was created.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}
}

func TestNewStashScreen_ViewRendering(t *testing.T) {
	s := screens.NewNewStashScreen()
	s.SetMessageForTest("my feature stash")

	// Set some file counts.
	s.SetFileCountsForTest(3, 1, 2)

	output := s.ViewForTest(80, 24)

	tests := []struct {
		name     string
		contains string
	}{
		{"title", "New Stash"},
		{"message field", "my feature stash"},
		{"scope section", "Scope:"},
		{"staged count", "3 files"},
		{"unstaged count", "1 files"},
		{"untracked count", "2 files"},
		{"options section", "Options:"},
		{"keep index option", "Keep index"},
		{"patch mode option", "Patch mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(output, tt.contains) {
				t.Errorf("output missing %q:\n%s", tt.contains, output)
			}
		})
	}
}
```

### Step 4: Add test helper methods to the screen

Add to `internal/ui/screens/newstash.go` (exported for testing only):

```go
// --- Test helpers (exported for testing only) ---

// SetMessageForTest sets the text input value for testing.
func (s *NewStashScreen) SetMessageForTest(msg string) {
	s.input.SetValue(msg)
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

// ViewForTest renders the view for testing without needing AppState.
func (s *NewStashScreen) ViewForTest(width, height int) string {
	return s.View(core.AppState{}, width, height)
}
```

### Step 5: Ensure ModeNewStash is defined

Add to `internal/core/mode.go` (if not already present):

```go
const (
	ModeList     Mode = iota
	ModePreview
	ModeDetail
	ModeConflict
	ModeNewStash // New stash creation screen
)
```

### Step 6: Verify

```bash
# Unit: flag construction from toggles
go test -v -run TestBuildArgs ./internal/ui/screens/...

# Unit: Tab navigation
go test -v -run TestBuildArgs_TabNavigation ./internal/ui/screens/...

# Unit: view rendering
go test -v -run TestNewStashScreen_ViewRendering ./internal/ui/screens/...

# Integration: create stash with message and flags
go test -v -run TestNewStashScreen_CreateStash_Integration ./internal/ui/screens/...

# Integration: file count detection
go test -v -run TestNewStashScreen_ScopeTogglesShowCorrectCounts ./internal/ui/screens/...

# Integration: no changes edge case
go test -v -run TestNewStashScreen_NoChanges ./internal/ui/screens/...

# Integration: --keep-index behavior
go test -v -run TestNewStashScreen_KeepIndex_Integration ./internal/ui/screens/...

# Integration: --staged only behavior
go test -v -run TestNewStashScreen_StagedOnly_Integration ./internal/ui/screens/...

# Full CI
make ci
```

## Verification

### Functional
```bash
# Unit: default args (staged + unstaged + keep-index)
go test -v -run TestBuildArgs_DefaultState ./internal/ui/screens/...
# Expected: ["stash", "push", "--keep-index"]

# Unit: all flag combinations via table-driven tests
go test -v -run TestBuildArgs_AllFlags ./internal/ui/screens/...
# Expected: correct flags for each combination

# Unit: Tab navigation cycles through 3 fields
go test -v -run TestBuildArgs_TabNavigation ./internal/ui/screens/...
# Expected: Message -> Scopes -> Options -> Message

# Unit: view contains all expected sections
go test -v -run TestNewStashScreen_ViewRendering ./internal/ui/screens/...
# Expected: title, message, scopes with counts, options

# Integration: stash created with correct message
go test -v -run TestNewStashScreen_CreateStash_Integration ./internal/ui/screens/...
# Expected: 1 stash with "test stash from screen" message

# Integration: scope counts match git status
go test -v -run TestNewStashScreen_ScopeTogglesShowCorrectCounts ./internal/ui/screens/...
# Expected: 2 staged, 1 unstaged, 3 untracked

# Integration: error on empty working tree
go test -v -run TestNewStashScreen_NoChanges ./internal/ui/screens/...
# Expected: error from git stash push

# Integration: --keep-index preserves staged files
go test -v -run TestNewStashScreen_KeepIndex_Integration ./internal/ui/screens/...
# Expected: staged file remains in index after stash

# Full CI
make ci
```

## Completion Criteria
1. `n` key opens the New Stash screen (ScreenProvider registered for ModeNewStash)
2. Cursor starts in the message field (message-first design per PRD Screen 6)
3. Scope toggles show correct file counts from `git status --porcelain`
4. Tab navigates between message, scopes, and options fields
5. Space toggles scope/option when focused
6. Enter creates stash: constructs `git stash push` with correct flags from toggles
7. `--staged` flag used when only Staged scope is enabled
8. `--include-untracked` flag used when Untracked scope is enabled
9. `--keep-index` flag used when Keep Index option is enabled
10. Patch mode triggers `tea.Exec` for interactive hunk picker
11. Esc cancels and returns to LIST mode
12. Error shown when no changes to stash (edge case)
13. All tests pass including integration tests with real git repos using `t.TempDir()`
14. `make ci` passes

## Commit
```
feat(newstash): add new stash creation screen with scope toggles

Implement FR-02.4 new stash screen with message-first design,
scope toggles (staged/unstaged/untracked with live file counts),
and options (keep-index, patch mode). Tab navigation between
fields, Enter creates stash with correct git flags.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.1 (FR-02.4), 10 (Screen 6), 11 (keyboard nav)
4. Read tasks 006 and 013 to understand dependencies
5. Execute steps 1-6 in order
6. Verify all functional checks pass
7. Update this file (Status: DONE) + `docs/PROGRESS.md`
8. Commit with the message above + move to next task
