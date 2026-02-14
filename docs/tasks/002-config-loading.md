# Task 002: Config Loading

## Status: TODO

## Depends On
- 000 (Repository Scaffold and Tooling) -- go.mod with go-toml/v2 and xdg deps, directory structure

## Parallelizable With
- 001 (Git Runner and Version Detection)
- 003 (Agni Theme and Icons)

## Problem
nidhi supports five configuration sources with strict priority ordering: CLI flags > env vars > git config > TOML file > built-in defaults (PRD section 12.1). Without a config package, no other component can access user preferences for stale thresholds, icon modes, export refs, log levels, or performance tuning. The config system must resolve values from all sources correctly, and every setting must have a sane default so nidhi works with zero configuration.

## PRD Reference
- Section 12.1 (Configuration Sources) -- priority order: CLI flags > env vars > git config > TOML > defaults
- Section 12.2 (Config File Format) -- full TOML schema with all sections and keys
- Section 12.3 (Git Config Integration) -- `git config --get nidhi.<key>` for supported keys
- Section 12.4 (Environment Variables) -- NIDHI_STALE_DAYS, NIDHI_ICONS, NIDHI_LOG_LEVEL, etc.
- Section 12.5 (CLI Flags) -- all flag names and types
- Section 12.6 (Sane Defaults Philosophy) -- default values with rationale

## Files to Create
- `internal/config/config.go` -- `Config` struct with all fields, TOML tags
- `internal/config/defaults.go` -- `DefaultConfig()` factory with all defaults from PRD section 12.6
- `internal/config/gitconfig.go` -- read values from `git config --get nidhi.<key>`
- `internal/config/loader.go` -- `Load()` function implementing priority resolution
- `internal/config/config_test.go` -- tests for each source, priority override, defaults, TOML parsing

## Execution Steps

### Step 1: Create `internal/config/config.go`

```go
package config

import "time"

// Config holds all nidhi configuration.
// Fields map 1:1 to the TOML schema in PRD section 12.2.
type Config struct {
	General     GeneralConfig     `toml:"general"`
	Export      ExportConfig      `toml:"export"`
	Theme       ThemeConfig       `toml:"theme"`
	Keys        KeysConfig        `toml:"keys"`
	Performance PerformanceConfig `toml:"performance"`
	Log         LogConfig         `toml:"log"`
}

// GeneralConfig holds general settings.
type GeneralConfig struct {
	// Icons controls the icon set: "auto" (detect Nerd Fonts), "nerd", "ascii".
	Icons string `toml:"icons"`
	// StaleDays is the number of days after which a stash is considered stale.
	StaleDays int `toml:"stale_days"`
	// KeepIndex preserves staged files when creating new stashes.
	KeepIndex bool `toml:"keep_index"`
	// AutoMessage auto-generates readable messages for default WIP stash messages.
	AutoMessage bool `toml:"auto_message"`
}

// ExportConfig holds export/sync settings.
type ExportConfig struct {
	// Ref is the default ref path for export. $USER is expanded at runtime.
	Ref string `toml:"ref"`
	// Remote is the default remote for export/import operations.
	Remote string `toml:"remote"`
}

// ThemeConfig holds theme settings.
type ThemeConfig struct {
	// Name is the built-in theme name ("agni") or path to a custom theme TOML.
	Name string `toml:"name"`
}

// KeysConfig holds keybinding overrides.
type KeysConfig struct {
	Apply string `toml:"apply,omitempty"`
	Pop   string `toml:"pop,omitempty"`
	Drop  string `toml:"drop,omitempty"`
}

// PerformanceConfig holds performance tuning settings.
type PerformanceConfig struct {
	// PreloadDiffs is the number of stash diffs to preload on startup.
	PreloadDiffs int `toml:"preload_diffs"`
	// SearchIndex controls when the search index is built: "eager" or "lazy".
	SearchIndex string `toml:"search_index"`
	// DiffCacheSize is the maximum number of diffs to keep in the LRU cache.
	DiffCacheSize int `toml:"diff_cache_size"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	// Level is the log level: "off", "error", "warn", "info", "debug".
	Level string `toml:"level"`
	// File is the log file path. Empty uses the default XDG state path.
	File string `toml:"file"`
}

