# Task 017: Rename Plugin

## Status: TODO

## Depends On
- 013 (Stash CRUD operations — drop/store primitives)
- 008 (Stash row component — inline edit mode with textinput.Model)

## Parallelizable With
- 015 (Conflict preview plugin)
- 016 (Undo plugin)
- 018 (New stash screen)

## Problem
Git provides no way to rename a stash after creation. The default messages (`WIP on main: abc1234 some msg`) are cryptic and unhelpful. The only workaround is a multi-step plumbing operation: record the SHA, drop the stash, then `git stash store -m "<new msg>" <sha>`. If the stash is not at position 0, all stashes above it must also be dropped and re-stored to preserve ordering — a fragile multi-step process that can corrupt the stash list if interrupted.

nidhi must provide inline rename with a single `r` keypress, handle the multi-step reorder safely using a crash-recovery journal, and preserve the original SHA.

## PRD Reference
- Section 6.2, FR-13 (Rename plugin) -- all sub-requirements FR-13.1 through FR-13.5
- Section 6.2, FR-16.4 (Transactional safety -- reorder journal, referenced by FR-13.5)
- Section 8.2 (Core Interfaces) -- `KeyHandler` interface
- Section 10, Screen 8 (Rename -- inline editing in LIST view)

## Files to Create
- `internal/plugins/rename/rename.go` -- plugin struct implementing `KeyHandler`, rename logic
- `internal/plugins/rename/journal.go` -- reorder journal for crash safety (FR-16.4)
- `internal/plugins/rename/journal_test.go` -- unit tests for journal read/write/cleanup
- `internal/plugins/rename/rename_test.go` -- integration tests for rename operations

## Files to Modify
- `internal/plugin/loader.go` -- register rename plugin

## Execution Steps

### Step 1: Create the reorder journal in `internal/plugins/rename/journal.go`

The journal records all stash SHAs and messages before a multi-step drop+restore operation. If the process crashes mid-operation, the journal enables recovery on next startup.

```go
package rename

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// JournalEntry records a stash's identity for crash recovery.
type JournalEntry struct {
	Index   int    `json:"index"`
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// Journal represents a reorder operation in progress.
type Journal struct {
	Operation string         `json:"operation"` // "rename", "reorder"
	Entries   []JournalEntry `json:"entries"`   // All stashes at time of operation
	TargetIdx int            `json:"target_idx"` // Index being renamed/moved
	NewMsg    string         `json:"new_msg"`    // New message (for rename)
	Step      int            `json:"step"`       // Current step (0-based)
	TotalSteps int           `json:"total_steps"`
}

// journalPath returns the path to the reorder journal file.
func journalPath() string {
	return filepath.Join(xdg.StateHome, "nidhi", "reorder-journal.json")
}

// WriteJournal persists the journal to disk.
func WriteJournal(j *Journal) error {
	path := journalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}

	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}

	return nil
}

// ReadJournal reads an existing journal from disk.
// Returns nil if no journal exists.
func ReadJournal() (*Journal, error) {
	path := journalPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read journal: %w", err)
	}

	var j Journal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("unmarshal journal: %w", err)
	}

	return &j, nil
}

// RemoveJournal deletes the journal file after a successful operation.
func RemoveJournal() error {
	path := journalPath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove journal: %w", err)
	}
	return nil
}

// HasIncompleteOperation checks if there is a journal from a previous
// interrupted operation.
func HasIncompleteOperation() bool {
	j, err := ReadJournal()
	return err == nil && j != nil
}
```

### Step 2: Create journal tests in `internal/plugins/rename/journal_test.go`

