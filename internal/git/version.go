package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Feature constants for version-gated capabilities.
const (
	FeatureBranchShowCurrent = "branch-show-current"
	FeatureMergeTree         = "merge-tree"
	FeatureStashExportImport = "stash-export-import"
)

var featureMinVersions = map[string]GitVersion{
	FeatureBranchShowCurrent: {Major: 2, Minor: 22, Patch: 0},
	FeatureMergeTree:         {Major: 2, Minor: 38, Patch: 0},
	FeatureStashExportImport: {Major: 2, Minor: 51, Patch: 0},
}

// GitVersion represents a parsed git version.
type GitVersion struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

func (v GitVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v GitVersion) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0
}

func (v GitVersion) AtLeast(major, minor, patch int) bool {
	if v.Major != major {
		return v.Major > major
	}
	if v.Minor != minor {
		return v.Minor > minor
	}
	return v.Patch >= patch
}

func (v GitVersion) Supports(feature string) bool {
	minVer, ok := featureMinVersions[feature]
	if !ok {
		return false
	}
	return v.AtLeast(minVer.Major, minVer.Minor, minVer.Patch)
}

// ParseVersion parses a git version string like "git version 2.53.0".
func ParseVersion(raw string) (GitVersion, error) {
	ver := GitVersion{Raw: raw}

	s := strings.TrimPrefix(raw, "git version ")

	if idx := strings.IndexByte(s, ' '); idx != -1 {
		s = s[:idx]
	}

	parts := strings.SplitN(s, ".", 4)
	if len(parts) < 2 {
		return ver, fmt.Errorf("invalid git version string: %q", raw)
	}

	var err error
	ver.Major, err = strconv.Atoi(parts[0])
	if err != nil {
		return ver, fmt.Errorf("invalid git major version in %q: %w", raw, err)
	}

	ver.Minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return ver, fmt.Errorf("invalid git minor version in %q: %w", raw, err)
	}

	if len(parts) >= 3 {
		ver.Patch, err = strconv.Atoi(parts[2])
		if err != nil {
			ver.Patch = 0
		}
	}

	return ver, nil
}

// DetectVersion runs `git version` and parses the output.
func DetectVersion(ctx context.Context, runner GitRunner) (GitVersion, error) {
	stdout, err := runner.Run(ctx, "version")
	if err != nil {
		return GitVersion{}, fmt.Errorf("detecting git version: %w", err)
	}
	return ParseVersion(stdout)
}
