package git

import (
	"bytes"
	"context"
	"errors"
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
	// Run executes a git command and returns stdout. A non-zero exit from git
	// is returned as an error. Use RunExitCode when the caller specifically
	// needs to inspect a non-zero exit code (e.g. merge-tree, where exit 1
	// signals "conflicts found" and is not an error).
	Run(ctx context.Context, args ...string) (string, error)
	// RunLines executes and returns stdout split by newline.
	RunLines(ctx context.Context, args ...string) ([]string, error)
	// RunExitCode executes and returns the exit code without treating
	// non-zero as an error. Use this when the exit code carries information
	// the caller must act on.
	RunExitCode(ctx context.Context, args ...string) (stdout string, exitCode int, err error)
}

// DefaultRunner is the production GitRunner that executes real git commands.
type DefaultRunner struct {
	// GitPath is the path to the git binary. Defaults to "git".
	GitPath string
	// WorkDir is the working directory for git commands.
	WorkDir string
	// Logger for git command tracing.
	Logger *slog.Logger
	// TraceGit enables detailed git command tracing.
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

func (r *DefaultRunner) Run(ctx context.Context, args ...string) (string, error) {
	stdout, exitCode, err := r.run(ctx, args...)
	if err != nil {
		return stdout, err
	}
	if exitCode != 0 {
		return stdout, fmt.Errorf("git %s exited %d", args[0], exitCode)
	}
	return stdout, nil
}

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

func (r *DefaultRunner) RunExitCode(ctx context.Context, args ...string) (string, int, error) {
	return r.run(ctx, args...)
}

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
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

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
		if ctx.Err() != nil {
			return outStr, exitCode, fmt.Errorf("git %s: %w", args[0], ctx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return outStr, exitCode, nil
		}
		return "", exitCode, fmt.Errorf("git %s: %w (stderr: %s)", args[0], err, strings.TrimSpace(stderr.String()))
	}

	return outStr, 0, nil
}
