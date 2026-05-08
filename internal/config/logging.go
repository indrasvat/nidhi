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
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}, nil
	}

	logPath := cfg.Log.File
	if logPath == "" {
		logPath = DefaultLogPath()
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, nil, err
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}

	level := ParseLogLevel(cfg.Log.Level)
	if cfg.TraceGit {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level})
	logger := slog.New(handler)

	cleanup := func() { _ = f.Close() }
	return logger, cleanup, nil
}

// ParseLogLevel converts a string log level to slog.Level.
func ParseLogLevel(s string) slog.Level {
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
		return slog.LevelError
	}
}
