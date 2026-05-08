package plugin

import (
	"fmt"
	"slices"
	"sync"
)

// RegistryEntry holds a plugin and its registration priority.
type RegistryEntry[T Plugin] struct {
	Plugin   T
	Priority int
}

// Registry is a generic, thread-safe plugin registry ordered by priority.
type Registry[T Plugin] struct {
	mu      sync.RWMutex
	entries []RegistryEntry[T]
	byID    map[string]int
}

// NewRegistry creates an empty plugin registry.
func NewRegistry[T Plugin]() *Registry[T] {
	return &Registry[T]{
		entries: make([]RegistryEntry[T], 0, 16),
		byID:    make(map[string]int),
	}
}

// Register adds a plugin with a given priority. Returns error if ID already registered.
func (r *Registry[T]) Register(plugin T, priority int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := plugin.ID()
	if _, exists := r.byID[id]; exists {
		return fmt.Errorf("plugin already registered: %s", id)
	}

	entry := RegistryEntry[T]{Plugin: plugin, Priority: priority}
	r.entries = append(r.entries, entry)

	slices.SortStableFunc(r.entries, func(a, b RegistryEntry[T]) int {
		return b.Priority - a.Priority
	})

	r.rebuildIndex()
	return nil
}

// Unregister removes a plugin by ID.
func (r *Registry[T]) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	idx, exists := r.byID[id]
	if !exists {
		return fmt.Errorf("plugin not found: %s", id)
	}

	r.entries = slices.Delete(r.entries, idx, idx+1)
	r.rebuildIndex()
	return nil
}

// Get returns a plugin by ID.
func (r *Registry[T]) Get(id string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	idx, exists := r.byID[id]
	if !exists {
		var zero T
		return zero, false
	}
	return r.entries[idx].Plugin, true
}

// All returns all registered plugins in priority order (highest first).
func (r *Registry[T]) All() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]T, len(r.entries))
	for i, e := range r.entries {
		result[i] = e.Plugin
	}
	return result
}

// Len returns the number of registered plugins.
func (r *Registry[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

func (r *Registry[T]) rebuildIndex() {
	r.byID = make(map[string]int, len(r.entries))
	for i, e := range r.entries {
		r.byID[e.Plugin.ID()] = i
	}
}

// ─── Keybinding Collision Detection ─────────────────────────

// KeyCollision describes a keybinding conflict between two plugins.
type KeyCollision struct {
	Key       string
	Mode      Mode
	PluginA   string
	PluginB   string
	PriorityA int
	PriorityB int
}

// DetectKeyCollisions checks a KeyHandler registry for keybinding conflicts.
func DetectKeyCollisions(reg *Registry[KeyHandler]) []KeyCollision {
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	type claim struct {
		pluginID string
		priority int
	}
	seen := make(map[string]claim)
	var collisions []KeyCollision

	for _, entry := range reg.entries {
		for _, kb := range entry.Plugin.KeyBindings() {
			modes := kb.Modes
			if len(modes) == 0 {
				modes = []Mode{Mode(-1)}
			}
			for _, mode := range modes {
				mapKey := fmt.Sprintf("%s:%d", kb.Key, mode)
				if prev, exists := seen[mapKey]; exists {
					collisions = append(collisions, KeyCollision{
						Key:       kb.Key,
						Mode:      mode,
						PluginA:   prev.pluginID,
						PluginB:   entry.Plugin.ID(),
						PriorityA: prev.priority,
						PriorityB: entry.Priority,
					})
				} else {
					seen[mapKey] = claim{
						pluginID: entry.Plugin.ID(),
						priority: entry.Priority,
					}
				}
			}
		}
	}

	return collisions
}
