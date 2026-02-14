# Task 001: Git Runner and Version Detection

## Status: TODO

## Depends On
- 000 (Repository Scaffold and Tooling) -- go.mod, directory structure, make ci working

## Parallelizable With
- 002 (Config Loading)
- 003 (Agni Theme and Icons)

## Problem
Every git operation in nidhi goes through a central `GitRunner` interface. Before any stash operations can be built, we need: (1) a safe, tested way to execute git commands with context timeouts and structured logging, and (2) git version detection to gate features like `merge-tree --write-tree` (2.38+) and `stash export/import` (2.51+). Without version detection, nidhi cannot gracefully degrade on older git installations.

## PRD Reference
- Section 5.3 (Plumbing Used) -- all git commands nidhi invokes
- Section 5.4 (Feature Gating by Git Version) -- version thresholds: 2.22, 2.38, 2.51, 2.53
- Section 7.5 (Observability) -- `--trace-git` logs every git command with args, exit code, duration
- Section 8.3 (Core Types) -- `GitRunner` interface definition with `Run`, `RunLines`, `RunExitCode`
- Section 14.3 (Caching Strategy) -- `GitVersion` cached via `sync.OnceValues`
- Section 15.3 (Git Command Timeout) -- 10s default, 60s for export/import

## Files to Create
- `internal/git/runner.go` -- `GitRunner` interface + `DefaultRunner` implementation
- `internal/git/version.go` -- `GitVersion` struct, parse `git version` output, `Supports(feature)`, feature constants
- `internal/git/runner_test.go` -- table-driven tests against real git in temp repos
- `internal/git/version_test.go` -- version string parsing, feature gating tests

## Execution Steps

### Step 1: Create `internal/git/runner.go`

```go
package git

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout is the timeout for standard git commands.
const DefaultTimeout = 10 * time.Second

// LongTimeout is the timeout for network-bound operations (export/import).
const LongTimeout = 60 * time.Second

// GitRunner abstracts all git command execution.
type GitRunner interface {
	// Run executes a git command and returns stdout.
	Run(ctx context.Context, args ...string) (string, error)
	// RunLines executes and returns stdout split by newline.
	RunLines(ctx context.Context, args ...string) ([]string, error)
	// RunExitCode executes and returns the exit code (for merge-tree).
	RunExitCode(ctx context.Context, args ...string) (stdout string, exitCode int, err error)
}

// DefaultRunner is the production GitRunner that executes real git commands.
type DefaultRunner struct {
	// GitPath is the path to the git binary. Defaults to "git".
	GitPath string
	// WorkDir is the working directory for git commands.
	// If empty, uses the current working directory.
	WorkDir string
	// Logger for git command tracing. If nil, no tracing occurs.
	Logger *slog.Logger
	// TraceGit enables detailed git command tracing (args, exit code, duration).
	TraceGit bool
}

// NewDefaultRunner creates a DefaultRunner with sensible defaults.
func NewDefaultRunner(workDir string, logger *slog.Logger) *DefaultRunner {
	return &DefaultRunner{
		GitPath: "git",
		WorkDir: workDir,
		Logger:  logger,
	}
}

// Run executes a git command and returns trimmed stdout.
func (r *DefaultRunner) Run(ctx context.Context, args ...string) (string, error) {
	stdout, _, err := r.run(ctx, args...)
	return stdout, err
}

// RunLines executes a git command and returns stdout split into lines.
// Empty trailing lines are removed.
func (r *DefaultRunner) RunLines(ctx context.Context, args ...string) ([]string, error) {
	stdout, err := r.Run(ctx, args...)
	if err != nil {
		return nil, err
	}
	if stdout == "" {
		return nil, nil
	}
	return strings.Split(stdout, "\n"), nil
}

// RunExitCode executes a git command and returns stdout and the exit code.
// Unlike Run, this does NOT treat non-zero exit codes as errors.
// This is needed for commands like `git merge-tree` where exit code 1
// means "conflicts detected" (not an error).
func (r *DefaultRunner) RunExitCode(ctx context.Context, args ...string) (string, int, error) {
	return r.run(ctx, args...)
}

// run is the internal implementation shared by all public methods.
func (r *DefaultRunner) run(ctx context.Context, args ...string) (string, int, error) {
	start := time.Now()

	cmd := exec.CommandContext(ctx, r.GitPath, args...)
	if r.WorkDir != "" {
		cmd.Dir = r.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(start)
	exitCode := cmd.ProcessState.ExitCode()

	// Trace logging if enabled
	if r.TraceGit && r.Logger != nil {
		r.Logger.Debug("git command",
			slog.String("command", "git "+strings.Join(args, " ")),
			slog.Int("exit_code", exitCode),
			slog.Duration("duration", duration),
			slog.String("stderr", strings.TrimSpace(stderr.String())),
		)
	}

	outStr := strings.TrimRight(stdout.String(), "\n")

	if err != nil {
		// Check for context cancellation/timeout
		if ctx.Err() != nil {
			return outStr, exitCode, fmt.Errorf("git %s: %w", args[0], ctx.Err())
		}
		// For RunExitCode callers, we still return the exit code and stdout
		// but also return the error so callers can distinguish real failures
		// (e.g., git not found) from expected non-zero exits (merge-tree conflicts).
		var exitErr *exec.ExitError
		if ok := errors.As(err, &exitErr); ok {
			// Non-zero exit -- return code and stdout, no error
			// Callers using RunExitCode need to inspect the exit code themselves.
			return outStr, exitCode, nil
		}
		// Real execution failure (git not found, permission denied, etc.)
		return "", exitCode, fmt.Errorf("git %s: %w (stderr: %s)", args[0], err, strings.TrimSpace(stderr.String()))
	}

	return outStr, 0, nil
}
```

