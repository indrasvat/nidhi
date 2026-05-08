# Task 011: PREVIEW Screen — Tab Toggle Diff View

## Status: TODO

## Depends On
- 010 (LIST screen — `internal/ui/screens/list.go`)
- 009 (diff view component — `internal/ui/components/diffview.go`)
- 007 (layout engine — `internal/ui/layout/split.go`)
- 004 (stash cache — `internal/git/cache.go`, `StashCache.Diff()`)

## Parallelizable With
- 012 (DETAIL screen — both extend the LIST screen independently)

## Problem
When a developer presses `Tab` in LIST mode, the screen should split: the stash list compresses to the top ~40% and a diff viewport appears in the bottom ~60%. This is PREVIEW mode (PRD §10 Screen 2). The diff is loaded from `StashCache.Diff(sha)` — if not yet cached, a loading indicator appears. `h`/`l` cycle through individual files in the diff. Moving the cursor in the list updates the preview in real-time. All LIST actions (apply, pop, drop, etc.) remain available. `Tab` toggles back to LIST mode.

## PRD Reference
- Section 6.1 FR-03.2 — PREVIEW mode definition
- Section 10 Screen 2 — PREVIEW layout spec, split ratios, file cycling
- Section 11.2 — PREVIEW mode keymap (j/k in list, h/l cycle files, Tab toggle, ^d/^u scroll, all LIST actions)
- Section 13.4 — `viewport.Model` for diff rendering
- Section 13.5 — Split pane (~80 lines), custom component
- Section 14.2 — Tab toggle < 50ms, diff from cache or ~30-50ms git call

## Files to Create
- `internal/ui/screens/preview.go` — PREVIEW mode screen model
- `internal/ui/screens/preview_test.go` — unit and integration tests

## Execution Steps

### Step 1: Define the PreviewScreen model

```go
// internal/ui/screens/preview.go
package screens

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/ui/layout"
)

// PreviewScreen implements PREVIEW mode — split layout with compressed list (top)
// and diff viewport (bottom).
type PreviewScreen struct {
	// list embeds the ListScreen for cursor navigation and rendering.
	list *ListScreen

	// viewport is the diff display pane.
	viewport viewport.Model

	// diffFiles holds the parsed file sections from the current diff.
	diffFiles []diffFileSection

	// fileIndex is the currently displayed file (cycled with h/l).
	fileIndex int

	// loading indicates that a diff is being fetched asynchronously.
	loading bool

	// currentSHA tracks which stash's diff is currently loaded,
	// so we know when to re-fetch on cursor move.
	currentSHA string

	// cache provides access to stash diffs.
	cache git.StashCache

	// dimensions
	width  int
	height int
}

// diffFileSection represents one file's chunk of the diff output.
type diffFileSection struct {
	Filename string
	Content  string
}

// listHeightRatio is the fraction of height allocated to the compressed list.
const listHeightRatio = 0.4

// NewPreviewScreen creates a new PREVIEW mode screen.
func NewPreviewScreen(list *ListScreen, cache git.StashCache) *PreviewScreen {
	return &PreviewScreen{
		list:  list,
		cache: cache,
	}
}
```

### Step 2: Implement diff file parsing and cycling

```go
// parseDiffFiles splits a unified diff into per-file sections.
// Each section starts with "diff --git a/... b/..." and includes all hunks.
func parseDiffFiles(diff string) []diffFileSection {
	if diff == "" {
		return nil
	}

	var sections []diffFileSection
	lines := strings.Split(diff, "\n")
	var current *diffFileSection

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			// Start a new file section.
			if current != nil {
				sections = append(sections, *current)
			}
			filename := extractFilename(line)
			current = &diffFileSection{
				Filename: filename,
				Content:  line + "\n",
			}
		} else if current != nil {
			current.Content += line + "\n"
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}

// extractFilename pulls the b/ path from "diff --git a/foo b/foo".
func extractFilename(diffLine string) string {
	parts := strings.SplitN(diffLine, " b/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return diffLine
}

// cycleFile moves the file index by delta and wraps around.
func (p *PreviewScreen) cycleFile(delta int) {
	if len(p.diffFiles) == 0 {
		return
	}
	p.fileIndex = (p.fileIndex + delta + len(p.diffFiles)) % len(p.diffFiles)
	p.viewport.SetContent(p.diffFiles[p.fileIndex].Content)
	p.viewport.GotoTop()
}

// fileProgressIndicator returns "filename (1/3)" style header text.
func (p *PreviewScreen) fileProgressIndicator() string {
	if len(p.diffFiles) == 0 {
		return ""
	}
	f := p.diffFiles[p.fileIndex]
	return fmt.Sprintf("%s (%d/%d)", f.Filename, p.fileIndex+1, len(p.diffFiles))
}
```

