package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/config"
	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/conflict"
	"github.com/indrasvat/nidhi/internal/plugins/filter"
	"github.com/indrasvat/nidhi/internal/plugins/rename"
	"github.com/indrasvat/nidhi/internal/plugins/reorder"
	"github.com/indrasvat/nidhi/internal/plugins/search"
	"github.com/indrasvat/nidhi/internal/plugins/stale"
	pluginsync "github.com/indrasvat/nidhi/internal/plugins/sync"
	"github.com/indrasvat/nidhi/internal/plugins/undo"
	"github.com/indrasvat/nidhi/internal/ui/components"
	"github.com/indrasvat/nidhi/internal/ui/layout"
	"github.com/indrasvat/nidhi/internal/ui/screens"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// Build metadata injected via ldflags at compile time.
// See Makefile LDFLAGS: -X main.version=... -X main.commit=... -X main.date=...
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	flags := parseFlags()

	if err := run(flags); err != nil {
		fmt.Fprintf(os.Stderr, "nidhi: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config.CLIFlags {
	var flags config.CLIFlags

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			fmt.Printf("nidhi %s (commit: %s, built: %s)\n", version, commit, date)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		case "--debug":
			flags.Debug = boolPtr(true)
		case "--trace-git":
			flags.TraceGit = boolPtr(true)
		case "--no-color":
			flags.NoColor = boolPtr(true)
		case "--no-animation":
			flags.NoAnimation = boolPtr(true)
		case "--log-level":
			if i+1 < len(args) {
				i++
				flags.LogLevel = &args[i]
			}
		case "--icons":
			if i+1 < len(args) {
				i++
				flags.Icons = &args[i]
			}
		case "-C", "--directory":
			if i+1 < len(args) {
				i++
				flags.Directory = &args[i]
			}
		}
	}

	return flags
}

func boolPtr(b bool) *bool { return &b }

