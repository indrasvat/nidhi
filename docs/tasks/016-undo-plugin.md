# Task 016: Undo & Recovery Plugin

## Status: TODO

## Depends On
- 013 (Stash CRUD operations — drop triggers AfterDrop hook)
- 007 (Toast component — undo toast with countdown timer)

## Parallelizable With
- 015 (Conflict preview plugin)
- 017 (Rename plugin)
- 018 (New stash screen)

## Problem
`git stash drop` and `git stash clear` are permanent. Once a stash is dropped, users have no way to recover it through normal git commands. The only recovery path is `git fsck --unreachable` which is slow, unreliable (depends on GC not having pruned), and requires knowledge of git internals.

nidhi must provide two recovery mechanisms: (1) immediate session-based undo using an in-memory ring buffer of dropped SHAs, and (2) cross-session recovery using `git fsck` to find orphaned stash-like commits. The session undo is fast and reliable; the cross-session recovery is best-effort.

## PRD Reference
- Section 6.2, FR-14 (Undo & Recovery) -- all sub-requirements FR-14.1 through FR-14.4
- Section 8.2 (Core Interfaces) -- `StashHook.AfterDrop`, `KeyHandler` interfaces
- Section 10, Screen 9 (DROP + UNDO Toast)
- Section 14 (Performance: undo must be instant)

## Files to Create
- `internal/plugins/undo/undo.go` -- plugin struct implementing `StashHook` + `KeyHandler`, ring buffer, undo logic
- `internal/plugins/undo/ringbuffer.go` -- generic ring buffer for undo entries
- `internal/plugins/undo/ringbuffer_test.go` -- unit tests for ring buffer
- `internal/plugins/undo/recovery.go` -- cross-session recovery via `git fsck`
- `internal/plugins/undo/recovery_test.go` -- integration tests for fsck-based recovery
- `internal/plugins/undo/undo_test.go` -- integration tests for drop+undo flow

## Files to Modify
- `internal/plugin/loader.go` -- register undo plugin

## Execution Steps

### Step 1: Create the ring buffer in `internal/plugins/undo/ringbuffer.go`

```go
package undo

import (
	"sync"
	"time"
)

// UndoEntry records a dropped stash for potential recovery.
type UndoEntry struct {
	SHA       string    // Full commit SHA of the dropped stash
	Message   string    // Stash message at time of drop
	Index     int       // Stash index at time of drop
	DroppedAt time.Time // When the drop occurred
}

// IsExpired returns true if the entry is older than the given duration.
func (e UndoEntry) IsExpired(ttl time.Duration) bool {
	return time.Since(e.DroppedAt) > ttl
}

// RingBuffer is a fixed-capacity LIFO ring buffer for undo entries.
// It is goroutine-safe and session-scoped (not persisted).
type RingBuffer struct {
	mu      sync.Mutex
	entries []UndoEntry
	cap     int
	head    int // Next write position
	count   int // Current number of entries
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		entries: make([]UndoEntry, capacity),
		cap:     capacity,
	}
}

// Push adds an entry to the buffer. If the buffer is full, the oldest
// entry is overwritten.
func (rb *RingBuffer) Push(entry UndoEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.cap
	if rb.count < rb.cap {
		rb.count++
	}
}

// Pop removes and returns the most recently pushed entry.
// Returns the entry and true, or a zero entry and false if empty.
func (rb *RingBuffer) Pop() (UndoEntry, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return UndoEntry{}, false
	}

	// Head points to the next write position, so the most recent entry
	// is at (head - 1 + cap) % cap.
	idx := (rb.head - 1 + rb.cap) % rb.cap
	entry := rb.entries[idx]
	rb.entries[idx] = UndoEntry{} // Clear for GC
	rb.head = idx
	rb.count--

	return entry, true
}

// Peek returns the most recently pushed entry without removing it.
func (rb *RingBuffer) Peek() (UndoEntry, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return UndoEntry{}, false
	}

	idx := (rb.head - 1 + rb.cap) % rb.cap
	return rb.entries[idx], true
}

// Len returns the current number of entries.
func (rb *RingBuffer) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

// Clear removes all entries.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for i := range rb.entries {
		rb.entries[i] = UndoEntry{}
	}
	rb.head = 0
	rb.count = 0
}
```

### Step 2: Create ring buffer tests in `internal/plugins/undo/ringbuffer_test.go`

