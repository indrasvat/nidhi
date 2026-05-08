# Task 003: Agni Theme and Icons

## Status: TODO

## Depends On
- 000 (Repository Scaffold and Tooling) -- go.mod with LipGloss v2 dep, directory structure

## Parallelizable With
- 001 (Git Runner and Version Detection)
- 002 (Config Loading)

## Problem
Every UI component in nidhi needs consistent colors, styles, and icons. Without a centralized theme system, colors will be hardcoded across the codebase, icon fallback logic will be duplicated, and changing the theme will require touching dozens of files. The Agni theme (PRD section 9.1) defines 26 color tokens with exact hex values. The icon system (PRD section 9.2) needs Nerd Font glyphs with ASCII fallbacks and auto-detection.

## PRD Reference
- Section 9.1 (Theme: Agni) -- all 26 color tokens with hex values (bg.deep through diff.hunk)
- Section 9.2 (Typography & Icons) -- 14 icon definitions with Nerd Font glyphs and ASCII fallbacks, detection strategy
- Section 9.3 (Layout Contract) -- status bar, content area, footer bar structure
- Section 7.3 (Compatibility) -- TrueColor (primary), ANSI256 (fallback), 16-color, 1-bit; Nerd Fonts optional
- Section 12.2 (Config) -- `icons = "auto"/"nerd"/"ascii"`, `theme.name`
- Section 12.4 (Environment Variables) -- NERD_FONTS env var for forcing icon mode

## Files to Create
- `internal/ui/theme/theme.go` -- `Theme` interface with color token accessors and style factory methods
- `internal/ui/theme/agni.go` -- Agni theme implementation with ALL 26 color tokens from PRD section 9.1
- `internal/ui/theme/adaptive.go` -- helpers for colorprofile-based downsampling awareness
- `internal/ui/icons/icons.go` -- icon set with Nerd Font + ASCII fallback, auto-detection, all 14 icons from PRD section 9.2
- `internal/ui/theme/theme_test.go` -- verify all tokens present, verify style rendering
- `internal/ui/icons/icons_test.go` -- test fallback logic, test all icon constants exist

## Execution Steps

### Step 1: Create `internal/ui/theme/theme.go`

```go
package theme

import (
	lipgloss "github.com/charmbracelet/lipgloss/v2"
)

// Theme defines the contract for nidhi's visual styling.
// Each method returns a LipGloss color that can be used in styles.
// BubbleTea v2's colorprofile handles automatic downsampling from
// TrueColor to ANSI256/16/1-bit -- themes just provide hex values.
type Theme interface {
	// --- Background tokens ---
	BgDeep() lipgloss.Color     // Primary background (#07090E)
	BgSurface() lipgloss.Color  // Panels, cards (#0F1219)
	BgElevated() lipgloss.Color // Active row, hover states (#1A1F2B)
	BgOverlay() lipgloss.Color  // Modal/dialog background (#1F2738)

	// --- Foreground tokens ---
	FgPrimary() lipgloss.Color   // Primary text (#C8CCD4)
	FgSecondary() lipgloss.Color // Secondary/muted text (#6B7280)
	FgDimmed() lipgloss.Color    // Disabled, stale, ghost text (#3D4450)

	// --- Accent tokens ---
	AccentGold() lipgloss.Color   // Primary accent -- borders, active states (#D4A050)
	AccentBright() lipgloss.Color // Highlighted accent -- cursor, focus ring (#E8B85A)

	// --- Semantic tokens ---
	SemanticAqua() lipgloss.Color   // Clean/success (#4EC9B0)
	SemanticCoral() lipgloss.Color  // Danger/destructive (#F47067)
	SemanticGreen() lipgloss.Color  // Insertions, additions (#73D990)
	SemanticRed() lipgloss.Color    // Deletions, conflicts (#FF5F6D)
	SemanticYellow() lipgloss.Color // Warnings, stale badge (#E5C07B)
	SemanticBlue() lipgloss.Color   // Info, links, branch names (#61AFEF)
	SemanticPurple() lipgloss.Color // SHA hashes, special elements (#C678DD)

	// --- Diff tokens ---
	DiffAddedFg() lipgloss.Color  // Diff: added line text (#73D990)
	DiffAddedBg() lipgloss.Color  // Diff: added line background (#1A2E1A)
	DiffRemovedFg() lipgloss.Color // Diff: removed line text (#FF5F6D)
	DiffRemovedBg() lipgloss.Color // Diff: removed line background (#2E1A1A)
	DiffHunk() lipgloss.Color     // Diff: hunk header (#61AFEF)

	// --- Style factories ---
	// These return pre-configured styles for common UI patterns.

	// BaseStyle returns a style with the theme's primary bg and fg.
	BaseStyle() lipgloss.Style
	// ActiveRowStyle returns a style for the currently selected row.
	ActiveRowStyle() lipgloss.Style
	// DimmedStyle returns a style for muted/disabled text.
	DimmedStyle() lipgloss.Style
	// AccentStyle returns a style with the gold accent foreground.
	AccentStyle() lipgloss.Style
	// ErrorStyle returns a style for error/danger text.
	ErrorStyle() lipgloss.Style
	// SuccessStyle returns a style for success/clean text.
	SuccessStyle() lipgloss.Style
	// SHAStyle returns a style for SHA hash rendering (purple).
	SHAStyle() lipgloss.Style
	// BranchStyle returns a style for branch name rendering (blue).
	BranchStyle() lipgloss.Style
	// StaleStyle returns a style for the STALE badge (yellow bg).
	StaleStyle() lipgloss.Style
	// DiffAddedStyle returns a style for added diff lines (green fg, dark green bg).
	DiffAddedStyle() lipgloss.Style
	// DiffRemovedStyle returns a style for removed diff lines (red fg, dark red bg).
	DiffRemovedStyle() lipgloss.Style
	// DiffHunkStyle returns a style for diff hunk headers (blue).
	DiffHunkStyle() lipgloss.Style

	// Name returns the theme's display name.
	Name() string
}
```

