# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""
Verify nidhi welcome/startup screen rendering.

Tests:
1. Launch nidhi and capture the welcome screen
2. Verify ASCII art logo "NIDHI" is visible
3. Verify feature cards (BROWSE, PREVIEW, MANAGE)
4. Verify "Press Enter to continue" CTA
5. Verify version info
6. Take screenshot for visual inspection

Verification Strategy:
- Read screen content after launch
- Check for key text elements
- Screenshot for visual review

Screenshots:
- welcome_screen.png: The startup welcome screen

Key Bindings:
- Enter: dismiss welcome screen
- q: quit

Usage:
    uv run .claude/automations/verify_welcome_screen.py
"""

import asyncio
import subprocess
import iterm2


async def take_screenshot(filename):
    """Take screenshot of iTerm2 window using Quartz."""
    from Quartz import CGWindowListCopyWindowInfo, kCGWindowListOptionOnScreenOnly, kCGNullWindowID

    windows = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID)
    iterm_window_id = None
    for w in windows:
        owner = w.get("kCGWindowOwnerName", "")
        if "iTerm2" in owner:
            iterm_window_id = w.get("kCGWindowNumber")
            break

    if iterm_window_id:
        subprocess.run(
            ["screencapture", "-l", str(iterm_window_id), filename],
            check=True,
        )
        print(f"Screenshot saved: {filename}")
    else:
        print("WARNING: Could not find iTerm2 window for screenshot")


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if window is None:
        print("ERROR: No iTerm2 window found")
        return

    tab = await window.async_create_tab()
    session = tab.current_session
    results = {"passed": 0, "failed": 0, "tests": []}

    def log_result(name, status, details=""):
        results["tests"].append({"name": name, "status": status, "details": details})
        if status == "PASS":
            results["passed"] += 1
        else:
            results["failed"] += 1
        print(f"  {'✓' if status == 'PASS' else '✗'} {name}: {details}")

    try:
        # Navigate to project directory and launch nidhi
        await session.async_send_text("cd /Users/indrasvat/code/github.com/indrasvat-nidhi\r")
        await asyncio.sleep(0.5)
        await session.async_send_text("bin/nidhi\r")
        await asyncio.sleep(2.0)  # Wait for TUI to render

        # Read screen content
        screen = await session.async_get_screen_contents()
        lines = []
        for i in range(screen.number_of_lines):
            line = screen.line(i).string
            lines.append(line)
        content = "\n".join(lines)

        print("=== WELCOME SCREEN CONTENT ===")
        for i, line in enumerate(lines):
            stripped = line.rstrip()
            if stripped:
                print(f"  {i:3d}: {stripped}")
        print("=== END ===\n")

        # Test 1: ASCII art logo present
        has_logo = any("███" in line for line in lines)
        if has_logo:
            log_result("ASCII art logo", "PASS", "Found block characters")
        else:
            log_result("ASCII art logo", "FAIL", "No block characters found")

        # Test 2: Feature cards
        has_browse = "BROWSE" in content
        has_preview = "PREVIEW" in content
        has_manage = "MANAGE" in content
        if has_browse and has_preview and has_manage:
            log_result("Feature cards", "PASS", "BROWSE, PREVIEW, MANAGE all found")
        else:
            log_result("Feature cards", "FAIL",
                       f"BROWSE={has_browse}, PREVIEW={has_preview}, MANAGE={has_manage}")

        # Test 3: CTA button
        has_cta = "Press Enter to continue" in content
        if has_cta:
            log_result("CTA button", "PASS", "Found 'Press Enter to continue'")
        else:
            log_result("CTA button", "FAIL", "Missing CTA text")

        # Test 4: Version info
        has_version = "dev" in content or "v0." in content or "Made with" in content
        if has_version:
            log_result("Version info", "PASS", "Found version text")
        else:
            log_result("Version info", "FAIL", "No version info found")

        # Test 5: Tagline
        has_tagline = "stash" in content.lower()
        if has_tagline:
            log_result("Tagline", "PASS", "Found stash-related tagline")
        else:
            log_result("Tagline", "FAIL", "No tagline found")

        # Take screenshot
        await take_screenshot(
            "/Users/indrasvat/code/github.com/indrasvat-nidhi/.claude/automations/screenshots/welcome_screen.png"
        )

        # Print summary
        print(f"\n{'='*50}")
        print(f"Results: {results['passed']} passed, {results['failed']} failed")
        print(f"{'='*50}")

    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()
    finally:
        # Cleanup: quit nidhi and close session
        await session.async_send_text("q")
        await asyncio.sleep(0.3)
        await session.async_send_text("\x03")
        await asyncio.sleep(0.2)
        await session.async_send_text("exit\r")
        await asyncio.sleep(0.3)
        await session.async_close()


iterm2.run_until_complete(main)
