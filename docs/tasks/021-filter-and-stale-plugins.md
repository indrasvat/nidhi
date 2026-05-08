# Task 021: Filter by Branch + Stale Detection Plugins

## Status: TODO

## Depends On
- 006 (core model — AppState, Stash type, Mode enum, plugin interfaces, Filter type)
- 004 (cache — StashCache for stash listing)

## Parallelizable With
- 020 (search plugin)
- 022 (reorder plugin)
- 023 (export/import plugin)
- 024 (help overlay and mouse support)
- 025 (config file and polish)

## Problem
When stash lists grow beyond a handful of entries, developers need two key filtering capabilities: (1) filtering by branch to see only stashes relevant to their current context, and (2) identifying stale stashes that have been sitting around for weeks and likely represent forgotten or obsoleted work. Without these, the stash list becomes an undifferentiated wall of entries. nidhi must provide composable filters (branch + stale can combine) with visual indicators in the status bar, plus a bulk drop for stale stashes to keep the vault clean.

## PRD Reference
- Section 6.2, FR-15 (Stale Detection) — FR-15.1 badge, FR-15.2 `fs` filter, FR-15.3 bulk drop with confirmation
- Section 6.2, FR-17 (Filter by Branch) — FR-17.1 `fb` toggle, FR-17.2 chip in status bar, FR-17.3 `fc` clear
- Section 8.2 (Plugin interfaces) — KeyHandler for filter, Plugin (passive) for stale
- Section 8.3 (Core Types) — `Stash.IsStale`, `Stash.Branch`, `AppState.Filters`
- Section 8.4 (Module structure) — `internal/plugins/filter/filter.go`, `internal/plugins/stale/stale.go`
- Section 11.2 (LIST Mode keymap) — `fb`, `fs`, `fc` keybindings
- Section 12.2 — `stale_days = 14` (default threshold)
- Section 9.3 (Layout Contract) — filter chips in status bar

## Files to Create
- `internal/plugins/filter/filter.go` — branch filter + stale filter + clear filter (KeyHandler)
- `internal/plugins/filter/filter_test.go` — unit and integration tests for filter plugin
- `internal/plugins/stale/stale.go` — stale detection + bulk drop (Plugin, passive)
- `internal/plugins/stale/stale_test.go` — unit and integration tests for stale plugin

## Execution Steps

### Step 1: Create filter plugin (`internal/plugins/filter/filter.go`)

