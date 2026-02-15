# nidhi — Makefile
# Purpose-built TUI for git stash mastery

# ─── Config ───────────────────────────────────────────────
BINARY     := nidhi
MODULE     := github.com/indrasvat/nidhi
BUILD_DIR  := bin
INSTALL_DIR := $(HOME)/.local/bin
CMD_DIR    := ./cmd/nidhi

# Build metadata (injected via ldflags)
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE       := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Tools
GOTESTSUM  := $(shell command -v gotestsum 2>/dev/null)
GOLANGCI   := $(shell command -v golangci-lint 2>/dev/null)
LEFTHOOK   := $(shell command -v lefthook 2>/dev/null)

# ─── Targets ──────────────────────────────────────────────
.PHONY: build test lint check ci e2e install install-tools install-hooks clean release coverage help

## build: Compile binary to bin/nidhi
build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)
	@echo "✓ Built $(BUILD_DIR)/$(BINARY)"

## test: Run all tests with race detection and coverage
test:
ifdef GOTESTSUM
	gotestsum -- -race -coverprofile=coverage.out ./...
else
	@echo "⚠ gotestsum not found, falling back to go test"
	go test -race -coverprofile=coverage.out ./...
endif

## lint: Run golangci-lint
lint:
ifdef GOLANGCI
	golangci-lint run
else
	@echo "✗ golangci-lint not found. Run: make install-tools" && exit 1
endif

## check: Run lint + test (pre-commit target)
check: lint test

## ci: CI-only target (lint + test, fail-fast, no fallbacks)
ci: lint test

## e2e: Run end-to-end tests
e2e:
ifdef GOTESTSUM
	gotestsum -- -race -v -count=1 ./internal/e2e/...
else
	go test -race -v -count=1 ./internal/e2e/...
endif

## install: Install binary to ~/.local/bin
install: build
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "✓ Installed to $(INSTALL_DIR)/$(BINARY)"

## install-tools: Install dev dependencies
install-tools:
	@echo "Installing dev tools..."
	go install gotest.tools/gotestsum@latest
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install github.com/goreleaser/goreleaser/v2@latest
	@echo "✓ Dev tools installed"
	@echo ""
	@echo "Also install lefthook (git hooks manager):"
	@echo "  brew install lefthook    # macOS"
	@echo "  go install github.com/evilmartians/lefthook@latest  # or via go"

## install-hooks: Install lefthook git hooks
install-hooks:
ifdef LEFTHOOK
	lefthook install
	@echo "✓ Git hooks installed"
else
	@echo "✗ lefthook not found. Run: brew install lefthook" && exit 1
endif

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR) coverage.out
	@echo "✓ Cleaned"

## release: Build release with goreleaser
release:
	goreleaser release --clean

## coverage: Open coverage report in browser
coverage: test
	go tool cover -html=coverage.out

## help: Show this help
help:
	@echo "nidhi — build targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
