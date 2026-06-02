package components

import (
	"unicode"
)

// CharRange marks a byte range [Start, End) within a string for emphasis.
type CharRange struct {
	Start int
	End   int
}

// ─── Tokenizer ──────────────────────────────────────────────

// token represents a word/punctuation/whitespace token with its byte offset.
type token struct {
	text  string
	start int // byte offset in original string
}

// tokenize splits a string into tokens at word boundaries.
// Punctuation and whitespace characters become individual tokens.
func tokenize(s string) []token {
	var tokens []token
	runes := []rune(s)
	i := 0
	bytePos := 0

	for i < len(runes) {
		r := runes[i]
		start := bytePos

		switch {
		case unicode.IsSpace(r):
			// Whitespace: each character is its own token.
			bytePos += len(string(r))
			tokens = append(tokens, token{text: string(r), start: start})
			i++

		case isPunct(r):
			// Punctuation: each character is its own token.
			bytePos += len(string(r))
			tokens = append(tokens, token{text: string(r), start: start})
			i++

		default:
			// Word: consume until whitespace or punctuation.
			j := i + 1
			for j < len(runes) && !unicode.IsSpace(runes[j]) && !isPunct(runes[j]) {
				j++
			}
			word := string(runes[i:j])
			tokens = append(tokens, token{text: word, start: start})
			bytePos += len(word)
			i = j
		}
	}
	return tokens
}

// isPunct returns true for characters that should be split as individual tokens.
func isPunct(r rune) bool {
	switch r {
	case '(', ')', '{', '}', '[', ']', '<', '>',
		',', '.', ';', ':', '!', '?', '"', '\'', '`',
		'@', '#', '$', '%', '^', '&', '*',
		'+', '-', '=', '/', '\\', '|', '~':
		return true
	}
	return false
}

// ─── Myers Diff (token-level) ───────────────────────────────

// editOp represents a diff operation.
type editOp int

const (
	editEqual  editOp = iota
	editInsert        // present in b, not in a
	editDelete        // present in a, not in b
)

// edit is a single diff operation referencing tokens.
type edit struct {
	op    editOp
	aIdx  int // index into a tokens (-1 for insert)
	bIdx  int // index into b tokens (-1 for delete)
}

// myersDiff computes the shortest edit script between two token slices
// using the Myers O(ND) algorithm. For the small sequences we operate on
// (typically <50 tokens per line), this is fast and simple.
func myersDiff(a, b []token) []edit {
	n := len(a)
	m := len(b)

	if n == 0 && m == 0 {
		return nil
	}
	if n == 0 {
		edits := make([]edit, m)
		for i := range m {
			edits[i] = edit{op: editInsert, aIdx: -1, bIdx: i}
		}
		return edits
	}
	if m == 0 {
		edits := make([]edit, n)
		for i := range n {
			edits[i] = edit{op: editDelete, aIdx: i, bIdx: -1}
		}
		return edits
	}

	// Myers algorithm.
	max := n + m
	// v[k+max] = furthest reaching x on diagonal k.
	v := make([]int, 2*max+1)
	// trace stores snapshots of v for backtracking.
	trace := make([][]int, 0, max+1)

	for d := range max + 1 {
		snapshot := make([]int, len(v))
		copy(snapshot, v)
		trace = append(trace, snapshot)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[k-1+max] < v[k+1+max]) {
				x = v[k+1+max] // move down
			} else {
				x = v[k-1+max] + 1 // move right
			}
			y := x - k

			// Follow diagonal (equal tokens).
			for x < n && y < m && a[x].text == b[y].text {
				x++
				y++
			}

			v[k+max] = x

			if x >= n && y >= m {
				// Backtrack to build edit script.
				return backtrack(trace, a, b, d, max)
			}
		}
	}

	// Fallback: should not reach here for valid inputs.
	return nil
}

