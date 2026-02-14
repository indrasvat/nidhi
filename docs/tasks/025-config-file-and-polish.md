# Task 025: Configuration File Support + Final Polish

## Status: TODO

## Depends On
- 002 (config loading — Config struct, defaults, gitconfig reader)
- 006 (core model — AppState, config integration points)

## Parallelizable With
- 020 (search plugin)
- 021 (filter and stale plugins)
- 022 (reorder plugin)
- 023 (export/import plugin)
- 024 (help overlay and mouse support)

## Problem
nidhi currently relies on built-in defaults with no way for users to customize behavior. The PRD specifies a four-source configuration pipeline — TOML file, git config, environment variables, and CLI flags — with strict priority ordering: CLI > env > git config > TOML > defaults. This task wires all four sources into the config loader, implements the TOML parser, adds environment variable support, connects CLI flags, sets up structured logging to a state file, and implements the `--debug` startup timing mode. This is the final integration task that makes nidhi configurable and observable.

## PRD Reference
- Section 12.1 (Configuration Sources Priority) — CLI > env > git config > TOML > defaults
- Section 12.2 (Config File Format) — `~/.config/nidhi/config.toml` with all sections
- Section 12.3 (Git Config Integration) — `nidhi.*` section in git config
- Section 12.4 (Environment Variables) — `NIDHI_*` vars, `NO_COLOR`, `REDUCE_MOTION`, `NERD_FONTS`
- Section 12.5 (CLI Flags) — `--log-level`, `--trace-git`, `--debug`, `--no-color`, `--no-animation`, `--icons`, `-C`
- Section 12.6 (Sane Defaults Philosophy) — every default chosen for optimal UX
- Section 7.5 (Observability) — `--debug` prints timing, `--trace-git` logs git commands, `--log-level` controls log output
- Section 4.2 (Supporting Libraries) — `go-toml/v2` for TOML, `adrg/xdg` for XDG paths
- Section 8.4 (Module structure) — `internal/config/config.go`, `internal/config/defaults.go`, `internal/config/gitconfig.go`

## Files to Create
- `internal/config/toml.go` — TOML config file parser
- `internal/config/envvars.go` — environment variable parser
- `internal/config/flags.go` — CLI flags parser and integration
- `internal/config/loader.go` — unified config loader with priority resolution
- `internal/config/logging.go` — structured logging setup via `log/slog`
- `internal/config/debug.go` — `--debug` startup timing breakdown
- `internal/config/config_test.go` — unit and integration tests

## Files to Modify
- `internal/config/config.go` — extend Config struct with all fields from PRD §12.2
- `internal/config/defaults.go` — ensure all defaults match PRD §12.6
- `internal/config/gitconfig.go` — ensure git config reader covers all `nidhi.*` keys
- `cmd/nidhi/main.go` — wire config loader, logging, --debug flag

## Execution Steps

### Step 1: Extend Config struct (`internal/config/config.go`)

Ensure the Config struct covers all configurable values from PRD §12.2:

```go
package config

import "time"

// Config holds all nidhi configuration.
type Config struct {
	General     GeneralConfig
	Export      ExportConfig
	Theme       ThemeConfig
	Keys        KeysConfig
	Performance PerformanceConfig
	Log         LogConfig

	// CLI-only flags (not in config file).
	Debug       bool   // --debug: print timing and exit.
	TraceGit    bool   // --trace-git: log all git commands.
	NoColor     bool   // --no-color: disable all colors.
	NoAnimation bool   // --no-animation: disable animations.
	Directory   string // -C: run as if started in <path>.
}

// GeneralConfig holds general settings.
type GeneralConfig struct {
	Icons       string // "auto", "nerd", "ascii". Default: "auto".
	StaleDays   int    // Staleness threshold in days. Default: 14.
	KeepIndex   bool   // Keep index when creating stashes. Default: true.
	AutoMessage bool   // Auto-generate readable messages. Default: true.
}

// ExportConfig holds export/import settings.
type ExportConfig struct {
	Ref    string // Default ref for export. Default: "refs/stashes/$USER".
	Remote string // Default remote. Default: "origin".
}

// ThemeConfig holds theme settings.
type ThemeConfig struct {
	Name string // "agni" or path to custom theme. Default: "agni".
}

// KeysConfig holds keybinding overrides.
type KeysConfig struct {
	Overrides map[string]string // action -> key, e.g. "apply" -> "a".
}

// PerformanceConfig holds performance tuning settings.
type PerformanceConfig struct {
	PreloadDiffs  int    // Max diffs to preload on startup. Default: 10.
	SearchIndex   string // "lazy" or "eager". Default: "lazy".
	DiffCacheSize int    // Max cached diffs in memory. Default: 50.
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string // "off", "error", "warn", "info", "debug". Default: "off".
	File  string // Log file path. Default: XDG state dir.
}
```

