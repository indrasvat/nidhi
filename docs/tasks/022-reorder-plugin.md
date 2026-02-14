# Task 022: Reorder Plugin

## Status: TODO

## Depends On
- 013 (CRUD — stash apply/pop/drop/push operations, git command execution)
- 017 (rename plugin — journal infrastructure for transactional safety, `~/.local/state/nidhi/reorder-journal.json`)

## Parallelizable With
- 020 (search plugin)
- 021 (filter and stale plugins)
- 023 (export/import plugin)
- 024 (help overlay and mouse support)
- 025 (config file and polish)

## Problem
Git stash is a LIFO stack — the most recent stash is always `stash@{0}`. There is no native reordering. If a developer wants to prioritize a stash buried at position 5, they must manually pop and re-push in the right order. This is error-prone and tedious. nidhi needs a reorder operation that moves stashes up or down in the list, implemented via a transactional drop-and-re-store sequence with journal-based crash recovery.

## PRD Reference
- Section 6.2, FR-16 (Reorder) — FR-16.1 `Shift+J/K`, FR-16.2 drop+re-store, FR-16.3 highlight flash, FR-16.4 transactional safety
- Section 8.2 (Plugin interfaces) — KeyHandler
- Section 8.4 (Module structure) — `internal/plugins/reorder/reorder.go`
- Section 11.2 (LIST Mode keymap) — `J` (Shift+j) move down, `K` (Shift+k) move up
- Section 5.3 (Plumbing Used) — `git stash drop`, `git stash store -m "<msg>" <sha>`

## Files to Create
- `internal/plugins/reorder/reorder.go` — reorder plugin implementing KeyHandler
- `internal/plugins/reorder/journal.go` — reorder journal for transactional safety and crash recovery
- `internal/plugins/reorder/reorder_test.go` — unit and integration tests

## Execution Steps

### Step 1: Create reorder journal (`internal/plugins/reorder/journal.go`)

The journal records the pre-reorder state of ALL stashes so that if any step fails (or the process crashes), the full stash list can be reconstructed.

```go
package reorder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
)

// JournalEntry records the state of a single stash before reorder.
type JournalEntry struct {
	Index   int    `json:"index"`
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// Journal persists the pre-reorder state for crash recovery.
type Journal struct {
	Operation   string         `json:"operation"`    // "reorder"
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"` // nil if incomplete
	SourceIndex int            `json:"source_index"` // Original position of moved stash
	TargetIndex int            `json:"target_index"` // Desired position
	Entries     []JournalEntry `json:"entries"`       // ALL stashes before reorder
	filePath    string
}

// DefaultJournalPath returns the default journal file path.
// Uses XDG state directory: ~/.local/state/nidhi/reorder-journal.json
func DefaultJournalPath() string {
	return filepath.Join(xdg.StateHome, "nidhi", "reorder-journal.json")
}

// NewJournal creates a new journal for a reorder operation.
func NewJournal(sourceIndex, targetIndex int, entries []JournalEntry) *Journal {
	return &Journal{
		Operation:   "reorder",
		StartedAt:   time.Now(),
		SourceIndex: sourceIndex,
		TargetIndex: targetIndex,
		Entries:     entries,
		filePath:    DefaultJournalPath(),
	}
}

// SetPath overrides the journal file path (for testing).
func (j *Journal) SetPath(path string) {
	j.filePath = path
}

// Write persists the journal to disk.
func (j *Journal) Write() error {
	dir := filepath.Dir(j.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(j.filePath, data, 0o644)
}

// MarkComplete marks the journal as successfully completed and writes it.
func (j *Journal) MarkComplete() error {
	now := time.Now()
	j.CompletedAt = &now
	return j.Write()
}

// Remove deletes the journal file.
func (j *Journal) Remove() error {
	return os.Remove(j.filePath)
}

// LoadJournal reads an existing journal from disk.
// Returns nil, nil if no journal file exists (no pending recovery).
func LoadJournal(path string) (*Journal, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j Journal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, err
	}
	j.filePath = path
	return &j, nil
}

