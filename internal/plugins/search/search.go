package search

import (
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

const (
	PluginID   = "search"
	PluginName = "Deep Fuzzy Search"
)

// Plugin implements KeyHandler + ScreenProvider for deep fuzzy search.
type Plugin struct {
	git    plugin.GitRunner
	cache  plugin.StashCache
	logger *slog.Logger
	th     theme.Theme

	index   *Index
	query   string
	cursor  int // Cursor position within query text
	scope   Scope
	results []SearchResult
	resCur  int // Cursor position in results list
	active  bool
}

// Ensure interface compliance.
var (
	_ plugin.KeyHandler     = (*Plugin)(nil)
	_ plugin.ScreenProvider = (*Plugin)(nil)
)

// New creates a new search plugin instance.
func New(th theme.Theme) *Plugin {
	return &Plugin{
		th:    th,
		index: NewIndex(),
		scope: ScopeAll,
	}
}

func (p *Plugin) ID() string   { return PluginID }
func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Init(ctx plugin.PluginContext) error {
	p.git = ctx.Git
	p.cache = ctx.Cache
	p.logger = ctx.Logger
	return nil
}

func (p *Plugin) Destroy() error { return nil }

// ─── KeyHandler ─────────────────────────────────────────────

// KeyBindings returns the keybindings this plugin provides.
func (p *Plugin) KeyBindings() []plugin.KeyBinding {
	return []plugin.KeyBinding{
		{Key: "/", Desc: "Search", Modes: []plugin.Mode{plugin.ModeList, plugin.ModePreview}, Priority: 100},
	}
}

// HandleKey handles the `/` key to activate search mode.
func (p *Plugin) HandleKey(key plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	if key.Key == "/" && !p.active {
		p.active = true
		p.query = ""
		p.cursor = 0
		p.results = nil
		p.resCur = 0
		state.Mode = plugin.ModeSearch

		// If index is not yet built, start building it now (lazy mode).
		var cmd tea.Cmd
		if !p.index.IsReady() && len(state.Stashes) > 0 {
			cmd = BuildIndexCmd(state.Stashes, p.cache, p.index)
		}
		return state, cmd
	}
	return state, nil
}

// ─── ScreenProvider ─────────────────────────────────────────

// Screens returns the search screen definition.
func (p *Plugin) Screens() []plugin.ScreenDef {
	return []plugin.ScreenDef{
		{Mode: plugin.ModeSearch, Name: "Search"},
	}
}

// Update handles messages when the search screen is active.
func (p *Plugin) Update(msg tea.Msg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return p.handleKey(msg, state)

	case IndexReadyMsg:
		// Re-run search with current query.
		if p.query != "" {
			p.results = p.index.Search(p.query, p.scope)
		}
		return state, nil
	}

	return state, nil
}

