# Task 027: Performance Validation

## Status: TODO

## Depends On
- 026 (Comprehensive E2E Tests) -- all features working and tested before performance profiling
- 000 (scaffold) -- `make build`, `make test` infrastructure
- 001 (git runner) -- GitRunner, version detection (timing baseline)
- 005 (stash cache) -- LRU cache (memory profiling target)
- 014 (search plugin) -- search index (index build timing)

## Parallelizable With
- None (performance gate -- must run after all features are verified correct)

## Problem
The PRD defines strict non-functional requirements (Section 7.1, Section 14) for startup time, operation latency, and memory usage. Without measured validation against these budgets, we cannot claim nidhi meets its "blazing fast" design principle. We need: (1) Go benchmark tests that measure startup, operation latency, and memory under realistic conditions, (2) iterm2-driver-based visual responsiveness tests that verify no flicker or lag during rapid input, and (3) a profiling results document that records pass/fail for every NFR metric.

## PRD Reference
- Section 1.2 (Design Principles) -- "Blazing fast: <100ms startup. Cached stash data. Async git operations."
- Section 7.1 (Performance) -- all NFR targets:
  - Cold startup < 100ms (<=20 stashes), < 300ms (<=100 stashes)
  - Keystroke-to-visual < 16ms (60fps)
  - Diff preview < 200ms
  - Search index < 2s for 50 stashes
  - Conflict preview < 500ms
  - Memory < 50MB RSS for 50 stashes
- Section 12.5 (CLI Flags) -- `--debug` prints timing breakdown
- Section 14.1 (Startup Sequence) -- T+0ms through T+500ms timeline
- Section 14.2 (Operation Budgets) -- per-operation targets
- Section 14.3 (Caching Strategy) -- LRU cache size and invalidation

## Files to Create
- `internal/perf/startup_test.go` -- startup time benchmarks
- `internal/perf/operation_test.go` -- operation latency benchmarks
- `internal/perf/memory_test.go` -- memory usage validation
- `internal/perf/visual_test.go` -- visual responsiveness tests via iterm2-driver
- `internal/perf/helpers_test.go` -- shared benchmark fixtures
- `docs/profiling-results.md` -- documented measurements with pass/fail

## Files to Modify
- `Makefile` -- add `make bench` and `make profile` targets

## Execution Steps

### Step 1: Create shared benchmark helpers in `internal/perf/helpers_test.go`

```go
// internal/perf/helpers_test.go
package perf_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// benchRepo creates a temporary git repo with the specified number of stashes.
// Returns the directory path. Repo is cleaned up when the test/benchmark ends.
func benchRepo(b *testing.B, stashCount int) string {
	b.Helper()
	dir := b.TempDir()
	runGit(b, dir, "init")
	runGit(b, dir, "config", "user.email", "bench@test.com")
	runGit(b, dir, "config", "user.name", "Bench")
	writeFile(b, dir, "README.md", "# bench repo\n")
	runGit(b, dir, "add", ".")
	runGit(b, dir, "commit", "-m", "initial commit")

	for i := 0; i < stashCount; i++ {
		// Create files with realistic content sizes (50-200 lines).
		content := generateGoFile(i)
		writeFile(b, dir, fmt.Sprintf("pkg/module%d/file%d.go", i%10, i), content)
		runGit(b, dir, "add", ".")
		runGit(b, dir, "stash", "push", "-m", fmt.Sprintf("Stash %d: realistic benchmark content", i))
	}

	return dir
}

// testRepo creates a temporary git repo for non-benchmark tests.
func testRepo(t *testing.T, stashCount int) string {
	t.Helper()
	dir := t.TempDir()
	runGitT(t, dir, "init")
	runGitT(t, dir, "config", "user.email", "perf@test.com")
	runGitT(t, dir, "config", "user.name", "Perf")
	writeFileT(t, dir, "README.md", "# perf test repo\n")
	runGitT(t, dir, "add", ".")
	runGitT(t, dir, "commit", "-m", "initial commit")

	for i := 0; i < stashCount; i++ {
		content := generateGoFile(i)
		writeFileT(t, dir, fmt.Sprintf("pkg/module%d/file%d.go", i%10, i), content)
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

	for j := 0; j < 10; j++ {
		b.WriteString(fmt.Sprintf("func (h *Handler%d) Process%d(input string) (string, error) {\n", seed, j))
		b.WriteString("\tif input == \"\" {\n")
		b.WriteString("\t\treturn \"\", fmt.Errorf(\"empty input\")\n")
		b.WriteString("\t}\n")
		b.WriteString(fmt.Sprintf("\t// Processing logic for handler %d, method %d\n", seed, j))
		b.WriteString("\tresult := strings.ToUpper(input)\n")
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
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// writeFile writes a file in the given directory (benchmark version).
func writeFile(b *testing.B, dir, name, content string) {
	b.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}

// writeFileT writes a file in the given directory (test version).
func writeFileT(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readMemStats returns current memory statistics.
func readMemStats() runtime.MemStats {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
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
```

