package plugin_test

import (
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/plugin"
)

type mockPlugin struct{}

func (m *mockPlugin) ID() string                        { return "mock" }
func (m *mockPlugin) Name() string                      { return "Mock Plugin" }
func (m *mockPlugin) Init(_ plugin.PluginContext) error { return nil }
func (m *mockPlugin) Destroy() error                    { return nil }

var _ plugin.Plugin = (*mockPlugin)(nil)

func TestModeString(t *testing.T) {
	tests := []struct {
		mode plugin.Mode
		want string
	}{
		{plugin.ModeList, "LIST"},
		{plugin.ModePreview, "PREVIEW"},
		{plugin.ModeDetail, "DETAIL"},
		{plugin.ModeSearch, "SEARCH"},
		{plugin.ModeNewStash, "NEW"},
		{plugin.ModeExport, "EXPORT"},
		{plugin.ModeImport, "IMPORT"},
		{plugin.ModeConflict, "CONFLICT"},
		{plugin.ModeHelp, "HELP"},
		{plugin.Mode(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestGitVersionAtLeast(t *testing.T) {
	tests := []struct {
		name  string
		ver   plugin.GitVersion
		major int
		minor int
		want  bool
	}{
		{"exact match", plugin.GitVersion{Major: 2, Minor: 38}, 2, 38, true},
		{"higher minor", plugin.GitVersion{Major: 2, Minor: 53}, 2, 38, true},
		{"lower minor", plugin.GitVersion{Major: 2, Minor: 37}, 2, 38, false},
		{"higher major", plugin.GitVersion{Major: 3, Minor: 0}, 2, 51, true},
		{"lower major", plugin.GitVersion{Major: 1, Minor: 99}, 2, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ver.AtLeast(tt.major, tt.minor); got != tt.want {
				t.Errorf("GitVersion{%d,%d}.AtLeast(%d,%d) = %v, want %v",
					tt.ver.Major, tt.ver.Minor, tt.major, tt.minor, got, tt.want)
			}
		})
	}
}

func TestStashStruct(t *testing.T) {
	s := plugin.Stash{
		Index: 0, SHA: "a3f7b2c1234567890", ShortSHA: "a3f7b2c",
		Message: "Fix auth", Branch: "main", Date: time.Now(),
		FileCount: 3, Insertions: 42, Deletions: 17,
	}
	if s.FileCount != 3 {
		t.Errorf("FileCount = %d", s.FileCount)
	}
}

func TestFilterStruct(t *testing.T) {
	f := plugin.Filter{ID: "branch", Label: "main", Value: "main"}
	if f.ID != "branch" {
		t.Errorf("Filter.ID = %s", f.ID)
	}
}

func TestKeyBindingStruct(t *testing.T) {
	kb := plugin.KeyBinding{
		Key: "a", Desc: "Apply stash",
		Modes: []plugin.Mode{plugin.ModeList, plugin.ModePreview}, Priority: 50,
	}
	if len(kb.Modes) != 2 {
		t.Errorf("Modes length = %d", len(kb.Modes))
	}
}

func TestPushOptionsStruct(t *testing.T) {
	opts := plugin.PushOptions{
		Message: "test", IncludeStaged: true, KeepIndex: true,
		Pathspecs: []string{"src/"},
	}
	if !opts.KeepIndex {
		t.Error("expected KeepIndex true")
	}
}

func TestEventStruct(t *testing.T) {
	e := plugin.Event{Type: "stashes.changed", Payload: 5}
	if e.Type != "stashes.changed" {
		t.Errorf("Type = %s", e.Type)
	}
}
