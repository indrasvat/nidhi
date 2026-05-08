package conflict

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/icons"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// RenderConflictScreen renders the conflict preview content area.
// Exported for testing.
func RenderConflictScreen(
	result *git.MergeTreeResult,
	stash *plugin.Stash,
	th theme.Theme,
	iconMode icons.Mode,
	cursor int,
	width, height int,
) string {
	var b strings.Builder

	// Styles — fall back to plain text if no theme.
	var (
		headerStyle  lipgloss.Style
		greenStyle   lipgloss.Style
		conflStyle   lipgloss.Style
		warnStyle    lipgloss.Style
		dimStyle     lipgloss.Style
		activeStyle  lipgloss.Style
		hunkOurStyle lipgloss.Style
		hunkTheir    lipgloss.Style
	)
	if th != nil {
		headerStyle = lipgloss.NewStyle().
			Foreground(th.FgPrimary()).
			Bold(true)
		greenStyle = lipgloss.NewStyle().Foreground(th.SemanticGreen())
		conflStyle = lipgloss.NewStyle().Foreground(th.SemanticYellow()).Bold(true)
		warnStyle = lipgloss.NewStyle().Foreground(th.SemanticCoral())
		dimStyle = lipgloss.NewStyle().Foreground(th.FgDimmed())
		activeStyle = lipgloss.NewStyle().
			Foreground(th.FgPrimary()).
			Background(th.BgElevated())
		hunkOurStyle = lipgloss.NewStyle().Foreground(th.DiffRemovedFg())
		hunkTheir = lipgloss.NewStyle().Foreground(th.DiffAddedFg())
	}

	// ─── Header ────────────────────────────────────────────
	header := fmt.Sprintf("  Conflict Preview: stash@{%d} — %s", stash.Index, stash.Message)
	if width > 0 && len(header) > width {
		header = header[:width-3] + "..."
	}
	if th != nil {
		b.WriteString(headerStyle.Render(header))
	} else {
		b.WriteString(header)
	}
	b.WriteString("\n\n")

	// ─── File list with status indicators ──────────────────
	conflictIcon := icons.Conflict.String(iconMode)
	cleanIcon := icons.Clean.String(iconMode)

	row := 0
	for _, f := range result.Files {
		icon, label := fileStatusDisplay(f, cleanIcon, conflictIcon)
		line := fmt.Sprintf("  %s %s", icon, f.Path)

		// Right-align the label.
		labelPadding := max(width-len(line)-len(label)-4, 2)
		line += strings.Repeat(" ", labelPadding) + label

		if th != nil {
			switch {
			case row == cursor:
				b.WriteString(activeStyle.Render(line))
			case f.Status == git.FileStatusConflicted:
				b.WriteString(conflStyle.Render(line))
			case f.Status == git.FileStatusClean:
				b.WriteString(greenStyle.Render(line))
			default:
				b.WriteString(dimStyle.Render(line))
			}
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
		row++
	}

	// ─── Untracked collisions (FR-10.6a) ───────────────────
	for _, c := range result.UntrackedCollisions {
		line := fmt.Sprintf("  \u26A0 %s", c.Path)
		label := "untracked collision"
		labelPadding := max(width-len(line)-len(label)-4, 2)
		line += strings.Repeat(" ", labelPadding) + label

		if th != nil {
			if row == cursor {
				b.WriteString(activeStyle.Render(line))
			} else {
				b.WriteString(warnStyle.Render(line))
			}
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
		row++
	}

	b.WriteString("\n")

	// ─── Conflict zone preview for first conflicted file ───
	for _, f := range result.Files {
		if f.Status == git.FileStatusConflicted && len(f.ConflictZones) > 0 {
			divider := fmt.Sprintf("  ─── %s conflict zone 1/%d ───", f.Path, len(f.ConflictZones))
			if th != nil {
				b.WriteString(dimStyle.Render(divider))
			} else {
				b.WriteString(divider)
			}
			b.WriteString("\n")

			zone := f.ConflictZones[0]
			ourMarker := "  <<<<<<< HEAD"
			if th != nil {
				b.WriteString(hunkOurStyle.Render(ourMarker))
			} else {
				b.WriteString(ourMarker)
			}
			b.WriteString("\n")

			for line := range strings.SplitSeq(zone.OurContent, "\n") {
				content := "    " + line
				if th != nil {
					b.WriteString(hunkOurStyle.Render(content))
				} else {
					b.WriteString(content)
				}
				b.WriteString("\n")
			}

			b.WriteString("  =======\n")

			for line := range strings.SplitSeq(zone.TheirContent, "\n") {
				content := "    " + line
				if th != nil {
					b.WriteString(hunkTheir.Render(content))
				} else {
					b.WriteString(content)
				}
				b.WriteString("\n")
			}

			stashMarker := "  >>>>>>> stash"
			if th != nil {
				b.WriteString(hunkTheir.Render(stashMarker))
			} else {
				b.WriteString(stashMarker)
			}
			b.WriteString("\n")
			break // Only show first conflicted file's first zone.
		}
	}

	return b.String()
}

func fileStatusDisplay(f git.MergeTreeFile, cleanIcon, conflictIcon string) (icon string, label string) {
	switch f.Status {
	case git.FileStatusClean:
		return cleanIcon, "clean apply"
	case git.FileStatusConflicted:
		zones := len(f.ConflictZones)
		if zones > 0 {
			return conflictIcon, fmt.Sprintf("%d conflict zones", zones)
		}
		return conflictIcon, "conflict"
	default:
		return "?", "unknown"
	}
}
