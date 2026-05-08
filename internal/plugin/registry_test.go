package plugin_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/indrasvat/nidhi/internal/plugin"
)

type stubPlugin struct {
	id   string
	name string
}

func (s *stubPlugin) ID() string                        { return s.id }
func (s *stubPlugin) Name() string                      { return s.name }
func (s *stubPlugin) Init(_ plugin.PluginContext) error { return nil }
func (s *stubPlugin) Destroy() error                    { return nil }

var _ plugin.Plugin = (*stubPlugin)(nil)

type stubKeyHandler struct {
	stubPlugin
	bindings []plugin.KeyBinding
}

func (s *stubKeyHandler) KeyBindings() []plugin.KeyBinding { return s.bindings }
func (s *stubKeyHandler) HandleKey(_ plugin.KeyEvent, state plugin.AppState) (plugin.AppState, tea.Cmd) {
	return state, nil
}

var _ plugin.KeyHandler = (*stubKeyHandler)(nil)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	p := &stubPlugin{id: "test", name: "Test"}
	if err := reg.Register(p, 10); err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	got, ok := reg.Get("test")
	if !ok {
		t.Fatal("Get() returned false")
	}
	if got.ID() != "test" {
		t.Errorf("ID = %q", got.ID())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	reg.Register(&stubPlugin{id: "dup"}, 10)
	if err := reg.Register(&stubPlugin{id: "dup"}, 10); err == nil {
		t.Fatal("duplicate should error")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Error("should return false")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	reg.Register(&stubPlugin{id: "removeme"}, 10)
	if err := reg.Unregister("removeme"); err != nil {
		t.Fatalf("Unregister() error: %v", err)
	}
	if reg.Len() != 0 {
		t.Errorf("Len = %d", reg.Len())
	}
}

func TestRegistry_UnregisterNotFound(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	if err := reg.Unregister("ghost"); err == nil {
		t.Error("should error")
	}
}

func TestRegistry_PriorityOrdering(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	reg.Register(&stubPlugin{id: "mid"}, 50)
	reg.Register(&stubPlugin{id: "low"}, 10)
	reg.Register(&stubPlugin{id: "high"}, 100)

	all := reg.All()
	want := []string{"high", "mid", "low"}
	for i, w := range want {
		if all[i].ID() != w {
			t.Errorf("All()[%d].ID() = %q, want %q", i, all[i].ID(), w)
		}
	}
}

func TestRegistry_Len(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	if reg.Len() != 0 {
		t.Errorf("Len = %d", reg.Len())
	}
	reg.Register(&stubPlugin{id: "a"}, 10)
	reg.Register(&stubPlugin{id: "b"}, 20)
	if reg.Len() != 2 {
		t.Errorf("Len = %d", reg.Len())
	}
}

func TestRegistry_AllReturnsDefensiveCopy(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	reg.Register(&stubPlugin{id: "x"}, 10)
	all := reg.All()
	all[0] = &stubPlugin{id: "hacked"}
	got, ok := reg.Get("x")
	if !ok || got.ID() != "x" {
		t.Error("mutating All() should not affect registry")
	}
}

func TestRegistry_UnregisterPreservesOrder(t *testing.T) {
	reg := plugin.NewRegistry[plugin.Plugin]()
	reg.Register(&stubPlugin{id: "a"}, 30)
	reg.Register(&stubPlugin{id: "b"}, 20)
	reg.Register(&stubPlugin{id: "c"}, 10)
	reg.Unregister("b")
	all := reg.All()
	if len(all) != 2 || all[0].ID() != "a" || all[1].ID() != "c" {
		t.Error("order wrong after unregister")
	}
}

func TestDetectKeyCollisions_NoCollision(t *testing.T) {
	reg := plugin.NewRegistry[plugin.KeyHandler]()
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p1"},
		bindings:   []plugin.KeyBinding{{Key: "a", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p2"},
		bindings:   []plugin.KeyBinding{{Key: "b", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)
	if collisions := plugin.DetectKeyCollisions(reg); len(collisions) != 0 {
		t.Errorf("expected 0 collisions, got %d", len(collisions))
	}
}

func TestDetectKeyCollisions_SameKeySameMode(t *testing.T) {
	reg := plugin.NewRegistry[plugin.KeyHandler]()
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "core"},
		bindings:   []plugin.KeyBinding{{Key: "a", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 50)
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "custom"},
		bindings:   []plugin.KeyBinding{{Key: "a", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)
	collisions := plugin.DetectKeyCollisions(reg)
	if len(collisions) != 1 {
		t.Fatalf("expected 1 collision, got %d", len(collisions))
	}
	if collisions[0].PluginA != "core" {
		t.Errorf("PluginA = %q, want core", collisions[0].PluginA)
	}
}

func TestDetectKeyCollisions_SameKeyDifferentMode(t *testing.T) {
	reg := plugin.NewRegistry[plugin.KeyHandler]()
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p1"},
		bindings:   []plugin.KeyBinding{{Key: "j", Modes: []plugin.Mode{plugin.ModeList}}},
	}, 10)
	reg.Register(&stubKeyHandler{
		stubPlugin: stubPlugin{id: "p2"},
		bindings:   []plugin.KeyBinding{{Key: "j", Modes: []plugin.Mode{plugin.ModeDetail}}},
	}, 10)
	if collisions := plugin.DetectKeyCollisions(reg); len(collisions) != 0 {
		t.Errorf("different modes should not collide, got %d", len(collisions))
	}
}
