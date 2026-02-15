package screens

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// mockCache implements plugin.StashCache for testing.
type mockCache struct {
	diffs map[string]string
}

func (m *mockCache) List(_ context.Context) ([]plugin.Stash, error) {
	return nil, nil
}

func (m *mockCache) Diff(_ context.Context, sha string) (string, error) {
	if d, ok := m.diffs[sha]; ok {
		return d, nil
	}
	return "", fmt.Errorf("diff not found for SHA %s", sha)
}

func (m *mockCache) Invalidate() {}

var _ plugin.StashCache = (*mockCache)(nil)

func testDiff() string {
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

func newTestPreview() (*PreviewScreen, core.AppState) {
	th := theme.NewAgni()
	cache := &mockCache{diffs: map[string]string{
		"sha0": testDiff(),
	}}
	list := NewListScreen(th)
	list.SetSize(120, 10)
	ps := NewPreviewScreen(list, cache, th)
	ps.width = 120
	ps.height = 28

	state := core.AppState{
		Stashes: makeStashes(5),
		Cursor:  0,
		Mode:    core.ModePreview,
	}
	return ps, state
}

func TestParseDiffFiles(t *testing.T) {
	sections := parseDiffFiles(testDiff())

	if len(sections) != 3 {
		t.Fatalf("expected 3 file sections, got %d", len(sections))
	}

	wantFiles := []string{"src/auth/token.go", "src/auth/config.go", "pkg/ratelimit/limiter.go"}
	for i, want := range wantFiles {
		if sections[i].Filename != want {
			t.Errorf("section[%d].Filename = %q, want %q", i, sections[i].Filename, want)
		}
	}

	for i, s := range sections {
		if !strings.Contains(s.Content, "diff --git") {
			t.Errorf("section[%d] should contain 'diff --git'", i)
		}
	}
}

func TestParseDiffFiles_Empty(t *testing.T) {
	sections := parseDiffFiles("")
	if len(sections) != 0 {
		t.Errorf("expected 0 sections, got %d", len(sections))
	}
}

func TestParseDiffFiles_SingleFile(t *testing.T) {
	diff := "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1,2 @@\n package main\n+// added\n"
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
				t.Errorf("extractFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreviewScreen_FileCycling(t *testing.T) {
	ps, _ := newTestPreview()
	ps.diffFiles = parseDiffFiles(testDiff())
	ps.fileIndex = 0

	tests := []struct {
		name      string
		delta     int
		wantIndex int
		wantFile  string
	}{
		{"next file", 1, 1, "src/auth/config.go"},
		{"next file again", 1, 2, "pkg/ratelimit/limiter.go"},
		{"wrap forward", 1, 0, "src/auth/token.go"},
		{"wrap backward", -1, 2, "pkg/ratelimit/limiter.go"},
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
	ps, _ := newTestPreview()
	ps.diffFiles = nil

	// Should not panic.
	ps.cycleFile(1)
	ps.cycleFile(-1)
	if ps.fileIndex != 0 {
		t.Errorf("fileIndex = %d, want 0", ps.fileIndex)
	}
}

func TestPreviewScreen_FileProgressIndicator(t *testing.T) {
	ps, _ := newTestPreview()
	ps.diffFiles = parseDiffFiles(testDiff())

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
	ps, _ := newTestPreview()
	ps.diffFiles = nil
	if got := ps.fileProgressIndicator(); got != "" {
		t.Errorf("indicator = %q, want empty", got)
	}
}

func TestPreviewScreen_TabTogglesBackToList(t *testing.T) {
	ps, state := newTestPreview()
	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	newState, _ := ps.Update(msg, state)
	if newState.Mode != core.ModeList {
		t.Errorf("mode = %v, want ModeList", newState.Mode)
	}
}

func TestPreviewScreen_EnterGoesToDetail(t *testing.T) {
	ps, state := newTestPreview()
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := ps.Update(msg, state)
	if newState.Mode != core.ModeDetail {
		t.Errorf("mode = %v, want ModeDetail", newState.Mode)
	}
}

func TestPreviewScreen_EnterOnEmptyDoesNothing(t *testing.T) {
	ps, state := newTestPreview()
	state.Stashes = nil
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := ps.Update(msg, state)
	if newState.Mode != core.ModePreview {
		t.Errorf("mode = %v, want ModePreview (no change on empty)", newState.Mode)
	}
}

func TestPreviewScreen_DiffLoadedMsg(t *testing.T) {
	ps, state := newTestPreview()
	ps.currentSHA = "sha0"
	ps.loading = true

	msg := DiffLoadedMsg{SHA: "sha0", Diff: testDiff()}
	_, _ = ps.Update(msg, state)

	if ps.loading {
		t.Error("loading should be false after DiffLoadedMsg")
	}
	if len(ps.diffFiles) != 3 {
		t.Errorf("diffFiles count = %d, want 3", len(ps.diffFiles))
	}
	if ps.fileIndex != 0 {
		t.Errorf("fileIndex = %d, want 0", ps.fileIndex)
	}
}

func TestPreviewScreen_DiffLoadedMsg_StaleResponse(t *testing.T) {
	ps, state := newTestPreview()
	ps.currentSHA = "newsha"
	ps.loading = true

	msg := DiffLoadedMsg{SHA: "oldsha", Diff: testDiff()}
	_, _ = ps.Update(msg, state)

	if !ps.loading {
		t.Error("loading should still be true (stale response ignored)")
	}
}

func TestPreviewScreen_DiffLoadedMsg_Error(t *testing.T) {
	ps, state := newTestPreview()
	ps.currentSHA = "sha0"
	ps.loading = true

	msg := DiffLoadedMsg{SHA: "sha0", Err: fmt.Errorf("git command failed")}
	_, _ = ps.Update(msg, state)

	if ps.loading {
		t.Error("loading should be false after error")
	}
}

func TestPreviewScreen_SplitMath(t *testing.T) {
	tests := []struct {
		totalHeight int
		wantListH   int
		wantViewH   int
	}{
		{24, 9, 14},  // 24*0.4=9.6→9, 24-9-1=14
		{40, 16, 23}, // 40*0.4=16, 40-16-1=23
		{10, 4, 5},   // 10*0.4=4, 10-4-1=5
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("height=%d", tt.totalHeight), func(t *testing.T) {
			ps, _ := newTestPreview()
			ps.width = 120
			ps.height = tt.totalHeight
			ps.recalcSplit()

			if ps.list.height != tt.wantListH {
				t.Errorf("list height = %d, want %d", ps.list.height, tt.wantListH)
			}
		})
	}
}

func TestPreviewScreen_CursorMoveReloadsDiff(t *testing.T) {
	ps, state := newTestPreview()
	ps.currentSHA = state.Stashes[0].SHA

	// Move cursor down — should trigger diff reload.
	msg := tea.KeyPressMsg{Text: "j"}
	_, cmd := ps.Update(msg, state)

	if cmd == nil {
		t.Error("expected a tea.Cmd for diff reload on cursor move")
	}
	if ps.currentSHA != state.Stashes[1].SHA {
		t.Errorf("currentSHA = %q, want %q", ps.currentSHA, state.Stashes[1].SHA)
	}
	if !ps.loading {
		t.Error("loading should be true while diff is being fetched")
	}
}

func TestPreviewScreen_CursorNoReloadIfSameStash(t *testing.T) {
	ps, state := newTestPreview()
	state.Stashes = makeStashes(1)
	ps.currentSHA = state.Stashes[0].SHA

	msg := tea.KeyPressMsg{Text: "j"}
	_, cmd := ps.Update(msg, state)

	if cmd != nil {
		t.Error("should not reload diff if cursor didn't actually move")
	}
}

func TestPreviewScreen_ViewShowsLoading(t *testing.T) {
	ps, state := newTestPreview()
	ps.loading = true

	view := ps.View(state, 120, 28)
	if !strings.Contains(view, "Loading diff") {
		t.Error("view should contain 'Loading diff' when loading")
	}
}

func TestPreviewScreen_ViewShowsDivider(t *testing.T) {
	ps, state := newTestPreview()
	ps.diffFiles = parseDiffFiles(testDiff())
	ps.fileIndex = 0

	view := ps.View(state, 120, 28)
	if !strings.Contains(view, "src/auth/token.go") {
		t.Error("divider should contain current file name")
	}
	if !strings.Contains(view, "(1/3)") {
		t.Error("divider should contain file progress")
	}
}

func TestPreviewScreen_ListActionsWork(t *testing.T) {
	ps, state := newTestPreview()

	// 'a' should dispatch apply command in PREVIEW mode.
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

func TestPreviewScreen_HLKeysOnEmptyDiff(t *testing.T) {
	ps, state := newTestPreview()
	ps.diffFiles = nil

	// h/l on empty diff should not panic.
	_, _ = ps.Update(tea.KeyPressMsg{Text: "h"}, state)
	_, _ = ps.Update(tea.KeyPressMsg{Text: "l"}, state)
}

func TestPreviewScreen_WindowResize(t *testing.T) {
	ps, state := newTestPreview()
	msg := tea.WindowSizeMsg{Width: 80, Height: 24}
	_, _ = ps.Update(msg, state)

	if ps.width != 80 {
		t.Errorf("width = %d, want 80", ps.width)
	}
}
