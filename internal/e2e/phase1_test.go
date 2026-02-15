package e2e

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/ui/screens"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── LIST screen E2E ────────────────────────────────────────

func TestE2E_ListScreenRendersAllStashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 5)
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 5 {
		t.Fatalf("expected 5 stashes, got %d", len(stashLines))
	}

	// Build stash objects (LIFO: most recent stash has highest index number).
	stashes := make([]core.Stash, 5)
	for i := range 5 {
		stashes[i] = core.Stash{
			Index:   i,
			Message: fmt.Sprintf("stash message %d", 4-i),
			SHA:     fmt.Sprintf("abc%d", i),
		}
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	view := ls.View(state, 120, 30)

	// Verify all 5 stash messages appear.
	for _, s := range stashes {
		assertScreenContains(t, view, s.Message)
	}
}

func TestE2E_CursorNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	setupTestRepo(t, 10) // validates the repo setup

	stashes := make([]core.Stash, 10)
	for i := range 10 {
		stashes[i] = core.Stash{
			Index:   i,
			Message: fmt.Sprintf("stash message %d", 9-i),
		}
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Resize to fit all stashes.
	sizeMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	state, _ = ls.Update(sizeMsg, state)

	// j moves down one at a time.
	for range 5 {
		msg := tea.KeyPressMsg{Text: "j"}
		state, _ = ls.Update(msg, state)
	}
	if ls.Cursor() != 5 {
		t.Errorf("after 5 j presses: cursor = %d, want 5", ls.Cursor())
	}

	// G jumps to last.
	msg := tea.KeyPressMsg{Text: "G"}
	state, _ = ls.Update(msg, state)
	if ls.Cursor() != 9 {
		t.Errorf("after G: cursor = %d, want 9", ls.Cursor())
	}

	// g jumps to first.
	msg = tea.KeyPressMsg{Text: "g"}
	state, _ = ls.Update(msg, state)
	if ls.Cursor() != 0 {
		t.Errorf("after g: cursor = %d, want 0", ls.Cursor())
	}

	// k at top stays at 0.
	msg = tea.KeyPressMsg{Text: "k"}
	_, _ = ls.Update(msg, state)
	if ls.Cursor() != 0 {
		t.Errorf("k at top: cursor = %d, want 0", ls.Cursor())
	}
}

func TestE2E_EmptyRepoEmptyState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 0)
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 0 {
		t.Fatalf("expected 0 stashes, got %d", len(stashLines))
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Mode: core.ModeList,
	}

	view := ls.View(state, 120, 30)
	assertScreenContains(t, view, "No stashes found")
	assertScreenNotContains(t, view, "stash message")
}

// ─── Mode transitions E2E ───────────────────────────────────

func TestE2E_TabToPreviewMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	stashes := makeStashes(3)
	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	state, _ = ls.Update(msg, state)

	if state.Mode != core.ModePreview {
		t.Errorf("mode = %v, want ModePreview after Tab", state.Mode)
	}
}

func TestE2E_EnterToDetailMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	stashes := makeStashes(3)
	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	state, _ = ls.Update(msg, state)

	if state.Mode != core.ModeDetail {
		t.Errorf("mode = %v, want ModeDetail after Enter", state.Mode)
	}
}

func TestE2E_EscReturnsFromDetail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	// Esc is handled by core/app.go (popMode), not by DetailScreen.
	// DetailScreen should NOT change mode on Esc — it passes through.
	th := theme.NewAgni()
	ds := screens.NewDetailScreen(th)
	state := core.AppState{Mode: core.ModeDetail}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	state, _ = ds.Update(msg, state)

	if state.Mode != core.ModeDetail {
		t.Errorf("Esc should not change mode at screen level: mode = %v, want ModeDetail", state.Mode)
	}
}

