package layout

// SplitOrientation defines the direction of a split pane.
type SplitOrientation int

const (
	// SplitVertical splits the content area top/bottom (for PREVIEW mode).
	SplitVertical SplitOrientation = iota
	// SplitHorizontal splits the content area left/right (for DETAIL mode).
	SplitHorizontal
)

// SplitRatio defines a proportional split between two panes.
type SplitRatio struct {
	// Primary is the fraction of space for the primary pane (0.0 to 1.0).
	Primary float64
	// MinPrimary is the minimum size (rows or cols) for the primary pane.
	MinPrimary int
	// MinSecondary is the minimum size (rows or cols) for the secondary pane.
	MinSecondary int
}

// PreviewSplitRatio is the default split for PREVIEW mode.
// PRD Screen 2: list compresses to ~40% of height.
var PreviewSplitRatio = SplitRatio{
	Primary:      0.40,
	MinPrimary:   3,
	MinSecondary: 5,
}

// DetailSplitRatio is the default split for DETAIL mode.
// PRD Screen 3: file tree ~25% width, diff ~75% width.
var DetailSplitRatio = SplitRatio{
	Primary:      0.25,
	MinPrimary:   15,
	MinSecondary: 40,
}

// SplitResult holds the computed dimensions of a two-pane split.
type SplitResult struct {
	PrimarySize   int
	SecondarySize int
	// DividerSize is 1 when both panes fit, 0 when collapsed.
	DividerSize int
}

// ComputeSplit calculates the sizes of two panes given the total available space
// and a split ratio. The divider takes 1 unit of space.
func ComputeSplit(totalSize int, ratio SplitRatio) SplitResult {
	const dividerSize = 1

	usable := totalSize - dividerSize
	if usable < ratio.MinPrimary+ratio.MinSecondary {
		// Not enough space for both panes + divider. Collapse secondary.
		return SplitResult{
			PrimarySize:   totalSize,
			SecondarySize: 0,
			DividerSize:   0,
		}
	}

	primarySize := max(int(float64(usable)*ratio.Primary), ratio.MinPrimary)

	secondarySize := usable - primarySize
	if secondarySize < ratio.MinSecondary {
		secondarySize = ratio.MinSecondary
		primarySize = usable - secondarySize
	}

	return SplitResult{
		PrimarySize:   primarySize,
		SecondarySize: secondarySize,
		DividerSize:   dividerSize,
	}
}
