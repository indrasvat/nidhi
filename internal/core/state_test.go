package core

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestNewAppState(t *testing.T) {
	ver := GitVersion{Major: 2, Minor: 53, Raw: "git version 2.53.0"}
	s := NewAppState("/path/to/repo", "main", ver)
	if s.Mode != ModeList {
		t.Errorf("Mode = %s", s.Mode)
	}
	if s.Width != 80 || s.Height != 24 {
		t.Errorf("dimensions = %dx%d", s.Width, s.Height)
	}
	if s.RepoPath != "/path/to/repo" || s.Branch != "main" {
		t.Errorf("RepoPath=%q Branch=%q", s.RepoPath, s.Branch)
	}
}

func TestWithStashes(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	stashes := []plugin.Stash{
		{Index: 0, SHA: "aaa"}, {Index: 1, SHA: "bbb"}, {Index: 2, SHA: "ccc"},
	}
	s = WithStashes(s, stashes)
	if len(s.Stashes) != 3 || s.Stashes[0].SHA != "aaa" {
		t.Errorf("stashes = %v", s.Stashes)
	}
}

func TestWithStashes_ClampsCursor(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s.Cursor = 5
	s = WithStashes(s, []plugin.Stash{{Index: 0}, {Index: 1}})
	if s.Cursor != 1 {
		t.Errorf("Cursor = %d, want 1", s.Cursor)
	}
}

func TestWithStashes_EmptyClamps(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s.Cursor = 3
	s = WithStashes(s, nil)
	if s.Cursor != 0 {
		t.Errorf("Cursor = %d, want 0", s.Cursor)
	}
}

func TestWithCursor(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		cursor int
		want   int
	}{
		{"normal", 5, 2, 2},
		{"clamp below", 5, -1, 0},
		{"clamp above", 5, 10, 4},
		{"empty", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewAppState("/repo", "main", GitVersion{})
			stashes := make([]plugin.Stash, tt.count)
			s = WithStashes(s, stashes)
			s = WithCursor(s, tt.cursor)
			if s.Cursor != tt.want {
				t.Errorf("Cursor = %d, want %d", s.Cursor, tt.want)
			}
		})
	}
}

func TestWithSize(t *testing.T) {
	s := WithSize(NewAppState("/repo", "main", GitVersion{}), 200, 60)
	if s.Width != 200 || s.Height != 60 {
		t.Errorf("dimensions = %dx%d", s.Width, s.Height)
	}
}

func TestWithMode(t *testing.T) {
	s := WithMode(NewAppState("/repo", "main", GitVersion{}), ModePreview)
	if s.Mode != ModePreview {
		t.Errorf("Mode = %s", s.Mode)
	}
}

func TestSelectedStash(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	s = WithStashes(s, []plugin.Stash{{Index: 0, SHA: "aaa"}, {Index: 1, SHA: "bbb"}})
	s = WithCursor(s, 1)
	got := SelectedStash(s)
	if got == nil || got.SHA != "bbb" {
		t.Errorf("SelectedStash = %v", got)
	}
}

func TestSelectedStash_Empty(t *testing.T) {
	s := NewAppState("/repo", "main", GitVersion{})
	if SelectedStash(s) != nil {
		t.Error("should be nil for empty stashes")
	}
}

func TestStateImmutability(t *testing.T) {
	s1 := WithStashes(NewAppState("/repo", "main", GitVersion{}), []plugin.Stash{{Index: 0}})
	s2 := WithCursor(s1, 0)
	_ = WithMode(s2, ModePreview)
	if s1.Mode != ModeList {
		t.Error("original state mutated")
	}
}

func TestWithFilters(t *testing.T) {
	s := WithFilters(NewAppState("/repo", "main", GitVersion{}), []Filter{{ID: "branch", Label: "main"}})
	if len(s.Filters) != 1 || s.Filters[0].ID != "branch" {
		t.Errorf("Filters = %v", s.Filters)
	}
}

func TestWithSearchQuery(t *testing.T) {
	s := WithSearchQuery(NewAppState("/repo", "main", GitVersion{}), "token")
	if s.SearchQuery != "token" {
		t.Errorf("SearchQuery = %q", s.SearchQuery)
	}
}
