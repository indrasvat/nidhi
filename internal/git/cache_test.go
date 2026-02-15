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

	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(stashes) != 3 {
		t.Fatalf("expected 3 stashes, got %d", len(stashes))
	}

	stashes2, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() second call error: %v", err)
	}
	if stashes[0].SHA != stashes2[0].SHA {
		t.Errorf("cached SHA mismatch")
	}
}

func TestDefaultStashCache_Invalidate(t *testing.T) {
	dir, runner := setupRepoWithStashes(t, 2)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(stashes) != 2 {
		t.Fatalf("expected 2 stashes, got %d", len(stashes))
	}

	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("new change"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "new stash")

	stashes, _ = cache.List(ctx)
	if len(stashes) != 2 {
		t.Errorf("before invalidation, expected 2 (cached), got %d", len(stashes))
	}

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

	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	diff, err := cache.Diff(ctx, stashes[0].SHA)
	if err != nil {
		t.Fatalf("Diff() error: %v", err)
	}
	if diff == "" {
		t.Error("diff should not be empty")
	}

	diff2, err := cache.Diff(ctx, stashes[0].SHA)
	if err != nil {
		t.Fatalf("Diff() cached error: %v", err)
	}
	if diff != diff2 {
		t.Error("cached diff should match original")
	}

	if cache.DiffCacheSize() != 1 {
		t.Errorf("DiffCacheSize() = %d, want 1", cache.DiffCacheSize())
	}
}

func TestDefaultStashCache_LRUEviction(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 4)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 2)
	ctx := context.Background()

	stashes, err := cache.List(ctx)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	for i := 0; i < 3 && i < len(stashes); i++ {
		_, err := cache.Diff(ctx, stashes[i].SHA)
		if err != nil {
			t.Fatalf("Diff(%d) error: %v", i, err)
		}
	}

	if cache.DiffCacheSize() != 2 {
		t.Errorf("DiffCacheSize() = %d, want 2 (LRU max)", cache.DiffCacheSize())
	}
}

func TestDefaultStashCache_SHAKeyStability(t *testing.T) {
	dir, runner := setupRepoWithStashes(t, 3)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	stashes, _ := cache.List(ctx)
	sha2 := stashes[2].SHA

	diff2, _ := cache.Diff(ctx, sha2)

	testHelper(t, dir, "git", "stash", "drop", "stash@{0}")
	cache.Invalidate()

	cachedDiff, err := cache.Diff(ctx, sha2)
	if err != nil {
		t.Fatalf("Diff() after drop error: %v", err)
	}
	if cachedDiff != diff2 {
		t.Error("cached diff should survive index shift (keyed by SHA)")
	}
}

func TestDefaultStashCache_ConcurrentAccess(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 3)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	stashes, _ := cache.List(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.List(ctx)
		}()
	}

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

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Invalidate()
		}()
	}

	wg.Wait()
}

func TestDefaultStashCache_PreloadDiffs(t *testing.T) {
	_, runner := setupRepoWithStashes(t, 5)
	cache := git.NewDefaultStashCache(runner, 14*24*time.Hour, 50)
	ctx := context.Background()

	cache.PreloadDiffs(ctx, 3)

	if cache.DiffCacheSize() != 3 {
		t.Errorf("after PreloadDiffs(3), cache size = %d, want 3", cache.DiffCacheSize())
	}
}