```go
package undo

import (
	"testing"
	"time"
)

func TestRingBuffer_PushPop(t *testing.T) {
	rb := NewRingBuffer(3)

	// Push 3 entries.
	rb.Push(UndoEntry{SHA: "aaa", Message: "first"})
	rb.Push(UndoEntry{SHA: "bbb", Message: "second"})
	rb.Push(UndoEntry{SHA: "ccc", Message: "third"})

	if rb.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rb.Len())
	}

	// Pop should return LIFO order.
	tests := []struct {
		wantSHA string
		wantOK  bool
	}{
		{"ccc", true},
		{"bbb", true},
		{"aaa", true},
	}

	for i, tt := range tests {
		entry, ok := rb.Pop()
		if ok != tt.wantOK {
			t.Fatalf("Pop() #%d: ok = %v, want %v", i, ok, tt.wantOK)
		}
		if entry.SHA != tt.wantSHA {
			t.Fatalf("Pop() #%d: SHA = %q, want %q", i, entry.SHA, tt.wantSHA)
		}
	}

	// Empty buffer.
	_, ok := rb.Pop()
	if ok {
		t.Fatal("Pop() on empty buffer should return false")
	}
}

func TestRingBuffer_Wrapping(t *testing.T) {
	rb := NewRingBuffer(3)

	// Push 5 entries (wraps at capacity 3).
	rb.Push(UndoEntry{SHA: "a1"})
	rb.Push(UndoEntry{SHA: "a2"})
	rb.Push(UndoEntry{SHA: "a3"})
	rb.Push(UndoEntry{SHA: "a4"}) // overwrites a1
	rb.Push(UndoEntry{SHA: "a5"}) // overwrites a2

	if rb.Len() != 3 {
		t.Fatalf("Len() = %d, want 3 (capped)", rb.Len())
	}

	// Should get a5, a4, a3 (LIFO, oldest overwritten).
	entry, ok := rb.Pop()
	if !ok || entry.SHA != "a5" {
		t.Fatalf("Pop() = %q, want a5", entry.SHA)
	}

	entry, ok = rb.Pop()
	if !ok || entry.SHA != "a4" {
		t.Fatalf("Pop() = %q, want a4", entry.SHA)
	}

	entry, ok = rb.Pop()
	if !ok || entry.SHA != "a3" {
		t.Fatalf("Pop() = %q, want a3", entry.SHA)
	}
}

func TestRingBuffer_WrapAt50(t *testing.T) {
	rb := NewRingBuffer(50) // PRD FR-14.2 specifies 50 entries

	// Fill to capacity.
	for i := range 50 {
		rb.Push(UndoEntry{SHA: "sha-" + string(rune('A'+i%26))})
	}

	if rb.Len() != 50 {
		t.Fatalf("Len() = %d, want 50", rb.Len())
	}

	// Push one more -- should overwrite oldest.
	rb.Push(UndoEntry{SHA: "overflow"})
	if rb.Len() != 50 {
		t.Fatalf("Len() after overflow = %d, want 50", rb.Len())
	}

	// Most recent should be "overflow".
	entry, ok := rb.Peek()
	if !ok || entry.SHA != "overflow" {
		t.Fatalf("Peek() = %q, want overflow", entry.SHA)
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Push(UndoEntry{SHA: "aaa"})
	rb.Push(UndoEntry{SHA: "bbb"})

	rb.Clear()

	if rb.Len() != 0 {
		t.Fatalf("Len() after Clear = %d, want 0", rb.Len())
	}

	_, ok := rb.Pop()
	if ok {
		t.Fatal("Pop() after Clear should return false")
	}
}

func TestRingBuffer_Peek(t *testing.T) {
	rb := NewRingBuffer(3)

	_, ok := rb.Peek()
	if ok {
		t.Fatal("Peek() on empty buffer should return false")
	}

	rb.Push(UndoEntry{SHA: "peek-me"})
	entry, ok := rb.Peek()
	if !ok || entry.SHA != "peek-me" {
		t.Fatalf("Peek() = %q, want peek-me", entry.SHA)
	}

	// Peek should not remove.
	if rb.Len() != 1 {
		t.Fatalf("Len() after Peek = %d, want 1", rb.Len())
	}
}

func TestUndoEntry_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		droppedAt time.Time
		ttl       time.Duration
		want      bool
	}{
		{
			name:      "fresh entry within TTL",
			droppedAt: time.Now().Add(-10 * time.Second),
			ttl:       30 * time.Second,
			want:      false,
		},
		{
			name:      "expired entry beyond TTL",
			droppedAt: time.Now().Add(-60 * time.Second),
			ttl:       30 * time.Second,
			want:      true,
		},
		{
			name:      "exactly at TTL boundary",
			droppedAt: time.Now().Add(-30 * time.Second),
			ttl:       30 * time.Second,
			want:      false, // time.Since is >= so boundary is not expired
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := UndoEntry{DroppedAt: tt.droppedAt}
			got := entry.IsExpired(tt.ttl)
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

### Step 3: Create the undo plugin in `internal/plugins/undo/undo.go`

```go
package undo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID    = "undo"
	UndoTTL     = 30 * time.Second
	BufferSize  = 50
)

