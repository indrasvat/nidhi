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

func TestFooter_NarrowWidthPreservesBadgeAndHelp(t *testing.T) {
	// At narrow widths the LIST footer (12 hints) overflows. The mode badge
	// and the "?" help hint must still be visible so users know where they
	// are and how to reach help. Mid-priority hints (rename, export, …) may
	// be elided.
	f := NewFooter(theme.NewAgni())
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModeList,
		Width: 80,
	})
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "LIST") {
		t.Errorf("narrow LIST footer should keep mode badge, got: %q", plain)
	}
	if !strings.Contains(plain, "help") {
		t.Errorf("narrow LIST footer should keep '?' help hint, got: %q", plain)
	}
	if !strings.Contains(plain, "j/k") {
		t.Errorf("narrow LIST footer should keep first hint 'j/k', got: %q", plain)
	}
}

func TestFooter_VeryNarrowWidthKeepsBadge(t *testing.T) {
	// Below the size where any hint fits, only the mode badge needs to survive.
	f := NewFooter(theme.NewAgni())
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModePreview,
		Width: 20,
	})
	plain := stripAnsi(rendered)

	if !strings.Contains(plain, "PREVIEW") {
		t.Errorf("very narrow PREVIEW footer should keep mode badge, got: %q", plain)
	}
}

func TestFooter_FullWidthShowsAllHints(t *testing.T) {
	// At wide terminals all hints should be present (no elision).
	f := NewFooter(theme.NewAgni())
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModeList,
		Width: 200,
	})
	plain := stripAnsi(rendered)

	for _, want := range []string{"nav", "detail", "preview", "rename", "export", "search", "help", "LIST"} {
		if !strings.Contains(plain, want) {
			t.Errorf("wide LIST footer missing %q, got: %q", want, plain)
		}
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
