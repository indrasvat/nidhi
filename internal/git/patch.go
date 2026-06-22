package git

import (
	"fmt"
	"strings"
)

// patch.go implements the diff model and "patch surgery" that powers the
// interactive partial-stash picker. Given a unified diff (from `git diff
// HEAD`), it parses files → hunks → lines, lets the caller mark individual
// added/removed lines as Selected, and synthesizes a new, valid unified diff
// containing ONLY the selected changes — with hunk headers recomputed so the
// result applies cleanly via `git apply --cached`.
//
// The transformation mirrors what `git add -p` does when you pick a subset of
// a hunk:
//   - selected added line   → kept as "+"
//   - unselected added line → dropped entirely
//   - selected removed line → kept as "-"
//   - unselected removed   → converted to a context " " line (it stays)
//   - context line         → kept as context

// LineKind classifies a line within a hunk body.
type LineKind int

const (
	// LineContext is an unchanged context line (" " prefix).
	LineContext LineKind = iota
	// LineAdded is an added line ("+" prefix).
	LineAdded
	// LineRemoved is a removed line ("-" prefix).
	LineRemoved
)

// PatchLine is a single line within a hunk body.
type PatchLine struct {
	Kind LineKind
	// Text is the line content WITHOUT its leading +/-/space prefix.
	Text string
	// Selected indicates the line is included in the partial stash.
	// Only meaningful for LineAdded / LineRemoved.
	Selected bool
	// NoNewline is true when this line was followed by the
	// "\ No newline at end of file" marker.
	NoNewline bool
}

// Hunk is one contiguous change region within a file.
type Hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	// Section is the trailing text after the closing "@@" (function context).
	Section string
	Lines   []PatchLine
}

// FileDiff is the diff for a single file.
type FileDiff struct {
	OldPath string
	NewPath string
	// Header holds the raw pre-hunk lines verbatim (diff --git, index,
	// mode lines, ---/+++), re-emitted unchanged when the file contributes.
	Header []string
	Hunks  []Hunk

	IsBinary bool
	IsNew    bool
	IsDelete bool
	IsRename bool
}

// WholeFileOnly reports whether a file must be selected all-or-nothing
// (new/deleted/binary/rename files have no safe line-level granularity in v1).
func (f *FileDiff) WholeFileOnly() bool {
	return f.IsBinary || f.IsNew || f.IsDelete || f.IsRename
}

// PatchSet is a parsed unified diff.
type PatchSet struct {
	Files []FileDiff
}

// SelState describes the aggregate selection of a file or hunk.
type SelState int

const (
	// SelNone means nothing is selected.
	SelNone SelState = iota
	// SelPartial means some but not all changes are selected.
	SelPartial
	// SelAll means every change is selected.
	SelAll
)

// Stats summarizes the currently-selected changes for the live tally.
type Stats struct {
	Added   int
	Removed int
	Hunks   int
	Files   int
}

// ─── Parsing ────────────────────────────────────────────────

// ParsePatch parses a unified diff produced by `git diff` into a PatchSet.
func ParsePatch(diff string) (*PatchSet, error) {
	ps := &PatchSet{}
	if strings.TrimSpace(diff) == "" {
		return ps, nil
	}

	lines := strings.Split(diff, "\n")
	// Drop a single trailing empty element from a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var cur *FileDiff
	var curHunk *Hunk

	flushHunk := func() {
		if cur != nil && curHunk != nil {
			cur.Hunks = append(cur.Hunks, *curHunk)
			curHunk = nil
		}
	}
	flushFile := func() {
		flushHunk()
		if cur != nil {
			ps.Files = append(ps.Files, *cur)
			cur = nil
		}
	}

	for _, raw := range lines {
		switch {
		case strings.HasPrefix(raw, "diff --git "):
			flushFile()
			cur = &FileDiff{Header: []string{raw}}
			cur.OldPath, cur.NewPath = parseDiffGitPaths(raw)

		case cur == nil:
			// Skip any preamble before the first "diff --git" header.
			continue

		case strings.HasPrefix(raw, "new file mode"):
			cur.IsNew = true
			cur.Header = append(cur.Header, raw)

		case strings.HasPrefix(raw, "deleted file mode"):
			cur.IsDelete = true
			cur.Header = append(cur.Header, raw)

		case strings.HasPrefix(raw, "rename from "), strings.HasPrefix(raw, "rename to "),
			strings.HasPrefix(raw, "copy from "), strings.HasPrefix(raw, "copy to "),
			strings.HasPrefix(raw, "similarity index"), strings.HasPrefix(raw, "dissimilarity index"):
			cur.IsRename = true
			cur.Header = append(cur.Header, raw)

		case strings.HasPrefix(raw, "Binary files "):
			cur.IsBinary = true
			cur.Header = append(cur.Header, raw)

		case strings.HasPrefix(raw, "index "),
			strings.HasPrefix(raw, "old mode "), strings.HasPrefix(raw, "new mode "):
			cur.Header = append(cur.Header, raw)

		case strings.HasPrefix(raw, "--- "):
			cur.Header = append(cur.Header, raw)

		case strings.HasPrefix(raw, "+++ "):
			cur.Header = append(cur.Header, raw)

		case strings.HasPrefix(raw, "@@"):
			flushHunk()
			oldStart, oldCount, newStart, newCount, section := parseHunkRange(raw)
			curHunk = &Hunk{
				OldStart: oldStart, OldCount: oldCount,
				NewStart: newStart, NewCount: newCount,
				Section: section,
			}

		case strings.HasPrefix(raw, "\\"):
			// "\ No newline at end of file" — attach to the previous line.
			if curHunk != nil && len(curHunk.Lines) > 0 {
				curHunk.Lines[len(curHunk.Lines)-1].NoNewline = true
			}

		case curHunk != nil && strings.HasPrefix(raw, "+"):
			curHunk.Lines = append(curHunk.Lines, PatchLine{Kind: LineAdded, Text: raw[1:]})

		case curHunk != nil && strings.HasPrefix(raw, "-"):
			curHunk.Lines = append(curHunk.Lines, PatchLine{Kind: LineRemoved, Text: raw[1:]})

		case curHunk != nil && strings.HasPrefix(raw, " "):
			curHunk.Lines = append(curHunk.Lines, PatchLine{Kind: LineContext, Text: raw[1:]})

		case curHunk != nil && raw == "":
			// A truly empty line inside a hunk body is an empty context line.
			curHunk.Lines = append(curHunk.Lines, PatchLine{Kind: LineContext, Text: ""})
		}
	}
	flushFile()

	return ps, nil
}

