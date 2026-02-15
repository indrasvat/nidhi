package layout

import (
	"testing"
)

func TestComputeDimensions(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		wantContent   int
		wantTier      Tier
	}{
		{"standard 80x24", 80, 24, 22, TierStandard},
		{"standard 120x40", 120, 40, 38, TierStandard},
		{"large 200x60", 200, 60, 58, TierLarge},
		{"minimal 60x20", 60, 20, 18, TierMinimal},
		{"tiny 40x10", 40, 10, 8, TierMinimal},
		{"zero height", 80, 0, 0, TierMinimal},
		{"height 1", 80, 1, 0, TierMinimal},
		{"height 2", 80, 2, 0, TierMinimal},
		{"height 3", 80, 3, 1, TierMinimal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ComputeDimensions(tt.width, tt.height)
			if d.ContentHeight != tt.wantContent {
				t.Errorf("ContentHeight = %d, want %d", d.ContentHeight, tt.wantContent)
			}
			if d.ContentWidth != tt.width {
				t.Errorf("ContentWidth = %d, want %d", d.ContentWidth, tt.width)
			}
			if d.Tier != tt.wantTier {
				t.Errorf("Tier = %s, want %s", d.Tier, tt.wantTier)
			}
		})
	}
}

func TestChromeHeight(t *testing.T) {
	if ChromeHeight != 2 {
		t.Errorf("ChromeHeight = %d, want 2", ChromeHeight)
	}
}

func TestComputeDimensions_ContentNeverNegative(t *testing.T) {
	for _, h := range []int{0, 1, -1} {
		d := ComputeDimensions(80, h)
		if d.ContentHeight < 0 {
			t.Errorf("ContentHeight should never be negative for height=%d, got %d", h, d.ContentHeight)
		}
	}
}
