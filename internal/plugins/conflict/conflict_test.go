package conflict_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/conflict"
	"github.com/indrasvat/nidhi/internal/ui/icons"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── Test helpers ───────────────────────────────────────────

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+dir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
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
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.name", "test")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	writeFile(t, dir, "README.md", "# test\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "initial commit")
	return dir
}

// ─── Integration tests ─────────────────────────────────────

func TestConflictPlugin_BeforeApply_ConflictDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	writeFile(t, dir, "config.go", "package main\n\nvar retries = 3\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add config")

	writeFile(t, dir, "config.go", "package main\n\nvar retries = 10\n")
	gitCmd(t, dir, "stash", "push", "-m", "bump to 10")

	writeFile(t, dir, "config.go", "package main\n\nvar retries = 5\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "set to 5")

	sha := gitCmd(t, dir, "rev-parse", "stash@{0}")

	runner := git.NewDefaultRunner(dir, nil)
	result, err := git.RunMergeTree(context.Background(), runner, sha)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	if !result.HasConflicts {
		t.Fatal("expected conflicts, got clean merge")
	}

	var foundConflicted bool
	for _, f := range result.Files {
		if f.Path == "config.go" && f.Status == git.FileStatusConflicted {
			foundConflicted = true
		}
	}
	if !foundConflicted {
		t.Error("expected config.go to be marked as conflicted")
	}
}

func TestConflictPlugin_BeforeApply_CleanMerge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add main")

	writeFile(t, dir, "other.go", "package main\n\nfunc other() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "other file")

	sha := gitCmd(t, dir, "rev-parse", "stash@{0}")

	runner := git.NewDefaultRunner(dir, nil)
	result, err := git.RunMergeTree(context.Background(), runner, sha)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	if result.HasConflicts {
		t.Fatal("expected clean merge, got conflicts")
	}
}

func TestConflictPlugin_UntrackedCollision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	writeFile(t, dir, "collision.txt", "from stash\n")
	gitCmd(t, dir, "stash", "push", "--include-untracked", "-m", "with untracked")

	writeFile(t, dir, "collision.txt", "already here\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add collision.txt")

	sha := gitCmd(t, dir, "rev-parse", "stash@{0}")

	runner := git.NewDefaultRunner(dir, nil)
	collisions, err := git.CheckUntrackedCollisions(context.Background(), runner, sha)
	if err != nil {
		t.Fatalf("CheckUntrackedCollisions: %v", err)
	}

	found := false
	for _, c := range collisions {
		if c.Path == "collision.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected collision.txt in collisions, got %v", collisions)
	}
}

func TestConflictPlugin_PluginInterface(t *testing.T) {
	p := conflict.New()
	if p.ID() != "conflict" {
		t.Errorf("ID() = %q, want %q", p.ID(), "conflict")
	}
	if p.Name() != "Conflict Preview" {
		t.Errorf("Name() = %q, want %q", p.Name(), "Conflict Preview")
	}

	screens := p.Screens()
	if len(screens) != 1 {
		t.Fatalf("Screens() returned %d, want 1", len(screens))
	}
	if screens[0].Name != "Conflict Preview" {
		t.Errorf("Screen name = %q", screens[0].Name)
	}
}

func TestConflictPlugin_ViewNoData(t *testing.T) {
	p := conflict.New()
	state := plugin.AppState{}
	view := p.View(state, 80, 24)
	if !strings.Contains(view, "No conflict data") {
		t.Errorf("expected 'No conflict data' message, got: %s", view)
	}
}

// ─── Screen rendering tests ────────────────────────────────