func run(flags config.CLIFlags) error {
	var timing *config.DebugTiming
	if flags.Debug != nil && *flags.Debug {
		timing = config.NewDebugTiming()
	}

	// Apply -C / --directory BEFORE loading config so that nidhi.* git config
	// values are read from the target repository, not the caller's CWD.
	if flags.Directory != nil && *flags.Directory != "" {
		if err := os.Chdir(*flags.Directory); err != nil {
			return fmt.Errorf("changing to directory %s: %w", *flags.Directory, err)
		}
	}

	// Load config (now reads git config from the target repo).
	cfg, err := config.Load(flags)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Set up structured logging.
	logger, logCleanup, err := config.SetupLogging(&cfg)
	if err != nil {
		// Fall back to stderr logger if log file setup fails.
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelError,
		}))
		logger.Error("failed to set up log file, falling back to stderr", "error", err)
	} else {
		defer logCleanup()
	}

	// Resolve the final working directory. cfg.Directory may have been set by
	// the config file even when --directory was not passed; honor that path
	// here for the runner, but only Chdir if we did not already do so above.
	workDir := cfg.Directory
	switch {
	case flags.Directory != nil && *flags.Directory != "":
		// Already changed to *flags.Directory above.
		workDir = *flags.Directory
	case workDir != "":
		if err := os.Chdir(workDir); err != nil {
			return fmt.Errorf("changing to directory %s: %w", workDir, err)
		}
	default:
		workDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Set up git runner and detect version.
	gitStart := time.Now()
	runner := git.NewDefaultRunner(workDir, logger)
	ctx, cancel := context.WithTimeout(context.Background(), git.DefaultTimeout)
	defer cancel()

	gitVer, err := git.DetectVersion(ctx, runner)
	if err != nil {
		return fmt.Errorf("detecting git version: %w", err)
	}

	// Detect current branch.
	branch, _ := runner.Run(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if timing != nil {
		timing.Since("git detection", gitStart)
	}

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
	pluginStart := time.Now()
	keyHandlers := plugin.NewRegistry[plugin.KeyHandler]()
	screenProviders := plugin.NewRegistry[plugin.ScreenProvider]()
	stashHooks := plugin.NewRegistry[plugin.StashHook]()
	th := theme.NewAgni()

	// Register conflict preview plugin.
	conflictPlugin := conflict.New()
	if err := conflictPlugin.Init(pctx); err != nil {
		logger.Error("failed to init conflict plugin", "error", err)
	} else {
		_ = screenProviders.Register(conflictPlugin, 100)
		_ = stashHooks.Register(conflictPlugin, 100)
		logger.Info("registered conflict preview plugin")
	}

	// Register undo & recovery plugin.
	undoPlugin := undo.New()
	if err := undoPlugin.Init(pctx); err != nil {
		logger.Error("failed to init undo plugin", "error", err)
	} else {
		_ = keyHandlers.Register(undoPlugin, 100)
		_ = stashHooks.Register(undoPlugin, 100)
		logger.Info("registered undo plugin")
	}

	// Register rename plugin.
	renamePlugin := rename.New()
	if err := renamePlugin.Init(pctx); err != nil {
		logger.Error("failed to init rename plugin", "error", err)
	} else {
		_ = keyHandlers.Register(renamePlugin, 100)
		logger.Info("registered rename plugin")
	}

	// Register search plugin.
	searchPlugin := search.New(th)
	if err := searchPlugin.Init(pctx); err != nil {
		logger.Error("failed to init search plugin", "error", err)
	} else {
		_ = keyHandlers.Register(searchPlugin, 100)
		_ = screenProviders.Register(searchPlugin, 100)
		logger.Info("registered search plugin")
	}

	// Register filter plugin.
	filterPlugin := filter.New()
	if err := filterPlugin.Init(pctx); err != nil {
		logger.Error("failed to init filter plugin", "error", err)
	} else {
		_ = keyHandlers.Register(filterPlugin, 90)
		logger.Info("registered filter plugin")
	}

	// Register stale detection plugin.
	stalePlugin := stale.New()
	if err := stalePlugin.Init(pctx); err != nil {
		logger.Error("failed to init stale plugin", "error", err)
	} else {
		logger.Info("registered stale detection plugin")
	}
	_ = stalePlugin // Passive plugin — no registry registration needed.

	// Register reorder plugin.
	reorderPlugin := reorder.New()
	if err := reorderPlugin.Init(pctx); err != nil {
		logger.Error("failed to init reorder plugin", "error", err)
	} else {
		_ = keyHandlers.Register(reorderPlugin, 100)
		logger.Info("registered reorder plugin")
	}

	// Register export/import sync plugin.
	syncPlugin := pluginsync.New(th)
	if err := syncPlugin.Init(pctx); err != nil {
		logger.Error("failed to init sync plugin", "error", err)
	} else {
		_ = keyHandlers.Register(syncPlugin, 100)
		_ = screenProviders.Register(syncPlugin, 100)
		logger.Info("registered sync plugin")
	}

	// Register new stash screen.
	newStashScreen := screens.NewNewStashScreen(th)
	if err := newStashScreen.Init(pctx); err != nil {
		logger.Error("failed to init new stash screen", "error", err)
	} else {
		_ = screenProviders.Register(newStashScreen, 100)
		logger.Info("registered new stash screen")
	}

	// Recover from any interrupted rename operations.
	if rename.HasIncompleteOperation() {
		recovered, recErr := rename.RecoverFromJournal(ctx, runner)
		if recErr != nil {
			logger.Error("failed to recover from interrupted rename", "error", recErr)
		} else if recovered > 0 {
			logger.Info("recovered stashes from interrupted rename", "count", recovered)
		}
	}

	// Recover from any interrupted reorder operations.
	if reorder.HasIncompleteOperation() {
		recovered, recErr := reorder.RecoverFromJournal(ctx, reorder.DefaultJournalPath(), runner)
		if recErr != nil {
			logger.Error("failed to recover from interrupted reorder", "error", recErr)
		} else if recovered > 0 {
			logger.Info("recovered stashes from interrupted reorder", "count", recovered)
		}
	}

	if timing != nil {
		timing.Since("plugin init", pluginStart)
	}

	// Create initial state.
	state := core.NewAppState(workDir, branch, pluginGitVer)

	// Create the model.
	model := core.New(state, pctx, bus, logger, keyHandlers, screenProviders, stashHooks)

	// Wire real UI screens into the core model.
	listScreen := screens.NewListScreen(th)
	previewScreen := screens.NewPreviewScreen(listScreen, &cacheAdapter{inner: gitCache}, th)
	detailScreen := screens.NewDetailScreen(th)
	helpOverlay := screens.NewHelpOverlay(th)
	toastModel := components.NewToastModel(th)

	repoName := filepath.Base(workDir)
	useNerd := cfg.General.Icons == "nerd"

	model.BgColor = th.BgDeep()
	model.Version = version
	model.Commit = commit
	model.Welcome = true

	model.UI = &uiRenderer{
		list:       listScreen,
		preview:    previewScreen,
		detail:     detailScreen,
		newStash:   newStashScreen,
		help:       helpOverlay,
		toast:      &toastModel,
		statusBar:  components.NewStatusBar(th),
		footer:     components.NewFooter(th),
		theme:      th,
		repoName:   repoName,
		useNerd:    useNerd,
		cache:      &cacheAdapter{inner: gitCache},
		appVersion: version,
		appCommit:  commit,
		screens:    screenProviders,
		stashOps:   git.NewStashOps(runner, gitCache),
		stashHooks: stashHooks,
	}

	// If --debug, print timing and exit without starting TUI.
	if cfg.Debug {
		timing.Since("model creation", pluginStart)
		timing.Print()
		return nil
	}

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

// uiRenderer implements core.UIRenderer using the real screen components.
// This breaks the circular import: core → ui/screens is not possible,
// but cmd/nidhi/ can import both core and ui/*.
type uiRenderer struct {
	list     *screens.ListScreen
	preview  *screens.PreviewScreen
	detail   *screens.DetailScreen
	newStash *screens.NewStashScreen
	help     *screens.HelpOverlay
	toast    *components.ToastModel

	statusBar components.StatusBar
	footer    components.Footer

	theme      theme.Theme
	repoName   string
	useNerd    bool
	cache      plugin.StashCache
	appVersion string
	appCommit  string
	screens    *plugin.Registry[plugin.ScreenProvider]

	// stashOps + stashHooks wire the LIST screen's a/p/d/b CRUD dispatches
	// (StashApplyMsg, StashPopMsg, …) to the actual git operations and any
	// registered StashHook plugins (conflict preview, undo recorder, etc.).
	stashOps   *git.StashOps
	stashHooks *plugin.Registry[plugin.StashHook]
}

// RenderWelcome renders the startup welcome screen.
func (u *uiRenderer) RenderWelcome(width, height int, version, commit string) string {
	return screens.RenderWelcome(u.theme, width, height, version, commit)
}

// RenderContent renders the full screen content for the current mode.
func (u *uiRenderer) RenderContent(state core.AppState) string {
	dims := layout.ComputeDimensions(state.Width, state.Height)

	// Status bar.
	sb := u.statusBar.Render(components.StatusBarParams{
		RepoName:   u.repoName,
		Branch:     state.Branch,
		StashCount: len(state.Stashes),
		GitVersion: state.GitVersion,
		AppVersion: u.appVersion,
		AppCommit:  u.appCommit,
		Width:      dims.TotalWidth,
		UseNerd:    u.useNerd,
	})

	// Footer.
	ft := u.footer.Render(components.FooterParams{
		Mode:  state.Mode,
		Width: dims.TotalWidth,
	})

	// Active screen content.
	var content string
	switch state.Mode {
	case core.ModeList:
		content = u.list.View(state, dims.ContentWidth, dims.ContentHeight)
	case core.ModePreview:
		content = u.preview.View(state, dims.ContentWidth, dims.ContentHeight)
	case core.ModeDetail:
		content = u.detail.View(state, dims.ContentWidth, dims.ContentHeight)
	case core.ModeNewStash:
		content = u.newStash.View(state, dims.ContentWidth, dims.ContentHeight)
	case core.ModeHelp:
		// Help renders over the list view background.
		bg := u.list.View(state, dims.ContentWidth, dims.ContentHeight)
		content = u.help.RenderWithDimmedBackground(bg, dims.ContentWidth, dims.ContentHeight)
	default:
		if provider := u.providerForMode(state.Mode); provider != nil {
			content = provider.View(state, dims.ContentWidth, dims.ContentHeight)
		} else {
			content = u.list.View(state, dims.ContentWidth, dims.ContentHeight)
		}
	}

	// Toast overlay (if visible, append to content).
	if toastView := u.toast.View(); toastView != "" {
		content = content + "\n" + toastView
	}

	return layout.Render(sb, content, ft)
}

// HandleMessage routes BubbleTea messages to the appropriate screen.
func (u *uiRenderer) HandleMessage(msg tea.Msg, state core.AppState) (core.AppState, tea.Cmd) {
	var cmds []tea.Cmd

	// Toast always gets a chance to handle tick messages.
	if _, ok := msg.(components.ToastTickMsg); ok {
		cmd := u.toast.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return state, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case core.InfoToastMsg:
		if cmd := u.toast.Show(msg.Text, components.ToastInfo); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return state, tea.Batch(cmds...)
	case core.ErrorMsg:
		if cmd := u.toast.Show(msg.Err.Error(), components.ToastError); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return state, tea.Batch(cmds...)
	case screens.StashApplyMsg:
		return state, u.runApply(msg.Stash)
	case screens.StashPopMsg:
		return state, u.runPop(msg.Stash)
	case screens.StashDropMsg:
		return state, u.runDrop(msg.Stash)
	case screens.StashBranchMsg:
		return state, u.runBranch(msg.Stash)
	case screens.StashRenameMsg:
		// Rename is handled inline by the rename plugin's HandleKey; the LIST
		// screen still emits this message for symmetry with apply/pop/drop, so
		// we acknowledge it as a no-op here. The plugin keypress handler runs
		// after UI delegation when the screen returned no command.
		return state, nil
	}

	// DiffLoadedMsg goes to preview and detail screens.
	if diffMsg, ok := msg.(screens.DiffLoadedMsg); ok {
		newState, cmd := u.preview.Update(msg, state)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if diffMsg.Err == nil {
			u.detail.SetDiff(diffMsg.Diff)
		}
		return newState, tea.Batch(cmds...)
	}

	if _, ok := msg.(tea.KeyPressMsg); !ok {
		newState := state
		if cmd := u.updateScreenProviders(msg, &newState); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if newState.Mode != state.Mode {
			return newState, tea.Batch(cmds...)
		}
	}

	// Window resize: update all screens.
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		newState, cmd := u.list.Update(msg, state)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		newState, cmd = u.preview.Update(msg, newState)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		newState, cmd = u.detail.Update(msg, newState)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		newState, cmd = u.newStash.Update(msg, newState)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return newState, tea.Batch(cmds...)
	}

	// Route to the active screen.
	switch state.Mode {
	case core.ModeList:
		newState, cmd := u.list.Update(msg, state)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// If entering preview mode, trigger diff load.
		if newState.Mode == core.ModePreview {
			if diffCmd := u.preview.EnsureDiffLoaded(newState); diffCmd != nil {
				cmds = append(cmds, diffCmd)
			}
		}
		return newState, tea.Batch(cmds...)
	case core.ModePreview:
		newState, cmd := u.preview.Update(msg, state)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return newState, tea.Batch(cmds...)
	case core.ModeDetail:
		newState, cmd := u.detail.Update(msg, state)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return newState, tea.Batch(cmds...)
	case core.ModeNewStash:
		newState, cmd := u.newStash.Update(msg, state)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return newState, tea.Batch(cmds...)
	default:
		if provider := u.providerForMode(state.Mode); provider != nil {
			newState, cmd := provider.Update(msg, state)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return newState, tea.Batch(cmds...)
		}
	}

	return state, tea.Batch(cmds...)
}

// OnModeChange handles side effects when the mode changes (e.g., loading diffs for PREVIEW).
func (u *uiRenderer) OnModeChange(prev, next core.Mode, state core.AppState) tea.Cmd {
	// Hide help when leaving ModeHelp.
	if prev == core.ModeHelp {
		u.help.Hide()
	}

	switch next {
	case core.ModeHelp:
		u.help.Show()
	case core.ModePreview:
		return u.preview.EnsureDiffLoaded(state)
	case core.ModeDetail:
		u.detail.ResetFocus()
		// If entering detail from list, trigger a diff load.
		// From preview, the diff is already loaded.
		if prev == core.ModeList {
			return u.preview.EnsureDiffLoaded(state)
		}
	}
	return nil
}

func (u *uiRenderer) providerForMode(mode core.Mode) plugin.ScreenProvider {
	if u.screens == nil {
		return nil
	}
	for _, provider := range u.screens.All() {
		for _, screen := range provider.Screens() {
			if screen.Mode == mode {
				return provider
			}
		}
	}
	return nil
}

func (u *uiRenderer) updateScreenProviders(msg tea.Msg, state *core.AppState) tea.Cmd {
	if u.screens == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, provider := range u.screens.All() {
		newState, cmd := provider.Update(msg, *state)
		*state = newState
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// ─── Stash CRUD wiring ──────────────────────────────────────
//
// The LIST screen dispatches StashApplyMsg/StashPopMsg/StashDropMsg/
// StashBranchMsg in response to the a/p/d/b keys. These helpers run the
// matching git operation through StashOps, fire any registered StashHook
// callbacks (BeforeApply for conflict preview, AfterDrop for undo
// recording), surface a toast, and emit core.StashMutatedMsg so the model
// invalidates the cache and reloads the list.

func (u *uiRenderer) runApply(stash plugin.Stash) tea.Cmd {
	cmds := u.beforeApplyCmds(stash)
	cmds = append(cmds, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), git.DefaultTimeout)
		defer cancel()
		result, err := u.stashOps.Apply(ctx, stash.Index)
		if err != nil {
			return core.ErrorMsg{Err: fmt.Errorf("apply: %w", err)}
		}
		if !result.Success {
			return core.ErrorMsg{Err: fmt.Errorf("apply failed: %s", result.Error)}
		}
		return core.InfoToastMsg{Text: "Applied: " + stash.Message}
	})
	return tea.Batch(cmds...)
}

