# Task 007: Layout Engine and Chrome Components

## Status: TODO

## Depends On
- 006 (Core BubbleTea model, ModeManager, AppState -- layout engine receives state from Model)
- 003 (Agni theme -- all chrome components use theme tokens for styling)

## Parallelizable With
- 008 (Stash row renderer -- can be built simultaneously; layout provides width constraints)
- 009 (Diff view and file tree -- same reasoning)

## Problem
Every screen in nidhi follows the same chrome layout: status bar (1 line) + content area (height - 2) + footer (1 line). This layout contract (PRD Section 9.3) is enforced by the layout engine, not by individual screens. Without it, every screen would have to manually compute available space, repeat status bar rendering, and duplicate footer logic. The layout engine also handles split pane layouts for PREVIEW mode (vertical 40/60 split) and DETAIL mode (horizontal 25/75 split), responsive breakpoint detection (80x24 minimal, 120x40 standard, 200x60 large), and the toast notification system with timed auto-dismiss.

## PRD Reference
- Section 9.3 (Layout Contract) -- status bar + content + footer, `lipgloss.JoinVertical`
- Section 10 Screen 1 (LIST) -- status bar content, footer keybind hints
- Section 10 Screen 2 (PREVIEW) -- vertical split with compressed list + diff viewport
- Section 10 Screen 3 (DETAIL) -- horizontal split with file tree + diff viewport
- Section 11.2 (Complete Keymap) -- mode-specific footer hints
- Section 15.2 (Error Display Patterns) -- toast classes: info, error, undo
- Section 13.2 (LipGloss v2 Features) -- JoinVertical, JoinHorizontal, NewStyle
- Section 9.1 (Agni Theme) -- all color tokens used in chrome
- Section 9.2 (Typography & Icons) -- app mark icon, Nerd Font detection

## Files to Create
- `internal/ui/layout/layout.go` -- LayoutEngine, Render method
- `internal/ui/layout/split.go` -- SplitPane for PREVIEW and DETAIL modes
- `internal/ui/layout/responsive.go` -- Breakpoint detection and column collapse rules
- `internal/ui/components/statusbar.go` -- Status bar renderer
- `internal/ui/components/footer.go` -- Footer with mode-specific keybind hints
- `internal/ui/components/toast.go` -- Timed toast notification with 3 classes
- `internal/ui/layout/layout_test.go` -- Layout math tests
- `internal/ui/layout/split_test.go` -- Split ratio tests
- `internal/ui/layout/responsive_test.go` -- Breakpoint detection tests
- `internal/ui/components/statusbar_test.go` -- Status bar rendering tests
- `internal/ui/components/footer_test.go` -- Footer rendering tests
- `internal/ui/components/toast_test.go` -- Toast timer and class tests

## Execution Steps

### Step 1: Create `internal/ui/layout/responsive.go`

Define breakpoints and the responsive tier system. This must come first because layout and split depend on it.

```go
// internal/ui/layout/responsive.go
package layout

// Tier represents a responsive layout tier.
type Tier int

const (
	// TierMinimal is for terminals 80x24 and below. Single-line stash rows,
	// truncated messages, no split panes.
	TierMinimal Tier = iota
	// TierStandard is for terminals between 80x24 and 120x40. Two-line stash rows,
	// preview split available.
	TierStandard
	// TierLarge is for terminals 200x60 and above. Full detail, generous spacing.
	TierLarge
)

// String returns the tier name.
func (t Tier) String() string {
	switch t {
	case TierMinimal:
		return "minimal"
	case TierStandard:
		return "standard"
	case TierLarge:
		return "large"
	default:
		return "unknown"
	}
}

// Breakpoints defines the width and height thresholds for each tier.
type Breakpoints struct {
	MinimalMaxWidth   int // Below this width -> TierMinimal
	StandardMinWidth  int // At or above this width -> TierStandard
	LargeMinWidth     int // At or above this width -> TierLarge
	MinimalMaxHeight  int
	StandardMinHeight int
	LargeMinHeight    int
}

// DefaultBreakpoints returns the default breakpoints from PRD Section 10.
func DefaultBreakpoints() Breakpoints {
	return Breakpoints{
		MinimalMaxWidth:   79,
		StandardMinWidth:  80,
		LargeMinWidth:     200,
		MinimalMaxHeight:  23,
		StandardMinHeight: 24,
		LargeMinHeight:    60,
	}
}

// DetectTier determines the responsive tier for the given terminal dimensions.
func DetectTier(width, height int, bp Breakpoints) Tier {
	if width >= bp.LargeMinWidth && height >= bp.LargeMinHeight {
		return TierLarge
	}
	if width >= bp.StandardMinWidth && height >= bp.StandardMinHeight {
		return TierStandard
	}
	return TierMinimal
}

// ShouldCollapseTwoLineRows returns true if stash rows should collapse to single-line.
// PRD Screen 1: "Below 100 cols, collapse to single-line rows."
func ShouldCollapseTwoLineRows(width int) bool {
	return width < 100
}

// ShouldTruncateMessage returns true if stash messages should be truncated.
// PRD Screen 1: "Below 80 cols, truncate message."
func ShouldTruncateMessage(width int) bool {
	return width < 80
}

// MaxMessageWidth returns the maximum width available for stash messages
// given the total terminal width and fixed column widths.
// Fixed columns: cursor (2) + index (4) + SHA (9) + age (8) + padding (4) = ~27
func MaxMessageWidth(totalWidth int) int {
	const fixedColumns = 27
	available := totalWidth - fixedColumns
	if available < 10 {
		return 10
	}
	return available
}
```

### Step 2: Create `internal/ui/layout/split.go`

Split pane calculations for PREVIEW and DETAIL modes.

