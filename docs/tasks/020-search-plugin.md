# Task 020: Deep Fuzzy Search Plugin

## Status: TODO

## Depends On
- 006 (core model — AppState, Stash type, Mode enum, plugin interfaces)
- 004 (cache — StashCache for lazy diff loading and LRU cache)

## Parallelizable With
- 021 (filter and stale plugins)
- 022 (reorder plugin)
- 023 (export/import plugin)
- 024 (help overlay and mouse support)
- 025 (config file and polish)

## Problem
Developers accumulate stashes but have no way to search across them. `git stash list | grep` only matches against the one-line summary — it cannot search diff content, file names, or branch names. A developer who stashed "that auth fix" three weeks ago has no efficient way to find it among 30+ stashes. nidhi needs a deep fuzzy search that indexes stash messages, file names, and full diff content, returning results with highlighted match context and letting the user jump directly to the matching stash.

## PRD Reference
- Section 6.2, FR-11 (Deep Fuzzy Search) — all sub-requirements FR-11.1 through FR-11.7
- Section 10, Screen 5 (SEARCH) — layout spec, text input, scope chips, result rendering
- Section 11.2 (SEARCH Mode keymap) — `/` opens, `Tab` cycles scopes, `j/k` navigates results, `Enter` jumps, `Esc` closes
- Section 8.2 (Plugin interfaces) — KeyHandler + ScreenProvider
- Section 8.4 (Module structure) — `internal/plugins/search/search.go`, `internal/plugins/search/index.go`
- Section 4.2 — `github.com/sahilm/fuzzy` for matching
- Section 7.1 — Search index build: background, non-blocking, <2s for 50 stashes
- Section 12.2 `[performance]` — `search_index = "lazy"` (default) vs `"eager"`
- Section 14.2 — Search keystroke target: <50ms in-memory fuzzy filter

## Files to Create
- `internal/plugins/search/search.go` — search plugin implementing KeyHandler + ScreenProvider
- `internal/plugins/search/index.go` — search index builder (async, non-blocking)
- `internal/plugins/search/search_test.go` — unit and integration tests

## Execution Steps

### Step 1: Create search index builder (`internal/plugins/search/index.go`)