// UndoToastMsg triggers the undo toast notification.
type UndoToastMsg struct {
	Entry UndoEntry
	TTL   time.Duration
}

// UndoToastExpiredMsg signals that the undo window has closed.
type UndoToastExpiredMsg struct {
	SHA string
}

// OpenRecoveryPickerMsg signals that the recovery picker screen should open.
type OpenRecoveryPickerMsg struct {
	Candidates []RecoveryCandidate
}

// Plugin implements plugin.StashHook and plugin.KeyHandler for
// the undo & recovery feature (PRD FR-14).
type Plugin struct {
	git    git.GitRunner
	cache  git.StashCache
	logger *slog.Logger
	buffer *RingBuffer
}

// New creates a new undo plugin.
func New() *Plugin {
	return &Plugin{
		buffer: NewRingBuffer(BufferSize),
	}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return "Undo & Recovery" }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.cache = ctx.Cache
	p.logger = ctx.Logger
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// AfterDrop implements StashHook. Records the dropped stash SHA and message
// in the undo ring buffer and triggers the undo toast (FR-14.1).
func (p *Plugin) AfterDrop(stash core.Stash, sha string) tea.Cmd {
	entry := UndoEntry{
		SHA:       sha,
		Message:   stash.Message,
		Index:     stash.Index,
		DroppedAt: time.Now(),
	}

	p.buffer.Push(entry)
	p.logger.Info("stash dropped, recorded for undo",
		"sha", sha,
		"index", stash.Index,
		"message", stash.Message,
	)

	return tea.Batch(
		// Show the undo toast.
		func() tea.Msg {
			return UndoToastMsg{Entry: entry, TTL: UndoTTL}
		},
		// Schedule toast expiration.
		tea.Tick(UndoTTL, func(t time.Time) tea.Msg {
			return UndoToastExpiredMsg{SHA: sha}
		}),
	)
}

// BeforeApply is a no-op for the undo plugin.
func (p *Plugin) BeforeApply(stash core.Stash) (proceed bool, cmd tea.Cmd) {
	return true, nil
}

// BeforePush is a no-op for the undo plugin.
func (p *Plugin) BeforePush(opts plugin.PushOptions) (plugin.PushOptions, error) {
	return opts, nil
}

// KeyBindings returns the keybindings for the undo plugin.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{
			Key:         "z",
			Description: "Undo last drop / Recovery picker",
			Modes:       []core.Mode{core.ModeList, core.ModePreview},
		},
	}
}

// HandleKey handles the `z` key press for undo/recovery (FR-14.2, FR-14.3).
func (p *Plugin) HandleKey(key plugin.KeyEvent, state core.AppState) (core.AppState, tea.Cmd) {
	if key.Text != "z" {
		return state, nil
	}

	// Check if we have a recent (non-expired) undo entry.
	entry, ok := p.buffer.Peek()
	if ok && !entry.IsExpired(UndoTTL) {
		// Session undo: restore immediately (FR-14.2).
		p.buffer.Pop()
		return state, p.restoreStash(entry)
	}

	// No recent undo available -- open cross-session recovery picker (FR-14.3).
	return state, p.openRecoveryPicker()
}

// restoreStash re-stores a dropped stash via `git stash store`.
func (p *Plugin) restoreStash(entry UndoEntry) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Restore the stash.
		_, err := p.git.Run(ctx, "stash", "store", "-m", entry.Message, entry.SHA)
		if err != nil {
			return core.ErrorMsg{Err: fmt.Errorf("undo restore: %w", err)}
		}

		// Invalidate cache since stash list changed.
		p.cache.Invalidate()

		p.logger.Info("stash restored via undo",
			"sha", entry.SHA,
			"message", entry.Message,
		)

		return core.StashMutatedMsg{}
	}
}

