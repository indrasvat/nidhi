# Task 004: Stash Parser and Cache

## Status: TODO

## Depends On
- 001 (Git Runner and Version Detection) -- `GitRunner` interface, `DefaultRunner` implementation

## Parallelizable With
- 002 (Config Loading)
- 003 (Agni Theme and Icons)

## Problem
The stash list is the heart of nidhi. Every screen depends on parsed stash data -- the LIST view renders it, PREVIEW loads diffs for it, DETAIL inspects files in it, search indexes it, and CRUD operations mutate it. Without a stash parser, nidhi has no data to display. Without a cache, every cursor movement would re-invoke git, making the TUI sluggish. The cache must be keyed by SHA (not index) because stash indices shift when entries are dropped or reordered.

## PRD Reference
- Section 5.1 (Core Stash Operations) -- `git stash list`, `git stash show` signatures
- Section 6.1 FR-01.3 (Stale Detection) -- staleness threshold, STALE badge
- Section 6.1 FR-01.4 (Auto-generated messages) -- replace "WIP on branch: sha msg" with "3 files: +42/-17 in src/auth"
- Section 8.3 (Core Types) -- `Stash` struct with all 12 fields, `StashCache` interface with `List`, `Diff`, `Invalidate`
- Section 14.1 (Startup Sequence) -- parse stash list synchronously, defer diff caching
- Section 14.3 (Caching Strategy) -- LRU cache for diffs, configurable size (default 50), invalidation after mutations

## Files to Create
- `internal/git/stash.go` -- `Stash` struct (PRD section 8.3), `ParseStashList()`, auto-message generation, staleness calculation
- `internal/git/cache.go` -- `StashCache` interface + `DefaultStashCache` implementation with LRU diff cache keyed by SHA
- `internal/git/stash_test.go` -- parse various formats, edge cases, auto-message, staleness
- `internal/git/cache_test.go` -- LRU eviction, cache invalidation, SHA keying stability, concurrent access

## Execution Steps

### Step 1: Create `internal/git/stash.go`

