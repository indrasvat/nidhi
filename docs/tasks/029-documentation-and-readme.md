# Task 029: Documentation and README

## Status: TODO

## Depends On
- 026 (Comprehensive E2E Tests) -- all features must be working for accurate documentation
- 003 (Agni Theme) -- theme details for README
- 021 (Help Overlay) -- keybind reference consistency

## Parallelizable With
- 028 (CI/CD and GitHub Actions) -- docs and CI can be written simultaneously

## Problem
The project has no user-facing documentation beyond the PRD (which is a development spec, not user docs). Users need: (1) a comprehensive README with installation instructions, screenshots, feature overview, and configuration reference, (2) terminal screenshots generated from the actual built binary to prove the TUI matches the design, and (3) a man page for `nidhi(1)`. Without documentation, users cannot discover, install, configure, or understand nidhi.

## PRD Reference
- Section 1.1 (One-line Vision) -- elevator pitch for README
- Section 1.2 (Design Principles) -- feature description source
- Section 6.1-6.2 (Functional Requirements) -- feature bullet points
- Section 9.1 (Agni Theme) -- theme description
- Section 9.2 (Typography & Icons) -- icon set details
- Section 11.2 (Complete Keymap) -- keybind reference table
- Section 12.1-12.6 (Configuration) -- config file format, env vars, CLI flags
- Section 17.2 (Distribution Channels) -- installation instructions
- Section 18 (Milestones) -- Phase 5 includes documentation

## Files to Create
- `README.md` -- comprehensive user-facing README (replaces placeholder)
- `docs/screenshots/` -- directory for terminal screenshots
- `docs/screenshots/generate.sh` -- script to generate screenshots using iterm2-driver
- `docs/man/nidhi.1` -- man page (roff format)
- `LICENSE` -- MIT license file

## Files to Modify
- `.gitignore` -- add `docs/screenshots/*.png` tracking note

## Execution Steps

### Step 1: Create screenshot generation script