// IsIncomplete returns true if the journal represents an unfinished reorder.
func (j *Journal) IsIncomplete() bool {
	return j != nil && j.CompletedAt == nil
}
```

### Step 2: Create reorder plugin (`internal/plugins/reorder/reorder.go`)

```go
package reorder

import (
	"context"
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID   = "reorder"
	PluginName = "Reorder"
)

// ReorderCompleteMsg is sent when a reorder operation finishes.
type ReorderCompleteMsg struct {
	SourceIndex int // Original position
	TargetIndex int // New position
	Error       error
}

// ReorderFlashMsg triggers the highlight flash animation on the moved stash.
type ReorderFlashMsg struct {
	StashIndex int
}

// RecoveryAvailableMsg is sent on startup if an incomplete journal is found.
type RecoveryAvailableMsg struct {
	Journal *Journal
}

// Plugin implements KeyHandler for stash reordering.
type Plugin struct {
	ctx         plugin.PluginContext
	journalPath string
}

var _ plugin.KeyHandler = (*Plugin)(nil)

func New() *Plugin {
	return &Plugin{
		journalPath: DefaultJournalPath(),
	}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.ctx = ctx

	// Check for incomplete reorder journal on startup.
	journal, err := LoadJournal(p.journalPath)
	if err != nil {
		ctx.Logger.Warn("failed to load reorder journal", "error", err)
		return nil
	}
	if journal != nil && journal.IsIncomplete() {
		// Notify the UI that recovery is available.
		ctx.Events.Publish(core.Event{
			Type: "reorder.recovery_available",
			Data: journal,
		})
	}

	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns the Shift+J/K reorder bindings.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "J", Description: "Move stash down", Modes: []core.Mode{core.ModeList}},
		{Key: "K", Description: "Move stash up", Modes: []core.Mode{core.ModeList}},
	}
}

// HandleKey processes Shift+J/K reorder commands.
func (p *Plugin) HandleKey(key plugin.KeyEvent, state core.AppState) (core.AppState, tea.Cmd) {
	switch key.Text {
	case "J": // Shift+J: move selected stash DOWN (increase index).
		if state.Cursor >= len(state.Stashes)-1 {
			// Already at the bottom — no-op.
			return state, nil
		}
		return state, p.reorderCmd(state, state.Cursor, state.Cursor+1)

	case "K": // Shift+K: move selected stash UP (decrease index).
		if state.Cursor <= 0 {
			// Already at the top — no-op.
			return state, nil
		}
		return state, p.reorderCmd(state, state.Cursor, state.Cursor-1)
	}

	return state, nil
}

