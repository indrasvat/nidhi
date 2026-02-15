package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/config"
	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/conflict"
)

// Build metadata injected via ldflags at compile time.
// See Makefile LDFLAGS: -X main.version=... -X main.commit=... -X main.date=...
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("nidhi %s (commit: %s, built: %s)\n", version, commit, date)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "nidhi: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load config.
	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up logger.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Detect working directory.
	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Set up git runner and detect version.
	runner := git.NewDefaultRunner(workDir, logger)
	ctx, cancel := context.WithTimeout(context.Background(), git.DefaultTimeout)
	defer cancel()

	gitVer, err := git.DetectVersion(ctx, runner)
	if err != nil {
		return fmt.Errorf("detecting git version: %w", err)
	}

	// Detect current branch.
	branch, _ := runner.Run(ctx, "rev-parse", "--abbrev-ref", "HEAD")

	// Build services.
	bus := core.NewBus()
	gitCache := git.NewDefaultStashCache(runner, cfg.StaleThreshold(), cfg.Performance.DiffCacheSize)

	pluginGitVer := plugin.GitVersion{
		Major: gitVer.Major,
		Minor: gitVer.Minor,
		Patch: gitVer.Patch,
		Raw:   gitVer.Raw,
	}

	pctx, err := plugin.NewPluginContext(
		runner,
		&cacheAdapter{inner: gitCache},
		&configAdapter{cfg: cfg},
		bus,
		logger,
		pluginGitVer,
		&themeAdapter{},
	)
	if err != nil {
		return fmt.Errorf("creating plugin context: %w", err)
	}

	// Create registries and register built-in plugins.
	keyHandlers := plugin.NewRegistry[plugin.KeyHandler]()
	screenProviders := plugin.NewRegistry[plugin.ScreenProvider]()
	stashHooks := plugin.NewRegistry[plugin.StashHook]()

	// Register conflict preview plugin.
	conflictPlugin := conflict.New()
	if err := conflictPlugin.Init(pctx); err != nil {
		logger.Error("failed to init conflict plugin", "error", err)
	} else {
		_ = screenProviders.Register(conflictPlugin, 100)
		_ = stashHooks.Register(conflictPlugin, 100)
		logger.Info("registered conflict preview plugin")
	}

	// Create initial state.
	state := core.NewAppState(workDir, branch, pluginGitVer)

	// Create the model.
	model := core.New(state, pctx, bus, logger, keyHandlers, screenProviders, stashHooks)

	// Run BubbleTea.
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}

// configAdapter wraps config.Config to implement plugin.ConfigStore.
type configAdapter struct {
	cfg config.Config
}

func (a *configAdapter) GetString(key string) string {
	switch key {
	case "icons":
		return a.cfg.General.Icons
	case "theme":
		return a.cfg.Theme.Name
	case "log_level":
		return a.cfg.Log.Level
	case "export_ref":
		return a.cfg.Export.Ref
	case "export_remote":
		return a.cfg.Export.Remote
	default:
		return ""
	}
}

func (a *configAdapter) GetInt(key string) int {
	switch key {
	case "stale_days":
		return a.cfg.General.StaleDays
	case "preload_diffs":
		return a.cfg.Performance.PreloadDiffs
	case "diff_cache_size":
		return a.cfg.Performance.DiffCacheSize
	default:
		return 0
	}
}

func (a *configAdapter) GetBool(key string) bool {
	switch key {
	case "keep_index":
		return a.cfg.General.KeepIndex
	case "auto_message":
		return a.cfg.General.AutoMessage
	default:
		return false
	}
}

// cacheAdapter bridges git.DefaultStashCache to plugin.StashCache
// by converting git.Stash → plugin.Stash.
type cacheAdapter struct {
	inner *git.DefaultStashCache
}

func (a *cacheAdapter) List(ctx context.Context) ([]plugin.Stash, error) {
	gitStashes, err := a.inner.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.Stash, len(gitStashes))
	for i, gs := range gitStashes {
		out[i] = plugin.Stash{
			Index:        gs.Index,
			SHA:          gs.SHA,
			ShortSHA:     gs.ShortSHA,
			Message:      gs.Message,
			RawMessage:   gs.RawMessage,
			Branch:       gs.Branch,
			Date:         gs.Date,
			FileCount:    gs.FileCount,
			Insertions:   gs.Insertions,
			Deletions:    gs.Deletions,
			IsStale:      gs.IsStale,
			HasUntracked: gs.HasUntracked,
		}
	}
	return out, nil
}

func (a *cacheAdapter) Diff(ctx context.Context, sha string) (string, error) {
	return a.inner.Diff(ctx, sha)
}

func (a *cacheAdapter) Invalidate() {
	a.inner.Invalidate()
}

// themeAdapter wraps the Agni theme to implement plugin.Theme.
type themeAdapter struct{}

func (a *themeAdapter) Color(_ string) string { return "" }

func printUsage() {
	fmt.Print(`nidhi -- purpose-built TUI for git stash mastery

Usage:
  nidhi [flags]

Flags:
  -h, --help              Show this help message
  -v, --version           Show version information
      --log-level string  Log level (off, error, warn, info, debug)
      --trace-git         Log all git commands with args, exit code, duration
      --debug             Print startup timing breakdown and exit
      --no-color          Disable all colors
      --no-animation      Disable animations
      --icons string      Icon set: auto (default), nerd, ascii
  -C, --directory string  Run as if started in <path>
`)
}