func TestRenderConflictScreen_Icons(t *testing.T) {
	result := &git.MergeTreeResult{
		HasConflicts: true,
		TreeSHA:      "abc123",
		Files: []git.MergeTreeFile{
			{Path: "clean.go", Status: git.FileStatusClean},
			{Path: "broken.go", Status: git.FileStatusConflicted},
		},
		UntrackedCollisions: []git.UntrackedCollision{
			{Path: "extra.txt"},
		},
	}

	stash := &plugin.Stash{
		Index:   0,
		SHA:     "abc123",
		Message: "test stash",
	}

	output := conflict.RenderConflictScreen(result, stash, nil, icons.ModeASCII, 0, 80, 24)

	tests := []struct {
		name     string
		contains string
	}{
		{"clean icon", icons.Clean.ASCII},
		{"conflict icon", icons.Conflict.ASCII},
		{"untracked collision icon", "\u26A0"},
		{"clean file path", "clean.go"},
		{"conflicted file path", "broken.go"},
		{"untracked collision path", "extra.txt"},
		{"clean label", "clean apply"},
		{"conflict label", "conflict"},
		{"untracked label", "untracked collision"},
		{"stash header", "stash@{0}"},
		{"stash message", "test stash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(output, tt.contains) {
				t.Errorf("output missing %q:\n%s", tt.contains, output)
			}
		})
	}
}

func TestRenderConflictScreen_WithTheme(t *testing.T) {
	result := &git.MergeTreeResult{
		HasConflicts: true,
		TreeSHA:      "abc123",
		Files: []git.MergeTreeFile{
			{Path: "file.go", Status: git.FileStatusConflicted},
		},
	}

	stash := &plugin.Stash{Index: 1, SHA: "def456", Message: "themed"}

	th := theme.NewAgni()
	output := conflict.RenderConflictScreen(result, stash, th, icons.ModeASCII, 0, 80, 24)

	// With theme, ANSI escape codes should be present.
	if !strings.Contains(output, "file.go") {
		t.Errorf("expected file.go in themed output, got:\n%s", output)
	}
	if !strings.Contains(output, "stash@{1}") {
		t.Errorf("expected stash@{1} in themed output, got:\n%s", output)
	}
}

func TestRenderConflictScreen_NoFiles(t *testing.T) {
	result := &git.MergeTreeResult{
		HasConflicts: false,
		TreeSHA:      "abc123",
	}

	stash := &plugin.Stash{Index: 2, SHA: "abc123", Message: "empty stash"}

	output := conflict.RenderConflictScreen(result, stash, nil, icons.ModeASCII, 0, 80, 24)

	if !strings.Contains(output, "stash@{2}") {
		t.Errorf("expected stash index in header, got:\n%s", output)
	}
}

// ─── Plugin logic tests ────────────────────────────────────

// mockRunner implements plugin.GitRunner for unit testing.
type mockRunner struct {
	runFn         func(ctx context.Context, args ...string) (string, error)
	runExitCodeFn func(ctx context.Context, args ...string) (string, int, error)
}

func (m *mockRunner) Run(ctx context.Context, args ...string) (string, error) {
	if m.runFn != nil {
		return m.runFn(ctx, args...)
	}
	return "", nil
}

