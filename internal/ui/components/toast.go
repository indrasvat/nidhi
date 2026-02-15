package components

import (
	"fmt"
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ToastClass defines the visual class of a toast notification.
type ToastClass int

const (
	// ToastInfo is a green toast for success messages (auto-dismiss 5s).
	// Mockup: .toast-ok with --green color.
	ToastInfo ToastClass = iota
	// ToastError is a red toast for errors (auto-dismiss 5s).
	ToastError
	// ToastUndo is a blue toast with recovery key (auto-dismiss 30s).
	// Mockup: .toast-undo with --blue color, not yellow.
	ToastUndo
)

// Duration returns the auto-dismiss duration for a toast class.
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

// Toast represents a single toast notification.
type Toast struct {
	Message     string
	Class       ToastClass
	RecoveryKey string
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

// ToastModel manages toast display state.
type ToastModel struct {
	active *Toast
	theme  theme.Theme
	width  int
}

// NewToastModel creates a new toast model.
func NewToastModel(th theme.Theme) ToastModel {
	return ToastModel{theme: th}
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

// ToastTickMsg is the internal tick message for auto-dismiss.
type ToastTickMsg struct{}

// Update handles tick messages for auto-dismiss.
func (m *ToastModel) Update(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(ToastTickMsg); ok {
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

// tick returns a Cmd that sends a ToastTickMsg after 1 second.
func (m *ToastModel) tick() tea.Cmd {
	return tea.Tick(time.Second, func(_ time.Time) tea.Msg {
		return ToastTickMsg{}
	})
}

// toastColor returns the theme color for a toast class.
// Matches mockup: .toast-ok=green, .toast-warn=red (error), .toast-undo=blue.
func (m *ToastModel) toastColor(class ToastClass) color.Color {
	if m.theme != nil {
		switch class {
		case ToastInfo:
			return m.theme.SemanticGreen()
		case ToastError:
			return m.theme.SemanticRed()
		case ToastUndo:
			return m.theme.SemanticBlue()
		}
	}
	// Fallback defaults.
	switch class {
	case ToastInfo:
		return lipgloss.Color("#73D990")
	case ToastError:
		return lipgloss.Color("#FF5F6D")
	case ToastUndo:
		return lipgloss.Color("#61AFEF")
	default:
		return lipgloss.Color("#6B7280")
	}
}

// toastIcon returns the leading icon for a toast class (from mockup).
func toastIcon(class ToastClass) string {
	switch class {
	case ToastInfo:
		return "✓"
	case ToastError:
		return "✗"
	case ToastUndo:
		return "↩"
	default:
		return "·"
	}
}

// View renders the toast notification. Returns empty string if no toast is active.
// Matches mockup: inline bar with icon + message + right-aligned key badge.
func (m *ToastModel) View() string {
	toast := m.Active()
	if toast == nil {
		return ""
	}

	toastFg := m.toastColor(toast.Class)

	iconStyle := lipgloss.NewStyle().Foreground(toastFg)
	msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#C8CCD4"))

	var content string
	icon := iconStyle.Render(toastIcon(toast.Class))

	if toast.Class == ToastUndo && toast.RecoveryKey != "" {
		remaining := toast.RemainingSeconds()
		content = icon + " " + msgStyle.Render(toast.Message) + " " +
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("#D4A050")).
				Background(lipgloss.Color("#1F2738")).
				Bold(true).
				Render(fmt.Sprintf("%s undo (%ds)", toast.RecoveryKey, remaining))
	} else {
		content = icon + " " + msgStyle.Render(toast.Message)
	}

	barStyle := lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width)

	return barStyle.Render(" " + content)
}
