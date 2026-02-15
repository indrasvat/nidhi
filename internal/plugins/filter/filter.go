package filter

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID   = "filter"
	PluginName = "Branch & Stale Filter"

	FilterIDBranch = "branch"
	FilterIDStale  = "stale"
)

// Plugin implements KeyHandler for filter toggling.
//
// Keybindings:
//   - f: toggle branch filter (show only stashes from state.Branch)
//   - F: toggle stale filter (show only stashes with IsStale=true)
//
// PRD specifies fb/fs/fc two-key sequences, but the current key dispatcher
// doesn't support multi-key accumulation. Using f/F until that's added.
type Plugin struct {
	logger        *slog.Logger
	activeBranch  bool
	activeStale   bool
	currentBranch string
}

var _ plugin.KeyHandler = (*Plugin)(nil)

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.logger = ctx.Logger
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns the filter keybindings.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "f", Desc: "Filter: current branch", Modes: []plugin.Mode{plugin.ModeList, plugin.ModePreview}, Priority: 90},
		{Key: "F", Desc: "Filter: stale stashes", Modes: []plugin.Mode{plugin.ModeList, plugin.ModePreview}, Priority: 90},
	}
}

// HandleKey processes filter key events.
func (p *Plugin) HandleKey(key plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch key.Key {
	case "f":
		p.activeBranch = !p.activeBranch
		p.currentBranch = state.Branch
		state = p.applyFilters(state)
		return state, nil

	case "F":
		p.activeStale = !p.activeStale
		state = p.applyFilters(state)
		return state, nil
	}

	return state, nil
}

// applyFilters computes the active filter list and stores it in AppState.
func (p *Plugin) applyFilters(state plugin.AppState) plugin.AppState {
	state.Filters = nil

	if p.activeBranch {
		state.Filters = append(state.Filters, plugin.Filter{
			ID:    FilterIDBranch,
			Label: "⎇ " + p.currentBranch,
			Value: p.currentBranch,
		})
	}

	if p.activeStale {
		state.Filters = append(state.Filters, plugin.Filter{
			ID:    FilterIDStale,
			Label: "⌛ stale",
			Value: "true",
		})
	}

	state.Cursor = 0
	return state
}

// IsActive returns true if any filter is currently active.
func (p *Plugin) IsActive() bool {
	return p.activeBranch || p.activeStale
}

// IsBranchActive returns true if the branch filter is active.
func (p *Plugin) IsBranchActive() bool {
	return p.activeBranch
}

// IsStaleActive returns true if the stale filter is active.
func (p *Plugin) IsStaleActive() bool {
	return p.activeStale
}

// ClearAll deactivates all filters.
func (p *Plugin) ClearAll() {
	p.activeBranch = false
	p.activeStale = false
}

// FilterStashes applies all active filters to a stash list and returns
// only the stashes that match ALL active filters (AND composition).
// Filters are interpreted by their ID:
//   - "branch": matches stashes where Branch == filter.Value
//   - "stale": matches stashes where IsStale == true
func FilterStashes(stashes []plugin.Stash, filters []plugin.Filter) []plugin.Stash {
	if len(filters) == 0 {
		return stashes
	}

	var result []plugin.Stash
	for _, s := range stashes {
		if matchesAll(s, filters) {
			result = append(result, s)
		}
	}
	return result
}

func matchesAll(s plugin.Stash, filters []plugin.Filter) bool {
	for _, f := range filters {
		switch f.ID {
		case FilterIDBranch:
			if s.Branch != f.Value {
				return false
			}
		case FilterIDStale:
			if !s.IsStale {
				return false
			}
		}
	}
	return true
}
