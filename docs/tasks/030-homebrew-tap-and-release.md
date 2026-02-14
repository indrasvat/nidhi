# Task 030: Homebrew Tap and Release

## Status: TODO

## Depends On
- 028 (CI/CD and GitHub Actions) -- release pipeline must be in place
- 029 (Documentation and README) -- README, LICENSE, man page must exist before release

## Parallelizable With
- None (final task -- everything must be complete)

## Problem
The project is fully built and tested, but has no distribution packaging. Users cannot install nidhi via Homebrew, and there is no v0.1.0 release with pre-built binaries. We need: (1) a Homebrew tap repository with a formula that installs nidhi from GitHub Releases, (2) verification that `go install` works for the published module, (3) a tagged v0.1.0 release with goreleaser-produced binaries and changelog, and (4) a final smoke test proving the released binary works correctly in a real terminal.

## PRD Reference
- Section 17.2 (Distribution Channels) -- Homebrew, go install, GitHub Releases
- Section 17.3 (Versioning) -- semantic versioning, v0.x.y until stable
- Section 18 (Milestones) -- Phase 1 Core (v0.1.0 "First Light") through Phase 5 Release (v1.0.0)
- Section 21.7 (.goreleaser.yml) -- goreleaser config with brew tap

## Files to Create
- `docs/homebrew-formula.md` -- documentation for the Homebrew tap setup process
- `docs/release-checklist.md` -- step-by-step release checklist
- `scripts/smoke-test.sh` -- automated release smoke test script

## Files to Modify
- None in this repo (Homebrew tap is a separate repository)

## Execution Steps

### Step 1: Document Homebrew tap setup in `docs/homebrew-formula.md`

```markdown
# Homebrew Tap Setup: indrasvat/homebrew-tap

This document describes how to set up and maintain the Homebrew tap for nidhi.

## Repository Structure

The Homebrew tap lives at `github.com/indrasvat/homebrew-tap`:

```
homebrew-tap/
├── Formula/
│   └── nidhi.rb      # Homebrew formula (auto-updated by goreleaser)
└── README.md
```

## Creating the Tap Repository

### Step 1: Create the repo

```bash
# Create the homebrew-tap repo on GitHub.
gh repo create indrasvat/homebrew-tap --public --description "Homebrew formulae for indrasvat's tools"

# Clone and initialize.
git clone https://github.com/indrasvat/homebrew-tap.git
cd homebrew-tap
mkdir -p Formula
```

### Step 2: Create the initial formula

goreleaser auto-generates `Formula/nidhi.rb` on each release. For the initial
setup, create a placeholder:

```ruby
# Formula/nidhi.rb
# This formula is auto-updated by goreleaser on each release.
# Manual edits will be overwritten.

class Nidhi < Formula
  desc "Purpose-built TUI for git stash mastery"
  homepage "https://github.com/indrasvat/nidhi"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_darwin_arm64.tar.gz"
      # sha256 will be filled by goreleaser
    end
    on_intel do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_darwin_amd64.tar.gz"
      # sha256 will be filled by goreleaser
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_linux_arm64.tar.gz"
      # sha256 will be filled by goreleaser
    end
    on_intel do
      url "https://github.com/indrasvat/nidhi/releases/download/v#{version}/nidhi_#{version}_linux_amd64.tar.gz"
      # sha256 will be filled by goreleaser
    end
  end

  def install
    bin.install "nidhi"
  end

  def post_install
    ohai "nidhi installed! Run 'nidhi' in any git repo to get started."
    ohai "  Quick start: j/k navigate, Tab preview, Enter detail, ? help"
  end

  test do
    assert_match "nidhi", shell_output("#{bin}/nidhi --version")
  end
end
```

### Step 3: Push and test

```bash
git add Formula/nidhi.rb README.md
git commit -m "feat: add nidhi formula"
git push origin main
```

### Step 4: goreleaser automation

On each nidhi release, goreleaser (via `.goreleaser.yml` brews section)
automatically:
1. Generates a new `Formula/nidhi.rb` with correct URLs and SHA256 checksums
2. Commits it to `indrasvat/homebrew-tap`
3. This requires a `HOMEBREW_TAP_TOKEN` secret in the nidhi repo's GitHub settings

To set up:
```bash
# Create a fine-grained personal access token with:
#   - Repository access: indrasvat/homebrew-tap
#   - Permissions: Contents (read/write)
# Add it as a secret in the nidhi repo:
gh secret set HOMEBREW_TAP_TOKEN --repo indrasvat/nidhi
```

## Testing the Formula

```bash
# Install from the tap.
brew tap indrasvat/tap
brew install nidhi