```go
// internal/ui/layout/split.go
package layout

// SplitOrientation defines the direction of a split pane.
type SplitOrientation int

const (
	// SplitVertical splits the content area top/bottom (for PREVIEW mode).
	SplitVertical SplitOrientation = iota
	// SplitHorizontal splits the content area left/right (for DETAIL mode).
	SplitHorizontal
)

// SplitRatio defines a proportional split between two panes.
type SplitRatio struct {
	// Primary is the fraction of space for the primary pane (0.0 to 1.0).
	Primary float64
	// MinPrimary is the minimum size (rows or cols) for the primary pane.
	MinPrimary int
	// MinSecondary is the minimum size (rows or cols) for the secondary pane.
	MinSecondary int
}

// PreviewSplitRatio is the default split for PREVIEW mode.
// PRD Screen 2: list compresses to ~40% of height.
var PreviewSplitRatio = SplitRatio{
	Primary:      0.40,
	MinPrimary:   3, // At least 3 visible stash rows.
	MinSecondary: 5, // At least 5 lines of diff.
}

// DetailSplitRatio is the default split for DETAIL mode.
// PRD Screen 3: file tree ~25% width, diff ~75% width.
var DetailSplitRatio = SplitRatio{
	Primary:      0.25,
	MinPrimary:   15, // At least 15 cols for file tree.
	MinSecondary: 40, // At least 40 cols for diff viewport.
}

// SplitResult holds the computed dimensions of a two-pane split.
type SplitResult struct {
	PrimarySize   int
	SecondarySize int
	// DividerSize is 1 for vertical splits (horizontal line), 1 for horizontal
	// splits (vertical line), 0 if no divider fits.
	DividerSize int
}

// ComputeSplit calculates the sizes of two panes given the total available space
// and a split ratio. The divider takes 1 unit of space.
func ComputeSplit(totalSize int, ratio SplitRatio) SplitResult {
	const dividerSize = 1

	usable := totalSize - dividerSize
	if usable < ratio.MinPrimary+ratio.MinSecondary {
		// Not enough space for both panes + divider.
		// Fall back: give everything to primary (collapse secondary).
		if totalSize < ratio.MinPrimary {
			return SplitResult{
				PrimarySize:   totalSize,
				SecondarySize: 0,
				DividerSize:   0,
			}
		}
		return SplitResult{
			PrimarySize:   totalSize,
			SecondarySize: 0,
			DividerSize:   0,
		}
	}

	primarySize := int(float64(usable) * ratio.Primary)

	// Enforce minimums.
	if primarySize < ratio.MinPrimary {
		primarySize = ratio.MinPrimary
	}

	secondarySize := usable - primarySize
	if secondarySize < ratio.MinSecondary {
		secondarySize = ratio.MinSecondary
		primarySize = usable - secondarySize
	}

	return SplitResult{
		PrimarySize:   primarySize,
		SecondarySize: secondarySize,
		DividerSize:   dividerSize,
	}
}
```

### Step 3: Create `internal/ui/layout/layout.go`

The layout engine computes the three-band layout and provides a `Render` function.

```go
// internal/ui/layout/layout.go
package layout

import (
	lipgloss "github.com/charmbracelet/lipgloss/v2"
)

const (
	// StatusBarHeight is always 1 line.
	StatusBarHeight = 1
	// FooterHeight is always 1 line.
	FooterHeight = 1
	// ChromeHeight is the total vertical space consumed by status bar + footer.
	ChromeHeight = StatusBarHeight + FooterHeight
)

// Dimensions holds the computed dimensions for each layout region.
type Dimensions struct {
	// Total terminal dimensions.
	TotalWidth  int
	TotalHeight int

	// Content area (between status bar and footer).
	ContentWidth  int
	ContentHeight int

	// Responsive tier.
	Tier Tier
}

// ComputeDimensions calculates the available content area given terminal size.
func ComputeDimensions(width, height int) Dimensions {
	contentHeight := height - ChromeHeight
	if contentHeight < 0 {
		contentHeight = 0
	}

	return Dimensions{
		TotalWidth:    width,
		TotalHeight:   height,
		ContentWidth:  width,
		ContentHeight: contentHeight,
		Tier:          DetectTier(width, height, DefaultBreakpoints()),
	}
}

// Render composes the three-band layout: status bar + content + footer.
// All three strings must already be rendered to the correct width.
func Render(statusBar, content, footer string) string {
	return lipgloss.JoinVertical(lipgloss.Top, statusBar, content, footer)
}

// RenderSplitVertical composes a vertical split (top/bottom panes with divider).
// Used for PREVIEW mode (list on top, diff on bottom).
func RenderSplitVertical(top, divider, bottom string) string {
	return lipgloss.JoinVertical(lipgloss.Top, top, divider, bottom)
}

// RenderSplitHorizontal composes a horizontal split (left/right panes).
// Used for DETAIL mode (file tree on left, diff on right).
func RenderSplitHorizontal(left, right string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// PadToWidth pads a string to the given width using spaces.
// If the string is already wider, it is truncated.
func PadToWidth(s string, width int) string {
	style := lipgloss.NewStyle().Width(width).MaxWidth(width)
	return style.Render(s)
}

// PadToHeight pads a string to the given height by appending empty lines.
func PadToHeight(s string, height int) string {
	style := lipgloss.NewStyle().Height(height).MaxHeight(height)
	return style.Render(s)
}

// FitContent constrains content to the given width and height.
func FitContent(s string, width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Height(height).
		MaxHeight(height)
	return style.Render(s)
}
```

### Step 4: Create `internal/ui/components/statusbar.go`

