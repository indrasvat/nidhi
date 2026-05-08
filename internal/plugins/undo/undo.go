package undo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID   = "undo"
	UndoTTL    = 30 * time.Second
	BufferSize = 50
)

// UndoToastMsg triggers the undo toast notification.
type UndoToastMsg struct {
	Entry UndoEntry
	TTL   time.Duration
}

// UndoToastExpiredMsg signals that the undo window has closed.
type UndoToastExpiredMsg struct {
	SHA string
}

// OpenRecoveryPickerMsg signals that the recovery picker screen should open.
type OpenRecoveryPickerMsg struct {
	Candidates []RecoveryCandidate
}

// Plugin implements plugin.StashHook and plugin.KeyHandler for
// the undo & recovery feature (PRD FR-14).
type Plugin struct {
	git    plugin.GitRunner
	cache  plugin.StashCache
	logger *slog.Logger
	buffer *RingBuffer
}

// New creates a new undo plugin.
func New() *Plugin {
	return &Plugin{
		buffer: NewRingBuffer(BufferSize),
	}
}

var (
	_ plugin.StashHook  = (*Plugin)(nil)
	_ plugin.KeyHandler = (*Plugin)(nil)
)

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return "Undo & Recovery" }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.cache = ctx.Cache
	p.logger = ctx.Logger
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// ─── StashHook ──────────────────────────────────────────────

// AfterDrop records the dropped stash SHA and message in the undo ring buffer
// and triggers the undo toast (FR-14.1).
func (p *Plugin) AfterDrop(stash plugin.Stash, sha string) tea.Cmd {
	entry := UndoEntry{
		SHA:       sha,
		Message:   stash.Message,
		Index:     stash.Index,
		DroppedAt: time.Now(),
	}

	p.buffer.Push(entry)
	p.logger.Info("stash dropped, recorded for undo",
		"sha", sha,
		"index", stash.Index,
		"message", stash.Message,
	)

	return tea.Batch(
		func() tea.Msg {
			return UndoToastMsg{Entry: entry, TTL: UndoTTL}
		},
		tea.Tick(UndoTTL, func(_ time.Time) tea.Msg {
			return UndoToastExpiredMsg{SHA: sha}
		}),
	)
}

// BeforeApply is a no-op for the undo plugin.
func (p *Plugin) BeforeApply(_ plugin.Stash) (proceed bool, cmd tea.Cmd) {
	return true, nil
}

// BeforePush is a no-op for the undo plugin.
func (p *Plugin) BeforePush(opts plugin.PushOptions) (plugin.PushOptions, error) {
	return opts, nil
}

// ─── KeyHandler ─────────────────────────────────────────────

// KeyBindings returns the keybindings for the undo plugin.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{
			Key:   "z",
			Desc:  "Undo last drop / Recovery picker",
			Modes: []plugin.Mode{plugin.ModeList, plugin.ModePreview},
		},
	}
}

// HandleKey handles the `z` key press for undo/recovery (FR-14.2, FR-14.3).
func (p *Plugin) HandleKey(key plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	if key.Key != "z" {
		return state, nil
	}

	// Check if we have a recent (non-expired) undo entry.
	entry, ok := p.buffer.Peek()
	if ok && !entry.IsExpired(UndoTTL) {
		// Session undo: restore immediately (FR-14.2).
		p.buffer.Pop()
		return state, p.restoreStash(entry)
	}

	// No recent undo available — open cross-session recovery picker (FR-14.3).
	return state, p.openRecoveryPicker()
}

func (p *Plugin) restoreStash(entry UndoEntry) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		_, err := p.git.Run(ctx, "stash", "store", "-m", entry.Message, entry.SHA)
		if err != nil {
			return core.ErrorMsg{Err: fmt.Errorf("undo restore: %w", err)}
		}

		p.cache.Invalidate()

		p.logger.Info("stash restored via undo",
			"sha", entry.SHA,
			"message", entry.Message,
		)

		return core.StashMutatedMsg{}
	}
}

func (p *Plugin) openRecoveryPicker() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		candidates, err := FindDroppedStashes(ctx, p.git)
		if err != nil {
			p.logger.Error("recovery scan failed", "error", err)
			return core.ErrorMsg{Err: fmt.Errorf("recovery scan: %w", err)}
		}

		if len(candidates) == 0 {
			return core.InfoToastMsg{Text: "No recoverable stashes found."}
		}

		return OpenRecoveryPickerMsg{Candidates: candidates}
	}
}

// Buffer returns the undo ring buffer for testing.
func (p *Plugin) Buffer() *RingBuffer {
	return p.buffer
}
