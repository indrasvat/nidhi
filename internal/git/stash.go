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
type Stash struct {
	Index        int
	SHA          string
	ShortSHA     string
	Message      string
	RawMessage   string
	Branch       string
	Date         time.Time
	FileCount    int
	Insertions   int
	Deletions    int
	IsStale      bool
	HasUntracked bool
}

const stashListFormat = "%H%x00%h%x00%gs%x00%aI"

var wipPattern = regexp.MustCompile(`^WIP on (.+): ([0-9a-f]+) (.*)$`)
var onBranchPattern = regexp.MustCompile(`^On (.+): (.+)$`)

// ParseStashList parses the output of `git stash list --format=<format>`.
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
			continue
		}
		stashes = append(stashes, s)
	}

	return stashes
}

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
		date, err = time.Parse("2006-01-02T15:04:05-07:00", dateStr)
		if err != nil {
			date = time.Time{}
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

	if !date.IsZero() && staleThreshold > 0 {
		s.IsStale = now.Sub(date) > staleThreshold
	}

	return s, nil
}

func extractBranch(rawMessage string) string {
	if m := wipPattern.FindStringSubmatch(rawMessage); m != nil {
		return m[1]
	}
	if m := onBranchPattern.FindStringSubmatch(rawMessage); m != nil {
		return m[1]
	}
	return ""
}

func generateMessage(rawMessage string) string {
	if m := wipPattern.FindStringSubmatch(rawMessage); m != nil {
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

// GenerateAutoMessage creates a summary message from diff stats (FR-01.4).
func GenerateAutoMessage(fileCount, insertions, deletions int, topDirs []string) string {
	var b strings.Builder

	if fileCount == 1 {
		b.WriteString("1 file")
	} else {
		fmt.Fprintf(&b, "%d files", fileCount)
	}

	fmt.Fprintf(&b, ": +%d/-%d", insertions, deletions)

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

// ListStashes runs `git stash list` and parses results.
func ListStashes(ctx context.Context, runner GitRunner, staleThreshold time.Duration) ([]Stash, error) {
	output, err := runner.Run(ctx, "stash", "list", "--format="+stashListFormat)
	if err != nil {
		return nil, fmt.Errorf("listing stashes: %w", err)
	}

	stashes := ParseStashList(output, staleThreshold)

	for i := range stashes {
		if err := enrichDiffStats(ctx, runner, &stashes[i]); err != nil {
			continue
		}
	}

	return stashes, nil
}

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

	untrackedOut, err := runner.Run(ctx, "stash", "show", "--include-untracked", "--name-only", ref)
	if err == nil {
		untrackedLines := strings.Split(strings.TrimSpace(untrackedOut), "\n")
		if len(untrackedLines) > s.FileCount {
			s.HasUntracked = true
		}
	}

	return nil
}
