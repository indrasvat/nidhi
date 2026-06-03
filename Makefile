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
.PHONY: build test lint check ci e2e bench bench-short profile perf-test install install-tools install-hooks clean release coverage coverage-check release-check release-dry-run install-script-test smoke-test release-prep help

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

## bench: Run performance benchmarks
bench:
	go test -bench=. -benchmem -timeout 600s ./internal/perf/...

## bench-short: Run quick performance benchmarks
bench-short:
	go test -bench=. -benchmem -benchtime=1s -timeout 120s ./internal/perf/...

## profile: Run benchmarks with CPU and memory profiling
profile:
	mkdir -p profiles
	go test -bench=BenchmarkStartup -benchmem -cpuprofile=profiles/cpu-startup.prof -memprofile=profiles/mem-startup.prof -timeout 120s ./internal/perf/...
	go test -bench=BenchmarkCursorMove -benchmem -cpuprofile=profiles/cpu-cursor.prof -memprofile=profiles/mem-cursor.prof -timeout 120s ./internal/perf/...
	@echo "Profiles written to profiles/. View with: go tool pprof profiles/<file>.prof"

## perf-test: Run performance validation tests (latency and memory assertions)
perf-test:
	go test -v -timeout 600s -count=1 -run 'Test(Startup|OperationLatency|Memory)_' ./internal/perf/...

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

## coverage: Generate coverage report
coverage: test
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## coverage-check: Check coverage meets minimum threshold
coverage-check: coverage
	@echo "Checking coverage thresholds..."
	@CORE_COV=$$(go tool cover -func=coverage.out | grep 'internal/core/' | awk '{sum+=$$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}'); \
	GIT_COV=$$(go tool cover -func=coverage.out | grep 'internal/git/' | awk '{sum+=$$NF; n++} END {if(n>0) printf "%.1f", sum/n; else print "0.0"}'); \
	echo "internal/core coverage: $$CORE_COV%"; \
	echo "internal/git coverage: $$GIT_COV%"; \
	echo "$$CORE_COV 70.0" | awk '{if ($$1 < $$2) {print "FAIL: internal/core below 70%"; exit 1}}'; \
	echo "$$GIT_COV 70.0" | awk '{if ($$1 < $$2) {print "FAIL: internal/git below 70%"; exit 1}}'

## release-check: Validate goreleaser configuration
release-check:
	goreleaser check
	@echo "goreleaser config is valid"

## release-dry-run: Dry-run goreleaser (build but don't publish)
release-dry-run:
	goreleaser release --snapshot --clean

## install-script-test: Test install.sh against a local fake GitHub release
install-script-test:
	./scripts/test-install.sh

## smoke-test: Run release smoke test
smoke-test: build
	./scripts/smoke-test.sh

## release-prep: Full release preparation (ci + e2e + bench + coverage + release + installer + smoke test)
release-prep: ci e2e bench coverage-check release-check install-script-test smoke-test
	@echo ""
	@echo "=== Release preparation complete ==="
	@echo "All checks passed. Ready to tag and release."
	@echo ""
	@echo "Next steps:"
	@echo "  1. git tag -a v0.1.0 -m 'Phase 1: First Light'"
	@echo "  2. git push origin v0.1.0"
	@echo "  3. Monitor: https://github.com/indrasvat/nidhi/actions"
	@echo "  4. Verify: https://github.com/indrasvat/nidhi/releases"

## help: Show this help
help:
	@echo "nidhi — build targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
