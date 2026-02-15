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

### LipGloss v2
- `AdaptiveColor` is NOT in main lipgloss/v2 package. Moved to `lipgloss/v2/compat`.
- RESOLVED: Use `charm.land/lipgloss/v2` import path (NOT `github.com/charmbracelet/lipgloss/v2`). The tagged versions (beta.3) declare old module path, but Bubbles v2 RC1 pulls in a pseudo-version with `charm.land/` module path. Use `go get charm.land/lipgloss/v2@v2.0.0-beta.3.0.20251106192539-4b304240aab7` for exact version.
- `cellbuf` and `ansi` packages must be version-matched. Upgrading `ansi` without upgrading `cellbuf` breaks the build (API signature changes: `Italic()` → `Italic(bool)`).

### Git Operations
- `git stash export/import` requires Git ≥ 2.51. Feature-gate at runtime.
- `git merge-tree --write-tree` requires Git ≥ 2.38. Exit code 0 = clean, 1 = conflicts.

### Go Patterns
- Files ending in `_test.go` are ONLY compiled by `go test`, not `go run` or `go build`. Name throwaway verification files differently.
- `git.Stash` and `plugin.Stash` are separate types with identical fields. Use a `cacheAdapter` in main.go to convert between them. Future refactor: make `git.Stash` a type alias for `plugin.Stash`.
- `git.GitVersion.AtLeast(major, minor, patch)` takes 3 args but `plugin.GitVersion.AtLeast(major, minor)` takes 2 — different signatures, cannot alias. Convert manually.

### Architecture
- `plugin` package is the canonical source for domain types (Stash, AppState, GitVersion, interfaces).
- `git` package has its own parallel types that must be bridged via adapters in `cmd/nidhi/main.go`.
- `core` package uses type aliases (`type AppState = plugin.AppState`) to avoid import indirection.