```go
package search

import (
	"context"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
)

// Scope defines which fields to search.
type Scope int

const (
	ScopeAll      Scope = iota // Search everything
	ScopeMessages              // Stash messages only
	ScopeFiles                 // File names only
	ScopeDiffs                 // Diff content only
	ScopeBranch                // Branch names only
)

// ScopeNames maps scope values to display labels for chips.
var ScopeNames = []string{"All", "Messages", "Files", "Diffs", "Branch"}

// IndexEntry represents one searchable item tied to a stash.
type IndexEntry struct {
	StashIndex int    // Index of the stash in the stash list
	StashSHA   string // SHA of the stash commit
	Message    string // Stash message
	Branch     string // Branch where stash was created
	FileName   string // File name (one entry per file)
	DiffLine   string // A single line of diff content
	LineNum    int    // Line number within the diff
	Scope      Scope  // Which scope this entry belongs to
}

// SearchResult represents a match result with context.
type SearchResult struct {
	StashIndex   int
	StashSHA     string
	StashMessage string
	MatchText    string   // The text that matched
	MatchScope   Scope    // Where the match was found
	FileName     string   // File name (for file/diff matches)
	LineNum      int      // Line number (for diff matches)
	Score        int      // Fuzzy match score (higher = better)
	MatchedIndexes []int  // Character positions of matches (for highlighting)
}

// IndexBuildingMsg is sent when the index starts building.
type IndexBuildingMsg struct{}

// IndexProgressMsg is sent periodically during indexing.
type IndexProgressMsg struct {
	Processed int
	Total     int
}

// IndexReadyMsg is sent when the index is fully built.
type IndexReadyMsg struct {
	EntryCount int
}

// Index holds the searchable data for all stashes.
type Index struct {
	mu      sync.RWMutex
	entries []IndexEntry
	ready   bool
	partial bool // true if some entries are available but index is still building
}

// NewIndex creates an empty search index.
func NewIndex() *Index {
	return &Index{}
}

// IsReady returns true if the index is fully built.
func (idx *Index) IsReady() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.ready
}

// HasPartialResults returns true if some results are available.
func (idx *Index) HasPartialResults() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.partial || idx.ready
}

// EntryCount returns the number of indexed entries.
func (idx *Index) EntryCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// addEntries appends entries to the index (thread-safe).
func (idx *Index) addEntries(entries []IndexEntry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.entries = append(idx.entries, entries...)
	idx.partial = true
}

// markReady marks the index as fully built.
func (idx *Index) markReady() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.ready = true
}

// Search performs fuzzy search across the index filtered by scope.
// Returns results sorted by score (descending). Deduplicates by stash SHA
// so that a stash with multiple diff line matches collapses to the best one.
func (idx *Index) Search(query string, scope Scope) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(query) == 0 || len(idx.entries) == 0 {
		return nil
	}

	// Collect source strings for fuzzy matching, filtered by scope.
	var filtered []IndexEntry
	var sources []string
	for _, e := range idx.entries {
		if scope != ScopeAll && e.Scope != scope {
			continue
		}
		filtered = append(filtered, e)
		switch e.Scope {
		case ScopeMessages:
			sources = append(sources, e.Message)
		case ScopeFiles:
			sources = append(sources, e.FileName)
		case ScopeDiffs:
			sources = append(sources, e.DiffLine)
		case ScopeBranch:
			sources = append(sources, e.Branch)
		default:
			sources = append(sources, e.Message)
		}
	}

	matches := fuzzy.Find(query, sources)

	// Convert to SearchResults, deduplicate by stash SHA + scope.
	seen := make(map[string]int) // key: "sha:scope" -> index in results
	var results []SearchResult
	for _, m := range matches {
		entry := filtered[m.Index]
		key := entry.StashSHA + ":" + string(rune(entry.Scope))
		if existingIdx, ok := seen[key]; ok {
			// Keep the higher-scoring match.
			if m.Score > results[existingIdx].Score {
				results[existingIdx] = SearchResult{
					StashIndex:     entry.StashIndex,
					StashSHA:       entry.StashSHA,
					StashMessage:   entry.Message,
					MatchText:      m.Str,
					MatchScope:     entry.Scope,
					FileName:       entry.FileName,
					LineNum:        entry.LineNum,
					Score:          m.Score,
					MatchedIndexes: m.MatchedIndexes,
				}
			}
			continue
		}
		seen[key] = len(results)
		results = append(results, SearchResult{
			StashIndex:     entry.StashIndex,
			StashSHA:       entry.StashSHA,
			StashMessage:   entry.Message,
			MatchText:      m.Str,
			MatchScope:     entry.Scope,
			FileName:       entry.FileName,
			LineNum:        entry.LineNum,
			Score:          m.Score,
			MatchedIndexes: m.MatchedIndexes,
		})
	}

	return results
}

// BuildCmd returns a tea.Cmd that builds the search index asynchronously.
// It indexes stash messages, file names, and diff content for each stash.
// Sends IndexProgressMsg periodically and IndexReadyMsg when done.
func BuildCmd(stashes []core.Stash, cache git.StashCache) tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder — in reality, this returns a sequence of commands
		// via tea.Batch or tea.Sequence to allow progressive results.
		// See the full implementation in the execution step below.
		return IndexBuildingMsg{}
	}
}
```

**Actual implementation details for `BuildCmd`:**

The index builder must:
1. Start by indexing messages and branch names (fast, no git calls needed).
2. For each stash, run `git stash show <sha>` to get file names (one git call per stash).
3. For each stash, run `git stash show -p <sha>` to get diff content (one git call per stash).
4. After each stash is processed, add entries to the index and send `IndexProgressMsg`.
5. When all stashes are processed, mark the index ready and send `IndexReadyMsg`.
6. Use `context.WithCancel` so the build can be cancelled (e.g., if user navigates away).

For the file name indexing, parse `git stash show <sha>` output:
```
 src/auth/token.go | 5 +++--
 src/auth/config.go | 2 +-
```
Extract the file path from each line.

For the diff content indexing, parse `git stash show -p <sha>` output line by line. Each non-header, non-hunk-header line becomes an IndexEntry with `Scope = ScopeDiffs`. Track line numbers from `@@` hunk headers.

### Step 2: Create search plugin (`internal/plugins/search/search.go`)

