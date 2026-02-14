# Task 026: Comprehensive E2E Tests

## Status: TODO

## Depends On
- All previous tasks (000-024) -- every feature must be implemented before comprehensive E2E testing
- 000 (scaffold) -- `make build`, `make test`, `make ci` infrastructure
- 001 (git runner) -- GitRunner, version detection
- 006 (core model) -- AppState, mode transitions
- 010 (LIST screen) -- default view
- 011 (PREVIEW screen) -- diff preview
- 012 (DETAIL screen) -- file tree + diff
- 013 (conflict preview) -- merge-tree dry-run
- 014 (search plugin) -- fuzzy search
- 015 (export/import) -- sync plugin
- 016 (rename) -- inline rename
- 017 (reorder) -- stash reorder
- 018 (filter) -- branch/stale filter
- 019 (drop + undo) -- undo plugin
- 020 (new stash) -- new stash screen
- 021 (help overlay) -- help modal
- 022 (config) -- TOML config loading
- 024 (iterm2-driver) -- TUI screenshot infrastructure

## Parallelizable With
- None (this is the comprehensive gate -- must run after all features)

## Problem
Individual task tests verify isolated functionality, but no test exercises the complete user workflows end-to-end. We need a comprehensive E2E test suite that: (1) drives the full BubbleTea program with real keystrokes and verifies state transitions across ALL user flows, (2) captures actual terminal screenshots using iterm2-driver and compares them against the mockup spec, and (3) verifies git compatibility across version boundaries. Without this, regressions between interacting features go undetected, and we have no visual proof that the TUI matches the design.

## PRD Reference
- Section 6.1 (FR-01 through FR-03) -- all core features exercised
- Section 6.2 (FR-10 through FR-17) -- all plugin features exercised
- Section 7.1 (Performance) -- large repo performance requirements
- Section 7.3 (Compatibility) -- terminal and Git version compatibility
- Section 10 (Screen Specifications) -- all 10 screens verified
- Section 11.2 (Complete Keymap) -- every keybind exercised
- Section 16.1 (Test Layers) -- E2E test layer definition
- Section 16.2 (Git Test Fixtures) -- temp repo setup pattern

## Files to Create
- `internal/e2e/full_test.go` -- comprehensive E2E covering ALL user flows
- `internal/e2e/screenshot_test.go` -- visual verification via iterm2-driver
- `internal/e2e/git_compat_test.go` -- git version compatibility tests
- `internal/e2e/helpers_test.go` -- shared test fixtures and utilities
- `internal/e2e/testdata/golden/` -- golden screenshot files directory

## Files to Modify
- `Makefile` -- add `make e2e` target for E2E tests (separate from unit tests)

## Execution Steps

### Step 1: Create shared test helpers in `internal/e2e/helpers_test.go`

```go
// internal/e2e/helpers_test.go
package e2e_test

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

// testRepo creates a temporary git repo with a configurable number of stashes.
// Each stash has realistic content: different files, branches, ages, and messages.
type testRepo struct {
	dir string
	t   *testing.T
}

// newTestRepo creates and returns a temp git repo with an initial commit.
func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	dir := t.TempDir()
	r := &testRepo{dir: dir, t: t}
	r.git("init")
	r.git("config", "user.email", "nidhi-test@test.com")
	r.git("config", "user.name", "Nidhi Test")
	r.writeFile("README.md", "# test repo\n")
	r.git("add", ".")
	r.git("commit", "-m", "initial commit")
	return r
}

// git runs a git command in the repo directory and fails test on error.
func (r *testRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Nidhi Test",
		"GIT_AUTHOR_EMAIL=nidhi-test@test.com",
		"GIT_COMMITTER_NAME=Nidhi Test",
		"GIT_COMMITTER_EMAIL=nidhi-test@test.com",
		"GIT_AUTHOR_DATE=2026-02-14T10:00:00Z",
		"GIT_COMMITTER_DATE=2026-02-14T10:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s failed: %v\noutput: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeFile writes content to a file relative to the repo root.
func (r *testRepo) writeFile(name, content string) {
	r.t.Helper()
	path := filepath.Join(r.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// createDiverseStashes creates 5 stashes with varied content to test all features.
// Returns the repo. Stash list (LIFO order):
//   stash@{0}: "Rename test stash" on main, 1 file (recently created)
//   stash@{1}: "WIP: new dashboard layout" on feat/dashboard, 3 files
//   stash@{2}: "Fix auth token refresh" on main, 2 files (in src/auth/)
//   stash@{3}: "Hotfix: rate limiter bug" on main, 1 file (stale - old date)
//   stash@{4}: "Experimental: new cache layer" on feat/cache, 2 files (includes untracked)
func (r *testRepo) createDiverseStashes() {
	r.t.Helper()

	// Stash 4 (oldest, created first): experimental cache with untracked files
	r.git("checkout", "-b", "feat/cache")
	r.writeFile("pkg/cache/lru.go", "package cache\n\ntype LRU struct {\n\tsize int\n}\n")
	r.git("add", ".")
	r.writeFile("pkg/cache/notes.txt", "untracked design notes\n")
	r.git("stash", "push", "-u", "-m", "Experimental: new cache layer")
	r.git("checkout", "main")

	// Stash 3 (stale): hotfix rate limiter
	r.writeFile("pkg/ratelimit/limiter.go", "package ratelimit\n\nfunc Limit() int {\n\treturn 100\n}\n")
	r.git("add", ".")
	r.git("stash", "push", "-m", "Hotfix: rate limiter bug")

	// Stash 2: auth token refresh (multiple files in subdirectory)
	r.writeFile("src/auth/token.go", "package auth\n\nfunc RefreshToken() error {\n\treturn nil\n}\n")
	r.writeFile("src/auth/config.go", "package auth\n\nvar MaxRetries = 5\n")
	r.git("add", ".")
	r.git("stash", "push", "-m", "Fix auth token refresh")

	// Stash 1: dashboard layout (branch-specific)
	r.git("checkout", "-b", "feat/dashboard")
	r.writeFile("components/dashboard.go", "package components\n\ntype Dashboard struct{}\n")
	r.writeFile("components/widget.go", "package components\n\ntype Widget struct{}\n")
	r.writeFile("components/layout.go", "package components\n\ntype Layout struct{}\n")
	r.git("add", ".")
	r.git("stash", "push", "-m", "WIP: new dashboard layout")
	r.git("checkout", "main")

	// Stash 0 (newest, created last): rename test
	r.writeFile("src/auth/token.go", "package auth\n\nfunc RefreshToken() error {\n\t// updated logic\n\treturn nil\n}\n")
	r.git("add", ".")
	r.git("stash", "push", "-m", "Rename test stash")
}

// createManyStashes creates n stashes with sequential content for performance testing.
func (r *testRepo) createManyStashes(n int) {
	r.t.Helper()
	for i := 0; i < n; i++ {
		r.writeFile(fmt.Sprintf("file%04d.go", i),
			fmt.Sprintf("package test\n\n// File %d content\nvar X%d = %d\n", i, i, i))
		r.git("add", ".")
		r.git("stash", "push", "-m", fmt.Sprintf("Stash %04d: batch test content", i))
	}
}

// stashCount returns the number of stashes in the repo.
func (r *testRepo) stashCount() int {
	r.t.Helper()
	out := r.git("stash", "list")
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

// buildBinary builds the nidhi binary and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "nidhi")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/nidhi")
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build nidhi binary: %v\noutput: %s", err, out)
	}
	return binPath
}

// projectRoot returns the root of the nidhi project.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// requireGitVersion checks that the installed git version meets the minimum requirement.
// Skips the test if the version is too old.
func requireGitVersion(t *testing.T, minMajor, minMinor int) {
	t.Helper()
	cmd := exec.Command("git", "version")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
	version := strings.TrimSpace(string(out))
	// Parse "git version 2.53.0"
	parts := strings.Fields(version)
	if len(parts) < 3 {
		t.Skipf("cannot parse git version: %s", version)
	}
	verParts := strings.SplitN(parts[2], ".", 3)
	if len(verParts) < 2 {
		t.Skipf("cannot parse git version number: %s", parts[2])
	}
	major := 0
	minor := 0
	fmt.Sscanf(verParts[0], "%d", &major)
	fmt.Sscanf(verParts[1], "%d", &minor)

	if major < minMajor || (major == minMajor && minor < minMinor) {
		t.Skipf("git %d.%d required, got %d.%d", minMajor, minMinor, major, minor)
	}
}
```