```bash
#!/usr/bin/env bash
# docs/screenshots/generate.sh
# Generate terminal screenshots for README using iterm2-driver.
#
# Prerequisites:
#   - macOS with iTerm2
#   - iterm2-driver in PATH
#   - nidhi binary built (make build)
#
# Usage:
#   ./docs/screenshots/generate.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BIN="$PROJECT_ROOT/bin/nidhi"
SCREENSHOT_DIR="$SCRIPT_DIR"

# Verify prerequisites.
if ! command -v iterm2-driver &>/dev/null; then
    echo "ERROR: iterm2-driver not found. Install it to generate screenshots."
    exit 1
fi

if [ ! -f "$BIN" ]; then
    echo "Building nidhi..."
    make -C "$PROJECT_ROOT" build
fi

# Create a demo repo with diverse, realistic stashes.
DEMO_DIR=$(mktemp -d)
trap "rm -rf $DEMO_DIR" EXIT

cd "$DEMO_DIR"
git init
git config user.email "demo@nidhi.dev"
git config user.name "Nidhi Demo"

# Initial commit.
cat > README.md << 'INNEREOF'
# nidhi demo repo
A demo repository for generating nidhi screenshots.
INNEREOF
git add . && git commit -m "initial commit"

# Stash 4 (oldest): experimental cache layer on feature branch.
git checkout -b feat/cache
mkdir -p pkg/cache
cat > pkg/cache/lru.go << 'INNEREOF'
package cache

import "sync"

// LRU implements a least-recently-used cache with configurable eviction.
type LRU struct {
    mu       sync.RWMutex
    capacity int
    items    map[string]*entry
}

func New(capacity int) *LRU {
    return &LRU{capacity: capacity, items: make(map[string]*entry)}
}

func (c *LRU) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    e, ok := c.items[key]
    if !ok { return nil, false }
    return e.value, true
}
INNEREOF
git add . && git stash push -m "Experimental: new cache layer with LRU eviction"
git checkout main

# Stash 3 (stale): hotfix for rate limiter.
mkdir -p pkg/ratelimit
cat > pkg/ratelimit/limiter.go << 'INNEREOF'
package ratelimit

import "time"

// Limiter implements a token bucket rate limiter.
func NewLimiter(rate int, burst int) *Limiter {
    return &Limiter{rate: rate, burst: burst, last: time.Now()}
}
INNEREOF
git add . && git stash push -m "Hotfix: rate limiter token bucket overflow"

# Stash 2: auth token refresh (multiple files).
mkdir -p src/auth
cat > src/auth/token.go << 'INNEREOF'
package auth

import "fmt"

func RefreshToken(provider TokenProvider, token *Token) (*Token, error) {
    if token.IsExpired() {
        newToken, err := provider.Refresh(token)
        if err != nil {
            return nil, fmt.Errorf("refresh: %w", err)
        }
        return newToken, nil
    }
    return token, nil
}
INNEREOF
cat > src/auth/config.go << 'INNEREOF'
package auth

var (
    MaxRetries    = 5
    RetryInterval = "100ms"
    TokenTTL      = "24h"
)
INNEREOF
git add . && git stash push -m "Fix auth token refresh with retry logic"

# Stash 1: dashboard layout on feature branch.
git checkout -b feat/dashboard
mkdir -p components
cat > components/dashboard.go << 'INNEREOF'
package components

type Dashboard struct {
    Widgets []Widget
    Layout  GridLayout
    Theme   string
}

func NewDashboard(theme string) *Dashboard {
    return &Dashboard{Theme: theme, Layout: DefaultGrid()}
}
INNEREOF
cat > components/widget.go << 'INNEREOF'
package components

type Widget struct {
    ID     string
    Title  string
    Data   interface{}
    Width  int
    Height int
}
INNEREOF
cat > components/layout.go << 'INNEREOF'
package components

type GridLayout struct {
    Cols int
    Rows int
    Gap  int
}

func DefaultGrid() GridLayout {
    return GridLayout{Cols: 3, Rows: 2, Gap: 8}
}
INNEREOF
git add . && git stash push -m "WIP: new dashboard layout with grid system"
git checkout main

# Stash 0 (newest): API endpoint refactor.
mkdir -p api
cat > api/handlers.go << 'INNEREOF'
package api

import "net/http"

func RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("/api/v2/stashes", handleStashes)
    mux.HandleFunc("/api/v2/stashes/export", handleExport)
    mux.HandleFunc("/api/v2/health", handleHealth)
}
INNEREOF
git add . && git stash push -m "Refactor API endpoints to v2 namespace"

echo "Demo repo created at: $DEMO_DIR"
echo "Stash count: $(git stash list | wc -l)"
echo ""

# Generate screenshots.
echo "=== Capturing screenshots ==="

# Screen 1: LIST mode (default view)
echo "Capturing: LIST mode..."
iterm2-driver \
    --cols 120 --rows 40 \
    --delay 2000ms \
    --screenshot "$SCREENSHOT_DIR/list.png" \
    -- "$BIN" -C "$DEMO_DIR"
echo "  -> $SCREENSHOT_DIR/list.png"

# Screen 2: PREVIEW mode
echo "Capturing: PREVIEW mode..."
iterm2-driver \
    --cols 120 --rows 40 \
    --send-key Tab \
    --delay 1500ms \
    --screenshot "$SCREENSHOT_DIR/preview.png" \
    -- "$BIN" -C "$DEMO_DIR"
echo "  -> $SCREENSHOT_DIR/preview.png"

# Screen 3: DETAIL mode
echo "Capturing: DETAIL mode..."
iterm2-driver \
    --cols 120 --rows 40 \
    --send-key Tab \
    --send-key Enter \
    --delay 1500ms \
    --screenshot "$SCREENSHOT_DIR/detail.png" \
    -- "$BIN" -C "$DEMO_DIR"
echo "  -> $SCREENSHOT_DIR/detail.png"

# Screen 5: SEARCH mode
echo "Capturing: SEARCH mode..."
iterm2-driver \
    --cols 120 --rows 40 \
    --send-key "/" \
    --send-key "a" --send-key "u" --send-key "t" --send-key "h" \
    --delay 1500ms \
    --screenshot "$SCREENSHOT_DIR/search.png" \
    -- "$BIN" -C "$DEMO_DIR"
echo "  -> $SCREENSHOT_DIR/search.png"

# Screen 10: HELP overlay
echo "Capturing: HELP overlay..."
iterm2-driver \
    --cols 120 --rows 40 \
    --send-key "?" \
    --delay 1500ms \
    --screenshot "$SCREENSHOT_DIR/help.png" \
    -- "$BIN" -C "$DEMO_DIR"
echo "  -> $SCREENSHOT_DIR/help.png"

echo ""
echo "=== Done ==="
echo "Screenshots saved to: $SCREENSHOT_DIR/"
ls -la "$SCREENSHOT_DIR"/*.png 2>/dev/null || echo "(no screenshots captured -- check iterm2-driver)"
```