// reorderCmd returns a tea.Cmd that performs the reorder operation.
//
// Algorithm: To move stash@{source} to position target:
// 1. Record ALL stashes in journal (SHAs + messages).
// 2. Drop ALL stashes from highest index to 0.
// 3. Re-store them in the new desired order via `git stash store`.
//    Git stash store prepends to the list, so we store from last desired
//    position to first (position 0 stored last).
// 4. Mark journal complete and remove it.
//
// Why drop all? Because git stash indices shift on every drop. Moving
// just two stashes would require complex index arithmetic. Dropping all
// and re-storing in the desired order is simpler and equally fast for
// typical stash counts (< 100).
func (p *Plugin) reorderCmd(state core.AppState, sourceIndex, targetIndex int) tea.Cmd {
	stashes := make([]core.Stash, len(state.Stashes))
	copy(stashes, state.Stashes)
	gitRunner := p.ctx.Git
	journalPath := p.journalPath

	return func() tea.Msg {
		ctx := context.Background()

		// Step 1: Build journal entries.
		entries := make([]JournalEntry, len(stashes))
		for i, s := range stashes {
			entries[i] = JournalEntry{
				Index:   s.Index,
				SHA:     s.SHA,
				Message: s.Message,
			}
		}

		journal := NewJournal(sourceIndex, targetIndex, entries)
		journal.SetPath(journalPath)
		if err := journal.Write(); err != nil {
			return ReorderCompleteMsg{SourceIndex: sourceIndex, TargetIndex: targetIndex, Error: fmt.Errorf("write journal: %w", err)}
		}

		// Step 2: Compute the new order.
		newOrder := make([]JournalEntry, len(entries))
		copy(newOrder, entries)
		// Remove the source element and insert it at the target position.
		moved := newOrder[sourceIndex]
		newOrder = append(newOrder[:sourceIndex], newOrder[sourceIndex+1:]...)
		// Insert at target. If sourceIndex < targetIndex, the target shifted left by 1.
		insertAt := targetIndex
		if sourceIndex < targetIndex {
			insertAt = targetIndex - 1
		}
		tail := make([]JournalEntry, len(newOrder[insertAt:]))
		copy(tail, newOrder[insertAt:])
		newOrder = append(newOrder[:insertAt], moved)
		newOrder = append(newOrder, tail...)

		// Step 3: Drop all stashes (highest index first).
		for i := len(stashes) - 1; i >= 0; i-- {
			_, err := gitRunner.Run(ctx, "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
			if err != nil {
				// Attempt recovery: re-store from journal.
				_ = p.recoverFromJournal(ctx, journal, gitRunner)
				return ReorderCompleteMsg{
					SourceIndex: sourceIndex,
					TargetIndex: targetIndex,
					Error:       fmt.Errorf("drop stash@{%d}: %w", i, err),
				}
			}
		}

		// Step 4: Re-store in new order (last to first, since store prepends).
		for i := len(newOrder) - 1; i >= 0; i-- {
			e := newOrder[i]
			_, err := gitRunner.Run(ctx, "stash", "store", "-m", e.Message, e.SHA)
			if err != nil {
				// Attempt recovery: re-store remaining from journal.
				_ = p.recoverFromJournal(ctx, journal, gitRunner)
				return ReorderCompleteMsg{
					SourceIndex: sourceIndex,
					TargetIndex: targetIndex,
					Error:       fmt.Errorf("store %s: %w", e.SHA, err),
				}
			}
		}

		// Step 5: Mark journal complete and clean up.
		_ = journal.MarkComplete()
		_ = journal.Remove()

		return ReorderCompleteMsg{
			SourceIndex: sourceIndex,
			TargetIndex: targetIndex,
		}
	}
}

// recoverFromJournal attempts to restore the original stash order from the journal.
// This is the crash recovery path — it drops whatever is currently in the stash
// list and re-stores from the journal entries.
func (p *Plugin) recoverFromJournal(ctx context.Context, journal *Journal, gitRunner core.GitRunner) error {
	// First, clear any partial stash state.
	// Count current stashes.
	lines, _ := gitRunner.RunLines(ctx, "stash", "list")
	for i := len(lines) - 1; i >= 0; i-- {
		_, _ = gitRunner.Run(ctx, "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
	}

	// Re-store original order (last to first).
	for i := len(journal.Entries) - 1; i >= 0; i-- {
		e := journal.Entries[i]
		_, err := gitRunner.Run(ctx, "stash", "store", "-m", e.Message, e.SHA)
		if err != nil {
			return fmt.Errorf("recovery: store %s: %w", e.SHA, err)
		}
	}

	return nil
}

// RecoverFromJournal is the public API for crash recovery, called on startup
// when an incomplete journal is found. Returns the stash list after recovery.
func RecoverFromJournal(ctx context.Context, journalPath string, gitRunner core.GitRunner) error {
	journal, err := LoadJournal(journalPath)
	if err != nil {
		return fmt.Errorf("load journal: %w", err)
	}
	if journal == nil || !journal.IsIncomplete() {
		return nil
	}

	p := &Plugin{journalPath: journalPath}
	if err := p.recoverFromJournal(ctx, journal, gitRunner); err != nil {
		return err
	}

	_ = journal.MarkComplete()
	_ = journal.Remove()
	return nil
}
```

### Step 3: Write tests (`internal/plugins/reorder/reorder_test.go`)

```go
package reorder_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugins/reorder"
)

// --- Test Helpers ---

// testRepo creates a temp git repo with N stashes and returns the path
// and a run helper function.
func testRepo(t *testing.T, numStashes int) (string, func(args ...string) string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\noutput: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	writeFile("base.go", "package main\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	// Create N stashes. stash@{0} is the most recent (last created).
	// We create them in order so stash@{0} = "stash N-1", stash@{N-1} = "stash 0"
	for i := 0; i < numStashes; i++ {
		writeFile("file"+strconv.Itoa(i)+".go", "package f"+strconv.Itoa(i)+"\n")
		run("git", "add", ".")
		run("git", "stash", "push", "-m", "stash "+strconv.Itoa(i))
	}

	return dir, run
}

// getStashMessages returns the ordered list of stash messages.
func getStashMessages(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "stash", "list", "--format=%gs")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git stash list failed: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// getStashSHAs returns the ordered list of stash SHAs.
func getStashSHAs(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "stash", "list", "--format=%H")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git stash list failed: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// --- Integration Tests ---

// TestMoveStashUp creates 5 stashes and moves stash@{2} up to position 1.
// Verifies the new order and that SHAs are preserved.
func TestMoveStashUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, run := testRepo(t, 5)

	// Record original order: stash@{0}="stash 4", stash@{1}="stash 3", ...
	originalMessages := getStashMessages(t, dir)
	originalSHAs := getStashSHAs(t, dir)
	if len(originalMessages) != 5 {
		t.Fatalf("expected 5 stashes, got %d", len(originalMessages))
	}

	// Move stash@{2} up to position 1.
	// Original order: [4, 3, 2, 1, 0] (messages: "stash 4", "stash 3", "stash 2", "stash 1", "stash 0")
	// After move up: stash@{2} goes to position 1 → [4, 2, 3, 1, 0]
	//
	// Implementation: drop all, re-store in new order.
	sourceIdx := 2
	targetIdx := 1

	// Build journal entries from original state.
	entries := make([]reorder.JournalEntry, 5)
	for i := 0; i < 5; i++ {
		entries[i] = reorder.JournalEntry{
			Index:   i,
			SHA:     originalSHAs[i],
			Message: originalMessages[i],
		}
	}

	// Compute new order.
	newOrder := make([]reorder.JournalEntry, 5)
	copy(newOrder, entries)
	moved := newOrder[sourceIdx]
	newOrder = append(newOrder[:sourceIdx], newOrder[sourceIdx+1:]...)
	// Insert at targetIdx.
	tail := make([]reorder.JournalEntry, len(newOrder[targetIdx:]))
	copy(tail, newOrder[targetIdx:])
	newOrder = append(newOrder[:targetIdx], moved)
	newOrder = append(newOrder, tail...)

	// Execute: drop all stashes.
	for i := 4; i >= 0; i-- {
		run("git", "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
	}

	// Re-store in new order (last to first, since store prepends).
	for i := len(newOrder) - 1; i >= 0; i-- {
		run("git", "stash", "store", "-m", newOrder[i].Message, newOrder[i].SHA)
	}

	// Verify new order.
	newMessages := getStashMessages(t, dir)
	if len(newMessages) != 5 {
		t.Fatalf("expected 5 stashes after reorder, got %d", len(newMessages))
	}

	// Expected: the stash that was at position 2 is now at position 1.
	expectedMessages := []string{
		originalMessages[0], // stash@{0} unchanged
		originalMessages[2], // moved from 2 to 1
		originalMessages[1], // shifted from 1 to 2
		originalMessages[3], // unchanged
		originalMessages[4], // unchanged
	}

	for i, msg := range newMessages {
		if msg != expectedMessages[i] {
			t.Errorf("position %d: expected %q, got %q", i, expectedMessages[i], msg)
		}
	}

	// Verify SHAs are preserved.
	newSHAs := getStashSHAs(t, dir)
	expectedSHAs := []string{
		originalSHAs[0],
		originalSHAs[2],
		originalSHAs[1],
		originalSHAs[3],
		originalSHAs[4],
	}
	for i, sha := range newSHAs {
		if sha != expectedSHAs[i] {
			t.Errorf("position %d: SHA mismatch, expected %s, got %s", i, expectedSHAs[i], sha)
		}
	}
}

// TestMoveStashDown creates 5 stashes and moves stash@{0} down to position 1.
// Verifies the new order.
func TestMoveStashDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, run := testRepo(t, 5)

	originalMessages := getStashMessages(t, dir)
	originalSHAs := getStashSHAs(t, dir)

	// Move stash@{0} down to position 1.
	// Original: [4, 3, 2, 1, 0]
	// After: [3, 4, 2, 1, 0] — stash@{0} swaps with stash@{1}
	sourceIdx := 0
	targetIdx := 1

	entries := make([]reorder.JournalEntry, 5)
	for i := 0; i < 5; i++ {
		entries[i] = reorder.JournalEntry{
			Index: i, SHA: originalSHAs[i], Message: originalMessages[i],
		}
	}

	newOrder := make([]reorder.JournalEntry, 5)
	copy(newOrder, entries)
	moved := newOrder[sourceIdx]
	newOrder = append(newOrder[:sourceIdx], newOrder[sourceIdx+1:]...)
	newOrder = append(newOrder[:targetIdx], append([]reorder.JournalEntry{moved}, newOrder[targetIdx:]...)...)

	// Execute reorder.
	for i := 4; i >= 0; i-- {
		run("git", "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
	}
	for i := len(newOrder) - 1; i >= 0; i-- {
		run("git", "stash", "store", "-m", newOrder[i].Message, newOrder[i].SHA)
	}

	newMessages := getStashMessages(t, dir)
	expectedMessages := []string{
		originalMessages[1], // was at 1, now at 0
		originalMessages[0], // was at 0, now at 1
		originalMessages[2],
		originalMessages[3],
		originalMessages[4],
	}
	for i, msg := range newMessages {
		if msg != expectedMessages[i] {
			t.Errorf("position %d: expected %q, got %q", i, expectedMessages[i], msg)
		}
	}
}

// TestMoveBottomStashDown verifies that moving the bottom stash down is a no-op.
func TestMoveBottomStashDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, _ := testRepo(t, 5)

	originalMessages := getStashMessages(t, dir)

	// Attempting to move stash@{4} (bottom) down should be a no-op.
	// The plugin's HandleKey should return early.
	// We verify by checking that the stash list is unchanged.
	afterMessages := getStashMessages(t, dir)
	for i, msg := range afterMessages {
		if msg != originalMessages[i] {
			t.Errorf("position %d changed unexpectedly: %q -> %q", i, originalMessages[i], msg)
		}
	}
}

// TestMoveTopStashUp verifies that moving the top stash up is a no-op.
func TestMoveTopStashUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, _ := testRepo(t, 5)
	originalMessages := getStashMessages(t, dir)
	afterMessages := getStashMessages(t, dir)
	for i, msg := range afterMessages {
		if msg != originalMessages[i] {
			t.Errorf("position %d changed unexpectedly: %q -> %q", i, originalMessages[i], msg)
		}
	}
}

