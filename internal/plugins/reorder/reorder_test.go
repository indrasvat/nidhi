package reorder_test

import (
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/indrasvat/nidhi/internal/plugin"
	"github.com/indrasvat/nidhi/internal/plugins/reorder"
)

// ─── Journal Unit Tests ─────────────────────────────────────

func TestJournalWriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-journal.json")

	entries := []reorder.JournalEntry{
		{Index: 0, SHA: "aaa111", Message: "stash 0"},
		{Index: 1, SHA: "bbb222", Message: "stash 1"},
		{Index: 2, SHA: "ccc333", Message: "stash 2"},
	}

	j := reorder.NewJournal(1, 0, entries)
	j.SetPath(path)

	if err := j.Write(); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify file is valid JSON.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	loaded, err := reorder.LoadJournal(path)
	if err != nil {
		t.Fatalf("LoadJournal failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadJournal returned nil")
	}
	if loaded.SourceIndex != 1 {
		t.Errorf("expected SourceIndex=1, got %d", loaded.SourceIndex)
	}
	if loaded.TargetIndex != 0 {
		t.Errorf("expected TargetIndex=0, got %d", loaded.TargetIndex)
	}
	if len(loaded.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded.Entries))
	}
	if loaded.Entries[2].SHA != "ccc333" {
		t.Errorf("expected SHA 'ccc333', got %q", loaded.Entries[2].SHA)
	}
	if !loaded.IsIncomplete() {
		t.Error("expected journal to be incomplete")
	}
}

func TestJournalMarkComplete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-journal.json")

	j := reorder.NewJournal(0, 1, []reorder.JournalEntry{
		{Index: 0, SHA: "aaa", Message: "msg"},
	})
	j.SetPath(path)
	if err := j.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err := j.MarkComplete(); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	if j.IsIncomplete() {
		t.Error("expected complete after MarkComplete")
	}

	loaded, err := reorder.LoadJournal(path)
	if err != nil {
		t.Fatalf("LoadJournal: %v", err)
	}
	if loaded.IsIncomplete() {
		t.Error("reloaded journal should be complete")
	}
}

func TestJournalRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-journal.json")

	j := reorder.NewJournal(0, 1, nil)
	j.SetPath(path)
	if err := j.Write(); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := j.Remove(); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		t.Error("expected journal file to be removed")
	}
}

func TestJournalRemoveNonexistent(t *testing.T) {
	j := reorder.NewJournal(0, 1, nil)
	j.SetPath("/tmp/nonexistent-nidhi-test-journal-xyz.json")
	if err := j.Remove(); err != nil {
		t.Errorf("expected no error removing nonexistent, got %v", err)
	}
}

func TestJournalLoadNonexistent(t *testing.T) {
	j, err := reorder.LoadJournal("/tmp/nonexistent-nidhi-test-journal-xyz.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if j != nil {
		t.Error("expected nil journal for nonexistent path")
	}
}

func TestJournalLoadInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{invalid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := reorder.LoadJournal(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestIsIncomplete_NilJournal(t *testing.T) {
	var j *reorder.Journal
	if j.IsIncomplete() {
		t.Error("nil journal should not be incomplete")
	}
}

// ─── ComputeNewOrder Unit Tests ─────────────────────────────

func TestComputeNewOrder_MoveDown(t *testing.T) {
	// [A, B, C, D, E] → move A (0) to position 1 → [B, A, C, D, E]
	entries := []reorder.JournalEntry{
		{Index: 0, SHA: "A", Message: "A"},
		{Index: 1, SHA: "B", Message: "B"},
		{Index: 2, SHA: "C", Message: "C"},
		{Index: 3, SHA: "D", Message: "D"},
		{Index: 4, SHA: "E", Message: "E"},
	}
	result := reorder.ComputeNewOrder(entries, 0, 1)
	expected := []string{"B", "A", "C", "D", "E"}
	for i, e := range result {
		if e.SHA != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], e.SHA)
		}
	}
}

func TestComputeNewOrder_MoveUp(t *testing.T) {
	// [A, B, C, D, E] → move C (2) to position 1 → [A, C, B, D, E]
	entries := []reorder.JournalEntry{
		{Index: 0, SHA: "A", Message: "A"},
		{Index: 1, SHA: "B", Message: "B"},
		{Index: 2, SHA: "C", Message: "C"},
		{Index: 3, SHA: "D", Message: "D"},
		{Index: 4, SHA: "E", Message: "E"},
	}
	result := reorder.ComputeNewOrder(entries, 2, 1)
	expected := []string{"A", "C", "B", "D", "E"}
	for i, e := range result {
		if e.SHA != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], e.SHA)
		}
	}
}

