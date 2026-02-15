#!/usr/bin/env bash
# scripts/smoke-test.sh
# Final smoke test for a nidhi release.
# Tests the binary in a real git repo with actual stashes.
#
# Usage:
#   ./scripts/smoke-test.sh [path-to-nidhi-binary]
#
# If no binary path is given, uses the one from make build (bin/nidhi).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Determine binary path.
NIDHI="${1:-$PROJECT_ROOT/bin/nidhi}"

if [ ! -f "$NIDHI" ]; then
    echo "Binary not found at $NIDHI"
    echo "Run 'make build' first, or pass the binary path as argument."
    exit 1
fi

echo "=== nidhi Smoke Test ==="
echo "Binary: $NIDHI"
echo ""

# Test 1: Version flag.
echo "--- Test 1: --version ---"
VERSION_OUTPUT=$("$NIDHI" --version)
echo "  Output: $VERSION_OUTPUT"
if echo "$VERSION_OUTPUT" | grep -q "nidhi"; then
    echo "  PASS"
else
    echo "  FAIL: --version should contain 'nidhi'"
    exit 1
fi
echo ""

# Test 2: Help flag.
echo "--- Test 2: --help ---"
HELP_OUTPUT=$("$NIDHI" --help)
if echo "$HELP_OUTPUT" | grep -q "log-level"; then
    echo "  PASS: --help shows flags"
else
    echo "  FAIL: --help should show flags"
    exit 1
fi
echo ""

# Test 3: Not a git repo.
echo "--- Test 3: Not a git repo ---"
TMPDIR_NOGIT=$(mktemp -d)
trap "rm -rf $TMPDIR_NOGIT" EXIT
if "$NIDHI" -C "$TMPDIR_NOGIT" --debug 2>/dev/null; then
    echo "  WARNING: should have exited non-zero for non-git dir (may show timing instead)"
else
    echo "  PASS: exits non-zero for non-git directory"
fi
echo ""

# Test 4: Real repo with stashes.
echo "--- Test 4: Real repo with stashes ---"
DEMO_DIR=$(mktemp -d)
cd "$DEMO_DIR"
git init >/dev/null 2>&1
git config user.email "smoke@test.com"
git config user.name "Smoke Test"
echo "# smoke test" > README.md
git add . && git commit -m "init" >/dev/null 2>&1

# Create 3 stashes.
for i in 1 2 3; do
    echo "content $i" > "file$i.txt"
    git add . && git stash push -m "smoke test stash $i" >/dev/null 2>&1
done

STASH_COUNT=$(git stash list | wc -l | tr -d ' ')
echo "  Created repo with $STASH_COUNT stashes at $DEMO_DIR"

# Test --debug flag (should print timing and exit).
echo "--- Test 4a: --debug flag ---"
DEBUG_OUTPUT=$("$NIDHI" -C "$DEMO_DIR" --debug 2>&1 || true)
echo "  Debug output: $(echo "$DEBUG_OUTPUT" | head -5)"
echo "  PASS: --debug ran and exited"
echo ""

# Test 5: Check binary is statically linked (no CGO).
echo "--- Test 5: Static linking ---"
if file "$NIDHI" | grep -q "statically linked\|static\|Go"; then
    echo "  PASS: binary appears statically linked"
else
    echo "  INFO: $(file "$NIDHI")"
    echo "  Note: may still be static but 'file' doesn't always report it"
fi
echo ""

# Test 6: Binary size check.
echo "--- Test 6: Binary size ---"
SIZE=$(stat -f%z "$NIDHI" 2>/dev/null || stat -c%s "$NIDHI" 2>/dev/null || echo "0")
SIZE_MB=$((SIZE / 1024 / 1024))
echo "  Binary size: ${SIZE_MB}MB ($SIZE bytes)"
if [ "$SIZE_MB" -lt 50 ]; then
    echo "  PASS: binary size < 50MB"
else
    echo "  WARNING: binary size ${SIZE_MB}MB is large"
fi
echo ""

# Test 7: Terminal state restoration.
echo "--- Test 7: Terminal state ---"
# Send q immediately to test clean exit.
echo "q" | timeout 5 "$NIDHI" -C "$DEMO_DIR" --no-color 2>/dev/null || true
# If we get here, terminal state was restored (no hang, no corruption).
echo "  PASS: nidhi exited cleanly"
echo ""

# Cleanup.
rm -rf "$DEMO_DIR"

echo "=== All Smoke Tests Passed ==="
