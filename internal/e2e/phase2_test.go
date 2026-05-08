package e2e

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/rename"
	"github.com/indrasvat/nidhi/internal/plugins/undo"
)

// ─── Plugin test doubles ────────────────────────────────────

type pluginNoopCache struct{}

func (c *pluginNoopCache) List(_ context.Context) ([]plugin.Stash, error) { return nil, nil }
func (c *pluginNoopCache) Diff(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *pluginNoopCache) Invalidate() {}

var _ plugin.StashCache = (*pluginNoopCache)(nil)

// newRenamePlugin creates an initialized rename plugin for testing.
func newRenamePlugin(t *testing.T, dir string) *rename.Plugin {
	t.Helper()
	runner := git.NewDefaultRunner(dir, nil)
	p := rename.New()
	pctx := plugin.PluginContext{
		Git:    runner,
		Cache:  &pluginNoopCache{},
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init rename plugin: %v", err)
	}
	return p
}

// buildPluginStashes builds a []plugin.Stash slice from the repo state.
func buildPluginStashes(t *testing.T, dir string) []plugin.Stash {
	t.Helper()
	shas := stashSHAs(t, dir)
	msgs := stashMessages(t, dir)
	if len(shas) != len(msgs) {
		t.Fatalf("SHA count %d != message count %d", len(shas), len(msgs))
	}
	stashes := make([]plugin.Stash, len(shas))
	for i := range shas {
		stashes[i] = plugin.Stash{
			Index:      i,
			SHA:        shas[i],
			Message:    msgs[i],
			RawMessage: msgs[i],
		}
	}
	return stashes
}

// =============================================================================
// Conflict Preview Flow (FR-10)
// =============================================================================

func TestE2E_ConflictFlow_ConflictsDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := setupTestRepo(t, 0)

	// Create a file, commit, stash a conflicting change, then change HEAD.
	writeFile(t, dir, "config.go", "package main\n\nvar retries = 3\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add config")

	// Stash: change retries to 10.
	writeFile(t, dir, "config.go", "package main\n\nvar retries = 10\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "bump retries")

	// HEAD: change retries to 5 (conflict!).
	writeFile(t, dir, "config.go", "package main\n\nvar retries = 5\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "set retries to 5")

	stashCommit := stashSHA(t, dir, 0)
	runner := git.NewDefaultRunner(dir, nil)

	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	if !result.HasConflicts {
		t.Fatal("expected conflicts, got clean merge")
	}

	var conflicted bool
	for _, f := range result.Files {
		if f.Path == "config.go" && f.Status == git.FileStatusConflicted {
			conflicted = true
		}
	}
	if !conflicted {
		t.Error("expected config.go to be marked as conflicted")
	}
}

func TestE2E_ConflictFlow_CleanApply(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := setupTestRepo(t, 0)

	// Stash changes to utils.go, HEAD modifies main.go — no conflict.
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add main")

	writeFile(t, dir, "utils.go", "package main\n\nfunc helper() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "add utils")

	writeFile(t, dir, "main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "update main")

	stashCommit := stashSHA(t, dir, 0)
	runner := git.NewDefaultRunner(dir, nil)

	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	if result.HasConflicts {
		t.Error("expected clean merge, got conflicts")
	}

	// Apply should succeed cleanly.
	gitCmd(t, dir, "stash", "apply", "stash@{0}")

	if !fileExists(dir, "utils.go") {
		t.Error("utils.go should exist after clean apply")
	}
}

// =============================================================================
// Undo Flow (FR-14)
// =============================================================================

func TestE2E_UndoFlow_DropAndRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	// Create 3 stashes: alpha, beta, gamma.
	for i, msg := range []string{"alpha", "beta", "gamma"} {
		writeFile(t, dir, "file.go", fmt.Sprintf("package main // v%d %s\n", i, msg))
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}
	// Order: gamma(0), beta(1), alpha(2)

	if got := stashCount(t, dir); got != 3 {
		t.Fatalf("initial stash count = %d, want 3", got)
	}

	// Drop stash@{0} (gamma).
	sha := stashSHA(t, dir, 0)
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	if got := stashCount(t, dir); got != 2 {
		t.Fatalf("stash count after drop = %d, want 2", got)
	}

	// Undo: restore gamma via git stash store.
	runner := git.NewDefaultRunner(dir, nil)
	_, err := runner.Run(context.Background(), "stash", "store", "-m", "gamma", sha)
	if err != nil {
		t.Fatalf("stash store: %v", err)
	}

	if got := stashCount(t, dir); got != 3 {
		t.Fatalf("stash count after undo = %d, want 3", got)
	}

	// Verify: gamma is back at position 0.
	msgs := stashMessages(t, dir)
	if !strings.Contains(msgs[0], "gamma") {
		t.Errorf("stash@{0} = %q, want gamma", msgs[0])
	}
}

func TestE2E_UndoFlow_RingBufferLIFO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	// Create 5 stashes.
	for i := range 5 {
		msg := fmt.Sprintf("stash-%d", i)
		writeFile(t, dir, "file.go", fmt.Sprintf("package main // %s\n", msg))
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}

	type dropped struct {
		sha string
		msg string
	}
	var drops []dropped

	// Drop top 3 stashes, recording SHAs.
	for range 3 {
		sha := stashSHA(t, dir, 0)
		msgs := stashMessages(t, dir)
		drops = append(drops, dropped{sha: sha, msg: msgs[0]})
		gitCmd(t, dir, "stash", "drop", "stash@{0}")
	}

	if got := stashCount(t, dir); got != 2 {
		t.Fatalf("stash count after 3 drops = %d, want 2", got)
	}

	// Undo in LIFO order (most recently dropped first).
	runner := git.NewDefaultRunner(dir, nil)
	for i := len(drops) - 1; i >= 0; i-- {
		_, err := runner.Run(context.Background(), "stash", "store", "-m", drops[i].msg, drops[i].sha)
		if err != nil {
			t.Fatalf("undo %d: %v", i, err)
		}
	}

	if got := stashCount(t, dir); got != 5 {
		t.Fatalf("stash count after undo all = %d, want 5", got)
	}
}

func TestE2E_UndoFlow_CrossSessionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	// Create and drop a stash.
	writeFile(t, dir, "lost.go", "package main\n\nfunc lost() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "lost work")

	sha := stashSHA(t, dir, 0)
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	// Simulate "restart app" — no in-memory undo buffer.
	// Use git fsck to find the orphaned commit.
	runner := git.NewDefaultRunner(dir, nil)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}

	// Find our dropped stash in the candidates.
	var found *undo.RecoveryCandidate
	for _, c := range candidates {
		if c.SHA == sha {
			found = &c
			break
		}
	}
	if found == nil {
		t.Fatalf("dropped stash %s not found via fsck (found %d candidates)", sha[:8], len(candidates))
	}

	// Restore it.
	if err := undo.RestoreCandidate(context.Background(), runner, *found); err != nil {
		t.Fatalf("RestoreCandidate: %v", err)
	}

	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count after recovery = %d, want 1", got)
	}
}