```go
// internal/ui/components/statusbar.go
package components

import (
	"fmt"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// StatusBarParams holds the data needed to render the status bar.
type StatusBarParams struct {
	Branch     string
	StashCount int
	GitVersion plugin.GitVersion
	Width      int
	UseNerd    bool // Whether to use Nerd Font icons.
}

// StatusBar renders the status bar: app icon + "nidhi" + branch + stash count + git version.
// PRD Section 9.3: status bar is always 1 line.
type StatusBar struct {
	// Theme tokens (hex colors).
	BgColor      string // bg.surface
	FgColor      string // fg.primary
	AccentColor  string // accent.gold
	BranchColor  string // semantic.blue
	DimmedColor  string // fg.secondary
}

// NewStatusBar creates a StatusBar with default Agni theme colors.
func NewStatusBar() StatusBar {
	return StatusBar{
		BgColor:     "#0F1219",
		FgColor:     "#C8CCD4",
		AccentColor: "#D4A050",
		BranchColor: "#61AFEF",
		DimmedColor: "#6B7280",
	}
}

// Render renders the status bar for the given params.
func (sb StatusBar) Render(p StatusBarParams) string {
	bg := lipgloss.Color(sb.BgColor)
	fg := lipgloss.Color(sb.FgColor)
	accent := lipgloss.Color(sb.AccentColor)
	branchColor := lipgloss.Color(sb.BranchColor)
	dimmed := lipgloss.Color(sb.DimmedColor)

	barStyle := lipgloss.NewStyle().
		Background(bg).
		Width(p.Width).
		MaxWidth(p.Width)

	// App icon + name.
	var appMark string
	if p.UseNerd {
		appMark = "\U000F062C" // nf-md-source_branch (U+F062C)
	} else {
		appMark = "\u2261" // ≡
	}

	iconStyle := lipgloss.NewStyle().
		Foreground(accent).
		Background(bg).
		Bold(true)

	nameStyle := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg).
		Bold(true)

	branchStyle := lipgloss.NewStyle().
		Foreground(branchColor).
		Background(bg)

	countStyle := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg)

	versionStyle := lipgloss.NewStyle().
		Foreground(dimmed).
		Background(bg)

	// Build left side.
	left := iconStyle.Render(appMark) + " " +
		nameStyle.Render("nidhi") + "  " +
		branchStyle.Render(p.Branch) + "  " +
		countStyle.Render(fmt.Sprintf("%d stashes", p.StashCount))

	// Build right side.
	right := versionStyle.Render(fmt.Sprintf("git %d.%d", p.GitVersion.Major, p.GitVersion.Minor))

	// Compute spacing between left and right.
	// Use lipgloss string width for accurate measurement with wide chars.
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := p.Width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}
	spacing := lipgloss.NewStyle().
		Background(bg).
		Width(gap).
		Render("")

	return barStyle.Render(left + spacing + right)
}
```

### Step 5: Create `internal/ui/components/footer.go`

```go
// internal/ui/components/footer.go
package components

import (
	"fmt"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

// KeyHint represents a single key hint shown in the footer.
type KeyHint struct {
	Key  string // e.g. "j/k"
	Desc string // e.g. "nav"
}

// FooterParams holds the data needed to render the footer.
type FooterParams struct {
	Mode  plugin.Mode
	Width int
}

// Footer renders the context-sensitive footer bar.
// PRD Section 9.3: footer is 1 line with keybind hints + mode badge.
type Footer struct {
	BgColor     string // bg.surface
	FgColor     string // fg.secondary
	KeyColor    string // fg.primary
	BadgeBg     string // accent.gold
	BadgeFg     string // bg.deep
}

// NewFooter creates a Footer with default Agni theme colors.
func NewFooter() Footer {
	return Footer{
		BgColor:  "#0F1219",
		FgColor:  "#6B7280",
		KeyColor: "#C8CCD4",
		BadgeBg:  "#D4A050",
		BadgeFg:  "#07090E",
	}
}

// HintsForMode returns the keybind hints for a given mode.
// From PRD Section 11.2 complete keymap.
func HintsForMode(mode plugin.Mode) []KeyHint {
	switch mode {
	case plugin.ModeList:
		return []KeyHint{
			{"j/k", "nav"},
			{"a", "apply"},
			{"p", "pop"},
			{"d", "drop"},
			{"n", "new"},
			{"/", "find"},
			{"Tab", "preview"},
			{"Enter", "detail"},
		}
	case plugin.ModePreview:
		return []KeyHint{
			{"j/k", "nav"},
			{"h/l", "files"},
			{"Tab", "list"},
			{"\u2303d/\u2303u", "scroll"},
			{"Enter", "detail"},
		}
	case plugin.ModeDetail:
		return []KeyHint{
			{"Tab", "focus"},
			{"j/k", "nav"},
			{"\u2303d/\u2303u", "scroll"},
			{"Esc", "back"},
		}
	case plugin.ModeSearch:
		return []KeyHint{
			{"Tab", "scope"},
			{"Enter", "jump"},
			{"Esc", "close"},
		}
	case plugin.ModeNewStash, plugin.ModeExport, plugin.ModeImport:
		return []KeyHint{
			{"Tab", "next"},
			{"Space", "toggle"},
			{"Enter", "confirm"},
			{"Esc", "cancel"},
		}
	case plugin.ModeConflict:
		return []KeyHint{
			{"a", "apply anyway"},
			{"b", "branch first"},
			{"Esc", "cancel"},
		}
	case plugin.ModeHelp:
		return []KeyHint{
			{"Esc", "close"},
			{"?", "close"},
		}
	default:
		return []KeyHint{{"Esc", "back"}}
	}
}

// Render renders the footer for the given params.
func (f Footer) Render(p FooterParams) string {
	bg := lipgloss.Color(f.BgColor)
	fg := lipgloss.Color(f.FgColor)
	keyFg := lipgloss.Color(f.KeyColor)
	badgeBg := lipgloss.Color(f.BadgeBg)
	badgeFg := lipgloss.Color(f.BadgeFg)

	barStyle := lipgloss.NewStyle().
		Background(bg).
		Width(p.Width).
		MaxWidth(p.Width)

	keyStyle := lipgloss.NewStyle().
		Foreground(keyFg).
		Background(bg).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(fg).
		Background(bg)

	badgeStyle := lipgloss.NewStyle().
		Foreground(badgeFg).
		Background(badgeBg).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	// Build hints.
	hints := HintsForMode(p.Mode)
	var hintsStr string
	for i, h := range hints {
		if i > 0 {
			hintsStr += descStyle.Render("  ")
		}
		hintsStr += keyStyle.Render(h.Key) + " " + descStyle.Render(h.Desc)
	}

	// Mode badge (right-aligned).
	badge := badgeStyle.Render(p.Mode.String())

	// Compute spacing.
	hintsWidth := lipgloss.Width(hintsStr)
	badgeWidth := lipgloss.Width(badge)
	gap := p.Width - hintsWidth - badgeWidth - 2 // 2 for margin
	if gap < 1 {
		gap = 1
	}
	spacing := lipgloss.NewStyle().
		Background(bg).
		Width(gap).
		Render("")

	return barStyle.Render(" " + hintsStr + spacing + badge)
}

// BadgeColorForMode returns a semantic color for the mode badge.
func BadgeColorForMode(mode plugin.Mode) string {
	switch mode {
	case plugin.ModeList:
		return "#D4A050" // accent.gold
	case plugin.ModePreview:
		return "#61AFEF" // semantic.blue
	case plugin.ModeDetail:
		return "#4EC9B0" // semantic.aqua
	case plugin.ModeSearch:
		return "#C678DD" // semantic.purple
	case plugin.ModeConflict:
		return "#E5C07B" // semantic.yellow
	default:
		return "#6B7280" // fg.secondary
	}
}
```

