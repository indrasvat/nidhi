package git_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
)

// testCache implements git.StashCache for testing.
// Only tracks Invalidate() calls.
type testCache struct {
	invalidated bool
}

func (c *testCache) List(_ context.Context) ([]git.Stash, error)      { return nil, nil }
func (c *testCache) Diff(_ context.Context, _ string) (string, error) { return "", nil }
func (c *testCache) Invalidate()                                      { c.invalidated = true }
func (c *testCache) PreloadDiffs(_ context.Context, _ int)            {}

var _ git.StashCache = (*testCache)(nil)

// setupOpsRepo creates a git repo with N stashes, each adding a unique file.
func setupOpsRepo(t *testing.T, numStashes int) (string, *git.DefaultRunner) {
	t.Helper()
	dir := setupTempRepo(t)

	filePath := filepath.Join(dir, "base.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "commit", "-m", "add base")

	for i := range numStashes {
		name := fmt.Sprintf("stash_%d.go", i)
		content := fmt.Sprintf("package main\n\n// Stash %d\nfunc S%d() {}\n", i, i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		testHelper(t, dir, "git", "add", ".")
		testHelper(t, dir, "git", "stash", "push", "-m", fmt.Sprintf("stash message %d", i))
	}

	runner := git.NewDefaultRunner(dir, nil)
	return dir, runner
}

// countStashes returns the number of stashes in the repo.
func countStashes(t *testing.T, dir string) int {
	t.Helper()
	out := testHelper(t, dir, "git", "stash", "list")
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

// ─── Apply ──────────────────────────────────────────────────

func TestStashOps_Apply(t *testing.T) {
	dir, runner := setupOpsRepo(t, 1)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
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

	// Stash should still be in the list (apply preserves stash).
	if count := countStashes(t, dir); count != 1 {
		t.Errorf("stash count = %d, want 1 (apply preserves stash)", count)
	}

	// File should be applied to working tree.
	content, err := os.ReadFile(filepath.Join(dir, "stash_0.go"))
	if err != nil {
		t.Fatalf("reading applied file: %v", err)
	}
	if !strings.Contains(string(content), "Stash 0") {
		t.Error("applied file should contain stash content")
	}

	// Apply should NOT invalidate cache.
	if cache.invalidated {
		t.Error("Apply should not invalidate cache")
	}
}

func TestStashOps_Apply_ConflictFails(t *testing.T) {
	dir, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	// Create a file and stash it.
	if err := os.WriteFile(filepath.Join(dir, "conflict.go"), []byte("func A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "stash", "push", "-m", "will conflict")

	// Create a conflicting committed change.
	if err := os.WriteFile(filepath.Join(dir, "conflict.go"), []byte("func B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "commit", "-m", "conflicting change")

	// Modify again so working tree is dirty on same file.
	if err := os.WriteFile(filepath.Join(dir, "conflict.go"), []byte("func C() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ops.Apply(ctx, 0)
	if err == nil {
		t.Error("Apply should fail on conflicting working tree")
	}
}

// ─── Pop ────────────────────────────────────────────────────

func TestStashOps_Pop(t *testing.T) {
	dir, runner := setupOpsRepo(t, 1)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	result, err := ops.Pop(ctx, 0)
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Pop should succeed")
	}
	if result.SHA == "" {
		t.Error("Pop should return the stash SHA")
	}

	// Stash should be REMOVED.
	if count := countStashes(t, dir); count != 0 {
		t.Errorf("stash count = %d, want 0 (pop removes stash)", count)
	}

	if !cache.invalidated {
		t.Error("Pop should invalidate cache")
	}
}

func TestStashOps_Pop_NonExistent(t *testing.T) {
	_, runner := setupOpsRepo(t, 1)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	_, err := ops.Pop(ctx, 5)
	if err == nil {
		t.Error("Pop should fail for non-existent stash index")
	}
}

// ─── Drop ───────────────────────────────────────────────────

func TestStashOps_Drop(t *testing.T) {
	dir, runner := setupOpsRepo(t, 3)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

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

	if count := countStashes(t, dir); count != 2 {
		t.Errorf("stash count = %d, want 2 after dropping 1 of 3", count)
	}

	if !cache.invalidated {
		t.Error("Drop should invalidate cache")
	}
}

func TestStashOps_Drop_ReturnsCorrectSHA(t *testing.T) {
	dir, runner := setupOpsRepo(t, 2)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	expectedSHA := testHelper(t, dir, "git", "rev-parse", "stash@{0}")

	result, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
	if result.SHA != expectedSHA {
		t.Errorf("SHA = %q, want %q", result.SHA, expectedSHA)
	}
}

func TestStashOps_Drop_NonExistent(t *testing.T) {
	_, runner := setupOpsRepo(t, 1)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	_, err := ops.Drop(ctx, 99)
	if err == nil {
		t.Error("Drop should fail for non-existent stash index")
	}
}

// ─── Push ───────────────────────────────────────────────────

func TestStashOps_Push(t *testing.T) {
	dir, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\nfunc F() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", ".")

	result, err := ops.Push(ctx, git.PushOptions{Message: "my custom stash"})
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if !result.Success {
		t.Errorf("Push should succeed, got error: %s", result.Error)
	}

	if count := countStashes(t, dir); count != 1 {
		t.Errorf("stash count = %d, want 1", count)
	}

	out := testHelper(t, dir, "git", "stash", "list")
	if !strings.Contains(out, "my custom stash") {
		t.Errorf("stash list should contain custom message, got: %s", out)
	}

	if !cache.invalidated {
		t.Error("Push should invalidate cache")
	}
}

func TestStashOps_Push_KeepIndex(t *testing.T) {
	dir, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(dir, "keep.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", ".")

	_, err := ops.Push(ctx, git.PushOptions{Message: "keep index", KeepIndex: true})
	if err != nil {
		t.Fatalf("Push --keep-index failed: %v", err)
	}

	// With --keep-index, staged changes should still be in the index.
	out := testHelper(t, dir, "git", "diff", "--cached", "--name-only")
	if !strings.Contains(out, "keep.go") {
		t.Error("--keep-index should preserve staged files in the index")
	}
}

func TestStashOps_Push_IncludeUntracked(t *testing.T) {
	dir, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	// Create untracked file (not added to index).
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also need a tracked modification.
	if err := os.WriteFile(filepath.Join(dir, "base.go"), []byte("package main\n// modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ops.Push(ctx, git.PushOptions{Message: "include untracked", IncludeUntracked: true})
	if err != nil {
		t.Fatalf("Push --include-untracked failed: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "untracked.txt")); !os.IsNotExist(statErr) {
		t.Error("untracked file should be removed after stash push --include-untracked")
	}
}

func TestStashOps_Push_Pathspecs(t *testing.T) {
	dir, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(dir, "include.go"), []byte("package main\nfunc Inc() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exclude.go"), []byte("package main\nfunc Exc() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", ".")

	_, err := ops.Push(ctx, git.PushOptions{
		Message:   "partial stash",
		Pathspecs: []string{"include.go"},
	})
	if err != nil {
		t.Fatalf("Push with pathspecs failed: %v", err)
	}

	// exclude.go should still be staged.
	out := testHelper(t, dir, "git", "diff", "--cached", "--name-only")
	if !strings.Contains(out, "exclude.go") {
		t.Error("exclude.go should still be staged after partial stash")
	}
}

func TestStashOps_Push_NoChanges(t *testing.T) {
	_, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	_, err := ops.Push(ctx, git.PushOptions{Message: "nothing to stash"})
	if err == nil {
		t.Error("Push should fail when there are no changes")
	}
}

// ─── BranchFromStash ────────────────────────────────────────

func TestStashOps_BranchFromStash(t *testing.T) {
	dir, runner := setupOpsRepo(t, 1)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	branchName := "feature/from-stash"
	result, err := ops.BranchFromStash(ctx, 0, branchName)
	if err != nil {
		t.Fatalf("BranchFromStash failed: %v", err)
	}
	if !result.Success {
		t.Errorf("BranchFromStash should succeed, got error: %s", result.Error)
	}

	// Should be on the new branch.
	current := testHelper(t, dir, "git", "branch", "--show-current")
	if current != branchName {
		t.Errorf("current branch = %q, want %q", current, branchName)
	}

	// Stash should be removed.
	if count := countStashes(t, dir); count != 0 {
		t.Errorf("stash count = %d, want 0 (branch removes stash)", count)
	}

	if !cache.invalidated {
		t.Error("BranchFromStash should invalidate cache")
	}
}

// ─── ClearAll ───────────────────────────────────────────────

func TestStashOps_ClearAll(t *testing.T) {
	dir, runner := setupOpsRepo(t, 5)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	result, err := ops.ClearAll(ctx)
	if err != nil {
		t.Fatalf("ClearAll failed: %v", err)
	}

	if len(result.Entries) != 5 {
		t.Errorf("captured %d entries, want 5", len(result.Entries))
	}
	for i, entry := range result.Entries {
		if entry.SHA == "" {
			t.Errorf("entry[%d].SHA is empty", i)
		}
	}

	if count := countStashes(t, dir); count != 0 {
		t.Errorf("stash count = %d, want 0 after ClearAll", count)
	}

	if !cache.invalidated {
		t.Error("ClearAll should invalidate cache")
	}
}

func TestStashOps_ClearAll_Empty(t *testing.T) {
	_, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	result, err := ops.ClearAll(ctx)
	if err != nil {
		t.Fatalf("ClearAll on empty repo failed: %v", err)
	}
	if len(result.Entries) != 0 {
		t.Errorf("captured %d entries, want 0", len(result.Entries))
	}
}

// ─── RestoreStash ───────────────────────────────────────────

func TestStashOps_RestoreStash(t *testing.T) {
	dir, runner := setupOpsRepo(t, 1)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	// Drop the stash and capture SHA.
	result, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
	if count := countStashes(t, dir); count != 0 {
		t.Fatalf("stash should be gone after drop, got %d", count)
	}

	// Restore using the captured SHA.
	cache.invalidated = false
	err = ops.RestoreStash(ctx, result.SHA, "restored stash")
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}

	if count := countStashes(t, dir); count != 1 {
		t.Errorf("stash count = %d, want 1 after restore", count)
	}

	out := testHelper(t, dir, "git", "stash", "list")
	if !strings.Contains(out, "restored stash") {
		t.Errorf("restored stash should have custom message, got: %s", out)
	}

	if !cache.invalidated {
		t.Error("RestoreStash should invalidate cache")
	}
}

// ─── Timeout ────────────────────────────────────────────────

func TestStashOps_Timeout(t *testing.T) {
	_, runner := setupOpsRepo(t, 1)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // ensure timeout has passed

	_, err := ops.Apply(ctx, 0)
	if err == nil {
		t.Error("Apply should fail with cancelled context")
	}
}

// ─── Multi-operation sequence ───────────────────────────────

func TestStashOps_Sequence(t *testing.T) {
	dir, runner := setupOpsRepo(t, 0)
	cache := &testCache{}
	ops := git.NewStashOps(runner, cache)
	ctx := context.Background()

	// Create 3 stashes.
	for i := range 3 {
		name := fmt.Sprintf("seq_%d.go", i)
		if err := os.WriteFile(filepath.Join(dir, name), fmt.Appendf(nil, "package main\nfunc Seq%d() {}\n", i), 0o644); err != nil {
			t.Fatal(err)
		}
		testHelper(t, dir, "git", "add", ".")
		_, err := ops.Push(ctx, git.PushOptions{Message: fmt.Sprintf("seq %d", i)})
		if err != nil {
			t.Fatalf("Push %d failed: %v", i, err)
		}
	}

	if count := countStashes(t, dir); count != 3 {
		t.Fatalf("stash count = %d, want 3", count)
	}

	// Drop the newest (index 0).
	dropResult, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}

	// Apply the new top.
	_, err = ops.Apply(ctx, 0)
	if err != nil {
		t.Fatalf("Apply after drop failed: %v", err)
	}

	// Restore the dropped stash.
	err = ops.RestoreStash(ctx, dropResult.SHA, dropResult.Message)
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}

	if count := countStashes(t, dir); count != 3 {
		t.Errorf("stash count = %d, want 3 after restore", count)
	}
}
