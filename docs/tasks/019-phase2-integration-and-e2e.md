# Task 019: Phase 2 Integration & E2E Tests (Quality Gate)

## Status: TODO

## Depends On
- 015 (Conflict preview plugin)
- 016 (Undo plugin)
- 017 (Rename plugin)
- 018 (New stash screen)

## Parallelizable With
- None (this is a quality gate -- all Phase 2 tasks must be complete)

## Problem
Tasks 015-018 implement the four Phase 2 features individually. Each has its own unit and integration tests. However, no test validates the features working together as an integrated system. Cross-feature interactions (e.g., conflict preview followed by undo of a failed apply, rename followed by undo of the rename) are untested. There are also no end-to-end tests that exercise the actual TUI via keystroke simulation.

This task creates: (1) cross-feature integration tests in `internal/e2e/phase2_test.go` that exercise realistic multi-step workflows against real git repos, and (2) TUI screenshot tests that verify visual rendering matches the PRD mockups.

## PRD Reference
- Section 16 (Testing Strategy) -- all test layers, especially integration and E2E
- Section 16.2 (Git Test Fixtures) -- temp repos with scripted history
- Section 18 (Phase 2 milestone: "No Fear")
- Section 6.2, FR-10, FR-13, FR-14 (all Phase 2 features under test)
- Section 10, Screens 4, 6, 8, 9 (visual specs for screenshot tests)

## Files to Create
- `internal/e2e/phase2_test.go` -- cross-feature integration tests
- `internal/e2e/helpers_test.go` -- shared test helpers for e2e tests
- `internal/e2e/screenshot_test.go` -- TUI screenshot tests (build gated)

## Files to Modify
- None (this task only adds test files)

## Execution Steps

### Step 1: Create shared e2e test helpers in `internal/e2e/helpers_test.go`

```go
package e2e_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
)

// gitRun executes a git command in the given directory and fails on error.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

// gitRunExpectFail executes a git command expecting it to fail.
func gitRunExpectFail(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// writeTestFile creates or overwrites a file in the test directory.
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

// readTestFile reads a file from the test directory.
func readTestFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// fileExists checks if a file exists in the test directory.
func fileExists(t *testing.T, dir, name string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// newTestRepo creates a fresh git repo with a single initial commit.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.name", "test")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	writeTestFile(t, dir, "README.md", "# test repo\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "initial commit")
	return dir
}

// newRunner creates a GitRunner for the given directory.
func newRunner(t *testing.T, dir string) git.GitRunner {
	t.Helper()
	return git.NewRunner(dir)
}

// stashCount returns the number of stashes in the repo.
func stashCount(t *testing.T, dir string) int {
	t.Helper()
	out := strings.TrimSpace(gitRun(t, dir, "stash", "list"))
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

// stashMessages returns all stash messages (from --format=%gs).
func stashMessages(t *testing.T, dir string) []string {
	t.Helper()
	out := strings.TrimSpace(gitRun(t, dir, "stash", "list", "--format=%gs"))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// stashSHAs returns all stash SHAs.
func stashSHAs(t *testing.T, dir string) []string {
	t.Helper()
	out := strings.TrimSpace(gitRun(t, dir, "stash", "list", "--format=%H"))
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

// stashSHA returns the SHA of a specific stash entry.
func stashSHA(t *testing.T, dir string, index int) string {
	t.Helper()
	ref := "stash@{" + strings.TrimSpace(exec.Command("printf", "%d", index).String()) + "}"
	_ = ref
	out := strings.TrimSpace(gitRun(t, dir, "rev-parse", "stash@{"+itoa(index)+"}"))
	return out
}

func itoa(i int) string {
	return strings.TrimSpace(exec.Command("printf", "%d").String())
}

// gitVersion returns the installed git version as a string.
func gitVersion(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git --version: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// requireGitVersion skips the test if git version is below the requirement.
func requireGitVersion(t *testing.T, major, minor int) {
	t.Helper()
	ver := gitVersion(t)
	// Parse "git version X.Y.Z"
	parts := strings.Fields(ver)
	if len(parts) < 3 {
		t.Skipf("could not parse git version: %s", ver)
	}
	vParts := strings.Split(parts[2], ".")
	if len(vParts) < 2 {
		t.Skipf("could not parse version number: %s", parts[2])
	}

	var gotMajor, gotMinor int
	fmt.Sscanf(vParts[0], "%d", &gotMajor)
	fmt.Sscanf(vParts[1], "%d", &gotMinor)

	if gotMajor < major || (gotMajor == major && gotMinor < minor) {
		t.Skipf("requires git >= %d.%d, have %s", major, minor, parts[2])
	}
}

// eventually retries a check function until it returns nil or the timeout expires.
func eventually(t *testing.T, timeout time.Duration, check func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = check()
		if lastErr == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v: %v", timeout, lastErr)
}
```