### Step 6: Create `internal/ui/components/toast.go`

```go
// internal/ui/components/toast.go
package components

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "github.com/charmbracelet/lipgloss/v2"
)

// ToastClass defines the visual class of a toast notification.
type ToastClass int

const (
	// ToastInfo is a green-bordered informational toast (auto-dismiss 5s).
	ToastInfo ToastClass = iota
	// ToastError is a red-bordered error toast (auto-dismiss 5s).
	ToastError
	// ToastUndo is a yellow-bordered undo toast (auto-dismiss 30s) with recovery key.
	ToastUndo
)

// ToastDuration returns the auto-dismiss duration for a toast class.
func (c ToastClass) Duration() time.Duration {
	switch c {
	case ToastInfo:
		return 5 * time.Second
	case ToastError:
		return 5 * time.Second
	case ToastUndo:
		return 30 * time.Second
	default:
		return 5 * time.Second
	}
}

// String returns the class name.
func (c ToastClass) String() string {
	switch c {
	case ToastInfo:
		return "info"
	case ToastError:
		return "error"
	case ToastUndo:
		return "undo"
	default:
		return "unknown"
	}
}

// ToastBorderColor returns the Agni theme border color for a toast class.
func (c ToastClass) BorderColor() string {
	switch c {
	case ToastInfo:
		return "#73D990" // semantic.green
	case ToastError:
		return "#FF5F6D" // semantic.red
	case ToastUndo:
		return "#E5C07B" // semantic.yellow
	default:
		return "#6B7280"
	}
}

// Toast represents a single toast notification.
type Toast struct {
	Message     string
	Class       ToastClass
	RecoveryKey string    // Only for ToastUndo (e.g. "z").
	CreatedAt   time.Time
	Duration    time.Duration
}

// IsExpired returns true if the toast has outlived its duration.
func (t Toast) IsExpired() bool {
	return time.Since(t.CreatedAt) >= t.Duration
}

// RemainingSeconds returns the seconds remaining before auto-dismiss.
func (t Toast) RemainingSeconds() int {
	remaining := t.Duration - time.Since(t.CreatedAt)
	if remaining < 0 {
		return 0
	}
	return int(remaining.Seconds())
}

// ToastModel manages toast display state. It handles the tick timer
// for auto-dismiss and renders the toast overlay.
type ToastModel struct {
	active *Toast
	width  int
}

// NewToastModel creates a new toast model.
func NewToastModel() ToastModel {
	return ToastModel{}
}

// Show displays a new toast, replacing any existing one.
func (m *ToastModel) Show(message string, class ToastClass) tea.Cmd {
	m.active = &Toast{
		Message:   message,
		Class:     class,
		CreatedAt: time.Now(),
		Duration:  class.Duration(),
	}
	return m.tick()
}

// ShowUndo displays an undo toast with a recovery key.
func (m *ToastModel) ShowUndo(message, recoveryKey string) tea.Cmd {
	m.active = &Toast{
		Message:     message,
		Class:       ToastUndo,
		RecoveryKey: recoveryKey,
		CreatedAt:   time.Now(),
		Duration:    ToastUndo.Duration(),
	}
	return m.tick()
}

// Dismiss clears the active toast.
func (m *ToastModel) Dismiss() {
	m.active = nil
}

// IsVisible returns true if a toast is currently displayed.
func (m *ToastModel) IsVisible() bool {
	return m.active != nil && !m.active.IsExpired()
}

// Active returns the active toast, or nil if none.
func (m *ToastModel) Active() *Toast {
	if m.active == nil || m.active.IsExpired() {
		return nil
	}
	return m.active
}

// SetWidth sets the available width for rendering.
func (m *ToastModel) SetWidth(w int) {
	m.width = w
}

// toastTickMsg is the internal tick message for auto-dismiss.
type toastTickMsg struct{}

// Update handles tick messages for auto-dismiss.
func (m *ToastModel) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case toastTickMsg:
		if m.active == nil {
			return nil
		}
		if m.active.IsExpired() {
			m.active = nil
			return nil
		}
		return m.tick()
	}
	return nil
}

// tick returns a Cmd that sends a toastTickMsg after 1 second.
func (m *ToastModel) tick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return toastTickMsg{}
	})
}

// View renders the toast notification. Returns empty string if no toast is active.
func (m *ToastModel) View() string {
	toast := m.Active()
	if toast == nil {
		return ""
	}

	borderColor := lipgloss.Color(toast.Class.BorderColor())

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Foreground(lipgloss.Color("#C8CCD4")).
		Background(lipgloss.Color("#1A1F2B")).
		PaddingLeft(1).
		PaddingRight(1)

	var content string
	if toast.Class == ToastUndo && toast.RecoveryKey != "" {
		remaining := toast.RemainingSeconds()
		content = fmt.Sprintf("%s Press %s to undo (%ds)", toast.Message, toast.RecoveryKey, remaining)
	} else {
		content = toast.Message
	}

	// Constrain to width.
	maxWidth := m.width - 4 // Account for borders and padding.
	if maxWidth > 0 {
		style = style.MaxWidth(maxWidth)
	}

	return style.Render(content)
}
```

