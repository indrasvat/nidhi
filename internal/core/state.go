package core

import "github.com/indrasvat/nidhi/internal/plugin"

// AppState is a type alias for plugin.AppState.
type AppState = plugin.AppState

// Stash is a type alias for plugin.Stash.
type Stash = plugin.Stash

// Filter is a type alias for plugin.Filter.
type Filter = plugin.Filter

// GitVersion is a type alias for plugin.GitVersion.
type GitVersion = plugin.GitVersion

// NewAppState creates an AppState with sensible defaults.
func NewAppState(repoPath, branch string, gitVer GitVersion) AppState {
	return AppState{
		Mode:       ModeList,
		Cursor:     0,
		Width:      80,
		Height:     24,
		GitVersion: gitVer,
		RepoPath:   repoPath,
		Branch:     branch,
	}
}

// WithStashes returns a copy with updated stashes. Cursor is clamped.
func WithStashes(s AppState, stashes []Stash) AppState {
	s.Stashes = stashes
	if s.Cursor >= len(stashes) {
		s.Cursor = max(0, len(stashes)-1)
	}
	return s
}

// WithCursor returns a copy with the cursor clamped to [0, len(Stashes)-1].
func WithCursor(s AppState, cursor int) AppState {
	if len(s.Stashes) == 0 {
		s.Cursor = 0
		return s
	}
	s.Cursor = clamp(cursor, 0, len(s.Stashes)-1)
	return s
}

// WithSize returns a copy with updated terminal dimensions.
func WithSize(s AppState, width, height int) AppState {
	s.Width = width
	s.Height = height
	return s
}

// WithMode returns a copy with the mode changed.
func WithMode(s AppState, mode Mode) AppState {
	s.Mode = mode
	return s
}

// WithFilters returns a copy with updated filters.
func WithFilters(s AppState, filters []Filter) AppState {
	s.Filters = filters
	return s
}

// WithSearchQuery returns a copy with updated search query.
func WithSearchQuery(s AppState, query string) AppState {
	s.SearchQuery = query
	return s
}

// WithRepoInfo returns a copy with updated repository metadata.
func WithRepoInfo(s AppState, info plugin.RepoInfo) AppState {
	s.RepoInfo = info
	return s
}

// SelectedStash returns the currently selected stash, or nil if none.
func SelectedStash(s AppState) *Stash {
	if len(s.Stashes) == 0 || s.Cursor < 0 || s.Cursor >= len(s.Stashes) {
		return nil
	}
	stash := s.Stashes[s.Cursor]
	return &stash
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
