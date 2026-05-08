# Task 023: Export/Import & Remote Sync Plugin

## Status: TODO

## Depends On
- 006 (core model — AppState, Stash type, Mode enum, plugin interfaces)
- 001 (GitRunner — git command execution, version detection)

## Parallelizable With
- 020 (search plugin)
- 021 (filter and stale plugins)
- 022 (reorder plugin)
- 024 (help overlay and mouse support)
- 025 (config file and polish)

## Problem
Before Git 2.51, sharing stashes across machines required arcane plumbing: `git stash create`, push a commit to a custom ref, fetch on the other side, `git stash store`. Git 2.51 introduced first-class `git stash export` and `git stash import` commands that make this workflow ergonomic. nidhi must wrap these commands with a multi-select export screen (choose stashes, set ref path, pick remote, see live command preview) and an import screen (fetch, preview incoming stashes, confirm). The plugin must gracefully degrade: if Git < 2.51, pressing `e` or `i` shows an informational message about upgrading rather than failing silently.

## PRD Reference
- Section 5.2 (Export/Import) — `git stash export --to-ref <ref> [stash...]`, `git stash import <commit>`, full workflow
- Section 6.2, FR-12 (Export/Import & Remote Sync) — FR-12.1 through FR-12.7
- Section 10, Screen 7 (EXPORT) — layout spec, multi-select, ref input, remote selector, command preview
- Section 11.2 (NEW/EXPORT/IMPORT keymap) — Tab/Shift+Tab fields, Space toggle, Enter confirm, Esc cancel
- Section 8.2 (Plugin interfaces) — KeyHandler + ScreenProvider
- Section 8.4 (Module structure) — `internal/plugins/sync/sync.go`, `internal/ui/screens/export.go`, `internal/ui/screens/importscreen.go`
- Section 5.4 (Feature Gating) — Git >= 2.51 for export/import
- Section 12.2 — `export.ref = "refs/stashes/$USER"`, `export.remote = "origin"`
- Section 10, Screen 7, FR-10.6a / codex #20 — ref validation via `git check-ref-format`, remote from `git remote` list

## Files to Create
- `internal/plugins/sync/sync.go` — sync plugin implementing KeyHandler + ScreenProvider
- `internal/ui/screens/export.go` — export screen model and view
- `internal/ui/screens/importscreen.go` — import screen model and view
- `internal/plugins/sync/sync_test.go` — unit and integration tests

## Execution Steps

### Step 1: Create sync plugin (`internal/plugins/sync/sync.go`)

