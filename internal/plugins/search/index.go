package search

import (
	"context"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// Scope defines which fields to search.
type Scope int

const (
	ScopeAll      Scope = iota // Search everything
	ScopeMessages              // Stash messages only
	ScopeFiles                 // File names only
	ScopeDiffs                 // Diff content only
	ScopeBranch                // Branch names only
)

// IndexEntry represents one searchable item tied to a stash.
type IndexEntry struct {
	StashIndex int
	StashSHA   string
	Message    string
	Branch     string
	FileName   string
	DiffLine   string
	LineNum    int
	Scope      Scope
}

// SearchResult represents a match result with context.
type SearchResult struct {
	StashIndex     int
	StashSHA       string
	StashMessage   string
	MatchText      string
	MatchScope     Scope
	FileName       string
	LineNum        int
	Score          int
	MatchedIndexes []int
}

// IndexReadyMsg is sent when the index is fully built.
type IndexReadyMsg struct {
	EntryCount int
}

// Index holds the searchable data for all stashes.
type Index struct {
	mu      sync.RWMutex
	entries []IndexEntry
	ready   bool
	partial bool
}

// NewIndex creates an empty search index.
func NewIndex() *Index {
	return &Index{}
}

// IsReady returns true if the index is fully built.
func (idx *Index) IsReady() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.ready
}

// HasPartialResults returns true if some results are available.
func (idx *Index) HasPartialResults() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.partial || idx.ready
}

// EntryCount returns the number of indexed entries.
func (idx *Index) EntryCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// addEntries appends entries to the index (thread-safe).
func (idx *Index) addEntries(entries []IndexEntry) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.entries = append(idx.entries, entries...)
	idx.partial = true
}

// markReady marks the index as fully built.
func (idx *Index) markReady() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.ready = true
}

// Reset clears the index for rebuilding.
func (idx *Index) Reset() {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.entries = nil
	idx.ready = false
	idx.partial = false
}

// Search performs fuzzy search across the index filtered by scope.
// Returns results sorted by score (descending). Deduplicates by stash SHA + scope
// so that a stash with multiple diff line matches collapses to the best one.
func (idx *Index) Search(query string, scope Scope) []SearchResult {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(query) == 0 || len(idx.entries) == 0 {
		return nil
	}

	// Collect source strings for fuzzy matching, filtered by scope.
	var filtered []IndexEntry
	var sources []string
	for _, e := range idx.entries {
		if scope != ScopeAll && e.Scope != scope {
			continue
		}
		filtered = append(filtered, e)
		switch e.Scope {
		case ScopeMessages:
			sources = append(sources, e.Message)
		case ScopeFiles:
			sources = append(sources, e.FileName)
		case ScopeDiffs:
			sources = append(sources, e.DiffLine)
		case ScopeBranch:
			sources = append(sources, e.Branch)
		default:
			sources = append(sources, e.Message)
		}
	}

	if len(sources) == 0 {
		return nil
	}

	matches := fuzzy.Find(query, sources)

	// Convert to SearchResults, deduplicate by stash SHA + scope.
	seen := make(map[string]int) // key: "sha:scope" -> index in results
	var results []SearchResult
	for _, m := range matches {
		entry := filtered[m.Index]
		key := entry.StashSHA + ":" + strconv.Itoa(int(entry.Scope))
		if existingIdx, ok := seen[key]; ok {
			if m.Score > results[existingIdx].Score {
				results[existingIdx] = SearchResult{
					StashIndex:     entry.StashIndex,
					StashSHA:       entry.StashSHA,
					StashMessage:   entry.Message,
					MatchText:      m.Str,
					MatchScope:     entry.Scope,
					FileName:       entry.FileName,
					LineNum:        entry.LineNum,
					Score:          m.Score,
					MatchedIndexes: m.MatchedIndexes,
				}
			}
			continue
		}
		seen[key] = len(results)
		results = append(results, SearchResult{
			StashIndex:     entry.StashIndex,
			StashSHA:       entry.StashSHA,
			StashMessage:   entry.Message,
			MatchText:      m.Str,
			MatchScope:     entry.Scope,
			FileName:       entry.FileName,
			LineNum:        entry.LineNum,
			Score:          m.Score,
			MatchedIndexes: m.MatchedIndexes,
		})
	}

	return results
}

