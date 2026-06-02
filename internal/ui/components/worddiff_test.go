package components

import (
	"testing"
)

func TestTokenize_SimpleWords(t *testing.T) {
	tokens := tokenize("hello world")
	texts := tokenTexts(tokens)
	want := []string{"hello", " ", "world"}
	assertStrSlice(t, texts, want)
}

func TestTokenize_Punctuation(t *testing.T) {
	tokens := tokenize("foo(bar, baz)")
	texts := tokenTexts(tokens)
	want := []string{"foo", "(", "bar", ",", " ", "baz", ")"}
	assertStrSlice(t, texts, want)
}

func TestTokenize_Empty(t *testing.T) {
	tokens := tokenize("")
	if len(tokens) != 0 {
		t.Errorf("tokenize empty string: got %d tokens, want 0", len(tokens))
	}
}

func TestTokenize_ByteOffsets(t *testing.T) {
	tokens := tokenize("ab cd")
	// "ab" at 0, " " at 2, "cd" at 3
	if tokens[0].start != 0 {
		t.Errorf("first token start = %d, want 0", tokens[0].start)
	}
	if tokens[1].start != 2 {
		t.Errorf("space token start = %d, want 2", tokens[1].start)
	}
	if tokens[2].start != 3 {
		t.Errorf("third token start = %d, want 3", tokens[2].start)
	}
}

func TestMyersDiff_Identical(t *testing.T) {
	a := tokenize("hello world")
	b := tokenize("hello world")
	edits := myersDiff(a, b)
	for _, e := range edits {
		if e.op != editEqual {
			t.Errorf("identical inputs should produce only Equal ops, got %d", e.op)
		}
	}
}

func TestMyersDiff_SingleWordChange(t *testing.T) {
	a := tokenize("return nil")
	b := tokenize("return err")
	edits := myersDiff(a, b)

	var deletes, inserts, equals int
	for _, e := range edits {
		switch e.op {
		case editDelete:
			deletes++
		case editInsert:
			inserts++
		case editEqual:
			equals++
		}
	}

	if deletes != 1 || inserts != 1 {
		t.Errorf("single word change: deletes=%d, inserts=%d, want 1,1", deletes, inserts)
	}
	if equals < 2 {
		t.Errorf("should have at least 2 equal ops (return + space), got %d", equals)
	}
}

func TestMyersDiff_EmptyInputs(t *testing.T) {
	edits := myersDiff(nil, nil)
	if len(edits) != 0 {
		t.Errorf("both empty: got %d edits, want 0", len(edits))
	}

	a := tokenize("hello")
	edits = myersDiff(a, nil)
	if len(edits) != 1 || edits[0].op != editDelete {
		t.Errorf("a non-empty, b empty: expected 1 delete, got %v", edits)
	}

	edits = myersDiff(nil, a)
	if len(edits) != 1 || edits[0].op != editInsert {
		t.Errorf("a empty, b non-empty: expected 1 insert, got %v", edits)
	}
}

func TestPairChangedLines_SimplePair(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffLineRemoved, Content: "-old"},
		{Type: DiffLineAdded, Content: "+new"},
	}
	pairs := pairChangedLines(lines)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0].removedIdx != 0 || pairs[0].addedIdx != 1 {
		t.Errorf("pair indices: got (%d,%d), want (0,1)", pairs[0].removedIdx, pairs[0].addedIdx)
	}
}

func TestPairChangedLines_MultiplePairs(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffLineRemoved, Content: "-a"},
		{Type: DiffLineRemoved, Content: "-b"},
		{Type: DiffLineAdded, Content: "+c"},
		{Type: DiffLineAdded, Content: "+d"},
	}
	pairs := pairChangedLines(lines)
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
}

func TestPairChangedLines_PureAdds(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffLineContext, Content: " ctx"},
		{Type: DiffLineAdded, Content: "+new"},
	}
	pairs := pairChangedLines(lines)
	if len(pairs) != 0 {
		t.Errorf("pure adds should produce 0 pairs, got %d", len(pairs))
	}
}

func TestPairChangedLines_Unbalanced(t *testing.T) {
	lines := []DiffLine{
		{Type: DiffLineRemoved, Content: "-a"},
		{Type: DiffLineRemoved, Content: "-b"},
		{Type: DiffLineAdded, Content: "+c"},
	}
	pairs := pairChangedLines(lines)
	// Should pair -a with +c, leaving -b unpaired.
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair for unbalanced, got %d", len(pairs))
	}
}

func TestComputeEmphasis_SingleWordChange(t *testing.T) {
	remEmph, addEmph := computeEmphasis(
		"-        return nil, ErrExpired",
		"+        return newToken, nil",
	)
	if len(remEmph) == 0 {
		t.Error("expected emphasis ranges on removed line")
	}
	if len(addEmph) == 0 {
		t.Error("expected emphasis ranges on added line")
	}
}

func TestComputeEmphasis_IdenticalContent(t *testing.T) {
	remEmph, addEmph := computeEmphasis("-same content", "+same content")
	if len(remEmph) != 0 || len(addEmph) != 0 {
		t.Errorf("identical content should have no emphasis, got rem=%d add=%d", len(remEmph), len(addEmph))
	}
}

func TestAnnotateEmphasis_Integration(t *testing.T) {
	lines := parseDiff("diff --git a/f b/f\n--- a/f\n+++ b/f\n@@ -1,2 +1,2 @@\n context\n-old value\n+new value")
	annotateEmphasis(lines)

	var hasEmphasis bool
	for _, l := range lines {
		if len(l.Emphasis) > 0 {
			hasEmphasis = true
			break
		}
	}
	if !hasEmphasis {
		t.Error("annotateEmphasis should produce emphasis on paired -/+ lines")
	}
}

func TestMergeRanges(t *testing.T) {
	tests := []struct {
		name  string
		input []CharRange
		want  int
	}{
		{"empty", nil, 0},
		{"single", []CharRange{{0, 5}}, 1},
		{"adjacent", []CharRange{{0, 3}, {3, 6}}, 1},
		{"gap", []CharRange{{0, 3}, {5, 8}}, 2},
		{"overlap", []CharRange{{0, 5}, {3, 8}}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := mergeRanges(tt.input)
			if len(merged) != tt.want {
				t.Errorf("mergeRanges: got %d ranges, want %d", len(merged), tt.want)
			}
		})
	}
}

func TestStripDiffPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+added", "added"},
		{"-removed", "removed"},
		{" context", " context"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripDiffPrefix(tt.input)
		if got != tt.want {
			t.Errorf("stripDiffPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────

func tokenTexts(tokens []token) []string {
	texts := make([]string, len(tokens))
	for i, t := range tokens {
		texts[i] = t.text
	}
	return texts
}

func assertStrSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("length mismatch: got %d, want %d\ngot:  %v\nwant: %v", len(got), len(want), got, want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
