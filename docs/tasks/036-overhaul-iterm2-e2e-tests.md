# Task 036: Overhaul iTerm2 E2E Tests for Real Interaction Testing

**Priority:** P0 — Critical (testing infrastructure broken)
**Depends on:** 034, 035

## Problem

Current iTerm2 automation scripts are superficial — they verify "can I enter a mode" but never
exercise the actual key interactions within each mode. This allowed broken detail-mode scrolling,
missing footer hints, and missing version display to ship undetected.

### What Current Tests DON'T Test
- j/k navigation within DETAIL mode (never tested)
- Tab focus toggle between tree↔diff (never tested)
- Arrow key navigation in any mode (never tested)
- Footer hint content matches the current mode (never tested)
- `?` help accessible from PREVIEW and DETAIL modes (never tested)
- Version info visible in status bar (checked but not enforced)
- Diff pane content changes when selecting different files (never tested)

### Root Cause
The scripts were written to pass, not to catch bugs. They check for keyword presence
("working", "staged", "DETAIL") without verifying that key presses produce expected state changes.

## Fix Strategy

Rewrite `comprehensive_tui_test.py` with INTERACTION-BASED testing:

### Test Matrix (minimum coverage)

| Mode | Action | Expected Result | Verification |
|------|--------|-----------------|--------------|
| LIST | j/k | Cursor moves | Screen content changes |
| LIST | ↑/↓ | Cursor moves | Screen content changes |
| LIST | Tab | Enter PREVIEW | Footer shows PREVIEW badge |
| LIST | ? | Open HELP | Help overlay visible |
| PREVIEW | j/k | Stash changes, diff reloads | Diff content changes |
| PREVIEW | Tab | Return to LIST | Footer shows LIST badge |
| PREVIEW | Enter | Enter DETAIL | Footer shows DETAIL badge |
| PREVIEW | ? | Open HELP | Help overlay visible |
| DETAIL | j/k | File selection changes | Left pane cursor moves |
| DETAIL | ↑/↓ | File selection changes | Left pane cursor moves |
| DETAIL | Tab | Focus toggles tree↔diff | Screen content changes |
| DETAIL | ? | Open HELP | Help overlay visible |
| DETAIL | Esc | Return to prev mode | Footer shows prev badge |
| ALL | Footer | Has `?` hint | "help" in footer line |
| ALL | Status | Has version info | Version string in top line |

### Test Implementation Pattern
```python
async def test_detail_jk_navigation(session):
    # Enter detail mode
    await send_key(session, "\r")  # Enter
    await wait(1.5)

    # Read initial state
    before = await read_screen(session)

    # Press j (should move file cursor)
    await send_key(session, "j")
    await wait(0.3)
    after_j = await read_screen(session)

    # Verify CONTENT CHANGED (not just that a keyword exists)
    assert before != after_j, "j key should change screen in DETAIL mode"
```

## Files to Modify

- `.claude/automations/comprehensive_tui_test.py` — Complete rewrite

## Acceptance Criteria

- [ ] Tests exercise j/k in LIST, PREVIEW, and DETAIL modes
- [ ] Tests exercise arrow keys in all three modes
- [ ] Tests verify Tab focus toggle in DETAIL mode
- [ ] Tests verify `?` opens help from PREVIEW and DETAIL
- [ ] Tests verify footer `?` hint present in all modes
- [ ] Tests verify version info in status bar
- [ ] Tests verify screen CONTENT CHANGES on key press (not just keyword search)
- [ ] Failed test produces clear output showing expected vs actual
