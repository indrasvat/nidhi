# Task 000: Repository Scaffold and Tooling

## Status: TODO

## Depends On
- None (foundation task)

## Parallelizable With
- None (all subsequent tasks depend on this)

## Problem
The nidhi project has bootstrap files (CLAUDE.md, Makefile, lefthook.yml, .gitignore) but no Go module, no source code, no linter config, no release config, and no Claude Code hooks. Nothing compiles. Every subsequent task requires a working `go build`, `go test`, and `make ci` pipeline.

## PRD Reference
- Section 4.1 (Core Dependencies) -- Go 1.26, BubbleTea v2, LipGloss v2, Bubbles v2 with pinned versions
- Section 4.2 (Supporting Libraries) -- go-toml/v2, xdg, colorprofile, sahilm/fuzzy
- Section 4.3 (Build & Dev) -- golangci-lint v2, goreleaser, gotestsum, lefthook
- Section 8.4 (Module Structure) -- directory layout, cmd/nidhi/main.go
- Section 12.5 (CLI Flags) -- `--help`, `--version`, `--log-level`, `--trace-git`, `--debug`, `--no-color`, `--no-animation`, `--icons`, `-C`
- Section 21.6 (Project Initialization Checklist) -- bootstrap sequence
- Section 21.7 (.goreleaser.yml) -- exact YAML content
- Section 21.8 (.golangci.yml) -- exact YAML content with v2 schema

## Files to Create
- `go.mod` -- module `github.com/indrasvat/nidhi`, Go 1.26, pinned Charm v2 deps
- `cmd/nidhi/main.go` -- minimal CLI entrypoint with `--help`, `--version` (ldflags)
- `.golangci.yml` -- v2 schema from PRD section 21.8
- `.goreleaser.yml` -- from PRD section 21.7
- `.claude/settings.json` -- Claude Code hooks (PreToolUse commit gate, Stop Learnings reminder)

## Files to Modify
- `CLAUDE.md` -- update Learnings section with any discoveries

## Execution Steps

### Step 1: Create directory structure

```bash
mkdir -p cmd/nidhi
mkdir -p internal/{core,git,plugin,plugins,ui/{theme,layout,components,screens,icons},config}
```

### Step 2: Initialize Go module

```bash
go mod init github.com/indrasvat/nidhi
```

Edit `go.mod` to pin Go 1.26:

```go
module github.com/indrasvat/nidhi

go 1.26
```

### Step 3: Create `cmd/nidhi/main.go`

```go
package main

import (
	"fmt"
	"os"
)

// Build metadata injected via ldflags at compile time.
// See Makefile LDFLAGS: -X main.version=... -X main.commit=... -X main.date=...
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("nidhi %s (commit: %s, built: %s)\n", version, commit, date)
			os.Exit(0)
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	// TODO: Initialize config, git runner, BubbleTea program
	fmt.Println("nidhi -- purpose-built TUI for git stash mastery")
	fmt.Println("Run with --help for usage.")
}

func printUsage() {
	fmt.Print(`nidhi -- purpose-built TUI for git stash mastery

Usage:
  nidhi [flags]

Flags:
  -h, --help              Show this help message
  -v, --version           Show version information
      --log-level string  Log level (off, error, warn, info, debug)
      --trace-git         Log all git commands with args, exit code, duration
      --debug             Print startup timing breakdown and exit
      --no-color          Disable all colors
      --no-animation      Disable animations
      --icons string      Icon set: auto (default), nerd, ascii
  -C, --directory string  Run as if started in <path>
`)
}
```

### Step 4: Add Charm v2 dependencies

Run `go get` for pinned Charm v2 pre-release versions:

```bash
go get charm.land/bubbletea/v2@latest
go get charm.land/bubbles/v2@latest
# LipGloss v2 comes transitively via Bubbles v2
```

Then add supporting libraries:

```bash
go get github.com/pelletier/go-toml/v2@latest
go get github.com/adrg/xdg@latest
go get github.com/sahilm/fuzzy@latest
```

Run `go mod tidy` to clean up.

### Step 5: Create `.golangci.yml`

Use the exact v2 schema from PRD section 21.8:

```yaml
version: "2"

run:
  timeout: 3m
  go: "1.26"

linters:
  enable:
    - errcheck
    - govet
    - staticcheck
    - unused
    - gosimple
    - ineffassign
    - typecheck
    - misspell
    - revive
  settings:
    revive:
      rules:
        - name: exported
          disabled: true  # we'll enable this once APIs stabilize
  exclusions:
    rules:
      - path: _test\.go
        linters: [errcheck]

formatters:
  enable:
    - gofmt
    - goimports
```

