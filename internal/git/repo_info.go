package git

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

const (
	repoInfoKeyBare             = "layout.bare"
	repoInfoKeyShallow          = "layout.shallow"
	repoInfoKeyObjectFormat     = "object.format"
	repoInfoKeyReferencesFormat = "references.format"
)

// RepoInfo holds repository metadata reported by `git repo info`.
type RepoInfo struct {
	Available        bool
	Bare             bool
	Shallow          bool
	ObjectFormat     string
	ReferencesFormat string
}

// LoadRepoInfo discovers supported `git repo info` keys and loads the metadata
// nidhi can display. It expects Git 2.54+ because it uses `git repo info --keys`.
func LoadRepoInfo(ctx context.Context, runner GitRunner) (RepoInfo, error) {
	keyOutput, exitCode, err := runner.RunExitCode(ctx, "repo", "info", "--keys", "--format=lines")
	if err != nil {
		return RepoInfo{}, fmt.Errorf("git repo info --keys: %w", err)
	}
	if exitCode != 0 {
		return RepoInfo{}, fmt.Errorf("git repo info --keys failed (exit %d)", exitCode)
	}

	available := parseRepoInfoKeys(keyOutput)
	wanted := filterAvailableRepoInfoKeys(available, []string{
		repoInfoKeyBare,
		repoInfoKeyShallow,
		repoInfoKeyObjectFormat,
		repoInfoKeyReferencesFormat,
	})

	info := RepoInfo{Available: true}
	if len(wanted) == 0 {
		return info, nil
	}

	args := append([]string{"repo", "info", "--format=lines"}, wanted...)
	output, exitCode, err := runner.RunExitCode(ctx, args...)
	if err != nil {
		return RepoInfo{}, fmt.Errorf("git repo info: %w", err)
	}
	if exitCode != 0 {
		return RepoInfo{}, fmt.Errorf("git repo info failed (exit %d)", exitCode)
	}

	for key, value := range parseRepoInfoLines(output) {
		switch key {
		case repoInfoKeyBare:
			info.Bare = value == "true"
		case repoInfoKeyShallow:
			info.Shallow = value == "true"
		case repoInfoKeyObjectFormat:
			info.ObjectFormat = value
		case repoInfoKeyReferencesFormat:
			info.ReferencesFormat = value
		}
	}

	return info, nil
}

func parseRepoInfoKeys(output string) map[string]bool {
	keys := make(map[string]bool)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		keys[line] = true
	}
	return keys
}

func filterAvailableRepoInfoKeys(available map[string]bool, wanted []string) []string {
	keys := make([]string, 0, len(wanted))
	for _, key := range wanted {
		if available[key] {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func parseRepoInfoLines(output string) map[string]string {
	values := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}