```go
package search

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID   = "search"
	PluginName = "Deep Fuzzy Search"
)

// Plugin implements KeyHandler + ScreenProvider for search.
type Plugin struct {
	ctx     plugin.PluginContext
	index   *Index
	input   textinput.Model
	scope   Scope
	results []SearchResult
	cursor  int
	active  bool
}

// Ensure interface compliance.
var (
	_ plugin.KeyHandler     = (*Plugin)(nil)
	_ plugin.ScreenProvider = (*Plugin)(nil)
)

func New() *Plugin {
	ti := textinput.New()
	ti.Placeholder = "Search stashes..."
	ti.CharLimit = 256
	return &Plugin{
		index: NewIndex(),
		input: ti,
		scope: ScopeAll,
	}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.ctx = ctx
	// If config says eager indexing, start building now.
	// Otherwise, defer to first search activation.
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns the keybindings this plugin provides.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "/", Description: "Search", Modes: []core.Mode{core.ModeList, core.ModePreview}},
	}
}

// HandleKey handles the `/` key to activate search mode.
func (p *Plugin) HandleKey(key plugin.KeyEvent, state core.AppState) (core.AppState, tea.Cmd) {
	if key.Text == "/" && !p.active {
		p.active = true
		p.input.Reset()
		p.input.Focus()
		p.results = nil
		p.cursor = 0
		state.Mode = core.ModeSearch

		// If index is not yet built, start building it now (lazy mode).
		var cmd tea.Cmd
		if !p.index.IsReady() {
			cmd = BuildCmd(state.Stashes, p.ctx.Cache)
		}
		return state, cmd
	}
	return state, nil
}

// Screens returns the search screen definition.
func (p *Plugin) Screens() []plugin.ScreenDef {
	return []plugin.ScreenDef{
		{ID: "search", Mode: core.ModeSearch},
	}
}

// Update handles messages when the search screen is active.
func (p *Plugin) Update(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return p.handleSearchKey(msg, state)

	case IndexBuildingMsg:
		// Index building started — no action needed, just wait for progress.
		return state, nil

	case IndexProgressMsg:
		// Update a loading indicator if desired.
		// Re-run search with current query to show progressive results.
		if p.input.Value() != "" {
			p.results = p.index.Search(p.input.Value(), p.scope)
		}
		return state, nil

	case IndexReadyMsg:
		// Index is fully built. Re-run search with current query.
		if p.input.Value() != "" {
			p.results = p.index.Search(p.input.Value(), p.scope)
		}
		return state, nil
	}

	// Forward to text input.
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)

	// Live filtering: re-search on every keystroke.
	query := p.input.Value()
	if query != "" && p.index.HasPartialResults() {
		p.results = p.index.Search(query, p.scope)
		p.cursor = 0 // Reset cursor on new results.
	} else if query == "" {
		p.results = nil
		p.cursor = 0
	}

	return state, cmd
}

// handleSearchKey processes key events within the search screen.
func (p *Plugin) handleSearchKey(msg tea.KeyPressMsg, state core.AppState) (core.AppState, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape:
		// Close search, return to previous mode.
		p.active = false
		p.input.Blur()
		state.Mode = core.ModeList
		return state, nil

	case msg.Code == tea.KeyTab:
		// Cycle scope chips: All -> Messages -> Files -> Diffs -> Branch -> All.
		p.scope = (p.scope + 1) % Scope(len(ScopeNames))
		// Re-run search with new scope.
		if p.input.Value() != "" {
			p.results = p.index.Search(p.input.Value(), p.scope)
			p.cursor = 0
		}
		return state, nil

	case msg.Text == "j" || msg.Code == tea.KeyDown:
		if p.cursor < len(p.results)-1 {
			p.cursor++
		}
		return state, nil

	case msg.Text == "k" || msg.Code == tea.KeyUp:
		if p.cursor > 0 {
			p.cursor--
		}
		return state, nil

	case msg.Code == tea.KeyEnter:
		if len(p.results) > 0 && p.cursor < len(p.results) {
			result := p.results[p.cursor]
			// Jump to the matched stash in the list.
			state.Cursor = result.StashIndex
			state.SearchQuery = p.input.Value()
			p.active = false
			p.input.Blur()
			// If the match was in a diff, open PREVIEW mode.
			if result.MatchScope == ScopeDiffs || result.MatchScope == ScopeFiles {
				state.Mode = core.ModePreview
			} else {
				state.Mode = core.ModeList
			}
			return state, nil
		}
		return state, nil
	}

	// All other keys go to the text input.
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return state, cmd
}

// View renders the search screen.
// width/height represent the available content area (between status bar and footer).
func (p *Plugin) View(state core.AppState, width, height int) string {
	var b strings.Builder

	// Line 1: Search input.
	b.WriteString("  " + p.input.View())
	b.WriteString("\n")

	// Line 2: Scope chips.
	b.WriteString("  ")
	for i, name := range ScopeNames {
		if Scope(i) == p.scope {
			// Active chip: bold, with brackets.
			b.WriteString("[" + name + "]")
		} else {
			// Inactive chip: dimmed.
			b.WriteString(" " + name + " ")
		}
		b.WriteString(" ")
	}
	b.WriteString("\n\n")

	// Results.
	if len(p.results) == 0 {
		if p.input.Value() != "" {
			b.WriteString("  No matches found.")
			if !p.index.IsReady() {
				b.WriteString(" (indexing in progress...)")
			}
		}
	} else {
		maxVisible := height - 4 // Account for input, chips, and padding.
		start := 0
		if p.cursor >= maxVisible {
			start = p.cursor - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(p.results) {
			end = len(p.results)
		}
		for i := start; i < end; i++ {
			r := p.results[i]
			cursor := "  "
			if i == p.cursor {
				cursor = "▸ "
			}
			// Stash header line.
			b.WriteString(cursor + "stash@{" + itoa(r.StashIndex) + "} " + r.StashMessage + "\n")
			// Match context line (for file/diff matches).
			if r.FileName != "" {
				context := "    " + r.FileName
				if r.LineNum > 0 {
					context += ":" + itoa(r.LineNum)
				}
				context += "  " + r.MatchText
				b.WriteString(context + "\n")
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func itoa(i int) string {
	return strings.TrimSpace(strings.Replace(strings.Replace(
		strings.Replace(strings.Replace(strings.Replace(
			strings.Replace(strings.Replace(strings.Replace(
				strings.Replace(strings.Replace("          ", "", 1),
					"", "", 0), "", "", 0), "", "", 0), "", "", 0),
			"", "", 0), "", "", 0), "", "", 0), "", "", 0), "", "", 0))
	// NOTE: In the actual implementation, use strconv.Itoa(i).
}
```