// backtrack reconstructs the edit script from the Myers trace.
func backtrack(trace [][]int, a, b []token, d, max int) []edit {
	var edits []edit
	x := len(a)
	y := len(b)

	for di := d; di > 0; di-- {
		v := trace[di]
		k := x - y

		var prevK int
		if k == -di || (k != di && v[k-1+max] < v[k+1+max]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := v[prevK+max]
		prevY := prevX - prevK

		// Diagonal moves (equal tokens) — walk backwards.
		for x > prevX && y > prevY {
			x--
			y--
			edits = append(edits, edit{op: editEqual, aIdx: x, bIdx: y})
		}

		if di > 0 {
			if x == prevX {
				// Insert.
				y--
				edits = append(edits, edit{op: editInsert, aIdx: -1, bIdx: y})
			} else {
				// Delete.
				x--
				edits = append(edits, edit{op: editDelete, aIdx: x, bIdx: -1})
			}
		}
	}

	// Remaining diagonal at d=0.
	for x > 0 && y > 0 {
		x--
		y--
		edits = append(edits, edit{op: editEqual, aIdx: x, bIdx: y})
	}

	// Reverse to get forward order.
	for i, j := 0, len(edits)-1; i < j; i, j = i+1, j-1 {
		edits[i], edits[j] = edits[j], edits[i]
	}

	return edits
}

// ─── Line Pairing ───────────────────────────────────────────

// linePair pairs a removed line with an added line for word-level diffing.
type linePair struct {
	removedIdx int // index into []DiffLine
	addedIdx   int
}

// pairChangedLines walks the diff lines and pairs adjacent removed→added
// sequences 1:1. Unpaired lines (count mismatch) get no emphasis.
func pairChangedLines(lines []DiffLine) []linePair {
	var pairs []linePair
	i := 0
	for i < len(lines) {
		// Skip non-change lines (context, header, hunk).
		if lines[i].Type != DiffLineRemoved && lines[i].Type != DiffLineAdded {
			i++
			continue
		}

		// Find a contiguous block of removed lines.
		remStart := i
		for i < len(lines) && lines[i].Type == DiffLineRemoved {
			i++
		}
		remEnd := i

		// Find a contiguous block of added lines immediately after.
		addStart := i
		for i < len(lines) && lines[i].Type == DiffLineAdded {
			i++
		}
		addEnd := i

		remCount := remEnd - remStart
		addCount := addEnd - addStart

		if remCount == 0 || addCount == 0 {
			// Pure adds or pure deletes — no pairing.
			continue
		}

		// Pair 1:1 up to the minimum count.
		pairCount := min(remCount, addCount)
		for j := range pairCount {
			pairs = append(pairs, linePair{
				removedIdx: remStart + j,
				addedIdx:   addStart + j,
			})
		}
	}
	return pairs
}

// ─── Emphasis Annotation ────────────────────────────────────

// computeEmphasis computes emphasis ranges for a removed/added line pair.
// It tokenizes both lines (skipping the leading +/- prefix), diffs them,
// and returns the byte ranges within each line that are changed.
func computeEmphasis(removedContent, addedContent string) (removedEmph, addedEmph []CharRange) {
	// Strip the leading +/- diff prefix for comparison.
	remText := stripDiffPrefix(removedContent)
	addText := stripDiffPrefix(addedContent)

	if remText == addText {
		return nil, nil // identical after prefix strip
	}

	remTokens := tokenize(remText)
	addTokens := tokenize(addText)

	edits := myersDiff(remTokens, addTokens)

	// The prefix offset: the diff prefix character(s) shift all byte positions.
	remOffset := len(removedContent) - len(remText)
	addOffset := len(addedContent) - len(addText)

	for _, e := range edits {
		switch e.op {
		case editDelete:
			tok := remTokens[e.aIdx]
			removedEmph = append(removedEmph, CharRange{
				Start: tok.start + remOffset,
				End:   tok.start + remOffset + len(tok.text),
			})
		case editInsert:
			tok := addTokens[e.bIdx]
			addedEmph = append(addedEmph, CharRange{
				Start: tok.start + addOffset,
				End:   tok.start + addOffset + len(tok.text),
			})
		}
	}

	// Merge adjacent ranges (tokens that are next to each other).
	removedEmph = mergeRanges(removedEmph)
	addedEmph = mergeRanges(addedEmph)

	return removedEmph, addedEmph
}

// stripDiffPrefix removes the leading +/- character from a diff line.
func stripDiffPrefix(s string) string {
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		return s[1:]
	}
	return s
}

// mergeRanges merges adjacent or overlapping CharRanges.
func mergeRanges(ranges []CharRange) []CharRange {
	if len(ranges) <= 1 {
		return ranges
	}
	merged := []CharRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End {
			if r.End > last.End {
				last.End = r.End
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// annotateEmphasis walks the parsed diff lines, pairs adjacent removed/added
// lines, computes word-level diffs, and fills in the Emphasis field.
func annotateEmphasis(lines []DiffLine) {
	pairs := pairChangedLines(lines)
	for _, p := range pairs {
		remEmph, addEmph := computeEmphasis(
			lines[p.removedIdx].Content,
			lines[p.addedIdx].Content,
		)
		lines[p.removedIdx].Emphasis = remEmph
		lines[p.addedIdx].Emphasis = addEmph
	}
}
