package git_test

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

func testHelper(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\noutput: %s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func setupTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testHelper(t, dir, "git", "init")
	testHelper(t, dir, "git", "config", "user.email", "test@test.com")
	testHelper(t, dir, "git", "config", "user.name", "Test")
	testHelper(t, dir, "git", "commit", "--allow-empty", "-m", "initial commit")
	return dir
}

func TestDefaultRunner_Run(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{
			name: "git version returns non-empty",
			args: []string{"version"},
			want: "git version",
		},
		{
			name: "git rev-parse HEAD returns SHA",
			args: []string{"rev-parse", "HEAD"},
		},
		{
			name:    "invalid subcommand returns error",
			args:    []string{"not-a-real-command"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runner.Run(ctx, tt.args...)
			if tt.wantErr {
				// git may return non-zero for invalid commands
				// but we might not get an exec error
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("output %q does not contain %q", got, tt.want)
			}
		})
	}
}

func TestDefaultRunner_RunLines(t *testing.T) {
	dir := setupTempRepo(t)

	filePath := filepath.Join(dir, "file1.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "file1.txt")
	testHelper(t, dir, "git", "commit", "-m", "add file1")

	if err := os.WriteFile(filePath, []byte("change1"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "stash one")

	if err := os.WriteFile(filePath, []byte("change2"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "stash two")

	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	lines, err := runner.RunLines(ctx, "stash", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 stash lines, got %d: %v", len(lines), lines)
	}

	if !strings.Contains(lines[0], "stash two") {
		t.Errorf("first line should contain 'stash two', got %q", lines[0])
	}
	if !strings.Contains(lines[1], "stash one") {
		t.Errorf("second line should contain 'stash one', got %q", lines[1])
	}
}

func TestDefaultRunner_RunLines_EmptyResult(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	lines, err := runner.RunLines(ctx, "stash", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil for empty stash list, got %v", lines)
	}
}

func TestDefaultRunner_RunExitCode(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	stdout, exitCode, err := runner.RunExitCode(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if stdout == "" {
		t.Error("expected non-empty stdout for rev-parse HEAD")
	}
}

func TestDefaultRunner_ContextTimeout(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	_, err := runner.Run(ctx, "version")
	if err == nil {
		t.Error("expected error for timed-out context, got nil")
	}
}

func TestDefaultRunner_WorkDir(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	out, err := runner.Run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDir, _ := filepath.EvalSymlinks(dir)
	actualDir, _ := filepath.EvalSymlinks(out)

	if actualDir != expectedDir {
		t.Errorf("expected work dir %q, got %q", expectedDir, actualDir)
	}
}