```go
package git

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Stash represents a single stash entry.
// Fields match PRD section 8.3.
type Stash struct {
	Index        int       // stash@{Index}
	SHA          string    // Full commit SHA
	ShortSHA     string    // Abbreviated SHA (first 7 chars)
	Message      string    // User message or auto-generated
	RawMessage   string    // Original message from git
	Branch       string    // Branch where stash was created
	Date         time.Time // Creation timestamp
	FileCount    int       // Number of files changed
	Insertions   int       // Lines added
	Deletions    int       // Lines deleted
	IsStale      bool      // Older than staleness threshold
	HasUntracked bool      // Includes untracked files (stash created with -u or -a)
}

// stashListFormat is the custom format string for `git stash list`.
// Fields separated by \x00 (null byte) for reliable parsing.
// %H = full SHA, %h = short SHA, %gs = stash message (reflog subject),
// %aI = author date ISO 8601
const stashListFormat = "%H%x00%h%x00%gs%x00%aI"

// wipPattern matches the default "WIP on <branch>: <sha> <msg>" format.
var wipPattern = regexp.MustCompile(`^WIP on (.+): ([0-9a-f]+) (.*)$`)

// onBranchPattern matches "On <branch>: <message>" format.
var onBranchPattern = regexp.MustCompile(`^On (.+): (.+)$`)

// ParseStashList parses the output of `git stash list --format=<format>`
// into a slice of Stash structs. staleThreshold is used to compute IsStale.
func ParseStashList(output string, staleThreshold time.Duration) []Stash {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	stashes := make([]Stash, 0, len(lines))
	now := time.Now()

	for i, line := range lines {
		s, err := parseStashLine(line, i, now, staleThreshold)
		if err != nil {
			// Skip malformed lines rather than failing entirely
			continue
		}
		stashes = append(stashes, s)
	}

	return stashes
}

// parseStashLine parses a single line from `git stash list --format=<format>`.
func parseStashLine(line string, index int, now time.Time, staleThreshold time.Duration) (Stash, error) {
	parts := strings.SplitN(line, "\x00", 4)
	if len(parts) != 4 {
		return Stash{}, fmt.Errorf("expected 4 fields, got %d in line: %q", len(parts), line)
	}

	sha := parts[0]
	shortSHA := parts[1]
	rawMessage := parts[2]
	dateStr := parts[3]

	date, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// Try alternative formats
		date, err = time.Parse("2006-01-02T15:04:05-07:00", dateStr)
		if err != nil {
			date = time.Time{} // zero time if unparseable
		}
	}

	branch := extractBranch(rawMessage)
	message := generateMessage(rawMessage)

	s := Stash{
		Index:      index,
		SHA:        sha,
		ShortSHA:   shortSHA,
		Message:    message,
		RawMessage: rawMessage,
		Branch:     branch,
		Date:       date,
	}

	// Compute staleness
	if !date.IsZero() && staleThreshold > 0 {
		s.IsStale = now.Sub(date) > staleThreshold
	}

	return s, nil
}

// extractBranch extracts the branch name from a stash message.
// Handles both "WIP on <branch>: ..." and "On <branch>: ..." formats.
func extractBranch(rawMessage string) string {
	if m := wipPattern.FindStringSubmatch(rawMessage); m != nil {
		return m[1]
	}
	if m := onBranchPattern.FindStringSubmatch(rawMessage); m != nil {
		return m[1]
	}
	return ""
}

// generateMessage produces a user-friendly message from the raw stash message.
// For default "WIP on branch: sha msg" messages, it returns the underlying
// commit message portion. For user-provided messages ("On branch: message"),
// it returns the message as-is. This is the basic version; full auto-message
// generation (FR-01.4) with file scope summaries requires diff data.
func generateMessage(rawMessage string) string {
	if m := wipPattern.FindStringSubmatch(rawMessage); m != nil {
		// Default WIP message -- return the commit message portion
		commitMsg := strings.TrimSpace(m[3])
		if commitMsg == "" {
			return "WIP (no message)"
		}
		return commitMsg
	}
	if m := onBranchPattern.FindStringSubmatch(rawMessage); m != nil {
		return m[2]
	}
	return rawMessage
}

// GenerateAutoMessage creates a summary message from diff stats.
// Implements FR-01.4: "3 files: +42/-17 in src/auth, pkg/db".
// This is called when the stash has a default WIP message and diff stats
// are available.
func GenerateAutoMessage(fileCount, insertions, deletions int, topDirs []string) string {
	var b strings.Builder

	// File count
	if fileCount == 1 {
		b.WriteString("1 file")
	} else {
		fmt.Fprintf(&b, "%d files", fileCount)
	}

	// Diff stat
	fmt.Fprintf(&b, ": +%d/-%d", insertions, deletions)

	// Top directories
	if len(topDirs) > 0 {
		b.WriteString(" in ")
		if len(topDirs) <= 3 {
			b.WriteString(strings.Join(topDirs, ", "))
		} else {
			b.WriteString(strings.Join(topDirs[:3], ", "))
			fmt.Fprintf(&b, " +%d more", len(topDirs)-3)
		}
	}

	return b.String()
}

// ListStashes runs `git stash list` with the custom format and parses results.
func ListStashes(ctx context.Context, runner GitRunner, staleThreshold time.Duration) ([]Stash, error) {
	output, err := runner.Run(ctx, "stash", "list", "--format="+stashListFormat)
	if err != nil {
		return nil, fmt.Errorf("listing stashes: %w", err)
	}

	stashes := ParseStashList(output, staleThreshold)

	// Enrich with diff stats (file count, insertions, deletions)
	for i := range stashes {
		if err := enrichDiffStats(ctx, runner, &stashes[i]); err != nil {
			// Non-fatal: stats are nice-to-have
			continue
		}
	}

	return stashes, nil
}

// enrichDiffStats populates FileCount, Insertions, Deletions, HasUntracked
// from `git stash show --stat --include-untracked`.
func enrichDiffStats(ctx context.Context, runner GitRunner, s *Stash) error {
	ref := fmt.Sprintf("stash@{%d}", s.Index)
	output, err := runner.Run(ctx, "stash", "show", "--numstat", ref)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		s.FileCount++
		if ins, err := strconv.Atoi(fields[0]); err == nil {
			s.Insertions += ins
		}
		if del, err := strconv.Atoi(fields[1]); err == nil {
			s.Deletions += del
		}
	}

	// Check for untracked files -- try `git stash show --include-untracked`
	// If the stash has untracked files, the output will include them.
	untrackedOut, err := runner.Run(ctx, "stash", "show", "--include-untracked", "--name-only", ref)
	if err == nil {
		// If --include-untracked output has more files than --numstat,
		// those extra files are untracked
		untrackedLines := strings.Split(strings.TrimSpace(untrackedOut), "\n")
		if len(untrackedLines) > s.FileCount {
			s.HasUntracked = true
		}
	}

	return nil
}
```