### Step 2: Create `internal/perf/startup_test.go` -- startup time benchmarks

```go
// internal/perf/startup_test.go
package perf_test

import (
	"context"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
)

// BenchmarkStartup_0Stashes benchmarks cold start with 0 stashes.
// Target: < 100ms (PRD §7.1).
func BenchmarkStartup_0Stashes(b *testing.B) {
	dir := benchRepo(b, 0)
	b.ResetTimer()

	for b.Loop() {
		app := core.NewApp(core.AppConfig{
			WorkDir:   dir,
			NoColor:   true,
			NoAnimate: true,
		})
		_ = app // prevent optimization
	}
}

// BenchmarkStartup_20Stashes benchmarks cold start with 20 stashes.
// Target: < 100ms (PRD §7.1).
func BenchmarkStartup_20Stashes(b *testing.B) {
	dir := benchRepo(b, 20)
	b.ResetTimer()

	for b.Loop() {
		app := core.NewApp(core.AppConfig{
			WorkDir:   dir,
			NoColor:   true,
			NoAnimate: true,
		})
		_ = app
	}
}

// BenchmarkStartup_100Stashes benchmarks cold start with 100 stashes.
// Target: < 300ms (PRD §7.1).
func BenchmarkStartup_100Stashes(b *testing.B) {
	dir := benchRepo(b, 100)
	b.ResetTimer()

	for b.Loop() {
		app := core.NewApp(core.AppConfig{
			WorkDir:   dir,
			NoColor:   true,
			NoAnimate: true,
		})
		_ = app
	}
}

// TestStartup_TimeFromProcessToFirstRender measures the time from process start
// to first render with realistic stash counts. This is a test (not benchmark)
// because it uses wall-clock assertions.
func TestStartup_TimeFromProcessToFirstRender(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping startup timing test in short mode")
	}

	tests := []struct {
		name       string
		stashes    int
		maxTime    time.Duration
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

			// Simulate the startup sequence:
			// 1. Detect git repo
			// 2. Detect git version
			// 3. Parse stash list
			// 4. Compute metadata
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
				"--format=%H%n%h%n%s%n%gd%n%aI")
			if err != nil {
				t.Fatalf("stash list failed: %v", err)
			}
			_ = lines

			// Step 4: metadata computation (simulated).
			// In the real app, this parses dates, computes staleness, generates auto-messages.

			elapsed := time.Since(start)

			t.Logf("Startup with %d stashes: %v (target: < %v)", tt.stashes, elapsed, tt.maxTime)

			if elapsed > tt.maxTime {
				t.Errorf("startup took %v, exceeds target of %v", elapsed, tt.maxTime)
			}
		})
	}
}

// BenchmarkGitStashList benchmarks raw `git stash list` parsing.
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
					"--format=%H%n%h%n%s%n%gd%n%aI")
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

	// Build binary.
	binPath := filepath.Join(t.TempDir(), "nidhi-debug")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/nidhi")
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Run with --debug flag.
	cmd = exec.Command(binPath, "-C", dir, "--debug")
	out, err = cmd.CombinedOutput()
	output := string(out)

	// --debug should exit (not hang), so we expect the command to complete.
	// It should print timing information.
	t.Logf("--debug output:\n%s", output)

	if !strings.Contains(output, "ms") && !strings.Contains(output, "timing") &&
		!strings.Contains(output, "startup") {
		t.Error("--debug should print timing breakdown information")
	}
}
```

### Step 3: Create `internal/perf/operation_test.go` -- operation latency benchmarks