### Step 2: Create TOML parser (`internal/config/toml.go`)

```go
package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	toml "github.com/pelletier/go-toml/v2"
)

// DefaultConfigPath returns the default TOML config file path.
// Uses XDG config directory: ~/.config/nidhi/config.toml
func DefaultConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "nidhi", "config.toml")
}

// tomlFile mirrors the TOML structure for deserialization.
type tomlFile struct {
	General struct {
		Icons       *string `toml:"icons"`
		StaleDays   *int    `toml:"stale_days"`
		KeepIndex   *bool   `toml:"keep_index"`
		AutoMessage *bool   `toml:"auto_message"`
	} `toml:"general"`
	Export struct {
		Ref    *string `toml:"ref"`
		Remote *string `toml:"remote"`
	} `toml:"export"`
	Theme struct {
		Name *string `toml:"name"`
	} `toml:"theme"`
	Keys struct {
		Overrides map[string]string `toml:"overrides"`
	} `toml:"keys"`
	Performance struct {
		PreloadDiffs  *int    `toml:"preload_diffs"`
		SearchIndex   *string `toml:"search_index"`
		DiffCacheSize *int    `toml:"diff_cache_size"`
	} `toml:"performance"`
	Log struct {
		Level *string `toml:"level"`
		File  *string `toml:"file"`
	} `toml:"log"`
}

// LoadTOML reads and parses a TOML config file.
// Returns an empty Config (no overrides) if the file does not exist.
// Uses pointer fields to distinguish "not set" from "set to zero value".
func LoadTOML(path string) (*tomlFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &tomlFile{}, nil
	}
	if err != nil {
		return nil, err
	}

	var tf tomlFile
	if err := toml.Unmarshal(data, &tf); err != nil {
		return nil, err
	}
	return &tf, nil
}

// applyTOML applies non-nil TOML values to a Config struct.
func applyTOML(cfg *Config, tf *tomlFile) {
	if tf.General.Icons != nil {
		cfg.General.Icons = *tf.General.Icons
	}
	if tf.General.StaleDays != nil {
		cfg.General.StaleDays = *tf.General.StaleDays
	}
	if tf.General.KeepIndex != nil {
		cfg.General.KeepIndex = *tf.General.KeepIndex
	}
	if tf.General.AutoMessage != nil {
		cfg.General.AutoMessage = *tf.General.AutoMessage
	}
	if tf.Export.Ref != nil {
		cfg.Export.Ref = *tf.Export.Ref
	}
	if tf.Export.Remote != nil {
		cfg.Export.Remote = *tf.Export.Remote
	}
	if tf.Theme.Name != nil {
		cfg.Theme.Name = *tf.Theme.Name
	}
	if len(tf.Keys.Overrides) > 0 {
		cfg.Keys.Overrides = tf.Keys.Overrides
	}
	if tf.Performance.PreloadDiffs != nil {
		cfg.Performance.PreloadDiffs = *tf.Performance.PreloadDiffs
	}
	if tf.Performance.SearchIndex != nil {
		cfg.Performance.SearchIndex = *tf.Performance.SearchIndex
	}
	if tf.Performance.DiffCacheSize != nil {
		cfg.Performance.DiffCacheSize = *tf.Performance.DiffCacheSize
	}
	if tf.Log.Level != nil {
		cfg.Log.Level = *tf.Log.Level
	}
	if tf.Log.File != nil {
		cfg.Log.File = *tf.Log.File
	}
}
```

