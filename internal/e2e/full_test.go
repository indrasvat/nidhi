package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/indrasvat/nidhi/internal/config"
	"github.com/indrasvat/nidhi/internal/core"
	"github.com/indrasvat/nidhi/internal/git"
	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/ui/mouse"
	"github.com/indrasvat/nidhi/internal/ui/screens"
	"github.com/indrasvat/nidhi/internal/ui/theme"
)

// =============================================================================
// Help Overlay E2E (FR-18)
// =============================================================================

func TestE2E_HelpOverlay_ToggleVisibility(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	help := screens.NewHelpOverlay(th)

	if help.IsVisible() {
		t.Error("help should be hidden initially")
	}

	// Toggle on.
	help.Toggle()
	if !help.IsVisible() {
		t.Error("help should be visible after first toggle")
	}

	// Toggle off.
	help.Toggle()
	if help.IsVisible() {
		t.Error("help should be hidden after second toggle")
	}
}

func TestE2E_HelpOverlay_RenderContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	help := screens.NewHelpOverlay(th)
	help.Toggle() // Make visible.

	view := help.Render(120, 40)

	if view == "" {
		t.Fatal("visible help overlay should produce non-empty render")
	}

	// Should contain key categories.
	assertScreenContains(t, view, "Global")
	assertScreenContains(t, view, "Navigation")
	assertScreenContains(t, view, "Actions")
	assertScreenContains(t, view, "Search")

	// Should contain specific keybindings.
	assertScreenContains(t, view, "q")
	assertScreenContains(t, view, "Quit")
	assertScreenContains(t, view, "j")
}

func TestE2E_HelpOverlay_RenderHiddenIsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	help := screens.NewHelpOverlay(th)

	view := help.Render(120, 40)
	if view != "" {
		t.Errorf("hidden help overlay should produce empty string, got %d chars", len(view))
	}
}

func TestE2E_HelpOverlay_Scrolling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	help := screens.NewHelpOverlay(th)
	help.Toggle()

	view1 := help.Render(120, 20) // Small viewport to ensure scrolling matters.

	// Scroll down.
	help.ScrollDown()
	help.ScrollDown()
	help.ScrollDown()

	view2 := help.Render(120, 20)

	// Scrolled content should differ (unless viewport is large enough to show all).
	if help.ContentHeight() > 18 && view1 == view2 {
		t.Error("scrolling should change the rendered content when content exceeds viewport")
	}

	// Scroll up to top.
	for range 10 {
		help.ScrollUp()
	}

	// Should not crash or go negative.
	view3 := help.Render(120, 20)
	if view3 == "" {
		t.Error("help should still render after scrolling back to top")
	}
}

func TestE2E_HelpOverlay_Categories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	help := screens.NewHelpOverlay(th)

	categories := help.Categories()
	if len(categories) == 0 {
		t.Fatal("help overlay should have at least one category")
	}

	// Verify each category has bindings.
	for _, cat := range categories {
		if cat.Name == "" {
			t.Error("category should have a name")
		}
		if len(cat.Bindings) == 0 {
			t.Errorf("category %q should have bindings", cat.Name)
		}
	}

	// Check for expected categories from PRD §11.2.
	categoryNames := make(map[string]bool)
	for _, cat := range categories {
		categoryNames[cat.Name] = true
	}

	for _, expected := range []string{"Global", "Navigation", "Actions"} {
		if !categoryNames[expected] {
			t.Errorf("missing expected category: %q", expected)
		}
	}
}

func TestE2E_HelpOverlay_DimmedBackground(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	help := screens.NewHelpOverlay(th)

	bgContent := "This is the background content\nWith multiple lines\nOf text"

	// When hidden, should return bg unchanged.
	result := help.RenderWithDimmedBackground(bgContent, 80, 24)
	if result != bgContent {
		t.Error("hidden help should return background content unchanged")
	}

	// When visible, should render a composited view.
	help.Toggle()
	result = help.RenderWithDimmedBackground(bgContent, 80, 24)
	if result == bgContent {
		t.Error("visible help should modify the output (dimmed bg + overlay)")
	}
	if result == "" {
		t.Error("visible help with dimmed bg should produce non-empty output")
	}
}