// CLIFlags holds values parsed from command-line flags.
// Pointer types are used so we can distinguish "not set" (nil) from "set to zero value".
type CLIFlags struct {
	LogLevel    *string
	TraceGit    *bool
	Debug       *bool
	NoColor     *bool
	NoAnimation *bool
	Icons       *string
	Directory   *string
}

// StaleThreshold returns the staleness duration based on StaleDays.
func (c *Config) StaleThreshold() time.Duration {
	return time.Duration(c.General.StaleDays) * 24 * time.Hour
}
```

### Step 2: Create `internal/config/defaults.go`

```go
package config

// DefaultConfig returns a Config populated with all default values
// from PRD section 12.6.
//
// Every default is chosen to match what a developer would configure
// if they had infinite time.
func DefaultConfig() Config {
	return Config{
		General: GeneralConfig{
			Icons:       "auto",    // Detect and use the best available
			StaleDays:   14,        // Two weeks is long enough to forget what a stash was for
			KeepIndex:   true,      // Most stash operations should preserve staged work
			AutoMessage: true,      // "WIP on main: abc1234" is useless, auto-generated is useful
		},
		Export: ExportConfig{
			Ref:    "refs/stashes/$USER", // Namespaced per user, won't collide
			Remote: "origin",
		},
		Theme: ThemeConfig{
			Name: "agni", // Purpose-built for nidhi
		},
		Keys: KeysConfig{},
		Performance: PerformanceConfig{
			PreloadDiffs:  10,     // Preload diffs for the 10 most recent stashes
			SearchIndex:   "lazy", // Don't pay the cost until the user searches
			DiffCacheSize: 50,     // Enough for typical usage
		},
		Log: LogConfig{
			Level: "off",
			File:  "", // Default XDG state path resolved at runtime
		},
	}
}
```

### Step 3: Create `internal/config/gitconfig.go`

```go
package config

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// gitConfigTimeout is the timeout for git config lookups.
const gitConfigTimeout = 5 * time.Second

// LoadFromGitConfig reads nidhi-specific values from git config.
// Keys are read via `git config --get nidhi.<key>`.
// Only keys that exist in git config override the provided config.
func LoadFromGitConfig(cfg *Config) {
	ctx, cancel := context.WithTimeout(context.Background(), gitConfigTimeout)
	defer cancel()

	if v, ok := gitConfigGet(ctx, "nidhi.stale-days"); ok {
		if days, err := strconv.Atoi(v); err == nil && days > 0 {
			cfg.General.StaleDays = days
		}
	}

	if v, ok := gitConfigGet(ctx, "nidhi.keep-index"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.General.KeepIndex = b
		}
	}

	if v, ok := gitConfigGet(ctx, "nidhi.icons"); ok {
		v = strings.TrimSpace(v)
		if v == "auto" || v == "nerd" || v == "ascii" {
			cfg.General.Icons = v
		}
	}

	if v, ok := gitConfigGet(ctx, "nidhi.export-ref"); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			cfg.Export.Ref = v
		}
	}

	if v, ok := gitConfigGet(ctx, "nidhi.auto-message"); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.General.AutoMessage = b
		}
	}

	if v, ok := gitConfigGet(ctx, "nidhi.theme"); ok {
		v = strings.TrimSpace(v)
		if v != "" {
			cfg.Theme.Name = v
		}
	}

	if v, ok := gitConfigGet(ctx, "nidhi.log-level"); ok {
		v = strings.TrimSpace(v)
		if isValidLogLevel(v) {
			cfg.Log.Level = v
		}
	}
}