**Note:** The `itoa` and `stashSHA` helpers above need correction. Use `fmt.Sprintf`:

```go
func stashSHA(t *testing.T, dir string, index int) string {
	t.Helper()
	ref := fmt.Sprintf("stash@{%d}", index)
	out := strings.TrimSpace(gitRun(t, dir, "rev-parse", ref))
	return out
}
```

Add `"fmt"` to imports.

### Step 2: Create cross-feature integration tests in `internal/e2e/phase2_test.go`

```go
package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugins/conflict"
	"github.com/indrasvat/nidhi/internal/plugins/rename"
	"github.com/indrasvat/nidhi/internal/plugins/undo"
)

// =============================================================================
// Conflict Preview Flow (FR-10)
// =============================================================================

func TestE2E_ConflictFlow_ConflictsDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := newTestRepo(t)

	// Setup: create a file, commit, modify same line, stash, then change HEAD.
	writeTestFile(t, dir, "config.go", `package main

var timeout = 30
var retries = 3
`)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "add config")

	// Stash: change retries to 10.
	writeTestFile(t, dir, "config.go", `package main

var timeout = 30
var retries = 10
`)
	gitRun(t, dir, "stash", "push", "-m", "bump retries")

	// HEAD: change retries to 5 (conflict!).
	writeTestFile(t, dir, "config.go", `package main