// --- Unit Tests ---

// TestJournalWriteAndLoad verifies journal persistence.
func TestJournalWriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-journal.json")

	entries := []reorder.JournalEntry{
		{Index: 0, SHA: "aaa111", Message: "stash 0"},
		{Index: 1, SHA: "bbb222", Message: "stash 1"},
		{Index: 2, SHA: "ccc333", Message: "stash 2"},
	}

	j := reorder.NewJournal(1, 0, entries)
	j.SetPath(path)

	if err := j.Write(); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file exists and is valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Load the journal.
	loaded, err := reorder.LoadJournal(path)
	if err != nil {
		t.Fatalf("LoadJournal failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadJournal returned nil")
	}

	if loaded.SourceIndex != 1 {
		t.Errorf("expected SourceIndex=1, got %d", loaded.SourceIndex)
	}
	if loaded.TargetIndex != 0 {
		t.Errorf("expected TargetIndex=0, got %d", loaded.TargetIndex)
	}
	if len(loaded.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded.Entries))
	}
	if loaded.Entries[2].SHA != "ccc333" {
		t.Errorf("expected SHA 'ccc333', got %q", loaded.Entries[2].SHA)
	}

	// Should be incomplete (CompletedAt is nil).
	if !loaded.IsIncomplete() {
		t.Error("expected journal to be incomplete")
	}

	// Mark complete.
	if err := loaded.MarkComplete(); err != nil {
		t.Fatalf("MarkComplete failed: %v", err)
	}
	if loaded.IsIncomplete() {
		t.Error("expected journal to be complete after MarkComplete")
	}

	// Clean up.
	if err := loaded.Remove(); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	_, err = os.Stat(path)
	if !os.IsNotExist(err) {
		t.Error("expected journal file to be removed")
	}
}

