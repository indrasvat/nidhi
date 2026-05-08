package undo

import (
	"context"
	"fmt"
	"strings"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// RecoveryCandidate represents a potentially recoverable stash commit
// found via `git fsck`.
type RecoveryCandidate struct {
	SHA     string // Full commit SHA
	Message string // Commit message (may be stash-format or user-provided)
	Date    string // Commit date string
}

// FindDroppedStashes discovers orphaned commits that look like stash entries
// using `git fsck --unreachable --no-reflogs`.
//
// This is best-effort: commits may have been garbage-collected.
// FR-14.3: Cross-session recovery.
func FindDroppedStashes(ctx context.Context, runner plugin.GitRunner) ([]RecoveryCandidate, error) {
	stdout, err := runner.Run(ctx, "fsck", "--unreachable", "--no-reflogs")
	if err != nil {
		return nil, fmt.Errorf("git fsck: %w", err)
	}

	var candidates []RecoveryCandidate

	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "unreachable commit ") {
			continue
		}

		sha := strings.TrimPrefix(line, "unreachable commit ")
		sha = strings.TrimSpace(sha)
		if sha == "" {
			continue
		}

		// Stash commits have 2-3 parents. Check parent count.
		catOutput, err := runner.Run(ctx, "cat-file", "-p", sha)
		if err != nil {
			continue
		}

		parentCount := 0
		for pl := range strings.SplitSeq(catOutput, "\n") {
			if strings.HasPrefix(strings.TrimSpace(pl), "parent ") {
				parentCount++
			}
		}
		if parentCount < 2 {
			continue // Not a stash-like commit
		}

		// Get the commit message.
		msg, err := runner.Run(ctx, "log", "--format=%s", "-1", sha)
		if err != nil {
			msg = "(unknown message)"
		}
		msg = strings.TrimSpace(msg)

		// Get the commit date.
		date, err := runner.Run(ctx, "log", "--format=%ar", "-1", sha)
		if err != nil {
			date = "(unknown date)"
		}
		date = strings.TrimSpace(date)

		candidates = append(candidates, RecoveryCandidate{
			SHA:     sha,
			Message: msg,
			Date:    date,
		})
	}

	return candidates, nil
}

// RestoreCandidate restores a recovery candidate as a stash entry.
func RestoreCandidate(ctx context.Context, runner plugin.GitRunner, candidate RecoveryCandidate) error {
	_, err := runner.Run(ctx, "stash", "store", "-m", candidate.Message, candidate.SHA)
	if err != nil {
		return fmt.Errorf("restore candidate %s: %w", candidate.SHA[:min(8, len(candidate.SHA))], err)
	}
	return nil
}