```go
package sync

import (
	"context"
	"fmt"
	"os/user"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID   = "sync"
	PluginName = "Export/Import & Remote Sync"

	// MinGitVersionExport is the minimum git version required for export/import.
	MinGitVersionMajor = 2
	MinGitVersionMinor = 51
)

// Plugin implements KeyHandler + ScreenProvider for export/import.
type Plugin struct {
	ctx            plugin.PluginContext
	exportEnabled  bool   // false if Git < 2.51
	defaultRef     string // from config, e.g. "refs/stashes/$USER"
	defaultRemote  string // from config, e.g. "origin"
}

var (
	_ plugin.KeyHandler     = (*Plugin)(nil)
	_ plugin.ScreenProvider = (*Plugin)(nil)
)

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.ctx = ctx

	// Check git version for export/import support.
	p.exportEnabled = ctx.GitVer.AtLeast(MinGitVersionMajor, MinGitVersionMinor)

	// Load config defaults.
	p.defaultRef = ctx.Config.GetString("export.ref", "refs/stashes/$USER")
	p.defaultRemote = ctx.Config.GetString("export.remote", "origin")

	// Expand $USER in ref.
	if u, err := user.Current(); err == nil {
		p.defaultRef = strings.ReplaceAll(p.defaultRef, "$USER", u.Username)
	}

	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns export/import key bindings.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "e", Description: "Export stashes", Modes: []core.Mode{core.ModeList}},
		{Key: "i", Description: "Import stashes", Modes: []core.Mode{core.ModeList}},
	}
}

// HandleKey processes e/i key events.
func (p *Plugin) HandleKey(key plugin.KeyEvent, state core.AppState) (core.AppState, tea.Cmd) {
	switch key.Text {
	case "e":
		if !p.exportEnabled {
			// Git < 2.51 — show info message (FR-12.7).
			return state, func() tea.Msg {
				return core.ToastMsg{
					Text:  "Export requires Git >= 2.51. Current: " + state.GitVersion.String(),
					Level: core.ToastInfo,
				}
			}
		}
		state.Mode = core.ModeExport
		return state, p.initExportCmd(state)

	case "i":
		if !p.exportEnabled {
			return state, func() tea.Msg {
				return core.ToastMsg{
					Text:  "Import requires Git >= 2.51. Current: " + state.GitVersion.String(),
					Level: core.ToastInfo,
				}
			}
		}
		state.Mode = core.ModeImport
		return state, p.initImportCmd(state)
	}

	return state, nil
}

// Screens returns export and import screen definitions.
func (p *Plugin) Screens() []plugin.ScreenDef {
	return []plugin.ScreenDef{
		{ID: "export", Mode: core.ModeExport},
		{ID: "import", Mode: core.ModeImport},
	}
}

// Update handles messages when export or import screen is active.
func (p *Plugin) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	// Delegate to the active screen's model (export or import).
	// The screen models are stored as plugin-internal state.
	// This is the dispatch point — actual handling is in export.go / importscreen.go.
	return state, nil
}

// View renders the active screen (export or import).
func (p *Plugin) View(state core.AppState, width, height int) string {
	// Delegate to the active screen's view.
	return ""
}

// --- Commands ---

// RemoteListMsg carries the list of configured git remotes.
type RemoteListMsg struct {
	Remotes []Remote
}

// Remote represents a git remote.
type Remote struct {
	Name string
	URL  string
}

// initExportCmd fetches the remote list for the export screen.
func (p *Plugin) initExportCmd(state core.AppState) tea.Cmd {
	git := p.ctx.Git
	return func() tea.Msg {
		ctx := context.Background()
		// Get remote list.
		lines, err := git.RunLines(ctx, "remote", "-v")
		if err != nil {
			return core.ErrorMsg{Error: fmt.Errorf("list remotes: %w", err)}
		}
		remotes := parseRemotes(lines)
		return RemoteListMsg{Remotes: remotes}
	}
}

// initImportCmd fetches the remote list for the import screen.
func (p *Plugin) initImportCmd(state core.AppState) tea.Cmd {
	return p.initExportCmd(state) // Same initial data needed.
}

// parseRemotes parses `git remote -v` output into Remote structs.
// Deduplicates by name (git remote -v shows fetch and push URLs).
func parseRemotes(lines []string) []Remote {
	seen := make(map[string]bool)
	var remotes []Remote
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		url := parts[1]
		if !seen[name] {
			seen[name] = true
			remotes = append(remotes, Remote{Name: name, URL: url})
		}
	}
	return remotes
}

// --- Ref Validation ---

// ValidateRef checks if a ref name is valid using `git check-ref-format`.
func ValidateRef(ctx context.Context, git core.GitRunner, ref string) error {
	_, err := git.Run(ctx, "check-ref-format", ref)
	if err != nil {
		return fmt.Errorf("invalid ref format %q: %w", ref, err)
	}
	return nil
}

// --- Export Execution ---

// ExportMsg is the request to export selected stashes.
type ExportMsg struct {
	StashIndices []int  // Indices of stashes to export
	Ref          string // Target ref (e.g. "refs/stashes/user")
	Remote       string // Remote name (e.g. "origin")
}

// ExportResultMsg is the result of an export operation.
type ExportResultMsg struct {
	ExportedCount int
	Ref           string
	Remote        string
	Error         error
}

// ExportCmd executes the export workflow:
// 1. git stash export --to-ref <ref> [stash indices...]
// 2. git push --no-verify --force <remote> <ref>
func ExportCmd(git core.GitRunner, msg ExportMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Step 1: Validate ref format.
		if err := ValidateRef(ctx, git, msg.Ref); err != nil {
			return ExportResultMsg{Error: err}
		}

		// Step 2: Export stashes to ref.
		args := []string{"stash", "export", "--to-ref", msg.Ref}
		for _, idx := range msg.StashIndices {
			args = append(args, "stash@{"+strconv.Itoa(idx)+"}")
		}
		_, err := git.Run(ctx, args...)
		if err != nil {
			return ExportResultMsg{Error: fmt.Errorf("export: %w", err)}
		}

		// Step 3: Push to remote.
		_, err = git.Run(ctx, "push", "--no-verify", "--force", msg.Remote, msg.Ref)
		if err != nil {
			return ExportResultMsg{Error: fmt.Errorf("push: %w", err)}
		}

		return ExportResultMsg{
			ExportedCount: len(msg.StashIndices),
			Ref:           msg.Ref,
			Remote:        msg.Remote,
		}
	}
}

// --- Import Execution ---

// ImportMsg is the request to import stashes from a remote ref.
type ImportMsg struct {
	Ref    string
	Remote string
}

// ImportResultMsg is the result of an import operation.
type ImportResultMsg struct {
	ImportedCount int
	Error         error
}

// ImportCmd executes the import workflow:
// 1. git fetch <remote> <ref>:<ref>
// 2. git stash import <ref>
func ImportCmd(git core.GitRunner, msg ImportMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Step 1: Fetch from remote.
		refSpec := msg.Ref + ":" + msg.Ref
		_, err := git.Run(ctx, "fetch", msg.Remote, refSpec)
		if err != nil {
			return ImportResultMsg{Error: fmt.Errorf("fetch: %w", err)}
		}

		// Step 2: Import stashes.
		_, err = git.Run(ctx, "stash", "import", msg.Ref)
		if err != nil {
			return ImportResultMsg{Error: fmt.Errorf("import: %w", err)}
		}

		return ImportResultMsg{ImportedCount: -1} // Count unknown until list refresh.
	}
}
```

