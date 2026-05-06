package search_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/search"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── Test doubles ───────────────────────────────────────────

type testTheme struct{ theme.Theme }

func newTestTheme() *testTheme {
	return &testTheme{Theme: theme.NewAgni()}
}

type noopCache struct{}

func (c *noopCache) List(_ context.Context) ([]plugin.Stash, error) { return nil, nil }
func (c *noopCache) Diff(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *noopCache) Invalidate() {}

var _ plugin.StashCache = (*noopCache)(nil)

// diffCache returns a fixed diff for any SHA.
type diffCache struct {
	diffs map[string]string
}

func (c *diffCache) List(_ context.Context) ([]plugin.Stash, error) { return nil, nil }
func (c *diffCache) Diff(_ context.Context, sha string) (string, error) {
	if d, ok := c.diffs[sha]; ok {
		return d, nil
	}
	return "", nil
}
func (c *diffCache) Invalidate() {}

var _ plugin.StashCache = (*diffCache)(nil)

// ─── Index Unit Tests ───────────────────────────────────────

func TestFuzzyMatchScoring(t *testing.T) {
	idx := search.NewIndex()

	entries := []search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "Fix auth token refresh", Scope: search.ScopeMessages},
		{StashIndex: 1, StashSHA: "bbb", Message: "WIP: new dashboard layout", Scope: search.ScopeMessages},
		{StashIndex: 2, StashSHA: "ccc", Message: "Token rotation implementation", Scope: search.ScopeMessages},
		{StashIndex: 0, StashSHA: "aaa", FileName: "src/auth/token.go", Scope: search.ScopeFiles},
		{StashIndex: 1, StashSHA: "bbb", FileName: "src/ui/dashboard.go", Scope: search.ScopeFiles},
	}
	idx.AddEntriesForTest(entries)

	results := idx.Search("token", search.ScopeAll)
	if len(results) == 0 {
		t.Fatal("expected results for 'token', got none")
	}

	// "Fix auth token refresh" and "Token rotation" should both match.
	foundStash0 := false
	foundStash2 := false
	for _, r := range results {
		if r.StashIndex == 0 {
			foundStash0 = true
		}
		if r.StashIndex == 2 {
			foundStash2 = true
		}
	}
	if !foundStash0 {
		t.Error("expected stash@{0} ('Fix auth token refresh') in results")
	}
	if !foundStash2 {
		t.Error("expected stash@{2} ('Token rotation implementation') in results")
	}
}

func TestScopeFilterRestrictsSearch(t *testing.T) {
	idx := search.NewIndex()
	entries := []search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "auth changes", Scope: search.ScopeMessages},
		{StashIndex: 0, StashSHA: "aaa", FileName: "auth.go", Scope: search.ScopeFiles},
		{StashIndex: 0, StashSHA: "aaa", DiffLine: "func AuthMiddleware()", Scope: search.ScopeDiffs},
		{StashIndex: 0, StashSHA: "aaa", Branch: "feature/auth", Scope: search.ScopeBranch},
	}
	idx.AddEntriesForTest(entries)

	tests := []struct {
		scope    search.Scope
		expected search.Scope
	}{
		{search.ScopeMessages, search.ScopeMessages},
		{search.ScopeFiles, search.ScopeFiles},
		{search.ScopeDiffs, search.ScopeDiffs},
		{search.ScopeBranch, search.ScopeBranch},
	}

	for _, tt := range tests {
		results := idx.Search("auth", tt.scope)
		if len(results) == 0 {
			t.Errorf("expected results for scope %d, got none", tt.scope)
			continue
		}
		for _, r := range results {
			if r.MatchScope != tt.expected {
				t.Errorf("scope %d: expected MatchScope=%d, got %d", tt.scope, tt.expected, r.MatchScope)
			}
		}
	}
}

func TestScopeAllReturnsAllScopes(t *testing.T) {
	idx := search.NewIndex()
	entries := []search.IndexEntry{
		{StashIndex: 0, StashSHA: "a1", Message: "auth changes", Scope: search.ScopeMessages},
		{StashIndex: 1, StashSHA: "b1", FileName: "auth.go", Scope: search.ScopeFiles},
		{StashIndex: 2, StashSHA: "c1", DiffLine: "func AuthHandler()", Scope: search.ScopeDiffs},
		{StashIndex: 3, StashSHA: "d1", Branch: "feature/auth", Scope: search.ScopeBranch},
	}
	idx.AddEntriesForTest(entries)

	results := idx.Search("auth", search.ScopeAll)
	if len(results) < 3 {
		t.Errorf("expected at least 3 results for ScopeAll 'auth', got %d", len(results))
	}
}

