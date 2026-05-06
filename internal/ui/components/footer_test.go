package components

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func TestHintsForMode_ListMode(t *testing.T) {
	hints := HintsForMode(plugin.ModeList)
	if len(hints) == 0 {
		t.Fatal("LIST mode should have keybind hints")
	}

	descs := make(map[string]bool)
	for _, h := range hints {
		descs[h.Desc] = true
	}

	// LIST footer: nav, detail, preview, new, apply, pop, drop, rename, pin, export, search, help.
	required := []string{"nav", "detail", "preview", "new", "apply", "pop", "drop", "pin", "search", "help"}
	for _, d := range required {
		if !descs[d] {
			t.Errorf("LIST mode missing hint desc %q", d)
		}
	}
}

func TestHintsForMode_PreviewMode(t *testing.T) {
	hints := HintsForMode(plugin.ModePreview)
	descs := make(map[string]bool)
	for _, h := range hints {
		descs[h.Desc] = true
	}

	// Mockup PREVIEW footer: stashes, files, close, detail, apply, pop.
	if !descs["files"] {
		t.Error("PREVIEW mode should have 'files' hint")
	}
	if !descs["close"] {
		t.Error("PREVIEW mode should have 'close' hint for Tab")
	}
	if !descs["pin"] {
		t.Error("PREVIEW mode should have 'pin' hint")
	}
}

func TestHintsForMode_DetailMode(t *testing.T) {
	hints := HintsForMode(plugin.ModeDetail)
	descs := make(map[string]bool)
	for _, h := range hints {
		descs[h.Desc] = true
	}

	// Mockup DETAIL footer: files, tree↔diff, scroll, apply, pop, branch, rename, back.
	if !descs["back"] {
		t.Error("DETAIL mode should have 'back' hint for esc")
	}
	if !descs["branch"] {
		t.Error("DETAIL mode should have 'branch' hint")
	}
}

func TestHintsForMode_AllModesHaveHints(t *testing.T) {
	modes := []plugin.Mode{
		plugin.ModeList, plugin.ModePreview, plugin.ModeDetail,
		plugin.ModeSearch, plugin.ModeNewStash, plugin.ModeExport,
		plugin.ModeImport, plugin.ModeConflict, plugin.ModeHelp,
	}

	for _, mode := range modes {
		hints := HintsForMode(mode)
		if len(hints) == 0 {
			t.Errorf("mode %s has no keybind hints", mode)
		}
	}
}

func TestFooter_ContainsModeBadge(t *testing.T) {
	f := NewFooter(theme.NewAgni())
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModeList,
		Width: 120,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "LIST") {
		t.Errorf("footer should contain mode badge 'LIST', got: %q", plain)
	}
}

func TestFooter_ContainsKeyHints(t *testing.T) {
	f := NewFooter(theme.NewAgni())
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModeList,
		Width: 120,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "apply") {
		t.Errorf("footer should contain 'apply' hint, got: %q", plain)
	}
}

func TestFooter_PreviewModeBadge(t *testing.T) {
	f := NewFooter(theme.NewAgni())
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModePreview,
		Width: 120,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "PREVIEW") {
		t.Errorf("footer should contain 'PREVIEW' badge, got: %q", plain)
	}
}

func TestFooter_DetailModeBadge(t *testing.T) {
	f := NewFooter(theme.NewAgni())
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModeDetail,
		Width: 120,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "DETAIL") {
		t.Errorf("footer should contain 'DETAIL' badge, got: %q", plain)
	}
}

func TestBadgeColorForMode_ReturnsNonNilColors(t *testing.T) {
	th := theme.NewAgni()
	modes := []plugin.Mode{
		plugin.ModeList, plugin.ModePreview, plugin.ModeDetail,
		plugin.ModeSearch, plugin.ModeNewStash, plugin.ModeExport,
		plugin.ModeImport, plugin.ModeConflict, plugin.ModeHelp,
	}

	for _, mode := range modes {
		c := BadgeColorForMode(mode, th)
		if c == nil {
			t.Errorf("BadgeColorForMode(%s) returned nil", mode)
		}
	}
}
