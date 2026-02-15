package undo_test

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/undo"
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

func stashCount(t *testing.T, dir string) int {
	t.Helper()
	out := gitCmd(t, dir, "stash", "list")
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
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

// ─── Integration tests ─────────────────────────────────────

func TestDropAndUndoWithin30s(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	writeFile(t, dir, "feature.go", "package main\n\nfunc feature() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "my feature")

	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	sha := gitCmd(t, dir, "rev-parse", "stash@{0}")
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	if got := stashCount(t, dir); got != 0 {
		t.Fatalf("stash count after drop = %d, want 0", got)
	}

	// Undo: restore via `git stash store`.
	runner := git.NewDefaultRunner(dir, nil)
	_, err := runner.Run(context.Background(), "stash", "store", "-m", "my feature", sha)
	if err != nil {
		t.Fatalf("stash store failed: %v", err)
	}

	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count after undo = %d, want 1", got)
	}

	listOut := gitCmd(t, dir, "stash", "list")
	if !strings.Contains(listOut, "my feature") {
		t.Errorf("restored stash missing message, got: %s", listOut)
	}
}

func TestDropMultipleAndUndoLIFO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create 5 stashes.
	messages := []string{"stash-A", "stash-B", "stash-C", "stash-D", "stash-E"}
	for _, msg := range messages {
		writeFile(t, dir, "file.go", "package main // version "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}

	if got := stashCount(t, dir); got != 5 {
		t.Fatalf("stash count = %d, want 5", got)
	}

	// Drop last 3 stashes (indices 0, 0, 0 since they shift).
	droppedSHAs := make([]string, 3)
	droppedMsgs := make([]string, 3)
	for i := range 3 {
		sha := gitCmd(t, dir, "rev-parse", "stash@{0}")
		listLine := gitCmd(t, dir, "stash", "list", "--format=%gs", "-1")
		droppedSHAs[i] = sha
		droppedMsgs[i] = listLine
		gitCmd(t, dir, "stash", "drop", "stash@{0}")
	}

	if got := stashCount(t, dir); got != 2 {
		t.Fatalf("stash count after drops = %d, want 2", got)
	}

	// Undo 3 drops in LIFO order.
	runner := git.NewDefaultRunner(dir, nil)
	for i := 2; i >= 0; i-- {
		_, err := runner.Run(context.Background(), "stash", "store", "-m", droppedMsgs[i], droppedSHAs[i])
		if err != nil {
			t.Fatalf("stash store #%d failed: %v", i, err)
		}
	}

	if got := stashCount(t, dir); got != 5 {
		t.Fatalf("stash count after undo = %d, want 5", got)
	}
}

func TestRingBuffer_UndoSimulation(t *testing.T) {
	rb := undo.NewRingBuffer(50)

	now := time.Now()
	for i := range 5 {
		rb.Push(undo.UndoEntry{
			SHA:       "sha-" + string(rune('A'+i)),
			Message:   "stash-" + string(rune('A'+i)),
			Index:     i,
			DroppedAt: now,
		})
	}

	// Undo 3 in LIFO order.
	for i := range 3 {
		entry, ok := rb.Pop()
		if !ok {
			t.Fatalf("Pop() #%d failed", i)
		}
		want := string(rune('A' + 4 - i))
		if !strings.HasSuffix(entry.SHA, want) {
			t.Errorf("Pop() #%d: SHA = %q, want suffix %q", i, entry.SHA, want)
		}
	}

	if rb.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", rb.Len())
	}
}

func TestUndoToastExpiry(t *testing.T) {
	entry := undo.UndoEntry{
		SHA:       "abc123",
		Message:   "test stash",
		DroppedAt: time.Now(),
	}

	if entry.IsExpired(30 * time.Second) {
		t.Fatal("entry should not be expired immediately after creation")
	}

	oldEntry := undo.UndoEntry{
		SHA:       "def456",
		Message:   "old stash",
		DroppedAt: time.Now().Add(-31 * time.Second),
	}

	if !oldEntry.IsExpired(30 * time.Second) {
		t.Fatal("entry should be expired after 31s with 30s TTL")
	}
}

// ─── Plugin interface tests ────────────────────────────────

