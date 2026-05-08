# Task 013: Stash CRUD Operations

## Status: TODO

## Depends On
- 001 (GitRunner — `internal/git/runner.go`)
- 004 (StashCache — `internal/git/cache.go`)

## Parallelizable With
- 010 (LIST screen), 011 (PREVIEW screen), 012 (DETAIL screen) — all can be developed in parallel since screens dispatch commands that this task implements

## Problem
nidhi needs functions that execute the core git stash CRUD operations: Apply, Pop, Drop, Push (new stash), BranchFromStash, and ClearAll (PRD §6.1 FR-02). Each operation wraps git plumbing commands, captures the stash SHA before destructive actions (for undo support in Phase 2), and invalidates the stash cache after mutations. Every function must be tested against real temporary git repositories to ensure correctness — no mocking git behavior.

## PRD Reference
- Section 5.1 — Core stash operation signatures
- Section 6.1 FR-02 — CRUD requirements (FR-02.1 through FR-02.6)
- Section 8.3 — `GitRunner` interface
- Section 14.2 — Apply/Pop < 200ms, Rename < 100ms, Undo < 50ms
- Section 15.3 — Git command timeout (10s default)
- Section 16.2 — Integration tests with temp git repos

## Files to Create
- `internal/git/operations.go` — CRUD operation functions
- `internal/git/operations_test.go` — integration tests with real git repos

## Execution Steps

### Step 1: Define the operations interface and result types

```go
// internal/git/operations.go
package git

import (
	"context"
	"fmt"
	"strings"
)

// OperationResult captures the outcome of a stash operation.
type OperationResult struct {
	// Success indicates whether the operation completed without error.
	Success bool
	// SHA is the commit SHA of the affected stash (for undo support).
	SHA string
	// Message is the stash message (for undo display).
	Message string
	// Error is the git error output if the operation failed.
	Error string
}

// PushOptions configures the behavior of a new stash push.
type PushOptions struct {
	// Message is the stash message (required — nidhi is message-first).
	Message string
	// KeepIndex preserves staged changes in the working tree.
	KeepIndex bool
	// IncludeUntracked includes untracked files in the stash.
	IncludeUntracked bool
	// Staged stashes only the staged changes (Git 2.35+).
	Staged bool
	// Pathspecs limits the stash to specific file patterns.
	Pathspecs []string
}

// StashOps provides git stash CRUD operations.
// All methods take a context for timeout support and use GitRunner
// for command execution.
type StashOps struct {
	runner GitRunner
	cache  StashCache
}

// NewStashOps creates a new StashOps instance.
func NewStashOps(runner GitRunner, cache StashCache) *StashOps {
	return &StashOps{
		runner: runner,
		cache:  cache,
	}
}
```

### Step 2: Implement Apply

```go
// Apply applies the stash at the given index to the working tree.
// The stash is preserved in the list (FR-02.1).
// Returns the SHA of the applied stash for reference.
func (s *StashOps) Apply(ctx context.Context, index int) (OperationResult, error) {
	// Get the SHA before applying (for undo/reference).
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}

	// Get the message for display.
	msg, _ := s.getStashMessage(ctx, index)

	// Apply the stash.
	ref := fmt.Sprintf("stash@{%d}", index)
	output, err := s.runner.Run(ctx, "stash", "apply", ref)
	if err != nil {
		return OperationResult{
			Success: false,
			SHA:     sha,
			Message: msg,
			Error:   output,
		}, fmt.Errorf("git stash apply %s: %w", ref, err)
	}

	// Do NOT invalidate cache — apply does not modify the stash list.
	return OperationResult{
		Success: true,
		SHA:     sha,
		Message: msg,
	}, nil
}
```

### Step 3: Implement Pop