### Step 2: Create `internal/git/cache.go`

```go
package git

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StashCache provides cached access to stash data.
// See PRD section 8.3 and 14.3.
type StashCache interface {
	// List returns all stashes. Cached until Invalidate().
	List(ctx context.Context) ([]Stash, error)
	// Diff returns the diff for a stash by SHA. Lazily loaded, LRU cached.
	// Keyed by stash SHA (not index) since indices shift after drop/reorder.
	Diff(ctx context.Context, sha string) (string, error)
	// Invalidate clears the list cache. Called after mutations.
	Invalidate()
	// PreloadDiffs preloads diffs for the top N stashes in the background.
	PreloadDiffs(ctx context.Context, n int)
}

// DefaultStashCache implements StashCache with an LRU diff cache.
type DefaultStashCache struct {
	runner         GitRunner
	staleThreshold time.Duration
	maxDiffCache   int

	// List cache
	mu      sync.RWMutex
	stashes []Stash
	valid   bool

	// LRU diff cache keyed by SHA
	diffMu    sync.RWMutex
	diffCache map[string]*diffEntry
	diffOrder []string // Most recently used SHAs (front = most recent)
}

// diffEntry holds a cached diff result.
type diffEntry struct {
	content string
	err     error
}

// NewDefaultStashCache creates a new DefaultStashCache.
//
// Parameters:
//   - runner: GitRunner for executing git commands
//   - staleThreshold: duration after which stashes are considered stale
//   - maxDiffCache: maximum number of diffs to keep in the LRU cache
func NewDefaultStashCache(runner GitRunner, staleThreshold time.Duration, maxDiffCache int) *DefaultStashCache {
	if maxDiffCache <= 0 {
		maxDiffCache = 50
	}
	return &DefaultStashCache{
		runner:         runner,
		staleThreshold: staleThreshold,
		maxDiffCache:   maxDiffCache,
		diffCache:      make(map[string]*diffEntry),
		diffOrder:      make([]string, 0, maxDiffCache),
	}
}

// Verify DefaultStashCache implements StashCache at compile time.
var _ StashCache = (*DefaultStashCache)(nil)

// List returns all stashes. Results are cached until Invalidate() is called.
func (c *DefaultStashCache) List(ctx context.Context) ([]Stash, error) {
	c.mu.RLock()
	if c.valid {
		stashes := c.stashes
		c.mu.RUnlock()
		return stashes, nil
	}
	c.mu.RUnlock()

	// Cache miss -- fetch fresh data
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.valid {
		return c.stashes, nil
	}

	stashes, err := ListStashes(ctx, c.runner, c.staleThreshold)
	if err != nil {
		return nil, err
	}

	c.stashes = stashes
	c.valid = true
	return stashes, nil
}

// Diff returns the diff for a stash identified by SHA.
// Results are LRU-cached with a configurable maximum size.
func (c *DefaultStashCache) Diff(ctx context.Context, sha string) (string, error) {
	// Check cache first
	c.diffMu.RLock()
	if entry, ok := c.diffCache[sha]; ok {
		c.diffMu.RUnlock()
		// Move to front of LRU (write lock needed)
		c.touchLRU(sha)
		return entry.content, entry.err
	}
	c.diffMu.RUnlock()

	// Cache miss -- fetch the diff
	// We need to find the stash ref for this SHA.
	// Since we key by SHA, we need to resolve which stash@{N} has this SHA.
	stashes, err := c.List(ctx)
	if err != nil {
		return "", fmt.Errorf("loading stash list for diff lookup: %w", err)
	}

	ref := ""
	for _, s := range stashes {
		if s.SHA == sha {
			ref = fmt.Sprintf("stash@{%d}", s.Index)
			break
		}
	}
	if ref == "" {
		return "", fmt.Errorf("stash with SHA %s not found in stash list", sha)
	}

	diff, diffErr := c.runner.Run(ctx, "stash", "show", "-p", "--include-untracked", ref)

	// Cache the result (even errors, to avoid re-fetching broken diffs)
	c.diffMu.Lock()
	c.diffCache[sha] = &diffEntry{content: diff, err: diffErr}
	c.diffOrder = append([]string{sha}, c.diffOrder...)
	c.evictLRU()
	c.diffMu.Unlock()

	return diff, diffErr
}

// Invalidate clears the stash list cache. Diff cache is preserved
// because diffs are keyed by SHA, which doesn't change when indices shift.
func (c *DefaultStashCache) Invalidate() {
	c.mu.Lock()
	c.stashes = nil
	c.valid = false
	c.mu.Unlock()
}

// InvalidateAll clears both the list cache and the diff cache.
// Used when a stash is dropped (its SHA is no longer valid).
func (c *DefaultStashCache) InvalidateAll() {
	c.Invalidate()

	c.diffMu.Lock()
	c.diffCache = make(map[string]*diffEntry)
	c.diffOrder = c.diffOrder[:0]
	c.diffMu.Unlock()
}

// InvalidateDiff removes a specific SHA from the diff cache.
func (c *DefaultStashCache) InvalidateDiff(sha string) {
	c.diffMu.Lock()
	delete(c.diffCache, sha)
	for i, s := range c.diffOrder {
		if s == sha {
			c.diffOrder = append(c.diffOrder[:i], c.diffOrder[i+1:]...)
			break
		}
	}
	c.diffMu.Unlock()
}

// PreloadDiffs preloads diffs for the first N stashes.
// This is called asynchronously after startup (PRD section 14.1).
func (c *DefaultStashCache) PreloadDiffs(ctx context.Context, n int) {
	stashes, err := c.List(ctx)
	if err != nil {
		return
	}

	limit := n
	if limit > len(stashes) {
		limit = len(stashes)
	}

	for i := 0; i < limit; i++ {
		if ctx.Err() != nil {
			return // Context cancelled, stop preloading
		}
		// Diff() handles caching internally
		_, _ = c.Diff(ctx, stashes[i].SHA)
	}
}

// DiffCacheSize returns the current number of cached diffs.
func (c *DefaultStashCache) DiffCacheSize() int {
	c.diffMu.RLock()
	defer c.diffMu.RUnlock()
	return len(c.diffCache)
}

// touchLRU moves a SHA to the front of the LRU order.
func (c *DefaultStashCache) touchLRU(sha string) {
	c.diffMu.Lock()
	defer c.diffMu.Unlock()

	for i, s := range c.diffOrder {
		if s == sha {
			// Remove from current position
			c.diffOrder = append(c.diffOrder[:i], c.diffOrder[i+1:]...)
			// Add to front
			c.diffOrder = append([]string{sha}, c.diffOrder...)
			return
		}
	}
}

// evictLRU removes the least recently used entries if cache exceeds max size.
// Must be called with diffMu held.
func (c *DefaultStashCache) evictLRU() {
	for len(c.diffOrder) > c.maxDiffCache {
		// Remove last entry (least recently used)
		evictSHA := c.diffOrder[len(c.diffOrder)-1]
		c.diffOrder = c.diffOrder[:len(c.diffOrder)-1]
		delete(c.diffCache, evictSHA)
	}
}
```

