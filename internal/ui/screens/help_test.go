package screens_test

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/screens"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func TestHelpOverlayContainsAllCategories(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())

	categories := help.Categories()
	expected := []string{
		"Global",
		"Navigation",
		"Actions",
		"Search & Filter",
		"Export & Import",
	}

	if len(categories) != len(expected) {
		t.Fatalf("expected %d categories, got %d", len(expected), len(categories))
	}
	for i, name := range expected {
		if categories[i].Name != name {
			t.Errorf("category %d: expected %q, got %q", i, name, categories[i].Name)
		}
	}
}

func TestHelpOverlayContainsKeyBindings(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())
	help.Toggle()

	rendered := help.Render(80, 60)

	required := []string{
		"j / ↓", "a", "p", "d", "n", "r", "/",
		"f", "F", "e", "i", "z", "?", "Esc", "J", "K",
	}
	for _, key := range required {
		if !strings.Contains(rendered, key) {
			t.Errorf("help overlay missing keybinding %q", key)
		}
	}
}

func TestHelpToggleOnOff(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())

	if help.IsVisible() {
		t.Error("expected hidden initially")
	}
	help.Toggle()
	if !help.IsVisible() {
		t.Error("expected visible after toggle")
	}
	help.Toggle()
	if help.IsVisible() {
		t.Error("expected hidden after second toggle")
	}
}

func TestHelpRendersEmptyWhenHidden(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())
	if help.Render(80, 40) != "" {
		t.Error("expected empty string when not visible")
	}
}

func TestHelpOverlayAdaptsToTerminalSize(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())
	help.Toggle()

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"standard 80x24", 80, 24},
		{"wide 200x50", 200, 50},
		{"narrow 60x24", 60, 24},
		{"tall 80x60", 80, 60},
		{"minimum 40x10", 40, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := help.Render(tt.width, tt.height)
			if rendered == "" {
				t.Fatal("expected non-empty render")
			}
			lines := strings.Split(rendered, "\n")
			if len(lines) > tt.height {
				t.Errorf("rendered %d lines but terminal height is %d", len(lines), tt.height)
			}
		})
	}
}

func TestHelpContentHeight(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())
	height := help.ContentHeight()
	if height <= 0 {
		t.Errorf("expected positive content height, got %d", height)
	}

	totalBindings := 0
	for _, cat := range help.Categories() {
		totalBindings += len(cat.Bindings)
	}
	minHeight := 2 + len(help.Categories())*3 + totalBindings
	if height < minHeight {
		t.Errorf("content height %d < expected minimum %d", height, minHeight)
	}
}

func TestHelpScrollUpDown(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())
	help.Toggle()

	help.ScrollDown()
	help.ScrollDown()
	help.ScrollDown()
	help.ScrollUp()

	// Verify no crash and renders.
	rendered := help.Render(80, 60)
	if rendered == "" {
		t.Error("expected non-empty render after scroll")
	}
}

func TestHelpScrollUpAtZeroNoop(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())
	help.Toggle()

	help.ScrollUp()
	help.ScrollUp()

	rendered := help.Render(80, 60)
	if rendered == "" {
		t.Error("expected non-empty render")
	}
}

func TestHelpRenderWithDimmedBackground(t *testing.T) {
	help := screens.NewHelpOverlay(theme.NewAgni())
	bg := "some background content\nline 2\nline 3"

	// Not visible: returns bg unchanged.
	result := help.RenderWithDimmedBackground(bg, 80, 24)
	if result != bg {
		t.Error("expected original bg when help is not visible")
	}

	// Visible: returns composited content.
	help.Toggle()
	result = help.RenderWithDimmedBackground(bg, 80, 40)
	if result == bg {
		t.Error("expected different content when help is visible")
	}
	if result == "" {
		t.Error("expected non-empty composited result")
	}
}