```go
package rename

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJournal_WriteReadCleanup(t *testing.T) {
	// Override the journal path for testing.
	tmpDir := t.TempDir()
	origXDG := os.Getenv("XDG_STATE_HOME")
	t.Setenv("XDG_STATE_HOME", tmpDir)
	defer func() {
		if origXDG != "" {
			os.Setenv("XDG_STATE_HOME", origXDG)
		}
	}()

	journal := &Journal{
		Operation: "rename",
		Entries: []JournalEntry{
			{Index: 0, SHA: "sha-aaa", Message: "newest stash"},
			{Index: 1, SHA: "sha-bbb", Message: "target stash"},
			{Index: 2, SHA: "sha-ccc", Message: "oldest stash"},
		},
		TargetIdx:  1,
		NewMsg:     "renamed target",
		Step:       0,
		TotalSteps: 4,
	}

	// Write.
	err := WriteJournal(journal)
	if err != nil {
		t.Fatalf("WriteJournal: %v", err)
	}

	// Verify file exists.
	path := filepath.Join(tmpDir, "nidhi", "reorder-journal.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("journal file not found: %v", err)
	}

	// Read back.
	got, err := ReadJournal()
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if got == nil {
		t.Fatal("ReadJournal returned nil")
	}

	if got.Operation != "rename" {
		t.Errorf("Operation = %q, want rename", got.Operation)
	}
	if len(got.Entries) != 3 {
		t.Errorf("Entries = %d, want 3", len(got.Entries))
	}
	if got.TargetIdx != 1 {
		t.Errorf("TargetIdx = %d, want 1", got.TargetIdx)
	}
	if got.NewMsg != "renamed target" {
		t.Errorf("NewMsg = %q, want 'renamed target'", got.NewMsg)
	}

	// Cleanup.
	err = RemoveJournal()
	if err != nil {
		t.Fatalf("RemoveJournal: %v", err)
	}

	// Verify gone.
	j, err := ReadJournal()
	if err != nil {
		t.Fatalf("ReadJournal after remove: %v", err)
	}
	if j != nil {
		t.Error("expected nil journal after remove")
	}
}

func TestJournal_ReadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	j, err := ReadJournal()
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if j != nil {
		t.Error("expected nil for non-existent journal")
	}
}

func TestHasIncompleteOperation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// No journal -- should be false.
	if HasIncompleteOperation() {
		t.Error("expected false with no journal")
	}

	// Write a journal -- should be true.
	journal := &Journal{
		Operation: "rename",
		Entries:   []JournalEntry{{Index: 0, SHA: "abc", Message: "test"}},
	}
	if err := WriteJournal(journal); err != nil {
		t.Fatal(err)
	}

	if !HasIncompleteOperation() {
		t.Error("expected true with journal present")
	}
}

func TestJournal_Entries_Roundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	entries := []JournalEntry{
		{Index: 0, SHA: "sha0", Message: "msg with \"quotes\" and\nnewlines"},
		{Index: 1, SHA: "sha1", Message: ""},
		{Index: 2, SHA: "sha2", Message: "normal message"},
	}

	journal := &Journal{
		Operation:  "rename",
		Entries:    entries,
		TargetIdx:  0,
		NewMsg:     "new msg with \"special\" chars",
		Step:       2,
		TotalSteps: 6,
	}

	if err := WriteJournal(journal); err != nil {
		t.Fatal(err)
	}

	got, err := ReadJournal()
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Entries) != len(entries) {
		t.Fatalf("entries count = %d, want %d", len(got.Entries), len(entries))
	}

	for i, e := range got.Entries {
		if e.SHA != entries[i].SHA {
			t.Errorf("entry[%d].SHA = %q, want %q", i, e.SHA, entries[i].SHA)
		}
		if e.Message != entries[i].Message {
			t.Errorf("entry[%d].Message = %q, want %q", i, e.Message, entries[i].Message)
		}
	}

	if got.Step != 2 {
		t.Errorf("Step = %d, want 2", got.Step)
	}
}
```

### Step 3: Create the rename plugin in `internal/plugins/rename/rename.go`