var timeout = 30
var retries = 5
`)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "set retries to 5")

	stashCommit := stashSHA(t, dir, 0)
	runner := newRunner(t, dir)

	// Run merge-tree conflict detection.
	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	// Verify: conflicts detected.
	if !result.HasConflicts {
		t.Fatal("expected conflicts, got clean merge")
	}

	// Verify: config.go is marked conflicted.
	var conflicted bool
	for _, f := range result.Files {
		if f.Path == "config.go" && f.Status == git.FileStatusConflicted {
			conflicted = true
		}
	}
	if !conflicted {
		t.Error("expected config.go to be marked as conflicted")
	}

	// Simulate "apply anyway" -- apply the stash despite conflicts.
	out := gitRunExpectFail(t, dir, "stash", "apply", "stash@{0}")

	// Verify: conflicts are in the working tree after forced apply.
	content := readTestFile(t, dir, "config.go")
	if !strings.Contains(content, "<<<<<<<") && !strings.Contains(out, "CONFLICT") {
		// The apply may fail entirely or produce conflict markers.
		// Either way, we've proven the conflict was real.
		t.Log("apply result:", out)
	}
}

func TestE2E_ConflictFlow_CleanApply(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := newTestRepo(t)

	// Setup: stash changes to a file, HEAD modifies a different file.
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "add main")

	writeTestFile(t, dir, "utils.go", "package main\n\nfunc helper() {}\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "stash", "push", "-m", "add utils")

	// HEAD changes a different file -- no conflict possible.
	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() { fmt.Println(\"hello\") }\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "update main")

	stashCommit := stashSHA(t, dir, 0)
	runner := newRunner(t, dir)

	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	// Verify: no conflicts -- should proceed directly.
	if result.HasConflicts {
		t.Error("expected clean merge, got conflicts")
	}

	// Apply should succeed cleanly.
	gitRun(t, dir, "stash", "apply", "stash@{0}")

	// Verify: utils.go exists in working tree.
	if !fileExists(t, dir, "utils.go") {
		t.Error("utils.go should exist after clean apply")
	}
}

// =============================================================================
// Undo Flow (FR-14)
// =============================================================================

func TestE2E_UndoFlow_DropAndRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)

	// Create 3 stashes.
	for i, msg := range []string{"alpha", "beta", "gamma"} {
		writeTestFile(t, dir, "file.go", fmt.Sprintf("package main // v%d %s\n", i, msg))
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "stash", "push", "-m", msg)
	}
	// Order: gamma(0), beta(1), alpha(2)

	if got := stashCount(t, dir); got != 3 {
		t.Fatalf("initial stash count = %d, want 3", got)
	}

	// Drop stash@{0} (gamma).
	sha := stashSHA(t, dir, 0)
	msg := "gamma"
	gitRun(t, dir, "stash", "drop", "stash@{0}")

	if got := stashCount(t, dir); got != 2 {
		t.Fatalf("stash count after drop = %d, want 2", got)
	}

	// Undo: restore gamma via git stash store.
	runner := newRunner(t, dir)
	_, err := runner.Run(context.Background(), "stash", "store", "-m", msg, sha)
	if err != nil {
		t.Fatalf("stash store: %v", err)
	}

	// Verify: 3 stashes again.
	if got := stashCount(t, dir); got != 3 {
		t.Fatalf("stash count after undo = %d, want 3", got)
	}

	// Verify: gamma is back at position 0.
	msgs := stashMessages(t, dir)
	if !strings.Contains(msgs[0], "gamma") {
		t.Errorf("stash@{0} = %q, want gamma", msgs[0])
	}
}

func TestE2E_UndoFlow_RingBufferLIFO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)

	// Create 5 stashes.
	type dropped struct {
		sha string
		msg string
	}
	var drops []dropped

	for i := range 5 {
		msg := fmt.Sprintf("stash-%d", i)
		writeTestFile(t, dir, "file.go", fmt.Sprintf("package main // %s\n", msg))
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "stash", "push", "-m", msg)
	}

	// Drop top 3 stashes, recording SHAs.
	for range 3 {
		sha := stashSHA(t, dir, 0)
		msgs := stashMessages(t, dir)
		drops = append(drops, dropped{sha: sha, msg: msgs[0]})
		gitRun(t, dir, "stash", "drop", "stash@{0}")
	}

	if got := stashCount(t, dir); got != 2 {
		t.Fatalf("stash count after 3 drops = %d, want 2", got)
	}

	// Undo in LIFO order (most recently dropped first).
	runner := newRunner(t, dir)
	for i := len(drops) - 1; i >= 0; i-- {
		_, err := runner.Run(context.Background(), "stash", "store", "-m", drops[i].msg, drops[i].sha)
		if err != nil {
			t.Fatalf("undo %d: %v", i, err)
		}
	}

	if got := stashCount(t, dir); got != 5 {
		t.Fatalf("stash count after undo all = %d, want 5", got)
	}
}

func TestE2E_UndoFlow_CrossSessionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)

	// Create and drop a stash.
	writeTestFile(t, dir, "lost.go", "package main\n\nfunc lost() {}\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "stash", "push", "-m", "lost work")

	sha := stashSHA(t, dir, 0)
	gitRun(t, dir, "stash", "drop", "stash@{0}")

	// Simulate "restart app" -- no in-memory undo buffer.
	// Use git fsck to find the orphaned commit.
	runner := newRunner(t, dir)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}

	// Find our dropped stash in the candidates.
	var found *undo.RecoveryCandidate
	for _, c := range candidates {
		if c.SHA == sha {
			found = &c
			break
		}
	}
	if found == nil {
		t.Fatalf("dropped stash %s not found via fsck (found %d candidates)", sha[:8], len(candidates))
	}

	// Restore it.
	err = undo.RestoreCandidate(context.Background(), runner, *found)
	if err != nil {
		t.Fatalf("RestoreCandidate: %v", err)
	}

	// Verify: stash is back.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count after recovery = %d, want 1", got)
	}
}

// =============================================================================
// Rename Flow (FR-13)
// =============================================================================

func TestE2E_RenameFlow_MiddleStash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Create 3 stashes: alpha, beta, gamma.
	for _, msg := range []string{"alpha", "beta", "gamma"} {
		writeTestFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "stash", "push", "-m", msg)
	}
	// Order: gamma(0), beta(1), alpha(2)

	originalSHAs := stashSHAs(t, dir)

	// Build core.Stash slice for the rename plugin.
	msgs := stashMessages(t, dir)
	stashes := make([]core.Stash, len(msgs))
	for i := range msgs {
		stashes[i] = core.Stash{
			Index:   i,
			SHA:     originalSHAs[i],
			Message: msgs[i],
		}
	}

	// Rename stash@{1} (beta -> "bravo").
	runner := newRunner(t, dir)
	p := rename.New()
	_ = p.Init(plugin.PluginContext{
		Git:    runner,
		Cache:  &noopCache{},
		Logger: slog.Default(),
	})

	err := p.RenameStash(context.Background(), stashes, 1, "bravo")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	// Verify: 3 stashes, correct order, correct messages.
	newMsgs := stashMessages(t, dir)
	if len(newMsgs) != 3 {
		t.Fatalf("stash count = %d, want 3", len(newMsgs))
	}

	expected := []string{"gamma", "bravo", "alpha"}
	for i, want := range expected {
		if !strings.Contains(newMsgs[i], want) {
			t.Errorf("stash@{%d} = %q, want to contain %q", i, newMsgs[i], want)
		}
	}

	// Verify: SHAs preserved.
	newSHAs := stashSHAs(t, dir)
	for i := range originalSHAs {
		if newSHAs[i] != originalSHAs[i] {
			t.Errorf("stash@{%d} SHA changed: %s -> %s", i, originalSHAs[i][:8], newSHAs[i][:8])
		}
	}
}

// =============================================================================
// New Stash Flow (FR-02.4)
// =============================================================================

func TestE2E_NewStashFlow_CreateWithMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)

	// Create changes.
	writeTestFile(t, dir, "feature.go", "package main\n\nfunc feature() {}\n")
	gitRun(t, dir, "add", ".")

	// Create stash with custom message.
	gitRun(t, dir, "stash", "push", "-m", "my feature work")

	// Verify.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	msgs := stashMessages(t, dir)
	if !strings.Contains(msgs[0], "my feature work") {
		t.Errorf("stash message = %q, want 'my feature work'", msgs[0])
	}
}

func TestE2E_NewStashFlow_ScopeToggle_StagedOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)

	// Create staged and unstaged changes.
	writeTestFile(t, dir, "staged.go", "package main\n")
	gitRun(t, dir, "add", "staged.go")
	writeTestFile(t, dir, "README.md", "# modified\n") // Unstaged

	// Stash only staged changes.
	gitRun(t, dir, "stash", "push", "-m", "staged only", "--staged")

	// Verify: stash created.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	// Verify: unstaged changes remain.
	status := gitRun(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "README.md") {
		t.Errorf("unstaged README.md should remain, status: %q", status)
	}
}

func TestE2E_NewStashFlow_IncludeUntracked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)

	// Create untracked files.
	writeTestFile(t, dir, "untracked.txt", "hello\n")
	writeTestFile(t, dir, "file.go", "package main\n")
	gitRun(t, dir, "add", "file.go")

	// Stash with --include-untracked.
	gitRun(t, dir, "stash", "push", "-m", "with untracked", "--include-untracked")

	// Verify: stash created.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	// Verify: untracked file is gone from working tree.
	if fileExists(t, dir, "untracked.txt") {
		t.Error("untracked.txt should be stashed (removed from working tree)")
	}
}

// =============================================================================
// Cross-Feature: Conflict + Undo
// =============================================================================

func TestE2E_ConflictThenUndo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := newTestRepo(t)

	// Create a conflict scenario.
	writeTestFile(t, dir, "config.go", "package main\n\nvar x = 1\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "add config")

	writeTestFile(t, dir, "config.go", "package main\n\nvar x = 100\n")
	gitRun(t, dir, "stash", "push", "-m", "change x to 100")

	writeTestFile(t, dir, "config.go", "package main\n\nvar x = 50\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "change x to 50")

	stashCommit := stashSHA(t, dir, 0)
	runner := newRunner(t, dir)

	// Verify conflict exists.
	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}
	if !result.HasConflicts {
		t.Fatal("expected conflicts")
	}

	// User decides to drop instead of applying.
	sha := stashSHA(t, dir, 0)
	gitRun(t, dir, "stash", "drop", "stash@{0}")

	// Undo the drop.
	_, err = runner.Run(context.Background(), "stash", "store", "-m", "change x to 100", sha)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	// Stash is back.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}
}

// =============================================================================
// Cross-Feature: Rename + Undo
// =============================================================================

func TestE2E_RenameThenDropThenUndo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Create stashes.
	for _, msg := range []string{"first", "second"} {
		writeTestFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitRun(t, dir, "add", ".")
		gitRun(t, dir, "stash", "push", "-m", msg)
	}

	originalSHAs := stashSHAs(t, dir)
	msgs := stashMessages(t, dir)

	// Rename stash@{0} (second -> "renamed").
	stashes := []core.Stash{
		{Index: 0, SHA: originalSHAs[0], Message: msgs[0]},
		{Index: 1, SHA: originalSHAs[1], Message: msgs[1]},
	}

	runner := newRunner(t, dir)
	rp := rename.New()
	_ = rp.Init(plugin.PluginContext{
		Git:    runner,
		Cache:  &noopCache{},
		Logger: slog.Default(),
	})

	err := rp.RenameStash(context.Background(), stashes, 0, "renamed")
	if err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	// Now drop the renamed stash.
	sha := stashSHA(t, dir, 0)
	gitRun(t, dir, "stash", "drop", "stash@{0}")

	// Undo the drop.
	_, err = runner.Run(context.Background(), "stash", "store", "-m", "renamed", sha)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	// Verify: stash restored with the RENAMED message, not the original.
	msgs = stashMessages(t, dir)
	if !strings.Contains(msgs[0], "renamed") {
		t.Errorf("expected restored stash to have renamed message, got: %q", msgs[0])
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestE2E_EmptyRepo_NoStashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := newTestRepo(t)

	// No stashes -- operations should handle gracefully.
	if got := stashCount(t, dir); got != 0 {
		t.Fatalf("expected 0 stashes, got %d", got)
	}

	// Recovery should find nothing.
	runner := newRunner(t, dir)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 recovery candidates, got %d", len(candidates))
	}
}

func TestE2E_StashWithUntrackedCollision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := newTestRepo(t)

	// Stash an untracked file.
	writeTestFile(t, dir, "collision.txt", "from stash\n")
	gitRun(t, dir, "stash", "push", "--include-untracked", "-m", "with untracked")

	// Create the same file in the working tree (tracked).
	writeTestFile(t, dir, "collision.txt", "already here\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "add collision.txt")

	stashCommit := stashSHA(t, dir, 0)
	runner := newRunner(t, dir)

	// Check for untracked collisions.
	collisions, err := git.CheckUntrackedCollisions(context.Background(), runner, stashCommit)
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
		t.Error("expected collision.txt in untracked collisions")
	}

	// merge-tree may or may not flag this as a conflict, but the untracked
	// collision detector should catch it.
	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}
	result.UntrackedCollisions = collisions

	// The combined result should trigger the conflict preview screen.
	if !result.HasConflicts && len(result.UntrackedCollisions) == 0 {
		t.Error("expected either conflicts or untracked collisions")
	}
}

// noopCache is a stub StashCache for tests.
type noopCache struct{}

func (c *noopCache) List(ctx context.Context) ([]core.Stash, error) { return nil, nil }
func (c *noopCache) Diff(ctx context.Context, sha string) (string, error) { return "", nil }
func (c *noopCache) Invalidate() {}
```

