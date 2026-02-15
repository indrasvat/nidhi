package core

import (
	"context"
	"io"
	"log/slog"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

type mockCache struct {
	stashes []plugin.Stash
}

func (m *mockCache) List(_ context.Context) ([]plugin.Stash, error)   { return m.stashes, nil }
func (m *mockCache) Diff(_ context.Context, _ string) (string, error) { return "", nil }
func (m *mockCache) Invalidate()                                      {}

type mockGit struct{}

func (m *mockGit) Run(_ context.Context, _ ...string) (string, error)        { return "", nil }
func (m *mockGit) RunLines(_ context.Context, _ ...string) ([]string, error) { return nil, nil }
func (m *mockGit) RunExitCode(_ context.Context, _ ...string) (string, int, error) {
	return "", 0, nil
}

type mockConfig struct{}

func (m *mockConfig) GetString(_ string) string { return "" }
func (m *mockConfig) GetInt(_ string) int       { return 0 }
func (m *mockConfig) GetBool(_ string) bool     { return false }

type mockTheme struct{}

func (m *mockTheme) Color(_ string) string { return "#000000" }

func newTestModel(stashes []plugin.Stash) *Model {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bus := NewBus()
	cache := &mockCache{stashes: stashes}
	gitVer := plugin.GitVersion{Major: 2, Minor: 53}

	pctx := plugin.PluginContext{
		Git: &mockGit{}, Cache: cache, Config: &mockConfig{},
		Events: bus, Logger: logger, GitVer: gitVer, Theme: &mockTheme{},
	}

	state := NewAppState("/test/repo", "main", gitVer)
	state = WithStashes(state, stashes)

	return New(state, pctx, bus, logger,
		plugin.NewRegistry[plugin.KeyHandler](),
		plugin.NewRegistry[plugin.ScreenProvider](),
		plugin.NewRegistry[plugin.StashHook](),
	)
}

func testStashes() []plugin.Stash {
	return []plugin.Stash{
		{Index: 0, SHA: "aaa", Message: "First"},
		{Index: 1, SHA: "bbb", Message: "Second"},
		{Index: 2, SHA: "ccc", Message: "Third"},
	}
}

func TestModel_InitialState(t *testing.T) {
	m := newTestModel(testStashes())
	state := m.State()
	if state.Mode != ModeList || state.Branch != "main" || len(state.Stashes) != 3 {
		t.Errorf("state = %+v", state)
	}
}

func TestModel_WindowSizeMsg(t *testing.T) {
	m := newTestModel(testStashes())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	model := updated.(*Model)
	if model.State().Width != 200 || model.State().Height != 60 {
		t.Errorf("dimensions = %dx%d", model.State().Width, model.State().Height)
	}
}

func TestModel_CursorNavigation(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	updated, _ := m.Update(tea.KeyPressMsg{Text: "j"})
	model := updated.(*Model)
	if model.State().Cursor != 1 {
		t.Errorf("after j: Cursor = %d", model.State().Cursor)
	}

	updated, _ = model.Update(tea.KeyPressMsg{Text: "j"})
	model = updated.(*Model)
	if model.State().Cursor != 2 {
		t.Errorf("after jj: Cursor = %d", model.State().Cursor)
	}

	// j at end should clamp
	updated, _ = model.Update(tea.KeyPressMsg{Text: "j"})
	model = updated.(*Model)
	if model.State().Cursor != 2 {
		t.Errorf("after jjj: Cursor = %d", model.State().Cursor)
	}

	updated, _ = model.Update(tea.KeyPressMsg{Text: "k"})
	model = updated.(*Model)
	if model.State().Cursor != 1 {
		t.Errorf("after k: Cursor = %d", model.State().Cursor)
	}
}

func TestModel_JumpToFirstLast(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true
	m.state = WithCursor(m.state, 1)

	updated, _ := m.Update(tea.KeyPressMsg{Text: "g"})
	model := updated.(*Model)
	if model.State().Cursor != 0 {
		t.Errorf("after g: Cursor = %d", model.State().Cursor)
	}

	updated, _ = model.Update(tea.KeyPressMsg{Text: "G"})
	model = updated.(*Model)
	if model.State().Cursor != 2 {
		t.Errorf("after G: Cursor = %d", model.State().Cursor)
	}
}

func TestModel_ModeTransitionViaKeys(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	// Tab -> PREVIEW
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	model := updated.(*Model)
	if model.State().Mode != ModePreview {
		t.Errorf("after Tab: Mode = %s", model.State().Mode)
	}

	// Esc -> LIST
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(*Model)
	if model.State().Mode != ModeList {
		t.Errorf("after Esc: Mode = %s", model.State().Mode)
	}

	// Enter -> DETAIL
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(*Model)
	if model.State().Mode != ModeDetail {
		t.Errorf("after Enter: Mode = %s", model.State().Mode)
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	updated, _ := m.Update(tea.KeyPressMsg{Text: "?"})
	model := updated.(*Model)
	if model.State().Mode != ModeHelp {
		t.Errorf("after ?: Mode = %s", model.State().Mode)
	}

	updated, _ = model.Update(tea.KeyPressMsg{Text: "?"})
	model = updated.(*Model)
	if model.State().Mode != ModeList {
		t.Errorf("after ?? : Mode = %s", model.State().Mode)
	}
}

func TestModel_ViewBeforeReady(t *testing.T) {
	m := newTestModel(testStashes())
	v := m.View()
	if !v.AltScreen {
		t.Error("should use AltScreen")
	}
}

func TestModel_ViewAfterReady(t *testing.T) {
	m := newTestModel(testStashes())
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := m.View()
	if !v.AltScreen {
		t.Error("should use AltScreen")
	}
}

func TestModel_StashesLoadedMsg(t *testing.T) {
	m := newTestModel(nil)
	m.ready = true
	updated, _ := m.Update(stashesLoadedMsg{stashes: []plugin.Stash{{Index: 0, SHA: "xxx"}}})
	model := updated.(*Model)
	if len(model.State().Stashes) != 1 || model.State().Stashes[0].SHA != "xxx" {
		t.Errorf("stashes = %v", model.State().Stashes)
	}
}

func TestModel_EscAtListIsNoop(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model := updated.(*Model)
	if model.State().Mode != ModeList {
		t.Errorf("Esc at LIST: Mode = %s", model.State().Mode)
	}
}

func TestModel_DeepModeStack(t *testing.T) {
	m := newTestModel(testStashes())
	m.ready = true

	m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if m.State().Mode != ModePreview {
		t.Fatalf("expected PREVIEW, got %s", m.State().Mode)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.State().Mode != ModeDetail {
		t.Fatalf("expected DETAIL, got %s", m.State().Mode)
	}

	m.Update(tea.KeyPressMsg{Text: "?"})
	if m.State().Mode != ModeHelp {
		t.Fatalf("expected HELP, got %s", m.State().Mode)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State().Mode != ModeDetail {
		t.Errorf("expected DETAIL after Esc from HELP, got %s", m.State().Mode)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State().Mode != ModePreview {
		t.Errorf("expected PREVIEW, got %s", m.State().Mode)
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.State().Mode != ModeList {
		t.Errorf("expected LIST, got %s", m.State().Mode)
	}
}
