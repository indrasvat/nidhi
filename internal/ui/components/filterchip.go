package components

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// Chip represents a single filter chip.
type Chip struct {
	Label  string
	Active bool
}

// ChipGroupModel manages a group of toggle chips.
type ChipGroupModel struct {
	Chips  []Chip
	cursor int
	theme  theme.Theme
}

// NewChipGroupModel creates a chip group with the given labels and theme.
// The first chip starts active.
func NewChipGroupModel(labels []string, th theme.Theme) ChipGroupModel {
	chips := make([]Chip, len(labels))
	for i, l := range labels {
		chips[i] = Chip{Label: l, Active: i == 0}
	}
	return ChipGroupModel{
		Chips:  chips,
		cursor: 0,
		theme:  th,
	}
}

// SearchScopeChips creates the standard search scope chip group.
// PRD Section 10 Screen 5: All, Messages, Files, Diffs, Branch.
func SearchScopeChips(th theme.Theme) ChipGroupModel {
	return NewChipGroupModel([]string{"All", "Messages", "Files", "Diffs", "Branch"}, th)
}

// Next moves focus to the next chip (wraps around).
func (m *ChipGroupModel) Next() {
	if len(m.Chips) == 0 {
		return
	}
	m.cursor = (m.cursor + 1) % len(m.Chips)
}

// Prev moves focus to the previous chip (wraps around).
func (m *ChipGroupModel) Prev() {
	if len(m.Chips) == 0 {
		return
	}
	m.cursor = (m.cursor - 1 + len(m.Chips)) % len(m.Chips)
}

// Toggle toggles the active state of the focused chip.
// If "All" (index 0) is toggled on, all others are deactivated.
// If any other chip is toggled on, "All" is deactivated.
func (m *ChipGroupModel) Toggle() {
	if len(m.Chips) == 0 {
		return
	}

	m.Chips[m.cursor].Active = !m.Chips[m.cursor].Active

	if m.cursor == 0 && m.Chips[0].Active {
		// "All" toggled on: deactivate all others.
		for i := 1; i < len(m.Chips); i++ {
			m.Chips[i].Active = false
		}
	} else if m.cursor != 0 && m.Chips[m.cursor].Active {
		// Specific chip toggled on: deactivate "All".
		m.Chips[0].Active = false
	}

	// If no chips are active, reactivate "All".
	anyActive := false
	for _, c := range m.Chips {
		if c.Active {
			anyActive = true
			break
		}
	}
	if !anyActive {
		m.Chips[0].Active = true
	}
}

// ActiveLabels returns the labels of all active chips.
func (m *ChipGroupModel) ActiveLabels() []string {
	var labels []string
	for _, c := range m.Chips {
		if c.Active {
			labels = append(labels, c.Label)
		}
	}
	return labels
}

// Cursor returns the currently focused chip index.
func (m *ChipGroupModel) Cursor() int {
	return m.cursor
}

// SetCursor sets the cursor position directly.
func (m *ChipGroupModel) SetCursor(pos int) {
	if pos >= 0 && pos < len(m.Chips) {
		m.cursor = pos
	}
}

// View renders the chip group as a horizontal row.
func (m *ChipGroupModel) View() string {
	if len(m.Chips) == 0 {
		return ""
	}

	th := m.theme
	var chips []string

	for i, chip := range m.Chips {
		var style lipgloss.Style

		if chip.Active {
			style = lipgloss.NewStyle().
				Foreground(th.BgDeep()).
				Background(th.AccentGold()).
				Bold(true).
				PaddingLeft(1).
				PaddingRight(1)
		} else {
			style = lipgloss.NewStyle().
				Foreground(th.FgSecondary()).
				Background(th.BgElevated()).
				PaddingLeft(1).
				PaddingRight(1)
		}

		// Add focus indicator for the currently selected chip.
		if i == m.cursor {
			style = style.
				Border(lipgloss.RoundedBorder()).
				BorderForeground(th.AccentBright())
		}

		chips = append(chips, style.Render(chip.Label))
	}

	return strings.Join(chips, " ")
}
