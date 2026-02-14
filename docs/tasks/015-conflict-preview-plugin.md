# Task 015: Conflict Preview Plugin

## Status: TODO

## Depends On
- 013 (Stash CRUD operations)
- 006 (Core model — Stash struct, AppState, Mode)

## Parallelizable With
- 016 (Undo plugin)
- 017 (Rename plugin)
- 018 (New stash screen)

## Problem
When a user presses `a` (apply) or `p` (pop), git silently attempts the merge. If there are conflicts, the working tree ends up in a partially merged state with conflict markers. The user has no way to preview what will happen before committing to the operation. This causes lost work, broken working trees, and manual cleanup.

nidhi must intercept apply/pop operations, dry-run them via `git merge-tree --write-tree`, parse the output, and present a conflict preview screen before proceeding. If the stash contains untracked files that collide with existing working tree files, this must also be detected and shown as a separate warning (since `merge-tree` does not cover this case).

## PRD Reference
- Section 6.2, FR-10 (Conflict Preview plugin) -- all sub-requirements FR-10.1 through FR-10.7
- Section 8.2 (Core Interfaces) -- `StashHook`, `ScreenProvider` interfaces
- Section 10, Screen 4 (Conflict Preview layout and behavior)
- Section 4.1 (Git >= 2.38 for `merge-tree --write-tree`)
- Section 9.1 (Agni theme colors for conflict indicators)
- Section 9.2 (Icons: conflict, clean)

## Files to Create
- `internal/plugins/conflict/conflict.go` -- plugin struct implementing `StashHook` + `ScreenProvider`, `BeforeApply` logic
- `internal/plugins/conflict/screen.go` -- conflict preview screen rendering and key handling
- `internal/plugins/conflict/conflict_test.go` -- integration + unit tests for merge-tree conflict detection
- `internal/plugins/conflict/screen_test.go` -- unit tests for screen rendering
- `internal/git/mergetree.go` -- `MergeTreeResult` struct, `RunMergeTree` function, output parser
- `internal/git/mergetree_test.go` -- unit + integration tests for merge-tree parsing

## Files to Modify
- `internal/plugin/interfaces.go` -- ensure `StashHook` and `ScreenProvider` interfaces exist (should already exist from Phase 1)
- `internal/plugin/loader.go` -- register conflict plugin
- `internal/core/events.go` -- add `SwitchToConflictScreenMsg` message type

## Execution Steps

### Step 1: Define merge-tree result types in `internal/git/mergetree.go`