**Important implementation notes:**

- `tea.KeyEscape`, `tea.KeyTab`, `tea.KeyEnter`, `tea.KeyDown`, `tea.KeyUp` are constants from BubbleTea v2's `tea` package. In v2, `KeyPressMsg` has a `Code` field (int32) for special keys and a `Text` field (string) for printable characters.
- The `View` method returns only the content area string. The core Layout Engine wraps it with status bar and footer.
- Fuzzy match highlighting in the rendered output should use the `MatchedIndexes` from `sahilm/fuzzy` to apply the theme's `accent.bright` color to matched characters.

### Step 3: Implement progressive indexing via `tea.Cmd` chain

The `BuildCmd` function must return a chain of `tea.Cmd`s to enable progressive results:

```go
// BuildIndexSequence returns a series of commands that build the index incrementally.
// Each command indexes one stash and returns an IndexProgressMsg.
// The final command returns IndexReadyMsg.
func BuildIndexSequence(stashes []core.Stash, cache git.StashCache, idx *Index) tea.Cmd {
	return func() tea.Msg {
		// Phase 1: Index messages and branches (instant, no git calls).
		var messageEntries []IndexEntry
		for _, s := range stashes {
			messageEntries = append(messageEntries, IndexEntry{
				StashIndex: s.Index,
				StashSHA:   s.SHA,
				Message:    s.Message,
				Branch:     s.Branch,
				Scope:      ScopeMessages,
			})
			messageEntries = append(messageEntries, IndexEntry{
				StashIndex: s.Index,
				StashSHA:   s.SHA,
				Message:    s.Message,
				Branch:     s.Branch,
				Scope:      ScopeBranch,
			})
		}
		idx.addEntries(messageEntries)

		// Phase 2: Index files and diffs (requires git calls, done per-stash).
		for i, s := range stashes {
			ctx := context.Background()

			// Get file names from cache (which calls git stash show).
			diff, err := cache.Diff(ctx, s.SHA)
			if err != nil {
				continue
			}

			fileEntries, diffEntries := parseDiffForIndex(s, diff)
			idx.addEntries(fileEntries)
			idx.addEntries(diffEntries)

			// Send progress (in practice, return IndexProgressMsg from the Cmd).
			_ = i // Progress: i+1 of len(stashes)
		}

		idx.markReady()
		return IndexReadyMsg{EntryCount: idx.EntryCount()}
	}
}

// parseDiffForIndex extracts file names and diff lines from a unified diff string.
func parseDiffForIndex(stash core.Stash, diff string) (fileEntries, diffEntries []IndexEntry) {
	lines := strings.Split(diff, "\n")
	currentFile := ""
	lineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// Extract file name: "diff --git a/foo.go b/foo.go" -> "foo.go"
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				currentFile = parts[1]
				fileEntries = append(fileEntries, IndexEntry{
					StashIndex: stash.Index,
					StashSHA:   stash.SHA,
					Message:    stash.Message,
					Branch:     stash.Branch,
					FileName:   currentFile,
					Scope:      ScopeFiles,
				})
			}
			continue
		}
		if strings.HasPrefix(line, "@@") {
			// Parse hunk header for line numbers.
			// Format: @@ -start,count +start,count @@ context
			// Extract the +start number.
			if idx := strings.Index(line, "+"); idx != -1 {
				numStr := ""
				for _, c := range line[idx+1:] {
					if c >= '0' && c <= '9' {
						numStr += string(c)
					} else {
						break
					}
				}
				if n, err := strconv.Atoi(numStr); err == nil {
					lineNum = n
				}
			}
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}
		// Content line (added, removed, or context).
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && currentFile != "" {
			diffEntries = append(diffEntries, IndexEntry{
				StashIndex: stash.Index,
				StashSHA:   stash.SHA,
				Message:    stash.Message,
				Branch:     stash.Branch,
				FileName:   currentFile,
				DiffLine:   trimmed,
				LineNum:    lineNum,
				Scope:      ScopeDiffs,
			})
		}
		if !strings.HasPrefix(line, "-") {
			lineNum++
		}
	}
	return
}
```