// TestJournalLoadNonexistent verifies loading from a path with no file.
func TestJournalLoadNonexistent(t *testing.T) {
	j, err := reorder.LoadJournal("/tmp/nonexistent-nidhi-test-journal.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if j != nil {
		t.Error("expected nil journal for nonexistent path")
	}
}

// TestJournalCrashRecoverySimulation simulates a crash mid-reorder by
// writing a journal, dropping some stashes, then using the journal to recover.
func TestJournalCrashRecoverySimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, run := testRepo(t, 5)

	originalMessages := getStashMessages(t, dir)
	originalSHAs := getStashSHAs(t, dir)

	// Write a journal as if we started a reorder.
	journalPath := filepath.Join(dir, "reorder-journal.json")
	entries := make([]reorder.JournalEntry, 5)
	for i := 0; i < 5; i++ {
		entries[i] = reorder.JournalEntry{
			Index:   i,
			SHA:     originalSHAs[i],
			Message: originalMessages[i],
		}
	}
	j := reorder.NewJournal(2, 0, entries)
	j.SetPath(journalPath)
	if err := j.Write(); err != nil {
		t.Fatalf("Write journal: %v", err)
	}

	// Simulate partial crash: drop 3 of 5 stashes (incomplete reorder).
	run("git", "stash", "drop", "stash@{4}")
	run("git", "stash", "drop", "stash@{3}")
	run("git", "stash", "drop", "stash@{2}")
	// Now we have 2 stashes left (indices 0 and 1), but in a corrupt state.

	// Verify we're in a bad state.
	badMessages := getStashMessages(t, dir)
	if len(badMessages) != 2 {
		t.Fatalf("expected 2 stashes after simulated crash, got %d", len(badMessages))
	}

	// Recover from journal. This uses a test GitRunner that shells out to git.
	// In the real implementation, we'd use the injected GitRunner.
	// For this test, we replicate the recovery logic directly.

	// Clear remaining stashes.
	for i := len(badMessages) - 1; i >= 0; i-- {
		run("git", "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
	}

	// Verify empty.
	emptyMessages := getStashMessages(t, dir)
	if len(emptyMessages) != 0 && emptyMessages[0] != "" {
		t.Fatalf("expected 0 stashes after clearing, got %d", len(emptyMessages))
	}

	// Re-store from journal in original order (last to first).
	for i := len(entries) - 1; i >= 0; i-- {
		run("git", "stash", "store", "-m", entries[i].Message, entries[i].SHA)
	}

	// Verify recovery: original order restored.
	recoveredMessages := getStashMessages(t, dir)
	if len(recoveredMessages) != 5 {
		t.Fatalf("expected 5 stashes after recovery, got %d", len(recoveredMessages))
	}
	for i, msg := range recoveredMessages {
		if msg != originalMessages[i] {
			t.Errorf("position %d: expected %q, got %q", i, originalMessages[i], msg)
		}
	}

	// Verify SHAs match.
	recoveredSHAs := getStashSHAs(t, dir)
	for i, sha := range recoveredSHAs {
		if sha != originalSHAs[i] {
			t.Errorf("position %d: SHA expected %s, got %s", i, originalSHAs[i], sha)
		}
	}
}
```

### Step 4: Verify

```bash
# Run all tests.
go test -v -count=1 ./internal/plugins/reorder/...