func TestEmptyQueryReturnsNoResults(t *testing.T) {
	idx := search.NewIndex()
	idx.AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "test", Scope: search.ScopeMessages},
	})
	results := idx.Search("", search.ScopeAll)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty query, got %d", len(results))
	}
}

func TestEmptyIndexReturnsNoResults(t *testing.T) {
	idx := search.NewIndex()
	results := idx.Search("anything", search.ScopeAll)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty index, got %d", len(results))
	}
}

func TestDeduplicatesByStashAndScope(t *testing.T) {
	idx := search.NewIndex()
	// Multiple diff lines from the same stash should deduplicate.
	entries := []search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", DiffLine: "func authenticate()", FileName: "auth.go", LineNum: 10, Scope: search.ScopeDiffs},
		{StashIndex: 0, StashSHA: "aaa", DiffLine: "var authToken string", FileName: "auth.go", LineNum: 20, Scope: search.ScopeDiffs},
		{StashIndex: 0, StashSHA: "aaa", DiffLine: "type authConfig struct", FileName: "config.go", LineNum: 5, Scope: search.ScopeDiffs},
	}
	idx.AddEntriesForTest(entries)

	results := idx.Search("auth", search.ScopeDiffs)
	// All three entries are from stash "aaa" with ScopeDiffs → should collapse to 1 result.
	if len(results) != 1 {
		t.Errorf("expected 1 deduplicated result, got %d", len(results))
	}
}

func TestLazyVsEagerIndexing(t *testing.T) {
	idx := search.NewIndex()

	if idx.IsReady() {
		t.Error("newly created index should not be ready")
	}
	if idx.HasPartialResults() {
		t.Error("newly created index should not have partial results")
	}

	idx.AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "test", Scope: search.ScopeMessages},
	})
	if !idx.HasPartialResults() {
		t.Error("index with entries should have partial results")
	}
	if idx.IsReady() {
		t.Error("index with entries but not marked ready should not be ready")
	}

	idx.MarkReadyForTest()
	if !idx.IsReady() {
		t.Error("index marked ready should be ready")
	}
}

func TestIndexReset(t *testing.T) {
	idx := search.NewIndex()
	idx.AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "test", Scope: search.ScopeMessages},
	})
	idx.MarkReadyForTest()

	if idx.EntryCount() == 0 {
		t.Fatal("expected entries before reset")
	}

	idx.Reset()

	if idx.IsReady() {
		t.Error("index should not be ready after reset")
	}
	if idx.EntryCount() != 0 {
		t.Errorf("expected 0 entries after reset, got %d", idx.EntryCount())
	}
}

func TestEntryCount(t *testing.T) {
	idx := search.NewIndex()
	if idx.EntryCount() != 0 {
		t.Errorf("expected 0, got %d", idx.EntryCount())
	}

	idx.AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "a", Message: "one", Scope: search.ScopeMessages},
		{StashIndex: 1, StashSHA: "b", Message: "two", Scope: search.ScopeMessages},
	})
	if idx.EntryCount() != 2 {
		t.Errorf("expected 2, got %d", idx.EntryCount())
	}
}

// ─── Diff Parser Tests ──────────────────────────────────────

