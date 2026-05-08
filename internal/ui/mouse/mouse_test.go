package mouse_test

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/mouse"
)

func stateWithNStashes(n, height int) plugin.AppState {
	stashes := make([]plugin.Stash, n)
	for i := range n {
		stashes[i] = plugin.Stash{Index: i}
	}
	return plugin.AppState{
		Height:  height,
		Stashes: stashes,
	}
}

func TestClickOnRowSelectsCorrectStash(t *testing.T) {
	h := mouse.NewHandler()
	h.SetRowHeight(2)

	state := stateWithNStashes(5, 24)

	tests := []struct {
		y        int
		expected int // -1 = no action
	}{
		{1, 0},
		{2, 0},
		{3, 1},
		{4, 1},
		{5, 2},
		{7, 3},
		{9, 4},
		{11, -1}, // Beyond 5 stashes * 2 rows
	}

	for _, tt := range tests {
		result := h.HandleClick(5, tt.y, state)
		if tt.expected == -1 {
			if result.Action != mouse.NoAction {
				t.Errorf("y=%d: expected NoAction, got %d", tt.y, result.Action)
			}
		} else {
			if result.Action != mouse.SelectRow {
				t.Errorf("y=%d: expected SelectRow, got %d", tt.y, result.Action)
			}
			if result.RowIndex != tt.expected {
				t.Errorf("y=%d: expected row %d, got %d", tt.y, tt.expected, result.RowIndex)
			}
		}
	}
}

func TestClickOnRowCompactMode(t *testing.T) {
	h := mouse.NewHandler()
	h.SetRowHeight(1)

	state := stateWithNStashes(3, 24)

	tests := []struct {
		y        int
		expected int
	}{
		{1, 0},
		{2, 1},
		{3, 2},
		{4, -1},
	}

	for _, tt := range tests {
		result := h.HandleClick(5, tt.y, state)
		if tt.expected == -1 {
			if result.Action != mouse.NoAction {
				t.Errorf("y=%d: expected NoAction, got %d", tt.y, result.Action)
			}
		} else {
			if result.RowIndex != tt.expected {
				t.Errorf("y=%d: expected row %d, got %d", tt.y, tt.expected, result.RowIndex)
			}
		}
	}
}

func TestScrollWheelUp(t *testing.T) {
	h := mouse.NewHandler()
	result := h.HandleWheel(-1)
	if result.Action != mouse.ScrollUp {
		t.Errorf("expected ScrollUp, got %d", result.Action)
	}
}

func TestScrollWheelDown(t *testing.T) {
	h := mouse.NewHandler()
	result := h.HandleWheel(1)
	if result.Action != mouse.ScrollDown {
		t.Errorf("expected ScrollDown, got %d", result.Action)
	}
}

func TestClickOnChip(t *testing.T) {
	h := mouse.NewHandler()
	h.SetChipRegions([]mouse.ChipRegion{
		{X: 10, Y: 3, Width: 5, ID: "scope:all"},
		{X: 16, Y: 3, Width: 10, ID: "scope:messages"},
		{X: 27, Y: 3, Width: 7, ID: "scope:files"},
	})

	state := stateWithNStashes(0, 24)

	result := h.HandleClick(12, 3, state)
	if result.Action != mouse.ToggleChip {
		t.Errorf("expected ToggleChip, got %d", result.Action)
	}
	if result.ChipID != "scope:all" {
		t.Errorf("expected chip 'scope:all', got %q", result.ChipID)
	}

	result = h.HandleClick(20, 3, state)
	if result.ChipID != "scope:messages" {
		t.Errorf("expected chip 'scope:messages', got %q", result.ChipID)
	}

	result = h.HandleClick(40, 3, state)
	if result.Action != mouse.NoAction {
		t.Errorf("expected NoAction for miss, got %d", result.Action)
	}
}

func TestClickOnCheckbox(t *testing.T) {
	h := mouse.NewHandler()
	h.SetCheckboxRegions([]mouse.CheckboxRegion{
		{X: 4, Y: 5, Index: 0},
		{X: 4, Y: 6, Index: 1},
		{X: 4, Y: 7, Index: 2},
	})

	state := stateWithNStashes(0, 24)

	result := h.HandleClick(5, 6, state)
	if result.Action != mouse.ToggleCheckbox {
		t.Errorf("expected ToggleCheckbox, got %d", result.Action)
	}
	if result.CBIndex != 1 {
		t.Errorf("expected checkbox index 1, got %d", result.CBIndex)
	}
}

func TestClickOnStatusBarIgnored(t *testing.T) {
	h := mouse.NewHandler()
	state := stateWithNStashes(3, 24)

	result := h.HandleClick(10, 0, state)
	if result.Action != mouse.NoAction {
		t.Errorf("expected NoAction for status bar, got %d", result.Action)
	}
}

func TestClickOnFooterIgnored(t *testing.T) {
	h := mouse.NewHandler()
	state := stateWithNStashes(3, 24)

	// Footer is at y=23 (height-1).
	result := h.HandleClick(10, 23, state)
	if result.Action != mouse.NoAction {
		t.Errorf("expected NoAction for footer, got %d", result.Action)
	}
}
