package stale

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
)

const (
	PluginID    = "stale"
	PluginName  = "Stale Detection"
	DefaultDays = 14
)

// Plugin implements passive stale detection.
// It does not register keybindings — it computes staleness on stash lists
// and provides bulk drop capability for stale stashes.
type Plugin struct {
	logger    *slog.Logger
	git       plugin.GitRunner
	cache     plugin.StashCache
	threshold time.Duration
}

var _ plugin.Plugin = (*Plugin)(nil)

func New() *Plugin {
	return &Plugin{
		threshold: time.Duration(DefaultDays) * 24 * time.Hour,
	}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.logger = ctx.Logger
	p.git = ctx.Git
	p.cache = ctx.Cache
	// Read threshold from config. GetInt returns 0 if not set.
	if ctx.Config != nil {
		staleDays := ctx.Config.GetInt("stale_days")
		if staleDays > 0 {
			p.threshold = time.Duration(staleDays) * 24 * time.Hour
		}
	}
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// Threshold returns the current staleness threshold.
func (p *Plugin) Threshold() time.Duration {
	return p.threshold
}

// SetThreshold overrides the stale threshold (useful for testing).
func (p *Plugin) SetThreshold(d time.Duration) {
	p.threshold = d
}

// MarkStale computes IsStale for each stash based on current time.
func (p *Plugin) MarkStale(stashes []plugin.Stash) []plugin.Stash {
	return MarkStaleWithTime(stashes, time.Now(), p.threshold)
}

// MarkStaleWithTime computes IsStale using a provided reference time.
func MarkStaleWithTime(stashes []plugin.Stash, now time.Time, threshold time.Duration) []plugin.Stash {
	for i := range stashes {
		age := now.Sub(stashes[i].Date)
		stashes[i].IsStale = age >= threshold
	}
	return stashes
}

// StaleStashes returns only the stashes that are marked stale.
func StaleStashes(stashes []plugin.Stash) []plugin.Stash {
	var result []plugin.Stash
	for _, s := range stashes {
		if s.IsStale {
			result = append(result, s)
		}
	}
	return result
}

// StaleCount returns the number of stale stashes.
func StaleCount(stashes []plugin.Stash) int {
	count := 0
	for _, s := range stashes {
		if s.IsStale {
			count++
		}
	}
	return count
}

// BulkDropResultMsg is the result of a bulk drop operation.
type BulkDropResultMsg struct {
	Dropped int
	Err     error
}

// BulkDropStaleCmd returns a tea.Cmd that drops all stale stashes.
// Drops from highest index first to preserve index validity.
func BulkDropStaleCmd(staleStashes []plugin.Stash, runner plugin.GitRunner) tea.Cmd {
	return func() tea.Msg {
		// Sort by index descending to avoid index shifts during drops.
		sorted := make([]plugin.Stash, len(staleStashes))
		copy(sorted, staleStashes)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Index > sorted[j].Index
		})

		ctx := context.Background()
		dropped := 0
		for _, s := range sorted {
			ref := fmt.Sprintf("stash@{%d}", s.Index)
			_, err := runner.Run(ctx, "stash", "drop", ref)
			if err != nil {
				return BulkDropResultMsg{Dropped: dropped, Err: err}
			}
			dropped++
		}

		return BulkDropResultMsg{Dropped: dropped}
	}
}
