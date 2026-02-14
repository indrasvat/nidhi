# nidhi — Implementation Progress

> **STRICT RULE:** This file MUST be updated at the end of every coding session.

## Current Phase

Phase 1: Core (v0.1.0) — "First Light"

## Status: 🔴 Not Started

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

## Task List

| # | Task | Phase | Status | Depends On |
|---|---|---|---|---|
| 000 | Repository scaffold & tooling | Setup | TODO | — |
| 001 | Git runner & version detection | P1 | TODO | 000 |
| 002 | Config loading | P1 | TODO | 000 |
| 003 | Agni theme & icons | P1 | TODO | 000 |
| 004 | Stash parser & cache | P1 | TODO | 001 |
| 005 | Plugin interfaces & registry | P1 | TODO | 001, 002 |
| 006 | Core BubbleTea model & mode mgmt | P1 | TODO | 005, 004, 003 |
| 007 | Layout engine & chrome | P1 | TODO | 006, 003 |
| 008 | Stash row renderer | P1 | TODO | 003, 007 |
| 009 | Diff view & file tree | P1 | TODO | 003, 007 |
| 010 | LIST screen | P1 | TODO | 006, 008, 007 |
| 011 | PREVIEW screen | P1 | TODO | 010, 009, 004 |
| 012 | DETAIL screen | P1 | TODO | 010, 009, 007 |
| 013 | Stash CRUD operations | P1 | TODO | 001, 004 |
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