func TestE2E_FullModeTransitionCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	stashes := makeStashes(3)
	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// LIST → Tab → PREVIEW.
	state, _ = ls.Update(tea.KeyPressMsg{Code: tea.KeyTab}, state)
	if state.Mode != core.ModePreview {
		t.Fatalf("Tab: mode = %v, want ModePreview", state.Mode)
	}

	// Reset to LIST for next transition.
	state.Mode = core.ModeList

	// LIST → Enter → DETAIL.
	state, _ = ls.Update(tea.KeyPressMsg{Code: tea.KeyEnter}, state)
	if state.Mode != core.ModeDetail {
		t.Fatalf("Enter: mode = %v, want ModeDetail", state.Mode)
	}

	// DETAIL → Esc: DetailScreen passes through (core handles Esc via popMode).
	ds := screens.NewDetailScreen(th)
	state, _ = ds.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, state)
	if state.Mode != core.ModeDetail {
		t.Fatalf("Esc should not change mode at screen level: mode = %v, want ModeDetail", state.Mode)
	}
}

// ─── CRUD operations E2E ────────────────────────────────────

func TestE2E_ApplyStash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 1)
	ctx := context.Background()

	runner := git.NewDefaultRunner(dir, nil)
	cache := &noopCache{}
	ops := git.NewStashOps(runner, cache)

	result, err := ops.Apply(ctx, 0)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("Apply should succeed: %s", result.Error)
	}

	// Stash should still be in the list (apply preserves).
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 1 {
		t.Errorf("stash count = %d, want 1 (apply preserves stash)", len(stashLines))
	}

	// Stashed file should now be in the working tree.
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if len(out) == 0 {
		t.Error("working tree should have changes after apply")
	}
}

func TestE2E_DropStashWithUndo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 3)
	ctx := context.Background()

	runner := git.NewDefaultRunner(dir, nil)
	cache := &noopCache{}
	ops := git.NewStashOps(runner, cache)

	// Drop the top stash.
	result, err := ops.Drop(ctx, 0)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
	if result.SHA == "" {
		t.Error("Drop should return SHA for undo")
	}

	// Verify stash count decreased.
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 2 {
		t.Errorf("stash count = %d, want 2 after drop", len(stashLines))
	}

	// Undo: restore the dropped stash.
	err = ops.RestoreStash(ctx, result.SHA, "restored via undo")
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}

	stashLines = gitStashList(t, dir)
	if len(stashLines) != 3 {
		t.Errorf("stash count = %d, want 3 after restore", len(stashLines))
	}
}

func TestE2E_CRUDSequence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupTestRepo(t, 3)
	ctx := context.Background()
	runner := git.NewDefaultRunner(dir, nil)
	cache := &noopCache{}
	ops := git.NewStashOps(runner, cache)

	// Verify starting state: 3 stashes.
	if n := len(gitStashList(t, dir)); n != 3 {
		t.Fatalf("initial: expected 3 stashes, got %d", n)
	}

	// Apply stash@{0} — preserves stash.
	applyResult, err := ops.Apply(ctx, 0)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if !applyResult.Success {
		t.Fatalf("Apply should succeed: %s", applyResult.Error)
	}
	if n := len(gitStashList(t, dir)); n != 3 {
		t.Fatalf("after apply: expected 3 stashes, got %d", n)
	}

	// Clean working tree for next operation.
	cleanCmd := exec.Command("git", "checkout", ".")
	cleanCmd.Dir = dir
	_ = cleanCmd.Run()

	// Drop stash@{1}.
	dropResult, err := ops.Drop(ctx, 1)
	if err != nil {
		t.Fatalf("Drop failed: %v", err)
	}
	if n := len(gitStashList(t, dir)); n != 2 {
		t.Fatalf("after drop: expected 2 stashes, got %d", n)
	}

	// Undo the drop.
	err = ops.RestoreStash(ctx, dropResult.SHA, dropResult.Message)
	if err != nil {
		t.Fatalf("RestoreStash failed: %v", err)
	}
	if n := len(gitStashList(t, dir)); n != 3 {
		t.Fatalf("after undo: expected 3 stashes, got %d", n)
	}

	// Pop stash@{0}.
	_, err = ops.Pop(ctx, 0)
	if err != nil {
		t.Fatalf("Pop failed: %v", err)
	}
	if n := len(gitStashList(t, dir)); n != 2 {
		t.Fatalf("after pop: expected 2 stashes, got %d", n)
	}
}

