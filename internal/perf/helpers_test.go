package perf_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// benchRepo creates a temporary git repo with the specified number of stashes.
func benchRepo(b *testing.B, stashCount int) string {
	b.Helper()
	dir := b.TempDir()
	runGit(b, dir, "init", "-b", "main")
	runGit(b, dir, "config", "user.email", "bench@test.com")
	runGit(b, dir, "config", "user.name", "Bench")
	writeBenchFile(b, dir, "README.md", "# bench repo\n")
	runGit(b, dir, "add", ".")
	runGit(b, dir, "commit", "-m", "initial commit")

	for i := range stashCount {
		content := generateGoFile(i)
		writeBenchFile(b, dir, fmt.Sprintf("pkg/module%d/file%d.go", i%10, i), content)
		runGit(b, dir, "add", ".")
		runGit(b, dir, "stash", "push", "-m", fmt.Sprintf("Stash %d: benchmark content", i))
	}

	return dir
}

// testRepo creates a temporary git repo for non-benchmark tests.
func testRepo(t *testing.T, stashCount int) string {
	t.Helper()
	dir := t.TempDir()
	runGitT(t, dir, "init", "-b", "main")
	runGitT(t, dir, "config", "user.email", "perf@test.com")
	runGitT(t, dir, "config", "user.name", "Perf")
	writeTestFile(t, dir, "README.md", "# perf test repo\n")
	runGitT(t, dir, "add", ".")
	runGitT(t, dir, "commit", "-m", "initial commit")

	for i := range stashCount {
		content := generateGoFile(i)
		writeTestFile(t, dir, fmt.Sprintf("pkg/module%d/file%d.go", i%10, i), content)
		runGitT(t, dir, "add", ".")
		runGitT(t, dir, "stash", "push", "-m", fmt.Sprintf("Stash %d: test content", i))
	}

	return dir
}

// generateGoFile creates a realistic Go source file with ~100 lines.
func generateGoFile(seed int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("package module%d\n\n", seed%10))
	b.WriteString(fmt.Sprintf("// File%d contains benchmark test content.\n", seed))
	b.WriteString(fmt.Sprintf("type Handler%d struct {\n", seed))
	b.WriteString("\tname    string\n")
	b.WriteString("\tconfig  map[string]interface{}\n")
	b.WriteString("\tenabled bool\n")
	b.WriteString("}\n\n")

	for j := range 10 {
		b.WriteString(fmt.Sprintf("func (h *Handler%d) Process%d(input string) (string, error) {\n", seed, j))
		b.WriteString("\tif input == \"\" {\n")
		b.WriteString("\t\treturn \"\", nil\n")
		b.WriteString("\t}\n")
		b.WriteString(fmt.Sprintf("\t// Processing logic for handler %d, method %d\n", seed, j))
		b.WriteString("\tresult := input + \"_processed\"\n")
		b.WriteString("\treturn result, nil\n")
		b.WriteString("}\n\n")
	}

	return b.String()
}

// runGit runs a git command in the given directory (benchmark version).
func runGit(b *testing.B, dir string, args ...string) string {
	b.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Bench",
		"GIT_AUTHOR_EMAIL=bench@test.com",
		"GIT_COMMITTER_NAME=Bench",
		"GIT_COMMITTER_EMAIL=bench@test.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+dir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// runGitT runs a git command in the given directory (test version).
func runGitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Perf",
		"GIT_AUTHOR_EMAIL=perf@test.com",
		"GIT_COMMITTER_NAME=Perf",
		"GIT_COMMITTER_EMAIL=perf@test.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+dir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeBenchFile writes a file in the given directory (benchmark version).
func writeBenchFile(b *testing.B, dir, name, content string) {
	b.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}

// writeTestFile writes a file in the given directory (test version).
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

// projectRoot returns the project root directory.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test file to find go.mod.
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
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// formatBytes formats bytes as a human-readable string.
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
