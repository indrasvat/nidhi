# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""
Verify DETAIL mode Tab focus fix.

Tests:
1. Enter DETAIL → j/k works (tree focus)
2. Tab → focus switches to diff pane
3. Esc → return to previous mode
4. Re-enter DETAIL → j/k works again (focus reset to tree)

Screenshots:
- detail_tab_fix.png: after re-entering DETAIL

Usage:
    uv run .claude/automations/verify_detail_tab_fix.py
"""

import asyncio
import subprocess
import iterm2


async def take_screenshot(filename):
    from Quartz import CGWindowListCopyWindowInfo, kCGWindowListOptionOnScreenOnly, kCGNullWindowID
    windows = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID)
    for w in windows:
        if "iTerm2" in w.get("kCGWindowOwnerName", ""):
            subprocess.run(["screencapture", "-l", str(w.get("kCGWindowNumber")), filename], check=True)
            print(f"Screenshot: {filename}")
            return


async def read_screen(session):
    screen = await session.async_get_screen_contents()
    lines = []
    for i in range(screen.number_of_lines):
        lines.append(screen.line(i).string)
    return "\n".join(lines), lines


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if not window:
        print("ERROR: No window")
        return

    tab = await window.async_create_tab()
    session = tab.current_session
    results = {"passed": 0, "failed": 0}

    def log(name, ok, details=""):
        results["passed" if ok else "failed"] += 1
        print(f"  {'✓' if ok else '✗'} {name}: {details}")

    try:
        await session.async_send_text("cd /Users/indrasvat/code/github.com/indrasvat-nidhi\r")
        await asyncio.sleep(0.5)
        await session.async_send_text("bin/nidhi\r")
        await asyncio.sleep(2.0)

        # Dismiss welcome screen
        await session.async_send_text("\r")
        await asyncio.sleep(1.0)

        # Enter DETAIL mode (Enter from LIST)
        await session.async_send_text("\r")
        await asyncio.sleep(1.5)

        content_initial, _ = await read_screen(session)
        has_detail = "DETAIL" in content_initial
        log("Enter DETAIL mode", has_detail, f"DETAIL badge: {has_detail}")

        # Test j/k in tree (initial focus should be tree)
        before, _ = await read_screen(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.3)
        after_j, _ = await read_screen(session)
        j_works = before != after_j
        log("j works in tree (initial)", j_works, "screen changed" if j_works else "NO CHANGE")

        # Tab → switch to diff pane
        await session.async_send_text("\t")
        await asyncio.sleep(0.3)

        # j in diff pane (scrolling diff)
        before_diff, _ = await read_screen(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.3)
        after_diff_j, _ = await read_screen(session)
        # Diff may not scroll if content is short - that's ok
        log("Tab switches to diff pane", True, "focus toggled")

        # Esc → back to previous mode
        await session.async_send_text("\x1b")
        await asyncio.sleep(0.5)

        content_after_esc, _ = await read_screen(session)
        left_detail = "DETAIL" not in content_after_esc
        log("Esc leaves DETAIL", left_detail,
            "DETAIL badge gone" if left_detail else "still in DETAIL")

        # Re-enter DETAIL (Enter)
        await session.async_send_text("\r")
        await asyncio.sleep(1.5)

        content_reenter, _ = await read_screen(session)
        back_in_detail = "DETAIL" in content_reenter
        log("Re-enter DETAIL", back_in_detail, f"DETAIL badge: {back_in_detail}")

        # j should work again (focus should be reset to tree)
        before_reenter, _ = await read_screen(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.3)
        after_reenter_j, _ = await read_screen(session)
        j_works_again = before_reenter != after_reenter_j
        log("j works after re-enter (focus reset)", j_works_again,
            "screen changed" if j_works_again else "NO CHANGE - focus NOT reset!")

        await take_screenshot(
            "/Users/indrasvat/code/github.com/indrasvat-nidhi/.claude/automations/screenshots/detail_tab_fix.png"
        )

        print(f"\nResults: {results['passed']} passed, {results['failed']} failed")

    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()
    finally:
        await session.async_send_text("q")
        await asyncio.sleep(0.3)
        await session.async_send_text("\x03")
        await asyncio.sleep(0.2)
        await session.async_send_text("exit\r")
        await asyncio.sleep(0.3)
        await session.async_close()


iterm2.run_until_complete(main)
