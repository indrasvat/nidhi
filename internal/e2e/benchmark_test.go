package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// BenchmarkStartupTime measures git stash list parsing time (PRD §7.1: < 100ms for <= 20 stashes).
func BenchmarkStartupTime(b *testing.B) {
	dir := benchSetupRepo(b, 20)
	ctx := context.Background()

	b.ResetTimer()
	for b.Loop() {
		cmd := exec.CommandContext(ctx, "git", "stash", "list",
			"--format=%H %h %s %D %ai")
		cmd.Dir = dir
		cmd.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "HOME=" + dir}
		out, err := cmd.Output()
		if err != nil {
			b.Fatal(err)
		}
		if len(out) == 0 {
			b.Fatal("empty output")
		}
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
		if err := os.WriteFile(dir+"/"+name, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
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
