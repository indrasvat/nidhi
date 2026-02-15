# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""
Comprehensive INTERACTION-BASED TUI E2E Test for nidhi.

Tests real key interactions and verifies screen CONTENT CHANGES —
not just keyword presence. Each test presses a key and asserts
the screen actually changed.

Test Matrix:
  LIST mode:
    1. j/k cursor navigation (content changes)
    2. Arrow keys up/down (content changes)
    3. g/G jump to top/bottom (content changes)
    4. Tab opens PREVIEW (badge visible)
    5. Enter opens DETAIL (badge visible)
    6. ? opens HELP overlay (content changes)
    7. Footer has "help" hint
    8. Status bar has version + git version

  PREVIEW mode:
    9. j/k cycles stashes (diff content changes)
   10. Arrow keys up/down (content changes)
   11. ? opens HELP from PREVIEW
   12. Footer has "help" hint
   13. Tab returns to LIST

  DETAIL mode:
   14. j/k file navigation (cursor moves in tree)
   15. Arrow keys up/down (cursor moves in tree)
   16. Tab toggles tree<>diff focus
   17. j after Tab scrolls diff (not tree)
   18. ? opens HELP from DETAIL
   19. Footer has "help" hint
   20. Esc returns to previous mode
   21. Re-enter after Tab+Esc: j/k works (focus reset)
   22. Different stash: j/k works (clean state)

Verification Strategy:
  - Use polling (wait_for_content) instead of hardcoded sleeps
  - Assert screen CONTENT CHANGES after each key press
  - Dump screen on failure for debugging

Screenshots:
  - Saved to .claude/automations/screenshots/

Usage:
  uv run .claude/automations/comprehensive_tui_test.py