### Step 3: Create environment variable parser (`internal/config/envvars.go`)

```go
package config

import (
	"os"
	"strconv"
)

// EnvVarMap maps environment variable names to config field paths.
// From PRD §12.4.
var EnvVarMap = map[string]string{
	"NIDHI_STALE_DAYS": "general.stale_days",
	"NIDHI_ICONS":      "general.icons",
	"NIDHI_LOG_LEVEL":  "log.level",
	"NIDHI_EXPORT_REF": "export.ref",
	"NIDHI_THEME":      "theme.name",
}

// StandardEnvVars are non-NIDHI env vars we respect.
// From PRD §12.4.
var StandardEnvVars = []string{
	"NO_COLOR",       // Standard: disable all color.
	"REDUCE_MOTION",  // Standard: disable animations.
	"NERD_FONTS",     // Force Nerd Font on/off.
}

// applyEnvVars reads environment variables and applies them to Config.
// Only overrides values that are explicitly set in the environment.
func applyEnvVars(cfg *Config) {
	if v := os.Getenv("NIDHI_STALE_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil {
			cfg.General.StaleDays = days
		}
	}
	if v := os.Getenv("NIDHI_ICONS"); v != "" {
		cfg.General.Icons = v
	}
	if v := os.Getenv("NIDHI_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("NIDHI_EXPORT_REF"); v != "" {
		cfg.Export.Ref = v
	}
	if v := os.Getenv("NIDHI_THEME"); v != "" {
		cfg.Theme.Name = v
	}

	// Standard env vars.
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		cfg.NoColor = true
	}
	if _, ok := os.LookupEnv("REDUCE_MOTION"); ok {
		cfg.NoAnimation = true
	}
	if v := os.Getenv("NERD_FONTS"); v == "1" || v == "true" {
		cfg.General.Icons = "nerd"
	} else if v == "0" || v == "false" {
		cfg.General.Icons = "ascii"
	}
}
```

### Step 4: Create CLI flags parser (`internal/config/flags.go`)

```go
package config

// CLIFlags holds values parsed from command-line flags.
// Pointer types distinguish "not set" from "set to zero/empty value".
type CLIFlags struct {
	LogLevel    *string // --log-level
	TraceGit    *bool   // --trace-git
	Debug       *bool   // --debug
	NoColor     *bool   // --no-color
	NoAnimation *bool   // --no-animation
	Icons       *string // --icons
	Directory   *string // -C, --directory
}

// applyCLIFlags applies CLI flag values to Config.
// CLI flags have the highest priority — they override everything.
func applyCLIFlags(cfg *Config, flags *CLIFlags) {
	if flags == nil {
		return
	}
	if flags.LogLevel != nil {
		cfg.Log.Level = *flags.LogLevel
	}
	if flags.TraceGit != nil {
		cfg.TraceGit = *flags.TraceGit
	}
	if flags.Debug != nil {
		cfg.Debug = *flags.Debug
	}
	if flags.NoColor != nil {
		cfg.NoColor = *flags.NoColor
	}
	if flags.NoAnimation != nil {
		cfg.NoAnimation = *flags.NoAnimation
	}
	if flags.Icons != nil {
		cfg.General.Icons = *flags.Icons
	}
	if flags.Directory != nil {
		cfg.Directory = *flags.Directory
	}
}
```

### Step 5: Create unified config loader (`internal/config/loader.go`)

