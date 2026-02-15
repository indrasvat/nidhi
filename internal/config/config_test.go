package config_test

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/config"
)

func clearNidhiEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"NIDHI_STALE_DAYS", "NIDHI_ICONS", "NIDHI_LOG_LEVEL",
		"NIDHI_EXPORT_REF", "NIDHI_THEME",
		"NO_COLOR", "REDUCE_MOTION", "NERD_FONTS",
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

// --- Standard env var tests ---

func TestLoad_NOCOLORSetsNoColor(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)
	t.Setenv("NO_COLOR", "1")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.NoColor {
		t.Error("expected NoColor=true when NO_COLOR is set")
	}
}

func TestLoad_NOCOLOREmptyValueStillSets(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)
	// NO_COLOR spec says presence alone is sufficient.
	t.Setenv("NO_COLOR", "")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.NoColor {
		t.Error("expected NoColor=true when NO_COLOR is present (even empty)")
	}
}

func TestLoad_REDUCEMOTIONSetsNoAnimation(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)
	t.Setenv("REDUCE_MOTION", "1")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.NoAnimation {
		t.Error("expected NoAnimation=true when REDUCE_MOTION is set")
	}
}

func TestLoad_NERDFONTSTrue(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)
	t.Setenv("NERD_FONTS", "1")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.General.Icons != "nerd" {
		t.Errorf("Icons = %q, want %q when NERD_FONTS=1", cfg.General.Icons, "nerd")
	}
}

func TestLoad_NERDFONTSFalse(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)
	t.Setenv("NERD_FONTS", "0")

	cfg, err := config.Load(config.CLIFlags{})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.General.Icons != "ascii" {
		t.Errorf("Icons = %q, want %q when NERD_FONTS=0", cfg.General.Icons, "ascii")
	}
}

// --- CLI flags for new fields ---

func TestLoad_CLIFlags_TraceGit(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	traceGit := true
	cfg, err := config.Load(config.CLIFlags{TraceGit: &traceGit})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.TraceGit {
		t.Error("expected TraceGit=true from CLI flag")
	}
}

func TestLoad_CLIFlags_Debug(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	debug := true
	cfg, err := config.Load(config.CLIFlags{Debug: &debug})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.Debug {
		t.Error("expected Debug=true from CLI flag")
	}
}

func TestLoad_CLIFlags_NoColor(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	noColor := true
	cfg, err := config.Load(config.CLIFlags{NoColor: &noColor})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.NoColor {
		t.Error("expected NoColor=true from CLI flag")
	}
}

func TestLoad_CLIFlags_NoAnimation(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	noAnim := true
	cfg, err := config.Load(config.CLIFlags{NoAnimation: &noAnim})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.NoAnimation {
		t.Error("expected NoAnimation=true from CLI flag")
	}
}

func TestLoad_CLIFlags_Directory(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	dir := "/some/path"
	cfg, err := config.Load(config.CLIFlags{Directory: &dir})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Directory != "/some/path" {
		t.Errorf("Directory = %q, want %q", cfg.Directory, "/some/path")
	}
}

// --- Default CLI-only fields ---

func TestDefaultConfig_CLIOnlyFieldsAreZero(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.Debug {
		t.Error("expected Debug=false by default")
	}
	if cfg.TraceGit {
		t.Error("expected TraceGit=false by default")
	}
	if cfg.NoColor {
		t.Error("expected NoColor=false by default")
	}
	if cfg.NoAnimation {
		t.Error("expected NoAnimation=false by default")
	}
	if cfg.Directory != "" {
		t.Errorf("expected empty Directory by default, got %q", cfg.Directory)
	}
}

// --- Logging tests ---