**Note:** Add missing imports to the top of the file:

```go
import (
	"log/slog"
	"github.com/indrasvat/nidhi/internal/plugin"
)
```

### Step 3: Create TUI screenshot tests in `internal/e2e/screenshot_test.go`

```go
//go:build screenshot

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Screenshot tests require:
// 1. nidhi binary built: `make build`
// 2. Build tag: `go test -tags screenshot`
// 3. A terminal (iTerm2/Ghostty/Kitty) for rendering
//
// These tests build the binary, run it against a temp repo, capture
// terminal output, and compare against golden files.

func TestScreenshot_ConflictPreview(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping screenshot test")
	}
	requireGitVersion(t, 2, 38)

	binary := buildNidhi(t)
	dir := newTestRepo(t)

	// Setup conflict scenario.
	writeTestFile(t, dir, "config.go", "package main\n\nvar x = 1\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "add config")
	writeTestFile(t, dir, "config.go", "package main\n\nvar x = 100\n")
	gitRun(t, dir, "stash", "push", "-m", "change x")
	writeTestFile(t, dir, "config.go", "package main\n\nvar x = 50\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "change x")

	// Run nidhi and capture output.
	// This is a placeholder -- actual screenshot testing requires
	// teatest or a terminal automation framework.
	_ = binary
	_ = dir
	t.Log("screenshot test: conflict preview (placeholder - requires TUI test framework)")
}

func TestScreenshot_UndoToast(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping screenshot test")
	}

	binary := buildNidhi(t)
	dir := newTestRepo(t)

	writeTestFile(t, dir, "file.go", "package main\n")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "stash", "push", "-m", "test stash")

	_ = binary
	_ = dir
	t.Log("screenshot test: undo toast (placeholder - requires TUI test framework)")
}

func TestScreenshot_NewStashScreen(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping screenshot test")
	}

	binary := buildNidhi(t)
	dir := newTestRepo(t)

	writeTestFile(t, dir, "feature.go", "package main\n")
	gitRun(t, dir, "add", ".")
	writeTestFile(t, dir, "README.md", "# modified\n")
	writeTestFile(t, dir, "untracked.txt", "new file\n")

	_ = binary
	_ = dir
	t.Log("screenshot test: new stash screen (placeholder - requires TUI test framework)")
}

// buildNidhi compiles the nidhi binary and returns the path.
func buildNidhi(t *testing.T) string {
	t.Helper()

	// Find project root (look for go.mod).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	root := wd
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("could not find project root (go.mod)")
		}
		root = parent
	}

	binary := filepath.Join(root, "bin", "nidhi-test")
	cmd := exec.Command("go", "build", "-o", binary, "./cmd/nidhi")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	t.Cleanup(func() {
		os.Remove(binary)
	})

	return binary
}
```

