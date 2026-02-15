package theme_test

import (
	"image/color"
	"testing"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func TestAgni_ImplementsTheme(t *testing.T) {
	var th theme.Theme = theme.NewAgni()
	if th.Name() == "" {
		t.Fatal("NewAgni() returned theme with empty name")
	}
}

func TestAgni_Name(t *testing.T) {
	th := theme.NewAgni()
	if th.Name() != "agni" {
		t.Errorf("Name() = %q, want %q", th.Name(), "agni")
	}
}

func TestAgni_AllColorTokensPresent(t *testing.T) {
	th := theme.NewAgni()

	tokens := []struct {
		name string
		got  color.Color
		hex  string
	}{
		{"BgDeep", th.BgDeep(), "#07090E"},
		{"BgSurface", th.BgSurface(), "#0F1219"},
		{"BgElevated", th.BgElevated(), "#1A1F2B"},
		{"BgOverlay", th.BgOverlay(), "#1F2738"},
		{"FgPrimary", th.FgPrimary(), "#C8CCD4"},
		{"FgSecondary", th.FgSecondary(), "#6B7280"},
		{"FgDimmed", th.FgDimmed(), "#3D4450"},
		{"AccentGold", th.AccentGold(), "#D4A050"},
		{"AccentBright", th.AccentBright(), "#E8B85A"},
		{"SemanticAqua", th.SemanticAqua(), "#4EC9B0"},
		{"SemanticCoral", th.SemanticCoral(), "#F47067"},
		{"SemanticGreen", th.SemanticGreen(), "#73D990"},
		{"SemanticRed", th.SemanticRed(), "#FF5F6D"},
		{"SemanticYellow", th.SemanticYellow(), "#E5C07B"},
		{"SemanticBlue", th.SemanticBlue(), "#61AFEF"},
		{"SemanticPurple", th.SemanticPurple(), "#C678DD"},
		{"DiffAddedFg", th.DiffAddedFg(), "#73D990"},
		{"DiffAddedBg", th.DiffAddedBg(), "#1A2E1A"},
		{"DiffRemovedFg", th.DiffRemovedFg(), "#FF5F6D"},
		{"DiffRemovedBg", th.DiffRemovedBg(), "#2E1A1A"},
		{"DiffHunk", th.DiffHunk(), "#61AFEF"},
	}

	for _, tt := range tokens {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the color was created from the expected hex
			expected := lipgloss.Color(tt.hex)
			if tt.got != expected {
				t.Errorf("%s color mismatch: got %v, want color from %s", tt.name, tt.got, tt.hex)
			}
		})
	}
}

func TestAgni_TokenCount(t *testing.T) {
	th := theme.NewAgni()
	tokens := []color.Color{
		th.BgDeep(), th.BgSurface(), th.BgElevated(), th.BgOverlay(),
		th.FgPrimary(), th.FgSecondary(), th.FgDimmed(),
		th.AccentGold(), th.AccentBright(),
		th.SemanticAqua(), th.SemanticCoral(), th.SemanticGreen(),
		th.SemanticRed(), th.SemanticYellow(), th.SemanticBlue(),
		th.SemanticPurple(),
		th.DiffAddedFg(), th.DiffAddedBg(),
		th.DiffRemovedFg(), th.DiffRemovedBg(), th.DiffHunk(),
	}

	if len(tokens) != 21 {
		t.Errorf("token count = %d, want 21", len(tokens))
	}
}

func TestAgni_StyleFactories(t *testing.T) {
	th := theme.NewAgni()

	styles := []struct {
		name  string
		style func() string
	}{
		{"BaseStyle", func() string { return th.BaseStyle().Render("test") }},
		{"ActiveRowStyle", func() string { return th.ActiveRowStyle().Render("test") }},
		{"DimmedStyle", func() string { return th.DimmedStyle().Render("test") }},
		{"AccentStyle", func() string { return th.AccentStyle().Render("test") }},
		{"ErrorStyle", func() string { return th.ErrorStyle().Render("test") }},
		{"SuccessStyle", func() string { return th.SuccessStyle().Render("test") }},
		{"SHAStyle", func() string { return th.SHAStyle().Render("test") }},
		{"BranchStyle", func() string { return th.BranchStyle().Render("test") }},
		{"StaleStyle", func() string { return th.StaleStyle().Render("test") }},
		{"DiffAddedStyle", func() string { return th.DiffAddedStyle().Render("test") }},
		{"DiffRemovedStyle", func() string { return th.DiffRemovedStyle().Render("test") }},
		{"DiffHunkStyle", func() string { return th.DiffHunkStyle().Render("test") }},
	}

	for _, tt := range styles {
		t.Run(tt.name, func(t *testing.T) {
			rendered := tt.style()
			if rendered == "" {
				t.Errorf("%s rendered empty string", tt.name)
			}
		})
	}
}

func TestColorProfile_String(t *testing.T) {
	tests := []struct {
		profile theme.ColorProfile
		want    string
	}{
		{theme.ProfileTrueColor, "truecolor"},
		{theme.ProfileANSI256, "ansi256"},
		{theme.ProfileANSI, "ansi"},
		{theme.ProfileNoColor, "nocolor"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.profile.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThemeForProfile(t *testing.T) {
	for _, p := range []theme.ColorProfile{
		theme.ProfileTrueColor, theme.ProfileANSI256,
		theme.ProfileANSI, theme.ProfileNoColor,
	} {
		th := theme.ThemeForProfile(p)
		if th == nil {
			t.Errorf("ThemeForProfile(%s) returned nil", p)
		}
		if th.Name() != "agni" {
			t.Errorf("ThemeForProfile(%s).Name() = %q, want %q", p, th.Name(), "agni")
		}
	}
}