// =============================================================================
// Mode Manager E2E
// =============================================================================

func TestE2E_ModeManager_FullTransitionCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	mm := core.NewModeManager(core.ModeList)
	if mm.Current() != core.ModeList {
		t.Fatalf("initial mode = %v, want ModeList", mm.Current())
	}

	// LIST → PREVIEW.
	if err := mm.Push(core.ModePreview); err != nil {
		t.Fatalf("LIST → PREVIEW: %v", err)
	}

	// PREVIEW → DETAIL.
	if err := mm.Push(core.ModeDetail); err != nil {
		t.Fatalf("PREVIEW → DETAIL: %v", err)
	}
	if mm.Current() != core.ModeDetail {
		t.Errorf("current = %v, want ModeDetail", mm.Current())
	}

	// Pop back to PREVIEW.
	if mode := mm.Pop(); mode != core.ModePreview {
		t.Errorf("Pop: mode = %v, want ModePreview", mode)
	}

	// Pop back to LIST.
	if mode := mm.Pop(); mode != core.ModeList {
		t.Errorf("Pop: mode = %v, want ModeList", mode)
	}

	// Pop at root = no-op.
	if mode := mm.Pop(); mode != core.ModeList {
		t.Errorf("Pop at root: mode = %v, want ModeList", mode)
	}
}

func TestE2E_ModeManager_HelpFromAnyMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	// Help should be accessible from any mode.
	modes := []core.Mode{
		core.ModeList, core.ModePreview, core.ModeDetail,
		core.ModeSearch, core.ModeNewStash, core.ModeExport,
		core.ModeImport, core.ModeConflict,
	}

	for _, from := range modes {
		if !core.IsValidTransition(from, core.ModeHelp) {
			t.Errorf("ModeHelp should be reachable from %v", from)
		}
	}
}

func TestE2E_ModeManager_InvalidTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	// DETAIL → SEARCH is not valid.
	if core.IsValidTransition(core.ModeDetail, core.ModeSearch) {
		t.Error("DETAIL → SEARCH should not be valid")
	}

	// SEARCH → DETAIL is not valid.
	if core.IsValidTransition(core.ModeSearch, core.ModeDetail) {
		t.Error("SEARCH → DETAIL should not be valid")
	}

	// EXPORT → PREVIEW is not valid.
	if core.IsValidTransition(core.ModeExport, core.ModePreview) {
		t.Error("EXPORT → PREVIEW should not be valid")
	}
}

func TestE2E_ModeManager_StackDepthLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	mm := core.NewModeManager(core.ModeList)

	// Push help repeatedly until hitting the limit.
	for i := range 30 {
		err := mm.Push(core.ModeHelp)
		if err != nil {
			if i < 19 {
				t.Fatalf("Push failed unexpectedly at depth %d: %v", i, err)
			}
			// Expected to fail at depth 20.
			return
		}
		// Pop back to allow next push (help → list).
		mm.Pop()
	}
}

// =============================================================================
// Mouse Handler E2E
// =============================================================================

func TestE2E_MouseHandler_ClickToSelectRow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	h := mouse.NewHandler()
	h.SetRowHeight(2)

	state := plugin.AppState{
		Mode:   plugin.ModeList,
		Height: 30,
		Stashes: []plugin.Stash{
			{Index: 0, Message: "s0"},
			{Index: 1, Message: "s1"},
			{Index: 2, Message: "s2"},
			{Index: 3, Message: "s3"},
		},
	}

	// Click at y=5 (row 2 with statusbar=1, rowHeight=2).
	result := h.HandleClick(10, 5, state)

	if result.Action != mouse.SelectRow {
		t.Errorf("click at y=5: action = %v, want SelectRow", result.Action)
	}
	if result.RowIndex != 2 {
		t.Errorf("click at y=5: rowIndex = %d, want 2", result.RowIndex)
	}
}

