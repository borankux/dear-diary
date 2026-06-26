package process

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SummaryWriter renders extracted assets to markdown files.
type SummaryWriter struct {
	outDir string
}

// NewSummaryWriter creates a writer that outputs to the given directory.
func NewSummaryWriter(outDir string) *SummaryWriter {
	return &SummaryWriter{outDir: outDir}
}

// WriteAll regenerates todos.md and memories.md from the store.
func (w *SummaryWriter) WriteAll(store *Store) error {
	if err := os.MkdirAll(w.outDir, 0o755); err != nil {
		return err
	}
	if err := w.writeTodos(store); err != nil {
		return err
	}
	if err := w.writeMemories(store); err != nil {
		return err
	}
	return nil
}

func (w *SummaryWriter) writeTodos(store *Store) error {
	todos, err := store.ListActiveTodos()
	if err != nil {
		return err
	}
	path := filepath.Join(w.outDir, "todos.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# 活跃 Todo 列表\n\n")
	fmt.Fprintf(f, "> 自动生成于 %s\n\n", time.Now().Format("2006-01-02 15:04"))
	if len(todos) == 0 {
		fmt.Fprintln(f, "暂无活跃 Todo。")
		return nil
	}
	for _, t := range todos {
		fmt.Fprintf(f, "- [ ] %s\n", t.Text)
		if t.SourceFile != "" {
			fmt.Fprintf(f, "  - 来源: %s\n", t.SourceFile)
		}
	}
	return nil
}

func (w *SummaryWriter) writeMemories(store *Store) error {
	memories, err := store.ListMemories()
	if err != nil {
		return err
	}
	path := filepath.Join(w.outDir, "memories.md")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "# Memory 摘要\n\n")
	fmt.Fprintf(f, "> 自动生成于 %s\n\n", time.Now().Format("2006-01-02 15:04"))
	if len(memories) == 0 {
		fmt.Fprintln(f, "暂无 Memory。")
		return nil
	}
	for _, m := range memories {
		fmt.Fprintf(f, "## %s\n\n", m.Topic)
		fmt.Fprintf(f, "%s\n\n", m.Summary)
		if m.SourceFile != "" {
			fmt.Fprintf(f, "- 来源: %s\n\n", m.SourceFile)
		}
	}
	return nil
}