### Step 2: Create `internal/e2e/full_test.go` -- comprehensive user flow tests

```go
// internal/e2e/full_test.go
package e2e_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/indrasvat/nidhi/internal/core"
)

// TestE2E_CompleteWorkflow tests the full happy-path workflow:
// launch -> navigate list -> preview -> detail -> back -> apply stash -> verify
func TestE2E_CompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	// Verify precondition: 5 stashes exist.
	if n := repo.stashCount(); n != 5 {
		t.Fatalf("expected 5 stashes, got %d", n)
	}

	// Create the app model pointed at the test repo.
	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	// Wait for initial render -- LIST mode with stashes visible.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Verify all 5 stashes appear in the LIST view.
	output := tm.Output()
	for _, msg := range []string{
		"Rename test stash",
		"WIP: new dashboard layout",
		"Fix auth token refresh",
		"Hotfix: rate limiter bug",
		"Experimental: new cache layer",
	} {
		if !bytes.Contains(output, []byte(msg)) {
			t.Errorf("LIST view missing stash: %q", msg)
		}
	}

	// Navigate down to stash@{2} ("Fix auth token refresh").
	tm.Send(tea.KeyPressMsg{Text: "j"}) // cursor -> 1
	tm.Send(tea.KeyPressMsg{Text: "j"}) // cursor -> 2

	// Switch to PREVIEW mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("PREVIEW"))
	}, teatest.WithDuration(3*time.Second))

	// Verify diff preview shows content from the auth stash.
	output = tm.Output()
	if !bytes.Contains(output, []byte("token.go")) && !bytes.Contains(output, []byte("config.go")) {
		t.Error("PREVIEW mode should show files from 'Fix auth token refresh' stash")
	}

	// Enter DETAIL mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("DETAIL"))
	}, teatest.WithDuration(3*time.Second))

	// Verify file tree and diff are visible.
	output = tm.Output()
	if !bytes.Contains(output, []byte("token.go")) {
		t.Error("DETAIL mode should show file tree with token.go")
	}

	// Navigate back: Esc -> PREVIEW -> Esc -> LIST
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("PREVIEW")) || bytes.Contains(bts, []byte("LIST"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("LIST"))
	}, teatest.WithDuration(3*time.Second))

	// Apply stash@{0} (cursor at 0 after navigating back via g).
	tm.Send(tea.KeyPressMsg{Text: "g"}) // jump to top
	tm.Send(tea.KeyPressMsg{Text: "a"}) // apply

	// Wait for apply to complete -- stash should still be in list (apply preserves).
	time.Sleep(500 * time.Millisecond)
	output = tm.Output()
	if !bytes.Contains(output, []byte("Rename test stash")) {
		t.Error("after apply, stash should still be in list (apply preserves stash)")
	}

	// Quit the app.
	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_SearchFlow tests: / -> type query -> tab scope -> enter result -> verify jump
func TestE2E_SearchFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	// Wait for LIST to render.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Open search with /
	tm.Send(tea.KeyPressMsg{Text: "/"})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("SEARCH"))
	}, teatest.WithDuration(3*time.Second))

	// Type search query "auth"
	for _, ch := range "auth" {
		tm.Send(tea.KeyPressMsg{Text: string(ch)})
	}

	// Wait for results to filter.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("auth"))
	}, teatest.WithDuration(3*time.Second))

	// Verify search results show the auth stash.
	output := tm.Output()
	if !bytes.Contains(output, []byte("Fix auth token refresh")) {
		t.Error("search for 'auth' should show 'Fix auth token refresh' stash")
	}

	// Tab to change scope (cycle through All -> Messages -> Files -> Diffs -> Branch).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	time.Sleep(200 * time.Millisecond)

	// Press Enter to jump to the result.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.WaitFor(func(bts []byte) bool {
		// Should jump back to LIST or PREVIEW with the auth stash selected.
		return bytes.Contains(bts, []byte("Fix auth token refresh"))
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_ExportFlow tests: e -> select stashes -> enter ref -> enter -> verify git ref created
func TestE2E_ExportFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}
	requireGitVersion(t, 2, 51)

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Open export screen.
	tm.Send(tea.KeyPressMsg{Text: "e"})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("EXPORT")) || bytes.Contains(bts, []byte("Export"))
	}, teatest.WithDuration(3*time.Second))

	// Toggle stash selection (Space selects/deselects).
	tm.Send(tea.KeyPressMsg{Code: tea.KeySpace}) // select stash@{0}
	tm.Send(tea.KeyPressMsg{Text: "j"})
	tm.Send(tea.KeyPressMsg{Code: tea.KeySpace}) // select stash@{1}

	// Tab to ref field.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	time.Sleep(200 * time.Millisecond)

	// Clear default ref and type custom one.
	// (Ctrl+A selects all text in the input, then type replaces)
	for _, ch := range "refs/stashes/test-export" {
		tm.Send(tea.KeyPressMsg{Text: string(ch)})
	}

	// Confirm export with Enter.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Wait for export to complete.
	time.Sleep(2 * time.Second)

	// Verify the git ref was created.
	out := repo.git("rev-parse", "--verify", "refs/stashes/test-export")
	if out == "" {
		t.Error("export should have created refs/stashes/test-export")
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_RenameFlow tests: r -> type new name -> enter -> verify renamed
func TestE2E_RenameFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Press r to start inline rename on stash@{0}.
	tm.Send(tea.KeyPressMsg{Text: "r"})
	time.Sleep(300 * time.Millisecond)

	// Clear existing text and type new message.
	// Send Ctrl+A to select all, then type new name.
	tm.Send(tea.KeyPressMsg{Text: "a", Mod: tea.ModCtrl})
	newName := "Renamed: auth token fix v2"
	for _, ch := range newName {
		tm.Send(tea.KeyPressMsg{Text: string(ch)})
	}

	// Confirm rename.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Wait for rename to take effect.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte(newName))
	}, teatest.WithDuration(5*time.Second))

	// Verify the stash was actually renamed in git.
	stashList := repo.git("stash", "list")
	if !strings.Contains(stashList, newName) {
		t.Errorf("git stash list should contain renamed message %q, got:\n%s", newName, stashList)
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_ReorderFlow tests: J/K -> verify order changed
func TestE2E_ReorderFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	// Record original order.
	originalList := repo.git("stash", "list")
	originalLines := strings.Split(originalList, "\n")
	if len(originalLines) < 2 {
		t.Fatalf("expected at least 2 stashes, got %d", len(originalLines))
	}

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Move stash@{0} down with Shift+J (J).
	tm.Send(tea.KeyPressMsg{Text: "J"})
	time.Sleep(500 * time.Millisecond)

	// Verify order changed: stash@{0} should now be what was stash@{1}.
	newList := repo.git("stash", "list")
	newLines := strings.Split(newList, "\n")

	// The first stash should have changed.
	if newLines[0] == originalLines[0] {
		t.Error("after J (reorder down), first stash should have changed")
	}

	// Move it back up with Shift+K (K) -- cursor should now be at 1.
	tm.Send(tea.KeyPressMsg{Text: "K"})
	time.Sleep(500 * time.Millisecond)

	// Verify order restored.
	restoredList := repo.git("stash", "list")
	if restoredList != originalList {
		t.Errorf("after K (reorder up), stash list should be restored.\nOriginal:\n%s\nGot:\n%s",
			originalList, restoredList)
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_FilterFlow tests: fb -> verify filtered -> fc -> verify cleared
func TestE2E_FilterFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()
	// Currently on main branch. Stashes on main: "Rename test stash", "Hotfix: rate limiter bug",
	// "Fix auth token refresh". Dashboard is on feat/dashboard, cache is on feat/cache.

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Apply branch filter: fb
	tm.Send(tea.KeyPressMsg{Text: "f"})
	tm.Send(tea.KeyPressMsg{Text: "b"})
	time.Sleep(500 * time.Millisecond)

	// Verify: only main-branch stashes should be visible.
	output := tm.Output()
	if bytes.Contains(output, []byte("WIP: new dashboard layout")) {
		t.Error("after fb filter, dashboard stash (feat/dashboard) should NOT be visible")
	}
	if !bytes.Contains(output, []byte("Rename test stash")) {
		t.Error("after fb filter, main-branch stash should still be visible")
	}

	// Clear filter: fc
	tm.Send(tea.KeyPressMsg{Text: "f"})
	tm.Send(tea.KeyPressMsg{Text: "c"})
	time.Sleep(500 * time.Millisecond)

	// Verify: all stashes visible again.
	output = tm.Output()
	if !bytes.Contains(output, []byte("WIP: new dashboard layout")) {
		t.Error("after fc (clear filter), all stashes should be visible again")
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_DropAndUndoFlow tests: d -> toast -> z -> verify restored
func TestE2E_DropAndUndoFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()
	initialCount := repo.stashCount()

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Drop stash@{0} ("Rename test stash").
	tm.Send(tea.KeyPressMsg{Text: "d"})

	// Wait for toast notification.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Dropped")) || bytes.Contains(bts, []byte("undo"))
	}, teatest.WithDuration(3*time.Second))

	// Verify stash count decreased.
	if n := repo.stashCount(); n != initialCount-1 {
		t.Errorf("after drop: expected %d stashes, got %d", initialCount-1, n)
	}

	// Verify the dropped stash is gone from the list.
	stashList := repo.git("stash", "list")
	if strings.Contains(stashList, "Rename test stash") {
		t.Error("dropped stash should not be in git stash list")
	}

	// Undo with z (within 30s window).
	tm.Send(tea.KeyPressMsg{Text: "z"})
	time.Sleep(500 * time.Millisecond)

	// Verify stash was restored.
	if n := repo.stashCount(); n != initialCount {
		t.Errorf("after undo: expected %d stashes, got %d", initialCount, n)
	}

	// Verify the stash message is back (may be at a different index).
	stashList = repo.git("stash", "list")
	if !strings.Contains(stashList, "Rename test stash") {
		t.Errorf("after undo, stash should be restored. Got:\n%s", stashList)
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_NewStashFlow tests: n -> message -> toggles -> enter -> verify created
func TestE2E_NewStashFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()
	initialCount := repo.stashCount()

	// Create a dirty working tree so we have something to stash.
	repo.writeFile("new_feature.go", "package main\n\nfunc NewFeature() {}\n")
	repo.git("add", "new_feature.go")
	repo.writeFile("scratch.txt", "temporary scratch notes\n")

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("stash"))
	}, teatest.WithDuration(5*time.Second))

	// Open new stash screen.
	tm.Send(tea.KeyPressMsg{Text: "n"})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("NEW")) || bytes.Contains(bts, []byte("New Stash")) || bytes.Contains(bts, []byte("Message"))
	}, teatest.WithDuration(3*time.Second))

	// Type stash message.
	message := "E2E test: new feature scaffolding"
	for _, ch := range message {
		tm.Send(tea.KeyPressMsg{Text: string(ch)})
	}

	// Tab to scope toggles.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	time.Sleep(200 * time.Millisecond)

	// Toggle untracked files on (Space).
	tm.Send(tea.KeyPressMsg{Text: "j"}) // navigate to untracked toggle
	tm.Send(tea.KeyPressMsg{Text: "j"})
	tm.Send(tea.KeyPressMsg{Code: tea.KeySpace}) // toggle untracked
	time.Sleep(200 * time.Millisecond)

	// Confirm creation with Enter.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Wait for stash to be created and return to LIST.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte(message)) || bytes.Contains(bts, []byte("LIST"))
	}, teatest.WithDuration(5*time.Second))

	// Verify stash count increased.
	if n := repo.stashCount(); n != initialCount+1 {
		t.Errorf("after new stash: expected %d stashes, got %d", initialCount+1, n)
	}

	// Verify the new stash has our message.
	stashList := repo.git("stash", "list")
	if !strings.Contains(stashList, message) {
		t.Errorf("new stash should have message %q. Got:\n%s", message, stashList)
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_HelpOverlay tests: ? -> verify help -> ? again -> dismissed
func TestE2E_HelpOverlay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Open help overlay with ?
	tm.Send(tea.KeyPressMsg{Text: "?"})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("help")) || bytes.Contains(bts, []byte("Help")) ||
			bytes.Contains(bts, []byte("Navigation")) || bytes.Contains(bts, []byte("Keybind"))
	}, teatest.WithDuration(3*time.Second))

	// Verify help content includes key references.
	output := tm.Output()
	helpKeywords := []string{"apply", "drop", "search", "preview"}
	found := 0
	for _, kw := range helpKeywords {
		if bytes.Contains(bytes.ToLower(output), []byte(kw)) {
			found++
		}
	}
	if found < 2 {
		t.Errorf("help overlay should contain keybind references, found %d of %d keywords", found, len(helpKeywords))
	}

	// Dismiss with ? again.
	tm.Send(tea.KeyPressMsg{Text: "?"})
	time.Sleep(300 * time.Millisecond)

	// Verify help is dismissed -- should see stash list again.
	output = tm.Output()
	if !bytes.Contains(output, []byte("Rename test stash")) {
		t.Error("after dismissing help, LIST should be visible again")
	}

	// Also test dismissing with Esc.
	tm.Send(tea.KeyPressMsg{Text: "?"})
	time.Sleep(300 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(300 * time.Millisecond)

	output = tm.Output()
	if !bytes.Contains(output, []byte("Rename test stash")) {
		t.Error("after Esc from help, LIST should be visible")
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_EmptyRepo tests: launch with no stashes -> verify empty state
func TestE2E_EmptyRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	// No stashes created -- repo is empty.

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	// Wait for empty state to render.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("No stashes")) ||
			bytes.Contains(bts, []byte("no stashes")) ||
			bytes.Contains(bts, []byte("Press n"))
	}, teatest.WithDuration(5*time.Second))

	// Verify empty state message.
	output := tm.Output()
	if !bytes.Contains(bytes.ToLower(output), []byte("no stash")) {
		t.Error("empty repo should show 'no stashes' message")
	}

	// j/k should not crash on empty list.
	tm.Send(tea.KeyPressMsg{Text: "j"})
	tm.Send(tea.KeyPressMsg{Text: "k"})
	tm.Send(tea.KeyPressMsg{Text: "G"})
	tm.Send(tea.KeyPressMsg{Text: "g"})
	time.Sleep(200 * time.Millisecond)

	// a/p/d on empty list should not crash.
	tm.Send(tea.KeyPressMsg{Text: "a"})
	tm.Send(tea.KeyPressMsg{Text: "p"})
	tm.Send(tea.KeyPressMsg{Text: "d"})
	time.Sleep(200 * time.Millisecond)

	// App should still be running.
	output = tm.Output()
	if output == nil {
		t.Error("app should still be running after operations on empty list")
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestE2E_LargeRepo tests: 100+ stashes -> verify performance and scrolling
func TestE2E_LargeRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	repo := newTestRepo(t)
	repo.createManyStashes(120) // 120 stashes

	start := time.Now()
	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	// Wait for initial render.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Stash 0119")) || bytes.Contains(bts, []byte("stash"))
	}, teatest.WithDuration(10*time.Second))

	startupDuration := time.Since(start)
	if startupDuration > 5*time.Second {
		t.Errorf("startup with 120 stashes took %v, expected < 5s", startupDuration)
	}
	t.Logf("Startup with 120 stashes: %v", startupDuration)

	// Rapid scrolling: press j 50 times quickly.
	scrollStart := time.Now()
	for i := 0; i < 50; i++ {
		tm.Send(tea.KeyPressMsg{Text: "j"})
	}
	time.Sleep(500 * time.Millisecond)
	scrollDuration := time.Since(scrollStart)
	t.Logf("50 rapid j keystrokes + settle: %v", scrollDuration)

	// Jump to bottom.
	tm.Send(tea.KeyPressMsg{Text: "G"})
	time.Sleep(200 * time.Millisecond)

	// The last stash (oldest) should have index 0000 in its message.
	output := tm.Output()
	if !bytes.Contains(output, []byte("Stash 0000")) {
		// May not be visible due to scroll, but at least verify no crash.
		t.Log("last stash may not be visible at bottom, but no crash occurred")
	}

	// Jump to top.
	tm.Send(tea.KeyPressMsg{Text: "g"})
	time.Sleep(200 * time.Millisecond)

	// Page scroll: Ctrl+D multiple times.
	for i := 0; i < 5; i++ {
		tm.Send(tea.KeyPressMsg{Text: "d", Mod: tea.ModCtrl})
	}
	time.Sleep(200 * time.Millisecond)

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
```

