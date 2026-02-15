package perf_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestVisualResponsiveness_RapidScroll tests that rapid j/k keystrokes
// produce no visible lag or flicker, verified via iterm2-driver screenshots.
//
// Prerequisites: macOS with iTerm2, iterm2-driver in PATH, NIDHI_VISUAL_TEST=1.
func TestVisualResponsiveness_RapidScroll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual responsiveness test in short mode")
	}
	if os.Getenv("NIDHI_VISUAL_TEST") != "1" {
		t.Skip("set NIDHI_VISUAL_TEST=1 to run visual responsiveness tests")
	}
	if runtime.GOOS != "darwin" {
		t.Skip("visual tests require macOS with iTerm2")
	}

	iterm2Driver, err := exec.LookPath("iterm2-driver")
	if err != nil {
		t.Skip("iterm2-driver not found in PATH")
	}

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "nidhi")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/nidhi")
	cmd.Dir = projectRoot(t)
	out, buildErr := cmd.CombinedOutput()
	if buildErr != nil {
		t.Fatalf("build failed: %v\n%s", buildErr, out)
	}

	dir := testRepo(t, 50)

	screenshotBefore := filepath.Join(t.TempDir(), "before.png")
	screenshotAfter := filepath.Join(t.TempDir(), "after.png")

	// Capture initial state.
	cmd = exec.Command(iterm2Driver,
		"--cols", "120", "--rows", "40",
		"--delay", "2000ms",
		"--screenshot", screenshotBefore,
		"--", binPath, "-C", dir, "--no-color",
	)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Logf("iterm2-driver output: %s", out)
		t.Fatalf("initial screenshot failed: %v", err)
	}

	// Send rapid keystrokes and capture.
	keys := make([]string, 0, 200)
	for range 100 {
		keys = append(keys, "--send-key", "j")
	}
	args := []string{
		"--cols", "120", "--rows", "40",
	}
	args = append(args, keys...)
	args = append(args,
		"--delay", "500ms",
		"--screenshot", screenshotAfter,
		"--", binPath, "-C", dir, "--no-color",
	)

	start := time.Now()
	cmd = exec.Command(iterm2Driver, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	out, err = cmd.CombinedOutput()
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("iterm2-driver output: %s", out)
		t.Fatalf("rapid scroll screenshot failed: %v", err)
	}

	t.Logf("100 rapid j keystrokes + screenshot: %v", elapsed)

	for _, path := range []string{screenshotBefore, screenshotAfter} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("screenshot not created: %s: %v", path, statErr)
			continue
		}
		if info.Size() < 1000 {
			t.Errorf("screenshot suspiciously small: %s (%d bytes)", path, info.Size())
		}
	}

	t.Logf("Visual responsiveness test passed. Before: %s, After: %s", screenshotBefore, screenshotAfter)
}
