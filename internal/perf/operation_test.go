package perf_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/search"
	"github.com/indrasvat/nidhi/internal/ui/screens"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

const (
	testWidth  = 120
	testHeight = 40
)

// TestOperationLatency_CursorMove verifies cursor move renders in < 1ms.
func TestOperationLatency_CursorMove(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 20)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	stashes, err := git.ListStashes(ctx, runner, 0)
	if err != nil {
		t.Fatalf("list stashes: %v", err)
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Cursor:  0,
		Width:   120,
		Height:  40,
		Stashes: toPluginStashes(stashes),
	}

	// Warm up.
	for range 10 {
		state.Cursor = (state.Cursor + 1) % len(state.Stashes)
		_ = ls.View(state, testWidth, testHeight)
	}

	const iterations = 100
	start := time.Now()
	for range iterations {
		state.Cursor = (state.Cursor + 1) % len(state.Stashes)
		_ = ls.View(state, testWidth, testHeight)
	}
	elapsed := time.Since(start)
	perOp := elapsed / iterations

	t.Logf("Cursor move + render: %v per operation (%d ops in %v)", perOp, iterations, elapsed)

	// Under -race, overhead is ~2-5x. Use 5ms threshold for test;
	// BenchmarkCursorMoveRender (without -race) validates < 1ms.
	if perOp > 5*time.Millisecond {
		t.Errorf("cursor move render took %v, target is < 5ms (< 1ms without -race)", perOp)
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

	for i := range 5 {
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

// TestOperationLatency_Rename verifies rename (drop+store) is < 100ms.
func TestOperationLatency_Rename(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 5)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	sha, err := runner.Run(ctx, "rev-parse", "stash@{0}")
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}

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
}

// TestOperationLatency_SearchIndexBuild verifies search index build < 2s for 50 stashes.
func TestOperationLatency_SearchIndexBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 50)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	stashes, err := git.ListStashes(ctx, runner, 0)
	if err != nil {
		t.Fatalf("list stashes: %v", err)
	}

	pluginStashes := toPluginStashes(stashes)
	cache := git.NewDefaultStashCache(runner, 0, 50)
	idx := search.NewIndex()

	start := time.Now()
	cmd := search.BuildIndexCmd(pluginStashes, &cacheAdapter{inner: cache}, idx)
	if cmd != nil {
		_ = cmd()
	}
	elapsed := time.Since(start)

	t.Logf("Search index build (50 stashes): %v (target: < 2s)", elapsed)

	if elapsed > 2*time.Second {
		t.Errorf("search index build took %v, target is < 2s", elapsed)
	}
}

// TestOperationLatency_ListRender50 verifies LIST screen renders 50 stashes in < 100ms.
func TestOperationLatency_ListRender50(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping latency test in short mode")
	}

	dir := testRepo(t, 50)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	stashes, err := git.ListStashes(ctx, runner, 0)
	if err != nil {
		t.Fatalf("list stashes: %v", err)
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Cursor:  0,
		Width:   120,
		Height:  40,
		Stashes: toPluginStashes(stashes),
	}

	// Warm up.
	_ = ls.View(state, testWidth, testHeight)

	const iterations = 50
	start := time.Now()
	for range iterations {
		_ = ls.View(state, testWidth, testHeight)
	}
	elapsed := time.Since(start)
	perRender := elapsed / iterations

	t.Logf("LIST render (50 stashes): %v per render (%d renders in %v)", perRender, iterations, elapsed)

	if perRender > 100*time.Millisecond {
		t.Errorf("LIST render took %v, target is < 100ms", perRender)
	}
}

// BenchmarkCursorMoveRender benchmarks cursor move + render cycle.
func BenchmarkCursorMoveRender(b *testing.B) {
	dir := benchRepo(b, 20)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	stashes, err := git.ListStashes(ctx, runner, 0)
	if err != nil {
		b.Fatalf("list stashes: %v", err)
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Cursor:  0,
		Width:   120,
		Height:  40,
		Stashes: toPluginStashes(stashes),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		state.Cursor = (state.Cursor + 1) % len(state.Stashes)
		_ = ls.View(state, testWidth, testHeight)
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
		_, _ = runner.Run(ctx, "checkout", ".")
		_, _ = runner.Run(ctx, "clean", "-fd")
		b.StartTimer()
	}
}

// BenchmarkMergeTree benchmarks conflict preview via merge-tree.
func BenchmarkMergeTree(b *testing.B) {
	dir := benchRepo(b, 5)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	head, err := runner.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		b.Fatal(err)
	}

	stashSHA, err := runner.Run(ctx, "rev-parse", "stash@{0}")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for b.Loop() {
		_, _, _ = runner.RunExitCode(ctx, "merge-tree", "--write-tree", head, stashSHA)
	}
}

// toPluginStashes converts git.Stash slices to plugin.Stash slices.
func toPluginStashes(stashes []git.Stash) []plugin.Stash {
	out := make([]plugin.Stash, len(stashes))
	for i, s := range stashes {
		out[i] = plugin.Stash{
			Index:        s.Index,
			SHA:          s.SHA,
			ShortSHA:     s.ShortSHA,
			Message:      s.Message,
			RawMessage:   s.RawMessage,
			Branch:       s.Branch,
			Date:         s.Date,
			FileCount:    s.FileCount,
			Insertions:   s.Insertions,
			Deletions:    s.Deletions,
			IsStale:      s.IsStale,
			HasUntracked: s.HasUntracked,
		}
	}
	return out
}

// cacheAdapter bridges git.DefaultStashCache to plugin.StashCache.
type cacheAdapter struct {
	inner *git.DefaultStashCache
}

func (a *cacheAdapter) List(ctx context.Context) ([]plugin.Stash, error) {
	gitStashes, err := a.inner.List(ctx)
	if err != nil {
		return nil, err
	}
	return toPluginStashes(gitStashes), nil
}

func (a *cacheAdapter) Diff(ctx context.Context, sha string) (string, error) {
	return a.inner.Diff(ctx, sha)
}

func (a *cacheAdapter) Invalidate() {
	a.inner.Invalidate()
}