### Step 3: Create `internal/git/stash_test.go`

```go
package git_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if s.ShortSHA != "abc123d" {
		t.Errorf("ShortSHA = %q, want %q", s.ShortSHA, "abc123d")
	}
	if s.Message != "fix auth bug" {
		t.Errorf("Message = %q, want %q", s.Message, "fix auth bug")
	}
	if s.RawMessage != "On main: fix auth bug" {
		t.Errorf("RawMessage = %q, want %q", s.RawMessage, "On main: fix auth bug")
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
	// Auto-message should extract the commit message portion
	if s.Message != "add login form" {
		t.Errorf("Message = %q, want %q (should extract from WIP)", s.Message, "add login form")
	}
	if s.RawMessage != "WIP on feature/auth: abc1234 add login form" {
		t.Errorf("RawMessage = %q, want original", s.RawMessage)
	}
}

func TestParseStashList_MultipleStashes(t *testing.T) {
	date1 := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	date2 := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339) // 30 days old

	input := fmt.Sprintf(
		"sha1\x00sh1\x00On main: fix one\x00%s\nsha2\x00sh2\x00On develop: fix two\x00%s",
		date1, date2,
	)
	stashes := git.ParseStashList(input, 14*24*time.Hour)

	if len(stashes) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(stashes))
	}

	if stashes[0].Index != 0 {
		t.Errorf("first stash Index = %d, want 0", stashes[0].Index)
	}
	if stashes[1].Index != 1 {
		t.Errorf("second stash Index = %d, want 1", stashes[1].Index)
	}

	// Second stash should be stale (30 days > 14 days threshold)
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

	stashes = git.ParseStashList("   \n  ", 14*24*time.Hour)
	if stashes != nil {
		t.Errorf("expected nil for whitespace input, got %v", stashes)
	}
}

func TestParseStashList_UnicodeMessage(t *testing.T) {
	date := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("sha1\x00sh1\x00On main: \u4FEE\u590D\u767B\u5F55\u95EE\u9898 (fix login)\x00%s", date)
	stashes := git.ParseStashList(line, 14*24*time.Hour)

	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash, got %d", len(stashes))
	}

	if !strings.Contains(stashes[0].Message, "\u4FEE\u590D") {
		t.Errorf("Message should contain unicode chars, got %q", stashes[0].Message)
	}
}

func TestParseStashList_WIPEmptyCommitMessage(t *testing.T) {
	date := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("sha1\x00sh1\x00WIP on main: abc1234 \x00%s", date)
	stashes := git.ParseStashList(line, 14*24*time.Hour)

	if len(stashes) != 1 {
		t.Fatalf("expected 1 stash, got %d", len(stashes))
	}

	if stashes[0].Message != "WIP (no message)" {
		t.Errorf("Message = %q, want %q", stashes[0].Message, "WIP (no message)")
	}
}

func TestParseStashList_Staleness(t *testing.T) {
	tests := []struct {
		name      string
		age       time.Duration
		threshold time.Duration
		wantStale bool
	}{
		{"fresh (1h, threshold 14d)", 1 * time.Hour, 14 * 24 * time.Hour, false},
		{"borderline fresh (13d, threshold 14d)", 13 * 24 * time.Hour, 14 * 24 * time.Hour, false},
		{"exactly stale (14d+1s, threshold 14d)", 14*24*time.Hour + time.Second, 14 * 24 * time.Hour, true},
		{"very stale (60d, threshold 14d)", 60 * 24 * time.Hour, 14 * 24 * time.Hour, true},
		{"custom threshold (3d, threshold 2d)", 3 * 24 * time.Hour, 2 * 24 * time.Hour, true},
		{"zero threshold disables staleness", 100 * 24 * time.Hour, 0, false},
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
		{
			name: "single file, one dir",
			fileCount: 1, insertions: 42, deletions: 17,
			topDirs: []string{"src/auth"},
			want:    "1 file: +42/-17 in src/auth",
		},
		{
			name: "multiple files, multiple dirs",
			fileCount: 3, insertions: 42, deletions: 17,
			topDirs: []string{"src/auth", "pkg/db"},
			want:    "3 files: +42/-17 in src/auth, pkg/db",
		},
		{
			name: "many dirs (truncated)",
			fileCount: 10, insertions: 200, deletions: 50,
			topDirs: []string{"src/auth", "pkg/db", "internal/cache", "cmd/server"},
			want:    "10 files: +200/-50 in src/auth, pkg/db, internal/cache +1 more",
		},
		{
			name: "no dirs",
			fileCount: 5, insertions: 10, deletions: 3,
			topDirs: nil,
			want:    "5 files: +10/-3",
		},
		{
			name: "zero changes",
			fileCount: 0, insertions: 0, deletions: 0,
			topDirs: nil,
			want:    "0 files: +0/-0",
		},
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

	// Create a file and commit it
	filePath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "main.go")
	testHelper(t, dir, "git", "commit", "-m", "add main.go")

	// Create a stash with a custom message
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "add hello print")

	// Create another stash (default WIP message)
	if err := os.WriteFile(filePath, []byte("package main\n\nfunc main() {\n\tfmt.Println(\"world\")\n}\n"), 0o644); err != nil {
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

	// Most recent stash first (index 0)
	if stashes[0].Index != 0 {
		t.Errorf("first stash Index = %d, want 0", stashes[0].Index)
	}
	if stashes[1].Index != 1 {
		t.Errorf("second stash Index = %d, want 1", stashes[1].Index)
	}

	// SHA should be non-empty
	if stashes[0].SHA == "" {
		t.Error("first stash SHA should not be empty")
	}
	if stashes[1].SHA == "" {
		t.Error("second stash SHA should not be empty")
	}

	// The second stash (index 1) should have the custom message
	if stashes[1].Message != "add hello print" {
		t.Errorf("second stash Message = %q, want %q", stashes[1].Message, "add hello print")
	}

	// Both should have file stats
	if stashes[0].FileCount == 0 {
		t.Error("first stash FileCount should be > 0")
	}

	t.Logf("Stash 0: %+v", stashes[0])
	t.Logf("Stash 1: %+v", stashes[1])
}
```