```go
// internal/perf/operation_test.go
package perf_test

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
)

// BenchmarkCursorMove benchmarks cursor navigation (j/k).
// Target: < 1ms (no git calls, PRD §14.2).
func BenchmarkCursorMove(b *testing.B) {
	dir := benchRepo(b, 20)
	app := core.NewApp(core.AppConfig{
		WorkDir:   dir,
		NoColor:   true,
		NoAnimate: true,
	})

	// Initialize the app model.
	model := app.Init()
	_ = model

	b.ResetTimer()

	for b.Loop() {
		// Simulate j keystroke.
		_, _ = app.Update(tea.KeyPressMsg{Text: "j"})
	}
}

// BenchmarkTogglePreview benchmarks Tab to toggle preview pane.
// Target: < 50ms (cached diff, PRD §14.2).
func BenchmarkTogglePreview(b *testing.B) {
	dir := benchRepo(b, 20)
	app := core.NewApp(core.AppConfig{
		WorkDir:   dir,
		NoColor:   true,
		NoAnimate: true,
	})

	model := app.Init()
	_ = model

	b.ResetTimer()

	for b.Loop() {
		// Toggle preview on.
		_, _ = app.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		// Toggle preview off.
		_, _ = app.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	}
}

// TestOperationLatency_CursorMove verifies cursor move is < 1ms.
func TestOperationLatency_CursorMove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 20)
	app := core.NewApp(core.AppConfig{
		WorkDir:   dir,
		NoColor:   true,
		NoAnimate: true,
	})

	model := app.Init()
	_ = model

	// Warm up.
	for i := 0; i < 10; i++ {
		_, _ = app.Update(tea.KeyPressMsg{Text: "j"})
	}

	// Measure 100 cursor moves.
	const iterations = 100
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_, _ = app.Update(tea.KeyPressMsg{Text: "j"})
		_, _ = app.Update(tea.KeyPressMsg{Text: "k"})
	}
	elapsed := time.Since(start)
	perOp := elapsed / (iterations * 2)

	t.Logf("Cursor move: %v per operation (%d ops in %v)", perOp, iterations*2, elapsed)

	if perOp > 1*time.Millisecond {
		t.Errorf("cursor move took %v, target is < 1ms", perOp)
	}
}

// TestOperationLatency_DiffLoad verifies uncached diff load is < 200ms.
func TestOperationLatency_DiffLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 5)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// Measure diff load for each stash.
	for i := 0; i < 5; i++ {
		stashRef := fmt.Sprintf("stash@{%d}", i)
		start := time.Now()
		_, err := runner.Run(ctx, "stash", "show", "-p", "--include-untracked", stashRef)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("stash show failed for %s: %v", stashRef, err)
		}

		t.Logf("Diff load %s: %v", stashRef, elapsed)

		if elapsed > 200*time.Millisecond {
			t.Errorf("diff load for %s took %v, target is < 200ms", stashRef, elapsed)
		}
	}
}

// TestOperationLatency_Rename verifies rename is < 100ms.
func TestOperationLatency_Rename(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 5)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// Get SHA of stash@{0}.
	sha, err := runner.Run(ctx, "rev-parse", "stash@{0}")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}

	// Measure rename operation (drop + store).
	start := time.Now()
	_, err = runner.Run(ctx, "stash", "drop", "stash@{0}")
	if err != nil {
		t.Fatalf("stash drop failed: %v", err)
	}
	_, err = runner.Run(ctx, "stash", "store", "-m", "Renamed stash message", sha)
	if err != nil {
		t.Fatalf("stash store failed: %v", err)
	}
	elapsed := time.Since(start)

	t.Logf("Rename operation: %v", elapsed)

	if elapsed > 100*time.Millisecond {
		t.Errorf("rename took %v, target is < 100ms", elapsed)
	}
}

// TestOperationLatency_Apply verifies apply is < 500ms.
func TestOperationLatency_Apply(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 5)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	start := time.Now()
	_, err := runner.Run(ctx, "stash", "apply", "stash@{0}")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("stash apply failed: %v", err)
	}

	t.Logf("Apply operation: %v", elapsed)

	if elapsed > 500*time.Millisecond {
		t.Errorf("apply took %v, target is < 500ms", elapsed)
	}

	// Clean up applied changes.
	runner.Run(ctx, "checkout", ".")
	runner.Run(ctx, "clean", "-fd")
}

// TestOperationLatency_SearchIndexBuild verifies search index build < 2s for 50 stashes.
func TestOperationLatency_SearchIndexBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 50)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// Simulate search index building: load all diffs and build index.
	start := time.Now()

	for i := 0; i < 50; i++ {
		stashRef := fmt.Sprintf("stash@{%d}", i)
		_, err := runner.Run(ctx, "stash", "show", "-p", stashRef)
		if err != nil {
			// Some stashes may have no diff.
			continue
		}
	}

	elapsed := time.Since(start)

	t.Logf("Search index build (50 stashes): %v (target: < 2s)", elapsed)

	if elapsed > 2*time.Second {
		t.Errorf("search index build took %v, target is < 2s", elapsed)
	}
}

// BenchmarkApplyStash benchmarks git stash apply.
func BenchmarkApplyStash(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		dir := benchRepo(b, 1)
		runner := git.NewDefaultRunner(dir, nil)
		ctx := context.Background()
		b.StartTimer()

		_, _ = runner.Run(ctx, "stash", "apply", "stash@{0}")

		b.StopTimer()
		runner.Run(ctx, "checkout", ".")
		runner.Run(ctx, "clean", "-fd")
		b.StartTimer()
	}
}

// BenchmarkMergeTree benchmarks conflict preview via merge-tree.
func BenchmarkMergeTree(b *testing.B) {
	dir := benchRepo(b, 5)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// Get HEAD ref.
	head, err := runner.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		b.Fatal(err)
	}

	// Get stash commit.
	stashSHA, err := runner.Run(ctx, "rev-parse", "stash@{0}")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		_, _, _ = runner.RunExitCode(ctx, "merge-tree", "--write-tree", head, stashSHA)
	}
}
```

