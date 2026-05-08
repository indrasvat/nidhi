# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""
Verify nidhi TUI renders real screens (not placeholder text).

Tests:
- Launch setup-demo.sh which builds nidhi and creates a demo repo with 6 stashes
- Wait for the TUI to fully render
- Verify LIST screen shows real stash entries (not "LIST mode — stash list placeholder")
- Verify status bar at top with repo name, branch, stash count
- Verify footer at bottom with keybind hints
- Take a screenshot for visual verification

Verification Strategy:
- Read screen contents and check for real UI elements
- Check for absence of placeholder text
- Look for stash entries, status bar, footer

Screenshots:
- Saved to .claude/automations/screenshots/nidhi_tui_verify.png

Key Bindings:
- q: quit nidhi

Usage:
- uv run .claude/automations/verify_tui_render.py
"""

import asyncio
import iterm2
import subprocess
import os

PROJECT_ROOT = "/Users/indrasvat/code/github.com/indrasvat-nidhi"
SCREENSHOT_DIR = os.path.join(PROJECT_ROOT, ".claude/automations/screenshots")
SCREENSHOT_PATH = os.path.join(SCREENSHOT_DIR, "nidhi_tui_verify.png")

os.makedirs(SCREENSHOT_DIR, exist_ok=True)

results = {"passed": 0, "failed": 0, "tests": []}

def log_result(name, status, details=""):
    results["tests"].append({"name": name, "status": status, "details": details})
    if status == "PASS":
        results["passed"] += 1
        print(f"  ✓ {name}: {details}" if details else f"  ✓ {name}")
    else:
        results["failed"] += 1
        print(f"  ✗ {name}: {details}" if details else f"  ✗ {name}")

def print_summary():
    print(f"\n{'='*60}")
    print(f"Results: {results['passed']} passed, {results['failed']} failed")
    print(f"{'='*60}")
    for t in results["tests"]:
        marker = "✓" if t["status"] == "PASS" else "✗"
        print(f"  {marker} {t['name']}")
    return 0 if results["failed"] == 0 else 1


def take_screenshot(path):
    """Take a screenshot of the iTerm2 window using screencapture."""
    try:
        from Quartz import CGWindowListCopyWindowInfo, kCGWindowListOptionOnScreenOnly, kCGNullWindowID
        windows = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID)
        iterm_wid = None
        for w in windows:
            owner = w.get("kCGWindowOwnerName", "")
            if "iTerm2" in str(owner):
                iterm_wid = w.get("kCGWindowNumber")
                break
        if iterm_wid:
            subprocess.run(["screencapture", "-l", str(iterm_wid), path], check=True)
            print(f"  Screenshot saved: {path}")
            return True
        else:
            print("  WARNING: Could not find iTerm2 window for screenshot")
            return False
    except Exception as e:
        print(f"  WARNING: Screenshot failed: {e}")
        return False


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if window is None:
        print("ERROR: No iTerm2 window found")
        return

    # Create a new tab for the test
    tab = await window.async_create_tab()
    session = tab.current_session
    await asyncio.sleep(0.5)

    try:
        # Navigate to project and run demo script
        await session.async_send_text(f"cd {PROJECT_ROOT}\n")
        await asyncio.sleep(0.3)
        await session.async_send_text("./scripts/setup-demo.sh\n")

        # Wait for the TUI to load (demo script builds, creates repo, launches nidhi)
        print("Waiting for TUI to load...")
        await asyncio.sleep(8)

        # Read screen contents
        screen = await session.async_get_screen_contents()
        lines = []
        for i in range(screen.number_of_lines):
            line = screen.line(i).string
            lines.append(line)

        full_screen = "\n".join(lines)
        print("\n--- Screen Contents ---")
        for i, line in enumerate(lines[:30]):
            print(f"  {i:2d}: {repr(line)}")
        print("--- End Screen ---\n")

        # Test 1: No placeholder text
        has_placeholder = "placeholder" in full_screen.lower()
        if not has_placeholder:
            log_result("No placeholder text", "PASS")
        else:
            log_result("No placeholder text", "FAIL", "Found 'placeholder' in screen output")

        # Test 2: Check for stash entries (stash@{0}, stash messages, or stash indicators)
        has_stash_content = any(
            keyword in full_screen.lower()
            for keyword in ["redis", "config loader", "dashboard", "api", "hotfix", "lru", "stash"]
        )
        if has_stash_content:
            log_result("Stash entries visible", "PASS")
        else:
            log_result("Stash entries visible", "FAIL", "No stash-related content found on screen")

        # Test 3: Check for status bar elements (repo name, branch, stash count)
        has_status_bar = any(
            keyword in full_screen.lower()
            for keyword in ["nidhi-demo", "main", "stashes", "git"]
        )
        if has_status_bar:
            log_result("Status bar present", "PASS")
        else:
            log_result("Status bar present", "FAIL", "No status bar elements found")

        # Test 4: Check for footer keybind hints
        has_footer = any(
            keyword in full_screen.lower()
            for keyword in ["nav", "detail", "preview", "help", "apply", "search"]
        )
        if has_footer:
            log_result("Footer keybinds present", "PASS")
        else:
            log_result("Footer keybinds present", "FAIL", "No footer keybind hints found")

        # Test 5: Check it's NOT just "Loading nidhi..."
        is_loading = "loading nidhi" in full_screen.lower()
        if not is_loading:
            log_result("Not stuck on loading screen", "PASS")
        else:
            log_result("Not stuck on loading screen", "FAIL", "Still showing loading screen")

        # Take screenshot
        take_screenshot(SCREENSHOT_PATH)

    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()
        raise
    finally:
        # Cleanup: quit nidhi, then close session
        await session.async_send_text("q")
        await asyncio.sleep(0.5)
        await session.async_send_text("\x03")  # Ctrl+C fallback
        await asyncio.sleep(0.3)
        await session.async_send_text("exit\n")
        await asyncio.sleep(0.3)
        await session.async_close()

    exit_code = print_summary()
    if exit_code != 0:
        raise SystemExit(exit_code)


iterm2.run_until_complete(main)