### Step 2: Create export screen (`internal/ui/screens/export.go`)

```go
package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugins/sync"
)

// ExportScreen manages the export workflow UI.
type ExportScreen struct {
	stashes    []core.Stash
	selected   []bool     // Selection state per stash
	refInput   textinput.Model
	remotes    []sync.Remote
	remoteIdx  int        // Currently selected remote
	focusField int        // 0=stash list, 1=ref input, 2=remote selector
	executing  bool
}

// NewExportScreen creates a new export screen with defaults.
func NewExportScreen(stashes []core.Stash, defaultRef string) *ExportScreen {
	ti := textinput.New()
	ti.SetValue(defaultRef)
	ti.CharLimit = 256
	ti.Placeholder = "refs/stashes/username"

	selected := make([]bool, len(stashes))
	// Default: select all stashes.
	for i := range selected {
		selected[i] = true
	}

	return &ExportScreen{
		stashes:  stashes,
		selected: selected,
		refInput: ti,
	}
}

// SelectedIndices returns the indices of selected stashes.
func (s *ExportScreen) SelectedIndices() []int {
	var indices []int
	for i, sel := range s.selected {
		if sel {
			indices = append(indices, s.stashes[i].Index)
		}
	}
	return indices
}

// CommandPreview returns the exact commands that will be executed.
func (s *ExportScreen) CommandPreview() string {
	indices := s.SelectedIndices()
	if len(indices) == 0 || s.refInput.Value() == "" {
		return "(select stashes and enter a ref path)"
	}

	var stashArgs []string
	for _, idx := range indices {
		stashArgs = append(stashArgs, fmt.Sprintf("stash@{%d}", idx))
	}

	remote := "origin"
	if s.remoteIdx < len(s.remotes) {
		remote = s.remotes[s.remoteIdx].Name
	}
	ref := s.refInput.Value()

	line1 := fmt.Sprintf("$ git stash export --to-ref %s %s", ref, strings.Join(stashArgs, " "))
	line2 := fmt.Sprintf("$ git push --no-verify --force %s %s", remote, ref)
	return line1 + "\n" + line2
}

// View renders the export screen.
func (s *ExportScreen) View(width, height int) string {
	var b strings.Builder

	b.WriteString("\n  Select stashes to export:\n")
	for i, stash := range s.stashes {
		check := "[ ]"
		if s.selected[i] {
			check = "[✓]"
		}
		b.WriteString(fmt.Sprintf("    %s  %d  %s\n", check, stash.Index, stash.Message))
	}

	b.WriteString(fmt.Sprintf("\n  Ref:    %s\n", s.refInput.View()))

	b.WriteString("  Remote: ")
	if len(s.remotes) > 0 {
		remote := s.remotes[s.remoteIdx]
		b.WriteString(fmt.Sprintf("%s (%s)", remote.Name, remote.URL))
	} else {
		b.WriteString("(no remotes configured)")
	}
	b.WriteString("\n")

	b.WriteString("\n  Command preview:\n")
	for _, line := range strings.Split(s.CommandPreview(), "\n") {
		b.WriteString("  " + line + "\n")
	}

	return b.String()
}
```

