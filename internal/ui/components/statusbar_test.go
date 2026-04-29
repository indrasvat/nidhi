package components

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func TestStatusBar_ContainsExpectedElements(t *testing.T) {
	sb := NewStatusBar(theme.NewAgni())
	rendered := sb.Render(StatusBarParams{
		RepoName:   "drishti-led",
		Branch:     "main",
		StashCount: 5,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 53},
		Width:      120,
		UseNerd:    false,
	})

	plain := stripAnsi(rendered)

	checks := []struct {
		name     string
		contains string
	}{
		{"app mark", "◆"},
		{"repo name", "drishti-led"},
		{"branch", "main"},
		{"stash count", "5 stashes"},
		{"git version", "git 2.53"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(plain, c.contains) {
				t.Errorf("status bar should contain %q, got: %q", c.contains, plain)
			}
		})
	}
}

func TestStatusBar_BranchPrefix(t *testing.T) {
	sb := NewStatusBar(theme.NewAgni())
	rendered := sb.Render(StatusBarParams{
		RepoName:   "myrepo",
		Branch:     "feat/auth",
		StashCount: 3,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 51},
		Width:      120,
		UseNerd:    false,
	})

	plain := stripAnsi(rendered)
	// Mockup uses ⎇ prefix before branch name.
	if !strings.Contains(plain, "⎇ feat/auth") {
		t.Errorf("branch should have ⎇ prefix, got: %q", plain)
	}
}

func TestStatusBar_FallbackRepoName(t *testing.T) {
	sb := NewStatusBar(theme.NewAgni())
	rendered := sb.Render(StatusBarParams{
		Branch:     "main",
		StashCount: 0,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 38},
		Width:      80,
		UseNerd:    false,
	})

	plain := stripAnsi(rendered)
	// When no repo name, should fall back to "nidhi".
	if !strings.Contains(plain, "nidhi") {
		t.Errorf("should fallback to 'nidhi' when RepoName is empty, got: %q", plain)
	}
}

func TestStatusBar_ZeroStashes(t *testing.T) {
	sb := NewStatusBar(theme.NewAgni())
	rendered := sb.Render(StatusBarParams{
		RepoName:   "myrepo",
		Branch:     "main",
		StashCount: 0,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 38},
		Width:      80,
		UseNerd:    false,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "0 stashes") {
		t.Errorf("should show '0 stashes', got: %q", plain)
	}
}

func TestStatusBar_RepoInfo(t *testing.T) {
	sb := NewStatusBar(theme.NewAgni())
	rendered := sb.Render(StatusBarParams{
		RepoName:   "myrepo",
		Branch:     "main",
		StashCount: 1,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 54},
		RepoInfo: plugin.RepoInfo{
			Available:        true,
			ObjectFormat:     "sha256",
			ReferencesFormat: "reftable",
		},
		Width:   120,
		UseNerd: false,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "sha256/reftable") {
		t.Errorf("should show repo format label, got: %q", plain)
	}
}

func TestStatusBar_Width(t *testing.T) {
	sb := NewStatusBar(theme.NewAgni())
	rendered := sb.Render(StatusBarParams{
		RepoName:   "myrepo",
		Branch:     "main",
		StashCount: 5,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 53},
		Width:      80,
		UseNerd:    false,
	})

	width := lipgloss.Width(rendered)
	if width > 80 {
		t.Errorf("status bar width = %d, should be <= 80", width)
	}
}

// stripAnsi removes ANSI escape sequences for content testing.
func stripAnsi(s string) string {
	result := make([]byte, 0, len(s))
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}