func (p *Plugin) handleKey(msg tea.KeyPressMsg, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEscape || msg.Text == "escape":
		p.active = false
		state.Mode = plugin.ModeList
		return state, nil

	case msg.Code == tea.KeyTab || msg.Text == "tab":
		// Cycle scope: All -> Messages -> Files -> Diffs -> Branch -> All.
		p.scope = (p.scope + 1) % 5
		p.rerunSearch()
		return state, nil

	case msg.Code == tea.KeyEnter || msg.Text == "enter":
		if len(p.results) > 0 && p.resCur < len(p.results) {
			result := p.results[p.resCur]
			state.Cursor = result.StashIndex
			state.SearchQuery = p.query
			p.active = false
			if result.MatchScope == ScopeDiffs || result.MatchScope == ScopeFiles {
				state.Mode = plugin.ModePreview
			} else {
				state.Mode = plugin.ModeList
			}
			return state, nil
		}
		return state, nil

	case msg.Text == "down" || msg.Code == tea.KeyDown:
		if p.resCur < len(p.results)-1 {
			p.resCur++
		}
		return state, nil

	case msg.Text == "up" || msg.Code == tea.KeyUp:
		if p.resCur > 0 {
			p.resCur--
		}
		return state, nil

	case msg.Text == "backspace" || msg.Code == 8:
		if p.cursor > 0 {
			p.query = p.query[:p.cursor-1] + p.query[p.cursor:]
			p.cursor--
			p.rerunSearch()
		}
		return state, nil

	case msg.Text == "left":
		if p.cursor > 0 {
			p.cursor--
		}
		return state, nil

	case msg.Text == "right":
		if p.cursor < len(p.query) {
			p.cursor++
		}
		return state, nil

	case msg.Text == "home" || (msg.Text == "a" && msg.Mod.Contains(tea.ModCtrl)):
		p.cursor = 0
		return state, nil

	case msg.Text == "end" || (msg.Text == "e" && msg.Mod.Contains(tea.ModCtrl)):
		p.cursor = len(p.query)
		return state, nil

	case msg.Text == "n" && msg.Mod.Contains(tea.ModCtrl):
		// Ctrl+N: next result.
		if p.resCur < len(p.results)-1 {
			p.resCur++
		}
		return state, nil

	case msg.Text == "p" && msg.Mod.Contains(tea.ModCtrl):
		// Ctrl+P: previous result.
		if p.resCur > 0 {
			p.resCur--
		}
		return state, nil

	default:
		// Insert printable characters.
		if len(msg.Text) > 0 && msg.Mod == 0 && len(p.query) < 256 {
			p.query = p.query[:p.cursor] + msg.Text + p.query[p.cursor:]
			p.cursor += len(msg.Text)
			p.rerunSearch()
		}
		return state, nil
	}
}

// rerunSearch re-runs the fuzzy search with the current query and scope.
func (p *Plugin) rerunSearch() {
	if p.query != "" && p.index.HasPartialResults() {
		p.results = p.index.Search(p.query, p.scope)
		p.resCur = 0
	} else if p.query == "" {
		p.results = nil
		p.resCur = 0
	}
}

