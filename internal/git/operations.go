package git

import (
	"context"
	"fmt"
	"strings"
)

// ─── Result types ───────────────────────────────────────────

// OperationResult captures the outcome of a stash operation.
type OperationResult struct {
	// Success indicates whether the operation completed without error.
	Success bool
	// SHA is the commit SHA of the affected stash (for undo support).
	SHA string
	// Message is the stash message (for undo display).
	Message string
	// Error is the git error output if the operation failed.
	Error string
}

// PushOptions configures the behavior of a new stash push.
type PushOptions struct {
	// Message is the stash message (required — nidhi is message-first).
	Message string
	// KeepIndex preserves staged changes in the working tree.
	KeepIndex bool
	// IncludeUntracked includes untracked files in the stash.
	IncludeUntracked bool
	// Staged stashes only the staged changes (Git 2.35+).
	Staged bool
	// Pathspecs limits the stash to specific file patterns.
	Pathspecs []string
}

// ClearAllResult captures all SHAs before clearing, for potential bulk undo.
type ClearAllResult struct {
	Entries []OperationResult
}

// ─── StashOps ───────────────────────────────────────────────

// StashOps provides git stash CRUD operations.
// All methods take a context for timeout support and use GitRunner
// for command execution.
type StashOps struct {
	runner GitRunner
	cache  StashCache
}

// NewStashOps creates a new StashOps instance.
func NewStashOps(runner GitRunner, cache StashCache) *StashOps {
	return &StashOps{runner: runner, cache: cache}
}

// Apply applies the stash at the given index to the working tree.
// The stash is preserved in the list (FR-02.1).
func (s *StashOps) Apply(ctx context.Context, index int) (OperationResult, error) {
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}
	msg, _ := s.getStashMessage(ctx, index)

	ref := fmt.Sprintf("stash@{%d}", index)
	output, exitCode, err := s.runner.RunExitCode(ctx, "stash", "apply", ref)
	if err != nil {
		return OperationResult{}, fmt.Errorf("git stash apply: %w", err)
	}
	if exitCode != 0 {
		return OperationResult{Success: false, SHA: sha, Message: msg, Error: output},
			fmt.Errorf("git stash apply %s failed (exit %d)", ref, exitCode)
	}

	// Apply does NOT invalidate cache — stash list is unchanged.
	return OperationResult{Success: true, SHA: sha, Message: msg}, nil
}

// Pop applies and removes the stash at the given index (FR-02.2).
// Captures the SHA before popping for potential undo.
func (s *StashOps) Pop(ctx context.Context, index int) (OperationResult, error) {
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}
	msg, _ := s.getStashMessage(ctx, index)

	ref := fmt.Sprintf("stash@{%d}", index)
	output, exitCode, err := s.runner.RunExitCode(ctx, "stash", "pop", ref)
	if err != nil {
		return OperationResult{}, fmt.Errorf("git stash pop: %w", err)
	}
	if exitCode != 0 {
		return OperationResult{Success: false, SHA: sha, Message: msg, Error: output},
			fmt.Errorf("git stash pop %s failed (exit %d)", ref, exitCode)
	}

	s.cache.Invalidate()
	return OperationResult{Success: true, SHA: sha, Message: msg}, nil
}

// Drop removes the stash at the given index without applying (FR-02.3).
// Returns the SHA so the caller can display an undo toast.
func (s *StashOps) Drop(ctx context.Context, index int) (OperationResult, error) {
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}
	msg, _ := s.getStashMessage(ctx, index)

	ref := fmt.Sprintf("stash@{%d}", index)
	output, exitCode, err := s.runner.RunExitCode(ctx, "stash", "drop", ref)
	if err != nil {
		return OperationResult{}, fmt.Errorf("git stash drop: %w", err)
	}
	if exitCode != 0 {
		return OperationResult{Success: false, SHA: sha, Message: msg, Error: output},
			fmt.Errorf("git stash drop %s failed (exit %d)", ref, exitCode)
	}

	s.cache.Invalidate()
	return OperationResult{Success: true, SHA: sha, Message: msg}, nil
}