### Step 3: Implement the diff loading command

```go
// DiffLoadedMsg is sent when a diff finishes loading from the cache.
type DiffLoadedMsg struct {
	SHA     string
	Diff    string
	Err     error
}

// loadDiffCmd returns a tea.Cmd that fetches the diff for a stash SHA.
func loadDiffCmd(cache git.StashCache, sha string) tea.Cmd {
	return func() tea.Msg {
		diff, err := cache.Diff(context.Background(), sha)
		return DiffLoadedMsg{SHA: sha, Diff: diff, Err: err}
	}
}
```

### Step 4: Implement Update method

```go
// Update handles messages for the PREVIEW screen.
func (p *PreviewScreen) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height - layout.ChromeHeight
		p.recalcSplit()
		return state, nil

	case DiffLoadedMsg:
		if msg.SHA != p.currentSHA {
			return state, nil // stale response for a different stash
		}
		p.loading = false
		if msg.Err != nil {
			p.viewport.SetContent(fmt.Sprintf("Error loading diff: %v", msg.Err))
			return state, nil
		}
		p.diffFiles = parseDiffFiles(msg.Diff)
		p.fileIndex = 0
		if len(p.diffFiles) > 0 {
			p.viewport.SetContent(p.diffFiles[0].Content)
		} else {
			p.viewport.SetContent("(no changes)")
		}
		return state, nil

	case tea.KeyPressMsg:
		return p.handleKey(msg, state)
	}

	return state, nil
}

func (p *PreviewScreen) handleKey(msg tea.KeyPressMsg, state core.AppState) (core.AppState, tea.Cmd) {
	n := len(state.Stashes)

	switch {
	// File cycling in diff pane
	case msg.Text == "h":
		p.cycleFile(-1)
		return state, nil
	case msg.Text == "l":
		p.cycleFile(1)
		return state, nil

	// Diff viewport page scroll
	case msg.Text == "d" && msg.Mod == tea.ModCtrl:
		p.viewport.HalfViewDown()
		return state, nil
	case msg.Text == "u" && msg.Mod == tea.ModCtrl:
		p.viewport.HalfViewUp()
		return state, nil

	// Tab toggles back to LIST
	case msg.Code == tea.KeyTab:
		state.Mode = core.ModeList
		return state, nil

	// Enter goes to DETAIL
	case msg.Code == tea.KeyEnter:
		if n > 0 {
			state.Mode = core.ModeDetail
		}
		return state, nil

	// j/k navigate the list and reload diff for the new selection
	case msg.Text == "j":
		prevCursor := p.list.cursor
		state, _ = p.list.Update(msg, state)
		return state, p.maybeReloadDiff(prevCursor, state)
	case msg.Text == "k":
		prevCursor := p.list.cursor
		state, _ = p.list.Update(msg, state)
		return state, p.maybeReloadDiff(prevCursor, state)

	// g/G jump navigation (also reloads diff)
	case msg.Text == "g":
		prevCursor := p.list.cursor
		state, _ = p.list.Update(msg, state)
		return state, p.maybeReloadDiff(prevCursor, state)
	case msg.Text == "G":
		prevCursor := p.list.cursor
		state, _ = p.list.Update(msg, state)
		return state, p.maybeReloadDiff(prevCursor, state)

	// All LIST CRUD actions are still available
	case msg.Text == "a", msg.Text == "p",
		msg.Text == "d" && msg.Mod == 0,
		msg.Text == "n", msg.Text == "r",
		msg.Text == "e", msg.Text == "i",
		msg.Text == "b":
		return p.list.Update(msg, state)
	}

	return state, nil
}

// maybeReloadDiff fetches the diff if the cursor moved to a different stash.
func (p *PreviewScreen) maybeReloadDiff(prevCursor int, state core.AppState) tea.Cmd {
	if p.list.cursor == prevCursor || len(state.Stashes) == 0 {
		return nil
	}
	stash := state.Stashes[p.list.cursor]
	if stash.SHA == p.currentSHA {
		return nil
	}
	p.loading = true
	p.currentSHA = stash.SHA
	return loadDiffCmd(p.cache, stash.SHA)
}

// recalcSplit adjusts the viewport dimensions based on the split ratio.
func (p *PreviewScreen) recalcSplit() {
	listH := int(float64(p.height) * listHeightRatio)
	viewH := p.height - listH - 1 // 1 for the divider line
	p.list.width = p.width
	p.list.height = listH
	p.viewport.Width = p.width
	p.viewport.Height = max(viewH, 1)
}
```

