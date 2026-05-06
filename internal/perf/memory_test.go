package perf_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

// TestMemory_RSSUnder50MB verifies heap < 40MB for 50 stashes (PRD §7.1: RSS < 50MB).
func TestMemory_RSSUnder50MB(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping memory test in short mode")
	}

	dir := testRepo(t, 50)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	// Load stash list and preload diffs (simulates app startup).
	cache := git.NewDefaultStashCache(runner, 0, 50)
	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("list stashes: %v", err)
	}

	// Preload first 10 diffs.
	cache.PreloadDiffs(ctx, 10)

	_ = stashes

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	heapDelta := int64(after.HeapInuse) - int64(baseline.HeapInuse)
	heapUsed := uint64(max(heapDelta, 0))
	totalAlloc := after.TotalAlloc - baseline.TotalAlloc

	t.Logf("Memory with 50 stashes + 10 preloaded diffs:")
	t.Logf("  Heap in use:  %s", formatBytes(heapUsed))
	t.Logf("  Total alloc:  %s", formatBytes(totalAlloc))
	t.Logf("  NumGC:        %d", after.NumGC-baseline.NumGC)

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
	cache := git.NewDefaultStashCache(runner, 0, 10)

	// List stashes to populate the list cache.
	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("list stashes: %v", err)
	}

	// Load diffs for all 20 stashes (should evict older entries).
	for _, s := range stashes {
		_, _ = cache.Diff(ctx, s.SHA)
	}

	cacheSize := cache.DiffCacheSize()
	t.Logf("Diff cache size after loading %d diffs: %d (limit: 10)", len(stashes), cacheSize)

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

	cache := git.NewDefaultStashCache(runner, 0, 10)

	// Warm up the cache.
	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("list stashes: %v", err)
	}

	// Measure baseline memory.
	runtime.GC()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)

	// Perform 100 operation cycles: list + diff load + invalidate.
	const iterations = 100
	for i := range iterations {
		_, err := cache.List(ctx)
		if err != nil {
			t.Fatalf("cache.List failed at iteration %d: %v", i, err)
		}

		// Load a diff for a random stash.
		_, _ = cache.Diff(ctx, stashes[i%len(stashes)].SHA)

		// Invalidate cache periodically.
		if i%20 == 0 {
			cache.Invalidate()
		}
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	heapGrowth := int64(after.HeapInuse) - int64(baseline.HeapInuse)
	t.Logf("Memory after %d operations:", iterations)
	t.Logf("  Heap growth: %s", formatBytes(uint64(max(heapGrowth, 0))))
	t.Logf("  NumGC: %d", after.NumGC-baseline.NumGC)

	const maxGrowth = 10 * 1024 * 1024 // 10MB
	if heapGrowth > maxGrowth {
		t.Errorf("heap grew by %s after %d operations, possible memory leak",
			formatBytes(uint64(heapGrowth)), iterations)
	}
}

// BenchmarkStashListParsing benchmarks stash list parsing with cache.
func BenchmarkStashListParsing(b *testing.B) {
	sizes := []int{10, 50, 100}
	for _, n := range sizes {
		b.Run(fmt.Sprintf("%d_stashes", n), func(b *testing.B) {
			dir := benchRepo(b, n)
			runner := git.NewDefaultRunner(dir, nil)
			ctx := context.Background()
			b.ResetTimer()

			for b.Loop() {
				// Force fresh parse each time.
				_, err := git.ListStashes(ctx, runner, 0)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkDiffLoad benchmarks diff loading.
func BenchmarkDiffLoad(b *testing.B) {
	dir := benchRepo(b, 5)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()
	b.ResetTimer()

	for b.Loop() {
		_, _ = runner.Run(ctx, "stash", "show", "-p", "--include-untracked", "stash@{0}")
	}
}
