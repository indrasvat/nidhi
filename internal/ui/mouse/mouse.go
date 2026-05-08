package mouse

import "github.com/indrasvat/nidhi/internal/plugin"

// Handler processes mouse events and translates them to state changes.
// Mouse support is strictly additive — it enhances keyboard navigation
// but never provides functionality unavailable via keyboard.
type Handler struct {
	statusBarHeight int
	footerHeight    int
	rowHeight       int
	listStartY      int
	chipRegions     []ChipRegion
	checkboxRegions []CheckboxRegion
}

// ChipRegion defines a clickable chip area.
type ChipRegion struct {
	X, Y, Width int
	ID          string
}

// CheckboxRegion defines a clickable checkbox area.
type CheckboxRegion struct {
	X, Y  int
	Index int
}

// Action represents the result of processing a mouse event.
type Action int

const (
	NoAction       Action = iota
	SelectRow             // Click on stash row.
	ScrollUp              // Scroll wheel up.
	ScrollDown            // Scroll wheel down.
	ToggleChip            // Click on scope/filter chip.
	ToggleCheckbox        // Click on checkbox.
)

// Result is the outcome of processing a mouse event.
type Result struct {
	Action   Action
	RowIndex int
	ChipID   string
	CBIndex  int
}

// NewHandler creates a Handler with default layout.
func NewHandler() *Handler {
	return &Handler{
		statusBarHeight: 1,
		footerHeight:    1,
		rowHeight:       2,
		listStartY:      1,
	}
}

// SetRowHeight updates the row height (1 for compact mode, 2 for default).
func (h *Handler) SetRowHeight(rh int) {
	h.rowHeight = rh
}

// SetChipRegions updates the clickable chip regions.
func (h *Handler) SetChipRegions(regions []ChipRegion) {
	h.chipRegions = regions
}

// SetCheckboxRegions updates the clickable checkbox regions.
func (h *Handler) SetCheckboxRegions(regions []CheckboxRegion) {
	h.checkboxRegions = regions
}

// HandleClick processes a mouse click at (x, y) and returns the action.
func (h *Handler) HandleClick(x, y int, state plugin.AppState) Result {
	// Check if click is in the stash list area.
	if y >= h.listStartY && y < state.Height-h.footerHeight {
		relativeY := y - h.listStartY
		rowIndex := relativeY / h.rowHeight
		if rowIndex >= 0 && rowIndex < len(state.Stashes) {
			return Result{Action: SelectRow, RowIndex: rowIndex}
		}
	}

	// Check if click is on a chip.
	for _, chip := range h.chipRegions {
		if y == chip.Y && x >= chip.X && x < chip.X+chip.Width {
			return Result{Action: ToggleChip, ChipID: chip.ID}
		}
	}

	// Check if click is on a checkbox.
	for _, cb := range h.checkboxRegions {
		if y == cb.Y && x >= cb.X && x < cb.X+3 {
			return Result{Action: ToggleCheckbox, CBIndex: cb.Index}
		}
	}

	return Result{Action: NoAction}
}

// HandleWheel processes scroll wheel events.
// direction: -1 for scroll up, +1 for scroll down.
func (h *Handler) HandleWheel(direction int) Result {
	if direction < 0 {
		return Result{Action: ScrollUp}
	}
	return Result{Action: ScrollDown}
}