# Verify.
nidhi --version
nidhi --help

# Test formula from source.
brew install --build-from-source indrasvat/tap/nidhi

# Run Homebrew's formula test.
brew test nidhi
```

## Updating

goreleaser handles updates automatically. Manual update if needed:

```bash
cd homebrew-tap
# Edit Formula/nidhi.rb with new version, URLs, SHA256s.
brew audit --strict Formula/nidhi.rb
git add -A && git commit -m "chore: update nidhi to vX.Y.Z" && git push
```
```

### Step 2: Create release checklist in `docs/release-checklist.md`

```markdown
# nidhi Release Checklist

## Pre-release

- [ ] All tasks in `docs/PROGRESS.md` are marked DONE
- [ ] `make ci` passes (lint + test + build)
- [ ] `make e2e` passes (E2E tests)
- [ ] `make bench` passes (performance benchmarks meet targets)
- [ ] `make coverage-check` passes (>70% on core packages)
- [ ] `goreleaser check` validates config
- [ ] `goreleaser release --snapshot --clean` produces binaries for all platforms
- [ ] README.md is complete with screenshots
- [ ] LICENSE file exists
- [ ] CLAUDE.md Learnings section is up to date
- [ ] docs/PROGRESS.md is fully updated

## Tagging

```bash
# Verify clean working tree.
git status  # should be clean

# Create annotated tag.
git tag -a v0.1.0 -m "Phase 1: First Light

First release of nidhi -- purpose-built TUI for git stash mastery.

Features:
- LIST, PREVIEW, DETAIL modes with progressive disclosure
- Stash CRUD: apply, pop, drop, create
- Conflict preview via merge-tree (Git 2.38+)
- Deep fuzzy search across messages, files, and diffs
- Export/Import stashes (Git 2.51+)
- Inline rename
- Undo drops (session + cross-session recovery)
- Stale detection with badges
- Reorder stashes
- Branch and stale filters
- Help overlay
- Agni theme (custom dark theme)
- Responsive layout (80x24 to ultrawide)
- Nerd Font support with ASCII fallback
- Zero-config with TOML/git config/env var/CLI flag overrides"

# Verify the tag.
git tag -l -n1 v0.1.0

# Push tag (triggers release workflow).
git push origin v0.1.0
```

## Release Verification

After goreleaser creates the release:

- [ ] GitHub Release page exists at https://github.com/indrasvat/nidhi/releases/tag/v0.1.0
- [ ] Changelog is populated (grouped by feat/fix/perf)
- [ ] Binaries attached:
  - [ ] `nidhi_0.1.0_darwin_arm64.tar.gz`
  - [ ] `nidhi_0.1.0_darwin_amd64.tar.gz`
  - [ ] `nidhi_0.1.0_linux_amd64.tar.gz`
  - [ ] `nidhi_0.1.0_linux_arm64.tar.gz`
  - [ ] `nidhi_0.1.0_windows_amd64.zip`
  - [ ] `checksums.txt`
- [ ] Homebrew formula updated in `indrasvat/homebrew-tap`

## Post-release Smoke Test

```bash
# Test 1: Download and run binary on a clean machine.
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
curl -Lo nidhi.tar.gz "https://github.com/indrasvat/nidhi/releases/download/v0.1.0/nidhi_0.1.0_$(uname -s | tr A-Z a-z)_$(uname -m).tar.gz"
tar xzf nidhi.tar.gz
./nidhi --version  # Should print: nidhi 0.1.0 (commit: ..., built: ...)

# Test 2: Homebrew install.
brew tap indrasvat/tap
brew install nidhi
nidhi --version

# Test 3: go install.
go install github.com/indrasvat/nidhi/cmd/nidhi@v0.1.0
nidhi --version

# Test 4: Run in a real repo.
cd /path/to/any/git/repo
nidhi  # Verify it launches, shows stashes, responds to keys, exits cleanly
```

## Version History

| Version | Phase | Codename | Date |
|---------|-------|----------|------|
| v0.1.0 | Phase 1: Core | First Light | TBD |
| v0.2.0 | Phase 2: Safety Net | No Fear | TBD |
| v0.3.0 | Phase 3: Power User | Master of Stashes | TBD |
| v0.4.0 | Phase 4: Sync | Across Machines | TBD |
| v1.0.0 | Phase 5: Polish | Release | TBD |
```

