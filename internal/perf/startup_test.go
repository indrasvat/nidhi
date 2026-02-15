package perf_test

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
)

// BenchmarkStartup_0Stashes benchmarks cold start with 0 stashes.
// Target: < 100ms (PRD §7.1).
func BenchmarkStartup_0Stashes(b *testing.B) {
	dir := benchRepo(b, 0)
	b.ResetTimer()

	for b.Loop() {
		runner := git.NewDefaultRunner(dir, nil)
		ctx := context.Background()
		_, _ = git.DetectVersion(ctx, runner)
		_, _ = runner.RunLines(ctx, "stash", "list", "--format=%H%n%h%n%gs%n%aI")
	}
}

// BenchmarkStartup_20Stashes benchmarks cold start with 20 stashes.
// Target: < 100ms (PRD §7.1).
func BenchmarkStartup_20Stashes(b *testing.B) {
	dir := benchRepo(b, 20)
	b.ResetTimer()

	for b.Loop() {
		runner := git.NewDefaultRunner(dir, nil)
		ctx := context.Background()
		_, _ = git.DetectVersion(ctx, runner)
		_, _ = runner.RunLines(ctx, "stash", "list", "--format=%H%n%h%n%gs%n%aI")
	}
}

// BenchmarkStartup_100Stashes benchmarks cold start with 100 stashes.
// Target: < 300ms (PRD §7.1).
func BenchmarkStartup_100Stashes(b *testing.B) {
	dir := benchRepo(b, 100)
	b.ResetTimer()

	for b.Loop() {
		runner := git.NewDefaultRunner(dir, nil)
		ctx := context.Background()
		_, _ = git.DetectVersion(ctx, runner)
		_, _ = runner.RunLines(ctx, "stash", "list", "--format=%H%n%h%n%gs%n%aI")
	}
}

// TestStartup_TimeFromProcessToFirstRender measures startup time from
// process start to first render with realistic stash counts.
func TestStartup_TimeFromProcessToFirstRender(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping startup timing test in short mode")
	}

	tests := []struct {
		name    string
		stashes int
		maxTime time.Duration
	}{
		{"0 stashes", 0, 100 * time.Millisecond},
		{"5 stashes", 5, 100 * time.Millisecond},
		{"20 stashes", 20, 100 * time.Millisecond},
		{"50 stashes", 50, 200 * time.Millisecond},
		{"100 stashes", 100, 300 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := testRepo(t, tt.stashes)

			start := time.Now()

			ctx := context.Background()
			runner := git.NewDefaultRunner(dir, nil)

			// Step 1: repo detection.
			_, err := runner.Run(ctx, "rev-parse", "--git-dir")
			if err != nil {
				t.Fatalf("rev-parse failed: %v", err)
			}

			// Step 2: version detection.
			ver, err := git.DetectVersion(ctx, runner)
			if err != nil {
				t.Fatalf("version detection failed: %v", err)
			}
			_ = ver

			// Step 3: stash list.
			lines, err := runner.RunLines(ctx, "stash", "list",
				"--format=%H%n%h%n%gs%n%aI")
			if err != nil {
				t.Fatalf("stash list failed: %v", err)
			}
			_ = lines

			elapsed := time.Since(start)

			t.Logf("Startup with %d stashes: %v (target: < %v)", tt.stashes, elapsed, tt.maxTime)

			if elapsed > tt.maxTime {
				t.Errorf("startup took %v, exceeds target of %v", elapsed, tt.maxTime)
			}
		})
	}
}

// BenchmarkGitStashList benchmarks raw `git stash list` parsing by size.
func BenchmarkGitStashList(b *testing.B) {
	sizes := []int{0, 5, 20, 50, 100}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("%d_stashes", n), func(b *testing.B) {
			dir := benchRepo(b, n)
			runner := git.NewDefaultRunner(dir, nil)
			ctx := context.Background()
			b.ResetTimer()

			for b.Loop() {
				_, err := runner.RunLines(ctx, "stash", "list",
					"--format=%H%n%h%n%gs%n%aI")
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkVersionDetection benchmarks git version detection.
func BenchmarkVersionDetection(b *testing.B) {
	dir := benchRepo(b, 0)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, err := git.DetectVersion(ctx, runner)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestStartup_DebugFlag verifies that --debug prints timing breakdown and exits.
func TestStartup_DebugFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping debug flag test in short mode")
	}

	dir := testRepo(t, 10)

	binPath := filepath.Join(t.TempDir(), "nidhi-debug")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/nidhi")
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	cmd = exec.Command(binPath, "-C", dir, "--debug")
	out, err = cmd.CombinedOutput()
	// --debug exits with code 0 after printing timing.
	if err != nil {
		t.Logf("output: %s", out)
		t.Fatalf("--debug exited with error: %v", err)
	}

	output := string(out)
	t.Logf("--debug output:\n%s", output)

	if !strings.Contains(output, "timing") && !strings.Contains(output, "TOTAL") {
		t.Error("--debug should print timing breakdown information")
	}
}