func TestComputeNewOrder_MoveToEnd(t *testing.T) {
	// [A, B, C, D] → move B (1) to position 3 → [A, C, D, B]
	entries := []reorder.JournalEntry{
		{Index: 0, SHA: "A", Message: "A"},
		{Index: 1, SHA: "B", Message: "B"},
		{Index: 2, SHA: "C", Message: "C"},
		{Index: 3, SHA: "D", Message: "D"},
	}
	result := reorder.ComputeNewOrder(entries, 1, 3)
	expected := []string{"A", "C", "D", "B"}
	for i, e := range result {
		if e.SHA != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], e.SHA)
		}
	}
}

func TestComputeNewOrder_MoveToStart(t *testing.T) {
	// [A, B, C, D] → move D (3) to position 0 → [D, A, B, C]
	entries := []reorder.JournalEntry{
		{Index: 0, SHA: "A", Message: "A"},
		{Index: 1, SHA: "B", Message: "B"},
		{Index: 2, SHA: "C", Message: "C"},
		{Index: 3, SHA: "D", Message: "D"},
	}
	result := reorder.ComputeNewOrder(entries, 3, 0)
	expected := []string{"D", "A", "B", "C"}
	for i, e := range result {
		if e.SHA != expected[i] {
			t.Errorf("position %d: expected %s, got %s", i, expected[i], e.SHA)
		}
	}
}

func TestComputeNewOrder_TwoElements(t *testing.T) {
	entries := []reorder.JournalEntry{
		{Index: 0, SHA: "A", Message: "A"},
		{Index: 1, SHA: "B", Message: "B"},
	}

	// Swap: move 0 to 1 → [B, A]
	result := reorder.ComputeNewOrder(entries, 0, 1)
	if result[0].SHA != "B" || result[1].SHA != "A" {
		t.Errorf("swap 0→1: expected [B,A], got [%s,%s]", result[0].SHA, result[1].SHA)
	}

	// Swap: move 1 to 0 → [B, A]
	result = reorder.ComputeNewOrder(entries, 1, 0)
	if result[0].SHA != "B" || result[1].SHA != "A" {
		t.Errorf("swap 1→0: expected [B,A], got [%s,%s]", result[0].SHA, result[1].SHA)
	}
}

// ─── Plugin Unit Tests ──────────────────────────────────────

func newTestPlugin(t *testing.T) *reorder.Plugin {
	t.Helper()
	p := reorder.New()
	pctx := plugin.PluginContext{
		Logger: slog.Default(),
	}
	if err := p.Init(pctx); err != nil {
		t.Fatalf("init reorder plugin: %v", err)
	}
	return p
}

func TestPlugin_KeyBindings(t *testing.T) {
	p := newTestPlugin(t)
	bindings := p.KeyBindings()
	if len(bindings) != 2 {
		t.Fatalf("expected 2 keybindings, got %d", len(bindings))
	}
	if bindings[0].Key != "J" {
		t.Errorf("expected first key 'J', got %q", bindings[0].Key)
	}
	if bindings[1].Key != "K" {
		t.Errorf("expected second key 'K', got %q", bindings[1].Key)
	}
}

func TestPlugin_MoveDownAtBottom_NoOp(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{
		Cursor:  2,
		Stashes: []plugin.Stash{{Index: 0}, {Index: 1}, {Index: 2}},
	}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "J"}, state)
	if cmd != nil {
		t.Error("expected nil cmd when cursor at bottom")
	}
}

func TestPlugin_MoveUpAtTop_NoOp(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{
		Cursor:  0,
		Stashes: []plugin.Stash{{Index: 0}, {Index: 1}, {Index: 2}},
	}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "K"}, state)
	if cmd != nil {
		t.Error("expected nil cmd when cursor at top")
	}
}

func TestPlugin_UnknownKeyIgnored(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{
		Cursor:  1,
		Stashes: []plugin.Stash{{Index: 0}, {Index: 1}},
	}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "x"}, state)
	if cmd != nil {
		t.Error("expected nil cmd for unknown key")
	}
}