```go
package git

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

// FileConflictStatus represents the merge status of a single file.
type FileConflictStatus int

const (
	FileStatusClean      FileConflictStatus = iota // No conflicts, clean merge
	FileStatusConflicted                           // Merge conflict detected
	FileStatusUnknown                              // Could not determine status
)

// MergeTreeFile represents a single file in the merge-tree result.
type MergeTreeFile struct {
	Path           string
	Status         FileConflictStatus
	ConflictZones  []ConflictZone // Non-empty only if Status == FileStatusConflicted
}

// ConflictZone represents a single conflict hunk in a file.
type ConflictZone struct {
	OurContent   string // Content from HEAD (ours)
	TheirContent string // Content from stash (theirs)
	BaseContent  string // Content from merge base
}

// UntrackedCollision represents an untracked file in the stash that already
// exists in the working tree (FR-10.6a).
type UntrackedCollision struct {
	Path string
}

// MergeTreeResult holds the parsed output of `git merge-tree --write-tree`.
type MergeTreeResult struct {
	HasConflicts         bool
	TreeSHA              string                // The resulting tree SHA (first line of output)
	Files                []MergeTreeFile
	UntrackedCollisions  []UntrackedCollision  // Not from merge-tree; populated separately
	Informational        []string              // Informational messages from merge-tree
}

// RunMergeTree executes `git merge-tree --write-tree HEAD <stashCommit>` and
// parses the result. Returns the parsed result and any execution error.
//
// Exit code 0 = no conflicts (clean merge).
// Exit code 1 = conflicts detected.
// Any other exit code or error is propagated.
func RunMergeTree(ctx context.Context, runner GitRunner, stashCommit string) (MergeTreeResult, error) {
	stdout, exitCode, err := runner.RunExitCode(ctx, "merge-tree", "--write-tree", "HEAD", stashCommit)
	if err != nil {
		return MergeTreeResult{}, fmt.Errorf("merge-tree: %w", err)
	}

	result := ParseMergeTreeOutput(stdout)

	switch exitCode {
	case 0:
		result.HasConflicts = false
	case 1:
		result.HasConflicts = true
	default:
		return result, fmt.Errorf("merge-tree: unexpected exit code %d", exitCode)
	}

	return result, nil
}

// ParseMergeTreeOutput parses the stdout of `git merge-tree --write-tree`.
//
// Format (clean):
//
//	<tree-sha>
//
// Format (conflicts):
//
//	<tree-sha>
//	<empty line>
//	<Informational messages>
//	<empty line>
//	<Conflict messages>
//
// Conflict lines look like:
//
//	CONFLICT (content): Merge conflict in <path>
//	Auto-merging <path>
func ParseMergeTreeOutput(output string) MergeTreeResult {
	result := MergeTreeResult{}
	scanner := bufio.NewScanner(strings.NewReader(output))

	// First line is always the tree SHA.
	if scanner.Scan() {
		result.TreeSHA = strings.TrimSpace(scanner.Text())
	}

	seenFiles := make(map[string]*MergeTreeFile)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "CONFLICT"):
			// CONFLICT (content): Merge conflict in <path>
			path := parseConflictPath(line)
			if path != "" {
				f := getOrCreateFile(seenFiles, path)
				f.Status = FileStatusConflicted
				result.HasConflicts = true
			}
			result.Informational = append(result.Informational, line)

		case strings.HasPrefix(line, "Auto-merging"):
			// Auto-merging <path>
			path := strings.TrimPrefix(line, "Auto-merging ")
			path = strings.TrimSpace(path)
			if path != "" {
				f := getOrCreateFile(seenFiles, path)
				if f.Status != FileStatusConflicted {
					f.Status = FileStatusClean
				}
			}
			result.Informational = append(result.Informational, line)

		default:
			result.Informational = append(result.Informational, line)
		}
	}

	// Collect files in stable order.
	for _, f := range seenFiles {
		result.Files = append(result.Files, *f)
	}

	return result
}

func parseConflictPath(line string) string {
	// Format: "CONFLICT (content): Merge conflict in <path>"
	idx := strings.Index(line, "Merge conflict in ")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[idx+len("Merge conflict in "):])
}

func getOrCreateFile(m map[string]*MergeTreeFile, path string) *MergeTreeFile {
	if f, ok := m[path]; ok {
		return f
	}
	f := &MergeTreeFile{Path: path, Status: FileStatusUnknown}
	m[path] = f
	return f
}

// CheckUntrackedCollisions compares the stash's untracked files against the
// working tree. Returns paths that exist in both the stash (as untracked)
// and the working tree (FR-10.6a).
func CheckUntrackedCollisions(ctx context.Context, runner GitRunner, stashCommit string) ([]UntrackedCollision, error) {
	// Get the list of untracked files in the stash.
	// Stash commits have up to 3 parents: index, working tree, and untracked (3rd parent).
	// List the untracked tree: git diff-tree --no-commit-id --name-only -r <stash>^3
	untrackedFiles, exitCode, err := runner.RunExitCode(ctx, "diff-tree", "--no-commit-id", "--name-only", "-r", stashCommit+"^3")
	if err != nil || exitCode != 0 {
		// No untracked parent (stash was created without -u/--include-untracked).
		return nil, nil
	}

	// Get working tree files via ls-files.
	existingFiles, err := runner.Run(ctx, "ls-files")
	if err != nil {
		return nil, fmt.Errorf("ls-files: %w", err)
	}

	existing := make(map[string]struct{})
	for _, f := range strings.Split(strings.TrimSpace(existingFiles), "\n") {
		if f != "" {
			existing[f] = struct{}{}
		}
	}

	var collisions []UntrackedCollision
	for _, f := range strings.Split(strings.TrimSpace(untrackedFiles), "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := existing[f]; ok {
			collisions = append(collisions, UntrackedCollision{Path: f})
		}
	}

	return collisions, nil
}
```

