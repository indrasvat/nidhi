package conflict

import (
	"context"
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/icons"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

const PluginID = "conflict"

// SwitchToConflictScreenMsg is sent when conflicts are detected and the
// conflict preview screen should be shown.
type SwitchToConflictScreenMsg struct {
	Stash  plugin.Stash
	Result git.MergeTreeResult
	IsPop  bool // true if the user pressed pop, false if apply
}

// Plugin implements plugin.StashHook and plugin.ScreenProvider for
// the conflict preview feature (PRD FR-10).
type Plugin struct {
	git       plugin.GitRunner
	logger    *slog.Logger
	gitVer    plugin.GitVersion
	th        theme.Theme
	iconMode  icons.Mode
	lastStash *plugin.Stash
	lastIsPop bool
	result    *git.MergeTreeResult
	cursor    int // selected file in conflict list
	scroll    int // vertical scroll offset for conflict zone preview
}

// New creates a new conflict preview plugin.
func New() *Plugin {
	return &Plugin{}
}

var (
	_ plugin.StashHook      = (*Plugin)(nil)
	_ plugin.ScreenProvider = (*Plugin)(nil)
)

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return "Conflict Preview" }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.logger = ctx.Logger
	p.gitVer = ctx.GitVer
	if th, ok := ctx.Theme.(theme.Theme); ok {
		p.th = th
	}
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// ─── StashHook ──────────────────────────────────────────────

// BeforeApply runs merge-tree to check for conflicts before applying/popping.
func (p *Plugin) BeforeApply(stash plugin.Stash) (proceed bool, cmd tea.Cmd) {
	// FR-10.7: Requires Git >= 2.38. If unavailable, skip and proceed directly.
	if !p.gitVer.AtLeast(2, 38) {
		p.logger.Info("git version < 2.38, skipping conflict preview")
		return true, func() tea.Msg {
			return core.InfoToastMsg{
				Text: "Conflict preview requires Git >= 2.38. Applying directly.",
			}
		}
	}

	ctx := context.Background()

	// Run merge-tree dry run.
	result, err := git.RunMergeTree(ctx, p.git, stash.SHA)
	if err != nil {
		p.logger.Error("merge-tree failed", "error", err)
		// Fail-open: proceed with apply, don't block the user.
		return true, nil
	}

	// Check for untracked file collisions (FR-10.6a).
	collisions, err := git.CheckUntrackedCollisions(ctx, p.git, stash.SHA)
	if err != nil {
		p.logger.Warn("untracked collision check failed", "error", err)
	}
	result.UntrackedCollisions = collisions

	// If clean and no untracked collisions, proceed immediately.
	if !result.HasConflicts && len(collisions) == 0 {
		return true, nil
	}

	// Conflicts or collisions detected — switch to conflict preview screen.
	p.result = &result
	p.lastStash = &stash
	p.cursor = 0
	p.scroll = 0

	return false, func() tea.Msg {
		return SwitchToConflictScreenMsg{
			Stash:  stash,
			Result: result,
			IsPop:  false,
		}
	}
}

// AfterDrop is a no-op for the conflict plugin.
func (p *Plugin) AfterDrop(_ plugin.Stash, _ string) tea.Cmd {
	return nil
}

// BeforePush is a no-op for the conflict plugin.
func (p *Plugin) BeforePush(opts plugin.PushOptions) (plugin.PushOptions, error) {
	return opts, nil
}

// ─── ScreenProvider ─────────────────────────────────────────

// Screens returns the conflict preview screen definition.
func (p *Plugin) Screens() []plugin.ScreenDef {
	return []plugin.ScreenDef{
		{
			Name: "Conflict Preview",
			Mode: core.ModeConflict,
		},
	}
}

// Update handles messages when the conflict preview screen is active.
func (p *Plugin) Update(msg tea.Msg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case SwitchToConflictScreenMsg:
		p.result = &msg.Result
		stash := msg.Stash
		p.lastStash = &stash
		p.lastIsPop = msg.IsPop
		p.cursor = 0
		p.scroll = 0
		state.Mode = core.ModeConflict
		return state, nil

	case tea.KeyPressMsg:
		return p.handleKey(msg, state)
	}
	return state, nil
}

func (p *Plugin) handleKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg.Text {
	case "j", "down":
		if p.result != nil {
			totalItems := len(p.result.Files) + len(p.result.UntrackedCollisions)
			if p.cursor < totalItems-1 {
				p.cursor++
			}
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "a":
		// Apply anyway.
		return state, p.applyAnyway(false)
	case "p":
		// Pop anyway.
		return state, p.applyAnyway(true)
	case "b":
		// Branch first — prompt for branch name.
		return state, p.branchFirst()
	case "escape":
		// Cancel — return to LIST mode.
		state.Mode = core.ModeList
		return state, nil
	}
	return state, nil
}

// View renders the conflict preview screen.
func (p *Plugin) View(state plugin.AppState, width, height int) string {
	if p.result == nil || p.lastStash == nil {
		return "No conflict data available."
	}
	return RenderConflictScreen(p.result, p.lastStash, p.th, p.iconMode, p.cursor, width, height)
}

func (p *Plugin) applyAnyway(isPop bool) tea.Cmd {
	stash := p.lastStash
	if stash == nil {
		return nil
	}
	return func() tea.Msg {
		ctx := context.Background()
		ref := fmt.Sprintf("stash@{%d}", stash.Index)
		if isPop {
			_, _, err := p.git.RunExitCode(ctx, "stash", "pop", ref)
			if err != nil {
				return core.ErrorMsg{Err: fmt.Errorf("stash pop: %w", err)}
			}
		} else {
			_, _, err := p.git.RunExitCode(ctx, "stash", "apply", ref)
			if err != nil {
				return core.ErrorMsg{Err: fmt.Errorf("stash apply: %w", err)}
			}
		}
		return core.StashMutatedMsg{}
	}
}

func (p *Plugin) branchFirst() tea.Cmd {
	stash := p.lastStash
	if stash == nil {
		return nil
	}
	return func() tea.Msg {
		return core.PromptBranchNameMsg{Stash: *stash}
	}
}
