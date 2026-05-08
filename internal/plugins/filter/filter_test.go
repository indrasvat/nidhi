package filter_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/filter"
	"github.com/indrasvat/nidhi/internal/plugins/stale"
)

func newTestPlugin(t *testing.T) *filter.Plugin {
	t.Helper()
	p := filter.New()
	pctx := plugin.PluginContext{
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init filter plugin: %v", err)
	}
	return p
}

// ─── FilterStashes Unit Tests ───────────────────────────────

func TestFilterStashes_NoFilters(t *testing.T) {
	stashes := []plugin.Stash{
		{Index: 0}, {Index: 1}, {Index: 2},
	}
	result := filter.FilterStashes(stashes, nil)
	if len(result) != 3 {
		t.Errorf("expected 3 stashes with no filters, got %d", len(result))
	}
}

func TestFilterStashes_BranchFilter(t *testing.T) {
	stashes := []plugin.Stash{
		{Index: 0, Branch: "main"},
		{Index: 1, Branch: "feature/auth"},
		{Index: 2, Branch: "main"},
		{Index: 3, Branch: "feature/auth"},
	}

	filters := []plugin.Filter{
		{ID: filter.FilterIDBranch, Label: "⎇ main", Value: "main"},
	}
	result := filter.FilterStashes(stashes, filters)
	if len(result) != 2 {
		t.Fatalf("expected 2 main stashes, got %d", len(result))
	}
	for _, s := range result {
		if s.Branch != "main" {
			t.Errorf("expected branch 'main', got %q", s.Branch)
		}
	}
}

func TestFilterStashes_StaleFilter(t *testing.T) {
	stashes := []plugin.Stash{
		{Index: 0, IsStale: false},
		{Index: 1, IsStale: true},
		{Index: 2, IsStale: false},
		{Index: 3, IsStale: true},
	}

	filters := []plugin.Filter{
		{ID: filter.FilterIDStale, Label: "⌛ stale", Value: "true"},
	}
	result := filter.FilterStashes(stashes, filters)
	if len(result) != 2 {
		t.Fatalf("expected 2 stale stashes, got %d", len(result))
	}
	for _, s := range result {
		if !s.IsStale {
			t.Errorf("expected IsStale=true for stash@{%d}", s.Index)
		}
	}
}

func TestFilterStashes_Composition(t *testing.T) {
	now := time.Now()
	stashes := []plugin.Stash{
		{Index: 0, Branch: "main", Date: now, IsStale: false},
		{Index: 1, Branch: "main", Date: now.Add(-30 * 24 * time.Hour), IsStale: true},
		{Index: 2, Branch: "feature", Date: now, IsStale: false},
		{Index: 3, Branch: "feature", Date: now.Add(-30 * 24 * time.Hour), IsStale: true},
	}

	// Branch=main AND stale → only stash@{1}.
	filters := []plugin.Filter{
		{ID: filter.FilterIDBranch, Value: "main"},
		{ID: filter.FilterIDStale, Value: "true"},
	}
	result := filter.FilterStashes(stashes, filters)
	if len(result) != 1 {
		t.Fatalf("composed filter: expected 1 result, got %d", len(result))
	}
	if result[0].Index != 1 {
		t.Errorf("composed filter: expected stash@{1}, got stash@{%d}", result[0].Index)
	}
}

func TestFilterStashes_EmptyList(t *testing.T) {
	filters := []plugin.Filter{{ID: filter.FilterIDBranch, Value: "main"}}
	result := filter.FilterStashes(nil, filters)
	if len(result) != 0 {
		t.Errorf("expected 0 results for empty stash list, got %d", len(result))
	}
}

func TestFilterStashes_NoMatches(t *testing.T) {
	stashes := []plugin.Stash{
		{Index: 0, Branch: "main"},
		{Index: 1, Branch: "main"},
	}
	filters := []plugin.Filter{{ID: filter.FilterIDBranch, Value: "feature"}}
	result := filter.FilterStashes(stashes, filters)
	if len(result) != 0 {
		t.Errorf("expected 0 results when no stashes match, got %d", len(result))
	}
}

// ─── Plugin KeyHandler Tests ────────────────────────────────

func TestPlugin_KeyBindings(t *testing.T) {
	p := newTestPlugin(t)
	bindings := p.KeyBindings()
	if len(bindings) != 2 {
		t.Fatalf("expected 2 keybindings, got %d", len(bindings))
	}
	if bindings[0].Key != "f" {
		t.Errorf("expected first key 'f', got %q", bindings[0].Key)
	}
	if bindings[1].Key != "F" {
		t.Errorf("expected second key 'F', got %q", bindings[1].Key)
	}
}

