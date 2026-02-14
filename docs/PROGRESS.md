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