func TestPlugin_MoveDown_CursorFollows(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{
		Cursor: 0,
		Stashes: []plugin.Stash{
			{Index: 0, SHA: "aaa", Message: "msg0"},
			{Index: 1, SHA: "bbb", Message: "msg1"},
		},
	}
	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "J"}, state)
	if cmd == nil {
		t.Error("expected non-nil cmd for valid move down")
	}
	if newState.Cursor != 1 {
		t.Errorf("expected cursor to follow stash to 1, got %d", newState.Cursor)
	}
}

func TestPlugin_MoveUp_CursorFollows(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{
		Cursor: 1,
		Stashes: []plugin.Stash{
			{Index: 0, SHA: "aaa", Message: "msg0"},
			{Index: 1, SHA: "bbb", Message: "msg1"},
		},
	}
	newState, cmd := p.HandleKey(plugin.KeyEvent{Key: "K"}, state)
	if cmd == nil {
		t.Error("expected non-nil cmd for valid move up")
	}
	if newState.Cursor != 0 {
		t.Errorf("expected cursor to follow stash to 0, got %d", newState.Cursor)
	}
}

func TestPlugin_EmptyStashList(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{Cursor: 0}
	_, cmd := p.HandleKey(plugin.KeyEvent{Key: "J"}, state)
	if cmd != nil {
		t.Error("expected nil cmd for empty stash list")
	}
}

func TestPlugin_SingleStash(t *testing.T) {
	p := newTestPlugin(t)
	state := plugin.AppState{
		Cursor:  0,
		Stashes: []plugin.Stash{{Index: 0}},
	}
	_, cmdJ := p.HandleKey(plugin.KeyEvent{Key: "J"}, state)
	if cmdJ != nil {
		t.Error("expected nil cmd for single stash move down")
	}
	_, cmdK := p.HandleKey(plugin.KeyEvent{Key: "K"}, state)
	if cmdK != nil {
		t.Error("expected nil cmd for single stash move up")
	}
}

// ─── Git Integration Tests ──────────────────────────────────

