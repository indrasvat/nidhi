# Task 032: Fix Terminal Background Color Bleed

**Priority:** P0 — Critical (visual quality)
**Depends on:** None
**Blocks:** 037

## Problem

Background color bleed is visible in every screen. Empty terminal cells (below the stash list, trailing whitespace, empty lines) show the terminal's default dark background instead of the Agni theme's `bg.deep` (#07090E). This creates a "two-tone" appearance: styled content has the correct background, empty areas have a different shade.

The iterm2-driver screen dumps show `\x00` null bytes for empty cells, confirming they have no styled content.

## Root Cause

From yukti CLAUDE.md (lines 192-244):
- `lipgloss.Background()` only applies to **explicitly rendered characters**
- Empty terminal cells have NO characters, so they use the terminal's default background
- Padding with styled spaces doesn't reliably fill all empty cells

## Fix: termenv.SetBackgroundColor()

Use the `termenv` library (already a BubbleTea transitive dependency) to set the terminal's default background color via OSC 11 escape sequence:

```go
import "github.com/muesli/termenv"

func run(flags config.CLIFlags) error {
    // Set terminal background BEFORE starting BubbleTea
    output := termenv.NewOutput(os.Stdout)
    output.SetBackgroundColor(output.Color("#07090E"))  // Agni bg.deep

    // ... create model, etc ...

    p := tea.NewProgram(model)
    _, err := p.Run()

    // Reset terminal colors AFTER TUI exits
    output.Reset()

    return err
}
```

**Why this works:**
- OSC 11 (`\033]11;#RRGGBB\007`) changes the terminal's **default** background
- ALL cells (including empty ones) now use Agni's background
- `output.Reset()` restores original colors when app exits
- BubbleTea's alternate screen mode keeps main terminal unaffected

## Files to Modify

- `cmd/nidhi/main.go` — Add termenv.SetBackgroundColor before tea.NewProgram, output.Reset after Run
- `go.mod` — May need explicit `github.com/muesli/termenv` import (likely already transitive)

## Critical Notes (from yukti learnings)

- Call `output.Reset()` BEFORE any `os.Exit()` — defer won't run after os.Exit
- The color string format should match the Agni theme bg.deep: `"#07090E"`
- Works with iTerm2, Terminal.app, Alacritty, Kitty, and most modern terminals

## Acceptance Criteria

- [ ] No visible background color difference between styled content and empty areas
- [ ] Terminal background matches Agni bg.deep (#07090E)
- [ ] Terminal colors properly restored when nidhi exits (via q or Ctrl+C)
- [ ] Works correctly with alternate screen mode
- [ ] Verify with iterm2-driver screenshot: uniform background color
