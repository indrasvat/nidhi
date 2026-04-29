# nidhi

> Purpose-built TUI for git stash mastery.
> *"Your stashes are treasure. Treat them that way."*

[![CI](https://github.com/indrasvat/nidhi/actions/workflows/ci.yml/badge.svg)](https://github.com/indrasvat/nidhi/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/indrasvat/nidhi/branch/main/graph/badge.svg)](https://codecov.io/gh/indrasvat/nidhi)
[![Go Version](https://img.shields.io/github/go-mod-go-version/indrasvat/nidhi)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/indrasvat/nidhi)](https://github.com/indrasvat/nidhi/releases)

<!-- Screenshot: LIST mode with Agni theme -->
<!-- ![nidhi LIST mode](docs/screenshots/list.png) -->

nidhi turns `git stash` from a write-and-forget black hole into a visible, searchable, portable treasure vault. Built with [BubbleTea v2](https://charm.land/bubbletea), [LipGloss v2](https://github.com/charmbracelet/lipgloss), and [Bubbles v2](https://charm.land/bubbles) on Go 1.26.

---

## Features

- **Three-tier progressive disclosure**: LIST (scan all stashes) -> PREVIEW (inspect diffs) -> DETAIL (deep-dive with file tree)
- **Conflict preview**: Dry-run `git merge-tree` before applying -- see conflicts before they happen
- **Deep fuzzy search**: Search across stash messages, filenames, and diff content
- **Export/Import**: Share stashes across machines via `git stash export/import` (Git 2.51+)
- **Repo format awareness**: Shows Git 2.54+ repository metadata such as object and reference formats
- **Inline rename**: Give stashes meaningful names without losing them
- **Undo drops**: Every drop is recoverable -- `z` to undo, reflog fallback for older drops
- **Stale detection**: Stashes older than 14 days get a `STALE` badge
- **Reorder**: `Shift+J/K` to move stashes up and down
- **Branch filter**: Show only stashes from the current branch
- **New stash creation**: Message-first design with scope toggles (staged/unstaged/untracked)
- **Agni theme**: Custom dark theme ("Ember on Deep Ocean") -- distinctive and readable
- **Nerd Font support**: Auto-detected with ASCII fallback
- **Responsive layout**: Works from 80x24 to ultrawide terminals

## Requirements

- **Git**: >= 2.22 (core features), >= 2.38 (conflict preview), >= 2.51 (export/import)
- **Terminal**: Any modern terminal (Ghostty, Kitty, iTerm2, Alacritty, WezTerm, Terminal.app, Windows Terminal)
- **OS**: macOS (primary), Linux, Windows (WSL2)

## Installation

### Homebrew (macOS/Linux)

```bash
brew install indrasvat/tap/nidhi
```

### Go install

```bash
go install github.com/indrasvat/nidhi/cmd/nidhi@latest
```

### GitHub Releases

Download pre-built binaries from the [Releases page](https://github.com/indrasvat/nidhi/releases):

- macOS: `nidhi_*_darwin_arm64.tar.gz` (Apple Silicon), `nidhi_*_darwin_amd64.tar.gz` (Intel)
- Linux: `nidhi_*_linux_amd64.tar.gz`, `nidhi_*_linux_arm64.tar.gz`
- Windows: `nidhi_*_windows_amd64.zip`

### Build from source

```bash
git clone https://github.com/indrasvat/nidhi.git
cd nidhi
make build
# Binary at ./bin/nidhi
```

## Quick Start

```bash
# Navigate to any git repo with stashes.
cd your-project

# Launch nidhi.
nidhi

# That's it. Use j/k to navigate, Tab to preview, Enter for details.
```

### Basic workflow

1. **Browse**: `j`/`k` to move through stashes, see messages, ages, and diff stats
2. **Preview**: `Tab` to toggle the diff preview pane
3. **Deep dive**: `Enter` for full-screen file tree + diff view
4. **Apply**: `a` to apply a stash (keeps it), `p` to pop (removes it)
5. **Search**: `/` to fuzzy-search across all stash content
6. **Create**: `n` to create a new stash with a meaningful message

## Keybinds

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor down / up |
| `g` / `G` | Jump to first / last stash |
| `Ctrl+d` / `Ctrl+u` | Page down / up |
| `Tab` | Toggle PREVIEW mode |
| `Enter` | Enter DETAIL mode |
| `Esc` | Go back |

### Actions

| Key | Action |
|-----|--------|
| `a` | Apply selected stash |
| `p` | Pop selected stash |
| `d` | Drop selected stash (with undo) |
| `D` | Drop ALL stashes (double-confirm) |
| `n` | Create new stash |
| `r` | Rename selected stash |
| `b` | Create branch from stash |
| `z` | Undo last drop |

### Search & Filter

| Key | Action |
|-----|--------|
| `/` | Open search |
| `fb` | Filter: current branch only |
| `fs` | Filter: stale stashes only |
| `fc` | Clear all filters |

### Power User

| Key | Action |
|-----|--------|
| `J` / `K` | Move stash down / up (reorder) |
| `e` | Export stashes |
| `i` | Import stashes |
| `?` | Help overlay |
| `q` | Quit |

Press `?` in-app for the complete keybind reference.

## Configuration

nidhi works with zero configuration. All settings have sensible defaults.

### Config file

`~/.config/nidhi/config.toml`:

```toml
[general]
icons = "auto"          # "auto", "nerd", "ascii"
stale_days = 14         # Stale threshold in days
keep_index = true       # Keep staged files when stashing
auto_message = true     # Auto-generate readable stash messages

[export]
ref = "refs/stashes/$USER"
remote = "origin"

[theme]
name = "agni"           # Built-in theme

[performance]
preload_diffs = 10      # Diffs to preload on startup
search_index = "lazy"   # "eager" or "lazy"
diff_cache_size = 50    # Max cached diffs

[log]
level = "off"           # "off", "error", "warn", "info", "debug"
```

### Environment variables

| Variable | Description |
|----------|-------------|
| `NIDHI_STALE_DAYS` | Stale threshold in days |
| `NIDHI_ICONS` | Icon set (`auto`, `nerd`, `ascii`) |
| `NIDHI_LOG_LEVEL` | Log level |
| `NIDHI_THEME` | Theme name |
| `NO_COLOR` | Disable all colors |
| `REDUCE_MOTION` | Disable animations |

### Git config

```ini
[nidhi]
    stale-days = 14
    keep-index = true
    icons = nerd
```

### CLI flags

```
nidhi [flags]

  -h, --help              Show help
  -v, --version           Show version
      --log-level string  Log level (off, error, warn, info, debug)
      --trace-git         Log all git commands
      --debug             Print startup timing and exit
      --no-color          Disable colors
      --no-animation      Disable animations
      --icons string      Icon set (auto, nerd, ascii)
  -C, --directory string  Run in <path>
```

Priority: CLI flags > environment > git config > config file > defaults.

## Theme: Agni

nidhi ships with **Agni** ("Ember on Deep Ocean"), a custom dark theme with warm gold accents on a deep navy background. Designed for:

- High contrast and readability in all lighting
- Semantic color coding: green for additions, red for deletions, yellow for warnings, blue for info
- Progressive dimming for stale stashes
- Automatic downsampling for 256-color and 16-color terminals

## Architecture

```
cmd/nidhi/main.go        CLI entrypoint, flag parsing
internal/core/            BubbleTea model, state, mode management
internal/git/             Git runner, stash operations, cache, version detection
internal/plugin/          Plugin system (registry, interfaces, loader)
internal/plugins/         Built-in plugins (conflict, search, sync, rename, undo, stale, reorder, filter)
internal/ui/theme/        Agni theme, theme interface
internal/ui/layout/       Layout engine, responsive breakpoints
internal/ui/components/   Reusable UI components
internal/ui/screens/      Screen implementations (LIST, PREVIEW, DETAIL, etc.)
internal/config/          Configuration loading
```

## Contributing

1. Read `CLAUDE.md` for coding conventions and architecture decisions
2. Use conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`)
3. Run `make check` before committing (lint + test)
4. One feature/fix per PR

```bash
# Development setup
make install-tools    # Install dev dependencies
make install-hooks    # Install git hooks (lefthook)
make build            # Build
make test             # Test
make lint             # Lint
make ci               # Full CI pipeline
```

## License

[MIT](LICENSE)

---

Built by [indrasvat](https://github.com/indrasvat). Inspired by the philosophy that developer tools should be beautiful, fast, and forgiving.