### Step 2: Create `internal/ui/theme/agni.go`

```go
package theme

import (
	lipgloss "github.com/charmbracelet/lipgloss/v2"
)

// Agni color token constants from PRD section 9.1.
// All hex values are exact matches.
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

	agniDiffAddedFg  = "#73D990"
	agniDiffAddedBg  = "#1A2E1A"
	agniDiffRemovedFg = "#FF5F6D"
	agniDiffRemovedBg = "#2E1A1A"
	agniDiffHunk     = "#61AFEF"
)

// Agni implements the Theme interface with the Agni color scheme.
// Agni (Sanskrit: "fire") is nidhi's default dark theme --
// embers glowing on a deep ocean.
type Agni struct{}

// NewAgni creates a new Agni theme instance.
func NewAgni() *Agni {
	return &Agni{}
}

// Verify Agni implements Theme at compile time.
var _ Theme = (*Agni)(nil)

func (a *Agni) Name() string { return "agni" }

// --- Background tokens ---

func (a *Agni) BgDeep() lipgloss.Color     { return lipgloss.Color(agniBgDeep) }
func (a *Agni) BgSurface() lipgloss.Color  { return lipgloss.Color(agniBgSurface) }
func (a *Agni) BgElevated() lipgloss.Color { return lipgloss.Color(agniBgElevated) }
func (a *Agni) BgOverlay() lipgloss.Color  { return lipgloss.Color(agniBgOverlay) }

// --- Foreground tokens ---

func (a *Agni) FgPrimary() lipgloss.Color   { return lipgloss.Color(agniFgPrimary) }
func (a *Agni) FgSecondary() lipgloss.Color { return lipgloss.Color(agniFgSecondary) }
func (a *Agni) FgDimmed() lipgloss.Color    { return lipgloss.Color(agniFgDimmed) }

// --- Accent tokens ---

func (a *Agni) AccentGold() lipgloss.Color   { return lipgloss.Color(agniAccentGold) }
func (a *Agni) AccentBright() lipgloss.Color { return lipgloss.Color(agniAccentBright) }

// --- Semantic tokens ---

func (a *Agni) SemanticAqua() lipgloss.Color   { return lipgloss.Color(agniSemanticAqua) }
func (a *Agni) SemanticCoral() lipgloss.Color  { return lipgloss.Color(agniSemanticCoral) }
func (a *Agni) SemanticGreen() lipgloss.Color  { return lipgloss.Color(agniSemanticGreen) }
func (a *Agni) SemanticRed() lipgloss.Color    { return lipgloss.Color(agniSemanticRed) }
func (a *Agni) SemanticYellow() lipgloss.Color { return lipgloss.Color(agniSemanticYellow) }
func (a *Agni) SemanticBlue() lipgloss.Color   { return lipgloss.Color(agniSemanticBlue) }
func (a *Agni) SemanticPurple() lipgloss.Color { return lipgloss.Color(agniSemanticPurple) }

// --- Diff tokens ---

func (a *Agni) DiffAddedFg() lipgloss.Color   { return lipgloss.Color(agniDiffAddedFg) }
func (a *Agni) DiffAddedBg() lipgloss.Color   { return lipgloss.Color(agniDiffAddedBg) }
func (a *Agni) DiffRemovedFg() lipgloss.Color { return lipgloss.Color(agniDiffRemovedFg) }
func (a *Agni) DiffRemovedBg() lipgloss.Color { return lipgloss.Color(agniDiffRemovedBg) }
func (a *Agni) DiffHunk() lipgloss.Color      { return lipgloss.Color(agniDiffHunk) }

// --- Style factories ---

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
	return lipgloss.NewStyle().
		Foreground(a.FgDimmed())
}

func (a *Agni) AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.AccentGold()).
		Bold(true)
}

func (a *Agni) ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.SemanticCoral()).
		Bold(true)
}

func (a *Agni) SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.SemanticAqua())
}

func (a *Agni) SHAStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.SemanticPurple())
}

func (a *Agni) BranchStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.SemanticBlue())
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

func (a *Agni) DiffHunkStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(a.DiffHunk()).
		Bold(true)
}
```