Make the script executable:

```bash
chmod +x docs/screenshots/generate.sh
```

### Step 2: Create `README.md`

```markdown
# निधि (nidhi)

> Purpose-built TUI for git stash mastery.
> *"Your stashes are treasure. Treat them that way."*

[![CI](https://github.com/indrasvat/nidhi/actions/workflows/ci.yml/badge.svg)](https://github.com/indrasvat/nidhi/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/indrasvat/nidhi/branch/main/graph/badge.svg)](https://codecov.io/gh/indrasvat/nidhi)
[![Go Version](https://img.shields.io/github/go-mod/go-version/indrasvat/nidhi)](go.mod)
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
```

### Step 3: Create `LICENSE`

```text
MIT License

Copyright (c) 2026 indrasvat

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

### Step 4: Create man page `docs/man/nidhi.1`

```roff
.\" nidhi(1) man page
.TH NIDHI 1 "February 2026" "nidhi v0.1.0" "User Commands"
.SH NAME
nidhi \- purpose-built TUI for git stash mastery
.SH SYNOPSIS
.B nidhi
[\fIOPTIONS\fR]
.SH DESCRIPTION
.B nidhi
is a terminal user interface for managing Git stashes. It provides a
three-tier progressive disclosure interface (LIST, PREVIEW, DETAIL) with
fuzzy search, conflict preview, export/import, undo, rename, reorder,
and branch filtering capabilities.
.PP
nidhi turns
.B git stash
from a write-and-forget black hole into a visible, searchable, portable
treasure vault.
.SH OPTIONS
.TP
.BR \-h ", " \-\-help
Show help message and exit.
.TP
.BR \-v ", " \-\-version
Show version information and exit.
.TP
.BI \-\-log\-level " LEVEL"
Set log level. Valid values: off, error, warn, info, debug. Default: off.
Logs are written to ~/.local/state/nidhi/nidhi.log.
.TP
.B \-\-trace\-git
Log all git commands with arguments, exit code, and duration.
.TP
.B \-\-debug
Print startup timing breakdown and exit. Useful for performance diagnostics.
.TP
.B \-\-no\-color
Disable all colors. Equivalent to setting NO_COLOR=1.
.TP
.B \-\-no\-animation
Disable all animations. Equivalent to setting REDUCE_MOTION=1.
.TP
.BI \-\-icons " SET"
Icon set to use. Valid values: auto (default), nerd (Nerd Fonts), ascii.
.TP
.BR \-C ", " \-\-directory " \fIPATH\fR"
Run as if nidhi was started in the given directory.
.SH KEYBOARD NAVIGATION
.SS Global
.TP
.B q, Ctrl+c
Quit nidhi.
.TP
.B ?
Toggle help overlay.
.TP
.B Esc
Go back / close overlay.
.SS LIST Mode
.TP
.B j/k
Move cursor down/up.
.TP
.B g/G
Jump to first/last stash.
.TP
.B Ctrl+d/Ctrl+u
Page down/up.
.TP
.B a
Apply selected stash.
.TP
.B p
Pop selected stash.
.TP
.B d
Drop selected stash (with undo toast).
.TP
.B n
Create new stash.
.TP
.B r
Rename selected stash.
.TP
.B /
Open search.
.TP
.B Tab
Toggle PREVIEW mode.
.TP
.B Enter
Enter DETAIL mode.
.TP
.B z
Undo last drop.
.TP
.B J/K
Move stash down/up (reorder).
.TP
.B e/i
Export/import stashes.
.TP
.B fb/fs/fc
Filter by branch / stale / clear filters.
.SS PREVIEW Mode
.TP
.B h/l
Cycle through changed files in the diff preview.
.TP
.B Tab
Toggle back to LIST mode.
.SS DETAIL Mode
.TP
.B Tab
Switch focus between file tree and diff viewport.
.SH CONFIGURATION
nidhi reads configuration from (in priority order):
.IP 1. 4
CLI flags
.IP 2. 4
Environment variables (NIDHI_*)
.IP 3. 4
Git config (nidhi.* section)
.IP 4. 4
Config file (~/.config/nidhi/config.toml)
.IP 5. 4
Built-in defaults
.SH ENVIRONMENT
.TP
.B NIDHI_STALE_DAYS
Stale threshold in days (default: 14).
.TP
.B NIDHI_ICONS
Icon set: auto, nerd, ascii.
.TP
.B NIDHI_LOG_LEVEL
Log level: off, error, warn, info, debug.
.TP
.B NO_COLOR
If set, disables all color output.
.TP
.B REDUCE_MOTION
If set, disables all animations.
.TP
.B NERD_FONTS
If set to 1, forces Nerd Font icons.
.SH FILES
.TP
.I ~/.config/nidhi/config.toml
User configuration file.
.TP
.I ~/.local/state/nidhi/nidhi.log
Debug log file (when --log-level is set).
.TP
.I ~/.local/state/nidhi/reorder-journal.json
Reorder operation journal for crash recovery.
.SH GIT VERSION REQUIREMENTS
.TP
.B Git >= 2.22
Core features (stash push, branch --show-current).
.TP
.B Git >= 2.38
Conflict preview via merge-tree --write-tree.
.TP
.B Git >= 2.51
Export/import stashes.
.SH EXIT STATUS
.TP
.B 0
Normal exit.
.TP
.B 1
Error (not a git repo, git not found, etc.).
.SH EXAMPLES
.TP
Launch nidhi in the current directory:
.B nidhi
.TP
Launch in a specific repository:
.B nidhi -C /path/to/repo
.TP
Launch with debug timing:
.B nidhi --debug
.TP
Launch with ASCII icons and no color:
.B nidhi --icons ascii --no-color
.SH BUGS
Report bugs at https://github.com/indrasvat/nidhi/issues.
.SH AUTHOR
indrasvat <https://github.com/indrasvat>
.SH SEE ALSO
.BR git-stash (1),
.BR git (1),
.BR lazygit (1)
```

### Step 5: Create screenshot directory

```bash
mkdir -p docs/screenshots
mkdir -p docs/man
```

### Step 6: Generate screenshots (if on macOS with iTerm2)

```bash
# Build the binary first.
make build

