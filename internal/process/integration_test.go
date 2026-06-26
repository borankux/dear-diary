package process

import (
	"os"
	"path/filepath"
	"testing"
)

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunnerEndToEnd(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}

	dir := t.TempDir()
	outDir := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "process.db")

	content := `# 2026-06-25 周四

## 09:00

今天要做几件事：
- 把 dear-diary 的 AI 提炼功能跑通
- 记得给植物浇水

还学到了一个 Go 技巧：defer 可以成对使用来测量函数耗时。
`
	monthDir := filepath.Join(dir, "2026-06")
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(monthDir, "2026-06-25.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	runner, err := NewRunnerWithStore(dir, outDir, storePath)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	defer runner.Close()

	if err := runner.Run(); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if runner.machine.State() != StateDone {
		t.Fatalf("expected Done, got %s", runner.machine.State())
	}

	// Check markdown outputs exist.
	if _, err := os.Stat(filepath.Join(outDir, "todos.md")); err != nil {
		t.Fatalf("todos.md missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "memories.md")); err != nil {
		t.Fatalf("memories.md missing: %v", err)
	}

	t.Logf("todos.md:\n%s", mustRead(t, filepath.Join(outDir, "todos.md")))
	t.Logf("memories.md:\n%s", mustRead(t, filepath.Join(outDir, "memories.md")))

	todos, err := runner.store.ListActiveTodos()
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) == 0 {
		t.Fatal("expected at least one todo extracted")
	}

	memories, err := runner.store.ListMemories()
	if err != nil {
		t.Fatal(err)
	}
	if len(memories) == 0 {
		t.Fatal("expected at least one memory extracted")
	}
}