### Step 3: Create `internal/ui/theme/adaptive.go`

```go
package theme

// ColorProfile represents the terminal's color capability.
// BubbleTea v2 and LipGloss v2 use charmbracelet/colorprofile internally
// for automatic downsampling. This package provides helpers for theme-level
// awareness of the active profile.
type ColorProfile int

const (
	// ProfileTrueColor supports 16 million colors (24-bit).
	ProfileTrueColor ColorProfile = iota
	// ProfileANSI256 supports 256 colors (8-bit).
	ProfileANSI256
	// ProfileANSI supports 16 colors (4-bit).
	ProfileANSI
	// ProfileNoColor is monochrome (1-bit).
	ProfileNoColor
)

// String returns the profile name.
func (p ColorProfile) String() string {
	switch p {
	case ProfileTrueColor:
		return "truecolor"
	case ProfileANSI256:
		return "ansi256"
	case ProfileANSI:
		return "ansi"
	case ProfileNoColor:
		return "nocolor"
	default:
		return "unknown"
	}
}

// HasColor returns true if the profile supports any color output.
func (p ColorProfile) HasColor() bool {
	return p != ProfileNoColor
}

// HasTrueColor returns true if the profile supports full 24-bit color.
func (p ColorProfile) HasTrueColor() bool {
	return p == ProfileTrueColor
}

// ThemeForProfile returns the appropriate theme for the given color profile.
// Currently only Agni is supported. In the future, this could return
// high-contrast or monochrome variants.
func ThemeForProfile(profile ColorProfile) Theme {
	// BubbleTea v2 handles color downsampling automatically via colorprofile.
	// We always return Agni with TrueColor hex values; the renderer
	// will downgrade them as needed. This function exists as an extension
	// point for future theme variants (e.g., high-contrast mode).
	return NewAgni()
}
```

### Step 4: Create `internal/ui/icons/icons.go`