```go
package rename

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const PluginID = "rename"

// StartRenameMsg signals that inline rename mode should activate.
type StartRenameMsg struct {
	StashIndex int
	OldMessage string
}

// RenameCompleteMsg signals that a rename operation finished.
type RenameCompleteMsg struct {
	StashIndex int
	NewMessage string
}

// Plugin implements plugin.KeyHandler for the rename feature (PRD FR-13).
type Plugin struct {
	git    git.GitRunner
	cache  git.StashCache
	logger *slog.Logger
}

// New creates a new rename plugin.
func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return "Rename" }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.cache = ctx.Cache
	p.logger = ctx.Logger
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns the keybindings for the rename plugin.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{
			Key:         "r",
			Description: "Rename stash message",
			Modes:       []core.Mode{core.ModeList},
		},
	}
}

// HandleKey handles the `r` key press to start inline rename (FR-13.1).
func (p *Plugin) HandleKey(key plugin.KeyEvent, state core.AppState) (core.AppState, tea.Cmd) {
	if key.Text != "r" {
		return state, nil
	}

	if state.Cursor < 0 || state.Cursor >= len(state.Stashes) {
		return state, nil
	}

	stash := state.Stashes[state.Cursor]

	return state, func() tea.Msg {
		return StartRenameMsg{
			StashIndex: stash.Index,
			OldMessage: stash.Message,
		}
	}
}

// RenameStash performs the actual rename operation.
//
// For stash@{0} (top): simple drop + store.
// For stash@{n} where n > 0: must drop and re-store all stashes above
// to preserve ordering (FR-13.5). Uses journal for crash safety (FR-16.4).
func (p *Plugin) RenameStash(ctx context.Context, stashes []core.Stash, targetIdx int, newMsg string) error {
	if targetIdx < 0 || targetIdx >= len(stashes) {
		return fmt.Errorf("rename: index %d out of range (have %d stashes)", targetIdx, len(stashes))
	}

	target := stashes[targetIdx]

	// Simple case: renaming the top stash (index 0).
	if targetIdx == 0 {
		return p.renameTop(ctx, target, newMsg)
	}

	// Complex case: must reorder to preserve indices.
	return p.renameWithReorder(ctx, stashes, targetIdx, newMsg)
}

// renameTop renames stash@{0} -- the simplest case.
func (p *Plugin) renameTop(ctx context.Context, stash core.Stash, newMsg string) error {
	// Record journal for crash safety.
	journal := &Journal{
		Operation:  "rename",
		Entries:    []JournalEntry{{Index: 0, SHA: stash.SHA, Message: stash.Message}},
		TargetIdx:  0,
		NewMsg:     newMsg,
		Step:       0,
		TotalSteps: 2,
	}
	if err := WriteJournal(journal); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}

	// Step 1: Drop the stash.
	_, err := p.git.Run(ctx, "stash", "drop", "stash@{0}")
	if err != nil {
		return fmt.Errorf("drop stash@{0}: %w", err)
	}

	journal.Step = 1
	_ = WriteJournal(journal)

	// Step 2: Re-store with new message.
	_, err = p.git.Run(ctx, "stash", "store", "-m", newMsg, stash.SHA)
	if err != nil {
		return fmt.Errorf("store with new message: %w", err)
	}

	// Cleanup journal on success.
	_ = RemoveJournal()

	p.cache.Invalidate()

	p.logger.Info("stash renamed",
		"index", 0,
		"sha", stash.SHA,
		"old_msg", stash.Message,
		"new_msg", newMsg,
	)

	return nil
}

// renameWithReorder renames a stash at index > 0 by dropping all stashes
// from 0 to targetIdx and re-storing them in the correct order with the
// target stash having the new message. All other stashes keep their
// original messages and SHAs.
func (p *Plugin) renameWithReorder(ctx context.Context, stashes []core.Stash, targetIdx int, newMsg string) error {
	// Build journal entries for all stashes that will be affected.
	entries := make([]JournalEntry, targetIdx+1)
	for i := 0; i <= targetIdx; i++ {
		entries[i] = JournalEntry{
			Index:   i,
			SHA:     stashes[i].SHA,
			Message: stashes[i].Message,
		}
	}

	journal := &Journal{
		Operation:  "rename",
		Entries:    entries,
		TargetIdx:  targetIdx,
		NewMsg:     newMsg,
		Step:       0,
		TotalSteps: (targetIdx+1)*2 + 1, // drops + stores + cleanup
	}
	if err := WriteJournal(journal); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}

	// Phase 1: Drop all stashes from index 0 to targetIdx.
	// We always drop stash@{0} because indices shift as we drop.
	for i := 0; i <= targetIdx; i++ {
		_, err := p.git.Run(ctx, "stash", "drop", "stash@{0}")
		if err != nil {
			return fmt.Errorf("drop stash@{0} (step %d): %w", i, err)
		}
		journal.Step++
		_ = WriteJournal(journal)
	}

	// Phase 2: Re-store in reverse order (highest index first) so that
	// after all stores, index 0 is back at the top.
	for i := targetIdx; i >= 0; i-- {
		msg := entries[i].Message
		if i == targetIdx {
			msg = newMsg // Apply the renamed message to the target.
		}

		_, err := p.git.Run(ctx, "stash", "store", "-m", msg, entries[i].SHA)
		if err != nil {
			return fmt.Errorf("store stash (original index %d): %w", i, err)
		}
		journal.Step++
		_ = WriteJournal(journal)
	}

	// Cleanup journal on success.
	_ = RemoveJournal()

	p.cache.Invalidate()

	p.logger.Info("stash renamed with reorder",
		"target_index", targetIdx,
		"sha", stashes[targetIdx].SHA,
		"old_msg", stashes[targetIdx].Message,
		"new_msg", newMsg,
	)

	return nil
}

// RecoverFromJournal attempts to recover from an interrupted rename operation.
// It re-stores all stashes from the journal that were dropped but not yet
// re-stored. Returns the number of stashes recovered.
func RecoverFromJournal(ctx context.Context, runner git.GitRunner) (int, error) {
	journal, err := ReadJournal()
	if err != nil {
		return 0, err
	}
	if journal == nil {
		return 0, nil
	}

	// Determine which entries still need to be restored.
	// The journal records how many steps completed. The first N steps
	// are drops, the remaining steps are stores.
	dropsDone := journal.Step
	if dropsDone > len(journal.Entries) {
		dropsDone = len(journal.Entries)
	}
	storesDone := 0
	if journal.Step > len(journal.Entries) {
		storesDone = journal.Step - len(journal.Entries)
	}

	recovered := 0

	// Re-store entries that were dropped but not yet stored.
	// Entries are stored in reverse order (highest index first).
	for i := len(journal.Entries) - 1 - storesDone; i >= 0; i-- {
		entry := journal.Entries[i]
		msg := entry.Message
		if i == journal.TargetIdx {
			msg = journal.NewMsg
		}

		_, err := runner.Run(ctx, "stash", "store", "-m", msg, entry.SHA)
		if err != nil {
			return recovered, fmt.Errorf("recover store %s: %w", entry.SHA[:8], err)
		}
		recovered++
	}

	_ = RemoveJournal()
	return recovered, nil
}

// ListStashSHAs returns all current stash SHAs for verification.
func ListStashSHAs(ctx context.Context, runner git.GitRunner) ([]string, error) {
	lines, err := runner.RunLines(ctx, "stash", "list", "--format=%H")
	if err != nil {
		return nil, err
	}

	var shas []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			shas = append(shas, line)
		}
	}
	return shas, nil
}
```