```go
package config

import (
	"context"
	"fmt"
)

// GitConfigReader reads git config values.
type GitConfigReader interface {
	Get(ctx context.Context, key string) (string, error)
}

// LoadConfig loads configuration from all sources with priority resolution.
// Priority: CLI flags > env vars > git config > TOML file > built-in defaults.
//
// Steps:
// 1. Start with built-in defaults.
// 2. Load TOML file and apply.
// 3. Load git config values and apply.
// 4. Apply environment variables.
// 5. Apply CLI flags (highest priority).
func LoadConfig(ctx context.Context, tomlPath string, gitCfg GitConfigReader, flags *CLIFlags) (*Config, error) {
	// Step 1: Built-in defaults.
	cfg := DefaultConfig()

	// Step 2: TOML file.
	tf, err := LoadTOML(tomlPath)
	if err != nil {
		return nil, fmt.Errorf("load config file %s: %w", tomlPath, err)
	}
	applyTOML(&cfg, tf)

	// Step 3: Git config.
	if gitCfg != nil {
		applyGitConfig(ctx, &cfg, gitCfg)
	}

	// Step 4: Environment variables.
	applyEnvVars(&cfg)

	// Step 5: CLI flags.
	applyCLIFlags(&cfg, flags)

	return &cfg, nil
}

// DefaultConfig returns the built-in default configuration.
// Every default matches PRD §12.6 "Sane Defaults Philosophy".
func DefaultConfig() Config {
	return Config{
		General: GeneralConfig{
			Icons:       "auto",
			StaleDays:   14,
			KeepIndex:   true,
			AutoMessage: true,
		},
		Export: ExportConfig{
			Ref:    "refs/stashes/$USER",
			Remote: "origin",
		},
		Theme: ThemeConfig{
			Name: "agni",
		},
		Keys: KeysConfig{
			Overrides: make(map[string]string),
		},
		Performance: PerformanceConfig{
			PreloadDiffs:  10,
			SearchIndex:   "lazy",
			DiffCacheSize: 50,
		},
		Log: LogConfig{
			Level: "off",
			File:  "", // Empty = use DefaultLogPath().
		},
	}
}

// applyGitConfig reads nidhi.* keys from git config and applies them.
func applyGitConfig(ctx context.Context, cfg *Config, reader GitConfigReader) {
	if v, err := reader.Get(ctx, "nidhi.stale-days"); err == nil && v != "" {
		if days, err := parseInt(v); err == nil {
			cfg.General.StaleDays = days
		}
	}
	if v, err := reader.Get(ctx, "nidhi.keep-index"); err == nil && v != "" {
		cfg.General.KeepIndex = v == "true" || v == "1"
	}
	if v, err := reader.Get(ctx, "nidhi.icons"); err == nil && v != "" {
		cfg.General.Icons = v
	}
	if v, err := reader.Get(ctx, "nidhi.export-ref"); err == nil && v != "" {
		cfg.Export.Ref = v
	}
	if v, err := reader.Get(ctx, "nidhi.export-remote"); err == nil && v != "" {
		cfg.Export.Remote = v
	}
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
```

### Step 6: Create logging setup (`internal/config/logging.go`)

```go
package config

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// DefaultLogPath returns the default log file path.
// Uses XDG state directory: ~/.local/state/nidhi/nidhi.log
func DefaultLogPath() string {
	return filepath.Join(xdg.StateHome, "nidhi", "nidhi.log")
}

// SetupLogging initializes structured logging based on config.
// Returns the logger and a cleanup function to close the log file.
func SetupLogging(cfg *Config) (*slog.Logger, func(), error) {
	if cfg.Log.Level == "off" && !cfg.TraceGit {
		// No logging — return a no-op logger.
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}, nil
	}

	// Determine log file path.
	logPath := cfg.Log.File
	if logPath == "" {
		logPath = DefaultLogPath()
	}

	// Ensure directory exists.
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}

	// Open log file (append mode).
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	// Parse log level.
	level := parseLogLevel(cfg.Log.Level)
	if cfg.TraceGit {
		// trace-git implies at least debug level.
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)

	cleanup := func() {
		f.Close()
	}

	return logger, cleanup, nil
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelError // "off" still creates a logger with error level.
	}
}
```

### Step 7: Create debug timing (`internal/config/debug.go`)

