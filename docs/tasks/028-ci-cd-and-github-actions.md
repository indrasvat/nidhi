# Task 028: CI/CD and GitHub Actions

## Status: TODO

## Depends On
- 027 (Performance Validation) -- all tests pass before CI enforcement
- 000 (scaffold) -- Makefile, .golangci.yml, .goreleaser.yml

## Parallelizable With
- 029 (Documentation and README) -- CI and docs can be written simultaneously

## Problem
The project has no automated CI/CD pipeline. Tests, linting, and coverage only run locally via `make ci`. We need: (1) a GitHub Actions CI workflow that runs lint + tests on every push and PR with a Go 1.26 matrix across Linux and macOS, (2) a release workflow triggered by version tags that uses goreleaser to build cross-platform binaries and create GitHub Releases, (3) coverage enforcement > 70% on core packages, and (4) a goreleaser config that produces Homebrew formulae and signed checksums.

## PRD Reference
- Section 4.3 (Build & Dev) -- golangci-lint v2, goreleaser, gotestsum, lefthook
- Section 16.3 (CI/CD) -- GitHub Actions matrix, coverage enforcement > 70%
- Section 17.1 (Build) -- make targets
- Section 17.2 (Distribution Channels) -- Homebrew, go install, GitHub Releases
- Section 17.3 (Versioning) -- semantic versioning, conventional commits
- Section 21.7 (.goreleaser.yml) -- exact config content
- Section 21.8 (.golangci.yml) -- exact config content

## Files to Create
- `.github/workflows/ci.yml` -- CI pipeline (lint, test, coverage)
- `.github/workflows/release.yml` -- release pipeline (goreleaser)
- `.github/dependabot.yml` -- automated dependency updates

## Files to Modify
- `.goreleaser.yml` -- expand with checksum, changelog, announcement config
- `Makefile` -- ensure `make ci` is CI-compatible, add `make coverage` target

## Execution Steps

### Step 1: Create `.github/workflows/ci.yml`

```yaml
# .github/workflows/ci.yml
# CI pipeline for nidhi — runs on every push and pull request.
# Matrix: Go 1.26.x on ubuntu-latest and macos-latest.

name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

permissions:
  contents: read

env:
  GO_VERSION: "1.26"

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
          args: --timeout=3m

  test:
    name: Test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest]
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Install gotestsum
        run: go install gotest.tools/gotestsum@latest

      - name: Run tests
        run: |
          gotestsum --format=standard-verbose -- \
            -race \
            -coverprofile=coverage.out \
            -covermode=atomic \
            -timeout=300s \
            ./...

      - name: Check coverage (core packages)
        if: matrix.os == 'ubuntu-latest'
        run: |
          # Extract coverage for internal/core/ and internal/git/.
          CORE_COV=$(go tool cover -func=coverage.out | grep '^github.com/indrasvat/nidhi/internal/core/' | awk '{sum+=$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')
          GIT_COV=$(go tool cover -func=coverage.out | grep '^github.com/indrasvat/nidhi/internal/git/' | awk '{sum+=$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}')
          TOTAL_COV=$(go tool cover -func=coverage.out | tail -1 | awk '{print $NF}')

          echo "Coverage: total=$TOTAL_COV, internal/core=$CORE_COV%, internal/git=$GIT_COV%"

          # Enforce > 70% on core packages.
          check_coverage() {
            local pkg=$1
            local cov=$2
            local min=70.0
            if [ "$(echo "$cov < $min" | bc -l)" -eq 1 ]; then
              echo "FAIL: $pkg coverage $cov% < $min%"
              return 1
            else
              echo "PASS: $pkg coverage $cov% >= $min%"
              return 0
            fi
          }

          FAILED=0
          check_coverage "internal/core" "$CORE_COV" || FAILED=1
          check_coverage "internal/git" "$GIT_COV" || FAILED=1

          if [ "$FAILED" -eq 1 ]; then
            echo "::error::Coverage below 70% threshold on core packages"
            exit 1
          fi

      - name: Upload coverage
        if: matrix.os == 'ubuntu-latest'
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out
          flags: unittests
          fail_ci_if_error: false
        env:
          CODECOV_TOKEN: ${{ secrets.CODECOV_TOKEN }}

      - name: Upload coverage artifact
        if: matrix.os == 'ubuntu-latest'
        uses: actions/upload-artifact@v4
        with:
          name: coverage-report
          path: coverage.out
          retention-days: 30

  build:
    name: Build (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest]
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Build binary
        run: make build

      - name: Verify binary
        run: |
          ./bin/nidhi --version
          ./bin/nidhi --help

      - name: Verify goreleaser config
        if: matrix.os == 'ubuntu-latest'
        run: |
          go install github.com/goreleaser/goreleaser/v2@latest
          goreleaser check

  e2e:
    name: E2E Tests
    runs-on: ubuntu-latest
    needs: [lint, test, build]
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Install gotestsum
        run: go install gotest.tools/gotestsum@latest

      - name: Run E2E tests
        run: |
          gotestsum --format=standard-verbose -- \
            -v \
            -timeout=600s \
            -count=1 \
            ./internal/e2e/...
        env:
          # Skip screenshot tests in CI (no iTerm2).
          NIDHI_SCREENSHOT_TEST: "0"
```