### Step 4: Write tests (`internal/plugins/search/search_test.go`)

```go
package search_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugins/search"
)

// --- Test Helpers ---

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
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
			t.Fatalf("command %v failed: %v\noutput: %s", args, err, out)
		}
	}

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	run("git", "commit", "--allow-empty", "-m", "init")

	return dir, run, writeFile
}

// --- Unit Tests ---

// TestFuzzyMatchScoring verifies that fuzzy.Find returns matches
// with correct scores across different scopes.
func TestFuzzyMatchScoring(t *testing.T) {
	idx := search.NewIndex()

	// Add entries with known content.
	entries := []search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "Fix auth token refresh", Scope: search.ScopeMessages},
		{StashIndex: 1, StashSHA: "bbb", Message: "WIP: new dashboard layout", Scope: search.ScopeMessages},
		{StashIndex: 2, StashSHA: "ccc", Message: "Token rotation implementation", Scope: search.ScopeMessages},
		{StashIndex: 0, StashSHA: "aaa", FileName: "src/auth/token.go", Scope: search.ScopeFiles},
		{StashIndex: 1, StashSHA: "bbb", FileName: "src/ui/dashboard.go", Scope: search.ScopeFiles},
	}
	idx.AddEntriesForTest(entries) // Exposed test helper that calls addEntries.

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

// TestScopeFilterRestrictsSearch verifies that scope filtering
// limits results to the specified scope only.
func TestScopeFilterRestrictsSearch(t *testing.T) {
	idx := search.NewIndex()
	entries := []search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "auth changes", Scope: search.ScopeMessages},
		{StashIndex: 0, StashSHA: "aaa", FileName: "auth.go", Scope: search.ScopeFiles},
		{StashIndex: 0, StashSHA: "aaa", DiffLine: "func AuthMiddleware()", Scope: search.ScopeDiffs},
		{StashIndex: 0, StashSHA: "aaa", Branch: "feature/auth", Scope: search.ScopeBranch},
	}
	idx.AddEntriesForTest(entries)

	// Search with ScopeMessages — only message match should appear.
	results := idx.Search("auth", search.ScopeMessages)
	for _, r := range results {
		if r.MatchScope != search.ScopeMessages {
			t.Errorf("expected only ScopeMessages results, got scope=%d", r.MatchScope)
		}
	}

	// Search with ScopeFiles — only file match should appear.
	results = idx.Search("auth", search.ScopeFiles)
	for _, r := range results {
		if r.MatchScope != search.ScopeFiles {
			t.Errorf("expected only ScopeFiles results, got scope=%d", r.MatchScope)
		}
	}

	// Search with ScopeDiffs — only diff match should appear.
	results = idx.Search("auth", search.ScopeDiffs)
	for _, r := range results {
		if r.MatchScope != search.ScopeDiffs {
			t.Errorf("expected only ScopeDiffs results, got scope=%d", r.MatchScope)
		}
	}

	// Search with ScopeBranch — only branch match should appear.
	results = idx.Search("auth", search.ScopeBranch)
	for _, r := range results {
		if r.MatchScope != search.ScopeBranch {
			t.Errorf("expected only ScopeBranch results, got scope=%d", r.MatchScope)
		}
	}
}

// TestIndexBuilderProcessesDiffs verifies that parseDiffForIndex
// correctly extracts file names and diff lines.
func TestIndexBuilderProcessesDiffs(t *testing.T) {
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
	stash := core.Stash{Index: 0, SHA: "abc123", Message: "Fix auth", Branch: "main"}
	fileEntries, diffEntries := search.ParseDiffForIndexExported(stash, diff)

	// Verify file entries.
	if len(fileEntries) != 2 {
		t.Fatalf("expected 2 file entries, got %d", len(fileEntries))
	}
	if fileEntries[0].FileName != "src/auth/token.go" {
		t.Errorf("expected first file 'src/auth/token.go', got %q", fileEntries[0].FileName)
	}
	if fileEntries[1].FileName != "src/auth/config.go" {
		t.Errorf("expected second file 'src/auth/config.go', got %q", fileEntries[1].FileName)
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
}

// TestEmptyQueryReturnsNoResults verifies empty search returns nothing.
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

// TestLazyVsEagerIndexing verifies that the index respects the
// lazy/eager configuration setting.
func TestLazyVsEagerIndexing(t *testing.T) {
	idx := search.NewIndex()

	// Freshly created index should not be ready.
	if idx.IsReady() {
		t.Error("newly created index should not be ready")
	}
	if idx.HasPartialResults() {
		t.Error("newly created index should not have partial results")
	}

	// After adding entries, should have partial results.
	idx.AddEntriesForTest([]search.IndexEntry{
		{StashIndex: 0, StashSHA: "aaa", Message: "test", Scope: search.ScopeMessages},
	})
	if !idx.HasPartialResults() {
		t.Error("index with entries should have partial results")
	}
	if idx.IsReady() {
		t.Error("index with entries but not marked ready should not be ready")
	}

	// After marking ready, should be ready.
	idx.MarkReadyForTest()
	if !idx.IsReady() {
		t.Error("index marked ready should be ready")
	}
}

// --- Integration Tests ---

// TestSearchFindsStashByMessage creates a real repo with stashes and
// verifies that searching for a keyword in a stash message finds the correct stash.
func TestSearchFindsStashByMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	run := func(args ...string) string {
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
			t.Fatalf("command %v failed: %v\noutput: %s", args, err, out)
		}
		return string(out)
	}

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Setup repo with 10 stashes, each with a unique message.
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	writeFile("base.go", "package main\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	messages := []string{
		"Fix authentication bug",
		"Add dashboard component",
		"Refactor database layer",
		"Update API endpoints",
		"Fix rate limiter",
		"Add logging middleware",
		"Token rotation feature",
		"Cache invalidation logic",
		"UI theme updates",
		"Performance optimization",
	}

	for i, msg := range messages {
		writeFile("file"+strconv.Itoa(i)+".go", "package p"+strconv.Itoa(i)+"\n")
		run("git", "add", ".")
		run("git", "stash", "push", "-m", msg)
	}

	// Build the index from the stash list.
	// In the real implementation, we would use the GitRunner and StashCache.
	// For this test, we build the index directly.
	idx := search.NewIndex()
	for i, msg := range messages {
		idx.AddEntriesForTest([]search.IndexEntry{
			{StashIndex: i, StashSHA: "sha" + strconv.Itoa(i), Message: msg, Scope: search.ScopeMessages},
		})
	}
	idx.MarkReadyForTest()

	// Search for "token" — should find "Token rotation feature" (index 6).
	results := idx.Search("token", search.ScopeAll)
	if len(results) == 0 {
		t.Fatal("expected results for 'token', got none")
	}
	found := false
	for _, r := range results {
		if r.StashIndex == 6 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find stash@{6} ('Token rotation feature') for query 'token'")
	}

	// Search for "database" — should find "Refactor database layer" (index 2).
	results = idx.Search("database", search.ScopeAll)
	found = false
	for _, r := range results {
		if r.StashIndex == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find stash@{2} ('Refactor database layer') for query 'database'")
	}
}

// TestSearchFindsDiffContent creates a repo, stashes with specific
// diff content, and verifies that searching for content within the diff
// returns the correct stash with file:line context.
func TestSearchFindsDiffContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	run := func(args ...string) string {
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
			t.Fatalf("command %v failed: %v\noutput: %s", args, err, out)
		}
		return string(out)
	}

	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Setup repo.
	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	writeFile("main.go", "package main\n\nfunc main() {}\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	// Create a stash with specific diff content.
	writeFile("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"UniqueSearchToken12345\")\n}\n")
	run("git", "add", ".")
	run("git", "stash", "push", "-m", "Add unique output")

	// Get the diff for this stash.
	diffOutput := run("git", "stash", "show", "-p", "stash@{0}")

	// Parse the diff into index entries.
	stash := core.Stash{Index: 0, SHA: "realsha", Message: "Add unique output", Branch: "main"}
	fileEntries, diffEntries := search.ParseDiffForIndexExported(stash, diffOutput)

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
```