// View renders the search screen.
func (p *Plugin) View(state plugin.AppState, width, height int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(p.th.FgPrimary())
	dimStyle := lipgloss.NewStyle().Foreground(p.th.FgDimmed())
	activeStyle := lipgloss.NewStyle().Foreground(p.th.AccentGold())
	accentStyle := lipgloss.NewStyle().Foreground(p.th.AccentBright())
	matchStyle := lipgloss.NewStyle().Foreground(p.th.AccentBright()).Bold(true)

	scopeNames := []string{"All", "Messages", "Files", "Diffs", "Branch"}

	var b strings.Builder

	// Header.
	b.WriteString("  ")
	b.WriteString(headerStyle.Render("Search"))
	b.WriteString("\n\n")

	// Search input with cursor.
	label := activeStyle.Render("  / ")
	b.WriteString(label)

	if p.query == "" {
		cursorStyle := lipgloss.NewStyle().Reverse(true)
		b.WriteString(cursorStyle.Render(" "))
		b.WriteString(dimStyle.Render(" Search stashes..."))
	} else {
		before := p.query[:p.cursor]
		after := p.query[p.cursor:]
		cursorChar := " "
		if p.cursor < len(p.query) {
			cursorChar = string(p.query[p.cursor])
			after = after[1:]
		}
		cursorStyle := lipgloss.NewStyle().Reverse(true)
		b.WriteString(before)
		b.WriteString(cursorStyle.Render(cursorChar))
		b.WriteString(after)
	}
	b.WriteString("\n\n")

	// Scope chips.
	b.WriteString("  ")
	for i, name := range scopeNames {
		if Scope(i) == p.scope {
			chip := accentStyle.Bold(true).Render("[" + name + "]")
			b.WriteString(chip)
		} else {
			b.WriteString(dimStyle.Render(" " + name + " "))
		}
		b.WriteString(" ")
	}
	b.WriteString("\n\n")

	// Results.
	if len(p.results) == 0 {
		if p.query != "" {
			msg := "  No matches found."
			if !p.index.IsReady() {
				msg += " (indexing...)"
			}
			b.WriteString(dimStyle.Render(msg))
		} else if !p.index.IsReady() && p.index.HasPartialResults() {
			b.WriteString(dimStyle.Render("  Building search index..."))
		}
	} else {
		maxVisible := max(1, height-7) // Account for header, input, chips, padding.
		start := 0
		if p.resCur >= maxVisible {
			start = p.resCur - maxVisible + 1
		}
		end := min(start+maxVisible, len(p.results))

		// Result count.
		countStr := dimStyle.Render(fmt.Sprintf("  %d result", len(p.results)))
		if len(p.results) != 1 {
			countStr += dimStyle.Render("s")
		}
		b.WriteString(countStr)
		b.WriteString("\n\n")

		for i := start; i < end; i++ {
			r := p.results[i]
			cursor := "  "
			if i == p.resCur {
				cursor = activeStyle.Render("> ")
			}

			// Stash header: stash@{N} message.
			stashRef := dimStyle.Render(fmt.Sprintf("stash@{%d}", r.StashIndex))
			fmt.Fprintf(&b, "  %s%s %s\n", cursor, stashRef, r.StashMessage)

			// Match context with highlighted matches.
			if r.MatchText != "" {
				contextLine := "      "
				scopeTag := dimStyle.Render(fmt.Sprintf("[%s]", scopeNames[r.MatchScope]))
				contextLine += scopeTag + " "

				if r.FileName != "" && r.MatchScope == ScopeDiffs {
					contextLine += dimStyle.Render(r.FileName)
					if r.LineNum > 0 {
						contextLine += dimStyle.Render(fmt.Sprintf(":%d", r.LineNum))
					}
					contextLine += " "
				}

				// Render match text with highlighted characters.
				highlighted := highlightMatches(r.MatchText, r.MatchedIndexes, matchStyle)
				contextLine += highlighted
				b.WriteString(contextLine + "\n")
			}
		}
	}

	// Footer hints.
	b.WriteString("\n")
	hints := dimStyle.Render("  Tab: scope  Ctrl+N/P: navigate  Enter: jump  Esc: close")
	b.WriteString(hints)

	return b.String()
}

// highlightMatches renders a string with matched character positions highlighted.
func highlightMatches(text string, indexes []int, style lipgloss.Style) string {
	if len(indexes) == 0 {
		return text
	}

	matchSet := make(map[int]bool, len(indexes))
	for _, idx := range indexes {
		matchSet[idx] = true
	}

	var b strings.Builder
	for i, ch := range text {
		if matchSet[i] {
			b.WriteString(style.Render(string(ch)))
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// ─── Test helpers ───────────────────────────────────────────

// SetQueryForTest sets the search query for testing.
func (p *Plugin) SetQueryForTest(q string) {
	p.query = q
	p.cursor = len(q)
}

// SetScopeForTest sets the scope for testing.
func (p *Plugin) SetScopeForTest(s Scope) {
	p.scope = s
}

// SetResultsForTest sets search results for testing.
func (p *Plugin) SetResultsForTest(results []SearchResult) {
	p.results = results
}

// SetActiveForTest sets the active state for testing.
func (p *Plugin) SetActiveForTest(active bool) {
	p.active = active
}

// QueryForTest returns the current query for testing.
func (p *Plugin) QueryForTest() string {
	return p.query
}

// ScopeForTest returns the current scope for testing.
func (p *Plugin) ScopeForTest() Scope {
	return p.scope
}

// ResultsCursorForTest returns the results cursor for testing.
func (p *Plugin) ResultsCursorForTest() int {
	return p.resCur
}

// ResultsForTest returns the current results for testing.
func (p *Plugin) ResultsForTest() []SearchResult {
	return p.results
}

// IndexForTest returns the index for testing.
func (p *Plugin) IndexForTest() *Index {
	return p.index
}

// IsActiveForTest returns whether the plugin is active.
func (p *Plugin) IsActiveForTest() bool {
	return p.active
}