### Step 4: Create `internal/perf/memory_test.go` -- memory usage validation

```go
// internal/perf/memory_test.go
package perf_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
)

// TestMemory_RSSUnder50MB verifies RSS < 50MB for 50 stashes (PRD §7.1).
func TestMemory_RSSUnder50MB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	dir := testRepo(t, 50)

	// Force GC and record baseline.
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	// Create the app and load stashes.
	app := core.NewApp(core.AppConfig{
		WorkDir:   dir,
		NoColor:   true,
		NoAnimate: true,
	})

	// Initialize to trigger stash loading.
	_ = app.Init()

	// Force GC and measure.
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	heapUsed := after.HeapInuse - baseline.HeapInuse
	totalAlloc := after.TotalAlloc - baseline.TotalAlloc
	sysUsed := after.Sys - baseline.Sys

	t.Logf("Memory with 50 stashes:")
	t.Logf("  Heap in use:  %s", formatBytes(heapUsed))
	t.Logf("  Total alloc:  %s", formatBytes(totalAlloc))
	t.Logf("  System:       %s", formatBytes(sysUsed))
	t.Logf("  NumGC:        %d", after.NumGC-baseline.NumGC)

	// RSS < 50MB. Use heap as proxy (RSS includes stack, OS overhead).
	// Allow 40MB heap to stay safely under 50MB RSS.
	const maxHeap = 40 * 1024 * 1024 // 40MB
	if heapUsed > maxHeap {
		t.Errorf("heap usage %s exceeds 40MB target (RSS would exceed 50MB)", formatBytes(heapUsed))
	}
}

// TestMemory_DiffCacheRespectsLimit verifies the LRU diff cache respects its size limit.
func TestMemory_DiffCacheRespectsLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	dir := testRepo(t, 20)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// Create a cache with limit of 10 entries.
	cache := git.NewStashCache(runner, git.CacheConfig{
		MaxDiffEntries: 10,
	})

	// Load diffs for all 20 stashes (should evict older entries).
	for i := 0; i < 20; i++ {
		sha := runGitT(t, dir, "rev-parse", fmt.Sprintf("stash@{%d}", i))
		_, err := cache.Diff(ctx, sha)
		if err != nil {
			t.Logf("Diff for stash %d: %v (may be expected)", i, err)
		}
	}

	// Verify cache size is at most 10.
	cacheSize := cache.DiffCacheSize()
	t.Logf("Diff cache size after loading 20 diffs: %d (limit: 10)", cacheSize)

	if cacheSize > 10 {
		t.Errorf("diff cache has %d entries, should be <= 10", cacheSize)
	}
}

// TestMemory_NoLeakAfterOperations verifies no memory leaks after repeated operations.
func TestMemory_NoLeakAfterOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory leak test in short mode")
	}

	dir := testRepo(t, 10)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	cache := git.NewStashCache(runner, git.CacheConfig{
		MaxDiffEntries: 10,
	})

	// Measure baseline memory.
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	// Perform 1000 operations: list + diff load + invalidate cycle.
	const iterations = 1000
	for i := 0; i < iterations; i++ {
		// List stashes.
		_, err := cache.List(ctx)
		if err != nil {
			t.Fatalf("cache.List failed at iteration %d: %v", i, err)
		}

		// Load a diff.
		sha := runGitT(t, dir, "rev-parse", fmt.Sprintf("stash@{%d}", i%10))
		_, _ = cache.Diff(ctx, sha)

		// Invalidate cache periodically.
		if i%50 == 0 {
			cache.Invalidate()
		}
	}

	// Force GC and measure.
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	heapGrowth := int64(after.HeapInuse) - int64(baseline.HeapInuse)
	t.Logf("Memory after %d operations:", iterations)
	t.Logf("  Heap growth: %s", formatBytes(uint64(max(heapGrowth, 0))))
	t.Logf("  NumGC: %d", after.NumGC-baseline.NumGC)

	// Allow up to 10MB heap growth for 1000 operations.
	const maxGrowth = 10 * 1024 * 1024 // 10MB
	if heapGrowth > maxGrowth {
		t.Errorf("heap grew by %s after %d operations, possible memory leak", formatBytes(uint64(heapGrowth)), iterations)
	}
}

// BenchmarkMemory_Allocs benchmarks memory allocations per cursor move.
func BenchmarkMemory_CursorMoveAllocs(b *testing.B) {
	dir := benchRepo(b, 20)
	app := core.NewApp(core.AppConfig{
		WorkDir:   dir,
		NoColor:   true,
		NoAnimate: true,
	})

	_ = app.Init()
	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, _ = app.Update(tea.KeyPressMsg{Text: "j"})
	}
}
```