# Generate screenshots.
./docs/screenshots/generate.sh
```

### Step 7: Verify README renders correctly

```bash
# Check that all referenced files exist.
test -f README.md
test -f LICENSE
test -f docs/man/nidhi.1
test -f docs/screenshots/generate.sh
test -x docs/screenshots/generate.sh

# Verify man page renders.
man -l docs/man/nidhi.1
```

### Step 8: Verify man page formatting

```bash
# Check man page for formatting errors.
nroff -man docs/man/nidhi.1 | head -50

# Verify critical sections exist.
grep -q '.TH NIDHI' docs/man/nidhi.1
grep -q 'SYNOPSIS' docs/man/nidhi.1
grep -q 'DESCRIPTION' docs/man/nidhi.1
grep -q 'OPTIONS' docs/man/nidhi.1
grep -q 'KEYBOARD' docs/man/nidhi.1
grep -q 'CONFIGURATION' docs/man/nidhi.1
grep -q 'ENVIRONMENT' docs/man/nidhi.1
grep -q 'EXAMPLES' docs/man/nidhi.1
```

### Step 9: Run `make ci`

```bash
make ci
```

## Verification

### README
```bash
# README exists and is non-trivial.
test -f README.md
wc -l README.md
# Expected: > 200 lines

# README contains all required sections.
grep -q 'Installation' README.md
grep -q 'Homebrew' README.md
grep -q 'go install' README.md
grep -q 'Quick Start' README.md
grep -q 'Keybinds' README.md
grep -q 'Configuration' README.md
grep -q 'config.toml' README.md
grep -q 'Agni' README.md
grep -q 'Architecture' README.md
grep -q 'Contributing' README.md
grep -q 'License' README.md
grep -q 'MIT' README.md

