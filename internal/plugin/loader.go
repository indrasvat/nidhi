package plugin

import "log/slog"

// BuiltinPluginIDs lists all built-in plugin identifiers.
var BuiltinPluginIDs = []string{
	"conflict",
	"search",
	"sync",
	"rename",
	"undo",
	"stale",
	"reorder",
	"filter",
}

// LoadBuiltins registers all built-in plugins into the provided registries.
// As plugins are implemented, they replace these stubs.
func LoadBuiltins(
	_ *Registry[KeyHandler],
	_ *Registry[ScreenProvider],
	_ *Registry[StashHook],
	_ PluginContext,
	logger *slog.Logger,
) (int, []error) {
	logger.Info("plugin loader: built-in plugin registration is stubbed")
	return 0, nil
}