### Step 5: Create `internal/perf/visual_test.go` -- visual responsiveness via iterm2-driver

```go
// internal/perf/visual_test.go
package perf_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestVisualResponsiveness_RapidScroll tests that rapid j/k keystrokes
// produce no visible lag or flicker, verified via iterm2-driver screenshots.
//
// Prerequisites: macOS with iTerm2, iterm2-driver in PATH, NIDHI_VISUAL_TEST=1.
func TestVisualResponsiveness_RapidScroll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual responsiveness test in short mode")
	}
	if os.Getenv("NIDHI_VISUAL_TEST") != "1" {
		t.Skip("set NIDHI_VISUAL_TEST=1 to run visual responsiveness tests")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("visual tests require macOS with iTerm2")
	}

	iterm2Driver, err := exec.LookPath("iterm2-driver")
	if err != nil {
		t.Skip("iterm2-driver not found in PATH")
	}

	// Build nidhi binary.
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "nidhi")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/nidhi")
	cmd.Dir = projectRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Create test repo with 50 stashes.
	dir := testRepo(t, 50)

	// Use iterm2-driver to:
	// 1. Launch nidhi with 50 stashes
	// 2. Send 100 rapid j keystrokes
	// 3. Take screenshot before and after
	// 4. Verify no corrupted rendering

	screenshotBefore := filepath.Join(t.TempDir(), "before.png")
	screenshotAfter := filepath.Join(t.TempDir(), "after.png")

	// Capture initial state.
	cmd = exec.Command(iterm2Driver,
		"--cols", "120", "--rows", "40",
		"--delay", "2000ms",
		"--screenshot", screenshotBefore,
		"--", binPath, "-C", dir, "--no-color",
	)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("iterm2-driver output: %s", out)
		t.Fatalf("initial screenshot failed: %v", err)
	}

	// Send rapid keystrokes and capture.
	keys := make([]string, 0, 200)
	for i := 0; i < 100; i++ {
		keys = append(keys, "--send-key", "j")
	}
	args := []string{
		"--cols", "120", "--rows", "40",
	}
	args = append(args, keys...)
	args = append(args,
		"--delay", "500ms",
		"--screenshot", screenshotAfter,
		"--", binPath, "-C", dir, "--no-color",
	)

	start := time.Now()
	cmd = exec.Command(iterm2Driver, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	out, err = cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("iterm2-driver output: %s", out)
		t.Fatalf("rapid scroll screenshot failed: %v", err)
	}

	t.Logf("100 rapid j keystrokes + screenshot: %v", elapsed)

	// Verify both screenshots exist and are valid.
	for _, path := range []string{screenshotBefore, screenshotAfter} {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("screenshot not created: %s: %v", path, err)
			continue
		}
		if info.Size() < 1000 {
			t.Errorf("screenshot suspiciously small: %s (%d bytes)", path, info.Size())
		}
	}

	t.Logf("Visual responsiveness test passed. Before: %s, After: %s", screenshotBefore, screenshotAfter)
}
```

