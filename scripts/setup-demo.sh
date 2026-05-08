#!/usr/bin/env bash
# scripts/setup-demo.sh
# Creates a demo git repo at /tmp/nidhi-demo with 6 diverse stashes
# that showcase all nidhi features. Then launches nidhi.
#
# Usage:
#   ./scripts/setup-demo.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$PROJECT_ROOT/bin/nidhi"
DEMO=/tmp/nidhi-demo

# Build if needed
if [ ! -f "$BIN" ]; then
    echo "Building nidhi..."
    make -C "$PROJECT_ROOT" build
fi

# Clean slate
rm -rf "$DEMO"
mkdir -p "$DEMO" && cd "$DEMO"

git init
git config user.email "demo@nidhi.dev"
git config user.name "Demo User"

# ── Initial commit ──
cat > main.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello, nidhi demo!")
}
EOF
cat > README.md << 'EOF'
# Demo Project
A sample project for trying out nidhi.
EOF
mkdir -p pkg/auth pkg/cache
cat > pkg/auth/auth.go << 'EOF'
package auth

import "fmt"

func Login(user, pass string) error {
    if user == "" {
        return fmt.Errorf("empty username")
    }
    return nil
}
EOF
git add . && git commit -m "initial commit"

# ── Stash 5 (oldest): LRU cache on feature branch ──
git checkout -b feat/cache
cat > pkg/cache/lru.go << 'EOF'
package cache

import "sync"

type LRU struct {
    mu       sync.RWMutex
    capacity int
    items    map[string]*entry
}

type entry struct {
    key   string
    value interface{}
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

func (c *LRU) Set(key string, val interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = &entry{key: key, value: val}
}
EOF
git add . && git stash push -m "Experimental: LRU cache with mutex"
git checkout main

# ── Stash 4: auth hotfix ──
cat > pkg/auth/auth.go << 'EOF'
package auth

import (
    "fmt"
    "strings"
)

func Login(user, pass string) error {
    if user == "" {
        return fmt.Errorf("empty username")
    }
    if len(pass) < 8 {
        return fmt.Errorf("password too short")
    }
    return nil
}

func Sanitize(input string) string {
    return strings.TrimSpace(input)
}
EOF
git add . && git stash push -m "Hotfix: add password validation and input sanitize"

# ── Stash 3: multi-file API refactor ──
mkdir -p api
cat > api/handlers.go << 'EOF'
package api

import "net/http"

func RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("/api/v2/users", handleUsers)
    mux.HandleFunc("/api/v2/health", handleHealth)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte(`{"users":[]}`))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte(`{"status":"ok"}`))
}
EOF
cat > api/middleware.go << 'EOF'
package api

import (
    "log"
    "net/http"
    "time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
    })
}
EOF
git add . && git stash push -m "Refactor API to v2 with logging middleware"

# ── Stash 2: dashboard WIP on feature branch ──
git checkout -b feat/dashboard
mkdir -p ui
cat > ui/dashboard.go << 'EOF'
package ui

type Dashboard struct {
    Widgets []Widget
    Theme   string
    Cols    int
}

type Widget struct {
    ID    string
    Title string
    Width int
}

func NewDashboard() *Dashboard {
    return &Dashboard{Theme: "dark", Cols: 3}
}
EOF
cat > ui/render.go << 'EOF'
package ui

import "fmt"

func (d *Dashboard) Render() string {
    out := fmt.Sprintf("Dashboard (%s theme, %d cols)\n", d.Theme, d.Cols)
    for _, w := range d.Widgets {
        out += fmt.Sprintf("  [%s] %s (w=%d)\n", w.ID, w.Title, w.Width)
    }
    return out
}
EOF
git add . && git stash push -m "WIP: dashboard layout with grid renderer"
git checkout main

# ── Stash 1: config loader (will CONFLICT with current main.go) ──
cat > main.go << 'EOF'
package main

import (
    "fmt"
    "os"
)

func main() {
    cfg := os.Getenv("APP_CONFIG")
    if cfg == "" {
        cfg = "default"
    }
    fmt.Printf("Running with config: %s\n", cfg)
}
EOF
git add . && git stash push -m "Add config loader with env override"

# ── Stash 0 (newest): Redis cache + TODO ──
echo "TODO: write tests" > TODO.md
mkdir -p pkg/cache
cat > pkg/cache/redis.go << 'EOF'
package cache

type RedisCache struct {
    Addr string
    DB   int
}

func NewRedis(addr string) *RedisCache {
    return &RedisCache{Addr: addr, DB: 0}
}
EOF
git add TODO.md pkg/cache/redis.go
git stash push -m "Add Redis cache adapter and TODO list"

# ── Now diverge main.go so stash 1 will show conflicts ──
cat > main.go << 'EOF'
package main

import (
    "fmt"
    "flag"
)

func main() {
    debug := flag.Bool("debug", false, "enable debug")
    flag.Parse()
    if *debug {
        fmt.Println("debug mode on")
    }
    fmt.Println("Hello, nidhi demo!")
}
EOF
git add main.go && git commit -m "switch main.go to use flag package"

echo ""
echo "========================================"
echo " Demo repo ready at: $DEMO"
echo " Stashes: $(git stash list | wc -l | tr -d ' ')"
echo "========================================"
git stash list
echo ""
echo "Feature highlights to try:"
echo "  - j/k       navigate stashes"
echo "  - Tab       preview diffs"
echo "  - Enter     detail view with file tree"
echo "  - /         search (try 'auth' or 'cache')"
echo "  - r         rename a stash"
echo "  - J/K       reorder stashes"
echo "  - a on stash 1 → conflict preview!"
echo "  - d then z  drop then undo"
echo "  - n         create a new stash"
echo "  - ?         help overlay"
echo ""
echo "Launching nidhi..."
exec "$BIN" -C "$DEMO"
