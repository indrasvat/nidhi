package screens

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func newTestNewStash() *NewStashScreen {
	return NewNewStashScreen(theme.NewAgni())
}

func TestBuildArgs_DefaultState(t *testing.T) {
	s := newTestNewStash()
	args := s.BuildArgs()

	// Default: staged + unstaged enabled, keep-index enabled.
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
		name           string
		message        string
		staged         bool
		unstaged       bool
		untracked      bool
		keepIndex      bool
		patchMode      bool
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "message only",
			message:      "my stash",
			staged:       true,
			unstaged:     true,
			keepIndex:    false,
			wantContains: []string{"-m", "my stash"},
		},
		{
			name:         "staged only",
			message:      "staged stuff",
			staged:       true,
			unstaged:     false,
			keepIndex:    false,
			wantContains: []string{"--staged", "-m", "staged stuff"},
		},
		{
			name:         "include untracked",
			staged:       true,
			unstaged:     true,
			untracked:    true,
			keepIndex:    false,
			wantContains: []string{"--include-untracked"},
		},
		{
			name:         "keep index enabled",
			staged:       true,
			unstaged:     true,
			keepIndex:    true,
			wantContains: []string{"--keep-index"},
		},
		{
			name:         "patch mode",
			staged:       true,
			unstaged:     true,
			patchMode:    true,
			wantContains: []string{"--patch"},
		},
		{
			name:           "all disabled except staged",
			staged:         true,
			unstaged:       false,
			keepIndex:      false,
			wantContains:   []string{"--staged"},
			wantNotContain: []string{"--keep-index", "--include-untracked", "--patch"},
		},
		{
			name:           "no message",
			staged:         true,
			unstaged:       true,
			keepIndex:      false,
			wantNotContain: []string{"-m"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestNewStash()
			if tt.message != "" {
				s.SetMessageForTest(tt.message)
			}
			s.SetScopesForTest(tt.staged, tt.unstaged, tt.untracked)
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

func TestTabNavigation(t *testing.T) {
	s := newTestNewStash()

	if s.GetFocusForTest() != FocusMessage {
		t.Errorf("initial focus = %d, want FocusMessage", s.GetFocusForTest())
	}

	s.CycleFocusForTest()
	if s.GetFocusForTest() != FocusScopes {
		t.Errorf("after 1 tab: focus = %d, want FocusScopes", s.GetFocusForTest())
	}

	s.CycleFocusForTest()
	if s.GetFocusForTest() != FocusOptions {
		t.Errorf("after 2 tabs: focus = %d, want FocusOptions", s.GetFocusForTest())
	}

	s.CycleFocusForTest()
	if s.GetFocusForTest() != FocusMessage {
		t.Errorf("after 3 tabs: focus = %d, want FocusMessage (wrap)", s.GetFocusForTest())
	}
}

func TestNewStashScreen_ViewRendering(t *testing.T) {
	s := newTestNewStash()
	s.SetMessageForTest("my feature stash")
	s.SetFileCountsForTest(3, 1, 2)

	output := s.View(core.AppState{}, 80, 24)

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
		{"keep index", "Keep index"},
		{"patch mode", "Patch mode"},
		{"footer hints", "Tab:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(output, tt.contains) {
				t.Errorf("output missing %q:\n%s", tt.contains, output)
			}
		})
	}
}

func TestNewStashScreen_MessageInput(t *testing.T) {
	s := newTestNewStash()

	// Type some characters.
	state := core.AppState{Mode: core.ModeNewStash}

	// Type "hello"
	for _, ch := range "hello" {
		state, _ = s.handleKey(tea.KeyPressMsg{Text: string(ch)}, state)
	}
	if s.MessageForTest() != "hello" {
		t.Errorf("message = %q, want 'hello'", s.MessageForTest())
	}

	// Backspace removes last char.
	state, _ = s.handleKey(tea.KeyPressMsg{Text: "backspace"}, state)
	if s.MessageForTest() != "hell" {
		t.Errorf("message after backspace = %q, want 'hell'", s.MessageForTest())
	}

	// Left moves cursor, then type inserts.
	state, _ = s.handleKey(tea.KeyPressMsg{Text: "left"}, state)
	state, _ = s.handleKey(tea.KeyPressMsg{Text: "left"}, state)
	state, _ = s.handleKey(tea.KeyPressMsg{Text: "X"}, state)
	if s.MessageForTest() != "heXll" {
		t.Errorf("message after insert = %q, want 'heXll'", s.MessageForTest())
	}
	_ = state
}