// openRecoveryPicker discovers orphaned stash commits and opens the picker.
func (p *Plugin) openRecoveryPicker() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		candidates, err := FindDroppedStashes(ctx, p.git)
		if err != nil {
			p.logger.Error("recovery scan failed", "error", err)
			return core.ErrorMsg{Err: fmt.Errorf("recovery scan: %w", err)}
		}

		if len(candidates) == 0 {
			return core.InfoToastMsg{Text: "No recoverable stashes found."}
		}

		return OpenRecoveryPickerMsg{Candidates: candidates}
	}
}
```

### Step 4: Create cross-session recovery in `internal/plugins/undo/recovery.go`

```go
package undo

import (
	"context"
	"fmt"
	"strings"

	"github.com/indrasvat/nidhi/internal/git"
)

// RecoveryCandidate represents a potentially recoverable stash commit
// found via `git fsck`.
type RecoveryCandidate struct {
	SHA     string // Full commit SHA
	Message string // Commit message (may be stash-format or user-provided)
	Date    string // Commit date string
}

// FindDroppedStashes discovers orphaned commits that look like stash entries
// using `git fsck --unreachable --no-reflogs`.
//
// This is best-effort: commits may have been garbage-collected.
// FR-14.3: Cross-session recovery.
func FindDroppedStashes(ctx context.Context, runner git.GitRunner) ([]RecoveryCandidate, error) {
	// Find all unreachable commits.
	stdout, err := runner.Run(ctx, "fsck", "--unreachable", "--no-reflogs")
	if err != nil {
		return nil, fmt.Errorf("git fsck: %w", err)
	}

	var candidates []RecoveryCandidate

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "unreachable commit ") {
			continue
		}

		sha := strings.TrimPrefix(line, "unreachable commit ")
		sha = strings.TrimSpace(sha)
		if sha == "" {
			continue
		}

		// Check if this commit looks like a stash entry.
		// Stash commits have a specific parent structure (2-3 parents).
		parentLine, err := runner.Run(ctx, "cat-file", "-p", sha)
		if err != nil {
			continue
		}

		// Count parent lines to identify stash-like commits.
		// Stash commits have 2 parents (index + working tree) or
		// 3 parents (index + working tree + untracked).
		parentCount := 0
		for _, pl := range strings.Split(parentLine, "\n") {
			if strings.HasPrefix(strings.TrimSpace(pl), "parent ") {
				parentCount++
			}
		}
		if parentCount < 2 {
			continue // Not a stash-like commit
		}

		// Get the commit message.
		msg, err := runner.Run(ctx, "log", "--format=%s", "-1", sha)
		if err != nil {
			msg = "(unknown message)"
		}
		msg = strings.TrimSpace(msg)

		// Get the commit date.
		date, err := runner.Run(ctx, "log", "--format=%ar", "-1", sha)
		if err != nil {
			date = "(unknown date)"
		}
		date = strings.TrimSpace(date)

		candidates = append(candidates, RecoveryCandidate{
			SHA:     sha,
			Message: msg,
			Date:    date,
		})
	}

	return candidates, nil
}

