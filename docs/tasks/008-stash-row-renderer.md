# Task 008: Stash Row Renderer

## Status: TODO

## Depends On
- 003 (Agni theme -- all colors, progressive dimming, stale badge styling)
- 007 (Layout engine -- provides width constraints, responsive tier, column collapse rules)

## Parallelizable With
- 009 (Diff view and file tree -- independent component, can be built simultaneously)

## Problem
The stash row is the most frequently rendered component in nidhi. Every stash in the list is displayed as a row with rich visual information: cursor indicator, index, SHA, message, age, file scope, diff stats, stale badge, and progressive dimming for older entries. The row must adapt to available width (collapsing from two lines to one below 100 cols, truncating messages below 80 cols), support an inline rename mode where the message becomes an editable text input, and use the selected state (`bg.elevated` background) correctly. Getting this component right is critical because it directly affects the user's first impression and the daily browsing experience.

## PRD Reference
- Section 10 Screen 1 (LIST) -- row anatomy, responsive behavior, BubbleTea mapping
- Section 6.1 FR-01 (Stash List & Navigation) -- cursor, stale badge, progressive dimming, auto-messages
- Section 6.2 FR-13 (Rename Plugin) -- inline rename mode
- Section 9.1 (Agni Theme) -- all color tokens for row elements
- Section 9.2 (Typography & Icons) -- Nerd Font / ASCII fallback
- Section 13.5 (Custom Components) -- stash row rationale (~100 lines)
- Section 13.2 (LipGloss v2 Features) -- NewStyle, immutable styles

## Files to Create
- `internal/ui/components/stashrow.go` -- stash row renderer with all visual states
- `internal/ui/components/stashrow_test.go` -- table-driven tests, responsive collapse, golden snapshots

## Execution Steps

### Step 1: Create `internal/ui/components/stashrow.go`

