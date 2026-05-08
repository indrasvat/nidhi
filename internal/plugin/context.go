package plugin

import (
	"errors"
	"log/slog"
)

// PluginContext provides plugins with access to core services.
type PluginContext struct {
	Git    GitRunner
	Cache  StashCache
	Config ConfigStore
	Events EventBus
	Logger *slog.Logger
	GitVer GitVersion
	Theme  Theme
}

// NewPluginContext creates a PluginContext, validating required services.
func NewPluginContext(
	git GitRunner,
	cache StashCache,
	config ConfigStore,
	events EventBus,
	logger *slog.Logger,
	gitVer GitVersion,
	theme Theme,
) (PluginContext, error) {
	if git == nil {
		return PluginContext{}, errors.New("plugin context: GitRunner is required")
	}
	if cache == nil {
		return PluginContext{}, errors.New("plugin context: StashCache is required")
	}
	if config == nil {
		return PluginContext{}, errors.New("plugin context: ConfigStore is required")
	}
	if events == nil {
		return PluginContext{}, errors.New("plugin context: EventBus is required")
	}
	if logger == nil {
		return PluginContext{}, errors.New("plugin context: Logger is required")
	}
	if theme == nil {
		return PluginContext{}, errors.New("plugin context: Theme is required")
	}
	return PluginContext{
		Git:    git,
		Cache:  cache,
		Config: config,
		Events: events,
		Logger: logger,
		GitVer: gitVer,
		Theme:  theme,
	}, nil
}
