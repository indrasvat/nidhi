package components

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// DiffLineType classifies a line in a unified diff.
type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdded
	DiffLineRemoved
	DiffLineHunk   // @@ ... @@
	DiffLineHeader // diff --git, index, ---, +++
)

// DiffLine represents a single parsed line from a unified diff.
type DiffLine struct {
	Type    DiffLineType
	Content string
	OldNum  int // Line number in the old file (0 if not applicable).
	NewNum  int // Line number in the new file (0 if not applicable).
}

// DiffViewModel manages a scrollable diff view with syntax coloring.
type DiffViewModel struct {
	lines  []DiffLine
	theme  theme.Theme
	width  int
	height int
	offset int // Scroll offset (first visible line).
	ready  bool
}

// NewDiffViewModel creates a new diff view model.
func NewDiffViewModel(th theme.Theme, width, height int) DiffViewModel {
	return DiffViewModel{
		theme:  th,
		width:  width,
		height: height,
	}
}

// SetContent parses and displays a unified diff string.
func (d *DiffViewModel) SetContent(diffStr string) {
	d.lines = parseDiff(diffStr)
	d.offset = 0
	d.ready = len(d.lines) > 0
}

// SetSize updates the viewport dimensions.
func (d *DiffViewModel) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// ScrollDown scrolls down by n lines.
func (d *DiffViewModel) ScrollDown(n int) {
	maxOffset := max(len(d.lines)-d.height, 0)
	d.offset = min(d.offset+n, maxOffset)
}

// ScrollUp scrolls up by n lines.
func (d *DiffViewModel) ScrollUp(n int) {
	d.offset = max(d.offset-n, 0)
}

// ScrollPercent returns the current scroll percentage (0.0-1.0).
func (d *DiffViewModel) ScrollPercent() float64 {
	if len(d.lines) <= d.height {
		return 1.0
	}
	maxOffset := len(d.lines) - d.height
	return float64(d.offset) / float64(maxOffset)
}

// LineCount returns the total number of diff lines.
func (d *DiffViewModel) LineCount() int {
	return len(d.lines)
}

// View returns the rendered diff viewport.
func (d *DiffViewModel) View() string {
	if !d.ready {
		style := lipgloss.NewStyle().
			Foreground(d.theme.FgDimmed()).
			Width(d.width).
			Height(d.height)
		return style.Render("No diff loaded")
	}

	return d.renderLines()
}

// renderLines renders visible diff lines with syntax coloring and line numbers.
func (d *DiffViewModel) renderLines() string {
	if len(d.lines) == 0 {
		return ""
	}

	th := d.theme
	bg := th.BgDeep()

	lineNumStyle := lipgloss.NewStyle().
		Foreground(th.FgDimmed()).
		Background(bg).
		Width(4).
		Align(lipgloss.Right)

	addedStyle := lipgloss.NewStyle().
		Foreground(th.DiffAddedFg()).
		Background(th.DiffAddedBg())

	removedStyle := lipgloss.NewStyle().
		Foreground(th.DiffRemovedFg()).
		Background(th.DiffRemovedBg())

	hunkStyle := lipgloss.NewStyle().
		Foreground(th.DiffHunk()).
		Background(bg).
		Bold(true)

	contextStyle := lipgloss.NewStyle().
		Foreground(th.FgSecondary()).
		Background(bg)

	headerStyle := lipgloss.NewStyle().
		Foreground(th.DiffHunk()).
		Background(bg).
		Bold(true)

	sepStyle := lipgloss.NewStyle().
		Foreground(th.FgDimmed()).
		Background(bg)

	// Determine visible range.
	end := min(d.offset+d.height, len(d.lines))
	contentWidth := max(d.width-6, 1) // 4 (line num) + 3 (separator)

	var rendered []string
	for i := d.offset; i < end; i++ {
		dl := d.lines[i]
		var lineNum string
		var content string

		switch dl.Type {
		case DiffLineAdded:
			if dl.NewNum > 0 {
				lineNum = lineNumStyle.Render(fmt.Sprintf("%d", dl.NewNum))
			} else {
				lineNum = lineNumStyle.Render("")
			}
			content = addedStyle.Render(padToWidth(dl.Content, contentWidth))
		case DiffLineRemoved:
			if dl.OldNum > 0 {
				lineNum = lineNumStyle.Render(fmt.Sprintf("%d", dl.OldNum))
			} else {
				lineNum = lineNumStyle.Render("")
			}
			content = removedStyle.Render(padToWidth(dl.Content, contentWidth))
		case DiffLineHunk:
			lineNum = lineNumStyle.Render("")
			content = hunkStyle.Render(padToWidth(dl.Content, contentWidth))
		case DiffLineHeader:
			lineNum = lineNumStyle.Render("")
			content = headerStyle.Render(padToWidth(dl.Content, contentWidth))
		default: // Context
			num := ""
			if dl.NewNum > 0 {
				num = fmt.Sprintf("%d", dl.NewNum)
			}
			lineNum = lineNumStyle.Render(num)
			content = contextStyle.Render(padToWidth(dl.Content, contentWidth))
		}

		sep := sepStyle.Render(" \u2502 ") // vertical line separator
		rendered = append(rendered, lineNum+sep+content)
	}

	return strings.Join(rendered, "\n")
}

