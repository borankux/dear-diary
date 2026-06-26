package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHTMLWriterLoadDiariesExcludesGeneratedMarkdown(t *testing.T) {
	root := t.TempDir()
	diaryPath := filepath.Join(root, "2026-06", "2026-06-25.md")
	generatedPath := filepath.Join(root, "process", "todos.md")
	for _, path := range []string{diaryPath, generatedPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# hello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writer := NewHTMLWriter(t.TempDir(), root)
	entries, err := writer.loadDiaries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 diary entry, got %d", len(entries))
	}
	if entries[0].RawPath != diaryPath {
		t.Fatalf("expected %s, got %s", diaryPath, entries[0].RawPath)
	}
	if entries[0].URL != "entries/2026-06-25.html" {
		t.Fatalf("unexpected diary page URL: %q", entries[0].URL)
	}
}

func TestSummarizeDiaryExtractsReadableMetadata(t *testing.T) {
	title, body, sections, excerpt := summarizeDiary([]byte(`# 2026-06-26 周五

## 09:00

今天把 dashboard 从数据 dump 改成阅读报告。

## 13:30

- 保留重点
- 收起历史
`))

	if title != "2026-06-26 周五" {
		t.Fatalf("unexpected title: %q", title)
	}
	if strings.Contains(string(body), "# 2026-06-26") {
		t.Fatalf("body should not include the top-level diary title: %q", string(body))
	}
	if len(sections) != 2 || sections[0] != "09:00" || sections[1] != "13:30" {
		t.Fatalf("unexpected sections: %#v", sections)
	}
	if !strings.Contains(excerpt, "dashboard") || strings.Contains(excerpt, "##") {
		t.Fatalf("unexpected excerpt: %q", excerpt)
	}
}

func TestHTMLWriterCapsReadOnlyDashboardLists(t *testing.T) {
	root := t.TempDir()
	outDir := t.TempDir()
	store := newTestStore(t)

	for i := 0; i < maxDashboardTodos+3; i++ {
		if err := store.InsertTodo("todo item", "/a/2026-06/2026-06-26.md"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < maxDashboardMemories+2; i++ {
		if err := store.InsertMemory("memory topic", "memory summary", "/a/2026-06/2026-06-26.md"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < maxDashboardCandidates+1; i++ {
		inserted, err := store.InsertCandidateIfNew(Candidate{
			Type:       CandidateTypeTodo,
			Title:      "candidate item",
			Content:    "candidate content",
			SourceFile: "/a/2026-06/2026-06-26.md",
			SourceHash: "hash-" + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !inserted {
			t.Fatalf("candidate %d was not inserted", i)
		}
	}

	for _, date := range []string{"2026-06-26", "2026-06-25", "2026-05-10"} {
		path := filepath.Join(root, date[:7], date+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("# "+date+" 周五\n\n## 09:00\n\n正文\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writer := NewHTMLWriter(outDir, root)
	if err := writer.WriteAll(store); err != nil {
		t.Fatal(err)
	}
	html := mustRead(t, writer.DashboardPath())

	for _, want := range []string{
		"Read-only dashboard",
		"日历入口",
		"最近日记",
		"只显示最近 12 个，还有 3 个未显示。",
		"还有 1 条候选没有显示。",
		"还有 2 条记忆没有显示。",
		"2026 年 6 月",
		`href="entries/2026-06-26.html"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, `<input type="checkbox">`) {
		t.Fatal("read-only dashboard should not render fake checkboxes")
	}

	dayPage := mustRead(t, filepath.Join(outDir, "entries", "2026-06-26.html"))
	for _, want := range []string{
		"返回 Dashboard",
		"2026-06-26 周五",
		`<h2 id="09-00">09:00</h2>`,
		"正文",
	} {
		if !strings.Contains(dayPage, want) {
			t.Fatalf("diary page missing %q\n%s", want, dayPage)
		}
	}
}

func TestBuildCalendarMonthsUsesMondayStartAndDiaryLinks(t *testing.T) {
	months := buildCalendarMonths([]DiaryEntry{
		{Date: "2026-05-10", URL: "entries/2026-05-10.html"},
		{Date: "2026-06-26", URL: "entries/2026-06-26.html"},
	}, time.Date(2026, 6, 26, 12, 0, 0, 0, time.Local))

	if len(months) != 2 {
		t.Fatalf("expected 2 months, got %d", len(months))
	}
	if months[0].Month != "2026-06" || !months[0].Open {
		t.Fatalf("unexpected latest month: %+v", months[0])
	}
	var june26 CalendarDay
	for _, day := range months[0].Days {
		if day.Date == "2026-06-26" {
			june26 = day
			break
		}
	}
	if !june26.IsWritten || !june26.IsToday || june26.URL != "entries/2026-06-26.html" {
		t.Fatalf("bad June 26 calendar day: %+v", june26)
	}
	if months[1].Month != "2026-05" || len(months[1].Days) == 0 || !months[1].Days[0].IsPadding {
		t.Fatalf("May 2026 should include Monday-first padding cells: %+v", months[1])
	}
}
