package process

import (
	"strings"
	"unicode"
)

// Deduplicator decides whether an extracted item is new or a duplicate of an existing one.
// MVP uses deterministic normalization; AI-assisted merge can be plugged in later.
type Deduplicator struct {
	store *Store
}

// NewDeduplicator creates a deduplicator backed by the given store.
func NewDeduplicator(store *Store) *Deduplicator {
	return &Deduplicator{store: store}
}

// DedupResult contains only the items that should be merged into the database.
type DedupResult struct {
	NewTodos    []DedupedTodo
	NewMemories []DedupedMemory
}

// DedupedTodo is a todo candidate that survived deduplication.
type DedupedTodo struct {
	Text       string
	SourceFile string
}

// DedupedMemory is a memory candidate that survived deduplication.
type DedupedMemory struct {
	Topic      string
	Summary    string
	SourceFile string
}

// Dedup compares extracted candidates against existing active todos and memories.
// It returns only the genuinely new items, keeping the first source file attribution.
func (d *Deduplicator) Dedup(candidates *Extracted, sourceFile string, seenTodo, seenMemory map[string]struct{}) (*DedupResult, error) {
	existingTodos, err := d.store.ListActiveTodos()
	if err != nil {
		return nil, err
	}
	existingMemories, err := d.store.ListMemories()
	if err != nil {
		return nil, err
	}

	for _, t := range existingTodos {
		seenTodo[normalize(t.Text)] = struct{}{}
	}
	for _, m := range existingMemories {
		seenMemory[normalize(m.Topic)] = struct{}{}
	}

	result := &DedupResult{}
	for _, text := range candidates.Todos {
		key := normalize(text)
		if _, dup := seenTodo[key]; dup {
			continue
		}
		result.NewTodos = append(result.NewTodos, DedupedTodo{Text: text, SourceFile: sourceFile})
		seenTodo[key] = struct{}{}
	}
	for _, m := range candidates.Memories {
		key := normalize(m.Topic)
		if _, dup := seenMemory[key]; dup {
			continue
		}
		result.NewMemories = append(result.NewMemories, DedupedMemory{Topic: m.Topic, Summary: m.Summary, SourceFile: sourceFile})
		seenMemory[key] = struct{}{}
	}

	return result, nil
}

// normalize prepares text for equality comparison.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsSpace(r) {
			if b.Len() > 0 && b.String()[b.Len()-1] != ' ' {
				b.WriteRune(' ')
			}
			continue
		}
		if unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
