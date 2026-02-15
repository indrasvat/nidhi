package rename

import (
	"context"
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const PluginID = "rename"

// StartRenameMsg signals that inline rename mode should activate.
type StartRenameMsg struct {
	StashIndex int
	OldMessage string
}

// RenameCompleteMsg signals that a rename operation finished.
type RenameCompleteMsg struct {
	StashIndex int
	NewMessage string
}

// Plugin implements plugin.KeyHandler for the rename feature (PRD FR-13).
type Plugin struct {
	git    plugin.GitRunner
	cache  plugin.StashCache
	logger *slog.Logger
}

// New creates a new rename plugin.
func New() *Plugin {
	return &Plugin{}
}

var _ plugin.KeyHandler = (*Plugin)(nil)

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return "Rename" }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.cache = ctx.Cache
	p.logger = ctx.Logger
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns the keybindings for the rename plugin.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{
			Key:   "r",
			Desc:  "Rename stash message",
			Modes: []plugin.Mode{plugin.ModeList},
		},
	}
}

// HandleKey handles the `r` key press to start inline rename (FR-13.1).
func (p *Plugin) HandleKey(key plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	if key.Key != "r" {
		return state, nil
	}

	if state.Cursor < 0 || state.Cursor >= len(state.Stashes) {
		return state, nil
	}

	stash := state.Stashes[state.Cursor]

	return state, func() tea.Msg {
		return StartRenameMsg{
			StashIndex: stash.Index,
			OldMessage: stash.Message,
		}
	}
}

// RenameStash performs the actual rename operation.
//
// For stash@{0} (top): simple drop + store.
// For stash@{n} where n > 0: must drop and re-store all stashes above
// to preserve ordering (FR-13.5). Uses journal for crash safety (FR-16.4).
func (p *Plugin) RenameStash(ctx context.Context, stashes []plugin.Stash, targetIdx int, newMsg string) error {
	if targetIdx < 0 || targetIdx >= len(stashes) {
		return fmt.Errorf("rename: index %d out of range (have %d stashes)", targetIdx, len(stashes))
	}

	target := stashes[targetIdx]

	if targetIdx == 0 {
		return p.renameTop(ctx, target, newMsg)
	}

	return p.renameWithReorder(ctx, stashes, targetIdx, newMsg)
}

// renameTop renames stash@{0} — the simplest case.
func (p *Plugin) renameTop(ctx context.Context, stash plugin.Stash, newMsg string) error {
	journal := &Journal{
		Operation:  "rename",
		Entries:    []JournalEntry{{Index: 0, SHA: stash.SHA, Message: stash.Message}},
		TargetIdx:  0,
		NewMsg:     newMsg,
		Step:       0,
		TotalSteps: 2,
	}
	if err := WriteJournal(journal); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}

	_, err := p.git.Run(ctx, "stash", "drop", "stash@{0}")
	if err != nil {
		return fmt.Errorf("drop stash@{0}: %w", err)
	}

	journal.Step = 1
	_ = WriteJournal(journal)

	_, err = p.git.Run(ctx, "stash", "store", "-m", newMsg, stash.SHA)
	if err != nil {
		return fmt.Errorf("store with new message: %w", err)
	}

	_ = RemoveJournal()

	p.cache.Invalidate()

	p.logger.Info("stash renamed",
		"index", 0,
		"sha", stash.SHA,
		"old_msg", stash.Message,
		"new_msg", newMsg,
	)

	return nil
}

// renameWithReorder renames a stash at index > 0 by dropping all stashes
// from 0 to targetIdx and re-storing them in the correct order with the
// target stash having the new message.
func (p *Plugin) renameWithReorder(ctx context.Context, stashes []plugin.Stash, targetIdx int, newMsg string) error {
	entries := make([]JournalEntry, targetIdx+1)
	for i := 0; i <= targetIdx; i++ {
		entries[i] = JournalEntry{
			Index:   i,
			SHA:     stashes[i].SHA,
			Message: stashes[i].Message,
		}
	}

	journal := &Journal{
		Operation:  "rename",
		Entries:    entries,
		TargetIdx:  targetIdx,
		NewMsg:     newMsg,
		Step:       0,
		TotalSteps: (targetIdx+1)*2 + 1,
	}
	if err := WriteJournal(journal); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}

	// Phase 1: Drop all stashes from index 0 to targetIdx.
	// Always drop stash@{0} since indices shift.
	for i := 0; i <= targetIdx; i++ {
		_, err := p.git.Run(ctx, "stash", "drop", "stash@{0}")
		if err != nil {
			return fmt.Errorf("drop stash@{0} (step %d): %w", i, err)
		}
		journal.Step++
		_ = WriteJournal(journal)
	}

	// Phase 2: Re-store in reverse order (highest index first) so that
	// after all stores, index 0 is back at the top.
	for i := targetIdx; i >= 0; i-- {
		msg := entries[i].Message
		if i == targetIdx {
			msg = newMsg
		}

		_, err := p.git.Run(ctx, "stash", "store", "-m", msg, entries[i].SHA)
		if err != nil {
			return fmt.Errorf("store stash (original index %d): %w", i, err)
		}
		journal.Step++
		_ = WriteJournal(journal)
	}

	_ = RemoveJournal()

	p.cache.Invalidate()

	p.logger.Info("stash renamed with reorder",
		"target_index", targetIdx,
		"sha", stashes[targetIdx].SHA,
		"old_msg", stashes[targetIdx].Message,
		"new_msg", newMsg,
	)

	return nil
}

// RecoverFromJournal attempts to recover from an interrupted rename operation.
// It re-stores all stashes from the journal that were dropped but not yet
// re-stored. Returns the number of stashes recovered.
func RecoverFromJournal(ctx context.Context, runner plugin.GitRunner) (int, error) {
	journal, err := ReadJournal()
	if err != nil {
		return 0, err
	}
	if journal == nil {
		return 0, nil
	}

	storesDone := max(journal.Step-len(journal.Entries), 0)

	recovered := 0

	// Re-store entries that were dropped but not yet stored.
	// Entries are stored in reverse order (highest index first).
	for i := len(journal.Entries) - 1 - storesDone; i >= 0; i-- {
		entry := journal.Entries[i]
		msg := entry.Message
		if i == journal.TargetIdx {
			msg = journal.NewMsg
		}

		_, err := runner.Run(ctx, "stash", "store", "-m", msg, entry.SHA)
		if err != nil {
			return recovered, fmt.Errorf("recover store %s: %w", entry.SHA[:min(8, len(entry.SHA))], err)
		}
		recovered++
	}

	_ = RemoveJournal()
	return recovered, nil
}

// PerformRename creates a tea.Cmd that executes the rename and sends a completion message.
func (p *Plugin) PerformRename(stashes []plugin.Stash, targetIdx int, newMsg string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := p.RenameStash(ctx, stashes, targetIdx, newMsg)
		if err != nil {
			return core.ErrorMsg{Err: fmt.Errorf("rename: %w", err)}
		}

		return RenameCompleteMsg{
			StashIndex: targetIdx,
			NewMessage: newMsg,
		}
	}
}