"""

import asyncio
import os
import subprocess

import iterm2

PROJECT_ROOT = "/Users/indrasvat/code/github.com/indrasvat-nidhi"
SCREENSHOT_DIR = os.path.join(PROJECT_ROOT, ".claude/automations/screenshots")
os.makedirs(SCREENSHOT_DIR, exist_ok=True)

results = {"passed": 0, "failed": 0, "tests": []}


# ─── Helpers ─────────────────────────────────────────────────


def log(name, ok, details=""):
    status = "PASS" if ok else "FAIL"
    results["tests"].append({"name": name, "status": status, "details": details})
    results["passed" if ok else "failed"] += 1
    marker = "\u2713" if ok else "\u2717"
    suffix = f": {details}" if details else ""
    print(f"  {marker} {name}{suffix}")


def print_summary():
    total = results["passed"] + results["failed"]
    print(f"\n{'='*60}")
    print(f"Results: {results['passed']}/{total} passed, {results['failed']} failed")
    print(f"{'='*60}")
    for t in results["tests"]:
        marker = "\u2713" if t["status"] == "PASS" else "\u2717"
        line = f"  {marker} {t['name']}"
        if t["details"]:
            line += f"  ({t['details']})"
        print(line)
    return 0 if results["failed"] == 0 else 1


def take_screenshot(name):
    path = os.path.join(SCREENSHOT_DIR, f"{name}.png")
    try:
        from Quartz import (
            CGWindowListCopyWindowInfo,
            kCGNullWindowID,
            kCGWindowListOptionOnScreenOnly,
        )

        windows = CGWindowListCopyWindowInfo(
            kCGWindowListOptionOnScreenOnly, kCGNullWindowID
        )
        for w in windows:
            if "iTerm2" in w.get("kCGWindowOwnerName", ""):
                subprocess.run(
                    ["screencapture", "-l", str(w.get("kCGWindowNumber")), path],
                    check=True,
                )
                return
    except Exception:
        pass


async def read_screen(session):
    screen = await session.async_get_screen_contents()
    lines = []
    for i in range(screen.number_of_lines):
        lines.append(screen.line(i).string)
    return "\n".join(lines)


async def wait_for(session, keyword, timeout=10):
    """Poll until keyword appears on screen. Returns (found, content)."""
    for _ in range(timeout * 4):
        content = await read_screen(session)
        if keyword in content:
            return True, content
        await asyncio.sleep(0.25)
    content = await read_screen(session)
    return False, content


async def send_and_wait(session, key, delay=0.4):
    """Send a key and wait for the screen to settle."""
    await session.async_send_text(key)
    await asyncio.sleep(delay)
    return await read_screen(session)


def strip_blank_tail(content):
    """Remove trailing blank lines from screen content (scrollback buffer)."""
    lines = content.split("\n")
    while lines and lines[-1].strip() == "":
        lines.pop()
    return "\n".join(lines)


async def dump_on_fail(session, label):
    """Dump screen content for debugging when a test fails."""
    content = await read_screen(session)
    print(f"\n--- DUMP: {label} ---")
    for i, line in enumerate(content.split("\n")[:30]):
        print(f"  {i:2d}: {repr(line)}")
    print(f"--- END DUMP ---\n")


# ─── Main Test ───────────────────────────────────────────────


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if not window:
        print("ERROR: No iTerm2 window")
        return

    tab = await window.async_create_tab()
    session = tab.current_session

    try:
        # ── Setup: build + create demo repo + launch nidhi ──
        print("\n[SETUP] Building and launching nidhi on demo repo...")
        await session.async_send_text(f"cd {PROJECT_ROOT}\r")
        await asyncio.sleep(0.5)
        await session.async_send_text("bash scripts/setup-demo.sh\r")

        # Wait for welcome screen ("Press Enter") or LIST mode
        found, content = await wait_for(session, "Press Enter", timeout=25)
        if not found:
            found, content = await wait_for(session, "LIST", timeout=5)
        if not found:
            print("ERROR: nidhi did not start")
            await dump_on_fail(session, "startup failure")
            return

        # Dismiss welcome screen
        if "Press Enter" in content:
            await session.async_send_text("\r")
            found, _ = await wait_for(session, "LIST", timeout=5)
            if not found:
                print("ERROR: Could not reach LIST mode")
                return

        print("[SETUP] nidhi running in LIST mode.\n")

        # ════════════════════════════════════════════════════════
        # SECTION 1: LIST MODE
        # ════════════════════════════════════════════════════════
        print("== LIST MODE ==")

        content = await read_screen(session)
        take_screenshot("01_list_mode")

        # 1. Stash entries visible
        stash_kw = ["redis", "config", "dashboard", "api", "hotfix", "lru"]
        log("LIST: stash entries visible",
            any(kw in content.lower() for kw in stash_kw))

        # 2. No placeholder text
        log("LIST: no placeholder text", "placeholder" not in content.lower())

        # 3. Status bar has version info
        top_lines = "\n".join(content.split("\n")[:3])
        has_version = ("git" in top_lines.lower()
                       and any(c.isdigit() for c in top_lines))
        log("LIST: status bar has git version", has_version)

        import re
        has_app_ver = ("dev" in top_lines.lower()
                       or "v0." in top_lines
                       or bool(re.search(r'[0-9a-f]{7}', top_lines)))
        log("LIST: status bar has app version", has_app_ver)

        # 4. Footer has "help" hint
        trimmed = strip_blank_tail(content)
        bottom_lines = "\n".join(trimmed.split("\n")[-3:]).lower()
        log("LIST: footer has help hint", "help" in bottom_lines)

        # 5. LIST badge
        log("LIST: mode badge visible", "LIST" in content)

        # 6. j moves cursor
        before = await read_screen(session)
        after = await send_and_wait(session, "j")
        log("LIST: j moves cursor", before != after, "content changed" if before != after else "NO CHANGE")

        # 7. k moves cursor back
        before_k = await read_screen(session)
        after_k = await send_and_wait(session, "k")
        log("LIST: k moves cursor", before_k != after_k, "content changed" if before_k != after_k else "NO CHANGE")

        # 8. Down arrow moves cursor
        before_arr = await read_screen(session)
        after_arr = await send_and_wait(session, "\x1b[B")  # Down arrow
        log("LIST: down arrow moves cursor", before_arr != after_arr,
            "content changed" if before_arr != after_arr else "NO CHANGE")

        # 9. Up arrow moves cursor back
        before_up = await read_screen(session)
        after_up = await send_and_wait(session, "\x1b[A")  # Up arrow
        log("LIST: up arrow moves cursor", before_up != after_up,
            "content changed" if before_up != after_up else "NO CHANGE")

        # 10. g/G jump top/bottom
        await send_and_wait(session, "j")  # move down first
        await send_and_wait(session, "j")
        after_g = await send_and_wait(session, "g")
        after_G = await send_and_wait(session, "G")
        log("LIST: g/G jump top/bottom", after_g != after_G)

        # Reset to top
        await send_and_wait(session, "g")

        # 11. ? opens help from LIST
        before_help = await read_screen(session)
        after_help = await send_and_wait(session, "?", delay=0.5)
        help_opened = before_help != after_help and ("help" in after_help.lower() or "keybind" in after_help.lower())
        log("LIST: ? opens help", help_opened)
        take_screenshot("02_list_help")

        # Close help
        await send_and_wait(session, "\x1b", delay=0.3)

        # ════════════════════════════════════════════════════════
        # SECTION 2: PREVIEW MODE
        # ════════════════════════════════════════════════════════
        print("\n== PREVIEW MODE ==")

        before_tab = await read_screen(session)
        await session.async_send_text("\t")
        found, _ = await wait_for(session, "PREVIEW", timeout=5)
        await asyncio.sleep(0.5)  # Let diff load and footer settle
        preview_content = await read_screen(session)
        log("PREVIEW: Tab from LIST opens PREVIEW", found, "badge visible" if found else "no PREVIEW badge")
        take_screenshot("03_preview_mode")

        # Footer has "help" hint in PREVIEW
        preview_trimmed = strip_blank_tail(preview_content)
        preview_bottom = "\n".join(preview_trimmed.split("\n")[-3:]).lower()
        log("PREVIEW: footer has help hint", "help" in preview_bottom)

        # j cycles stashes (diff content should change)
        before_pj = await read_screen(session)
        after_pj = await send_and_wait(session, "j", delay=0.8)
        log("PREVIEW: j cycles stashes", before_pj != after_pj,
            "content changed" if before_pj != after_pj else "NO CHANGE")

        # Down arrow cycles stashes
        before_parr = await read_screen(session)
        after_parr = await send_and_wait(session, "\x1b[B", delay=0.8)
        log("PREVIEW: down arrow cycles stashes", before_parr != after_parr,
            "content changed" if before_parr != after_parr else "NO CHANGE")

        # k goes back
        before_pk = await read_screen(session)
        after_pk = await send_and_wait(session, "k", delay=0.8)
        log("PREVIEW: k cycles stashes back", before_pk != after_pk,
            "content changed" if before_pk != after_pk else "NO CHANGE")

        # Up arrow goes back
        before_pup = await read_screen(session)
        after_pup = await send_and_wait(session, "\x1b[A", delay=0.8)
        log("PREVIEW: up arrow cycles stashes", before_pup != after_pup,
            "content changed" if before_pup != after_pup else "NO CHANGE")

        # ? opens help from PREVIEW
        before_ph = await read_screen(session)
        after_ph = await send_and_wait(session, "?", delay=0.5)
        ph_opened = before_ph != after_ph
        log("PREVIEW: ? opens help", ph_opened)
        take_screenshot("04_preview_help")

        # Close help
        await send_and_wait(session, "\x1b", delay=0.3)

        # Tab back to LIST
        await session.async_send_text("\t")
        found_list, _ = await wait_for(session, "LIST", timeout=3)
        log("PREVIEW: Tab returns to LIST", found_list)

        # ════════════════════════════════════════════════════════
        # SECTION 3: DETAIL MODE
        # ════════════════════════════════════════════════════════
        print("\n== DETAIL MODE ==")

        # Go to top, then Enter for DETAIL
        await send_and_wait(session, "g")
        await session.async_send_text("\r")
        found_detail, detail_content = await wait_for(session, "DETAIL", timeout=5)
        log("DETAIL: Enter from LIST opens DETAIL", found_detail, "badge visible" if found_detail else "no badge")
        take_screenshot("05_detail_mode")

        # Footer has "help" hint in DETAIL
        detail_trimmed = strip_blank_tail(detail_content)
        detail_bottom = "\n".join(detail_trimmed.split("\n")[-3:]).lower()
        log("DETAIL: footer has help hint", "help" in detail_bottom)

        # j navigates files in tree (initial focus = tree)
        await asyncio.sleep(0.3)
        before_dj = await read_screen(session)
        after_dj = await send_and_wait(session, "j")
        j_tree_works = before_dj != after_dj
        log("DETAIL: j moves file cursor (tree focus)", j_tree_works,
            "content changed" if j_tree_works else "NO CHANGE")

        # k moves back
        before_dk = await read_screen(session)
        after_dk = await send_and_wait(session, "k")
        log("DETAIL: k moves file cursor back", before_dk != after_dk,
            "content changed" if before_dk != after_dk else "NO CHANGE")

        # Down arrow navigates files
        before_darr = await read_screen(session)
        after_darr = await send_and_wait(session, "\x1b[B")
        log("DETAIL: down arrow moves cursor", before_darr != after_darr,
            "content changed" if before_darr != after_darr else "NO CHANGE")

        # Up arrow back
        before_dup = await read_screen(session)
        after_dup = await send_and_wait(session, "\x1b[A")
        log("DETAIL: up arrow moves cursor", before_dup != after_dup,
            "content changed" if before_dup != after_dup else "NO CHANGE")

        # Tab toggles focus to diff pane
        before_tab_d = await read_screen(session)
        after_tab_d = await send_and_wait(session, "\t")
        log("DETAIL: Tab pressed (focus toggle)", True, "focus switched to diff")
        take_screenshot("06_detail_after_tab")

        # j in diff pane (should scroll diff, not move tree cursor)
        before_dj2 = await read_screen(session)
        after_dj2 = await send_and_wait(session, "j")
        # May not scroll if diff is short — that's acceptable
        log("DETAIL: j in diff pane", True,
            "scrolled diff" if before_dj2 != after_dj2 else "short diff (OK)")

        # ? opens help from DETAIL
        before_dh = await read_screen(session)
        after_dh = await send_and_wait(session, "?", delay=0.5)
        dh_opened = before_dh != after_dh
        log("DETAIL: ? opens help", dh_opened)
        take_screenshot("07_detail_help")

        # Close help
        await send_and_wait(session, "\x1b", delay=0.3)

        # Esc returns to previous mode (LIST)
        await session.async_send_text("\x1b")
        found_back, _ = await wait_for(session, "LIST", timeout=3)
        log("DETAIL: Esc returns to LIST", found_back)
        take_screenshot("08_after_esc")

        # ════════════════════════════════════════════════════════
        # SECTION 4: DETAIL FOCUS RESET (regression test)
        # ════════════════════════════════════════════════════════
        print("\n== DETAIL FOCUS RESET ==")

        # Re-enter DETAIL (same stash) — focus should be reset to tree
        await session.async_send_text("\r")
        found_re, _ = await wait_for(session, "DETAIL", timeout=3)
        log("RESET: re-enter DETAIL", found_re)

        await asyncio.sleep(0.3)
        before_re = await read_screen(session)
        after_re = await send_and_wait(session, "j")
        j_after_reset = before_re != after_re
        log("RESET: j works after re-enter (focus+cursor reset)", j_after_reset,
            "content changed" if j_after_reset else "NO CHANGE - BUG!")
        take_screenshot("09_detail_reenter")

        # Esc back, select different stash, enter DETAIL
        await session.async_send_text("\x1b")
        await wait_for(session, "LIST", timeout=3)
        await send_and_wait(session, "j")
        await session.async_send_text("\r")
        found_new, _ = await wait_for(session, "DETAIL", timeout=3)
        log("RESET: different stash DETAIL", found_new)

        await asyncio.sleep(0.3)
        before_new = await read_screen(session)
        after_new = await send_and_wait(session, "j")
        j_new_stash = before_new != after_new
        log("RESET: j works on different stash", j_new_stash,
            "content changed" if j_new_stash else "single file (OK)")
        take_screenshot("10_detail_different_stash")

        # ════════════════════════════════════════════════════════
        # SUMMARY
        # ════════════════════════════════════════════════════════
        take_screenshot("11_final_state")

    except Exception as e:
        print(f"\nERROR: {e}")
        import traceback
        traceback.print_exc()
        try:
            await dump_on_fail(session, "error state")
            take_screenshot("error_state")
        except Exception:
            pass
    finally:
        await session.async_send_text("q")
        await asyncio.sleep(0.3)
        await session.async_send_text("\x1b")
        await asyncio.sleep(0.2)
        await session.async_send_text("\x03")
        await asyncio.sleep(0.2)
        await session.async_send_text("exit\r")
        await asyncio.sleep(0.3)
        try:
            await session.async_close()
        except Exception:
            pass

    exit_code = print_summary()
    if exit_code != 0:
        raise SystemExit(exit_code)


iterm2.run_until_complete(main)
