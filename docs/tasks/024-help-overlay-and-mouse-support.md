# Task 024: Help Overlay + Mouse Support

## Status: TODO

## Depends On
- 006 (core model — AppState, Mode enum, plugin interfaces)
- 007 (layout — Layout engine, status bar, footer bar, split pane)

## Parallelizable With
- 020 (search plugin)
- 021 (filter and stale plugins)
- 022 (reorder plugin)
- 023 (export/import plugin)
- 025 (config file and polish)

## Problem
nidhi has a rich keybinding system spanning multiple modes (LIST, PREVIEW, DETAIL, SEARCH, EXPORT, IMPORT) with three tiers of key complexity (single keys, double-key sequences, ctrl combos). New users need a discoverable way to see all keybindings without leaving the app. Additionally, while nidhi is keyboard-first, mouse support is expected by a segment of terminal users for casual interactions (clicking rows, scroll wheel, clicking chips). Both features are additive — neither is required for any workflow, but both significantly improve discoverability and comfort.

## PRD Reference
- Section 10, Screen 10 (HELP Modal Overlay) — triggered by `?`, centered modal via Canvas compositing, background dimmed, organized by category
- Section 11.1 (Three-Tier Hierarchy) — single key, double key/Shift, Ctrl combo
- Section 11.2 (Complete Keymap) — full keybind reference by mode
- Section 11.3 (Mouse Support) — click row, scroll wheel, click chip, click checkbox
- Section 13.2 (LipGloss v2 Features) — `lipgloss.NewCanvas` + `lipgloss.NewLayer` for modal overlays
- Section 13.3 (Canvas Compositing for Modals) — code example for dimmed background + foreground overlay
- Section 13.4 (Bubbles v2) — `key.Binding`, `help.Model`
- Section 8.2 (Plugin interfaces) — ScreenProvider for help overlay
- Section 8.4 (Module structure) — `internal/ui/screens/help.go`
- Section 7.4 (Accessibility) — keyboard-only operation, mouse is additive
- Section 13.1 (BubbleTea v2 Features) — `tea.View` struct with `MouseMode` field

## Files to Create
- `internal/ui/screens/help.go` — help overlay screen (ScreenProvider)
- `internal/ui/mouse.go` — mouse event handling
- `internal/ui/screens/help_test.go` — unit tests for help overlay
- `internal/ui/mouse_test.go` — unit tests for mouse handling

## Execution Steps

### Step 1: Create help overlay screen (`internal/ui/screens/help.go`)

