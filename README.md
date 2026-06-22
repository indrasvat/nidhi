<h1 align="center">nidhi</h1>

<p align="center">
  <strong>Purpose-built TUI for <code>git stash</code> mastery</strong><br>
  <em>"Your stashes are treasure. Treat them that way."</em><br>
  <sub>nidhi · <em>nih-dhee</em> · Sanskrit for "treasure"</sub><br>
  <img src="https://img.shields.io/badge/nidhi-treasure-D4A24C?style=flat&labelColor=07090E" alt="Treasure">
</p>

<p align="center">
  <a href="https://github.com/indrasvat/nidhi/actions/workflows/ci.yml"><img src="https://github.com/indrasvat/nidhi/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/indrasvat/nidhi"><img src="https://codecov.io/gh/indrasvat/nidhi/branch/main/graph/badge.svg" alt="Coverage"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/indrasvat/nidhi/releases"><img src="https://img.shields.io/github/v/release/indrasvat/nidhi" alt="Release"></a>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#usage">Usage</a> •
  <a href="#tui-controls">TUI Controls</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#roadmap">Roadmap</a>
</p>

---

## Overview

nidhi turns `git stash` from a write-and-forget black hole into a visible, searchable, portable treasure vault. Every stash is rendered as a row with cursor navigation, diff preview, and inline rename. Drops are recoverable. Conflicts are previewed before they happen. Stashes can be exported across machines.