### Step 5: Verify

```bash
# Run unit tests for the search package.
go test -v -count=1 ./internal/plugins/search/...

# Run integration tests (not short mode).
go test -v -count=1 -run 'TestSearchFinds' ./internal/plugins/search/...

# Full CI pipeline.
make ci
```

## Verification

### Functional
```bash
# Unit tests pass
go test -v -count=1 -run 'TestFuzzyMatchScoring|TestScopeFilter|TestIndexBuilder|TestEmpty|TestLazyVsEager' ./internal/plugins/search/...

# Integration tests pass
go test -v -count=1 -run 'TestSearchFindsStashByMessage|TestSearchFindsDiffContent' ./internal/plugins/search/...

# search.go compiles and implements KeyHandler + ScreenProvider interfaces
go vet ./internal/plugins/search/...

# No lint issues
golangci-lint run ./internal/plugins/search/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/plugins/search/search.go` implements `KeyHandler` and `ScreenProvider` interfaces
2. `internal/plugins/search/index.go` implements async, non-blocking index builder
3. `/` key opens search overlay with text input and scope chips
4. `Tab` cycles scope chips (All, Messages, Files, Diffs, Branch)
5. `Esc` closes search and returns to LIST mode
6. `Enter` on a result jumps to the matching stash; opens PREVIEW if match is in diff
7. `j/k` navigates results within the search screen
8. Live filtering: results update on every keystroke via `sahilm/fuzzy`
9. Index builds asynchronously via `tea.Cmd` — UI never blocks
10. Progressive results: partial matches shown while indexing continues
11. Lazy indexing: index builds on first `/` press (default config)
12. All unit tests pass: fuzzy match scoring, scope filtering, diff parsing, empty query, lazy/eager behavior
13. All integration tests pass: 10-stash repo message search, diff content search with file:line context
14. `make ci` passes (lint + test)

## Commit
```
feat(search): add deep fuzzy search plugin with async index builder

Implement search plugin (KeyHandler + ScreenProvider) with live fuzzy
filtering across stash messages, file names, diff content, and branch
names. Index builds asynchronously via tea.Cmd with progressive results.
Scope chips (All/Messages/Files/Diffs/Branch) filter search targets.
Uses sahilm/fuzzy for matching. Enter on result jumps to stash in list
and opens preview for diff matches.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.2 (FR-11), 10 (Screen 5), 11.2 (SEARCH keymap), 8.2 (interfaces), 4.2, 7.1, 12.2, 14.2
4. Verify dependencies: task 006 (core model with AppState, Stash, Mode types) and task 004 (StashCache) are DONE
5. Create `internal/plugins/search/index.go` with Index type, entry types, and diff parser
6. Create `internal/plugins/search/search.go` with Plugin implementing KeyHandler + ScreenProvider
7. Create `internal/plugins/search/search_test.go` with all unit and integration tests
8. Run `go test -v -count=1 ./internal/plugins/search/...`
9. Run `make ci`
10. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
11. Commit with the message above
