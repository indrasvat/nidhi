# nidhi — Performance Profiling Results

> This document records measured performance against PRD §7.1 and §14 targets.
> Updated after each profiling run.

## Test Environment

| Field | Value |
|---|---|
| Date | 2026-02-14 |
| Machine | Apple M-series, macOS Darwin 25.2.0 |
| Go | 1.26 |
| Git | 2.53.0 |
| OS | macOS 15.x |

## Startup Time (PRD §7.1, §14.1)

| Stash Count | Target | Measured | Status |
|---|---|---|---|
| 0 stashes | < 100ms | 15ms | PASS |
| 5 stashes | < 100ms | 21ms | PASS |
| 20 stashes | < 100ms | 24ms | PASS |
| 50 stashes | < 200ms | 26ms | PASS |
| 100 stashes | < 300ms | 31ms | PASS |

`--debug` startup timing: 48ms total (git detection 9.8ms, plugin init 1.5ms, model creation 1.5ms).

## Operation Latency (PRD §14.2)

| Operation | Target | Measured | Status |
|---|---|---|---|
| Cursor move + render (j/k) | < 1ms | 587us | PASS |
| LIST render (50 stashes) | < 100ms | 624us | PASS |
| Diff load (uncached) | < 200ms | ~7ms | PASS |
| Apply stash | < 500ms | 151ms | PASS |
| Rename (drop+store) | < 100ms | 13ms | PASS |
| Search index build (50 stashes) | < 2s | 971ms | PASS |

## Memory Usage (PRD §7.1)

| Scenario | Target | Measured | Status |
|---|---|---|---|
| 50 stashes + 10 diffs loaded | < 50MB RSS | 168 KB heap | PASS |
| Diff cache (limit 10) | Respects limit | 10/10 | PASS |
| After 100 operations | No leak (< 10MB growth) | 0 B growth | PASS |
| Total allocations (50 stashes) | Reasonable | 2.6 MB | PASS |

## How to Run

```bash
# Full benchmark suite
make bench

# Performance validation tests (with pass/fail assertions)
make perf-test

# CPU and memory profiling
make profile
go tool pprof profiles/cpu-startup.prof

# Visual responsiveness (macOS + iTerm2)
NIDHI_VISUAL_TEST=1 go test -v -run TestVisualResponsiveness ./internal/perf/...
```