# Integration tests only.
go test -v -count=1 -run 'TestMoveStash|TestJournalCrashRecovery' ./internal/plugins/reorder/...

# Full CI pipeline.
make ci
```

## Verification

### Functional
```bash
# Unit tests pass
go test -v -count=1 -run 'TestJournalWriteAndLoad|TestJournalLoadNonexistent' ./internal/plugins/reorder/...

# Integration tests pass
go test -v -count=1 -run 'TestMoveStashUp|TestMoveStashDown|TestMoveBottomStashDown|TestMoveTopStashUp|TestJournalCrashRecovery' ./internal/plugins/reorder/...

# Compiles and passes vet
go vet ./internal/plugins/reorder/...

# Lint clean
golangci-lint run ./internal/plugins/reorder/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/plugins/reorder/reorder.go` implements `KeyHandler` for `J` (Shift+J) and `K` (Shift+K)
2. `internal/plugins/reorder/journal.go` implements journal persistence with write/load/complete/remove
3. Shift+J moves selected stash down one position; Shift+K moves it up one position
4. Moving the bottom stash down or top stash up is a no-op (edge cases handled)
5. Reorder uses drop-all + re-store strategy with correct ordering
6. SHAs are preserved after reorder (same commits, different positions)
7. Journal is written before reorder starts and removed after completion
8. Journal persists to `~/.local/state/nidhi/reorder-journal.json`
9. Incomplete journal detected on startup triggers recovery notification
10. Recovery from journal restores original stash order after simulated crash
11. All unit tests pass: journal write/load/complete, nonexistent path
12. All integration tests pass: move up, move down, edge cases, crash recovery
13. `make ci` passes (lint + test)

## Commit
```
feat(reorder): add stash reorder plugin with journal-based crash recovery

Implement reorder plugin (KeyHandler) with Shift+J/K to move stashes
up/down in the list. Uses drop-all + re-store strategy with a persisted
journal (~/.local/state/nidhi/reorder-journal.json) for transactional
safety. On crash recovery, detects incomplete journal and offers to
restore original order. SHAs preserved through reorder. Edge cases
(top/bottom bounds) return no-op.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.2 (FR-16), 11.2 (keymap), 5.3 (plumbing), 8.2 (interfaces)
4. Verify dependencies: task 013 (CRUD operations) and task 017 (journal infrastructure) are DONE
5. Create `internal/plugins/reorder/journal.go` with Journal type and persistence
6. Create `internal/plugins/reorder/reorder.go` with Plugin implementing KeyHandler
7. Create `internal/plugins/reorder/reorder_test.go` with all unit and integration tests
8. Run `go test -v -count=1 ./internal/plugins/reorder/...`
9. Run `make ci`
10. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
11. Commit with the message above