// gitConfigGet runs `git config --get <key>` and returns the value.
// Returns ("", false) if the key is not set or git is not available.
func gitConfigGet(ctx context.Context, key string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", key)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// isValidLogLevel checks if the given string is a valid log level.
func isValidLogLevel(level string) bool {
	switch level {
	case "off", "error", "warn", "info", "debug":
		return true
	}
	return false
}
```

### Step 4: Create `internal/config/loader.go`

```go
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	toml "github.com/pelletier/go-toml/v2"
)

// Load resolves configuration from all sources in priority order:
//   1. CLI flags (highest)
//   2. Environment variables (NIDHI_*)
//   3. Git config (nidhi.* section)
//   4. Config file (~/.config/nidhi/config.toml)
//   5. Built-in defaults (lowest)
//
// Each higher-priority source overrides lower-priority values.
func Load(flags CLIFlags) (Config, error) {
	// Start with defaults (lowest priority)
	cfg := DefaultConfig()

	// Layer 4: TOML config file
	if err := loadFromTOML(&cfg); err != nil {
		// Config file is optional -- only error if file exists but is invalid
		if !os.IsNotExist(err) {
			return cfg, err
		}
	}

	// Layer 3: Git config
	LoadFromGitConfig(&cfg)

	// Layer 2: Environment variables
	loadFromEnv(&cfg)

	// Layer 1: CLI flags (highest priority)
	applyFlags(&cfg, flags)

	return cfg, nil
}

// ConfigFilePath returns the path to the TOML config file.
// Uses XDG_CONFIG_HOME (~/.config/nidhi/config.toml by default).
func ConfigFilePath() string {
	return filepath.Join(xdg.ConfigHome, "nidhi", "config.toml")
}

// loadFromTOML reads and parses the TOML config file.
func loadFromTOML(cfg *Config) error {
	path := ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, cfg)
}

// loadFromEnv reads NIDHI_* environment variables and applies them.
func loadFromEnv(cfg *Config) {
	if v := os.Getenv("NIDHI_STALE_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil && days > 0 {
			cfg.General.StaleDays = days
		}
	}

	if v := os.Getenv("NIDHI_ICONS"); v != "" {
		v = strings.TrimSpace(v)
		if v == "auto" || v == "nerd" || v == "ascii" {
			cfg.General.Icons = v
		}
	}

	if v := os.Getenv("NIDHI_LOG_LEVEL"); v != "" {
		v = strings.TrimSpace(v)
		if isValidLogLevel(v) {
			cfg.Log.Level = v
		}
	}

	if v := os.Getenv("NIDHI_EXPORT_REF"); v != "" {
		cfg.Export.Ref = strings.TrimSpace(v)
	}

	if v := os.Getenv("NIDHI_THEME"); v != "" {
		cfg.Theme.Name = strings.TrimSpace(v)
	}
}

// applyFlags applies CLI flag overrides. Only non-nil pointer values override.
func applyFlags(cfg *Config, flags CLIFlags) {
	if flags.LogLevel != nil {
		if isValidLogLevel(*flags.LogLevel) {
			cfg.Log.Level = *flags.LogLevel
		}
	}

	if flags.Icons != nil {
		v := strings.TrimSpace(*flags.Icons)
		if v == "auto" || v == "nerd" || v == "ascii" {
			cfg.General.Icons = v
		}
	}
}
```

### Step 5: Create `internal/config/config_test.go`

```go
package config_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/config"
)

// --- DefaultConfig tests ---