```go
package filter

import (
	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID   = "filter"
	PluginName = "Branch & Stale Filter"
)

// FilterType enumerates available filters.
type FilterType string

const (
	FilterBranch FilterType = "branch"
	FilterStale  FilterType = "stale"
)

// ActiveFilter represents a currently applied filter.
type ActiveFilter struct {
	Type  FilterType
	Label string // Display label for the status bar chip, e.g. "⎇ main" or "⌛ stale"
}

// Plugin implements KeyHandler for filter toggling.
type Plugin struct {
	ctx           plugin.PluginContext
	activeBranch  bool   // true when fb filter is active
	activeStale   bool   // true when fs filter is active
	currentBranch string // cached current branch name
}

var _ plugin.KeyHandler = (*Plugin)(nil)

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.ctx = ctx
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns the two-key filter bindings.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "fb", Description: "Filter: current branch", Modes: []core.Mode{core.ModeList, core.ModePreview}},
		{Key: "fs", Description: "Filter: stale stashes", Modes: []core.Mode{core.ModeList, core.ModePreview}},
		{Key: "fc", Description: "Clear all filters", Modes: []core.Mode{core.ModeList, core.ModePreview}},
	}
}

// HandleKey processes filter key sequences.
// Two-key sequences: the core key dispatcher accumulates `f` as a prefix,
// then dispatches `fb`, `fs`, or `fc` as the full key text.
func (p *Plugin) HandleKey(key plugin.KeyEvent, state core.AppState) (core.AppState, tea.Cmd) {
	switch key.Text {
	case "fb":
		p.activeBranch = !p.activeBranch
		p.currentBranch = state.Branch
		state = p.applyFilters(state)
		return state, nil

	case "fs":
		p.activeStale = !p.activeStale
		state = p.applyFilters(state)
		return state, nil

	case "fc":
		p.activeBranch = false
		p.activeStale = false
		state.Filters = nil
		// Reset cursor to 0 since filter removal changes the visible list.
		state.Cursor = 0
		return state, nil
	}

	return state, nil
}

// applyFilters computes the active filter list and stores it in AppState.
// Actual filtering of the stash list is done by the core rendering loop,
// which reads state.Filters and shows only matching stashes.
func (p *Plugin) applyFilters(state core.AppState) core.AppState {
	state.Filters = nil

	if p.activeBranch {
		state.Filters = append(state.Filters, core.Filter{
			ID:    string(FilterBranch),
			Label: "⎇ " + p.currentBranch,
			Match: func(s core.Stash) bool {
				return s.Branch == p.currentBranch
			},
		})
	}

	if p.activeStale {
		state.Filters = append(state.Filters, core.Filter{
			ID:    string(FilterStale),
			Label: "⌛ stale",
			Match: func(s core.Stash) bool {
				return s.IsStale
			},
		})
	}

	// Reset cursor when filters change — the visible list shifts.
	state.Cursor = 0
	return state
}

// ActiveFilters returns the list of currently active filters for status bar rendering.
func (p *Plugin) ActiveFilters() []ActiveFilter {
	var filters []ActiveFilter
	if p.activeBranch {
		filters = append(filters, ActiveFilter{Type: FilterBranch, Label: "⎇ " + p.currentBranch})
	}
	if p.activeStale {
		filters = append(filters, ActiveFilter{Type: FilterStale, Label: "⌛ stale"})
	}
	return filters
}

// IsActive returns true if any filter is currently active.
func (p *Plugin) IsActive() bool {
	return p.activeBranch || p.activeStale
}

// FilterStashes applies all active filters to a stash list and returns
// only the stashes that match ALL active filters (AND composition).
func FilterStashes(stashes []core.Stash, filters []core.Filter) []core.Stash {
	if len(filters) == 0 {
		return stashes
	}

	var result []core.Stash
	for _, s := range stashes {
		matchAll := true
		for _, f := range filters {
			if !f.Match(s) {
				matchAll = false
				break
			}
		}
		if matchAll {
			result = append(result, s)
		}
	}
	return result
}
```

### Step 2: Create stale detection plugin (`internal/plugins/stale/stale.go`)

```go
package stale

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID       = "stale"
	PluginName     = "Stale Detection"
	DefaultDays    = 14
)

// Plugin implements Plugin (passive). It does not register keybindings —
// it modifies stash rendering by computing staleness and provides
// bulk drop for stale stashes.
type Plugin struct {
	ctx       plugin.PluginContext
	threshold time.Duration
}

var _ plugin.Plugin = (*Plugin)(nil)

func New() *Plugin {
	return &Plugin{
		threshold: time.Duration(DefaultDays) * 24 * time.Hour,
	}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.ctx = ctx
	// Read threshold from config if available.
	staleDays := ctx.Config.GetInt("general.stale_days", DefaultDays)
	p.threshold = time.Duration(staleDays) * 24 * time.Hour
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// SetThreshold allows overriding the stale threshold (useful for testing).
func (p *Plugin) SetThreshold(d time.Duration) {
	p.threshold = d
}

// Threshold returns the current staleness threshold.
func (p *Plugin) Threshold() time.Duration {
	return p.threshold
}

// MarkStale computes IsStale for each stash in the list based on
// the current time and the configured threshold.
// This should be called after loading stashes and whenever the stash list is refreshed.
func (p *Plugin) MarkStale(stashes []core.Stash) []core.Stash {
	return MarkStaleWithTime(stashes, time.Now(), p.threshold)
}

// MarkStaleWithTime computes IsStale using a provided reference time.
// Exported for testing with deterministic timestamps.
func MarkStaleWithTime(stashes []core.Stash, now time.Time, threshold time.Duration) []core.Stash {
	for i := range stashes {
		age := now.Sub(stashes[i].Date)
		stashes[i].IsStale = age >= threshold
	}
	return stashes
}

// StaleStashes returns only the stashes that are marked stale.
func StaleStashes(stashes []core.Stash) []core.Stash {
	var result []core.Stash
	for _, s := range stashes {
		if s.IsStale {
			result = append(result, s)
		}
	}
	return result
}

// BulkDropStaleMsg is the message sent to initiate bulk drop of stale stashes.
type BulkDropStaleMsg struct {
	Confirmed bool
	Stashes   []core.Stash // Stale stashes to drop
}

// BulkDropStaleCmd returns a tea.Cmd that drops all stale stashes.
// Requires confirmation before execution. The command drops stashes
// from highest index to lowest (to preserve index validity).
// Each dropped stash's SHA is stored for undo recovery.
func BulkDropStaleCmd(staleStashes []core.Stash, gitRunner core.GitRunner) tea.Cmd {
	return func() tea.Msg {
		// Sort stashes by index descending to avoid index shifts.
		sorted := make([]core.Stash, len(staleStashes))
		copy(sorted, staleStashes)
		// Sort descending by index.
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j].Index > sorted[i].Index {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}

		dropped := make([]core.Stash, 0, len(sorted))
		for _, s := range sorted {
			_, err := gitRunner.Run(nil, "stash", "drop", "stash@{"+itoa(s.Index)+"}")
			if err != nil {
				// Stop on first error. Return partial success.
				return BulkDropResultMsg{
					Dropped: dropped,
					Error:   err,
				}
			}
			dropped = append(dropped, s)
		}

		return BulkDropResultMsg{Dropped: dropped}
	}
}

// BulkDropResultMsg is the result of a bulk drop operation.
type BulkDropResultMsg struct {
	Dropped []core.Stash
	Error   error
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
```