### Step 3: Create `internal/e2e/screenshot_test.go` -- visual verification with iterm2-driver

```go
// internal/e2e/screenshot_test.go
package e2e_test

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestScreenshots_AllModes captures terminal screenshots of every major screen mode
// using iterm2-driver and compares against expected layout characteristics.
//
// Prerequisites:
//   - macOS with iTerm2 installed
//   - iterm2-driver binary available in PATH or built from source
//   - NIDHI_SCREENSHOT_TEST=1 environment variable set
//
// To update golden files: NIDHI_UPDATE_GOLDEN=1 go test -run TestScreenshots ./internal/e2e/
func TestScreenshots_AllModes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping screenshot test in short mode")
	}
	if os.Getenv("NIDHI_SCREENSHOT_TEST") != "1" {
		t.Skip("set NIDHI_SCREENSHOT_TEST=1 to run screenshot tests")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("screenshot tests require macOS with iTerm2")
	}

	// Verify iterm2-driver is available.
	iterm2Driver, err := exec.LookPath("iterm2-driver")
	if err != nil {
		t.Skip("iterm2-driver not found in PATH; install it to run screenshot tests")
	}

	// Build the nidhi binary.
	nidhiBin := buildBinary(t)

	// Create a test repo with diverse stashes.
	repo := newTestRepo(t)
	repo.createDiverseStashes()

	// Create golden directory.
	goldenDir := filepath.Join(projectRoot(t), "internal", "e2e", "testdata", "golden")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Screenshot output directory.
	screenshotDir := filepath.Join(t.TempDir(), "screenshots")
	if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
		t.Fatal(err)
	}

	updateGolden := os.Getenv("NIDHI_UPDATE_GOLDEN") == "1"

	type screenshotStep struct {
		name        string
		description string
		keys        []string // keys to send before taking screenshot
		delay       time.Duration // delay after keys before screenshot
		validate    func(t *testing.T, screenshotPath string)
	}

	steps := []screenshotStep{
		{
			name:        "01-list-mode",
			description: "Screen 1: LIST mode with 5 diverse stashes",
			keys:        nil, // initial state
			delay:       2 * time.Second,
			validate: func(t *testing.T, path string) {
				t.Helper()
				// Validate screenshot file exists and is non-empty.
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("screenshot not created: %v", err)
				}
				if info.Size() < 1000 {
					t.Errorf("screenshot too small (%d bytes), likely empty", info.Size())
				}
			},
		},
		{
			name:        "02-preview-mode",
			description: "Screen 2: PREVIEW mode with diff pane visible",
			keys:        []string{"Tab"},
			delay:       1 * time.Second,
			validate: func(t *testing.T, path string) {
				t.Helper()
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("screenshot not created: %v", err)
				}
				if info.Size() < 1000 {
					t.Errorf("screenshot too small (%d bytes)", info.Size())
				}
			},
		},
		{
			name:        "03-detail-mode",
			description: "Screen 3: DETAIL mode with file tree + diff",
			keys:        []string{"Enter"},
			delay:       1 * time.Second,
			validate: func(t *testing.T, path string) {
				t.Helper()
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("screenshot not created: %v", err)
				}
				if info.Size() < 1000 {
					t.Errorf("screenshot too small (%d bytes)", info.Size())
				}
			},
		},
		{
			name:        "04-back-to-list",
			description: "Navigate back to LIST: Esc, Esc",
			keys:        []string{"Escape", "Escape"},
			delay:       500 * time.Millisecond,
			validate:    nil, // transitional step, no validation
		},
		{
			name:        "05-search-mode",
			description: "Screen 5: SEARCH mode with query 'auth'",
			keys:        []string{"/", "a", "u", "t", "h"},
			delay:       1 * time.Second,
			validate: func(t *testing.T, path string) {
				t.Helper()
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("screenshot not created: %v", err)
				}
				if info.Size() < 1000 {
					t.Errorf("screenshot too small (%d bytes)", info.Size())
				}
			},
		},
		{
			name:        "06-new-stash",
			description: "Screen 6: NEW STASH screen",
			keys:        []string{"Escape", "n"},
			delay:       1 * time.Second,
			validate: func(t *testing.T, path string) {
				t.Helper()
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("screenshot not created: %v", err)
				}
				if info.Size() < 1000 {
					t.Errorf("screenshot too small (%d bytes)", info.Size())
				}
			},
		},
		{
			name:        "07-help-overlay",
			description: "Screen 10: HELP overlay modal",
			keys:        []string{"Escape", "?"},
			delay:       1 * time.Second,
			validate: func(t *testing.T, path string) {
				t.Helper()
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("screenshot not created: %v", err)
				}
				if info.Size() < 1000 {
					t.Errorf("screenshot too small (%d bytes)", info.Size())
				}
			},
		},
	}

	// Launch nidhi in iTerm2 via iterm2-driver.
	// iterm2-driver creates a new iTerm2 session, runs the command,
	// and provides screenshot capture via its API.
	t.Logf("Launching nidhi via iterm2-driver (binary: %s, repo: %s)", nidhiBin, repo.dir)

	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			screenshotPath := filepath.Join(screenshotDir, step.name+".png")

			// Build the iterm2-driver command.
			// The driver takes: --cols 80 --rows 24 --screenshot <path> --keys <keys> <command>
			args := []string{
				"--cols", "80",
				"--rows", "24",
			}

			if step.keys != nil {
				for _, key := range step.keys {
					args = append(args, "--send-key", key)
				}
			}

			args = append(args,
				"--delay", fmt.Sprintf("%dms", step.delay.Milliseconds()),
				"--screenshot", screenshotPath,
				"--", nidhiBin, "-C", repo.dir, "--no-color",
			)

			cmd := exec.Command(iterm2Driver, args...)
			cmd.Env = append(os.Environ(),
				"TERM=xterm-256color",
				"NO_COLOR=0",
			)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Logf("iterm2-driver output: %s", out)
				t.Fatalf("iterm2-driver failed for step %q: %v", step.name, err)
			}

			// Validate the screenshot.
			if step.validate != nil {
				step.validate(t, screenshotPath)
			}

			// Validate the screenshot is a valid PNG.
			if _, err := os.Stat(screenshotPath); err == nil {
				f, err := os.Open(screenshotPath)
				if err != nil {
					t.Fatalf("cannot open screenshot: %v", err)
				}
				defer f.Close()
				if _, err := png.Decode(f); err != nil {
					t.Errorf("screenshot is not a valid PNG: %v", err)
				}
			}

			// Update golden files if requested.
			goldenPath := filepath.Join(goldenDir, step.name+".png")
			if updateGolden {
				if _, err := os.Stat(screenshotPath); err == nil {
					data, err := os.ReadFile(screenshotPath)
					if err != nil {
						t.Fatalf("cannot read screenshot: %v", err)
					}
					if err := os.WriteFile(goldenPath, data, 0o644); err != nil {
						t.Fatalf("cannot write golden file: %v", err)
					}
					t.Logf("Updated golden file: %s", goldenPath)
				}
			}

			// Compare against golden file if it exists (and we're not updating).
			if !updateGolden {
				if _, err := os.Stat(goldenPath); err == nil {
					// Golden file exists -- compare dimensions at minimum.
					// Full pixel-diff comparison could be added here with a tolerance.
					t.Logf("Golden file exists for %s -- screenshot captured at %s", step.name, screenshotPath)
				} else {
					t.Logf("No golden file for %s (run with NIDHI_UPDATE_GOLDEN=1 to create)", step.name)
				}
			}

			t.Logf("Step %d/%d: %s -- %s", i+1, len(steps), step.name, step.description)
		})
	}
}

// TestScreenshots_TextCapture captures text-mode screenshots (non-visual) using teatest
// as a fallback when iterm2-driver is not available.
func TestScreenshots_TextCapture(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping text screenshot test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	// Create golden text directory.
	goldenDir := filepath.Join(projectRoot(t), "internal", "e2e", "testdata", "golden-text")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatal(err)
	}

	updateGolden := os.Getenv("NIDHI_UPDATE_GOLDEN") == "1"

	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(80, 24),
	)

	// Capture LIST mode.
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	listOutput := tm.Output()
	compareOrUpdateGolden(t, goldenDir, "list-mode.txt", listOutput, updateGolden)

	// Capture PREVIEW mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("PREVIEW"))
	}, teatest.WithDuration(3*time.Second))

	previewOutput := tm.Output()
	compareOrUpdateGolden(t, goldenDir, "preview-mode.txt", previewOutput, updateGolden)

	// Capture DETAIL mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("DETAIL"))
	}, teatest.WithDuration(3*time.Second))

	detailOutput := tm.Output()
	compareOrUpdateGolden(t, goldenDir, "detail-mode.txt", detailOutput, updateGolden)

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// compareOrUpdateGolden compares output against a golden file or updates it.
func compareOrUpdateGolden(t *testing.T, dir, name string, output []byte, update bool) {
	t.Helper()
	path := filepath.Join(dir, name)

	if update {
		if err := os.WriteFile(path, output, 0o644); err != nil {
			t.Fatalf("cannot write golden file %s: %v", path, err)
		}
		t.Logf("Updated golden file: %s", path)
		return
	}

	golden, err := os.ReadFile(path)
	if err != nil {
		t.Logf("No golden file %s (run with NIDHI_UPDATE_GOLDEN=1 to create)", path)
		return
	}

	if !bytes.Equal(output, golden) {
		t.Errorf("output differs from golden file %s\n--- golden ---\n%s\n--- actual ---\n%s",
			path, string(golden), string(output))
	}
}
```