func (m *mockRunner) RunLines(ctx context.Context, args ...string) ([]string, error) {
	out, err := m.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

func (m *mockRunner) RunExitCode(ctx context.Context, args ...string) (string, int, error) {
	if m.runExitCodeFn != nil {
		return m.runExitCodeFn(ctx, args...)
	}
	return "", 0, nil
}

func TestConflictPlugin_Update_SwitchMsg(t *testing.T) {
	p := conflict.New()

	state := plugin.AppState{Mode: plugin.ModeList}
	msg := conflict.SwitchToConflictScreenMsg{
		Stash: plugin.Stash{Index: 0, SHA: "abc", Message: "test"},
		Result: git.MergeTreeResult{
			HasConflicts: true,
			Files: []git.MergeTreeFile{
				{Path: "file.go", Status: git.FileStatusConflicted},
			},
		},
		IsPop: true,
	}

	newState, _ := p.Update(msg, state)
	if newState.Mode != plugin.ModeConflict {
		t.Errorf("expected ModeConflict, got %v", newState.Mode)
	}
}

func TestConflictPlugin_Update_EscReturnsToList(t *testing.T) {
	p := conflict.New()

	state := plugin.AppState{Mode: plugin.ModeConflict}
	msg := tea.KeyPressMsg{Text: "escape"}

	newState, _ := p.Update(msg, state)
	if newState.Mode != plugin.ModeList {
		t.Errorf("expected ModeList after Esc, got %v", newState.Mode)
	}
}

func TestConflictPlugin_Update_CursorNavigation(t *testing.T) {
	p := conflict.New()

	// First set up state via SwitchToConflictScreenMsg.
	result := git.MergeTreeResult{
		HasConflicts: true,
		Files: []git.MergeTreeFile{
			{Path: "a.go", Status: git.FileStatusClean},
			{Path: "b.go", Status: git.FileStatusConflicted},
			{Path: "c.go", Status: git.FileStatusConflicted},
		},
	}
	state := plugin.AppState{Mode: plugin.ModeConflict}
	p.Update(conflict.SwitchToConflictScreenMsg{
		Stash:  plugin.Stash{Index: 0, SHA: "x"},
		Result: result,
	}, state)

	// Move cursor down.
	p.Update(tea.KeyPressMsg{Text: "j"}, state)
	p.Update(tea.KeyPressMsg{Text: "j"}, state)
	// Move cursor up.
	p.Update(tea.KeyPressMsg{Text: "k"}, state)

	// View should still render without panic.
	view := p.View(state, 80, 24)
	if !strings.Contains(view, "a.go") {
		t.Errorf("expected a.go in view, got:\n%s", view)
	}
}

func TestConflictPlugin_Update_ApplyAnyway(t *testing.T) {
	p := conflict.New()

	// Set up state.
	state := plugin.AppState{Mode: plugin.ModeConflict}
	p.Update(conflict.SwitchToConflictScreenMsg{
		Stash:  plugin.Stash{Index: 0, SHA: "abc"},
		Result: git.MergeTreeResult{HasConflicts: true},
	}, state)

	// Press 'a' to apply anyway.
	_, cmd := p.Update(tea.KeyPressMsg{Text: "a"}, state)
	if cmd == nil {
		t.Error("expected non-nil cmd from 'a' key")
	}
}

func TestConflictPlugin_Update_PopAnyway(t *testing.T) {
	p := conflict.New()

	state := plugin.AppState{Mode: plugin.ModeConflict}
	p.Update(conflict.SwitchToConflictScreenMsg{
		Stash:  plugin.Stash{Index: 0, SHA: "abc"},
		Result: git.MergeTreeResult{HasConflicts: true},
	}, state)

	_, cmd := p.Update(tea.KeyPressMsg{Text: "p"}, state)
	if cmd == nil {
		t.Error("expected non-nil cmd from 'p' key")
	}
}

func TestConflictPlugin_Update_BranchFirst(t *testing.T) {
	p := conflict.New()

	state := plugin.AppState{Mode: plugin.ModeConflict}
	p.Update(conflict.SwitchToConflictScreenMsg{
		Stash:  plugin.Stash{Index: 0, SHA: "abc"},
		Result: git.MergeTreeResult{HasConflicts: true},
	}, state)

	_, cmd := p.Update(tea.KeyPressMsg{Text: "b"}, state)
	if cmd == nil {
		t.Error("expected non-nil cmd from 'b' key")
	}
}

func TestConflictPlugin_BeforeApply_OldGit(t *testing.T) {
	p := conflict.New()
	// Simulate old git by not setting a version >= 2.38.
	p.Init(plugin.PluginContext{
		Git:    &mockRunner{},
		Cache:  &noopCache{},
		Config: &noopConfig{},
		Events: &noopBus{},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		GitVer: plugin.GitVersion{Major: 2, Minor: 37},
		Theme:  &noopTheme{},
	})

	proceed, cmd := p.BeforeApply(plugin.Stash{Index: 0, SHA: "abc"})
	if !proceed {
		t.Error("expected proceed=true for old git")
	}
	if cmd == nil {
		t.Error("expected info toast cmd for old git")
	}
}

func TestConflictPlugin_BeforeApply_CleanMerge_Proceed(t *testing.T) {
	p := conflict.New()
	p.Init(plugin.PluginContext{
		Git: &mockRunner{
			runExitCodeFn: func(_ context.Context, args ...string) (string, int, error) {
				if len(args) > 0 && args[0] == "merge-tree" {
					return "abc123\n", 0, nil
				}
				if len(args) > 0 && args[0] == "ls-tree" {
					return "", 1, nil // No untracked parent.
				}
				return "", 0, nil
			},
		},
		Cache:  &noopCache{},
		Config: &noopConfig{},
		Events: &noopBus{},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		GitVer: plugin.GitVersion{Major: 2, Minor: 53},
		Theme:  &noopTheme{},
	})

	proceed, _ := p.BeforeApply(plugin.Stash{Index: 0, SHA: "abc"})
	if !proceed {
		t.Error("expected proceed=true for clean merge")
	}
}

func TestConflictPlugin_BeforeApply_ConflictBlocks(t *testing.T) {
	p := conflict.New()
	p.Init(plugin.PluginContext{
		Git: &mockRunner{
			runExitCodeFn: func(_ context.Context, args ...string) (string, int, error) {
				if len(args) > 0 && args[0] == "merge-tree" {
					return "abc123\n\nAuto-merging config.go\nCONFLICT (content): Merge conflict in config.go\n", 1, nil
				}
				if len(args) > 0 && args[0] == "ls-tree" {
					return "", 1, nil
				}
				return "", 0, nil
			},
		},
		Cache:  &noopCache{},
		Config: &noopConfig{},
		Events: &noopBus{},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		GitVer: plugin.GitVersion{Major: 2, Minor: 53},
		Theme:  &noopTheme{},
	})

	proceed, cmd := p.BeforeApply(plugin.Stash{Index: 0, SHA: "abc"})
	if proceed {
		t.Error("expected proceed=false for conflict")
	}
	if cmd == nil {
		t.Error("expected SwitchToConflictScreenMsg cmd")
	}
}

// ─── Test doubles ──────────────────────────────────────────

type noopCache struct{}

func (c *noopCache) List(_ context.Context) ([]plugin.Stash, error) { return nil, nil }
func (c *noopCache) Diff(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *noopCache) Invalidate() {}

type noopConfig struct{}

func (c *noopConfig) GetString(_ string) string { return "" }
func (c *noopConfig) GetInt(_ string) int       { return 0 }
func (c *noopConfig) GetBool(_ string) bool     { return false }

type noopBus struct{}

func (b *noopBus) Publish(_ plugin.Event)                   {}
func (b *noopBus) Subscribe(_ string, _ func(plugin.Event)) {}

type noopTheme struct{}

func (t *noopTheme) Color(_ string) string { return "" }

func TestRenderConflictScreen_ConflictZones(t *testing.T) {
	result := &git.MergeTreeResult{
		HasConflicts: true,
		TreeSHA:      "abc123",
		Files: []git.MergeTreeFile{
			{
				Path:   "config.go",
				Status: git.FileStatusConflicted,
				ConflictZones: []git.ConflictZone{
					{
						OurContent:   "maxRetries = 5",
						TheirContent: "maxRetries = 10",
						BaseContent:  "maxRetries = 3",
					},
				},
			},
		},
	}

	stash := &plugin.Stash{Index: 0, SHA: "abc", Message: "zones"}

	output := conflict.RenderConflictScreen(result, stash, nil, icons.ModeASCII, 0, 80, 24)

	if !strings.Contains(output, "<<<<<<< HEAD") {
		t.Errorf("expected <<<<<<< HEAD marker, got:\n%s", output)
	}
	if !strings.Contains(output, "maxRetries = 5") {
		t.Errorf("expected our content, got:\n%s", output)
	}
	if !strings.Contains(output, "=======") {
		t.Errorf("expected separator, got:\n%s", output)
	}
	if !strings.Contains(output, "maxRetries = 10") {
		t.Errorf("expected their content, got:\n%s", output)
	}
	if !strings.Contains(output, ">>>>>>> stash") {
		t.Errorf("expected >>>>>>> stash marker, got:\n%s", output)
	}
	if !strings.Contains(output, "conflict zone 1/1") {
		t.Errorf("expected conflict zone count, got:\n%s", output)
	}
}
