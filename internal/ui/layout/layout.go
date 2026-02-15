package layout

import (
	lipgloss "charm.land/lipgloss/v2"
)

const (
	// StatusBarHeight is always 1 line.
	StatusBarHeight = 1
	// FooterHeight is always 1 line.
	FooterHeight = 1
	// ChromeHeight is the total vertical space consumed by status bar + footer.
	ChromeHeight = StatusBarHeight + FooterHeight
)

// Dimensions holds the computed dimensions for each layout region.
type Dimensions struct {
	TotalWidth  int
	TotalHeight int

	// Content area (between status bar and footer).
	ContentWidth  int
	ContentHeight int

	Tier Tier
}

// ComputeDimensions calculates the available content area given terminal size.
func ComputeDimensions(width, height int) Dimensions {
	contentHeight := max(height-ChromeHeight, 0)

	return Dimensions{
		TotalWidth:    width,
		TotalHeight:   height,
		ContentWidth:  width,
		ContentHeight: contentHeight,
		Tier:          DetectTier(width, height, DefaultBreakpoints()),
	}
}

// Render composes the three-band layout: status bar + content + footer.
// All three strings must already be rendered to the correct width.
func Render(statusBar, content, footer string) string {
	return lipgloss.JoinVertical(lipgloss.Top, statusBar, content, footer)
}

// RenderSplitVertical composes a vertical split (top/bottom panes with divider).
// Used for PREVIEW mode (list on top, diff on bottom).
func RenderSplitVertical(top, divider, bottom string) string {
	return lipgloss.JoinVertical(lipgloss.Top, top, divider, bottom)
}

// RenderSplitHorizontal composes a horizontal split (left/right panes).
// Used for DETAIL mode (file tree on left, diff on right).
func RenderSplitHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// PadToWidth pads a string to the given width using spaces.
func PadToWidth(s string, width int) string {
	style := lipgloss.NewStyle().Width(width).MaxWidth(width)
	return style.Render(s)
}

// PadToHeight pads a string to the given height by appending empty lines.
func PadToHeight(s string, height int) string {
	style := lipgloss.NewStyle().Height(height).MaxHeight(height)
	return style.Render(s)
}

// FitContent constrains content to the given width and height.
func FitContent(s string, width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Height(height).
		MaxHeight(height)
	return style.Render(s)
}