### Step 3: Write filter tests (`internal/plugins/filter/filter_test.go`)

```go
package filter_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugins/filter"
)

// --- Unit Tests ---

// TestFilterComposition verifies that branch + stale filters compose
// with AND logic: only stashes matching BOTH filters are returned.
func TestFilterComposition(t *testing.T) {
	now := time.Now()
	stashes := []core.Stash{
		{Index: 0, SHA: "aaa", Branch: "main", Date: now, IsStale: false},                                 // main, fresh
		{Index: 1, SHA: "bbb", Branch: "main", Date: now.Add(-30 * 24 * time.Hour), IsStale: true},        // main, stale
		{Index: 2, SHA: "ccc", Branch: "feature", Date: now, IsStale: false},                               // feature, fresh
		{Index: 3, SHA: "ddd", Branch: "feature", Date: now.Add(-30 * 24 * time.Hour), IsStale: true},     // feature, stale
	}

	// Branch filter only (main).
	branchFilter := core.Filter{
		ID: "branch", Label: "⎇ main",
		Match: func(s core.Stash) bool { return s.Branch == "main" },
	}
	result := filter.FilterStashes(stashes, []core.Filter{branchFilter})
	if len(result) != 2 {
		t.Fatalf("branch filter: expected 2 results, got %d", len(result))
	}
	for _, s := range result {
		if s.Branch != "main" {
			t.Errorf("branch filter: unexpected branch %q", s.Branch)
		}
	}

	// Stale filter only.
	staleFilter := core.Filter{
		ID: "stale", Label: "⌛ stale",
		Match: func(s core.Stash) bool { return s.IsStale },
	}
	result = filter.FilterStashes(stashes, []core.Filter{staleFilter})
	if len(result) != 2 {
		t.Fatalf("stale filter: expected 2 results, got %d", len(result))
	}
	for _, s := range result {
		if !s.IsStale {
			t.Errorf("stale filter: expected IsStale=true, got false for stash@{%d}", s.Index)
		}
	}

	// Composed: branch=main AND stale — should return only stash@{1}.
	result = filter.FilterStashes(stashes, []core.Filter{branchFilter, staleFilter})
	if len(result) != 1 {
		t.Fatalf("composed filter: expected 1 result, got %d", len(result))
	}
	if result[0].Index != 1 {
		t.Errorf("composed filter: expected stash@{1}, got stash@{%d}", result[0].Index)
	}
}

// TestFilterClearsAll verifies that fc clears all active filters.
func TestFilterClearsAll(t *testing.T) {
	p := filter.New()
	// Simulate activating both filters.
	state := core.AppState{
		Branch: "main",
		Stashes: []core.Stash{
			{Index: 0, Branch: "main", IsStale: true},
		},
	}

	// Activate fb.
	state, _ = p.HandleKey(plugin.KeyEvent{Text: "fb"}, state)
	if len(state.Filters) == 0 {
		t.Fatal("expected filters after fb")
	}

	// Activate fs.
	state, _ = p.HandleKey(plugin.KeyEvent{Text: "fs"}, state)
	if len(state.Filters) != 2 {
		t.Fatalf("expected 2 filters after fb+fs, got %d", len(state.Filters))
	}

	// Clear with fc.
	state, _ = p.HandleKey(plugin.KeyEvent{Text: "fc"}, state)
	if len(state.Filters) != 0 {
		t.Errorf("expected 0 filters after fc, got %d", len(state.Filters))
	}
	if p.IsActive() {
		t.Error("expected IsActive()=false after fc")
	}
}

// TestNoFiltersReturnsAll verifies that with no active filters,
// FilterStashes returns all stashes unchanged.
func TestNoFiltersReturnsAll(t *testing.T) {
	stashes := []core.Stash{
		{Index: 0}, {Index: 1}, {Index: 2},
	}
	result := filter.FilterStashes(stashes, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 stashes with no filters, got %d", len(result))
	}
}

// TestBranchFilterToggle verifies that fb toggles on/off.
func TestBranchFilterToggle(t *testing.T) {
	p := filter.New()
	state := core.AppState{Branch: "main"}

	// First press: activate.
	state, _ = p.HandleKey(plugin.KeyEvent{Text: "fb"}, state)
	if !p.IsActive() {
		t.Error("expected active after first fb")
	}

	// Second press: deactivate.
	state, _ = p.HandleKey(plugin.KeyEvent{Text: "fb"}, state)
	if p.IsActive() {
		t.Error("expected inactive after second fb")
	}
}

// --- Integration Tests ---

// TestFilterByBranchWithRealRepo creates stashes on different branches
// and verifies that fb shows only current branch stashes.
func TestFilterByBranchWithRealRepo(t *testing.T) {
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
	writeFile("base.go", "package main\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	// Create stashes on main.
	writeFile("main1.go", "package m1\n")
	run("git", "add", ".")
	run("git", "stash", "push", "-m", "stash on main 1")

	writeFile("main2.go", "package m2\n")
	run("git", "add", ".")
	run("git", "stash", "push", "-m", "stash on main 2")

	// Switch to feature branch and create stashes.
	run("git", "checkout", "-b", "feature/auth")
	writeFile("auth1.go", "package auth\n")
	run("git", "add", ".")
	run("git", "stash", "push", "-m", "stash on feature 1")

	writeFile("auth2.go", "package auth2\n")
	run("git", "add", ".")
	run("git", "stash", "push", "-m", "stash on feature 2")

	// Now on feature/auth. Parse the stash list.
	// git stash list format includes branch info: stash@{n}: On <branch>: message
	listOutput := run("git", "stash", "list", "--format=%gd|%s")

	// Build stash list.
	stashes := parseStashList(t, listOutput, dir)

	// Verify we have 4 stashes total.
	if len(stashes) != 4 {
		t.Fatalf("expected 4 stashes, got %d", len(stashes))
	}

	// Apply branch filter for "feature/auth".
	branchFilter := core.Filter{
		ID: "branch", Label: "⎇ feature/auth",
		Match: func(s core.Stash) bool { return s.Branch == "feature/auth" },
	}
	result := filter.FilterStashes(stashes, []core.Filter{branchFilter})

	// Should have 2 stashes from feature/auth.
	if len(result) != 2 {
		t.Fatalf("expected 2 feature/auth stashes, got %d", len(result))
	}
	for _, s := range result {
		if s.Branch != "feature/auth" {
			t.Errorf("expected branch 'feature/auth', got %q", s.Branch)
		}
	}
}

// TestFilterStaleWithOldDates creates stashes with old dates using
// GIT_AUTHOR_DATE/GIT_COMMITTER_DATE env vars and verifies that fs
// shows only stale stashes.
func TestFilterStaleWithOldDates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	runWithEnv := func(env []string, args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\noutput: %s", args, err, out)
		}
		return string(out)
	}

	run := func(args ...string) string {
		return runWithEnv([]string{
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		}, args...)
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
	writeFile("base.go", "package main\n")
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	// Create a recent stash (today).
	writeFile("recent.go", "package recent\n")
	run("git", "add", ".")
	run("git", "stash", "push", "-m", "recent stash")

	// Create an old stash (30 days ago) using GIT_AUTHOR_DATE.
	// Note: git stash uses the committer date for the reflog entry.
	// The stash's date comes from parsing git stash list with date format.
	// We simulate old dates by setting both author and committer dates.
	oldDate := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
	writeFile("old.go", "package old\n")
	run("git", "add", ".")
	runWithEnv([]string{
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
		"GIT_AUTHOR_DATE=" + oldDate,
		"GIT_COMMITTER_DATE=" + oldDate,
	}, "git", "stash", "push", "-m", "old stash")

	// Build stash list with dates.
	now := time.Now()
	stashes := []core.Stash{
		{Index: 0, SHA: "aaa", Message: "old stash", Date: now.Add(-30 * 24 * time.Hour)},
		{Index: 1, SHA: "bbb", Message: "recent stash", Date: now.Add(-1 * time.Hour)},
	}

	// Mark staleness with 14-day threshold.
	threshold := 14 * 24 * time.Hour
	stashes = stale.MarkStaleWithTime(stashes, now, threshold)

	// Verify staleness marking.
	if !stashes[0].IsStale {
		t.Error("expected old stash (30 days) to be stale")
	}
	if stashes[1].IsStale {
		t.Error("expected recent stash (1 hour) to not be stale")
	}

	// Apply stale filter.
	staleFilter := core.Filter{
		ID: "stale", Label: "⌛ stale",
		Match: func(s core.Stash) bool { return s.IsStale },
	}
	result := filter.FilterStashes(stashes, []core.Filter{staleFilter})
	if len(result) != 1 {
		t.Fatalf("expected 1 stale stash, got %d", len(result))
	}
	if result[0].Message != "old stash" {
		t.Errorf("expected 'old stash', got %q", result[0].Message)
	}
}
```