```go
package screens

import (
	"strings"

	lipgloss "github.com/charmbracelet/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// HelpOverlay renders the full keybind reference as a modal overlay.
type HelpOverlay struct {
	visible     bool
	scrollY     int
	theme       theme.Theme
	categories  []KeybindCategory
}

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
					{Key: "r", Description: "Rename stash (inline)", ModeBadge: "LIST"},
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
					{Key: "fb", Description: "Filter: current branch", ModeBadge: "LIST"},
					{Key: "fs", Description: "Filter: stale stashes", ModeBadge: "LIST"},
					{Key: "fc", Description: "Clear all filters", ModeBadge: "LIST"},
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
		},
	}
}

// Toggle flips the help overlay visibility.
func (h *HelpOverlay) Toggle() {
	h.visible = !h.visible
	h.scrollY = 0
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
	lines := 0
	lines += 2 // Title + blank line
	for _, cat := range h.categories {
		lines += 2 // Category header + blank line
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

	// Calculate overlay dimensions.
	overlayWidth := min(width-4, 70) // Max 70 chars wide, 2 padding each side
	overlayHeight := min(height-4, h.ContentHeight()+4) // +4 for border/padding

	var b strings.Builder

	// Title.
	title := " Help — Keybind Reference "
	b.WriteString(centerText(title, overlayWidth))
	b.WriteString("\n\n")

	// Render categories.
	for _, cat := range h.categories {
		b.WriteString(" " + cat.Name + "\n")
		for _, bind := range cat.Bindings {
			// Format: "  key          description            [MODE]"
			keyCol := padRight(bind.Key, 14)
			descCol := padRight(bind.Description, overlayWidth-14-len(bind.ModeBadge)-6)
			badge := "[" + bind.ModeBadge + "]"
			b.WriteString("  " + keyCol + descCol + badge + "\n")
		}
		b.WriteString("\n")
	}

	content := b.String()

	// Apply scroll offset.
	lines := strings.Split(content, "\n")
	if h.scrollY >= len(lines) {
		h.scrollY = max(0, len(lines)-1)
	}
	visibleLines := lines[h.scrollY:]
	if len(visibleLines) > overlayHeight-2 { // -2 for top/bottom border
		visibleLines = visibleLines[:overlayHeight-2]
	}
	scrolledContent := strings.Join(visibleLines, "\n")

	// Apply border style.
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(h.theme.AccentGold())).
		Width(overlayWidth).
		Height(overlayHeight - 2). // Border adds 2 lines
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

	// Dim the background.
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(h.theme.FgDimmed()))
	dimmedBg := dimStyle.Render(bgContent)

	// Create canvas layers.
	bgLayer := lipgloss.NewLayer(dimmedBg)
	// Center the overlay.
	overlayWidth := lipgloss.Width(overlay)
	overlayHeight := lipgloss.Height(overlay)
	fgLayer := lipgloss.NewLayer(overlay).
		X(width/2 - overlayWidth/2).
		Y(height/2 - overlayHeight/2).
		Z(1)

	canvas := lipgloss.NewCanvas(bgLayer, fgLayer)
	return canvas.Render()
}

// --- Helpers ---

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

### Step 2: Create mouse support (`internal/ui/mouse.go`)

```go
package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
)

// MouseHandler processes mouse events and translates them to state changes.
// Mouse support is strictly additive — it enhances keyboard navigation
// but never provides functionality unavailable via keyboard.
type MouseHandler struct {
	// Layout information needed to map click coordinates to UI elements.
	statusBarHeight int // Typically 1 line.
	footerHeight    int // Typically 1 line.
	rowHeight       int // Height of each stash row (1 or 2 lines depending on responsive mode).
	listStartY      int // Y offset where the stash list starts (after status bar).
	listStartX      int // X offset (for indented rows).
	chipRegions     []ChipRegion // Clickable regions for filter/scope chips.
	checkboxRegions []CheckboxRegion // Clickable regions for export checkboxes.
}

// ChipRegion defines a clickable chip area.
type ChipRegion struct {
	X, Y, Width int
	ID          string // e.g. "scope:all", "scope:messages", "filter:branch"
}

// CheckboxRegion defines a clickable checkbox area.
type CheckboxRegion struct {
	X, Y   int
	Index  int // Stash index for export selection.
}

// NewMouseHandler creates a MouseHandler with default layout.
func NewMouseHandler() *MouseHandler {
	return &MouseHandler{
		statusBarHeight: 1,
		footerHeight:    1,
		rowHeight:       2, // Default: 2-line rows.
		listStartY:      1, // After status bar.
	}
}

// SetRowHeight updates the row height (1 for compact mode, 2 for default).
func (m *MouseHandler) SetRowHeight(h int) {
	m.rowHeight = h
}

// SetChipRegions updates the clickable chip regions.
func (m *MouseHandler) SetChipRegions(regions []ChipRegion) {
	m.chipRegions = regions
}

// SetCheckboxRegions updates the clickable checkbox regions.
func (m *MouseHandler) SetCheckboxRegions(regions []CheckboxRegion) {
	m.checkboxRegions = regions
}

// MouseAction represents the result of processing a mouse event.
type MouseAction int

const (
	MouseNoAction        MouseAction = iota
	MouseSelectRow                   // Click on stash row -> select it.
	MouseScrollUp                    // Scroll wheel up -> scroll list/viewport up.
	MouseScrollDown                  // Scroll wheel down -> scroll list/viewport down.
	MouseToggleChip                  // Click on scope/filter chip -> toggle it.
	MouseToggleCheckbox              // Click on checkbox -> toggle selection.
)