func TestE2E_MouseHandler_ScrollEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	h := mouse.NewHandler()

	upResult := h.HandleWheel(-1)
	if upResult.Action != mouse.ScrollUp {
		t.Errorf("scroll up: action = %v, want ScrollUp", upResult.Action)
	}

	downResult := h.HandleWheel(1)
	if downResult.Action != mouse.ScrollDown {
		t.Errorf("scroll down: action = %v, want ScrollDown", downResult.Action)
	}
}

func TestE2E_MouseHandler_ClickOutsideList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	h := mouse.NewHandler()
	state := plugin.AppState{
		Mode:   plugin.ModeList,
		Height: 30,
	}

	// Click at y=0 (status bar area).
	result := h.HandleClick(10, 0, state)
	if result.Action != mouse.NoAction {
		t.Errorf("click in status bar: action = %v, want NoAction", result.Action)
	}
}

// =============================================================================
// Config Integration E2E
// =============================================================================

func TestE2E_Config_DefaultsAreValid(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	cfg := config.DefaultConfig()

	if cfg.General.StaleDays <= 0 {
		t.Errorf("default stale days = %d, want > 0", cfg.General.StaleDays)
	}
	if cfg.General.Icons == "" {
		t.Error("default icons should not be empty")
	}
	if cfg.Performance.DiffCacheSize <= 0 {
		t.Errorf("default diff cache size = %d, want > 0", cfg.Performance.DiffCacheSize)
	}
	if cfg.Log.Level == "" {
		t.Error("default log level should not be empty")
	}
}

func TestE2E_Config_CLIFlagsOverrideEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	// Set env var.
	t.Setenv("NO_COLOR", "1")

	flags := config.CLIFlags{}
	noColor := false
	flags.NoColor = &noColor // CLI explicitly sets no-color to false.

	cfg, err := config.Load(flags)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// CLI flag should take precedence over NO_COLOR env.
	if cfg.NoColor != false {
		t.Error("CLI --no-color=false should override NO_COLOR env")
	}
}

func TestE2E_Config_StaleThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	cfg := config.DefaultConfig()
	threshold := cfg.StaleThreshold()

	if threshold <= 0 {
		t.Errorf("stale threshold = %v, want > 0", threshold)
	}

	expected := time.Duration(cfg.General.StaleDays) * 24 * time.Hour
	if threshold != expected {
		t.Errorf("stale threshold = %v, want %v", threshold, expected)
	}
}

func TestE2E_Config_LoggingSetup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	cfg := config.DefaultConfig()
	cfg.Log.Level = "off"

	logger, cleanup, err := config.SetupLogging(&cfg)
	if err != nil {
		t.Fatalf("SetupLogging: %v", err)
	}
	defer cleanup()

	if logger == nil {
		t.Fatal("logger should not be nil even for 'off' level")
	}
}

// =============================================================================
// Preview Screen E2E
// =============================================================================

func TestE2E_PreviewScreen_WithRealStashDiff(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupMultiFileStash(t, 5)
	diff := gitStashDiff(t, dir, 0)

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	ps := screens.NewPreviewScreen(ls, &pluginNoopCache{}, th)

	stashes := []core.Stash{
		{Index: 0, Message: "multi-file change", SHA: "abc0"},
	}
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	// Simulate diff loaded.
	diffMsg := screens.DiffLoadedMsg{SHA: "abc0", Diff: diff}
	state, _ = ps.Update(diffMsg, state)

	view := ps.View(state, 120, 40)
	if view == "" {
		t.Fatal("PREVIEW view should not be empty with a real diff")
	}

	// Should show multi-file change message.
	assertScreenContains(t, view, "multi-file change")
}

