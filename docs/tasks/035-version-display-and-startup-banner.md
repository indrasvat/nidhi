# Task 035: Build Version Display and Startup Banner

**Priority:** P1 — Important (UX polish)
**Depends on:** 034

## Problem

The `version`, `commit`, `date` vars exist in `cmd/nidhi/main.go` (injected via ldflags) but
are only shown via `--version` CLI flag. No version info is visible in the running TUI.
There is also no startup ASCII art banner.

## Fix Strategy

### Status Bar Version Display
Add app version alongside git version on the right side of the status bar.
Current: `git 2.53`
New: `nidhi dev · git 2.53` (or `nidhi v0.1.0 (abc1234) · git 2.53`)

Changes:
- Add `AppVersion` and `AppCommit` fields to `StatusBarParams`
- Update `StatusBar.Render()` to show app version on the right
- Pass version from `main.go` through `uiRenderer`

### Startup Banner
Replace the plain "Loading nidhi..." text with a styled banner:
```
   ◆ nidhi
   v0.1.0-dev (abc1234)
```

Changes:
- Add `Version` and `Commit` fields to `core.Model`
- Update `Model.View()` loading screen to use styled banner
- Set fields from `main.go`

## Files to Modify

- `internal/ui/components/statusbar.go` — Add version to StatusBarParams and render
- `internal/core/app.go` — Add Version/Commit fields, update loading screen
- `cmd/nidhi/main.go` — Pass version info to Model and uiRenderer

## Acceptance Criteria

- [ ] Status bar shows `nidhi {version} · git {version}` on the right
- [ ] Loading screen shows styled nidhi banner with version
- [ ] `make build` injects real version via ldflags
- [ ] `go run` shows "dev" as version (default)
