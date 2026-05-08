package git

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StashCache provides cached access to stash data.
type StashCache interface {
	List(ctx context.Context) ([]Stash, error)
	Diff(ctx context.Context, sha string) (string, error)
	Invalidate()
	PreloadDiffs(ctx context.Context, n int)
}

// DefaultStashCache implements StashCache with an LRU diff cache.
type DefaultStashCache struct {
	runner         GitRunner
	staleThreshold time.Duration
	maxDiffCache   int

	mu      sync.RWMutex
	stashes []Stash
	valid   bool

	diffMu    sync.RWMutex
	diffCache map[string]*diffEntry
	diffOrder []string
}

type diffEntry struct {
	content string
	err     error
}

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

var _ StashCache = (*DefaultStashCache)(nil)

func (c *DefaultStashCache) List(ctx context.Context) ([]Stash, error) {
	c.mu.RLock()
	if c.valid {
		stashes := c.stashes
		c.mu.RUnlock()
		return stashes, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

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

func (c *DefaultStashCache) Diff(ctx context.Context, sha string) (string, error) {
	c.diffMu.RLock()
	if entry, ok := c.diffCache[sha]; ok {
		c.diffMu.RUnlock()
		c.touchLRU(sha)
		return entry.content, entry.err
	}
	c.diffMu.RUnlock()

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

	c.diffMu.Lock()
	c.diffCache[sha] = &diffEntry{content: diff, err: diffErr}
	c.diffOrder = append([]string{sha}, c.diffOrder...)
	c.evictLRU()
	c.diffMu.Unlock()

	return diff, diffErr
}

func (c *DefaultStashCache) Invalidate() {
	c.mu.Lock()
	c.stashes = nil
	c.valid = false
	c.mu.Unlock()
}

// InvalidateAll clears both the list cache and the diff cache.
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
			return
		}
		_, _ = c.Diff(ctx, stashes[i].SHA)
	}
}

// DiffCacheSize returns the current number of cached diffs.
func (c *DefaultStashCache) DiffCacheSize() int {
	c.diffMu.RLock()
	defer c.diffMu.RUnlock()
	return len(c.diffCache)
}

func (c *DefaultStashCache) touchLRU(sha string) {
	c.diffMu.Lock()
	defer c.diffMu.Unlock()

	for i, s := range c.diffOrder {
		if s == sha {
			c.diffOrder = append(c.diffOrder[:i], c.diffOrder[i+1:]...)
			c.diffOrder = append([]string{sha}, c.diffOrder...)
			return
		}
	}
}

func (c *DefaultStashCache) evictLRU() {
	for len(c.diffOrder) > c.maxDiffCache {
		evictSHA := c.diffOrder[len(c.diffOrder)-1]
		c.diffOrder = c.diffOrder[:len(c.diffOrder)-1]
		delete(c.diffCache, evictSHA)
	}
}
