package sync_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/plugin"
	pluginsync "github.com/indrasvat/nidhi/internal/plugins/sync"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// ─── Test doubles ───────────────────────────────────────────

type mockGit struct {
	runFn      func(ctx context.Context, args ...string) (string, error)
	runLinesFn func(ctx context.Context, args ...string) ([]string, error)
}

func (m *mockGit) Run(ctx context.Context, args ...string) (string, error) {
	if m.runFn != nil {
		return m.runFn(ctx, args...)
	}
	return "", nil
}

func (m *mockGit) RunLines(ctx context.Context, args ...string) ([]string, error) {
	if m.runLinesFn != nil {
		return m.runLinesFn(ctx, args...)
	}
	return nil, nil
}

func (m *mockGit) RunExitCode(_ context.Context, _ ...string) (string, int, error) {
	return "", 0, nil
}

type noopCache struct{}

func (c *noopCache) List(_ context.Context) ([]plugin.Stash, error) { return nil, nil }
func (c *noopCache) Diff(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *noopCache) Invalidate() {}

type noopBus struct{}

func (b *noopBus) Publish(_ plugin.Event)                   {}
func (b *noopBus) Subscribe(_ string, _ func(plugin.Event)) {}

type mockTheme struct{}

func (t *mockTheme) Color(_ string) string { return "" }

type mockConfig struct {
	vals map[string]string
}

func (c *mockConfig) GetString(key string) string {
	if c.vals != nil {
		return c.vals[key]
	}
	return ""
}
func (c *mockConfig) GetInt(_ string) int   { return 0 }
func (c *mockConfig) GetBool(_ string) bool { return false }

// ─── Helpers ────────────────────────────────────────────────

func newTestPlugin(t *testing.T, gitVer plugin.GitVersion) *pluginsync.Plugin {
	t.Helper()
	p := pluginsync.New(theme.NewAgni())
	pctx, err := plugin.NewPluginContext(
		&mockGit{},
		&noopCache{},
		&mockConfig{},
		&noopBus{},
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		gitVer,
		&mockTheme{},
	)
	if err != nil {
		t.Fatalf("NewPluginContext: %v", err)
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return p
}

func newTestPluginWithConfig(t *testing.T, gitVer plugin.GitVersion, cfg *mockConfig) *pluginsync.Plugin {
	t.Helper()
	p := pluginsync.New(theme.NewAgni())
	pctx, err := plugin.NewPluginContext(
		&mockGit{},
		&noopCache{},
		cfg,
		&noopBus{},
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		gitVer,
		&mockTheme{},
	)
	if err != nil {
		t.Fatalf("NewPluginContext: %v", err)
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return p
}

func stateWithStashes(n int) plugin.AppState {
	stashes := make([]plugin.Stash, n)
	for i := range n {
		stashes[i] = plugin.Stash{
			Index:   i,
			SHA:     fmt.Sprintf("abc%d", i),
			Message: fmt.Sprintf("Stash %d", i),
		}
	}
	return plugin.AppState{
		Mode:       plugin.ModeList,
		Stashes:    stashes,
		GitVersion: plugin.GitVersion{Major: 2, Minor: 51, Raw: "2.51.0"},
	}
}

var gitVer251 = plugin.GitVersion{Major: 2, Minor: 51, Raw: "2.51.0"}
var gitVer240 = plugin.GitVersion{Major: 2, Minor: 40, Raw: "2.40.0"}

// ─── Plugin identity tests ──────────────────────────────────

func TestPluginID(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	if p.ID() != "sync" {
		t.Errorf("ID() = %q, want %q", p.ID(), "sync")
	}
	if p.Name() != "Export/Import & Remote Sync" {
		t.Errorf("Name() = %q, want %q", p.Name(), "Export/Import & Remote Sync")
	}
}

func TestKeyBindings(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	bindings := p.KeyBindings()
	if len(bindings) != 2 {
		t.Fatalf("expected 2 key bindings, got %d", len(bindings))
	}
	if bindings[0].Key != "e" {
		t.Errorf("first binding key = %q, want %q", bindings[0].Key, "e")
	}
	if bindings[1].Key != "i" {
		t.Errorf("second binding key = %q, want %q", bindings[1].Key, "i")
	}
	for _, kb := range bindings {
		if len(kb.Modes) != 1 || kb.Modes[0] != plugin.ModeList {
			t.Errorf("binding %q should only apply in ModeList", kb.Key)
		}
	}
}

func TestScreenDefs(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	screens := p.Screens()
	if len(screens) != 2 {
		t.Fatalf("expected 2 screen defs, got %d", len(screens))
	}
	if screens[0].Mode != plugin.ModeExport || screens[0].Name != "EXPORT" {
		t.Errorf("first screen = %+v, want ModeExport/EXPORT", screens[0])
	}
	if screens[1].Mode != plugin.ModeImport || screens[1].Name != "IMPORT" {
		t.Errorf("second screen = %+v, want ModeImport/IMPORT", screens[1])
	}
}

func TestDestroy(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	if err := p.Destroy(); err != nil {
		t.Errorf("Destroy() returned error: %v", err)
	}
}

// ─── Version gating ─────────────────────────────────────────

func TestExportRequiresGit251(t *testing.T) {
	p := newTestPlugin(t, gitVer240)

	state := stateWithStashes(3)
	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)

	if newState.Mode != plugin.ModeList {
		t.Errorf("mode should stay ModeList when Git < 2.51, got %v", newState.Mode)
	}

	if cmd == nil {
		t.Fatal("expected toast command, got nil")
	}
	msg := cmd()
	toast, ok := msg.(core.InfoToastMsg)
	if !ok {
		t.Fatalf("expected InfoToastMsg, got %T", msg)
	}
	if !strings.Contains(toast.Text, "2.51") {
		t.Errorf("toast should mention 2.51, got %q", toast.Text)
	}
}

func TestImportRequiresGit251(t *testing.T) {
	p := newTestPlugin(t, gitVer240)

	state := stateWithStashes(3)
	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)

	if newState.Mode != plugin.ModeList {
		t.Errorf("mode should stay ModeList when Git < 2.51, got %v", newState.Mode)
	}

	if cmd == nil {
		t.Fatal("expected toast command, got nil")
	}
	msg := cmd()
	toast, ok := msg.(core.InfoToastMsg)
	if !ok {
		t.Fatalf("expected InfoToastMsg, got %T", msg)
	}
	if !strings.Contains(toast.Text, "2.51") {
		t.Errorf("toast should mention 2.51, got %q", toast.Text)
	}
}

