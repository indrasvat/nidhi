package components

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// StatusBarParams holds the data needed to render the status bar.
type StatusBarParams struct {
	RepoName   string
	Branch     string
	StashCount int
	GitVersion plugin.GitVersion
	Width      int
	UseNerd    bool
}

// StatusBar renders the status bar matching the mockup: ◆ {repo} ⎇ {branch} [{N} stashes] ... git {version}.
// PRD Section 9.3: status bar is always 1 line.
type StatusBar struct {
	Theme theme.Theme
}

// NewStatusBar creates a StatusBar with the given theme.
func NewStatusBar(th theme.Theme) StatusBar {
	return StatusBar{Theme: th}
}

// Render renders the status bar for the given params.
func (sb StatusBar) Render(p StatusBarParams) string {
	th := sb.Theme

	barStyle := lipgloss.NewStyle().
		Background(th.BgSurface()).
		Width(p.Width).
		MaxWidth(p.Width)

	// App mark: ◆ (mockup uses this diamond, not Nerd Font).
	var appMark string
	if p.UseNerd {
		appMark = "◆"
	} else {
		appMark = "◆"
	}

	markStyle := styledFgBg(th.AccentGold(), th.BgSurface()).Bold(true)
	repoStyle := styledFgBg(th.FgPrimary(), th.BgSurface()).Bold(true)
	branchStyle := styledFgBg(th.SemanticAqua(), th.BgSurface())
	countStyle := lipgloss.NewStyle().
		Foreground(th.AccentGold()).
		Background(th.BgSurface())
	versionStyle := styledFgBg(th.FgSecondary(), th.BgSurface())

	// Build left side: ◆ repoName  ⎇ branch  N stashes
	repoName := p.RepoName
	if repoName == "" {
		repoName = "nidhi"
	}

	left := markStyle.Render(appMark) + " " +
		repoStyle.Render(repoName) + "  " +
		branchStyle.Render("⎇ "+p.Branch) + "  " +
		countStyle.Render(fmt.Sprintf("%d stashes", p.StashCount))

	// Build right side: git version.
	right := versionStyle.Render(fmt.Sprintf("git %d.%d", p.GitVersion.Major, p.GitVersion.Minor))

	// Compute spacing between left and right.
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := max(p.Width-leftWidth-rightWidth, 1)
	spacing := styledFgBg(th.BgSurface(), th.BgSurface()).
		Width(gap).
		Render(strings.Repeat(" ", gap))

	return barStyle.Render(left + spacing + right)
}

// styledFgBg creates a style with foreground and background colors.
func styledFgBg(fg, bg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(bg)
}
