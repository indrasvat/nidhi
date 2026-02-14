# निधि (nidhi) — Product Requirements Document

> **Purpose-built TUI for git stash mastery.**
> *"Your stashes are treasure. Treat them that way."*

| Field | Value |
|---|---|
| **Version** | `0.1.0-alpha` |
| **Author** | इन्द्रस्वत् (indrasvat) |
| **Date** | 2026-02-14 |
| **Status** | Draft |
| **License** | MIT |
| **Repository** | `github.com/indrasvat/nidhi` |

---

## Table of Contents

1. [Vision & Philosophy](#1-vision--philosophy)
2. [Problem Statement](#2-problem-statement)
3. [Target Audience](#3-target-audience)
4. [Technology Stack](#4-technology-stack)
5. [Git Stash — Complete Reference](#5-git-stash--complete-reference)
6. [Functional Requirements](#6-functional-requirements)
7. [Non-Functional Requirements](#7-non-functional-requirements)
8. [Architecture & Plugin System](#8-architecture--plugin-system)
9. [UI/UX Design System](#9-uiux-design-system)
10. [Screen Specifications](#10-screen-specifications)
11. [Keyboard Navigation](#11-keyboard-navigation)
12. [Configuration & Defaults](#12-configuration--defaults)
13. [BubbleTea/LipGloss Implementation Map](#13-bubbletealipgloss-implementation-map)
14. [Performance Budget](#14-performance-budget)
15. [Error Handling & Recovery](#15-error-handling--recovery)
16. [Testing Strategy](#16-testing-strategy)
17. [Build, Release & Distribution](#17-build-release--distribution)
18. [Milestones & Phasing](#18-milestones--phasing)
19. [Glossary](#19-glossary)
20. [External References](#20-external-references)

---

## 1. Vision & Philosophy

### 1.1 One-line Vision

nidhi turns `git stash` from a write-and-forget black hole into a visible, searchable, portable treasure vault.

### 1.2 Design Principles

| Principle | Meaning | Implementation |
|---|---|---|
| **"It just works"** | Zero config needed. Auto-detect everything. | Detect repo, branch, git version, terminal colors, Nerd Fonts on startup. Sane defaults for every setting. |
| **Progressive disclosure** | Simple by default, powerful on demand. | Three tiers: LIST (scan) → PREVIEW (inspect) → DETAIL (deep-dive). Features reveal themselves as you need them. |
| **Forgiving UX** | Every destructive action is recoverable. | Undo via reflog for drops. Confirm dialogs for bulk ops. Toast notifications with recovery keys. |
| **Blazing fast** | The TUI must never feel slower than the CLI. | <100ms startup. Cached stash data. Async git operations. No spinners for <200ms ops. |
| **Muscle memory first** | Keybinds should feel natural to vim/lazygit/fzf users. | `j/k` nav, `/` search, `tab` preview, `enter` detail. Single keys for 95% of actions. |
| **Small core, plugins for the rest** | The core is git stash CRUD + navigation. Everything else is a plugin. | Well-defined Go interfaces. Conflict preview, export/import, search — all plugins. |
| **Terminal-native beauty** | Not a web app in a terminal. A terminal app that belongs in a terminal. | Purpose-built Agni theme. Nerd Font icons. Respectful of terminal conventions. No pseudo-GUI chrome. |

### 1.3 What nidhi Is NOT

- Not a general Git TUI (that's lazygit/tig).
- Not a diff viewer (that's delta/difftastic).
- Not a Git GUI (that's GitKraken/Tower).
- Not trying to replace `git stash` — it wraps and enhances it.

---

## 2. Problem Statement

### 2.1 The Stash Black Hole

`git stash` is used by ~52% of developers regularly ([Stack Overflow Developer Survey](https://survey.stackoverflow.co/), [GitKraken Git Report](https://www.gitkraken.com/reports/best-developer-tools)), making it the most-used "advanced" Git command after the basics. Yet its UX is among the worst:

| Pain Point | Current UX | Impact |
|---|---|---|
| **Blind management** | `git stash list` shows cryptic "WIP on main: abc1234" messages. No diff preview. | Developers pop wrong stashes, lose context, avoid using stash at all. |
| **No conflict awareness** | `git stash apply` can fail with merge conflicts — no way to preview before applying. | Lost work, broken working tree, manual cleanup. |
| **Unrenaming** | Default messages are useless. No way to rename after creation. | Stashes become unidentifiable. |
| **Local-only** | Before Git 2.51, stashes couldn't be shared across machines. | WFH→office workflow broken. Pair programming friction. |
| **No search** | Can't search across stash diffs/content. Only `git stash list` with grep. | "I know I stashed that fix somewhere..." |
| **Destructive by default** | `git stash drop` and `git stash clear` are permanent. No undo. | Accidental data loss. |
| **No staleness detection** | Stashes accumulate silently. No "hey, this stash is 3 months old." | Stash lists grow unbounded, become useless. |

### 2.2 Why a Standalone TUI

lazygit and tig both have stash panels, but they treat stash as a secondary concern — a sidebar item among dozens. nidhi treats stash as the *only* concern, which means:

- Dedicated screen real estate for diff preview.
- Purpose-built conflict preview workflow.
- Deep search across all stash content.
- First-class export/import with remote sync.
- Stash-specific keybinds without collision.
- Rename, reorder, bulk operations.

---

## 3. Target Audience

### 3.1 Primary

- **Professional backend/platform engineers** who context-switch between branches frequently, maintain multiple WIP states, and work across machines.
- **Open source contributors** who juggle multiple PRs and need to stash/unstash work-in-progress changes.
- **Terminal-native developers** who live in the terminal and want every tool to feel cohesive (vim/tmux/lazygit users).

### 3.2 Secondary

- **Git learners** who find stash intimidating — nidhi's progressive disclosure makes it approachable.
- **Team leads** who want to share stashes across team members via export/import.

### 3.3 Minimum Viable Skill

- Knows what `git stash` does.
- Comfortable navigating with `j/k` or arrow keys.
- That's it. Everything else is discoverable.

---

## 4. Technology Stack

### 4.1 Core Dependencies

| Component | Version | Import Path | Notes |
|---|---|---|---|
| **Go** | ≥1.26 | — | Latest stable (released 2026-02-10). Self-referential generics, Green Tea GC, `new()` with expressions. ([Go 1.26 release notes](https://go.dev/doc/go1.26)) |
| **BubbleTea** | v2 (RC2+) | `charm.land/bubbletea/v2` | Cursed Renderer, `tea.View` struct, `KeyPressMsg`/`KeyReleaseMsg`, keyboard enhancements, Mode 2026 sync output, built-in color downsampling. ([Discussion #1374](https://github.com/charmbracelet/bubbletea/discussions/1374)) |
| **LipGloss** | v2 | `charm.land/lipgloss/v2` | Canvas/Layer compositing for modals, immutable styles, pure I/O (no fighting with BubbleTea), tree/table packages. ([Releases](https://github.com/charmbracelet/lipgloss/releases)) |
| **Bubbles** | v2 | `charm.land/bubbles/v2` | `viewport.Model`, `textinput.Model`, `key.Binding`, `help.Model`. ([Releases](https://github.com/charmbracelet/bubbles/releases)) |
| **Git** | ≥2.38 (core), ≥2.51 (export/import) | — | `merge-tree --write-tree` (2.38+), `stash export/import` (2.51+). Latest: 2.53.0 (2026-02-02). ([git-scm.com](https://git-scm.com/)) |

### 4.2 Supporting Libraries

| Library | Purpose |
|---|---|
| `github.com/sahilm/fuzzy` | Fuzzy matching for search |
| `github.com/pelletier/go-toml/v2` | Config file parsing |
| `github.com/adrg/xdg` | XDG base directory resolution |
| `github.com/muesli/termenv` | Terminal capability detection (used by LipGloss internally) |

### 4.3 Build & Dev

| Tool | Purpose |
|---|---|
| Go 1.26 toolchain | Build, test, vet, lint |
| `golangci-lint` | Linting |
| `goreleaser` | Cross-platform release builds |
| `task` (Taskfile) | Build automation |
| `gotestsum` | Test runner with pretty output |

### 4.4 Go Language Features Used

| Feature | Where Used |
|---|---|
| **Generics** | Plugin registry `Registry[T Plugin]`, generic event bus, typed result containers |
| **Self-referential generics** (Go 1.26) | Plugin constraint interfaces e.g. `type Renderer[R Renderer[R]]` |
| **`new()` with expressions** (Go 1.26) | Cleaner pointer initialization for config defaults |
| **Iterators** (`iter.Seq`, Go 1.23+) | Stash list iteration, search result streaming |
| **`sync.OnceValues`** (Go 1.21+) | Lazy singleton initialization for git version detection |
| **`slices`/`maps` packages** | Stash sorting, filtering, dedup |
| **Structured logging** (`log/slog`) | Debug logging to file |
| **Context-based cancellation** | Async git operations with timeout |
| **Embed** (`embed.FS`) | Default config, help text |

---

## 5. Git Stash — Complete Reference

All git stash subcommands nidhi wraps or invokes, with exact CLI signatures from the [git-stash(1) documentation](https://git-scm.com/docs/git-stash) for Git 2.53.0.

### 5.1 Core Stash Operations

```
git stash list [<log-options>]
git stash show [-u | --include-untracked | --only-untracked] [<diff-options>] [<stash>]
git stash drop [-q | --quiet] [<stash>]
git stash pop [--index] [-q | --quiet] [<stash>]
git stash apply [--index] [-q | --quiet] [<stash>]
git stash branch <branchname> [<stash>]
git stash push [-p | --patch] [-S | --staged] [-k | --[no-]keep-index]
               [-q | --quiet] [-u | --include-untracked] [-a | --all]
               [(-m | --message) <message>]
               [--pathspec-from-file=<file> [--pathspec-file-nul]]
               [--] [<pathspec>…]
git stash clear
git stash create [<message>]
git stash store [(-m | --message) <message>] [-q | --quiet] <commit>
```

**Docs:** [git-stash(1)](https://git-scm.com/docs/git-stash)

### 5.2 Export/Import (Git ≥ 2.51)

```
git stash export (--print | --to-ref <ref>) [<stash>…]
git stash import <commit>
```

- `--print`: Outputs object ID of commit chain to stdout (for scripts).
- `--to-ref <ref>`: Stores exported chain at a ref (e.g. `refs/stashes/$USER`).
- `import <commit>`: Appends imported stash entries to local stash list.

**Workflow for remote sync:**
```bash
# Export and push
git stash export --to-ref refs/stashes/$USER
git push --no-verify --force origin refs/stashes/$USER

# Fetch and import on another machine
git fetch origin refs/stashes/$USER:refs/stashes/$USER
git stash import refs/stashes/$USER
```

**Docs:** [git-stash(1) — export](https://git-scm.com/docs/git-stash), [Git 2.51 release notes](https://windowsforum.com/threads/git-2-51-cruft-aware-midx-path-walk-packing-portable-stashes-on-windows.378250/)

### 5.3 Plumbing Used

| Command | Purpose in nidhi | Since |
|---|---|---|
| `git merge-tree --write-tree HEAD <stash>` | Conflict preview dry-run. Exit 0 = clean, 1 = conflicts. Outputs tree OID + conflicted file info. | Git 2.38 |
| `git stash store -m "<msg>" <sha>` | Rename (drop old + store with new message, same SHA). Also used for undo (re-store dropped commit). | Git 1.7.7 |
| `git fsck --unreachable --no-reflogs` | Deep undo recovery — find orphaned stash commits after reflog expiry. | Git 1.5.0 |
| `git stash show -p [--include-untracked] <stash>` | Diff content for preview pane and search indexing. | Git 1.5.3 |
| `git stash list --format='...'` | Custom formatting for stash metadata (hash, date, message, branch). | Git 1.6.0 |
| `git rev-parse --git-dir` | Detect repo root. | Git 1.0 |
| `git version` | Detect git version for feature gating. | Git 1.0 |

**Docs:** [git-merge-tree(1)](https://git-scm.com/docs/git-merge-tree), [git-fsck(1)](https://git-scm.com/docs/git-fsck)

### 5.4 Feature Gating by Git Version

| Git Version | Features Available |
|---|---|
| < 2.38 | Core stash CRUD only. No conflict preview. Warning on startup. |
| ≥ 2.38 | + Conflict preview via `merge-tree --write-tree` |
| ≥ 2.51 | + Export/Import, remote sync |
| ≥ 2.53 | Full feature set (latest) |

nidhi detects git version on startup and gracefully disables unavailable features with informational badges in the UI.

---

## 6. Functional Requirements

### 6.1 Core Features (Built-in, not plugins)

These form the irreducible core of nidhi. They are always available and cannot be disabled.

#### FR-01: Stash List & Navigation

| ID | Requirement |
|---|---|
| FR-01.1 | Display all stashes in a scrollable list with: index, abbreviated SHA, message, branch name, relative age, file count, and total diff stat (+/-). |
| FR-01.2 | Cursor navigation via `j/k` or arrow keys. |
| FR-01.3 | Stashes older than the configurable staleness threshold (default: 14 days) display a `STALE` badge. |
| FR-01.4 | Auto-generated readable messages when stash message is the default "WIP on branch: sha msg" format. Replace with a summary like "3 files: +42/-17 in src/auth, pkg/db". |
| FR-01.5 | Progressive dimming: older stashes render with increasingly muted foreground colors. |
| FR-01.6 | Empty state: when no stashes exist, show a helpful message with the key to create a new stash. |
| FR-01.7 | Stash count displayed in the status bar at all times. |

#### FR-02: Stash CRUD

| ID | Requirement |
|---|---|
| FR-02.1 | **Apply** (`a`): Apply the selected stash to the working tree. Preserves the stash in the list. |
| FR-02.2 | **Pop** (`p`): Apply and remove the selected stash. |
| FR-02.3 | **Drop** (`d`): Remove the selected stash without applying. Show toast with undo key. |
| FR-02.4 | **New stash** (`n`): Open the New Stash screen with message input, scope toggles (staged/unstaged/untracked), and options (keep-index, patch mode). |
| FR-02.5 | **Branch from stash** (`b`): Create a new branch from the selected stash and check it out. Prompts for branch name. |
| FR-02.6 | **Clear all** (`D`): Drop all stashes. Requires double-confirmation. Stores all SHAs for bulk undo. |

#### FR-03: Mode Switching

| ID | Requirement |
|---|---|
| FR-03.1 | **LIST mode** (default): Compact list, no preview pane. Maximum information density. |
| FR-03.2 | **PREVIEW mode** (`Tab`): Toggle a bottom pane showing the diff of the selected stash. List compresses vertically. `h/l` cycles through changed files in the diff. |
| FR-03.3 | **DETAIL mode** (`Enter`): Full-screen view with file tree (left) + diff viewport (right). `Tab` switches focus between tree and diff. |
| FR-03.4 | `Esc` always returns to the previous mode. Never a dead end. |

### 6.2 Plugin Features

These are implemented as plugins conforming to the `Plugin` interface but ship as built-in plugins enabled by default.

#### FR-10: Conflict Preview (Plugin: `conflict`)

| ID | Requirement |
|---|---|
| FR-10.1 | When user presses `a` (apply) or `p` (pop), run `git merge-tree --write-tree HEAD <stash-commit>` before actually applying. |
| FR-10.2 | If exit code is 0 (no conflicts): proceed with apply/pop immediately. |
| FR-10.3 | If exit code is 1 (conflicts): show the Conflict Preview screen with per-file conflict status. |
| FR-10.4 | Per-file status indicators: ✓ clean (green), ⚡ conflict (yellow/amber), ? unknown (gray). |
| FR-10.5 | Inline preview of conflict zones for each conflicted file. |
| FR-10.6 | Options from conflict screen: "Apply anyway" / "Pop anyway" / "Branch first" / "Cancel". |
| FR-10.7 | Requires Git ≥ 2.38. If unavailable, skip conflict preview and apply directly (with a one-time info toast). |

**Git plumbing:** [git-merge-tree(1) — `--write-tree` mode](https://git-scm.com/docs/git-merge-tree)

#### FR-11: Deep Fuzzy Search (Plugin: `search`)

| ID | Requirement |
|---|---|
| FR-11.1 | `/` opens the search overlay with a text input and scope filter chips. |
| FR-11.2 | Scope filters: All (default), Messages, Files, Diffs, Branch. |
| FR-11.3 | Search is fuzzy (using `sahilm/fuzzy` or equivalent). |
| FR-11.4 | For diff search: index `git stash show -p` output for all stashes on startup (async, non-blocking). |
| FR-11.5 | Results show matched stash with highlighted match context (file:line for diff matches). |
| FR-11.6 | Live filtering: results update on every keystroke. |
| FR-11.7 | `Enter` on a result jumps to that stash in the list (and opens preview if the match was in a diff). |

#### FR-12: Export/Import & Remote Sync (Plugin: `sync`)

| ID | Requirement |
|---|---|
| FR-12.1 | `e` opens the Export screen. |
| FR-12.2 | Export screen: multi-select stashes with checkboxes, editable ref path (default: `refs/stashes/$USER`), remote selector (from `git remote`), live command preview. |
| FR-12.3 | Export executes: `git stash export --to-ref <ref> [stash indices...]` then `git push --no-verify --force <remote> <ref>`. |
| FR-12.4 | `i` opens the Import screen. |
| FR-12.5 | Import screen: fetch from remote, show incoming stashes with preview, confirm before importing. |
| FR-12.6 | Import executes: `git fetch <remote> <ref>:<ref>` then `git stash import <ref>`. |
| FR-12.7 | Requires Git ≥ 2.51. If unavailable, `e`/`i` show an informational message about upgrading Git. |

**Git plumbing:** [git-stash(1) — export/import](https://git-scm.com/docs/git-stash)

#### FR-13: Rename (Plugin: `rename`)

| ID | Requirement |
|---|---|
| FR-13.1 | `r` activates inline rename: the selected stash's message becomes an editable text input. |
| FR-13.2 | Previous message shown dimmed above/below for reference. |
| FR-13.3 | `Enter` saves, `Esc` cancels. |
| FR-13.4 | Implementation: `git stash drop stash@{n}` then `git stash store -m "<new message>" <sha>`. SHA is preserved. |
| FR-13.5 | If the stash is not at position 0, the operation preserves ordering by dropping and re-storing all stashes above it. |

#### FR-14: Undo & Recovery (Plugin: `undo`)

| ID | Requirement |
|---|---|
| FR-14.1 | After any drop operation, show a toast notification: "Dropped stash@{n}. Press `z` to undo (30s)". |
| FR-14.2 | `z` within 30 seconds: immediately re-stores the dropped stash via `git stash store -m "<msg>" <sha>`. |
| FR-14.3 | `z` after 30 seconds (or on a fresh launch): opens a Recovery Picker showing recently dropped stashes from `git fsck --unreachable --no-reflogs | grep commit`. |
| FR-14.4 | In-memory undo stack (ring buffer, 50 entries). Persisted across the current session only. |

#### FR-15: Stale Detection (Plugin: `stale`)

| ID | Requirement |
|---|---|
| FR-15.1 | Stashes older than the staleness threshold (configurable, default: 14 days) get a `STALE` badge in the list. |
| FR-15.2 | `fs` filter: show only stale stashes. |
| FR-15.3 | Bulk drop stale stashes with confirmation. |

#### FR-16: Reorder (Plugin: `reorder`)

| ID | Requirement |
|---|---|
| FR-16.1 | `Shift+J` / `Shift+K`: move the selected stash down/up in the list. |
| FR-16.2 | Implementation: drop + re-store sequence to change reflog ordering. |
| FR-16.3 | Visual feedback: the moved stash animates (highlight flash) to its new position. |

#### FR-17: Filter by Branch (Plugin: `filter`)

| ID | Requirement |
|---|---|
| FR-17.1 | `fb`: Toggle filter to show only stashes created on the current branch. |
| FR-17.2 | Active filter shown as a chip/badge in the status bar. |
| FR-17.3 | `fc`: Clear all filters. |

---

## 7. Non-Functional Requirements

### 7.1 Performance

| Metric | Target | How |
|---|---|---|
| **Cold startup to interactive** | < 100ms for repos with ≤ 20 stashes | Parse `git stash list` output synchronously (fast). Defer diff caching. |
| **Cold startup to interactive** | < 300ms for repos with ≤ 100 stashes | Same. Batch `git stash list` with `--format`. |
| **Keystroke-to-visual-update** | < 16ms (single frame at 60fps) | BubbleTea's Cursed Renderer. No layout recomputation on simple cursor moves. |
| **Diff preview load** | < 200ms for typical stashes (< 500 lines) | Lazy load. Cache in memory. Stream large diffs. |
| **Search index build** | Background, non-blocking. < 2s for 50 stashes. | Async `tea.Cmd`. Progressive results. |
| **Conflict preview** | < 500ms | `git merge-tree` is fast for typical repos. Show spinner only if > 200ms. |
| **Export/Import** | Network-bound. Show progress. | Async with `tea.Cmd`. Progress bar for push/fetch. |
| **Memory** | < 50MB RSS for typical usage (≤ 50 stashes) | Don't load all diffs into memory simultaneously. LRU cache for diff content. |

### 7.2 Reliability

| Requirement | Implementation |
|---|---|
| No data loss, ever. | Every destructive git operation is preceded by storing the SHA. Undo is always possible within the session. |
| Graceful degradation on old Git | Feature-gate by detected git version. Core CRUD works on any Git ≥ 2.0. |
| Clean terminal restore on crash | BubbleTea v2 handles alt-screen cleanup. Add `defer` for panic recovery. |
| No corruption of git state | All operations use standard `git stash` plumbing. No direct `.git/` manipulation. |
| Works in bare repos | Detect bare repo, show informational message, exit cleanly. |

### 7.3 Compatibility

| Dimension | Requirement |
|---|---|
| **OS** | macOS (primary), Linux, Windows (WSL2). Native Windows is best-effort. |
| **Terminal** | Ghostty, Kitty, iTerm2, Alacritty, WezTerm, Terminal.app, GNOME Terminal, Windows Terminal. |
| **Color** | TrueColor (primary), ANSI256 (fallback), 16-color (minimal), 1-bit (monochrome). BubbleTea v2's built-in color downsampling handles this automatically. |
| **Nerd Fonts** | Optional. Auto-detect. Fall back to ASCII glyphs if not available. |
| **Locale** | UTF-8. CJK character width handled by LipGloss. |
| **Git** | ≥ 2.0 (core), ≥ 2.38 (conflict preview), ≥ 2.51 (export/import). |
| **SSH** | Works over SSH (no mouse-dependent features required). |
| **Screen** | Min 80×24. Responsive layout adapts to available space. |

### 7.4 Accessibility

| Requirement | Implementation |
|---|---|
| Keyboard-only operation | No feature requires a mouse. Mouse support is additive. |
| High-contrast mode | Configurable. Theme overrides in config. |
| Screen reader hints | Semantic alt-text in status bar messages. |
| No flashing/animation that can't be disabled | `--no-animation` flag. Respects `REDUCE_MOTION` env var. |

### 7.5 Observability

| Requirement | Implementation |
|---|---|
| Debug logging | `--log-level debug` writes to `~/.local/state/nidhi/nidhi.log` via `log/slog`. |
| Git command tracing | `--trace-git` logs every git command invoked with args, exit code, duration. |
| Startup timing | `--debug` flag prints timing breakdown (git detection, stash parse, render). |

---

## 8. Architecture & Plugin System

### 8.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────┐
│                    main.go                          │
│              CLI parsing, config load               │
└───────────────────────┬─────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────┐
│                   Core Engine                       │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐           │
│  │ Git     │ │ Stash    │ │ Mode      │           │
│  │ Runner  │ │ Cache    │ │ Manager   │           │
│  └─────────┘ └──────────┘ └───────────┘           │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐           │
│  │ Event   │ │ Config   │ │ Theme     │           │
│  │ Bus     │ │ Store    │ │ Engine    │           │
│  └─────────┘ └──────────┘ └───────────┘           │
└───────────────────────┬─────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────┐
│                  Plugin Host                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐           │
│  │ conflict │ │ search   │ │ sync     │           │
│  │          │ │          │ │          │           │
│  ├──────────┤ ├──────────┤ ├──────────┤           │
│  │ rename   │ │ undo     │ │ stale    │           │
│  │          │ │          │ │          │           │
│  ├──────────┤ ├──────────┤ ├──────────┤           │
│  │ reorder  │ │ filter   │ │ (user)   │           │
│  └──────────┘ └──────────┘ └──────────┘           │
└───────────────────────┬─────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────┐
│                  UI Layer (BubbleTea v2)             │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐           │
│  │ Screen  │ │ Layout   │ │ Overlay   │           │
│  │ Router  │ │ Engine   │ │ Manager   │           │
│  └─────────┘ └──────────┘ └───────────┘           │
│  ┌─────────┐ ┌──────────┐ ┌───────────┐           │
│  │ Status  │ │ Footer   │ │ Toast     │           │
│  │ Bar     │ │ Bar      │ │ Manager   │           │
│  └─────────┘ └──────────┘ └───────────┘           │
└─────────────────────────────────────────────────────┘
```

### 8.2 Core Interfaces (Go)

```go
// Plugin is the base interface all plugins must implement.
type Plugin interface {
    // ID returns a unique identifier for the plugin.
    ID() string
    // Name returns the human-readable name.
    Name() string
    // Init is called once during startup with access to the core services.
    Init(ctx PluginContext) error
    // Destroy is called on shutdown for cleanup.
    Destroy() error
}

// KeyHandler plugins can register keybindings.
type KeyHandler interface {
    Plugin
    // KeyBindings returns the keybindings this plugin provides.
    // They are merged into the global keymap. Conflicts are resolved
    // by priority (core > plugin > user).
    KeyBindings() []KeyBinding
    // HandleKey is called when a registered key is pressed.
    HandleKey(key KeyEvent, state AppState) (AppState, tea.Cmd)
}

// ScreenProvider plugins can register new screen modes.
type ScreenProvider interface {
    Plugin
    // Screens returns screen definitions this plugin provides.
    Screens() []ScreenDef
    // Update handles messages when the screen is active.
    Update(msg tea.Msg, state AppState) (AppState, tea.Cmd)
    // View renders the screen content (between status bar and footer).
    View(state AppState, width, height int) string
}

// StashHook plugins can intercept stash operations.
type StashHook interface {
    Plugin
    // BeforeApply is called before a stash is applied. Return an error to abort.
    BeforeApply(stash Stash) (proceed bool, cmd tea.Cmd)
    // AfterDrop is called after a stash is dropped.
    AfterDrop(stash Stash, sha string) tea.Cmd
    // BeforePush is called before creating a new stash.
    BeforePush(opts PushOptions) (PushOptions, error)
}

// PluginContext provides plugins with access to core services.
type PluginContext struct {
    Git       GitRunner
    Cache     StashCache
    Config    ConfigStore
    Events    EventBus
    Logger    *slog.Logger
    GitVer    GitVersion
    Theme     Theme
}
```

### 8.3 Core Types

```go
// Stash represents a single stash entry.
type Stash struct {
    Index       int       // stash@{Index}
    SHA         string    // Full commit SHA
    ShortSHA    string    // Abbreviated SHA
    Message     string    // User message or auto-generated
    RawMessage  string    // Original message from git
    Branch      string    // Branch where stash was created
    Date        time.Time // Creation timestamp
    FileCount   int       // Number of files changed
    Insertions  int       // Lines added
    Deletions   int       // Lines deleted
    IsStale     bool      // Older than staleness threshold
    HasUntracked bool     // Includes untracked files
}

// AppState is the immutable snapshot of application state passed to plugins.
type AppState struct {
    Mode        Mode
    Stashes     []Stash
    Cursor      int
    Filters     []Filter
    SearchQuery string
    Width       int
    Height      int
    GitVersion  GitVersion
    RepoPath    string
    Branch      string
}

// GitRunner abstracts all git command execution.
type GitRunner interface {
    // Run executes a git command and returns stdout.
    Run(ctx context.Context, args ...string) (string, error)
    // RunLines executes and returns stdout split by newline.
    RunLines(ctx context.Context, args ...string) ([]string, error)
    // RunExitCode executes and returns the exit code (for merge-tree).
    RunExitCode(ctx context.Context, args ...string) (stdout string, exitCode int, err error)
}

// StashCache provides cached access to stash data.
type StashCache interface {
    // List returns all stashes. Cached until Invalidate().
    List(ctx context.Context) ([]Stash, error)
    // Diff returns the diff for a stash. Lazily loaded, LRU cached.
    Diff(ctx context.Context, index int) (string, error)
    // Invalidate clears the cache. Called after mutations.
    Invalidate()
}

// EventBus for decoupled communication between core and plugins.
type EventBus interface {
    Publish(event Event)
    Subscribe(eventType string, handler func(Event))
}
```

### 8.4 Module Structure

```
nidhi/
├── cmd/
│   └── nidhi/
│       └── main.go              # CLI entrypoint
├── internal/
│   ├── core/
│   │   ├── app.go               # Top-level BubbleTea model
│   │   ├── mode.go              # Mode enum and transitions
│   │   ├── state.go             # AppState, immutable snapshots
│   │   └── events.go            # Event types
│   ├── git/
│   │   ├── runner.go            # GitRunner implementation
│   │   ├── stash.go             # Stash parsing and operations
│   │   ├── cache.go             # StashCache with LRU
│   │   ├── version.go           # Git version detection and feature gating
│   │   └── mergetree.go         # merge-tree wrapper for conflict preview
│   ├── plugin/
│   │   ├── registry.go          # Plugin registry with generics
│   │   ├── context.go           # PluginContext factory
│   │   ├── interfaces.go        # All plugin interfaces
│   │   └── loader.go            # Plugin discovery and init
│   ├── plugins/                  # Built-in plugins
│   │   ├── conflict/
│   │   │   └── conflict.go      # Conflict preview plugin
│   │   ├── search/
│   │   │   ├── search.go        # Search plugin
│   │   │   └── index.go         # Search index builder
│   │   ├── sync/
│   │   │   └── sync.go          # Export/Import plugin
│   │   ├── rename/
│   │   │   └── rename.go        # Rename plugin
│   │   ├── undo/
│   │   │   └── undo.go          # Undo/Recovery plugin
│   │   ├── stale/
│   │   │   └── stale.go         # Stale detection plugin
│   │   ├── reorder/
│   │   │   └── reorder.go       # Reorder plugin
│   │   └── filter/
│   │       └── filter.go        # Branch filter plugin
│   ├── ui/
│   │   ├── theme/
│   │   │   ├── agni.go          # Agni theme definition
│   │   │   ├── theme.go         # Theme interface
│   │   │   └── adaptive.go      # Color downsampling helpers
│   │   ├── layout/
│   │   │   ├── layout.go        # Layout engine (hbar + content + fbar)
│   │   │   ├── split.go         # Split pane management
│   │   │   └── responsive.go    # Responsive breakpoints
│   │   ├── components/
│   │   │   ├── statusbar.go     # Status bar component
│   │   │   ├── footer.go        # Footer/keybind bar
│   │   │   ├── toast.go         # Toast notification with timer
│   │   │   ├── confirm.go       # Confirmation dialog (Canvas overlay)
│   │   │   ├── stashrow.go      # Single stash row renderer
│   │   │   ├── filetree.go      # File tree for detail view
│   │   │   ├── diffview.go      # Diff viewport with syntax highlighting
│   │   │   └── filterchip.go    # Filter chip toggle group
│   │   ├── screens/
│   │   │   ├── list.go          # LIST mode screen
│   │   │   ├── preview.go       # PREVIEW mode screen
│   │   │   ├── detail.go        # DETAIL mode screen
│   │   │   ├── newstash.go      # New stash creation screen
│   │   │   ├── export.go        # Export screen
│   │   │   ├── importscreen.go  # Import screen
│   │   │   └── help.go          # Help overlay
│   │   └── icons/
│   │       └── icons.go         # Nerd Font icons with ASCII fallback
│   └── config/
│       ├── config.go            # Config struct and loading
│       ├── defaults.go          # Default values
│       └── gitconfig.go         # Read from git config
├── go.mod
├── go.sum
├── Taskfile.yml
├── .goreleaser.yml
├── README.md
└── LICENSE
```

---

## 9. UI/UX Design System

### 9.1 Theme: Agni (अग्नि) — "Ember on Deep Ocean"

A custom dark theme designed to be distinctive from Catppuccin, Tokyo Night, Dracula, and Nord while maintaining excellent readability.

| Token | Hex | Usage |
|---|---|---|
| `bg.deep` | `#07090E` | Primary background |
| `bg.surface` | `#0F1219` | Panels, cards |
| `bg.elevated` | `#1A1F2B` | Active row, hover states |
| `bg.overlay` | `#1F2738` | Modal/dialog background |
| `fg.primary` | `#C8CCD4` | Primary text |
| `fg.secondary` | `#6B7280` | Secondary/muted text |
| `fg.dimmed` | `#3D4450` | Disabled, stale, ghost text |
| `accent.gold` | `#D4A050` | Primary accent — borders, active states |
| `accent.bright` | `#E8B85A` | Highlighted accent — cursor, focus ring |
| `semantic.aqua` | `#4EC9B0` | Clean/success — clean merge, applied stash |
| `semantic.coral` | `#F47067` | Danger/destructive — drop, clear, error |
| `semantic.green` | `#73D990` | Insertions, additions, success |
| `semantic.red` | `#FF5F6D` | Deletions, conflicts, critical |
| `semantic.yellow` | `#E5C07B` | Warnings, conflicts, stale badge |
| `semantic.blue` | `#61AFEF` | Info, links, branch names |
| `semantic.purple` | `#C678DD` | SHA hashes, special elements |
| `diff.added.fg` | `#73D990` | Diff: added line text |
| `diff.added.bg` | `#1A2E1A` | Diff: added line background |
| `diff.removed.fg` | `#FF5F6D` | Diff: removed line text |
| `diff.removed.bg` | `#2E1A1A` | Diff: removed line background |
| `diff.hunk` | `#61AFEF` | Diff: hunk header |

### 9.2 Typography & Icons

| Element | Nerd Font | ASCII Fallback |
|---|---|---|
| App mark | `󰘬` (nf-md-source_branch) | `≡` |
| Stash item | `` (nf-oct-archive) | `▪` |
| File: modified | `` (nf-oct-diff_modified) | `~` |
| File: added | `` (nf-oct-diff_added) | `+` |
| File: removed | `` (nf-oct-diff_removed) | `-` |
| File: renamed | `` (nf-oct-diff_renamed) | `→` |
| Conflict | `⚡` | `!` |
| Clean | `✓` | `√` |
| Stale badge | `` (nf-fa-clock_o) | `⌛` |
| Export | `` (nf-fa-upload) | `↑` |
| Import | `` (nf-fa-download) | `↓` |
| Search | `` (nf-fa-search) | `/` |
| Undo | `` (nf-fa-undo) | `↺` |
| Branch | `` (nf-oct-git_branch) | `⎇` |

Nerd Font detection: Check `$NERD_FONTS` env var, or attempt to render a known Nerd Font glyph and check width. Configurable override in config (`icons = "nerd"` / `"ascii"` / `"auto"`).

### 9.3 Layout Contract

Every screen in nidhi follows this layout:

```
┌─ Status Bar ──────────────────────────────────────┐
│ 󰘬 nidhi  main  12 stashes  ● synced  git 2.53 │
├───────────────────────────────────────────────────┤
│                                                   │
│              Content Area                         │
│         (varies by screen/mode)                   │
│                                                   │
├───────────────────────────────────────────────────┤
│ j/k nav  a apply  p pop  d drop  / search   LIST │
└───────────────────────────────────────────────────┘
```

- **Status bar** (1 line): App identity, repo info, stash count, sync status, git version.
- **Content area** (height - 2): Screen-specific content.
- **Footer bar** (1 line): Context-sensitive keybind hints + mode badge (right-aligned, color-coded).

Built with: `lipgloss.JoinVertical(lipgloss.Top, statusBar, content, footerBar)`

### 9.4 HTML Mockups

| Mockup | Screens Covered | File |
|---|---|---|
| **v1 — Initial** | Split-pane stash browser | `nidhi-mockup.html` |
| **v2 — Progressive Disclosure** | LIST (A), PREVIEW (B), DETAIL (C), Export, Confirm | `nidhi-v2-mockup.html` |
| **Full Design Spec** | All 10 screens with Agni theme, Nerd Font icons, keybind spec | `nidhi-full-mockup.html` |

---

## 10. Screen Specifications

### Screen 1: LIST (Default)

**Purpose:** Quick scan of all stashes. Maximum information density.

**Layout:**
```
┌─ Status Bar ──────────────────────────────────────┐
│ 󰘬 nidhi  main  5 stashes  git 2.53            │
├───────────────────────────────────────────────────┤
│ ▸  0  a3f7b2c  Fix auth token refresh     3h ago │
│     src/auth: +42/-17 · 3 files                   │
│                                                   │
│    1  e91c4d8  WIP: new dashboard layout   2d ago │
│     components/: +128/-34 · 7 files               │
│                                                   │
│    2  f28a1e5  Hotfix: rate limiter bug   12d ago │
│     pkg/ratelimit: +8/-3 · 1 file        ⌛ STALE │
│                                                   │
├───────────────────────────────────────────────────┤
│ j/k ↕  a apply  p pop  d drop  n new  / find LIST│
└───────────────────────────────────────────────────┘
```

**Row anatomy:**
- Cursor indicator (`▸` for selected, space otherwise)
- Stash index (dimmed for non-selected)
- Abbreviated SHA (purple `#C678DD`)
- Message (primary fg; auto-generated if default WIP message)
- Relative age (secondary fg, right-aligned)
- Second line: file scope summary, diff stat, file count
- Optional: `STALE` badge (yellow bg) for old stashes

**Responsive:** Below 100 cols, collapse to single-line rows (drop second line). Below 80 cols, truncate message.

**BubbleTea mapping:** Custom list (not `list.Model` — too opinionated). Array of `Stash` + cursor int. Render each row with `lipgloss.NewStyle()`. Selected row gets `bg.elevated` background.

### Screen 2: PREVIEW (Tab toggle)

**Purpose:** Inspect a stash's diff without leaving the list.

**Layout:**
```
┌─ Status Bar ──────────────────────────────────────┐
├───────────────────────────────────────────────────┤
│ ▸ 0  a3f7b2c  Fix auth token refresh       3h   │
│   1  e91c4d8  WIP: dashboard             2d ago  │
│   2  f28a1e5  Hotfix: rate limiter      12d ago  │
├───── src/auth/token.go (1/3) ─────────────────────┤
│ @@ -42,7 +42,12 @@ func RefreshToken(...)         │
│   if token.IsExpired() {                          │
│ -   return nil, ErrExpired                        │
│ +   newToken, err := provider.Refresh(token)      │
│ +   if err != nil {                               │
│ +     return nil, fmt.Errorf("refresh: %w", err)  │
│ +   }                                             │
├───────────────────────────────────────────────────┤
│ h/l ◀▶ files  Tab list  ^d/^u scroll       PREVIEW│
└───────────────────────────────────────────────────┘
```

**Behavior:**
- List compresses to ~40% of height (3-5 visible rows).
- Bottom pane: `viewport.Model` showing diff for selected stash.
- `h/l` cycles through changed files.
- File name + progress indicator shown in pane header.
- Cursor movement in list updates the preview pane.
- `Tab` toggles back to LIST mode (preview pane closes).

**BubbleTea mapping:** `lipgloss.JoinVertical(lipgloss.Top, compressedList, divider, viewport)`. Height split based on `WindowSizeMsg`.

### Screen 3: DETAIL (Full-screen)

**Purpose:** Deep dive into a single stash with file tree navigation.

**Layout:**
```
┌─ Status Bar ──────────────────────────────────────┐
├─── File Tree ─────┬── Diff ───────────────────────┤
│ ▼ staged (2)      │ @@ -42,7 +42,12 @@           │
│   ~ token.go      │   if token.IsExpired() {      │
│   + refresh.go    │ -   return nil, ErrExpired     │
│ ▼ working (1)     │ +   newToken, err := ...      │
│   ~ config.go     │ +   if err != nil {           │
│ ▶ untracked (0)   │ +     return nil, fmt.Err...  │
│                   │ +   }                         │
│                   │    return newToken, nil        │
├───────────────────┴───────────────────────────────┤
│ Tab focus  j/k nav  ^d/^u scroll  Esc back  DETAIL│
└───────────────────────────────────────────────────┘
```

**Behavior:**
- Left pane: File tree grouped by staged/working/untracked. ~25% width.
- Right pane: `viewport.Model` showing diff for selected file. ~75% width.
- `Tab` switches focus between tree and diff.
- `j/k` in tree: navigate files. In diff: scroll.
- `^d/^u` in diff: page scroll.
- File tree uses `lipgloss/tree` for rendering.
- `Esc` returns to previous mode (LIST or PREVIEW).

**BubbleTea mapping:** `lipgloss.JoinHorizontal(lipgloss.Top, treePanel, diffViewport)`. Focus tracked by `focusedPane` enum. Two child models that receive `KeyPressMsg` conditionally.

### Screen 4: CONFLICT PREVIEW

**Purpose:** Show predicted merge conflicts before applying/popping a stash.

**Triggered by:** `a` (apply) or `p` (pop) when `git merge-tree` detects conflicts.

**Layout:**
```
┌─ Status Bar ──────────────────────────────────────┐
├─── Conflict Preview: stash@{0} ───────────────────┤
│                                                   │
│  ✓ src/auth/token.go          clean apply         │
│  ⚡ src/auth/config.go         3 conflict zones    │
│  ✓ pkg/ratelimit/limiter.go   clean apply         │
│                                                   │
│ ─── src/auth/config.go conflict zone 1/3 ──────── │
│  <<<<<<< HEAD                                     │
│    maxRetries: 5                                  │
│  =======                                          │
│    maxRetries: 10                                 │
│  >>>>>>> stash                                    │
│                                                   │
├───────────────────────────────────────────────────┤
│ a apply anyway  b branch first  Esc cancel CONFLICT│
└───────────────────────────────────────────────────┘
```

**Behavior:**
- File list with per-file status: ✓ clean (green), ⚡ conflict (yellow).
- Below: inline preview of conflict zones for the selected conflicted file.
- Options: Apply anyway / Pop anyway / Branch first / Cancel.
- "Branch first" creates a new branch, applies stash there.

### Screen 5: SEARCH

**Purpose:** Find stashes by content across messages, filenames, and diff content.

**Layout:**
```
┌─ Status Bar ──────────────────────────────────────┐
├─── Search ────────────────────────────────────────┤
│  🔍 refresh tok█                                  │
│  [All] [Messages] [Files] [Diffs] [Branch]       │
│                                                   │
│  stash@{0} Fix auth token refresh                 │
│    src/auth/token.go:42  newToken := Refresh(tok  │
│                                                   │
│  stash@{3} Token rotation implementation          │
│    src/auth/rotate.go:15  func RotateToken(tok... │
│                                                   │
├───────────────────────────────────────────────────┤
│ Tab scope  Enter jump  Esc close            SEARCH│
└───────────────────────────────────────────────────┘
```

**Behavior:**
- Text input with live fuzzy filtering.
- Scope chips toggle which fields to search.
- Results show stash + match context with highlighted matches.
- `Enter` on result → jump to that stash in LIST and open PREVIEW.

### Screen 6: NEW STASH

**Purpose:** Create a new stash with an intentional message and scope.

**Layout:**
```
┌─ Status Bar ──────────────────────────────────────┐
├─── New Stash ─────────────────────────────────────┤
│                                                   │
│  Message: Fix auth token refresh logic█           │
│                                                   │
│  Scope:                                           │
│    [✓] Staged changes (3 files)                   │
│    [✓] Unstaged changes (1 file)                  │
│    [ ] Untracked files (2 files)                  │
│                                                   │
│  Options:                                         │
│    [✓] Keep index (don't unstage staged files)    │
│    [ ] Patch mode (select hunks)                  │
│    [ ] Auto-export after creation                 │
│                                                   │
├───────────────────────────────────────────────────┤
│ Tab next field  Enter create  Esc cancel       NEW│
└───────────────────────────────────────────────────┘
```

**Behavior:**
- Cursor starts in the message field (message-first design).
- Scope toggles with file counts from `git status`.
- Keep-index enabled by default (configurable).
- `Enter` creates the stash and returns to LIST.
- "Patch mode" opens git's interactive hunk picker (via `tea.Exec`).

### Screen 7: EXPORT

**Purpose:** Export stashes to a ref and push to remote.

**Layout:**
```
┌─ Status Bar ──────────────────────────────────────┐
├─── Export ────────────────────────────────────────┤
│                                                   │
│  Select stashes to export:                        │
│    [✓]  0  Fix auth token refresh                 │
│    [✓]  1  WIP: dashboard layout                  │
│    [ ]  2  Hotfix: rate limiter                   │
│                                                   │
│  Ref:    refs/stashes/indrasvat█                  │
│  Remote: origin (github.com/indrasvat/proj)       │
│                                                   │
│  Command preview:                                 │
│  $ git stash export --to-ref refs/stashes/...     │
│  $ git push --no-verify --force origin refs/...   │
│                                                   │
├───────────────────────────────────────────────────┤
│ Space toggle  Enter export  Esc cancel       EXPORT│
└───────────────────────────────────────────────────┘
```

### Screen 8: RENAME (Inline)

**Purpose:** Rename a stash's message without losing it.

Not a separate screen — inline editing in the LIST view. When `r` is pressed, the selected row's message becomes an editable `textinput.Model`. Previous message shown dimmed.

### Screen 9: DROP + UNDO (Toast)

**Purpose:** Recoverable stash deletion.

Not a separate screen — toast overlay on the current screen.

```
┌─────────────────────────────────────────────┐
│  Dropped stash@{0}. Press z to undo (27s)  │
└─────────────────────────────────────────────┘
```

Toast auto-dismisses after 30 seconds. `z` recovers immediately.

### Screen 10: HELP (Modal Overlay)

**Purpose:** Full keybind reference.

**Triggered by:** `?`

Rendered as a centered modal via LipGloss Canvas compositing (`lipgloss.NewCanvas` + `lipgloss.NewLayer`). Background content is dimmed. Organized by category: Navigation, Actions, Search/Filter, Export/Import. Mode badges shown.

`Esc` or `?` dismisses.

---

## 11. Keyboard Navigation

### 11.1 Three-Tier Hierarchy

| Tier | Pattern | Coverage | Examples |
|---|---|---|---|
| **Tier 1** | Single key | ~95% of daily use | `j` `k` `a` `p` `d` `n` `r` `e` `/` `?` `Tab` `Enter` `Esc` |
| **Tier 2** | Double key / Shift | ~4% (filters, reorder) | `fb` (filter branch) `fs` (filter stale) `fc` (clear filter) `J` `K` (reorder) |
| **Tier 3** | Ctrl combo | ~1% (viewport, select) | `^d` `^u` (page scroll) `^p` (patch mode) `^a` (select all in export) |

### 11.2 Complete Keymap

#### Global (all modes)

| Key | Action | Notes |
|---|---|---|
| `q` / `^c` | Quit | Confirm if unsaved export selection |
| `?` | Toggle help overlay | |
| `Esc` | Back / close overlay | Context-dependent |

#### LIST Mode

| Key | Action |
|---|---|
| `j` / `↓` | Cursor down |
| `k` / `↑` | Cursor up |
| `g` | Jump to first stash |
| `G` | Jump to last stash |
| `a` | Apply selected stash (triggers conflict preview if needed) |
| `p` | Pop selected stash (triggers conflict preview if needed) |
| `d` | Drop selected stash (shows undo toast) |
| `D` | Drop ALL stashes (double-confirm) |
| `n` | New stash screen |
| `r` | Inline rename |
| `b` | Branch from stash |
| `e` | Export screen |
| `i` | Import screen |
| `/` | Search |
| `Tab` | Toggle PREVIEW mode |
| `Enter` | Enter DETAIL mode |
| `z` | Undo last drop |
| `J` (Shift+j) | Move stash down |
| `K` (Shift+k) | Move stash up |
| `fb` | Filter: current branch only |
| `fs` | Filter: stale stashes only |
| `fc` | Clear all filters |

#### PREVIEW Mode

| Key | Action |
|---|---|
| `j` / `k` | Move cursor in list (updates preview) |
| `h` / `l` | Cycle files in preview diff |
| `Tab` | Toggle back to LIST |
| `Enter` | Enter DETAIL |
| `^d` / `^u` | Page scroll in preview |
| All LIST actions | Still available (apply, pop, drop, etc.) |

#### DETAIL Mode

| Key | Action |
|---|---|
| `Tab` | Switch focus: tree ↔ diff |
| `j` / `k` | Navigate (tree or diff, depending on focus) |
| `^d` / `^u` | Page scroll in diff |
| `Enter` | Expand/collapse tree node (when tree focused) |
| `Esc` | Back to LIST/PREVIEW |

#### SEARCH Mode

| Key | Action |
|---|---|
| Type | Filter query (live) |
| `Tab` | Cycle scope filters |
| `j` / `k` | Navigate results |
| `Enter` | Jump to selected result |
| `Esc` | Close search |

#### NEW / EXPORT / IMPORT Screens

| Key | Action |
|---|---|
| `Tab` | Next field |
| `Shift+Tab` | Previous field |
| `Space` | Toggle checkbox |
| `Enter` | Confirm/execute |
| `Esc` | Cancel and return |

### 11.3 Mouse Support (Additive)

| Action | Effect |
|---|---|
| Click on stash row | Select it |
| Scroll wheel | Scroll list / viewport |
| Click on scope chip | Toggle filter |
| Click on checkbox | Toggle selection |

Mouse is never required. All interactions are keyboard-first.

---

## 12. Configuration & Defaults

### 12.1 Configuration Sources (Priority Order)

1. **CLI flags** (highest)
2. **Environment variables** (`NIDHI_*`)
3. **Git config** (`nidhi.*` section)
4. **Config file** (`~/.config/nidhi/config.toml`)
5. **Built-in defaults** (lowest)

### 12.2 Config File Format

```toml
# ~/.config/nidhi/config.toml

[general]
# Icons: "auto" (detect Nerd Fonts), "nerd", "ascii"
icons = "auto"
# Stale threshold in days
stale_days = 14
# Keep index when creating new stashes
keep_index = true
# Auto-generate readable messages for default WIP messages
auto_message = true

[export]
# Default ref path for export. $USER is expanded.
ref = "refs/stashes/$USER"
# Default remote
remote = "origin"

[theme]
# Built-in theme: "agni" (default), or path to custom theme TOML
name = "agni"

[keys]
# Override default keybindings (advanced)
# apply = "a"
# pop = "p"

[performance]
# Max stashes to load diffs for on startup (rest are lazy)
preload_diffs = 10
# Search index: "eager" (on startup) or "lazy" (on first search)
search_index = "lazy"
# Max cached diffs in memory
diff_cache_size = 50

[log]
# Log level: "off", "error", "warn", "info", "debug"
level = "off"
# Log file path. Empty = default (~/.local/state/nidhi/nidhi.log)
file = ""
```

### 12.3 Git Config Integration

```ini
# .gitconfig or repo .git/config
[nidhi]
    stale-days = 14
    keep-index = true
    export-ref = refs/stashes/robin
    icons = nerd
```

Read via `git config --get nidhi.<key>`.

### 12.4 Environment Variables

| Variable | Maps to | Example |
|---|---|---|
| `NIDHI_STALE_DAYS` | `general.stale_days` | `NIDHI_STALE_DAYS=7` |
| `NIDHI_ICONS` | `general.icons` | `NIDHI_ICONS=ascii` |
| `NIDHI_LOG_LEVEL` | `log.level` | `NIDHI_LOG_LEVEL=debug` |
| `NIDHI_EXPORT_REF` | `export.ref` | `NIDHI_EXPORT_REF=refs/stashes/me` |
| `NIDHI_THEME` | `theme.name` | `NIDHI_THEME=agni` |
| `NO_COLOR` | Disables all color | Standard |
| `REDUCE_MOTION` | Disables animations | |
| `NERD_FONTS` | Force Nerd Font on/off | `NERD_FONTS=1` |

### 12.5 CLI Flags

```
nidhi [flags]

Flags:
  -h, --help              Show help
  -v, --version           Show version
      --log-level string  Log level (off, error, warn, info, debug)
      --trace-git         Log all git commands
      --no-color          Disable colors
      --no-animation      Disable animations
      --icons string      Icon set (auto, nerd, ascii)
  -C, --directory string  Run as if started in <path>
```

### 12.6 Sane Defaults Philosophy

Every default is chosen to match what a developer would configure if they had infinite time:

| Setting | Default | Rationale |
|---|---|---|
| Icons | `auto` | Detect and use the best available. |
| Stale threshold | 14 days | Two weeks is long enough to forget what a stash was for. |
| Keep index | `true` | Most stash operations should preserve staged work. |
| Auto-message | `true` | "WIP on main: abc1234 fix typo" is useless. "3 files: +42/-17 in src/auth" is useful. |
| Export ref | `refs/stashes/$USER` | Namespaced per user. Won't collide. |
| Search index | `lazy` | Don't pay the cost until the user searches. |
| Diff preload | 10 | Preload diffs for the 10 most recent stashes. Enough for immediate preview. |
| Theme | Agni | Purpose-built for nidhi. |

---

## 13. BubbleTea/LipGloss Implementation Map

### 13.1 BubbleTea v2 Features Used

| Feature | nidhi Usage | Notes |
|---|---|---|
| **`tea.View` struct** | Return `tea.View{AltScreen: true, Content: rendered}` from `View()`. | Replaces v1's `tea.EnterAltScreen` command. Declarative alt-screen management. |
| **`tea.KeyPressMsg`** | Primary keyboard input handler. `msg.Code` for special keys, `msg.Text` for printable. | Replaces v1's `tea.KeyMsg` with `Type`/`Runes`. |
| **`tea.KeyReleaseMsg`** | Not used in nidhi (no key-release interactions). | Available for future use. |
| **Keyboard enhancements** | Detect via `tea.KeyboardEnhancementsMsg`. Enable `shift+enter`, `ctrl+m` disambiguation in supporting terminals (Ghostty, Kitty, iTerm2, WezTerm). | Fallback: standard bindings only. |
| **`tea.WindowSizeMsg`** | Responsive layout recalculation. Stored in model. | Split between panels based on ratios. |
| **`tea.Cmd` / `tea.Batch`** | All async git operations. `tea.Cmd` for single ops, `tea.Batch` for parallel (e.g. diff preloading). | |
| **`tea.Exec`** | Shell out to `git stash push -p` for patch mode. Returns control to nidhi on completion. | |
| **Mode 2026 sync output** | Cursed Renderer uses Mode 2026 for flicker-free rendering. Automatic in v2. | No action needed — built into the renderer. |
| **`tea.WithFPS(60)`** | 60fps cap (default). | Sufficient for TUI. |
| **Color downsampling** | Automatic via `colorprofile`. Agni theme colors downsampled to ANSI256/16/1 as needed. | No manual fallback colors needed. |
| **`tea.Printf`** | Debug output during development. | |

### 13.2 LipGloss v2 Features Used

| Feature | nidhi Usage | Code Example |
|---|---|---|
| **`lipgloss.NewStyle()`** | All styling. Immutable in v2 — no accidental mutation. | `style := lipgloss.NewStyle().Foreground(theme.Gold).Bold(true)` |
| **`lipgloss.JoinVertical`** | Layout: status bar + content + footer. | `lipgloss.JoinVertical(lipgloss.Top, statusBar, content, footer)` |
| **`lipgloss.JoinHorizontal`** | Layout: file tree + diff in DETAIL mode. | `lipgloss.JoinHorizontal(lipgloss.Top, tree, diff)` |
| **`lipgloss.NewCanvas` + `NewLayer`** | Modal overlays (help, confirm dialog). Z-indexed compositing. | See §13.3 below. |
| **`lipgloss.Place`** | Centering modal content within the canvas. | `lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, modal)` |
| **`lipgloss/tree`** | File tree in DETAIL mode. | Built-in tree rendering with enumerators. |
| **`lipgloss/table`** | Help screen keybind table. | |
| **Border styles** | Panel borders, modal borders. | `lipgloss.RoundedBorder()`, custom `lipgloss.Border{}` for active panels. |

### 13.3 Canvas Compositing for Modals

```go
func (m model) renderWithOverlay(content, overlay string) string {
    // Dim the background content
    dimmed := lipgloss.NewStyle().
        Foreground(lipgloss.Color(m.theme.FgDimmed)).
        Render(content)

    // Create layers
    bgLayer := lipgloss.NewLayer(dimmed)
    fgLayer := lipgloss.NewLayer(overlay).
        X(m.width/2 - overlayWidth/2).  // Centered
        Y(m.height/2 - overlayHeight/2).
        Z(1) // Above background

    // Compose
    canvas := lipgloss.NewCanvas(bgLayer, fgLayer)
    return canvas.Render()
}
```

### 13.4 Bubbles v2 Components Used

| Component | nidhi Usage |
|---|---|
| `viewport.Model` | Diff preview (PREVIEW mode), diff panel (DETAIL mode), conflict zone preview. Handles `^d/^u` page scroll, scroll position tracking. |
| `textinput.Model` | Search query input, rename input, new stash message, branch name, export ref path. |
| `key.Binding` / `key.Map` | All keybindings. Organized per-mode. `help.Model` auto-generates help text from bindings. |
| `help.Model` | Footer keybind rendering. Auto-formats from `key.Map`. |

### 13.5 Custom Components (Not in Bubbles)

| Component | Why Custom | Approx. Lines |
|---|---|---|
| **Stash list** | `list.Model` is too opinionated (built-in filtering, status bar). We need bare-metal row rendering with conditional inline editing (rename). | ~200 |
| **Split pane** | No built-in split. Two child models, proportional width from `WindowSizeMsg`, focus tracking. | ~80 |
| **Toast** | Timed notification with auto-dismiss via `tea.Tick`. Undo action binding. | ~60 |
| **Filter chips** | Toggle group rendered as styled inline badges. | ~40 |
| **Stash row** | Conditional rendering: normal vs. inline-edit (rename) vs. selected vs. dimmed (stale). | ~100 |
| **Confirm dialog** | Modal with options. Canvas compositing over dimmed background. | ~80 |

---

## 14. Performance Budget

### 14.1 Startup Sequence

```
T+0ms     main() → parse flags, load config
T+5ms     git rev-parse --git-dir (detect repo)
T+8ms     git version (feature gate)
T+10ms    git branch --show-current
T+15ms    git stash list --format=... (parse all stashes)
T+30ms    Compute stash metadata (age, staleness, auto-messages)
T+35ms    Initialize BubbleTea program
T+50ms    First render (LIST mode)
T+50ms    ──── INTERACTIVE ────
T+100ms   Background: preload diffs for top 10 stashes
T+500ms   Background: build search index (if eager mode)
```

**Target: Interactive in < 100ms for ≤ 20 stashes.**

### 14.2 Operation Budgets

| Operation | Target | Strategy |
|---|---|---|
| Cursor move (j/k) | < 1ms | No git calls. Just change cursor int and re-render row styles. |
| Toggle preview (Tab) | < 50ms | Load diff from cache if available, else `git stash show -p` (~30-50ms). |
| Apply/Pop (no conflicts) | < 200ms | `git merge-tree` (~50ms) + `git stash apply/pop` (~100ms). |
| Conflict preview | < 500ms | `git merge-tree --write-tree` + parse output. |
| Search (keystroke) | < 50ms | In-memory fuzzy filter over pre-built index. |
| Export | Network-bound | Show progress indicator. Async. |
| Rename | < 100ms | `git stash drop` + `git stash store` (two fast ops). |
| Undo | < 50ms | `git stash store` (single op, SHA already in memory). |

### 14.3 Caching Strategy

| Data | Cache | Invalidation |
|---|---|---|
| Stash list | In-memory, full | After any mutation (apply, pop, drop, push, import, reorder, rename) |
| Stash diffs | LRU cache, configurable size (default 50) | After mutation affecting that stash |
| Search index | In-memory, built from cached diffs | Rebuild after mutation |
| Git version | `sync.OnceValues` (once per session) | Never (doesn't change during session) |
| Repo/branch | On startup | Refresh on `SIGWINCH` or focus (future) |

---

## 15. Error Handling & Recovery

### 15.1 Error Categories

| Category | Example | Handling |
|---|---|---|
| **User error** | Apply stash with conflicts | Show conflict preview. Never a dead end. |
| **Git error** | `git stash pop` fails | Show error toast with git's stderr. Don't crash. Stash is preserved (pop didn't complete). |
| **Environment error** | Not in a git repo | Show friendly message: "Not a git repository. Run nidhi from inside a repo." Exit 1. |
| **Feature unavailable** | `git stash export` on Git < 2.51 | Disable feature in UI. Show badge "Requires Git ≥ 2.51" on first attempt. |
| **Terminal error** | Terminal too small (< 80×24) | Show minimal message: "Terminal too small. Need at least 80×24." |
| **Panic** | Unexpected crash | `defer` recovery in main. Log stack trace. Restore terminal state. Print: "nidhi crashed. Log at ~/.local/state/nidhi/nidhi.log". |

### 15.2 Error Display Patterns

- **Toast** (non-fatal): Appears at bottom of screen. Auto-dismisses after 5s. Red border for errors, yellow for warnings.
- **Inline** (recoverable): Error text replaces the operation's expected output. E.g., rename failure shows error where the new message would have been.
- **Full-screen** (fatal): Clean message on a blank screen. Exit code. Log path.

### 15.3 Git Command Timeout

All git commands execute with `context.WithTimeout(ctx, 10*time.Second)`. Export/import operations use 60s timeout. If a command times out, show toast: "Git command timed out. Check repo status."

---

## 16. Testing Strategy

### 16.1 Test Layers

| Layer | Scope | Tools |
|---|---|---|
| **Unit** | Individual functions: stash parsing, message generation, theme color computation, config merging. | `go test`, table-driven tests. |
| **Integration** | Git operations against a real repo: apply, pop, drop, export, import, merge-tree. | Temp git repos created in `TestMain`. |
| **UI** | BubbleTea model tests: send messages, assert state transitions and view output. | `teatest` (BubbleTea's testing package). |
| **Snapshot** | View rendering: capture rendered strings and compare against golden files. | Custom snapshot testing with golden file updates via `--update` flag. |
| **E2E** | Full app interaction: start nidhi, send keystrokes, verify output. | `teatest` with `Send()` and `WaitFor()`. |

### 16.2 Git Test Fixtures

Every integration test creates a temporary git repo with a scripted history:

```go
func setupTestRepo(t *testing.T) string {
    dir := t.TempDir()
    run(t, dir, "git", "init")
    run(t, dir, "git", "commit", "--allow-empty", "-m", "init")
    // Create stashes with known content
    writeFile(t, dir, "file.go", "package main")
    run(t, dir, "git", "stash", "push", "-m", "test stash 1")
    // ...
    return dir
}
```

### 16.3 CI/CD

- Run on every push via GitHub Actions.
- Matrix: Go 1.25.x + Go 1.26.x × Linux + macOS.
- Lint: `golangci-lint run`.
- Test: `gotestsum -- -race -coverprofile=coverage.out ./...`.
- Coverage: Enforce > 70% on core and git packages.

---

## 17. Build, Release & Distribution

### 17.1 Build

```bash
# Development
task build           # go build -o bin/nidhi ./cmd/nidhi
task test            # gotestsum
task lint            # golangci-lint run

# Release
task release         # goreleaser release
```

### 17.2 Distribution Channels

| Channel | Command |
|---|---|
| **Homebrew** | `brew install indrasvat/tap/nidhi` |
| **Go install** | `go install github.com/indrasvat/nidhi/cmd/nidhi@latest` |
| **GitHub Releases** | Pre-built binaries for macOS (arm64, amd64), Linux (arm64, amd64), Windows (amd64). |
| **AUR** | `yay -S nidhi` (community-maintained) |
| **Nix** | Flake in repo |

### 17.3 Versioning

Semantic versioning. `v0.x.y` until the API (config format, plugin interface) stabilizes. Conventional commits for changelog generation.

---

## 18. Milestones & Phasing

### Phase 1: Core (v0.1.0) — "First Light"

**Goal:** Usable stash browser that's already better than `git stash list`.

| Feature | Done Criteria |
|---|---|
| LIST mode with navigation | Scrollable stash list, cursor, row rendering, progressive dimming |
| PREVIEW mode | Tab toggle, diff viewport, file cycling |
| DETAIL mode | File tree + diff split pane |
| Basic CRUD | Apply, pop, drop (no conflict preview, no undo) |
| Agni theme | Full theme applied |
| Responsive layout | Works at 80×24 through 200×60 |
| `--help`, `--version` | CLI basics |

### Phase 2: Safety Net (v0.2.0) — "No Fear"

| Feature | Done Criteria |
|---|---|
| Conflict preview plugin | `merge-tree` dry-run, conflict screen |
| Undo plugin | Toast, z-key recovery, reflog fallback |
| Rename plugin | Inline rename with drop+store |
| New stash screen | Message-first, scope toggles, keep-index |

### Phase 3: Power User (v0.3.0) — "Master of Stashes"

| Feature | Done Criteria |
|---|---|
| Deep search plugin | Fuzzy search across messages/files/diffs |
| Filter plugin | Branch filter, stale filter |
| Stale detection plugin | Badge, bulk drop |
| Reorder plugin | Shift+J/K move |

### Phase 4: Sync (v0.4.0) — "Across Machines"

| Feature | Done Criteria |
|---|---|
| Export plugin | Multi-select, ref path, remote selector, push |
| Import plugin | Fetch, preview, import |
| Git version gating | Graceful feature disable for old Git |

### Phase 5: Polish (v1.0.0) — "Release"

| Feature | Done Criteria |
|---|---|
| Help overlay | Full keybind reference |
| Config file support | TOML + git config + env vars |
| Mouse support | Click, scroll |
| Custom themes | Theme file format |
| Comprehensive tests | > 70% coverage |
| Documentation | README, man page, website |
| Homebrew tap | `brew install` works |

---

## 19. Glossary

| Term | Definition |
|---|---|
| **निधि (nidhi)** | Sanskrit: "treasure, treasure trove, hoard." The name reflects that stashes are valuable work-in-progress that deserves better management. |
| **अग्नि (Agni)** | Sanskrit: "fire." The name of nidhi's default dark theme — embers glowing on a deep ocean. |
| **Progressive disclosure** | UI design pattern where features are revealed gradually based on user need, reducing cognitive load. |
| **Conflict preview** | nidhi's feature to dry-run a stash apply via `git merge-tree` and show predicted conflicts before committing to the operation. |
| **Stale stash** | A stash older than the configured threshold (default 14 days) that likely represents forgotten work. |
| **Cursed Renderer** | BubbleTea v2's rendering engine based on ncurses algorithms, optimized for minimal terminal output. |
| **Canvas compositing** | LipGloss v2's feature for layering rendered content with Z-index, used for modal overlays. |
| **Mode 2026** | Terminal synchronized output protocol that prevents flicker during redraws. Supported by BubbleTea v2's Cursed Renderer. |

---

## 20. External References

### Git Documentation

| Resource | URL |
|---|---|
| git-stash(1) — full reference | https://git-scm.com/docs/git-stash |
| git-merge-tree(1) — `--write-tree` mode | https://git-scm.com/docs/git-merge-tree |
| git-fsck(1) — unreachable object recovery | https://git-scm.com/docs/git-fsck |
| Git 2.53.0 Release Notes | https://git-scm.com/docs/git |
| Git 2.51 — Stash export/import | https://about.gitlab.com/blog/whats-new-in-git-2-51-0/ |
| Git stash internals guide | https://alchemists.io/articles/git_stashes |

### Charmbracelet / TUI Framework

| Resource | URL |
|---|---|
| BubbleTea v2 — What's New | https://github.com/charmbracelet/bubbletea/discussions/1374 |
| BubbleTea v2 — Go package docs | https://pkg.go.dev/charm.land/bubbletea/v2 |
| LipGloss v2 — Go package docs | https://pkg.go.dev/charm.land/lipgloss/v2 |
| LipGloss — Canvas compositing PR | https://github.com/charmbracelet/lipgloss/pull/471 |
| Bubbles v2 — Releases | https://github.com/charmbracelet/bubbles/releases |

### Go

| Resource | URL |
|---|---|
| Go 1.26 Release Notes | https://go.dev/doc/go1.26 |
| Go 1.25 Release Notes | https://go.dev/doc/go1.25 |
| Go Release History | https://go.dev/doc/devel/release |

### Design Mockups (This Project)

| Mockup | Description |
|---|---|
| `nidhi-mockup.html` | v1 initial split-pane design |
| `nidhi-v2-mockup.html` | v2 progressive disclosure redesign |
| `nidhi-full-mockup.html` | Complete 10-screen spec with Agni theme, all states |

### Inspiration

| Tool | What We Learn From It |
|---|---|
| [lazygit](https://github.com/jesseduffield/lazygit) | Panel-based layout, keybind discoverability, vim-like navigation |
| [yazi](https://github.com/sxyazi/yazi) | Speed, progressive disclosure, preview pane pattern |
| [fzf](https://github.com/junegunn/fzf) | Live fuzzy filtering UX, minimal chrome |
| [delta](https://github.com/dandavison/delta) | Beautiful diff rendering, syntax highlighting |
| [tig](https://github.com/jonas/tig) | ncurses Git browser, split views |

---

*निधि — Your stashes are treasure. Treat them that way.*