### Step 6: Create `.goreleaser.yml`

Use the exact config from PRD section 21.7:

```yaml
version: 2
builds:
  - main: ./cmd/nidhi
    binary: nidhi
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
brews:
  - repository:
      owner: indrasvat
      name: homebrew-tap
    homepage: "https://github.com/indrasvat/nidhi"
    description: "Purpose-built TUI for git stash mastery"
```

### Step 7: Create `.claude/settings.json`

```bash
mkdir -p .claude
```

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "intercept",
            "condition": "command contains 'git commit'",
            "message": "STOP: Before committing, verify:\n1. All tests pass (make test)\n2. Linter passes (make lint)\n3. CLAUDE.md Learnings section is updated\n4. docs/PROGRESS.md is updated"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "notify",
            "message": "REMINDER: Update CLAUDE.md Learnings section and docs/PROGRESS.md before ending session."
          }
        ]
      }
    ]
  }
}
```

### Step 8: Verify the build

```bash
go mod tidy
go build ./cmd/nidhi
go build -ldflags "-s -w -X main.version=0.1.0-dev -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/nidhi ./cmd/nidhi
```

### Step 9: Verify CLI flags

```bash
./bin/nidhi --version
# Expected: nidhi 0.1.0-dev (commit: <sha>, built: <timestamp>)

./bin/nidhi --help
# Expected: Full usage text with all flags listed

./bin/nidhi
# Expected: Placeholder message
```

### Step 10: Verify Makefile integration

```bash
make build
# Expected: bin/nidhi binary created

make test
# Expected: pass (no test files yet = pass)

make lint
# Expected: pass on minimal codebase
```

### Step 11: Run `make ci` end-to-end

```bash
make ci
# Expected: lint passes, tests pass
```

## Verification

### Functional
```bash
# Module compiles
go build ./cmd/nidhi

# Binary builds with ldflags
make build
test -f bin/nidhi

# Version flag works
./bin/nidhi --version | grep -q "nidhi"

# Help flag works
./bin/nidhi --help | grep -q "log-level"
./bin/nidhi --help | grep -q "trace-git"
./bin/nidhi --help | grep -q "directory"

# go.mod has correct module path
grep -q "module github.com/indrasvat/nidhi" go.mod

# go.mod has Go 1.26
grep -q "go 1.26" go.mod

# Charm v2 deps are present
grep -q "charm.land/bubbletea/v2" go.mod
grep -q "charm.land/bubbles/v2" go.mod
```

### Tooling
```bash
# Linter config is valid v2 schema
grep -q 'version: "2"' .golangci.yml

# Goreleaser config exists
test -f .goreleaser.yml

# Claude Code hooks exist
test -f .claude/settings.json

# Lint passes
make lint

# Tests pass (no tests yet = pass)
make test
```

### CI Pipeline
```bash
# Full CI pipeline
make ci
```

## Completion Criteria
1. `go mod tidy` exits 0 with no changes
2. `go build ./cmd/nidhi` exits 0
3. `make build` produces `bin/nidhi`
4. `./bin/nidhi --version` prints version string with commit and date
5. `./bin/nidhi --help` prints all flags from PRD section 12.5
6. `make ci` (lint + test) passes with exit 0
7. `.golangci.yml` uses v2 schema with all linters from PRD section 21.8
8. `.goreleaser.yml` matches PRD section 21.7
9. `.claude/settings.json` has PreToolUse and Stop hooks
10. All directories from PRD section 8.4 module structure exist
11. `go.mod` has `github.com/indrasvat/nidhi` module path with Go 1.26
12. Charm v2 deps (bubbletea, bubbles) are in go.mod with pinned versions

## Commit
```
chore: scaffold repository with Go module, CLI entrypoint, and tooling

Initialize go.mod (github.com/indrasvat/nidhi, Go 1.26) with pinned
Charm v2 dependencies. Add cmd/nidhi/main.go with --help and --version
flags. Add .golangci.yml (v2 schema), .goreleaser.yml, and Claude Code
hooks. All make targets (build, test, lint, ci) pass.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 4, 8.4, 12.5, 21.6-21.8
4. Execute steps 1-11 in order
5. Verify all functional, tooling, and CI checks pass
6. Update this file (Status: DONE) + `docs/PROGRESS.md`
7. Commit with the message above + move to task 001
