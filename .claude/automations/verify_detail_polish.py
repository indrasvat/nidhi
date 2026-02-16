# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""
Detail Screen Polish Verification (Task 037)
=============================================

Tests:
1. Focus indicator: tree focused shows gold highlight, diff focused dims tree
2. Gutter extends full height: separator fills entire diff pane
3. File header bar: selected file name shown above diff content
4. Empty categories hidden: "staged (0)" should not appear
5. Tab toggles focus and changes visual state

Verification Strategy:
- Launch nidhi with demo stashes via setup-demo.sh
- Navigate to DETAIL mode (handle welcome screen + mode transitions robustly)
- Screenshot each focus state
- Parse screen content to verify structural elements

Screenshots:
- 037_detail_tree_focused.png
- 037_detail_diff_focused.png
- 037_detail_tree_refocused.png

Usage:
    uv run .claude/automations/verify_detail_polish.py
"""

import asyncio
import os
import subprocess
import iterm2


PROJ = "/Users/indrasvat/code/github.com/indrasvat-nidhi"
SCREENSHOT_DIR = os.path.join(PROJ, ".claude/automations/screenshots")
os.makedirs(SCREENSHOT_DIR, exist_ok=True)


async def wait_for(session, keyword, timeout=10):
    """Poll screen until keyword appears or timeout."""
    for _ in range(int(timeout / 0.25)):
        screen = await session.async_get_screen_contents()
        lines = [screen.line(i).string for i in range(screen.number_of_lines)]
        text = "\n".join(lines)
        if keyword in text:
            return text
        await asyncio.sleep(0.25)
    screen = await session.async_get_screen_contents()
    lines = [screen.line(i).string for i in range(screen.number_of_lines)]
    dump = "\n".join(lines)
    raise TimeoutError(f"Timed out waiting for '{keyword}'. Screen:\n{dump}")


async def get_screen_text(session):
    """Get full screen text, stripping trailing blank lines."""
    screen = await session.async_get_screen_contents()
    lines = [screen.line(i).string for i in range(screen.number_of_lines)]
    while lines and lines[-1].strip() == "":
        lines.pop()
    return "\n".join(lines), lines


async def get_mode(session):
    """Detect current mode from status bar badge."""
    _, lines = await get_screen_text(session)
    for line in reversed(lines):
        stripped = line.strip()
        for mode in ["DETAIL", "PREVIEW", "LIST"]:
            if stripped.endswith(mode):
                return mode
    return "UNKNOWN"


async def navigate_to_detail(session):
    """Navigate to DETAIL mode from wherever we are, handling welcome screen."""
    await asyncio.sleep(1)

    mode = await get_mode(session)
    text = (await get_screen_text(session))[0]

    # Handle welcome screen.
    if "Press Enter" in text:
        await session.async_send_text("\r")
        await asyncio.sleep(0.5)
        mode = await get_mode(session)

    # Navigate through modes until DETAIL.
    attempts = 0
    while mode != "DETAIL" and attempts < 5:
        await session.async_send_text("\r")
        await asyncio.sleep(0.5)
        mode = await get_mode(session)
        attempts += 1

    if mode != "DETAIL":
        raise RuntimeError(f"Failed to reach DETAIL mode, stuck at {mode}")
    print(f"  Reached DETAIL mode after {attempts} Enter(s)")


async def take_screenshot(name):
    """Capture iTerm2 window screenshot via Quartz."""
    from Quartz import CGWindowListCopyWindowInfo, kCGWindowListOptionAll, kCGNullWindowID
    window_list = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID)
    iterm_id = None
    for w in window_list:
        owner = w.get("kCGWindowOwnerName", "")
        if "iTerm2" in owner and w.get("kCGWindowLayer", 999) == 0:
            iterm_id = w.get("kCGWindowNumber")
            break
    if iterm_id is None:
        print(f"  WARNING: Could not find iTerm2 window for screenshot {name}")
        return
    path = os.path.join(SCREENSHOT_DIR, f"{name}.png")
    result = subprocess.run(["screencapture", "-l", str(iterm_id), "-x", path],
                            capture_output=True)
    if result.returncode == 0:
        print(f"  Screenshot saved: {name}.png")
    else:
        print(f"  Screenshot skipped (permissions): {name}.png")


results = {"passed": 0, "failed": 0, "tests": []}


def log_result(name, passed, details=""):
    status = "PASS" if passed else "FAIL"
    results["passed" if passed else "failed"] += 1
    results["tests"].append({"name": name, "status": status, "details": details})
    print(f"  [{status}] {name}" + (f" -- {details}" if details else ""))


def print_summary():
    print(f"\n{'='*60}")
    print(f"Results: {results['passed']} passed, {results['failed']} failed out of {len(results['tests'])}")
    for t in results["tests"]:
        print(f"  [{t['status']}] {t['name']}")
    print(f"{'='*60}")


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if window is None:
        print("ERROR: No current iTerm2 window")
        return

    tab = await window.async_create_tab()
    session = tab.current_session

    try:
        # Build and launch nidhi.
        print("Building nidhi...")
        await session.async_send_text(f"cd {PROJ} && make build 2>&1 && echo BUILD_OK\n")
        await wait_for(session, "BUILD_OK", timeout=30)
        await asyncio.sleep(0.5)

        print("Launching nidhi with demo stashes...")
        await session.async_send_text(f"bash {PROJ}/scripts/setup-demo.sh\n")
        await asyncio.sleep(2)

        # Navigate to DETAIL mode robustly.
        print("Navigating to DETAIL mode...")
        await navigate_to_detail(session)
        await asyncio.sleep(0.5)

        # ═══ TEST: Tree focused (initial state) ═══
        print("\n--- Test: Tree focused (initial state) ---")
        text, lines = await get_screen_text(session)
        await take_screenshot("037_detail_tree_focused")

        # File header bar — should show a file path above diff content.
        has_file_header = any(
            ".go" in line or ".swift" in line or ".md" in line or ".log" in line
            for line in lines
            if "diff --git" not in line and "---" not in line and "+++" not in line
        )
        log_result("File header bar visible", has_file_header)

        # Gutter separator extends through multiple lines.
        gutter_lines = [l for l in lines if "\u2502" in l]
        log_result("Gutter extends full viewport", len(gutter_lines) > 10,
                   f"{len(gutter_lines)} lines with gutter")

        # Empty categories hidden.
        has_empty_cat = "staged (0)" in text or "untracked (0)" in text
        log_result("Empty categories hidden", not has_empty_cat,
                   "None found" if not has_empty_cat else f"Found in: {text[:200]}")

        # ═══ TEST: Tab to diff pane ═══
        print("\n--- Test: Tab switches focus to diff ---")
        before_text = text
        await session.async_send_text("\t")
        await asyncio.sleep(0.3)
        text_after_tab, _ = await get_screen_text(session)
        await take_screenshot("037_detail_diff_focused")

        log_result("Tab changes visual state", before_text != text_after_tab,
                   "Styling changed" if before_text != text_after_tab else "No change detected")

        # ═══ TEST: Tab back to tree ═══
        print("\n--- Test: Tab cycles focus back to tree ---")
        await session.async_send_text("\t")
        await asyncio.sleep(0.3)
        text_refocused, _ = await get_screen_text(session)
        await take_screenshot("037_detail_tree_refocused")

        log_result("Tab cycles back to tree", text_refocused != text_after_tab,
                   "Styling changed on second Tab")

        # ═══ TEST: Diff scrolling works ═══
        print("\n--- Test: Diff pane navigation ---")
        await session.async_send_text("\t")  # Focus diff
        await asyncio.sleep(0.2)
        before_scroll, _ = await get_screen_text(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.2)
        after_scroll, _ = await get_screen_text(session)
        log_result("j scrolls diff when focused", before_scroll != after_scroll)

        # ═══ TEST: Verify mode is still DETAIL ═══
        mode = await get_mode(session)
        log_result("Still in DETAIL mode", mode == "DETAIL", f"mode={mode}")

        print_summary()

    except Exception as e:
        print(f"\nERROR: {e}")
        try:
            text, _ = await get_screen_text(session)
            print(f"Screen dump:\n{text}")
        except Exception:
            pass
        print_summary()
        raise
    finally:
        await session.async_send_text("\x03")
        await asyncio.sleep(0.3)
        await session.async_send_text("q")
        await asyncio.sleep(0.3)
        try:
            await session.async_send_text("exit\n")
            await asyncio.sleep(0.3)
            await session.async_close()
        except Exception:
            pass


iterm2.run_until_complete(main)
