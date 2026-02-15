package core

import (
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
)

func TestModeManager_InitialMode(t *testing.T) {
	mm := NewModeManager(ModeList)
	if mm.Current() != ModeList {
		t.Errorf("Current() = %s, want LIST", mm.Current())
	}
	if mm.Depth() != 1 {
		t.Errorf("Depth() = %d, want 1", mm.Depth())
	}
}

func TestModeManager_PushPop(t *testing.T) {
	mm := NewModeManager(ModeList)

	if err := mm.Push(ModePreview); err != nil {
		t.Fatalf("Push(PREVIEW) error: %v", err)
	}
	if mm.Current() != ModePreview {
		t.Errorf("after Push: Current() = %s", mm.Current())
	}
	if mm.Depth() != 2 {
		t.Errorf("Depth() = %d, want 2", mm.Depth())
	}

	if err := mm.Push(ModeDetail); err != nil {
		t.Fatalf("Push(DETAIL) error: %v", err)
	}

	got := mm.Pop()
	if got != ModePreview {
		t.Errorf("Pop() = %s, want PREVIEW", got)
	}

	got = mm.Pop()
	if got != ModeList {
		t.Errorf("Pop() = %s, want LIST", got)
	}

	// Pop at root is no-op.
	got = mm.Pop()
	if got != ModeList {
		t.Errorf("Pop() at root = %s, want LIST", got)
	}
}

func TestModeManager_InvalidTransition(t *testing.T) {
	mm := NewModeManager(ModeDetail)
	err := mm.Push(ModeExport)
	if err == nil {
		t.Error("Push(EXPORT) from DETAIL should fail")
	}
	if mm.Current() != ModeDetail {
		t.Errorf("Current() = %s, want DETAIL", mm.Current())
	}
}

func TestModeManager_HelpFromAnyMode(t *testing.T) {
	modes := []Mode{
		ModeList, ModePreview, ModeDetail, ModeSearch,
		ModeNewStash, ModeExport, ModeImport, ModeConflict,
	}
	for _, mode := range modes {
		t.Run(mode.String(), func(t *testing.T) {
			mm := NewModeManager(ModeList)
			if mode != ModeList {
				mm.Reset(mode)
			}
			if err := mm.Push(ModeHelp); err != nil {
				t.Errorf("Push(HELP) from %s should succeed: %v", mode, err)
			}
		})
	}
}

func TestModeManager_Reset(t *testing.T) {
	mm := NewModeManager(ModeList)
	mm.Push(ModePreview)
	mm.Push(ModeDetail)
	mm.Reset(ModeList)
	if mm.Current() != ModeList || mm.Depth() != 1 {
		t.Errorf("after Reset: Current=%s Depth=%d", mm.Current(), mm.Depth())
	}
}

func TestModeManager_History(t *testing.T) {
	mm := NewModeManager(ModeList)
	mm.Push(ModePreview)
	mm.Push(ModeHelp)
	history := mm.History()
	want := []Mode{ModeList, ModePreview, ModeHelp}
	if len(history) != len(want) {
		t.Fatalf("History length = %d, want %d", len(history), len(want))
	}
	for i, m := range want {
		if history[i] != m {
			t.Errorf("History[%d] = %s, want %s", i, history[i], m)
		}
	}
}

func TestModeManager_StackOverflow(t *testing.T) {
	mm := NewModeManager(ModeList)
	for mm.Depth() < maxModeStackDepth {
		if err := mm.Push(ModeHelp); err != nil {
			break
		}
	}
	err := mm.Push(ModeHelp)
	if err == nil && mm.Depth() > maxModeStackDepth {
		t.Error("should fail at max depth")
	}
}

func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		from plugin.Mode
		to   plugin.Mode
		want bool
	}{
		{ModeList, ModePreview, true},
		{ModeList, ModeDetail, true},
		{ModeList, ModeSearch, true},
		{ModeList, ModeNewStash, true},
		{ModeList, ModeExport, true},
		{ModeList, ModeImport, true},
		{ModeList, ModeConflict, true},
		{ModeList, ModeHelp, true},
		{ModePreview, ModeList, true},
		{ModePreview, ModeDetail, true},
		{ModePreview, ModeSearch, true},
		{ModePreview, ModeExport, false},
		{ModeDetail, ModeList, true},
		{ModeDetail, ModePreview, true},
		{ModeDetail, ModeHelp, true},
		{ModeDetail, ModeExport, false},
		{ModeSearch, ModeList, true},
		{ModeSearch, ModePreview, true},
		{ModeSearch, ModeDetail, false},
		{ModeNewStash, ModeList, true},
		{ModeNewStash, ModeDetail, false},
		{ModeExport, ModeList, true},
		{ModeImport, ModeList, true},
		{ModeConflict, ModeList, true},
		{ModeConflict, ModeDetail, true},
		{ModeConflict, ModePreview, false},
		{ModeHelp, ModeList, true},
		{ModeHelp, ModeDetail, false},
	}

	for _, tt := range tests {
		name := tt.from.String() + "->" + tt.to.String()
		t.Run(name, func(t *testing.T) {
			if got := IsValidTransition(tt.from, tt.to); got != tt.want {
				t.Errorf("IsValidTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