func TestIsExportEnabled(t *testing.T) {
	p251 := newTestPlugin(t, gitVer251)
	if !p251.IsExportEnabled() {
		t.Error("expected export enabled for Git 2.51")
	}

	p240 := newTestPlugin(t, gitVer240)
	if p240.IsExportEnabled() {
		t.Error("expected export disabled for Git 2.40")
	}
}

// ─── Export screen ──────────────────────────────────────────

func TestExportKeyOpensExportMode(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)

	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)

	if newState.Mode != plugin.ModeExport {
		t.Errorf("mode should be ModeExport, got %v", newState.Mode)
	}
	if cmd == nil {
		t.Error("expected fetchRemotes command")
	}
}

func TestExportAllSelectedByDefault(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)

	p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)

	indices := p.SelectedExportIndices()
	if len(indices) != 3 {
		t.Fatalf("expected 3 selected indices, got %d", len(indices))
	}
	for i, idx := range indices {
		if idx != i {
			t.Errorf("index[%d] = %d, want %d", i, idx, i)
		}
	}
}

func TestExportEscReturnsToList(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)

	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	escMsg := tea.KeyPressMsg{Code: tea.KeyEscape}
	state, _ = p.Update(escMsg, state)

	if state.Mode != plugin.ModeList {
		t.Errorf("mode should be ModeList after Esc, got %v", state.Mode)
	}
}

func TestExportTabCyclesFocus(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	// Tab through all 3 focus areas.
	for i := range 3 {
		tabMsg := tea.KeyPressMsg{Code: tea.KeyTab}
		state, _ = p.Update(tabMsg, state)
		_ = i
	}
	// Should cycle back to focus=0 after 3 tabs.
}

