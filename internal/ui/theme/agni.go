package theme

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"
)

// Agni color token hex values from PRD section 9.1.
const (
	agniBgDeep     = "#07090E"
	agniBgSurface  = "#0F1219"
	agniBgElevated = "#1A1F2B"
	agniBgOverlay  = "#1F2738"

	agniFgPrimary   = "#C8CCD4"
	agniFgSecondary = "#6B7280"
	agniFgDimmed    = "#3D4450"

	agniAccentGold   = "#D4A050"
	agniAccentBright = "#E8B85A"

	agniSemanticAqua   = "#4EC9B0"
	agniSemanticCoral  = "#F47067"
	agniSemanticGreen  = "#73D990"
	agniSemanticRed    = "#FF5F6D"
	agniSemanticYellow = "#E5C07B"
	agniSemanticBlue   = "#61AFEF"
	agniSemanticPurple = "#C678DD"

	agniDiffAddedFg   = "#73D990"
	agniDiffAddedBg   = "#1A2E1A"
	agniDiffRemovedFg = "#FF5F6D"
	agniDiffRemovedBg = "#2E1A1A"
	agniDiffHunk      = "#61AFEF"

	agniDiffAddedEmphFg   = "#A8F0B8"
	agniDiffAddedEmphBg   = "#264D26"
	agniDiffRemovedEmphFg = "#FF8A94"
	agniDiffRemovedEmphBg = "#4D2626"
)

// Agni implements the Theme interface with the Agni color scheme.
type Agni struct{}

func NewAgni() *Agni { return &Agni{} }

var _ Theme = (*Agni)(nil)

func (a *Agni) Name() string { return "agni" }

func (a *Agni) BgDeep() color.Color     { return lipgloss.Color(agniBgDeep) }
func (a *Agni) BgSurface() color.Color  { return lipgloss.Color(agniBgSurface) }
func (a *Agni) BgElevated() color.Color { return lipgloss.Color(agniBgElevated) }
func (a *Agni) BgOverlay() color.Color  { return lipgloss.Color(agniBgOverlay) }

func (a *Agni) FgPrimary() color.Color   { return lipgloss.Color(agniFgPrimary) }
func (a *Agni) FgSecondary() color.Color { return lipgloss.Color(agniFgSecondary) }
func (a *Agni) FgDimmed() color.Color    { return lipgloss.Color(agniFgDimmed) }

func (a *Agni) AccentGold() color.Color   { return lipgloss.Color(agniAccentGold) }
func (a *Agni) AccentBright() color.Color { return lipgloss.Color(agniAccentBright) }

func (a *Agni) SemanticAqua() color.Color   { return lipgloss.Color(agniSemanticAqua) }
func (a *Agni) SemanticCoral() color.Color  { return lipgloss.Color(agniSemanticCoral) }
func (a *Agni) SemanticGreen() color.Color  { return lipgloss.Color(agniSemanticGreen) }
func (a *Agni) SemanticRed() color.Color    { return lipgloss.Color(agniSemanticRed) }
func (a *Agni) SemanticYellow() color.Color { return lipgloss.Color(agniSemanticYellow) }
func (a *Agni) SemanticBlue() color.Color   { return lipgloss.Color(agniSemanticBlue) }
func (a *Agni) SemanticPurple() color.Color { return lipgloss.Color(agniSemanticPurple) }

func (a *Agni) DiffAddedFg() color.Color       { return lipgloss.Color(agniDiffAddedFg) }
func (a *Agni) DiffAddedBg() color.Color       { return lipgloss.Color(agniDiffAddedBg) }
func (a *Agni) DiffRemovedFg() color.Color     { return lipgloss.Color(agniDiffRemovedFg) }
func (a *Agni) DiffRemovedBg() color.Color     { return lipgloss.Color(agniDiffRemovedBg) }
func (a *Agni) DiffAddedEmphFg() color.Color   { return lipgloss.Color(agniDiffAddedEmphFg) }
func (a *Agni) DiffAddedEmphBg() color.Color   { return lipgloss.Color(agniDiffAddedEmphBg) }
func (a *Agni) DiffRemovedEmphFg() color.Color { return lipgloss.Color(agniDiffRemovedEmphFg) }
func (a *Agni) DiffRemovedEmphBg() color.Color { return lipgloss.Color(agniDiffRemovedEmphBg) }
func (a *Agni) DiffHunk() color.Color          { return lipgloss.Color(agniDiffHunk) }

func (a *Agni) BaseStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.FgPrimary()).
		Background(a.BgDeep())
}

func (a *Agni) ActiveRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.FgPrimary()).
		Background(a.BgElevated()).
		Bold(true)
}

func (a *Agni) DimmedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(a.FgDimmed())
}

func (a *Agni) AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(a.AccentGold()).Bold(true)
}

func (a *Agni) ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(a.SemanticCoral()).Bold(true)
}

func (a *Agni) SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(a.SemanticAqua())
}

func (a *Agni) SHAStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(a.SemanticPurple())
}

func (a *Agni) BranchStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(a.SemanticBlue())
}

func (a *Agni) StaleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.BgDeep()).
		Background(a.SemanticYellow()).
		Bold(true).
		Padding(0, 1)
}

func (a *Agni) DiffAddedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.DiffAddedFg()).
		Background(a.DiffAddedBg())
}

func (a *Agni) DiffRemovedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.DiffRemovedFg()).
		Background(a.DiffRemovedBg())
}

func (a *Agni) DiffAddedEmphStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.DiffAddedEmphFg()).
		Background(a.DiffAddedEmphBg()).
		Bold(true)
}

func (a *Agni) DiffRemovedEmphStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.DiffRemovedEmphFg()).
		Background(a.DiffRemovedEmphBg()).
		Bold(true)
}

func (a *Agni) DiffHunkStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(a.DiffHunk()).Bold(true)
}
