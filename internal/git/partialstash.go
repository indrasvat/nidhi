package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

// partialstash.go orchestrates creating a stash from a subset of working-tree
// changes (the patch produced by patch.go's surgery). It uses only standard
// git plumbing — no interactive prompts — and is transactional: a crash-safe
// journal records the pre-existing staged state so it can be restored, and the
// `git apply --check` gate ensures we never half-modify the working tree.
//
// Mechanism (Git ≥ 2.35 for `stash push --staged`):
//  1. Snapshot the existing index:  git diff --cached --binary
//  2. Reset the index to HEAD (worktree untouched):  git reset -q
//  3. Stage exactly the selection:   git apply --cached <selected.patch>
//  4. Stash only the staged subset:  git stash push --staged -m <msg>
//     (git removes the selection from index AND worktree)
//  5. Restore the original staged state:  git apply --cached <restore.patch>

// PartialResult captures the outcome of a partial stash creation.
type PartialResult struct {
	Success bool
	SHA     string
	Message string
	// Note is a non-fatal advisory (e.g. staged changes could not be
	// fully restored and are now unstaged).
	Note string
}

// CreatePartialStash creates a stash containing only the changes described by
// patch (a unified diff applicable to the index reset to HEAD). The remaining
// working-tree changes are left in place.
func CreatePartialStash(ctx context.Context, runner GitRunner, patch, message string) (PartialResult, error) {
	if strings.TrimSpace(patch) == "" {
		return PartialResult{}, errors.New("no changes selected")
	}

	patchFile, cleanupPatch, err := writeTempPatch("nidhi-partial-*.patch", patch)
	if err != nil {
		return PartialResult{}, err
	}
	defer cleanupPatch()

	// 1. Snapshot pre-existing staged state.
	staged, _, err := runner.RunExitCode(ctx, "diff", "--cached", "--binary")
	if err != nil {
		return PartialResult{}, fmt.Errorf("snapshot index: %w", err)
	}
	hasStaged := strings.TrimSpace(staged) != ""

	var restoreFile string
	if hasStaged {
		var cleanupRestore func()
		restoreFile, cleanupRestore, err = writeTempPatch("nidhi-restore-*.patch", staged+"\n")
		if err != nil {
			return PartialResult{}, err
		}
		defer cleanupRestore()
	}

	jrnl := newPartialJournal(staged)
	_ = jrnl.write() // best-effort; journal failure must not block the op

	rollback := func() {
		// Return to the pre-operation state as closely as possible.
		_, _, _ = runner.RunExitCode(ctx, "reset", "-q")
		if hasStaged {
			_, _, _ = runner.RunExitCode(ctx, "apply", "--cached", restoreFile)
		}
		_ = jrnl.remove()
	}

	// 2. Reset index to HEAD (only needed when something was staged).
	if hasStaged {
		if _, code, rerr := runner.RunExitCode(ctx, "reset", "-q"); rerr != nil || code != 0 {
			rollback()
			return PartialResult{}, fmt.Errorf("reset index failed (exit %d): %w", code, rerr)
		}
	}

	// 3. Validate then stage the selection.
	if out, code, cerr := runner.RunExitCode(ctx, "apply", "--cached", "--check", patchFile); cerr != nil || code != 0 {
		rollback()
		return PartialResult{}, fmt.Errorf("selected changes do not apply cleanly: %s", strings.TrimSpace(out))
	}
	if out, code, aerr := runner.RunExitCode(ctx, "apply", "--cached", patchFile); aerr != nil || code != 0 {
		rollback()
		return PartialResult{}, fmt.Errorf("staging selection failed: %s", strings.TrimSpace(out))
	}

	// 4. Stash only the staged subset.
	out, code, serr := runner.RunExitCode(ctx, "stash", "push", "--staged", "-m", message)
	if serr != nil || code != 0 {
		rollback()
		return PartialResult{}, fmt.Errorf("git stash push --staged failed (exit %d): %s", code, strings.TrimSpace(out))
	}
	if strings.Contains(out, "No local changes to save") {
		rollback()
		return PartialResult{}, errors.New("no changes to stash")
	}

	// 5. Restore the original staged state.
	note := ""
	if hasStaged {
		if _, ccode, _ := runner.RunExitCode(ctx, "apply", "--cached", "--check", restoreFile); ccode == 0 {
			_, _, _ = runner.RunExitCode(ctx, "apply", "--cached", restoreFile)
		} else {
			note = "pre-existing staged changes are now unstaged"
		}
	}

	_ = jrnl.markComplete()
	_ = jrnl.remove()

	sha := partialStashSHA(ctx, runner)
	return PartialResult{Success: true, SHA: sha, Message: message, Note: note}, nil
}

func partialStashSHA(ctx context.Context, runner GitRunner) string {
	out, code, err := runner.RunExitCode(ctx, "rev-parse", "stash@{0}")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}

func writeTempPatch(pattern, content string) (string, func(), error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp patch: %w", err)
	}
	name := f.Name()
	cleanup := func() { _ = os.Remove(name) }
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("write temp patch: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("close temp patch: %w", err)
	}
	return name, cleanup, nil
}

// ─── Crash-safe journal ─────────────────────────────────────

// partialJournal records the pre-operation staged state so an interrupted
// partial stash can restore the index on the next launch.
type partialJournal struct {
	Operation    string     `json:"operation"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	RestorePatch string     `json:"restore_patch"`
	filePath     string
}

// DefaultPartialJournalPath returns the journal path for partial stash ops.
func DefaultPartialJournalPath() string {
	return filepath.Join(xdg.StateHome, "nidhi", "partial-journal.json")
}

func newPartialJournal(restorePatch string) *partialJournal {
	return &partialJournal{
		Operation:    "partial-stash",
		StartedAt:    time.Now(),
		RestorePatch: restorePatch,
		filePath:     DefaultPartialJournalPath(),
	}
}

func (j *partialJournal) write() error {
	if err := os.MkdirAll(filepath.Dir(j.filePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(j.filePath, data, 0o644)
}

func (j *partialJournal) markComplete() error {
	now := time.Now()
	j.CompletedAt = &now
	return j.write()
}

func (j *partialJournal) remove() error {
	err := os.Remove(j.filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HasIncompletePartialStash reports whether a previous partial stash was
// interrupted before completion.
func HasIncompletePartialStash() bool {
	data, err := os.ReadFile(DefaultPartialJournalPath())
	if err != nil {
		return false
	}
	var j partialJournal
	if err := json.Unmarshal(data, &j); err != nil {
		return false
	}
	return j.CompletedAt == nil
}

// RecoverPartialStash restores the index from an interrupted partial stash and
// clears the journal. Best-effort: returns nil if there is nothing to recover.
func RecoverPartialStash(ctx context.Context, runner GitRunner) error {
	path := DefaultPartialJournalPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var j partialJournal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil
	}
	if j.CompletedAt != nil {
		_ = os.Remove(path)
		return nil
	}
	if strings.TrimSpace(j.RestorePatch) != "" {
		restoreFile, cleanup, werr := writeTempPatch("nidhi-recover-*.patch", j.RestorePatch+"\n")
		if werr == nil {
			defer cleanup()
			if _, code, _ := runner.RunExitCode(ctx, "apply", "--cached", "--check", restoreFile); code == 0 {
				_, _, _ = runner.RunExitCode(ctx, "apply", "--cached", restoreFile)
			}
		}
	}
	return os.Remove(path)
}
