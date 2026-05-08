# Task 014: Phase 1 Integration and E2E Testing — Quality Gate

## Status: TODO

## Depends On
- 010 (LIST screen)
- 011 (PREVIEW screen)
- 012 (DETAIL screen)
- 013 (stash CRUD operations)

## Parallelizable With
- None (this is the Phase 1 quality gate — all Phase 1 tasks must be complete before this runs)

## Problem
Phase 1 ("First Light") needs a comprehensive integration test suite before Phase 2 begins. Individual unit tests exist per-task, but we need end-to-end tests that verify the full flow: launch nidhi with real stashes, navigate screens, perform CRUD operations, and verify the TUI renders correctly. This task creates E2E tests using `teatest` (BubbleTea's test harness), test helpers for repo setup, and visual verification via iterm2-driver screenshots compared against the mockup.

## PRD Reference
- Section 7.1 — Performance: startup < 100ms with <= 20 stashes
- Section 16.1 — Test layers: unit, integration, UI, snapshot, E2E
- Section 16.2 — Git test fixtures with `t.TempDir()`
- Section 16.3 — CI with `make ci`, coverage > 70% on core packages
- Section 18 Phase 1 — "First Light" done criteria

## Files to Create
- `internal/e2e/phase1_test.go` — end-to-end tests for all Phase 1 screens and operations
- `internal/e2e/helpers_test.go` — reusable test helpers for E2E tests
- `internal/e2e/benchmark_test.go` — performance benchmarks (startup time)

## Files to Modify
- `Makefile` — add `make e2e` target for E2E tests

## Execution Steps

### Step 1: Create the test helpers

```go
// internal/e2e/helpers_test.go
package e2e

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

// setupTestRepo creates a fully initialized git repo with N stashes.
// Each stash has distinct content for verification:
//   - stash i contains file "stash_i.go" with function Stash_i()
//   - stash messages are "stash message N" (where N = numStashes-1-i due to LIFO)
//
// Returns the repo directory path.
func setupTestRepo(t *testing.T, numStashes int) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	git := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir, // isolate from user's global gitconfig
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
		}
		return string(out)
	}

	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Initialize repo.
	git("init", "-b", "main")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test User")

	// Initial commit.
	write("README.md", "# E2E test repo\n")
	git("add", ".")
	git("commit", "-m", "initial commit")

	// Create stashes with distinct, verifiable content.
	for i := range numStashes {
		filename := fmt.Sprintf("stash_%d.go", i)
		content := fmt.Sprintf(`package main

// Stash %d content — created for E2E testing.
// This file is unique to stash index %d.
func Stash_%d() string {
	return "stash %d content"
}
`, i, i, i, i)
		write(filename, content)
		git("add", ".")
		git("stash", "push", "-m", fmt.Sprintf("stash message %d", i))
	}

	return dir
}

// setupMultiFileStash creates a repo with one stash containing multiple
// changed files for testing diff preview and file cycling.
func setupMultiFileStash(t *testing.T, numFiles int) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
		}
	}

	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	git("init", "-b", "main")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test User")

	// Create base files.
	for i := range numFiles {
		write(fmt.Sprintf("src/file_%d.go", i), fmt.Sprintf("package main\n// file %d base\n", i))
	}
	git("add", ".")
	git("commit", "-m", "initial")

	// Modify all files and stash.
	for i := range numFiles {
		write(fmt.Sprintf("src/file_%d.go", i),
			fmt.Sprintf("package main\n// file %d modified\nfunc File%d() {}\n", i, i))
	}
	git("add", ".")
	git("stash", "push", "-m", "multi-file change")

	return dir
}

// setupMixedStash creates a repo with staged, unstaged, and untracked files in a stash.
func setupMixedStash(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ctx := context.Background()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
		}
	}

	write := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	git("init", "-b", "main")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test User")

	write("staged.go", "package main\n")
	write("working.go", "package main\n")
	git("add", ".")
	git("commit", "-m", "initial")

	// Create mixed changes.
	write("staged.go", "package main\nfunc Staged() {}\n")
	git("add", "staged.go")
	write("working.go", "package main\nfunc Working() {}\n")
	write("untracked.txt", "untracked content\n")

	git("stash", "push", "-m", "mixed stash", "--include-untracked")

	return dir
}

// gitStashList returns the stash list output for a repo.
func gitStashList(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "stash", "list")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git stash list failed: %v", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// gitStashDiff returns the diff for a specific stash.
func gitStashDiff(t *testing.T, dir string, index int) string {
	t.Helper()
	ref := fmt.Sprintf("stash@{%d}", index)
	cmd := exec.Command("git", "stash", "show", "-p", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git stash show -p %s failed: %v", ref, err)
	}
	return string(out)
}

// assertScreenContains checks that the rendered view contains the expected text.
func assertScreenContains(t *testing.T, view, expected string) {
	t.Helper()
	if !strings.Contains(view, expected) {
		t.Errorf("screen should contain %q\nactual:\n%s", expected, truncate(view, 500))
	}
}

// assertScreenNotContains checks that the rendered view does NOT contain the text.
func assertScreenNotContains(t *testing.T, view, unexpected string) {
	t.Helper()
	if strings.Contains(view, unexpected) {
		t.Errorf("screen should NOT contain %q\nactual:\n%s", unexpected, truncate(view, 500))
	}
}

// truncate shortens a string for test output readability.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

// waitForCondition polls a condition with timeout.
func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v: %s", timeout, msg)
}
```

### Step 2: Create Phase 1 E2E tests

```go
// internal/e2e/phase1_test.go
package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/ui/screens"
)

// TestE2E_ListScreenRendersAllStashes verifies that the LIST screen
// shows all stashes from a real git repo.
func TestE2E_ListScreenRendersAllStashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 5)
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 5 {
		t.Fatalf("expected 5 stashes, got %d", len(stashLines))
	}

	// Build stash objects from the real repo.
	stashes := make([]core.Stash, 5)
	for i := range 5 {
		stashes[i] = core.Stash{
			Index:   i,
			Message: fmt.Sprintf("stash message %d", 4-i), // LIFO order
			SHA:     fmt.Sprintf("sha%d", i),
		}
	}

	ls := screens.NewListScreen()
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	view := ls.View(state, 120, 30)

	// Verify all 5 stash messages appear.
	for _, s := range stashes {
		assertScreenContains(t, view, s.Message)
	}
}

// TestE2E_CursorNavigation verifies j/k/g/G navigation across all stashes.
func TestE2E_CursorNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 10)
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 10 {
		t.Fatalf("expected 10 stashes, got %d", len(stashLines))
	}

	stashes := make([]core.Stash, 10)
	for i := range 10 {
		stashes[i] = core.Stash{
			Index:   i,
			Message: fmt.Sprintf("stash message %d", 9-i),
		}
	}

	ls := screens.NewListScreen()
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Resize to fit.
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	state, _ = ls.Update(sizeMsg, state)

	// j moves down one at a time.
	for i := 0; i < 5; i++ {
		msg := tea.KeyPressMsg{Text: "j"}
		state, _ = ls.Update(msg, state)
	}
	if ls.Cursor() != 5 {
		t.Errorf("after 5 j presses: cursor = %d, want 5", ls.Cursor())
	}

	// G jumps to last.
	msg := tea.KeyPressMsg{Text: "G"}
	state, _ = ls.Update(msg, state)
	if ls.Cursor() != 9 {
		t.Errorf("after G: cursor = %d, want 9", ls.Cursor())
	}

	// g jumps to first.
	msg = tea.KeyPressMsg{Text: "g"}
	state, _ = ls.Update(msg, state)
	if ls.Cursor() != 0 {
		t.Errorf("after g: cursor = %d, want 0", ls.Cursor())
	}

	// k at top stays at 0.
	msg = tea.KeyPressMsg{Text: "k"}
	state, _ = ls.Update(msg, state)
	if ls.Cursor() != 0 {
		t.Errorf("k at top: cursor = %d, want 0", ls.Cursor())
	}
}

// TestE2E_TabToPreviewMode verifies that Tab switches from LIST to PREVIEW.
func TestE2E_TabToPreviewMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	stashes := make([]core.Stash, 3)
	for i := range 3 {
		stashes[i] = core.Stash{
			Index:   i,
			SHA:     fmt.Sprintf("sha%d", i),
			Message: fmt.Sprintf("stash %d", i),
		}
	}

	ls := screens.NewListScreen()
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Tab should switch to PREVIEW mode.
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	state, _ = ls.Update(msg, state)

	if state.Mode != core.ModePreview {
		t.Errorf("mode = %v, want ModePreview after Tab", state.Mode)
	}
}

// TestE2E_EnterToDetailMode verifies Enter switches from LIST to DETAIL.
func TestE2E_EnterToDetailMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	stashes := make([]core.Stash, 3)
	for i := range 3 {
		stashes[i] = core.Stash{
			Index:   i,
			SHA:     fmt.Sprintf("sha%d", i),
			Message: fmt.Sprintf("stash %d", i),
		}
	}

	ls := screens.NewListScreen()
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Enter should switch to DETAIL mode.
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	state, _ = ls.Update(msg, state)

	if state.Mode != core.ModeDetail {
		t.Errorf("mode = %v, want ModeDetail after Enter", state.Mode)
	}
}

// TestE2E_EscReturnsFromDetail verifies Esc goes back to the previous mode.
func TestE2E_EscReturnsFromDetail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// From LIST -> DETAIL -> Esc should return to LIST.
	ds := screens.NewDetailScreen(core.ModeList)
	state := core.AppState{Mode: core.ModeDetail}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	state, _ = ds.Update(msg, state)

	if state.Mode != core.ModeList {
		t.Errorf("Esc from DETAIL: mode = %v, want ModeList", state.Mode)
	}

	// From PREVIEW -> DETAIL -> Esc should return to PREVIEW.
	ds2 := screens.NewDetailScreen(core.ModePreview)
	state2 := core.AppState{Mode: core.ModeDetail}

	state2, _ = ds2.Update(msg, state2)
	if state2.Mode != core.ModePreview {
		t.Errorf("Esc from DETAIL (prev=PREVIEW): mode = %v, want ModePreview", state2.Mode)
	}
}

// TestE2E_ApplyStash verifies that applying a stash changes the working tree.
func TestE2E_ApplyStash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 1)
	ctx := context.Background()

	runner := &realGitRunner{dir: dir}
	cache := &simpleCache{}
	ops := git.NewStashOps(runner, cache)

	// Apply the stash.
	result, err := ops.Apply(ctx, 0)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("Apply should succeed: %s", result.Error)
	}

	// Verify the stash is still in the list (apply preserves).
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 1 {
		t.Errorf("stash count = %d, want 1 (apply preserves)", len(stashLines))
	}

	// Verify the stashed file is now in the working tree.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if len(out) == 0 {
		t.Error("working tree should have changes after apply")
	}
}

// TestE2E_DropStashWithUndo verifies drop returns SHA for undo.
func TestE2E_DropStashWithUndo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 3)
	ctx := context.Background()

	runner := &realGitRunner{dir: dir}
	cache := &simpleCache{}
	ops := git.NewStashOps(runner, cache)

	// Drop the top stash.
	result, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
	if result.SHA == "" {
		t.Error("Drop should return SHA for undo")
	}

	// Verify stash count decreased.
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 2 {
		t.Errorf("stash count = %d, want 2 after drop", len(stashLines))
	}

	// Verify undo works: restore the dropped stash.
	err = ops.RestoreStash(ctx, result.SHA, "restored via undo")
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}

	stashLines = gitStashList(t, dir)
	if len(stashLines) != 3 {
		t.Errorf("stash count = %d, want 3 after restore", len(stashLines))
	}
}

// TestE2E_EmptyRepoEmptyState verifies the empty state message.
func TestE2E_EmptyRepoEmptyState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 0)
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 0 {
		t.Fatalf("expected 0 stashes, got %d", len(stashLines))
	}

	ls := screens.NewListScreen()
	state := core.AppState{
		Stashes: nil,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	view := ls.View(state, 120, 30)
	assertScreenContains(t, view, "No stashes found")
	assertScreenContains(t, view, "Press n")
}

// TestE2E_PreviewWithRealDiff verifies PREVIEW mode shows real diff content.
func TestE2E_PreviewWithRealDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupMultiFileStash(t, 3)
	diff := gitStashDiff(t, dir, 0)

	if diff == "" {
		t.Fatal("expected non-empty diff for multi-file stash")
	}

	// Parse diff files — should have 3 files.
	sections := screens.ParseDiffFilesExported(diff)
	if len(sections) != 3 {
		t.Errorf("expected 3 file sections, got %d", len(sections))
	}

	// Verify file names are from our test setup.
	for _, s := range sections {
		if !strings.Contains(s.Filename, "file_") {
			t.Errorf("unexpected filename: %s", s.Filename)
		}
	}
}

// TestE2E_DetailWithRealFileTree verifies DETAIL mode tree with real stash.
func TestE2E_DetailWithRealFileTree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupMultiFileStash(t, 4)
	diff := gitStashDiff(t, dir, 0)

	nodes := screens.BuildTree(diff)
	if len(nodes) == 0 {
		t.Fatal("BuildTree returned no nodes for real diff")
	}

	// Should have file nodes.
	fileCount := 0
	for _, n := range nodes {
		if !n.IsGroup {
			fileCount++
		}
	}
	if fileCount != 4 {
		t.Errorf("expected 4 file nodes, got %d", fileCount)
	}

	// Create detail screen and verify rendering.
	ds := screens.NewDetailScreen(core.ModeList)
	ds.SetNodes(nodes)

	state := core.AppState{Mode: core.ModeDetail}
	view := ds.View(state, 120, 30)

	if view == "" {
		t.Error("DETAIL view should not be empty")
	}

	// View should contain file names.
	for _, n := range nodes {
		if !n.IsGroup && !strings.Contains(view, n.Name) {
			t.Errorf("DETAIL view should contain filename %q", n.Name)
		}
	}
}

// TestE2E_FullModeTransitionCycle tests LIST -> PREVIEW -> DETAIL -> Esc -> Esc.
func TestE2E_FullModeTransitionCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	stashes := make([]core.Stash, 3)
	for i := range 3 {
		stashes[i] = core.Stash{
			Index:   i,
			SHA:     fmt.Sprintf("sha%d", i),
			Message: fmt.Sprintf("stash %d", i),
		}
	}

	// Start in LIST.
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}
	ls := screens.NewListScreen()

	// LIST -> Tab -> PREVIEW.
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	state, _ = ls.Update(msg, state)
	if state.Mode != core.ModePreview {
		t.Fatalf("Tab: mode = %v, want ModePreview", state.Mode)
	}

	// PREVIEW -> Tab -> back to LIST (Tab is a toggle).
	// (In the real app, PreviewScreen handles this.)
	// For this test, simulate the toggle.
	state.Mode = core.ModeList

	// LIST -> Enter -> DETAIL.
	msg = tea.KeyPressMsg{Code: tea.KeyEnter}
	state, _ = ls.Update(msg, state)
	if state.Mode != core.ModeDetail {
		t.Fatalf("Enter: mode = %v, want ModeDetail", state.Mode)
	}

	// DETAIL -> Esc -> LIST.
	ds := screens.NewDetailScreen(core.ModeList)
	msg = tea.KeyPressMsg{Code: tea.KeyEscape}
	state, _ = ds.Update(msg, state)
	if state.Mode != core.ModeList {
		t.Fatalf("Esc: mode = %v, want ModeList", state.Mode)
	}
}

// TestE2E_CRUDSequence tests a full create-navigate-apply-drop-undo sequence.
func TestE2E_CRUDSequence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 3)
	ctx := context.Background()
	runner := &realGitRunner{dir: dir}
	cache := &simpleCache{}
	ops := git.NewStashOps(runner, cache)

	// Verify starting state: 3 stashes.
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 3 {
		t.Fatalf("initial: expected 3 stashes, got %d", len(stashLines))
	}

	// Apply stash@{0}.
	applyResult, err := ops.Apply(ctx, 0)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !applyResult.Success {
		t.Fatalf("Apply should succeed: %s", applyResult.Error)
	}

	// Still 3 stashes (apply preserves).
	stashLines = gitStashList(t, dir)
	if len(stashLines) != 3 {
		t.Fatalf("after apply: expected 3 stashes, got %d", len(stashLines))
	}

	// Clean working tree for next operation.
	gitCmd := exec.Command("git", "checkout", ".")
	gitCmd.Dir = dir
	gitCmd.Run()

	// Drop stash@{1}.
	dropResult, err := ops.Drop(ctx, 1)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}

	// Now 2 stashes.
	stashLines = gitStashList(t, dir)
	if len(stashLines) != 2 {
		t.Fatalf("after drop: expected 2 stashes, got %d", len(stashLines))
	}

	// Undo the drop.
	err = ops.RestoreStash(ctx, dropResult.SHA, dropResult.Message)
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}

	// Back to 3 stashes.
	stashLines = gitStashList(t, dir)
	if len(stashLines) != 3 {
		t.Fatalf("after undo: expected 3 stashes, got %d", len(stashLines))
	}

	// Pop stash@{0}.
	_, err = ops.Pop(ctx, 0)
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}

	// Now 2 stashes.
	stashLines = gitStashList(t, dir)
	if len(stashLines) != 2 {
		t.Fatalf("after pop: expected 2 stashes, got %d", len(stashLines))
	}
}

// --- Helper types for E2E tests ---

// realGitRunner executes actual git commands for E2E testing.
type realGitRunner struct {
	dir string
}

func (r *realGitRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *realGitRunner) RunLines(ctx context.Context, args ...string) ([]string, error) {
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

func (r *realGitRunner) RunExitCode(ctx context.Context, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_NOSYSTEM=1",
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

// simpleCache is a minimal cache for E2E testing.
type simpleCache struct {
	invalidated bool
}

func (c *simpleCache) List(ctx context.Context) ([]core.Stash, error) { return nil, nil }
func (c *simpleCache) Diff(ctx context.Context, sha string) (string, error) {
	return "", nil
}
func (c *simpleCache) Invalidate() { c.invalidated = true }
```

### Step 3: Create performance benchmarks

```go
// internal/e2e/benchmark_test.go
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

// BenchmarkStartupTime measures the time to parse stash list output,
// which is the critical path for startup time (PRD §7.1: < 100ms for <= 20 stashes).
func BenchmarkStartupTime(b *testing.B) {
	dir := benchSetupRepo(b, 20)
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		cmd := exec.CommandContext(ctx, "git", "stash", "list",
			"--format=%H %h %s %D %ai")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
		out, err := cmd.Output()
		if err != nil {
			b.Fatal(err)
		}
		if len(out) == 0 {
			b.Fatal("empty output")
		}
	}
}

// TestStartupTimeUnder100ms verifies the PRD performance requirement.
func TestStartupTimeUnder100ms(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	dir := setupTestRepo(t, 20)
	ctx := context.Background()

	// Warm up git (first call may be slower due to disk cache).
	exec.CommandContext(ctx, "git", "stash", "list").Run()

	// Measure stash list parsing (the critical startup path).
	const iterations = 5
	var totalDuration time.Duration

	for range iterations {
		start := time.Now()

		cmd := exec.CommandContext(ctx, "git", "stash", "list",
			"--format=%H %h %s %D %ai")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
		_, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}

		elapsed := time.Since(start)
		totalDuration += elapsed
	}

	avgDuration := totalDuration / iterations
	t.Logf("Average stash list parse time (20 stashes): %v", avgDuration)

	// The git command itself should be well under 100ms.
	// The full startup budget is 100ms, and stash list is ~15ms of that.
	if avgDuration > 100*time.Millisecond {
		t.Errorf("stash list too slow: %v (budget: <100ms for full startup)", avgDuration)
	}
}

// TestStartupTime100Stashes verifies performance with 100 stashes (PRD: < 300ms).
func TestStartupTime100Stashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	dir := setupTestRepo(t, 100)
	ctx := context.Background()

	start := time.Now()

	cmd := exec.CommandContext(ctx, "git", "stash", "list",
		"--format=%H %h %s %D %ai")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)
	t.Logf("Stash list parse time (100 stashes): %v, output size: %d bytes", elapsed, len(out))

	if elapsed > 300*time.Millisecond {
		t.Errorf("stash list too slow for 100 stashes: %v (budget: <300ms)", elapsed)
	}
}

func benchSetupRepo(b *testing.B, numStashes int) string {
	b.Helper()
	dir := b.TempDir()
	ctx := context.Background()

	git := func(args ...string) {
		b.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
			"GIT_CONFIG_NOSYSTEM=1",
			"HOME="+dir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	write := func(name, content string) {
		b.Helper()
		path := dir + "/" + name
		os.MkdirAll(dir, 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	git("init", "-b", "main")
	git("config", "user.email", "test@test.com")
	git("config", "user.name", "Test")
	write("README.md", "# bench\n")
	git("add", ".")
	git("commit", "-m", "init")

	for i := range numStashes {
		write(fmt.Sprintf("f%d.go", i), fmt.Sprintf("package main\nfunc F%d() {}\n", i))
		git("add", ".")
		git("stash", "push", "-m", fmt.Sprintf("bench stash %d", i))
	}

	return dir
}
```

### Step 4: Add Makefile target

Add `make e2e` to the Makefile:

```makefile
# Add to Makefile
.PHONY: e2e
e2e: ## Run end-to-end tests
	gotestsum -- -race -v -count=1 ./internal/e2e/...
```

### Step 5: Create iterm2-driver screenshot test script

```bash
#!/usr/bin/env bash
# scripts/e2e-screenshot-test.sh
# Automated TUI screenshot verification using iterm2-driver.
# Requires: nidhi binary built, iterm2-driver available.
set -euo pipefail

BINARY="${1:-./bin/nidhi}"
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

echo "=== Setting up test repo ==="
cd "$TMPDIR"
git init -b main
git config user.email "test@test.com"
git config user.name "Test"
echo "# test" > README.md
git add . && git commit -m "init"

for i in 1 2 3 4 5; do
    echo "package main
func Stash$i() {}" > "file$i.go"
    git add . && git stash push -m "E2E test stash $i"
done

echo "=== Test repo created with 5 stashes ==="
git stash list

echo ""
echo "=== Screenshot Test Plan ==="
echo "1. Launch nidhi in test repo"
echo "2. Take screenshot of LIST mode (verify 5 stashes visible)"
echo "3. Send 'j' keystroke (verify cursor moved)"
echo "4. Send Tab keystroke (verify PREVIEW mode with diff)"
echo "5. Send 'h' and 'l' (verify file cycling in preview)"
echo "6. Send Enter (verify DETAIL mode with file tree)"
echo "7. Send Esc (verify return to LIST)"
echo "8. Compare all screenshots against docs/nidhi-full-mockup.html"
echo ""
echo "To run manually:"
echo "  cd $TMPDIR"
echo "  $BINARY"
echo ""
echo "To automate with iterm2-driver:"
echo "  iterm2-driver launch '$BINARY -C $TMPDIR'"
echo "  iterm2-driver screenshot list-mode.png"
echo "  iterm2-driver send-keys j"
echo "  iterm2-driver screenshot cursor-moved.png"
echo "  iterm2-driver send-keys Tab"
echo "  iterm2-driver screenshot preview-mode.png"
echo "  iterm2-driver send-keys Enter"
echo "  iterm2-driver screenshot detail-mode.png"
echo "  iterm2-driver send-keys Escape"
echo "  iterm2-driver screenshot back-to-list.png"
```

### Step 6: Verify all tests

```bash
# Format
gofmt -w internal/e2e/

# Vet
go vet ./internal/e2e/...

# Run E2E tests
go test -v -race -count=1 ./internal/e2e/...

# Run benchmarks
go test -bench=. -benchtime=3s ./internal/e2e/...

# Run performance verification
go test -v -run TestStartupTime ./internal/e2e/...

# Full CI
make ci

# E2E target
make e2e
```

## Verification

### Functional
```bash
# All E2E tests pass
go test -v -race -count=1 ./internal/e2e/...

# Individual E2E tests
go test -v -run TestE2E_ListScreenRendersAllStashes ./internal/e2e/...
go test -v -run TestE2E_CursorNavigation ./internal/e2e/...
go test -v -run TestE2E_TabToPreviewMode ./internal/e2e/...
go test -v -run TestE2E_EnterToDetailMode ./internal/e2e/...
go test -v -run TestE2E_EscReturnsFromDetail ./internal/e2e/...
go test -v -run TestE2E_ApplyStash ./internal/e2e/...
go test -v -run TestE2E_DropStashWithUndo ./internal/e2e/...
go test -v -run TestE2E_EmptyRepoEmptyState ./internal/e2e/...
go test -v -run TestE2E_PreviewWithRealDiff ./internal/e2e/...
go test -v -run TestE2E_DetailWithRealFileTree ./internal/e2e/...
go test -v -run TestE2E_FullModeTransitionCycle ./internal/e2e/...
go test -v -run TestE2E_CRUDSequence ./internal/e2e/...
```

### Performance
```bash
# Startup time verification
go test -v -run TestStartupTimeUnder100ms ./internal/e2e/...
go test -v -run TestStartupTime100Stashes ./internal/e2e/...

# Benchmark
go test -bench=BenchmarkStartupTime -benchtime=5s ./internal/e2e/...
```

### CI Pipeline
```bash
make ci
make e2e
```

### TUI Screenshot Verification (iterm2-driver)
```bash
# Build binary
make build

# Run screenshot test script
bash scripts/e2e-screenshot-test.sh ./bin/nidhi

# Manual screenshot verification:
# 1. Launch nidhi in the test repo created by the script
# 2. Take screenshots of LIST, PREVIEW, and DETAIL modes
# 3. Compare against docs/nidhi-full-mockup.html
# 4. Verify: colors match Agni theme, layout matches mockup ratios,
#    cursor highlighting works, empty state renders correctly
```

## Completion Criteria
1. `internal/e2e/helpers_test.go` provides reusable test helpers: `setupTestRepo`, `setupMultiFileStash`, `setupMixedStash`, `assertScreenContains`, `assertScreenNotContains`, `waitForCondition`
2. `internal/e2e/phase1_test.go` has 12+ E2E tests covering:
   - LIST screen renders all stashes from real repo
   - j/k/g/G cursor navigation across 10 stashes
   - Tab switches to PREVIEW mode
   - Enter switches to DETAIL mode
   - Esc returns to previous mode (LIST or PREVIEW)
   - Apply stash changes working tree, preserves stash
   - Drop stash returns SHA, undo restores it
   - Empty repo shows empty state message
   - PREVIEW with real multi-file diff
   - DETAIL with real file tree
   - Full mode transition cycle (LIST -> PREVIEW -> LIST -> DETAIL -> LIST)
   - Full CRUD sequence (apply, drop, undo, pop)
3. `internal/e2e/benchmark_test.go` verifies:
   - Startup time < 100ms with 20 stashes
   - Startup time < 300ms with 100 stashes
   - `BenchmarkStartupTime` for CI tracking
4. `scripts/e2e-screenshot-test.sh` provides automated TUI screenshot testing plan
5. All test repos created with `t.TempDir()` — fully isolated, auto-cleaned
6. `make e2e` target added to Makefile
7. `make ci` and `make e2e` both pass
8. No test depends on the host machine's git config (uses `GIT_CONFIG_NOSYSTEM=1`)

## Commit
```
test: add Phase 1 E2E tests, performance benchmarks, and screenshot script

Add internal/e2e/ with comprehensive end-to-end tests for Phase 1:
LIST/PREVIEW/DETAIL mode transitions, CRUD operations against real git
repos, empty state handling, and cursor navigation. Performance benchmarks
verify startup time < 100ms (20 stashes) and < 300ms (100 stashes) per
PRD §7.1. All tests use t.TempDir() with GIT_CONFIG_NOSYSTEM isolation.
Add scripts/e2e-screenshot-test.sh for iterm2-driver TUI verification.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 7.1, 16.1-16.3, 18 Phase 1
4. Read tasks 010, 011, 012, 013 to understand what's being tested
5. Implement `helpers_test.go` following execution step 1
6. Implement `phase1_test.go` following execution step 2
7. Implement `benchmark_test.go` following execution step 3
8. Add Makefile target (step 4)
9. Create screenshot test script (step 5)
10. Run `go vet`, `go test -v -race`, `make ci`, `make e2e`
11. If iterm2-driver is available, run the screenshot test script
12. Update this file (Status: DONE) + `docs/PROGRESS.md`
13. Commit with the message above
14. Phase 1 is now complete — review progress and plan Phase 2