### Step 2: Create `internal/git/mergetree_test.go`

```go
package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

// run executes a command in the given directory and fails the test on error.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\noutput: %s", name, args, err, out)
	}
	return string(out)
}

// writeFile creates or overwrites a file in dir with the given content.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupTestRepo creates a git repo with an initial commit.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.name", "test")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	writeFile(t, dir, "README.md", "# test repo\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "initial commit")
	return dir
}

func TestParseMergeTreeOutput(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		wantHasConflict bool
		wantTreeSHA    string
		wantFiles      int
	}{
		{
			name:           "clean merge - tree SHA only",
			output:         "abc123def456\n",
			wantHasConflict: false,
			wantTreeSHA:    "abc123def456",
			wantFiles:      0,
		},
		{
			name: "single conflict",
			output: `abc123def456

Auto-merging src/auth/config.go
CONFLICT (content): Merge conflict in src/auth/config.go
`,
			wantHasConflict: true,
			wantTreeSHA:    "abc123def456",
			wantFiles:      1,
		},
		{
			name: "mixed clean and conflict",
			output: `abc123def456

Auto-merging src/auth/token.go
Auto-merging src/auth/config.go
CONFLICT (content): Merge conflict in src/auth/config.go
`,
			wantHasConflict: true,
			wantTreeSHA:    "abc123def456",
			wantFiles:      2,
		},
		{
			name: "multiple conflicts",
			output: `abc123def456

Auto-merging file1.go
CONFLICT (content): Merge conflict in file1.go
Auto-merging file2.go
CONFLICT (content): Merge conflict in file2.go
Auto-merging file3.go
`,
			wantHasConflict: true,
			wantTreeSHA:    "abc123def456",
			wantFiles:      3,
		},
		{
			name:           "empty output",
			output:         "",
			wantHasConflict: false,
			wantTreeSHA:    "",
			wantFiles:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := git.ParseMergeTreeOutput(tt.output)

			if result.HasConflicts != tt.wantHasConflict {
				t.Errorf("HasConflicts = %v, want %v", result.HasConflicts, tt.wantHasConflict)
			}
			if result.TreeSHA != tt.wantTreeSHA {
				t.Errorf("TreeSHA = %q, want %q", result.TreeSHA, tt.wantTreeSHA)
			}
			if len(result.Files) != tt.wantFiles {
				t.Errorf("len(Files) = %d, want %d", len(result.Files), tt.wantFiles)
			}
		})
	}
}

func TestParseMergeTreeOutput_FileStatuses(t *testing.T) {
	output := `deadbeef1234