# README has badges.
grep -q 'badge.svg' README.md
grep -q 'codecov' README.md
```

### LICENSE
```bash
# LICENSE exists.
test -f LICENSE
grep -q 'MIT License' LICENSE
grep -q 'indrasvat' LICENSE
grep -q '2026' LICENSE
```

### Man Page
```bash
# Man page exists and is valid roff.
test -f docs/man/nidhi.1
nroff -man docs/man/nidhi.1 > /dev/null 2>&1

# Man page has all critical sections.
grep -q 'SYNOPSIS' docs/man/nidhi.1
grep -q 'DESCRIPTION' docs/man/nidhi.1
grep -q 'OPTIONS' docs/man/nidhi.1
grep -q 'KEYBOARD' docs/man/nidhi.1
grep -q 'CONFIGURATION' docs/man/nidhi.1
grep -q 'ENVIRONMENT' docs/man/nidhi.1
```

### Screenshots
```bash
# Screenshot generation script exists and is executable.
test -f docs/screenshots/generate.sh
test -x docs/screenshots/generate.sh

# Screenshots directory exists.
test -d docs/screenshots

# If screenshots were generated, verify they are valid PNGs.
for f in docs/screenshots/*.png; do
  [ -f "$f" ] && file "$f" | grep -q PNG && echo "PASS: $f" || echo "SKIP: $f"
done
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `README.md` is comprehensive (> 200 lines) with: hero section, badges, one-line description, features, requirements, installation (Homebrew/go install/releases/source), quick start, keybind reference, configuration, theme description, architecture, contributing, license
2. README references CI badge, coverage badge, Go version badge, license badge, release badge
3. README installation section covers all channels from PRD Section 17.2
4. README keybind table matches PRD Section 11.2 complete keymap
5. README configuration section shows example TOML, env vars, git config, CLI flags with priority order
6. `LICENSE` is MIT with correct year (2026) and author (indrasvat)
7. `docs/man/nidhi.1` is valid roff format with NAME, SYNOPSIS, DESCRIPTION, OPTIONS, KEYBOARD NAVIGATION, CONFIGURATION, ENVIRONMENT, FILES, EXIT STATUS, EXAMPLES, AUTHOR, SEE ALSO
8. `docs/screenshots/generate.sh` is executable and creates demo repo with diverse stashes, captures LIST, PREVIEW, DETAIL, SEARCH, HELP screenshots via iterm2-driver
9. `docs/screenshots/` directory exists (screenshots generated on macOS with iTerm2)
10. README screenshot references are present (commented out until screenshots exist)
11. Man page renders correctly with `nroff -man`
12. `make ci` passes

## Commit
```
docs: add comprehensive README, man page, and screenshot generation

Write README.md with installation (Homebrew, go install, releases, source),
quick start, full keybind reference, configuration (TOML, env, git config,
CLI flags), Agni theme description, architecture overview, and contributing
guide. Add MIT LICENSE. Add docs/man/nidhi.1 man page with all sections.
Add docs/screenshots/generate.sh for iterm2-driver screenshot capture.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 1.1-1.2, 6.1-6.2, 9.1-9.2, 11.2, 12.1-12.6, 17.2, 18
4. Verify tasks 026 is DONE in `docs/PROGRESS.md`
5. Create `docs/screenshots/` and `docs/man/` directories
6. Write `docs/screenshots/generate.sh` and make executable
7. Write comprehensive `README.md`
8. Write `LICENSE` (MIT)
9. Write `docs/man/nidhi.1` man page
10. If on macOS with iTerm2: run screenshot generation script
11. Verify README sections, LICENSE content, man page rendering
12. Run `make ci`
13. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
14. Commit with the message above