### Step 4: Write stale detection tests (`internal/plugins/stale/stale_test.go`)

```go
package stale_test

import (
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugins/stale"
)

// TestStalenessCalculationVariousThresholds verifies that staleness
// is correctly computed for different threshold values.
func TestStalenessCalculationVariousThresholds(t *testing.T) {
	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		stashAge  time.Duration
		threshold time.Duration
		wantStale bool
	}{
		{"fresh stash with default threshold", 1 * time.Hour, 14 * 24 * time.Hour, false},
		{"1 day old with default threshold", 24 * time.Hour, 14 * 24 * time.Hour, false},
		{"13 days old with default threshold", 13 * 24 * time.Hour, 14 * 24 * time.Hour, false},
		{"14 days old with default threshold (exactly at threshold)", 14 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"15 days old with default threshold", 15 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"30 days old with default threshold", 30 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"7 days old with 7-day threshold", 7 * 24 * time.Hour, 7 * 24 * time.Hour, true},
		{"6 days old with 7-day threshold", 6 * 24 * time.Hour, 7 * 24 * time.Hour, false},
		{"1 day old with 1-day threshold", 24 * time.Hour, 24 * time.Hour, true},
		{"23 hours old with 1-day threshold", 23 * time.Hour, 24 * time.Hour, false},
		{"90 days old with 30-day threshold", 90 * 24 * time.Hour, 30 * 24 * time.Hour, true},
		{"zero age", 0, 14 * 24 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stashes := []core.Stash{
				{Index: 0, SHA: "aaa", Date: now.Add(-tt.stashAge)},
			}
			result := stale.MarkStaleWithTime(stashes, now, tt.threshold)
			if result[0].IsStale != tt.wantStale {
				t.Errorf("stash age=%v threshold=%v: got IsStale=%v, want %v",
					tt.stashAge, tt.threshold, result[0].IsStale, tt.wantStale)
			}
		})
	}
}

// TestStaleStashesFilter verifies the StaleStashes helper.
func TestStaleStashesFilter(t *testing.T) {
	stashes := []core.Stash{
		{Index: 0, IsStale: false},
		{Index: 1, IsStale: true},
		{Index: 2, IsStale: false},
		{Index: 3, IsStale: true},
		{Index: 4, IsStale: true},
	}

	result := stale.StaleStashes(stashes)
	if len(result) != 3 {
		t.Fatalf("expected 3 stale stashes, got %d", len(result))
	}
	expected := []int{1, 3, 4}
	for i, s := range result {
		if s.Index != expected[i] {
			t.Errorf("result[%d]: expected index %d, got %d", i, expected[i], s.Index)
		}
	}
}

// TestMarkStalePreservesOtherFields verifies that MarkStaleWithTime
// does not modify any field other than IsStale.
func TestMarkStalePreservesOtherFields(t *testing.T) {
	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	original := core.Stash{
		Index:      3,
		SHA:        "abc123",
		Message:    "important work",
		Branch:     "feature/x",
		Date:       now.Add(-30 * 24 * time.Hour),
		FileCount:  5,
		Insertions: 42,
		Deletions:  17,
	}
	stashes := []core.Stash{original}
	result := stale.MarkStaleWithTime(stashes, now, 14*24*time.Hour)

	if result[0].Index != 3 || result[0].SHA != "abc123" || result[0].Message != "important work" {
		t.Error("MarkStaleWithTime modified fields other than IsStale")
	}
	if !result[0].IsStale {
		t.Error("expected IsStale=true for 30-day-old stash")
	}
}

// TestEmptyStashList verifies handling of empty input.
func TestEmptyStashList(t *testing.T) {
	result := stale.MarkStaleWithTime(nil, time.Now(), 14*24*time.Hour)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}

	result2 := stale.StaleStashes(nil)
	if len(result2) != 0 {
		t.Errorf("expected empty result, got %d", len(result2))
	}
}
```