```go
// internal/ui/components/stashrow.go
package components

import (
	"fmt"
	"math"
	"strings"
	"time"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// ─── Agni Theme Color Tokens ────────────────────────────────

// StashRowColors holds all color tokens used in stash row rendering.
type StashRowColors struct {
	BgDeep     string // #07090E — primary background
	BgElevated string // #1A1F2B — selected row background
	FgPrimary  string // #C8CCD4 — message text
	FgSecondary string // #6B7280 — age, secondary info
	FgDimmed   string // #3D4450 — disabled, ghost text
	SHAColor   string // #C678DD — purple for SHA
	AccentGold string // #D4A050 — cursor indicator
	StaleBg    string // #E5C07B — stale badge background
	StaleFg    string // #07090E — stale badge foreground
	GreenFg    string // #73D990 — insertions
	RedFg      string // #FF5F6D — deletions
	BlueFg     string // #61AFEF — branch
}

// DefaultStashRowColors returns the Agni theme colors for stash rows.
func DefaultStashRowColors() StashRowColors {
	return StashRowColors{
		BgDeep:     "#07090E",
		BgElevated: "#1A1F2B",
		FgPrimary:  "#C8CCD4",
		FgSecondary: "#6B7280",
		FgDimmed:   "#3D4450",
		SHAColor:   "#C678DD",
		AccentGold: "#D4A050",
		StaleBg:    "#E5C07B",
		StaleFg:    "#07090E",
		GreenFg:    "#73D990",
		RedFg:      "#FF5F6D",
		BlueFg:     "#61AFEF",
	}
}

// ─── Stash Row Renderer ─────────────────────────────────────

// StashRowParams controls the rendering of a single stash row.
type StashRowParams struct {
	Stash      plugin.Stash
	Selected   bool // True if this row has the cursor.
	Width      int  // Available width for the row.
	UseNerd    bool // Whether to use Nerd Font icons.
	TotalCount int  // Total number of stashes (for progressive dimming calculation).
	Now        time.Time // Current time for relative age calculation.
}

// StashRowRenderer renders stash rows with configurable colors.
type StashRowRenderer struct {
	Colors StashRowColors
}

// NewStashRowRenderer creates a renderer with default Agni colors.
func NewStashRowRenderer() StashRowRenderer {
	return StashRowRenderer{Colors: DefaultStashRowColors()}
}

// Render renders a single stash row as a string.
// Returns a one-line or two-line string depending on width.
func (r StashRowRenderer) Render(p StashRowParams) string {
	c := r.Colors

	// Compute progressive dimming factor.
	// Older stashes get increasingly muted foreground color.
	dimFactor := r.dimmingFactor(p.Stash.Index, p.TotalCount)

	// Choose foreground colors based on selection and dimming.
	var (
		bgColor     = lipgloss.Color(c.BgDeep)
		msgFg       = r.blendColor(c.FgPrimary, c.FgDimmed, dimFactor)
		indexFg     = r.blendColor(c.FgSecondary, c.FgDimmed, dimFactor)
		shaFg       = r.blendColor(c.SHAColor, c.FgDimmed, dimFactor)
		ageFg       = lipgloss.Color(c.FgSecondary)
		statFg      = r.blendColor(c.FgSecondary, c.FgDimmed, dimFactor)
	)

	if p.Selected {
		bgColor = lipgloss.Color(c.BgElevated)
		// Selected row gets full brightness regardless of dimming.
		msgFg = lipgloss.Color(c.FgPrimary)
		indexFg = lipgloss.Color(c.FgPrimary)
		shaFg = lipgloss.Color(c.SHAColor)
	}

	// ── Row elements ────────────────────────────────────
	//
	// Layout: [cursor 2] [index 4] [sha 9] [message ...] [age right-aligned]
	// Line 2: [spacing 15] [file scope + diff stat]

	// Cursor indicator.
	var cursor string
	if p.Selected {
		cursorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(c.AccentGold)).
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
	ageWidth := lipgloss.Width(age) + 2 // age + padding
	msgMaxWidth := p.Width - fixedWidth - ageWidth
	if msgMaxWidth < 5 {
		msgMaxWidth = 5
	}

	// Truncate message if needed.
	if lipgloss.Width(message) > msgMaxWidth {
		message = truncateString(message, msgMaxWidth-1) + "\u2026" // ...
	}

	msgStyle := lipgloss.NewStyle().
		Foreground(msgFg).
		Background(bgColor)
	msgRendered := msgStyle.Render(message)

	// Compute spacing between message and age.
	line1ContentWidth := 2 + 4 + 9 + lipgloss.Width(message)
	gap := p.Width - line1ContentWidth - ageWidth
	if gap < 0 {
		gap = 0
	}
	spacing := lipgloss.NewStyle().
		Background(bgColor).
		Width(gap).
		Render("")

	ageRendered := lipgloss.NewStyle().Background(bgColor).Render(" ") +
		ageStyle.Render(age) +
		lipgloss.NewStyle().Background(bgColor).Render(" ")

	line1 := cursor + indexRendered + shaRendered + msgRendered + spacing + ageRendered

	// Stale badge (appended to age if stale).
	if p.Stash.IsStale {
		badgeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(c.StaleFg)).
			Background(lipgloss.Color(c.StaleBg)).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)
		var staleIcon string
		if p.UseNerd {
			staleIcon = " STALE" // nf-fa-clock_o + STALE
		} else {
			staleIcon = "\u231b STALE" // hourglass + STALE
		}
		line1 += badgeStyle.Render(staleIcon)
	}

	// Pad line 1 to full width.
	line1 = padLine(line1, p.Width, bgColor)

	// ── Line 2 (file scope + diff stat) ─────────────────
	// Only render if width >= 100 (PRD: "Below 100 cols, collapse to single-line rows").
	if p.Width < 100 {
		return line1
	}

	// Second line indentation (align with message column).
	indent := lipgloss.NewStyle().
		Background(bgColor).
		Width(15).
		Render("")

	// File scope summary.
	scope := fileScope(p.Stash)
	scopeStyle := lipgloss.NewStyle().
		Foreground(statFg).
		Background(bgColor)

	// Diff stat: +N/-M
	diffStat := formatDiffStat(p.Stash.Insertions, p.Stash.Deletions, c.GreenFg, c.RedFg, bgColor)

	// File count.
	fileCountStr := fmt.Sprintf("%d file", p.Stash.FileCount)
	if p.Stash.FileCount != 1 {
		fileCountStr += "s"
	}
	fileCountStyle := lipgloss.NewStyle().
		Foreground(statFg).
		Background(bgColor)

	line2 := indent +
		scopeStyle.Render(scope) +
		lipgloss.NewStyle().Background(bgColor).Render(" ") +
		diffStat +
		lipgloss.NewStyle().Background(bgColor).Render(" \u00b7 ") + // middle dot
		fileCountStyle.Render(fileCountStr)

	line2 = padLine(line2, p.Width, bgColor)

	return line1 + "\n" + line2
}

// InlineRenameRow renders a stash row in inline rename mode.
// The message area is replaced with an editable text input value.
func (r StashRowRenderer) InlineRenameRow(p StashRowParams, editText string, cursorPos int) string {
	c := r.Colors

	bgColor := lipgloss.Color(c.BgElevated)
	cursor := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.AccentGold)).
		Background(bgColor).
		Bold(true).
		Render("\u25b8 ")

	indexStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.FgPrimary)).
		Background(bgColor).
		Width(3).
		Align(lipgloss.Right)
	indexRendered := indexStyle.Render(fmt.Sprintf("%d", p.Stash.Index)) +
		lipgloss.NewStyle().Background(bgColor).Render(" ")

	shaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.SHAColor)).
		Background(bgColor)
	shaRendered := shaStyle.Render(p.Stash.ShortSHA[:min(7, len(p.Stash.ShortSHA))]) +
		lipgloss.NewStyle().Background(bgColor).Render("  ")

	// Edit text with cursor indicator.
	editStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.AccentGold)).
		Background(bgColor).
		Bold(true)
	editRendered := editStyle.Render(editText + "\u2588") // block cursor

	line1 := cursor + indexRendered + shaRendered + editRendered

	// Previous message shown dimmed below.
	prevStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.FgDimmed)).
		Background(bgColor).
		Italic(true)
	indent := lipgloss.NewStyle().Background(bgColor).Width(15).Render("")
	line2 := indent + prevStyle.Render("was: "+p.Stash.Message)

	line1 = padLine(line1, p.Width, bgColor)
	line2 = padLine(line2, p.Width, bgColor)

	return line1 + "\n" + line2
}

// ─── Helper Functions ───────────────────────────────────────

// dimmingFactor returns a value between 0.0 (no dimming) and 1.0 (fully dimmed)
// based on the stash's position in the list. Stash 0 = no dimming, last stash = max dimming.
func (r StashRowRenderer) dimmingFactor(index, total int) float64 {
	if total <= 1 {
		return 0
	}
	// Use a square-root curve so dimming is gentle at first, stronger for very old stashes.
	normalized := float64(index) / float64(total-1)
	return math.Sqrt(normalized) * 0.6 // Cap at 60% dimming.
}

// blendColor blends between two hex colors based on factor (0.0 = color1, 1.0 = color2).
// Returns a lipgloss.Color.
func (r StashRowRenderer) blendColor(hex1, hex2 string, factor float64) lipgloss.Color {
	r1, g1, b1 := hexToRGB(hex1)
	r2, g2, b2 := hexToRGB(hex2)

	rr := uint8(float64(r1) + (float64(r2)-float64(r1))*factor)
	gg := uint8(float64(g1) + (float64(g2)-float64(g1))*factor)
	bb := uint8(float64(b1) + (float64(b2)-float64(b1))*factor)

	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", rr, gg, bb))
}

// hexToRGB converts a hex color string to RGB components.
func hexToRGB(hex string) (uint8, uint8, uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
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

// fileScope produces a brief file scope summary (e.g. "src/auth:" or "3 dirs").
func fileScope(s plugin.Stash) string {
	// For now, use a simplified scope based on message or file count.
	// Full implementation would parse file paths from the diff.
	if s.FileCount <= 0 {
		return ""
	}
	if s.FileCount == 1 {
		return "1 file:"
	}
	return fmt.Sprintf("%d files:", s.FileCount)
}

// formatDiffStat renders +N/-M with green/red coloring.
func formatDiffStat(insertions, deletions int, greenHex, redHex string, bg lipgloss.Color) string {
	addStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(greenHex)).
		Background(bg)
	delStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(redHex)).
		Background(bg)
	return addStyle.Render(fmt.Sprintf("+%d", insertions)) +
		lipgloss.NewStyle().Background(bg).Render("/") +
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
func padLine(line string, width int, bg lipgloss.Color) string {
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
```