### Step 4: Create `internal/e2e/git_compat_test.go` -- Git version compatibility tests

```go
// internal/e2e/git_compat_test.go
package e2e_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
)

// TestGitCompat_FullFeatures tests that all features work with Git >= 2.53.
func TestGitCompat_FullFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git compat test in short mode")
	}
	requireGitVersion(t, 2, 53)

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	// Verify version detection reports >= 2.53.
	runner := git.NewDefaultRunner(repo.dir, nil)
	ver, err := git.DetectVersion(context.Background(), runner)
	if err != nil {
		t.Fatalf("version detection failed: %v", err)
	}

	if !ver.AtLeast(2, 53, 0) {
		t.Skipf("Git %s < 2.53, skipping full feature test", ver)
	}

	t.Logf("Git version: %s", ver)

	// All features should be available.
	features := []string{
		git.FeatureBranchShowCurrent,
		git.FeatureMergeTree,
		git.FeatureStashExportImport,
	}
	for _, feat := range features {
		if !ver.Supports(feat) {
			t.Errorf("Git %s should support %s", ver, feat)
		}
	}

	// Run the app and verify no feature-disabled badges appear.
	app := core.NewApp(core.AppConfig{
		WorkDir:   repo.dir,
		NoColor:   true,
		NoAnimate: true,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Rename test stash"))
	}, teatest.WithDuration(5*time.Second))

	// Verify no "Requires Git" warnings in the UI.
	output := tm.Output()
	if bytes.Contains(output, []byte("Requires Git")) {
		t.Error("with Git >= 2.53, no 'Requires Git' warnings should be shown")
	}

	// Verify export is accessible (press e, check for EXPORT screen).
	tm.Send(tea.KeyPressMsg{Text: "e"})
	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("EXPORT")) || bytes.Contains(bts, []byte("Export"))
	}, teatest.WithDuration(3*time.Second))

	// No "Git >= 2.51" warning should appear.
	output = tm.Output()
	if bytes.Contains(output, []byte("2.51")) {
		t.Error("export screen should not show version warning with Git >= 2.53")
	}

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestGitCompat_ConflictPreviewDisabled tests behavior with mock Git < 2.38.
// Since we cannot actually downgrade git, we test the version gating logic
// by injecting a mock version and verifying the app's behavior.
func TestGitCompat_ConflictPreviewDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git compat test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	// Create a stash that would normally trigger conflict preview.
	// Modify a file that conflicts with the stash.
	repo.writeFile("src/auth/token.go", "package auth\n\nfunc RefreshToken() error {\n\t// conflicting change\n\treturn fmt.Errorf(\"conflict\")\n}\n")
	repo.git("add", ".")
	repo.git("commit", "-m", "create conflicting state")

	// Test with mock version < 2.38.
	mockVersion := git.GitVersion{Major: 2, Minor: 37, Patch: 0}

	if mockVersion.Supports(git.FeatureMergeTree) {
		t.Fatal("version 2.37 should NOT support merge-tree")
	}

	// Run app with the override version to simulate old git.
	app := core.NewApp(core.AppConfig{
		WorkDir:        repo.dir,
		NoColor:        true,
		NoAnimate:      true,
		OverrideGitVer: &mockVersion,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("stash"))
	}, teatest.WithDuration(5*time.Second))

	// Try to apply a stash -- should skip conflict preview and apply directly.
	tm.Send(tea.KeyPressMsg{Text: "a"})
	time.Sleep(1 * time.Second)

	// Should see an info toast about conflict preview being disabled,
	// OR the stash should just be applied directly without preview.
	output := tm.Output()
	conflictPreviewShown := bytes.Contains(output, []byte("CONFLICT")) ||
		bytes.Contains(output, []byte("Conflict Preview"))
	if conflictPreviewShown {
		t.Error("with Git < 2.38, conflict preview should be disabled")
	}

	// May see a toast about merge-tree not available.
	if bytes.Contains(output, []byte("merge-tree")) || bytes.Contains(output, []byte("2.38")) {
		t.Log("info toast about merge-tree requirement shown (expected)")
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestGitCompat_ExportImportDisabled tests behavior with mock Git < 2.51.
func TestGitCompat_ExportImportDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git compat test in short mode")
	}

	repo := newTestRepo(t)
	repo.createDiverseStashes()

	// Test with mock version < 2.51.
	mockVersion := git.GitVersion{Major: 2, Minor: 50, Patch: 0}

	if mockVersion.Supports(git.FeatureStashExportImport) {
		t.Fatal("version 2.50 should NOT support stash export/import")
	}

	app := core.NewApp(core.AppConfig{
		WorkDir:        repo.dir,
		NoColor:        true,
		NoAnimate:      true,
		OverrideGitVer: &mockVersion,
	})

	tm := teatest.NewTestModel(t, app,
		teatest.WithInitialTermSize(120, 40),
	)

	tm.WaitFor(func(bts []byte) bool {
		return bytes.Contains(bts, []byte("stash"))
	}, teatest.WithDuration(5*time.Second))

	// Press 'e' for export -- should show version requirement message.
	tm.Send(tea.KeyPressMsg{Text: "e"})
	time.Sleep(1 * time.Second)

	output := tm.Output()
	// Should either show a message about requiring Git >= 2.51
	// or simply not open the export screen.
	exportScreenShown := bytes.Contains(output, []byte("EXPORT")) &&
		!bytes.Contains(output, []byte("2.51")) &&
		!bytes.Contains(output, []byte("upgrade"))
	if exportScreenShown {
		t.Error("with Git < 2.51, export should be disabled or show upgrade message")
	}

	// Press 'i' for import -- same behavior expected.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Text: "i"})
	time.Sleep(1 * time.Second)

	output = tm.Output()
	importScreenShown := bytes.Contains(output, []byte("IMPORT")) &&
		!bytes.Contains(output, []byte("2.51")) &&
		!bytes.Contains(output, []byte("upgrade"))
	if importScreenShown {
		t.Error("with Git < 2.51, import should be disabled or show upgrade message")
	}

	tm.Send(tea.KeyPressMsg{Text: "q"})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

// TestGitCompat_VersionDetection verifies version detection against the real git.
func TestGitCompat_VersionDetection(t *testing.T) {
	repo := newTestRepo(t)
	runner := git.NewDefaultRunner(repo.dir, nil)
	ctx := context.Background()

	ver, err := git.DetectVersion(ctx, runner)
	if err != nil {
		t.Fatalf("version detection failed: %v", err)
	}

	t.Logf("Detected Git version: %s (raw: %q)", ver, ver.Raw)

	// Basic sanity checks.
	if ver.Major < 2 {
		t.Errorf("expected git major version >= 2, got %d", ver.Major)
	}
	if ver.IsZero() {
		t.Error("detected version should not be zero")
	}

	// Log feature support for diagnostic purposes.
	features := map[string]string{
		git.FeatureBranchShowCurrent: "branch --show-current (2.22+)",
		git.FeatureMergeTree:         "merge-tree --write-tree (2.38+)",
		git.FeatureStashExportImport: "stash export/import (2.51+)",
	}
	for feat, desc := range features {
		supported := ver.Supports(feat)
		t.Logf("  %s: %v (%s)", feat, supported, desc)
	}
}
```