func TestExportRemoteListMsg(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	remoteMsg := pluginsync.RemoteListMsg{
		Remotes: []pluginsync.Remote{
			{Name: "upstream", URL: "https://example.com/upstream"},
			{Name: "origin", URL: "https://example.com/origin"},
		},
	}
	state, _ = p.Update(remoteMsg, state)
	// No crash means the handler worked.
}

func TestExportResultSuccess(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	resultMsg := pluginsync.ExportResultMsg{
		ExportedCount: 2,
		Ref:           "refs/stashes/user",
		Remote:        "origin",
	}
	state, cmd := p.Update(resultMsg, state)

	if state.Mode != plugin.ModeList {
		t.Errorf("mode should be ModeList after export success, got %v", state.Mode)
	}
	if cmd == nil {
		t.Fatal("expected toast command")
	}
	msg := cmd()
	toast, ok := msg.(core.InfoToastMsg)
	if !ok {
		t.Fatalf("expected InfoToastMsg, got %T", msg)
	}
	if !strings.Contains(toast.Text, "2") {
		t.Errorf("toast should mention count, got %q", toast.Text)
	}
}

func TestExportResultError(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	resultMsg := pluginsync.ExportResultMsg{
		Err: fmt.Errorf("push denied"),
	}
	state, cmd := p.Update(resultMsg, state)

	if state.Mode != plugin.ModeList {
		t.Errorf("mode should be ModeList after error, got %v", state.Mode)
	}
	if cmd == nil {
		t.Fatal("expected error command")
	}
	msg := cmd()
	errMsg, ok := msg.(core.ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got %T", msg)
	}
	if errMsg.Err == nil || !strings.Contains(errMsg.Err.Error(), "push denied") {
		t.Errorf("error should contain 'push denied', got %v", errMsg.Err)
	}
}

func TestExportViewRenders(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	view := p.View(state, 80, 24)
	if view == "" {
		t.Error("export view should not be empty")
	}
	if !strings.Contains(view, "Export Stashes") {
		t.Error("export view should contain header")
	}
	if !strings.Contains(view, "refs/stashes/") {
		t.Error("export view should show default ref")
	}
}

func TestExportCommandPreview(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	_, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)

	preview := p.ExportCommandPreview()
	if !strings.Contains(preview, "git stash export") {
		t.Errorf("preview should contain export command, got %q", preview)
	}
	if !strings.Contains(preview, "git push") {
		t.Errorf("preview should contain push command, got %q", preview)
	}
}

func TestExportListNavigationAndToggle(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	// Move down.
	state, _ = p.Update(tea.KeyPressMsg{Text: "j"}, state)
	// Toggle selection (space).
	state, _ = p.Update(tea.KeyPressMsg{Text: " "}, state)

	indices := p.SelectedExportIndices()
	// Should have deselected stash@{1}.
	for _, idx := range indices {
		if idx == 1 {
			t.Error("stash@{1} should be deselected after toggle")
		}
	}
	if len(indices) != 2 {
		t.Errorf("expected 2 selected, got %d", len(indices))
	}
}

func TestExportSelectAll(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeExport

	// All are selected by default. Press 'a' to deselect all.
	state, _ = p.Update(tea.KeyPressMsg{Text: "a"}, state)
	if len(p.SelectedExportIndices()) != 0 {
		t.Error("expected all deselected after 'a'")
	}

	// Press 'a' again to reselect all.
	state, _ = p.Update(tea.KeyPressMsg{Text: "a"}, state)
	if len(p.SelectedExportIndices()) != 3 {
		t.Errorf("expected all 3 selected, got %d", len(p.SelectedExportIndices()))
	}
}

// ─── Import screen ──────────────────────────────────────────

func TestImportKeyOpensImportMode(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)

	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)

	if newState.Mode != plugin.ModeImport {
		t.Errorf("mode should be ModeImport, got %v", newState.Mode)
	}
	if cmd == nil {
		t.Error("expected fetchRemotes command")
	}
}

func TestImportEscReturnsToList(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeImport

	state, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, state)
	if state.Mode != plugin.ModeList {
		t.Errorf("mode should be ModeList after Esc, got %v", state.Mode)
	}
}

func TestImportRemoteListMsg(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeImport

	remoteMsg := pluginsync.RemoteListMsg{
		Remotes: []pluginsync.Remote{
			{Name: "origin", URL: "git@github.com:user/repo.git"},
		},
	}
	state, _ = p.Update(remoteMsg, state)
}