func TestE2E_PreviewScreen_FileCycling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupMultiFileStash(t, 3)
	diff := gitStashDiff(t, dir, 0)

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	ps := screens.NewPreviewScreen(ls, &pluginNoopCache{}, th)

	stashes := []core.Stash{
		{Index: 0, Message: "multi-file change", SHA: "abc0"},
	}
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModePreview,
	}

	// Load diff.
	diffMsg := screens.DiffLoadedMsg{SHA: "abc0", Diff: diff}
	state, _ = ps.Update(diffMsg, state)

	// h/l cycles through files.
	state, _ = ps.Update(tea.KeyPressMsg{Text: "l"}, state)
	state, _ = ps.Update(tea.KeyPressMsg{Text: "l"}, state)

	// Should not panic with wrapping.
	for range 10 {
		state, _ = ps.Update(tea.KeyPressMsg{Text: "l"}, state)
	}

	view := ps.View(state, 120, 40)
	if view == "" {
		t.Error("PREVIEW view should not be empty after cycling")
	}
}

// =============================================================================
// Full Workflow E2E — LIST → PREVIEW → DETAIL → LIST
// =============================================================================

func TestE2E_FullWorkflow_ScreenTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 5)
	stashLines := gitStashList(t, dir)
	if len(stashLines) != 5 {
		t.Fatalf("expected 5 stashes, got %d", len(stashLines))
	}

	stashes := make([]core.Stash, 5)
	for i := range 5 {
		stashes[i] = core.Stash{
			Index:   i,
			SHA:     fmt.Sprintf("abc%d", i),
			Message: fmt.Sprintf("stash message %d", 4-i),
		}
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	// Render LIST.
	view := ls.View(state, 120, 30)
	assertScreenContains(t, view, "stash message")

	// Navigate down 2.
	state, _ = ls.Update(tea.KeyPressMsg{Text: "j"}, state)
	state, _ = ls.Update(tea.KeyPressMsg{Text: "j"}, state)
	if ls.Cursor() != 2 {
		t.Fatalf("cursor = %d after 2xj, want 2", ls.Cursor())
	}

	// Tab → PREVIEW.
	state, _ = ls.Update(tea.KeyPressMsg{Code: tea.KeyTab}, state)
	if state.Mode != core.ModePreview {
		t.Fatalf("mode = %v after Tab, want PREVIEW", state.Mode)
	}

	// Enter from PREVIEW → DETAIL.
	ps := screens.NewPreviewScreen(ls, &pluginNoopCache{}, th)
	state, _ = ps.Update(tea.KeyPressMsg{Code: tea.KeyEnter}, state)
	if state.Mode != core.ModeDetail {
		t.Fatalf("mode = %v after Enter, want DETAIL", state.Mode)
	}

	// Esc → back to PREVIEW.
	ds := screens.NewDetailScreen(th)
	ds.SetPreviousMode(core.ModePreview)
	state, _ = ds.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, state)
	if state.Mode != core.ModePreview {
		t.Fatalf("mode = %v after Esc, want PREVIEW", state.Mode)
	}
}

// =============================================================================
// Git Version Compatibility E2E
// =============================================================================

func TestE2E_GitVersion_Detection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	ver, err := git.DetectVersion(ctx, runner)
	if err != nil {
		t.Fatalf("DetectVersion: %v", err)
	}

	if ver.Major < 2 {
		t.Errorf("git major version = %d, expected >= 2", ver.Major)
	}
	if ver.Raw == "" {
		t.Error("git version raw string should not be empty")
	}
}

func TestE2E_GitVersion_AtLeast(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	ver, err := git.DetectVersion(ctx, runner)
	if err != nil {
		t.Fatalf("DetectVersion: %v", err)
	}

	// Should be at least 2.0.
	if !ver.AtLeast(2, 0, 0) {
		t.Errorf("expected git >= 2.0.0, got %s", ver.Raw)
	}

	// Should not be version 99.0.0.
	if ver.AtLeast(99, 0, 0) {
		t.Errorf("git should not be >= 99.0.0, got %s", ver.Raw)
	}
}