### Step 3: Create smoke test script in `scripts/smoke-test.sh`

```bash
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
```

Make the script executable:

```bash
chmod +x scripts/smoke-test.sh
```

### Step 4: Verify `go install` works

```bash
# Test that the module structure supports go install.
# This verifies cmd/nidhi/main.go is at the correct import path.
go build ./cmd/nidhi
./bin/nidhi --version
```

### Step 5: Test goreleaser dry-run

```bash
# Verify goreleaser produces all expected artifacts.
goreleaser release --snapshot --clean

# Check outputs.
ls dist/nidhi_*_darwin_arm64.tar.gz
ls dist/nidhi_*_darwin_amd64.tar.gz
ls dist/nidhi_*_linux_amd64.tar.gz
ls dist/nidhi_*_linux_arm64.tar.gz
ls dist/nidhi_*_windows_amd64.zip
ls dist/checksums.txt

# Verify binary inside archive works.
TMPDIR=$(mktemp -d)
tar xzf dist/nidhi_*_darwin_arm64.tar.gz -C "$TMPDIR"
"$TMPDIR/nidhi" --version
rm -rf "$TMPDIR"
```

### Step 6: Run smoke test

```bash
# Build the binary.
make build

# Run smoke test.
./scripts/smoke-test.sh

# Run smoke test with a specific binary (e.g., from goreleaser).
./scripts/smoke-test.sh dist/nidhi_snapshot_darwin_arm64/nidhi
```

### Step 7: Run final smoke test with iterm2-driver (macOS only)

```bash
# Prerequisites: macOS, iTerm2, iterm2-driver in PATH.

# Build binary.
make build

# Create a real test repo.
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
git init && git config user.email "test@test.com" && git config user.name "Test"
echo "# test" > README.md && git add . && git commit -m "init"

# Create diverse stashes.
for msg in "Fix auth bug" "WIP: dashboard" "Hotfix: rate limit" "Experimental: cache" "Cleanup: dead code"; do
    echo "content for: $msg" > "$(echo $msg | tr ' :' '_').txt"
    git add . && git stash push -m "$msg"
done

# Launch nidhi and take screenshot.
iterm2-driver \
    --cols 120 --rows 40 \
    --delay 3000ms \
    --screenshot /tmp/nidhi-smoke-list.png \
    -- bin/nidhi -C "$TMPDIR"

# Navigate, preview, detail, and capture each.
iterm2-driver \
    --cols 120 --rows 40 \
    --send-key Tab \
    --delay 2000ms \
    --screenshot /tmp/nidhi-smoke-preview.png \
    -- bin/nidhi -C "$TMPDIR"

echo "Screenshots saved to /tmp/nidhi-smoke-*.png"
open /tmp/nidhi-smoke-list.png
open /tmp/nidhi-smoke-preview.png

# Verify: Agni theme visible, correct layout, correct keybinds in footer.
# Verify: stash count in status bar, cursor on first row, diff in preview.

# Clean up.
rm -rf "$TMPDIR"
```

### Step 8: Prepare the release tag

```bash
# Verify everything is clean.
git status  # should be clean

# Verify CI passes.
make ci

# Verify goreleaser.
goreleaser check
goreleaser release --snapshot --clean

# Run smoke test.
./scripts/smoke-test.sh

# Tag the release (do NOT push yet -- this is a dry run).
echo "Ready to tag v0.1.0"
echo "Run: git tag -a v0.1.0 -m 'Phase 1: First Light'"
echo "Then: git push origin v0.1.0"
```

### Step 9: Add Makefile targets

Add to the existing Makefile:

```makefile
.PHONY: smoke-test
smoke-test: build ## Run release smoke test
	./scripts/smoke-test.sh

.PHONY: release-prep
release-prep: ci e2e bench coverage-check release-check smoke-test ## Full release preparation
	@echo ""
	@echo "=== Release preparation complete ==="
	@echo "All checks passed. Ready to tag and release."
	@echo ""
	@echo "Next steps:"
	@echo "  1. git tag -a v0.1.0 -m 'Phase 1: First Light'"
	@echo "  2. git push origin v0.1.0"
	@echo "  3. Monitor: https://github.com/indrasvat/nidhi/actions"
	@echo "  4. Verify: https://github.com/indrasvat/nidhi/releases"
```

## Verification

