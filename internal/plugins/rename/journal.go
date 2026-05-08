package rename

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
)

// JournalEntry records a stash's identity for crash recovery.
type JournalEntry struct {
	Index   int    `json:"index"`
	SHA     string `json:"sha"`
	Message string `json:"message"`
}

// Journal represents a reorder operation in progress.
type Journal struct {
	Operation  string         `json:"operation"`
	Entries    []JournalEntry `json:"entries"`
	TargetIdx  int            `json:"target_idx"`
	NewMsg     string         `json:"new_msg"`
	Step       int            `json:"step"`
	TotalSteps int            `json:"total_steps"`
}

// journalPath returns the path to the reorder journal file.
func journalPath() string {
	return filepath.Join(xdg.StateHome, "nidhi", "reorder-journal.json")
}

// WriteJournal persists the journal to disk.
func WriteJournal(j *Journal) error {
	path := journalPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}

	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write journal: %w", err)
	}

	return nil
}

// ReadJournal reads an existing journal from disk.
// Returns nil if no journal exists.
func ReadJournal() (*Journal, error) {
	path := journalPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read journal: %w", err)
	}

	var j Journal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("unmarshal journal: %w", err)
	}

	return &j, nil
}

// RemoveJournal deletes the journal file after a successful operation.
func RemoveJournal() error {
	path := journalPath()
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove journal: %w", err)
	}
	return nil
}

// HasIncompleteOperation checks if there is a journal from a previous
// interrupted operation.
func HasIncompleteOperation() bool {
	j, err := ReadJournal()
	return err == nil && j != nil
}