### Step 2: Create `internal/ui/components/stashrow_test.go`

```go
// internal/ui/components/stashrow_test.go
package components

import (
	"strings"
	"testing"
	"time"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

func testStash(index int) plugin.Stash {
	return plugin.Stash{
		Index:        index,
		SHA:          "a3f7b2c1234567890abcdef1234567890abcdef0",
		ShortSHA:     "a3f7b2c",
		Message:      "Fix auth token refresh",
		RawMessage:   "WIP on main: a3f7b2c Fix auth token refresh",
		Branch:       "main",
		Date:         time.Now().Add(-3 * time.Hour),
		FileCount:    3,
		Insertions:   42,
		Deletions:    17,
		IsStale:      false,
		HasUntracked: false,
	}
}

func staleStash(index int) plugin.Stash {
	s := testStash(index)
	s.Date = time.Now().Add(-30 * 24 * time.Hour) // 30 days ago
	s.IsStale = true
	s.Message = "Old forgotten stash"
	return s
}

func TestStashRow_ContainsExpectedElements(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(rendered)

	checks := []struct {
		name     string
		contains string
	}{
		{"index", "0"},
		{"SHA", "a3f7b2c"},
		{"message", "Fix auth token refresh"},
		{"age", "3h ago"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(plain, c.contains) {
				t.Errorf("row should contain %q, got:\n%s", c.contains, plain)
			}
		})
	}
}

func TestStashRow_SelectedState(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	selected := r.Render(StashRowParams{
		Stash:      testStash(0),
		Selected:   true,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(selected)

	// Selected row should have the cursor indicator.
	if !strings.Contains(plain, "\u25b8") { // ▸
		t.Error("selected row should have cursor indicator ▸")
	}
}

func TestStashRow_UnselectedNoCursor(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	unselected := r.Render(StashRowParams{
		Stash:      testStash(1),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(unselected)
	if strings.Contains(plain, "\u25b8") { // ▸
		t.Error("unselected row should NOT have cursor indicator ▸")
	}
}

func TestStashRow_StaleBadge(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      staleStash(2),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "STALE") {
		t.Error("stale stash should have STALE badge")
	}
}

func TestStashRow_NoStaleBadgeWhenNotStale(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	plain := stripAnsi(rendered)
	if strings.Contains(plain, "STALE") {
		t.Error("non-stale stash should NOT have STALE badge")
	}
}

func TestStashRow_SingleLineBelow100Cols(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0),
		Selected:   false,
		Width:      80, // Below 100 -> single line.
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) != 1 {
		t.Errorf("below 100 cols: expected 1 line, got %d lines", len(lines))
	}
}

func TestStashRow_TwoLinesAbove100Cols(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0),
		Selected:   false,
		Width:      120, // Above 100 -> two lines.
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) != 2 {
		t.Errorf("above 100 cols: expected 2 lines, got %d", len(lines))
	}
}

func TestStashRow_SecondLineContainsDiffStat(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	rendered := r.Render(StashRowParams{
		Stash:      testStash(0),
		Selected:   false,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	})

	lines := strings.Split(rendered, "\n")
	if len(lines) < 2 {
		t.Fatal("expected 2 lines")
	}

	plain := stripAnsi(lines[1])
	if !strings.Contains(plain, "+42") {
		t.Errorf("second line should contain insertions '+42', got: %q", plain)
	}
	if !strings.Contains(plain, "-17") {
		t.Errorf("second line should contain deletions '-17', got: %q", plain)
	}
}

func TestStashRow_ProgressiveDimming(t *testing.T) {
	r := NewStashRowRenderer()

	// Dimming factor for first stash should be 0 (no dimming).
	factor0 := r.dimmingFactor(0, 10)
	if factor0 != 0 {
		t.Errorf("dimmingFactor(0, 10) = %f, want 0", factor0)
	}

	// Dimming factor for last stash should be > 0.
	factorLast := r.dimmingFactor(9, 10)
	if factorLast <= 0 {
		t.Errorf("dimmingFactor(9, 10) = %f, want > 0", factorLast)
	}
	if factorLast > 0.61 { // Capped at ~0.6.
		t.Errorf("dimmingFactor(9, 10) = %f, should be capped at ~0.6", factorLast)
	}

	// Middle stash should have intermediate dimming.
	factorMid := r.dimmingFactor(5, 10)
	if factorMid <= factor0 || factorMid >= factorLast {
		t.Errorf("dimmingFactor(5, 10) = %f, should be between %f and %f",
			factorMid, factor0, factorLast)
	}
}

func TestStashRow_DimmingFactorSingleStash(t *testing.T) {
	r := NewStashRowRenderer()

	// Single stash: no dimming.
	factor := r.dimmingFactor(0, 1)
	if factor != 0 {
		t.Errorf("dimmingFactor(0, 1) = %f, want 0", factor)
	}
}

func TestStashRow_InlineRename(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	rendered := r.InlineRenameRow(StashRowParams{
		Stash:      testStash(0),
		Selected:   true,
		Width:      120,
		UseNerd:    false,
		TotalCount: 5,
		Now:        now,
	}, "New message text", 16)

	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "New message text") {
		t.Error("inline rename should show the edit text")
	}
	if !strings.Contains(plain, "was:") {
		t.Error("inline rename should show the previous message")
	}
}

// ─── Relative Age Tests ─────────────────────────────────────

func TestRelativeAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "now"},
		{"minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"hours", now.Add(-3 * time.Hour), "3h ago"},
		{"days", now.Add(-2 * 24 * time.Hour), "2d ago"},
		{"months", now.Add(-45 * 24 * time.Hour), "1mo ago"},
		{"years", now.Add(-400 * 24 * time.Hour), "1y ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeAge(tt.date, now)
			if got != tt.want {
				t.Errorf("relativeAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── Color Blending Tests ───────────────────────────────────

func TestHexToRGB(t *testing.T) {
	tests := []struct {
		hex     string
		r, g, b uint8
	}{
		{"#FFFFFF", 255, 255, 255},
		{"#000000", 0, 0, 0},
		{"#C678DD", 198, 120, 221},
		{"#07090E", 7, 9, 14},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			r, g, b := hexToRGB(tt.hex)
			if r != tt.r || g != tt.g || b != tt.b {
				t.Errorf("hexToRGB(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tt.hex, r, g, b, tt.r, tt.g, tt.b)
			}
		})
	}
}

func TestBlendColor(t *testing.T) {
	r := NewStashRowRenderer()

	// Factor 0 should return the first color.
	c0 := r.blendColor("#FFFFFF", "#000000", 0.0)
	if string(c0) != "#FFFFFF" {
		t.Errorf("blendColor(white, black, 0) = %s, want #FFFFFF", c0)
	}

	// Factor 1 should return the second color.
	c1 := r.blendColor("#FFFFFF", "#000000", 1.0)
	if string(c1) != "#000000" {
		t.Errorf("blendColor(white, black, 1) = %s, want #000000", c1)
	}

	// Factor 0.5 should be a middle gray.
	cMid := r.blendColor("#FFFFFF", "#000000", 0.5)
	midStr := string(cMid)
	// Should be approximately #808080 (127/128 due to rounding).
	if !strings.HasPrefix(midStr, "#7") && !strings.HasPrefix(midStr, "#8") {
		t.Errorf("blendColor(white, black, 0.5) = %s, want ~#7F7F7F", midStr)
	}
}

// ─── Width constraint tests ─────────────────────────────────

func TestStashRow_WidthConstraint(t *testing.T) {
	r := NewStashRowRenderer()
	now := time.Now()

	widths := []int{60, 80, 100, 120, 200}

	for _, w := range widths {
		rendered := r.Render(StashRowParams{
			Stash:      testStash(0),
			Selected:   false,
			Width:      w,
			UseNerd:    false,
			TotalCount: 5,
			Now:        now,
		})

		lines := strings.Split(rendered, "\n")
		for i, line := range lines {
			lineWidth := lipgloss.Width(line)
			if lineWidth > w+2 { // Allow small tolerance for border chars.
				t.Errorf("width=%d, line %d: rendered width = %d, exceeds target",
					w, i, lineWidth)
			}
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxWidth int
		want     string
	}{
		{"hello world", 5, "hello"},
		{"short", 10, "short"},
		{"", 5, ""},
		{"abc", 0, ""},
		{"unicode \u00e4\u00f6\u00fc", 9, "unicode \u00e4"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxWidth)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q",
					tt.input, tt.maxWidth, got, tt.want)
			}
		})
	}
}

func TestFileScope(t *testing.T) {
	tests := []struct {
		name      string
		fileCount int
		want      string
	}{
		{"no files", 0, ""},
		{"one file", 1, "1 file:"},
		{"multiple files", 3, "3 files:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := plugin.Stash{FileCount: tt.fileCount}
			got := fileScope(s)
			if got != tt.want {
				t.Errorf("fileScope() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

## Verification

### Functional
```bash
# From project root
cd /Users/indrasvat/code/github.com/indrasvat-nidhi

