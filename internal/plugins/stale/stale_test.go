package stale_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/stale"
)

func TestStalenessCalculation(t *testing.T) {
	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		stashAge  time.Duration
		threshold time.Duration
		wantStale bool
	}{
		{"fresh 1 hour", 1 * time.Hour, 14 * 24 * time.Hour, false},
		{"1 day", 24 * time.Hour, 14 * 24 * time.Hour, false},
		{"13 days", 13 * 24 * time.Hour, 14 * 24 * time.Hour, false},
		{"14 days (at threshold)", 14 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"15 days", 15 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"30 days", 30 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"7 days with 7-day threshold", 7 * 24 * time.Hour, 7 * 24 * time.Hour, true},
		{"6 days with 7-day threshold", 6 * 24 * time.Hour, 7 * 24 * time.Hour, false},
		{"1 day with 1-day threshold", 24 * time.Hour, 24 * time.Hour, true},
		{"23 hours with 1-day threshold", 23 * time.Hour, 24 * time.Hour, false},
		{"90 days with 30-day threshold", 90 * 24 * time.Hour, 30 * 24 * time.Hour, true},
		{"zero age", 0, 14 * 24 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stashes := []plugin.Stash{
				{Index: 0, SHA: "aaa", Date: now.Add(-tt.stashAge)},
			}
			result := stale.MarkStaleWithTime(stashes, now, tt.threshold)
			if result[0].IsStale != tt.wantStale {
				t.Errorf("age=%v threshold=%v: got IsStale=%v, want %v",
					tt.stashAge, tt.threshold, result[0].IsStale, tt.wantStale)
			}
		})
	}
}

func TestStaleStashesFilter(t *testing.T) {
	stashes := []plugin.Stash{
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

func TestStaleCount(t *testing.T) {
	stashes := []plugin.Stash{
		{Index: 0, IsStale: true},
		{Index: 1, IsStale: false},
		{Index: 2, IsStale: true},
	}
	count := stale.StaleCount(stashes)
	if count != 2 {
		t.Errorf("expected 2 stale, got %d", count)
	}
}

func TestMarkStalePreservesOtherFields(t *testing.T) {
	now := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)
	stashes := []plugin.Stash{
		{
			Index:      3,
			SHA:        "abc123",
			Message:    "important work",
			Branch:     "feature/x",
			Date:       now.Add(-30 * 24 * time.Hour),
			FileCount:  5,
			Insertions: 42,
			Deletions:  17,
		},
	}
	result := stale.MarkStaleWithTime(stashes, now, 14*24*time.Hour)

	s := result[0]
	if s.Index != 3 || s.SHA != "abc123" || s.Message != "important work" {
		t.Error("MarkStaleWithTime modified fields other than IsStale")
	}
	if s.Branch != "feature/x" || s.FileCount != 5 || s.Insertions != 42 || s.Deletions != 17 {
		t.Error("MarkStaleWithTime modified fields other than IsStale")
	}
	if !s.IsStale {
		t.Error("expected IsStale=true for 30-day-old stash")
	}
}

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

func TestMarkStale_MultipleStashes(t *testing.T) {
	now := time.Now()
	threshold := 7 * 24 * time.Hour

	stashes := []plugin.Stash{
		{Index: 0, Date: now.Add(-1 * time.Hour)},
		{Index: 1, Date: now.Add(-3 * 24 * time.Hour)},
		{Index: 2, Date: now.Add(-7 * 24 * time.Hour)},
		{Index: 3, Date: now.Add(-10 * 24 * time.Hour)},
		{Index: 4, Date: now.Add(-30 * 24 * time.Hour)},
	}

	result := stale.MarkStaleWithTime(stashes, now, threshold)
	if result[0].IsStale || result[1].IsStale {
		t.Error("fresh stashes should not be stale")
	}
	if !result[2].IsStale || !result[3].IsStale || !result[4].IsStale {
		t.Error("old stashes should be stale")
	}
}

func TestPlugin_DefaultThreshold(t *testing.T) {
	p := stale.New()
	expected := time.Duration(stale.DefaultDays) * 24 * time.Hour
	if p.Threshold() != expected {
		t.Errorf("expected default threshold %v, got %v", expected, p.Threshold())
	}
}

func TestPlugin_SetThreshold(t *testing.T) {
	p := stale.New()
	custom := 7 * 24 * time.Hour
	p.SetThreshold(custom)
	if p.Threshold() != custom {
		t.Errorf("expected threshold %v, got %v", custom, p.Threshold())
	}
}

func TestPlugin_Init(t *testing.T) {
	p := stale.New()
	pctx := plugin.PluginContext{
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init stale plugin: %v", err)
	}
	if p.ID() != "stale" {
		t.Errorf("expected ID 'stale', got %q", p.ID())
	}
}

func TestPlugin_MarkStale(t *testing.T) {
	p := stale.New()
	p.SetThreshold(7 * 24 * time.Hour)

	stashes := []plugin.Stash{
		{Index: 0, Date: time.Now().Add(-1 * time.Hour)},
		{Index: 1, Date: time.Now().Add(-10 * 24 * time.Hour)},
	}

	result := p.MarkStale(stashes)
	if result[0].IsStale {
		t.Error("expected fresh stash to not be stale")
	}
	if !result[1].IsStale {
		t.Error("expected old stash to be stale")
	}
}
