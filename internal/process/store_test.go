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

func TestTodoPrioritySortingAndStatusLifecycle(t *testing.T) {
	s := newTestStore(t)

	if err := s.InsertTodo("low priority", "/a/2026-06/2026-06-25.md"); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertTodo("high priority", "/a/2026-06/2026-06-25.md"); err != nil {
		t.Fatal(err)
	}
	todos, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]int)
	for _, todo := range todos {
		ids[todo.Text] = todo.ID
	}
	low := 20
	high := 90
	if err := s.SetTodoPriority(ids["low priority"], &low); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTodoPriority(ids["high priority"], &high); err != nil {
		t.Fatal(err)
	}

	todos, err = s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 2 || todos[0].Text != "high priority" || !todos[0].HasPriority || todos[0].Priority != 90 {
		t.Fatalf("expected high priority todo first, got %+v", todos)
	}
	if err := s.SetTodoStatus(todos[0].ID, TodoStatusInProgress); err != nil {
		t.Fatal(err)
	}
	active, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	inProgress, err := s.ListTodosByStatus(TodoStatusInProgress)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || len(inProgress) != 1 || inProgress[0].Text != "high priority" {
		t.Fatalf("unexpected lifecycle lists: active=%+v inProgress=%+v", active, inProgress)
	}
	counts, err := s.TodoStatusCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Active != 1 || counts.InProgress != 1 {
		t.Fatalf("unexpected lifecycle counts: %+v", counts)
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
	counts, err := s.CandidateStatusCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Pending != 0 || counts.Accepted != 1 || counts.Rejected != 0 {
		t.Fatalf("unexpected candidate counts: %+v", counts)
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

func TestCandidateDedupIgnoresChangedSourceHashForSameDate(t *testing.T) {
	s := newTestStore(t)
	c := Candidate{
		Type:       CandidateTypeTodo,
		Title:      "Write dashboard review",
		Content:    "Review the dashboard after diary processing.",
		SourceFile: "/a/2026-06/2026-06-26.md",
		SourceDate: "2026-06-26",
		SourceHash: "hash-before-edit",
	}
	inserted, err := s.InsertCandidateIfNew(c)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("expected first insert")
	}
	pending, err := s.ListPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptCandidate(pending[0].ID); err != nil {
		t.Fatal(err)
	}

	c.SourceHash = "hash-after-edit"
	inserted, err = s.InsertCandidateIfNew(c)
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("same-date duplicate should not reappear just because source hash changed")
	}

	c.SourceDate = "2026-06-27"
	c.SourceFile = "/a/2026-06/2026-06-27.md"
	inserted, err = s.InsertCandidateIfNew(c)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted {
		t.Fatal("same content on a different diary date should be allowed")
	}
}

func TestMergeDuplicateItems(t *testing.T) {
	s := newTestStore(t)
	candidates := []Candidate{
		{
			Type:       CandidateTypeTodo,
			Title:      "Close dashboard loop",
			Content:    "Close dashboard loop from AI Inbox.",
			SourceFile: "/a/2026-06/2026-06-26.md",
			SourceDate: "2026-06-26",
			SourceHash: "hash-a",
		},
		{
			Type:       CandidateTypeTodo,
			Title:      "Close dashboard loop",
			Content:    "Close dashboard loop from AI Inbox.",
			SourceFile: "/a/2026-06/2026-06-27.md",
			SourceDate: "2026-06-27",
			SourceHash: "hash-b",
		},
	}
	for _, c := range candidates {
		inserted, err := s.InsertCandidateIfNew(c)
		if err != nil {
			t.Fatal(err)
		}
		if !inserted {
			t.Fatal("expected candidate insert")
		}
	}
	if err := s.InsertTodo("Merge duplicate dashboard task", "/a/2026-06/2026-06-26.md"); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertTodo("Merge duplicate dashboard task", "/a/2026-06/2026-06-27.md"); err != nil {
		t.Fatal(err)
	}

	result, err := s.MergeDuplicateItems()
	if err != nil {
		t.Fatal(err)
	}
	if result.CandidateMerged != 1 || result.TodoMerged != 1 {
		t.Fatalf("unexpected merge result: %+v", result)
	}
	pending, err := s.ListPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending candidate after merge, got %d", len(pending))
	}
	todos, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 active todo after merge, got %d", len(todos))
	}
	archived, err := s.ListTodosByStatus(TodoStatusArchived)
	if err != nil {
		t.Fatal(err)
	}
	if len(archived) != 1 {
		t.Fatalf("expected 1 archived duplicate todo, got %d", len(archived))
	}
}

func TestMergeDuplicateItemsKeepsDoneTodo(t *testing.T) {
	s := newTestStore(t)
	if err := s.InsertTodo("准备明天路演", "/a/2026-06/2026-06-28.md"); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertTodo("准备明天路演", "/a/2026-06/2026-06-28.md"); err != nil {
		t.Fatal(err)
	}
	todos, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 active todos before merge, got %d", len(todos))
	}
	if err := s.MarkTodoDone(todos[0].ID); err != nil {
		t.Fatal(err)
	}

	result, err := s.MergeDuplicateItems()
	if err != nil {
		t.Fatal(err)
	}
	if result.TodoMerged != 1 {
		t.Fatalf("expected 1 merged todo, got %+v", result)
	}
	active, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("expected duplicate active todo archived, got %+v", active)
	}
	done, err := s.ListTodosByStatus(TodoStatusDone)
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 {
		t.Fatalf("expected done representative preserved, got %d", len(done))
	}
}

func TestPromoteAllPendingCandidates(t *testing.T) {
	s := newTestStore(t)
	candidates := []Candidate{
		{
			Type:       CandidateTypeTodo,
			Title:      "One click promote",
			Content:    "Promote all pending candidates.",
			SourceFile: "/a/2026-06/2026-06-26.md",
			SourceDate: "2026-06-26",
			SourceHash: "hash-a",
		},
		{
			Type:       CandidateTypeMemory,
			Title:      "Inbox semantics",
			Content:    "AI Inbox is not a mandatory review queue.",
			SourceFile: "/a/2026-06/2026-06-26.md",
			SourceDate: "2026-06-26",
			SourceHash: "hash-b",
		},
	}
	for _, c := range candidates {
		inserted, err := s.InsertCandidateIfNew(c)
		if err != nil {
			t.Fatal(err)
		}
		if !inserted {
			t.Fatal("expected candidate insert")
		}
	}

	result, err := s.PromoteAllPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if result.PromotedTodos != 1 || result.PromotedMemories != 1 {
		t.Fatalf("unexpected promotion result: %+v", result)
	}
	pending, err := s.ListPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected empty inbox, got %d", len(pending))
	}
	todos, err := s.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 active todo, got %d", len(todos))
	}
	memories, err := s.ListMemories()
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) != 1 {
		t.Fatalf("expected 1 active memory, got %d", len(memories))
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
	done, err := s.ListTodosByStatus("done")
	if err != nil {
		t.Fatal(err)
	}
	archived, err := s.ListTodosByStatus("archived")
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 || len(archived) != 1 {
		t.Fatalf("expected one done and one archived todo, got done=%+v archived=%+v", done, archived)
	}
	counts, err := s.TodoStatusCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts.Active != 0 || counts.Done != 1 || counts.Archived != 1 {
		t.Fatalf("unexpected todo counts: %+v", counts)
	}
}
