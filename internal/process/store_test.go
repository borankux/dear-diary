package process

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "process.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoreChangedFiles(t *testing.T) {
	s := newTestStore(t)

	files := []FileInfo{
		{Path: "/a/2026-06-24.md", Hash: "h1", ModTime: time.Now().UTC()},
		{Path: "/a/2026-06-25.md", Hash: "h2", ModTime: time.Now().UTC()},
	}

	changed, err := s.ChangedFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 2 {
		t.Fatalf("expected 2 changed, got %d", len(changed))
	}

	// Persist snapshots.
	for _, f := range files {
		if err := s.UpdateSnapshot(f); err != nil {
			t.Fatal(err)
		}
	}

	// Same files again -> no changes.
	changed, err = s.ChangedFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 0 {
		t.Fatalf("expected 0 changed, got %d", len(changed))
	}

	// Modified file -> changed.
	files[0].Hash = "h1-modified"
	changed, err = s.ChangedFiles(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 || changed[0].Path != files[0].Path {
		t.Fatalf("expected 1 changed for %s, got %v", files[0].Path, changed)
	}
}

func TestStoreRunLifecycle(t *testing.T) {
	s := newTestStore(t)

	if err := s.CreateRun("run-1", "hash-1"); err != nil {
		t.Fatal(err)
	}

	done, err := s.HasSuccessfulRun("hash-1")
	if err != nil {
		t.Fatal(err)
	}
	if done {
		t.Fatal("run should not be successful yet")
	}

	if err := s.FinishRun("run-1", StateDone, 0); err != nil {
		t.Fatal(err)
	}

	done, err = s.HasSuccessfulRun("hash-1")
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("run should now be successful")
	}
}

func TestStoreInsertAndList(t *testing.T) {
	s := newTestStore(t)

	if err := s.InsertTodo("buy milk", "/a/2026-06-25.md"); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertMemory("Go tips", "use defer", "/a/2026-06-25.md"); err != nil {
		t.Fatal(err)
	}

	todos, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 || todos[0].Text != "buy milk" {
		t.Fatalf("unexpected todos: %v", todos)
	}

	memories, err := s.ListMemories()
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 || memories[0].Topic != "Go tips" {
		t.Fatalf("unexpected memories: %v", memories)
	}
}

func TestCandidateLifecycle(t *testing.T) {
	s := newTestStore(t)

	c := Candidate{
		Type:         CandidateTypeTodo,
		Title:        "Close the loop",
		Content:      "Make AI output reviewable before it becomes a todo.",
		SourceFile:   "/a/2026-06/2026-06-25.md",
		SourceDate:   "2026-06-25",
		SourceHash:   "hash-1",
		EvidenceText: "AI output should not directly enter final tables.",
		RawAIJSON:    `{"items":[]}`,
		Confidence:   0.9,
	}

	inserted, err := s.InsertCandidateIfNew(c)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("expected first candidate insert")
	}
	inserted, err = s.InsertCandidateIfNew(c)
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("duplicate candidate should not be inserted")
	}

	pending, err := s.ListPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending candidate, got %d", len(pending))
	}
	if err := s.AcceptCandidate(pending[0].ID); err != nil {
		t.Fatal(err)
	}

	pending, err = s.ListPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending candidates, got %d", len(pending))
	}
	todos, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 || todos[0].Text != "Close the loop" || todos[0].EvidenceText == "" {
		t.Fatalf("unexpected accepted todo: %+v", todos)
	}
}

func TestCandidateRejectPreventsResurface(t *testing.T) {
	s := newTestStore(t)
	c := Candidate{
		Type:       CandidateTypeMemory,
		Title:      "Provider boundary",
		Content:    "DeepSeek is a current provider, not product identity.",
		SourceFile: "/a/2026-06/2026-06-25.md",
		SourceHash: "hash-2",
	}
	inserted, err := s.InsertCandidateIfNew(c)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("expected insert")
	}
	pending, err := s.ListPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RejectCandidate(pending[0].ID); err != nil {
		t.Fatal(err)
	}
	inserted, err = s.InsertCandidateIfNew(c)
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("rejected duplicate should not resurface")
	}
}

func TestTodoDoneAndArchive(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertTodo("ship closure core", "/a/2026-06/2026-06-25.md"); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertTodo("archive me", "/a/2026-06/2026-06-25.md"); err != nil {
		t.Fatal(err)
	}
	todos, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 active todos, got %d", len(todos))
	}
	if err := s.MarkTodoDone(todos[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := s.ArchiveTodo(todos[1].ID); err != nil {
		t.Fatal(err)
	}
	todos, err = s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 0 {
		t.Fatalf("expected no active todos after done/archive, got %+v", todos)
	}
}
