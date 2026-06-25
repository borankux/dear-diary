package search

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setup 写入几个测试日记到一个临时目录，返回目录路径。
func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entries := []struct {
		date, body string
	}{
		{"2026-06-25", "# today\n\n## 09:00\n\n今天写 Go 代码，用 Bubbletea 做 TUI。\n"},
		{"2026-06-24", "# yesterday\n\n## 22:00\n\n读了 Bubbletea 源码。\n"},
		{"2026-06-23", "# day before\n\n## 08:00\n\n普通的一天，没特别的事。\n"},
	}
	for _, e := range entries {
		t1, _ := time.Parse("2006-01-02", e.date)
		monthDir := filepath.Join(dir, t1.Format("2006-01"))
		os.MkdirAll(monthDir, 0o755)
		path := filepath.Join(monthDir, e.date+".md")
		os.WriteFile(path, []byte(e.body), 0o644)
	}
	return dir
}

func TestSearchFallback(t *testing.T) {
	dir := setup(t)
	// 强制走 fallback（不依赖 rg 是否安装）
	results, err := searchWithGo(dir, "Bubbletea")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d: %+v", len(results), results)
	}
	// 应按日期倒序
	if results[0].Date != "2026-06-25" {
		t.Errorf("results[0].Date = %s, want 2026-06-25", results[0].Date)
	}
	if results[1].Date != "2026-06-24" {
		t.Errorf("results[1].Date = %s, want 2026-06-24", results[1].Date)
	}
}

func TestSearchNoKeyword(t *testing.T) {
	dir := setup(t)
	results, err := Search(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Errorf("expected nil for empty keyword, got %v", results)
	}
}

func TestSearchNoMatch(t *testing.T) {
	dir := setup(t)
	results, err := searchWithGo(dir, "不存在的内容_zzz")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 matches, got %d", len(results))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	dir := setup(t)
	results, err := searchWithGo(dir, "bubbletea")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 case-insensitive matches, got %d", len(results))
	}
}

func TestSearchRgIfAvailable(t *testing.T) {
	dir := setup(t)
	// 仅当 rg 安装时验证
	results, err := Search(dir, "Bubbletea")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected at least 1 result")
	}
	// 验证按日期倒序
	for i := 1; i < len(results); i++ {
		if results[i].Date > results[i-1].Date {
			t.Errorf("not sorted desc: %s before %s", results[i-1].Date, results[i].Date)
		}
	}
}