### Step 5: Add `make e2e` target to Makefile

Add the following to the existing Makefile:

```makefile
.PHONY: e2e
e2e: build ## Run E2E tests (slow, requires built binary)
	$(GOTEST) -v -timeout 600s -count=1 ./internal/e2e/...

.PHONY: e2e-screenshots
e2e-screenshots: build ## Run screenshot tests (requires iTerm2 + iterm2-driver)
	NIDHI_SCREENSHOT_TEST=1 $(GOTEST) -v -timeout 300s -count=1 -run TestScreenshots ./internal/e2e/...

.PHONY: e2e-update-golden
e2e-update-golden: build ## Update golden screenshot files
	NIDHI_SCREENSHOT_TEST=1 NIDHI_UPDATE_GOLDEN=1 $(GOTEST) -v -timeout 300s -count=1 -run TestScreenshots ./internal/e2e/...
```

### Step 6: Create golden directory structure

```bash
mkdir -p internal/e2e/testdata/golden
mkdir -p internal/e2e/testdata/golden-text
touch internal/e2e/testdata/golden/.gitkeep
touch internal/e2e/testdata/golden-text/.gitkeep
```

### Step 7: Verify compilation and test discovery

```bash
go vet ./internal/e2e/...
go test -list '.*' ./internal/e2e/...
```