```go
package config

import (
	"fmt"
	"os"
	"time"
)

// TimingEntry records a named timing measurement.
type TimingEntry struct {
	Name     string
	Duration time.Duration
}

// DebugTiming collects startup timing measurements and prints them.
type DebugTiming struct {
	start   time.Time
	entries []TimingEntry
}

// NewDebugTiming creates a new timing collector.
func NewDebugTiming() *DebugTiming {
	return &DebugTiming{
		start: time.Now(),
	}
}

// Record records a timing entry.
func (dt *DebugTiming) Record(name string, d time.Duration) {
	dt.entries = append(dt.entries, TimingEntry{Name: name, Duration: d})
}

// Since records the time since a given start for a named step.
func (dt *DebugTiming) Since(name string, start time.Time) {
	dt.entries = append(dt.entries, TimingEntry{Name: name, Duration: time.Since(start)})
}

// PrintAndExit prints the timing breakdown and exits with code 0.
// This implements the --debug flag behavior from PRD §7.5.
func (dt *DebugTiming) PrintAndExit() {
	total := time.Since(dt.start)

	fmt.Println("nidhi startup timing breakdown:")
	fmt.Println("================================")

	for _, e := range dt.entries {
		pct := float64(e.Duration) / float64(total) * 100
		fmt.Printf("  %-30s %8s  (%4.1f%%)\n", e.Name, e.Duration.Round(time.Microsecond), pct)
	}

	fmt.Printf("  %-30s %8s  (100%%)\n", "TOTAL", total.Round(time.Microsecond))
	fmt.Println()
	fmt.Printf("Interactive in: %s\n", total.Round(time.Millisecond))

	os.Exit(0)
}
```

### Step 8: Write tests (`internal/config/config_test.go`)

```go
package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/indrasvat/nidhi/internal/config"
)

// --- TOML Tests ---

// TestLoadTOMLConfig writes a TOML config file, loads it, and verifies values.
func TestLoadTOMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
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
preload_diffs = 20
search_index = "eager"
diff_cache_size = 100

[log]
level = "debug"
file = "/tmp/nidhi-test.log"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(context.Background(), path, nil, nil)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.General.Icons != "nerd" {
		t.Errorf("Icons: expected 'nerd', got %q", cfg.General.Icons)
	}
	if cfg.General.StaleDays != 7 {
		t.Errorf("StaleDays: expected 7, got %d", cfg.General.StaleDays)
	}
	if cfg.General.KeepIndex != false {
		t.Errorf("KeepIndex: expected false, got true")
	}
	if cfg.General.AutoMessage != false {
		t.Errorf("AutoMessage: expected false, got true")
	}
	if cfg.Export.Ref != "refs/stashes/testuser" {
		t.Errorf("Export.Ref: expected 'refs/stashes/testuser', got %q", cfg.Export.Ref)
	}
	if cfg.Export.Remote != "upstream" {
		t.Errorf("Export.Remote: expected 'upstream', got %q", cfg.Export.Remote)
	}
	if cfg.Theme.Name != "custom" {
		t.Errorf("Theme.Name: expected 'custom', got %q", cfg.Theme.Name)
	}
	if cfg.Performance.PreloadDiffs != 20 {
		t.Errorf("PreloadDiffs: expected 20, got %d", cfg.Performance.PreloadDiffs)
	}
	if cfg.Performance.SearchIndex != "eager" {
		t.Errorf("SearchIndex: expected 'eager', got %q", cfg.Performance.SearchIndex)
	}
	if cfg.Performance.DiffCacheSize != 100 {
		t.Errorf("DiffCacheSize: expected 100, got %d", cfg.Performance.DiffCacheSize)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level: expected 'debug', got %q", cfg.Log.Level)
	}
	if cfg.Log.File != "/tmp/nidhi-test.log" {
		t.Errorf("Log.File: expected '/tmp/nidhi-test.log', got %q", cfg.Log.File)
	}
}

// TestLoadTOMLNonexistent verifies defaults are used when no file exists.
func TestLoadTOMLNonexistent(t *testing.T) {
	cfg, err := config.LoadConfig(context.Background(), "/nonexistent/config.toml", nil, nil)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	defaults := config.DefaultConfig()
	if cfg.General.StaleDays != defaults.General.StaleDays {
		t.Errorf("expected default StaleDays=%d, got %d", defaults.General.StaleDays, cfg.General.StaleDays)
	}
	if cfg.General.Icons != defaults.General.Icons {
		t.Errorf("expected default Icons=%q, got %q", defaults.General.Icons, cfg.General.Icons)
	}
}

// --- Environment Variable Tests ---

// TestEnvVarsOverrideTOML verifies that env vars override TOML values.
func TestEnvVarsOverrideTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[general]
stale_days = 7
icons = "nerd"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set env vars that should override TOML.
	t.Setenv("NIDHI_STALE_DAYS", "3")
	t.Setenv("NIDHI_ICONS", "ascii")

	cfg, err := config.LoadConfig(context.Background(), path, nil, nil)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.General.StaleDays != 3 {
		t.Errorf("expected StaleDays=3 from env, got %d", cfg.General.StaleDays)
	}
	if cfg.General.Icons != "ascii" {
		t.Errorf("expected Icons='ascii' from env, got %q", cfg.General.Icons)
	}
}

// TestNOCOLORDisablesColors verifies the NO_COLOR standard env var.
func TestNOCOLORDisablesColors(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	cfg, err := config.LoadConfig(context.Background(), "/nonexistent", nil, nil)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.NoColor {
		t.Error("expected NoColor=true when NO_COLOR is set")
	}
}

// TestREDUCEMOTIONDisablesAnimations verifies the REDUCE_MOTION env var.
func TestREDUCEMOTIONDisablesAnimations(t *testing.T) {
	t.Setenv("REDUCE_MOTION", "1")

	cfg, err := config.LoadConfig(context.Background(), "/nonexistent", nil, nil)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if !cfg.NoAnimation {
		t.Error("expected NoAnimation=true when REDUCE_MOTION is set")
	}
}

// --- Git Config Tests ---

// TestGitConfigOverridesToml verifies that git config values override TOML.
func TestGitConfigOverridesToml(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")

	content := `
