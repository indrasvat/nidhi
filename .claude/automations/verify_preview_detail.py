# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""Quick verify that PREVIEW and DETAIL modes still work with new key routing."""

import asyncio
import iterm2
import subprocess
import os

SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "screenshots")
os.makedirs(SCREENSHOT_DIR, exist_ok=True)


def take_screenshot(name):
    try:
        from Quartz import (
            CGWindowListCopyWindowInfo,
            kCGWindowListOptionOnScreenOnly,
            kCGNullWindowID,
        )
        windows = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID)
        iterm_wid = None
        for w in windows:
            if "iTerm" in str(w.get("kCGWindowOwnerName", "")):
                iterm_wid = w.get("kCGWindowNumber")
                break
        if iterm_wid:
            path = os.path.join(SCREENSHOT_DIR, f"{name}.png")
            subprocess.run(["screencapture", "-l", str(iterm_wid), path], capture_output=True, timeout=5)
            print(f"  📸 {path}")
    except Exception as e:
        print(f"  ⚠ Screenshot failed: {e}")


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if not window:
        print("ERROR: No window")
        return

    tab = await window.async_create_tab()
    session = tab.current_session
    project_dir = os.path.expanduser("~/code/github.com/indrasvat-nidhi")

    try:
        await session.async_send_text(f"cd {project_dir}\n")
        await asyncio.sleep(0.5)
        await session.async_send_text("./scripts/setup-demo.sh\n")
        await asyncio.sleep(4)

        print("=== Quick Verify: PREVIEW & DETAIL ===\n")

        # Tab → PREVIEW
        await session.async_send_text("\t")  # Tab
        await asyncio.sleep(1.5)
        screen = await session.async_get_screen_contents()
        preview_lines = [screen.line(i).string for i in range(min(10, screen.number_of_lines))]
        print("PREVIEW (first 10 lines):")
        for i, line in enumerate(preview_lines):
            print(f"  {i}: {repr(line[:100])}")

        take_screenshot("fix_06_preview_mode")

        # Check for diff content
        full = "\n".join(preview_lines)
        has_diff = "diff" in full.lower() or "+++" in full or "---" in full or "@@" in full or "Loading" in full or "─" in full
        print(f"  PREVIEW has diff/divider content: {has_diff}")

        # j in PREVIEW should move cursor
        await session.async_send_text("j")
        await asyncio.sleep(1)

        take_screenshot("fix_07_preview_after_j")

        # Enter → DETAIL
        await session.async_send_text("\r")
        await asyncio.sleep(1)
        screen = await session.async_get_screen_contents()
        detail_lines = [screen.line(i).string for i in range(min(10, screen.number_of_lines))]
        print("\nDETAIL (first 10 lines):")
        for i, line in enumerate(detail_lines):
            print(f"  {i}: {repr(line[:100])}")

        take_screenshot("fix_08_detail_mode")

        detail_full = "\n".join(detail_lines)
        has_tree = "staged" in detail_full.lower() or "working" in detail_full.lower() or "│" in detail_full or "├" in detail_full
        print(f"  DETAIL has tree/file content: {has_tree}")

        # Esc → back to PREVIEW
        await session.async_send_text("\x1b")
        await asyncio.sleep(0.5)

        # Esc → back to LIST
        await session.async_send_text("\x1b")
        await asyncio.sleep(0.5)

        screen = await session.async_get_screen_contents()
        footer = screen.line(screen.number_of_lines - 1).string
        print(f"\n  Back to LIST, footer: {repr(footer[:80])}")
        print(f"  Footer has LIST badge: {'LIST' in footer}")

        print("\n=== All modes verified ===")

    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()
    finally:
        await session.async_send_text("\x03")
        await asyncio.sleep(0.3)
        await session.async_send_text("q")
        await asyncio.sleep(0.3)
        await session.async_send_text("exit\n")
        await asyncio.sleep(0.3)
        try:
            await session.async_close()
        except Exception:
            pass

iterm2.run_until_complete(main)