```go
// Pop applies and removes the stash at the given index (FR-02.2).
// Captures the SHA before popping for potential undo.
func (s *StashOps) Pop(ctx context.Context, index int) (OperationResult, error) {
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}

	msg, _ := s.getStashMessage(ctx, index)

	ref := fmt.Sprintf("stash@{%d}", index)
	output, err := s.runner.Run(ctx, "stash", "pop", ref)
	if err != nil {
		return OperationResult{
			Success: false,
			SHA:     sha,
			Message: msg,
			Error:   output,
		}, fmt.Errorf("git stash pop %s: %w", ref, err)
	}

	// Invalidate cache — stash list has changed.
	s.cache.Invalidate()

	return OperationResult{
		Success: true,
		SHA:     sha,
		Message: msg,
	}, nil
}
```

### Step 4: Implement Drop

```go
// Drop removes the stash at the given index without applying (FR-02.3).
// Returns the SHA so the caller can display an undo toast.
func (s *StashOps) Drop(ctx context.Context, index int) (OperationResult, error) {
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}

	msg, _ := s.getStashMessage(ctx, index)

	ref := fmt.Sprintf("stash@{%d}", index)
	output, err := s.runner.Run(ctx, "stash", "drop", ref)
	if err != nil {
		return OperationResult{
			Success: false,
			SHA:     sha,
			Message: msg,
			Error:   output,
		}, fmt.Errorf("git stash drop %s: %w", ref, err)
	}

	s.cache.Invalidate()

	return OperationResult{
		Success: true,
		SHA:     sha,
		Message: msg,
	}, nil
}
```

### Step 5: Implement Push (new stash)

```go
// Push creates a new stash with the given options (FR-02.4).
func (s *StashOps) Push(ctx context.Context, opts PushOptions) (OperationResult, error) {
	args := []string{"stash", "push"}

	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}
	if opts.KeepIndex {
		args = append(args, "--keep-index")
	}
	if opts.IncludeUntracked {
		args = append(args, "--include-untracked")
	}
	if opts.Staged {
		args = append(args, "--staged")
	}

	// Pathspecs go after "--".
	if len(opts.Pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, opts.Pathspecs...)
	}

	output, err := s.runner.Run(ctx, args...)
	if err != nil {
		return OperationResult{
			Success: false,
			Error:   output,
		}, fmt.Errorf("git stash push: %w", err)
	}

	s.cache.Invalidate()

	// Get the SHA of the newly created stash (now at index 0).
	sha, _ := s.getStashSHA(ctx, 0)

	return OperationResult{
		Success: true,
		SHA:     sha,
		Message: opts.Message,
	}, nil
}
```

### Step 6: Implement BranchFromStash

```go
// BranchFromStash creates a new branch from the stash and checks it out (FR-02.5).
// This applies the stash, creates the branch, and removes the stash entry.
func (s *StashOps) BranchFromStash(ctx context.Context, index int, branchName string) (OperationResult, error) {
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}

	msg, _ := s.getStashMessage(ctx, index)

	ref := fmt.Sprintf("stash@{%d}", index)
	output, err := s.runner.Run(ctx, "stash", "branch", branchName, ref)
	if err != nil {
		return OperationResult{
			Success: false,
			SHA:     sha,
			Message: msg,
			Error:   output,
		}, fmt.Errorf("git stash branch %s %s: %w", branchName, ref, err)
	}

	s.cache.Invalidate()

	return OperationResult{
		Success: true,
		SHA:     sha,
		Message: msg,
	}, nil
}
```

### Step 7: Implement ClearAll