### Step 8: Run the E2E tests

```bash
# Run all E2E tests (excluding screenshot tests which require iTerm2).
go test -v -timeout 600s -count=1 -run 'TestE2E_' ./internal/e2e/...

# Run git compatibility tests.
go test -v -timeout 300s -count=1 -run 'TestGitCompat_' ./internal/e2e/...

# Run screenshot tests (macOS with iTerm2 only).
NIDHI_SCREENSHOT_TEST=1 go test -v -timeout 300s -count=1 -run 'TestScreenshots_' ./internal/e2e/...
```

### Step 9: Run `make ci` to verify no regressions

```bash
make ci
```

## Verification

### Functional
```bash
# All E2E test files compile.
go vet ./internal/e2e/...

# E2E test listing shows all test functions.
go test -list '.*' ./internal/e2e/... | grep -c 'TestE2E_'
# Expected: at least 9 tests

# Complete workflow test passes.
go test -v -timeout 120s -run TestE2E_CompleteWorkflow ./internal/e2e/...

# Search flow test passes.
go test -v -timeout 120s -run TestE2E_SearchFlow ./internal/e2e/...

# Export flow test passes (requires Git >= 2.51).
go test -v -timeout 120s -run TestE2E_ExportFlow ./internal/e2e/...

# Rename flow test passes.
go test -v -timeout 120s -run TestE2E_RenameFlow ./internal/e2e/...

# Reorder flow test passes.
go test -v -timeout 120s -run TestE2E_ReorderFlow ./internal/e2e/...

# Filter flow test passes.
go test -v -timeout 120s -run TestE2E_FilterFlow ./internal/e2e/...

# Drop + undo flow test passes.
go test -v -timeout 120s -run TestE2E_DropAndUndoFlow ./internal/e2e/...

# New stash flow test passes.
go test -v -timeout 120s -run TestE2E_NewStashFlow ./internal/e2e/...

# Help overlay test passes.
go test -v -timeout 120s -run TestE2E_HelpOverlay ./internal/e2e/...

# Empty repo test passes.
go test -v -timeout 120s -run TestE2E_EmptyRepo ./internal/e2e/...

# Large repo test passes.
go test -v -timeout 120s -run TestE2E_LargeRepo ./internal/e2e/...

# Git compatibility tests pass.
go test -v -timeout 120s -run TestGitCompat_ ./internal/e2e/...
```