// MouseResult is the outcome of processing a mouse event.
type MouseResult struct {
	Action   MouseAction
	RowIndex int    // For MouseSelectRow: which row was clicked.
	ChipID   string // For MouseToggleChip: which chip was clicked.
	CBIndex  int    // For MouseToggleCheckbox: which checkbox was clicked.
}

// HandleMouse processes a BubbleTea v2 mouse event and returns the action.
// Uses tea.MouseClickMsg for clicks and tea.MouseWheelMsg for scrolling.
func (m *MouseHandler) HandleMouseClick(x, y int, state core.AppState) MouseResult {
	// Check if click is in the stash list area.
	if y >= m.listStartY && y < state.Height-m.footerHeight {
		// Calculate which row was clicked.
		relativeY := y - m.listStartY
		rowIndex := relativeY / m.rowHeight
		if rowIndex >= 0 && rowIndex < len(state.Stashes) {
			return MouseResult{Action: MouseSelectRow, RowIndex: rowIndex}
		}
	}

	// Check if click is on a chip.
	for _, chip := range m.chipRegions {
		if y == chip.Y && x >= chip.X && x < chip.X+chip.Width {
			return MouseResult{Action: MouseToggleChip, ChipID: chip.ID}
		}
	}

	// Check if click is on a checkbox.
	for _, cb := range m.checkboxRegions {
		if y == cb.Y && x >= cb.X && x < cb.X+3 { // Checkbox is ~3 chars wide: [✓]
			return MouseResult{Action: MouseToggleCheckbox, CBIndex: cb.Index}
		}
	}

	return MouseResult{Action: MouseNoAction}
}

// HandleMouseWheel processes scroll wheel events.
// direction: -1 for scroll up, +1 for scroll down.
func (m *MouseHandler) HandleMouseWheel(direction int) MouseResult {
	if direction < 0 {
		return MouseResult{Action: MouseScrollUp}
	}
	return MouseResult{Action: MouseScrollDown}
}
```

### Step 3: Write help overlay tests (`internal/ui/screens/help_test.go`)

```go
package screens_test

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/screens"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// TestHelpOverlayContainsAllCategories verifies that the help overlay
// includes all required keybind categories from PRD §11.2.
func TestHelpOverlayContainsAllCategories(t *testing.T) {
	th := theme.DefaultTheme() // Use default Agni theme.
	help := screens.NewHelpOverlay(th)

	categories := help.Categories()
	expectedCategories := []string{
		"Global",
		"Navigation",
		"Actions",
		"Search & Filter",
		"Export & Import",
	}

	if len(categories) != len(expectedCategories) {
		t.Fatalf("expected %d categories, got %d", len(expectedCategories), len(categories))
	}

	for i, expected := range expectedCategories {
		if categories[i].Name != expected {
			t.Errorf("category %d: expected %q, got %q", i, expected, categories[i].Name)
		}
	}
}

// TestHelpOverlayContainsKeyBindings verifies that critical keybindings
// are present in the rendered help overlay.
func TestHelpOverlayContainsKeyBindings(t *testing.T) {
	th := theme.DefaultTheme()
	help := screens.NewHelpOverlay(th)
	help.Toggle() // Make visible.

	rendered := help.Render(80, 40)

	requiredBindings := []string{
		"j / ↓",       // Navigation
		"a",            // Apply
		"p",            // Pop
		"d",            // Drop
		"n",            // New stash
		"r",            // Rename
		"/",            // Search
		"fb",           // Filter branch
		"fs",           // Filter stale
		"e",            // Export
		"i",            // Import
		"z",            // Undo
		"?",            // Help toggle
		"Esc",          // Back
		"J",            // Move down
		"K",            // Move up
	}

	for _, binding := range requiredBindings {
		if !strings.Contains(rendered, binding) {
			t.Errorf("help overlay missing keybinding %q", binding)
		}
	}
}