```go
// ClearAllResult captures all SHAs before clearing, for potential bulk undo.
type ClearAllResult struct {
	// Entries records every stash that was cleared, with SHA and message.
	Entries []OperationResult
}

// ClearAll drops all stashes (FR-02.6).
// Captures all SHAs and messages BEFORE clearing so bulk undo is possible.
func (s *StashOps) ClearAll(ctx context.Context) (ClearAllResult, error) {
	// First, capture all stash SHAs and messages.
	output, err := s.runner.Run(ctx, "stash", "list", "--format=%H %s")
	if err != nil {
		return ClearAllResult{}, fmt.Errorf("git stash list: %w", err)
	}

	var entries []OperationResult
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		sha := parts[0]
		msg := ""
		if len(parts) > 1 {
			msg = parts[1]
		}
		entries = append(entries, OperationResult{
			Success: true,
			SHA:     sha,
			Message: msg,
		})
	}

	// Now clear all stashes.
	_, err = s.runner.Run(ctx, "stash", "clear")
	if err != nil {
		return ClearAllResult{}, fmt.Errorf("git stash clear: %w", err)
	}

	s.cache.Invalidate()

	return ClearAllResult{Entries: entries}, nil
}
```

### Step 8: Implement helper methods

```go
// getStashSHA returns the full commit SHA for a stash at the given index.
func (s *StashOps) getStashSHA(ctx context.Context, index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	output, err := s.runner.Run(ctx, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(output), nil
}

// getStashMessage returns the message for a stash at the given index.
func (s *StashOps) getStashMessage(ctx context.Context, index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	output, err := s.runner.Run(ctx, "stash", "list", "--format=%s", "-1",
		fmt.Sprintf("--skip=%d", index))
	if err != nil {
		return "", fmt.Errorf("get stash message: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// RestoreStash re-stores a previously dropped stash using its SHA and message.
// Used by the undo system to recover dropped stashes.
func (s *StashOps) RestoreStash(ctx context.Context, sha, message string) error {
	args := []string{"stash", "store"}
	if message != "" {
		args = append(args, "-m", message)
	}
	args = append(args, sha)

	_, err := s.runner.Run(ctx, args...)
	if err != nil {
		return fmt.Errorf("git stash store: %w", err)
	}

	s.cache.Invalidate()
	return nil
}
```

### Step 9: Write comprehensive integration tests