### Step 7: Create test files

#### `internal/ui/layout/layout_test.go`

```go
// internal/ui/layout/layout_test.go
package layout

import (
	"testing"
)

func TestComputeDimensions(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		wantContent   int
		wantTier      Tier
	}{
		{"standard 80x24", 80, 24, 22, TierStandard},
		{"standard 120x40", 120, 40, 38, TierStandard},
		{"large 200x60", 200, 60, 58, TierLarge},
		{"minimal 60x20", 60, 20, 18, TierMinimal},
		{"tiny 40x10", 40, 10, 8, TierMinimal},
		{"zero height", 80, 0, 0, TierMinimal},
		{"height 1", 80, 1, 0, TierMinimal},
		{"height 2", 80, 2, 0, TierMinimal},
		{"height 3", 80, 3, 1, TierMinimal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := ComputeDimensions(tt.width, tt.height)
			if d.ContentHeight != tt.wantContent {
				t.Errorf("ContentHeight = %d, want %d", d.ContentHeight, tt.wantContent)
			}
			if d.ContentWidth != tt.width {
				t.Errorf("ContentWidth = %d, want %d", d.ContentWidth, tt.width)
			}
			if d.Tier != tt.wantTier {
				t.Errorf("Tier = %s, want %s", d.Tier, tt.wantTier)
			}
		})
	}
}

func TestChromeHeight(t *testing.T) {
	// Status bar (1) + Footer (1) = 2.
	if ChromeHeight != 2 {
		t.Errorf("ChromeHeight = %d, want 2", ChromeHeight)
	}
}

func TestComputeDimensions_ContentNeverNegative(t *testing.T) {
	d := ComputeDimensions(80, 0)
	if d.ContentHeight < 0 {
		t.Errorf("ContentHeight should never be negative, got %d", d.ContentHeight)
	}

	d = ComputeDimensions(80, 1)
	if d.ContentHeight < 0 {
		t.Errorf("ContentHeight should never be negative for height=1, got %d", d.ContentHeight)
	}
}
```

#### `internal/ui/layout/split_test.go`

```go
// internal/ui/layout/split_test.go
package layout

import (
	"testing"
)

func TestComputeSplit_Preview(t *testing.T) {
	// PREVIEW mode: 40/60 vertical split on a 40-line content area.
	result := ComputeSplit(40, PreviewSplitRatio)

	// Primary (list) should be ~40% of (40 - 1 divider) = ~15.6 -> 15
	if result.PrimarySize < 3 {
		t.Errorf("PrimarySize = %d, want >= 3 (MinPrimary)", result.PrimarySize)
	}
	if result.SecondarySize < 5 {
		t.Errorf("SecondarySize = %d, want >= 5 (MinSecondary)", result.SecondarySize)
	}
	if result.DividerSize != 1 {
		t.Errorf("DividerSize = %d, want 1", result.DividerSize)
	}

	total := result.PrimarySize + result.SecondarySize + result.DividerSize
	if total != 40 {
		t.Errorf("total %d + %d + %d = %d, want 40",
			result.PrimarySize, result.SecondarySize, result.DividerSize, total)
	}
}

func TestComputeSplit_Detail(t *testing.T) {
	// DETAIL mode: 25/75 horizontal split on 120-col content area.
	result := ComputeSplit(120, DetailSplitRatio)

	// Primary (tree) should be ~25% of (120 - 1) = ~29.75 -> 29
	if result.PrimarySize < 15 {
		t.Errorf("PrimarySize = %d, want >= 15 (MinPrimary)", result.PrimarySize)
	}
	if result.SecondarySize < 40 {
		t.Errorf("SecondarySize = %d, want >= 40 (MinSecondary)", result.SecondarySize)
	}

	total := result.PrimarySize + result.SecondarySize + result.DividerSize
	if total != 120 {
		t.Errorf("total = %d, want 120", total)
	}
}

func TestComputeSplit_TooSmall(t *testing.T) {
	// Not enough space for both panes.
	result := ComputeSplit(10, DetailSplitRatio)

	// Should collapse: everything to primary, no divider.
	if result.SecondarySize != 0 {
		t.Errorf("SecondarySize = %d, want 0 (collapsed)", result.SecondarySize)
	}
	if result.DividerSize != 0 {
		t.Errorf("DividerSize = %d, want 0 (collapsed)", result.DividerSize)
	}
}

func TestComputeSplit_MinimumEnforced(t *testing.T) {
	// Just barely enough for both panes.
	minTotal := PreviewSplitRatio.MinPrimary + PreviewSplitRatio.MinSecondary + 1
	result := ComputeSplit(minTotal, PreviewSplitRatio)

	if result.PrimarySize < PreviewSplitRatio.MinPrimary {
		t.Errorf("PrimarySize = %d, below minimum %d",
			result.PrimarySize, PreviewSplitRatio.MinPrimary)
	}
	if result.SecondarySize < PreviewSplitRatio.MinSecondary {
		t.Errorf("SecondarySize = %d, below minimum %d",
			result.SecondarySize, PreviewSplitRatio.MinSecondary)
	}
}

func TestComputeSplit_SumsCorrectly(t *testing.T) {
	// Test across a range of sizes.
	for totalSize := 10; totalSize <= 200; totalSize++ {
		result := ComputeSplit(totalSize, PreviewSplitRatio)
		actual := result.PrimarySize + result.SecondarySize + result.DividerSize
		if result.SecondarySize > 0 && actual != totalSize {
			t.Errorf("totalSize=%d: %d + %d + %d = %d, want %d",
				totalSize, result.PrimarySize, result.SecondarySize, result.DividerSize,
				actual, totalSize)
		}
	}
}
```

