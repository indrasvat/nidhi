package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/filter"
	"github.com/indrasvat/nidhi/internal/plugins/reorder"
	"github.com/indrasvat/nidhi/internal/plugins/search"
	"github.com/indrasvat/nidhi/internal/plugins/stale"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// =============================================================================
// Search Plugin E2E (FR-15)
// =============================================================================

func TestE2E_SearchPlugin_BuildIndexAndQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	// Create stashes with distinct messages.
	for _, msg := range []string{
		"fix: login timeout on slow networks",
		"feat: add dashboard widget",
		"chore: update dependencies",
		"fix: null pointer in auth module",
		"feat: new cache layer",
	} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}

	stashes := buildPluginStashes(t, dir)

	// Build the search index using BuildIndexCmd (the real async path).
	idx := search.NewIndex()
	cmd := search.BuildIndexCmd(stashes, &pluginNoopCache{}, idx)
	msg := cmd() // Execute synchronously for testing.

	readyMsg, ok := msg.(search.IndexReadyMsg)
	if !ok {
		t.Fatalf("expected IndexReadyMsg, got %T", msg)
	}
	if readyMsg.EntryCount == 0 {
		t.Fatal("index should have entries after build")
	}

	// Search for "fix" should match 2 stashes.
	results := idx.Search("fix", search.ScopeAll)
	if len(results) < 2 {
		t.Errorf("search 'fix': got %d results, want >= 2", len(results))
	}

	// Search for "dashboard" should match 1 stash.
	results = idx.Search("dashboard", search.ScopeAll)
	if len(results) < 1 {
		t.Errorf("search 'dashboard': got %d results, want >= 1", len(results))
	}

	// Search for "nonexistent" should match 0.
	results = idx.Search("xyznonexistent", search.ScopeAll)
	if len(results) != 0 {
		t.Errorf("search 'xyznonexistent': got %d results, want 0", len(results))
	}
}

func TestE2E_SearchPlugin_ScopeFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	// Create stashes on different branches.
	writeFile(t, dir, "file.go", "package main // main branch stash\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "main branch work")

	gitCmd(t, dir, "checkout", "-b", "feat/search")
	writeFile(t, dir, "search.go", "package main\nfunc search() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "search feature work")
	gitCmd(t, dir, "checkout", "main")

	stashes := buildPluginStashes(t, dir)

	idx := search.NewIndex()
	cmd := search.BuildIndexCmd(stashes, &pluginNoopCache{}, idx)
	cmd() // Build synchronously.

	// ScopeAll should find both.
	results := idx.Search("work", search.ScopeAll)
	if len(results) < 2 {
		t.Errorf("ScopeAll search 'work': got %d results, want >= 2", len(results))
	}

	// ScopeMessages should find both (both have "work" in message).
	results = idx.Search("work", search.ScopeMessages)
	if len(results) < 2 {
		t.Errorf("ScopeMessages search 'work': got %d results, want >= 2", len(results))
	}
}

func TestE2E_SearchPlugin_Activation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	p := search.New(th)
	pctx := plugin.PluginContext{
		Logger: slog.Default(),
		Cache:  &pluginNoopCache{},
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	stashes := makePluginStashes(5)
	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: stashes,
		Cursor:  0,
	}

	// "/" should activate search mode.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "/"}, state)
	if state.Mode != plugin.ModeSearch {
		t.Errorf("mode = %v, want ModeSearch after '/'", state.Mode)
	}
}

func TestE2E_SearchPlugin_EmptyQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	idx := search.NewIndex()
	// Build with some entries.
	idx.AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "abc", Message: "test stash", Scope: search.ScopeMessages},
	})
	idx.MarkReadyForTest()

	// Empty query should return no results.
	results := idx.Search("", search.ScopeAll)
	if len(results) != 0 {
		t.Errorf("empty query: got %d results, want 0", len(results))
	}
}

// =============================================================================
// Filter Plugin E2E (FR-17)
// =============================================================================