Auto-merging clean.go
Auto-merging conflicted.go
CONFLICT (content): Merge conflict in conflicted.go
`
	result := git.ParseMergeTreeOutput(output)

	statusByPath := make(map[string]git.FileConflictStatus)
	for _, f := range result.Files {
		statusByPath[f.Path] = f.Status
	}

	if s, ok := statusByPath["clean.go"]; !ok || s != git.FileStatusClean {
		t.Errorf("clean.go status = %v, want FileStatusClean", s)
	}
	if s, ok := statusByPath["conflicted.go"]; !ok || s != git.FileStatusConflicted {
		t.Errorf("conflicted.go status = %v, want FileStatusConflicted", s)
	}
}

func TestRunMergeTree_Integration_ConflictDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := setupTestRepo(t)

	// Create a file and commit.
	writeFile(t, dir, "config.go", "package main\n\nvar maxRetries = 3\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add config")

	// Modify the same line and stash.
	writeFile(t, dir, "config.go", "package main\n\nvar maxRetries = 10\n")
	run(t, dir, "git", "stash", "push", "-m", "bump retries to 10")

	// Modify the same line differently on HEAD.
	writeFile(t, dir, "config.go", "package main\n\nvar maxRetries = 5\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "set retries to 5")

	// Get stash commit SHA.
	stashSHA := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))

	// Run merge-tree.
	runner := git.NewRunner(dir)
	result, err := git.RunMergeTree(context.Background(), runner, stashSHA)
	if err != nil {
		t.Fatalf("RunMergeTree failed: %v", err)
	}

	if !result.HasConflicts {
		t.Error("expected HasConflicts=true, got false")
	}

	// Should have at least one conflicted file.
	var foundConflicted bool
	for _, f := range result.Files {
		if f.Path == "config.go" && f.Status == git.FileStatusConflicted {
			foundConflicted = true
		}
	}
	if !foundConflicted {
		t.Error("expected config.go to be marked as conflicted")
	}
}

func TestRunMergeTree_Integration_CleanApply(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := setupTestRepo(t)

	// Create a file and commit.
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add main")

	// Modify a different file and stash.
	writeFile(t, dir, "utils.go", "package main\n\nfunc helper() {}\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "stash", "push", "-m", "add utils")

	// HEAD has not changed the same files -- clean apply.
	stashSHA := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewRunner(dir)
	result, err := git.RunMergeTree(context.Background(), runner, stashSHA)
	if err != nil {
		t.Fatalf("RunMergeTree failed: %v", err)
	}

	if result.HasConflicts {
		t.Error("expected HasConflicts=false for clean apply, got true")
	}
}

func TestCheckUntrackedCollisions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := setupTestRepo(t)

	// Create an untracked file and stash it with --include-untracked.
	writeFile(t, dir, "newfile.txt", "stashed content\n")
	run(t, dir, "git", "stash", "push", "--include-untracked", "-m", "with untracked")

	// Now create the same untracked file in the working tree.
	writeFile(t, dir, "newfile.txt", "existing content\n")
	run(t, dir, "git", "add", "newfile.txt")
	run(t, dir, "git", "commit", "-m", "add newfile")

	stashSHA := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewRunner(dir)
	collisions, err := git.CheckUntrackedCollisions(context.Background(), runner, stashSHA)
	if err != nil {
		t.Fatalf("CheckUntrackedCollisions failed: %v", err)
	}

	if len(collisions) == 0 {
		t.Error("expected at least one untracked collision for newfile.txt")
	}

	var found bool
	for _, c := range collisions {
		if c.Path == "newfile.txt" {
			found = true
		}
	}
	if !found {
		t.Error("expected newfile.txt in collisions list")
	}
}
```

**Note:** Add `"strings"` to the imports in the integration test file above (used by `strings.TrimSpace`).

### Step 3: Create the conflict plugin in `internal/plugins/conflict/conflict.go`