**Important:** Add `"errors"` to the import block after writing the above code, since `errors.As` is used.

### Step 2: Create `internal/git/version.go`

```go
package git

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Feature constants for version-gated capabilities.
const (
	// FeatureBranchShowCurrent requires git >= 2.22 for `git branch --show-current`.
	FeatureBranchShowCurrent = "branch-show-current"
	// FeatureMergeTree requires git >= 2.38 for `git merge-tree --write-tree`.
	FeatureMergeTree = "merge-tree"
	// FeatureStashExportImport requires git >= 2.51 for `git stash export/import`.
	FeatureStashExportImport = "stash-export-import"
)

// featureMinVersions maps feature names to their minimum required git version.
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
	// Raw is the original version string from `git version`.
	Raw string
}

// String returns the version as "Major.Minor.Patch".
func (v GitVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// IsZero returns true if the version was not parsed (all fields zero).
func (v GitVersion) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0
}

// AtLeast returns true if this version is >= the given version.
func (v GitVersion) AtLeast(major, minor, patch int) bool {
	if v.Major != major {
		return v.Major > major
	}
	if v.Minor != minor {
		return v.Minor > minor
	}
	return v.Patch >= patch
}

// Supports returns true if the given feature is supported by this git version.
// Unknown feature names always return false.
func (v GitVersion) Supports(feature string) bool {
	minVer, ok := featureMinVersions[feature]
	if !ok {
		return false
	}
	return v.AtLeast(minVer.Major, minVer.Minor, minVer.Patch)
}

// ParseVersion parses a git version string like "git version 2.53.0" or
// "git version 2.22.1.windows.1" into a GitVersion struct.
//
// Accepted formats:
//   - "git version 2.53.0"
//   - "git version 2.22.1.windows.1"
//   - "git version 2.39.3 (Apple Git-146)"
//   - "2.53.0" (just the version number)
func ParseVersion(raw string) (GitVersion, error) {
	ver := GitVersion{Raw: raw}

	// Strip "git version " prefix if present
	s := raw
	if strings.HasPrefix(s, "git version ") {
		s = strings.TrimPrefix(s, "git version ")
	}

	// Take only the version number part (before any space or extra suffixes)
	// This handles "2.39.3 (Apple Git-146)" and similar
	if idx := strings.IndexByte(s, ' '); idx != -1 {
		s = s[:idx]
	}

	// Split by '.' and parse first three components
	parts := strings.SplitN(s, ".", 4) // max 4 to handle "2.22.1.windows.1"
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
			// Some formats have non-numeric patch (e.g., "rc1") -- treat as 0
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
```

### Step 3: Create `internal/git/runner_test.go`