### Step 5: Implement the View method

```go
// View renders the PREVIEW mode: compressed list (top) + divider + diff viewport (bottom).
func (p *PreviewScreen) View(state core.AppState, width, height int) string {
	p.width = width
	p.height = height
	p.recalcSplit()

	// Top pane: compressed stash list.
	listH := int(float64(height) * listHeightRatio)
	listView := p.list.View(state, width, listH)

	// Divider with file progress indicator.
	dividerText := p.fileProgressIndicator()
	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D4A050")). // accent.gold
		Width(width)
	divider := dividerStyle.Render(
		fmt.Sprintf("───── %s %s", dividerText, strings.Repeat("─", max(0, width-len(dividerText)-7))))

	// Bottom pane: diff viewport (or loading indicator).
	var diffView string
	if p.loading {
		diffView = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")).
			Width(width).
			Render("Loading diff...")
	} else {
		diffView = p.viewport.View()
	}

	return lipgloss.JoinVertical(lipgloss.Top, listView, divider, diffView)
}

// EnsureDiffLoaded triggers a diff load for the currently selected stash if not already loaded.
// Called when entering PREVIEW mode.
func (p *PreviewScreen) EnsureDiffLoaded(state core.AppState) tea.Cmd {
	if len(state.Stashes) == 0 {
		return nil
	}
	stash := state.Stashes[p.list.cursor]
	if stash.SHA == p.currentSHA {
		return nil
	}
	p.loading = true
	p.currentSHA = stash.SHA
	return loadDiffCmd(p.cache, stash.SHA)
}
```

### Step 6: Write comprehensive unit tests

```go
// internal/ui/screens/preview_test.go
package screens

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
)

// mockCache implements git.StashCache for testing.
type mockCache struct {
	diffs map[string]string
}

func (m *mockCache) List(ctx context.Context) ([]core.Stash, error) {
	return nil, nil
}

func (m *mockCache) Diff(ctx context.Context, sha string) (string, error) {
	if d, ok := m.diffs[sha]; ok {
		return d, nil
	}
	return "", fmt.Errorf("diff not found for SHA %s", sha)
}

func (m *mockCache) Invalidate() {}

// sampleDiff returns a unified diff with multiple files for testing.
func sampleDiff() string {
	return `diff --git a/src/auth/token.go b/src/auth/token.go
