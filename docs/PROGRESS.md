# nidhi — Implementation Progress

> **STRICT RULE:** This file MUST be updated at the end of every coding session.

## Current Phase

Phase 1: Core (v0.1.0) — "First Light"

## Status: 🟡 In Progress

### Milestone Targets (from PRD §18)

#### Phase 1: Core (v0.1.0) — "First Light"
- [ ] LIST mode with navigation (scrollable stash list, cursor, row rendering, progressive dimming)
- [ ] PREVIEW mode (Tab toggle, diff viewport, file cycling)
- [ ] DETAIL mode (file tree + diff split pane)
- [ ] Basic CRUD (apply, pop, drop — no conflict preview, no undo yet)
- [ ] Agni theme (full theme applied)
- [ ] Responsive layout (80×24 through 200×60)
- [ ] `--help`, `--version` CLI basics

#### Phase 2: Safety Net (v0.2.0) — "No Fear"
- [ ] Conflict preview plugin (merge-tree dry-run, conflict screen)
- [ ] Undo plugin (toast, z-key recovery, reflog fallback)
- [ ] Rename plugin (inline rename with drop+store)
- [ ] New stash screen (message-first, scope toggles, keep-index)

#### Phase 3: Power User (v0.3.0) — "Master of Stashes"
- [ ] Deep search plugin (fuzzy search across messages/files/diffs)
- [ ] Filter plugin (branch filter, stale filter)
- [ ] Stale detection plugin (badge, bulk drop)
- [ ] Reorder plugin (Shift+J/K move)

#### Phase 4: Sync (v0.4.0) — "Across Machines"
- [ ] Export plugin (multi-select, ref path, remote selector, push)
- [ ] Import plugin (fetch, preview, import)
- [ ] Git version gating (graceful feature disable for old Git)

#### Phase 5: Polish (v1.0.0) — "Release"
- [ ] Help overlay (full keybind reference)
- [ ] Config file support (TOML + git config + env vars)
- [ ] Mouse support (click, scroll)
- [ ] Custom themes (theme file format)
- [ ] Comprehensive tests (>70% coverage)
- [ ] Documentation (README, man page)
- [ ] Homebrew tap

## Session Log

### 2026-02-14
- Created PRD (docs/PRD.md) with 21 sections covering full product spec
- Created HTML mockup (docs/nidhi-full-mockup.html) with 10 screens
- Verified all tech stack claims: BubbleTea v2 (RC2), LipGloss v2 (beta.3), Bubbles v2 (RC1), Go 1.26, Git 2.53.0
- Created bootstrap files: CLAUDE.md, AGENTS.md, Makefile, lefthook.yml, docs/PROGRESS.md
- Key findings: tea.View.Content is tea.Layer interface (not string), AdaptiveColor moved to lipgloss/v2/compat, teatest at github.com/charmbracelet/x/exp/teatest, termenv replaced by colorprofile in v2
- Incorporated codex review (28 findings) and gemini review (7 findings) into PRD
- Created 31 detailed task files (docs/tasks/000-030) covering full implementation roadmap:
  - Phase 1 (000-014): Scaffold, git runner, config, theme, parser/cache, plugins, core model, layout, components, screens, CRUD, E2E
  - Phase 2 (015-019): Conflict preview, undo, rename, new stash screen, integration gate
  - Phase 3 (020-022): Search, filter/stale, reorder plugins
  - Phase 4 (023): Export/import plugin
  - Phase 5 (024-025): Help overlay, mouse support, config polish
  - Final (026-030): Comprehensive E2E, performance validation, CI/CD, docs, release

### 2026-02-15
- Implemented tasks 005-006: plugin interfaces/registry, core BubbleTea model, mode management
- Fixed Charm v2 dependency graph: switched lipgloss to charm.land/lipgloss/v2, added charm.land/bubbles/v2@rc.1
- Wired main.go to launch real BubbleTea TUI program with all services
- iTerm2 visual testing confirmed: TUI launches, empty state renders, j/k/? keys work, q quits cleanly
- 287 tests passing, 0 lint issues, ~80%+ coverage across packages

- Implemented task 007: layout engine, status bar, footer, toast components
  - layout/responsive.go: three-tier breakpoints (minimal 80x24, standard 120x40, large 200x60), column collapse rules
  - layout/split.go: configurable split panes — PREVIEW (40/60 vertical), DETAIL (25/75 horizontal) with minimum enforcement
  - layout/layout.go: three-band chrome (status bar + content + footer), JoinVertical/JoinHorizontal composition, dimension helpers
  - components/statusbar.go: renders ◆ mark, repo name, ⎇ branch, stash count, git version — matched to mockup
  - components/footer.go: mode-specific keybind hints with color-coded mode badge — PREVIEW=aqua, DETAIL=blue per mockup
  - components/toast.go: timed toast with 3 classes (info=green 5s, error=red 5s, undo=blue 30s), auto-dismiss via tea.Tick
  - All components use theme.Theme interface, not hardcoded hex values
  - 353 tests passing, 0 lint issues, 84% coverage on components, 72% on layout

- Implemented tasks 008-009: stash row renderer, diff view, file tree, filter chips
  - components/stashrow.go: two-line responsive row — line 1: cursor/index/SHA/message/age, line 2 (>=100 cols): branch/diffstat/filecount/tags
  - Progressive dimming via blendColor() with square-root curve capped at 60%
  - Inline rename support with block cursor and "was:" hint
  - components/diffview.go: unified diff parser with line-number gutter, scrollable viewport, theme-aware syntax coloring
  - components/filetree.go: file tree grouped by staged/working/untracked, collapsible categories, category-specific colors (green/yellow/coral per mockup)
  - components/filterchip.go: toggle chip group for search scopes — exclusive "All" behavior, auto-reactivation
  - All components use theme.Theme interface, not hardcoded hex values
  - 420 tests passing, 0 lint issues, 88.8% coverage on components