### Step 2: Create `.github/workflows/release.yml`

```yaml
# .github/workflows/release.yml
# Release pipeline — triggered by version tags (v*).
# Uses goreleaser to build cross-platform binaries and create GitHub Releases.

name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write
  packages: write

env:
  GO_VERSION: "1.26"

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0 # goreleaser needs full history for changelog

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Run tests before release
        run: |
          go install gotest.tools/gotestsum@latest
          gotestsum --format=standard-verbose -- \
            -race \
            -timeout=300s \
            ./...

      - name: Run goreleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          # HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}

      - name: Upload release artifacts
        uses: actions/upload-artifact@v4
        with:
          name: release-binaries
          path: dist/*.tar.gz
          retention-days: 90
```

### Step 3: Create `.github/dependabot.yml`

```yaml
# .github/dependabot.yml
# Automated dependency updates for Go modules and GitHub Actions.

version: 2
updates:
  # Go modules.
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
    labels:
      - "dependencies"
      - "go"
    commit-message:
      prefix: "chore(deps)"
    open-pull-requests-limit: 10

  # GitHub Actions.
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
      day: "monday"
    labels:
      - "dependencies"
      - "ci"
    commit-message:
      prefix: "chore(ci)"
    open-pull-requests-limit: 5
```

### Step 4: Expand `.goreleaser.yml`

Replace the existing `.goreleaser.yml` with the complete version:

```yaml
# .goreleaser.yml — nidhi release configuration.
# Docs: https://goreleaser.com
version: 2

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - main: ./cmd/nidhi
    binary: nidhi
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    name_template: >-
      {{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}
    files:
      - README.md
      - LICENSE

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  use: github
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
      - "^ci:"
      - Merge pull request
      - Merge branch
  groups:
    - title: "New Features"
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: "Bug Fixes"
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: "Performance"
      regexp: '^.*?perf(\([[:word:]]+\))??!?:.+$'
      order: 2
    - title: "Other"
      order: 999

brews:
  - repository:
      owner: indrasvat
      name: homebrew-tap
      # token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    homepage: "https://github.com/indrasvat/nidhi"
    description: "Purpose-built TUI for git stash mastery"
    license: "MIT"
    install: |
      bin.install "nidhi"
    test: |
      system "#{bin}/nidhi", "--version"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com

# Signing (optional — uncomment when GPG key is available).
# signs:
#   - artifacts: checksum
#     args:
#       - "--batch"
#       - "--local-user"
#       - "{{ .Env.GPG_FINGERPRINT }}"
#       - "--output"
#       - "${signature}"
#       - "--detach-sign"
#       - "${artifact}"

release:
  github:
    owner: indrasvat
    name: nidhi
  prerelease: auto
  name_template: "{{ .ProjectName }} v{{ .Version }}"
  header: |
    ## nidhi v{{ .Version }}

    Purpose-built TUI for git stash mastery.
  footer: |
    ---
    **Full Changelog**: https://github.com/indrasvat/nidhi/compare/{{ .PreviousTag }}...{{ .Tag }}
```

### Step 5: Add Makefile targets for CI/CD

Add to the existing Makefile:

```makefile
.PHONY: coverage
coverage: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out -covermode=atomic -timeout=300s ./...
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: coverage-check
coverage-check: coverage ## Check coverage meets minimum threshold
	@echo "Checking coverage thresholds..."
	@CORE_COV=$$(go tool cover -func=coverage.out | grep 'internal/core/' | awk '{sum+=$$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}'); \
	GIT_COV=$$(go tool cover -func=coverage.out | grep 'internal/git/' | awk '{sum+=$$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}'); \
	echo "internal/core coverage: $$CORE_COV%"; \
	echo "internal/git coverage: $$GIT_COV%"; \
	echo "$$CORE_COV 70.0" | awk '{if ($$1 < $$2) {print "FAIL: internal/core below 70%"; exit 1}}'; \
	echo "$$GIT_COV 70.0" | awk '{if ($$1 < $$2) {print "FAIL: internal/git below 70%"; exit 1}}'

.PHONY: release-check
release-check: ## Validate goreleaser configuration
	goreleaser check
	@echo "goreleaser config is valid"

.PHONY: release-dry-run
release-dry-run: ## Dry-run goreleaser (build but don't publish)
	goreleaser release --snapshot --clean
```

