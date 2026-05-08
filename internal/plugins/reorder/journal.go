package reorder

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
)

// JournalEntry records the state of a single stash before reorder.
type JournalEntry struct {
	Index   int    `json:"index"`
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// Journal persists the pre-reorder state for crash recovery.
type Journal struct {
	Operation   string         `json:"operation"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	SourceIndex int            `json:"source_index"`
	TargetIndex int            `json:"target_index"`
	Entries     []JournalEntry `json:"entries"`
	filePath    string
}

// DefaultJournalPath returns the default journal file path.
// Uses XDG state directory: ~/.local/state/nidhi/move-journal.json
// (distinct from rename plugin's reorder-journal.json)
func DefaultJournalPath() string {
	return filepath.Join(xdg.StateHome, "nidhi", "move-journal.json")
}

// NewJournal creates a new journal for a reorder operation.
func NewJournal(sourceIndex, targetIndex int, entries []JournalEntry) *Journal {
	return &Journal{
		Operation:   "reorder",
		StartedAt:   time.Now(),
		SourceIndex: sourceIndex,
		TargetIndex: targetIndex,
		Entries:     entries,
		filePath:    DefaultJournalPath(),
	}
}

// SetPath overrides the journal file path (for testing).
func (j *Journal) SetPath(path string) {
	j.filePath = path
}

// Write persists the journal to disk.
func (j *Journal) Write() error {
	dir := filepath.Dir(j.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}
	return os.WriteFile(j.filePath, data, 0o644)
}

// MarkComplete marks the journal as successfully completed and writes it.
func (j *Journal) MarkComplete() error {
	now := time.Now()
	j.CompletedAt = &now
	return j.Write()
}

// Remove deletes the journal file.
func (j *Journal) Remove() error {
	err := os.Remove(j.filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove journal: %w", err)
	}
	return nil
}

// LoadJournal reads an existing journal from disk.
// Returns nil, nil if no journal file exists.
func LoadJournal(path string) (*Journal, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read journal: %w", err)
	}
	var j Journal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("unmarshal journal: %w", err)
	}
	j.filePath = path
	return &j, nil
}

// IsIncomplete returns true if the journal represents an unfinished reorder.
func (j *Journal) IsIncomplete() bool {
	return j != nil && j.CompletedAt == nil
}

// HasIncompleteOperation checks if there is a journal from a previous
// interrupted operation.
func HasIncompleteOperation() bool {
	j, err := LoadJournal(DefaultJournalPath())
	return err == nil && j != nil && j.IsIncomplete()
}
