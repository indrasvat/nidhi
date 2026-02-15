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