// TestHelpToggleOnOff verifies that ? toggles the help overlay visibility.
func TestHelpToggleOnOff(t *testing.T) {
	th := theme.DefaultTheme()
	help := screens.NewHelpOverlay(th)

	// Initially hidden.
	if help.IsVisible() {
		t.Error("expected help overlay to be hidden initially")
	}

	// First toggle: show.
	help.Toggle()
	if !help.IsVisible() {
		t.Error("expected help overlay to be visible after first toggle")
	}

	// Second toggle: hide.
	help.Toggle()
	if help.IsVisible() {
		t.Error("expected help overlay to be hidden after second toggle")
	}
}

// TestHelpOverlayRendersEmpty verifies that Render returns empty when not visible.
func TestHelpOverlayRendersEmpty(t *testing.T) {
	th := theme.DefaultTheme()
	help := screens.NewHelpOverlay(th)

	rendered := help.Render(80, 40)
	if rendered != "" {
		t.Errorf("expected empty string when help is not visible, got %q", rendered)
	}
}

// TestHelpOverlayAdaptsToTerminalSize verifies that the help overlay
// dimensions adapt to different terminal sizes.
func TestHelpOverlayAdaptsToTerminalSize(t *testing.T) {
	th := theme.DefaultTheme()
	help := screens.NewHelpOverlay(th)
	help.Toggle()

	tests := []struct {
		name        string
		width       int
		height      int
		maxOverlayW int // Expected max overlay width
	}{
		{"standard 80x24", 80, 24, 70},
		{"wide 200x50", 200, 50, 70},
		{"narrow 60x24", 60, 24, 56}, // 60-4=56
		{"tall 80x60", 80, 60, 70},
		{"minimum 40x10", 40, 10, 36}, // 40-4=36
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := help.Render(tt.width, tt.height)
			if rendered == "" {
				t.Fatal("expected non-empty render")
			}

			// Verify no line exceeds the terminal width.
			lines := strings.Split(rendered, "\n")
			for i, line := range lines {
				// Note: lipgloss border characters may expand. We check for reasonableness.
				if len(line) > tt.width+10 { // Allow some slack for ANSI escape codes.
					t.Errorf("line %d exceeds width %d: len=%d", i, tt.width, len(line))
				}
			}

			// Verify the rendered height doesn't exceed terminal height.
			if len(lines) > tt.height {
				t.Errorf("rendered %d lines but terminal height is %d", len(lines), tt.height)
			}
		})
	}
}

// TestHelpContentHeight verifies ContentHeight calculation.
func TestHelpContentHeight(t *testing.T) {
	th := theme.DefaultTheme()
	help := screens.NewHelpOverlay(th)

	height := help.ContentHeight()
	if height <= 0 {
		t.Errorf("expected positive content height, got %d", height)
	}

	// Verify it accounts for all categories and bindings.
	totalBindings := 0
	for _, cat := range help.Categories() {
		totalBindings += len(cat.Bindings)
	}
	// Minimum: title(1) + blank(1) + categories*3 + bindings
	minHeight := 2 + len(help.Categories())*3 + totalBindings
	if height < minHeight {
		t.Errorf("content height %d is less than expected minimum %d", height, minHeight)
	}
}
```

### Step 4: Write mouse handler tests (`internal/ui/mouse_test.go`)

```go
package ui_test

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/ui"
)

// TestMouseClickOnRowSelectsCorrectStash verifies that clicking on a
// stash row at a given Y coordinate selects the correct stash index.
func TestMouseClickOnRowSelectsCorrectStash(t *testing.T) {
	mh := ui.NewMouseHandler()
	mh.SetRowHeight(2) // 2-line rows.

	state := core.AppState{
		Height: 24,
		Stashes: []core.Stash{
			{Index: 0}, {Index: 1}, {Index: 2}, {Index: 3}, {Index: 4},
		},
	}

	tests := []struct {
		y        int
		expected int // Expected row index, -1 if no action.
	}{
		{1, 0},  // Row 0: y=1 (status bar at y=0)
		{2, 0},  // Row 0: second line of row 0
		{3, 1},  // Row 1: y=3
		{4, 1},  // Row 1: second line
		{5, 2},  // Row 2
		{7, 3},  // Row 3
		{9, 4},  // Row 4
		{11, -1}, // Beyond stash list (only 5 stashes * 2 rows = y 1-10)
	}

	for _, tt := range tests {
		result := mh.HandleMouseClick(5, tt.y, state)
		if tt.expected == -1 {
			if result.Action != ui.MouseNoAction {
				t.Errorf("y=%d: expected no action, got action=%d", tt.y, result.Action)
			}
		} else {
			if result.Action != ui.MouseSelectRow {
				t.Errorf("y=%d: expected MouseSelectRow, got %d", tt.y, result.Action)
			}
			if result.RowIndex != tt.expected {
				t.Errorf("y=%d: expected row %d, got %d", tt.y, tt.expected, result.RowIndex)
			}
		}
	}
}

