package git_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/git"
)

type repoInfoRunner struct {
	outputs map[string]repoInfoResult
	calls   [][]string
}

type repoInfoResult struct {
	stdout   string
	exitCode int
	err      error
}

func (r *repoInfoRunner) Run(ctx context.Context, args ...string) (string, error) {
	stdout, _, err := r.RunExitCode(ctx, args...)
	return stdout, err
}

func (r *repoInfoRunner) RunLines(ctx context.Context, args ...string) ([]string, error) {
	stdout, err := r.Run(ctx, args...)
	if err != nil || stdout == "" {
		return nil, err
	}
	return strings.Split(stdout, "\n"), nil
}

func (r *repoInfoRunner) RunExitCode(_ context.Context, args ...string) (string, int, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	result, ok := r.outputs[strings.Join(args, "\x00")]
	if !ok {
		return "", 1, nil
	}
	return result.stdout, result.exitCode, result.err
}

func repoInfoKey(args ...string) string {
	return strings.Join(args, "\x00")
}

func TestLoadRepoInfo(t *testing.T) {
	runner := &repoInfoRunner{outputs: map[string]repoInfoResult{
		repoInfoKey("repo", "info", "--keys", "--format=lines"): {
			stdout: "layout.bare\nlayout.shallow\nobject.format\nreferences.format\n",
		},
		repoInfoKey("repo", "info", "--format=lines", "layout.bare", "layout.shallow", "object.format", "references.format"): {
			stdout: "layout.bare=false\nlayout.shallow=true\nobject.format=sha256\nreferences.format=reftable\n",
		},
	}}

	info, err := git.LoadRepoInfo(context.Background(), runner)
	if err != nil {
		t.Fatalf("LoadRepoInfo: %v", err)
	}

	if !info.Available {
		t.Fatal("repo info should be marked available")
	}
	if info.Bare {
		t.Error("Bare = true, want false")
	}
	if !info.Shallow {
		t.Error("Shallow = false, want true")
	}
	if info.ObjectFormat != "sha256" {
		t.Errorf("ObjectFormat = %q, want sha256", info.ObjectFormat)
	}
	if info.ReferencesFormat != "reftable" {
		t.Errorf("ReferencesFormat = %q, want reftable", info.ReferencesFormat)
	}
}

func TestLoadRepoInfoOnlyRequestsAvailableKeys(t *testing.T) {
	runner := &repoInfoRunner{outputs: map[string]repoInfoResult{
		repoInfoKey("repo", "info", "--keys", "--format=lines"): {
			stdout: "object.format\n",
		},
		repoInfoKey("repo", "info", "--format=lines", "object.format"): {
			stdout: "object.format=sha1\n",
		},
	}}

	info, err := git.LoadRepoInfo(context.Background(), runner)
	if err != nil {
		t.Fatalf("LoadRepoInfo: %v", err)
	}
	if info.ObjectFormat != "sha1" {
		t.Errorf("ObjectFormat = %q, want sha1", info.ObjectFormat)
	}

	wantCall := []string{"repo", "info", "--format=lines", "object.format"}
	if !reflect.DeepEqual(runner.calls[1], wantCall) {
		t.Errorf("second call = %v, want %v", runner.calls[1], wantCall)
	}
}

func TestLoadRepoInfoKeyDiscoveryFailure(t *testing.T) {
	runner := &repoInfoRunner{outputs: map[string]repoInfoResult{
		repoInfoKey("repo", "info", "--keys", "--format=lines"): {
			exitCode: 1,
		},
	}}

	if _, err := git.LoadRepoInfo(context.Background(), runner); err == nil {
		t.Fatal("expected error for failing key discovery")
	}
}

func TestLoadRepoInfoCommandError(t *testing.T) {
	runner := &repoInfoRunner{outputs: map[string]repoInfoResult{
		repoInfoKey("repo", "info", "--keys", "--format=lines"): {
			err: errors.New("boom"),
		},
	}}

	if _, err := git.LoadRepoInfo(context.Background(), runner); err == nil {
		t.Fatal("expected command error")
	}
}