#### `internal/ui/layout/responsive_test.go`

```go
// internal/ui/layout/responsive_test.go
package layout

import (
	"testing"
)

func TestDetectTier(t *testing.T) {
	bp := DefaultBreakpoints()

	tests := []struct {
		name   string
		w, h   int
		want   Tier
	}{
		{"minimal 60x20", 60, 20, TierMinimal},
		{"minimal 79x23", 79, 23, TierMinimal},
		{"standard 80x24", 80, 24, TierStandard},
		{"standard 120x40", 120, 40, TierStandard},
		{"standard 199x59", 199, 59, TierStandard},
		{"large 200x60", 200, 60, TierLarge},
		{"large 300x100", 300, 100, TierLarge},
		{"wide but short", 200, 20, TierMinimal},
		{"tall but narrow", 60, 60, TierMinimal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTier(tt.w, tt.h, bp)
			if got != tt.want {
				t.Errorf("DetectTier(%d, %d) = %s, want %s", tt.w, tt.h, got, tt.want)
			}
		})
	}
}

func TestShouldCollapseTwoLineRows(t *testing.T) {
	tests := []struct {
		width int
		want  bool
	}{
		{60, true},
		{80, true},
		{99, true},
		{100, false},
		{120, false},
		{200, false},
	}

	for _, tt := range tests {
		got := ShouldCollapseTwoLineRows(tt.width)
		if got != tt.want {
			t.Errorf("ShouldCollapseTwoLineRows(%d) = %v, want %v", tt.width, got, tt.want)
		}
	}
}

func TestShouldTruncateMessage(t *testing.T) {
	tests := []struct {
		width int
		want  bool
	}{
		{60, true},
		{79, true},
		{80, false},
		{120, false},
	}

	for _, tt := range tests {
		got := ShouldTruncateMessage(tt.width)
		if got != tt.want {
			t.Errorf("ShouldTruncateMessage(%d) = %v, want %v", tt.width, got, tt.want)
		}
	}
}

func TestMaxMessageWidth(t *testing.T) {
	tests := []struct {
		totalWidth int
		wantMin    int
	}{
		{80, 10},
		{120, 80},
		{200, 150},
		{30, 10}, // Very small terminal, clamped to minimum 10.
	}

	for _, tt := range tests {
		got := MaxMessageWidth(tt.totalWidth)
		if got < tt.wantMin {
			t.Errorf("MaxMessageWidth(%d) = %d, want >= %d", tt.totalWidth, got, tt.wantMin)
		}
	}
}

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierMinimal, "minimal"},
		{TierStandard, "standard"},
		{TierLarge, "large"},
		{Tier(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.tier.String(); got != tt.want {
			t.Errorf("Tier(%d).String() = %q, want %q", tt.tier, got, tt.want)
		}
	}
}
```

#### `internal/ui/components/statusbar_test.go`

```go
// internal/ui/components/statusbar_test.go
package components

import (
	"strings"
	"testing"

	lipgloss "github.com/charmbracelet/lipgloss/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestStatusBar_ContainsExpectedElements(t *testing.T) {
	sb := NewStatusBar()
	rendered := sb.Render(StatusBarParams{
		Branch:     "main",
		StashCount: 5,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 53},
		Width:      120,
		UseNerd:    false,
	})

	// Strip ANSI for content checks.
	plain := stripAnsi(rendered)

	checks := []struct {
		name    string
		contains string
	}{
		{"app name", "nidhi"},
		{"branch", "main"},
		{"stash count", "5 stashes"},
		{"git version", "git 2.53"},
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(plain, c.contains) {
				t.Errorf("status bar should contain %q, got: %q", c.contains, plain)
			}
		})
	}
}

func TestStatusBar_ASCIIFallback(t *testing.T) {
	sb := NewStatusBar()
	rendered := sb.Render(StatusBarParams{
		Branch:     "main",
		StashCount: 0,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 53},
		Width:      80,
		UseNerd:    false,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "\u2261") && !strings.Contains(plain, "nidhi") {
		t.Errorf("ASCII mode should use ≡ or at least show 'nidhi', got: %q", plain)
	}
}

func TestStatusBar_ZeroStashes(t *testing.T) {
	sb := NewStatusBar()
	rendered := sb.Render(StatusBarParams{
		Branch:     "feature/auth",
		StashCount: 0,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 38},
		Width:      80,
		UseNerd:    false,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "0 stashes") {
		t.Errorf("should show '0 stashes', got: %q", plain)
	}
}

func TestStatusBar_Width(t *testing.T) {
	sb := NewStatusBar()
	rendered := sb.Render(StatusBarParams{
		Branch:     "main",
		StashCount: 5,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 53},
		Width:      80,
		UseNerd:    false,
	})

	width := lipgloss.Width(rendered)
	if width > 80 {
		t.Errorf("status bar width = %d, should be <= 80", width)
	}
}

// stripAnsi removes ANSI escape sequences for content testing.
func stripAnsi(s string) string {
	// Simple ANSI stripper for test purposes.
	result := make([]byte, 0, len(s))
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result = append(result, s[i])
	}
	return string(result)
}
```

