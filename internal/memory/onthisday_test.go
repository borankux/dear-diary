package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/borankux/dear-diary/internal/storage"
)

func writeDiary(t *testing.T, s *storage.Storage, date, body string) {
	t.Helper()
	tt, _ := time.Parse("2006-01-02", date)
	monthDir := filepath.Join(s.RootDir(), tt.Format("2006-01"))
	os.MkdirAll(monthDir, 0o755)
	path := filepath.Join(monthDir, date+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOnThisDayFindsOneYearAgo(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	writeDiary(t, s, "2025-06-25", "# 2025-06-25 周三\n\n## 09:00\n\n一年前的今天我在写 Go。\n")
	today, _ := time.Parse("2006-01-02", "2026-06-25")

	results := OnThisDay(s, today, 5)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(results), results)
	}
	if results[0].YearsAgo != 1 {
		t.Errorf("YearsAgo = %d, want 1", results[0].YearsAgo)
	}
	if results[0].FirstLine != "一年前的今天我在写 Go。" {
		t.Errorf("FirstLine = %q", results[0].FirstLine)
	}
}

func TestOnThisDaySkipsEmptyTemplate(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	// 仅模板，无内容
	writeDiary(t, s, "2025-06-25", "# 2025-06-25 周三\n\n## 09:00\n\n")
	today, _ := time.Parse("2006-01-02", "2026-06-25")

	results := OnThisDay(s, today, 5)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty template, got %d", len(results))
	}
}

func TestOnThisDayMultipleYears(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	writeDiary(t, s, "2024-06-25", "# title\n\n## 09:00\n\n两年前。\n")
	writeDiary(t, s, "2023-06-25", "# title\n\n## 09:00\n\n三年前。\n")
	today, _ := time.Parse("2006-01-02", "2026-06-25")

	results := OnThisDay(s, today, 5)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2: %+v", len(results), results)
	}
	if results[0].YearsAgo != 2 {
		t.Errorf("first YearsAgo = %d, want 2", results[0].YearsAgo)
	}
	if results[1].YearsAgo != 3 {
		t.Errorf("second YearsAgo = %d, want 3", results[1].YearsAgo)
	}
}

func TestOnThisDayEmptyRepo(t *testing.T) {
	s := storage.NewWithRoot(t.TempDir())
	today, _ := time.Parse("2006-01-02", "2026-06-25")
	results := OnThisDay(s, today, 5)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestFirstMeaningfulLine(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"normal", "# title\n\n## 09:00\n\nhello world\n", "hello world"},
		{"multi_timestamp", "# t\n\n## 09:00\n\nfirst\n\n## 22:00\n\nsecond\n", "first"},
		{"only_title", "# title\n", ""},
		{"only_template", "# t\n\n## 09:00\n\n", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstMeaningfulLine(c.input); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