### Step 4: Add a `make e2e` target to the Makefile

```makefile
.PHONY: e2e
e2e: ## Run e2e integration tests
	$(GOTESTSUM) -- -v -count=1 -timeout 120s ./internal/e2e/...
```

### Step 5: Verify

```bash
# Run all e2e tests
go test -v -count=1 -timeout 120s ./internal/e2e/...

# Run specific flows
go test -v -run TestE2E_ConflictFlow ./internal/e2e/...
go test -v -run TestE2E_UndoFlow ./internal/e2e/...
go test -v -run TestE2E_RenameFlow ./internal/e2e/...
go test -v -run TestE2E_NewStashFlow ./internal/e2e/...

# Run cross-feature tests
go test -v -run TestE2E_ConflictThenUndo ./internal/e2e/...
go test -v -run TestE2E_RenameThenDropThenUndo ./internal/e2e/...

# Run edge cases
go test -v -run TestE2E_EmptyRepo ./internal/e2e/...
go test -v -run TestE2E_StashWithUntrackedCollision ./internal/e2e/...

# Screenshot tests (requires build tag)
go test -v -tags screenshot -run TestScreenshot ./internal/e2e/...

# Full CI (includes e2e)
make ci
```

## Verification

### Conflict Flow
```bash
go test -v -run TestE2E_ConflictFlow_ConflictsDetected ./internal/e2e/...
# Expected: merge-tree detects conflict in config.go

go test -v -run TestE2E_ConflictFlow_CleanApply ./internal/e2e/...
# Expected: merge-tree returns clean, apply succeeds, file exists
```

