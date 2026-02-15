# nidhi — Implementation Progress

> **STRICT RULE:** This file MUST be updated at the end of every coding session.

## Current Phase

Phase 5: Polish (v1.0.0) — "Release"

## Status: 🟢 Phase 4 Complete, Phase 5 TODO

### Milestone Targets (from PRD §18)

#### Phase 1: Core (v0.1.0) — "First Light"
- [x] LIST mode with navigation (scrollable stash list, cursor, row rendering, progressive dimming)
- [x] PREVIEW mode (Tab toggle, diff viewport, file cycling)
- [x] DETAIL mode (file tree + diff split pane)
- [x] Basic CRUD (apply, pop, drop — no conflict preview, no undo yet)
- [x] Agni theme (full theme applied)
- [x] Responsive layout (80×24 through 200×60)
- [x] `--help`, `--version` CLI basics

#### Phase 2: Safety Net (v0.2.0) — "No Fear"
- [x] Conflict preview plugin (merge-tree dry-run, conflict screen)
- [x] Undo plugin (toast, z-key recovery, reflog fallback)
- [x] Rename plugin (inline rename with drop+store)
- [x] New stash screen (message-first, scope toggles, keep-index)

#### Phase 3: Power User (v0.3.0) — "Master of Stashes"
- [x] Deep search plugin (fuzzy search across messages/files/diffs)
- [x] Filter plugin (branch filter, stale filter)
- [x] Stale detection plugin (badge, bulk drop)
- [x] Reorder plugin (Shift+J/K move)

#### Phase 4: Sync (v0.4.0) — "Across Machines"
- [x] Export plugin (multi-select, ref path, remote selector, push)
- [x] Import plugin (fetch, preview, import)
- [x] Git version gating (graceful feature disable for old Git)

#### Phase 5: Polish (v1.0.0) — "Release"
- [x] Help overlay (full keybind reference)
- [x] Config file support (TOML + git config + env vars + CLI flags + structured logging + debug timing)
- [x] Mouse support (click, scroll)
- [ ] Custom themes (theme file format)
- [x] Comprehensive tests (>70% coverage)
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

- Implemented task 014: Phase 1 E2E tests and performance benchmarks
  - internal/e2e/helpers_test.go: setupTestRepo, setupMultiFileStash, gitStashList, gitStashDiff, assertScreenContains/NotContains, noopCache
  - internal/e2e/phase1_test.go: 12 E2E tests covering LIST rendering, cursor navigation, empty state, mode transitions (Tab/Enter/Esc), full cycle, CRUD sequence, detail with real diff, focus switching
  - internal/e2e/benchmark_test.go: BenchmarkStartupTime, TestStartupTimeUnder100ms (avg 19ms), TestStartupTime100Stashes (24ms)
  - Makefile: added `make e2e` target
  - All tests use t.TempDir() + GIT_CONFIG_NOSYSTEM=1 for full isolation
  - Phase 1 ("First Light") is now COMPLETE — 559 tests passing, 0 lint issues
  - Performance: 19ms avg for 20 stashes, 24ms for 100 stashes (budget: <100ms / <300ms)