// padToWidth pads a string with spaces to the target width.
func padToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// ─── Diff Parser ────────────────────────────────────────────

// parseDiff parses a unified diff string into typed lines with line numbers.
func parseDiff(diff string) []DiffLine {
	if diff == "" {
		return nil
	}
	rawLines := strings.Split(diff, "\n")
	var result []DiffLine

	var oldNum, newNum int

	for _, raw := range rawLines {
		if raw == "" && len(result) > 0 {
			result = append(result, DiffLine{Type: DiffLineContext, Content: "", NewNum: newNum})
			newNum++
			oldNum++
			continue
		}

		switch {
		case strings.HasPrefix(raw, "@@"):
			oldNum, newNum = parseHunkHeader(raw)
			result = append(result, DiffLine{Type: DiffLineHunk, Content: raw})

		case strings.HasPrefix(raw, "+++ "):
			result = append(result, DiffLine{Type: DiffLineHeader, Content: raw})

		case strings.HasPrefix(raw, "--- "):
			result = append(result, DiffLine{Type: DiffLineHeader, Content: raw})

		case strings.HasPrefix(raw, "+"):
			result = append(result, DiffLine{
				Type:    DiffLineAdded,
				Content: raw,
				NewNum:  newNum,
			})
			newNum++

		case strings.HasPrefix(raw, "-"):
			result = append(result, DiffLine{
				Type:    DiffLineRemoved,
				Content: raw,
				OldNum:  oldNum,
			})
			oldNum++

		case strings.HasPrefix(raw, "diff ") || strings.HasPrefix(raw, "index "):
			result = append(result, DiffLine{Type: DiffLineHeader, Content: raw})

		default:
			result = append(result, DiffLine{
				Type:    DiffLineContext,
				Content: raw,
				OldNum:  oldNum,
				NewNum:  newNum,
			})
			oldNum++
			newNum++
		}
	}

	return result
}

// parseHunkHeader extracts starting line numbers from a @@ header.
func parseHunkHeader(header string) (oldStart, newStart int) {
	header = strings.TrimPrefix(header, "@@")
	idx := strings.Index(header, "@@")
	if idx > 0 {
		header = header[:idx]
	}
	header = strings.TrimSpace(header)

	for p := range strings.FieldsSeq(header) {
		if strings.HasPrefix(p, "-") {
			_, _ = fmt.Sscanf(p, "-%d", &oldStart)
		} else if strings.HasPrefix(p, "+") {
			_, _ = fmt.Sscanf(p, "+%d", &newStart)
		}
	}
	return
}