### Step 6: Verify CI workflow locally

```bash
# Verify all workflow files are valid YAML.
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"
python3 -c "import yaml; yaml.safe_load(open('.github/dependabot.yml'))"

# Verify goreleaser config.
goreleaser check

# Run the same commands that CI will run.
make lint
make test
make build
./bin/nidhi --version
./bin/nidhi --help

# Run coverage check.
make coverage-check

# Dry-run goreleaser.
goreleaser release --snapshot --clean
```

### Step 7: Test with `act` (optional -- local GitHub Actions runner)

```bash
# Install act if available.
# brew install act

# Test the CI workflow locally.
act push --workflows .github/workflows/ci.yml

# Test the release workflow with a fake tag.
act push --workflows .github/workflows/release.yml --env GITHUB_REF=refs/tags/v0.1.0-test
```

### Step 8: Verify the complete CI pipeline

```bash
make ci
make coverage-check
make release-check
```

## Verification

### Workflow Files
```bash
# CI workflow exists and is valid YAML.
test -f .github/workflows/ci.yml
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"

# Release workflow exists and is valid YAML.
test -f .github/workflows/release.yml
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"

# Dependabot config exists and is valid YAML.
test -f .github/dependabot.yml
python3 -c "import yaml; yaml.safe_load(open('.github/dependabot.yml'))"
```

### Goreleaser
```bash
# Config is valid.
goreleaser check

# Dry-run produces binaries.
goreleaser release --snapshot --clean
ls dist/nidhi_*_darwin_arm64.tar.gz
ls dist/nidhi_*_linux_amd64.tar.gz
ls dist/nidhi_*_windows_amd64.zip
ls dist/checksums.txt
```

### Coverage
```bash
# Coverage report generates.
make coverage
test -f coverage.out
test -f coverage.html

# Coverage check passes.
make coverage-check
```

### Makefile Targets
```bash
# All new targets work.
make coverage
make coverage-check
make release-check
make release-dry-run
```

### CI Pipeline Simulation
```bash
# Simulate what CI runs.
make lint && make test && make build && ./bin/nidhi --version
```

## Completion Criteria
1. `.github/workflows/ci.yml` triggers on push to main and pull requests
2. CI matrix runs Go 1.26.x on ubuntu-latest and macos-latest
3. CI pipeline: checkout, setup-go, lint (golangci-lint), test (gotestsum + race + coverage), build + verify
4. Coverage enforcement: > 70% on `internal/core/` and `internal/git/` packages
5. Coverage uploaded to Codecov (with fallback -- `fail_ci_if_error: false`)
6. Go module cache and build cache enabled for fast CI runs
7. E2E tests run after lint+test+build succeed (screenshot tests skipped in CI)
8. `.github/workflows/release.yml` triggers on `v*` tags
9. Release workflow: full test run, goreleaser with `--clean`, GitHub Release with changelog
10. `.goreleaser.yml` builds for macOS (arm64, amd64), Linux (arm64, amd64), Windows (amd64)
11. goreleaser produces: tar.gz archives, zip for Windows, SHA256 checksums, Homebrew formula
12. Changelog groups: New Features, Bug Fixes, Performance, Other (excludes docs/test/chore commits)
13. `.github/dependabot.yml` monitors gomod and github-actions weekly
14. Makefile has `coverage`, `coverage-check`, `release-check`, `release-dry-run` targets
15. `goreleaser check` validates config
16. `make ci` passes locally
17. Dry-run goreleaser produces binaries for all platforms

## Commit
```
ci: add GitHub Actions CI/CD pipeline with goreleaser releases

Add .github/workflows/ci.yml (lint + test + coverage on Go 1.26 matrix
across Linux and macOS, >70% coverage gate on core packages, E2E tests,
Codecov upload) and release.yml (goreleaser on v* tags, cross-platform
binaries, GitHub Releases with changelog, Homebrew formula). Expand
.goreleaser.yml with checksums, changelog groups, brew tap config.
Add dependabot.yml for weekly dependency updates.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 4.3, 16.3, 17.1-17.3, 21.7-21.8
4. Verify task 027 is DONE in `docs/PROGRESS.md`
5. Create `.github/workflows/` directory
6. Write `ci.yml` with lint, test, coverage, build, e2e jobs
7. Write `release.yml` with goreleaser
8. Write `dependabot.yml`
9. Expand `.goreleaser.yml` with full config
10. Add Makefile targets
11. Validate YAML files and goreleaser config
12. Run `make ci` locally
13. Run `goreleaser release --snapshot --clean` to verify dry-run
14. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
15. Commit with the message above