```go
package conflict

import (
	"context"
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const PluginID = "conflict"

// SwitchToConflictScreenMsg is sent when conflicts are detected and the
// conflict preview screen should be shown.
type SwitchToConflictScreenMsg struct {
	Stash      core.Stash
	Result     git.MergeTreeResult
	IsPop      bool // true if the user pressed pop, false if apply
}

// Plugin implements plugin.StashHook and plugin.ScreenProvider for
// the conflict preview feature (PRD FR-10).
type Plugin struct {
	git       git.GitRunner
	logger    *slog.Logger
	gitVer    plugin.GitVersion
	lastResult *git.MergeTreeResult // cached for screen rendering
	lastStash  *core.Stash
	lastIsPop  bool
}

// New creates a new conflict preview plugin.
func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return "Conflict Preview" }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.logger = ctx.Logger
	p.gitVer = ctx.GitVer
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// BeforeApply implements StashHook. Runs merge-tree to check for conflicts
// before applying or popping a stash.
func (p *Plugin) BeforeApply(stash core.Stash) (proceed bool, cmd tea.Cmd) {
	// FR-10.7: Requires Git >= 2.38. If unavailable, skip and proceed directly.
	if !p.gitVer.AtLeast(2, 38) {
		p.logger.Info("git version < 2.38, skipping conflict preview")
		return true, func() tea.Msg {
			return core.InfoToastMsg{
				Text: "Conflict preview requires Git >= 2.38. Applying directly.",
			}
		}
	}

	ctx := context.Background()

	// Run merge-tree dry run.
	result, err := git.RunMergeTree(ctx, p.git, stash.SHA)
	if err != nil {
		p.logger.Error("merge-tree failed", "error", err)
		// On error, proceed with apply (fail-open, don't block the user).
		return true, nil
	}

	// Check for untracked file collisions (FR-10.6a).
	collisions, err := git.CheckUntrackedCollisions(ctx, p.git, stash.SHA)
	if err != nil {
		p.logger.Warn("untracked collision check failed", "error", err)
		// Non-fatal; continue with whatever merge-tree found.
	}
	result.UntrackedCollisions = collisions

	// If clean and no untracked collisions, proceed immediately.
	if !result.HasConflicts && len(collisions) == 0 {
		return true, nil
	}

	// Conflicts or collisions detected -- switch to conflict preview screen.
	p.lastResult = &result
	p.lastStash = &stash

	return false, func() tea.Msg {
		return SwitchToConflictScreenMsg{
			Stash:  stash,
			Result: result,
			IsPop:  false, // Caller sets this based on the operation
		}
	}
}

// AfterDrop is a no-op for the conflict plugin.
func (p *Plugin) AfterDrop(stash core.Stash, sha string) tea.Cmd {
	return nil
}

// BeforePush is a no-op for the conflict plugin.
func (p *Plugin) BeforePush(opts plugin.PushOptions) (plugin.PushOptions, error) {
	return opts, nil
}

// Screens returns the conflict preview screen definition.
func (p *Plugin) Screens() []plugin.ScreenDef {
	return []plugin.ScreenDef{
		{
			ID:   "conflict-preview",
			Name: "Conflict Preview",
			Mode: core.ModeConflict,
		},
	}
}

// Update handles messages when the conflict preview screen is active.
func (p *Plugin) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Text {
		case "a":
			// Apply anyway.
			return state, p.applyAnyway(false)
		case "p":
			// Pop anyway.
			return state, p.applyAnyway(true)
		case "b":
			// Branch first -- prompt for branch name.
			return state, p.branchFirst()
		case "escape":
			// Cancel -- return to LIST mode.
			state.Mode = core.ModeList
			return state, nil
		}
	}
	return state, nil
}

// View renders the conflict preview screen.
func (p *Plugin) View(state core.AppState, width, height int) string {
	if p.lastResult == nil {
		return "No conflict data available."
	}
	return renderConflictScreen(p.lastResult, p.lastStash, width, height)
}

func (p *Plugin) applyAnyway(isPop bool) tea.Cmd {
	stash := p.lastStash
	if stash == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		ref := fmt.Sprintf("stash@{%d}", stash.Index)
		if isPop {
			_, err := p.git.Run(ctx, "stash", "pop", ref)
			if err != nil {
				return core.ErrorMsg{Err: fmt.Errorf("stash pop: %w", err)}
			}
		} else {
			_, err := p.git.Run(ctx, "stash", "apply", ref)
			if err != nil {
				return core.ErrorMsg{Err: fmt.Errorf("stash apply: %w", err)}
			}
		}
		return core.StashMutatedMsg{}
	}
}

func (p *Plugin) branchFirst() tea.Cmd {
	stash := p.lastStash
	if stash == nil {
		return nil
	}
	return func() tea.Msg {
		return core.PromptBranchNameMsg{Stash: *stash}
	}
}
```