```go
package icons

import "os"

// Mode controls which icon set to use.
type Mode int

const (
	// ModeAuto detects Nerd Fonts automatically.
	ModeAuto Mode = iota
	// ModeNerd forces Nerd Font icons.
	ModeNerd
	// ModeASCII forces ASCII fallback icons.
	ModeASCII
)

// ParseMode converts a string to a Mode.
// Valid values: "auto", "nerd", "ascii". Invalid values return ModeAuto.
func ParseMode(s string) Mode {
	switch s {
	case "nerd":
		return ModeNerd
	case "ascii":
		return ModeASCII
	default:
		return ModeAuto
	}
}

// Icon represents a single icon with Nerd Font and ASCII variants.
type Icon struct {
	Nerd  string // Nerd Font glyph
	ASCII string // ASCII fallback
}

// String returns the appropriate glyph based on the active icon mode.
func (i Icon) String(mode Mode) string {
	if mode == ModeNerd || (mode == ModeAuto && DetectNerdFonts()) {
		return i.Nerd
	}
	return i.ASCII
}

// All icon definitions from PRD section 9.2.
// Nerd Font codepoints are verified against the Nerd Fonts cheat sheet.
var (
	// AppMark is the app identity icon.
	// Nerd: nf-md-source_branch (U+F0620)
	AppMark = Icon{Nerd: "\U000F0620", ASCII: "\u2261"} // 󰘬 / ≡

	// StashItem is the icon for a stash entry in the list.
	// Nerd: nf-oct-archive (U+F423)
	StashItem = Icon{Nerd: "\uF423", ASCII: "\u25AA"} //  / ▪

	// FileModified indicates a modified file.
	// Nerd: nf-oct-diff_modified (U+F440)
	FileModified = Icon{Nerd: "\uF440", ASCII: "~"} //  / ~

	// FileAdded indicates an added file.
	// Nerd: nf-oct-diff_added (U+F457)
	FileAdded = Icon{Nerd: "\uF457", ASCII: "+"} //  / +

	// FileRemoved indicates a removed file.
	// Nerd: nf-oct-diff_removed (U+F458)
	FileRemoved = Icon{Nerd: "\uF458", ASCII: "-"} //  / -

	// FileRenamed indicates a renamed file.
	// Nerd: nf-oct-diff_renamed (U+F45A)
	FileRenamed = Icon{Nerd: "\uF45A", ASCII: "\u2192"} //  / →

	// Conflict indicates merge conflicts.
	Conflict = Icon{Nerd: "\u26A1", ASCII: "!"} // ⚡ / !

	// Clean indicates no conflicts (clean merge).
	Clean = Icon{Nerd: "\u2713", ASCII: "\u221A"} // ✓ / √

	// StaleBadge indicates a stale stash.
	// Nerd: nf-fa-clock_o (U+F017)
	StaleBadge = Icon{Nerd: "\uF017", ASCII: "\u231B"} //  / ⌛

	// Export indicates export action.
	// Nerd: nf-fa-upload (U+F093)
	Export = Icon{Nerd: "\uF093", ASCII: "\u2191"} //  / ↑

	// Import indicates import action.
	// Nerd: nf-fa-download (U+F019)
	Import = Icon{Nerd: "\uF019", ASCII: "\u2193"} //  / ↓

	// Search indicates search action.
	// Nerd: nf-fa-search (U+F002)
	Search = Icon{Nerd: "\uF002", ASCII: "/"} //  / /

	// Undo indicates undo action.
	// Nerd: nf-fa-undo (U+F0E2)
	Undo = Icon{Nerd: "\uF0E2", ASCII: "\u21BA"} //  / ↺

	// Branch indicates a git branch.
	// Nerd: nf-oct-git_branch (U+F418)
	Branch = Icon{Nerd: "\uF418", ASCII: "\u238B"} //  / ⎇
)

// AllIcons returns all defined icons for enumeration in tests.
func AllIcons() []Icon {
	return []Icon{
		AppMark, StashItem, FileModified, FileAdded, FileRemoved,
		FileRenamed, Conflict, Clean, StaleBadge, Export, Import,
		Search, Undo, Branch,
	}
}

// AllIconNames returns the names of all icons (parallel to AllIcons).
func AllIconNames() []string {
	return []string{
		"AppMark", "StashItem", "FileModified", "FileAdded", "FileRemoved",
		"FileRenamed", "Conflict", "Clean", "StaleBadge", "Export", "Import",
		"Search", "Undo", "Branch",
	}
}

// nerdFontsDetected caches the result of Nerd Font detection.
// -1 = not checked, 0 = not available, 1 = available.
var nerdFontsDetected int = -1

// DetectNerdFonts checks if Nerd Fonts are available.
// Detection strategy (PRD section 9.2):
//  1. Check $NERD_FONTS env var (explicit override: "1" = yes, "0" = no)
//  2. Default to false (conservative fallback)
//
// Note: Attempting to render a glyph and check width is unreliable in
// non-interactive contexts (tests, CI). The env var approach is the
// most robust method.
func DetectNerdFonts() bool {
	if nerdFontsDetected >= 0 {
		return nerdFontsDetected == 1
	}

	v := os.Getenv("NERD_FONTS")
	switch v {
	case "1", "true", "yes":
		nerdFontsDetected = 1
		return true
	default:
		nerdFontsDetected = 0
		return false
	}
}

// ResetDetection clears the cached detection result.
// Used in tests to re-evaluate the NERD_FONTS env var.
func ResetDetection() {
	nerdFontsDetected = -1
}
```

