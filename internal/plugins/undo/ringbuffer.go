package undo

import (
	"sync"
	"time"
)

// UndoEntry records a dropped stash for potential recovery.
type UndoEntry struct {
	SHA       string    // Full commit SHA of the dropped stash
	Message   string    // Stash message at time of drop
	Index     int       // Stash index at time of drop
	DroppedAt time.Time // When the drop occurred
}

// IsExpired returns true if the entry is older than the given duration.
func (e UndoEntry) IsExpired(ttl time.Duration) bool {
	return time.Since(e.DroppedAt) > ttl
}

// RingBuffer is a fixed-capacity LIFO ring buffer for undo entries.
// It is goroutine-safe and session-scoped (not persisted).
type RingBuffer struct {
	mu      sync.Mutex
	entries []UndoEntry
	cap     int
	head    int // Next write position
	count   int // Current number of entries
}

// NewRingBuffer creates a ring buffer with the given capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		entries: make([]UndoEntry, capacity),
		cap:     capacity,
	}
}

// Push adds an entry to the buffer. If the buffer is full, the oldest
// entry is overwritten.
func (rb *RingBuffer) Push(entry UndoEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.cap
	if rb.count < rb.cap {
		rb.count++
	}
}

// Pop removes and returns the most recently pushed entry.
// Returns the entry and true, or a zero entry and false if empty.
func (rb *RingBuffer) Pop() (UndoEntry, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return UndoEntry{}, false
	}

	idx := (rb.head - 1 + rb.cap) % rb.cap
	entry := rb.entries[idx]
	rb.entries[idx] = UndoEntry{} // Clear for GC
	rb.head = idx
	rb.count--

	return entry, true
}

// Peek returns the most recently pushed entry without removing it.
func (rb *RingBuffer) Peek() (UndoEntry, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.count == 0 {
		return UndoEntry{}, false
	}

	idx := (rb.head - 1 + rb.cap) % rb.cap
	return rb.entries[idx], true
}

// Len returns the current number of entries.
func (rb *RingBuffer) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

// Clear removes all entries.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	for i := range rb.entries {
		rb.entries[i] = UndoEntry{}
	}
	rb.head = 0
	rb.count = 0
}