[general]
stale_days = 14
icons = "auto"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a mock git config reader.
	mockGitCfg := &mockGitConfigReader{
		values: map[string]string{
			"nidhi.stale-days":  "21",
			"nidhi.icons":      "nerd",
			"nidhi.export-ref": "refs/stashes/fromgit",
		},
	}

	cfg, err := config.LoadConfig(context.Background(), tomlPath, mockGitCfg, nil)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Git config overrides TOML.
	if cfg.General.StaleDays != 21 {
		t.Errorf("expected StaleDays=21 from git config, got %d", cfg.General.StaleDays)
	}
	if cfg.General.Icons != "nerd" {
		t.Errorf("expected Icons='nerd' from git config, got %q", cfg.General.Icons)
	}
	if cfg.Export.Ref != "refs/stashes/fromgit" {
		t.Errorf("expected Ref from git config, got %q", cfg.Export.Ref)
	}
}

// --- CLI Flag Tests ---

// TestCLIFlagsOverrideEverything verifies that CLI flags have the highest priority.
func TestCLIFlagsOverrideEverything(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")

	content := `
[general]
icons = "nerd"

[log]
level = "info"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Set env var.
	t.Setenv("NIDHI_ICONS", "ascii")

	// Set CLI flags that should override everything.
	icons := "auto"
	logLevel := "debug"
	noColor := true
	debug := true
	flags := &config.CLIFlags{
		Icons:    &icons,
		LogLevel: &logLevel,
		NoColor:  &noColor,
		Debug:    &debug,
	}

	cfg, err := config.LoadConfig(context.Background(), tomlPath, nil, flags)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// CLI flags should win over both TOML and env vars.
	if cfg.General.Icons != "auto" {
		t.Errorf("expected Icons='auto' from CLI flag, got %q", cfg.General.Icons)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected Log.Level='debug' from CLI flag, got %q", cfg.Log.Level)
	}
	if !cfg.NoColor {
		t.Error("expected NoColor=true from CLI flag")
	}
	if !cfg.Debug {
		t.Error("expected Debug=true from CLI flag")
	}
}

// --- Priority Resolution Tests ---

// TestPriorityResolutionAllSources verifies the full priority chain:
// CLI > env > git config > TOML > defaults.
func TestPriorityResolutionAllSources(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")

	content := `
[general]
stale_days = 7
icons = "nerd"

[log]
level = "warn"
`
	if err := os.WriteFile(tomlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Git config sets stale_days=21 (overrides TOML's 7).
	mockGitCfg := &mockGitConfigReader{
		values: map[string]string{
			"nidhi.stale-days": "21",
		},
	}

	// Env var sets stale_days=3 (overrides git config's 21).
	t.Setenv("NIDHI_STALE_DAYS", "3")

	// CLI flag sets log_level=debug (overrides TOML's "warn").
	logLevel := "debug"
	flags := &config.CLIFlags{
		LogLevel: &logLevel,
	}

	cfg, err := config.LoadConfig(context.Background(), tomlPath, mockGitCfg, flags)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// stale_days: env (3) > git (21) > TOML (7) > default (14)
	if cfg.General.StaleDays != 3 {
		t.Errorf("stale_days: expected 3 (env), got %d", cfg.General.StaleDays)
	}

	// icons: TOML (nerd) — no env/git/CLI override for icons in this test.
	if cfg.General.Icons != "nerd" {
		t.Errorf("icons: expected 'nerd' (TOML), got %q", cfg.General.Icons)
	}

	// log level: CLI (debug) > TOML (warn)
	if cfg.Log.Level != "debug" {
		t.Errorf("log level: expected 'debug' (CLI), got %q", cfg.Log.Level)
	}

	// export.ref: no override → default
	if cfg.Export.Ref != "refs/stashes/$USER" {
		t.Errorf("export.ref: expected default, got %q", cfg.Export.Ref)
	}
}

// --- XDG Path Tests ---

// TestXDGPathResolution verifies that default paths use XDG directories.
func TestXDGPathResolution(t *testing.T) {
	configPath := config.DefaultConfigPath()
	if configPath == "" {
		t.Error("DefaultConfigPath returned empty string")
	}
	// Should contain "nidhi/config.toml".
	if !filepath.IsAbs(configPath) {
		t.Errorf("expected absolute path, got %q", configPath)
	}

	logPath := config.DefaultLogPath()
	if logPath == "" {
		t.Error("DefaultLogPath returned empty string")
	}
	if !filepath.IsAbs(logPath) {
		t.Errorf("expected absolute path, got %q", logPath)
	}
	// Should contain "nidhi/nidhi.log".
	if !contains(logPath, "nidhi") {
		t.Errorf("expected 'nidhi' in log path, got %q", logPath)
	}
}

// --- Debug Timing Tests ---

// TestDebugPrintsTiming verifies the debug timing breakdown output.
func TestDebugPrintsTiming(t *testing.T) {
	dt := config.NewDebugTiming()

	// Record some entries.
	dt.Record("config load", 5*time.Millisecond)
	dt.Record("git detection", 3*time.Millisecond)
	dt.Record("stash parsing", 15*time.Millisecond)
	dt.Record("first render", 10*time.Millisecond)

	// We can't test PrintAndExit (it calls os.Exit), but we can
	// verify the entries are recorded.
	if len(dt.Entries()) != 4 {
		t.Errorf("expected 4 timing entries, got %d", len(dt.Entries()))
	}
}

// --- Test Helpers ---

type mockGitConfigReader struct {
	values map[string]string
}

func (m *mockGitConfigReader) Get(_ context.Context, key string) (string, error) {
	v, ok := m.values[key]
	if !ok {
		return "", fmt.Errorf("key not found: %s", key)
	}
	return v, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

### Step 9: Verify

```bash
# All config tests.
go test -v -count=1 ./internal/config/...

# Full CI pipeline.
make ci
```

## Verification

### Functional
```bash
# TOML tests pass
go test -v -count=1 -run 'TestLoadTOML' ./internal/config/...

# Env var tests pass
go test -v -count=1 -run 'TestEnvVars|TestNOCOLOR|TestREDUCEMOTION' ./internal/config/...

# Git config tests pass
go test -v -count=1 -run 'TestGitConfig' ./internal/config/...

# CLI flag tests pass
go test -v -count=1 -run 'TestCLIFlags' ./internal/config/...

# Priority resolution tests pass
go test -v -count=1 -run 'TestPriority' ./internal/config/...

# XDG path tests pass
go test -v -count=1 -run 'TestXDG' ./internal/config/...

# Debug timing tests pass
go test -v -count=1 -run 'TestDebug' ./internal/config/...

# Compiles and passes vet
go vet ./internal/config/...

# Lint clean
golangci-lint run ./internal/config/...
```

### CI Pipeline
```bash
make ci
```

## Completion Criteria
1. `internal/config/toml.go` parses TOML from `~/.config/nidhi/config.toml` with all sections from PRD §12.2
2. `internal/config/envvars.go` parses all `NIDHI_*` env vars from PRD §12.4 plus `NO_COLOR`, `REDUCE_MOTION`, `NERD_FONTS`
3. `internal/config/flags.go` parses all CLI flags from PRD §12.5
4. `internal/config/loader.go` loads config with correct priority: CLI > env > git config > TOML > defaults
5. `internal/config/logging.go` sets up `log/slog` structured logging to `~/.local/state/nidhi/nidhi.log`
6. `internal/config/debug.go` implements `--debug` flag: prints startup timing breakdown and exits
7. `--trace-git` enables debug-level logging for git command tracing
8. `NO_COLOR` sets `cfg.NoColor = true`
9. `REDUCE_MOTION` sets `cfg.NoAnimation = true`
10. XDG base directory resolution via `adrg/xdg` for both config and state paths
11. TOML missing → defaults used (no error)
12. All unit tests pass: TOML load, env override, git config, CLI flags, priority resolution, XDG paths, debug timing
13. All integration tests pass: write TOML + load, set env vars + verify override, mock git config + verify
14. `make ci` passes (lint + test)

## Commit
```
feat(config): add TOML config, env vars, CLI flags, and structured logging

Implement the full configuration pipeline: TOML file (~/.config/nidhi/
config.toml) parsed with go-toml/v2, environment variables (NIDHI_*,
NO_COLOR, REDUCE_MOTION), git config (nidhi.* section), and CLI flags.
Priority: CLI > env > git config > TOML > defaults per PRD §12.1.
Add structured logging via log/slog to ~/.local/state/nidhi/nidhi.log
with --trace-git for git command tracing. Implement --debug for startup
timing breakdown. XDG paths via adrg/xdg.
```

## Session Protocol
1. Run `date`
2. Read `CLAUDE.md`
3. Read this task + PRD sections 12.1-12.6 (config), 7.5 (observability), 4.2 (libraries)
4. Verify dependencies: task 002 (config struct/defaults/gitconfig) and task 006 (core model) are DONE
5. Extend `internal/config/config.go` with all Config fields
6. Create `internal/config/toml.go` with TOML parser using go-toml/v2
7. Create `internal/config/envvars.go` with all env var mappings
8. Create `internal/config/flags.go` with CLI flag types
9. Create `internal/config/loader.go` with unified LoadConfig function
10. Create `internal/config/logging.go` with slog setup
11. Create `internal/config/debug.go` with timing breakdown
12. Create `internal/config/config_test.go` with all tests
13. Wire config loader into `cmd/nidhi/main.go`
14. Run `go test -v -count=1 ./internal/config/...`
15. Run `make ci`
16. Update this file (Status: DONE) + `docs/PROGRESS.md` + `CLAUDE.md` Learnings
17. Commit with the message above