func TestSetupLogging_OffLevel(t *testing.T) {
	cfg := config.DefaultConfig() // Level="off", TraceGit=false
	logger, cleanup, err := config.SetupLogging(&cfg)
	if err != nil {
		t.Fatalf("SetupLogging() error: %v", err)
	}
	defer cleanup()
	if logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestSetupLogging_DebugLevel(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.DefaultConfig()
	cfg.Log.Level = "debug"
	cfg.Log.File = logFile

	logger, cleanup, err := config.SetupLogging(&cfg)
	if err != nil {
		t.Fatalf("SetupLogging() error: %v", err)
	}
	defer cleanup()

	logger.Debug("test message", "key", "value")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(data), "test message") {
		t.Error("expected log file to contain 'test message'")
	}
}

func TestSetupLogging_TraceGitForcesDebug(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	cfg := config.DefaultConfig()
	cfg.Log.Level = "off"
	cfg.TraceGit = true
	cfg.Log.File = logFile

	logger, cleanup, err := config.SetupLogging(&cfg)
	if err != nil {
		t.Fatalf("SetupLogging() error: %v", err)
	}
	defer cleanup()

	logger.Debug("trace debug msg")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if !strings.Contains(string(data), "trace debug msg") {
		t.Error("trace-git should enable debug level")
	}
}

func TestSetupLogging_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "subdir", "nested", "test.log")

	cfg := config.DefaultConfig()
	cfg.Log.Level = "info"
	cfg.Log.File = logFile

	_, cleanup, err := config.SetupLogging(&cfg)
	if err != nil {
		t.Fatalf("SetupLogging() error: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(filepath.Dir(logFile)); os.IsNotExist(err) {
		t.Error("expected log directory to be created")
	}
}

func TestDefaultLogPath(t *testing.T) {
	path := config.DefaultLogPath()
	if path == "" {
		t.Error("DefaultLogPath returned empty string")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	if !strings.Contains(path, "nidhi") {
		t.Errorf("expected 'nidhi' in log path, got %q", path)
	}
}

// --- Debug timing tests ---

func TestDebugTiming_Record(t *testing.T) {
	dt := config.NewDebugTiming()
	dt.Record("step1", 5*time.Millisecond)
	dt.Record("step2", 10*time.Millisecond)

	entries := dt.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Name != "step1" {
		t.Errorf("entry[0].Name = %q, want %q", entries[0].Name, "step1")
	}
	if entries[1].Duration != 10*time.Millisecond {
		t.Errorf("entry[1].Duration = %v, want 10ms", entries[1].Duration)
	}
}

func TestDebugTiming_Since(t *testing.T) {
	dt := config.NewDebugTiming()
	start := time.Now()
	time.Sleep(time.Millisecond)
	dt.Since("test step", start)

	entries := dt.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Duration < time.Millisecond {
		t.Errorf("expected duration >= 1ms, got %v", entries[0].Duration)
	}
}

func TestDebugTiming_EntriesRetursCopy(t *testing.T) {
	dt := config.NewDebugTiming()
	dt.Record("step1", time.Millisecond)

	entries := dt.Entries()
	entries[0].Name = "modified"

	// Original should be unaffected.
	orig := dt.Entries()
	if orig[0].Name != "step1" {
		t.Error("Entries() should return a copy, not a reference")
	}
}

// --- ParseLogLevel tests ---

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"off", slog.LevelError},
		{"unknown", slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := config.ParseLogLevel(tt.input)
			if got != tt.want {
				t.Errorf("ParseLogLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- Priority resolution with new env vars ---

func TestLoad_CLIFlagsOverrideNOCOLOR(t *testing.T) {
	config.SetConfigFilePath(filepath.Join(t.TempDir(), "nonexistent.toml"))
	t.Cleanup(func() { config.SetConfigFilePath("") })
	clearNidhiEnv(t)

	// NO_COLOR sets NoColor=true via env.
	t.Setenv("NO_COLOR", "1")
	// But CLI flag should override.
	noColor := false
	cfg, err := config.Load(config.CLIFlags{NoColor: &noColor})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.NoColor {
		t.Error("CLI --no-color=false should override NO_COLOR env")
	}
}
