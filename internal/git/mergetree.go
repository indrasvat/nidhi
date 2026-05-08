package git

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

// FileConflictStatus represents the merge status of a single file.
type FileConflictStatus int

const (
	FileStatusClean      FileConflictStatus = iota // No conflicts, clean merge
	FileStatusConflicted                           // Merge conflict detected
	FileStatusUnknown                              // Could not determine status
)

// MergeTreeFile represents a single file in the merge-tree result.
type MergeTreeFile struct {
	Path          string
	Status        FileConflictStatus
	ConflictZones []ConflictZone // Non-empty only if Status == FileStatusConflicted
}

// ConflictZone represents a single conflict hunk in a file.
type ConflictZone struct {
	OurContent   string // Content from HEAD (ours)
	TheirContent string // Content from stash (theirs)
	BaseContent  string // Content from merge base
}

// UntrackedCollision represents an untracked file in the stash that already
// exists in the working tree (FR-10.6a).
type UntrackedCollision struct {
	Path string
}

// MergeTreeResult holds the parsed output of `git merge-tree --write-tree`.
type MergeTreeResult struct {
	HasConflicts        bool
	TreeSHA             string // The resulting tree SHA (first line of output)
	Files               []MergeTreeFile
	UntrackedCollisions []UntrackedCollision // Not from merge-tree; populated separately
	Informational       []string             // Informational messages from merge-tree
}

// RunMergeTree executes `git merge-tree --write-tree HEAD <stashCommit>` and
// parses the result.
//
// Exit code 0 = no conflicts (clean merge).
// Exit code 1 = conflicts detected.
func RunMergeTree(ctx context.Context, runner GitRunner, stashCommit string) (MergeTreeResult, error) {
	stdout, exitCode, err := runner.RunExitCode(ctx, "merge-tree", "--write-tree", "HEAD", stashCommit)
	if err != nil {
		return MergeTreeResult{}, fmt.Errorf("merge-tree: %w", err)
	}

	result := ParseMergeTreeOutput(stdout)

	switch exitCode {
	case 0:
		result.HasConflicts = false
	case 1:
		result.HasConflicts = true
	default:
		return result, fmt.Errorf("merge-tree: unexpected exit code %d", exitCode)
	}

	return result, nil
}

// ParseMergeTreeOutput parses the stdout of `git merge-tree --write-tree`.
//
// Format (clean):
//
//	<tree-sha>
//
// Format (conflicts):
//
//	<tree-sha>
//	<empty line>
//	<informational/conflict messages>
func ParseMergeTreeOutput(output string) MergeTreeResult {
	result := MergeTreeResult{}
	scanner := bufio.NewScanner(strings.NewReader(output))

	// First line is always the tree SHA.
	if scanner.Scan() {
		result.TreeSHA = strings.TrimSpace(scanner.Text())
	}

	seenFiles := make(map[string]*MergeTreeFile)
	var fileOrder []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "CONFLICT"):
			path := parseConflictPath(line)
			if path != "" {
				f := getOrCreateFile(seenFiles, &fileOrder, path)
				f.Status = FileStatusConflicted
				result.HasConflicts = true
			}
			result.Informational = append(result.Informational, line)

		case strings.HasPrefix(line, "Auto-merging"):
			path := strings.TrimPrefix(line, "Auto-merging ")
			path = strings.TrimSpace(path)
			if path != "" {
				f := getOrCreateFile(seenFiles, &fileOrder, path)
				if f.Status != FileStatusConflicted {
					f.Status = FileStatusClean
				}
			}
			result.Informational = append(result.Informational, line)

		default:
			result.Informational = append(result.Informational, line)
		}
	}

	// Collect files in stable insertion order.
	for _, path := range fileOrder {
		result.Files = append(result.Files, *seenFiles[path])
	}

	return result
}

func parseConflictPath(line string) string {
	// Format: "CONFLICT (content): Merge conflict in <path>"
	_, after, ok := strings.Cut(line, "Merge conflict in ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(after)
}

func getOrCreateFile(m map[string]*MergeTreeFile, order *[]string, path string) *MergeTreeFile {
	if f, ok := m[path]; ok {
		return f
	}
	f := &MergeTreeFile{Path: path, Status: FileStatusUnknown}
	m[path] = f
	*order = append(*order, path)
	return f
}

// CheckUntrackedCollisions compares the stash's untracked files against the
// working tree. Returns paths that exist in both (FR-10.6a).
func CheckUntrackedCollisions(ctx context.Context, runner GitRunner, stashCommit string) ([]UntrackedCollision, error) {
	// Stash commits can have a 3rd parent for untracked files.
	// Use ls-tree (not diff-tree) because the untracked commit is rootless
	// and diff-tree produces no output for commits without parents.
	untrackedFiles, exitCode, err := runner.RunExitCode(ctx, "ls-tree", "--name-only", "-r", stashCommit+"^3")
	if err != nil || exitCode != 0 {
		// No untracked parent — stash was created without --include-untracked.
		return nil, nil
	}

	existingFiles, err := runner.Run(ctx, "ls-files")
	if err != nil {
		return nil, fmt.Errorf("ls-files: %w", err)
	}

	existing := make(map[string]struct{})
	for f := range strings.SplitSeq(strings.TrimSpace(existingFiles), "\n") {
		if f != "" {
			existing[f] = struct{}{}
		}
	}

	var collisions []UntrackedCollision
	for f := range strings.SplitSeq(strings.TrimSpace(untrackedFiles), "\n") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := existing[f]; ok {
			collisions = append(collisions, UntrackedCollision{Path: f})
		}
	}

	return collisions, nil
}