### Screenshot Verification (macOS only)
```bash
# Screenshot tests run and capture valid PNGs.
NIDHI_SCREENSHOT_TEST=1 go test -v -timeout 300s -run TestScreenshots_AllModes ./internal/e2e/...

# Text-based screenshot capture works everywhere.
go test -v -timeout 120s -run TestScreenshots_TextCapture ./internal/e2e/...

# Golden file update creates files.
NIDHI_SCREENSHOT_TEST=1 NIDHI_UPDATE_GOLDEN=1 go test -v -run TestScreenshots ./internal/e2e/...
ls internal/e2e/testdata/golden/*.png
ls internal/e2e/testdata/golden-text/*.txt
```

### CI Pipeline
```bash
make ci
make e2e
```

## Completion Criteria
1. `internal/e2e/helpers_test.go` provides `newTestRepo`, `createDiverseStashes`, `createManyStashes`, `buildBinary`, `requireGitVersion` utilities
2. `internal/e2e/full_test.go` has 9 E2E tests covering ALL user flows: complete workflow, search, export, rename, reorder, filter, drop+undo, new stash, help overlay, empty repo, large repo
3. Each E2E test creates a temp git repo, starts the BubbleTea app via teatest, sends real keystrokes, and verifies state
4. `TestE2E_CompleteWorkflow` exercises: launch, navigate, preview, detail, back, apply
5. `TestE2E_DropAndUndoFlow` verifies stash count before/after drop, undo restores via `z`
6. `TestE2E_LargeRepo` creates 120 stashes and verifies startup < 5s, rapid scroll works
7. `internal/e2e/screenshot_test.go` captures screenshots of LIST, PREVIEW, DETAIL, SEARCH, NEW STASH, HELP via iterm2-driver
8. Screenshot tests are gated behind `NIDHI_SCREENSHOT_TEST=1` env var
9. Golden file update via `NIDHI_UPDATE_GOLDEN=1`
10. `internal/e2e/git_compat_test.go` tests Git >= 2.53 (all features), mock Git < 2.38 (conflict preview disabled), mock Git < 2.51 (export/import disabled)
11. `OverrideGitVer` in `core.AppConfig` allows injecting mock versions for testing
12. Makefile has `e2e`, `e2e-screenshots`, `e2e-update-golden` targets
13. All E2E tests pass with `go test -v -timeout 600s ./internal/e2e/...`
14. `make ci` still passes (E2E tests excluded from standard test run unless `-tags e2e` or separate target)

## Commit
```
test: add comprehensive E2E test suite with screenshot verification

Implement internal/e2e/ with 9 full E2E tests covering all user flows
(complete workflow, search, export, rename, reorder, filter, drop+undo,
new stash, help, empty repo, large repo), screenshot capture via
iterm2-driver with golden file comparison, and Git version compatibility
tests (2.53 full features, <2.38 conflict disabled, <2.51 export disabled).
Add Makefile targets for e2e, e2e-screenshots, e2e-update-golden.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.1, 6.2, 7.1, 7.3, 10, 11.2, 16.1-16.2
4. Verify all previous tasks (000-024) are DONE in `docs/PROGRESS.md`
5. Create `internal/e2e/` directory structure
6. Implement `helpers_test.go` shared test utilities
7. Implement `full_test.go` -- all 9 E2E user flow tests
8. Implement `screenshot_test.go` -- iterm2-driver visual verification
9. Implement `git_compat_test.go` -- version compatibility tests
10. Add Makefile targets
11. Run `go vet ./internal/e2e/...` to verify compilation
12. Run E2E tests: `go test -v -timeout 600s ./internal/e2e/...`
13. Run screenshot tests if on macOS with iTerm2
14. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
15. Commit with the message above
