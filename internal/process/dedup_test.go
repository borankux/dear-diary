package process

import (
	"testing"
)

func TestDeduplicatorFiltersExactDuplicates(t *testing.T) {
	s := newTestStore(t)

	// Seed an existing todo and memory.
	if err := s.InsertTodo("buy milk", "/a/2026-06-24.md"); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertMemory("Go tips", "use defer", "/a/2026-06-24.md"); err != nil {
		t.Fatal(err)
	}

	d := NewDeduplicator(s)
	candidates := &Extracted{
		Todos: []string{"Buy milk!", "call mom"},
		Memories: []MemoryExtract{
			{Topic: "Go Tips", Summary: "defer is great"},
			{Topic: "AI workflow", Summary: "state machines help"},
		},
	}

	seenTodo := make(map[string]struct{})
	seenMemory := make(map[string]struct{})
	res, err := d.Dedup(candidates, "/a/2026-06-25.md", seenTodo, seenMemory)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.NewTodos) != 1 || res.NewTodos[0].Text != "call mom" {
		t.Fatalf("expected 1 new todo 'call mom', got %v", res.NewTodos)
	}
	if len(res.NewMemories) != 1 || res.NewMemories[0].Topic != "AI workflow" {
		t.Fatalf("expected 1 new memory 'AI workflow', got %v", res.NewMemories)
	}
}

func TestDeduplicatorFiltersWithinSameRun(t *testing.T) {
	s := newTestStore(t)
	d := NewDeduplicator(s)

	candidates := &Extracted{
		Todos: []string{"buy milk", "Buy Milk!", "buy  milk"},
	}

	seenTodo := make(map[string]struct{})
	res, err := d.Dedup(candidates, "/a/2026-06-25.md", seenTodo, seenMemory())
	if err != nil {
		t.Fatal(err)
	}

	if len(res.NewTodos) != 1 || res.NewTodos[0].Text != "buy milk" {
		t.Fatalf("expected 1 new todo after within-run dedup, got %v", res.NewTodos)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Buy Milk!", "buy milk"},
		{"  buy   milk  ", "buy milk"},
		{"Go tips: defer", "go tips defer"},
	}
	for _, tc := range cases {
		got := normalize(tc.in)
		if got != tc.want {
			t.Fatalf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func seenMemory() map[string]struct{} {
	return make(map[string]struct{})
}