### Step 4: Create the conflict screen renderer in `internal/plugins/conflict/screen.go`

```go
package conflict

import (
	"fmt"
	"strings"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
)

// renderConflictScreen renders the conflict preview content area.
func renderConflictScreen(result *git.MergeTreeResult, stash *core.Stash, width, height int) string {
	var b strings.Builder

	// Header.
	header := fmt.Sprintf("  Conflict Preview: stash@{%d} — %s", stash.Index, stash.Message)
	if len(header) > width {
		header = header[:width-3] + "..."
	}
	b.WriteString(header)
	b.WriteString("\n\n")

	// File list with status indicators.
	for _, f := range result.Files {
		icon, label := fileStatusDisplay(f.Status)
		line := fmt.Sprintf("  %s %s", icon, f.Path)

		// Right-align the label.
		padding := width - len(line) - len(label) - 4
		if padding < 2 {
			padding = 2
		}
		line += strings.Repeat(" ", padding) + label

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Untracked collisions (FR-10.6a).
	for _, c := range result.UntrackedCollisions {
		line := fmt.Sprintf("  %s %s", untrackedCollisionIcon(), c.Path)
		label := "untracked collision"
		padding := width - len(line) - len(label) - 4
		if padding < 2 {
			padding = 2
		}
		line += strings.Repeat(" ", padding) + label
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Inline conflict zone preview for the first conflicted file.
	for _, f := range result.Files {
		if f.Status == git.FileStatusConflicted && len(f.ConflictZones) > 0 {
			divider := fmt.Sprintf("  --- %s conflict zone 1/%d ---", f.Path, len(f.ConflictZones))
			b.WriteString(divider)
			b.WriteString("\n")
			zone := f.ConflictZones[0]
			b.WriteString("  <<<<<<< HEAD\n")
			for _, line := range strings.Split(zone.OurContent, "\n") {
				b.WriteString("    " + line + "\n")
			}
			b.WriteString("  =======\n")
			for _, line := range strings.Split(zone.TheirContent, "\n") {
				b.WriteString("    " + line + "\n")
			}
			b.WriteString("  >>>>>>> stash\n")
			break // Only show the first conflicted file's first zone in compact view.
		}
	}

	return b.String()
}

// fileStatusDisplay returns the icon and label for a file conflict status.
func fileStatusDisplay(status git.FileConflictStatus) (icon string, label string) {
	switch status {
	case git.FileStatusClean:
		return "\u2713", "clean apply" // checkmark
	case git.FileStatusConflicted:
		return "\u26A1", "conflict" // lightning bolt
	default:
		return "?", "unknown"
	}
}

// untrackedCollisionIcon returns the warning icon for untracked file collisions.
func untrackedCollisionIcon() string {
	return "\u26A0" // warning triangle
}
```

### Step 5: Create conflict plugin tests in `internal/plugins/conflict/conflict_test.go`

```go
package conflict_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugins/conflict"
)

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\noutput: %s", name, args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.name", "test")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	writeFile(t, dir, "README.md", "# test\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")
	return dir
}

func TestConflictPlugin_BeforeApply_ConflictDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create file, commit, modify same line, stash, then change HEAD differently.
	writeFile(t, dir, "config.go", "package main\n\nvar retries = 3\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add config")

	writeFile(t, dir, "config.go", "package main\n\nvar retries = 10\n")
	run(t, dir, "git", "stash", "push", "-m", "bump to 10")

	writeFile(t, dir, "config.go", "package main\n\nvar retries = 5\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "set to 5")

	sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewRunner(dir)
	result, err := git.RunMergeTree(context.Background(), runner, sha)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	if !result.HasConflicts {
		t.Fatal("expected conflicts, got clean merge")
	}
}

func TestConflictPlugin_BeforeApply_CleanMerge(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add main")

	// Stash a change to a different file -- no conflict possible.
	writeFile(t, dir, "other.go", "package main\n\nfunc other() {}\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "stash", "push", "-m", "other file")

	sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewRunner(dir)
	result, err := git.RunMergeTree(context.Background(), runner, sha)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	if result.HasConflicts {
		t.Fatal("expected clean merge, got conflicts")
	}
}

func TestConflictPlugin_UntrackedCollision(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTestRepo(t)

	// Create untracked file and stash with --include-untracked.
	writeFile(t, dir, "collision.txt", "from stash\n")
	run(t, dir, "git", "stash", "push", "--include-untracked", "-m", "with untracked")

	// Create the same file in the working tree and commit it.
	writeFile(t, dir, "collision.txt", "already here\n")
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "add collision.txt")

	sha := strings.TrimSpace(run(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewRunner(dir)
	collisions, err := git.CheckUntrackedCollisions(context.Background(), runner, sha)
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
		t.Errorf("expected collision.txt in collisions, got %v", collisions)
	}
}
```