### Documentation
```bash
# Homebrew formula doc exists.
test -f docs/homebrew-formula.md

# Release checklist exists.
test -f docs/release-checklist.md

# Smoke test script exists and is executable.
test -f scripts/smoke-test.sh
test -x scripts/smoke-test.sh
```

### Goreleaser
```bash
# Config is valid.
goreleaser check

# Dry-run produces all artifacts.
goreleaser release --snapshot --clean
ls dist/nidhi_*_darwin_arm64.tar.gz
ls dist/nidhi_*_darwin_amd64.tar.gz
ls dist/nidhi_*_linux_amd64.tar.gz
ls dist/nidhi_*_linux_arm64.tar.gz
ls dist/nidhi_*_windows_amd64.zip
ls dist/checksums.txt
```

### Smoke Test
```bash
# Smoke test passes.
make build
./scripts/smoke-test.sh

# All 7 smoke test checks pass:
# 1. --version prints version string
# 2. --help shows all flags
# 3. Non-git directory exits non-zero
# 4. Real repo with stashes shows --debug output
# 5. Binary size < 50MB
# 6. Terminal state restored after exit
```

### Go Install
```bash
# Module structure supports go install.
go build ./cmd/nidhi
./bin/nidhi --version
```

### Binary Verification
```bash
# Extract binary from goreleaser archive and verify.
TMPDIR=$(mktemp -d)
tar xzf dist/nidhi_*_$(go env GOOS)_$(go env GOARCH).tar.gz -C "$TMPDIR"
"$TMPDIR/nidhi" --version | grep -q "nidhi"
echo "Binary verification: PASS"
rm -rf "$TMPDIR"
```

### Visual Verification (macOS + iTerm2)
```bash
# Screenshot smoke test.
# Launch nidhi in a real repo, verify:
# - Agni theme colors correct
# - Status bar shows repo info
# - Footer shows keybinds
# - Stash list renders correctly
# - Preview mode shows diff
# - Help overlay opens and closes
# - Clean exit restores terminal
```

### Release Readiness
```bash
# Full release preparation.
make release-prep
```

## Completion Criteria
1. `docs/homebrew-formula.md` documents: tap repo creation, formula structure, goreleaser automation, token setup, testing commands
2. `docs/release-checklist.md` documents: pre-release checks, tagging command, release verification, post-release smoke test, version history table
3. `scripts/smoke-test.sh` is executable and tests: --version, --help, non-git repo handling, real repo with stashes, --debug flag, static linking, binary size, terminal state restoration
4. Smoke test passes on fresh build (`make build && ./scripts/smoke-test.sh`)
5. goreleaser dry-run (`goreleaser release --snapshot --clean`) produces binaries for all 5 platform/arch combinations
6. Extracted binary from goreleaser archive runs `--version` correctly
7. `go build ./cmd/nidhi` succeeds (go install compatibility)
8. Makefile has `smoke-test` and `release-prep` targets
9. `make release-prep` runs full pipeline: ci, e2e, bench, coverage-check, release-check, smoke-test
10. Version plan documented: v0.1.0 (Core), v0.2.0 (Safety Net), v0.3.0 (Power User), v0.4.0 (Sync), v1.0.0 (Release)
11. Homebrew formula template includes: description, license, platform-specific URLs, install block, post_install message, test block
12. Release tag format documented: `git tag -a v0.1.0 -m "Phase 1: First Light"`
13. All verification steps pass

## Commit
```
chore: add Homebrew tap docs, release checklist, and smoke test

Document Homebrew tap setup (indrasvat/homebrew-tap with goreleaser
automation), create release checklist with pre-release/tagging/verification
steps, add scripts/smoke-test.sh (7 checks: version, help, non-git,
real repo, debug flag, binary size, terminal restore). Add Makefile
targets smoke-test and release-prep for full release pipeline validation.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 17.2-17.3, 18, 21.7
4. Verify tasks 028 and 029 are DONE in `docs/PROGRESS.md`
5. Create `docs/homebrew-formula.md` with tap setup instructions
6. Create `docs/release-checklist.md` with step-by-step process
7. Create `scripts/smoke-test.sh` and make executable
8. Add Makefile targets
9. Run `goreleaser check` and `goreleaser release --snapshot --clean`
10. Run `make build && ./scripts/smoke-test.sh`
11. If on macOS with iTerm2: run visual smoke test with iterm2-driver
12. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
13. Commit with the message above
14. Final: update `docs/PROGRESS.md` to reflect Phase 1 complete