### Step 4: Create rename integration tests in `internal/plugins/rename/rename_test.go`

```go
package rename_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugins/rename"
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

func stashList(t *testing.T, dir string) []string {
	t.Helper()
	out := strings.TrimSpace(run(t, dir, "git", "stash", "list", "--format=%gs"))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func stashSHAs(t *testing.T, dir string) []string {
	t.Helper()
	out := strings.TrimSpace(run(t, dir, "git", "stash", "list", "--format=%H"))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// buildStashes creates a set of test stashes with known messages and returns
// a slice of core.Stash structs matching the current state.
func buildStashes(t *testing.T, dir string, messages []string) []core.Stash {
	t.Helper()
	for _, msg := range messages {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		run(t, dir, "git", "add", ".")
		run(t, dir, "git", "stash", "push", "-m", msg)
	}

	// Build core.Stash slice from current state.
	list := stashList(t, dir)
	shas := stashSHAs(t, dir)
	stashes := make([]core.Stash, len(list))
	for i := range list {
		stashes[i] = core.Stash{
			Index:   i,
			SHA:     shas[i],
			Message: list[i],
		}
	}
	return stashes
}

func TestRenameTop_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	stashes := buildStashes(t, dir, []string{"first", "second", "third"})

	// Stash order after creation (LIFO): third(0), second(1), first(2)
	// Rename stash@{0} (the top stash = "third").
	runner := git.NewRunner(dir)
	p := rename.New()
	p.Init(plugin.PluginContext{
		Git:    runner,
		Cache:  &noopCache{},
		Logger: slog.Default(),
	})

	originalSHA := stashes[0].SHA

	err := p.RenameStash(context.Background(), stashes, 0, "renamed third")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	// Verify: stash@{0} has new message.
	list := stashList(t, dir)
	if len(list) != 3 {
		t.Fatalf("stash count = %d, want 3", len(list))
	}
	if !strings.Contains(list[0], "renamed third") {
		t.Errorf("stash@{0} message = %q, want 'renamed third'", list[0])
	}

	// Verify: SHA is preserved.
	shas := stashSHAs(t, dir)
	if shas[0] != originalSHA {
		t.Errorf("stash@{0} SHA changed: %s -> %s", originalSHA[:8], shas[0][:8])
	}

	// Verify: journal is cleaned up.
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

	runner := git.NewRunner(dir)
	p := rename.New()
	p.Init(plugin.PluginContext{
		Git:    runner,
		Cache:  &noopCache{},
		Logger: slog.Default(),
	})

	// Record all SHAs before rename.
	originalSHAs := stashSHAs(t, dir)

	// Rename stash@{1} (beta -> "renamed beta").
	err := p.RenameStash(context.Background(), stashes, 1, "renamed beta")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	// Verify: 3 stashes still exist.
	list := stashList(t, dir)
	if len(list) != 3 {
		t.Fatalf("stash count = %d, want 3", len(list))
	}

	// Verify: stash@{1} has the new message.
	if !strings.Contains(list[1], "renamed beta") {
		t.Errorf("stash@{1} message = %q, want 'renamed beta'", list[1])
	}

	// Verify: stash@{0} and stash@{2} keep their original messages.
	if !strings.Contains(list[0], "gamma") {
		t.Errorf("stash@{0} message = %q, want 'gamma'", list[0])
	}
	if !strings.Contains(list[2], "alpha") {
		t.Errorf("stash@{2} message = %q, want 'alpha'", list[2])
	}

	// Verify: all SHAs are preserved.
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

	messages := []string{"one", "two", "three", "four", "five"}
	stashes := buildStashes(t, dir, messages)
	// Order: five(0), four(1), three(2), two(3), one(4)

	runner := git.NewRunner(dir)
	p := rename.New()
	p.Init(plugin.PluginContext{
		Git:    runner,
		Cache:  &noopCache{},
		Logger: slog.Default(),
	})

	originalSHAs := stashSHAs(t, dir)

	// Rename stash@{2} (three -> "tres").
	err := p.RenameStash(context.Background(), stashes, 2, "tres")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	list := stashList(t, dir)
	if len(list) != 5 {
		t.Fatalf("stash count = %d, want 5", len(list))
	}

	// Verify ordering and messages.
	expected := []string{"five", "four", "tres", "two", "one"}
	for i, want := range expected {
		if !strings.Contains(list[i], want) {
			t.Errorf("stash@{%d} = %q, want to contain %q", i, list[i], want)
		}
	}

	// Verify SHAs preserved.
	newSHAs := stashSHAs(t, dir)
	for i := range originalSHAs {
		if newSHAs[i] != originalSHAs[i] {
			t.Errorf("stash@{%d} SHA changed", i)
		}
	}
}

func TestRenameJournalRecovery(t *testing.T) {
	// This tests the journal recovery mechanism by simulating
	// a crash after drops but before stores.
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	dir := setupTestRepo(t)
	stashes := buildStashes(t, dir, []string{"first", "second"})
	// Order: second(0), first(1)

	// Write a journal as if we dropped stash@{0} and stash@{1} to rename first(1)
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
	run(t, dir, "git", "stash", "drop", "stash@{0}")
	run(t, dir, "git", "stash", "drop", "stash@{0}")

	// Verify stashes are gone.
	list := stashList(t, dir)
	if len(list) != 0 {
		t.Fatalf("expected 0 stashes after simulated crash, got %d", len(list))
	}

	// Run recovery.
	runner := git.NewRunner(dir)
	recovered, err := rename.RecoverFromJournal(context.Background(), runner)
	if err != nil {
		t.Fatalf("RecoverFromJournal: %v", err)
	}

	if recovered != 2 {
		t.Errorf("recovered = %d, want 2", recovered)
	}

	// Verify stashes are restored.
	list = stashList(t, dir)
	if len(list) != 2 {
		t.Fatalf("stash count after recovery = %d, want 2", len(list))
	}
}

// noopCache is a stub for testing that does nothing.
type noopCache struct{}

func (c *noopCache) List(ctx context.Context) ([]core.Stash, error) { return nil, nil }
func (c *noopCache) Diff(ctx context.Context, sha string) (string, error) { return "", nil }
func (c *noopCache) Invalidate() {}
```

