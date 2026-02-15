# Task 031: Fix Cursor Navigation and Key Event Routing

**Priority:** P0 — Critical (TUI is broken)
**Depends on:** None
**Blocks:** 035, 037

## Problem

j/k cursor navigation does not work. Pressing j/k does NOT move the visual cursor indicator (▸). Screenshots before and after j press are identical. The cursor appears stuck on stash 0.

## Root Cause

**Dual cursor management** — there are TWO independent cursor states:

1. **Core model** (`internal/core/app.go` lines 178-192): `handleListKeys()` handles j/k by calling `WithCursor(m.state, m.state.Cursor+1)`, updating `state.Cursor`.

2. **ListScreen** (`internal/ui/screens/list.go`): Has its OWN `l.cursor` and `l.offset` fields with proper `moveCursor()`, `clampCursor()`, scroll management.

The core model catches j/k FIRST in `handleListKeys()` and returns without forwarding to the UIRenderer. The ListScreen never receives the key events, so `l.cursor` never changes. Since `ListScreen.View()` uses `l.cursor` for rendering the ▸ indicator and row highlighting, the visual cursor stays frozen.

## Fix Strategy

**Option A (Recommended): Delegate LIST/PREVIEW key handling to UIRenderer**

Instead of having core handle j/k/g/G directly, forward these to UIRenderer.HandleMessage() which routes to ListScreen.handleKey().

1. In `core/app.go`, remove j/k/g/G/Tab/Enter handling from `handleListKeys()` and `handlePreviewKeys()`.
2. In the default branch of `handleKeyPress()`, forward all key events to `m.UI.HandleMessage()` when in LIST or PREVIEW mode.
3. In `cmd/nidhi/main.go`, `uiRenderer.HandleMessage()` routes `tea.KeyPressMsg` to `u.list.Update()` or `u.preview.Update()` based on mode.
4. ListScreen and PreviewScreen already have proper key handling with `moveCursor()`, scroll offset, etc.

**Option B: Sync state.Cursor → ListScreen on every render**

Keep core handling but sync the cursor position before rendering. Less clean architecturally.

## Files to Modify

- `internal/core/app.go` — Remove LIST/PREVIEW-specific key handlers from core; delegate to UI
- `cmd/nidhi/main.go` — Update UIRenderer.HandleMessage to route key events
- Potentially `internal/ui/screens/list.go` — Ensure Update/handleKey returns proper state updates

## Acceptance Criteria

- [ ] j/k moves the ▸ cursor indicator between stash rows
- [ ] g jumps to first stash, G jumps to last
- [ ] Scroll offset updates when cursor moves beyond visible rows
- [ ] In PREVIEW mode, j/k moves cursor AND updates the diff preview
- [ ] In DETAIL mode, navigation works for file tree
- [ ] All 933 existing tests still pass
- [ ] Verify with iterm2-driver: take screenshot before and after j, confirm ▸ moved

## Testing

```bash
make test
uv run .claude/automations/comprehensive_tui_test.py
```

## Yukti Learnings Applied

- **Switch type assertion shadowing** (yukti CLAUDE.md lines 419-462): If modifying the Update switch, use `switch typedMsg := msg.(type)` not `switch msg := msg.(type)` to avoid shadowing the outer msg variable.
