package components

import (
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/nidhi/internal/ui/theme"
)

func TestToastClass_Duration(t *testing.T) {
	tests := []struct {
		class ToastClass
		want  time.Duration
	}{
		{ToastInfo, 5 * time.Second},
		{ToastError, 5 * time.Second},
		{ToastUndo, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.class.String(), func(t *testing.T) {
			if got := tt.class.Duration(); got != tt.want {
				t.Errorf("Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToast_IsExpired(t *testing.T) {
	toast := Toast{
		CreatedAt: time.Now(),
		Duration:  5 * time.Second,
	}
	if toast.IsExpired() {
		t.Error("fresh toast should not be expired")
	}

	toast = Toast{
		CreatedAt: time.Now().Add(-10 * time.Second),
		Duration:  5 * time.Second,
	}
	if !toast.IsExpired() {
		t.Error("old toast should be expired")
	}
}

func TestToast_RemainingSeconds(t *testing.T) {
	toast := Toast{
		CreatedAt: time.Now().Add(-10 * time.Second),
		Duration:  30 * time.Second,
	}
	remaining := toast.RemainingSeconds()
	if remaining < 19 || remaining > 21 {
		t.Errorf("RemainingSeconds() = %d, want ~20", remaining)
	}
}

func TestToast_RemainingSecondsExpired(t *testing.T) {
	toast := Toast{
		CreatedAt: time.Now().Add(-60 * time.Second),
		Duration:  5 * time.Second,
	}
	if toast.RemainingSeconds() != 0 {
		t.Errorf("expired toast RemainingSeconds() = %d, want 0", toast.RemainingSeconds())
	}
}

func TestToastModel_ShowAndDismiss(t *testing.T) {
	tm := NewToastModel(theme.NewAgni())

	if tm.IsVisible() {
		t.Error("new ToastModel should not be visible")
	}

	tm.Show("Test message", ToastInfo)

	if !tm.IsVisible() {
		t.Error("toast should be visible after Show()")
	}

	active := tm.Active()
	if active == nil {
		t.Fatal("Active() should not be nil")
	}
	if active.Message != "Test message" {
		t.Errorf("Message = %q, want %q", active.Message, "Test message")
	}
	if active.Class != ToastInfo {
		t.Errorf("Class = %v, want ToastInfo", active.Class)
	}

	tm.Dismiss()

	if tm.IsVisible() {
		t.Error("toast should not be visible after Dismiss()")
	}
}

func TestToastModel_ShowUndo(t *testing.T) {
	tm := NewToastModel(theme.NewAgni())
	tm.ShowUndo("Dropped stash@{0}", "z")

	active := tm.Active()
	if active == nil {
		t.Fatal("Active() should not be nil")
	}
	if active.Class != ToastUndo {
		t.Errorf("Class = %v, want ToastUndo", active.Class)
	}
	if active.RecoveryKey != "z" {
		t.Errorf("RecoveryKey = %q, want %q", active.RecoveryKey, "z")
	}
}

func TestToastModel_ShowReplacesExisting(t *testing.T) {
	tm := NewToastModel(theme.NewAgni())
	tm.Show("First", ToastInfo)
	tm.Show("Second", ToastError)

	active := tm.Active()
	if active == nil {
		t.Fatal("Active() should not be nil")
	}
	if active.Message != "Second" {
		t.Errorf("show should replace existing: Message = %q, want %q", active.Message, "Second")
	}
}

func TestToastModel_ViewEmpty(t *testing.T) {
	tm := NewToastModel(theme.NewAgni())
	tm.SetWidth(80)

	view := tm.View()
	if view != "" {
		t.Errorf("View() with no toast should be empty, got: %q", view)
	}
}

func TestToastModel_ViewRendersContent(t *testing.T) {
	tm := NewToastModel(theme.NewAgni())
	tm.SetWidth(80)
	tm.Show("Operation succeeded", ToastInfo)

	view := tm.View()
	if view == "" {
		t.Error("View() should not be empty when toast is active")
	}

	plain := stripAnsi(view)
	if !strings.Contains(plain, "Operation succeeded") {
		t.Errorf("View should contain toast message, got: %q", plain)
	}
}

func TestToastModel_UndoViewShowsRecoveryInfo(t *testing.T) {
	tm := NewToastModel(theme.NewAgni())
	tm.SetWidth(120)
	tm.ShowUndo("Dropped stash@{0}", "z")

	view := tm.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "undo") {
		t.Errorf("undo toast should contain 'undo', got: %q", plain)
	}
	if !strings.Contains(plain, "z") {
		t.Errorf("undo toast should contain recovery key 'z', got: %q", plain)
	}
}

func TestToastModel_InfoViewShowsCheckmark(t *testing.T) {
	tm := NewToastModel(theme.NewAgni())
	tm.SetWidth(80)
	tm.Show("Applied stash@{0}", ToastInfo)

	view := tm.View()
	plain := stripAnsi(view)

	if !strings.Contains(plain, "✓") {
		t.Errorf("info toast should contain ✓ icon, got: %q", plain)
	}
}

func TestToastClass_String(t *testing.T) {
	tests := []struct {
		class ToastClass
		want  string
	}{
		{ToastInfo, "info"},
		{ToastError, "error"},
		{ToastUndo, "undo"},
	}

	for _, tt := range tests {
		if got := tt.class.String(); got != tt.want {
			t.Errorf("ToastClass(%d).String() = %q, want %q", tt.class, got, tt.want)
		}
	}
}