Built with [BubbleTea v2](https://charm.land/bubbletea), [LipGloss v2](https://github.com/charmbracelet/lipgloss), and [Bubbles v2](https://charm.land/bubbles) on Go 1.26.

**Modes** — nidhi is organised around progressive disclosure:

| Mode | Purpose | Reach |
|------|---------|-------|
| `LIST` | Scan all stashes — index, SHA, message, branch, age, diffstat | open by default |
| `PREVIEW` | 40/60 split — list above, diff below, file cycling | `Tab` |
| `DETAIL` | Full-screen file tree (left) + diff view (right) | `Enter` |
| `SEARCH` | Fuzzy search across messages, files, diff content, branches | `/` |
| `NEW` | Create a stash with message-first, scoped staging | `n` |
| `EXPORT` / `IMPORT` | Push / fetch stashes via `git stash export/import` (Git ≥2.51) | `e` / `i` |
| `CONFLICT` | Per-file conflict preview before applying (Git ≥2.38) | auto on apply |
| `HELP` | Centred modal with the full keymap | `?` |

## Features

### Browse and inspect
- **Three-tier progressive disclosure** — LIST → PREVIEW → DETAIL, every transition reversible with `Esc`.
- **Stash row renderer** — index, short SHA, message, branch, age, ±diffstat, file count; progressive dimming so old stashes recede.
- **Unified diff view** — line-number gutter, syntax-aware add/remove/context coloring, scrollable viewport.
- **File tree** — staged / working / untracked categories with status icons, expandable, mockup-matched colors.
- **File cycling** in PREVIEW (`h` / `l`) so you can walk a multi-file stash without leaving the split view.
- **Tree↔diff focus toggle** in DETAIL (`Tab`); `j`/`k` and `Ctrl+d`/`Ctrl+u` always operate on the focused pane.
- **Session pin markers** — `m` to pin a stash within the current session for visual tracking, no Git mutation.

### Mutations, safely
- **Apply / pop / drop** with cache invalidation, SHA capture for undo, and clean error toasts.
- **Conflict preview** — `merge-tree --write-tree` dry-run before any apply; per-file conflict screen on collision; untracked-file collision detection via `ls-tree stash^3`.
- **Undo drop** — `z` reverses the last drop. 50-entry LIFO ring buffer, 30-second toast, cross-session recovery via `git fsck --unreachable` for older drops.
- **Inline rename** — `r` renames a stash by drop+`git stash store`. Multi-step reorder for non-top stashes uses a crash-safe JSON journal under `~/.local/state/nidhi/`.
- **Reorder** — `Shift+J`/`Shift+K` moves the selected stash; transactional drop-all + re-store with journal-backed crash recovery.
- **New stash** — `n` opens a message-first form with scope toggles (staged/unstaged/untracked, live file counts), keep-index, and patch mode (opens the native PARTIAL picker — see below).
- **Partial stash** — `P` opens a visual hunk/line picker: scroll a live diff, toggle individual hunks (or drill into lines with `v`), watch a live `+X/−Y` tally, then stash exactly what you chose while the rest of the working tree stays put. No interactive `git stash push -p` prompt. Requires Git ≥2.35.
- **Branch from stash** — `b` to materialize a stash as a fresh branch via `git stash branch`.
- **Drop-all** — `D` clears every stash with double-confirmation; SHAs are captured for bulk undo.

### Find and filter
- **Deep fuzzy search** — `/` opens search across stash messages, file names, diff content, and branch names. Powered by [`sahilm/fuzzy`](https://github.com/sahilm/fuzzy) with character-level highlight.
- **Scope chips** — `Tab` cycles `[All] / Messages / Files / Diffs / Branch`.
- **Lazy index** — search index builds on first invocation; messages and branches are instant, files and diff lines stream in asynchronously.
- **Branch filter** — `f` to show only stashes from the current branch.
- **Stale filter** — `F` to show only stale stashes (default 14 days, configurable).
- **Filter composition** — branch + stale combine with AND logic; status bar chips show active filters.

### Sync across machines
- **Export** — `e` opens a multi-select export screen, target ref path with `$USER` expansion, remote selector, live `git stash export` + `git push --force` command preview. Requires Git ≥2.51.
- **Import** — `i` fetches a remote ref, previews the incoming stashes, and runs `git stash import`.

### Polish
- **Welcome screen** — first-launch ASCII NIDHI logo with feature cards. Dismiss with `Enter`.
- **Help overlay** — `?` from any mode renders the complete keymap as a centered modal with dimmed background (LipGloss Canvas compositing).
- **Mouse support** — click a row to select, scroll-wheel through the list, click a filter chip to toggle, click a checkbox in export.
- **Mode-aware footer** — color-coded mode badge, key hints elide gracefully on narrow terminals so the badge and `?` help always survive.
- **Agni theme** — "Ember on Deep Ocean" — warm gold accents on deep navy, automatic 256→16-color downsampling via [`charmbracelet/colorprofile`](https://github.com/charmbracelet/colorprofile).
- **Background fill** — `tea.View.BackgroundColor` paints empty terminal cells via OSC 11 — no two-tone bleed.
- **Responsive layout** — three breakpoints (80×24 / 120×40 / 200×60) with column collapse rules; PREVIEW falls back to single-pane on narrow widths.
- **Nerd Font auto-detection** — falls back to ASCII glyphs when `terminfo` reports no Nerd Font.
- **Structured logging** — `slog` JSON to `~/.local/state/nidhi/nidhi.log`, configurable per-level; `--trace-git` records every git invocation.
- **Debug timing** — `--debug` prints a startup timing breakdown and exits.

## Installation

### One-line install (macOS / Linux)

```bash
curl -sSfL https://raw.githubusercontent.com/indrasvat/nidhi/main/install.sh | sh
```

Detects platform, downloads the latest SemVer GitHub release, verifies the SHA-256 checksum, installs to `~/.local/bin`, clears the macOS Gatekeeper quarantine flag, verifies `nidhi --version`, and warns if `~/.local/bin` is not on `$PATH`. Pin a version with `--version v0.1.0` or change the install dir with `--dir /usr/local/bin`:

```bash
curl -sSfL https://raw.githubusercontent.com/indrasvat/nidhi/main/install.sh \
    | sh -s -- --version v0.1.0 --dir ~/.local/bin
```

### From Releases

Pre-built binaries for macOS Apple Silicon (`darwin/arm64`) and Linux (`linux/amd64`, `linux/arm64`) are at [Releases](https://github.com/indrasvat/nidhi/releases).

```bash
# Example: macOS Apple Silicon
curl -LO https://github.com/indrasvat/nidhi/releases/download/v0.1.0/nidhi_0.1.0_darwin_arm64.tar.gz
tar -xzf nidhi_0.1.0_darwin_arm64.tar.gz
chmod +x nidhi
sudo mv nidhi /usr/local/bin/
xattr -d com.apple.quarantine /usr/local/bin/nidhi  # macOS Gatekeeper
```

### Go install

```bash
go install github.com/indrasvat/nidhi/cmd/nidhi@latest
```

### From source

```bash
git clone https://github.com/indrasvat/nidhi.git
cd nidhi
make build
# Binary at ./bin/nidhi
```

### Requirements

- **Git ≥ 2.22** for core features.
- **Git ≥ 2.38** for conflict preview (`merge-tree --write-tree`).
- **Git ≥ 2.51** for export / import (`git stash export/import`).
- **Go 1.26+** if building from source.
- A terminal that supports truecolor for the full Agni palette (Ghostty, Kitty, iTerm2, Alacritty, WezTerm, Terminal.app, Windows Terminal). Anything older is auto-downsampled.

## Usage

### Quick Start

```bash
cd your-project
nidhi
```

That's it. `j`/`k` to move, `Tab` to preview, `Enter` for the deep view, `?` for help, `q` to quit.

```bash
# Run against a different repository.
nidhi -C ~/code/other-repo

# With debug logging.
nidhi --log-level debug --trace-git

# Print startup timing and exit.
nidhi --debug

# Disable colors / animations (CI / accessibility).
nidhi --no-color --no-animation
```

### Typical workflow

1. **Browse** — `j`/`k` (or arrows) walks the list, `g`/`G` jump to top/bottom.
2. **Preview** — `Tab` opens the diff split. `h`/`l` cycles files within the stash.
3. **Deep dive** — `Enter` for the full-screen tree + diff view. `Tab` toggles tree↔diff focus.
4. **Search** — `/` then type a query. `Tab` cycles scopes, `Ctrl+N`/`Ctrl+P` walks results, `Enter` jumps.
5. **Mutate** — `a` apply, `p` pop, `d` drop (then `z` to undo), `r` rename, `b` branch from stash.
6. **Create** — `n` opens the message-first new-stash form.
7. **Sync** — `e` export, `i` import (Git ≥2.51).

## CLI Reference

### Flags

```bash
nidhi [flags]
```

| Flag | Short | Description |
|------|-------|-------------|
| `--help` | `-h` | Show help |
| `--version` | `-v` | Show version + commit + build date |
| `--log-level <level>` | | `off`, `error`, `warn`, `info`, `debug` |
| `--trace-git` | | Log every git invocation with args, exit code, duration |
| `--debug` | | Print startup timing breakdown and exit |
| `--no-color` | | Disable all colors (also `NO_COLOR=1`) |
| `--no-animation` | | Disable animations (also `REDUCE_MOTION=1`) |
| `--icons <set>` | | `auto` (default), `nerd`, `ascii` |
| `--directory <path>` | `-C` | Run as if started in `<path>` |

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Clean exit (`q`, EOF) |
| `1` | Runtime error (logged to stderr) |

## TUI Controls

### Global

| Key | Action |
|-----|--------|
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |
| `Esc` | Back / close overlay |

### LIST mode

| Key | Action |
|-----|--------|
| `j` / `↓` | Cursor down |
| `k` / `↑` | Cursor up |
| `g` / `G` | Jump to first / last stash |
| `Ctrl+d` / `Ctrl+u` | Page down / up |
| `Tab` | Open PREVIEW |
| `Enter` | Open DETAIL |
| `a` | Apply stash (with conflict preview) |
| `p` | Pop stash (apply + drop) |
| `d` | Drop stash (undo with `z`) |
| `D` | Drop ALL stashes (double-confirm) |
| `n` | New stash |
| `P` | Partial stash — visual hunk/line picker (Git ≥2.35) |
| `r` | Rename stash (inline) |
| `b` | Create branch from stash |
| `m` | Pin / unpin (session-only marker) |
| `z` | Undo last drop |
| `J` / `K` | Move stash down / up (reorder) |
| `f` | Filter: current branch only |
| `F` | Filter: stale stashes only |
| `/` | Open search |
| `e` | Open export |
| `i` | Open import |

### PARTIAL mode (partial stash)

A visual hunk/line picker — stash exactly the changes you choose, without git's
interactive `y/n/q/a/d/s/e` prompt. Selected changes render in full color,
unselected ones dim, so the screen previews what the stash will contain.

| Key | Action |
|-----|--------|
| `j` / `k` (or arrows) | Move cursor (file/hunk in hunk-mode, lines in line-mode) |
| `space` | Toggle the focused file / hunk / line |
| `v` | Switch hunk ↔ line granularity |
| `a` | Toggle the whole file under the cursor |
| `A` | Toggle everything |
| `Enter` | Name & create the stash |
| `Esc` | Cancel (no changes made) |

### PREVIEW mode

| Key | Action |
|-----|--------|
| `j` / `k` (or arrows) | Cycle stashes |
| `h` / `l` | Cycle files within current stash |
| `Ctrl+d` / `Ctrl+u` | Scroll diff |
| `Tab` | Close (return to LIST) |
| `Enter` | Open DETAIL |
| `a` / `p` | Apply / pop |
| `m` | Pin / unpin |
| `?` | Help |

### DETAIL mode

| Key | Action |
|-----|--------|
| `j` / `k` (or arrows) | Move cursor in focused pane |
| `Tab` | Toggle tree ↔ diff focus |
| `↑` / `↓` | Scroll diff |
| `Ctrl+d` / `Ctrl+u` | Page scroll diff |
| `Enter` | Expand / collapse tree group |
| `a` / `p` | Apply / pop |
| `b` | Branch from stash |
| `r` | Rename |
| `Esc` | Back to previous mode |

### SEARCH mode

| Key | Action |
|-----|--------|
| (typing) | Live fuzzy filter |
| `Tab` | Cycle scope: `[All]` → `Messages` → `Files` → `Diffs` → `Branch` |
| `Ctrl+N` / `Ctrl+P` (or arrows) | Next / previous result |
| `Enter` | Jump to stash (LIST for messages/branch, PREVIEW for files/diffs) |
| `Esc` | Close search |

### NEW STASH

| Key | Action |
|-----|--------|
| (typing) | Edit message |
| `Tab` | Next field (message → scopes → options) |
| `Space` | Toggle scope / option |
| `Ctrl+P` | Patch mode (hands off to `git stash push -p`) |
| `Enter` | Create stash |
| `Esc` | Cancel |

### EXPORT / IMPORT

| Key | Action |
|-----|--------|
| `Space` | Toggle stash selection (export) |
| `a` | Select all (export) |
| `Tab` | Edit ref / cycle remote |
| `Enter` | Run export / import |
| `e` ↔ `i` | Switch between export and import |
| `Esc` | Back |

Press `?` in-app for the complete reference, or read the man page: `man nidhi`.

## Configuration

nidhi works with zero configuration. Override defaults in any of these places (highest priority first):

1. **CLI flags** (see [CLI Reference](#cli-reference))
2. **Environment variables**
3. **Git config** (`git config --global nidhi.<key> <value>`)
4. **Config file** (`~/.config/nidhi/config.toml`)
5. **Built-in defaults**

### Config file

```toml
# ~/.config/nidhi/config.toml

[general]
icons = "auto"          # "auto", "nerd", "ascii"
stale_days = 14         # Stale-badge threshold in days
keep_index = true       # Keep staged files when stashing (default for `n`)
auto_message = true     # Auto-generate readable stash messages

[export]
ref = "refs/stashes/$USER"   # Default ref path for export (supports $USER)
remote = "origin"            # Default remote for export/import

[theme]
name = "agni"           # Built-in theme

[performance]
preload_diffs = 10      # Diffs to preload on startup
search_index = "lazy"   # "eager" or "lazy"
diff_cache_size = 50    # Max cached diffs (LRU)

[log]
level = "off"           # "off", "error", "warn", "info", "debug"
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `NIDHI_STALE_DAYS` | Stale-badge threshold in days |
| `NIDHI_ICONS` | Icon set (`auto`, `nerd`, `ascii`) |
| `NIDHI_LOG_LEVEL` | Log level |
| `NIDHI_THEME` | Theme name |
| `NO_COLOR` | Presence disables all colors |
| `REDUCE_MOTION` | Presence disables animations |
| `NERD_FONTS` | `1`/`true` → nerd, `0`/`false` → ascii |

### Git config

```ini
[nidhi]
    stale-days = 14
    keep-index = true
    icons = nerd
```

### Logs

Structured JSON logs are written to `~/.local/state/nidhi/nidhi.log` (XDG state dir). Set `--log-level debug` or `--trace-git` to capture every git command with args, exit code, and duration.

## Theme: Agni

nidhi ships with **Agni** ("Ember on Deep Ocean"), a custom dark theme with warm gold accents (`#D4A24C`) on a deep navy background (`#07090E`). Designed for:

- High contrast and readability under any lighting.
- Semantic color coding — green additions, red deletions, blue info, yellow warnings, coral untracked.
- Progressive dimming for old stashes (square-root curve, capped at 60%).
- Mode-specific badge colors: LIST=gold, PREVIEW=aqua, DETAIL=blue, SEARCH=purple, NEW=green, EXPORT=orange.
- Automatic downsampling for 256-color and 16-color terminals via `charmbracelet/colorprofile`.

## Architecture

```
cmd/nidhi/main.go         CLI entrypoint, flag parsing, dependency injection
internal/core/            BubbleTea model, mode management, event bus, UI renderer interface
internal/git/             GitRunner, stash parser, LRU cache, version detection, merge-tree
internal/plugin/          Plugin interfaces, registry, context, loader
internal/plugins/
  conflict/               Merge-tree dry-run + conflict screen (Git ≥2.38)
  undo/                   Ring buffer + reflog recovery + 30s toast
  rename/                 Inline rename with journal-backed reorder
  search/                 Fuzzy search index + scope chips + result picker
  filter/                 Branch + stale filter composition
  stale/                  Staleness computation + bulk drop
  reorder/                Shift+J/K with transactional journal
  sync/                   Export / import + remote selector (Git ≥2.51)
internal/ui/
  theme/                  Agni theme + theme interface
  layout/                 Responsive breakpoints, split-pane engine, dimension helpers
  components/             Status bar, footer, toast, stash row, file tree, diff view, filter chip, confirm dialog
  screens/                LIST, PREVIEW, DETAIL, NEW, EXPORT, IMPORT, HELP
  mouse/                  Click + scroll mapping
  icons/                  Nerd font / ASCII icon resolution
internal/config/          TOML + git config + env vars + CLI flags + slog setup
internal/e2e/             Phase-grouped E2E tests against real git repos
internal/perf/            Startup, operation, memory, and visual benchmarks
```

The plugin host is the central seam — every non-core feature is a plugin implementing one or more of `KeyHandler`, `StashHook`, `ScreenProvider`, or `Renderer`. New behaviors can be added without touching `core/`. See `docs/PRD.md` §13 for the full plugin contract.

## Testing

```bash
make build          # Build → bin/nidhi
make test           # Race + coverage with gotestsum
make lint           # golangci-lint
make check          # lint + test
make e2e            # End-to-end tests against real git repos
make bench          # Performance benchmarks (startup, operation latency, memory)
make smoke-test     # 7-step release smoke test
```

Visual / interaction regression tests live under `.claude/automations/`:

```bash
uv run .claude/automations/comprehensive_tui_test.py    # 34 interaction tests
uv run .claude/automations/visual_quality_audit.py      # 70 layout + alignment + tearing checks
```

These drive a real iTerm2 session via the Python API, dismiss the welcome screen, and walk every mode. Screenshots land in `.claude/automations/screenshots/` (gitignored).

## Roadmap

Potential future enhancements:

- **Custom themes** — User-loadable theme files (currently only Agni ships).
- **Stash hooks** — Pre/post apply hooks for project-specific automation.
- **Inline word-level diff** — Highlight changed tokens within a line, not just whole-line add/remove.
- **Time-travel view** — Browse a stash across multiple commits.
- **Squash multiple stashes** — Combine related stashes into one.
- **Conflict resolution** — Open the conflict screen as an interactive merge driver, not just a preview.
- **Plugin marketplace** — Drop-in third-party plugins discovered at startup.

## Contributing

1. Read [`CLAUDE.md`](CLAUDE.md) for coding conventions, architecture decisions, and the running learnings log.
2. Use Conventional Commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`).
3. Run `make check` before committing — lefthook is wired up for pre-commit.
4. One feature / fix per PR. Reference the PRD section if applicable.

```bash
make install-tools    # Install dev dependencies (gotestsum, golangci-lint, …)
make install-hooks    # Install lefthook git hooks
make ci               # Full CI pipeline locally
```

## License

[MIT](LICENSE)

---

Built by [indrasvat](https://github.com/indrasvat). Inspired by the philosophy that developer tools should be beautiful, fast, and forgiving.