func TestDefaultConfig_Values(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"Icons", cfg.General.Icons, "auto"},
		{"StaleDays", cfg.General.StaleDays, 14},
		{"KeepIndex", cfg.General.KeepIndex, true},
		{"AutoMessage", cfg.General.AutoMessage, true},
		{"ExportRef", cfg.Export.Ref, "refs/stashes/$USER"},
		{"ExportRemote", cfg.Export.Remote, "origin"},
		{"ThemeName", cfg.Theme.Name, "agni"},
		{"PreloadDiffs", cfg.Performance.PreloadDiffs, 10},
		{"SearchIndex", cfg.Performance.SearchIndex, "lazy"},
		{"DiffCacheSize", cfg.Performance.DiffCacheSize, 50},
		{"LogLevel", cfg.Log.Level, "off"},
		{"LogFile", cfg.Log.File, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestDefaultConfig_StaleThreshold(t *testing.T) {
	cfg := config.DefaultConfig()
	threshold := cfg.StaleThreshold()

	// 14 days in hours
	wantHours := 14 * 24
	if int(threshold.Hours()) != wantHours {
		t.Errorf("StaleThreshold() = %v hours, want %v hours", threshold.Hours(), wantHours)
	}
}

// --- TOML parsing tests ---

func TestLoad_TOMLFile(t *testing.T) {
	// Create a temp config directory and file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "nidhi")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `
[general]
icons = "nerd"
stale_days = 7
keep_index = false
auto_message = false

[export]
ref = "refs/stashes/testuser"
remote = "upstream"

[theme]
name = "custom"

[performance]
preload_diffs = 5
search_index = "eager"
diff_cache_size = 100

[log]
level = "debug"
file = "/tmp/nidhi.log"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set XDG_CONFIG_HOME to our temp dir so Load() finds the config
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Clear env vars that might interfere
	t.Setenv("NIDHI_STALE_DAYS", "")
	t.Setenv("NIDHI_ICONS", "")
	t.Setenv("NIDHI_LOG_LEVEL", "")
	t.Setenv("NIDHI_EXPORT_REF", "")
	t.Setenv("NIDHI_THEME", "")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.General.Icons != "nerd" {
		t.Errorf("Icons = %q, want %q", cfg.General.Icons, "nerd")
	}
	if cfg.General.StaleDays != 7 {
		t.Errorf("StaleDays = %d, want %d", cfg.General.StaleDays, 7)
	}
	if cfg.General.KeepIndex != false {
		t.Errorf("KeepIndex = %v, want false", cfg.General.KeepIndex)
	}
	if cfg.Export.Ref != "refs/stashes/testuser" {
		t.Errorf("ExportRef = %q, want %q", cfg.Export.Ref, "refs/stashes/testuser")
	}
	if cfg.Performance.SearchIndex != "eager" {
		t.Errorf("SearchIndex = %q, want %q", cfg.Performance.SearchIndex, "eager")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.Log.Level, "debug")
	}
}

// --- Environment variable tests ---

func TestLoad_EnvVars(t *testing.T) {
	// Use a non-existent config dir so TOML doesn't interfere
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	t.Setenv("NIDHI_STALE_DAYS", "3")
	t.Setenv("NIDHI_ICONS", "ascii")
	t.Setenv("NIDHI_LOG_LEVEL", "info")
	t.Setenv("NIDHI_EXPORT_REF", "refs/stashes/envuser")
	t.Setenv("NIDHI_THEME", "dark")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.General.StaleDays != 3 {
		t.Errorf("StaleDays = %d, want 3", cfg.General.StaleDays)
	}
	if cfg.General.Icons != "ascii" {
		t.Errorf("Icons = %q, want %q", cfg.General.Icons, "ascii")
	}
	if cfg.Log.Level != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Export.Ref != "refs/stashes/envuser" {
		t.Errorf("ExportRef = %q, want %q", cfg.Export.Ref, "refs/stashes/envuser")
	}
	if cfg.Theme.Name != "dark" {
		t.Errorf("ThemeName = %q, want %q", cfg.Theme.Name, "dark")
	}
}

