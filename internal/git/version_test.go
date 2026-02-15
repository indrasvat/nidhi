package git_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantErr   bool
	}{
		{"standard format", "git version 2.53.0", 2, 53, 0, false},
		{"windows format", "git version 2.22.1.windows.1", 2, 22, 1, false},
		{"apple git format", "git version 2.39.3 (Apple Git-146)", 2, 39, 3, false},
		{"bare version number", "2.53.0", 2, 53, 0, false},
		{"old git version", "git version 1.7.7", 1, 7, 7, false},
		{"two-part version", "git version 2.38", 2, 38, 0, false},
		{"rc patch version", "git version 2.53.rc1", 2, 53, 0, false},
		{"empty string", "", 0, 0, 0, true},
		{"garbage input", "not a version", 0, 0, 0, true},
		{"single number", "2", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, err := git.ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ver.Major != tt.wantMajor {
				t.Errorf("major: got %d, want %d", ver.Major, tt.wantMajor)
			}
			if ver.Minor != tt.wantMinor {
				t.Errorf("minor: got %d, want %d", ver.Minor, tt.wantMinor)
			}
			if ver.Patch != tt.wantPatch {
				t.Errorf("patch: got %d, want %d", ver.Patch, tt.wantPatch)
			}
			if ver.Raw != tt.input {
				t.Errorf("raw: got %q, want %q", ver.Raw, tt.input)
			}
		})
	}
}

func TestGitVersion_AtLeast(t *testing.T) {
	tests := []struct {
		name  string
		ver   git.GitVersion
		major int
		minor int
		patch int
		want  bool
	}{
		{"exact match", git.GitVersion{Major: 2, Minor: 38, Patch: 0}, 2, 38, 0, true},
		{"higher minor", git.GitVersion{Major: 2, Minor: 53, Patch: 0}, 2, 38, 0, true},
		{"higher patch", git.GitVersion{Major: 2, Minor: 38, Patch: 5}, 2, 38, 0, true},
		{"lower minor", git.GitVersion{Major: 2, Minor: 37, Patch: 0}, 2, 38, 0, false},
		{"lower patch", git.GitVersion{Major: 2, Minor: 38, Patch: 0}, 2, 38, 1, false},
		{"higher major", git.GitVersion{Major: 3, Minor: 0, Patch: 0}, 2, 99, 99, true},
		{"lower major", git.GitVersion{Major: 1, Minor: 99, Patch: 99}, 2, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ver.AtLeast(tt.major, tt.minor, tt.patch)
			if got != tt.want {
				t.Errorf("(%d.%d.%d).AtLeast(%d, %d, %d) = %v, want %v",
					tt.ver.Major, tt.ver.Minor, tt.ver.Patch,
					tt.major, tt.minor, tt.patch, got, tt.want)
			}
		})
	}
}

func TestGitVersion_Supports(t *testing.T) {
	tests := []struct {
		name    string
		ver     git.GitVersion
		feature string
		want    bool
	}{
		{"2.53 supports branch-show-current", git.GitVersion{Major: 2, Minor: 53}, git.FeatureBranchShowCurrent, true},
		{"2.53 supports merge-tree", git.GitVersion{Major: 2, Minor: 53}, git.FeatureMergeTree, true},
		{"2.53 supports stash-export-import", git.GitVersion{Major: 2, Minor: 53}, git.FeatureStashExportImport, true},
		{"2.37 does not support merge-tree", git.GitVersion{Major: 2, Minor: 37}, git.FeatureMergeTree, false},
		{"2.38 supports merge-tree", git.GitVersion{Major: 2, Minor: 38}, git.FeatureMergeTree, true},
		{"2.50 does not support stash-export-import", git.GitVersion{Major: 2, Minor: 50}, git.FeatureStashExportImport, false},
		{"2.51 supports stash-export-import", git.GitVersion{Major: 2, Minor: 51}, git.FeatureStashExportImport, true},
		{"2.21 does not support branch-show-current", git.GitVersion{Major: 2, Minor: 21}, git.FeatureBranchShowCurrent, false},
		{"unknown feature always false", git.GitVersion{Major: 99, Minor: 99, Patch: 99}, "nonexistent-feature", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ver.Supports(tt.feature)
			if got != tt.want {
				t.Errorf("(%s).Supports(%q) = %v, want %v", tt.ver, tt.feature, got, tt.want)
			}
		})
	}
}

func TestGitVersion_String(t *testing.T) {
	ver := git.GitVersion{Major: 2, Minor: 53, Patch: 0}
	if s := ver.String(); s != "2.53.0" {
		t.Errorf("String() = %q, want %q", s, "2.53.0")
	}
}

func TestGitVersion_IsZero(t *testing.T) {
	zero := git.GitVersion{}
	if !zero.IsZero() {
		t.Error("expected zero value to be zero")
	}
	nonZero := git.GitVersion{Major: 2}
	if nonZero.IsZero() {
		t.Error("expected non-zero value to not be zero")
	}
}

func TestDetectVersion_RealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	ver, err := git.DetectVersion(ctx, runner)
	if err != nil {
		t.Fatalf("DetectVersion failed: %v", err)
	}

	if ver.IsZero() {
		t.Error("detected version should not be zero")
	}
	if ver.Major < 2 {
		t.Errorf("expected git major version >= 2, got %d", ver.Major)
	}

	t.Logf("Detected git version: %s (raw: %q)", ver, ver.Raw)
}