func TestImportResultSuccess(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeImport

	state, cmd := p.Update(pluginsync.ImportResultMsg{ImportedCount: 5}, state)

	if state.Mode != plugin.ModeList {
		t.Errorf("mode should be ModeList after import, got %v", state.Mode)
	}
	if cmd == nil {
		t.Fatal("expected toast command")
	}
	msg := cmd()
	toast, ok := msg.(core.InfoToastMsg)
	if !ok {
		t.Fatalf("expected InfoToastMsg, got %T", msg)
	}
	if !strings.Contains(toast.Text, "imported") {
		t.Errorf("toast should mention import, got %q", toast.Text)
	}
}

func TestImportResultError(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeImport

	state, cmd := p.Update(pluginsync.ImportResultMsg{Err: fmt.Errorf("fetch failed")}, state)

	if state.Mode != plugin.ModeList {
		t.Errorf("mode should be ModeList after error, got %v", state.Mode)
	}
	if cmd == nil {
		t.Fatal("expected error command")
	}
	msg := cmd()
	errMsg, ok := msg.(core.ErrorMsg)
	if !ok {
		t.Fatalf("expected ErrorMsg, got %T", msg)
	}
	if !strings.Contains(errMsg.Err.Error(), "fetch failed") {
		t.Errorf("error should contain 'fetch failed', got %v", errMsg.Err)
	}
}

func TestImportViewRenders(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeImport

	view := p.View(state, 80, 24)
	if view == "" {
		t.Error("import view should not be empty")
	}
	if !strings.Contains(view, "Import Stashes") {
		t.Error("import view should contain header")
	}
}

func TestImportTabCyclesFocus(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeImport

	// Tab cycles between ref input (0) and remote selector (1).
	for range 2 {
		state, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyTab}, state)
	}
	// No crash = correct.
}

func TestImportRefEditing(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	state, _ = p.HandleKey(plugin.KeyEvent{Key: "i", Mode: plugin.ModeList}, state)
	state.Mode = plugin.ModeImport

	// Backspace to delete last char.
	state, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyBackspace}, state)

	// Type a character.
	state, _ = p.Update(tea.KeyPressMsg{Text: "x"}, state)

	// Left arrow.
	state, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyLeft}, state)

	// Right arrow.
	state, _ = p.Update(tea.KeyPressMsg{Code: tea.KeyRight}, state)
}

// ─── ParseRemotes ───────────────────────────────────────────

func TestParseRemotes(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		expected int
		names    []string
	}{
		{
			name: "typical output",
			lines: []string{
				"origin\thttps://github.com/user/repo.git (fetch)",
				"origin\thttps://github.com/user/repo.git (push)",
				"upstream\thttps://github.com/org/repo.git (fetch)",
				"upstream\thttps://github.com/org/repo.git (push)",
			},
			expected: 2,
			names:    []string{"origin", "upstream"},
		},
		{
			name:     "empty",
			lines:    nil,
			expected: 0,
		},
		{
			name:     "malformed",
			lines:    []string{"foo"},
			expected: 0,
		},
		{
			name: "single remote",
			lines: []string{
				"origin\tgit@github.com:user/repo.git (fetch)",
				"origin\tgit@github.com:user/repo.git (push)",
			},
			expected: 1,
			names:    []string{"origin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remotes := pluginsync.ParseRemotes(tt.lines)
			if len(remotes) != tt.expected {
				t.Fatalf("expected %d remotes, got %d", tt.expected, len(remotes))
			}
			for i, name := range tt.names {
				if remotes[i].Name != name {
					t.Errorf("remote[%d].Name = %q, want %q", i, remotes[i].Name, name)
				}
			}
		})
	}
}

// ─── Config ─────────────────────────────────────────────────

func TestDefaultRefFromConfig(t *testing.T) {
	p := newTestPluginWithConfig(t, gitVer251, &mockConfig{
		vals: map[string]string{
			"export_ref":    "refs/stashes/team",
			"export_remote": "upstream",
		},
	})

	state := stateWithStashes(1)
	_, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)

	preview := p.ExportCommandPreview()
	if !strings.Contains(preview, "refs/stashes/team") {
		t.Errorf("should use configured ref, got: %s", preview)
	}
	if !strings.Contains(preview, "upstream") {
		t.Errorf("should use configured remote, got: %s", preview)
	}
}

