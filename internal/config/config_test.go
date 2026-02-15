package config_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/config"
)

func clearNidhiEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"NIDHI_STALE_DAYS", "NIDHI_ICONS", "NIDHI_LOG_LEVEL",
		"NIDHI_EXPORT_REF", "NIDHI_THEME",
	} {
		t.Setenv(key, "")
	}
}

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
	wantHours := 14 * 24
	if int(threshold.Hours()) != wantHours {
		t.Errorf("StaleThreshold() = %v hours, want %v hours", threshold.Hours(), wantHours)
	}
}

func TestLoad_TOMLFile(t *testing.T) {
	tmpDir := t.TempDir()

	tomlContent := `
[general]
icons = "nerd"
stale_days = 7
keep_index = false

[export]
ref = "refs/stashes/testuser"
remote = "upstream"

[performance]
preload_diffs = 5
search_index = "eager"
diff_cache_size = 100

[log]
level = "debug"
file = "/tmp/nidhi.log"
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	config.SetConfigFilePath(configPath)
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.General.Icons != "nerd" {
		t.Errorf("Icons = %q, want %q", cfg.General.Icons, "nerd")
	}
	if cfg.General.StaleDays != 7 {
		t.Errorf("StaleDays = %d, want 7", cfg.General.StaleDays)
	}
	if cfg.Performance.SearchIndex != "eager" {
		t.Errorf("SearchIndex = %q, want %q", cfg.Performance.SearchIndex, "eager")
	}
}

func TestLoad_EnvVars(t *testing.T) {
	// Point to a nonexistent config file so TOML loading is skipped.
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })

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
}

func TestLoad_CLIFlags_OverrideAll(t *testing.T) {
	tmpDir := t.TempDir()

	tomlContent := `
[general]
icons = "nerd"
[log]
level = "error"
`
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	config.SetConfigFilePath(configPath)
	t.Cleanup(func() { config.SetConfigFilePath("") })

	t.Setenv("NIDHI_ICONS", "ascii")
	t.Setenv("NIDHI_LOG_LEVEL", "warn")

	logLevel := "debug"
	icons := "auto"
	flags := config.CLIFlags{LogLevel: &logLevel, Icons: &icons}

	cfg, err := config.Load(flags)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("LogLevel = %q, want %q (CLI should override)", cfg.Log.Level, "debug")
	}
	if cfg.General.Icons != "auto" {
		t.Errorf("Icons = %q, want %q (CLI should override)", cfg.General.Icons, "auto")
	}
}

func TestLoad_NoConfigFile(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() should not error when config file is missing: %v", err)
	}

	if cfg.General.Icons != "auto" {
		t.Errorf("Icons = %q, want default %q", cfg.General.Icons, "auto")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("not valid [[["), 0o644); err != nil {
		t.Fatal(err)
	}

	config.SetConfigFilePath(configPath)
	t.Cleanup(func() { config.SetConfigFilePath("") })

	_, err := config.Load(config.CLIFlags{})
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}

func TestLoadFromGitConfig(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	dir := t.TempDir()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@test.com")
	runCmd(t, dir, "git", "config", "user.name", "Test")
	runCmd(t, dir, "git", "config", "nidhi.stale-days", "21")
	runCmd(t, dir, "git", "config", "nidhi.icons", "ascii")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	cfg := config.DefaultConfig()
	config.LoadFromGitConfig(&cfg)

	if cfg.General.StaleDays != 21 {
		t.Errorf("StaleDays = %d, want 21", cfg.General.StaleDays)
	}
	if cfg.General.Icons != "ascii" {
		t.Errorf("Icons = %q, want %q", cfg.General.Icons, "ascii")
	}
}

func runCmd(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}
