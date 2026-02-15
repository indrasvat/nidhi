package git_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

func TestParseMergeTreeOutput(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		wantHasConflict bool
		wantTreeSHA     string
		wantFiles       int
	}{
		{
			name:            "clean merge - tree SHA only",
			output:          "abc123def456\n",
			wantHasConflict: false,
			wantTreeSHA:     "abc123def456",
			wantFiles:       0,
		},
		{
			name: "single conflict",
			output: "abc123def456\n\n" +
				"Auto-merging src/auth/config.go\n" +
				"CONFLICT (content): Merge conflict in src/auth/config.go\n",
			wantHasConflict: true,
			wantTreeSHA:     "abc123def456",
			wantFiles:       1,
		},
		{
			name: "mixed clean and conflict",
			output: "abc123def456\n\n" +
				"Auto-merging src/auth/token.go\n" +
				"Auto-merging src/auth/config.go\n" +
				"CONFLICT (content): Merge conflict in src/auth/config.go\n",
			wantHasConflict: true,
			wantTreeSHA:     "abc123def456",
			wantFiles:       2,
		},
		{
			name: "multiple conflicts",
			output: "abc123def456\n\n" +
				"Auto-merging file1.go\n" +
				"CONFLICT (content): Merge conflict in file1.go\n" +
				"Auto-merging file2.go\n" +
				"CONFLICT (content): Merge conflict in file2.go\n" +
				"Auto-merging file3.go\n",
			wantHasConflict: true,
			wantTreeSHA:     "abc123def456",
			wantFiles:       3,
		},
		{
			name:            "empty output",
			output:          "",
			wantHasConflict: false,
			wantTreeSHA:     "",
			wantFiles:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := git.ParseMergeTreeOutput(tt.output)

			if result.HasConflicts != tt.wantHasConflict {
				t.Errorf("HasConflicts = %v, want %v", result.HasConflicts, tt.wantHasConflict)
			}
			if result.TreeSHA != tt.wantTreeSHA {
				t.Errorf("TreeSHA = %q, want %q", result.TreeSHA, tt.wantTreeSHA)
			}
			if len(result.Files) != tt.wantFiles {
				t.Errorf("len(Files) = %d, want %d", len(result.Files), tt.wantFiles)
			}
		})
	}
}

func TestParseMergeTreeOutput_FileStatuses(t *testing.T) {
	output := "deadbeef1234\n\n" +
		"Auto-merging clean.go\n" +
		"Auto-merging conflicted.go\n" +
		"CONFLICT (content): Merge conflict in conflicted.go\n"

	result := git.ParseMergeTreeOutput(output)

	statusByPath := make(map[string]git.FileConflictStatus)
	for _, f := range result.Files {
		statusByPath[f.Path] = f.Status
	}

	if s, ok := statusByPath["clean.go"]; !ok || s != git.FileStatusClean {
		t.Errorf("clean.go status = %v, want FileStatusClean", s)
	}
	if s, ok := statusByPath["conflicted.go"]; !ok || s != git.FileStatusConflicted {
		t.Errorf("conflicted.go status = %v, want FileStatusConflicted", s)
	}
}

func TestParseMergeTreeOutput_StableOrder(t *testing.T) {
	output := "abc\n\n" +
		"Auto-merging alpha.go\n" +
		"Auto-merging beta.go\n" +
		"CONFLICT (content): Merge conflict in beta.go\n" +
		"Auto-merging gamma.go\n"

	result := git.ParseMergeTreeOutput(output)

	if len(result.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(result.Files))
	}
	if result.Files[0].Path != "alpha.go" {
		t.Errorf("first file = %q, want alpha.go", result.Files[0].Path)
	}
	if result.Files[1].Path != "beta.go" {
		t.Errorf("second file = %q, want beta.go", result.Files[1].Path)
	}
	if result.Files[2].Path != "gamma.go" {
		t.Errorf("third file = %q, want gamma.go", result.Files[2].Path)
	}
}

// ─── Integration tests ─────────────────────────────────────

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunMergeTree_Integration_ConflictDetected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTempRepo(t)

	writeTestFile(t, dir, "config.go", "package main\n\nvar maxRetries = 3\n")
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "commit", "-m", "add config")

	writeTestFile(t, dir, "config.go", "package main\n\nvar maxRetries = 10\n")
	testHelper(t, dir, "git", "stash", "push", "-m", "bump retries to 10")

	writeTestFile(t, dir, "config.go", "package main\n\nvar maxRetries = 5\n")
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "commit", "-m", "set retries to 5")

	stashSHA := strings.TrimSpace(testHelper(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewDefaultRunner(dir, nil)
	result, err := git.RunMergeTree(context.Background(), runner, stashSHA)
	if err != nil {
		t.Fatalf("RunMergeTree failed: %v", err)
	}

	if !result.HasConflicts {
		t.Error("expected HasConflicts=true, got false")
	}

	var foundConflicted bool
	for _, f := range result.Files {
		if f.Path == "config.go" && f.Status == git.FileStatusConflicted {
			foundConflicted = true
		}
	}
	if !foundConflicted {
		t.Error("expected config.go to be marked as conflicted")
	}
}

func TestRunMergeTree_Integration_CleanApply(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTempRepo(t)

	writeTestFile(t, dir, "main.go", "package main\n\nfunc main() {}\n")
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "commit", "-m", "add main")

	writeTestFile(t, dir, "utils.go", "package main\n\nfunc helper() {}\n")
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "stash", "push", "-m", "add utils")

	stashSHA := strings.TrimSpace(testHelper(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewDefaultRunner(dir, nil)
	result, err := git.RunMergeTree(context.Background(), runner, stashSHA)
	if err != nil {
		t.Fatalf("RunMergeTree failed: %v", err)
	}

	if result.HasConflicts {
		t.Error("expected HasConflicts=false for clean apply, got true")
	}
}

func TestCheckUntrackedCollisions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTempRepo(t)

	writeTestFile(t, dir, "newfile.txt", "stashed content\n")
	testHelper(t, dir, "git", "stash", "push", "--include-untracked", "-m", "with untracked")

	writeTestFile(t, dir, "newfile.txt", "existing content\n")
	testHelper(t, dir, "git", "add", "newfile.txt")
	testHelper(t, dir, "git", "commit", "-m", "add newfile")

	stashSHA := strings.TrimSpace(testHelper(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewDefaultRunner(dir, nil)
	collisions, err := git.CheckUntrackedCollisions(context.Background(), runner, stashSHA)
	if err != nil {
		t.Fatalf("CheckUntrackedCollisions failed: %v", err)
	}

	var found bool
	for _, c := range collisions {
		if c.Path == "newfile.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected newfile.txt in collisions, got %v", collisions)
	}
}

func TestCheckUntrackedCollisions_NoUntracked(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	dir := setupTempRepo(t)

	writeTestFile(t, dir, "tracked.go", "package main\n")
	testHelper(t, dir, "git", "add", ".")
	testHelper(t, dir, "git", "stash", "push", "-m", "tracked only")

	stashSHA := strings.TrimSpace(testHelper(t, dir, "git", "rev-parse", "stash@{0}"))

	runner := git.NewDefaultRunner(dir, nil)
	collisions, err := git.CheckUntrackedCollisions(context.Background(), runner, stashSHA)
	if err != nil {
		t.Fatalf("CheckUntrackedCollisions failed: %v", err)
	}
	if len(collisions) != 0 {
		t.Errorf("expected no collisions, got %v", collisions)
	}
}
