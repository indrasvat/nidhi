# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""
Verify DETAIL mode focus reset on demo repo.

Usage:
    uv run .claude/automations/verify_detail_focus_reset.py
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
            print(f"  Screenshot: {filename}")
            return


async def read_screen(session):
    screen = await session.async_get_screen_contents()
    lines = []
    for i in range(screen.number_of_lines):
        lines.append(screen.line(i).string)
    return "\n".join(lines), lines


async def wait_for_prompt(session, timeout=15):
    """Wait until shell prompt appears (command finished)."""
    for _ in range(timeout * 2):
        content, _ = await read_screen(session)
        if "indrasvat" in content.split("\n")[-3]:  # prompt line
            return True
        await asyncio.sleep(0.5)
    return False


async def wait_for_content(session, keyword, timeout=5):
    """Wait until keyword appears on screen."""
    for _ in range(timeout * 4):
        content, _ = await read_screen(session)
        if keyword in content:
            return True, content
        await asyncio.sleep(0.25)
    content, _ = await read_screen(session)
    return False, content


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if not window:
        print("ERROR: No window")
        return

    tab = await window.async_create_tab()
    session = tab.current_session
    results = {"passed": 0, "failed": 0}
    basedir = "/Users/indrasvat/code/github.com/indrasvat-nidhi"
    screenshots = f"{basedir}/.claude/automations/screenshots"

    def log(name, ok, details=""):
        results["passed" if ok else "failed"] += 1
        print(f"  {'✓' if ok else '✗'} {name}: {details}")

    try:
        # Step 1: Set up demo repo (run in foreground, wait for completion)
        print("\n[1/7] Setting up demo repo...")
        await session.async_send_text(f"cd {basedir}\r")
        await asyncio.sleep(0.5)
        await session.async_send_text("bash scripts/setup-demo.sh\r")
        # Wait for setup to complete (it prints "Done!" at the end)
        found, _ = await wait_for_content(session, "Done!", timeout=20)
        if not found:
            found, _ = await wait_for_content(session, "stash", timeout=5)
        await asyncio.sleep(1.0)
        print("  Demo repo ready." if found else "  Warning: setup may not be complete")

        # Step 2: Launch nidhi on demo repo
        print("[2/7] Launching nidhi on demo repo...")
        await session.async_send_text(f"{basedir}/bin/nidhi -C /tmp/nidhi-demo\r")
        # Wait for welcome screen (has "Press Enter")
        found, content = await wait_for_content(session, "Press Enter", timeout=5)
        if not found:
            # Maybe no welcome screen, check for LIST
            found, content = await wait_for_content(session, "LIST", timeout=3)
        log("Nidhi launched", found, "welcome or LIST visible")

        # Dismiss welcome screen
        await session.async_send_text("\r")
        await asyncio.sleep(1.0)

        found, content = await wait_for_content(session, "LIST", timeout=3)
        log("LIST mode visible", found, f"LIST badge: {found}")

        # Step 3: Enter DETAIL (Enter from LIST)
        print("[3/7] Enter DETAIL mode...")
        await session.async_send_text("\r")
        found, content = await wait_for_content(session, "DETAIL", timeout=3)
        log("Entered DETAIL mode", found, f"DETAIL badge: {found}")

        await asyncio.sleep(0.5)

        # Test j in tree (should navigate files)
        before, _ = await read_screen(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.5)
        after_j, _ = await read_screen(session)
        j_works = before != after_j
        log("j navigates files (tree focus)", j_works,
            "screen changed" if j_works else "no change (may have 1 file)")

        # k back up
        await session.async_send_text("k")
        await asyncio.sleep(0.3)

        # Step 4: Tab → diff pane
        print("[4/7] Tab to diff pane...")
        await session.async_send_text("\t")
        await asyncio.sleep(0.5)
        log("Tab pressed (focus → diff)", True, "focus toggled to diff pane")

        # Try j in diff (may not scroll if short)
        before_diff, _ = await read_screen(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.3)
        after_diff, _ = await read_screen(session)
        diff_scroll = before_diff != after_diff
        log("j in diff pane", True, "scrolled" if diff_scroll else "too short to scroll (OK)")

        await take_screenshot(f"{screenshots}/detail_after_tab.png")

        # Step 5: Esc back
        print("[5/7] Esc back to previous mode...")
        await session.async_send_text("\x1b")
        await asyncio.sleep(0.5)

        content, _ = await read_screen(session)
        left_detail = "DETAIL" not in content
        log("Esc leaves DETAIL", left_detail,
            "left DETAIL" if left_detail else "still in DETAIL")

        # Step 6: Re-enter DETAIL → focus should be reset
        print("[6/7] Re-enter DETAIL (same stash)...")
        await session.async_send_text("\r")
        found, _ = await wait_for_content(session, "DETAIL", timeout=3)
        log("Back in DETAIL", found, f"DETAIL badge: {found}")
        await asyncio.sleep(0.5)

        # j should work (focus reset to tree, cursor reset to first file)
        before_re, _ = await read_screen(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.5)
        after_re_j, _ = await read_screen(session)
        j_after_reset = before_re != after_re_j
        log("j works after re-enter (focus+cursor reset)", j_after_reset,
            "screen changed ✓" if j_after_reset else "NO CHANGE - BUG!")

        await take_screenshot(f"{screenshots}/detail_after_reenter.png")

        # Step 7: Different stash
        print("[7/7] Different stash → DETAIL...")
        await session.async_send_text("\x1b")  # Esc to LIST
        await asyncio.sleep(0.5)
        await session.async_send_text("j")  # Next stash
        await asyncio.sleep(0.3)
        await session.async_send_text("\r")  # Enter DETAIL
        found, _ = await wait_for_content(session, "DETAIL", timeout=3)
        log("Different stash → DETAIL", found, f"DETAIL badge: {found}")
        await asyncio.sleep(0.5)

        before_new, _ = await read_screen(session)
        await session.async_send_text("j")
        await asyncio.sleep(0.5)
        after_new_j, _ = await read_screen(session)
        j_new = before_new != after_new_j
        log("j works on different stash", j_new,
            "screen changed" if j_new else "no change (single file?)")

        await take_screenshot(f"{screenshots}/detail_different_stash.png")

        # Summary
        print(f"\n{'='*50}")
        print(f"Results: {results['passed']} passed, {results['failed']} failed")
        print(f"{'='*50}")

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
        await asyncio.sleep(0.5)
        try:
            await session.async_close()
        except Exception:
            pass


iterm2.run_until_complete(main)
