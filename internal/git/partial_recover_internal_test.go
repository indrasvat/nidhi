package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// internalGitRun is a minimal git helper for tests in package git (which
// cannot use the package git_test helpers).
func internalGitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestRecoverPartialStashAt_ResetsIndex verifies that recovery resets a
// partially-staged selection even when there was no pre-existing staged state
// (fix for Codex P2: "Reset the index before replaying recovery patches").
func TestRecoverPartialStashAt_ResetsIndex(t *testing.T) {
	dir := t.TempDir()
	internalGitRun(t, dir, "init")
	internalGitRun(t, dir, "config", "user.email", "t@t.com")
	internalGitRun(t, dir, "config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	internalGitRun(t, dir, "add", ".")
	internalGitRun(t, dir, "commit", "-m", "base")

	// Simulate an interrupt that left the partial selection staged.
	if err := os.WriteFile(filepath.Join(dir, "leftover.txt"), []byte("staged by interrupted op\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	internalGitRun(t, dir, "add", "leftover.txt")

	runner := NewDefaultRunner(dir, nil)

	// Journal with NO pre-existing staged snapshot (RestorePatch empty) and
	// not marked complete — the case that previously left the index dirty.
	jpath := filepath.Join(t.TempDir(), "partial-journal.json")
	j := &partialJournal{
		Operation:    "partial-stash",
		StartedAt:    time.Now(),
		RestorePatch: "",
		filePath:     jpath,
	}
	if err := j.write(); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	if err := recoverPartialStashAt(context.Background(), runner, jpath); err != nil {
		t.Fatalf("recoverPartialStashAt: %v", err)
	}

	if cached := internalGitRun(t, dir, "diff", "--cached", "--name-only"); strings.TrimSpace(cached) != "" {
		t.Errorf("recovery should have reset the index, but staged: %q", cached)
	}
	if _, err := os.Stat(jpath); !os.IsNotExist(err) {
		t.Errorf("journal should be removed after recovery")
	}
}