func testRepo(t *testing.T, numStashes int) (string, func(args ...string) string) {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\noutput: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "base.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "init")

	for i := range numStashes {
		fname := filepath.Join(dir, "file"+strconv.Itoa(i)+".go")
		if err := os.WriteFile(fname, []byte("package f"+strconv.Itoa(i)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		run("git", "add", ".")
		run("git", "stash", "push", "-m", "stash "+strconv.Itoa(i))
	}

	return dir, run
}

func getStashMessages(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "stash", "list", "--format=%gs")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git stash list failed: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func getStashSHAs(t *testing.T, dir string) []string {
	t.Helper()
	cmd := exec.Command("git", "stash", "list", "--format=%H")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git stash list failed: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

// executeReorder performs the drop-all + re-store reorder using real git commands.
func executeReorder(t *testing.T, _ string, run func(...string) string, entries []reorder.JournalEntry, sourceIdx, targetIdx int) {
	t.Helper()
	newOrder := reorder.ComputeNewOrder(entries, sourceIdx, targetIdx)

	// Drop all stashes (highest first).
	for i := len(entries) - 1; i >= 0; i-- {
		run("git", "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
	}

	// Re-store in new order (last to first, since store prepends).
	for i := len(newOrder) - 1; i >= 0; i-- {
		run("git", "stash", "store", "-m", newOrder[i].Message, newOrder[i].SHA)
	}
}

func buildEntries(t *testing.T, dir string) []reorder.JournalEntry {
	t.Helper()
	messages := getStashMessages(t, dir)
	shas := getStashSHAs(t, dir)
	entries := make([]reorder.JournalEntry, len(messages))
	for i := range messages {
		entries[i] = reorder.JournalEntry{Index: i, SHA: shas[i], Message: messages[i]}
	}
	return entries
}

func TestMoveStashUp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, run := testRepo(t, 5)
	originalMessages := getStashMessages(t, dir)
	originalSHAs := getStashSHAs(t, dir)
	if len(originalMessages) != 5 {
		t.Fatalf("expected 5 stashes, got %d", len(originalMessages))
	}

	entries := buildEntries(t, dir)

	// Move stash@{2} up to position 1 (K pressed at cursor 2).
	executeReorder(t, dir, run, entries, 2, 1)

	newMessages := getStashMessages(t, dir)
	if len(newMessages) != 5 {
		t.Fatalf("expected 5 stashes after reorder, got %d", len(newMessages))
	}

	// Original: [4, 3, 2, 1, 0]. After move 2→1: [4, 2, 3, 1, 0].
	expectedMessages := []string{
		originalMessages[0],
		originalMessages[2],
		originalMessages[1],
		originalMessages[3],
		originalMessages[4],
	}
	for i, msg := range newMessages {
		if msg != expectedMessages[i] {
			t.Errorf("position %d: expected %q, got %q", i, expectedMessages[i], msg)
		}
	}

	// Verify SHAs preserved.
	newSHAs := getStashSHAs(t, dir)
	expectedSHAs := []string{
		originalSHAs[0], originalSHAs[2], originalSHAs[1],
		originalSHAs[3], originalSHAs[4],
	}
	for i, sha := range newSHAs {
		if sha != expectedSHAs[i] {
			t.Errorf("position %d: SHA mismatch", i)
		}
	}
}

func TestMoveStashDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, run := testRepo(t, 5)
	originalMessages := getStashMessages(t, dir)
	entries := buildEntries(t, dir)

	// Move stash@{0} down to position 1 (J pressed at cursor 0).
	executeReorder(t, dir, run, entries, 0, 1)

	newMessages := getStashMessages(t, dir)
	// Original: [4, 3, 2, 1, 0]. After move 0→1: [3, 4, 2, 1, 0].
	expectedMessages := []string{
		originalMessages[1],
		originalMessages[0],
		originalMessages[2],
		originalMessages[3],
		originalMessages[4],
	}
	for i, msg := range newMessages {
		if msg != expectedMessages[i] {
			t.Errorf("position %d: expected %q, got %q", i, expectedMessages[i], msg)
		}
	}
}

func TestMoveMiddleStashToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, run := testRepo(t, 4)
	originalMessages := getStashMessages(t, dir)
	entries := buildEntries(t, dir)

	// Move stash@{1} to position 3 (last).
	executeReorder(t, dir, run, entries, 1, 3)

	newMessages := getStashMessages(t, dir)
	// Original: [3, 2, 1, 0]. Move idx 1 to 3: [3, 1, 0, 2].
	expectedMessages := []string{
		originalMessages[0],
		originalMessages[2],
		originalMessages[3],
		originalMessages[1],
	}
	for i, msg := range newMessages {
		if msg != expectedMessages[i] {
			t.Errorf("position %d: expected %q, got %q", i, expectedMessages[i], msg)
		}
	}
}

func TestCrashRecoverySimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir, run := testRepo(t, 5)
	originalMessages := getStashMessages(t, dir)
	originalSHAs := getStashSHAs(t, dir)
	entries := buildEntries(t, dir)

	// Write journal as if we started a reorder.
	journalPath := filepath.Join(dir, "move-journal.json")
	j := reorder.NewJournal(2, 0, entries)
	j.SetPath(journalPath)
	if err := j.Write(); err != nil {
		t.Fatalf("Write journal: %v", err)
	}

	// Simulate partial crash: drop 3 of 5 stashes.
	run("git", "stash", "drop", "stash@{4}")
	run("git", "stash", "drop", "stash@{3}")
	run("git", "stash", "drop", "stash@{2}")

	badMessages := getStashMessages(t, dir)
	if len(badMessages) != 2 {
		t.Fatalf("expected 2 stashes after crash, got %d", len(badMessages))
	}

	// Manual recovery: clear remaining + re-store from journal.
	for i := len(badMessages) - 1; i >= 0; i-- {
		run("git", "stash", "drop", "stash@{"+strconv.Itoa(i)+"}")
	}
	for i := len(entries) - 1; i >= 0; i-- {
		run("git", "stash", "store", "-m", entries[i].Message, entries[i].SHA)
	}

	// Verify recovery: original order restored.
	recoveredMessages := getStashMessages(t, dir)
	if len(recoveredMessages) != 5 {
		t.Fatalf("expected 5 stashes after recovery, got %d", len(recoveredMessages))
	}
	for i, msg := range recoveredMessages {
		if msg != originalMessages[i] {
			t.Errorf("position %d: expected %q, got %q", i, originalMessages[i], msg)
		}
	}

	recoveredSHAs := getStashSHAs(t, dir)
	for i, sha := range recoveredSHAs {
		if sha != originalSHAs[i] {
			t.Errorf("position %d: SHA mismatch", i)
		}
	}
}
