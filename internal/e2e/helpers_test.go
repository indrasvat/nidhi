package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

// setupTestRepo creates a fully initialized git repo with N stashes.
// Each stash has distinct content for verification:
//   - stash i contains file "stash_i.go" with function Stash_i()
//   - stash messages are "stash message N" (LIFO: newest first)
//
// Returns the repo directory path.
func setupTestRepo(t *testing.T, numStashes int) string {
	t.Helper()
	dir := t.TempDir()

	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test User")

	writeFile(t, dir, "README.md", "# E2E test repo\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "initial commit")

	for i := range numStashes {
		filename := fmt.Sprintf("stash_%d.go", i)
		content := fmt.Sprintf("package main\n\nfunc Stash_%d() string {\n\treturn \"stash %d\"\n}\n", i, i)
		writeFile(t, dir, filename, content)
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", fmt.Sprintf("stash message %d", i))
	}

	return dir
}

// setupMultiFileStash creates a repo with one stash containing multiple
// changed files for testing diff preview and file cycling.
func setupMultiFileStash(t *testing.T, numFiles int) string {
	t.Helper()
	dir := t.TempDir()

	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test User")

	// Create base files.
	for i := range numFiles {
		writeFile(t, dir, fmt.Sprintf("src/file_%d.go", i),
			fmt.Sprintf("package main\n// file %d base\n", i))
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Modify all files and stash.
	for i := range numFiles {
		writeFile(t, dir, fmt.Sprintf("src/file_%d.go", i),
			fmt.Sprintf("package main\n// file %d modified\nfunc File%d() {}\n", i, i))
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "multi-file change")

	return dir
}

// ─── Git helpers ────────────────────────────────────────────

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
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
	return strings.TrimSpace(string(out))
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

// gitStashList returns the stash list lines for a repo.
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

// ─── Assertion helpers ──────────────────────────────────────

func assertScreenContains(t *testing.T, view, expected string) {
	t.Helper()
	if !strings.Contains(view, expected) {
		t.Errorf("screen should contain %q\nactual:\n%s", expected, truncate(view, 500))
	}
}

func assertScreenNotContains(t *testing.T, view, unexpected string) {
	t.Helper()
	if strings.Contains(view, unexpected) {
		t.Errorf("screen should NOT contain %q\nactual:\n%s", unexpected, truncate(view, 500))
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... (truncated)"
}

// ─── Test doubles ───────────────────────────────────────────

// noopCache is a minimal git.StashCache for StashOps E2E tests.
type noopCache struct {
	invalidated bool
}

func (c *noopCache) List(_ context.Context) ([]git.Stash, error) { return nil, nil }
func (c *noopCache) Diff(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *noopCache) Invalidate()                           { c.invalidated = true }
func (c *noopCache) PreloadDiffs(_ context.Context, _ int) {}

var _ git.StashCache = (*noopCache)(nil)