- Implemented task 015: Conflict preview plugin (merge-tree dry-run, conflict screen)
  - internal/git/mergetree.go: RunMergeTree (git merge-tree --write-tree), ParseMergeTreeOutput, CheckUntrackedCollisions
  - Uses ls-tree (not diff-tree) for rootless untracked commit detection
  - internal/plugins/conflict/conflict.go: Plugin implementing StashHook + ScreenProvider
  - BeforeApply: runs merge-tree dry-run, detects conflicts + untracked collisions
  - Git < 2.38 graceful degradation with info toast (FR-10.7)
  - Fail-open: on merge-tree error, proceed with apply (don't block user)
  - internal/plugins/conflict/screen.go: theme-aware conflict screen with icons, per-file status
  - Plugin registered in main.go (not loader.go, to avoid circular imports)
  - core/events.go: added InfoToastMsg, ErrorMsg, StashMutatedMsg, PromptBranchNameMsg
  - 22 conflict plugin tests (3 integration, 19 unit), 78.6% coverage
  - 600 total tests passing, 0 lint issues

- Implemented task 016: Undo & recovery plugin
  - internal/plugins/undo/ringbuffer.go: goroutine-safe LIFO ring buffer (50 entries), UndoEntry with IsExpired
  - internal/plugins/undo/recovery.go: FindDroppedStashes via git fsck --unreachable --no-reflogs, RestoreCandidate via git stash store
  - internal/plugins/undo/undo.go: Plugin implementing StashHook + KeyHandler
  - AfterDrop: records entry in ring buffer, triggers UndoToastMsg + tea.Tick expiration (30s TTL)
  - HandleKey("z"): recent entry → instant session undo, else → cross-session recovery picker via git fsck
  - Message types: UndoToastMsg, UndoToastExpiredMsg, OpenRecoveryPickerMsg
  - 21 undo plugin tests: ring buffer ops, integration drop/restore, recovery scanning, plugin interface
  - 623 total tests passing, 0 lint issues, 78.2% undo plugin coverage

- Implemented task 017: Rename plugin with reorder journal
  - internal/plugins/rename/journal.go: crash-safe JSON journal for reorder operations (XDG state dir)
  - internal/plugins/rename/rename.go: Plugin implementing KeyHandler for 'r' key
  - RenameStash: simple drop+store for stash@{0}, multi-step reorder for deeper stashes
  - Journal written before operations, cleaned up on success, enables recovery on startup
  - RecoverFromJournal: restores stashes from interrupted rename operations
  - All SHAs preserved across renames (including reorder)
  - 13 rename plugin tests: journal ops, integration rename flows, SHA preservation, recovery
  - 636 total tests passing, 0 lint issues, 79.6% rename plugin coverage

- Implemented task 018: New stash creation screen with scope toggles
  - internal/ui/screens/newstash.go: ScreenProvider for new stash creation
  - Message-first design: cursor starts in message field, custom text input (no bubbles dependency)
  - Scope toggles: Staged, Unstaged, Untracked with live file counts from git status --porcelain
  - Options: keep-index, patch mode (signals PatchModeMsg for tea.Exec)
  - Tab navigation between message, scopes, options; Space to toggle; Enter to create; Esc to cancel
  - BuildArgs constructs correct git stash push flags from UI state
  - Theme-aware rendering via Agni theme (headerStyle, dimStyle, greenStyle, activeStyle, errStyle)
  - 29 new tests: flag construction, tab navigation, view rendering, message input, scope/option toggles
  - 665 total tests passing, 0 lint issues, 77.7% screens coverage

- Implemented task 019: Phase 2 integration & E2E tests (quality gate)
  - internal/e2e/phase2_test.go: 13 cross-feature integration tests
  - Conflict flow: ConflictsDetected (merge-tree detects conflict in same-line change), CleanApply (different files merge cleanly)
  - Undo flow: DropAndRestore (drop+store roundtrip), RingBufferLIFO (3 drops undone in LIFO order), CrossSessionRecovery (fsck finds orphaned stash commit)
  - Rename flow: MiddleStash (rename stash@{1}, verify ordering + SHA preservation)
  - New stash flow: CreateWithMessage, ScopeToggle_StagedOnly (--staged flag), IncludeUntracked (--include-untracked)
  - Cross-feature: ConflictThenUndo (detect conflict, drop, undo restore), RenameThenDropThenUndo (rename then drop then undo preserves renamed message)
  - Edge cases: EmptyRepo_NoStashes, StashWithUntrackedCollision (ls-tree ^3 detects collision)
  - internal/e2e/screenshot_test.go: 3 placeholder screenshot tests behind `screenshot` build tag
  - Shared helpers added to helpers_test.go: stashCount, stashMessages, stashSHAs, stashSHA, fileExists, requireGitVersion
  - 678 total tests passing, 0 lint issues
  - **Phase 2 ("No Fear") is now COMPLETE**

- Implemented task 020: Deep fuzzy search plugin
  - internal/plugins/search/index.go: Index type with fuzzy search (sahilm/fuzzy), scope filtering (All/Messages/Files/Diffs/Branch)
  - ParseDiffForIndex: extracts file names and diff lines from unified diff output, tracks line numbers from hunk headers
  - BuildIndexCmd: async index builder via tea.Cmd — Phase 1 indexes messages/branches (instant), Phase 2 indexes files/diffs (git calls)
  - Deduplication: collapses multiple diff line matches to best score per stash+scope
  - Thread-safe Index with RWMutex, partial results support, Reset for rebuild
  - internal/plugins/search/search.go: Plugin implementing KeyHandler + ScreenProvider
  - Custom text input (no bubbles dependency) matching newstash.go pattern
  - "/" activates search from LIST/PREVIEW, Esc closes, Tab cycles scopes, Ctrl+N/P navigates results, Enter jumps to stash
  - Enter opens PREVIEW for diff/file matches, LIST for message/branch matches
  - Live filtering: re-runs fuzzy search on every keystroke
  - Theme-aware rendering: Agni theme colors, highlighted match characters, scope chips, result count
  - Lazy indexing: index builds on first "/" press (default), builds from AppState.Stashes + StashCache.Diff
  - Plugin registered in main.go as both KeyHandler and ScreenProvider
  - 33 search plugin tests: index unit tests, diff parser tests, plugin unit tests, view tests, 3 integration tests
  - 711 total tests passing, 0 lint issues, 86.4% search plugin coverage

- Implemented task 021: Filter & stale detection plugins
  - internal/plugins/filter/filter.go: Plugin implementing KeyHandler for filter toggling
  - `f` toggles branch filter (show only stashes from current branch), `F` toggles stale filter
  - Filters compose with AND logic via FilterStashes() — branch + stale = only stale stashes on current branch
  - Active filters stored in state.Filters with ID/Label/Value for status bar chip rendering
  - Cursor resets to 0 on filter change to avoid out-of-bounds
  - internal/plugins/stale/stale.go: Passive plugin for staleness computation
  - MarkStaleWithTime: deterministic staleness computation with configurable threshold (default 14 days)
  - StaleStashes/StaleCount: helper functions for filtering and counting stale entries
  - BulkDropStaleCmd: drops stale stashes from highest index first to preserve ordering
  - ConfigStore integration for stale_days setting
  - Both plugins registered in main.go
  - 14 filter tests (FilterStashes unit, KeyHandler toggle/compose, cursor reset, integration with stale marking)
  - 10 stale tests (staleness calculation table test with 12 cases, filter, count, preserve fields, empty list, plugin API)
  - 747 total tests passing, 0 lint issues, 92.9% filter / 55.8% stale coverage

- Implemented task 022: Reorder plugin with journal-based crash recovery
  - internal/plugins/reorder/reorder.go: Plugin implementing KeyHandler with Shift+J/K
  - J (Shift+J) moves selected stash down one position, K (Shift+K) moves up
  - Drop-all + re-store algorithm: drops all stashes, re-stores in new order via `git stash store`
  - Cursor follows the moved stash so selection is preserved
  - internal/plugins/reorder/journal.go: Journal persistence for transactional safety
  - Journal written before reorder, removed after success — crash recovery restores original order
  - Uses separate journal path (move-journal.json) to avoid conflict with rename's reorder-journal.json
  - ComputeNewOrder: correct array reordering with no off-by-one (fixed spec bug in index adjustment)
  - Recovery on startup: detects incomplete journal, clears partial state, re-stores from journal
  - Plugin registered in main.go as KeyHandler, recovery wired alongside rename recovery
  - 24 reorder tests: 7 journal unit tests, 5 ComputeNewOrder unit tests, 8 plugin unit tests, 4 git integration tests
  - 771 total tests passing, 0 lint issues, 54.1% reorder coverage

- Implemented task 023: Export/Import & Remote Sync plugin
  - internal/plugins/sync/sync.go: Plugin implementing KeyHandler + ScreenProvider (PRD FR-12)
  - Version gating: git stash export/import requires Git >= 2.51; shows toast on older Git
  - Export screen: multi-select stash list (Space toggle, 'a' select all), ref path input with cursor, remote selector
  - Import screen: ref path input with cursor, remote selector, fetch+import workflow
  - ExportCmd: validate ref → `git stash export --to-ref <ref>` → `git push --no-verify --force <remote> <ref>`
  - ImportCmd: `git fetch <remote> <ref>:<ref>` → `git stash import <ref>`
  - ParseRemotes: parses `git remote -v` output, deduplicates by name
  - ExportCommandPreview: live command preview updated on selection/ref/remote change
  - Config integration: export_ref, export_remote settings from config; $USER expansion in ref path
  - Custom text input for ref editing (no bubbles dependency), same pattern as newstash/search
  - Tab navigation between fields (3 in export, 2 in import), Esc cancels, Enter executes
  - Plugin registered in main.go as both KeyHandler and ScreenProvider
  - 46 sync plugin tests: identity/bindings, version gating, export screen flow, import screen flow, ParseRemotes, ExportCmd/ImportCmd unit tests, ValidateRef, edge cases
  - 817 total tests passing, 0 lint issues, 78.1% sync plugin coverage
  - **Phase 4 ("Across Machines") is now COMPLETE**

- Implemented task 024: Help overlay with Canvas compositing and mouse support
  - internal/ui/screens/help.go: HelpOverlay with all keybind categories from PRD §11.2
  - 5 categories: Global, Navigation, Actions, Search & Filter, Export & Import
  - LipGloss Canvas compositing: dimmed background + z-layered overlay via NewCanvas/NewLayer
  - Toggle on/off with `?`, scrollable content, adapts to terminal dimensions
  - RenderWithDimmedBackground for modal compositing over existing screen content
  - internal/ui/mouse/mouse.go: Mouse Handler for additive mouse support (PRD §11.3)
  - Click row → select stash, scroll wheel up/down, click chip → toggle filter, click checkbox → toggle selection
  - Status bar and footer clicks ignored (no-op)
  - Configurable row height (compact vs default), chip/checkbox regions
  - 10 help tests (categories, keybindings, toggle, scroll, adapt size, dimmed bg)
  - 8 mouse tests (row click, compact mode, scroll, chip click, checkbox click, status bar, footer)
  - 839 total tests passing, 0 lint issues, 100% mouse coverage, 79.7% screens coverage

- Implemented task 025: Config file polish — env vars, CLI flags, structured logging, debug timing
  - internal/config/config.go: Added CLI-only fields (Debug, TraceGit, NoColor, NoAnimation, Directory) with `toml:"-"` tags
  - internal/config/loader.go: Extended loadFromEnv() with NO_COLOR (presence-based), REDUCE_MOTION, NERD_FONTS (1/true→nerd, 0/false→ascii)
  - Extended applyFlags() with TraceGit, Debug, NoColor, NoAnimation, Directory
  - internal/config/logging.go: SetupLogging with slog → JSON handler, XDG state dir (~/.local/state/nidhi/nidhi.log)
  - TraceGit forces debug level, "off" level returns discard logger, auto-creates log directory
  - internal/config/debug.go: DebugTiming for --debug startup breakdown (Record, Since, Print, Entries)
  - cmd/nidhi/main.go: Full CLI flag parser, structured logging setup, --debug exit, -C directory override
  - 27 new config tests: NO_COLOR (including empty value), REDUCE_MOTION, NERD_FONTS, all CLI flags,
    logging setup (off/debug/trace-git/directory creation), DefaultLogPath, DebugTiming, ParseLogLevel, priority override
  - 866 total tests passing, 0 lint issues

- Implemented task 026: Comprehensive E2E tests
  - internal/e2e/phase3_test.go: 22 tests covering Phase 3+4 plugins
    - Search: index build & query, scope filtering (messages/files/diffs/branch), activation via '/', empty query
    - Filter: branch filter, stale filter, both filters composing
    - Stale: MarkStaleWithTime, stale count & filter integration, BulkDropStaleCmd
    - Reorder: J (move down), K (move up), boundary no-op, multiple swaps
    - Cross-feature: reorder→rename→drop sequence, filter+stale integration
  - internal/e2e/full_test.go: ~35 tests covering cross-cutting concerns
    - Help overlay: toggle, render, hidden, scrolling, categories, dimmed background
    - Mode manager: full cycle, help from any mode, invalid transitions, stack depth limit
    - Mouse: click to select row, scroll events, click outside list
    - Config: defaults valid, CLI flags override env, stale threshold, logging setup
    - Preview: real diff loading via DiffLoadedMsg, file cycling
    - Full workflow: LIST→PREVIEW→DETAIL→LIST transitions with real stash diffs
    - Git version: detection, AtLeast comparisons, merge-tree 2.38 gate, export/import feature gate
    - Performance: LIST render 50 stashes < 100ms, stash parsing 200 stashes < 500ms
    - State management: WithStashes cursor clamp, SelectedStash
    - New stash: empty screen rendering
    - Event bus: pub/sub delivery, multiple subscribers
    - Git ops: large stash with 20 files
    - Binary: build successfully, --help flag
  - Fixed buildPluginStashes to set RawMessage (needed for reorder plugin store operations)
  - 916 total tests passing, 0 lint issues

- Implemented task 027: Performance validation and NFR benchmarks
  - internal/perf/helpers_test.go: shared benchmark fixtures (benchRepo, testRepo, generateGoFile, runGit)
  - internal/perf/startup_test.go: startup benchmarks (0/20/100 stashes), timing assertion tests, --debug flag test
  - internal/perf/operation_test.go: operation latency benchmarks and tests
    - Cursor move + render: 587us (target < 1ms) PASS
    - LIST render 50 stashes: 624us (target < 100ms) PASS
    - Diff load: ~7ms (target < 200ms) PASS
    - Apply: 151ms (target < 500ms) PASS
    - Rename: 13ms (target < 100ms) PASS
    - Search index build 50 stashes: 971ms (target < 2s) PASS
  - internal/perf/memory_test.go: memory usage validation
    - 50 stashes: 168KB heap (target < 40MB) PASS
    - LRU cache: respects 10-entry limit PASS
    - No leak after 100 operations: 0B growth PASS
  - internal/perf/visual_test.go: iterm2-driver visual responsiveness (behind NIDHI_VISUAL_TEST=1 env flag)
  - Makefile: added bench, bench-short, profile, perf-test targets
  - docs/profiling-results.md: documented all measurements with pass/fail
  - 933 total tests passing, 0 lint issues

- Implemented task 028: CI/CD and GitHub Actions
  - .github/workflows/ci.yml: CI pipeline — lint, test (Go 1.26 matrix on Linux + macOS), coverage > 70% gate, build + verify, E2E tests
  - .github/workflows/release.yml: release pipeline — goreleaser on v* tags, cross-platform binaries, GitHub Releases
  - .github/dependabot.yml: weekly dependency updates for gomod and github-actions
  - .goreleaser.yml: expanded with checksums, changelog groups, Homebrew formula, release metadata
  - Makefile: added coverage, coverage-check, release-check, release-dry-run targets
  - goreleaser check validates config successfully

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
| 014 | Phase 1 integration & E2E | P1 | DONE | 010-013 |
| 015 | Conflict preview plugin | P2 | DONE | 013, 006 |
| 016 | Undo plugin | P2 | DONE | 013, 007 |
| 017 | Rename plugin | P2 | DONE | 013, 008 |
| 018 | New stash screen | P2 | DONE | 013, 006 |
| 019 | Phase 2 integration & E2E | P2 | DONE | 015-018 |
| 020 | Search plugin | P3 | DONE | 006, 004 |
| 021 | Filter & stale plugins | P3 | DONE | 006, 004 |
| 022 | Reorder plugin | P3 | DONE | 013, 017 |
| 023 | Export/import plugin | P4 | DONE | 006, 001 |
| 024 | Help overlay & mouse support | P5 | DONE | 006, 007 |
| 025 | Config file & polish | P5 | DONE | 002, 006 |
| 026 | Comprehensive E2E tests | Final | DONE | 000-024 |
| 027 | Performance validation | Final | DONE | 026 |
| 028 | CI/CD & GitHub Actions | Final | DONE | 027 |
| 029 | Documentation & README | Final | TODO | 026 |
| 030 | Homebrew tap & release | Final | TODO | 028, 029 |
