package git

import (
	"strings"
	"testing"
)

// ─── ParsePatch ─────────────────────────────────────────────

func TestParsePatch_SingleFileSingleHunk(t *testing.T) {
	diff := join(
		"diff --git a/foo.txt b/foo.txt",
		"index 1111111..2222222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,2 +1,3 @@",
		" a",
		"+b",
		" d",
	)

	ps, err := ParsePatch(diff)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	if len(ps.Files) != 1 {
		t.Fatalf("want 1 file, got %d", len(ps.Files))
	}
	f := ps.Files[0]
	if f.OldPath != "foo.txt" || f.NewPath != "foo.txt" {
		t.Errorf("paths: old=%q new=%q", f.OldPath, f.NewPath)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("want 1 hunk, got %d", len(f.Hunks))
	}
	h := f.Hunks[0]
	if h.OldStart != 1 || h.OldCount != 2 || h.NewStart != 1 || h.NewCount != 3 {
		t.Errorf("hunk header: -%d,%d +%d,%d", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
	}
	if len(h.Lines) != 3 {
		t.Fatalf("want 3 lines, got %d", len(h.Lines))
	}
	if h.Lines[0].Kind != LineContext || h.Lines[0].Text != "a" {
		t.Errorf("line0: %+v", h.Lines[0])
	}
	if h.Lines[1].Kind != LineAdded || h.Lines[1].Text != "b" {
		t.Errorf("line1: %+v", h.Lines[1])
	}
}

func TestParsePatch_MultiFile(t *testing.T) {
	diff := join(
		"diff --git a/one.txt b/one.txt",
		"index aaa..bbb 100644",
		"--- a/one.txt",
		"+++ b/one.txt",
		"@@ -1 +1,2 @@",
		" x",
		"+y",
		"diff --git a/two.txt b/two.txt",
		"index ccc..ddd 100644",
		"--- a/two.txt",
		"+++ b/two.txt",
		"@@ -5,2 +5,1 @@",
		" p",
		"-q",
	)
	ps, err := ParsePatch(diff)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	if len(ps.Files) != 2 {
		t.Fatalf("want 2 files, got %d", len(ps.Files))
	}
	if ps.Files[1].OldPath != "two.txt" {
		t.Errorf("file1 path = %q", ps.Files[1].OldPath)
	}
}

func TestParsePatch_NewAndDeletedAndBinary(t *testing.T) {
	diff := join(
		"diff --git a/new.txt b/new.txt",
		"new file mode 100644",
		"index 0000000..1111111",
		"--- /dev/null",
		"+++ b/new.txt",
		"@@ -0,0 +1,2 @@",
		"+hello",
		"+world",
		"diff --git a/gone.txt b/gone.txt",
		"deleted file mode 100644",
		"index 2222222..0000000",
		"--- a/gone.txt",
		"+++ /dev/null",
		"@@ -1,1 +0,0 @@",
		"-bye",
		"diff --git a/img.png b/img.png",
		"index 3333333..4444444 100644",
		"Binary files a/img.png and b/img.png differ",
	)
	ps, err := ParsePatch(diff)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	if len(ps.Files) != 3 {
		t.Fatalf("want 3 files, got %d", len(ps.Files))
	}
	if !ps.Files[0].IsNew {
		t.Error("new.txt should be IsNew")
	}
	if !ps.Files[1].IsDelete {
		t.Error("gone.txt should be IsDelete")
	}
	if !ps.Files[2].IsBinary {
		t.Error("img.png should be IsBinary")
	}
}

func TestParsePatch_Empty(t *testing.T) {
	ps, err := ParsePatch("")
	if err != nil {
		t.Fatalf("ParsePatch(\"\"): %v", err)
	}
	if len(ps.Files) != 0 {
		t.Errorf("want 0 files, got %d", len(ps.Files))
	}
}

// ─── BuildSelectedPatch ─────────────────────────────────────

func TestBuildSelectedPatch_SelectNothing(t *testing.T) {
	ps := mustParse(t, join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,2 +1,3 @@",
		" a",
		"+b",
		" d",
	))
	if got := ps.BuildSelectedPatch(); got != "" {
		t.Errorf("want empty patch, got:\n%s", got)
	}
}

