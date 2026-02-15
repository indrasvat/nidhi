package layout

import (
	"testing"
)

func TestDetectTier(t *testing.T) {
	bp := DefaultBreakpoints()

	tests := []struct {
		name string
		w, h int
		want Tier
	}{
		{"minimal 60x20", 60, 20, TierMinimal},
		{"minimal 79x23", 79, 23, TierMinimal},
		{"standard 80x24", 80, 24, TierStandard},
		{"standard 120x40", 120, 40, TierStandard},
		{"standard 199x59", 199, 59, TierStandard},
		{"large 200x60", 200, 60, TierLarge},
		{"large 300x100", 300, 100, TierLarge},
		{"wide but short", 200, 20, TierMinimal},
		{"tall but narrow", 60, 60, TierMinimal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTier(tt.w, tt.h, bp)
			if got != tt.want {
				t.Errorf("DetectTier(%d, %d) = %s, want %s", tt.w, tt.h, got, tt.want)
			}
		})
	}
}

func TestShouldCollapseTwoLineRows(t *testing.T) {
	tests := []struct {
		width int
		want  bool
	}{
		{60, true},
		{80, true},
		{99, true},
		{100, false},
		{120, false},
		{200, false},
	}

	for _, tt := range tests {
		got := ShouldCollapseTwoLineRows(tt.width)
		if got != tt.want {
			t.Errorf("ShouldCollapseTwoLineRows(%d) = %v, want %v", tt.width, got, tt.want)
		}
	}
}

func TestShouldTruncateMessage(t *testing.T) {
	tests := []struct {
		width int
		want  bool
	}{
		{60, true},
		{79, true},
		{80, false},
		{120, false},
	}

	for _, tt := range tests {
		got := ShouldTruncateMessage(tt.width)
		if got != tt.want {
			t.Errorf("ShouldTruncateMessage(%d) = %v, want %v", tt.width, got, tt.want)
		}
	}
}

func TestMaxMessageWidth(t *testing.T) {
	tests := []struct {
		totalWidth int
		wantMin    int
	}{
		{80, 10},
		{120, 80},
		{200, 150},
		{30, 10},
	}

	for _, tt := range tests {
		got := MaxMessageWidth(tt.totalWidth)
		if got < tt.wantMin {
			t.Errorf("MaxMessageWidth(%d) = %d, want >= %d", tt.totalWidth, got, tt.wantMin)
		}
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierMinimal, "minimal"},
		{TierStandard, "standard"},
		{TierLarge, "large"},
		{Tier(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}
