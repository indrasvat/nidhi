package config

// DefaultConfig returns a Config populated with all default values
// from PRD section 12.6.
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
		Keys: KeysConfig{},
		Performance: PerformanceConfig{
			PreloadDiffs:  10,
			SearchIndex:   "lazy",
			DiffCacheSize: 50,
		},
		Log: LogConfig{
			Level: "off",
			File:  "",
		},
	}
}
