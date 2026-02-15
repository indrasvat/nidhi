package config

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const gitConfigTimeout = 5 * time.Second

// LoadFromGitConfig reads nidhi-specific values from git config.
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

func gitConfigGet(ctx context.Context, key string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "config", "--get", key)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func isValidLogLevel(level string) bool {
	switch level {
	case "off", "error", "warn", "info", "debug":
		return true
	}
	return false
}
