package layout

// Tier represents a responsive layout tier.
type Tier int

const (
	// TierMinimal is for terminals 80x24 and below. Single-line stash rows,
	// truncated messages, no split panes.
	TierMinimal Tier = iota
	// TierStandard is for terminals between 80x24 and 120x40. Two-line stash rows,
	// preview split available.
	TierStandard
	// TierLarge is for terminals 200x60 and above. Full detail, generous spacing.
	TierLarge
)

// String returns the tier name.
func (t Tier) String() string {
	switch t {
	case TierMinimal:
		return "minimal"
	case TierStandard:
		return "standard"
	case TierLarge:
		return "large"
	default:
		return "unknown"
	}
}

// Breakpoints defines the width and height thresholds for each tier.
type Breakpoints struct {
	MinimalMaxWidth   int
	StandardMinWidth  int
	LargeMinWidth     int
	MinimalMaxHeight  int
	StandardMinHeight int
	LargeMinHeight    int
}

// DefaultBreakpoints returns the default breakpoints from PRD Section 10.
func DefaultBreakpoints() Breakpoints {
	return Breakpoints{
		MinimalMaxWidth:   79,
		StandardMinWidth:  80,
		LargeMinWidth:     200,
		MinimalMaxHeight:  23,
		StandardMinHeight: 24,
		LargeMinHeight:    60,
	}
}

// DetectTier determines the responsive tier for the given terminal dimensions.
func DetectTier(width, height int, bp Breakpoints) Tier {
	if width >= bp.LargeMinWidth && height >= bp.LargeMinHeight {
		return TierLarge
	}
	if width >= bp.StandardMinWidth && height >= bp.StandardMinHeight {
		return TierStandard
	}
	return TierMinimal
}

// ShouldCollapseTwoLineRows returns true if stash rows should collapse to single-line.
// PRD Screen 1: "Below 100 cols, collapse to single-line rows."
func ShouldCollapseTwoLineRows(width int) bool {
	return width < 100
}

// ShouldTruncateMessage returns true if stash messages should be truncated.
// PRD Screen 1: "Below 80 cols, truncate message."
func ShouldTruncateMessage(width int) bool {
	return width < 80
}

// MaxMessageWidth returns the maximum width available for stash messages
// given the total terminal width and fixed column widths.
// Fixed columns: cursor (2) + index (4) + SHA (9) + age (8) + padding (4) = ~27
func MaxMessageWidth(totalWidth int) int {
	const fixedColumns = 27
	available := totalWidth - fixedColumns
	if available < 10 {
		return 10
	}
	return available
}
