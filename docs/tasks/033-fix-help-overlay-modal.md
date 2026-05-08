# Task 033: Fix Help Overlay Modal Rendering

**Priority:** P0 — Critical (feature broken)
**Depends on:** 032 (background color fix helps overlay rendering)
**Blocks:** 037

## Problem

Pressing `?` changes the footer to "esc close HELP" but the help overlay does NOT render as a centered modal. The screen looks identical to the LIST view (same stash rows, same layout) with just the footer changed. No keybind reference, no modal box, no dimmed background.

The PRD mockup (Screen 10) and HTML mockup show a centered modal with:
- Title: "◆ nidhi keybindings"
- 2-column grid of keybind categories (Navigation, Actions, Search & Filter, Export/Import)
- Mode badges row at the bottom
- Version info: "nidhi v0.1.0 · Go 1.26 · BubbleTea v2 · Git ≥ 2.38"
- Dimmed list view visible in the background
- Border with gold accent

## Root Cause Analysis

The `uiRenderer.RenderContent()` (cmd/nidhi/main.go) does call `u.help.RenderWithDimmedBackground()` when in ModeHelp. But this function likely has issues:

1. **Missing ansi.Cut compositing** — The overlay may not be compositing correctly over the dimmed background. From yukti learnings, simple string replacement doesn't work; you need ANSI-aware slicing.

2. **Height/width issues** — The help content may not be properly centered or sized.

3. **Empty string padding** — If background lines are padded with empty strings, `ansi.Cut()` returns empty (yukti Bug 2, lines 464-501).

## Fix Strategy (from yukti learnings)

### 1. Use ansi.Cut for modal compositing

```go
import "github.com/charmbracelet/x/ansi"

func composeModalLine(bgLine, modalLine string, leftOffset, modalWidth, totalWidth int) string {
    leftPart := ansi.Cut(bgLine, 0, leftOffset)
    rightStart := leftOffset + modalWidth
    rightPart := ansi.Cut(bgLine, rightStart, totalWidth)
    return leftPart + "\033[0m" + modalLine + "\033[0m" + rightPart
}
```

### 2. Ensure exact height with full-width padding

```go
func ensureExactHeight(content string, height, width int) string {
    lines := strings.Split(content, "\n")
    if len(lines) > height { lines = lines[:height] }
    emptyLine := strings.Repeat(" ", width)
    for len(lines) < height { lines = append(lines, emptyLine) }
    return strings.Join(lines, "\n")
}
```

### 3. Dim background content
Apply dimming by re-styling or reducing foreground brightness of background lines.

### 4. Render modal box with borders
Build the modal content with:
- Rounded border (╭/╮/╰/╯)
- Title centered at top
- 2-column keybind grid
- Mode badges
- Version info at bottom

## Files to Modify

- `internal/ui/screens/help.go` — Fix RenderWithDimmedBackground to use ansi.Cut compositing
- `internal/ui/screens/help.go` — Add version info parameter, improve keybind content layout
- `cmd/nidhi/main.go` — Pass nidhi version info to help overlay

## Global Key Handling with Modals (yukti pattern)

From yukti CLAUDE.md lines 325-352: When help overlay is open, `?` should close it, and `Esc` should close it. Global key handlers must check if a modal is open before intercepting keys.

Current code in `core/app.go` already handles this:
```go
case msg.Text == "?":
    if m.modes.Current() == ModeHelp {
        m.popMode()
    } else {
        cmd := m.pushMode(ModeHelp)
        return m, cmd
    }
```

This is correct — `?` toggles help on/off.

## Acceptance Criteria

- [ ] `?` opens a centered modal overlay with dimmed list background visible behind it
- [ ] Modal shows keybind categories: Navigation, Actions, Search & Filter, Export/Import
- [ ] Modal shows mode badges row (LIST, PREVIEW, DETAIL, SEARCH, NEW, EXPORT, CONFIRM, HELP)
- [ ] Modal shows version info at bottom: "nidhi {version} · Git {version}"
- [ ] `Esc` or `?` closes the help modal, returning to previous view
- [ ] Help modal accessible from ALL modes (LIST, PREVIEW, DETAIL)
- [ ] Verify with iterm2-driver: screenshot shows centered modal with keybinds

## Yukti Learnings Applied

- **ansi.Cut for modal overlays** — ANSI-aware string slicing for compositing
- **Full-width padding** — Use `strings.Repeat(" ", width)`, never empty strings
- **Height management** — MaxHeight + ensureExactHeight pattern
- **Reset between segments** — Add `\033[0m` resets between bg and modal parts
- **Testing modals** — Verify both top and bottom borders visible, check by line position