func TestLoad_EnvVars_InvalidValues(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Invalid values should be ignored, defaults preserved
	t.Setenv("NIDHI_STALE_DAYS", "not-a-number")
	t.Setenv("NIDHI_ICONS", "invalid-icon-mode")
	t.Setenv("NIDHI_LOG_LEVEL", "invalid-level")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Should fall back to defaults
	if cfg.General.StaleDays != 14 {
		t.Errorf("StaleDays = %d, want default 14", cfg.General.StaleDays)
	}
	if cfg.General.Icons != "auto" {
		t.Errorf("Icons = %q, want default %q", cfg.General.Icons, "auto")
	}
	if cfg.Log.Level != "off" {
		t.Errorf("LogLevel = %q, want default %q", cfg.Log.Level, "off")
	}
}

// --- CLI flags priority tests ---

func TestLoad_CLIFlags_OverrideAll(t *testing.T) {
	// Set up TOML config
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "nidhi")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tomlContent := `
[general]
icons = "nerd"
[log]
level = "error"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Set env vars (should be overridden by CLI)
	t.Setenv("NIDHI_ICONS", "ascii")
	t.Setenv("NIDHI_LOG_LEVEL", "warn")

	// CLI flags have highest priority
	logLevel := "debug"
	icons := "auto"
	flags := config.CLIFlags{
		LogLevel: &logLevel,
		Icons:    &icons,
	}

	cfg, err := config.Load(flags)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// CLI flags should win over env vars and TOML
	if cfg.Log.Level != "debug" {
		t.Errorf("LogLevel = %q, want %q (CLI should override)", cfg.Log.Level, "debug")
	}
	if cfg.General.Icons != "auto" {
		t.Errorf("Icons = %q, want %q (CLI should override)", cfg.General.Icons, "auto")
	}
}

// --- Priority order integration test ---

func TestLoad_PriorityOrder(t *testing.T) {
	// This test verifies the complete priority chain:
	// CLI flags > env vars > git config > TOML > defaults
	//
	// We test: TOML sets stale_days=7, env sets stale_days (skipped),
	// CLI does not set stale_days. Result should be 7 (from TOML).

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "nidhi")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tomlContent := `
[general]
stale_days = 7
icons = "nerd"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("NIDHI_STALE_DAYS", "")
	t.Setenv("NIDHI_ICONS", "")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// TOML should override default (14 -> 7)
	if cfg.General.StaleDays != 7 {
		t.Errorf("StaleDays = %d, want 7 (TOML override)", cfg.General.StaleDays)
	}
	if cfg.General.Icons != "nerd" {
		t.Errorf("Icons = %q, want %q (TOML override)", cfg.General.Icons, "nerd")
	}
	// KeepIndex not in TOML, should be default (true)
	if cfg.General.KeepIndex != true {
		t.Errorf("KeepIndex = %v, want true (default)", cfg.General.KeepIndex)
	}
}

// --- Git config tests ---

func TestLoadFromGitConfig(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	// Create a temp git repo with nidhi config keys
	dir := t.TempDir()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@test.com")
	runCmd(t, dir, "git", "config", "user.name", "Test")
	runCmd(t, dir, "git", "config", "nidhi.stale-days", "21")
	runCmd(t, dir, "git", "config", "nidhi.icons", "ascii")
	runCmd(t, dir, "git", "config", "nidhi.keep-index", "false")
	runCmd(t, dir, "git", "config", "nidhi.export-ref", "refs/stashes/gituser")

	// Change to the temp repo directory so git config reads local config
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg := config.DefaultConfig()
	config.LoadFromGitConfig(&cfg)

	if cfg.General.StaleDays != 21 {
		t.Errorf("StaleDays = %d, want 21", cfg.General.StaleDays)
	}
	if cfg.General.Icons != "ascii" {
		t.Errorf("Icons = %q, want %q", cfg.General.Icons, "ascii")
	}
	if cfg.General.KeepIndex != false {
		t.Errorf("KeepIndex = %v, want false", cfg.General.KeepIndex)
	}
	if cfg.Export.Ref != "refs/stashes/gituser" {
		t.Errorf("ExportRef = %q, want %q", cfg.Export.Ref, "refs/stashes/gituser")
	}
}