func TestParseDiffForIndex_BasicDiff(t *testing.T) {
	diff := `diff --git a/src/auth/token.go b/src/auth/token.go
index abc1234..def5678 100644
--- a/src/auth/token.go
+++ b/src/auth/token.go
@@ -42,7 +42,12 @@ func RefreshToken(ctx context.Context) error {
     if token.IsExpired() {
-        return nil, ErrExpired
+        newToken, err := provider.Refresh(token)
+        if err != nil {
+            return nil, fmt.Errorf("refresh: %w", err)
+        }
diff --git a/src/auth/config.go b/src/auth/config.go
index 111222..333444 100644
--- a/src/auth/config.go
+++ b/src/auth/config.go
@@ -10,3 +10,4 @@ var defaultConfig = Config{
     MaxRetries: 5,
+    Timeout:    30 * time.Second,
`
	stash := plugin.Stash{Index: 0, SHA: "abc123", Message: "Fix auth", Branch: "main"}
	fileEntries, diffEntries := search.ParseDiffForIndex(stash, diff)

	if len(fileEntries) != 2 {
		t.Fatalf("expected 2 file entries, got %d", len(fileEntries))
	}
	if fileEntries[0].FileName != "src/auth/token.go" {
		t.Errorf("expected first file 'src/auth/token.go', got %q", fileEntries[0].FileName)
	}
	if fileEntries[1].FileName != "src/auth/config.go" {
		t.Errorf("expected second file 'src/auth/config.go', got %q", fileEntries[1].FileName)
	}

	// Verify all file entries have correct scope.
	for _, e := range fileEntries {
		if e.Scope != search.ScopeFiles {
			t.Errorf("expected ScopeFiles, got %d", e.Scope)
		}
		if e.StashSHA != "abc123" {
			t.Errorf("expected SHA 'abc123', got %q", e.StashSHA)
		}
	}

	// Verify diff entries exist and contain expected content.
	if len(diffEntries) == 0 {
		t.Fatal("expected diff entries, got none")
	}
	foundRefresh := false
	for _, e := range diffEntries {
		if strings.Contains(e.DiffLine, "provider.Refresh") {
			foundRefresh = true
			if e.FileName != "src/auth/token.go" {
				t.Errorf("expected file 'src/auth/token.go' for Refresh line, got %q", e.FileName)
			}
			break
		}
	}
	if !foundRefresh {
		t.Error("expected to find 'provider.Refresh' in diff entries")
	}

	// Verify diff entries have correct scope.
	for _, e := range diffEntries {
		if e.Scope != search.ScopeDiffs {
			t.Errorf("expected ScopeDiffs, got %d", e.Scope)
		}
	}
}

func TestParseDiffForIndex_EmptyDiff(t *testing.T) {
	stash := plugin.Stash{Index: 0, SHA: "empty", Message: "empty"}
	fileEntries, diffEntries := search.ParseDiffForIndex(stash, "")
	if len(fileEntries) != 0 {
		t.Errorf("expected 0 file entries for empty diff, got %d", len(fileEntries))
	}
	if len(diffEntries) != 0 {
		t.Errorf("expected 0 diff entries for empty diff, got %d", len(diffEntries))
	}
}

func TestParseDiffForIndex_SkipsHeaders(t *testing.T) {
	diff := `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new.go
@@ -0,0 +1,3 @@
+package main
+
+func New() {}
`
	stash := plugin.Stash{Index: 0, SHA: "new", Message: "new file"}
	fileEntries, diffEntries := search.ParseDiffForIndex(stash, diff)

	if len(fileEntries) != 1 {
		t.Fatalf("expected 1 file entry, got %d", len(fileEntries))
	}
	if fileEntries[0].FileName != "new.go" {
		t.Errorf("expected 'new.go', got %q", fileEntries[0].FileName)
	}

	// Should have diff entries for the added lines.
	foundPackage := false
	for _, e := range diffEntries {
		if strings.Contains(e.DiffLine, "package main") {
			foundPackage = true
			break
		}
	}
	if !foundPackage {
		t.Error("expected to find 'package main' in diff entries")
	}
}

// ─── BuildIndexCmd Test ─────────────────────────────────────

