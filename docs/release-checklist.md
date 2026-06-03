# nidhi Release Checklist

## Pre-release

- [ ] All tasks in `docs/PROGRESS.md` are marked DONE
- [ ] `make ci` passes (lint + test + build)
- [ ] `make e2e` passes (E2E tests)
- [ ] `make bench` passes (performance benchmarks meet targets)
- [ ] `make coverage-check` passes (>70% on core packages)
- [ ] `goreleaser check` validates config
- [ ] `goreleaser release --snapshot --clean` produces macOS Apple Silicon and Linux binaries
- [ ] `make install-script-test` passes against a local fake release
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
  - [ ] `nidhi_0.1.0_linux_amd64.tar.gz`
  - [ ] `nidhi_0.1.0_linux_arm64.tar.gz`
  - [ ] `checksums.txt`

## Post-release Smoke Test

```bash
# Test 1: Download and run binary on a clean machine.
TMPDIR=$(mktemp -d)
cd "$TMPDIR"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  arm64|aarch64) ARCH=arm64 ;;
  x86_64|amd64) ARCH=amd64 ;;
esac
curl -Lo nidhi.tar.gz "https://github.com/indrasvat/nidhi/releases/download/v0.1.0/nidhi_0.1.0_${OS}_${ARCH}.tar.gz"
tar xzf nidhi.tar.gz
./nidhi --version  # Should print: nidhi 0.1.0 (commit: ..., built: ...)

# Test 2: One-line installer.
curl -sSfL https://raw.githubusercontent.com/indrasvat/nidhi/main/install.sh | sh
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