func TestNewStashScreen_EscReturnsToList(t *testing.T) {
	s := newTestNewStash()
	state := core.AppState{Mode: core.ModeNewStash}

	state, _ = s.handleKey(tea.KeyPressMsg{Text: "escape"}, state)
	if state.Mode != core.ModeList {
		t.Errorf("mode after Esc = %v, want ModeList", state.Mode)
	}
}

func TestNewStashScreen_ScopeToggle(t *testing.T) {
	s := newTestNewStash()
	state := core.AppState{Mode: core.ModeNewStash}

	// Move to scopes focus.
	s.focus = FocusScopes
	s.scopeIdx = 2 // Untracked (initially disabled)

	if s.scopes[2].Enabled {
		t.Fatal("untracked should be disabled by default")
	}

	// Toggle with space.
	state, _ = s.handleKey(tea.KeyPressMsg{Text: " "}, state)
	if !s.scopes[2].Enabled {
		t.Error("untracked should be enabled after space toggle")
	}

	// Toggle again.
	state, _ = s.handleKey(tea.KeyPressMsg{Text: " "}, state)
	if s.scopes[2].Enabled {
		t.Error("untracked should be disabled after second toggle")
	}
	_ = state
}

func TestNewStashScreen_OptionToggle(t *testing.T) {
	s := newTestNewStash()
	state := core.AppState{Mode: core.ModeNewStash}

	s.focus = FocusOptions
	s.optIdx = 1 // Patch mode (initially disabled)

	if s.options[1].Enabled {
		t.Fatal("patch mode should be disabled by default")
	}

	state, _ = s.handleKey(tea.KeyPressMsg{Text: " "}, state)
	if !s.options[1].Enabled {
		t.Error("patch mode should be enabled after toggle")
	}
	_ = state
}

func TestNewStashScreen_CursorNavigation(t *testing.T) {
	s := newTestNewStash()
	state := core.AppState{Mode: core.ModeNewStash}

	// In scopes focus, j/k navigate.
	s.focus = FocusScopes
	s.scopeIdx = 0

	state, _ = s.handleKey(tea.KeyPressMsg{Text: "j"}, state)
	if s.scopeIdx != 1 {
		t.Errorf("scopeIdx after j = %d, want 1", s.scopeIdx)
	}

	state, _ = s.handleKey(tea.KeyPressMsg{Text: "j"}, state)
	if s.scopeIdx != 2 {
		t.Errorf("scopeIdx after j = %d, want 2", s.scopeIdx)
	}

	// Should clamp at end.
	state, _ = s.handleKey(tea.KeyPressMsg{Text: "j"}, state)
	if s.scopeIdx != 2 {
		t.Errorf("scopeIdx after j at end = %d, want 2", s.scopeIdx)
	}

	state, _ = s.handleKey(tea.KeyPressMsg{Text: "k"}, state)
	if s.scopeIdx != 1 {
		t.Errorf("scopeIdx after k = %d, want 1", s.scopeIdx)
	}
	_ = state
}

func TestNewStashScreen_Reset(t *testing.T) {
	s := newTestNewStash()
	s.SetMessageForTest("some message")
	s.focus = FocusOptions
	s.scopeIdx = 2
	s.optIdx = 1
	s.errMsg = "some error"
	s.scopes[0].Enabled = false
	s.scopes[2].Enabled = true
	s.options[0].Enabled = false
	s.options[1].Enabled = true

	s.reset()

	if s.message != "" {
		t.Errorf("message after reset = %q, want empty", s.message)
	}
	if s.focus != FocusMessage {
		t.Errorf("focus after reset = %d, want FocusMessage", s.focus)
	}
	if !s.scopes[0].Enabled || !s.scopes[1].Enabled || s.scopes[2].Enabled {
		t.Error("scopes not reset to defaults")
	}
	if !s.options[0].Enabled || s.options[1].Enabled {
		t.Error("options not reset to defaults")
	}
	if s.errMsg != "" {
		t.Errorf("errMsg after reset = %q", s.errMsg)
	}
}