func (u *uiRenderer) runPop(stash plugin.Stash) tea.Cmd {
	cmds := u.beforeApplyCmds(stash)
	cmds = append(cmds,
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), git.DefaultTimeout)
			defer cancel()
			result, err := u.stashOps.Pop(ctx, stash.Index)
			if err != nil {
				return core.ErrorMsg{Err: fmt.Errorf("pop: %w", err)}
			}
			if !result.Success {
				return core.ErrorMsg{Err: fmt.Errorf("pop failed: %s", result.Error)}
			}
			return core.InfoToastMsg{Text: "Popped: " + stash.Message}
		},
		u.afterDropCmd(stash),
		mutated(),
	)
	return tea.Batch(cmds...)
}

func (u *uiRenderer) runDrop(stash plugin.Stash) tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), git.DefaultTimeout)
			defer cancel()
			result, err := u.stashOps.Drop(ctx, stash.Index)
			if err != nil {
				return core.ErrorMsg{Err: fmt.Errorf("drop: %w", err)}
			}
			if !result.Success {
				return core.ErrorMsg{Err: fmt.Errorf("drop failed: %s", result.Error)}
			}
			return core.InfoToastMsg{Text: "Dropped: " + stash.Message + " (z to undo)"}
		},
		u.afterDropCmd(stash),
		mutated(),
	)
}

