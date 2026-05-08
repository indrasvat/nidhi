package components

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

const sampleDiff = `diff --git a/src/auth/token.go b/src/auth/token.go
index abc1234..def5678 100644
--- a/src/auth/token.go
+++ b/src/auth/token.go
@@ -42,7 +42,12 @@ func RefreshToken(ctx context.Context) error {
     if token.IsExpired() {
-        return nil, ErrExpired
+        newToken, err := provider.Refresh(token)
+        if err != nil {
+            return nil, fmt.Errorf("refresh: %w", err)
+        }
+        return newToken, nil
     }
     return token, nil
 }`

func TestParseDiff_LineTypes(t *testing.T) {
	lines := parseDiff(sampleDiff)

	if len(lines) == 0 {
		t.Fatal("parseDiff returned no lines")
	}

	var headers, hunks, added, removed, context int
	for _, l := range lines {
		switch l.Type {
		case DiffLineHeader:
			headers++
		case DiffLineHunk:
			hunks++
		case DiffLineAdded:
			added++
		case DiffLineRemoved:
			removed++
		case DiffLineContext:
			context++
		}
	}

	if headers < 3 {
		t.Errorf("expected >= 3 header lines (diff, ---, +++), got %d", headers)
	}
	if hunks != 1 {
		t.Errorf("expected 1 hunk header, got %d", hunks)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed line, got %d", removed)
	}
	if added != 5 {
		t.Errorf("expected 5 added lines, got %d", added)
	}
	if context < 2 {
		t.Errorf("expected >= 2 context lines, got %d", context)
	}
}

func TestParseDiff_LineNumbers(t *testing.T) {
	lines := parseDiff(sampleDiff)

	for _, l := range lines {
		if l.Type == DiffLineAdded && l.NewNum > 0 {
			if l.NewNum < 42 {
				t.Errorf("first added line NewNum = %d, expected >= 42", l.NewNum)
			}
			break
		}
	}
}

func TestParseHunkHeader(t *testing.T) {
	tests := []struct {
		header  string
		wantOld int
		wantNew int
	}{
		{"@@ -42,7 +42,12 @@ func RefreshToken", 42, 42},
		{"@@ -1,3 +1,5 @@", 1, 1},
		{"@@ -100 +200 @@", 100, 200},
		{"@@ -0,0 +1,10 @@", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			old, new := parseHunkHeader(tt.header)
			if old != tt.wantOld {
				t.Errorf("oldStart = %d, want %d", old, tt.wantOld)
			}
			if new != tt.wantNew {
				t.Errorf("newStart = %d, want %d", new, tt.wantNew)
			}
		})
	}
}

func TestDiffViewModel_SetContent(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 20)
	dv.SetContent(sampleDiff)

	if dv.LineCount() == 0 {
		t.Error("LineCount() should be > 0 after SetContent")
	}
}

func TestDiffViewModel_EmptyDiff(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 20)
	dv.SetContent("")

	if dv.LineCount() != 0 {
		t.Errorf("LineCount() for empty diff = %d, want 0", dv.LineCount())
	}
}

func TestDiffViewModel_ViewBeforeContent(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 20)
	view := dv.View()

	if !strings.Contains(stripAnsi(view), "No diff loaded") {
		t.Errorf("view before content should show placeholder, got: %q", stripAnsi(view))
	}
}

func TestDiffViewModel_SetSize(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 20)
	dv.SetContent(sampleDiff)

	dv.SetSize(120, 40)

	if dv.LineCount() == 0 {
		t.Error("LineCount() should still be > 0 after SetSize")
	}
}

func TestDiffViewModel_ScrollDown(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 5)
	dv.SetContent(sampleDiff)

	dv.ScrollDown(3)
	if dv.offset != 3 {
		t.Errorf("offset after ScrollDown(3) = %d, want 3", dv.offset)
	}

	// Should not scroll past end.
	dv.ScrollDown(1000)
	maxOffset := max(dv.LineCount()-5, 0)
	if dv.offset != maxOffset {
		t.Errorf("offset after large scroll = %d, want %d", dv.offset, maxOffset)
	}
}

