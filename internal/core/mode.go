package core

import (
	"fmt"

	"github.com/indrasvat/nidhi/internal/plugin"
)

// Mode is an alias for plugin.Mode.
type Mode = plugin.Mode

const (
	ModeList     = plugin.ModeList
	ModePreview  = plugin.ModePreview
	ModeDetail   = plugin.ModeDetail
	ModeSearch   = plugin.ModeSearch
	ModeNewStash = plugin.ModeNewStash
	ModeExport   = plugin.ModeExport
	ModeImport   = plugin.ModeImport
	ModeConflict = plugin.ModeConflict
	ModeHelp     = plugin.ModeHelp
	ModePartial  = plugin.ModePartial
)

const maxModeStackDepth = 20

// ModeManager manages mode transitions with a push/pop stack.
type ModeManager struct {
	stack []Mode
}

// NewModeManager creates a ModeManager starting in the given initial mode.
func NewModeManager(initial Mode) *ModeManager {
	return &ModeManager{stack: []Mode{initial}}
}

// Current returns the current (top of stack) mode.
func (m *ModeManager) Current() Mode {
	if len(m.stack) == 0 {
		return ModeList
	}
	return m.stack[len(m.stack)-1]
}

// Push transitions to a new mode by pushing it onto the stack.
func (m *ModeManager) Push(mode Mode) error {
	if len(m.stack) >= maxModeStackDepth {
		return fmt.Errorf("mode stack overflow: depth %d", len(m.stack))
	}
	if !isValidTransition(m.Current(), mode) {
		return fmt.Errorf("invalid mode transition: %s -> %s", m.Current(), mode)
	}
	m.stack = append(m.stack, mode)
	return nil
}

// Pop returns to the previous mode. At root it's a no-op.
func (m *ModeManager) Pop() Mode {
	if len(m.stack) <= 1 {
		return m.Current()
	}
	m.stack = m.stack[:len(m.stack)-1]
	return m.Current()
}

// Reset clears the stack and sets the mode.
func (m *ModeManager) Reset(mode Mode) {
	m.stack = []Mode{mode}
}

// Depth returns the current stack depth.
func (m *ModeManager) Depth() int {
	return len(m.stack)
}

// History returns a copy of the mode stack.
func (m *ModeManager) History() []Mode {
	result := make([]Mode, len(m.stack))
	copy(result, m.stack)
	return result
}

func isValidTransition(from, to Mode) bool {
	if to == ModeHelp {
		return true
	}

	switch from {
	case ModeList:
		return to == ModePreview || to == ModeDetail || to == ModeSearch ||
			to == ModeNewStash || to == ModeExport || to == ModeImport ||
			to == ModeConflict || to == ModePartial
	case ModePreview:
		return to == ModeList || to == ModeDetail || to == ModeSearch
	case ModeDetail:
		return to == ModeList || to == ModePreview
	case ModeSearch:
		return to == ModeList || to == ModePreview
	case ModeNewStash:
		return to == ModeList || to == ModePartial
	case ModePartial:
		return to == ModeList
	case ModeExport:
		return to == ModeList
	case ModeImport:
		return to == ModeList
	case ModeConflict:
		return to == ModeList || to == ModeDetail
	case ModeHelp:
		return to == ModeList
	default:
		return false
	}
}

// IsValidTransition is the exported version for testing.
func IsValidTransition(from, to Mode) bool {
	return isValidTransition(from, to)
}
