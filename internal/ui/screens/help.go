package screens

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// KeybindCategory groups related keybindings for display.
type KeybindCategory struct {
	Name     string
	Bindings []KeybindEntry
}

// KeybindEntry is a single keybinding for display.
type KeybindEntry struct {
	Key         string // e.g. "j/k", "Shift+J", "^d"
	Description string // e.g. "Move cursor up/down"
	ModeBadge   string // e.g. "LIST", "ALL", "PREVIEW"
}

// HelpOverlay renders the full keybind reference as a modal overlay.
// Triggered by `?` key, uses LipGloss Canvas compositing for z-layered display.
type HelpOverlay struct {
	visible    bool
	scrollY    int
	theme      theme.Theme
	categories []KeybindCategory
}

// NewHelpOverlay creates the help overlay with all keybindings from PRD §11.2.
func NewHelpOverlay(th theme.Theme) *HelpOverlay {
	return &HelpOverlay{
		theme: th,
		categories: []KeybindCategory{
			{
				Name: "Global",
				Bindings: []KeybindEntry{
					{Key: "q / ^c", Description: "Quit", ModeBadge: "ALL"},
					{Key: "?", Description: "Toggle help overlay", ModeBadge: "ALL"},
					{Key: "Esc", Description: "Back / close overlay", ModeBadge: "ALL"},
				},
			},
			{
				Name: "Navigation",
				Bindings: []KeybindEntry{
					{Key: "j / ↓", Description: "Cursor down", ModeBadge: "LIST"},
					{Key: "k / ↑", Description: "Cursor up", ModeBadge: "LIST"},
					{Key: "g", Description: "Jump to first stash", ModeBadge: "LIST"},
					{Key: "G", Description: "Jump to last stash", ModeBadge: "LIST"},
					{Key: "Tab", Description: "Toggle preview / switch focus", ModeBadge: "LIST/DETAIL"},
					{Key: "Enter", Description: "Enter detail view", ModeBadge: "LIST/PREVIEW"},
					{Key: "h / l", Description: "Cycle files in preview", ModeBadge: "PREVIEW"},
					{Key: "^d / ^u", Description: "Page scroll", ModeBadge: "PREVIEW/DETAIL"},
				},
			},
			{
				Name: "Actions",
				Bindings: []KeybindEntry{
					{Key: "a", Description: "Apply stash (with conflict preview)", ModeBadge: "LIST"},
					{Key: "p", Description: "Pop stash (apply + drop)", ModeBadge: "LIST"},
					{Key: "d", Description: "Drop stash (undo with z)", ModeBadge: "LIST"},
					{Key: "D", Description: "Drop ALL stashes (double-confirm)", ModeBadge: "LIST"},
					{Key: "n", Description: "New stash", ModeBadge: "LIST"},
					{Key: "P", Description: "Partial stash (pick hunks/lines)", ModeBadge: "LIST"},
					{Key: "r", Description: "Rename stash (inline)", ModeBadge: "LIST"},
					{Key: "m", Description: "Pin/unpin stash marker", ModeBadge: "LIST/PREVIEW"},
					{Key: "b", Description: "Branch from stash", ModeBadge: "LIST"},
					{Key: "z", Description: "Undo last drop", ModeBadge: "LIST"},
					{Key: "J", Description: "Move stash down", ModeBadge: "LIST"},
					{Key: "K", Description: "Move stash up", ModeBadge: "LIST"},
				},
			},
			{
				Name: "Search & Filter",
				Bindings: []KeybindEntry{
					{Key: "/", Description: "Open search", ModeBadge: "LIST"},
					{Key: "Tab", Description: "Cycle scope filters", ModeBadge: "SEARCH"},
					{Key: "Enter", Description: "Jump to result", ModeBadge: "SEARCH"},
					{Key: "f", Description: "Filter: current branch", ModeBadge: "LIST"},
					{Key: "F", Description: "Filter: stale stashes", ModeBadge: "LIST"},
				},
			},
			{
				Name: "Export & Import",
				Bindings: []KeybindEntry{
					{Key: "e", Description: "Export stashes to remote", ModeBadge: "LIST"},
					{Key: "i", Description: "Import stashes from remote", ModeBadge: "LIST"},
					{Key: "Space", Description: "Toggle selection", ModeBadge: "EXPORT"},
					{Key: "Tab", Description: "Next field", ModeBadge: "EXPORT/IMPORT"},
					{Key: "Enter", Description: "Confirm / execute", ModeBadge: "EXPORT/IMPORT"},
				},
			},
			{
				Name: "Partial Stash",
				Bindings: []KeybindEntry{
					{Key: "P", Description: "Open hunk/line picker", ModeBadge: "LIST"},
					{Key: "Space", Description: "Toggle file/hunk/line", ModeBadge: "PARTIAL"},
					{Key: "v", Description: "Hunk ↔ line granularity", ModeBadge: "PARTIAL"},
					{Key: "a / A", Description: "Toggle file / everything", ModeBadge: "PARTIAL"},
					{Key: "Enter", Description: "Name & create stash", ModeBadge: "PARTIAL"},
				},
			},
		},
	}
}