### Step 5: Wire the rename plugin into the plugin loader

Add to `internal/plugin/loader.go`:

```go
import "github.com/indrasvat/nidhi/internal/plugins/rename"

// In LoadBuiltinPlugins():
rename.New(),
```

### Step 6: Verify

```bash
# Journal unit tests
go test -v -run TestJournal ./internal/plugins/rename/...

# Integration: rename top stash
go test -v -run TestRenameTop_Integration ./internal/plugins/rename/...

# Integration: rename middle stash with reorder
go test -v -run TestRenameMiddle_Integration ./internal/plugins/rename/...

# Integration: ordering and SHA preservation
go test -v -run TestRenamePreservesOrdering ./internal/plugins/rename/...

# Integration: journal recovery
go test -v -run TestRenameJournalRecovery ./internal/plugins/rename/...

# Full CI
make ci
```

## Verification

### Functional
```bash
# Unit: journal write/read/cleanup cycle
go test -v -run TestJournal_WriteReadCleanup ./internal/plugins/rename/...
# Expected: journal persists and reads back correctly, removed cleanly

# Unit: journal handles non-existent file
go test -v -run TestJournal_ReadNonExistent ./internal/plugins/rename/...
# Expected: returns nil, no error

# Unit: journal entries roundtrip with special chars
go test -v -run TestJournal_Entries_Roundtrip ./internal/plugins/rename/...
# Expected: quotes, newlines, empty strings preserved

# Integration: rename stash@{0} (simple case)
go test -v -run TestRenameTop_Integration ./internal/plugins/rename/...
# Expected: message changed, SHA preserved, 3 stashes remain

# Integration: rename stash@{1} (reorder case)
go test -v -run TestRenameMiddle_Integration ./internal/plugins/rename/...
# Expected: message changed, ordering intact, all SHAs preserved

# Integration: rename with 5 stashes, verify full ordering
go test -v -run TestRenamePreservesOrdering_Integration ./internal/plugins/rename/...
# Expected: all 5 stashes in correct order, only target message changed

# Integration: journal recovery after simulated crash
go test -v -run TestRenameJournalRecovery ./internal/plugins/rename/...
# Expected: 2 stashes recovered from journal

# Full CI
make ci
```

