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
	if os.Getenv("DIARY_LLM_API_KEY") == "" && os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DIARY_LLM_API_KEY not set")
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

	candidates, err := runner.store.ListPendingCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one pending candidate extracted")
	}
}

func TestCandidatesFromExtractedPreserveSourceEvidence(t *testing.T) {
	ext := &Extracted{
		Items: []CandidateExtract{
			{
				Type:         "todo",
				Title:        "Review candidates",
				Content:      "Review AI candidates before accepting them.",
				EvidenceText: "AI 不能直接进入正式库",
				Confidence:   0.9,
			},
			{
				Type:         "memory",
				Title:        "Closure Core",
				Content:      "v0.4 focuses on lifecycle closure.",
				EvidenceText: "不是尽可能多的功能",
				Confidence:   0.8,
			},
		},
		RawJSON: `{"items":[]}`,
	}
	source := FileInfo{
		Path: "/tmp/diary/2026-06/2026-06-25.md",
		Hash: "hash-1",
	}
	candidates := candidatesFromExtracted(ext, source)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Type != CandidateTypeTodo || candidates[0].SourceDate != "2026-06-25" || candidates[0].EvidenceText == "" {
		t.Fatalf("bad todo candidate: %+v", candidates[0])
	}
	if candidates[1].Type != CandidateTypeMemory || candidates[1].SourceHash != "hash-1" || candidates[1].RawAIJSON == "" {
		t.Fatalf("bad memory candidate: %+v", candidates[1])
	}
}