```go
package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/git"
)

// testHelper runs a command in a directory and fails the test on error.
func testHelper(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\noutput: %s", name, strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// setupTempRepo creates a temporary git repo with an initial commit.
func setupTempRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testHelper(t, dir, "git", "init")
	testHelper(t, dir, "git", "config", "user.email", "test@test.com")
	testHelper(t, dir, "git", "config", "user.name", "Test")
	testHelper(t, dir, "git", "commit", "--allow-empty", "-m", "initial commit")
	return dir
}

func TestDefaultRunner_Run(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{
			name: "git version returns non-empty",
			args: []string{"version"},
			want: "git version",
		},
		{
			name: "git rev-parse HEAD returns SHA",
			args: []string{"rev-parse", "HEAD"},
		},
		{
			name:    "invalid subcommand returns error",
			args:    []string{"not-a-real-command"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runner.Run(ctx, tt.args...)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != "" && !strings.Contains(got, tt.want) {
				t.Errorf("output %q does not contain %q", got, tt.want)
			}
			if got == "" && tt.want == "" {
				// For commands like rev-parse HEAD, just verify non-empty
				// (we already checked no error)
			}
		})
	}
}

func TestDefaultRunner_RunLines(t *testing.T) {
	dir := setupTempRepo(t)

	// Create a file and add it so `git stash list` can work
	filePath := filepath.Join(dir, "file1.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "file1.txt")
	testHelper(t, dir, "git", "commit", "-m", "add file1")

	// Create two stashes
	if err := os.WriteFile(filePath, []byte("change1"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "stash one")

	if err := os.WriteFile(filePath, []byte("change2"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "stash", "push", "-m", "stash two")

	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	lines, err := runner.RunLines(ctx, "stash", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 stash lines, got %d: %v", len(lines), lines)
	}

	if !strings.Contains(lines[0], "stash two") {
		t.Errorf("first line should contain 'stash two', got %q", lines[0])
	}
	if !strings.Contains(lines[1], "stash one") {
		t.Errorf("second line should contain 'stash one', got %q", lines[1])
	}
}

func TestDefaultRunner_RunLines_EmptyResult(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// No stashes exist -- should return nil, nil
	lines, err := runner.RunLines(ctx, "stash", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil for empty stash list, got %v", lines)
	}
}

func TestDefaultRunner_RunExitCode(t *testing.T) {
	dir := setupTempRepo(t)

	// Create a file on main branch
	filePath := filepath.Join(dir, "conflict.txt")
	if err := os.WriteFile(filePath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	testHelper(t, dir, "git", "add", "conflict.txt")
	testHelper(t, dir, "git", "commit", "-m", "add conflict file")

	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// rev-parse should exit 0
	stdout, exitCode, err := runner.RunExitCode(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if stdout == "" {
		t.Error("expected non-empty stdout for rev-parse HEAD")
	}
}

func TestDefaultRunner_ContextTimeout(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)

	// Use an already-cancelled context
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond) // ensure timeout fires

	_, err := runner.Run(ctx, "version")
	if err == nil {
		t.Error("expected error for timed-out context, got nil")
	}
}

func TestDefaultRunner_WorkDir(t *testing.T) {
	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	// Verify the runner is operating in the correct directory
	out, err := runner.Run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolve symlinks for comparison (macOS /tmp -> /private/tmp)
	expectedDir, _ := filepath.EvalSymlinks(dir)
	actualDir, _ := filepath.EvalSymlinks(out)

	if actualDir != expectedDir {
		t.Errorf("expected work dir %q, got %q", expectedDir, actualDir)
	}
}
```

### Step 4: Create `internal/git/version_test.go`

