package components

import (
	"strings"
	"testing"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func testStash(index int, now time.Time) plugin.Stash {
	return plugin.Stash{
		Index:        index,
		SHA:          "a3f7b2c1234567890abcdef1234567890abcdef0",
		ShortSHA:     "a3f7b2c",
		Message:      "Fix auth token refresh",
		RawMessage:   "WIP on main: a3f7b2c Fix auth token refresh",
		Branch:       "main",
		Date:         now.Add(-3 * time.Hour),
		FileCount:    3,
		Insertions:   42,
		Deletions:    17,
		IsStale:      false,
		HasUntracked: false,
	}
}

func staleStash(index int, now time.Time) plugin.Stash {
	s := testStash(index, now)
	s.Date = now.Add(-30 * 24 * time.Hour)
	s.IsStale = true
	s.Message = "Old forgotten stash"
	return s
}

func TestStashRow_ContainsExpectedElements(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(rendered)

	checks := []struct {
		name     string
		contains string
	}{
		{"index", "0"},
		{"SHA", "a3f7b2c"},
		{"message", "Fix auth token refresh"},
		{"age", "3h ago"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(plain, c.contains) {
				t.Errorf("row should contain %q, got:\n%s", c.contains, plain)
			}
		})
	}
}

func TestStashRow_SelectedState(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	selected := r.Render(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   true,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(selected)

	if !strings.Contains(plain, "\u25b8") { // ▸
		t.Error("selected row should have cursor indicator \u25b8")
	}
}

func TestStashRow_UnselectedNoCursor(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	unselected := r.Render(StashRowParams{
		Stash:      testStash(1, now),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(unselected)
	if strings.Contains(plain, "\u25b8") {
		t.Error("unselected row should NOT have cursor indicator")
	}
}

func TestStashRow_StaleBadge(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      staleStash(2, now),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "STALE") {
		t.Error("stale stash should have STALE badge")
	}
}

func TestStashRow_NoStaleBadgeWhenNotStale(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(rendered)
	if strings.Contains(plain, "STALE") {
		t.Error("non-stale stash should NOT have STALE badge")
	}
}

func TestStashRow_SingleLineBelow100Cols(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   false,
		Width:      80,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) != 1 {
		t.Errorf("below 100 cols: expected 1 line, got %d lines", len(lines))
	}
}

func TestStashRow_TwoLinesAbove100Cols(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) != 2 {
		t.Errorf("above 100 cols: expected 2 lines, got %d", len(lines))
	}
}

func TestStashRow_SecondLineContainsDiffStat(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatal("expected 2 lines")
	}

	plain := stripAnsi(lines[1])
	if !strings.Contains(plain, "+42") {
		t.Errorf("second line should contain insertions '+42', got: %q", plain)
	}
	if !strings.Contains(plain, "-17") {
		t.Errorf("second line should contain deletions '-17', got: %q", plain)
	}
}

func TestStashRow_SecondLineContainsBranch(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatal("expected 2 lines")
	}

	plain := stripAnsi(lines[1])
	if !strings.Contains(plain, "main") {
		t.Errorf("second line should contain branch 'main', got: %q", plain)
	}
}

func TestStashRow_ProgressiveDimming(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())

	factor0 := r.dimmingFactor(0, 10)
	if factor0 != 0 {
		t.Errorf("dimmingFactor(0, 10) = %f, want 0", factor0)
	}

	factorLast := r.dimmingFactor(9, 10)
	if factorLast <= 0 {
		t.Errorf("dimmingFactor(9, 10) = %f, want > 0", factorLast)
	}
	if factorLast > 0.61 {
		t.Errorf("dimmingFactor(9, 10) = %f, should be capped at ~0.6", factorLast)
	}

	factorMid := r.dimmingFactor(5, 10)
	if factorMid <= factor0 || factorMid >= factorLast {
		t.Errorf("dimmingFactor(5, 10) = %f, should be between %f and %f",
			factorMid, factor0, factorLast)
	}
}

func TestStashRow_DimmingFactorSingleStash(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	factor := r.dimmingFactor(0, 1)
	if factor != 0 {
		t.Errorf("dimmingFactor(0, 1) = %f, want 0", factor)
	}
}

func TestStashRow_InlineRename(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	rendered := r.InlineRenameRow(StashRowParams{
		Stash:      testStash(0, now),
		Selected:   true,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	}, "New message text", 16)

	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "New message text") {
		t.Error("inline rename should show the edit text")
	}
	if !strings.Contains(plain, "was:") {
		t.Error("inline rename should show the previous message")
	}
}

func TestRelativeAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "now"},
		{"minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"hours", now.Add(-3 * time.Hour), "3h ago"},
		{"days", now.Add(-2 * 24 * time.Hour), "2d ago"},
		{"months", now.Add(-45 * 24 * time.Hour), "1mo ago"},
		{"years", now.Add(-400 * 24 * time.Hour), "1y ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeAge(tt.date, now)
			if got != tt.want {
				t.Errorf("relativeAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBlendColor(t *testing.T) {
	white := lipgloss.Color("#FFFFFF")
	black := lipgloss.Color("#000000")

	// Factor 0 should return the first color.
	c0 := blendColor(white, black, 0.0)
	r0, g0, b0, _ := c0.RGBA()
	if r0>>8 != 255 || g0>>8 != 255 || b0>>8 != 255 {
		t.Errorf("blendColor(white, black, 0) = (%d,%d,%d), want (255,255,255)",
			r0>>8, g0>>8, b0>>8)
	}

	// Factor 1 should return the second color.
	c1 := blendColor(white, black, 1.0)
	r1, g1, b1, _ := c1.RGBA()
	if r1>>8 != 0 || g1>>8 != 0 || b1>>8 != 0 {
		t.Errorf("blendColor(white, black, 1) = (%d,%d,%d), want (0,0,0)",
			r1>>8, g1>>8, b1>>8)
	}

	// Factor 0.5 should be a middle gray.
	cMid := blendColor(white, black, 0.5)
	rm, gm, bm, _ := cMid.RGBA()
	midR := rm >> 8
	if midR < 120 || midR > 135 {
		t.Errorf("blendColor(white, black, 0.5) red = %d, want ~127", midR)
	}
	if gm>>8 < 120 || gm>>8 > 135 {
		t.Errorf("blendColor(white, black, 0.5) green = %d, want ~127", gm>>8)
	}
	if bm>>8 < 120 || bm>>8 > 135 {
		t.Errorf("blendColor(white, black, 0.5) blue = %d, want ~127", bm>>8)
	}
}

func TestStashRow_WidthConstraint(t *testing.T) {
	r := NewStashRowRenderer(theme.NewAgni())
	now := time.Now()

	widths := []int{60, 80, 100, 120, 200}

	for _, w := range widths {
		rendered := r.Render(StashRowParams{
			Stash:      testStash(0, now),
			Selected:   false,
			Width:      w,
			UseNerd:    false,
			TotalCount: 5,
			Now:        now,
		})

		lines := strings.Split(rendered, "\n")
		for i, line := range lines {
			lineWidth := lipgloss.Width(line)
			if lineWidth > w+2 {
				t.Errorf("width=%d, line %d: rendered width = %d, exceeds target",
					w, i, lineWidth)
			}
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxWidth int
		want     string
	}{
		{"hello world", 5, "hello"},
		{"short", 10, "short"},
		{"", 5, ""},
		{"abc", 0, ""},
		{"unicode \u00e4\u00f6\u00fc", 9, "unicode \u00e4"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q",
					tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}