### Step 3: Create import screen (`internal/ui/screens/importscreen.go`)

```go
package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugins/sync"
)

// ImportScreen manages the import workflow UI.
type ImportScreen struct {
	refInput   textinput.Model
	remotes    []sync.Remote
	remoteIdx  int
	focusField int  // 0=remote, 1=ref input
	fetched    bool // true after successful fetch
	importing  bool
	incoming   []core.Stash // Stashes from the fetched ref (preview)
}

// NewImportScreen creates a new import screen.
func NewImportScreen(defaultRef string) *ImportScreen {
	ti := textinput.New()
	ti.SetValue(defaultRef)
	ti.CharLimit = 256
	ti.Placeholder = "refs/stashes/username"
	ti.Focus()

	return &ImportScreen{
		refInput: ti,
	}
}

// View renders the import screen.
func (s *ImportScreen) View(width, height int) string {
	var b strings.Builder

	b.WriteString("\n  Import stashes from remote:\n\n")

	b.WriteString("  Remote: ")
	if len(s.remotes) > 0 {
		remote := s.remotes[s.remoteIdx]
		b.WriteString(fmt.Sprintf("%s (%s)", remote.Name, remote.URL))
	} else {
		b.WriteString("(no remotes configured)")
	}
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("  Ref:    %s\n", s.refInput.View()))

	if s.fetched && len(s.incoming) > 0 {
		b.WriteString("\n  Incoming stashes:\n")
		for _, stash := range s.incoming {
			b.WriteString(fmt.Sprintf("    %s  %s\n", stash.ShortSHA, stash.Message))
		}
		b.WriteString("\n  Press Enter to import, Esc to cancel.\n")
	} else if s.fetched {
		b.WriteString("\n  No stashes found at this ref.\n")
	} else {
		b.WriteString("\n  Press Enter to fetch and preview.\n")
	}

	return b.String()
}
```

### Step 4: Write tests (`internal/plugins/sync/sync_test.go`)