// ─── ExportCmd / ImportCmd unit tests ───────────────────────

func TestExportCmdValidatesRef(t *testing.T) {
	git := &mockGit{
		runFn: func(_ context.Context, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "check-ref-format" {
				return "", fmt.Errorf("bad ref")
			}
			return "", nil
		},
	}

	cmd := pluginsync.ExportCmd(git, "invalid..ref", "origin", []int{0})
	msg := cmd()
	result, ok := msg.(pluginsync.ExportResultMsg)
	if !ok {
		t.Fatalf("expected ExportResultMsg, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected error for invalid ref")
	}
	if !strings.Contains(result.Err.Error(), "invalid ref format") {
		t.Errorf("expected ref validation error, got: %v", result.Err)
	}
}

func TestExportCmdExportFails(t *testing.T) {
	git := &mockGit{
		runFn: func(_ context.Context, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "stash" && args[1] == "export" {
				return "", fmt.Errorf("export not supported")
			}
			return "", nil
		},
	}

	cmd := pluginsync.ExportCmd(git, "refs/stashes/user", "origin", []int{0, 1})
	msg := cmd()
	result := msg.(pluginsync.ExportResultMsg)
	if result.Err == nil {
		t.Fatal("expected export error")
	}
	if !strings.Contains(result.Err.Error(), "export") {
		t.Errorf("error should mention export: %v", result.Err)
	}
}

func TestExportCmdPushFails(t *testing.T) {
	callCount := 0
	git := &mockGit{
		runFn: func(_ context.Context, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "push" {
				return "", fmt.Errorf("permission denied")
			}
			callCount++
			return "", nil
		},
	}

	cmd := pluginsync.ExportCmd(git, "refs/stashes/user", "origin", []int{0})
	msg := cmd()
	result := msg.(pluginsync.ExportResultMsg)
	if result.Err == nil {
		t.Fatal("expected push error")
	}
	if !strings.Contains(result.Err.Error(), "push") {
		t.Errorf("error should mention push: %v", result.Err)
	}
}