#### `internal/ui/components/footer_test.go`

```go
// internal/ui/components/footer_test.go
package components

import (
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestHintsForMode_ListMode(t *testing.T) {
	hints := HintsForMode(plugin.ModeList)
	if len(hints) == 0 {
		t.Fatal("LIST mode should have keybind hints")
	}

	// Check that essential LIST keys are present.
	keys := make(map[string]bool)
	for _, h := range hints {
		keys[h.Key] = true
	}

	required := []string{"j/k", "a", "d", "/"}
	for _, k := range required {
		if !keys[k] {
			t.Errorf("LIST mode missing key hint %q", k)
		}
	}
}

func TestHintsForMode_PreviewMode(t *testing.T) {
	hints := HintsForMode(plugin.ModePreview)
	keys := make(map[string]bool)
	for _, h := range hints {
		keys[h.Key] = true
	}

	if !keys["h/l"] {
		t.Error("PREVIEW mode should have h/l for file cycling")
	}
	if !keys["Tab"] {
		t.Error("PREVIEW mode should have Tab to toggle back to list")
	}
}

func TestHintsForMode_AllModesHaveHints(t *testing.T) {
	modes := []plugin.Mode{
		plugin.ModeList, plugin.ModePreview, plugin.ModeDetail,
		plugin.ModeSearch, plugin.ModeNewStash, plugin.ModeExport,
		plugin.ModeImport, plugin.ModeConflict, plugin.ModeHelp,
	}

	for _, mode := range modes {
		hints := HintsForMode(mode)
		if len(hints) == 0 {
			t.Errorf("mode %s has no keybind hints", mode)
		}
	}
}

func TestFooter_ContainsModeBadge(t *testing.T) {
	f := NewFooter()
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModeList,
		Width: 120,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "LIST") {
		t.Errorf("footer should contain mode badge 'LIST', got: %q", plain)
	}
}

func TestFooter_ContainsKeyHints(t *testing.T) {
	f := NewFooter()
	rendered := f.Render(FooterParams{
		Mode:  plugin.ModeList,
		Width: 120,
	})

	plain := stripAnsi(rendered)
	if !strings.Contains(plain, "apply") {
		t.Errorf("footer should contain 'apply' hint, got: %q", plain)
	}
}

func TestBadgeColorForMode(t *testing.T) {
	tests := []struct {
		mode    plugin.Mode
		wantNon string
	}{
		{plugin.ModeList, "#D4A050"},
		{plugin.ModePreview, "#61AFEF"},
		{plugin.ModeDetail, "#4EC9B0"},
		{plugin.ModeSearch, "#C678DD"},
		{plugin.ModeConflict, "#E5C07B"},
	}

	for _, tt := range tests {
		got := BadgeColorForMode(tt.mode)
		if got != tt.wantNon {
			t.Errorf("BadgeColorForMode(%s) = %q, want %q", tt.mode, got, tt.wantNon)
		}
	}
}
```

#### `internal/ui/components/toast_test.go`