// BuildIndexCmd returns a tea.Cmd that builds the search index asynchronously.
// It indexes stash messages, branch names, file names, and diff content.
// Sends IndexReadyMsg when done.
func BuildIndexCmd(stashes []plugin.Stash, cache plugin.StashCache, idx *Index) tea.Cmd {
	return func() tea.Msg {
		idx.Reset()

		// Phase 1: Index messages and branches (instant, no git calls).
		var messageEntries []IndexEntry
		for _, s := range stashes {
			messageEntries = append(messageEntries, IndexEntry{
				StashIndex: s.Index,
				StashSHA:   s.SHA,
				Message:    s.Message,
				Branch:     s.Branch,
				Scope:      ScopeMessages,
			})
			if s.Branch != "" {
				messageEntries = append(messageEntries, IndexEntry{
					StashIndex: s.Index,
					StashSHA:   s.SHA,
					Message:    s.Message,
					Branch:     s.Branch,
					Scope:      ScopeBranch,
				})
			}
		}
		idx.addEntries(messageEntries)

		// Phase 2: Index files and diffs (requires git calls, done per-stash).
		ctx := context.Background()
		for _, s := range stashes {
			diff, err := cache.Diff(ctx, s.SHA)
			if err != nil {
				continue
			}

			fileEntries, diffEntries := ParseDiffForIndex(s, diff)
			idx.addEntries(fileEntries)
			idx.addEntries(diffEntries)
		}

		idx.markReady()
		return IndexReadyMsg{EntryCount: idx.EntryCount()}
	}
}

// ParseDiffForIndex extracts file names and diff lines from a unified diff string.
func ParseDiffForIndex(stash plugin.Stash, diff string) (fileEntries, diffEntries []IndexEntry) {
	lines := strings.Split(diff, "\n")
	currentFile := ""
	lineNum := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			// Extract file name: "diff --git a/foo.go b/foo.go" -> "foo.go"
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				currentFile = parts[1]
				fileEntries = append(fileEntries, IndexEntry{
					StashIndex: stash.Index,
					StashSHA:   stash.SHA,
					Message:    stash.Message,
					Branch:     stash.Branch,
					FileName:   currentFile,
					Scope:      ScopeFiles,
				})
			}
			continue
		}
		if strings.HasPrefix(line, "@@") {
			// Parse hunk header for line numbers: @@ -start,count +start,count @@
			if _, after, ok := strings.Cut(line, "+"); ok {
				var numBuf [20]byte
				n := 0
				for _, c := range after {
					if c >= '0' && c <= '9' && n < len(numBuf) {
						numBuf[n] = byte(c)
						n++
					} else {
						break
					}
				}
				if parsed, err := strconv.Atoi(string(numBuf[:n])); err == nil {
					lineNum = parsed
				}
			}
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			continue
		}
		if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") || strings.HasPrefix(line, "similarity") ||
			strings.HasPrefix(line, "rename ") {
			continue
		}
		// Content line (added, removed, or context).
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 0 && currentFile != "" {
			diffEntries = append(diffEntries, IndexEntry{
				StashIndex: stash.Index,
				StashSHA:   stash.SHA,
				Message:    stash.Message,
				Branch:     stash.Branch,
				FileName:   currentFile,
				DiffLine:   trimmed,
				LineNum:    lineNum,
				Scope:      ScopeDiffs,
			})
		}
		if !strings.HasPrefix(line, "-") {
			lineNum++
		}
	}
	return
}

// ─── Test helpers ───────────────────────────────────────────

// AddEntriesForTest exposes addEntries for testing.
func (idx *Index) AddEntriesForTest(entries []IndexEntry) {
	idx.addEntries(entries)
}

// MarkReadyForTest exposes markReady for testing.
func (idx *Index) MarkReadyForTest() {
	idx.markReady()
}