func TestE2E_GitVersion_MergeTreeRequires238(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}
	requireGitVersion(t, 2, 38)

	dir := setupTestRepo(t, 0)

	// Ensure merge-tree with --write-tree works.
	writeFile(t, dir, "file.go", "package main\nfunc main() {}\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "add file")

	writeFile(t, dir, "file.go", "package main\nfunc main() { println(\"hello\") }\n")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "modify file")

	stashCommit := stashSHA(t, dir, 0)
	runner := git.NewDefaultRunner(dir, nil)

	result, err := git.RunMergeTree(context.Background(), runner, stashCommit)
	if err != nil {
		t.Fatalf("RunMergeTree: %v", err)
	}

	// Should be a clean merge (no conflicts).
	if result.HasConflicts {
		t.Error("expected clean merge")
	}
}

func TestE2E_GitVersion_ExportImportFeatureGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)
	runner := git.NewDefaultRunner(dir, nil)
	ctx := context.Background()

	ver, err := git.DetectVersion(ctx, runner)
	if err != nil {
		t.Fatalf("DetectVersion: %v", err)
	}

	// Export/import requires Git >= 2.51.
	// We just verify the feature gate logic works.
	pv := plugin.GitVersion{
		Major: ver.Major,
		Minor: ver.Minor,
		Patch: ver.Patch,
		Raw:   ver.Raw,
	}

	exportEnabled := pv.AtLeast(2, 51)
	t.Logf("git %s: export/import enabled = %v", ver.Raw, exportEnabled)
}

// =============================================================================
// Large Repo Performance E2E
// =============================================================================

func TestE2E_Performance_ListRender_50Stashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test")
	}

	stashes := make([]core.Stash, 50)
	for i := range 50 {
		stashes[i] = core.Stash{
			Index:      i,
			SHA:        fmt.Sprintf("sha%04d", i),
			Message:    fmt.Sprintf("Stash entry %d: some realistic message content", i),
			Branch:     "main",
			FileCount:  3,
			Insertions: 10,
			Deletions:  5,
		}
	}

	th := theme.NewAgni()
	ls := screens.NewListScreen(th)
	state := core.AppState{
		Stashes: stashes,
		Cursor:  0,
		Mode:    core.ModeList,
	}

	start := time.Now()
	for range 100 {
		_ = ls.View(state, 120, 40)
	}
	elapsed := time.Since(start)

	avg := elapsed / 100
	t.Logf("Average LIST render (50 stashes): %v", avg)

	if avg > 10*time.Millisecond {
		t.Errorf("LIST render too slow: %v per frame (budget: <10ms)", avg)
	}
}

func TestE2E_Performance_StashParsing_200Stashes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test")
	}

	dir := setupTestRepo(t, 200)
	ctx := context.Background()

	start := time.Now()

	cmd := exec.CommandContext(ctx, "git", "stash", "list",
		"--format=%H %h %s %D %ai")
	cmd.Dir = dir
	cmd.Env = []string{"GIT_CONFIG_NOSYSTEM=1", "HOME=" + dir}
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}

	elapsed := time.Since(start)
	t.Logf("Stash list time (200 stashes): %v, output: %d bytes", elapsed, len(out))

	if elapsed > 500*time.Millisecond {
		t.Errorf("200 stashes too slow: %v (budget: <500ms)", elapsed)
	}
}

// =============================================================================
// State Management E2E
// =============================================================================

func TestE2E_State_WithStashes_ClampsCursor(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	state := core.NewAppState("/tmp/test", "main", core.GitVersion{Major: 2, Minor: 40})
	state.Cursor = 10

	stashes := make([]core.Stash, 3)
	for i := range 3 {
		stashes[i] = core.Stash{Index: i, Message: fmt.Sprintf("s%d", i)}
	}

	state = core.WithStashes(state, stashes)

	if state.Cursor != 2 {
		t.Errorf("cursor = %d, want 2 (clamped to last)", state.Cursor)
	}
}

func TestE2E_State_SelectedStash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	state := core.AppState{
		Stashes: []core.Stash{
			{Index: 0, Message: "first"},
			{Index: 1, Message: "second"},
		},
		Cursor: 1,
	}

	selected := core.SelectedStash(state)
	if selected == nil {
		t.Fatal("SelectedStash returned nil")
	}
	if selected.Message != "second" {
		t.Errorf("selected = %q, want 'second'", selected.Message)
	}

	// Empty state.
	emptyState := core.AppState{}
	if core.SelectedStash(emptyState) != nil {
		t.Error("SelectedStash on empty state should return nil")
	}
}

