# Task 037: Detail Screen Polish — Focus Indicator, Gutter Fix, UX Improvements

**Status:** IN PROGRESS
**Branch:** `create-nidhi`
**Depends on:** Tasks 009, 012, 034

## Problem

The DETAIL screen (file tree + diff split view) has several visual/UX issues:

1. **No visual focus indicator** — Tab toggles keyboard focus between tree and diff, but there's no visual feedback showing which pane is active.
2. **Ghost line number in gutter** — Trailing empty string from `strings.Split()` creates a phantom line.
3. **Gutter separator doesn't extend to bottom** — The `│` stops at the last diff line, leaving blank rows below.
4. **No selected-file header** — The mockup shows a filename bar above the diff content (e.g., "LEDController.swift +68 −7").
5. **Divider doesn't indicate focus** — Static pipe divider between tree and diff.
6. **Empty categories shown** — "staged (0)" and "untracked (0)" waste vertical space when they have zero files.

## Changes

| File | What |
|------|------|
| `internal/ui/components/diffview.go` | Trim ghost lines, extend gutter, add focus state, add file header |
| `internal/ui/components/filetree.go` | Add focus state, dim unfocused cursor, hide empty categories |
| `internal/ui/screens/detail.go` | Wire focus to components, pass file name, style divider per focus |
| `*_test.go` | Updated tests for all changes |

## Acceptance Criteria

- [ ] Tab toggle shows visible focus indicator (gold highlight vs dimmed)
- [ ] No ghost empty line at bottom of diff
- [ ] Gutter `│` extends full viewport height
- [ ] File name header bar above diff content
- [ ] Empty categories hidden in file tree
- [ ] All existing tests pass
- [ ] `make build && make test` green
