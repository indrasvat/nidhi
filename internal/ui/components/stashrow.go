package components

import (
	"fmt"
	"image/color"
	"math"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// StashRowParams controls the rendering of a single stash row.
type StashRowParams struct {
	Stash      plugin.Stash
	Selected   bool      // True if this row has the cursor.
	Width      int       // Available width for the row.
	UseNerd    bool      // Whether to use Nerd Font icons.
	TotalCount int       // Total number of stashes (for progressive dimming).
	Now        time.Time // Current time for relative age calculation.
}

// StashRowRenderer renders stash rows with theme-aware colors.
type StashRowRenderer struct {
	Theme theme.Theme
}

// NewStashRowRenderer creates a renderer with the given theme.
func NewStashRowRenderer(th theme.Theme) StashRowRenderer {
	return StashRowRenderer{Theme: th}
}

// Render renders a single stash row as a string.
// Returns a one-line or two-line string depending on width.
func (r StashRowRenderer) Render(p StashRowParams) string {
	th := r.Theme

	// Compute progressive dimming factor.
	dimFactor := r.dimmingFactor(p.Stash.Index, p.TotalCount)

	// Choose foreground colors based on selection and dimming.
	var (
		bgColor color.Color
		msgFg   color.Color
		indexFg color.Color
		shaFg   color.Color
		ageFg   = th.FgSecondary()
		statFg  color.Color
	)

	if p.Selected {
		bgColor = th.BgElevated()
		// Selected row gets full brightness regardless of dimming.
		msgFg = th.FgPrimary()
		indexFg = th.AccentBright()
		shaFg = th.SemanticPurple()
		statFg = th.FgSecondary()
	} else {
		bgColor = th.BgDeep()
		msgFg = blendColor(th.FgPrimary(), th.FgDimmed(), dimFactor)
		indexFg = blendColor(th.FgSecondary(), th.FgDimmed(), dimFactor)
		shaFg = blendColor(th.SemanticPurple(), th.FgDimmed(), dimFactor)
		statFg = blendColor(th.FgSecondary(), th.FgDimmed(), dimFactor)
	}

	// ── Row elements ────────────────────────────────────
	// Layout: [cursor 2] [index 4] [sha 9] [message ...] [age right-aligned]
	// Line 2: [spacing 15] [branch + diff stat + file count]

	// Cursor indicator.
	var cursor string
	if p.Selected {
		cursorStyle := lipgloss.NewStyle().
			Foreground(th.AccentGold()).
			Background(bgColor).
			Bold(true)
		cursor = cursorStyle.Render("\u25b8 ") // ▸
	} else {
		cursor = lipgloss.NewStyle().Background(bgColor).Render("  ")
	}

	// Index.
	indexStr := fmt.Sprintf("%d", p.Stash.Index)
	indexStyle := lipgloss.NewStyle().
		Foreground(indexFg).
		Background(bgColor).
		Width(3).
		Align(lipgloss.Right)
	indexRendered := indexStyle.Render(indexStr) +
		lipgloss.NewStyle().Background(bgColor).Render(" ")

	// SHA.
	sha := p.Stash.ShortSHA
	if len(sha) > 7 {
		sha = sha[:7]
	}
	shaStyle := lipgloss.NewStyle().
		Foreground(shaFg).
		Background(bgColor)
	shaRendered := shaStyle.Render(sha) +
		lipgloss.NewStyle().Background(bgColor).Render("  ")

	// Message.
	message := p.Stash.Message
	if message == "" {
		message = p.Stash.RawMessage
	}

	// Age (right-aligned).
	age := relativeAge(p.Stash.Date, p.Now)
	ageStyle := lipgloss.NewStyle().
		Foreground(ageFg).
		Background(bgColor)

	// Compute available width for message.
	fixedWidth := 2 + 4 + 9 // cursor + index + sha
	ageWidth := lipgloss.Width(age) + 2
	msgMaxWidth := max(p.Width-fixedWidth-ageWidth, 5)

	// Truncate message if needed.
	if lipgloss.Width(message) > msgMaxWidth {
		message = truncateString(message, msgMaxWidth-1) + "\u2026" // ellipsis
	}

	msgStyle := lipgloss.NewStyle().
		Foreground(msgFg).
		Background(bgColor)
	msgRendered := msgStyle.Render(message)

	// Compute spacing between message and age.
	line1ContentWidth := 2 + 4 + 9 + lipgloss.Width(message)
	gap := max(p.Width-line1ContentWidth-ageWidth, 0)
	spacing := lipgloss.NewStyle().
		Background(bgColor).
		Width(gap).
		Render("")

	ageRendered := lipgloss.NewStyle().Background(bgColor).Render(" ") +
		ageStyle.Render(age) +
		lipgloss.NewStyle().Background(bgColor).Render(" ")

	line1 := cursor + indexRendered + shaRendered + msgRendered + spacing + ageRendered

	// Stale badge (appended to line 1 if stale).
	if p.Stash.IsStale {
		badgeStyle := th.StaleStyle()
		var staleIcon string
		if p.UseNerd {
			staleIcon = " STALE"
		} else {
			staleIcon = "\u231b STALE" // hourglass + STALE
		}
		line1 += badgeStyle.Render(staleIcon)
	}

	// Pad line 1 to full width.
	line1 = padLine(line1, p.Width, bgColor)

	// ── Line 2 (branch + diff stat + file count) ───────
	// Only render if width >= 100 (PRD: "Below 100 cols, collapse to single-line rows").
	if p.Width < 100 {
		return line1
	}

	// Second line indentation (align with message column).
	indent := lipgloss.NewStyle().
		Background(bgColor).
		Width(15).
		Render("")

	// Branch.
	branchFg := blendColor(th.SemanticAqua(), th.FgDimmed(), dimFactor)
	if p.Selected {
		branchFg = th.SemanticAqua()
	}
	branchStyle := lipgloss.NewStyle().
		Foreground(branchFg).
		Background(bgColor)
	branchRendered := branchStyle.Render("\u23e5 " + p.Stash.Branch) // ⎇

	// Diff stat: +N/-M
	diffStat := formatDiffStat(p.Stash.Insertions, p.Stash.Deletions, th, bgColor)

	// File count.
	fileCountStr := fmt.Sprintf("%d file", p.Stash.FileCount)
	if p.Stash.FileCount != 1 {
		fileCountStr += "s"
	}
	fileCountStyle := lipgloss.NewStyle().
		Foreground(statFg).
		Background(bgColor)

	sepStyle := lipgloss.NewStyle().
		Foreground(th.FgDimmed()).
		Background(bgColor)

	line2 := indent +
		branchRendered +
		sepStyle.Render(" \u00b7 ") + // middle dot
		diffStat +
		sepStyle.Render(" \u00b7 ") +
		fileCountStyle.Render(fileCountStr)

	// Tags (staged, untracked).
	if p.Stash.HasUntracked {
		tagStyle := lipgloss.NewStyle().
			Foreground(th.SemanticCoral()).
			Background(bgColor).
			Bold(true)
		line2 += sepStyle.Render(" ") + tagStyle.Render("+new")
	}

	line2 = padLine(line2, p.Width, bgColor)

	return line1 + "\n" + line2
}

// InlineRenameRow renders a stash row in inline rename mode.
func (r StashRowRenderer) InlineRenameRow(p StashRowParams, editText string, cursorPos int) string {
	th := r.Theme
	bgColor := th.BgElevated()

	cursor := lipgloss.NewStyle().
		Foreground(th.AccentGold()).
		Background(bgColor).
		Bold(true).
		Render("\u25b8 ")

	indexStyle := lipgloss.NewStyle().
		Foreground(th.AccentBright()).
		Background(bgColor).
		Width(3).
		Align(lipgloss.Right)
	indexRendered := indexStyle.Render(fmt.Sprintf("%d", p.Stash.Index)) +
		lipgloss.NewStyle().Background(bgColor).Render(" ")

	sha := p.Stash.ShortSHA
	if len(sha) > 7 {
		sha = sha[:7]
	}
	shaStyle := lipgloss.NewStyle().
		Foreground(th.SemanticPurple()).
		Background(bgColor)
	shaRendered := shaStyle.Render(sha) +
		lipgloss.NewStyle().Background(bgColor).Render("  ")

	// Edit text with block cursor.
	editStyle := lipgloss.NewStyle().
		Foreground(th.AccentGold()).
		Background(bgColor).
		Bold(true)
	editRendered := editStyle.Render(editText + "\u2588") // block cursor

	line1 := cursor + indexRendered + shaRendered + editRendered

	// Previous message shown dimmed below.
	prevStyle := lipgloss.NewStyle().
		Foreground(th.FgDimmed()).
		Background(bgColor).
		Italic(true)
	indentPad := lipgloss.NewStyle().Background(bgColor).Width(15).Render("")
	line2 := indentPad + prevStyle.Render("was: "+p.Stash.Message)

	line1 = padLine(line1, p.Width, bgColor)
	line2 = padLine(line2, p.Width, bgColor)

	return line1 + "\n" + line2
}

// ─── Helper Functions ───────────────────────────────────────

// dimmingFactor returns a value between 0.0 (no dimming) and 1.0 (fully dimmed)
// based on the stash's position in the list.
func (r StashRowRenderer) dimmingFactor(index, total int) float64 {
	if total <= 1 {
		return 0
	}
	normalized := float64(index) / float64(total-1)
	return math.Sqrt(normalized) * 0.6 // Cap at 60% dimming.
}

// blendColor blends between two colors based on factor (0.0 = c1, 1.0 = c2).
func blendColor(c1, c2 color.Color, factor float64) color.Color {
	r1, g1, b1, _ := c1.RGBA()
	r2, g2, b2, _ := c2.RGBA()

	rr := uint8(float64(r1>>8) + (float64(r2>>8)-float64(r1>>8))*factor)
	gg := uint8(float64(g1>>8) + (float64(g2>>8)-float64(g1>>8))*factor)
	bb := uint8(float64(b1>>8) + (float64(b2>>8)-float64(b1>>8))*factor)

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", rr, gg, bb))
}

// relativeAge formats a time as a relative age string.
func relativeAge(t time.Time, now time.Time) string {
	d := now.Sub(t)

	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / (24 * 30))
		return fmt.Sprintf("%dmo ago", months)
	default:
		years := int(d.Hours() / (24 * 365))
		return fmt.Sprintf("%dy ago", years)
	}
}

// formatDiffStat renders +N/-M with green/red coloring using theme.
func formatDiffStat(insertions, deletions int, th theme.Theme, bg color.Color) string {
	addStyle := lipgloss.NewStyle().
		Foreground(th.SemanticGreen()).
		Background(bg)
	delStyle := lipgloss.NewStyle().
		Foreground(th.SemanticRed()).
		Background(bg)
	sepStyle := lipgloss.NewStyle().
		Foreground(th.FgDimmed()).
		Background(bg)
	return addStyle.Render(fmt.Sprintf("+%d", insertions)) +
		sepStyle.Render("/") +
		delStyle.Render(fmt.Sprintf("-%d", deletions))
}

// truncateString truncates a string to maxWidth visible characters.
func truncateString(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	return string(runes[:maxWidth])
}

// padLine pads a rendered line to the full width with the given background color.
func padLine(line string, width int, bg color.Color) string {
	lineWidth := lipgloss.Width(line)
	if lineWidth >= width {
		return line
	}
	pad := lipgloss.NewStyle().
		Background(bg).
		Width(width - lineWidth).
		Render("")
	return line + pad
}