# Run stash row tests
go test -v -race ./internal/ui/components/...

# Check compilation
go build ./internal/ui/components/...

# Linter
make lint

# Full CI
make ci
```

### Visual Spot Check
```bash
# After the component is built and the LIST screen is wired (future task),
# verify visually by running:
# go run ./cmd/nidhi
# In a repo with stashes, verify:
# - Cursor ▸ appears on selected row
# - SHA is purple
# - Age is right-aligned
# - Stale badge appears on old stashes
# - Older stashes are progressively dimmed
# - Below 100 cols, rows collapse to single line
```

## Completion Criteria
1. `stashrow.go` compiles without errors
2. All tests in `stashrow_test.go` pass with `go test -v -race`
3. Row renders all required elements: cursor, index, SHA, message, relative age
4. Second line renders file scope, diff stat (+/-), file count when width >= 100
5. Below 100 cols, row collapses to single line (no second line)
6. Selected row uses `bg.elevated` background and cursor indicator `\u25b8`
7. Progressive dimming: stash 0 has full brightness, last stash is ~60% dimmed
8. `blendColor` correctly interpolates between two hex colors
9. `relativeAge` produces human-readable strings (now, Xm ago, Xh ago, Xd ago, Xmo ago, Xy ago)
10. Stale badge renders with yellow background for stashes marked `IsStale`
11. `InlineRenameRow` renders edit text with cursor and dimmed previous message
12. Row width never exceeds the specified `Width` parameter
13. `make lint` passes with no warnings

## Commit
```
feat(ui): add stash row renderer with progressive dimming and responsive collapse

Implement the stash row component for LIST mode:
- Renders cursor indicator, index, SHA (purple), message, relative age
- Two-line layout with file scope + diff stat at width >= 100, collapses
  to single line below 100 cols
- Progressive dimming: older stashes get increasingly muted foreground
  via square-root curve color blending (capped at 60%)
- STALE badge with yellow background for stashes past staleness threshold
- Selected state: bg.elevated background, full brightness override
- Inline rename mode: message becomes editable, previous shown dimmed
- Table-driven tests for rendering, dimming, age formatting, truncation,
  width constraints, and responsive collapse
```

## Session Protocol
1. Read this task file completely before writing any code.
2. Verify Task 003 (theme) and Task 007 (layout) are complete.
3. Write `stashrow.go` with all rendering logic.
4. Write `stashrow_test.go` with comprehensive table-driven tests.
5. Run `go test -v -race ./internal/ui/components/...` and fix any failures.
6. Visually inspect rendered output by printing to stdout in a scratch file (not `_test.go`).
7. Update `docs/PROGRESS.md` and `CLAUDE.md` Learnings section with any discoveries.