## Completion Criteria
1. `r` key activates inline rename mode on the selected stash row (textinput.Model from Bubbles v2)
2. Previous message shown dimmed for reference (FR-13.2)
3. Enter saves: performs drop+store sequence, preserving SHA (FR-13.4)
4. Esc cancels rename without any git operations
5. Stash at index 0: simple drop+store (2 git commands)
6. Stash at index > 0: drops all stashes from 0..n, re-stores in reverse order, preserving ordering (FR-13.5)
7. All SHAs are preserved across rename (including reorder)
8. Reorder journal written to `~/.local/state/nidhi/reorder-journal.json` before operation
9. Journal enables recovery from interrupted operations on next startup (FR-16.4)
10. All tests pass including integration tests with real git repos using `t.TempDir()`
11. `make ci` passes

## Commit
```
feat(rename): add inline rename plugin with reorder journal

Implement FR-13 rename plugin with inline text editing via `r` key.
Simple drop+store for stash@{0}; multi-step reorder with journal
for deeper stashes (FR-13.5). Journal enables crash recovery
(FR-16.4) by recording all SHAs before the operation. SHAs are
preserved across renames.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.2 (FR-13, FR-16.4), 8.2 (KeyHandler), 10 (Screen 8)
4. Read tasks 008 and 013 to understand dependencies
5. Execute steps 1-6 in order
6. Verify all functional checks pass
7. Update this file (Status: DONE) + `docs/PROGRESS.md`
8. Commit with the message above + move to next task
