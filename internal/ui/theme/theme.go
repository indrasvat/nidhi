package theme

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

// Theme defines the contract for nidhi's visual styling.
type Theme interface {
	BgDeep() color.Color
	BgSurface() color.Color
	BgElevated() color.Color
	BgOverlay() color.Color

	FgPrimary() color.Color
	FgSecondary() color.Color
	FgDimmed() color.Color

	AccentGold() color.Color
	AccentBright() color.Color

	SemanticAqua() color.Color
	SemanticCoral() color.Color
	SemanticGreen() color.Color
	SemanticRed() color.Color
	SemanticYellow() color.Color
	SemanticBlue() color.Color
	SemanticPurple() color.Color

	DiffAddedFg() color.Color
	DiffAddedBg() color.Color
	DiffRemovedFg() color.Color
	DiffRemovedBg() color.Color
	DiffHunk() color.Color

	BaseStyle() lipgloss.Style
	ActiveRowStyle() lipgloss.Style
	DimmedStyle() lipgloss.Style
	AccentStyle() lipgloss.Style
	ErrorStyle() lipgloss.Style
	SuccessStyle() lipgloss.Style
	SHAStyle() lipgloss.Style
	BranchStyle() lipgloss.Style
	StaleStyle() lipgloss.Style
	DiffAddedStyle() lipgloss.Style
	DiffRemovedStyle() lipgloss.Style
	DiffHunkStyle() lipgloss.Style

	Name() string
}