```go
package sync_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugins/sync"
)

// --- Test Helpers ---

// setupRepoWithRemote creates two temp repos: a "remote" bare repo and
// a "local" clone with stashes. Returns both paths.
func setupRepoWithRemote(t *testing.T) (localDir, remoteDir string) {
	t.Helper()

	// Create bare remote repo.
	remoteDir = t.TempDir()
	runIn := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v in %s failed: %v\noutput: %s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	writeFileIn := func(dir, name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	runIn(remoteDir, "git", "init", "--bare")

	// Clone to local.
	localDir = t.TempDir()
	runIn(localDir, "git", "clone", remoteDir, ".")
	runIn(localDir, "git", "config", "user.email", "test@test.com")
	runIn(localDir, "git", "config", "user.name", "Test")

	// Create initial commit and push.
	writeFileIn(localDir, "base.go", "package main\n")
	runIn(localDir, "git", "add", ".")
	runIn(localDir, "git", "commit", "-m", "init")
	runIn(localDir, "git", "push", "origin", "main")

	return localDir, remoteDir
}

// gitVersionAtLeast checks if the installed git is >= major.minor.
func gitVersionAtLeast(t *testing.T, major, minor int) bool {
	t.Helper()
	cmd := exec.Command("git", "version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git version failed: %v", err)
	}
	// "git version 2.53.0"
	parts := strings.Fields(string(out))
	if len(parts) < 3 {
		return false
	}
	ver := parts[2]
	nums := strings.SplitN(ver, ".", 3)
	if len(nums) < 2 {
		return false
	}
	maj := 0
	min := 0
	fmt.Sscanf(nums[0], "%d", &maj)
	fmt.Sscanf(nums[1], "%d", &min)
	return maj > major || (maj == major && min >= minor)
}

// --- Unit Tests ---

// TestRefValidation tests valid and invalid ref formats.
func TestRefValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	run := func(args ...string) (string, error) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}

	// Initialize a git repo so check-ref-format works.
	run("git", "init")

	tests := []struct {
		ref   string
		valid bool
	}{
		{"refs/stashes/user", true},
		{"refs/stashes/user/machine1", true},
		{"refs/heads/main", true},
		{"refs/stashes/user name", false},  // spaces not allowed
		{"refs/stashes/.hidden", false},     // starts with dot
		{"refs/stashes/user..double", false}, // double dots
		{"refs/stashes/user~1", false},       // tilde
		{"refs/stashes/user^0", false},       // caret
		{"refs/stashes/user:colon", false},   // colon
		{"refs/stashes/user\\back", false},   // backslash
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			_, err := run("git", "check-ref-format", tt.ref)
			isValid := err == nil
			if isValid != tt.valid {
				t.Errorf("ref %q: expected valid=%v, got %v", tt.ref, tt.valid, isValid)
			}
		})
	}
}

// TestRemoteParsing tests parsing of `git remote -v` output.
func TestRemoteParsing(t *testing.T) {
	lines := []string{
		"origin\thttps://github.com/user/repo.git (fetch)",
		"origin\thttps://github.com/user/repo.git (push)",
		"upstream\thttps://github.com/other/repo.git (fetch)",
		"upstream\thttps://github.com/other/repo.git (push)",
	}

	remotes := sync.ParseRemotesExported(lines)
	if len(remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(remotes))
	}
	if remotes[0].Name != "origin" {
		t.Errorf("expected first remote 'origin', got %q", remotes[0].Name)
	}
	if remotes[1].Name != "upstream" {
		t.Errorf("expected second remote 'upstream', got %q", remotes[1].Name)
	}
}

// TestRemoteParsingEmpty tests parsing when no remotes are configured.
func TestRemoteParsingEmpty(t *testing.T) {
	remotes := sync.ParseRemotesExported(nil)
	if len(remotes) != 0 {
		t.Errorf("expected 0 remotes, got %d", len(remotes))
	}
}

// TestFeatureGateGitVersion tests that export/import is disabled for Git < 2.51.
func TestFeatureGateGitVersion(t *testing.T) {
	// Mock a Git version below 2.51.
	// This tests the version comparison logic, not actual git.
	type mockVersion struct {
		Major int
		Minor int
	}

	tests := []struct {
		version mockVersion
		enabled bool
	}{
		{mockVersion{2, 38}, false},
		{mockVersion{2, 50}, false},
		{mockVersion{2, 51}, true},
		{mockVersion{2, 52}, true},
		{mockVersion{2, 53}, true},
		{mockVersion{3, 0}, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("git_%d.%d", tt.version.Major, tt.version.Minor), func(t *testing.T) {
			enabled := tt.version.Major > sync.MinGitVersionMajor ||
				(tt.version.Major == sync.MinGitVersionMajor && tt.version.Minor >= sync.MinGitVersionMinor)
			if enabled != tt.enabled {
				t.Errorf("git %d.%d: expected enabled=%v, got %v",
					tt.version.Major, tt.version.Minor, tt.enabled, enabled)
			}
		})
	}
}

// --- Integration Tests ---
// NOTE: These tests require `git stash export/import` which needs Git >= 2.51.
// They are gated with a version check and t.Skip().

// TestExportAndPushToRemote creates a repo with a remote, exports a stash,
// and verifies the ref was created and pushed.
func TestExportAndPushToRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !gitVersionAtLeast(t, 2, 51) {
		t.Skip("requires Git >= 2.51 for git stash export/import")
	}

	localDir, remoteDir := setupRepoWithRemote(t)

	runIn := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v in %s failed: %v\noutput: %s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	writeFileIn := func(dir, name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a stash in local repo.
	writeFileIn(localDir, "feature.go", "package feature\n\nfunc Do() {}\n")
	runIn(localDir, "git", "add", ".")
	runIn(localDir, "git", "stash", "push", "-m", "export test stash")

	// Verify stash exists.
	stashList := runIn(localDir, "git", "stash", "list")
	if !strings.Contains(stashList, "export test stash") {
		t.Fatalf("stash not found: %s", stashList)
	}

	// Export stash to ref.
	ref := "refs/stashes/testuser"
	runIn(localDir, "git", "stash", "export", "--to-ref", ref, "stash@{0}")

	// Verify ref exists locally.
	refOutput := runIn(localDir, "git", "show-ref", ref)
	if refOutput == "" {
		t.Fatal("expected ref to exist after export")
	}

	// Push to remote.
	runIn(localDir, "git", "push", "--no-verify", "--force", "origin", ref)

	// Verify ref exists on remote.
	remoteRefs := runIn(remoteDir, "git", "show-ref")
	if !strings.Contains(remoteRefs, ref) {
		t.Fatalf("expected ref %s on remote, got:\n%s", ref, remoteRefs)
	}
}

// TestFetchAndImportFromRemote exports from one clone, fetches in another,
// and imports to verify the full round-trip.
func TestFetchAndImportFromRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if !gitVersionAtLeast(t, 2, 51) {
		t.Skip("requires Git >= 2.51 for git stash export/import")
	}

	localDir, remoteDir := setupRepoWithRemote(t)

	runIn := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v in %s failed: %v\noutput: %s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	writeFileIn := func(dir, name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a stash in local and export it.
	writeFileIn(localDir, "feature.go", "package feature\n")
	runIn(localDir, "git", "add", ".")
	runIn(localDir, "git", "stash", "push", "-m", "roundtrip test")

	ref := "refs/stashes/testuser"
	runIn(localDir, "git", "stash", "export", "--to-ref", ref, "stash@{0}")
	runIn(localDir, "git", "push", "--no-verify", "--force", "origin", ref)

	// Create a second clone (simulating another machine).
	clone2Dir := t.TempDir()
	runIn(clone2Dir, "git", "clone", remoteDir, ".")
	runIn(clone2Dir, "git", "config", "user.email", "test@test.com")
	runIn(clone2Dir, "git", "config", "user.name", "Test")

	// Verify no stashes in clone2.
	stashList := runIn(clone2Dir, "git", "stash", "list")
	if stashList != "" {
		t.Fatalf("expected no stashes in clone2, got: %s", stashList)
	}

	// Fetch and import.
	runIn(clone2Dir, "git", "fetch", "origin", ref+":"+ref)
	runIn(clone2Dir, "git", "stash", "import", ref)

	// Verify stash was imported.
	stashList = runIn(clone2Dir, "git", "stash", "list")
	if !strings.Contains(stashList, "roundtrip test") {
		t.Fatalf("expected 'roundtrip test' in imported stash list, got: %s", stashList)
	}
}

// TestExportDisabledOnOldGit verifies that the plugin shows an info message
// when Git version is below 2.51.
func TestExportDisabledOnOldGit(t *testing.T) {
	// This is a unit test — we mock the git version check.
	// If git version is below 2.51, HandleKey for "e" should return a ToastMsg.

	// Version 2.38 (below 2.51).
	enabled := 2 > sync.MinGitVersionMajor ||
		(2 == sync.MinGitVersionMajor && 38 >= sync.MinGitVersionMinor)
	if enabled {
		t.Error("expected export disabled for git 2.38")
	}

	// Version 2.51 (at minimum).
	enabled = 2 > sync.MinGitVersionMajor ||
		(2 == sync.MinGitVersionMajor && 51 >= sync.MinGitVersionMinor)
	if !enabled {
		t.Error("expected export enabled for git 2.51")
	}
}
```