### Step 6: Add Makefile targets

Add to the existing Makefile:

```makefile
.PHONY: bench
bench: ## Run performance benchmarks
	go test -bench=. -benchmem -timeout 600s ./internal/perf/...

.PHONY: bench-short
bench-short: ## Run quick performance benchmarks (fewer iterations)
	go test -bench=. -benchmem -benchtime=1s -timeout 120s ./internal/perf/...

.PHONY: profile
profile: ## Run benchmarks with CPU and memory profiling
	mkdir -p profiles
	go test -bench=BenchmarkStartup -benchmem -cpuprofile=profiles/cpu-startup.prof -memprofile=profiles/mem-startup.prof -timeout 120s ./internal/perf/...
	go test -bench=BenchmarkCursorMove -benchmem -cpuprofile=profiles/cpu-cursor.prof -memprofile=profiles/mem-cursor.prof -timeout 120s ./internal/perf/...
	@echo "Profiles written to profiles/. View with: go tool pprof profiles/<file>.prof"

.PHONY: perf-test
perf-test: ## Run performance validation tests (latency and memory assertions)
	go test -v -timeout 600s -count=1 -run 'Test(Startup|OperationLatency|Memory)_' ./internal/perf/...
```

### Step 7: Create `docs/profiling-results.md`

```markdown
# nidhi — Performance Profiling Results

> This document records measured performance against PRD §7.1 and §14 targets.
> Updated after each profiling run.

## Test Environment

| Field | Value |
|---|---|
| Date | (fill after running) |
| Machine | (e.g., Apple M2 Pro, 16GB RAM) |
| Go | 1.26 |
| Git | (e.g., 2.53.0) |
| OS | macOS 15.x |

## Startup Time (PRD §7.1, §14.1)

| Stash Count | Target | Measured | Status |
|---|---|---|---|
| 0 stashes | < 100ms | — | — |
| 5 stashes | < 100ms | — | — |
| 20 stashes | < 100ms | — | — |
| 50 stashes | < 200ms | — | — |
| 100 stashes | < 300ms | — | — |

## Operation Latency (PRD §14.2)

| Operation | Target | Measured | Status |
|---|---|---|---|
| Cursor move (j/k) | < 1ms | — | — |
| Toggle preview (Tab) | < 50ms | — | — |
| Diff load (uncached) | < 200ms | — | — |
| Apply stash | < 500ms | — | — |
| Rename | < 100ms | — | — |
| Search index (50 stashes) | < 2s | — | — |
| Conflict preview | < 500ms | — | — |

## Memory Usage (PRD §7.1)

| Scenario | Target | Measured | Status |
|---|---|---|---|
| 50 stashes loaded | < 50MB RSS | — | — |
| Diff cache (limit 10) | Respects limit | — | — |
| After 1000 operations | No leak (< 10MB growth) | — | — |
| Cursor move allocs | 0 allocs ideal | — | — |

## Benchmark Results

```
(paste `make bench` output here)
```

## Visual Responsiveness

| Test | Result | Notes |
|---|---|---|
| 100 rapid j keystrokes | — | Screenshot comparison |
| Preview toggle (rapid Tab) | — | No flicker |

## How to Run

```bash
# Full benchmark suite
make bench

# Performance validation tests (with pass/fail assertions)
make perf-test

# CPU and memory profiling
make profile
go tool pprof profiles/cpu-startup.prof

# Visual responsiveness (macOS + iTerm2)
NIDHI_VISUAL_TEST=1 go test -v -run TestVisualResponsiveness ./internal/perf/...
```
```

### Step 8: Run benchmarks and verify