func TestDiffViewModel_ScrollUp(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 5)
	dv.SetContent(sampleDiff)

	dv.ScrollDown(5)
	dv.ScrollUp(2)
	if dv.offset != 3 {
		t.Errorf("offset after ScrollDown(5)+ScrollUp(2) = %d, want 3", dv.offset)
	}

	// Should not scroll past top.
	dv.ScrollUp(100)
	if dv.offset != 0 {
		t.Errorf("offset after large ScrollUp = %d, want 0", dv.offset)
	}
}

func TestDiffViewModel_ScrollPercent(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 5)
	dv.SetContent(sampleDiff)

	pct0 := dv.ScrollPercent()
	if pct0 != 0.0 {
		t.Errorf("ScrollPercent at top = %f, want 0.0", pct0)
	}

	dv.ScrollDown(1000)
	pct100 := dv.ScrollPercent()
	if pct100 != 1.0 {
		t.Errorf("ScrollPercent at bottom = %f, want 1.0", pct100)
	}
}

func TestDiffLineType_Coverage(t *testing.T) {
	diff := "diff --git a/f b/f\nindex abc..def 100644\n--- a/f\n+++ b/f\n@@ -1,2 +1,2 @@\n context\n-removed\n+added"
	lines := parseDiff(diff)

	typesSeen := make(map[DiffLineType]bool)
	for _, l := range lines {
		typesSeen[l.Type] = true
	}

	required := []DiffLineType{DiffLineHeader, DiffLineHunk, DiffLineAdded, DiffLineRemoved, DiffLineContext}
	for _, typ := range required {
		if !typesSeen[typ] {
			t.Errorf("DiffLineType %d not seen in parsed output", typ)
		}
	}
}

func TestDiffViewModel_ViewRendersContent(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 120, 30)
	dv.SetContent(sampleDiff)

	view := dv.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "token.go") {
		t.Errorf("view should contain filename from diff, got: %q", plain)
	}
}

func TestParseDiff_NoTrailingGhostLine(t *testing.T) {
	// Diff with trailing newline should not produce a ghost empty line.
	diff := "diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,2 +1,2 @@\n context\n-old\n+new\n"
	lines := parseDiff(diff)

	last := lines[len(lines)-1]
	if last.Content == "" && last.Type == DiffLineContext {
		t.Error("parseDiff should not produce a trailing ghost empty line")
	}
}

func TestDiffViewModel_GutterFillsViewport(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 20)
	// Short diff — fewer lines than viewport height.
	dv.SetContent("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1 +1 @@\n-old\n+new")

	view := dv.View()
	outputLines := strings.Split(view, "\n")

	// Output should fill the full viewport height (20 lines).
	if len(outputLines) < 20 {
		t.Errorf("gutter should fill viewport: got %d lines, want >= 20", len(outputLines))
	}

	// Every line should contain the gutter separator.
	for i, line := range outputLines {
		if !strings.Contains(line, "\u2502") {
			t.Errorf("line %d missing gutter separator: %q", i, stripAnsi(line))
		}
	}
}

func TestDiffViewModel_FileHeader(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 20)
	dv.SetContent(sampleDiff)
	dv.SetFileName("src/auth/token.go")

	view := dv.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "src/auth/token.go") {
		t.Errorf("view should contain file header, got: %q", plain)
	}
}

func TestDiffViewModel_FocusChangesStyle(t *testing.T) {
	dv := NewDiffViewModel(theme.NewAgni(), 80, 20)
	dv.SetContent(sampleDiff)
	dv.SetFileName("test.go")

	dv.SetFocused(true)
	focused := dv.View()

	dv.SetFocused(false)
	unfocused := dv.View()

	if focused == unfocused {
		t.Error("focused and unfocused views should differ in header styling")
	}
}