func TestE2E_FilterPlugin_BranchFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	p := filter.New()
	pctx := plugin.PluginContext{Logger: slog.Default()}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init filter: %v", err)
	}

	stashes := []plugin.Stash{
		{Index: 0, Message: "on main", Branch: "main"},
		{Index: 1, Message: "on feature", Branch: "feat/search"},
		{Index: 2, Message: "also on main", Branch: "main"},
		{Index: 3, Message: "on develop", Branch: "develop"},
	}

	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: stashes,
		Cursor:  0,
		Branch:  "main",
	}

	// Toggle branch filter with "f".
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)

	// Should have an active filter.
	if len(state.Filters) == 0 {
		t.Fatal("expected active filters after pressing 'f'")
	}

	var found bool
	for _, f := range state.Filters {
		if f.ID == filter.FilterIDBranch {
			found = true
			if f.Value != "main" {
				t.Errorf("branch filter value = %q, want 'main'", f.Value)
			}
		}
	}
	if !found {
		t.Error("expected branch filter in active filters")
	}

	// Toggle off with "f" again.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	for _, f := range state.Filters {
		if f.ID == filter.FilterIDBranch {
			t.Error("branch filter should be removed after second 'f'")
		}
	}
}

func TestE2E_FilterPlugin_StaleFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	p := filter.New()
	pctx := plugin.PluginContext{Logger: slog.Default()}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init filter: %v", err)
	}

	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: makePluginStashes(5),
		Cursor:  0,
	}

	// Toggle stale filter with "F".
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "F"}, state)

	if len(state.Filters) == 0 {
		t.Fatal("expected active filters after pressing 'F'")
	}

	var found bool
	for _, f := range state.Filters {
		if f.ID == filter.FilterIDStale {
			found = true
		}
	}
	if !found {
		t.Error("expected stale filter in active filters")
	}

	// Toggle off.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "F"}, state)
	for _, f := range state.Filters {
		if f.ID == filter.FilterIDStale {
			t.Error("stale filter should be removed after second 'F'")
		}
	}
}

func TestE2E_FilterPlugin_BothFiltersActive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	p := filter.New()
	pctx := plugin.PluginContext{Logger: slog.Default()}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init filter: %v", err)
	}

	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: makePluginStashes(5),
		Cursor:  0,
		Branch:  "main",
	}

	// Activate both filters.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "F"}, state)

	if len(state.Filters) < 2 {
		t.Errorf("expected 2 active filters, got %d", len(state.Filters))
	}

	hasBranch, hasStale := false, false
	for _, f := range state.Filters {
		if f.ID == filter.FilterIDBranch {
			hasBranch = true
		}
		if f.ID == filter.FilterIDStale {
			hasStale = true
		}
	}
	if !hasBranch || !hasStale {
		t.Errorf("expected both filters active: branch=%v stale=%v", hasBranch, hasStale)
	}
}

// =============================================================================
// Stale Detection E2E (FR-17)
// =============================================================================

func TestE2E_StaleDetection_MarkStale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	now := time.Now()
	stashes := []plugin.Stash{
		{Index: 0, Message: "recent", Date: now.Add(-1 * time.Hour)},
		{Index: 1, Message: "week old", Date: now.Add(-7 * 24 * time.Hour)},
		{Index: 2, Message: "month old", Date: now.Add(-30 * 24 * time.Hour)},
		{Index: 3, Message: "just now", Date: now},
	}

	threshold := 14 * 24 * time.Hour // 14 days
	marked := stale.MarkStaleWithTime(stashes, now, threshold)

	expectations := []struct {
		index int
		stale bool
	}{
		{0, false}, // 1 hour old
		{1, false}, // 7 days old
		{2, true},  // 30 days old
		{3, false}, // just now
	}

	for _, exp := range expectations {
		if marked[exp.index].IsStale != exp.stale {
			t.Errorf("stash %d: IsStale = %v, want %v (msg: %s)",
				exp.index, marked[exp.index].IsStale, exp.stale, marked[exp.index].Message)
		}
	}
}

