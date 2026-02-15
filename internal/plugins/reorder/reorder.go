package reorder

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID   = "reorder"
	PluginName = "Reorder"
)

// ReorderCompleteMsg is sent when a reorder operation finishes.
type ReorderCompleteMsg struct {
	SourceIndex int
	TargetIndex int
	Error       error
}

// Plugin implements KeyHandler for stash reordering (PRD FR-16).
// Shift+J moves the selected stash down, Shift+K moves it up.
type Plugin struct {
	git         plugin.GitRunner
	cache       plugin.StashCache
	events      plugin.EventBus
	logger      *slog.Logger
	journalPath string
}

var _ plugin.KeyHandler = (*Plugin)(nil)

func New() *Plugin {
	return &Plugin{
		journalPath: DefaultJournalPath(),
	}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.cache = ctx.Cache
	p.events = ctx.Events
	p.logger = ctx.Logger
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// KeyBindings returns the Shift+J/K reorder bindings.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "J", Desc: "Move stash down", Modes: []plugin.Mode{plugin.ModeList}},
		{Key: "K", Desc: "Move stash up", Modes: []plugin.Mode{plugin.ModeList}},
	}
}

// HandleKey processes Shift+J/K reorder commands.
// The cursor follows the moved stash so the user keeps selection.
func (p *Plugin) HandleKey(key plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch key.Key {
	case "J":
		if state.Cursor >= len(state.Stashes)-1 {
			return state, nil
		}
		cmd := p.reorderCmd(state, state.Cursor, state.Cursor+1)
		state.Cursor++
		return state, cmd

	case "K":
		if state.Cursor <= 0 {
			return state, nil
		}
		cmd := p.reorderCmd(state, state.Cursor, state.Cursor-1)
		state.Cursor--
		return state, cmd
	}

	return state, nil
}

// reorderCmd returns a tea.Cmd that performs the reorder operation.
//
// Algorithm: To move stash@{source} to position target:
//  1. Record ALL stashes in journal (SHAs + messages).
//  2. Drop ALL stashes from highest index to 0.
//  3. Re-store them in the new desired order via `git stash store`.
//     Stores from last position to first (store prepends, so position 0 stored last).
//  4. Mark journal complete and remove it.
func (p *Plugin) reorderCmd(state plugin.AppState, sourceIndex, targetIndex int) tea.Cmd {
	stashes := make([]plugin.Stash, len(state.Stashes))
	copy(stashes, state.Stashes)
	gitRunner := p.git
	cache := p.cache
	journalPath := p.journalPath
	logger := p.logger

	return func() tea.Msg {
		ctx := context.Background()

		// Step 1: Build journal entries.
		entries := make([]JournalEntry, len(stashes))
		for i, s := range stashes {
			entries[i] = JournalEntry{
				Index:   s.Index,
				SHA:     s.SHA,
				Message: s.RawMessage,
			}
		}

		journal := NewJournal(sourceIndex, targetIndex, entries)
		journal.SetPath(journalPath)
		if err := journal.Write(); err != nil {
			return ReorderCompleteMsg{
				SourceIndex: sourceIndex,
				TargetIndex: targetIndex,
				Error:       fmt.Errorf("write journal: %w", err),
			}
		}

		// Step 2: Compute the new order.
		newOrder := ComputeNewOrder(entries, sourceIndex, targetIndex)

		// Step 3: Drop all stashes (highest index first).
		for i := len(stashes) - 1; i >= 0; i-- {
			_, err := gitRunner.Run(ctx, "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
			if err != nil {
				return ReorderCompleteMsg{
					SourceIndex: sourceIndex,
					TargetIndex: targetIndex,
					Error:       fmt.Errorf("drop stash@{%d}: %w", i, err),
				}
			}
		}

		// Step 4: Re-store in new order (last to first, since store prepends).
		for i := len(newOrder) - 1; i >= 0; i-- {
			e := newOrder[i]
			_, err := gitRunner.Run(ctx, "stash", "store", "-m", e.Message, e.SHA)
			if err != nil {
				return ReorderCompleteMsg{
					SourceIndex: sourceIndex,
					TargetIndex: targetIndex,
					Error:       fmt.Errorf("store %s: %w", e.SHA, err),
				}
			}
		}

		// Step 5: Mark journal complete and clean up.
		_ = journal.MarkComplete()
		_ = journal.Remove()

		cache.Invalidate()

		logger.Info("stash reordered",
			"from", sourceIndex,
			"to", targetIndex,
		)

		return ReorderCompleteMsg{
			SourceIndex: sourceIndex,
			TargetIndex: targetIndex,
		}
	}
}

// ComputeNewOrder rearranges entries by moving sourceIndex to targetIndex.
// After removal of the source element, the target position in the reduced
// array is used directly — no index adjustment is needed because the caller
// specifies the desired final position.
func ComputeNewOrder(entries []JournalEntry, sourceIndex, targetIndex int) []JournalEntry {
	newOrder := make([]JournalEntry, len(entries))
	copy(newOrder, entries)

	moved := newOrder[sourceIndex]
	newOrder = append(newOrder[:sourceIndex], newOrder[sourceIndex+1:]...)

	// Insert at targetIndex in the reduced array.
	tail := make([]JournalEntry, len(newOrder[targetIndex:]))
	copy(tail, newOrder[targetIndex:])
	newOrder = append(newOrder[:targetIndex], moved)
	newOrder = append(newOrder, tail...)

	return newOrder
}

// RecoverFromJournal attempts to restore the original stash order from an
// incomplete journal. Called on startup when a crash was detected.
func RecoverFromJournal(ctx context.Context, journalPath string, gitRunner plugin.GitRunner) (int, error) {
	journal, err := LoadJournal(journalPath)
	if err != nil {
		return 0, fmt.Errorf("load journal: %w", err)
	}
	if journal == nil || !journal.IsIncomplete() {
		return 0, nil
	}

	// Clear any partial stash state.
	lines, _ := gitRunner.RunLines(ctx, "stash", "list")
	for i := len(lines) - 1; i >= 0; i-- {
		_, _ = gitRunner.Run(ctx, "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
	}

	// Re-store original order (last to first, since store prepends).
	recovered := 0
	for i := len(journal.Entries) - 1; i >= 0; i-- {
		e := journal.Entries[i]
		_, err := gitRunner.Run(ctx, "stash", "store", "-m", e.Message, e.SHA)
		if err != nil {
			return recovered, fmt.Errorf("recovery: store %s: %w", e.SHA, err)
		}
		recovered++
	}

	_ = journal.MarkComplete()
	_ = journal.Remove()
	return recovered, nil
}
