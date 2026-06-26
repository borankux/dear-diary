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