// Push creates a new stash with the given options (FR-02.4).
func (s *StashOps) Push(ctx context.Context, opts PushOptions) (OperationResult, error) {
	args := []string{"stash", "push"}

	if opts.Message != "" {
		args = append(args, "-m", opts.Message)
	}
	if opts.KeepIndex {
		args = append(args, "--keep-index")
	}
	if opts.IncludeUntracked {
		args = append(args, "--include-untracked")
	}
	if opts.Staged {
		args = append(args, "--staged")
	}
	if len(opts.Pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, opts.Pathspecs...)
	}

	output, exitCode, err := s.runner.RunExitCode(ctx, args...)
	if err != nil {
		return OperationResult{}, fmt.Errorf("git stash push: %w", err)
	}
	if exitCode != 0 {
		return OperationResult{Success: false, Error: output},
			fmt.Errorf("git stash push failed (exit %d)", exitCode)
	}

	// git stash push returns exit 0 even when there are no changes to stash.
	// Detect this via the output message.
	if strings.Contains(output, "No local changes to save") {
		return OperationResult{Success: false, Error: output},
			fmt.Errorf("git stash push: no local changes to save")
	}

	s.cache.Invalidate()

	// Get the SHA of the newly created stash (now at index 0).
	sha, _ := s.getStashSHA(ctx, 0)

	return OperationResult{Success: true, SHA: sha, Message: opts.Message}, nil
}

// BranchFromStash creates a new branch from the stash and checks it out (FR-02.5).
// This applies the stash, creates the branch, and removes the stash entry.
func (s *StashOps) BranchFromStash(ctx context.Context, index int, branchName string) (OperationResult, error) {
	sha, err := s.getStashSHA(ctx, index)
	if err != nil {
		return OperationResult{}, fmt.Errorf("get stash SHA: %w", err)
	}
	msg, _ := s.getStashMessage(ctx, index)

	ref := fmt.Sprintf("stash@{%d}", index)
	output, exitCode, err := s.runner.RunExitCode(ctx, "stash", "branch", branchName, ref)
	if err != nil {
		return OperationResult{}, fmt.Errorf("git stash branch: %w", err)
	}
	if exitCode != 0 {
		return OperationResult{Success: false, SHA: sha, Message: msg, Error: output},
			fmt.Errorf("git stash branch %s %s failed (exit %d)", branchName, ref, exitCode)
	}

	s.cache.Invalidate()
	return OperationResult{Success: true, SHA: sha, Message: msg}, nil
}

// ClearAll drops all stashes (FR-02.6).
// Captures all SHAs and messages BEFORE clearing so bulk undo is possible.
func (s *StashOps) ClearAll(ctx context.Context) (ClearAllResult, error) {
	// First, capture all stash SHAs and messages.
	output, err := s.runner.Run(ctx, "stash", "list", "--format=%H %s")
	if err != nil {
		return ClearAllResult{}, fmt.Errorf("git stash list: %w", err)
	}

	var entries []OperationResult
	if output != "" {
		for line := range strings.SplitSeq(output, "\n") {
			if line == "" {
				continue
			}
			sha, msg, _ := strings.Cut(line, " ")
			entries = append(entries, OperationResult{
				Success: true,
				SHA:     sha,
				Message: msg,
			})
		}
	}

	// Now clear all stashes.
	_, exitCode, err := s.runner.RunExitCode(ctx, "stash", "clear")
	if err != nil {
		return ClearAllResult{}, fmt.Errorf("git stash clear: %w", err)
	}
	if exitCode != 0 {
		return ClearAllResult{}, fmt.Errorf("git stash clear failed (exit %d)", exitCode)
	}

	s.cache.Invalidate()
	return ClearAllResult{Entries: entries}, nil
}

// RestoreStash re-stores a previously dropped stash using its SHA and message.
// Used by the undo system to recover dropped stashes.
func (s *StashOps) RestoreStash(ctx context.Context, sha, message string) error {
	args := []string{"stash", "store"}
	if message != "" {
		args = append(args, "-m", message)
	}
	args = append(args, sha)

	_, exitCode, err := s.runner.RunExitCode(ctx, args...)
	if err != nil {
		return fmt.Errorf("git stash store: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("git stash store %s failed (exit %d)", sha, exitCode)
	}

	s.cache.Invalidate()
	return nil
}

// ─── Helpers ────────────────────────────────────────────────

// getStashSHA returns the full commit SHA for a stash at the given index.
func (s *StashOps) getStashSHA(ctx context.Context, index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	output, exitCode, err := s.runner.RunExitCode(ctx, "rev-parse", ref)
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	if exitCode != 0 {
		return "", fmt.Errorf("stash %s not found", ref)
	}
	return strings.TrimSpace(output), nil
}

// getStashMessage returns the message for a stash at the given index.
func (s *StashOps) getStashMessage(ctx context.Context, index int) (string, error) {
	ref := fmt.Sprintf("stash@{%d}", index)
	output, exitCode, err := s.runner.RunExitCode(ctx, "log", "-1", "--format=%s", ref)
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		return "", fmt.Errorf("stash %s not found", ref)
	}
	return strings.TrimSpace(output), nil
}