```go
// internal/git/operations_test.go
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testRunner is a real GitRunner that executes git commands in a temp dir.
type testRunner struct {
	dir string
}

func (r *testRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *testRunner) RunLines(ctx context.Context, args ...string) ([]string, error) {
	out, err := r.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func (r *testRunner) RunExitCode(ctx context.Context, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode(), nil
		}
		return string(out), -1, err
	}
	return string(out), 0, nil
}

// testCache is a minimal StashCache for testing that just tracks invalidation.
type testCache struct {
	invalidated bool
}

func (c *testCache) List(ctx context.Context) ([]Stash, error) { return nil, nil }
func (c *testCache) Diff(ctx context.Context, sha string) (string, error) {
	return "", nil
}
func (c *testCache) Invalidate() { c.invalidated = true }

// setupTestRepo creates a git repo in a temp dir with an initial commit
// and optionally N stashes with known content.
func setupTestRepo(t *testing.T, numStashes int) (string, *testRunner) {
	t.Helper()
	dir := t.TempDir()
	runner := &testRunner{dir: dir}
	ctx := context.Background()

	mustRun := func(args ...string) {
		t.Helper()
		_, err := runner.Run(ctx, args...)
		if err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	mustRun("init")
	mustRun("config", "user.email", "test@test.com")
	mustRun("config", "user.name", "Test")

	// Initial commit (required for stash to work).
	writeTestFile(t, dir, "README.md", "# test repo\n")
	mustRun("add", ".")
	mustRun("commit", "-m", "initial commit")

	// Create N stashes with distinct content.
	for i := range numStashes {
		filename := fmt.Sprintf("stash_file_%d.go", i)
		content := fmt.Sprintf("package main\n\n// Stash %d content\nfunc stash%d() {}\n", i, i)
		writeTestFile(t, dir, filename, content)
		mustRun("add", ".")
		mustRun("stash", "push", "-m", fmt.Sprintf("stash message %d", i))
	}

	return dir, runner
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stashCount returns the number of stashes in the repo.
func stashCount(t *testing.T, runner *testRunner) int {
	t.Helper()
	ctx := context.Background()
	lines, err := runner.RunLines(ctx, "stash", "list")
	if err != nil {
		t.Fatalf("stash list failed: %v", err)
	}
	return len(lines)
}

// --- Tests ---

func TestApply(t *testing.T) {
	dir, runner := setupTestRepo(t, 1)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	result, err := ops.Apply(ctx, 0)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Apply should succeed, got error: %s", result.Error)
	}
	if result.SHA == "" {
		t.Error("Apply should return the stash SHA")
	}

	// Verify the stash is still in the list (apply preserves stash).
	count := stashCount(t, runner)
	if count != 1 {
		t.Errorf("stash count = %d, want 1 (apply preserves stash)", count)
	}

	// Verify the file was applied to working tree.
	path := filepath.Join(dir, "stash_file_0.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading applied file: %v", err)
	}
	if !strings.Contains(string(content), "Stash 0") {
		t.Error("applied file should contain stash content")
	}

	// Apply should NOT invalidate cache.
	if cache.invalidated {
		t.Error("Apply should not invalidate cache (stash list unchanged)")
	}
}

func TestPop(t *testing.T) {
	_, runner := setupTestRepo(t, 1)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	result, err := ops.Pop(ctx, 0)
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Pop should succeed, got error: %s", result.Error)
	}
	if result.SHA == "" {
		t.Error("Pop should return the stash SHA")
	}

	// Verify the stash is REMOVED from the list.
	count := stashCount(t, runner)
	if count != 0 {
		t.Errorf("stash count = %d, want 0 (pop removes stash)", count)
	}

	// Pop MUST invalidate cache.
	if !cache.invalidated {
		t.Error("Pop should invalidate cache")
	}
}

func TestDrop(t *testing.T) {
	_, runner := setupTestRepo(t, 3)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Drop the middle stash (index 1).
	result, err := ops.Drop(ctx, 1)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Drop should succeed")
	}
	if result.SHA == "" {
		t.Error("Drop should return the SHA for undo")
	}

	// Verify stash count decreased.
	count := stashCount(t, runner)
	if count != 2 {
		t.Errorf("stash count = %d, want 2 after dropping 1 of 3", count)
	}

	// Drop MUST invalidate cache.
	if !cache.invalidated {
		t.Error("Drop should invalidate cache")
	}
}

func TestDrop_ReturnsCorrectSHA(t *testing.T) {
	_, runner := setupTestRepo(t, 2)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Get the SHA of stash@{0} before dropping.
	expectedSHA, err := runner.Run(ctx, "rev-parse", "stash@{0}")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	expectedSHA = strings.TrimSpace(expectedSHA)

	result, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}

	if result.SHA != expectedSHA {
		t.Errorf("SHA = %q, want %q", result.SHA, expectedSHA)
	}
}

func TestPush(t *testing.T) {
	dir, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Create a file to stash.
	writeTestFile(t, dir, "new_feature.go", "package main\n\nfunc Feature() {}\n")
	runner.Run(ctx, "add", ".")

	result, err := ops.Push(ctx, PushOptions{
		Message: "my custom stash message",
	})
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Push should succeed, got error: %s", result.Error)
	}

	// Verify the stash was created.
	count := stashCount(t, runner)
	if count != 1 {
		t.Errorf("stash count = %d, want 1 after push", count)
	}

	// Verify the message is in the stash list.
	output, _ := runner.Run(ctx, "stash", "list")
	if !strings.Contains(output, "my custom stash message") {
		t.Errorf("stash list should contain custom message, got: %s", output)
	}

	// Push MUST invalidate cache.
	if !cache.invalidated {
		t.Error("Push should invalidate cache")
	}
}

func TestPush_WithKeepIndex(t *testing.T) {
	dir, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	writeTestFile(t, dir, "keep_me.go", "package main\n")
	runner.Run(ctx, "add", ".")

	_, err := ops.Push(ctx, PushOptions{
		Message:   "keep index test",
		KeepIndex: true,
	})
	if err != nil {
		t.Fatalf("Push with --keep-index failed: %v", err)
	}

	// With --keep-index, the staged changes should still be in the index.
	output, _ := runner.Run(ctx, "diff", "--cached", "--name-only")
	if !strings.Contains(output, "keep_me.go") {
		t.Error("--keep-index should preserve staged files in the index")
	}
}

func TestPush_WithIncludeUntracked(t *testing.T) {
	dir, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Create an untracked file (NOT added to index).
	writeTestFile(t, dir, "untracked.txt", "untracked content\n")

	// Also need a tracked modification for stash to work.
	writeTestFile(t, dir, "README.md", "# updated\n")

	_, err := ops.Push(ctx, PushOptions{
		Message:          "include untracked",
		IncludeUntracked: true,
	})
	if err != nil {
		t.Fatalf("Push with --include-untracked failed: %v", err)
	}

	// The untracked file should be gone from working tree.
	path := filepath.Join(dir, "untracked.txt")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("untracked file should be removed after stash push --include-untracked")
	}
}

func TestBranchFromStash(t *testing.T) {
	_, runner := setupTestRepo(t, 1)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	branchName := "feature/from-stash"
	result, err := ops.BranchFromStash(ctx, 0, branchName)
	if err != nil {
		t.Fatalf("BranchFromStash failed: %v", err)
	}
	if !result.Success {
		t.Errorf("BranchFromStash should succeed, got error: %s", result.Error)
	}

	// Verify we are now on the new branch.
	output, _ := runner.Run(ctx, "branch", "--show-current")
	currentBranch := strings.TrimSpace(output)
	if currentBranch != branchName {
		t.Errorf("current branch = %q, want %q", currentBranch, branchName)
	}

	// Verify the stash was removed (git stash branch removes it).
	count := stashCount(t, runner)
	if count != 0 {
		t.Errorf("stash count = %d, want 0 (branch removes stash)", count)
	}

	// BranchFromStash MUST invalidate cache.
	if !cache.invalidated {
		t.Error("BranchFromStash should invalidate cache")
	}
}

func TestClearAll(t *testing.T) {
	_, runner := setupTestRepo(t, 5)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	result, err := ops.ClearAll(ctx)
	if err != nil {
		t.Fatalf("ClearAll failed: %v", err)
	}

	// Verify all SHAs were captured.
	if len(result.Entries) != 5 {
		t.Errorf("captured %d entries, want 5", len(result.Entries))
	}

	// Each entry should have a non-empty SHA.
	for i, entry := range result.Entries {
		if entry.SHA == "" {
			t.Errorf("entry[%d].SHA is empty", i)
		}
	}

	// Verify all stashes are gone.
	count := stashCount(t, runner)
	if count != 0 {
		t.Errorf("stash count = %d, want 0 after ClearAll", count)
	}

	// ClearAll MUST invalidate cache.
	if !cache.invalidated {
		t.Error("ClearAll should invalidate cache")
	}
}

func TestClearAll_EmptyRepo(t *testing.T) {
	_, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	result, err := ops.ClearAll(ctx)
	if err != nil {
		t.Fatalf("ClearAll on empty repo failed: %v", err)
	}

	if len(result.Entries) != 0 {
		t.Errorf("captured %d entries, want 0 for empty repo", len(result.Entries))
	}
}

func TestRestoreStash(t *testing.T) {
	_, runner := setupTestRepo(t, 1)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Drop the stash and capture SHA.
	result, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}

	// Verify stash is gone.
	count := stashCount(t, runner)
	if count != 0 {
		t.Fatalf("stash should be gone after drop, got %d", count)
	}

	// Restore it using the captured SHA.
	cache.invalidated = false
	err = ops.RestoreStash(ctx, result.SHA, "restored stash")
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}

	// Verify stash is back.
	count = stashCount(t, runner)
	if count != 1 {
		t.Errorf("stash count = %d, want 1 after restore", count)
	}

	// Verify the restored message.
	output, _ := runner.Run(ctx, "stash", "list")
	if !strings.Contains(output, "restored stash") {
		t.Errorf("restored stash should have custom message, got: %s", output)
	}

	// Restore MUST invalidate cache.
	if !cache.invalidated {
		t.Error("RestoreStash should invalidate cache")
	}
}

func TestApply_ConflictingWorkingTree(t *testing.T) {
	dir, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Create a file and stash it.
	writeTestFile(t, dir, "conflict.go", "package main\n\nfunc A() {}\n")
	runner.Run(ctx, "add", ".")
	runner.Run(ctx, "stash", "push", "-m", "will conflict")

	// Now create conflicting content in the working tree.
	writeTestFile(t, dir, "conflict.go", "package main\n\nfunc B() {}\n")
	runner.Run(ctx, "add", ".")
	runner.Run(ctx, "commit", "-m", "conflicting change")

	// Modify the same file again so apply has something to conflict with.
	writeTestFile(t, dir, "conflict.go", "package main\n\nfunc C() {}\n")

	// Apply should fail due to conflict.
	_, err := ops.Apply(ctx, 0)
	if err == nil {
		t.Error("Apply should fail on conflicting working tree")
	}
}

func TestPop_NonExistentStash(t *testing.T) {
	_, runner := setupTestRepo(t, 1)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Try to pop index 5 when only index 0 exists.
	_, err := ops.Pop(ctx, 5)
	if err == nil {
		t.Error("Pop should fail for non-existent stash index")
	}
}

func TestDrop_NonExistentStash(t *testing.T) {
	_, runner := setupTestRepo(t, 1)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	_, err := ops.Drop(ctx, 99)
	if err == nil {
		t.Error("Drop should fail for non-existent stash index")
	}
}

func TestPush_NoChanges(t *testing.T) {
	_, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Push with no changes should fail.
	_, err := ops.Push(ctx, PushOptions{Message: "nothing to stash"})
	if err == nil {
		t.Error("Push should fail when there are no changes to stash")
	}
}

func TestOperations_Timeout(t *testing.T) {
	_, runner := setupTestRepo(t, 1)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)

	// Use an already-cancelled context.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ensure timeout has passed

	_, err := ops.Apply(ctx, 0)
	if err == nil {
		t.Error("Apply should fail with cancelled context")
	}
}

func TestPush_WithPathspecs(t *testing.T) {
	dir, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Create two files but only stash one.
	writeTestFile(t, dir, "include.go", "package main\nfunc Include() {}\n")
	writeTestFile(t, dir, "exclude.go", "package main\nfunc Exclude() {}\n")
	runner.Run(ctx, "add", ".")

	_, err := ops.Push(ctx, PushOptions{
		Message:   "partial stash",
		Pathspecs: []string{"include.go"},
	})
	if err != nil {
		t.Fatalf("Push with pathspecs failed: %v", err)
	}

	// The excluded file should still be in the working tree / index.
	output, _ := runner.Run(ctx, "diff", "--cached", "--name-only")
	if !strings.Contains(output, "exclude.go") {
		t.Error("exclude.go should still be staged after partial stash")
	}
}

func TestMultipleOperations_Sequence(t *testing.T) {
	dir, runner := setupTestRepo(t, 0)
	cache := &testCache{}
	ops := NewStashOps(runner, cache)
	ctx := context.Background()

	// Create 3 stashes.
	for i := range 3 {
		writeTestFile(t, dir, fmt.Sprintf("seq_%d.go", i),
			fmt.Sprintf("package main\nfunc Seq%d() {}\n", i))
		runner.Run(ctx, "add", ".")
		_, err := ops.Push(ctx, PushOptions{
			Message: fmt.Sprintf("sequence %d", i),
		})
		if err != nil {
			t.Fatalf("Push %d failed: %v", i, err)
		}
	}

	// Should have 3 stashes.
	count := stashCount(t, runner)
	if count != 3 {
		t.Fatalf("stash count = %d, want 3", count)
	}

	// Drop the newest (index 0).
	dropResult, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}

	// Apply the new top (was index 1, now index 0).
	_, err = ops.Apply(ctx, 0)
	if err != nil {
		t.Fatalf("Apply after drop failed: %v", err)
	}

	// Restore the dropped stash.
	err = ops.RestoreStash(ctx, dropResult.SHA, dropResult.Message)
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}

	// Should be back to 3 stashes.
	count = stashCount(t, runner)
	if count != 3 {
		t.Errorf("stash count = %d, want 3 after restore", count)
	}
}
```

