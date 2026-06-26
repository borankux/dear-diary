package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathFor(t *testing.T) {
	s := NewWithRoot("/tmp/diary-test")
	cases := []struct {
		in   string
		want string
	}{
		{"2026-06-25", "/tmp/diary-test/2026-06/2026-06-25.md"},
		{"2026-12-31", "/tmp/diary-test/2026-12/2026-12-31.md"},
		{"2026-01-01", "/tmp/diary-test/2026-01/2026-01-01.md"},
	}
	for _, c := range cases {
		tt, _ := time.Parse("2006-01-02", c.in)
		got := s.PathFor(tt)
		if got != c.want {
			t.Errorf("PathFor(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestEnsureFileCreates(t *testing.T) {
	dir := t.TempDir()
	s := NewWithRoot(dir)
	t1 := time.Date(2026, 6, 25, 11, 30, 0, 0, time.Local)
	path := s.PathFor(t1)

	isNew, err := s.EnsureFile(path, t1)
	if err != nil {
		t.Fatalf("EnsureFile: %v", err)
	}
	if !isNew {
		t.Error("expected isNew=true for first creation")
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "# 2026-06-25") {
		t.Errorf("missing title in:\n%s", content)
	}
	if !strings.Contains(content, "周四") {
		t.Errorf("missing weekday 周四 in:\n%s", content)
	}
	if !strings.Contains(content, "## 11:30") {
		t.Errorf("missing initial timestamp in:\n%s", content)
	}
}

func TestEnsureFileIdempotent(t *testing.T) {
	dir := t.TempDir()
	s := NewWithRoot(dir)
	t1 := time.Date(2026, 6, 25, 11, 30, 0, 0, time.Local)
	path := s.PathFor(t1)

	if _, err := s.EnsureFile(path, t1); err != nil {
		t.Fatal(err)
	}
	isNew, err := s.EnsureFile(path, t1)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Error("expected isNew=false on second call")
	}
}

func TestAppendTimestamp(t *testing.T) {
	dir := t.TempDir()
	s := NewWithRoot(dir)
	t1 := time.Date(2026, 6, 25, 8, 0, 0, 0, time.Local)
	t2 := time.Date(2026, 6, 25, 22, 30, 0, 0, time.Local)
	path := s.PathFor(t1)

	if _, err := s.EnsureFile(path, t1); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendTimestamp(path, t2); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	content := string(data)
	if strings.Count(content, "## ") != 2 {
		t.Errorf("expected 2 timestamp sections, got:\n%s", content)
	}
	if !strings.Contains(content, "## 22:30") {
		t.Errorf("missing appended timestamp:\n%s", content)
	}
}

func TestWrittenDaysInMonth(t *testing.T) {
	dir := t.TempDir()
	s := NewWithRoot(dir)

	// 创建 3 个不同日期的文件
	for _, day := range []int{1, 15, 30} {
		t1 := time.Date(2026, 6, day, 10, 0, 0, 0, time.Local)
		path := s.PathFor(t1)
		if _, err := s.EnsureFile(path, t1); err != nil {
			t.Fatal(err)
		}
	}

	days := s.WrittenDaysInMonth(2026, 6)
	if len(days) != 3 {
		t.Errorf("expected 3 days, got %d: %v", len(days), days)
	}
	if !days[1] || !days[15] || !days[30] {
		t.Errorf("expected days {1,15,30}, got %v", days)
	}

	// 不存在的月返回空
	empty := s.WrittenDaysInMonth(2025, 1)
	if len(empty) != 0 {
		t.Errorf("expected empty for missing month, got %v", empty)
	}
}

func TestAllMarkdownFilesSortedByPath(t *testing.T) {
	dir := t.TempDir()
	s := NewWithRoot(dir)

	for _, date := range []string{"2026-06-25", "2026-05-10", "2026-06-01"} {
		t1, _ := time.Parse("2006-01-02", date)
		path := s.PathFor(t1)
		if _, err := s.EnsureFile(path, t1); err != nil {
			t.Fatal(err)
		}
	}
	files, err := s.AllMarkdownFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("got %d files: %v", len(files), files)
	}
	// 字典序: 2026-05/... < 2026-06/2026-06-01.md < 2026-06/2026-06-25.md
	want0 := filepath.Join(dir, "2026-05", "2026-05-10.md")
	if files[0] != want0 {
		t.Errorf("files[0] = %s, want %s", files[0], want0)
	}
}

func TestDiaryFilePathFilter(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join("2026-06", "2026-06-25.md"), true},
		{filepath.Join("2026-06", "2026-06-25.txt"), false},
		{filepath.Join("2026-06", "notes.md"), false},
		{filepath.Join("process", "todos.md"), false},
		{filepath.Join("2026-05", "2026-06-25.md"), false},
	}
	for _, tc := range cases {
		if got := IsDiaryFilePath(tc.path); got != tc.want {
			t.Fatalf("IsDiaryFilePath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