// ─── Detail + Preview with real diffs ───────────────────────

func TestE2E_DetailWithRealDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupMultiFileStash(t, 4)
	diff := gitStashDiff(t, dir, 0)

	if diff == "" {
		t.Fatal("expected non-empty diff for multi-file stash")
	}

	th := theme.NewAgni()
	ds := screens.NewDetailScreen(th)
	ds.SetDiff(diff)

	state := core.AppState{Mode: core.ModeDetail}
	view := ds.View(state, 120, 30)

	if view == "" {
		t.Error("DETAIL view should not be empty")
	}

	// View should contain filenames from our test setup.
	for i := range 4 {
		assertScreenContains(t, view, fmt.Sprintf("file_%d.go", i))
	}
}

func TestE2E_DetailFocusSwitchingWithRealDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	dir := setupMultiFileStash(t, 3)
	diff := gitStashDiff(t, dir, 0)

	th := theme.NewAgni()
	ds := screens.NewDetailScreen(th)
	ds.SetDiff(diff)
	state := core.AppState{Mode: core.ModeDetail}

	// Initial focus: tree pane.
	if ds.Focused() != screens.PaneTree {
		t.Errorf("initial focus = %v, want PaneTree", ds.Focused())
	}

	// Tab switches to diff pane.
	state, _ = ds.Update(tea.KeyPressMsg{Code: tea.KeyTab}, state)
	if ds.Focused() != screens.PaneDiff {
		t.Errorf("after Tab: focus = %v, want PaneDiff", ds.Focused())
	}

	// Tab again switches back to tree.
	_, _ = ds.Update(tea.KeyPressMsg{Code: tea.KeyTab}, state)
	if ds.Focused() != screens.PaneTree {
		t.Errorf("after second Tab: focus = %v, want PaneTree", ds.Focused())
	}
}

// ─── Performance ────────────────────────────────────────────

func TestE2E_StartupTimeUnder100ms(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	dir := setupTestRepo(t, 20)
	ctx := context.Background()

	// Warm up git.
	warmUp := exec.CommandContext(ctx, "git", "stash", "list")
	warmUp.Dir = dir
	_ = warmUp.Run()

	const iterations = 5
	var totalDuration time.Duration

	for range iterations {
		start := time.Now()

		cmd := exec.CommandContext(ctx, "git", "stash", "list",
			"--format=%H %h %s %D %ai")
		cmd.Dir = dir
		cmd.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "HOME=" + dir}
		_, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}

		totalDuration += time.Since(start)
	}

	avg := totalDuration / iterations
	t.Logf("Average stash list time (20 stashes): %v", avg)

	if avg > 100*time.Millisecond {
		t.Errorf("stash list too slow: %v (budget: <100ms)", avg)
	}
}

func TestE2E_StartupTime100Stashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	dir := setupTestRepo(t, 100)
	ctx := context.Background()

	start := time.Now()

	cmd := exec.CommandContext(ctx, "git", "stash", "list",
		"--format=%H %h %s %D %ai")
	cmd.Dir = dir
	cmd.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "HOME=" + dir}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)
	t.Logf("Stash list time (100 stashes): %v, output size: %d bytes", elapsed, len(out))

	if elapsed > 300*time.Millisecond {
		t.Errorf("stash list too slow for 100 stashes: %v (budget: <300ms)", elapsed)
	}
}

// ─── Helpers ────────────────────────────────────────────────

func makeStashes(n int) []core.Stash {
	stashes := make([]core.Stash, n)
	for i := range n {
		stashes[i] = core.Stash{
			Index:   i,
			SHA:     fmt.Sprintf("abc%04d", i),
			Message: fmt.Sprintf("stash %d", i),
		}
	}
	return stashes
}