func parseDiffGitPaths(header string) (oldPath, newPath string) {
	// "diff --git a/old b/new"
	rest := strings.TrimPrefix(header, "diff --git ")
	// Split on " b/" which separates the two paths in the common case.
	if idx := strings.Index(rest, " b/"); idx >= 0 {
		oldPath = strings.TrimPrefix(rest[:idx], "a/")
		newPath = rest[idx+len(" b/"):]
		return oldPath, newPath
	}
	fields := strings.Fields(rest)
	if len(fields) == 2 {
		return strings.TrimPrefix(fields[0], "a/"), strings.TrimPrefix(fields[1], "b/")
	}
	return "", ""
}

func parseHunkRange(header string) (oldStart, oldCount, newStart, newCount int, section string) {
	// "@@ -a,b +c,d @@ section"
	body := strings.TrimPrefix(header, "@@")
	end := strings.Index(body, "@@")
	rangePart := body
	if end >= 0 {
		rangePart = body[:end]
		section = strings.TrimPrefix(body[end+2:], " ")
	}
	oldCount, newCount = 1, 1 // default when count omitted
	for field := range strings.FieldsSeq(rangePart) {
		switch {
		case strings.HasPrefix(field, "-"):
			oldStart, oldCount = parseStartCount(field[1:])
		case strings.HasPrefix(field, "+"):
			newStart, newCount = parseStartCount(field[1:])
		}
	}
	return oldStart, oldCount, newStart, newCount, section
}

func parseStartCount(s string) (start, count int) {
	count = 1
	if idx := strings.Index(s, ","); idx >= 0 {
		_, _ = fmt.Sscanf(s[:idx], "%d", &start)
		_, _ = fmt.Sscanf(s[idx+1:], "%d", &count)
		return start, count
	}
	_, _ = fmt.Sscanf(s, "%d", &start)
	return start, count
}

// ─── Selection helpers ──────────────────────────────────────

// SelectAll sets the selection state of every changeable line in the set.
func (ps *PatchSet) SelectAll(selected bool) {
	for i := range ps.Files {
		ps.Files[i].SetSelected(selected)
	}
}

// SetSelected sets every added/removed line in the file.
func (f *FileDiff) SetSelected(selected bool) {
	for i := range f.Hunks {
		f.Hunks[i].SetSelected(selected)
	}
}

// SetSelected sets every added/removed line in the hunk.
func (h *Hunk) SetSelected(selected bool) {
	for i := range h.Lines {
		if h.Lines[i].Kind != LineContext {
			h.Lines[i].Selected = selected
		}
	}
}

// SelectionState reports the aggregate selection state of the hunk.
func (h *Hunk) SelectionState() SelState {
	total, sel := 0, 0
	for _, ln := range h.Lines {
		if ln.Kind == LineContext {
			continue
		}
		total++
		if ln.Selected {
			sel++
		}
	}
	return aggregate(total, sel)
}

