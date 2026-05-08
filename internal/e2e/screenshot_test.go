//go:build screenshot

package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Screenshot tests require:
// 1. nidhi binary built: `make build`
// 2. Build tag: `go test -tags screenshot`
// 3. A terminal (iTerm2/Ghostty/Kitty) for rendering
//
// These are placeholder tests. The actual screenshot comparison
// framework will be implemented in Phase 5.

func TestScreenshot_ConflictPreview(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping screenshot test")
	}
	requireGitVersion(t, 2, 38)

	binary := buildNidhi(t)
	dir := setupTestRepo(t, 0)

	writeFile(t, dir, "config.go", "package main\n\nvar x = 1\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add config")
	writeFile(t, dir, "config.go", "package main\n\nvar x = 100\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "change x")
	writeFile(t, dir, "config.go", "package main\n\nvar x = 50\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "change x")

	_ = binary
	_ = dir
	t.Log("screenshot test: conflict preview (placeholder)")
}

func TestScreenshot_UndoToast(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping screenshot test")
	}

	binary := buildNidhi(t)
	dir := setupTestRepo(t, 1)

	_ = binary
	_ = dir
	t.Log("screenshot test: undo toast (placeholder)")
}

func TestScreenshot_NewStashScreen(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping screenshot test")
	}

	binary := buildNidhi(t)
	dir := setupTestRepo(t, 0)

	writeFile(t, dir, "feature.go", "package main\n")
	gitCmd(t, dir, "add", ".")
	writeFile(t, dir, "README.md", "# modified\n")
	writeFile(t, dir, "untracked.txt", "new file\n")

	_ = binary
	_ = dir
	t.Log("screenshot test: new stash screen (placeholder)")
}

// buildNidhi compiles the nidhi binary and returns the path.
func buildNidhi(t *testing.T) string {
	t.Helper()

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