func TestE2E_StaleDetection_StaleCountAndFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	now := time.Now()
	stashes := []plugin.Stash{
		{Index: 0, Message: "recent", Date: now},
		{Index: 1, Message: "old1", Date: now.Add(-30 * 24 * time.Hour)},
		{Index: 2, Message: "recent2", Date: now.Add(-1 * time.Hour)},
		{Index: 3, Message: "old2", Date: now.Add(-60 * 24 * time.Hour)},
		{Index: 4, Message: "old3", Date: now.Add(-90 * 24 * time.Hour)},
	}

	threshold := 14 * 24 * time.Hour
	marked := stale.MarkStaleWithTime(stashes, now, threshold)

	count := stale.StaleCount(marked)
	if count != 3 {
		t.Errorf("StaleCount = %d, want 3", count)
	}

	staleList := stale.StaleStashes(marked)
	if len(staleList) != 3 {
		t.Errorf("StaleStashes = %d entries, want 3", len(staleList))
	}
}

func TestE2E_StaleDetection_BulkDrop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	// Create 5 stashes.
	for i := range 5 {
		writeFile(t, dir, "file.go", fmt.Sprintf("package main // v%d\n", i))
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", fmt.Sprintf("stash %d", i))
	}

	if got := stashCount(t, dir); got != 5 {
		t.Fatalf("initial stash count = %d, want 5", got)
	}

	// Build stashes and manually mark indices 0 and 2 as stale.
	stashes := buildPluginStashes(t, dir)
	stashes[0].IsStale = true
	stashes[2].IsStale = true

	staleSub := stale.StaleStashes(stashes)
	if len(staleSub) != 2 {
		t.Fatalf("expected 2 stale stashes, got %d", len(staleSub))
	}

	runner := git.NewDefaultRunner(dir, nil)
	cmd := stale.BulkDropStaleCmd(staleSub, runner)
	msg := cmd()

	result, ok := msg.(stale.BulkDropResultMsg)
	if !ok {
		t.Fatalf("expected BulkDropResultMsg, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("BulkDrop error: %v", result.Err)
	}
	if result.Dropped != 2 {
		t.Errorf("BulkDrop dropped = %d, want 2", result.Dropped)
	}

	if got := stashCount(t, dir); got != 3 {
		t.Errorf("stash count after bulk drop = %d, want 3", got)
	}
}

// =============================================================================
// Reorder Plugin E2E (FR-16)
// =============================================================================

func TestE2E_ReorderPlugin_MoveDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dir := setupTestRepo(t, 0)
	for _, msg := range []string{"alpha", "beta", "gamma", "delta"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}
	// Order: delta(0), gamma(1), beta(2), alpha(3)

	stashes := buildPluginStashes(t, dir)
	runner := git.NewDefaultRunner(dir, nil)

	p := reorder.New()
	pctx := plugin.PluginContext{
		Git:    runner,
		Cache:  &pluginNoopCache{},
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init reorder: %v", err)
	}

	// Move stash@{0} (delta) down to position 1.
	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: stashes,
		Cursor:  0,
	}

	state, cmd := p.HandleKey(plugin.KeyEvent{Key: "J"}, state)

	// Cursor should follow the moved stash.
	if state.Cursor != 1 {
		t.Errorf("cursor = %d, want 1 after J", state.Cursor)
	}

	// Execute the reorder command.
	if cmd != nil {
		msg := cmd()
		result, ok := msg.(reorder.ReorderCompleteMsg)
		if !ok {
			t.Fatalf("expected ReorderCompleteMsg, got %T", msg)
		}
		if result.Error != nil {
			t.Fatalf("reorder error: %v", result.Error)
		}
	}

	// Verify new order: gamma(0), delta(1), beta(2), alpha(3).
	msgs := stashMessages(t, dir)
	expected := []string{"gamma", "delta", "beta", "alpha"}
	for i, want := range expected {
		if !strings.Contains(msgs[i], want) {
			t.Errorf("stash@{%d} = %q, want to contain %q", i, msgs[i], want)
		}
	}
}