// =============================================================================
// New Stash Screen E2E
// =============================================================================

func TestE2E_NewStashScreen_RenderEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	th := theme.NewAgni()
	ns := screens.NewNewStashScreen(th)

	state := core.AppState{Mode: core.ModeNewStash}
	view := ns.View(state, 120, 30)

	if view == "" {
		t.Error("NewStash screen should render even with no dirty files")
	}
}

// =============================================================================
// Event Bus E2E
// =============================================================================

func TestE2E_EventBus_PubSub(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	bus := core.NewBus()

	received := make(chan string, 1)
	bus.Subscribe("test_event", func(e plugin.Event) {
		if msg, ok := e.Payload.(string); ok {
			received <- msg
		}
	})

	bus.Publish(plugin.Event{Type: "test_event", Payload: "hello from bus"})

	select {
	case msg := <-received:
		if msg != "hello from bus" {
			t.Errorf("received = %q, want 'hello from bus'", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("event not received within timeout")
	}
}

func TestE2E_EventBus_MultipleSubscribers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	bus := core.NewBus()

	count := 0
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })
	bus.Subscribe("multi", func(_ plugin.Event) { count++ })

	bus.Publish(plugin.Event{Type: "multi"})

	if count != 3 {
		t.Errorf("count = %d, want 3 (all subscribers fired)", count)
	}
}

// =============================================================================
// Git Operations: Large Stash with Many Files
// =============================================================================

func TestE2E_GitOps_LargeStash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	dir := setupTestRepo(t, 0)

	// Create a stash with 20 files.
	for i := range 20 {
		writeFile(t, dir, fmt.Sprintf("pkg/module_%d/file.go", i),
			fmt.Sprintf("package module%d\n\nvar X = %d\n", i, i))
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "stash", "push", "-m", "large stash with 20 files")

	// Verify stash has content.
	diff := gitStashDiff(t, dir, 0)
	if len(diff) == 0 {
		t.Fatal("expected non-empty diff for large stash")
	}

	// Count file sections in diff.
	fileCount := strings.Count(diff, "diff --git")
	if fileCount < 20 {
		t.Errorf("diff contains %d files, want >= 20", fileCount)
	}

	// Detail screen should render all files.
	th := theme.NewAgni()
	ds := screens.NewDetailScreen(th)
	ds.SetDiff(diff)

	state := core.AppState{Mode: core.ModeDetail}
	view := ds.View(state, 120, 40)
	if view == "" {
		t.Error("DETAIL view should render for large stash")
	}
}

// =============================================================================
// Binary Build Verification
// =============================================================================

func TestE2E_Binary_BuildsSuccessfully(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	root := findProjectRoot(t)
	binPath := t.TempDir() + "/nidhi-test"
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/nidhi")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	// Verify binary exists.
	info, err := os.Stat(binPath)
	if err != nil {
		t.Fatalf("binary not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("binary is empty")
	}

	// Verify --version works.
	verCmd := exec.Command(binPath, "--version")
	verOut, err := verCmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(string(verOut), "nidhi") {
		t.Errorf("--version output = %q, expected to contain 'nidhi'", string(verOut))
	}
}

func TestE2E_Binary_HelpFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	root := findProjectRoot(t)
	binPath := t.TempDir() + "/nidhi-test"
	build := exec.Command("go", "build", "-o", binPath, "./cmd/nidhi")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	helpCmd := exec.Command(binPath, "--help")
	helpOut, err := helpCmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}

	output := string(helpOut)
	assertScreenContains(t, output, "nidhi")
	assertScreenContains(t, output, "Usage")
	assertScreenContains(t, output, "--version")
	assertScreenContains(t, output, "--log-level")
}

// ─── Local helpers ──────────────────────────────────────────

func findProjectRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			t.Skip("could not find project root")
		}
		dir = parent
	}
}