// RestoreCandidate restores a recovery candidate as a stash entry.
func RestoreCandidate(ctx context.Context, runner git.GitRunner, candidate RecoveryCandidate) error {
	_, err := runner.Run(ctx, "stash", "store", "-m", candidate.Message, candidate.SHA)
	if err != nil {
		return fmt.Errorf("restore candidate %s: %w", candidate.SHA[:8], err)
	}
	return nil
}
```

### Step 5: Create undo integration tests in `internal/plugins/undo/undo_test.go`

```go
package undo_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugins/undo"
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
	out := run(t, dir, "git", "stash", "list")
	out = strings.TrimSpace(out)
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func TestDropAndUndoWithin30s(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create a stash.
	writeFile(t, dir, "feature.go", "package main\n\nfunc feature() {}\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "stash", "push", "-m", "my feature")

	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	// Record SHA before drop.
	sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))

	// Drop the stash.
	run(t, dir, "git", "stash", "drop", "stash@{0}")

	if got := stashCount(t, dir); got != 0 {
		t.Fatalf("stash count after drop = %d, want 0", got)
	}

	// Undo: restore via `git stash store`.
	runner := git.NewRunner(dir)
	_, err := runner.Run(context.Background(), "stash", "store", "-m", "my feature", sha)
	if err != nil {
		t.Fatalf("stash store failed: %v", err)
	}

	// Verify stash is restored.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count after undo = %d, want 1", got)
	}

	// Verify message preserved.
	listOut := run(t, dir, "git", "stash", "list")
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
	shas := make([]string, 5)
	messages := []string{"stash-A", "stash-B", "stash-C", "stash-D", "stash-E"}
	for i, msg := range messages {
		writeFile(t, dir, "file.go", "package main // version "+msg+"\n")
		run(t, dir, "git", "add", ".")
		run(t, dir, "git", "stash", "push", "-m", msg)
		shas[i] = strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))
	}

	if got := stashCount(t, dir); got != 5 {
		t.Fatalf("stash count = %d, want 5", got)
	}

	// Drop last 3 stashes (indices 0, 0, 0 since they shift).
	droppedSHAs := make([]string, 3)
	droppedMsgs := make([]string, 3)
	for i := range 3 {
		sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))
		listLine := strings.TrimSpace(run(t, dir, "git", "stash", "list", "--format=%gs", "-1"))
		droppedSHAs[i] = sha
		droppedMsgs[i] = listLine
		run(t, dir, "git", "stash", "drop", "stash@{0}")
	}

	if got := stashCount(t, dir); got != 2 {
		t.Fatalf("stash count after drops = %d, want 2", got)
	}

	// Undo 3 drops in LIFO order.
	runner := git.NewRunner(dir)
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

	// Simulate dropping 5 stashes.
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
		// Most recent first: E, D, C.
		want := string(rune('A' + 4 - i))
		if !strings.HasSuffix(entry.SHA, want) {
			t.Errorf("Pop() #%d: SHA = %q, want suffix %q", i, entry.SHA, want)
		}
	}

	// 2 entries remain.
	if rb.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", rb.Len())
	}
}

func TestFindDroppedStashes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create and drop a stash.
	writeFile(t, dir, "lost.go", "package main\n\nfunc lost() {}\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "stash", "push", "-m", "lost stash")

	sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))
	run(t, dir, "git", "stash", "drop", "stash@{0}")

	// Run fsck to find orphaned commits.
	runner := git.NewRunner(dir)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}

	// The dropped stash commit should be discoverable via fsck.
	var found bool
	for _, c := range candidates {
		if c.SHA == sha {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected SHA %s in candidates, got %d candidates: %v", sha[:8], len(candidates), candidates)
	}
}

func TestUndoToastExpiry(t *testing.T) {
	// Test that the timer concept works correctly.
	entry := undo.UndoEntry{
		SHA:       "abc123",
		Message:   "test stash",
		DroppedAt: time.Now(),
	}

	// Should not be expired immediately.
	if entry.IsExpired(30 * time.Second) {
		t.Fatal("entry should not be expired immediately after creation")
	}

	// Simulate passage of time by creating an entry in the past.
	oldEntry := undo.UndoEntry{
		SHA:       "def456",
		Message:   "old stash",
		DroppedAt: time.Now().Add(-31 * time.Second),
	}

	if !oldEntry.IsExpired(30 * time.Second) {
		t.Fatal("entry should be expired after 31s with 30s TTL")
	}
}
```

### Step 6: Create recovery integration tests in `internal/plugins/undo/recovery_test.go`

```go
package undo_test

import (
	"context"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugins/undo"
)

func TestRestoreCandidate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create a stash, record its SHA, drop it.
	writeFile(t, dir, "recover.go", "package main\n\nfunc recover() {}\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "stash", "push", "-m", "recoverable")

	sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))
	run(t, dir, "git", "stash", "drop", "stash@{0}")

	// Restore via RestoreCandidate.
	runner := git.NewRunner(dir)
	candidate := undo.RecoveryCandidate{
		SHA:     sha,
		Message: "recoverable",
		Date:    "just now",
	}

	err := undo.RestoreCandidate(context.Background(), runner, candidate)
	if err != nil {
		t.Fatalf("RestoreCandidate: %v", err)
	}

	// Verify stash is back.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	listOut := run(t, dir, "git", "stash", "list")
	if !strings.Contains(listOut, "recoverable") {
		t.Errorf("restored stash missing message, got: %s", listOut)
	}
}

func TestFindDroppedStashes_NoOrphans(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Fresh repo with no dropped stashes -- should find nothing.
	runner := git.NewRunner(dir)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
}