func TestE2E_ReorderPlugin_MoveUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dir := setupTestRepo(t, 0)
	for _, msg := range []string{"alpha", "beta", "gamma"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}
	// Order: gamma(0), beta(1), alpha(2)

	stashes := buildPluginStashes(t, dir)
	runner := git.NewDefaultRunner(dir, nil)

	p := reorder.New()
	pctx := plugin.PluginContext{
		Git:    runner,
		Cache:  &pluginNoopCache{},
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init reorder: %v", err)
	}

	// Move stash@{2} (alpha) up to position 1.
	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: stashes,
		Cursor:  2,
	}

	state, cmd := p.HandleKey(plugin.KeyEvent{Key: "K"}, state)

	if state.Cursor != 1 {
		t.Errorf("cursor = %d, want 1 after K", state.Cursor)
	}

	if cmd != nil {
		msg := cmd()
		result, ok := msg.(reorder.ReorderCompleteMsg)
		if !ok {
			t.Fatalf("expected ReorderCompleteMsg, got %T", msg)
		}
		if result.Error != nil {
			t.Fatalf("reorder error: %v", result.Error)
		}
	}

	// Verify new order: gamma(0), alpha(1), beta(2).
	msgs := stashMessages(t, dir)
	expected := []string{"gamma", "alpha", "beta"}
	for i, want := range expected {
		if !strings.Contains(msgs[i], want) {
			t.Errorf("stash@{%d} = %q, want to contain %q", i, msgs[i], want)
		}
	}
}

func TestE2E_ReorderPlugin_BoundaryNoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	p := reorder.New()
	pctx := plugin.PluginContext{
		Logger: slog.Default(),
		Cache:  &pluginNoopCache{},
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init reorder: %v", err)
	}

	stashes := makePluginStashes(3)
	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: stashes,
		Cursor:  0,
	}

	// K at top → no-op.
	state, cmd := p.HandleKey(plugin.KeyEvent{Key: "K"}, state)
	if state.Cursor != 0 {
		t.Errorf("K at top: cursor = %d, want 0", state.Cursor)
	}
	if cmd != nil {
		t.Error("K at top should produce nil cmd")
	}

	// J at bottom → no-op.
	state.Cursor = 2
	state, cmd = p.HandleKey(plugin.KeyEvent{Key: "J"}, state)
	if state.Cursor != 2 {
		t.Errorf("J at bottom: cursor = %d, want 2", state.Cursor)
	}
	if cmd != nil {
		t.Error("J at bottom should produce nil cmd")
	}
}

func TestE2E_ReorderPlugin_MultipleSwaps(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dir := setupTestRepo(t, 0)
	for _, msg := range []string{"A", "B", "C", "D"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}
	// Order: D(0), C(1), B(2), A(3)

	runner := git.NewDefaultRunner(dir, nil)

	// Move D from 0 to 3 using 3 consecutive "J" presses.
	for i := range 3 {
		stashes := buildPluginStashes(t, dir)
		p := reorder.New()
		pctx := plugin.PluginContext{
			Git:    runner,
			Cache:  &pluginNoopCache{},
			Logger: slog.Default(),
		}
		if err := p.Init(pctx); err != nil {
			t.Fatalf("init reorder %d: %v", i, err)
		}

		state := plugin.AppState{
			Mode:    plugin.ModeList,
			Stashes: stashes,
			Cursor:  i,
		}

		_, cmd := p.HandleKey(plugin.KeyEvent{Key: "J"}, state)
		if cmd != nil {
			msg := cmd()
			if result, ok := msg.(reorder.ReorderCompleteMsg); ok && result.Error != nil {
				t.Fatalf("reorder step %d error: %v", i, result.Error)
			}
		}
	}

	// Expected: C(0), B(1), A(2), D(3).
	msgs := stashMessages(t, dir)
	expected := []string{"C", "B", "A", "D"}
	for i, want := range expected {
		if !strings.Contains(msgs[i], want) {
			t.Errorf("stash@{%d} = %q, want to contain %q", i, msgs[i], want)
		}
	}
}

// =============================================================================
// Cross-Feature: Reorder + Rename + Undo
// =============================================================================