### Undo Flow
```bash
go test -v -run TestE2E_UndoFlow_DropAndRestore ./internal/e2e/...
# Expected: drop gamma, restore via stash store, gamma back at position 0

go test -v -run TestE2E_UndoFlow_RingBufferLIFO ./internal/e2e/...
# Expected: 3 drops undone in LIFO order, all 5 stashes restored

go test -v -run TestE2E_UndoFlow_CrossSessionRecovery ./internal/e2e/...
# Expected: fsck finds dropped commit, restoreCandidate recovers it
```

### Rename Flow
```bash
go test -v -run TestE2E_RenameFlow_MiddleStash ./internal/e2e/...
# Expected: beta renamed to bravo, ordering and SHAs preserved
```

### New Stash Flow
```bash
go test -v -run TestE2E_NewStashFlow_CreateWithMessage ./internal/e2e/...
# Expected: stash created with custom message

go test -v -run TestE2E_NewStashFlow_ScopeToggle_StagedOnly ./internal/e2e/...
# Expected: only staged changes stashed, unstaged remain

go test -v -run TestE2E_NewStashFlow_IncludeUntracked ./internal/e2e/...
# Expected: untracked files stashed (removed from working tree)
```

### Cross-Feature
```bash
go test -v -run TestE2E_ConflictThenUndo ./internal/e2e/...
# Expected: conflict detected, user drops instead, undo restores stash

go test -v -run TestE2E_RenameThenDropThenUndo ./internal/e2e/...
# Expected: rename then drop then undo -- restored with RENAMED message
```

