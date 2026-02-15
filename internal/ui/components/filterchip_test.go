package components

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func TestChipGroupModel_Initial(t *testing.T) {
	m := SearchScopeChips(theme.NewAgni())

	if len(m.Chips) != 5 {
		t.Fatalf("expected 5 chips, got %d", len(m.Chips))
	}

	if !m.Chips[0].Active {
		t.Error("first chip should be active initially")
	}

	for i := 1; i < len(m.Chips); i++ {
		if m.Chips[i].Active {
			t.Errorf("chip %d (%q) should be inactive initially", i, m.Chips[i].Label)
		}
	}

	if m.Cursor() != 0 {
		t.Errorf("initial cursor = %d, want 0", m.Cursor())
	}
}

func TestChipGroupModel_Navigation(t *testing.T) {
	m := SearchScopeChips(theme.NewAgni())

	m.Next()
	if m.Cursor() != 1 {
		t.Errorf("after Next: cursor = %d, want 1", m.Cursor())
	}

	m.Next()
	if m.Cursor() != 2 {
		t.Errorf("after Next x2: cursor = %d, want 2", m.Cursor())
	}

	m.Prev()
	if m.Cursor() != 1 {
		t.Errorf("after Prev: cursor = %d, want 1", m.Cursor())
	}
}

func TestChipGroupModel_WrapAround(t *testing.T) {
	m := NewChipGroupModel([]string{"A", "B", "C"}, theme.NewAgni())

	m.Next() // 1
	m.Next() // 2
	m.Next() // 0 (wrapped)
	if m.Cursor() != 0 {
		t.Errorf("wrap forward: cursor = %d, want 0", m.Cursor())
	}

	m.Prev() // 2 (wrapped)
	if m.Cursor() != 2 {
		t.Errorf("wrap backward: cursor = %d, want 2", m.Cursor())
	}
}

func TestChipGroupModel_Toggle(t *testing.T) {
	m := SearchScopeChips(theme.NewAgni())

	m.Next() // cursor at 1 (Messages)
	m.Toggle()

	if !m.Chips[1].Active {
		t.Error("Messages should be active after toggle")
	}
	if m.Chips[0].Active {
		t.Error("All should be deactivated when a specific chip is activated")
	}
}

func TestChipGroupModel_ToggleAllDeactivatesOthers(t *testing.T) {
	m := SearchScopeChips(theme.NewAgni())

	// Activate Messages and Files.
	m.Next()
	m.Toggle()
	m.Next()
	m.Toggle()

	// Go back to "All" and activate it.
	m.SetCursor(0)
	m.Toggle()

	if !m.Chips[0].Active {
		t.Error("All should be active")
	}
	for i := 1; i < len(m.Chips); i++ {
		if m.Chips[i].Active {
			t.Errorf("chip %d should be inactive when All is active", i)
		}
	}
}

func TestChipGroupModel_NoChipsActiveResetsToAll(t *testing.T) {
	m := SearchScopeChips(theme.NewAgni())

	m.Next()   // cursor at 1
	m.Toggle() // activate Messages, deactivate All
	m.Toggle() // deactivate Messages

	if !m.Chips[0].Active {
		t.Error("All should auto-activate when no chips are active")
	}
}

func TestChipGroupModel_ActiveLabels(t *testing.T) {
	m := SearchScopeChips(theme.NewAgni())

	labels := m.ActiveLabels()
	if len(labels) != 1 || labels[0] != "All" {
		t.Errorf("initial ActiveLabels() = %v, want [All]", labels)
	}

	m.SetCursor(1)
	m.Toggle()
	m.SetCursor(2)
	m.Toggle()

	labels = m.ActiveLabels()
	if len(labels) != 2 {
		t.Errorf("ActiveLabels() = %v, want 2 items", labels)
	}
}

func TestChipGroupModel_ViewRendersAllChips(t *testing.T) {
	m := NewChipGroupModel([]string{"Alpha", "Beta", "Gamma"}, theme.NewAgni())
	view := m.View()
	plain := stripAnsi(view)

	for _, label := range []string{"Alpha", "Beta", "Gamma"} {
		if !strings.Contains(plain, label) {
			t.Errorf("view should contain chip label %q", label)
		}
	}
}

func TestChipGroupModel_EmptyGroup(t *testing.T) {
	m := NewChipGroupModel(nil, theme.NewAgni())

	// Should not panic.
	m.Next()
	m.Prev()
	m.Toggle()

	view := m.View()
	if view != "" {
		t.Errorf("empty chip group view should be empty, got: %q", view)
	}
}