// =============================================================================
// Rename Flow (FR-13)
// =============================================================================

func TestE2E_RenameFlow_MiddleStash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dir := setupTestRepo(t, 0)

	// Create 3 stashes: alpha, beta, gamma.
	for _, msg := range []string{"alpha", "beta", "gamma"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}
	// Order: gamma(0), beta(1), alpha(2)

	originalSHAs := stashSHAs(t, dir)
	stashes := buildPluginStashes(t, dir)

	// Rename stash@{1} (beta -> "bravo").
	p := newRenamePlugin(t, dir)
	if err := p.RenameStash(context.Background(), stashes, 1, "bravo"); err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	// Verify: 3 stashes, correct order.
	if got := stashCount(t, dir); got != 3 {
		t.Fatalf("stash count = %d, want 3", got)
	}

	newMsgs := stashMessages(t, dir)
	expected := []string{"gamma", "bravo", "alpha"}
	for i, want := range expected {
		if !strings.Contains(newMsgs[i], want) {
			t.Errorf("stash@{%d} = %q, want to contain %q", i, newMsgs[i], want)
		}
	}

	// Verify: SHAs preserved.
	newSHAs := stashSHAs(t, dir)
	for i := range originalSHAs {
		if newSHAs[i] != originalSHAs[i] {
			t.Errorf("stash@{%d} SHA changed: %s -> %s", i, originalSHAs[i][:8], newSHAs[i][:8])
		}
	}
}

// =============================================================================
// New Stash Flow (FR-02.4)
// =============================================================================

func TestE2E_NewStashFlow_CreateWithMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	writeFile(t, dir, "feature.go", "package main\n\nfunc feature() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "my feature work")

	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	msgs := stashMessages(t, dir)
	if !strings.Contains(msgs[0], "my feature work") {
		t.Errorf("stash message = %q, want 'my feature work'", msgs[0])
	}
}