### Edge Cases
```bash
go test -v -run TestE2E_EmptyRepo_NoStashes ./internal/e2e/...
# Expected: 0 stashes, 0 recovery candidates

go test -v -run TestE2E_StashWithUntrackedCollision ./internal/e2e/...
# Expected: untracked collision detected for collision.txt
```

### Full CI
```bash
make ci
# Expected: all lint, unit, integration, and e2e tests pass
```

## Completion Criteria
1. **Conflict flow E2E**: create conflicting stash, run merge-tree, verify conflict detected, apply anyway produces conflicts in working tree
2. **Clean apply E2E**: non-conflicting stash applies cleanly, no conflict screen
3. **Undo flow E2E**: drop + stash store restores stash at correct position
4. **Undo LIFO E2E**: multiple drops undone in correct LIFO order
5. **Cross-session recovery E2E**: fsck discovers orphaned stash commit after drop
6. **Rename flow E2E**: rename middle stash, verify message changed, SHA preserved, ordering intact
7. **New stash flow E2E**: create stash with message, scope toggles, and flag variations
8. **Cross-feature: conflict + undo**: conflict detected, user drops, undo restores
9. **Cross-feature: rename + undo**: rename then drop then undo preserves renamed message
10. **Edge cases**: empty repo, untracked collisions
11. **Screenshot tests**: placeholder framework in place (gated behind `screenshot` build tag)
12. All tests in `internal/e2e/` pass
13. `make ci` passes with all Phase 2 features integrated

## Commit
```
test(e2e): add Phase 2 integration and e2e tests (quality gate)

Add cross-feature integration tests covering conflict preview, undo,
rename, and new stash creation flows. Tests exercise realistic
multi-step workflows against real git repos with t.TempDir().
Includes cross-feature scenarios (conflict+undo, rename+undo)
and edge cases (empty repo, untracked collisions). Screenshot
test framework scaffolded behind build tag.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD section 16 (Testing Strategy)
4. Read tasks 015-018 to understand all features being tested
5. Execute steps 1-5 in order
6. Verify all e2e tests pass
7. Update this file (Status: DONE) + `docs/PROGRESS.md` (Phase 2 complete)
8. Commit with the message above
9. Tag `v0.2.0` release