### Step 4: Create `internal/git/cache_test.go`

```go
package git_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
)

// setupRepoWithStashes creates a temp repo with N stashes.
func setupRepoWithStashes(t *testing.T, count int) (string, *git.DefaultRunner) {
	t.Helper()
	dir := setupTempRepo(t)

	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "file.txt")
	testHelper(t, dir, "git", "commit", "-m", "add file")

	for i := 0; i < count; i++ {
		content := fmt.Sprintf("change %d", i)
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		testHelper(t, dir, "git", "stash", "push", "-m", fmt.Sprintf("stash %d", i))
	}

	runner := git.NewDefaultRunner(dir, nil)
	return dir, runner
}

func TestDefaultStashCache_List(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 3)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	// First call fetches from git
	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(stashes) != 3 {
		t.Fatalf("expected 3 stashes, got %d", len(stashes))
	}

	// Second call should return cached result
	stashes2, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() second call error: %v", err)
	}
	if len(stashes2) != 3 {
		t.Fatalf("cached result should have 3 stashes, got %d", len(stashes2))
	}

	// Verify same data (cached)
	if stashes[0].SHA != stashes2[0].SHA {
		t.Errorf("cached SHA mismatch: %q vs %q", stashes[0].SHA, stashes2[0].SHA)
	}
}

func TestDefaultStashCache_Invalidate(t *testing.T) {
	dir, runner := setupRepoWithStashes(t, 2)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	// Load initial list
	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(stashes) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(stashes))
	}

	// Add another stash
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("new change"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "new stash")

	// Without invalidation, cache returns old data
	stashes, _ = cache.List(ctx)
	if len(stashes) != 2 {
		t.Errorf("before invalidation, expected 2 (cached), got %d", len(stashes))
	}

	// After invalidation, cache fetches fresh data
	cache.Invalidate()
	stashes, err = cache.List(ctx)
	if err != nil {
		t.Fatalf("List() after invalidate error: %v", err)
	}
	if len(stashes) != 3 {
		t.Errorf("after invalidation, expected 3, got %d", len(stashes))
	}
}

func TestDefaultStashCache_Diff(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 2)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	// Get the list first to know SHAs
	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	// Fetch diff by SHA
	diff, err := cache.Diff(ctx, stashes[0].SHA)
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if diff == "" {
		t.Error("diff should not be empty")
	}

	// Second call should be cached
	diff2, err := cache.Diff(ctx, stashes[0].SHA)
	if err != nil {
		t.Fatalf("Diff() cached error: %v", err)
	}
	if diff != diff2 {
		t.Error("cached diff should match original")
	}

	// Verify cache size
	if cache.DiffCacheSize() != 1 {
		t.Errorf("DiffCacheSize() = %d, want 1", cache.DiffCacheSize())
	}
}

func TestDefaultStashCache_Diff_NonexistentSHA(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 1)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	// Load list first
	_, _ = cache.List(ctx)

	// Try to get diff for a SHA that doesn't exist
	_, err := cache.Diff(ctx, "nonexistent_sha_1234567890")
	if err == nil {
		t.Error("expected error for non-existent SHA")
	}
}

func TestDefaultStashCache_LRUEviction(t *testing.T) {
	// Create a cache with max size 2
	_, runner := setupRepoWithStashes(t, 4)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 2)
	ctx := context.Background()

	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(stashes) < 3 {
		t.Fatalf("need at least 3 stashes, got %d", len(stashes))
	}

	// Load diffs for 3 stashes (exceeds max of 2)
	for i := 0; i < 3; i++ {
		_, err := cache.Diff(ctx, stashes[i].SHA)
		if err != nil {
			t.Fatalf("Diff(%d) error: %v", i, err)
		}
	}

	// Cache should have evicted the oldest (first loaded)
	if cache.DiffCacheSize() != 2 {
		t.Errorf("DiffCacheSize() = %d, want 2 (LRU max)", cache.DiffCacheSize())
	}
}

func TestDefaultStashCache_SHAKeyStability(t *testing.T) {
	// This test verifies that diff cache is keyed by SHA (not index),
	// so dropping a stash doesn't invalidate unrelated cached diffs.
	dir, runner := setupRepoWithStashes(t, 3)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	stashes, _ := cache.List(ctx)
	sha0 := stashes[0].SHA
	sha2 := stashes[2].SHA

	// Cache diff for stash@{2}
	diff2, _ := cache.Diff(ctx, sha2)

	// Drop stash@{0} -- this shifts indices but not SHAs
	testHelper(t, dir, "git", "stash", "drop", "stash@{0}")

	// Invalidate list cache (indices changed)
	cache.Invalidate()

	// Reload list -- stash@{2} is now stash@{1}
	newStashes, _ := cache.List(ctx)

	// Find the stash with the same SHA
	found := false
	for _, s := range newStashes {
		if s.SHA == sha2 {
			found = true
			// Its index should have shifted
			if s.Index == 2 {
				t.Error("index should have shifted after drop")
			}
			break
		}
	}
	if !found {
		t.Error("stash with SHA should still exist after dropping a different stash")
	}

	// The diff for sha2 should still be in cache (keyed by SHA, not index)
	cachedDiff, err := cache.Diff(ctx, sha2)
	if err != nil {
		t.Fatalf("Diff() after drop error: %v", err)
	}
	if cachedDiff != diff2 {
		t.Error("cached diff should survive index shift (keyed by SHA)")
	}

	// Verify the dropped stash's SHA is no longer in the list
	for _, s := range newStashes {
		if s.SHA == sha0 {
			t.Error("dropped stash SHA should not be in the list")
		}
	}
}

func TestDefaultStashCache_ConcurrentAccess(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 3)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	// Pre-load the list
	stashes, _ := cache.List(ctx)

	// Concurrent reads should not race
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.List(ctx)
		}()
	}

	// Concurrent diff reads
	for i := 0; i < len(stashes); i++ {
		sha := stashes[i].SHA
		for j := 0; j < 3; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = cache.Diff(ctx, sha)
			}()
		}
	}

	// Concurrent invalidations
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Invalidate()
		}()
	}

	wg.Wait()
	// If we get here without data races, the test passes.
	// The -race flag will catch any races.
}

func TestDefaultStashCache_PreloadDiffs(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 5)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	// Preload top 3
	cache.PreloadDiffs(ctx, 3)

	// Should have cached 3 diffs
	if cache.DiffCacheSize() != 3 {
		t.Errorf("after PreloadDiffs(3), cache size = %d, want 3", cache.DiffCacheSize())
	}
}

func TestDefaultStashCache_PreloadDiffs_MoreThanAvailable(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 2)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	// Preload more than available
	cache.PreloadDiffs(ctx, 10)

	// Should only have cached the 2 that exist
	if cache.DiffCacheSize() != 2 {
		t.Errorf("after PreloadDiffs(10) with 2 stashes, cache size = %d, want 2", cache.DiffCacheSize())
	}
}

func TestDefaultStashCache_InvalidateDiff(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 2)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	stashes, _ := cache.List(ctx)

	// Cache both diffs
	_, _ = cache.Diff(ctx, stashes[0].SHA)
	_, _ = cache.Diff(ctx, stashes[1].SHA)

	if cache.DiffCacheSize() != 2 {
		t.Fatalf("expected 2 cached diffs, got %d", cache.DiffCacheSize())
	}

	// Invalidate one
	cache.InvalidateDiff(stashes[0].SHA)

	if cache.DiffCacheSize() != 1 {
		t.Errorf("after InvalidateDiff, cache size = %d, want 1", cache.DiffCacheSize())
	}
}
```