### Step 6: Create screen rendering tests in `internal/plugins/conflict/screen_test.go`

```go
package conflict_test

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugins/conflict"
)

func TestRenderConflictScreen_Icons(t *testing.T) {
	result := &git.MergeTreeResult{
		HasConflicts: true,
		TreeSHA:      "abc123",
		Files: []git.MergeTreeFile{
			{Path: "clean.go", Status: git.FileStatusClean},
			{Path: "broken.go", Status: git.FileStatusConflicted},
		},
		UntrackedCollisions: []git.UntrackedCollision{
			{Path: "extra.txt"},
		},
	}

	stash := &core.Stash{
		Index:   0,
		SHA:     "abc123",
		Message: "test stash",
	}

	// Use the exported RenderConflictScreen or test via View method.
	// For this test, we call the plugin's View method via the screen.
	p := conflict.New()
	// We need to set the internal state; use the exported test helper or
	// call View after injecting state.
	output := conflict.RenderConflictScreenForTest(result, stash, 80, 24)

	tests := []struct {
		name     string
		contains string
	}{
		{"clean icon for clean.go", "\u2713"},      // checkmark
		{"conflict icon for broken.go", "\u26A1"},  // lightning bolt
		{"untracked collision icon", "\u26A0"},      // warning triangle
		{"clean file path", "clean.go"},
		{"conflicted file path", "broken.go"},
		{"untracked collision path", "extra.txt"},
		{"clean label", "clean apply"},
		{"conflict label", "conflict"},
		{"untracked label", "untracked collision"},
		{"stash header", "stash@{0}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(output, tt.contains) {
				t.Errorf("output missing %q:\n%s", tt.contains, output)
			}
		})
	}
}

func TestRenderConflictScreen_NoFiles(t *testing.T) {
	result := &git.MergeTreeResult{
		HasConflicts: false,
		TreeSHA:      "abc123",
		Files:        nil,
	}

	stash := &core.Stash{
		Index:   2,
		SHA:     "abc123",
		Message: "empty stash",
	}

	output := conflict.RenderConflictScreenForTest(result, stash, 80, 24)

	if !strings.Contains(output, "stash@{2}") {
		t.Errorf("expected stash index in header, got:\n%s", output)
	}
}
```

**Note:** Export `RenderConflictScreenForTest` from `screen.go` as a test helper:

```go
// RenderConflictScreenForTest is exported for testing only.
func RenderConflictScreenForTest(result *git.MergeTreeResult, stash *core.Stash, width, height int) string {
	return renderConflictScreen(result, stash, width, height)
}
```

### Step 7: Wire the conflict plugin into the plugin loader

Add to `internal/plugin/loader.go`:

```go
import "github.com/indrasvat/nidhi/internal/plugins/conflict"

func LoadBuiltinPlugins() []Plugin {
	return []Plugin{
		conflict.New(),
		// ... other plugins
	}
}
```

### Step 8: Add the SwitchToConflictScreenMsg to core events

