package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/borankux/dear-diary/internal/process"
)

func TestCollectionAPIsUseFrontendJSONKeys(t *testing.T) {
	t.Setenv("DIARY_DB_PATH", filepath.Join(t.TempDir(), "process.db"))
	t.Setenv("DIARY_DIR", t.TempDir())

	store, err := process.NewStore("")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.InsertTodoFromCandidate("ship the dashboard", "/diary/2026-06-30.md", "2026-06-30", "hash", "evidence", 0); err != nil {
		t.Fatal(err)
	}
	todos, err := store.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	priority := 90
	if err := store.SetTodoPriority(todos[0].ID, &priority); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertCandidateIfNew(process.Candidate{
		Type:         process.CandidateTypeTodo,
		Title:        "Review dashboard",
		Content:      "Review dashboard content",
		SourceFile:   "/diary/2026-06-30.md",
		SourceDate:   "2026-06-30",
		EvidenceText: "dashboard evidence",
		RawAIJSON:    `{"large":"internal"}`,
		Confidence:   0.8,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertMemoryFromCandidate("Dashboard", "Readable board", "/diary/2026-06-30.md", "2026-06-30", "hash", "memory evidence", 0); err != nil {
		t.Fatal(err)
	}

	assertCollectionKeys(t, handleTodos, "id", "text", "status", "hasPriority", "ID", "Text")
	assertCollectionKeys(t, handleCandidates, "id", "type", "title", "content", "ID", "RawAIJSON")
	assertCollectionKeys(t, handleMemories, "id", "topic", "summary", "sourceDate", "ID", "Topic")
}

func assertCollectionKeys(t *testing.T, handler http.HandlerFunc, requiredA, requiredB, requiredC, requiredD, forbiddenA, forbiddenB string) {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d: %s", len(rows), rec.Body.String())
	}
	row := rows[0]
	for _, key := range []string{requiredA, requiredB, requiredC, requiredD} {
		if _, ok := row[key]; !ok {
			t.Fatalf("missing key %q in %#v", key, row)
		}
	}
	for _, key := range []string{forbiddenA, forbiddenB} {
		if _, ok := row[key]; ok {
			t.Fatalf("unexpected key %q in %#v", key, row)
		}
	}
}