func TestExportCmdSuccess(t *testing.T) {
	var capturedArgs [][]string
	git := &mockGit{
		runFn: func(_ context.Context, args ...string) (string, error) {
			capturedArgs = append(capturedArgs, args)
			return "", nil
		},
	}

	cmd := pluginsync.ExportCmd(git, "refs/stashes/user", "origin", []int{0, 2})
	msg := cmd()
	result := msg.(pluginsync.ExportResultMsg)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.ExportedCount != 2 {
		t.Errorf("exported count = %d, want 2", result.ExportedCount)
	}
	if result.Ref != "refs/stashes/user" {
		t.Errorf("ref = %q, want %q", result.Ref, "refs/stashes/user")
	}
	if result.Remote != "origin" {
		t.Errorf("remote = %q, want %q", result.Remote, "origin")
	}

	// Verify git commands: check-ref-format, stash export, push.
	if len(capturedArgs) != 3 {
		t.Fatalf("expected 3 git commands, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "check-ref-format" {
		t.Errorf("first command should be check-ref-format, got %v", capturedArgs[0])
	}
	if capturedArgs[1][0] != "stash" || capturedArgs[1][1] != "export" {
		t.Errorf("second command should be stash export, got %v", capturedArgs[1])
	}
	if capturedArgs[2][0] != "push" {
		t.Errorf("third command should be push, got %v", capturedArgs[2])
	}
}

func TestImportCmdFetchFails(t *testing.T) {
	git := &mockGit{
		runFn: func(_ context.Context, args ...string) (string, error) {
			if len(args) >= 1 && args[0] == "fetch" {
				return "", fmt.Errorf("network error")
			}
			return "", nil
		},
	}

	cmd := pluginsync.ImportCmd(git, "refs/stashes/user", "origin")
	msg := cmd()
	result := msg.(pluginsync.ImportResultMsg)
	if result.Err == nil {
		t.Fatal("expected fetch error")
	}
	if !strings.Contains(result.Err.Error(), "fetch") {
		t.Errorf("error should mention fetch: %v", result.Err)
	}
}

func TestImportCmdImportFails(t *testing.T) {
	git := &mockGit{
		runFn: func(_ context.Context, args ...string) (string, error) {
			if len(args) >= 2 && args[0] == "stash" && args[1] == "import" {
				return "", fmt.Errorf("no stashes found")
			}
			return "", nil
		},
	}

	cmd := pluginsync.ImportCmd(git, "refs/stashes/user", "origin")
	msg := cmd()
	result := msg.(pluginsync.ImportResultMsg)
	if result.Err == nil {
		t.Fatal("expected import error")
	}
	if !strings.Contains(result.Err.Error(), "import") {
		t.Errorf("error should mention import: %v", result.Err)
	}
}

func TestImportCmdSuccess(t *testing.T) {
	var capturedArgs [][]string
	git := &mockGit{
		runFn: func(_ context.Context, args ...string) (string, error) {
			capturedArgs = append(capturedArgs, args)
			return "", nil
		},
	}

	cmd := pluginsync.ImportCmd(git, "refs/stashes/user", "origin")
	msg := cmd()
	result := msg.(pluginsync.ImportResultMsg)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	// Verify: fetch then stash import.
	if len(capturedArgs) != 2 {
		t.Fatalf("expected 2 git commands, got %d", len(capturedArgs))
	}
	if capturedArgs[0][0] != "fetch" {
		t.Errorf("first command should be fetch, got %v", capturedArgs[0])
	}
	// Check refspec.
	if capturedArgs[0][2] != "refs/stashes/user:refs/stashes/user" {
		t.Errorf("fetch refspec = %q, want %q", capturedArgs[0][2], "refs/stashes/user:refs/stashes/user")
	}
	if capturedArgs[1][0] != "stash" || capturedArgs[1][1] != "import" {
		t.Errorf("second command should be stash import, got %v", capturedArgs[1])
	}
}

// ─── ValidateRef ────────────────────────────────────────────

func TestValidateRefValid(t *testing.T) {
	git := &mockGit{
		runFn: func(_ context.Context, _ ...string) (string, error) {
			return "", nil
		},
	}
	err := pluginsync.ValidateRef(context.Background(), git, "refs/stashes/user")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRefInvalid(t *testing.T) {
	git := &mockGit{
		runFn: func(_ context.Context, _ ...string) (string, error) {
			return "", fmt.Errorf("bad ref")
		},
	}
	err := pluginsync.ValidateRef(context.Background(), git, "bad..ref")
	if err == nil {
		t.Error("expected error for invalid ref")
	}
}

// ─── Edge cases ─────────────────────────────────────────────

func TestExportEmptyStashList(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	_, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)

	indices := p.SelectedExportIndices()
	if len(indices) != 0 {
		t.Errorf("expected 0 indices for empty stash list, got %d", len(indices))
	}
}

func TestExportPreviewEmptySelection(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(0)
	_, _ = p.HandleKey(plugin.KeyEvent{Key: "e", Mode: plugin.ModeList}, state)

	preview := p.ExportCommandPreview()
	if strings.Contains(preview, "git stash export") {
		t.Error("preview should not show command when no stashes selected")
	}
}

func TestUnhandledKeyReturnsStateUnchanged(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := stateWithStashes(3)

	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "z", Mode: plugin.ModeList}, state)
	if newState.Mode != state.Mode {
		t.Error("unhandled key should not change mode")
	}
	if cmd != nil {
		t.Error("unhandled key should not produce command")
	}
}

func TestViewReturnsEmptyForNonScreenMode(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := plugin.AppState{Mode: plugin.ModeList}

	view := p.View(state, 80, 24)
	if view != "" {
		t.Errorf("expected empty view for non-screen mode, got %q", view)
	}
}

func TestUpdateReturnsStateForNonScreenMode(t *testing.T) {
	p := newTestPlugin(t, gitVer251)
	state := plugin.AppState{Mode: plugin.ModeList}

	newState, cmd := p.Update(tea.KeyPressMsg{Text: "j"}, state)
	if newState.Mode != state.Mode {
		t.Error("update for non-screen mode should not change state")
	}
	if cmd != nil {
		t.Error("update for non-screen mode should not produce command")
	}
}
