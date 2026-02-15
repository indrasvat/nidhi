package components

import (
	"image/color"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// KeyHint represents a single key hint shown in the footer.
type KeyHint struct {
	Key  string
	Desc string
}

// FooterParams holds the data needed to render the footer.
type FooterParams struct {
	Mode  plugin.Mode
	Width int
}

// Footer renders the context-sensitive footer bar.
// PRD Section 9.3: footer is 1 line with keybind hints + mode badge.
type Footer struct {
	Theme theme.Theme
}

// NewFooter creates a Footer with the given theme.
func NewFooter(th theme.Theme) Footer {
	return Footer{Theme: th}
}

// HintsForMode returns the keybind hints for a given mode.
// From PRD Section 11.2 complete keymap, matching mockup footer bars.
func HintsForMode(mode plugin.Mode) []KeyHint {
	switch mode {
	case plugin.ModeList:
		return []KeyHint{
			{"j/k", "nav"},
			{"\u23CE", "detail"},
			{"\u21E5", "preview"},
			{"n", "new"},
			{"a", "apply"},
			{"p", "pop"},
			{"d", "drop"},
			{"r", "rename"},
			{"e", "export"},
			{"/", "search"},
			{"?", "help"},
		}
	case plugin.ModePreview:
		return []KeyHint{
			{"j/k", "stashes"},
			{"h/l", "files"},
			{"\u21E5", "close"},
			{"\u23CE", "detail"},
			{"a", "apply"},
			{"p", "pop"},
			{"?", "help"},
		}
	case plugin.ModeDetail:
		return []KeyHint{
			{"j/k", "files"},
			{"\u21E5", "tree\u2194diff"},
			{"\u2191\u2193", "scroll"},
			{"a", "apply"},
			{"p", "pop"},
			{"b", "branch"},
			{"r", "rename"},
			{"?", "help"},
			{"esc", "back"},
		}
	case plugin.ModeSearch:
		return []KeyHint{
			{"\u2191\u2193", "results"},
			{"\u23CE", "open"},
			{"\u21E5", "filter"},
			{"?", "help"},
			{"esc", "close"},
		}
	case plugin.ModeNewStash:
		return []KeyHint{
			{"\u23CE", "create"},
			{"\u21E5", "cycle scope"},
			{"^p", "patch mode"},
			{"?", "help"},
			{"esc", "cancel"},
		}
	case plugin.ModeExport:
		return []KeyHint{
			{"space", "toggle"},
			{"a", "all"},
			{"\u21E5", "edit ref"},
			{"i", "switch to import"},
			{"?", "help"},
			{"esc", "back"},
		}
	case plugin.ModeImport:
		return []KeyHint{
			{"space", "toggle"},
			{"\u23CE", "import"},
			{"e", "switch to export"},
			{"?", "help"},
			{"esc", "back"},
		}
	case plugin.ModeConflict:
		return []KeyHint{
			{"j/k", "files"},
			{"a", "apply anyway"},
			{"p", "pop anyway"},
			{"b", "branch first"},
			{"?", "help"},
			{"esc", "cancel"},
		}
	case plugin.ModeHelp:
		return []KeyHint{
			{"esc", "close"},
		}
	default:
		return []KeyHint{{"esc", "back"}}
	}
}

// Render renders the footer for the given params.
func (f Footer) Render(p FooterParams) string {
	th := f.Theme

	barStyle := lipgloss.NewStyle().
		Background(th.BgSurface()).
		Width(p.Width).
		MaxWidth(p.Width)

	// Key badge style (matching mockup .fk .k: bg-3, gold text, bordered).
	keyStyle := lipgloss.NewStyle().
		Foreground(th.AccentGold()).
		Background(th.BgOverlay()).
		Bold(true)

	descStyle := styledFgBg(th.FgSecondary(), th.BgSurface())

	// Mode badge with mode-specific colors matching mockup .fmode-* classes.
	badgeFg, badgeBg := badgeColorsForMode(p.Mode, th)
	badgeStyle := lipgloss.NewStyle().
		Foreground(badgeFg).
		Background(badgeBg).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	// Build hints.
	hints := HintsForMode(p.Mode)
	var hintsStr string
	sep := styledFgBg(th.BgSurface(), th.BgSurface()).Render(" ")
	for i, h := range hints {
		if i > 0 {
			hintsStr += sep
		}
		hintsStr += keyStyle.Render(h.Key) +
			styledFgBg(th.BgSurface(), th.BgSurface()).Render(" ") +
			descStyle.Render(h.Desc)
	}

	// Mode badge (right-aligned).
	badge := badgeStyle.Render(p.Mode.String())

	// Compute spacing.
	hintsWidth := lipgloss.Width(hintsStr)
	badgeWidth := lipgloss.Width(badge)
	gap := max(p.Width-hintsWidth-badgeWidth-2, 1)
	spacing := styledFgBg(th.BgSurface(), th.BgSurface()).
		Width(gap).
		Render("")

	return barStyle.Render(" " + hintsStr + spacing + badge)
}

// badgeColorsForMode returns the foreground and background colors for the mode badge.
// Matches the mockup's .fmode-* CSS classes exactly.
func badgeColorsForMode(mode plugin.Mode, th theme.Theme) (fg, bg color.Color) {
	switch mode {
	case plugin.ModeList:
		return th.AccentGold(), lipgloss.Color("#141810") // gold on gold-bg
	case plugin.ModePreview:
		return th.SemanticAqua(), lipgloss.Color("#0F1F1C") // aqua on aqua-bg
	case plugin.ModeDetail:
		return th.SemanticBlue(), lipgloss.Color("#101820") // blue on blue-bg
	case plugin.ModeSearch:
		return th.SemanticPurple(), lipgloss.Color("#161020") // purple on purple-bg
	case plugin.ModeExport:
		return lipgloss.Color("#E89B5A"), lipgloss.Color("#1C1410") // orange on orange-bg
	case plugin.ModeNewStash:
		return th.SemanticGreen(), lipgloss.Color("#101C12") // green on green-bg
	case plugin.ModeConflict:
		return th.SemanticYellow(), lipgloss.Color("#1C1810") // yellow on yellow-bg
	case plugin.ModeHelp:
		return th.FgSecondary(), lipgloss.Color("#121418") // dimmed on dim-bg
	default:
		return th.FgSecondary(), th.BgOverlay()
	}
}

// BadgeColorForMode returns the primary foreground color for a mode badge.
func BadgeColorForMode(mode plugin.Mode, th theme.Theme) color.Color {
	fg, _ := badgeColorsForMode(mode, th)
	return fg
}