```go
// internal/ui/components/toast_test.go
package components

import (
	"testing"
	"time"
)

func TestToastClass_Duration(t *testing.T) {
	tests := []struct {
		class ToastClass
		want  time.Duration
	}{
		{ToastInfo, 5 * time.Second},
		{ToastError, 5 * time.Second},
		{ToastUndo, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.class.String(), func(t *testing.T) {
			if got := tt.class.Duration(); got != tt.want {
				t.Errorf("Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToastClass_BorderColor(t *testing.T) {
	tests := []struct {
		class ToastClass
		want  string
	}{
		{ToastInfo, "#73D990"},
		{ToastError, "#FF5F6D"},
		{ToastUndo, "#E5C07B"},
	}

	for _, tt := range tests {
		t.Run(tt.class.String(), func(t *testing.T) {
			if got := tt.class.BorderColor(); got != tt.want {
				t.Errorf("BorderColor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToast_IsExpired(t *testing.T) {
	// Fresh toast should not be expired.
	toast := Toast{
		CreatedAt: time.Now(),
		Duration:  5 * time.Second,
	}
	if toast.IsExpired() {
		t.Error("fresh toast should not be expired")
	}

	// Toast created 10 seconds ago with 5s duration should be expired.
	toast = Toast{
		CreatedAt: time.Now().Add(-10 * time.Second),
		Duration:  5 * time.Second,
	}
	if !toast.IsExpired() {
		t.Error("old toast should be expired")
	}
}

func TestToast_RemainingSeconds(t *testing.T) {
	// Toast with 30s duration created 10s ago should have ~20s remaining.
	toast := Toast{
		CreatedAt: time.Now().Add(-10 * time.Second),
		Duration:  30 * time.Second,
	}
	remaining := toast.RemainingSeconds()
	if remaining < 19 || remaining > 21 {
		t.Errorf("RemainingSeconds() = %d, want ~20", remaining)
	}
}

func TestToast_RemainingSecondsExpired(t *testing.T) {
	toast := Toast{
		CreatedAt: time.Now().Add(-60 * time.Second),
		Duration:  5 * time.Second,
	}
	if toast.RemainingSeconds() != 0 {
		t.Errorf("expired toast RemainingSeconds() = %d, want 0", toast.RemainingSeconds())
	}
}

func TestToastModel_ShowAndDismiss(t *testing.T) {
	tm := NewToastModel()

	if tm.IsVisible() {
		t.Error("new ToastModel should not be visible")
	}

	tm.Show("Test message", ToastInfo)

	if !tm.IsVisible() {
		t.Error("toast should be visible after Show()")
	}

	active := tm.Active()
	if active == nil {
		t.Fatal("Active() should not be nil")
	}
	if active.Message != "Test message" {
		t.Errorf("Message = %q, want %q", active.Message, "Test message")
	}
	if active.Class != ToastInfo {
		t.Errorf("Class = %v, want ToastInfo", active.Class)
	}

	tm.Dismiss()

	if tm.IsVisible() {
		t.Error("toast should not be visible after Dismiss()")
	}
}

func TestToastModel_ShowUndo(t *testing.T) {
	tm := NewToastModel()
	tm.ShowUndo("Dropped stash@{0}", "z")

	active := tm.Active()
	if active == nil {
		t.Fatal("Active() should not be nil")
	}
	if active.Class != ToastUndo {
		t.Errorf("Class = %v, want ToastUndo", active.Class)
	}
	if active.RecoveryKey != "z" {
		t.Errorf("RecoveryKey = %q, want %q", active.RecoveryKey, "z")
	}
}

func TestToastModel_ShowReplacesExisting(t *testing.T) {
	tm := NewToastModel()
	tm.Show("First", ToastInfo)
	tm.Show("Second", ToastError)

	active := tm.Active()
	if active == nil {
		t.Fatal("Active() should not be nil")
	}
	if active.Message != "Second" {
		t.Errorf("show should replace existing: Message = %q, want %q", active.Message, "Second")
	}
}

func TestToastModel_ViewEmpty(t *testing.T) {
	tm := NewToastModel()
	tm.SetWidth(80)

	view := tm.View()
	if view != "" {
		t.Errorf("View() with no toast should be empty, got: %q", view)
	}
}

func TestToastModel_ViewRendersContent(t *testing.T) {
	tm := NewToastModel()
	tm.SetWidth(80)
	tm.Show("Operation succeeded", ToastInfo)

	view := tm.View()
	if view == "" {
		t.Error("View() should not be empty when toast is active")
	}
}

func TestToastModel_UndoViewShowsRecoveryKey(t *testing.T) {
	tm := NewToastModel()
	tm.SetWidth(120)
	tm.ShowUndo("Dropped stash@{0}", "z")

	view := tm.View()
	plain := stripAnsi(view)

	if !containsString(plain, "undo") {
		t.Errorf("undo toast should contain 'undo', got: %q", plain)
	}
}

func TestToastClass_String(t *testing.T) {
	tests := []struct {
		class ToastClass
		want  string
	}{
		{ToastInfo, "info"},
		{ToastError, "error"},
		{ToastUndo, "undo"},
	}

	for _, tt := range tests {
		if got := tt.class.String(); got != tt.want {
			t.Errorf("ToastClass(%d).String() = %q, want %q", tt.class, got, tt.want)
		}
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle ||
		len(haystack) > 0 && containsStringHelper(haystack, needle))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

## Verification

### Functional
```bash
# From project root
cd /Users/indrasvat/code/github.com/indrasvat-nidhi

# Ensure directory structure exists
ls internal/ui/layout/
ls internal/ui/components/

# Run tests for layout package
go test -v -race ./internal/ui/layout/...

# Run tests for components package
go test -v -race ./internal/ui/components/...

# Run all UI tests
go test -v -race ./internal/ui/...

# Check compilation
go build ./internal/ui/...

# Linter
make lint

# Full CI
make ci
```

## Completion Criteria
1. All source files compile: `layout.go`, `split.go`, `responsive.go`, `statusbar.go`, `footer.go`, `toast.go`
2. All test files pass with `go test -v -race ./internal/ui/...`
3. `ComputeDimensions` correctly computes content height as `terminal_height - 2`
4. Content height never goes negative (clamped to 0)
5. `ComputeSplit` for PREVIEW ratio produces ~40/60 split; sums equal total size
6. `ComputeSplit` for DETAIL ratio produces ~25/75 split; sums equal total size
7. `ComputeSplit` collapses gracefully when space is too small for both panes
8. `DetectTier` correctly classifies all breakpoint combinations
9. `ShouldCollapseTwoLineRows` returns true below 100 cols, false at 100+
10. Status bar renders with app name, branch, stash count, git version
11. Footer renders mode-specific keybind hints with a right-aligned mode badge
12. Toast model supports info/error/undo classes with correct durations (5s/5s/30s)
13. Toast auto-dismiss works via tick mechanism; undo toasts show recovery key and countdown
14. `make lint` passes with no warnings

## Commit
```
feat(ui): add layout engine, status bar, footer, and toast components

Implement the UI framework layer:
- layout/layout.go: three-band layout (status bar + content + footer),
  lipgloss.JoinVertical composition, dimension helpers
- layout/split.go: SplitPane with configurable ratios for PREVIEW
  (40/60 vertical) and DETAIL (25/75 horizontal) modes
- layout/responsive.go: three-tier breakpoints (minimal/standard/large),
  column collapse rules from PRD Section 10
- components/statusbar.go: renders app icon, branch, stash count, git
  version with Agni theme tokens
- components/footer.go: mode-specific keybind hints with color-coded
  mode badge
- components/toast.go: timed toast with 3 classes (info 5s, error 5s,
  undo 30s), auto-dismiss via tea.Tick, recovery key display
- Full tests for layout math, split ratios, responsive detection, toast
  timer, and chrome rendering
```

## Session Protocol
1. Read this task file completely before writing any code.
2. Verify Task 006 (core model) and Task 003 (theme) are complete.
3. Create `internal/ui/layout/` and `internal/ui/components/` directories if they do not exist.
4. Write layout files first: `responsive.go`, `split.go`, `layout.go`.
5. Write component files: `statusbar.go`, `footer.go`, `toast.go`.
6. Write all test files.
7. Run `go test -v -race ./internal/ui/...` and fix any failures. Update PROGRESS.md and CLAUDE.md.
