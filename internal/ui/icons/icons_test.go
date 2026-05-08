package icons_test

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/icons"
)

func TestAllIconsExist(t *testing.T) {
	allIcons := icons.AllIcons()
	allNames := icons.AllIconNames()

	if len(allIcons) != 14 {
		t.Errorf("icon count = %d, want 14", len(allIcons))
	}
	if len(allNames) != 14 {
		t.Errorf("name count = %d, want 14", len(allNames))
	}

	for i, icon := range allIcons {
		t.Run(allNames[i], func(t *testing.T) {
			if icon.Nerd == "" {
				t.Errorf("%s has empty Nerd glyph", allNames[i])
			}
			if icon.ASCII == "" {
				t.Errorf("%s has empty ASCII fallback", allNames[i])
			}
		})
	}
}

func TestIcon_String_NerdMode(t *testing.T) {
	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	if got := icon.String(icons.ModeNerd); got != "N" {
		t.Errorf("String(ModeNerd) = %q, want %q", got, "N")
	}
}

func TestIcon_String_ASCIIMode(t *testing.T) {
	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	if got := icon.String(icons.ModeASCII); got != "A" {
		t.Errorf("String(ModeASCII) = %q, want %q", got, "A")
	}
}

func TestIcon_String_AutoMode_NoNerdFonts(t *testing.T) {
	icons.ResetDetection()
	t.Setenv("NERD_FONTS", "0")

	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	if got := icon.String(icons.ModeAuto); got != "A" {
		t.Errorf("String(ModeAuto) without Nerd Fonts = %q, want %q", got, "A")
	}
	icons.ResetDetection()
}

func TestIcon_String_AutoMode_WithNerdFonts(t *testing.T) {
	icons.ResetDetection()
	t.Setenv("NERD_FONTS", "1")

	icon := icons.Icon{Nerd: "N", ASCII: "A"}
	if got := icon.String(icons.ModeAuto); got != "N" {
		t.Errorf("String(ModeAuto) with Nerd Fonts = %q, want %q", got, "N")
	}
	icons.ResetDetection()
}

func TestDetectNerdFonts_EnvVar(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{"on with 1", "1", true},
		{"on with true", "true", true},
		{"on with yes", "yes", true},
		{"off with 0", "0", false},
		{"off with false", "false", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icons.ResetDetection()
			t.Setenv("NERD_FONTS", tt.env)

			if got := icons.DetectNerdFonts(); got != tt.want {
				t.Errorf("DetectNerdFonts() with NERD_FONTS=%q = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
	icons.ResetDetection()
}

func TestParseMode(t *testing.T) {
	tests := []struct {
		input string
		want  icons.Mode
	}{
		{"nerd", icons.ModeNerd},
		{"ascii", icons.ModeASCII},
		{"auto", icons.ModeAuto},
		{"", icons.ModeAuto},
		{"invalid", icons.ModeAuto},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := icons.ParseMode(tt.input); got != tt.want {
				t.Errorf("ParseMode(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