// TestMouseClickOnRowCompactMode verifies row selection with 1-line rows.
func TestMouseClickOnRowCompactMode(t *testing.T) {
	mh := ui.NewMouseHandler()
	mh.SetRowHeight(1) // Compact: 1-line rows.

	state := core.AppState{
		Height: 24,
		Stashes: []core.Stash{
			{Index: 0}, {Index: 1}, {Index: 2},
		},
	}

	tests := []struct {
		y        int
		expected int
	}{
		{1, 0},
		{2, 1},
		{3, 2},
		{4, -1}, // Beyond stash list.
	}

	for _, tt := range tests {
		result := mh.HandleMouseClick(5, tt.y, state)
		if tt.expected == -1 {
			if result.Action != ui.MouseNoAction {
				t.Errorf("y=%d: expected no action, got %d", tt.y, result.Action)
			}
		} else {
			if result.RowIndex != tt.expected {
				t.Errorf("y=%d: expected row %d, got %d", tt.y, tt.expected, result.RowIndex)
			}
		}
	}
}

// TestScrollWheelUp verifies scroll wheel up returns MouseScrollUp.
func TestScrollWheelUp(t *testing.T) {
	mh := ui.NewMouseHandler()
	result := mh.HandleMouseWheel(-1)
	if result.Action != ui.MouseScrollUp {
		t.Errorf("expected MouseScrollUp, got %d", result.Action)
	}
}

// TestScrollWheelDown verifies scroll wheel down returns MouseScrollDown.
func TestScrollWheelDown(t *testing.T) {
	mh := ui.NewMouseHandler()
	result := mh.HandleMouseWheel(1)
	if result.Action != ui.MouseScrollDown {
		t.Errorf("expected MouseScrollDown, got %d", result.Action)
	}
}

// TestMouseClickOnChip verifies that clicking on a chip region
// returns the correct chip ID.
func TestMouseClickOnChip(t *testing.T) {
	mh := ui.NewMouseHandler()
	mh.SetChipRegions([]ui.ChipRegion{
		{X: 10, Y: 3, Width: 5, ID: "scope:all"},
		{X: 16, Y: 3, Width: 10, ID: "scope:messages"},
		{X: 27, Y: 3, Width: 7, ID: "scope:files"},
	})

	state := core.AppState{Height: 24}

	// Click on "scope:all" chip.
	result := mh.HandleMouseClick(12, 3, state)
	if result.Action != ui.MouseToggleChip {
		t.Errorf("expected MouseToggleChip, got %d", result.Action)
	}
	if result.ChipID != "scope:all" {
		t.Errorf("expected chip 'scope:all', got %q", result.ChipID)
	}

	// Click on "scope:messages" chip.
	result = mh.HandleMouseClick(20, 3, state)
	if result.ChipID != "scope:messages" {
		t.Errorf("expected chip 'scope:messages', got %q", result.ChipID)
	}

	// Click outside any chip.
	result = mh.HandleMouseClick(40, 3, state)
	if result.Action != ui.MouseNoAction {
		t.Errorf("expected no action for click outside chips, got %d", result.Action)
	}
}

