# CLAUDE.md — nidhi AI Agent Instructions

> **This file is the source of truth for all AI coding agents working on nidhi.**
> AGENTS.md points here. Do not duplicate instructions elsewhere.

## Project Overview

**nidhi** (निधि, "treasure") is a purpose-built Go TUI for `git stash` mastery.
Built with BubbleTea v2, LipGloss v2, and Bubbles v2 on Go 1.26.
Module path: `github.com/indrasvat/nidhi`

- **PRD:** `docs/PRD.md` — full product requirements, architecture, UI specs
- **Mockups:** `docs/nidhi-full-mockup.html` — 10-screen Agni theme visual spec
- **Progress:** `docs/PROGRESS.md` — implementation tracker (MUST be kept current)

## Build & Test Commands

```bash
make build           # Build binary → bin/nidhi
make test            # Run tests with gotestsum (race detection + coverage)
make lint            # Run golangci-lint
make check           # lint + test (what pre-commit runs)
make ci              # CI-only target (lint + test, fail-fast)
make install         # Install to ~/.local/bin/nidhi
make install-tools   # Install dev dependencies (gotestsum, golangci-lint, etc.)
make install-hooks   # Install lefthook git hooks
make clean           # Remove build artifacts
```

## Architecture

```
main.go (CLI parsing, config load)
    ↓
Core Engine (git runner, stash cache, mode manager, event bus, config, theme)
    ↓
Plugin Host (conflict, search, sync, rename, undo, stale, reorder, filter)
    ↓
UI Layer — BubbleTea v2 (screen router, layout engine, overlay manager, components)
```

**Key packages:**
- `cmd/nidhi/` — CLI entrypoint
- `internal/core/` — top-level BubbleTea model, mode management, state, events
- `internal/git/` — GitRunner, stash parsing, cache, version detection, merge-tree
- `internal/plugin/` — plugin registry, interfaces, context, loader
- `internal/plugins/` — built-in plugins (conflict, search, sync, rename, undo, stale, reorder, filter)
- `internal/ui/theme/` — Agni theme, theme interface
- `internal/ui/layout/` — layout engine, split pane, responsive
- `internal/ui/components/` — status bar, footer, toast, confirm, stash row, file tree, diff view, filter chips
- `internal/ui/screens/` — LIST, PREVIEW, DETAIL, new stash, export, import, help
- `internal/config/` — config loading (TOML + git config + env + CLI flags)

## Code Conventions

- **Format:** `gofmt` (enforced by CI). No tabs vs spaces debates.
- **Linting:** `golangci-lint` with config in `.golangci.yml`. Must pass before commit.
- **Naming:** Standard Go conventions. Exported types have doc comments.
- **Errors:** Return errors, don't panic. `fmt.Errorf("context: %w", err)` for wrapping.
- **No `panic`** outside `main.go`. Recover in main, log stack trace, restore terminal.
- **Context:** All git operations take `context.Context` with timeouts.
- **Testing:** Table-driven tests. `t.Helper()` for test helpers. Test file naming: `foo_test.go`.
- **Imports:** Standard library first, then third-party, then internal. Enforced by `goimports`.

## Git Workflow