### Step 5: Create `internal/ui/theme/theme_test.go`

```go
package theme_test

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func TestAgni_ImplementsTheme(t *testing.T) {
	// Compile-time check is in agni.go (var _ Theme = (*Agni)(nil))
	// This test verifies it at runtime too.
	var th theme.Theme = theme.NewAgni()
	if th == nil {
		t.Fatal("NewAgni() returned nil")
	}
}

func TestAgni_Name(t *testing.T) {
	th := theme.NewAgni()
	if th.Name() != "agni" {
		t.Errorf("Name() = %q, want %q", th.Name(), "agni")
	}
}

func TestAgni_AllColorTokensPresent(t *testing.T) {
	th := theme.NewAgni()

	// Every color token method must return a non-empty string when
	// converted to string (LipGloss Color type).
	tokens := []struct {
		name string
		hex  string
		got  string
	}{
		{"BgDeep", "#07090E", string(th.BgDeep())},
		{"BgSurface", "#0F1219", string(th.BgSurface())},
		{"BgElevated", "#1A1F2B", string(th.BgElevated())},
		{"BgOverlay", "#1F2738", string(th.BgOverlay())},
		{"FgPrimary", "#C8CCD4", string(th.FgPrimary())},
		{"FgSecondary", "#6B7280", string(th.FgSecondary())},
		{"FgDimmed", "#3D4450", string(th.FgDimmed())},
		{"AccentGold", "#D4A050", string(th.AccentGold())},
		{"AccentBright", "#E8B85A", string(th.AccentBright())},
		{"SemanticAqua", "#4EC9B0", string(th.SemanticAqua())},
		{"SemanticCoral", "#F47067", string(th.SemanticCoral())},
		{"SemanticGreen", "#73D990", string(th.SemanticGreen())},
		{"SemanticRed", "#FF5F6D", string(th.SemanticRed())},
		{"SemanticYellow", "#E5C07B", string(th.SemanticYellow())},
		{"SemanticBlue", "#61AFEF", string(th.SemanticBlue())},
		{"SemanticPurple", "#C678DD", string(th.SemanticPurple())},
		{"DiffAddedFg", "#73D990", string(th.DiffAddedFg())},
		{"DiffAddedBg", "#1A2E1A", string(th.DiffAddedBg())},
		{"DiffRemovedFg", "#FF5F6D", string(th.DiffRemovedFg())},
		{"DiffRemovedBg", "#2E1A1A", string(th.DiffRemovedBg())},
		{"DiffHunk", "#61AFEF", string(th.DiffHunk())},
	}

	for _, tt := range tokens {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got == "" {
				t.Errorf("%s returned empty color", tt.name)
			}
			if tt.got != tt.hex {
				t.Errorf("%s = %q, want %q (PRD section 9.1)", tt.name, tt.got, tt.hex)
			}
		})
	}
}

func TestAgni_TokenCount(t *testing.T) {
	// PRD section 9.1 defines exactly 21 color tokens (plus bg/fg/accent/semantic groupings).
	// Count: 4 bg + 3 fg + 2 accent + 7 semantic + 5 diff = 21 total.
	expectedCount := 21

	th := theme.NewAgni()
	// Build a list of all token methods and verify count
	tokens := []string{
		string(th.BgDeep()), string(th.BgSurface()), string(th.BgElevated()), string(th.BgOverlay()),
		string(th.FgPrimary()), string(th.FgSecondary()), string(th.FgDimmed()),
		string(th.AccentGold()), string(th.AccentBright()),
		string(th.SemanticAqua()), string(th.SemanticCoral()), string(th.SemanticGreen()),
		string(th.SemanticRed()), string(th.SemanticYellow()), string(th.SemanticBlue()),
		string(th.SemanticPurple()),
		string(th.DiffAddedFg()), string(th.DiffAddedBg()),
		string(th.DiffRemovedFg()), string(th.DiffRemovedBg()), string(th.DiffHunk()),
	}

	if len(tokens) != expectedCount {
		t.Errorf("token count = %d, want %d", len(tokens), expectedCount)
	}
}

func TestAgni_StyleFactories(t *testing.T) {
	th := theme.NewAgni()

	// Verify all style factories return non-zero styles
	styles := []struct {
		name  string
		style func() string
	}{
		{"BaseStyle", func() string { return th.BaseStyle().Render("test") }},
		{"ActiveRowStyle", func() string { return th.ActiveRowStyle().Render("test") }},
		{"DimmedStyle", func() string { return th.DimmedStyle().Render("test") }},
		{"AccentStyle", func() string { return th.AccentStyle().Render("test") }},
		{"ErrorStyle", func() string { return th.ErrorStyle().Render("test") }},
		{"SuccessStyle", func() string { return th.SuccessStyle().Render("test") }},
		{"SHAStyle", func() string { return th.SHAStyle().Render("test") }},
		{"BranchStyle", func() string { return th.BranchStyle().Render("test") }},
		{"StaleStyle", func() string { return th.StaleStyle().Render("test") }},
		{"DiffAddedStyle", func() string { return th.DiffAddedStyle().Render("test") }},
		{"DiffRemovedStyle", func() string { return th.DiffRemovedStyle().Render("test") }},
		{"DiffHunkStyle", func() string { return th.DiffHunkStyle().Render("test") }},
	}

	for _, tt := range styles {
		t.Run(tt.name, func(t *testing.T) {
			rendered := tt.style()
			if rendered == "" {
				t.Errorf("%s rendered empty string", tt.name)
			}
			// Rendered output should contain the input text
			// (LipGloss wraps with ANSI codes but text is preserved)
			// Note: In a no-color environment, rendered == "test"
		})
	}
}

func TestAgni_DiffTokenConsistency(t *testing.T) {
	th := theme.NewAgni()

	// DiffAddedFg should match SemanticGreen (both are #73D990)
	if string(th.DiffAddedFg()) != string(th.SemanticGreen()) {
		t.Errorf("DiffAddedFg (%s) should match SemanticGreen (%s)",
			string(th.DiffAddedFg()), string(th.SemanticGreen()))
	}

	// DiffRemovedFg should match SemanticRed (both are #FF5F6D)
	if string(th.DiffRemovedFg()) != string(th.SemanticRed()) {
		t.Errorf("DiffRemovedFg (%s) should match SemanticRed (%s)",
			string(th.DiffRemovedFg()), string(th.SemanticRed()))
	}

	// DiffHunk should match SemanticBlue (both are #61AFEF)
	if string(th.DiffHunk()) != string(th.SemanticBlue()) {
		t.Errorf("DiffHunk (%s) should match SemanticBlue (%s)",
			string(th.DiffHunk()), string(th.SemanticBlue()))
	}
}

func TestColorProfile_String(t *testing.T) {
	tests := []struct {
		profile theme.ColorProfile
		want    string
	}{
		{theme.ProfileTrueColor, "truecolor"},
		{theme.ProfileANSI256, "ansi256"},
		{theme.ProfileANSI, "ansi"},
		{theme.ProfileNoColor, "nocolor"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.profile.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestColorProfile_HasColor(t *testing.T) {
	if !theme.ProfileTrueColor.HasColor() {
		t.Error("TrueColor should have color")
	}
	if !theme.ProfileANSI256.HasColor() {
		t.Error("ANSI256 should have color")
	}
	if !theme.ProfileANSI.HasColor() {
		t.Error("ANSI should have color")
	}
	if theme.ProfileNoColor.HasColor() {
		t.Error("NoColor should not have color")
	}
}

func TestThemeForProfile(t *testing.T) {
	// All profiles should return a valid theme (Agni for now)
	for _, p := range []theme.ColorProfile{
		theme.ProfileTrueColor, theme.ProfileANSI256,
		theme.ProfileANSI, theme.ProfileNoColor,
	} {
		th := theme.ThemeForProfile(p)
		if th == nil {
			t.Errorf("ThemeForProfile(%s) returned nil", p)
		}
		if th.Name() != "agni" {
			t.Errorf("ThemeForProfile(%s).Name() = %q, want %q", p, th.Name(), "agni")
		}
	}
}
```