func TestE2E_ReorderThenRenameThenDrop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dir := setupTestRepo(t, 0)
	for _, msg := range []string{"first", "second", "third"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}
	// Order: third(0), second(1), first(2)

	runner := git.NewDefaultRunner(dir, nil)

	// Reorder: move "first" from pos 2 to pos 1 using K.
	stashes := buildPluginStashes(t, dir)
	p := reorder.New()
	pctx := plugin.PluginContext{
		Git:    runner,
		Cache:  &pluginNoopCache{},
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init reorder: %v", err)
	}

	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: stashes,
		Cursor:  2,
	}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "K"}, state)
	if cmd != nil {
		msg := cmd()
		if result, ok := msg.(reorder.ReorderCompleteMsg); ok && result.Error != nil {
			t.Fatalf("reorder error: %v", result.Error)
		}
	}

	// Now: third(0), first(1), second(2).
	msgs := stashMessages(t, dir)
	if !strings.Contains(msgs[1], "first") {
		t.Fatalf("after reorder: stash@{1} = %q, want 'first'", msgs[1])
	}

	// Rename stash@{1} "first" → "renamed".
	stashes = buildPluginStashes(t, dir)
	rp := newRenamePlugin(t, dir)
	if err := rp.RenameStash(context.Background(), stashes, 1, "renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	msgs = stashMessages(t, dir)
	if !strings.Contains(msgs[1], "renamed") {
		t.Errorf("after rename: stash@{1} = %q, want 'renamed'", msgs[1])
	}

	// Drop stash@{1}.
	sha := stashSHA(t, dir, 1)
	gitCmd(t, dir, "stash", "drop", "stash@{1}")

	if got := stashCount(t, dir); got != 2 {
		t.Fatalf("count after drop = %d, want 2", got)
	}

	// Undo: restore.
	ctx := context.Background()
	_, err := runner.Run(ctx, "stash", "store", "-m", "renamed", sha)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	if got := stashCount(t, dir); got != 3 {
		t.Fatalf("count after undo = %d, want 3", got)
	}

	// Verify restored stash has renamed message.
	msgs = stashMessages(t, dir)
	if !strings.Contains(msgs[0], "renamed") {
		t.Errorf("restored stash message = %q, want 'renamed'", msgs[0])
	}
}

// =============================================================================
// Cross-Feature: Filter + Stale
// =============================================================================

func TestE2E_FilterAndStaleIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	now := time.Now()
	stashes := []plugin.Stash{
		{Index: 0, Message: "recent main", Branch: "main", Date: now},
		{Index: 1, Message: "old main", Branch: "main", Date: now.Add(-30 * 24 * time.Hour)},
		{Index: 2, Message: "recent feature", Branch: "feat/x", Date: now},
		{Index: 3, Message: "old feature", Branch: "feat/x", Date: now.Add(-60 * 24 * time.Hour)},
	}

	// Mark stale.
	threshold := 14 * 24 * time.Hour
	stashes = stale.MarkStaleWithTime(stashes, now, threshold)

	// Active branch filter: main.
	fp := filter.New()
	fpCtx := plugin.PluginContext{Logger: slog.Default()}
	if err := fp.Init(fpCtx); err != nil {
		t.Fatalf("init filter: %v", err)
	}

	state := plugin.AppState{
		Mode:    plugin.ModeList,
		Stashes: stashes,
		Cursor:  0,
		Branch:  "main",
	}

	// Toggle both filters.
	state, _ = fp.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	state, _ = fp.HandleKey(plugin.KeyEvent{Key: "F"}, state)

	if len(state.Filters) != 2 {
		t.Errorf("expected 2 filters, got %d", len(state.Filters))
	}

	// Count stale on main branch only.
	var staleMainCount int
	for _, s := range stashes {
		if s.Branch == "main" && s.IsStale {
			staleMainCount++
		}
	}
	if staleMainCount != 1 {
		t.Errorf("stale main stashes = %d, want 1", staleMainCount)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func makePluginStashes(n int) []plugin.Stash {
	stashes := make([]plugin.Stash, n)
	for i := range n {
		stashes[i] = plugin.Stash{
			Index:   i,
			SHA:     fmt.Sprintf("abc%04d", i),
			Message: fmt.Sprintf("stash %d", i),
			Branch:  "main",
			Date:    time.Now(),
		}
	}
	return stashes
}