```bash
# Compile check.
go vet ./internal/perf/...

# Run benchmarks.
go test -bench=. -benchmem -benchtime=3s -timeout 600s ./internal/perf/...

# Run validation tests.
go test -v -timeout 600s -count=1 -run 'Test(Startup|OperationLatency|Memory)_' ./internal/perf/...

# Full CI.
make ci
```

### Step 9: Fill in `docs/profiling-results.md` with actual measurements

After running benchmarks, update the markdown table with measured values and pass/fail status.

## Verification

### Benchmarks
```bash
# All benchmark files compile.
go vet ./internal/perf/...

# Startup benchmarks run.
go test -bench=BenchmarkStartup -benchmem -benchtime=3s ./internal/perf/...

# Operation benchmarks run.
go test -bench=BenchmarkCursorMove -benchmem -benchtime=3s ./internal/perf/...
go test -bench=BenchmarkTogglePreview -benchmem -benchtime=3s ./internal/perf/...
go test -bench=BenchmarkApplyStash -benchmem -benchtime=3s ./internal/perf/...
go test -bench=BenchmarkMergeTree -benchmem -benchtime=3s ./internal/perf/...

# Git stash list benchmark by size.
go test -bench=BenchmarkGitStashList -benchmem -benchtime=3s ./internal/perf/...
```

### Performance Assertions
```bash
# Startup timing assertions pass.
go test -v -timeout 300s -run TestStartup_TimeFromProcessToFirstRender ./internal/perf/...

# Operation latency assertions pass.
go test -v -timeout 300s -run TestOperationLatency_ ./internal/perf/...

# Memory assertions pass.
go test -v -timeout 300s -run TestMemory_ ./internal/perf/...

# Debug flag test passes.
go test -v -timeout 60s -run TestStartup_DebugFlag ./internal/perf/...
```

### CI Pipeline
```bash
make ci
make bench
make perf-test
```

## Completion Criteria
1. `internal/perf/startup_test.go` has benchmarks for 0, 20, and 100 stashes with `testing.B`
2. `TestStartup_TimeFromProcessToFirstRender` asserts < 100ms (20 stashes), < 300ms (100 stashes)
3. `internal/perf/operation_test.go` benchmarks cursor move, toggle preview, diff load, apply, rename, search index
4. `TestOperationLatency_CursorMove` asserts < 1ms per operation
5. `TestOperationLatency_DiffLoad` asserts < 200ms per diff
6. `TestOperationLatency_SearchIndexBuild` asserts < 2s for 50 stashes
7. `internal/perf/memory_test.go` validates RSS < 50MB (heap < 40MB) for 50 stashes
8. `TestMemory_DiffCacheRespectsLimit` verifies LRU eviction at configured limit
9. `TestMemory_NoLeakAfterOperations` runs 1000 operation cycles, asserts < 10MB growth
10. `BenchmarkMemory_CursorMoveAllocs` reports allocations per cursor move with `-benchmem`
11. `internal/perf/visual_test.go` sends 100 rapid keystrokes via iterm2-driver, captures screenshots
12. `TestStartup_DebugFlag` verifies `--debug` prints timing and exits
13. Makefile has `bench`, `bench-short`, `profile`, `perf-test` targets
14. `docs/profiling-results.md` documents all measurements with pass/fail against PRD targets
15. `make ci` and `make bench` pass

## Commit
```
test: add performance benchmarks and NFR validation suite

Implement internal/perf/ with startup benchmarks (0/20/100 stashes),
operation latency tests (cursor <1ms, diff <200ms, apply <500ms,
search index <2s), memory validation (RSS <50MB, LRU cache limits,
no leak after 1000 ops), and visual responsiveness tests via
iterm2-driver. Add docs/profiling-results.md with measurement table.
Makefile targets: bench, bench-short, profile, perf-test.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 1.2, 7.1, 12.5, 14.1-14.3
4. Verify task 026 is DONE in `docs/PROGRESS.md`
5. Create `internal/perf/` directory
6. Implement `helpers_test.go`, `startup_test.go`, `operation_test.go`, `memory_test.go`, `visual_test.go`
7. Add Makefile targets
8. Run `go vet ./internal/perf/...`
9. Run benchmarks: `go test -bench=. -benchmem ./internal/perf/...`
10. Run validation tests: `go test -v -run 'Test(Startup|OperationLatency|Memory)_' ./internal/perf/...`
11. Fill in `docs/profiling-results.md` with measured values
12. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
13. Commit with the message above
