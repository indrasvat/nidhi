package rename_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/rename"
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

func stashMessages(t *testing.T, dir string) []string {
	t.Helper()
	out := gitCmd(t, dir, "stash", "list", "--format=%gs")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func stashSHAs(t *testing.T, dir string) []string {
	t.Helper()
	out := gitCmd(t, dir, "stash", "list", "--format=%H")
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// buildStashes creates test stashes and returns plugin.Stash slice matching current state.
func buildStashes(t *testing.T, dir string, messages []string) []plugin.Stash {
	t.Helper()
	for _, msg := range messages {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}

	list := stashMessages(t, dir)
	shas := stashSHAs(t, dir)
	stashes := make([]plugin.Stash, len(list))
	for i := range list {
		stashes[i] = plugin.Stash{
			Index:   i,
			SHA:     shas[i],
			Message: list[i],
		}
	}
	return stashes
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

func newTestPlugin(t *testing.T, dir string) *rename.Plugin {
	t.Helper()
	p := rename.New()
	_ = p.Init(plugin.PluginContext{
		Git:    git.NewDefaultRunner(dir, nil),
		Cache:  &noopCache{},
		Config: &noopConfig{},
		Events: &noopBus{},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Theme:  &noopTheme{},
	})
	return p
}

// ─── Integration tests ─────────────────────────────────────

func TestRenameTop_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	stashes := buildStashes(t, dir, []string{"first", "second", "third"})
	// Order: third(0), second(1), first(2)

	p := newTestPlugin(t, dir)
	originalSHA := stashes[0].SHA

	err := p.RenameStash(context.Background(), stashes, 0, "renamed third")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	list := stashMessages(t, dir)
	if len(list) != 3 {
		t.Fatalf("stash count = %d, want 3", len(list))
	}
	if !strings.Contains(list[0], "renamed third") {
		t.Errorf("stash@{0} message = %q, want 'renamed third'", list[0])
	}

	shas := stashSHAs(t, dir)
	if shas[0] != originalSHA {
		t.Errorf("stash@{0} SHA changed: %s -> %s", originalSHA[:8], shas[0][:8])
	}

	if rename.HasIncompleteOperation() {
		t.Error("journal should be cleaned up after successful rename")
	}
}

func TestRenameMiddle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	stashes := buildStashes(t, dir, []string{"alpha", "beta", "gamma"})
	// Order: gamma(0), beta(1), alpha(2)

	p := newTestPlugin(t, dir)
	originalSHAs := stashSHAs(t, dir)

	err := p.RenameStash(context.Background(), stashes, 1, "renamed beta")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	list := stashMessages(t, dir)
	if len(list) != 3 {
		t.Fatalf("stash count = %d, want 3", len(list))
	}

	if !strings.Contains(list[1], "renamed beta") {
		t.Errorf("stash@{1} message = %q, want 'renamed beta'", list[1])
	}

	if !strings.Contains(list[0], "gamma") {
		t.Errorf("stash@{0} message = %q, want 'gamma'", list[0])
	}
	if !strings.Contains(list[2], "alpha") {
		t.Errorf("stash@{2} message = %q, want 'alpha'", list[2])
	}

	newSHAs := stashSHAs(t, dir)
	for i, sha := range originalSHAs {
		if newSHAs[i] != sha {
			t.Errorf("stash@{%d} SHA changed: %s -> %s", i, sha[:8], newSHAs[i][:8])
		}
	}
}

func TestRenamePreservesOrdering_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	stashes := buildStashes(t, dir, []string{"one", "two", "three", "four", "five"})
	// Order: five(0), four(1), three(2), two(3), one(4)

	p := newTestPlugin(t, dir)
	originalSHAs := stashSHAs(t, dir)

	// Rename stash@{2} (three -> "tres").
	err := p.RenameStash(context.Background(), stashes, 2, "tres")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	list := stashMessages(t, dir)
	if len(list) != 5 {
		t.Fatalf("stash count = %d, want 5", len(list))
	}

	expected := []string{"five", "four", "tres", "two", "one"}
	for i, want := range expected {
		if !strings.Contains(list[i], want) {
			t.Errorf("stash@{%d} = %q, want to contain %q", i, list[i], want)
		}
	}

	newSHAs := stashSHAs(t, dir)
	for i := range originalSHAs {
		if newSHAs[i] != originalSHAs[i] {
			t.Errorf("stash@{%d} SHA changed", i)
		}
	}
}

func TestRenameJournalRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	stashes := buildStashes(t, dir, []string{"first", "second"})
	// Order: second(0), first(1)

	// Write a journal as if we dropped both stashes to rename first(1)
	// but crashed before re-storing.
	journal := &rename.Journal{
		Operation: "rename",
		Entries: []rename.JournalEntry{
			{Index: 0, SHA: stashes[0].SHA, Message: stashes[0].Message},
			{Index: 1, SHA: stashes[1].SHA, Message: stashes[1].Message},
		},
		TargetIdx:  1,
		NewMsg:     "renamed first",
		Step:       2, // Both drops completed
		TotalSteps: 5,
	}
	if err := rename.WriteJournal(journal); err != nil {
		t.Fatal(err)
	}

	// Manually drop both stashes to simulate the crash state.
	gitCmd(t, dir, "stash", "drop", "stash@{0}")
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	list := stashMessages(t, dir)
	if len(list) != 0 {
		t.Fatalf("expected 0 stashes after simulated crash, got %d", len(list))
	}

	// Run recovery.
	runner := git.NewDefaultRunner(dir, nil)
	recovered, err := rename.RecoverFromJournal(context.Background(), runner)
	if err != nil {
		t.Fatalf("RecoverFromJournal: %v", err)
	}

	if recovered != 2 {
		t.Errorf("recovered = %d, want 2", recovered)
	}

	list = stashMessages(t, dir)
	if len(list) != 2 {
		t.Fatalf("stash count after recovery = %d, want 2", len(list))
	}

	if rename.HasIncompleteOperation() {
		t.Error("journal should be cleaned up after recovery")
	}
}

// ─── Plugin interface tests ────────────────────────────────

func TestRenamePlugin_Interface(t *testing.T) {
	p := rename.New()
	if p.ID() != "rename" {
		t.Errorf("ID() = %q, want %q", p.ID(), "rename")
	}
	if p.Name() != "Rename" {
		t.Errorf("Name() = %q, want %q", p.Name(), "Rename")
	}

	bindings := p.KeyBindings()
	if len(bindings) != 1 {
		t.Fatalf("KeyBindings() returned %d, want 1", len(bindings))
	}
	if bindings[0].Key != "r" {
		t.Errorf("KeyBinding key = %q, want %q", bindings[0].Key, "r")
	}
}

func TestRenamePlugin_HandleKey_R(t *testing.T) {
	p := newTestPlugin(t, t.TempDir())

	state := plugin.AppState{
		Mode: plugin.ModeList,
		Stashes: []plugin.Stash{
			{Index: 0, SHA: "sha0", Message: "test stash"},
		},
		Cursor: 0,
	}

	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "r", Mode: plugin.ModeList}, state)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from HandleKey('r') with stashes")
	}
}

func TestRenamePlugin_HandleKey_WrongKey(t *testing.T) {
	p := rename.New()

	state := plugin.AppState{Mode: plugin.ModeList}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "x", Mode: plugin.ModeList}, state)
	if cmd != nil {
		t.Fatal("expected nil cmd from HandleKey('x')")
	}
}

func TestRenamePlugin_HandleKey_NoStashes(t *testing.T) {
	p := rename.New()

	state := plugin.AppState{Mode: plugin.ModeList, Cursor: 0}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "r", Mode: plugin.ModeList}, state)
	if cmd != nil {
		t.Fatal("expected nil cmd from HandleKey('r') with no stashes")
	}
}

func TestRenamePlugin_OutOfRange(t *testing.T) {
	p := newTestPlugin(t, t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	stashes := []plugin.Stash{
		{Index: 0, SHA: "sha0", Message: "only"},
	}

	err := p.RenameStash(context.Background(), stashes, 5, "nope")
	if err == nil {
		t.Fatal("expected error for out-of-range index")
	}
}