### Step 5: Verify compilation

```bash
go build ./internal/git/...
```

### Step 6: Run tests

```bash
go test -v -race -count=1 ./internal/git/...
```

### Step 7: Run `make ci`

```bash
make ci
```

## Verification

### Functional
```bash
# Package compiles
go build ./internal/git/...

# All tests pass with race detector
go test -v -race -count=1 ./internal/git/...

# Stash parsing tests
go test -v -run TestParseStashList ./internal/git/...
go test -v -run TestGenerateAutoMessage ./internal/git/...
go test -v -run TestListStashes_RealGit ./internal/git/...

# Cache tests
go test -v -run TestDefaultStashCache_List ./internal/git/...
go test -v -run TestDefaultStashCache_Diff ./internal/git/...
go test -v -run TestDefaultStashCache_LRUEviction ./internal/git/...
go test -v -run TestDefaultStashCache_SHAKeyStability ./internal/git/...
go test -v -run TestDefaultStashCache_ConcurrentAccess ./internal/git/...
go test -v -run TestDefaultStashCache_PreloadDiffs ./internal/git/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `Stash` struct has all 12 fields from PRD section 8.3: Index, SHA, ShortSHA, Message, RawMessage, Branch, Date, FileCount, Insertions, Deletions, IsStale, HasUntracked
2. `ParseStashList()` handles: standard "On branch: msg", WIP "WIP on branch: sha msg", empty input, unicode messages, multiple stashes
3. `generateMessage()` extracts commit message from WIP messages, passes through user messages
4. `GenerateAutoMessage()` produces "N files: +X/-Y in dir1, dir2" format (FR-01.4)
5. Staleness calculation works with configurable threshold, zero threshold disables staleness
6. `StashCache` interface has `List`, `Diff`, `Invalidate`, `PreloadDiffs`
7. `DefaultStashCache` implements LRU diff cache keyed by SHA (not index)
8. LRU eviction works correctly when cache exceeds max size
9. `Invalidate()` clears list cache but preserves diff cache (SHAs don't change)
10. `InvalidateDiff()` removes a specific SHA from diff cache
11. SHA-based keying survives index shifts after stash drops
12. Concurrent access to List, Diff, and Invalidate is race-free
13. `PreloadDiffs()` pre-caches diffs for top N stashes
14. Tests create real temp repos with real stashes using `t.TempDir()`
15. `go test -race ./internal/git/...` passes
16. `make ci` passes

## Commit
```
feat: add stash parser with auto-messages and LRU diff cache

Implement internal/git/stash.go with ParseStashList, auto-message
generation (FR-01.4), staleness calculation, and diff stat enrichment.
Implement internal/git/cache.go with DefaultStashCache using SHA-keyed
LRU diff cache, preloading, and concurrent-safe invalidation.
Table-driven tests against real git repos with race detection.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 5.1, 6.1 (FR-01.3, FR-01.4), 8.3, 14.1, 14.3
4. Execute steps 1-7 in order
5. Verify all functional and CI checks pass
6. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
7. Commit with the message above + move to next task
