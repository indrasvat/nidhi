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
	writeFile(t, dir, "recover.go", "package main\n\nfunc recoverMe() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "recoverable")

	sha := gitCmd(t, dir, "rev-parse", "stash@{0}")
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	// Restore via RestoreCandidate.
	runner := git.NewDefaultRunner(dir, nil)
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

	listOut := gitCmd(t, dir, "stash", "list")
	if !strings.Contains(listOut, "recoverable") {
		t.Errorf("restored stash missing message, got: %s", listOut)
	}
}

func TestFindDroppedStashes_NoOrphans(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Fresh repo with no dropped stashes — should find nothing.
	runner := git.NewDefaultRunner(dir, nil)
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
	for _, msg := range []string{"alpha", "beta", "gamma"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
		sha := gitCmd(t, dir, "rev-parse", "stash@{0}")
		droppedSHAs[sha] = true
	}

	// Drop all 3.
	for range 3 {
		gitCmd(t, dir, "stash", "drop", "stash@{0}")
	}

	runner := git.NewDefaultRunner(dir, nil)
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

func TestFindDroppedStashes_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create and drop a stash.
	writeFile(t, dir, "lost.go", "package main\n\nfunc lost() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "lost stash")

	sha := gitCmd(t, dir, "rev-parse", "stash@{0}")
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	// Run fsck to find orphaned commits.
	runner := git.NewDefaultRunner(dir, nil)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}

	// The dropped stash commit should be discoverable via fsck.
	var found bool
	for _, c := range candidates {
		if c.SHA == sha {
			found = true
			if c.Message == "" {
				t.Error("expected non-empty message for recovered candidate")
			}
			if c.Date == "" {
				t.Error("expected non-empty date for recovered candidate")
			}
			break
		}
	}
	if !found {
		t.Errorf("expected SHA %s in candidates, got %d candidates", sha[:8], len(candidates))
	}
}