func TestUndoPlugin_Interface(t *testing.T) {
	p := undo.New()
	if p.ID() != "undo" {
		t.Errorf("ID() = %q, want %q", p.ID(), "undo")
	}
	if p.Name() != "Undo & Recovery" {
		t.Errorf("Name() = %q, want %q", p.Name(), "Undo & Recovery")
	}

	bindings := p.KeyBindings()
	if len(bindings) != 1 {
		t.Fatalf("KeyBindings() returned %d, want 1", len(bindings))
	}
	if bindings[0].Key != "z" {
		t.Errorf("KeyBinding key = %q, want %q", bindings[0].Key, "z")
	}
}

func TestUndoPlugin_AfterDrop(t *testing.T) {
	p := undo.New()
	_ = p.Init(plugin.PluginContext{
		Git:    git.NewDefaultRunner(t.TempDir(), nil),
		Cache:  &noopCache{},
		Config: &noopConfig{},
		Events: &noopBus{},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Theme:  &noopTheme{},
	})

	stash := plugin.Stash{
		Index:   0,
		SHA:     "abc123",
		Message: "dropped stash",
	}

	cmd := p.AfterDrop(stash, "abc123")
	if cmd == nil {
		t.Fatal("expected non-nil cmd from AfterDrop")
	}

	// Buffer should have the entry.
	if p.Buffer().Len() != 1 {
		t.Fatalf("buffer Len() = %d, want 1", p.Buffer().Len())
	}

	entry, ok := p.Buffer().Peek()
	if !ok {
		t.Fatal("Peek() failed")
	}
	if entry.SHA != "abc123" {
		t.Errorf("entry SHA = %q, want %q", entry.SHA, "abc123")
	}
	if entry.Message != "dropped stash" {
		t.Errorf("entry Message = %q, want %q", entry.Message, "dropped stash")
	}
}

func TestUndoPlugin_HandleKey_WithRecentEntry(t *testing.T) {
	p := undo.New()
	_ = p.Init(plugin.PluginContext{
		Git:    git.NewDefaultRunner(t.TempDir(), nil),
		Cache:  &noopCache{},
		Config: &noopConfig{},
		Events: &noopBus{},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Theme:  &noopTheme{},
	})

	// Add a recent entry.
	p.Buffer().Push(undo.UndoEntry{
		SHA:       "recentsha",
		Message:   "recent",
		DroppedAt: time.Now(),
	})

	state := plugin.AppState{Mode: plugin.ModeList}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "z", Mode: plugin.ModeList}, state)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from HandleKey('z') with recent entry")
	}

	// Buffer should be empty after pop.
	if p.Buffer().Len() != 0 {
		t.Fatalf("buffer Len() = %d, want 0 after undo", p.Buffer().Len())
	}
}

func TestUndoPlugin_HandleKey_NoEntry(t *testing.T) {
	p := undo.New()
	_ = p.Init(plugin.PluginContext{
		Git:    git.NewDefaultRunner(t.TempDir(), nil),
		Cache:  &noopCache{},
		Config: &noopConfig{},
		Events: &noopBus{},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
		Theme:  &noopTheme{},
	})

	state := plugin.AppState{Mode: plugin.ModeList}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "z", Mode: plugin.ModeList}, state)
	if cmd == nil {
		t.Fatal("expected non-nil cmd (recovery picker) from HandleKey('z') with empty buffer")
	}
}

func TestUndoPlugin_HandleKey_WrongKey(t *testing.T) {
	p := undo.New()

	state := plugin.AppState{Mode: plugin.ModeList}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "x", Mode: plugin.ModeList}, state)
	if cmd != nil {
		t.Fatal("expected nil cmd from HandleKey('x')")
	}
}

func TestUndoPlugin_BeforeApply_Noop(t *testing.T) {
	p := undo.New()
	proceed, cmd := p.BeforeApply(plugin.Stash{})
	if !proceed {
		t.Error("expected proceed=true")
	}
	if cmd != nil {
		t.Error("expected nil cmd")
	}
}

func TestUndoPlugin_BeforePush_Noop(t *testing.T) {
	p := undo.New()
	opts, err := p.BeforePush(plugin.PushOptions{Message: "test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if opts.Message != "test" {
		t.Errorf("opts.Message = %q, want %q", opts.Message, "test")
	}
}