// --- No config file test ---

func TestLoad_NoConfigFile(t *testing.T) {
	// Point to non-existent config dir
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Clear env vars
	t.Setenv("NIDHI_STALE_DAYS", "")
	t.Setenv("NIDHI_ICONS", "")
	t.Setenv("NIDHI_LOG_LEVEL", "")
	t.Setenv("NIDHI_EXPORT_REF", "")
	t.Setenv("NIDHI_THEME", "")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() should not error when config file is missing: %v", err)
	}

	// Should match all defaults
	defaults := config.DefaultConfig()
	if cfg.General.Icons != defaults.General.Icons {
		t.Errorf("Icons = %q, want default %q", cfg.General.Icons, defaults.General.Icons)
	}
	if cfg.General.StaleDays != defaults.General.StaleDays {
		t.Errorf("StaleDays = %d, want default %d", cfg.General.StaleDays, defaults.General.StaleDays)
	}
}

// --- Invalid TOML test ---

func TestLoad_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "nidhi")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("this is not valid toml [[["), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	_, err := config.Load(config.CLIFlags{})
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}

// --- Helper ---

func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
```

### Step 6: Verify compilation

```bash
go build ./internal/config/...
```

### Step 7: Run tests

```bash
go test -v -race -count=1 ./internal/config/...
```

### Step 8: Run `make ci`

```bash
make ci
```

## Verification

### Functional
```bash
# Package compiles
go build ./internal/config/...

# All tests pass with race detector
go test -v -race -count=1 ./internal/config/...

# Default config tests
go test -v -run TestDefaultConfig ./internal/config/...

# TOML parsing tests
go test -v -run TestLoad_TOMLFile ./internal/config/...

# Env var tests
go test -v -run TestLoad_EnvVars ./internal/config/...

# CLI flag priority tests
go test -v -run TestLoad_CLIFlags ./internal/config/...

# Priority order integration test
go test -v -run TestLoad_PriorityOrder ./internal/config/...

# Git config tests
go test -v -run TestLoadFromGitConfig ./internal/config/...

# No config file graceful handling
go test -v -run TestLoad_NoConfigFile ./internal/config/...

# Invalid TOML error handling
go test -v -run TestLoad_InvalidTOML ./internal/config/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `Config` struct has all fields from PRD section 12.2: General (icons, stale_days, keep_index, auto_message), Export (ref, remote), Theme (name), Keys, Performance (preload_diffs, search_index, diff_cache_size), Log (level, file)
2. `DefaultConfig()` returns all defaults from PRD section 12.6 exactly
3. `Config.StaleThreshold()` returns correct `time.Duration` from `StaleDays`
4. `LoadFromGitConfig` reads from `git config --get nidhi.<key>` for: stale-days, keep-index, icons, export-ref, auto-message, theme, log-level
5. `Load()` implements priority: CLI flags > env vars > git config > TOML > defaults
6. Env vars supported: NIDHI_STALE_DAYS, NIDHI_ICONS, NIDHI_LOG_LEVEL, NIDHI_EXPORT_REF, NIDHI_THEME
7. Invalid values in env vars and git config are silently ignored (defaults preserved)
8. Missing config file is not an error (defaults used)
9. Invalid TOML file returns an error
10. `CLIFlags` uses pointer types to distinguish "not set" from "set to zero value"
11. All tests pass with `-race` flag
12. `make ci` passes

## Commit
```
feat: add config loading with TOML, git config, env vars, and defaults

Implement internal/config/ with Config struct (PRD section 12.2), defaults
(section 12.6), git config reader (section 12.3), env var loader (section
12.4), and priority-based Load() (section 12.1). Table-driven tests for
each source, priority override, and edge cases.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 12.1-12.6
4. Execute steps 1-8 in order
5. Verify all functional and CI checks pass
6. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
7. Commit with the message above + move to next task
