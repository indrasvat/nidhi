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
	AppVersion string // e.g., "dev", "v0.1.0"
	AppCommit  string // e.g., "abc1234"
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

	// Build right side: app version + git version.
	var versionParts []string
	if p.AppVersion != "" {
		appVer := p.AppVersion
		if p.AppCommit != "" && p.AppCommit != "unknown" && len(p.AppCommit) >= 7 {
			appVer += " (" + p.AppCommit[:7] + ")"
		}
		versionParts = append(versionParts, appVer)
	}
	versionParts = append(versionParts, fmt.Sprintf("git %d.%d", p.GitVersion.Major, p.GitVersion.Minor))
	right := versionStyle.Render(strings.Join(versionParts, " \u00b7 "))

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
