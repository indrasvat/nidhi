package undo

import (
	"testing"
	"time"
)

func TestRingBuffer_PushPop(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Push(UndoEntry{SHA: "aaa", Message: "first"})
	rb.Push(UndoEntry{SHA: "bbb", Message: "second"})
	rb.Push(UndoEntry{SHA: "ccc", Message: "third"})

	if rb.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", rb.Len())
	}

	// Pop should return LIFO order.
	tests := []struct {
		wantSHA string
		wantOK  bool
	}{
		{"ccc", true},
		{"bbb", true},
		{"aaa", true},
	}

	for i, tt := range tests {
		entry, ok := rb.Pop()
		if ok != tt.wantOK {
			t.Fatalf("Pop() #%d: ok = %v, want %v", i, ok, tt.wantOK)
		}
		if entry.SHA != tt.wantSHA {
			t.Fatalf("Pop() #%d: SHA = %q, want %q", i, entry.SHA, tt.wantSHA)
		}
	}

	// Empty buffer.
	_, ok := rb.Pop()
	if ok {
		t.Fatal("Pop() on empty buffer should return false")
	}
}

func TestRingBuffer_Wrapping(t *testing.T) {
	rb := NewRingBuffer(3)

	// Push 5 entries (wraps at capacity 3).
	rb.Push(UndoEntry{SHA: "a1"})
	rb.Push(UndoEntry{SHA: "a2"})
	rb.Push(UndoEntry{SHA: "a3"})
	rb.Push(UndoEntry{SHA: "a4"}) // overwrites a1
	rb.Push(UndoEntry{SHA: "a5"}) // overwrites a2

	if rb.Len() != 3 {
		t.Fatalf("Len() = %d, want 3 (capped)", rb.Len())
	}

	// Should get a5, a4, a3 (LIFO, oldest overwritten).
	entry, ok := rb.Pop()
	if !ok || entry.SHA != "a5" {
		t.Fatalf("Pop() = %q, want a5", entry.SHA)
	}

	entry, ok = rb.Pop()
	if !ok || entry.SHA != "a4" {
		t.Fatalf("Pop() = %q, want a4", entry.SHA)
	}

	entry, ok = rb.Pop()
	if !ok || entry.SHA != "a3" {
		t.Fatalf("Pop() = %q, want a3", entry.SHA)
	}
}

func TestRingBuffer_WrapAt50(t *testing.T) {
	rb := NewRingBuffer(50) // PRD FR-14.2 specifies 50 entries

	for i := range 50 {
		rb.Push(UndoEntry{SHA: "sha-" + string(rune('A'+i%26))})
	}

	if rb.Len() != 50 {
		t.Fatalf("Len() = %d, want 50", rb.Len())
	}

	// Push one more — should overwrite oldest.
	rb.Push(UndoEntry{SHA: "overflow"})
	if rb.Len() != 50 {
		t.Fatalf("Len() after overflow = %d, want 50", rb.Len())
	}

	entry, ok := rb.Peek()
	if !ok || entry.SHA != "overflow" {
		t.Fatalf("Peek() = %q, want overflow", entry.SHA)
	}
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Push(UndoEntry{SHA: "aaa"})
	rb.Push(UndoEntry{SHA: "bbb"})

	rb.Clear()

	if rb.Len() != 0 {
		t.Fatalf("Len() after Clear = %d, want 0", rb.Len())
	}

	_, ok := rb.Pop()
	if ok {
		t.Fatal("Pop() after Clear should return false")
	}
}

func TestRingBuffer_Peek(t *testing.T) {
	rb := NewRingBuffer(3)

	_, ok := rb.Peek()
	if ok {
		t.Fatal("Peek() on empty buffer should return false")
	}

	rb.Push(UndoEntry{SHA: "peek-me"})
	entry, ok := rb.Peek()
	if !ok || entry.SHA != "peek-me" {
		t.Fatalf("Peek() = %q, want peek-me", entry.SHA)
	}

	// Peek should not remove.
	if rb.Len() != 1 {
		t.Fatalf("Len() after Peek = %d, want 1", rb.Len())
	}
}

func TestUndoEntry_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		droppedAt time.Time
		ttl       time.Duration
		want      bool
	}{
		{
			name:      "fresh entry within TTL",
			droppedAt: time.Now().Add(-10 * time.Second),
			ttl:       30 * time.Second,
			want:      false,
		},
		{
			name:      "expired entry beyond TTL",
			droppedAt: time.Now().Add(-60 * time.Second),
			ttl:       30 * time.Second,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := UndoEntry{DroppedAt: tt.droppedAt}
			got := entry.IsExpired(tt.ttl)
			if got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}