func (u *uiRenderer) runBranch(stash plugin.Stash) tea.Cmd {
	// Branch creation needs a name from the user; for now derive a default
	// from the stash index. A future revision can prompt via a small modal
	// (see core.PromptBranchNameMsg, currently unwired).
	branchName := fmt.Sprintf("stash-%d-branch", stash.Index)
	return tea.Batch(
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), git.DefaultTimeout)
			defer cancel()
			result, err := u.stashOps.BranchFromStash(ctx, stash.Index, branchName)
			if err != nil {
				return core.ErrorMsg{Err: fmt.Errorf("branch: %w", err)}
			}
			if !result.Success {
				return core.ErrorMsg{Err: fmt.Errorf("branch failed: %s", result.Error)}
			}
			return core.InfoToastMsg{Text: "Created branch " + branchName + " from " + stash.Message}
		},
		// `git stash branch` drops the stash on success, so fire AfterDrop
		// hooks (undo recorder records the SHA for `z` recovery) and reload.
		u.afterDropCmd(stash),
		mutated(),
	)
}

// beforeApplyCmds invokes BeforeApply on every registered StashHook. If any
// hook returns proceed=false the operation is short-circuited and the hook's
// returned cmd is the only one that runs (typically opening a modal screen).
func (u *uiRenderer) beforeApplyCmds(stash plugin.Stash) []tea.Cmd {
	if u.stashHooks == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, hook := range u.stashHooks.All() {
		proceed, cmd := hook.BeforeApply(stash)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if !proceed {
			// One hook vetoed the apply; do not chain the actual operation.
			return []tea.Cmd{tea.Batch(cmds...)}
		}
	}
	return cmds
}

func (u *uiRenderer) afterDropCmd(stash plugin.Stash) tea.Cmd {
	if u.stashHooks == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, hook := range u.stashHooks.All() {
		if cmd := hook.AfterDrop(stash, stash.SHA); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// mutated returns a tea.Cmd that emits core.StashMutatedMsg, which core.Model
// handles by invalidating the cache and reloading the stash list.
func mutated() tea.Cmd {
	return func() tea.Msg { return core.StashMutatedMsg{} }
}

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