index abc1234..def5678 100644
--- a/src/auth/token.go
+++ b/src/auth/token.go
@@ -42,7 +42,12 @@ func RefreshToken(ctx context.Context) error {
   if token.IsExpired() {
-    return nil, ErrExpired
+    newToken, err := provider.Refresh(token)
+    if err != nil {
+      return nil, fmt.Errorf("refresh: %w", err)
+    }
diff --git a/src/auth/config.go b/src/auth/config.go
index aaa1111..bbb2222 100644
--- a/src/auth/config.go
+++ b/src/auth/config.go
@@ -10,3 +10,5 @@ var defaultConfig = Config{
   MaxRetries: 3,
+  Timeout:    30 * time.Second,
+  BackoffMax: 5 * time.Minute,
diff --git a/pkg/ratelimit/limiter.go b/pkg/ratelimit/limiter.go
index ccc3333..ddd4444 100644
--- a/pkg/ratelimit/limiter.go
+++ b/pkg/ratelimit/limiter.go
@@ -15,2 +15,4 @@ func NewLimiter(rate int) *Limiter {
   return &Limiter{rate: rate}
+  // TODO: add burst config
`
}

func TestParseDiffFiles(t *testing.T) {
	sections := parseDiffFiles(sampleDiff())

	if len(sections) != 3 {
		t.Fatalf("expected 3 file sections, got %d", len(sections))
	}

	wantFiles := []string{"src/auth/token.go", "src/auth/config.go", "pkg/ratelimit/limiter.go"}
	for i, want := range wantFiles {
		if sections[i].Filename != want {
			t.Errorf("section[%d].Filename = %q, want %q", i, sections[i].Filename, want)
		}
	}

	// Each section should contain diff content.
	for i, s := range sections {
		if !strings.Contains(s.Content, "diff --git") {
			t.Errorf("section[%d] should contain 'diff --git'", i)
		}
	}
}

func TestParseDiffFiles_Empty(t *testing.T) {
	sections := parseDiffFiles("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty diff, got %d", len(sections))
	}
}

func TestParseDiffFiles_SingleFile(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1,2 @@
 package main
+// added comment
`
	sections := parseDiffFiles(diff)
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	if sections[0].Filename != "main.go" {
		t.Errorf("filename = %q, want %q", sections[0].Filename, "main.go")
	}
}

func TestExtractFilename(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"diff --git a/foo.go b/foo.go", "foo.go"},
		{"diff --git a/src/bar/baz.go b/src/bar/baz.go", "src/bar/baz.go"},
		{"diff --git a/a b/b", "b"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := extractFilename(tt.line)
			if got != tt.want {
				t.Errorf("extractFilename(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestPreviewScreen_FileCycling(t *testing.T) {
	cache := &mockCache{
		diffs: map[string]string{"abc0000": sampleDiff()},
	}
	list := NewListScreen()
	list.width = 120
	list.height = 10
	ps := NewPreviewScreen(list, cache)

	// Simulate diff loaded.
	ps.diffFiles = parseDiffFiles(sampleDiff())
	ps.fileIndex = 0

	tests := []struct {
		name      string
		delta     int
		wantIndex int
		wantFile  string
	}{
		{"next file", 1, 1, "src/auth/config.go"},
		{"next file again", 1, 2, "pkg/ratelimit/limiter.go"},
		{"wrap around forward", 1, 0, "src/auth/token.go"},
		{"wrap around backward", -1, 2, "pkg/ratelimit/limiter.go"},
		{"prev file", -1, 1, "src/auth/config.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps.cycleFile(tt.delta)
			if ps.fileIndex != tt.wantIndex {
				t.Errorf("fileIndex = %d, want %d", ps.fileIndex, tt.wantIndex)
			}
			if ps.diffFiles[ps.fileIndex].Filename != tt.wantFile {
				t.Errorf("filename = %q, want %q",
					ps.diffFiles[ps.fileIndex].Filename, tt.wantFile)
			}
		})
	}
}

func TestPreviewScreen_FileCycling_Empty(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.diffFiles = nil

	// Should not panic on empty diff files.
	ps.cycleFile(1)
	ps.cycleFile(-1)

	if ps.fileIndex != 0 {
		t.Errorf("fileIndex = %d, want 0", ps.fileIndex)
	}
}

func TestPreviewScreen_FileProgressIndicator(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.diffFiles = parseDiffFiles(sampleDiff())
	ps.fileIndex = 0

	got := ps.fileProgressIndicator()
	if got != "src/auth/token.go (1/3)" {
		t.Errorf("indicator = %q, want %q", got, "src/auth/token.go (1/3)")
	}

	ps.fileIndex = 2
	got = ps.fileProgressIndicator()
	if got != "pkg/ratelimit/limiter.go (3/3)" {
		t.Errorf("indicator = %q, want %q", got, "pkg/ratelimit/limiter.go (3/3)")
	}
}

func TestPreviewScreen_FileProgressIndicator_Empty(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.diffFiles = nil

	got := ps.fileProgressIndicator()
	if got != "" {
		t.Errorf("indicator = %q, want empty", got)
	}
}

func TestPreviewScreen_TabTogglesBackToList(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	list.width = 120
	list.height = 10
	ps := NewPreviewScreen(list, cache)
	ps.width = 120
	ps.height = 30

	state := core.AppState{
		Stashes: makeStashes(5),
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	newState, _ := ps.Update(msg, state)

	if newState.Mode != core.ModeList {
		t.Errorf("mode = %v, want ModeList", newState.Mode)
	}
}

func TestPreviewScreen_EnterGoesToDetail(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	list.width = 120
	list.height = 10
	ps := NewPreviewScreen(list, cache)
	ps.width = 120
	ps.height = 30

	state := core.AppState{
		Stashes: makeStashes(5),
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := ps.Update(msg, state)

	if newState.Mode != core.ModeDetail {
		t.Errorf("mode = %v, want ModeDetail", newState.Mode)
	}
}

func TestPreviewScreen_EnterOnEmptyDoesNothing(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.width = 120
	ps.height = 30

	state := core.AppState{
		Stashes: nil,
		Mode:    core.ModePreview,
	}

	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := ps.Update(msg, state)

	if newState.Mode != core.ModePreview {
		t.Errorf("mode = %v, want ModePreview (no change on empty)", newState.Mode)
	}
}

func TestPreviewScreen_DiffLoadedMsg(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.currentSHA = "abc0000"
	ps.loading = true
	ps.width = 120
	ps.height = 30

	state := core.AppState{
		Stashes: makeStashes(5),
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	// Simulate diff loaded message.
	msg := DiffLoadedMsg{
		SHA:  "abc0000",
		Diff: sampleDiff(),
	}
	state, _ = ps.Update(msg, state)

	if ps.loading {
		t.Error("loading should be false after DiffLoadedMsg")
	}
	if len(ps.diffFiles) != 3 {
		t.Errorf("diffFiles count = %d, want 3", len(ps.diffFiles))
	}
	if ps.fileIndex != 0 {
		t.Errorf("fileIndex = %d, want 0 (reset on load)", ps.fileIndex)
	}
}

func TestPreviewScreen_DiffLoadedMsg_StaleResponse(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.currentSHA = "newsha"
	ps.loading = true

	state := core.AppState{
		Stashes: makeStashes(5),
		Mode:    core.ModePreview,
	}

	// Response for a different SHA — should be ignored.
	msg := DiffLoadedMsg{
		SHA:  "oldsha",
		Diff: sampleDiff(),
	}
	state, _ = ps.Update(msg, state)

	if !ps.loading {
		t.Error("loading should still be true (stale response ignored)")
	}
}

func TestPreviewScreen_DiffLoadedMsg_Error(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.currentSHA = "abc0000"
	ps.loading = true
	ps.width = 120
	ps.height = 30

	state := core.AppState{
		Stashes: makeStashes(5),
		Mode:    core.ModePreview,
	}

	msg := DiffLoadedMsg{
		SHA: "abc0000",
		Err: fmt.Errorf("git command failed"),
	}
	state, _ = ps.Update(msg, state)

	if ps.loading {
		t.Error("loading should be false after error")
	}
}

func TestPreviewScreen_SplitMath(t *testing.T) {
	tests := []struct {
		totalHeight   int
		wantListH     int
		wantViewportH int
	}{
		{totalHeight: 24, wantListH: 9, wantViewportH: 14},  // 24*0.4=9.6→9, 24-9-1=14
		{totalHeight: 40, wantListH: 16, wantViewportH: 23}, // 40*0.4=16, 40-16-1=23
		{totalHeight: 10, wantListH: 4, wantViewportH: 5},   // 10*0.4=4, 10-4-1=5
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("height=%d", tt.totalHeight), func(t *testing.T) {
			cache := &mockCache{diffs: map[string]string{}}
			list := NewListScreen()
			ps := NewPreviewScreen(list, cache)
			ps.width = 120
			ps.height = tt.totalHeight
			ps.recalcSplit()

			if ps.list.height != tt.wantListH {
				t.Errorf("list height = %d, want %d", ps.list.height, tt.wantListH)
			}
			if ps.viewport.Height != tt.wantViewportH {
				t.Errorf("viewport height = %d, want %d", ps.viewport.Height, tt.wantViewportH)
			}
		})
	}
}

func TestPreviewScreen_CursorMoveReloadsDiff(t *testing.T) {
	cache := &mockCache{
		diffs: map[string]string{
			"abc0000": sampleDiff(),
			"abc0001": "diff --git a/other.go b/other.go\n--- a/other.go\n+++ b/other.go\n",
		},
	}
	list := NewListScreen()
	list.width = 120
	list.height = 10
	ps := NewPreviewScreen(list, cache)
	ps.width = 120
	ps.height = 30
	ps.currentSHA = "abc0000"

	stashes := makeStashes(3)
	stashes[0].SHA = "abc0000"
	stashes[1].SHA = "abc0001"
	stashes[2].SHA = "abc0002"

	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	// Move cursor down — should trigger diff reload.
	msg := tea.KeyPressMsg{Text: "j"}
	_, cmd := ps.Update(msg, state)

	if cmd == nil {
		t.Error("expected a tea.Cmd for diff reload on cursor move")
	}
	if ps.currentSHA != "abc0001" {
		t.Errorf("currentSHA = %q, want %q", ps.currentSHA, "abc0001")
	}
	if !ps.loading {
		t.Error("loading should be true while diff is being fetched")
	}
}

func TestPreviewScreen_ViewShowsLoadingIndicator(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.loading = true

	state := core.AppState{
		Stashes: makeStashes(3),
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	view := ps.View(state, 120, 30)
	if !strings.Contains(view, "Loading diff") {
		t.Error("view should contain 'Loading diff' when loading")
	}
}

func TestPreviewScreen_ListActionsStillWork(t *testing.T) {
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	list.width = 120
	list.height = 10
	ps := NewPreviewScreen(list, cache)
	ps.width = 120
	ps.height = 30

	state := core.AppState{
		Stashes: makeStashes(5),
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	// 'a' should dispatch apply command even in PREVIEW mode.
	msg := tea.KeyPressMsg{Text: "a"}
	_, cmd := ps.Update(msg, state)
	if cmd == nil {
		t.Error("'a' in PREVIEW mode should dispatch apply command")
	}

	// 'n' should switch to new stash mode.
	msg = tea.KeyPressMsg{Text: "n"}
	newState, _ := ps.Update(msg, state)
	if newState.Mode != core.ModeNewStash {
		t.Errorf("'n' should switch to ModeNewStash, got %v", newState.Mode)
	}
}
```

### Step 7: Write integration test with real git repo

```go
func TestPreviewScreen_Integration_RealDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return string(out)
	}

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(content), 0o644)
	}

	// Create repo with a stash containing 3 changed files.
	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")
	writeFile("README.md", "# test")
	writeFile("src/main.go", "package main\n")
	writeFile("src/util.go", "package main\n")
	runGit("add", ".")
	runGit("commit", "-m", "initial")

	// Modify 3 files and stash.
	writeFile("README.md", "# test\n\nUpdated readme.\n")
	writeFile("src/main.go", "package main\n\nfunc main() {}\n")
	writeFile("src/util.go", "package main\n\nfunc helper() {}\n")
	runGit("add", ".")
	runGit("stash", "push", "-m", "multi-file change")

	// Get the stash diff directly from git to verify our parser.
	rawDiff := runGit("stash", "show", "-p", "stash@{0}")

	sections := parseDiffFiles(rawDiff)
	if len(sections) != 3 {
		t.Fatalf("expected 3 file sections from real diff, got %d", len(sections))
	}

	// Verify file cycling works with real diff content.
	cache := &mockCache{diffs: map[string]string{}}
	list := NewListScreen()
	ps := NewPreviewScreen(list, cache)
	ps.diffFiles = sections
	ps.fileIndex = 0

	// Cycle through all files and back.
	for i := range 3 {
		if ps.diffFiles[ps.fileIndex].Filename == "" {
			t.Errorf("file %d has empty filename", i)
		}
		ps.cycleFile(1)
	}
	// Should have wrapped back to 0.
	if ps.fileIndex != 0 {
		t.Errorf("after cycling 3 times through 3 files, index = %d, want 0", ps.fileIndex)
	}
}
```

### Step 8: Verify build and tests

```bash
gofmt -w internal/ui/screens/preview.go internal/ui/screens/preview_test.go
go vet ./internal/ui/screens/...
go test -v -race ./internal/ui/screens/...
make ci
```

## Verification

### Functional
```bash
# PREVIEW screen compiles
go build ./internal/ui/screens/...

# All tests pass
go test -v -race -count=1 ./internal/ui/screens/...

# Diff parsing tests
go test -v -run TestParseDiffFiles ./internal/ui/screens/...

# File cycling tests
go test -v -run TestPreviewScreen_FileCycling ./internal/ui/screens/...

# Mode toggle tests
go test -v -run TestPreviewScreen_Tab ./internal/ui/screens/...
go test -v -run TestPreviewScreen_Enter ./internal/ui/screens/...

# Split math tests
go test -v -run TestPreviewScreen_SplitMath ./internal/ui/screens/...

# Diff loading tests
go test -v -run TestPreviewScreen_DiffLoadedMsg ./internal/ui/screens/...

# Cursor movement reloads diff
go test -v -run TestPreviewScreen_CursorMoveReloadsDiff ./internal/ui/screens/...

# Integration test with real git diff
go test -v -run TestPreviewScreen_Integration_RealDiff ./internal/ui/screens/...
```

### CI Pipeline
```bash
make ci
```

### TUI Visual Verification (iterm2-driver)
```bash
# Build binary
make build

# Create test repo with multi-file stash
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
git init && git config user.email "t@t" && git config user.name "T"
echo "base" > a.go && echo "base" > b.go && echo "base" > c.go
git add . && git commit -m "init"
echo "modified" > a.go && echo "modified" > b.go && echo "modified" > c.go
git add . && git stash push -m "3 file change"

# Launch nidhi, press Tab to enter PREVIEW mode
# Verify: list compressed in top ~40%, diff in bottom ~60%
# Verify: file progress indicator shows "a.go (1/3)"
# Press l → verify "b.go (2/3)", press l → "c.go (3/3)", press l → wraps to "a.go (1/3)"
```

## Completion Criteria
1. `internal/ui/screens/preview.go` implements `PreviewScreen` with split layout
2. Top pane (~40%): compressed stash list via embedded `ListScreen`
3. Bottom pane (~60%): `viewport.Model` showing diff content
4. Divider line with file progress indicator (e.g., "src/auth/token.go (1/3)")
5. `h`/`l` cycles through files in the diff (wraps around)
6. Cursor movement in list triggers async diff reload via `StashCache.Diff(sha)`
7. Loading indicator shown while diff is being fetched
8. Stale diff responses (for a different SHA) are discarded
9. `Tab` toggles back to LIST mode, `Enter` goes to DETAIL mode
10. All LIST actions (`a`/`p`/`d`/`n`/`r`/`e`/`i`/`b`) still work in PREVIEW mode
11. `^d`/`^u` page scroll works in the diff viewport
12. All unit tests pass: diff parsing (3 cases), file cycling (5 cases), file progress indicator, Tab/Enter toggle, diff loading (3 cases), split math (3 cases), cursor reload, loading indicator, LIST actions
13. Integration test: creates stash with 3 files in temp repo, verifies diff parsing and file cycling
14. `make ci` passes

## Commit
```
feat: implement PREVIEW screen with split layout and diff viewport

Add internal/ui/screens/preview.go — PREVIEW mode (PRD §10 Screen 2)
with split layout: compressed stash list (~40% top) and diff viewport
(~60% bottom). Supports h/l file cycling with progress indicator,
async diff loading from StashCache, cursor-driven diff updates,
and ^d/^u viewport scroll. All LIST actions remain available.
Comprehensive tests including real git diff parsing integration test.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.1 FR-03.2, 10 (Screen 2), 11.2 (PREVIEW keymap)
4. Read tasks 010, 009, 007, 004 to understand dependencies
5. Implement `preview.go` following execution steps 1-5
6. Implement `preview_test.go` following execution steps 6-7
7. Run `go vet`, `go test -v -race`, `make ci`
8. If iterm2-driver is available, take screenshot of PREVIEW mode and compare with mockup
9. Update this file (Status: DONE) + `docs/PROGRESS.md`
10. Commit with the message above
