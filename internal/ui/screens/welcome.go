package screens

import (
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ASCII art logo for nidhi — block letters inspired by but distinct from yukti.
const nidhiLogo = `
███╗   ██╗██╗██████╗ ██╗  ██╗██╗
████╗  ██║██║██╔══██╗██║  ██║██║
██╔██╗ ██║██║██║  ██║███████║██║
██║╚██╗██║██║██║  ██║██╔══██║██║
██║ ╚████║██║██████╔╝██║  ██║██║
╚═╝  ╚═══╝╚═╝╚═════╝ ╚═╝  ╚═╝╚═╝`

// RenderWelcome renders the welcome/startup screen with logo, feature cards, and CTA.
func RenderWelcome(th theme.Theme, width, height int, version, commit string) string {
	// Gradient colors: warm gold → amber → ember (Agni fire theme).
	gradientColors := []color.Color{
		lipgloss.Color("#FFDD70"), // bright gold
		lipgloss.Color("#E8B85A"), // amber (AccentBright)
		lipgloss.Color("#D4A050"), // gold (AccentGold)
		lipgloss.Color("#C88040"), // warm amber
		lipgloss.Color("#B06030"), // deep amber
		lipgloss.Color("#8A4020"), // dark ember
	}

	logoRendered := renderGradientLogo(nidhiLogo, gradientColors)

	// Tagline.
	tagline := lipgloss.NewStyle().
		Foreground(th.FgSecondary()).
		Italic(true).
		Render("purpose-built TUI for git stash mastery")

	// Feature cards.
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.BgOverlay()).
		Padding(1, 2).
		Width(24).
		Align(lipgloss.Center)

	cardTitleStyle := lipgloss.NewStyle().
		Foreground(th.AccentGold()).
		Bold(true)

	cardDescStyle := lipgloss.NewStyle().
		Foreground(th.FgSecondary())

	features := []struct {
		icon  string
		title string
		desc  string
	}{
		{"📋", "BROWSE", "Navigate stashes\nwith vim-style\nkeys"},
		{"🔍", "PREVIEW", "Inline diffs\nwith file-by-file\ncycling"},
		{"⚡", "MANAGE", "Apply, pop, drop\nrename, branch\n& export"},
	}

	cards := make([]string, 0, len(features))
	for _, f := range features {
		title := cardTitleStyle.Render(f.icon + " " + f.title)
		desc := cardDescStyle.Render(f.desc)
		card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center, title, desc))
		cards = append(cards, card)
	}

	cardRow := lipgloss.JoinHorizontal(lipgloss.Top, cards[0], "  ", cards[1], "  ", cards[2])

	// CTA button.
	cta := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(th.SemanticAqua()).
		Foreground(th.SemanticAqua()).
		Padding(0, 3).
		Bold(true).
		Render("⏎  Press Enter to continue")

	// Version info.
	ver := version
	if ver == "" {
		ver = "dev"
	}
	if commit != "" && commit != "unknown" && len(commit) >= 7 {
		ver += " (" + commit[:7] + ")"
	}
	versionText := lipgloss.NewStyle().
		Foreground(th.FgDimmed()).
		Render(ver + " · Made with ◆")

	// Combine everything vertically, centered.
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		logoRendered,
		"",
		tagline,
		"",
		cardRow,
		"",
		"",
		cta,
		"",
		versionText,
	)

	// Center in viewport.
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// renderGradientLogo applies a vertical color gradient to ASCII art.
func renderGradientLogo(logoText string, colors []color.Color) string {
	lines := strings.Split(strings.TrimPrefix(logoText, "\n"), "\n")
	styledLines := make([]string, 0, len(lines))

	for i, line := range lines {
		colorIdx := i
		if colorIdx >= len(colors) {
			colorIdx = len(colors) - 1
		}

		style := lipgloss.NewStyle().
			Foreground(colors[colorIdx]).
			Bold(true)

		styledLines = append(styledLines, style.Render(line))
	}

	return strings.Join(styledLines, "\n")
}
