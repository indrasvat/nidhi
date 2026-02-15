# Task 034: Fix Detail Mode Interaction and All Key Routing Gaps

**Priority:** P0 — Critical (multiple features broken)
**Depends on:** 031, 033
**Blocks:** 036

## Problem

Detail mode key routing is completely broken — j/k, Tab, and arrow keys have zero effect.
Code review found 8 bugs across the key routing and footer systems.

## Bugs Found

### BUG 1: ModeDetail missing from core/app.go switch (CRITICAL)
`core/app.go:158-163` — `handleKeyPress()` only routes `ModeList` and `ModePreview`.
ModeDetail falls through to `delegateToPluginKeyHandlers()`, so j/k/Tab/arrows never reach `detail.go`.

### BUG 2: Tab intercepted globally, blocks Detail focus toggle (HIGH)
Core handles Tab for mode switching (List→Preview, Preview→List). Detail mode's Tab handler
(tree↔diff pane focus toggle in `detail.go:142-148`) is unreachable even with Bug 1 fixed
unless the new `handleDetailKeys` does NOT intercept Tab.

### BUG 3: Dead code Esc handler in detail.go (MEDIUM)
`detail.go:150-153` — Esc is handled globally in `core/app.go:153-154` via `popMode()`.
The detail screen's Esc case is dead code.

### BUG 4: `?` help hint missing from 7 mode footers (HIGH)
`footer.go` — `?` is a global key (works everywhere) but only ModeList shows `{"?", "help"}`.
Missing from: ModePreview, ModeDetail, ModeSearch, ModeNewStash, ModeExport, ModeImport, ModeConflict.

### BUG 5: No arrow key support in detail screen (MEDIUM)
`detail.go:155-172` — Only j/k handled, not up/down arrow keys.
Footer shows `↑↓ scroll`, implying arrow key support that doesn't exist.

### BUG 6: No arrow key support in preview screen (MEDIUM)
`preview.go:228-232` — Only j/k/g/G handled, no arrow equivalents.

### BUG 7: No arrow key support in list screen (LOW)
`list.go:166-219` — Only j/k/g/G handled, no arrow equivalents.

### BUG 8: Dead code Tab/Enter/Esc in list.go and preview.go (LOW)
Mode-switching keys in screen handlers are unreachable because core handles them first.
Harmless but confusing. Defer cleanup to later.

## Fix Strategy

### core/app.go
Add `handleDetailKeys()` that delegates ALL keys to UI (no Tab/Enter interception):
```go
case ModeDetail:
    return m.handleDetailKeys(msg)
```

### detail.go
- Remove dead Esc handler
- Add `tea.KeyUp`/`tea.KeyDown` arrow key support mirroring j/k behavior

### preview.go
- Add arrow key support for stash navigation (up/down mapped to k/j)

### list.go
- Add arrow key support for cursor navigation (up/down mapped to k/j)

### footer.go
- Add `{"?", "help"}` to all mode hint lists except ModeHelp

## Files to Modify

- `internal/core/app.go` — Add handleDetailKeys routing
- `internal/ui/screens/detail.go` — Arrow keys, remove dead Esc
- `internal/ui/screens/preview.go` — Arrow keys
- `internal/ui/screens/list.go` — Arrow keys
- `internal/ui/components/footer.go` — Add ? help hint to all modes

## Acceptance Criteria

- [ ] j/k navigate files in Detail mode file tree
- [ ] Tab toggles focus between tree and diff panes in Detail mode
- [ ] Arrow keys (up/down) work in all three modes (LIST, PREVIEW, DETAIL)
- [ ] `?` help hint visible in footer for ALL modes except HELP
- [ ] Esc from Detail returns to previous mode
- [ ] Verify with iTerm2 driver: j/k/Tab/arrows functional in Detail mode