func TestFindDroppedStashes_MultipleDrops(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create and drop 3 stashes.
	droppedSHAs := make(map[string]bool)
	for i, msg := range []string{"alpha", "beta", "gamma"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		run(t, dir, "git", "add", ".")
		run(t, dir, "git", "stash", "push", "-m", msg)
		sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))
		droppedSHAs[sha] = true
		_ = i
	}

	// Drop all 3.
	for range 3 {
		run(t, dir, "git", "stash", "drop", "stash@{0}")
	}

	runner := git.NewRunner(dir)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}

	// All 3 dropped SHAs should be discoverable.
	found := 0
	for _, c := range candidates {
		if droppedSHAs[c.SHA] {
			found++
		}
	}

	if found < 3 {
		t.Errorf("expected to find all 3 dropped stashes, found %d out of %d candidates", found, len(candidates))
	}
}
```

### Step 7: Wire the undo plugin into the plugin loader

Add to `internal/plugin/loader.go`:

```go
import "github.com/indrasvat/nidhi/internal/plugins/undo"

// In LoadBuiltinPlugins():
undo.New(),
```

### Step 8: Verify

```bash
# Ring buffer unit tests
go test -v -run TestRingBuffer ./internal/plugins/undo/...

# Undo entry expiry tests
go test -v -run TestUndoEntry_IsExpired ./internal/plugins/undo/...
go test -v -run TestUndoToastExpiry ./internal/plugins/undo/...

# Integration: drop + undo flow
go test -v -run TestDropAndUndo ./internal/plugins/undo/...

# Integration: LIFO undo order
go test -v -run TestDropMultipleAndUndoLIFO ./internal/plugins/undo/...

# Integration: fsck recovery
go test -v -run TestFindDroppedStashes ./internal/plugins/undo/...

# Full CI
make ci
```

## Verification

### Functional
```bash
# Unit: ring buffer push/pop LIFO semantics
go test -v -run TestRingBuffer_PushPop ./internal/plugins/undo/...
# Expected: entries returned in LIFO order

# Unit: ring buffer wraps at capacity
go test -v -run TestRingBuffer_Wrapping ./internal/plugins/undo/...
# Expected: oldest entries overwritten, newest preserved

# Unit: ring buffer wraps at 50 (PRD capacity)
go test -v -run TestRingBuffer_WrapAt50 ./internal/plugins/undo/...
# Expected: capacity stays at 50 after overflow

# Integration: drop + undo within 30s
go test -v -run TestDropAndUndoWithin30s ./internal/plugins/undo/...
# Expected: stash restored with correct message

# Integration: drop 5, undo 3 in LIFO order
go test -v -run TestDropMultipleAndUndoLIFO ./internal/plugins/undo/...
# Expected: correct 3 stashes restored, 2 remain

# Integration: fsck finds dropped commits
go test -v -run TestFindDroppedStashes_Integration ./internal/plugins/undo/...
# Expected: dropped stash SHA found in candidates

# Integration: restore candidate
go test -v -run TestRestoreCandidate_Integration ./internal/plugins/undo/...
# Expected: stash restored with message

# Full CI
make ci
```

### Edge Cases
```bash
# Empty ring buffer
go test -v -run TestRingBuffer_Clear ./internal/plugins/undo/...

# Toast expiry timing
go test -v -run TestUndoToastExpiry ./internal/plugins/undo/...

# No orphans in fresh repo
go test -v -run TestFindDroppedStashes_NoOrphans ./internal/plugins/undo/...
```

## Completion Criteria
1. `AfterDrop` records {SHA, message, index} in the ring buffer and triggers a 30s undo toast
2. `z` key within 30s of drop restores the stash via `git stash store -m "<msg>" <sha>` and invalidates cache
3. `z` key after 30s (or with empty buffer) opens the cross-session recovery picker
4. Ring buffer is LIFO, capped at 50 entries, session-only (not persisted) -- per FR-14.2
5. `git fsck --unreachable --no-reflogs` discovers orphaned stash-like commits (2+ parents)
6. Recovery picker shows found commits with messages and dates
7. All tests pass including integration tests with real git repos using `t.TempDir()`
8. `make ci` passes

## Commit
```
feat(undo): add undo & recovery plugin with ring buffer and fsck

Implement FR-14 undo plugin with in-memory ring buffer (50 entries,
LIFO, session-scoped) for immediate undo via `z` key within 30s.
Cross-session recovery uses `git fsck --unreachable --no-reflogs`
to discover orphaned stash commits. Includes toast integration
for undo countdown notifications.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.2 (FR-14), 8.2 (interfaces), 10 (Screen 9), 14
4. Read tasks 007 and 013 to understand dependencies
5. Execute steps 1-8 in order
6. Verify all functional and edge case checks pass
7. Update this file (Status: DONE) + `docs/PROGRESS.md`
8. Commit with the message above + move to next task