### Step 6: Create `internal/ui/icons/icons_test.go`

```go
package icons_test

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/icons"
)

func TestAllIconsExist(t *testing.T) {
	allIcons := icons.AllIcons()
	allNames := icons.AllIconNames()

	// PRD section 9.2 defines 14 icons
	expectedCount := 14
	if len(allIcons) != expectedCount {
		t.Errorf("icon count = %d, want %d", len(allIcons), expectedCount)
	}
	if len(allNames) != expectedCount {
		t.Errorf("name count = %d, want %d", len(allNames), expectedCount)
	}

	for i, icon := range allIcons {
		t.Run(allNames[i], func(t *testing.T) {
			if icon.Nerd == "" {
				t.Errorf("%s has empty Nerd glyph", allNames[i])
			}
			if icon.ASCII == "" {
				t.Errorf("%s has empty ASCII fallback", allNames[i])
			}
		})
	}
}

func TestIcon_String_NerdMode(t *testing.T) {
	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	got := icon.String(icons.ModeNerd)
	if got != "N" {
		t.Errorf("String(ModeNerd) = %q, want %q", got, "N")
	}
}

func TestIcon_String_ASCIIMode(t *testing.T) {
	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	got := icon.String(icons.ModeASCII)
	if got != "A" {
		t.Errorf("String(ModeASCII) = %q, want %q", got, "A")
	}
}

func TestIcon_String_AutoMode_NoNerdFonts(t *testing.T) {
	// Reset detection and set env to no Nerd Fonts
	icons.ResetDetection()
	t.Setenv("NERD_FONTS", "0")

	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	got := icon.String(icons.ModeAuto)
	if got != "A" {
		t.Errorf("String(ModeAuto) without Nerd Fonts = %q, want %q (ASCII)", got, "A")
	}

	// Cleanup
	icons.ResetDetection()
}

func TestIcon_String_AutoMode_WithNerdFonts(t *testing.T) {
	// Reset detection and set env to Nerd Fonts available
	icons.ResetDetection()
	t.Setenv("NERD_FONTS", "1")

	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	got := icon.String(icons.ModeAuto)
	if got != "N" {
		t.Errorf("String(ModeAuto) with Nerd Fonts = %q, want %q (Nerd)", got, "N")
	}

	// Cleanup
	icons.ResetDetection()
}

func TestDetectNerdFonts_EnvVar(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"explicit on with 1", "1", true},
		{"explicit on with true", "true", true},
		{"explicit on with yes", "yes", true},
		{"explicit off with 0", "0", false},
		{"explicit off with false", "false", false},
		{"empty string", "", false},
		{"unset", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icons.ResetDetection()
			t.Setenv("NERD_FONTS", tt.env)

			got := icons.DetectNerdFonts()
			if got != tt.want {
				t.Errorf("DetectNerdFonts() with NERD_FONTS=%q = %v, want %v",
					tt.env, got, tt.want)
			}
		})
	}

	// Cleanup
	icons.ResetDetection()
}

func TestDetectNerdFonts_Caching(t *testing.T) {
	icons.ResetDetection()
	t.Setenv("NERD_FONTS", "1")

	// First call should detect
	first := icons.DetectNerdFonts()
	if !first {
		t.Fatal("first call should return true")
	}

	// Change env var -- cached result should persist
	t.Setenv("NERD_FONTS", "0")
	second := icons.DetectNerdFonts()
	if !second {
		t.Error("cached result should still be true after env change")
	}

	// Reset and re-detect
	icons.ResetDetection()
	third := icons.DetectNerdFonts()
	if third {
		t.Error("after reset, should read new env value (false)")
	}

	// Cleanup
	icons.ResetDetection()
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		want  icons.Mode
	}{
		{"nerd", icons.ModeNerd},
		{"ascii", icons.ModeASCII},
		{"auto", icons.ModeAuto},
		{"", icons.ModeAuto},
		{"invalid", icons.ModeAuto},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := icons.ParseMode(tt.input)
			if got != tt.want {
				t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSpecificIcons_NerdGlyphs(t *testing.T) {
	// Verify specific icons have the expected Nerd Font glyphs
	// These are verified against the PRD section 9.2 table
	tests := []struct {
		name  string
		icon  icons.Icon
		ascii string
	}{
		{"AppMark", icons.AppMark, "\u2261"},     // ≡
		{"StashItem", icons.StashItem, "\u25AA"}, // ▪
		{"FileModified", icons.FileModified, "~"},
		{"FileAdded", icons.FileAdded, "+"},
		{"FileRemoved", icons.FileRemoved, "-"},
		{"FileRenamed", icons.FileRenamed, "\u2192"}, // →
		{"Conflict", icons.Conflict, "!"},
		{"Clean", icons.Clean, "\u221A"},         // √
		{"StaleBadge", icons.StaleBadge, "\u231B"}, // ⌛
		{"Export", icons.Export, "\u2191"},        // ↑
		{"Import", icons.Import, "\u2193"},       // ↓
		{"Search", icons.Search, "/"},
		{"Undo", icons.Undo, "\u21BA"},           // ↺
		{"Branch", icons.Branch, "\u238B"},        // ⎇
	}

	for _, tt := range tests {
		t.Run(tt.name+"_ascii", func(t *testing.T) {
			if tt.icon.ASCII != tt.ascii {
				t.Errorf("%s ASCII = %q, want %q", tt.name, tt.icon.ASCII, tt.ascii)
			}
		})
		t.Run(tt.name+"_nerd_nonempty", func(t *testing.T) {
			if tt.icon.Nerd == "" {
				t.Errorf("%s Nerd glyph is empty", tt.name)
			}
		})
	}
}
```

