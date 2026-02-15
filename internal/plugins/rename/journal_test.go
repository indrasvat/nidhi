package rename

import (
	"testing"
)

func TestJournal_WriteReadCleanup(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	journal := &Journal{
		Operation: "rename",
		Entries: []JournalEntry{
			{Index: 0, SHA: "sha-aaa", Message: "newest stash"},
			{Index: 1, SHA: "sha-bbb", Message: "target stash"},
			{Index: 2, SHA: "sha-ccc", Message: "oldest stash"},
		},
		TargetIdx:  1,
		NewMsg:     "renamed target",
		Step:       0,
		TotalSteps: 4,
	}

	if err := WriteJournal(journal); err != nil {
		t.Fatalf("WriteJournal: %v", err)
	}

	got, err := ReadJournal()
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if got == nil {
		t.Fatal("ReadJournal returned nil")
	}

	if got.Operation != "rename" {
		t.Errorf("Operation = %q, want rename", got.Operation)
	}
	if len(got.Entries) != 3 {
		t.Errorf("Entries = %d, want 3", len(got.Entries))
	}
	if got.TargetIdx != 1 {
		t.Errorf("TargetIdx = %d, want 1", got.TargetIdx)
	}
	if got.NewMsg != "renamed target" {
		t.Errorf("NewMsg = %q, want 'renamed target'", got.NewMsg)
	}

	if err := RemoveJournal(); err != nil {
		t.Fatalf("RemoveJournal: %v", err)
	}

	j, err := ReadJournal()
	if err != nil {
		t.Fatalf("ReadJournal after remove: %v", err)
	}
	if j != nil {
		t.Error("expected nil journal after remove")
	}
}

func TestJournal_ReadNonExistent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	j, err := ReadJournal()
	if err != nil {
		t.Fatalf("ReadJournal: %v", err)
	}
	if j != nil {
		t.Error("expected nil for non-existent journal")
	}
}

func TestHasIncompleteOperation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if HasIncompleteOperation() {
		t.Error("expected false with no journal")
	}

	journal := &Journal{
		Operation: "rename",
		Entries:   []JournalEntry{{Index: 0, SHA: "abc", Message: "test"}},
	}
	if err := WriteJournal(journal); err != nil {
		t.Fatal(err)
	}

	if !HasIncompleteOperation() {
		t.Error("expected true with journal present")
	}
}

func TestJournal_Entries_Roundtrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	entries := []JournalEntry{
		{Index: 0, SHA: "sha0", Message: "msg with \"quotes\" and\nnewlines"},
		{Index: 1, SHA: "sha1", Message: ""},
		{Index: 2, SHA: "sha2", Message: "normal message"},
	}

	journal := &Journal{
		Operation:  "rename",
		Entries:    entries,
		TargetIdx:  0,
		NewMsg:     "new msg with \"special\" chars",
		Step:       2,
		TotalSteps: 6,
	}

	if err := WriteJournal(journal); err != nil {
		t.Fatal(err)
	}

	got, err := ReadJournal()
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Entries) != len(entries) {
		t.Fatalf("entries count = %d, want %d", len(got.Entries), len(entries))
	}

	for i, e := range got.Entries {
		if e.SHA != entries[i].SHA {
			t.Errorf("entry[%d].SHA = %q, want %q", i, e.SHA, entries[i].SHA)
		}
		if e.Message != entries[i].Message {
			t.Errorf("entry[%d].Message = %q, want %q", i, e.Message, entries[i].Message)
		}
	}

	if got.Step != 2 {
		t.Errorf("Step = %d, want 2", got.Step)
	}
}
