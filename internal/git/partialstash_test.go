package git_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

// setupPartialRepo creates a repo with a committed base file, then applies
// `working` as the current working-tree content (unstaged, vs HEAD).
func setupPartialRepo(t *testing.T, base, working string) (string, *git.DefaultRunner) {
	t.Helper()
	dir := setupTempRepo(t)
	fp := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(fp, []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "commit", "-m", "base")
	if err := os.WriteFile(fp, []byte(working), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, git.NewDefaultRunner(dir, nil)
}

func diffHead(t *testing.T, dir string) string {
	t.Helper()
	return testHelper(t, dir, "git", "diff", "HEAD") + "\n"
}

func TestCreatePartialStash_SelectsOnlyChosenLine(t *testing.T) {
	dir, runner := setupPartialRepo(t,
		"l1\nl2\nl3\n",
		"l1\nADDED1\nl2\nADDED2\nl3\n",
	)
	ctx := context.Background()

	ps, err := git.ParsePatch(diffHead(t, dir))
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	// Select only ADDED1.
	sel := false
	for fi := range ps.Files {
		for hi := range ps.Files[fi].Hunks {
			for li := range ps.Files[fi].Hunks[hi].Lines {
				ln := &ps.Files[fi].Hunks[hi].Lines[li]
				if ln.Kind == git.LineAdded && ln.Text == "ADDED1" {
					ln.Selected = true
					sel = true
				}
			}
		}
	}
	if !sel {
		t.Fatal("did not find ADDED1 in parsed diff")
	}

	patch := ps.BuildSelectedPatch()
	res, err := git.CreatePartialStash(ctx, runner, patch, "partial test")
	if err != nil {
		t.Fatalf("CreatePartialStash: %v", err)
	}
	if !res.Success {
		t.Fatalf("not successful: %+v", res)
	}

	// Exactly one stash, containing ADDED1 but not ADDED2.
	if n := countStashes(t, dir); n != 1 {
		t.Fatalf("want 1 stash, got %d", n)
	}
	stashDiff := testHelper(t, dir, "git", "stash", "show", "-p", "stash@{0}")
	if !strings.Contains(stashDiff, "ADDED1") {
		t.Errorf("stash should contain ADDED1:\n%s", stashDiff)
	}
	if strings.Contains(stashDiff, "ADDED2") {
		t.Errorf("stash should NOT contain ADDED2:\n%s", stashDiff)
	}

	// Working tree should retain ADDED2 but not ADDED1.
	wt := testHelper(t, dir, "git", "diff", "HEAD")
	if !strings.Contains(wt, "ADDED2") {
		t.Errorf("working tree should retain ADDED2:\n%s", wt)
	}
	if strings.Contains(wt, "ADDED1") {
		t.Errorf("working tree should NOT retain ADDED1:\n%s", wt)
	}
}

func TestCreatePartialStash_NoOrphanStashInvariant(t *testing.T) {
	// Selecting only part of an adjacent modification can make
	// `git stash push --staged` fail AFTER creating the stash. Whatever the
	// outcome, the operation must never leave an orphan stash: success ⇒ 1
	// stash, failure ⇒ 0 (Codex P1).
	dir, runner := setupPartialRepo(t, "x\n", "A\nB\n")
	ctx := context.Background()

	ps, err := git.ParsePatch(diffHead(t, dir))
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	for fi := range ps.Files {
		for hi := range ps.Files[fi].Hunks {
			for li := range ps.Files[fi].Hunks[hi].Lines {
				ln := &ps.Files[fi].Hunks[hi].Lines[li]
				if ln.Kind == git.LineAdded && ln.Text == "A" {
					ln.Selected = true
				}
			}
		}
	}
	patch := ps.BuildSelectedPatch()
	_, opErr := git.CreatePartialStash(ctx, runner, patch, "subset")

	n := countStashes(t, dir)
	if opErr == nil {
		if n != 1 {
			t.Errorf("success should yield exactly 1 stash, got %d", n)
		}
	} else if n != 0 {
		t.Errorf("failure must leave no orphan stash, got %d", n)
	}
}

func TestCreatePartialStash_EmptyPatchErrors(t *testing.T) {
	dir, runner := setupPartialRepo(t, "a\n", "a\nb\n")
	_ = dir
	_, err := git.CreatePartialStash(context.Background(), runner, "", "msg")
	if err == nil {
		t.Fatal("expected error for empty patch")
	}
}

func TestCreatePartialStash_BadPatchLeavesRepoUnchanged(t *testing.T) {
	// Working tree is clean; an unrelated/non-applying patch must be rejected
	// without creating a stash or mutating anything.
	dir, runner := setupPartialRepo(t, "a\nb\nc\n", "a\nb\nc\n")
	ctx := context.Background()

	bad := strings.Join([]string{
		"diff --git a/file.txt b/file.txt",
		"--- a/file.txt",
		"+++ b/file.txt",
		"@@ -1,1 +1,2 @@",
		" TOTALLY WRONG CONTEXT",
		"+x",
		"",
	}, "\n")

	_, err := git.CreatePartialStash(ctx, runner, bad, "msg")
	if err == nil {
		t.Fatal("expected error for non-applying patch")
	}
	if n := countStashes(t, dir); n != 0 {
		t.Errorf("want 0 stashes after failed apply, got %d", n)
	}
	// File content unchanged.
	got, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(got) != "a\nb\nc\n" {
		t.Errorf("file content changed: %q", string(got))
	}
}

func TestCreatePartialStash_NewFileRoundTrips(t *testing.T) {
	// A newly-added (tracked) file appears in `git diff HEAD` as a new-file
	// hunk (-0,0 +1,N). Verify the synthesized patch applies via real git.
	dir, runner := setupPartialRepo(t, "base\n", "base\n")
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, "added.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "added.txt")

	ps, err := git.ParsePatch(diffHead(t, dir))
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	ps.SelectAll(true)
	patch := ps.BuildSelectedPatch()
	if _, err := git.CreatePartialStash(ctx, runner, patch, "new file stash"); err != nil {
		t.Fatalf("CreatePartialStash: %v", err)
	}
	if n := countStashes(t, dir); n != 1 {
		t.Fatalf("want 1 stash, got %d", n)
	}
	stashDiff := testHelper(t, dir, "git", "stash", "show", "-p", "stash@{0}")
	if !strings.Contains(stashDiff, "one") || !strings.Contains(stashDiff, "two") {
		t.Errorf("stash should contain the new file content:\n%s", stashDiff)
	}
}

func TestCreatePartialStash_PreservesPreexistingStaged(t *testing.T) {
	dir, runner := setupPartialRepo(t,
		"x1\nx2\nx3\n",
		"x1\nx2\nx3\n",
	)
	ctx := context.Background()

	// Create a second file, stage it (pre-existing staged change).
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "staged.txt")

	// Now introduce an unstaged change to file.txt and partially stash it.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x1\nNEW\nx2\nx3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ps, err := git.ParsePatch(diffHead(t, dir))
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	for fi := range ps.Files {
		if ps.Files[fi].NewPath == "file.txt" {
			ps.Files[fi].SetSelected(true)
		}
	}
	patch := ps.BuildSelectedPatch()
	if _, err := git.CreatePartialStash(ctx, runner, patch, "partial with staged"); err != nil {
		t.Fatalf("CreatePartialStash: %v", err)
	}

	// staged.txt must still be staged afterwards.
	cached := testHelper(t, dir, "git", "diff", "--cached", "--name-only")
	if !strings.Contains(cached, "staged.txt") {
		t.Errorf("pre-existing staged file should remain staged; cached=%q", cached)
	}
}