- Implemented tasks 010-012: LIST, PREVIEW, DETAIL screens
  - screens/list.go: custom scrollable list with cursor navigation, responsive row height (1-line < 100 cols, 2-line >= 100 cols)
  - Stash command message types: StashApplyMsg, StashPopMsg, StashDropMsg, StashRenameMsg, StashBranchMsg
  - Navigation: j/k, g/G jump, Ctrl+D/U page scroll, visibleRows formula: (height+1)/(rh+1)
  - Mode switching: Tab→PREVIEW, Enter→DETAIL, n→NEW_STASH, e→EXPORT, i→IMPORT
  - CRUD dispatch: a/p/d/r/b dispatch tea.Cmd messages for apply/pop/drop/rename/branch
  - Empty state rendering with theme colors
  - screens/preview.go: split layout 40% list (top) + divider with file progress + 60% diff (bottom)
  - Async diff loading via DiffLoadedMsg with stale response detection
  - parseDiffFiles/extractFilename for per-file diff splitting
  - File cycling (h/l), diff scroll (Ctrl+D/U), CRUD delegation to embedded ListScreen
  - screens/detail.go: horizontal split with FileTreeModel (25% left) + DiffViewModel (75% right)
  - Uses layout.ComputeSplit with DetailSplitRatio, collapses gracefully on narrow terminals
  - Tab switches focus between tree/diff panes, Esc returns to previous mode
  - j/k per-pane navigation, Enter expand/collapse tree groups, Ctrl+D/U page scroll
  - inferFileStatus from diff content: new file → Added, deleted → Removed, rename → Renamed, else Modified
  - selectFirstFile auto-advances past category headers on SetDiff
  - No bubbles dependency — uses custom DiffViewModel and FileTreeModel from task 009
  - 527 tests passing, 0 lint issues, 89.8% coverage on screens

- Implemented task 013: stash CRUD operations
  - internal/git/operations.go: StashOps struct with Apply, Pop, Drop, Push, BranchFromStash, ClearAll, RestoreStash
  - All operations use RunExitCode for proper error detection (Run() swallows ExitError)
  - SHA captured before destructive ops (Pop, Drop, ClearAll) for undo support
  - Push detects "No local changes to save" (git exits 0 even with nothing to stash)
  - ClearAll captures all SHAs+messages before clearing for bulk undo
  - RestoreStash uses `git stash store` for undo recovery
  - PushOptions supports: Message, KeepIndex, IncludeUntracked, Staged, Pathspecs
  - Cache invalidated on all mutating operations; Apply skips invalidation (list unchanged)
  - 18 integration tests in operations_test.go: real git repos, no mocks
  - 545 tests passing, 0 lint issues, 82.9% coverage on git package

## Task List

| # | Task | Phase | Status | Depends On |
|---|---|---|---|---|
| 000 | Repository scaffold & tooling | Setup | DONE | — |
| 001 | Git runner & version detection | P1 | DONE | 000 |
| 002 | Config loading | P1 | DONE | 000 |
| 003 | Agni theme & icons | P1 | DONE | 000 |
| 004 | Stash parser & cache | P1 | DONE | 001 |
| 005 | Plugin interfaces & registry | P1 | DONE | 001, 002 |
| 006 | Core BubbleTea model & mode mgmt | P1 | DONE | 005, 004, 003 |
| 007 | Layout engine & chrome | P1 | DONE | 006, 003 |
| 008 | Stash row renderer | P1 | DONE | 003, 007 |
| 009 | Diff view & file tree | P1 | DONE | 003, 007 |
| 010 | LIST screen | P1 | DONE | 006, 008, 007 |
| 011 | PREVIEW screen | P1 | DONE | 010, 009, 004 |
| 012 | DETAIL screen | P1 | DONE | 010, 009, 007 |
| 013 | Stash CRUD operations | P1 | DONE | 001, 004 |
| 014 | Phase 1 integration & E2E | P1 | TODO | 010-013 |
| 015 | Conflict preview plugin | P2 | TODO | 013, 006 |
| 016 | Undo plugin | P2 | TODO | 013, 007 |
| 017 | Rename plugin | P2 | TODO | 013, 008 |
| 018 | New stash screen | P2 | TODO | 013, 006 |
| 019 | Phase 2 integration & E2E | P2 | TODO | 015-018 |
| 020 | Search plugin | P3 | TODO | 006, 004 |
| 021 | Filter & stale plugins | P3 | TODO | 006, 004 |
| 022 | Reorder plugin | P3 | TODO | 013, 017 |
| 023 | Export/import plugin | P4 | TODO | 006, 001 |
| 024 | Help overlay & mouse support | P5 | TODO | 006, 007 |
| 025 | Config file & polish | P5 | TODO | 002, 006 |
| 026 | Comprehensive E2E tests | Final | TODO | 000-024 |
| 027 | Performance validation | Final | TODO | 026 |
| 028 | CI/CD & GitHub Actions | Final | TODO | 027 |
| 029 | Documentation & README | Final | TODO | 026 |
| 030 | Homebrew tap & release | Final | TODO | 028, 029 |