// Toggle flips the help overlay visibility.
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
	h.scrollY = 0
}

// Show makes the help overlay visible and resets scroll position.
func (h *HelpOverlay) Show() {
	h.visible = true
	h.scrollY = 0
}

// Hide makes the help overlay invisible.
func (h *HelpOverlay) Hide() {
	h.visible = false
}

// IsVisible returns whether the help overlay is currently shown.
func (h *HelpOverlay) IsVisible() bool {
	return h.visible
}

// ScrollDown scrolls the help content down by one line.
func (h *HelpOverlay) ScrollDown() {
	h.scrollY++
}

// ScrollUp scrolls the help content up by one line.
func (h *HelpOverlay) ScrollUp() {
	if h.scrollY > 0 {
		h.scrollY--
	}
}

// Categories returns the keybind categories (for testing).
func (h *HelpOverlay) Categories() []KeybindCategory {
	return h.categories
}

// ContentHeight returns the total height of the help content in lines.
func (h *HelpOverlay) ContentHeight() int {
	lines := 2 // Title + blank line
	for _, cat := range h.categories {
		lines += 2 // Category header + blank line after bindings
		lines += len(cat.Bindings)
		lines++ // Trailing blank line
	}
	return lines
}

// Render renders the help overlay content (without the canvas wrapper).
// The caller is responsible for compositing this on top of dimmed background
// using lipgloss.NewCanvas + lipgloss.NewLayer.
func (h *HelpOverlay) Render(width, height int) string {
	if !h.visible {
		return ""
	}

	overlayWidth := min(width-4, 70)
	overlayHeight := min(height-4, h.ContentHeight()+4)

	var b strings.Builder

	title := " Help — Keybind Reference "
	b.WriteString(centerText(title, overlayWidth))
	b.WriteString("\n\n")

	for _, cat := range h.categories {
		b.WriteString(" " + cat.Name + "\n")
		for _, bind := range cat.Bindings {
			keyCol := padRight(bind.Key, 14)
			badge := "[" + bind.ModeBadge + "]"
			descWidth := max(overlayWidth-14-len(badge)-6, 10)
			descCol := padRight(bind.Description, descWidth)
			b.WriteString("  " + keyCol + descCol + badge + "\n")
		}
		b.WriteString("\n")
	}

	content := b.String()

	lines := strings.Split(content, "\n")
	if h.scrollY >= len(lines) {
		h.scrollY = max(0, len(lines)-1)
	}
	visibleLines := lines[h.scrollY:]
	if len(visibleLines) > overlayHeight-2 {
		visibleLines = visibleLines[:overlayHeight-2]
	}
	scrolledContent := strings.Join(visibleLines, "\n")

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(h.theme.AccentGold()).
		Width(overlayWidth).
		Height(overlayHeight-2).
		Padding(0, 1)

	return borderStyle.Render(scrolledContent)
}

// RenderWithDimmedBackground composes the help overlay on top of dimmed content
// using LipGloss Canvas compositing.
func (h *HelpOverlay) RenderWithDimmedBackground(bgContent string, width, height int) string {
	if !h.visible {
		return bgContent
	}

	overlay := h.Render(width, height)

	dimStyle := lipgloss.NewStyle().
		Foreground(h.theme.FgDimmed())
	dimmedBg := dimStyle.Render(bgContent)

	bgLayer := lipgloss.NewLayer(dimmedBg)
	overlayW := lipgloss.Width(overlay)
	overlayH := lipgloss.Height(overlay)
	fgLayer := lipgloss.NewLayer(overlay).
		X(width/2 - overlayW/2).
		Y(height/2 - overlayH/2).
		Z(1)

	canvas := lipgloss.NewCanvas(bgLayer, fgLayer)
	return canvas.Render()
}

func centerText(text string, width int) string {
	if len(text) >= width {
		return text
	}
	pad := (width - len(text)) / 2
	return strings.Repeat(" ", pad) + text
}

func padRight(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}
	return text + strings.Repeat(" ", width-len(text))
}