func TestNewStashScreen_FileCountsMsg(t *testing.T) {
	s := newTestNewStash()
	state := core.AppState{Mode: core.ModeNewStash}

	msg := FileCountsMsg{Staged: 5, Unstaged: 3, Untracked: 7}
	state, _ = s.Update(msg, state)

	if s.scopes[0].Count != 5 {
		t.Errorf("staged count = %d, want 5", s.scopes[0].Count)
	}
	if s.scopes[1].Count != 3 {
		t.Errorf("unstaged count = %d, want 3", s.scopes[1].Count)
	}
	if s.scopes[2].Count != 7 {
		t.Errorf("untracked count = %d, want 7", s.scopes[2].Count)
	}
	_ = state
}

func TestNewStashScreen_StashCreatedMsgRefreshesState(t *testing.T) {
	s := newTestNewStash()
	s.SetMessageForTest("pending work")
	state := core.AppState{
		Mode:   core.ModeNewStash,
		Cursor: 4,
		Stashes: []core.Stash{
			{Index: 0, Message: "old"},
		},
	}
	fresh := []core.Stash{
		{Index: 0, SHA: "new", Message: "new stash"},
		{Index: 1, SHA: "old", Message: "old stash"},
	}

	newState, cmd := s.Update(StashCreatedMsg{Stashes: fresh}, state)
	if cmd != nil {
		t.Fatal("StashCreatedMsg should not return a command")
	}
	if newState.Mode != core.ModeList {
		t.Fatalf("mode = %v, want %v", newState.Mode, core.ModeList)
	}
	if len(newState.Stashes) != len(fresh) {
		t.Fatalf("stashes len = %d, want %d", len(newState.Stashes), len(fresh))
	}
	if newState.Stashes[0].Message != "new stash" {
		t.Fatalf("first stash = %q, want %q", newState.Stashes[0].Message, "new stash")
	}
	if newState.Cursor != len(fresh)-1 {
		t.Fatalf("cursor = %d, want %d", newState.Cursor, len(fresh)-1)
	}
	if s.MessageForTest() != "" {
		t.Fatalf("message was not reset: %q", s.MessageForTest())
	}
}

func TestNewStashScreen_CreateStashReloadsCache(t *testing.T) {
	s := newTestNewStash()
	fresh := []core.Stash{{Index: 0, SHA: "abc", Message: "fresh stash"}}
	cache := &newStashTestCache{stashes: fresh}
	runner := &newStashTestRunner{
		outputs: map[string]string{
			"status --porcelain":      " M file.go\n",
			"stash push --keep-index": "",
		},
	}
	s.git = runner
	s.cache = cache

	msg := s.createStash()()
	created, ok := msg.(StashCreatedMsg)
	if !ok {
		t.Fatalf("message = %T, want StashCreatedMsg", msg)
	}
	if !cache.invalidated {
		t.Fatal("cache was not invalidated")
	}
	if !cache.listed {
		t.Fatal("cache.List was not called after create")
	}
	if len(created.Stashes) != 1 || created.Stashes[0].Message != "fresh stash" {
		t.Fatalf("created stashes = %#v", created.Stashes)
	}
}

func TestNewStashScreen_EnterCreatesCmd(t *testing.T) {
	s := newTestNewStash()
	// Need git and cache for createStash.
	// Without them, the cmd will panic — just verify handleKey returns non-nil.
	// We test the integration separately.
	s.git = nil
	s.cache = nil

	state := core.AppState{Mode: core.ModeNewStash}

	// From message focus, enter should attempt create.
	_, cmd := s.handleKey(tea.KeyPressMsg{Text: "enter"}, state)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter in message focus")
	}
}

type newStashTestRunner struct {
	outputs map[string]string
	calls   []string
}

func (r *newStashTestRunner) Run(_ context.Context, args ...string) (string, error) {
	key := strings.Join(args, " ")
	r.calls = append(r.calls, key)
	return r.outputs[key], nil
}

func (r *newStashTestRunner) RunLines(ctx context.Context, args ...string) ([]string, error) {
	out, err := r.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(strings.TrimRight(out, "\n"), "\n"), nil
}

func (r *newStashTestRunner) RunExitCode(ctx context.Context, args ...string) (string, int, error) {
	out, err := r.Run(ctx, args...)
	if err != nil {
		return out, 1, err
	}
	return out, 0, nil
}

type newStashTestCache struct {
	stashes     []core.Stash
	invalidated bool
	listed      bool
}

func (c *newStashTestCache) List(_ context.Context) ([]core.Stash, error) {
	c.listed = true
	return c.stashes, nil
}

func (c *newStashTestCache) Diff(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (c *newStashTestCache) Invalidate() {
	c.invalidated = true
}