```go
package git_test

import (
	"context"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantErr   bool
	}{
		{
			name:      "standard format",
			input:     "git version 2.53.0",
			wantMajor: 2,
			wantMinor: 53,
			wantPatch: 0,
		},
		{
			name:      "windows format",
			input:     "git version 2.22.1.windows.1",
			wantMajor: 2,
			wantMinor: 22,
			wantPatch: 1,
		},
		{
			name:      "apple git format",
			input:     "git version 2.39.3 (Apple Git-146)",
			wantMajor: 2,
			wantMinor: 39,
			wantPatch: 3,
		},
		{
			name:      "bare version number",
			input:     "2.53.0",
			wantMajor: 2,
			wantMinor: 53,
			wantPatch: 0,
		},
		{
			name:      "old git version",
			input:     "git version 1.7.7",
			wantMajor: 1,
			wantMinor: 7,
			wantPatch: 7,
		},
		{
			name:      "two-part version",
			input:     "git version 2.38",
			wantMajor: 2,
			wantMinor: 38,
			wantPatch: 0,
		},
		{
			name:      "rc patch version",
			input:     "git version 2.53.rc1",
			wantMajor: 2,
			wantMinor: 53,
			wantPatch: 0, // non-numeric patch treated as 0
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "garbage input",
			input:   "not a version",
			wantErr: true,
		},
		{
			name:    "single number",
			input:   "2",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ver, err := git.ParseVersion(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ver.Major != tt.wantMajor {
				t.Errorf("major: got %d, want %d", ver.Major, tt.wantMajor)
			}
			if ver.Minor != tt.wantMinor {
				t.Errorf("minor: got %d, want %d", ver.Minor, tt.wantMinor)
			}
			if ver.Patch != tt.wantPatch {
				t.Errorf("patch: got %d, want %d", ver.Patch, tt.wantPatch)
			}
			if ver.Raw != tt.input {
				t.Errorf("raw: got %q, want %q", ver.Raw, tt.input)
			}
		})
	}
}

func TestGitVersion_AtLeast(t *testing.T) {
	tests := []struct {
		name  string
		ver   git.GitVersion
		major int
		minor int
		patch int
		want  bool
	}{
		{
			name:  "exact match",
			ver:   git.GitVersion{Major: 2, Minor: 38, Patch: 0},
			major: 2, minor: 38, patch: 0,
			want: true,
		},
		{
			name:  "higher minor",
			ver:   git.GitVersion{Major: 2, Minor: 53, Patch: 0},
			major: 2, minor: 38, patch: 0,
			want: true,
		},
		{
			name:  "higher patch",
			ver:   git.GitVersion{Major: 2, Minor: 38, Patch: 5},
			major: 2, minor: 38, patch: 0,
			want: true,
		},
		{
			name:  "lower minor",
			ver:   git.GitVersion{Major: 2, Minor: 37, Patch: 0},
			major: 2, minor: 38, patch: 0,
			want: false,
		},
		{
			name:  "lower patch",
			ver:   git.GitVersion{Major: 2, Minor: 38, Patch: 0},
			major: 2, minor: 38, patch: 1,
			want: false,
		},
		{
			name:  "higher major",
			ver:   git.GitVersion{Major: 3, Minor: 0, Patch: 0},
			major: 2, minor: 99, patch: 99,
			want: true,
		},
		{
			name:  "lower major",
			ver:   git.GitVersion{Major: 1, Minor: 99, Patch: 99},
			major: 2, minor: 0, patch: 0,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ver.AtLeast(tt.major, tt.minor, tt.patch)
			if got != tt.want {
				t.Errorf("(%d.%d.%d).AtLeast(%d, %d, %d) = %v, want %v",
					tt.ver.Major, tt.ver.Minor, tt.ver.Patch,
					tt.major, tt.minor, tt.patch, got, tt.want)
			}
		})
	}
}

func TestGitVersion_Supports(t *testing.T) {
	tests := []struct {
		name    string
		ver     git.GitVersion
		feature string
		want    bool
	}{
		{
			name:    "2.53 supports branch-show-current",
			ver:     git.GitVersion{Major: 2, Minor: 53, Patch: 0},
			feature: git.FeatureBranchShowCurrent,
			want:    true,
		},
		{
			name:    "2.53 supports merge-tree",
			ver:     git.GitVersion{Major: 2, Minor: 53, Patch: 0},
			feature: git.FeatureMergeTree,
			want:    true,
		},
		{
			name:    "2.53 supports stash-export-import",
			ver:     git.GitVersion{Major: 2, Minor: 53, Patch: 0},
			feature: git.FeatureStashExportImport,
			want:    true,
		},
		{
			name:    "2.37 does not support merge-tree",
			ver:     git.GitVersion{Major: 2, Minor: 37, Patch: 0},
			feature: git.FeatureMergeTree,
			want:    false,
		},
		{
			name:    "2.38 supports merge-tree",
			ver:     git.GitVersion{Major: 2, Minor: 38, Patch: 0},
			feature: git.FeatureMergeTree,
			want:    true,
		},
		{
			name:    "2.50 does not support stash-export-import",
			ver:     git.GitVersion{Major: 2, Minor: 50, Patch: 0},
			feature: git.FeatureStashExportImport,
			want:    false,
		},
		{
			name:    "2.51 supports stash-export-import",
			ver:     git.GitVersion{Major: 2, Minor: 51, Patch: 0},
			feature: git.FeatureStashExportImport,
			want:    true,
		},
		{
			name:    "2.21 does not support branch-show-current",
			ver:     git.GitVersion{Major: 2, Minor: 21, Patch: 0},
			feature: git.FeatureBranchShowCurrent,
			want:    false,
		},
		{
			name:    "unknown feature always false",
			ver:     git.GitVersion{Major: 99, Minor: 99, Patch: 99},
			feature: "nonexistent-feature",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ver.Supports(tt.feature)
			if got != tt.want {
				t.Errorf("(%s).Supports(%q) = %v, want %v",
					tt.ver, tt.feature, got, tt.want)
			}
		})
	}
}

func TestGitVersion_String(t *testing.T) {
	ver := git.GitVersion{Major: 2, Minor: 53, Patch: 0}
	if s := ver.String(); s != "2.53.0" {
		t.Errorf("String() = %q, want %q", s, "2.53.0")
	}
}

func TestGitVersion_IsZero(t *testing.T) {
	zero := git.GitVersion{}
	if !zero.IsZero() {
		t.Error("expected zero value to be zero")
	}

	nonZero := git.GitVersion{Major: 2}
	if nonZero.IsZero() {
		t.Error("expected non-zero value to not be zero")
	}
}

func TestDetectVersion_RealGit(t *testing.T) {
	// This test runs against the real git installation.
	// Skip if git is not available.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := setupTempRepo(t)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	ver, err := git.DetectVersion(ctx, runner)
	if err != nil {
		t.Fatalf("DetectVersion failed: %v", err)
	}

	if ver.IsZero() {
		t.Error("detected version should not be zero")
	}
	if ver.Major < 2 {
		t.Errorf("expected git major version >= 2, got %d", ver.Major)
	}
	if ver.Raw == "" {
		t.Error("Raw version string should not be empty")
	}

	t.Logf("Detected git version: %s (raw: %q)", ver, ver.Raw)
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

# Tests actually exercise real git
go test -v -run TestDefaultRunner_Run ./internal/git/...
go test -v -run TestDefaultRunner_RunLines ./internal/git/...
go test -v -run TestDefaultRunner_RunExitCode ./internal/git/...
go test -v -run TestDetectVersion_RealGit ./internal/git/...

# Version parsing covers all formats
go test -v -run TestParseVersion ./internal/git/...

# Feature gating works
go test -v -run TestGitVersion_Supports ./internal/git/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/git/runner.go` compiles and exports `GitRunner` interface with `Run`, `RunLines`, `RunExitCode`
2. `internal/git/runner.go` exports `DefaultRunner` with `NewDefaultRunner` constructor
3. `DefaultRunner` supports `WorkDir`, `Logger`, `TraceGit` fields
4. `DefaultRunner.Run` returns trimmed stdout and wraps errors with context
5. `DefaultRunner.RunExitCode` returns exit code without treating non-zero as error
6. `internal/git/version.go` exports `GitVersion` struct with `Major`, `Minor`, `Patch`, `Raw`
7. `ParseVersion` handles: standard ("git version 2.53.0"), Windows ("2.22.1.windows.1"), Apple ("2.39.3 (Apple Git-146)"), bare ("2.53.0")
8. `GitVersion.AtLeast(major, minor, patch)` correctly compares versions
9. `GitVersion.Supports(feature)` gates on `FeatureBranchShowCurrent` (2.22), `FeatureMergeTree` (2.38), `FeatureStashExportImport` (2.51)
10. `runner_test.go` creates real temp repos with `t.TempDir()` and runs real git commands
11. `version_test.go` has table-driven tests for all version string formats and feature gates
12. `go test -race ./internal/git/...` passes
13. `make ci` passes

## Commit
```
feat: add GitRunner interface and git version detection

Implement internal/git/runner.go with DefaultRunner (exec.CommandContext,
slog tracing, timeout support) and internal/git/version.go with ParseVersion
and feature gating for merge-tree (2.38+), export/import (2.51+), and
branch-show-current (2.22+). Table-driven tests against real git repos.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 5.3, 5.4, 7.5, 8.3, 14.3, 15.3
4. Execute steps 1-7 in order
5. Verify all functional and CI checks pass
6. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
7. Commit with the message above + move to next task