Add to `internal/core/events.go`:

```go
// InfoToastMsg triggers an informational toast notification.
type InfoToastMsg struct {
	Text string
}

// ErrorMsg wraps an error as a tea.Msg.
type ErrorMsg struct {
	Err error
}

// StashMutatedMsg signals that the stash list has changed and cache must be invalidated.
type StashMutatedMsg struct{}

// PromptBranchNameMsg signals that the user should be prompted for a branch name.
type PromptBranchNameMsg struct {
	Stash Stash
}
```

Add to `internal/core/mode.go`:

```go
const (
	ModeList     Mode = iota
	ModePreview
	ModeDetail
	ModeConflict  // Conflict preview screen
	ModeNewStash  // New stash creation screen
)
```

### Step 9: Verify

```bash
# Run all tests
make test

# Run only conflict-related tests
go test -v -run TestParseMergeTree ./internal/git/...
go test -v -run TestRunMergeTree ./internal/git/...
go test -v -run TestConflictPlugin ./internal/plugins/conflict/...
go test -v -run TestRenderConflictScreen ./internal/plugins/conflict/...

# Lint
make lint

# Full CI
make ci
```

## Verification

### Functional
```bash
# Unit tests: merge-tree output parsing
go test -v -run TestParseMergeTreeOutput ./internal/git/...
# Expected: all 5 table-driven test cases pass

# Integration: conflict detection with real git repo
go test -v -run TestRunMergeTree_Integration_ConflictDetected ./internal/git/...
# Expected: merge-tree returns HasConflicts=true, config.go marked conflicted

# Integration: clean apply
go test -v -run TestRunMergeTree_Integration_CleanApply ./internal/git/...
# Expected: merge-tree returns HasConflicts=false

# Integration: untracked file collision
go test -v -run TestCheckUntrackedCollisions_Integration ./internal/git/...
# Expected: collision detected for newfile.txt

# Screen rendering: correct icons per status
go test -v -run TestRenderConflictScreen_Icons ./internal/plugins/conflict/...
# Expected: checkmark for clean, lightning for conflict, warning for untracked

# Full CI pipeline
make ci
```

### Edge Cases
```bash
# File status mapping
go test -v -run TestParseMergeTreeOutput_FileStatuses ./internal/git/...

# Empty result
go test -v -run TestRenderConflictScreen_NoFiles ./internal/plugins/conflict/...
```

## Completion Criteria
1. `git merge-tree --write-tree` is invoked when user presses `a` or `p` on a stash
2. Exit code 0 (clean) proceeds immediately without showing conflict screen
3. Exit code 1 (conflicts) switches to the conflict preview screen with per-file status
4. Parse output correctly: `CONFLICT` lines mark files as conflicted, `Auto-merging` marks as clean
5. Untracked file collisions (stash untracked vs working tree) are detected and shown (FR-10.6a)
6. Screen shows correct icons: checkmark (clean), lightning (conflict), warning (untracked collision)
7. Screen options work: `a` apply anyway, `p` pop anyway, `b` branch first, `Esc` cancel
8. Git < 2.38: skip conflict preview, show info toast, apply directly (FR-10.7)
9. All tests pass including integration tests with real git repos using `t.TempDir()`
10. `make ci` passes

## Commit
```
feat(conflict): add conflict preview plugin with merge-tree dry-run

Implement FR-10 conflict preview plugin that intercepts apply/pop
operations, runs `git merge-tree --write-tree` to detect conflicts
before applying, and shows a preview screen with per-file status
indicators. Includes untracked file collision detection (FR-10.6a)
and graceful degradation for Git < 2.38 (FR-10.7).
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.2 (FR-10), 8.2 (interfaces), 10 (Screen 4), 16
4. Read tasks 006 and 013 to understand dependencies
5. Execute steps 1-9 in order
6. Verify all functional and edge case checks pass
7. Update this file (Status: DONE) + `docs/PROGRESS.md`
8. Commit with the message above + move to next task