func TestBuildIndexCmd(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,3 +1,5 @@
 package main
+import "fmt"
+func hello() { fmt.Println("hello") }
`
	cache := &diffCache{diffs: map[string]string{
		"sha1": diff,
		"sha2": "",
	}}

	stashes := []plugin.Stash{
		{Index: 0, SHA: "sha1", Message: "add hello", Branch: "main"},
		{Index: 1, SHA: "sha2", Message: "empty change", Branch: "dev"},
	}

	idx := search.NewIndex()
	cmd := search.BuildIndexCmd(stashes, cache, idx)

	// Execute the command.
	msg := cmd()
	readyMsg, ok := msg.(search.IndexReadyMsg)
	if !ok {
		t.Fatalf("expected IndexReadyMsg, got %T", msg)
	}

	if !idx.IsReady() {
		t.Error("index should be ready after BuildIndexCmd")
	}
	if readyMsg.EntryCount == 0 {
		t.Error("expected non-zero entry count")
	}

	// Verify we can search the built index.
	results := idx.Search("hello", search.ScopeAll)
	if len(results) == 0 {
		t.Error("expected results for 'hello' after indexing")
	}

	results = idx.Search("main", search.ScopeBranch)
	if len(results) == 0 {
		t.Error("expected results for 'main' in branch scope")
	}
}

// ─── Plugin Unit Tests ──────────────────────────────────────

func newTestPlugin(t *testing.T) *search.Plugin {
	t.Helper()
	th := newTestTheme()
	p := search.New(th)
	pctx := plugin.PluginContext{
		Git:    nil,
		Cache:  &noopCache{},
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init search plugin: %v", err)
	}
	return p
}

func TestPlugin_ID(t *testing.T) {
	p := newTestPlugin(t)
	if p.ID() != "search" {
		t.Errorf("expected ID 'search', got %q", p.ID())
	}
	if p.Name() != "Deep Fuzzy Search" {
		t.Errorf("expected Name 'Deep Fuzzy Search', got %q", p.Name())
	}
}

func TestPlugin_KeyBindings(t *testing.T) {
	p := newTestPlugin(t)
	bindings := p.KeyBindings()
	if len(bindings) != 1 {
		t.Fatalf("expected 1 keybinding, got %d", len(bindings))
	}
	kb := bindings[0]
	if kb.Key != "/" {
		t.Errorf("expected key '/', got %q", kb.Key)
	}
	if len(kb.Modes) != 2 {
		t.Errorf("expected 2 modes, got %d", len(kb.Modes))
	}
}

func TestPlugin_HandleKey_ActivatesSearch(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Mode: plugin.ModeList}

	key := plugin.KeyEvent{Key: "/", Mode: plugin.ModeList}
	newState, _ := p.HandleKey(key, state)

	if newState.Mode != plugin.ModeSearch {
		t.Errorf("expected ModeSearch, got %v", newState.Mode)
	}
	if !p.IsActiveForTest() {
		t.Error("expected plugin to be active after '/'")
	}
}

func TestPlugin_HandleKey_IgnoresWhenActive(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	state := plugin.AppState{Mode: plugin.ModeSearch}

	key := plugin.KeyEvent{Key: "/", Mode: plugin.ModeSearch}
	newState, _ := p.HandleKey(key, state)

	// Should not change anything since already active.
	if newState.Mode != plugin.ModeSearch {
		t.Errorf("expected mode unchanged, got %v", newState.Mode)
	}
}

func TestPlugin_Screens(t *testing.T) {
	p := newTestPlugin(t)
	screens := p.Screens()
	if len(screens) != 1 {
		t.Fatalf("expected 1 screen def, got %d", len(screens))
	}
	if screens[0].Mode != plugin.ModeSearch {
		t.Errorf("expected ModeSearch, got %v", screens[0].Mode)
	}
}

func TestPlugin_Update_EscClosesSearch(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	state := plugin.AppState{Mode: plugin.ModeSearch}

	msg := tea.KeyPressMsg{Code: tea.KeyEscape}
	newState, _ := p.Update(msg, state)

	if newState.Mode != plugin.ModeList {
		t.Errorf("expected ModeList after Esc, got %v", newState.Mode)
	}
	if p.IsActiveForTest() {
		t.Error("expected plugin inactive after Esc")
	}
}

func TestPlugin_Update_TextEscapeClosesSearch(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	state := plugin.AppState{Mode: plugin.ModeSearch}

	msg := tea.KeyPressMsg{Text: "escape"}
	newState, _ := p.Update(msg, state)

	if newState.Mode != plugin.ModeList {
		t.Errorf("expected ModeList after text escape, got %v", newState.Mode)
	}
	if p.IsActiveForTest() {
		t.Error("expected plugin inactive after text escape")
	}
}

func TestPlugin_Update_TabCyclesScope(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	state := plugin.AppState{Mode: plugin.ModeSearch}

	if p.ScopeForTest() != search.ScopeAll {
		t.Fatalf("expected initial scope ScopeAll")
	}

	msg := tea.KeyPressMsg{Code: tea.KeyTab}
	p.Update(msg, state)
	if p.ScopeForTest() != search.ScopeMessages {
		t.Errorf("expected ScopeMessages after 1 Tab, got %d", p.ScopeForTest())
	}

	p.Update(msg, state)
	if p.ScopeForTest() != search.ScopeFiles {
		t.Errorf("expected ScopeFiles after 2 Tabs, got %d", p.ScopeForTest())
	}

	p.Update(msg, state)
	if p.ScopeForTest() != search.ScopeDiffs {
		t.Errorf("expected ScopeDiffs after 3 Tabs, got %d", p.ScopeForTest())
	}

	p.Update(msg, state)
	if p.ScopeForTest() != search.ScopeBranch {
		t.Errorf("expected ScopeBranch after 4 Tabs, got %d", p.ScopeForTest())
	}

	p.Update(msg, state)
	if p.ScopeForTest() != search.ScopeAll {
		t.Errorf("expected ScopeAll after 5 Tabs (wrap), got %d", p.ScopeForTest())
	}
}

func TestPlugin_Update_TextTabCyclesScope(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	state := plugin.AppState{Mode: plugin.ModeSearch}

	msg := tea.KeyPressMsg{Text: "tab"}
	p.Update(msg, state)
	if p.ScopeForTest() != search.ScopeMessages {
		t.Errorf("expected ScopeMessages after text tab, got %d", p.ScopeForTest())
	}
}

func TestPlugin_Update_TextInput(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	state := plugin.AppState{Mode: plugin.ModeSearch}

	// Type "abc".
	for _, ch := range "abc" {
		msg := tea.KeyPressMsg{Text: string(ch)}
		p.Update(msg, state)
	}

	if p.QueryForTest() != "abc" {
		t.Errorf("expected query 'abc', got %q", p.QueryForTest())
	}

	// Backspace.
	msg := tea.KeyPressMsg{Text: "backspace"}
	p.Update(msg, state)
	if p.QueryForTest() != "ab" {
		t.Errorf("expected query 'ab' after backspace, got %q", p.QueryForTest())
	}
}

func TestPlugin_Update_ArrowKeysNavigateResults(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	p.SetResultsForTest([]search.SearchResult{
		{StashIndex: 0}, {StashIndex: 1}, {StashIndex: 2},
	})

	state := plugin.AppState{Mode: plugin.ModeSearch}

	p.Update(tea.KeyPressMsg{Code: tea.KeyDown}, state)
	if p.ResultsCursorForTest() != 1 {
		t.Errorf("expected cursor 1 after down arrow, got %d", p.ResultsCursorForTest())
	}

	p.Update(tea.KeyPressMsg{Text: "down"}, state)
	if p.ResultsCursorForTest() != 2 {
		t.Errorf("expected cursor 2 after text down, got %d", p.ResultsCursorForTest())
	}

	p.Update(tea.KeyPressMsg{Code: tea.KeyUp}, state)
	if p.ResultsCursorForTest() != 1 {
		t.Errorf("expected cursor 1 after up arrow, got %d", p.ResultsCursorForTest())
	}
}

func TestPlugin_Update_EnterJumpsToResult(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)

	// Pre-populate results.
	p.SetResultsForTest([]search.SearchResult{
		{StashIndex: 3, StashSHA: "abc", StashMessage: "test", MatchScope: search.ScopeMessages},
	})

	state := plugin.AppState{Mode: plugin.ModeSearch}
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := p.Update(msg, state)

	if newState.Cursor != 3 {
		t.Errorf("expected cursor at stash 3, got %d", newState.Cursor)
	}
	if newState.Mode != plugin.ModeList {
		t.Errorf("expected ModeList for message match, got %v", newState.Mode)
	}
}

func TestPlugin_Update_EnterOpensPreviewForDiffMatch(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)

	p.SetResultsForTest([]search.SearchResult{
		{StashIndex: 1, StashSHA: "def", StashMessage: "test", MatchScope: search.ScopeDiffs, FileName: "main.go"},
	})

	state := plugin.AppState{Mode: plugin.ModeSearch}
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := p.Update(msg, state)

	if newState.Mode != plugin.ModePreview {
		t.Errorf("expected ModePreview for diff match, got %v", newState.Mode)
	}
}

func TestPlugin_Update_EnterDoesNothingWithNoResults(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)

	state := plugin.AppState{Mode: plugin.ModeSearch}
	msg := tea.KeyPressMsg{Code: tea.KeyEnter}
	newState, _ := p.Update(msg, state)

	if newState.Mode != plugin.ModeSearch {
		t.Errorf("expected mode unchanged with no results, got %v", newState.Mode)
	}
}

func TestPlugin_Update_CtrlNP_NavigatesResults(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)

	p.SetResultsForTest([]search.SearchResult{
		{StashIndex: 0}, {StashIndex: 1}, {StashIndex: 2},
	})

	state := plugin.AppState{Mode: plugin.ModeSearch}

	// Ctrl+N moves down.
	msg := tea.KeyPressMsg{Text: "n", Mod: tea.ModCtrl}
	p.Update(msg, state)
	if p.ResultsCursorForTest() != 1 {
		t.Errorf("expected cursor 1 after Ctrl+N, got %d", p.ResultsCursorForTest())
	}

	p.Update(msg, state)
	if p.ResultsCursorForTest() != 2 {
		t.Errorf("expected cursor 2 after 2nd Ctrl+N, got %d", p.ResultsCursorForTest())
	}

	// Ctrl+N at end stays.
	p.Update(msg, state)
	if p.ResultsCursorForTest() != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", p.ResultsCursorForTest())
	}

	// Ctrl+P moves up.
	msg = tea.KeyPressMsg{Text: "p", Mod: tea.ModCtrl}
	p.Update(msg, state)
	if p.ResultsCursorForTest() != 1 {
		t.Errorf("expected cursor 1 after Ctrl+P, got %d", p.ResultsCursorForTest())
	}
}

func TestPlugin_Update_IndexReady_RerunsSearch(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	p.SetQueryForTest("hello")

	// Add some entries to the index.
	p.IndexForTest().AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "a", Message: "hello world", Scope: search.ScopeMessages},
	})
	p.IndexForTest().MarkReadyForTest()

	state := plugin.AppState{Mode: plugin.ModeSearch}
	msg := search.IndexReadyMsg{EntryCount: 1}
	p.Update(msg, state)

	results := p.ResultsForTest()
	if len(results) == 0 {
		t.Error("expected results after IndexReadyMsg with matching query")
	}
}

// ─── View Tests ─────────────────────────────────────────────

func TestPlugin_View_EmptyState(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Mode: plugin.ModeSearch}
	view := p.View(state, 120, 40)

	if !strings.Contains(view, "Search") {
		t.Error("expected 'Search' header in view")
	}
	if !strings.Contains(view, "/") {
		t.Error("expected '/' prompt in view")
	}
	if !strings.Contains(view, "Esc: close") {
		t.Error("expected footer hints in view")
	}
}

func TestPlugin_View_WithQuery(t *testing.T) {
	p := newTestPlugin(t)
	p.SetQueryForTest("token")
	p.IndexForTest().AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "a", Message: "test", Scope: search.ScopeMessages},
	})
	p.IndexForTest().MarkReadyForTest()

	state := plugin.AppState{Mode: plugin.ModeSearch}
	view := p.View(state, 120, 40)

	if !strings.Contains(view, "No matches") {
		t.Error("expected 'No matches' for non-matching query")
	}
}

func TestPlugin_View_WithResults(t *testing.T) {
	p := newTestPlugin(t)
	p.SetActiveForTest(true)
	p.SetResultsForTest([]search.SearchResult{
		{StashIndex: 0, StashSHA: "a", StashMessage: "Fix auth", MatchText: "auth", MatchScope: search.ScopeMessages},
		{StashIndex: 2, StashSHA: "c", StashMessage: "Token rotation", MatchText: "token.go", MatchScope: search.ScopeFiles, FileName: "token.go"},
	})

	state := plugin.AppState{Mode: plugin.ModeSearch}
	view := p.View(state, 120, 40)

	if !strings.Contains(view, "stash@{0}") {
		t.Error("expected stash@{0} in view")
	}
	if !strings.Contains(view, "Fix auth") {
		t.Error("expected 'Fix auth' in view")
	}
	// Result count is styled, so check for the number.
	if !strings.Contains(view, "2 result") {
		t.Error("expected '2 result' count in view")
	}
}

func TestPlugin_View_ScopeChips(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Mode: plugin.ModeSearch}
	view := p.View(state, 120, 40)

	for _, name := range []string{"All", "Messages", "Files", "Diffs", "Branch"} {
		if !strings.Contains(view, name) {
			t.Errorf("expected scope chip %q in view", name)
		}
	}
}

// ─── Integration Tests ──────────────────────────────────────

func TestIntegration_SearchFindsStashByMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := t.TempDir()

	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	writeFile(t, dir, "base.go", "package main\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "init")

	messages := []string{
		"Fix authentication bug",
		"Add dashboard component",
		"Refactor database layer",
		"Update API endpoints",
		"Token rotation feature",
	}

	for i, msg := range messages {
		writeFile(t, dir, fmt.Sprintf("file%d.go", i), fmt.Sprintf("package p%d\n", i))
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "stash", "push", "-m", msg)
	}

	// Build index from stash list.
	idx := search.NewIndex()
	for i, msg := range messages {
		idx.AddEntriesForTest([]search.IndexEntry{
			{StashIndex: i, StashSHA: fmt.Sprintf("sha%d", i), Message: msg, Scope: search.ScopeMessages},
		})
	}
	idx.MarkReadyForTest()

	// Search for "token" → should find "Token rotation feature" (index 4).
	results := idx.Search("token", search.ScopeAll)
	if len(results) == 0 {
		t.Fatal("expected results for 'token', got none")
	}
	found := false
	for _, r := range results {
		if r.StashIndex == 4 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected stash@{4} ('Token rotation feature') for query 'token'")
	}

	// Search for "database" → should find "Refactor database layer" (index 2).
	results = idx.Search("database", search.ScopeAll)
	found = false
	for _, r := range results {
		if r.StashIndex == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected stash@{2} ('Refactor database layer') for query 'database'")
	}
}

func TestIntegration_SearchFindsDiffContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := t.TempDir()

	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	writeFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "init")

	// Create a stash with specific diff content.
	writeFile(t, dir, "main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"UniqueSearchToken12345\")\n}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "Add unique output")

	// Get the diff for this stash.
	diffOutput := gitCmd(t, dir, "stash", "show", "-p", "stash@{0}")

	// Parse the diff into index entries.
	stash := plugin.Stash{Index: 0, SHA: "realsha", Message: "Add unique output", Branch: "main"}
	fileEntries, diffEntries := search.ParseDiffForIndex(stash, diffOutput)

	idx := search.NewIndex()
	idx.AddEntriesForTest(fileEntries)
	idx.AddEntriesForTest(diffEntries)
	idx.MarkReadyForTest()

	// Search for the unique token in diff content.
	results := idx.Search("UniqueSearchToken12345", search.ScopeDiffs)
	if len(results) == 0 {
		t.Fatal("expected results for 'UniqueSearchToken12345' in diffs, got none")
	}
	r := results[0]
	if r.StashIndex != 0 {
		t.Errorf("expected stash index 0, got %d", r.StashIndex)
	}
	if r.FileName == "" {
		t.Error("expected non-empty FileName in diff result")
	}
	if r.MatchScope != search.ScopeDiffs {
		t.Errorf("expected ScopeDiffs, got %d", r.MatchScope)
	}
}

func TestIntegration_BuildIndexWithRealDiffs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := t.TempDir()

	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@test.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	writeFile(t, dir, "app.go", "package main\nfunc Run() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "init")

	// Create stashes with varied content.
	writeFile(t, dir, "auth.go", "package main\nfunc Authenticate() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "authentication feature")

	writeFile(t, dir, "cache.go", "package main\nfunc CacheInvalidate() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "cache layer")

	// Get diffs.
	diff0 := gitCmd(t, dir, "stash", "show", "-p", "stash@{0}")
	diff1 := gitCmd(t, dir, "stash", "show", "-p", "stash@{1}")

	stashes := []plugin.Stash{
		{Index: 0, SHA: "sha0", Message: "cache layer", Branch: "main"},
		{Index: 1, SHA: "sha1", Message: "authentication feature", Branch: "main"},
	}

	cache := &diffCache{diffs: map[string]string{
		"sha0": diff0,
		"sha1": diff1,
	}}

	idx := search.NewIndex()
	cmd := search.BuildIndexCmd(stashes, cache, idx)
	msg := cmd()

	readyMsg, ok := msg.(search.IndexReadyMsg)
	if !ok {
		t.Fatalf("expected IndexReadyMsg, got %T", msg)
	}
	if readyMsg.EntryCount == 0 {
		t.Error("expected non-zero entry count")
	}

	// Search for file content.
	results := idx.Search("Authenticate", search.ScopeDiffs)
	if len(results) == 0 {
		t.Error("expected results for 'Authenticate' in diffs")
	}

	// Search for message.
	results = idx.Search("cache", search.ScopeMessages)
	if len(results) == 0 {
		t.Error("expected results for 'cache' in messages")
	}

	// Search for file name.
	results = idx.Search("auth.go", search.ScopeFiles)
	if len(results) == 0 {
		t.Error("expected results for 'auth.go' in files")
	}
}

// ─── Test helpers ───────────────────────────────────────────

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME="+dir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
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