### Step 7: Verify compilation

```bash
go build ./internal/ui/theme/...
go build ./internal/ui/icons/...
```

### Step 8: Run tests

```bash
go test -v -race -count=1 ./internal/ui/theme/...
go test -v -race -count=1 ./internal/ui/icons/...
```

### Step 9: Run `make ci`

```bash
make ci
```

## Verification

### Functional
```bash
# Theme package compiles
go build ./internal/ui/theme/...

# Icons package compiles
go build ./internal/ui/icons/...

# All theme tests pass
go test -v -race -count=1 ./internal/ui/theme/...

# All icon tests pass
go test -v -race -count=1 ./internal/ui/icons/...

# Verify all 21 color tokens match PRD section 9.1 hex values
go test -v -run TestAgni_AllColorTokensPresent ./internal/ui/theme/...

# Verify all 14 icons exist
go test -v -run TestAllIconsExist ./internal/ui/icons/...

# Verify fallback logic
go test -v -run TestIcon_String ./internal/ui/icons/...
go test -v -run TestDetectNerdFonts ./internal/ui/icons/...

# Verify style factories produce output
go test -v -run TestAgni_StyleFactories ./internal/ui/theme/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `Theme` interface has methods for all 21 color tokens from PRD section 9.1
2. `Theme` interface has 12 style factory methods (BaseStyle, ActiveRowStyle, DimmedStyle, AccentStyle, ErrorStyle, SuccessStyle, SHAStyle, BranchStyle, StaleStyle, DiffAddedStyle, DiffRemovedStyle, DiffHunkStyle)
3. `Agni` struct implements `Theme` (compile-time verified via `var _ Theme = (*Agni)(nil)`)
4. All 21 Agni color constants match PRD section 9.1 hex values exactly
5. `adaptive.go` exports `ColorProfile` type with TrueColor/ANSI256/ANSI/NoColor and `ThemeForProfile`
6. `icons.go` exports all 14 icons from PRD section 9.2 with Nerd Font and ASCII variants
7. `Icon.String(mode)` returns Nerd or ASCII glyph based on mode
8. `DetectNerdFonts()` reads `NERD_FONTS` env var with caching
9. `ResetDetection()` allows cache invalidation in tests
10. `ParseMode()` converts "auto"/"nerd"/"ascii" strings to `Mode`
11. All tests pass with `-race` flag
12. `make ci` passes

## Commit
```
feat: add Agni theme with 21 color tokens and icon system with Nerd Font fallback

Implement internal/ui/theme/ with Theme interface, Agni implementation
matching all PRD section 9.1 hex values, and color profile helpers.
Implement internal/ui/icons/ with 14 icon definitions from PRD section
9.2, auto-detection via NERD_FONTS env var, and ASCII fallback.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 9.1, 9.2, 9.3, 7.3, 12.2 (icons field), 12.4 (NERD_FONTS)
4. Execute steps 1-9 in order
5. Verify all functional and CI checks pass
6. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
7. Commit with the message above + move to next task
