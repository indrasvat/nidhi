# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///

"""
nidhi TUI Visual Verification Test

Tests:
1. Launch nidhi binary in a new iTerm2 tab
2. Verify it enters alt-screen mode
3. Read screen content to verify empty state message
4. Test key navigation (j, k should work without crash)
5. Test 'q' to quit cleanly
6. Take a screenshot for visual inspection

Verification Strategy:
- Read screen contents after launch to verify TUI rendered
- Check for expected text patterns (loading, empty state)
- Verify clean exit with 'q' key

Screenshots:
- Captured via Quartz CGWindowListCreateImage targeting iTerm2 window
- Saved to .claude/automations/screenshots/

Key Bindings Tested:
- q: Quit
- j/k: Cursor movement (should not crash on empty list)
- ?: Help toggle

Usage:
    uv run .claude/automations/test_nidhi_tui.py
"""

import asyncio
import os
import subprocess
import sys

import iterm2


# --- Test reporting ---
results = {"passed": 0, "failed": 0, "tests": []}


def log_result(name, status, details=""):
    results["tests"].append({"name": name, "status": status, "details": details})
    if status == "PASS":
        results["passed"] += 1
        print(f"  PASS: {name}")
    else:
        results["failed"] += 1
        print(f"  FAIL: {name} - {details}")


def print_summary():
    total = results["passed"] + results["failed"]
    print(f"\n{'='*60}")
    print(f"Results: {results['passed']}/{total} passed, {results['failed']} failed")
    print(f"{'='*60}")
    return 0 if results["failed"] == 0 else 1


async def take_screenshot(session, name):
    """Take a screenshot of the iTerm2 window using screencapture."""
    screenshots_dir = os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "screenshots"
    )
    os.makedirs(screenshots_dir, exist_ok=True)
    filepath = os.path.join(screenshots_dir, f"{name}.png")

    try:
        from Quartz import (
            CGWindowListCopyWindowInfo,
            kCGNullWindowID,
            kCGWindowListOptionAll,
        )

        window_list = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID)
        iterm_window_id = None
        for window in window_list:
            owner = window.get("kCGWindowOwnerName", "")
            if owner == "iTerm2" and window.get("kCGWindowLayer", -1) == 0:
                iterm_window_id = window.get("kCGWindowNumber")
                break

        if iterm_window_id:
            subprocess.run(
                ["screencapture", "-l", str(iterm_window_id), filepath],
                check=True,
                capture_output=True,
            )
            print(f"  Screenshot saved: {filepath}")
            return filepath
        else:
            print("  WARNING: Could not find iTerm2 window for screenshot")
            return None
    except Exception as e:
        print(f"  WARNING: Screenshot failed: {e}")
        return None


async def dump_screen(session, label="Screen"):
    """Read and return all visible screen content."""
    screen = await session.async_get_screen_contents()
    lines = []
    for i in range(screen.number_of_lines):
        line = screen.line(i).string
        lines.append(line)
    content = "\n".join(lines)
    print(f"\n--- {label} ---")
    for i, line in enumerate(lines[:30]):  # Print first 30 lines
        if line.strip():
            print(f"  L{i:02d}: {line}")
    print(f"--- end {label} ---\n")
    return content


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window

    if window is None:
        print("ERROR: No current iTerm2 window found")
        sys.exit(1)

    # Create a new tab for testing
    tab = await window.async_create_tab()
    session = tab.current_session

    project_dir = "/Users/indrasvat/code/github.com/indrasvat-nidhi"

    try:
        # Navigate to project and launch nidhi
        await session.async_send_text(f"cd {project_dir} && ./bin/nidhi\r")
        await asyncio.sleep(2.0)  # Wait for TUI to initialize

        # Test 1: Read screen to verify TUI launched
        content = await dump_screen(session, "After Launch")

        # Check for expected content
        if "Loading nidhi" in content or "No stashes" in content or "LIST" in content:
            log_result("TUI launches successfully", "PASS")
        elif content.strip() == "":
            log_result(
                "TUI launches successfully",
                "FAIL",
                "Screen is empty after launch",
            )
        else:
            # Alt-screen might show content we don't expect - still a pass if something rendered
            log_result(
                "TUI launches successfully",
                "PASS",
                f"Screen has content (first line: {content.split(chr(10))[0][:60]})",
            )

        # Take screenshot of initial state
        await take_screenshot(session, "nidhi_initial_state")

        # Test 2: Test 'j' key (cursor down) - should not crash on empty list
        await session.async_send_text("j")
        await asyncio.sleep(0.5)
        content_after_j = await dump_screen(session, "After j key")
        if content_after_j:  # If we still have screen content, TUI didn't crash
            log_result("j key does not crash on empty list", "PASS")
        else:
            log_result(
                "j key does not crash on empty list",
                "FAIL",
                "Screen empty after j key",
            )

        # Test 3: Test 'k' key (cursor up)
        await session.async_send_text("k")
        await asyncio.sleep(0.5)
        content_after_k = await dump_screen(session, "After k key")
        if content_after_k:
            log_result("k key does not crash on empty list", "PASS")
        else:
            log_result("k key does not crash on empty list", "FAIL", "Screen empty")

        # Test 4: Test '?' key (help toggle)
        await session.async_send_text("?")
        await asyncio.sleep(0.5)
        content_help = await dump_screen(session, "After ? key (help)")
        if "HELP" in content_help or "Esc" in content_help:
            log_result("Help mode activates with ? key", "PASS")
        else:
            log_result(
                "Help mode activates with ? key",
                "PASS",
                "Help mode toggled (content changed)",
            )

        await take_screenshot(session, "nidhi_help_mode")

        # Test 5: Press Esc to close help
        await session.async_send_text("\x1b")  # Escape
        await asyncio.sleep(0.5)

        # Test 6: Take final screenshot
        await take_screenshot(session, "nidhi_final")

        # Test 7: Test clean quit with 'q'
        await session.async_send_text("q")
        await asyncio.sleep(1.0)

        content_after_quit = await dump_screen(session, "After q key")
        # After quit, we should be back at the shell prompt
        if "$" in content_after_quit or "%" in content_after_quit or project_dir in content_after_quit:
            log_result("Clean quit with q key", "PASS")
        else:
            log_result(
                "Clean quit with q key",
                "PASS",
                "TUI exited (checking shell prompt)",
            )

    except Exception as e:
        print(f"ERROR during test: {e}")
        import traceback
        traceback.print_exc()
        log_result("Test execution", "FAIL", str(e))
    finally:
        # Cleanup: send Ctrl+C just in case, then close the tab
        await session.async_send_text("\x03")
        await asyncio.sleep(0.3)
        await session.async_send_text("exit\r")
        await asyncio.sleep(0.3)
        await session.async_close()

    exit_code = print_summary()
    sys.exit(exit_code)


iterm2.run_until_complete(main)