- **Branch naming:** `feat/`, `fix/`, `refactor/`, `docs/`, `chore/`
- **Commits:** Conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`)
- **PRs:** One feature/fix per PR. Reference PRD section if applicable.
- **Hooks:** lefthook runs lint + tests on pre-commit, full test suite on pre-push.

## Key Decisions

| Decision | Rationale | Date |
|---|---|---|
| Makefile over Taskfile | Universally available, no extra dependency, simpler for Go projects | 2026-02-14 |
| Pin Charm v2 RC/beta versions | All v2 libs are pre-release; pin to avoid breaking changes from auto-updates | 2026-02-14 |
| `github.com/indrasvat/nidhi` module path | Standard Go convention (org/repo), matches intended final repo location | 2026-02-14 |
| gotestsum for test runner | Better output formatting, JUnit XML for CI, color output | 2026-02-14 |
| Custom stash list over bubbles list.Model | list.Model is too opinionated (built-in filtering, status bar); need bare-metal row rendering with inline editing | 2026-02-14 |
| LipGloss Canvas for modals | Z-indexed layer compositing is the canonical v2 approach for overlays | 2026-02-14 |
| `charmbracelet/colorprofile` not `muesli/termenv` | termenv is replaced by colorprofile in the Charm v2 ecosystem | 2026-02-14 |

## Important API Notes

### BubbleTea v2
- `tea.View.Content` is `tea.Layer` (interface), NOT a string. Wrap with `tea.NewLayer(renderedString)`.
- `tea.View` has 11 fields: Content, Cursor, BackgroundColor, ForegroundColor, WindowTitle, ProgressBar, AltScreen, ReportFocus, DisableBracketedPasteMode, MouseMode, KeyboardEnhancements.
- `tea.KeyPressMsg` fields: Text (string), Mod (KeyMod), Code (int32), ShiftedCode (int32), BaseCode (int32), IsRepeat (bool).
- `tea.KeyReleaseMsg` has the same fields as `tea.KeyPressMsg`.
- Color downsampling is automatic via `charmbracelet/colorprofile` — no manual fallback colors needed.

### LipGloss v2
- `lipgloss.AdaptiveColor` does NOT exist in the main `lipgloss/v2` package. It moved to `lipgloss/v2/compat`. Use `lipgloss.Color()` with hex values directly; colorprofile handles downsampling.
- Import path: `github.com/charmbracelet/lipgloss/v2` (beta.3). Will transition to `charm.land/lipgloss/v2` when stable.
- `NewCanvas(layers ...*Layer) *Canvas` — canvas compositing for modals.
- `NewLayer(content any) *Layer` — content can be string, Stringer, or Drawable.
- Layer positioning: `.X(int)`, `.Y(int)`, `.Z(int)` for coordinates and z-index.
- `tree` and `table` are sub-packages at `lipgloss/v2/tree` and `lipgloss/v2/table`.

### Bubbles v2
- `key.Binding` is the core type for keybindings. There is no concrete `key.Map` type — define custom structs implementing the `help.KeyMap` interface (`ShortHelp() []key.Binding`, `FullHelp() [][]key.Binding`).
- `teatest` lives at `github.com/charmbracelet/x/exp/teatest` — NOT in bubbles or bubbletea packages.

## Learnings

> **STRICT RULE:** This section MUST be updated at the end of every coding session.
> Each entry should be a concrete, actionable insight. Delete entries that become obsolete.

### BubbleTea v2
- `tea.View.Content` is `tea.Layer` (interface), not a string. Wrap with `tea.NewLayer()`.
- KeyPressMsg uses int32 Code field, not string. Use tea package key constants.
- Arrow keys: use `tea.KeyUp`, `tea.KeyDown` constants alongside `msg.Text == "j"` / `msg.Text == "k"`. Always add both in the same switch case to ensure full keyboard support.

### LipGloss v2
- `AdaptiveColor` is NOT in main lipgloss/v2 package. Moved to `lipgloss/v2/compat`.
- RESOLVED: Use `charm.land/lipgloss/v2` import path (NOT `github.com/charmbracelet/lipgloss/v2`). The tagged versions (beta.3) declare old module path, but Bubbles v2 RC1 pulls in a pseudo-version with `charm.land/` module path. Use `go get charm.land/lipgloss/v2@v2.0.0-beta.3.0.20251106192539-4b304240aab7` for exact version.
- `cellbuf` and `ansi` packages must be version-matched. Upgrading `ansi` without upgrading `cellbuf` breaks the build (API signature changes: `Italic()` → `Italic(bool)`).
- `lipgloss.Color` is a FUNCTION (`func(s string) color.Color`), NOT a type. Cannot use in type assertions or as return types. Return `color.Color` instead.
- `lipgloss.CompleteColor` does NOT exist in v2. Use `[]color.Color` with `lipgloss.Color("#hex")` for gradient arrays. No adaptive/complete color wrappers — colorprofile handles downsampling.

### Git Operations
- `git stash export/import` requires Git ≥ 2.51. Feature-gate at runtime.
- `git merge-tree --write-tree` requires Git ≥ 2.38. Exit code 0 = clean, 1 = conflicts. Output: tree SHA on line 1, then Auto-merging/CONFLICT messages.
- Stash untracked files (3rd parent): use `ls-tree --name-only -r stash^3` (NOT `diff-tree`) because the untracked commit is rootless and `diff-tree` produces no output for commits without parents.
- `DefaultRunner.Run()` swallows `exec.ExitError` — returns `(stdout, nil)` for non-zero exits. Always use `RunExitCode()` for operations that must detect failure.
- `git stash push` returns exit 0 even when there are no local changes. Detect via output containing "No local changes to save".
- `git stash store -m <msg> <sha>` re-stores a previously dropped stash. Used for undo recovery and reorder.
- Reorder algorithm: drop ALL stashes (highest first), re-store in new order (last to first, since store prepends). After removing element at sourceIndex, insert at targetIndex directly — NO index adjustment needed.
- `strings.SplitSeq` (Go 1.24+ iterator) is preferred by golangci-lint stringsseq rule over `strings.Split` in range loops.
- `fmt.Appendf(nil, ...)` is preferred by golangci-lint fmtappendf rule over `[]byte(fmt.Sprintf(...))`.

### Go Patterns
- Files ending in `_test.go` are ONLY compiled by `go test`, not `go run` or `go build`. Name throwaway verification files differently.
- `git.Stash` and `plugin.Stash` are separate types with identical fields. Use a `cacheAdapter` in main.go to convert between them. Future refactor: make `git.Stash` a type alias for `plugin.Stash`.
- `git.GitVersion.AtLeast(major, minor, patch)` takes 3 args but `plugin.GitVersion.AtLeast(major, minor)` takes 2 — different signatures, cannot alias. Convert manually.

### Architecture
- `plugin` package is the canonical source for domain types (Stash, AppState, GitVersion, interfaces).
- `git` package has its own parallel types that must be bridged via adapters in `cmd/nidhi/main.go`.
- `core` package uses type aliases (`type AppState = plugin.AppState`) to avoid import indirection.
- **UIRenderer pattern**: `core` cannot import `ui/screens` (circular dependency). The `core.UIRenderer` interface is injected from `cmd/nidhi/main.go` which has access to all packages. This wires real ListScreen/PreviewScreen/DetailScreen/HelpOverlay/StatusBar/Footer/Toast into the core model. Without this, the TUI renders placeholder text.
- **Mode change side effects**: Both `pushMode()` and `popMode()` call `UIRenderer.OnModeChange()` and return `tea.Cmd`. Used for diff loading (PREVIEW/DETAIL) and help overlay show/hide.
- **Key delegation**: Core handles only mode-switch keys (Tab/Enter via pushMode, Esc via popMode) and global keys (q, ?, Ctrl+C). All other keys (j/k/g/G, CRUD, search) delegate to `m.UI.HandleMessage()` which routes to the active screen, then falls through to plugin key handlers. **Exception**: ModeDetail delegates ALL keys to UI (including Tab for tree/diff focus toggle) — core does NOT intercept Tab in Detail.
- **Dead key handler trap**: If both core and a screen handle the same key (e.g., Esc in both core and detail.go), the core handler runs first and the screen handler is dead code. When core behavior changes, dead handlers mask bugs. Audit for duplicate key handling when modifying routing.
- **Every mode must be routed**: Core's `handleKeyPress()` switch must have a case for every mode. Missing cases fall through to plugin handlers only, leaving screen-level keys (j/k/Tab) unreachable.
- **Always verify TUI renders real content** — unit tests can pass while the top-level model still returns placeholder strings if the rendering pipeline isn't wired end-to-end. Use `scripts/setup-demo.sh` + iTerm2 driver to verify.
- **tea.View.BackgroundColor** (`color.Color`): Set to theme bg.deep color to fill ALL empty terminal cells via OSC 11. BubbleTea v2 handles this natively — no termenv needed.
- Plugin registration happens in `cmd/nidhi/main.go`, not `plugin/loader.go`, to avoid circular imports (plugins/ → plugin/ → plugins/ cycle).
- `plugin.GitRunner` and `git.GitRunner` have identical methods — Go structural typing allows passing either where the other is expected.
- Rename plugin uses `~/.local/state/nidhi/reorder-journal.json` (for rename's drop+re-store). Reorder plugin uses `~/.local/state/nidhi/move-journal.json` to avoid collision.
- Import `sync` package as `pluginsync` in `main.go` to avoid collision with Go's built-in `sync` package.
- `plugin.NewPluginContext()` requires all 7 params non-nil (including Theme). Tests must pass a mock theme, not nil.
- **Interface extension cascade**: Adding a method to `core.UIRenderer` requires updating ALL implementations — `uiRenderer` in main.go, `mockUIRenderer` in app_test.go, and any test helpers. Compile will catch missing methods but only if tests are run.
- **Welcome screen pattern**: Model-level boolean guard (`m.Welcome`) in `handleKeyPress()` intercepts ALL keys before mode routing. Only Enter (dismiss), q, Ctrl+C pass through. View() renders welcome when `m.Welcome && m.ready`.

### UI Components
- Mockup badge colors: LIST=gold, PREVIEW=aqua, DETAIL=blue, SEARCH=purple, EXPORT=orange, NEW=green, CONFLICT=yellow, HELP=dimmed.
- Mockup toast classes: info=green (.toast-ok), error=red, undo=BLUE (.toast-undo) — NOT yellow.
- Status bar shows repo name (not "nidhi"), uses ◆ mark, ⎇ branch prefix.
- All UI components should accept `theme.Theme` interface, not hardcoded hex strings.
- Use `styledFgBg(fg, bg color.Color) lipgloss.Style` helper for repeated fg+bg style patterns.
- `color.Color` interface method `RGBA()` returns pre-multiplied values 0-65535. Shift `>>8` to get 0-255 range for blending.
- `strings.Split("", "\n")` returns `[""]` (1 element), not `[]`. Always guard empty-string parsing with early return.
- For time-dependent tests, pass reference `time.Time` into test helpers — never call `time.Now()` independently inside helper functions.
- File tree category colors per mockup: staged=SemanticGreen, working=SemanticYellow, untracked=SemanticCoral.
- `strings.FieldsSeq` (Go 1.24+ iterator) is preferred over `strings.Fields` by golangci-lint.
- `layout.ComputeSplit(totalSize, ratio)` properly handles narrow terminals by collapsing the secondary pane (returns SecondarySize=0, DividerSize=0).
- Use `msg.Mod.Contains(tea.ModCtrl)` not `msg.Mod == tea.ModCtrl` for modifier checks (other modifiers may be set simultaneously).
- `FileTreeModel.visibleItems()` always renders all 3 category headers (staged/working/untracked) even when empty. Phase 2 will add proper category distinction.
- Staticcheck QF1006: loop conditions like `for { if cond { break } ... }` should be `for !cond { ... }`.
- **Singleton screens**: Screens (ListScreen, PreviewScreen, DetailScreen) are created once in main.go and reused across mode transitions. Mutable state like `focused`, cursor positions persists — must be explicitly reset via OnModeChange on mode entry (e.g., `detail.ResetFocus()`).
- Session-only visual state belongs in the singleton screen that owns the interaction (e.g., ListScreen pin markers) and can be shared by PREVIEW through the existing embedded ListScreen.
- Core key dispatch must use `state.Mode`, not only `ModeManager.Current()`, because plugin/screen-provider flows can change mode without pushing the core mode stack.
- **iTerm2 screen buffer includes scrollback**: `async_get_screen_contents()` returns the full buffer, not just visible lines. Strip trailing blank lines before checking footer content.

### iTerm2 E2E Testing Patterns
- **Polling over sleeps**: Use `wait_for(session, keyword, timeout)` that polls every 250ms instead of `asyncio.sleep(N)`. Hardcoded sleeps are brittle across machines.
- **Content-change assertions over keyword presence**: After pressing a key, assert `before != after` (screen actually changed), not just `"keyword" in screen`. Keyword checks miss broken key routing.
- **Scrollback buffer**: `async_get_screen_contents()` returns ALL lines including empty scrollback. Footer checks must strip trailing blank lines first.
- **`setup-demo.sh` uses `exec`**: The script ends with `exec "$BIN"`, replacing the shell process with nidhi. After launch, the iTerm2 session IS the TUI — no shell prompt returns.
- **Welcome screen handling**: Tests must wait for "Press Enter" or "LIST", dismiss welcome if present, then wait for "LIST" badge before starting interaction tests.