// TestMouseClickOnCheckbox verifies that clicking on a checkbox
// returns the correct checkbox index.
func TestMouseClickOnCheckbox(t *testing.T) {
	mh := ui.NewMouseHandler()
	mh.SetCheckboxRegions([]ui.CheckboxRegion{
		{X: 4, Y: 5, Index: 0},
		{X: 4, Y: 6, Index: 1},
		{X: 4, Y: 7, Index: 2},
	})

	state := core.AppState{Height: 24}

	result := mh.HandleMouseClick(5, 6, state)
	if result.Action != ui.MouseToggleCheckbox {
		t.Errorf("expected MouseToggleCheckbox, got %d", result.Action)
	}
	if result.CBIndex != 1 {
		t.Errorf("expected checkbox index 1, got %d", result.CBIndex)
	}
}

// TestMouseNoActionOnStatusBar verifies that clicks on the status bar
// are ignored.
func TestMouseNoActionOnStatusBar(t *testing.T) {
	mh := ui.NewMouseHandler()
	state := core.AppState{
		Height:  24,
		Stashes: []core.Stash{{Index: 0}},
	}

	result := mh.HandleMouseClick(10, 0, state)
	if result.Action != ui.MouseNoAction {
		t.Errorf("expected no action for status bar click, got %d", result.Action)
	}
}
```

### Step 5: Verify

```bash
# Help overlay tests.
go test -v -count=1 ./internal/ui/screens/...

# Mouse handler tests.
go test -v -count=1 ./internal/ui/...

# Full CI pipeline.
make ci
```

## Verification

### Functional
```bash
# Help overlay unit tests pass
go test -v -count=1 -run 'TestHelpOverlay|TestHelpToggle|TestHelpContent' ./internal/ui/screens/...

# Mouse handler unit tests pass
go test -v -count=1 -run 'TestMouseClick|TestScrollWheel|TestMouseNoAction' ./internal/ui/...

# Compiles and passes vet
go vet ./internal/ui/screens/... ./internal/ui/...

# Lint clean
golangci-lint run ./internal/ui/screens/... ./internal/ui/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/ui/screens/help.go` renders a modal overlay with all keybind categories from PRD §11.2
2. Help overlay uses LipGloss Canvas compositing (`lipgloss.NewCanvas` + `lipgloss.NewLayer`) for modal rendering
3. Background content is dimmed when help is visible
4. `?` toggles help on/off
5. `Esc` or `?` dismisses the help overlay
6. Help overlay organized by category: Global, Navigation, Actions, Search & Filter, Export & Import
7. Mode badges shown for each keybinding (LIST, ALL, PREVIEW, etc.)
8. Help overlay dimensions adapt to terminal size (never exceeds available space)
9. `internal/ui/mouse.go` handles click on stash row -> correct row selected
10. Scroll wheel up/down increments/decrements cursor
11. Click on scope/filter chip -> toggles the chip
12. Click on checkbox -> toggles selection
13. Mouse events on status bar/footer are no-ops
14. All unit tests pass: categories, keybindings, toggle, terminal size adaptation, mouse clicks, scroll, chips, checkboxes
15. `make ci` passes (lint + test)

## Commit
```
feat(help,mouse): add help overlay with canvas compositing and mouse support

Implement help overlay (ScreenProvider) triggered by ? with all keybind
categories organized by function. Uses LipGloss Canvas + Layer for modal
compositing with dimmed background. Adapts to terminal size. Add mouse
support: click on row selects stash, scroll wheel navigates, click on
chip toggles filter, click on checkbox toggles selection. Mouse is
strictly additive — no feature requires it.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 10 (Screen 10), 11.1-11.3 (keymap + mouse), 13.2-13.3 (Canvas compositing), 13.4 (Bubbles), 7.4 (accessibility)
4. Verify dependencies: task 006 (core model) and task 007 (layout engine) are DONE
5. Create `internal/ui/screens/help.go` with HelpOverlay, all categories, Canvas compositing
6. Create `internal/ui/mouse.go` with MouseHandler for clicks and scroll
7. Create `internal/ui/screens/help_test.go` with all help overlay tests
8. Create `internal/ui/mouse_test.go` with all mouse handler tests
9. Run `go test -v -count=1 ./internal/ui/screens/... ./internal/ui/...`
10. Run `make ci`
11. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
12. Commit with the message above