func TestE2E_NewStashFlow_ScopeToggle_StagedOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 35) // --staged requires git >= 2.35

	dir := setupTestRepo(t, 0)

	// Create staged and unstaged changes.
	writeFile(t, dir, "staged.go", "package main\n")
	gitCmd(t, dir, "add", "staged.go")
	writeFile(t, dir, "README.md", "# modified\n") // Unstaged modification

	// Stash only staged changes.
	gitCmd(t, dir, "stash", "push", "-m", "staged only", "--staged")

	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	// Unstaged changes should remain.
	status := gitCmd(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "README.md") {
		t.Errorf("unstaged README.md should remain, status: %q", status)
	}
}

func TestE2E_NewStashFlow_IncludeUntracked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	writeFile(t, dir, "untracked.txt", "hello\n")
	writeFile(t, dir, "tracked.go", "package main\n")
	gitCmd(t, dir, "add", "tracked.go")
	gitCmd(t, dir, "stash", "push", "-m", "with untracked", "--include-untracked")

	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}

	if fileExists(dir, "untracked.txt") {
		t.Error("untracked.txt should be stashed (removed from working tree)")
	}
}

// =============================================================================
// Cross-Feature: Conflict + Undo
// =============================================================================

func TestE2E_ConflictThenUndo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := setupTestRepo(t, 0)

	// Create a conflict scenario.
	writeFile(t, dir, "config.go", "package main\n\nvar x = 1\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add config")

	writeFile(t, dir, "config.go", "package main\n\nvar x = 100\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "change x to 100")

	writeFile(t, dir, "config.go", "package main\n\nvar x = 50\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "change x to 50")

	stashCommit := stashSHA(t, dir, 0)
	runner := git.NewDefaultRunner(dir, nil)

	// Verify conflict exists.
	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}
	if !result.HasConflicts {
		t.Fatal("expected conflicts")
	}

	// User decides to drop instead of applying.
	sha := stashSHA(t, dir, 0)
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	// Undo the drop.
	_, err = runner.Run(context.Background(), "stash", "store", "-m", "change x to 100", sha)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	// Stash is back.
	if got := stashCount(t, dir); got != 1 {
		t.Fatalf("stash count = %d, want 1", got)
	}
}

// =============================================================================
// Cross-Feature: Rename + Undo
// =============================================================================

func TestE2E_RenameThenDropThenUndo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dir := setupTestRepo(t, 0)

	for _, msg := range []string{"first", "second"} {
		writeFile(t, dir, "file.go", "package main // "+msg+"\n")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}
	// Order: second(0), first(1)

	stashes := buildPluginStashes(t, dir)

	// Rename stash@{0} ("second" -> "renamed").
	p := newRenamePlugin(t, dir)
	if err := p.RenameStash(context.Background(), stashes, 0, "renamed"); err != nil {
		t.Fatalf("RenameStash: %v", err)
	}

	// Now drop the renamed stash.
	sha := stashSHA(t, dir, 0)
	gitCmd(t, dir, "stash", "drop", "stash@{0}")

	// Undo the drop.
	runner := git.NewDefaultRunner(dir, nil)
	_, err := runner.Run(context.Background(), "stash", "store", "-m", "renamed", sha)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}

	// Verify: stash restored with the RENAMED message, not the original.
	msgs := stashMessages(t, dir)
	if !strings.Contains(msgs[0], "renamed") {
		t.Errorf("expected restored stash to have renamed message, got: %q", msgs[0])
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestE2E_EmptyRepo_NoStashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	if got := stashCount(t, dir); got != 0 {
		t.Fatalf("expected 0 stashes, got %d", got)
	}

	// Recovery should find nothing.
	runner := git.NewDefaultRunner(dir, nil)
	candidates, err := undo.FindDroppedStashes(context.Background(), runner)
	if err != nil {
		t.Fatalf("FindDroppedStashes: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected 0 recovery candidates, got %d", len(candidates))
	}
}

func TestE2E_StashWithUntrackedCollision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := setupTestRepo(t, 0)

	// Stash an untracked file alongside a tracked change.
	writeFile(t, dir, "collision.txt", "from stash\n")
	writeFile(t, dir, "tracked.go", "package main\n")
	gitCmd(t, dir, "add", "tracked.go")
	gitCmd(t, dir, "stash", "push", "--include-untracked", "-m", "with untracked")

	// Create the same file in the working tree (tracked).
	writeFile(t, dir, "collision.txt", "already here\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add collision.txt")

	stashCommit := stashSHA(t, dir, 0)
	runner := git.NewDefaultRunner(dir, nil)

	// Check for untracked collisions.
	collisions, err := git.CheckUntrackedCollisions(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("CheckUntrackedCollisions: %v", err)
	}

	found := false
	for _, c := range collisions {
		if c.Path == "collision.txt" {
			found = true
		}
	}
	if !found {
		t.Error("expected collision.txt in untracked collisions")
	}
}