### Step 5: Verify

```bash
# Unit tests (always run).
go test -v -count=1 -run 'TestRefValidation|TestRemoteParsing|TestFeatureGate|TestExportDisabled' ./internal/plugins/sync/...

# Integration tests (skipped if Git < 2.51).
go test -v -count=1 -run 'TestExportAndPush|TestFetchAndImport' ./internal/plugins/sync/...

# Full CI pipeline.
make ci
```

## Verification

### Functional
```bash
# Unit tests pass
go test -v -count=1 -run 'TestRefValidation|TestRemoteParsing|TestFeatureGate|TestExportDisabled' ./internal/plugins/sync/...

# Integration tests pass (or skip if Git < 2.51)
go test -v -count=1 -run 'TestExportAndPush|TestFetchAndImport' ./internal/plugins/sync/...

# Export screen compiles
go vet ./internal/ui/screens/...

# Lint clean
golangci-lint run ./internal/plugins/sync/... ./internal/ui/screens/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/plugins/sync/sync.go` implements `KeyHandler` and `ScreenProvider`
2. `internal/ui/screens/export.go` renders export screen with multi-select, ref input, remote selector, command preview
3. `internal/ui/screens/importscreen.go` renders import screen with fetch, preview, confirm
4. `e` key opens Export screen; `i` key opens Import screen
5. Export executes: `git stash export --to-ref <ref> [stashes]` then `git push --no-verify --force <remote> <ref>`
6. Import executes: `git fetch <remote> <ref>:<ref>` then `git stash import <ref>`
7. Ref validation via `git check-ref-format` — invalid refs show error
8. Remote list populated from `git remote -v`
9. Live command preview updates as user changes selections/ref/remote
10. Git < 2.51: `e`/`i` show info toast about upgrading Git (FR-12.7)
11. Integration tests gate on `git version >= 2.51` with `t.Skip()`
12. All unit tests pass: ref validation, remote parsing, version gating
13. All integration tests pass (when Git >= 2.51): export+push, fetch+import round-trip
14. `make ci` passes (lint + test)

## Commit
```
feat(sync): add export/import plugin with remote sync (Git >= 2.51)

Implement sync plugin (KeyHandler + ScreenProvider) wrapping git stash
export/import commands from Git 2.51. Export screen provides multi-select
stash list, editable ref path, remote selector, and live command preview.
Import screen fetches from remote and previews incoming stashes before
importing. Ref validation via git check-ref-format. Graceful degradation:
Git < 2.51 shows info toast. Integration tests gated with version check.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 5.2 (export/import commands), 6.2 (FR-12), 10 (Screen 7), 11.2 (keymap), 5.4 (version gating), 12.2 (config)
4. Verify dependencies: task 006 (core model) and task 001 (GitRunner with version detection) are DONE
5. Create `internal/plugins/sync/sync.go` with Plugin, ExportCmd, ImportCmd, ValidateRef
6. Create `internal/ui/screens/export.go` with ExportScreen
7. Create `internal/ui/screens/importscreen.go` with ImportScreen
8. Create `internal/plugins/sync/sync_test.go` with all unit and integration tests
9. Run `go test -v -count=1 ./internal/plugins/sync/... ./internal/ui/screens/...`
10. Run `make ci`
11. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
12. Commit with the message above