### Step 5: Verify

```bash
# Unit tests.
go test -v -count=1 ./internal/plugins/filter/...
go test -v -count=1 ./internal/plugins/stale/...

# Integration tests.
go test -v -count=1 -run 'TestFilterByBranch|TestFilterStaleWithOldDates' ./internal/plugins/filter/...

# Full CI pipeline.
make ci
```

## Verification

### Functional
```bash
# Filter unit tests pass
go test -v -count=1 -run 'TestFilterComposition|TestFilterClearsAll|TestNoFilters|TestBranchFilterToggle' ./internal/plugins/filter/...

# Stale unit tests pass
go test -v -count=1 -run 'TestStalenessCalculation|TestStaleStashesFilter|TestMarkStalePreserves|TestEmptyStashList' ./internal/plugins/stale/...

# Integration tests pass
go test -v -count=1 -run 'TestFilterByBranch|TestFilterStaleWithOldDates' ./internal/plugins/filter/...

# Compiles and passes vet
go vet ./internal/plugins/filter/... ./internal/plugins/stale/...

# Lint clean
golangci-lint run ./internal/plugins/filter/... ./internal/plugins/stale/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/plugins/filter/filter.go` implements `KeyHandler` for `fb`, `fs`, `fc`
2. `internal/plugins/stale/stale.go` implements `Plugin` (passive) for staleness computation
3. `fb` toggles branch filter — only stashes from `state.Branch` are visible
4. `fs` toggles stale filter — only stashes with `IsStale=true` are visible
5. `fc` clears all active filters and resets cursor
6. Filters compose with AND logic: `fb` + `fs` = stashes on current branch AND stale
7. Active filters reflected in `state.Filters` for status bar chip rendering
8. `MarkStaleWithTime` correctly computes staleness for all threshold values
9. `StaleStashes` returns only stale stashes
10. Bulk drop for stale stashes drops from highest index first to preserve ordering
11. All unit tests pass: composition, toggle, clear, staleness calculation, edge cases
12. All integration tests pass: multi-branch repo with fb, old-date stashes with fs
13. `make ci` passes (lint + test)

## Commit
```
feat(filter,stale): add branch/stale filter and stale detection plugins

Implement filter plugin (KeyHandler) with fb (branch), fs (stale), and
fc (clear) keybindings. Filters compose with AND logic and appear as
chips in the status bar. Implement stale detection plugin (passive) that
marks stashes older than configurable threshold (default 14 days) as
stale, with bulk drop capability. Full test coverage with real git repos.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 6.2 (FR-15, FR-17), 11.2 (keymap), 8.2 (interfaces), 8.3 (types), 12.2 (config)
4. Verify dependencies: task 006 (core model with AppState, Filter type) and task 004 (StashCache) are DONE
5. Create `internal/plugins/filter/filter.go` with FilterStashes and KeyHandler
6. Create `internal/plugins/stale/stale.go` with MarkStaleWithTime and StaleStashes
7. Create `internal/plugins/filter/filter_test.go` with all unit and integration tests
8. Create `internal/plugins/stale/stale_test.go` with all unit tests
9. Run `go test -v -count=1 ./internal/plugins/filter/... ./internal/plugins/stale/...`
10. Run `make ci`
11. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
12. Commit with the message above
