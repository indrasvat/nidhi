package git_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
)

func TestParseStashList_Standard(t *testing.T) {
	now := time.Now()
	date := now.Add(-2 * time.Hour).Format(time.RFC3339)

	line := fmt.Sprintf("abc123def456\x00abc123d\x00On main: fix auth bug\x00%s", date)
	stashes := git.ParseStashList(line, 14*24*time.Hour)

	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash, got %d", len(stashes))
	}

	s := stashes[0]
	if s.Index != 0 {
		t.Errorf("Index = %d, want 0", s.Index)
	}
	if s.SHA != "abc123def456" {
		t.Errorf("SHA = %q, want %q", s.SHA, "abc123def456")
	}
	if s.Message != "fix auth bug" {
		t.Errorf("Message = %q, want %q", s.Message, "fix auth bug")
	}
	if s.Branch != "main" {
		t.Errorf("Branch = %q, want %q", s.Branch, "main")
	}
	if s.IsStale {
		t.Error("should not be stale (2 hours old, threshold 14 days)")
	}
}

func TestParseStashList_WIPMessage(t *testing.T) {
	date := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("sha1234\x00sha1234\x00WIP on feature/auth: abc1234 add login form\x00%s", date)
	stashes := git.ParseStashList(line, 14*24*time.Hour)

	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash, got %d", len(stashes))
	}

	s := stashes[0]
	if s.Branch != "feature/auth" {
		t.Errorf("Branch = %q, want %q", s.Branch, "feature/auth")
	}
	if s.Message != "add login form" {
		t.Errorf("Message = %q, want %q", s.Message, "add login form")
	}
}

func TestParseStashList_MultipleStashes(t *testing.T) {
	date1 := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	date2 := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)

	input := fmt.Sprintf(
		"sha1\x00sh1\x00On main: fix one\x00%s\nsha2\x00sh2\x00On develop: fix two\x00%s",
		date1, date2,
	)
	stashes := git.ParseStashList(input, 14*24*time.Hour)

	if len(stashes) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(stashes))
	}

	if !stashes[1].IsStale {
		t.Error("second stash should be stale (30 days old, threshold 14 days)")
	}
	if stashes[0].IsStale {
		t.Error("first stash should not be stale (1 hour old)")
	}
}

func TestParseStashList_EmptyInput(t *testing.T) {
	stashes := git.ParseStashList("", 14*24*time.Hour)
	if stashes != nil {
		t.Errorf("expected nil for empty input, got %v", stashes)
	}
}

func TestParseStashList_Staleness(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		threshold time.Duration
		wantStale bool
	}{
		{"fresh", 1 * time.Hour, 14 * 24 * time.Hour, false},
		{"stale", 15 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"zero threshold disables", 100 * 24 * time.Hour, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := time.Now().Add(-tt.age).Format(time.RFC3339)
			line := fmt.Sprintf("sha1\x00sh1\x00On main: test\x00%s", date)
			stashes := git.ParseStashList(line, tt.threshold)

			if len(stashes) != 1 {
				t.Fatalf("expected 1 stash, got %d", len(stashes))
			}
			if stashes[0].IsStale != tt.wantStale {
				t.Errorf("IsStale = %v, want %v", stashes[0].IsStale, tt.wantStale)
			}
		})
	}
}

func TestGenerateAutoMessage(t *testing.T) {
	tests := []struct {
		name       string
		fileCount  int
		insertions int
		deletions  int
		topDirs    []string
		want       string
	}{
		{"single file", 1, 42, 17, []string{"src/auth"}, "1 file: +42/-17 in src/auth"},
		{"multiple files", 3, 42, 17, []string{"src/auth", "pkg/db"}, "3 files: +42/-17 in src/auth, pkg/db"},
		{"many dirs", 10, 200, 50, []string{"a", "b", "c", "d"}, "10 files: +200/-50 in a, b, c +1 more"},
		{"no dirs", 5, 10, 3, nil, "5 files: +10/-3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.GenerateAutoMessage(tt.fileCount, tt.insertions, tt.deletions, tt.topDirs)
			if got != tt.want {
				t.Errorf("GenerateAutoMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListStashes_RealGit(t *testing.T) {
	dir := setupTempRepo(t)

	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "main.go")
	testHelper(t, dir, "git", "commit", "-m", "add main.go")

	if err := os.WriteFile(filePath, []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "add hello print")

	if err := os.WriteFile(filePath, []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"world\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push")

	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	stashes, err := git.ListStashes(ctx, runner, 14*24*time.Hour)
	if err != nil {
		t.Fatalf("ListStashes failed: %v", err)
	}

	if len(stashes) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(stashes))
	}

	if stashes[0].SHA == "" {
		t.Error("first stash SHA should not be empty")
	}
	if stashes[1].Message != "add hello print" {
		t.Errorf("second stash Message = %q, want %q", stashes[1].Message, "add hello print")
	}
	if stashes[0].FileCount == 0 {
		t.Error("first stash FileCount should be > 0")
	}

	t.Logf("Stash 0: %+v", stashes[0])
	t.Logf("Stash 1: %+v", stashes[1])
}