// SelectionState reports the aggregate selection state of the file.
func (f *FileDiff) SelectionState() SelState {
	total, sel := 0, 0
	for hi := range f.Hunks {
		for _, ln := range f.Hunks[hi].Lines {
			if ln.Kind == LineContext {
				continue
			}
			total++
			if ln.Selected {
				sel++
			}
		}
	}
	return aggregate(total, sel)
}

func aggregate(total, sel int) SelState {
	switch {
	case total == 0 || sel == 0:
		return SelNone
	case sel == total:
		return SelAll
	default:
		return SelPartial
	}
}

// SelectedStats summarizes the selected changes for the live tally.
func (ps *PatchSet) SelectedStats() Stats {
	var st Stats
	for fi := range ps.Files {
		fileHasSel := false
		for hi := range ps.Files[fi].Hunks {
			hunkHasSel := false
			for _, ln := range ps.Files[fi].Hunks[hi].Lines {
				if ln.Kind == LineContext || !ln.Selected {
					continue
				}
				switch ln.Kind {
				case LineAdded:
					st.Added++
				case LineRemoved:
					st.Removed++
				}
				hunkHasSel = true
			}
			if hunkHasSel {
				st.Hunks++
				fileHasSel = true
			}
		}
		if fileHasSel {
			st.Files++
		}
	}
	return st
}

// HasSelection reports whether anything at all is selected.
func (ps *PatchSet) HasSelection() bool {
	st := ps.SelectedStats()
	return st.Added > 0 || st.Removed > 0
}

// ─── Patch surgery ──────────────────────────────────────────

// BuildSelectedPatch synthesizes a unified diff containing only the selected
// changes, with hunk headers recomputed. Returns "" if nothing is selected.
func (ps *PatchSet) BuildSelectedPatch() string {
	var b strings.Builder
	for fi := range ps.Files {
		f := &ps.Files[fi]
		// Binary files cannot be partially stashed from a text diff; skip.
		if f.IsBinary {
			continue
		}
		fileBody, any := buildFileBody(f)
		if !any {
			continue
		}
		for _, hl := range f.Header {
			b.WriteString(hl)
			b.WriteByte('\n')
		}
		b.WriteString(fileBody)
	}
	return b.String()
}

// buildFileBody renders the selected hunks of one file. The bool result
// reports whether the file contributes anything.
func buildFileBody(f *FileDiff) (string, bool) {
	var b strings.Builder
	delta := 0
	any := false

	for hi := range f.Hunks {
		h := &f.Hunks[hi]
		body, oldCount, newCount, hunkHasChange := buildHunkBody(h)
		if !hunkHasChange {
			// Pure-context hunk after surgery contributes nothing and does
			// not shift later hunks (oldCount == newCount).
			continue
		}
		// New-side start = old start + cumulative emitted delta. For a pure
		// insertion hunk (oldCount == 0, git's "-X,0" convention) the new
		// content begins at X+1, so bump the base by one.
		base := h.OldStart
		if oldCount == 0 {
			base++
		}
		newStart := base + delta
		fmt.Fprintf(&b, "@@ -%s +%s @@", formatRange(h.OldStart, oldCount), formatRange(newStart, newCount))
		if h.Section != "" {
			b.WriteByte(' ')
			b.WriteString(h.Section)
		}
		b.WriteByte('\n')
		b.WriteString(body)
		delta += newCount - oldCount
		any = true
	}
	return b.String(), any
}

// buildHunkBody renders the transformed lines of a hunk and returns the
// recomputed old/new counts plus whether the hunk has any +/- after surgery.
func buildHunkBody(h *Hunk) (body string, oldCount, newCount int, hasChange bool) {
	var b strings.Builder
	for _, ln := range h.Lines {
		switch ln.Kind {
		case LineContext:
			writeBodyLine(&b, ' ', ln)
			oldCount++
			newCount++
		case LineAdded:
			if ln.Selected {
				writeBodyLine(&b, '+', ln)
				newCount++
				hasChange = true
			}
			// Unselected added lines are dropped entirely.
		case LineRemoved:
			if ln.Selected {
				writeBodyLine(&b, '-', ln)
				oldCount++
				hasChange = true
			} else {
				// Unselected removal stays as context.
				writeBodyLine(&b, ' ', ln)
				oldCount++
				newCount++
			}
		}
	}
	return b.String(), oldCount, newCount, hasChange
}

func writeBodyLine(b *strings.Builder, prefix byte, ln PatchLine) {
	b.WriteByte(prefix)
	b.WriteString(ln.Text)
	b.WriteByte('\n')
	if ln.NoNewline {
		b.WriteString("\\ No newline at end of file\n")
	}
}

// formatRange renders the "start,count" portion of a hunk header. Git omits
// the count when it is 1, but always-emitting it is valid and accepted by
// `git apply`; we emit it explicitly except for the count==1 case to match
// git's canonical output (which keeps round-trip tests exact).
func formatRange(start, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}