### Step 10: Verify build and tests

```bash
gofmt -w internal/git/operations.go internal/git/operations_test.go
go vet ./internal/git/...
go test -v -race -count=1 ./internal/git/...
make ci
```

## Verification

### Functional
```bash
# Operations compile
go build ./internal/git/...

# All tests pass (these are all integration tests with real git repos)
go test -v -race -count=1 ./internal/git/...

# Individual operation tests
go test -v -run TestApply ./internal/git/...
go test -v -run TestPop ./internal/git/...
go test -v -run TestDrop ./internal/git/...
go test -v -run TestPush ./internal/git/...
go test -v -run TestBranchFromStash ./internal/git/...
go test -v -run TestClearAll ./internal/git/...
go test -v -run TestRestoreStash ./internal/git/...

# Error case tests
go test -v -run TestApply_ConflictingWorkingTree ./internal/git/...
go test -v -run TestPop_NonExistentStash ./internal/git/...
go test -v -run TestDrop_NonExistentStash ./internal/git/...
go test -v -run TestPush_NoChanges ./internal/git/...

# Sequence test (create, drop, apply, restore)
go test -v -run TestMultipleOperations_Sequence ./internal/git/...

# Timeout test
go test -v -run TestOperations_Timeout ./internal/git/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/git/operations.go` implements `StashOps` with all 6 CRUD operations + `RestoreStash`
2. `Apply`: applies stash, preserves it in list, does NOT invalidate cache
3. `Pop`: applies + removes stash, returns SHA, invalidates cache
4. `Drop`: removes stash, returns SHA for undo, invalidates cache
5. `Push`: creates new stash with message and options (keep-index, include-untracked, staged, pathspecs)
6. `BranchFromStash`: creates branch, checks it out, removes stash, invalidates cache
7. `ClearAll`: captures all SHAs before clearing, returns `ClearAllResult`, invalidates cache
8. `RestoreStash`: re-stores a dropped stash using `git stash store`, invalidates cache
9. All operations capture SHA before destructive actions (for Phase 2 undo)
10. All operations accept `context.Context` for timeout support
11. Every test creates a fresh temp repo with `t.TempDir()` and runs real git commands
12. All 17+ tests pass: Apply (1), Pop (1), Drop (2), Push (4), BranchFromStash (1), ClearAll (2), RestoreStash (1), error cases (4), timeout (1), multi-op sequence (1)
13. `make ci` passes

## Commit
```
feat: implement git stash CRUD operations with real-repo integration tests

Add internal/git/operations.go with StashOps providing Apply, Pop, Drop,
Push, BranchFromStash, ClearAll, and RestoreStash. All operations capture
stash SHAs before destructive actions (for undo support) and invalidate
the cache after mutations. Comprehensive integration tests create fresh
temp git repos for each test — no mocking. Covers success paths, error
cases (conflicts, non-existent stashes, no changes), and multi-operation
sequences.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 5.1, 6.1 FR-02, 8.3 (GitRunner), 14.2, 15.3, 16.2
4. Read tasks 001, 004 to understand GitRunner and StashCache interfaces
5. Implement `operations.go` following execution steps 1-8
6. Implement `operations_test.go` following execution step 9
7. Run `go vet`, `go test -v -race -count=1`, `make ci`
8. Update this file (Status: DONE) + `docs/PROGRESS.md`
9. Commit with the message above