func TestPlugin_BranchFilterToggle(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Branch: "main"}

	// First press: activate.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	if !p.IsBranchActive() {
		t.Error("expected branch filter active after first 'f'")
	}
	if len(state.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(state.Filters))
	}
	if state.Filters[0].ID != filter.FilterIDBranch {
		t.Errorf("expected filter ID 'branch', got %q", state.Filters[0].ID)
	}
	if state.Filters[0].Value != "main" {
		t.Errorf("expected filter value 'main', got %q", state.Filters[0].Value)
	}

	// Second press: deactivate.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	if p.IsBranchActive() {
		t.Error("expected branch filter inactive after second 'f'")
	}
	if len(state.Filters) != 0 {
		t.Errorf("expected 0 filters, got %d", len(state.Filters))
	}
}

func TestPlugin_StaleFilterToggle(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{}

	// Activate stale filter.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "F"}, state)
	if !p.IsStaleActive() {
		t.Error("expected stale filter active after 'F'")
	}
	if len(state.Filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(state.Filters))
	}
	if state.Filters[0].ID != filter.FilterIDStale {
		t.Errorf("expected filter ID 'stale', got %q", state.Filters[0].ID)
	}

	// Deactivate.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "F"}, state)
	if p.IsStaleActive() {
		t.Error("expected stale filter inactive after second 'F'")
	}
}

func TestPlugin_BothFiltersActive(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Branch: "main"}

	// Activate both.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "F"}, state)

	if !p.IsActive() {
		t.Error("expected IsActive()=true with both filters")
	}
	if len(state.Filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(state.Filters))
	}
}

func TestPlugin_CursorResetOnFilterChange(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{
		Branch: "main",
		Cursor: 5,
		Stashes: []plugin.Stash{
			{Index: 0, Branch: "main"},
			{Index: 1, Branch: "main"},
		},
	}

	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	if state.Cursor != 0 {
		t.Errorf("expected cursor reset to 0 on filter change, got %d", state.Cursor)
	}
}

func TestPlugin_ClearAll(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Branch: "main"}

	// Activate both.
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "f"}, state)
	_, _ = p.HandleKey(plugin.KeyEvent{Key: "F"}, state)

	p.ClearAll()
	if p.IsActive() {
		t.Error("expected IsActive()=false after ClearAll")
	}
}

func TestPlugin_UnknownKeyIgnored(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Branch: "main"}

	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "x"}, state)
	if cmd != nil {
		t.Error("expected nil cmd for unknown key")
	}
	if len(newState.Filters) != 0 {
		t.Error("expected no filters for unknown key")
	}
}

// ─── Integration with Stale Marking ─────────────────────────

func TestIntegration_StaleFilterWithMarkedStashes(t *testing.T) {
	now := time.Now()
	threshold := 14 * 24 * time.Hour

	stashes := []plugin.Stash{
		{Index: 0, SHA: "a", Branch: "main", Message: "fresh", Date: now.Add(-1 * time.Hour)},
		{Index: 1, SHA: "b", Branch: "main", Message: "old", Date: now.Add(-30 * 24 * time.Hour)},
		{Index: 2, SHA: "c", Branch: "feature", Message: "recent", Date: now.Add(-2 * 24 * time.Hour)},
		{Index: 3, SHA: "d", Branch: "feature", Message: "ancient", Date: now.Add(-60 * 24 * time.Hour)},
	}

	// Mark staleness.
	stashes = stale.MarkStaleWithTime(stashes, now, threshold)

	// Branch filter: main only.
	branchFilters := []plugin.Filter{{ID: filter.FilterIDBranch, Value: "main"}}
	result := filter.FilterStashes(stashes, branchFilters)
	if len(result) != 2 {
		t.Fatalf("branch filter: expected 2, got %d", len(result))
	}

	// Stale filter only.
	staleFilters := []plugin.Filter{{ID: filter.FilterIDStale, Value: "true"}}
	result = filter.FilterStashes(stashes, staleFilters)
	if len(result) != 2 {
		t.Fatalf("stale filter: expected 2, got %d", len(result))
	}
	for _, s := range result {
		if !s.IsStale {
			t.Errorf("expected IsStale=true for stash@{%d}", s.Index)
		}
	}

	// Both: main AND stale → stash@{1} only.
	bothFilters := []plugin.Filter{
		{ID: filter.FilterIDBranch, Value: "main"},
		{ID: filter.FilterIDStale, Value: "true"},
	}
	result = filter.FilterStashes(stashes, bothFilters)
	if len(result) != 1 {
		t.Fatalf("composed: expected 1, got %d", len(result))
	}
	if result[0].Index != 1 {
		t.Errorf("composed: expected stash@{1}, got stash@{%d}", result[0].Index)
	}
}