func TestBuildSelectedPatch_RoundTripWholeHunk(t *testing.T) {
	src := join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,2 +1,3 @@",
		" a",
		"+b",
		" d",
	)
	ps := mustParse(t, src)
	ps.SelectAll(true)
	got := strings.TrimRight(ps.BuildSelectedPatch(), "\n")
	want := strings.TrimRight(src, "\n")
	if got != want {
		t.Errorf("round trip mismatch:\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestBuildSelectedPatch_DropUnselectedAdded(t *testing.T) {
	ps := mustParse(t, join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,2 +1,4 @@",
		" a",
		"+b",
		"+c",
		" d",
	))
	// Select only the first added line (b), not c.
	h := &ps.Files[0].Hunks[0]
	for i := range h.Lines {
		if h.Lines[i].Kind == LineAdded && h.Lines[i].Text == "b" {
			h.Lines[i].Selected = true
		}
	}
	got := strings.TrimRight(ps.BuildSelectedPatch(), "\n")
	want := join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,2 +1,3 @@",
		" a",
		"+b",
		" d",
	)
	want = strings.TrimRight(want, "\n")
	if got != want {
		t.Errorf("mismatch:\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestBuildSelectedPatch_UnselectedRemovedBecomesContext(t *testing.T) {
	ps := mustParse(t, join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,3 +1,1 @@",
		" a",
		"-b",
		"-c",
	))
	// Select only removal of b; keep c (its removal is unselected).
	h := &ps.Files[0].Hunks[0]
	for i := range h.Lines {
		if h.Lines[i].Kind == LineRemoved && h.Lines[i].Text == "b" {
			h.Lines[i].Selected = true
		}
	}
	got := strings.TrimRight(ps.BuildSelectedPatch(), "\n")
	want := strings.TrimRight(join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,3 +1,2 @@",
		" a",
		"-b",
		" c",
	), "\n")
	if got != want {
		t.Errorf("mismatch:\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestBuildSelectedPatch_MultiHunkNewStartRecompute(t *testing.T) {
	src := join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,1 +1,2 @@",
		" a",
		"+x",
		"@@ -10,1 +11,2 @@",
		" j",
		"+y",
	)
	ps := mustParse(t, src)
	// Select only the SECOND hunk's addition.
	h2 := &ps.Files[0].Hunks[1]
	for i := range h2.Lines {
		if h2.Lines[i].Kind == LineAdded {
			h2.Lines[i].Selected = true
		}
	}
	got := strings.TrimRight(ps.BuildSelectedPatch(), "\n")
	// First hunk drops out entirely (pure context), so the second hunk's
	// newStart must be recomputed back to its oldStart (no preceding delta).
	want := strings.TrimRight(join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -10 +10,2 @@",
		" j",
		"+y",
	), "\n")
	if got != want {
		t.Errorf("mismatch:\nGOT:\n%s\n\nWANT:\n%s", got, want)
	}
}

func TestSelectedStats(t *testing.T) {
	ps := mustParse(t, join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,2 +1,4 @@",
		" a",
		"+b",
		"+c",
		"-d",
		" e",
	))
	ps.SelectAll(true)
	st := ps.SelectedStats()
	if st.Added != 2 || st.Removed != 1 {
		t.Errorf("stats: +%d -%d (want +2 -1)", st.Added, st.Removed)
	}
	if st.Files != 1 || st.Hunks != 1 {
		t.Errorf("stats: files=%d hunks=%d (want 1/1)", st.Files, st.Hunks)
	}
}

func TestSelectionState_FileAndHunk(t *testing.T) {
	ps := mustParse(t, join(
		"diff --git a/foo.txt b/foo.txt",
		"index 111..222 100644",
		"--- a/foo.txt",
		"+++ b/foo.txt",
		"@@ -1,1 +1,3 @@",
		" a",
		"+b",
		"+c",
	))
	f := &ps.Files[0]
	if f.SelectionState() != SelNone {
		t.Errorf("fresh file should be SelNone, got %v", f.SelectionState())
	}
	h := &f.Hunks[0]
	// Select one of two added lines → partial.
	for i := range h.Lines {
		if h.Lines[i].Kind == LineAdded {
			h.Lines[i].Selected = true
			break
		}
	}
	if h.SelectionState() != SelPartial {
		t.Errorf("hunk should be SelPartial, got %v", h.SelectionState())
	}
	if f.SelectionState() != SelPartial {
		t.Errorf("file should be SelPartial, got %v", f.SelectionState())
	}
	h.SetSelected(true)
	if h.SelectionState() != SelAll {
		t.Errorf("hunk should be SelAll, got %v", h.SelectionState())
	}
}

func TestBuildSelectedPatch_SkipsBinaryEmitsNewFile(t *testing.T) {
	ps := mustParse(t, join(
		"diff --git a/new.txt b/new.txt",
		"new file mode 100644",
		"index 0000000..1111111",
		"--- /dev/null",
		"+++ b/new.txt",
		"@@ -0,0 +1,2 @@",
		"+hello",
		"+world",
		"diff --git a/img.png b/img.png",
		"index 333..444 100644",
		"Binary files a/img.png and b/img.png differ",
	))
	ps.SelectAll(true)
	got := ps.BuildSelectedPatch()
	if !strings.Contains(got, "new file mode") || !strings.Contains(got, "+hello") {
		t.Errorf("new file should be emitted whole:\n%s", got)
	}
	// New-file hunk must use git's +1 convention, not +0.
	if !strings.Contains(got, "@@ -0,0 +1,2 @@") {
		t.Errorf("new-file hunk header should be -0,0 +1,2:\n%s", got)
	}
	if strings.Contains(got, "img.png") {
		t.Errorf("binary file must be skipped, got:\n%s", got)
	}
}

func TestFileDiff_WholeFileOnly(t *testing.T) {
	ps := mustParse(t, join(
		"diff --git a/new.txt b/new.txt",
		"new file mode 100644",
		"index 0000000..1111111",
		"--- /dev/null",
		"+++ b/new.txt",
		"@@ -0,0 +1,1 @@",
		"+x",
	))
	if !ps.Files[0].WholeFileOnly() {
		t.Error("new file should be WholeFileOnly")
	}
}

// ─── helpers ────────────────────────────────────────────────

func join(lines ...string) string {
	return strings.Join(lines, "\n") + "\n"
}

func mustParse(t *testing.T, diff string) *PatchSet {
	t.Helper()
	ps, err := ParsePatch(diff)
	if err != nil {
		t.Fatalf("ParsePatch: %v", err)
	}
	return ps
}
