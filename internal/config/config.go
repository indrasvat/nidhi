package config

import "time"

// Config holds all nidhi configuration.
type Config struct {
	General     GeneralConfig     `toml:"general"`
	Export      ExportConfig      `toml:"export"`
	Theme       ThemeConfig       `toml:"theme"`
	Keys        KeysConfig        `toml:"keys"`
	Performance PerformanceConfig `toml:"performance"`
	Log         LogConfig         `toml:"log"`

	// CLI-only flags (not persisted in config file).
	Debug       bool   `toml:"-"` // --debug: print timing and exit.
	TraceGit    bool   `toml:"-"` // --trace-git: log all git commands.
	NoColor     bool   `toml:"-"` // --no-color / NO_COLOR: disable colors.
	NoAnimation bool   `toml:"-"` // --no-animation / REDUCE_MOTION: disable animations.
	Directory   string `toml:"-"` // -C: run as if started in <path>.
}

type GeneralConfig struct {
	Icons       string `toml:"icons"`
	StaleDays   int    `toml:"stale_days"`
	KeepIndex   bool   `toml:"keep_index"`
	AutoMessage bool   `toml:"auto_message"`
}

type ExportConfig struct {
	Ref    string `toml:"ref"`
	Remote string `toml:"remote"`
}

type ThemeConfig struct {
	Name string `toml:"name"`
}

type KeysConfig struct {
	Apply string `toml:"apply,omitempty"`
	Pop   string `toml:"pop,omitempty"`
	Drop  string `toml:"drop,omitempty"`
}

type PerformanceConfig struct {
	PreloadDiffs  int    `toml:"preload_diffs"`
	SearchIndex   string `toml:"search_index"`
	DiffCacheSize int    `toml:"diff_cache_size"`
}

type LogConfig struct {
	Level string `toml:"level"`
	File  string `toml:"file"`
}

// CLIFlags holds values parsed from command-line flags.
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
