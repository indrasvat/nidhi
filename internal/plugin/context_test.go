package plugin_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

type mockGitRunner struct{}

func (m *mockGitRunner) Run(_ context.Context, _ ...string) (string, error)        { return "", nil }
func (m *mockGitRunner) RunLines(_ context.Context, _ ...string) ([]string, error) { return nil, nil }
func (m *mockGitRunner) RunExitCode(_ context.Context, _ ...string) (string, int, error) {
	return "", 0, nil
}

type mockStashCache struct{}

func (m *mockStashCache) List(_ context.Context) ([]plugin.Stash, error)   { return nil, nil }
func (m *mockStashCache) Diff(_ context.Context, _ string) (string, error) { return "", nil }
func (m *mockStashCache) Invalidate()                                      {}

type mockConfigStore struct{}

func (m *mockConfigStore) GetString(_ string) string { return "" }
func (m *mockConfigStore) GetInt(_ string) int       { return 0 }
func (m *mockConfigStore) GetBool(_ string) bool     { return false }

type mockEventBus struct{}

func (m *mockEventBus) Publish(_ plugin.Event)                   {}
func (m *mockEventBus) Subscribe(_ string, _ func(plugin.Event)) {}

type mockTheme struct{}

func (m *mockTheme) Color(_ string) string { return "#000000" }

func validArgs() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
	return &mockGitRunner{}, &mockStashCache{}, &mockConfigStore{}, &mockEventBus{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		plugin.GitVersion{Major: 2, Minor: 53, Raw: "git version 2.53.0"},
		&mockTheme{}
}

func TestNewPluginContext_Success(t *testing.T) {
	git, cache, cfg, events, logger, gitVer, theme := validArgs()
	ctx, err := plugin.NewPluginContext(git, cache, cfg, events, logger, gitVer, theme)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if ctx.Git == nil || ctx.Cache == nil || ctx.Config == nil || ctx.Events == nil || ctx.Logger == nil || ctx.Theme == nil {
		t.Error("fields should not be nil")
	}
}

func TestNewPluginContext_NilFields(t *testing.T) {
	git, cache, cfg, events, logger, gitVer, theme := validArgs()

	tests := []struct {
		name   string
		modify func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme)
	}{
		{"nil GitRunner", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return nil, cache, cfg, events, logger, gitVer, theme
		}},
		{"nil StashCache", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, nil, cfg, events, logger, gitVer, theme
		}},
		{"nil ConfigStore", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, nil, events, logger, gitVer, theme
		}},
		{"nil EventBus", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, cfg, nil, logger, gitVer, theme
		}},
		{"nil Logger", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, cfg, events, nil, gitVer, theme
		}},
		{"nil Theme", func() (plugin.GitRunner, plugin.StashCache, plugin.ConfigStore, plugin.EventBus, *slog.Logger, plugin.GitVersion, plugin.Theme) {
			return git, cache, cfg, events, logger, gitVer, nil
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, ca, co, ev, lo, gv, th := tt.modify()
			_, err := plugin.NewPluginContext(g, ca, co, ev, lo, gv, th)
			if err == nil {
				t.Error("should error for nil field")
			}
		})
	}
}
