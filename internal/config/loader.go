package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/adrg/xdg"
	toml "github.com/pelletier/go-toml/v2"
)

// configFilePathOverride allows tests to set a custom config file path.
// When empty, the default XDG path is used.
var configFilePathOverride string

// SetConfigFilePath overrides the config file path (for testing).
func SetConfigFilePath(path string) {
	configFilePathOverride = path
}

// Load resolves configuration from all sources in priority order:
//  1. CLI flags (highest)
//  2. Environment variables (NIDHI_*)
//  3. Git config (nidhi.* section)
//  4. Config file (~/.config/nidhi/config.toml)
//  5. Built-in defaults (lowest)
func Load(flags CLIFlags) (Config, error) {
	cfg := DefaultConfig()

	if err := loadFromTOML(&cfg); err != nil {
		if !os.IsNotExist(err) {
			return cfg, err
		}
	}

	LoadFromGitConfig(&cfg)
	loadFromEnv(&cfg)
	applyFlags(&cfg, flags)

	return cfg, nil
}

// ConfigFilePath returns the path to the TOML config file.
func ConfigFilePath() string {
	if configFilePathOverride != "" {
		return configFilePathOverride
	}
	return filepath.Join(xdg.ConfigHome, "nidhi", "config.toml")
}

func loadFromTOML(cfg *Config) error {
	path := ConfigFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, cfg)
}

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

	// Standard env vars (PRD §12.4).
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		cfg.NoColor = true
	}
	if _, ok := os.LookupEnv("REDUCE_MOTION"); ok {
		cfg.NoAnimation = true
	}
	if v := os.Getenv("NERD_FONTS"); v != "" {
		switch v {
		case "1", "true":
			cfg.General.Icons = "nerd"
		case "0", "false":
			cfg.General.Icons = "ascii"
		}
	}
}

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
	if flags.Directory != nil && *flags.Directory != "" {
		cfg.Directory = *flags.Directory
	}
}
