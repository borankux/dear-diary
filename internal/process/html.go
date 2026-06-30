package process

import (
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/borankux/dear-diary/internal/storage"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

const (
	maxDashboardMemories = 8
	maxDashboardDiaries  = 7
	diaryPagesDir        = "entries"
)

// HTMLWriter renders a compact, single-viewport dashboard to HTML.
type HTMLWriter struct {
	outDir  string
	rootDir string
}

// NewHTMLWriter creates a writer that outputs to the given directory.
func NewHTMLWriter(outDir, rootDir string) *HTMLWriter {
	return &HTMLWriter{outDir: outDir, rootDir: rootDir}
}

// DiaryEntry holds a rendered diary for the dashboard.
type DiaryEntry struct {
	Date     string
	Title    string
	Excerpt  string
	Sections []string
	HTML     template.HTML
	RawPath  string
	Month    string
	URL      string
	Open     bool
}

// TodayStatus summarizes today's open loop for the dashboard.
type TodayStatus struct {
	Date     string
	Exists   bool
	Sections int
}

// CalendarMonth is a read-only month view for diary navigation.
type CalendarMonth struct {
	Month       string
	Title       string
	Count       int
	DaysInMonth int
	Open        bool
	Days        []CalendarDay
}

// CalendarDay represents one visible cell in a Monday-first month grid.
type CalendarDay struct {
	Day       int
	Date      string
	URL       string
	IsPadding bool
	IsWritten bool
	IsToday   bool
}

// TodoBoardColumn is a read-only lane in the todo lifecycle board.
type TodoBoardColumn struct {
	Key         string
	Title       string
	Description string
	Count       int
	EmptyText   string
	Items       []TodoBoardItem
}

// TodoBoardItem is a compact card for a candidate or todo.
type TodoBoardItem struct {
	ID          int
	Eyebrow     string
	Title       string
	Body        string
	HasPriority bool
	Priority    int
	SourceDate  string
	SourceFile  string
	SourceURL   string
}

type dashboardData struct {
	GeneratedAt      time.Time
	Today            TodayStatus
	CandidateCount   int
	TodoStatusCounts TodoCounts
	MemoryCount      int
	DiaryCount       int
	MemoryOverflow   int
	DiaryOverflow    int
	Memories         []Memory
	Diaries          []DiaryEntry
	CalendarMonths   []CalendarMonth
	TodoBoardColumns []TodoBoardColumn
}

// WriteAll regenerates dashboard.html from the store and diary files.
func (w *HTMLWriter) WriteAll(store *Store) error {
	if err := os.MkdirAll(w.outDir, 0o755); err != nil {
		return err
	}

	todos, err := store.ListActiveTodos()
	if err != nil {
		return err
	}
	inProgressTodos, err := store.ListTodosByStatus(TodoStatusInProgress)
	if err != nil {
		return err
	}
	doneTodos, err := store.ListTodosByStatus("done")
	if err != nil {
		return err
	}
	wontDoTodos, err := store.ListTodosByStatus(TodoStatusWontDo)
	if err != nil {
		return err
	}
	archivedTodos, err := store.ListTodosByStatus("archived")
	if err != nil {
		return err
	}
	otherTodos, err := store.ListTodosByStatus(TodoStatusOther)
	if err != nil {
		return err
	}
	todoCounts, err := store.TodoStatusCounts()
	if err != nil {
		return err
	}
	memories, err := store.ListMemories()
	if err != nil {
		return err
	}
	candidates, err := store.ListPendingCandidates()
	if err != nil {
		return err
	}
	diaries, err := w.loadDiaries()
	if err != nil {
		return err
	}
	if err := w.writeDiaryPages(diaries); err != nil {
		return err
	}

	visibleMemories, memoryOverflow := limitSlice(memories, maxDashboardMemories)
	visibleDiaries, diaryOverflow := limitSlice(diaries, maxDashboardDiaries)
	sourceURLs := diarySourceURLs(diaries)

	data := dashboardData{
		GeneratedAt:      time.Now(),
		Today:            w.todayStatus(),
		CandidateCount:   len(candidates),
		TodoStatusCounts: todoCounts,
		MemoryCount:      len(memories),
		DiaryCount:       len(diaries),
		MemoryOverflow:   memoryOverflow,
		DiaryOverflow:    diaryOverflow,
		Memories:         visibleMemories,
		Diaries:          visibleDiaries,
		CalendarMonths:   buildCalendarMonths(diaries, time.Now()),
		TodoBoardColumns: buildTodoBoardColumns(candidates, todos, inProgressTodos, doneTodos, wontDoTodos, archivedTodos, otherTodos, sourceURLs),
	}

	path := filepath.Join(w.outDir, "dashboard.html")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	tmpl, err := template.New("dashboard").Parse(dashboardTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(f, data)
}

func (w *HTMLWriter) loadDiaries() ([]DiaryEntry, error) {
	if w.rootDir == "" {
		return nil, nil
	}
	files, err := filepath.Glob(filepath.Join(w.rootDir, "*", "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	// Most recent first.
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}

	var entries []DiaryEntry
	for _, path := range files {
		if !storage.IsDiaryFilePath(path) {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		title, body, sections, excerpt := summarizeDiary(b)
		entries = append(entries, DiaryEntry{
			Date:     name,
			Title:    title,
			Excerpt:  excerpt,
			Sections: sections,
			HTML:     renderMarkdown(body),
			RawPath:  path,
			Month:    filepath.Base(filepath.Dir(path)),
			URL:      filepath.ToSlash(filepath.Join(diaryPagesDir, name+".html")),
		})
	}
	if len(entries) > 0 {
		entries[0].Open = true
	}
	return entries, nil
}

func (w *HTMLWriter) writeDiaryPages(entries []DiaryEntry) error {
	dir := filepath.Join(w.outDir, diaryPagesDir)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpl, err := template.New("diary-page").Parse(diaryPageTemplate)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Date+".html")
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		data := struct {
			GeneratedAt time.Time
			Entry       DiaryEntry
		}{
			GeneratedAt: time.Now(),
			Entry:       entry,
		}
		execErr := tmpl.Execute(f, data)
		closeErr := f.Close()
		if execErr != nil {
			return execErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func (w *HTMLWriter) todayStatus() TodayStatus {
	now := time.Now()
	status := TodayStatus{Date: now.Format("2006-01-02")}
	if w.rootDir == "" {
		return status
	}
	path := storage.NewWithRoot(w.rootDir).PathFor(now)
	b, err := os.ReadFile(path)
	if err != nil {
		return status
	}
	status.Exists = true
	status.Sections = strings.Count(string(b), "\n## ")
	if strings.HasPrefix(string(b), "## ") {
		status.Sections++
	}
	return status
}

func limitSlice[T any](items []T, max int) ([]T, int) {
	if len(items) <= max {
		return items, 0
	}
	return items[:max], len(items) - max
}

func buildTodoBoardColumns(candidates []Candidate, activeTodos, inProgressTodos, doneTodos, wontDoTodos, archivedTodos, otherTodos []Todo, sourceURLs map[string]string) []TodoBoardColumn {
	inboxItems := candidateBoardItems(candidates, sourceURLs)
	activeItems := todoBoardItems(activeTodos, "todo", sourceURLs)
	inProgressItems := todoBoardItems(inProgressTodos, "doing", sourceURLs)
	doneItems := todoBoardItems(doneTodos, "done", sourceURLs)
	wontDoItems := todoBoardItems(wontDoTodos, "wont do", sourceURLs)
	archivedItems := todoBoardItems(archivedTodos, "archived", sourceURLs)
	otherItems := todoBoardItems(otherTodos, "other", sourceURLs)

	return []TodoBoardColumn{
		{
			Key:         "inbox",
			Title:       "AI Inbox",
			Description: "AI 提取出的建议，只在你提升后才进入可信 todo/memory。",
			Count:       len(inboxItems),
			EmptyText:   "没有待提升候选。",
			Items:       inboxItems,
		},
		{
			Key:         "active",
			Title:       "Active",
			Description: "已经提升，尚未开始或未分类的真实 todo。",
			Count:       len(activeItems),
			EmptyText:   "没有 active todo。",
			Items:       activeItems,
		},
		{
			Key:         "in-progress",
			Title:       "In Progress",
			Description: "正在做，应该优先保持可见。",
			Count:       len(inProgressItems),
			EmptyText:   "没有正在做的 todo。",
			Items:       inProgressItems,
		},
		{
			Key:         "done",
			Title:       "Done",
			Description: "最近完成的 todo，用来看到闭环。",
			Count:       len(doneItems),
			EmptyText:   "还没有完成记录。",
			Items:       doneItems,
		},
		{
			Key:         "wont-do",
			Title:       "Won't Do",
			Description: "明确不打算做，避免继续占注意力。",
			Count:       len(wontDoItems),
			EmptyText:   "没有不打算做的 todo。",
			Items:       wontDoItems,
		},
		{
			Key:         "archived",
			Title:       "Archived",
			Description: "已收起但不算完成的 todo。",
			Count:       len(archivedItems),
			EmptyText:   "没有归档 todo。",
			Items:       archivedItems,
		},
		{
			Key:         "other",
			Title:       "Other",
			Description: "AI 或人工暂时无法归类的 todo。",
			Count:       len(otherItems),
			EmptyText:   "没有 other todo。",
			Items:       otherItems,
		},
	}
}

func candidateBoardItems(candidates []Candidate, sourceURLs map[string]string) []TodoBoardItem {
	items := make([]TodoBoardItem, 0, len(candidates))
	for _, c := range candidates {
		title := firstText(c.Title, c.Content)
		body := c.EvidenceText
		if body == "" && c.Title != "" {
			body = c.Content
		}
		sourceDate := sourceDateFor(c.SourceDate, c.SourceFile)
		items = append(items, TodoBoardItem{
			ID:         c.ID,
			Eyebrow:    strings.ToUpper(c.Type),
			Title:      truncateText(title, 110),
			Body:       truncateText(body, 150),
			SourceDate: sourceDate,
			SourceFile: sourceFileLabel(c.SourceFile),
			SourceURL:  sourceURLs[sourceDate],
		})
	}
	return items
}

func todoBoardItems(todos []Todo, label string, sourceURLs map[string]string) []TodoBoardItem {
	items := make([]TodoBoardItem, 0, len(todos))
	for _, t := range todos {
		sourceDate := sourceDateFor(t.SourceDate, t.SourceFile)
		items = append(items, TodoBoardItem{
			ID:          t.ID,
			Eyebrow:     strings.ToUpper(label),
			Title:       truncateText(t.Text, 120),
			Body:        truncateText(t.EvidenceText, 150),
			HasPriority: t.HasPriority,
			Priority:    t.Priority,
			SourceDate:  sourceDate,
			SourceFile:  sourceFileLabel(t.SourceFile),
			SourceURL:   sourceURLs[sourceDate],
		})
	}
	return items
}

func diarySourceURLs(diaries []DiaryEntry) map[string]string {
	sourceURLs := make(map[string]string, len(diaries))
	for _, entry := range diaries {
		sourceURLs[entry.Date] = entry.URL
	}
	return sourceURLs
}

func sourceDateFor(sourceDate, sourceFile string) string {
	if sourceDate != "" {
		return sourceDate
	}
	name := strings.TrimSuffix(filepath.Base(sourceFile), ".md")
	if _, err := time.Parse("2006-01-02", name); err == nil {
		return name
	}
	return ""
}

func sourceFileLabel(sourceFile string) string {
	if sourceFile == "" {
		return ""
	}
	return filepath.Base(sourceFile)
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func summarizeDiary(src []byte) (string, []byte, []string, string) {
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	title := ""
	bodyStart := 0
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# ") {
		title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "# "))
		bodyStart = 1
	}
	if title == "" {
		title = "Untitled diary"
	}

	var sections []string
	var plainParts []string
	for _, line := range lines[bodyStart:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			sections = append(sections, strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		if trimmed != "" {
			plainParts = append(plainParts, trimmed)
		}
	}

	body := strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	if body == "" {
		body = text
	}
	excerpt := truncateText(strings.Join(plainParts, " "), 150)
	return title, []byte(body), sections, excerpt
}

func truncateText(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func buildCalendarMonths(diaries []DiaryEntry, now time.Time) []CalendarMonth {
	written := make(map[string]DiaryEntry)
	monthStarts := make(map[string]time.Time)
	for _, entry := range diaries {
		d, err := time.Parse("2006-01-02", entry.Date)
		if err != nil {
			continue
		}
		written[entry.Date] = entry
		monthStarts[d.Format("2006-01")] = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
	}
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	monthStarts[currentMonth.Format("2006-01")] = currentMonth

	months := make([]string, 0, len(monthStarts))
	for month := range monthStarts {
		months = append(months, month)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))

	calendarMonths := make([]CalendarMonth, 0, len(months))
	for i, month := range months {
		start := monthStarts[month]
		daysInMonth := time.Date(start.Year(), start.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
		firstWeekday := (int(start.Weekday()) + 6) % 7
		days := make([]CalendarDay, 0, firstWeekday+daysInMonth)
		for p := 0; p < firstWeekday; p++ {
			days = append(days, CalendarDay{IsPadding: true})
		}
		count := 0
		for day := 1; day <= daysInMonth; day++ {
			date := time.Date(start.Year(), start.Month(), day, 0, 0, 0, 0, time.Local)
			dateString := date.Format("2006-01-02")
			entry, ok := written[dateString]
			if ok {
				count++
			}
			days = append(days, CalendarDay{
				Day:       day,
				Date:      dateString,
				URL:       entry.URL,
				IsWritten: ok,
				IsToday:   sameCalendarDay(date, now),
			})
		}
		calendarMonths = append(calendarMonths, CalendarMonth{
			Month:       month,
			Title:       fmt.Sprintf("%d 年 %d 月", start.Year(), int(start.Month())),
			Count:       count,
			DaysInMonth: daysInMonth,
			Open:        i == 0,
			Days:        days,
		})
	}
	return calendarMonths
}

func sameCalendarDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func renderMarkdown(src []byte) template.HTML {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(src)
	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank | html.SkipHTML}
	renderer := html.NewRenderer(opts)
	return template.HTML(markdown.Render(doc, renderer))
}

// DashboardPath returns the full path to dashboard.html.
func (w *HTMLWriter) DashboardPath() string {
	return filepath.Join(w.outDir, "dashboard.html")
}

// OpenDashboard opens the dashboard in the default browser.
func (w *HTMLWriter) OpenDashboard() error {
	return openURL("file://" + w.DashboardPath())
}

// RegenerateAndOpenDashboard reads the current store and opens the dashboard.
// It does not run AI extraction.
func RegenerateAndOpenDashboard() error {
	store, err := NewStore("")
	if err != nil {
		return err
	}
	defer store.Close()

	rootDir := storage.New().RootDir()
	writer := NewHTMLWriter(ProcessOutDir(), rootDir)
	if err := writer.WriteAll(store); err != nil {
		return err
	}
	return writer.OpenDashboard()
}

func openURL(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	return exec.Command(cmd, args...).Start()
}

const dashboardTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Dear Diary Dashboard</title>
	<style>
		:root {
			color-scheme: light;
			--bg: #eef2f1;
			--paper: #ffffff;
			--paper-soft: #f8faf9;
			--ink: #17201d;
			--text: #34413d;
			--muted: #6b7672;
			--quiet: #e7ecea;
			--line: #d9e0dd;
			--line-strong: #b9c5c0;
			--accent: #0f766e;
			--active: #2563eb;
			--progress: #0f766e;
			--done: #15803d;
			--wont: #b91c1c;
			--archived: #64748b;
			--other: #525252;
			--attention: #b45309;
			--memory: #7c3f22;
			--radius: 8px;
			--shadow: 0 16px 42px rgba(23, 32, 29, 0.08);
			--hairline: 1px solid var(--line);
		}
		* { box-sizing: border-box; }
		html, body {
			margin: 0;
			padding: 0;
		}
		body {
			font-family: "Avenir Next", "Hiragino Sans", "PingFang SC", "Helvetica Neue", Arial, sans-serif;
			background:
				linear-gradient(180deg, rgba(255, 255, 255, 0.72), rgba(255, 255, 255, 0) 360px),
				repeating-linear-gradient(90deg, rgba(23, 32, 29, 0.035) 0, rgba(23, 32, 29, 0.035) 1px, transparent 1px, transparent 96px),
				var(--bg);
			color: var(--ink);
			-webkit-font-smoothing: antialiased;
			-moz-osx-font-smoothing: grayscale;
			text-wrap: pretty;
		}
		a {
			color: inherit;
		}
		.app {
			width: min(1480px, calc(100% - 36px));
			margin: 0 auto;
			padding: 30px 0 56px;
		}
		header {
			display: flex;
			align-items: flex-end;
			justify-content: space-between;
			gap: 24px;
			padding-bottom: 18px;
			border-bottom: var(--hairline);
		}
		header h1 {
			margin: 0;
			font-family: "Iowan Old Style", "Songti SC", Georgia, serif;
			font-size: clamp(2rem, 4vw, 4.2rem);
			line-height: 0.92;
			font-weight: 700;
			letter-spacing: 0;
		}
		.kicker,
		header .meta,
		.overflow-note,
		.source,
		.entry-meta,
		.month-count,
		.board-description,
		.card-meta {
			color: var(--muted);
			font-size: 0.78rem;
			font-variant-numeric: tabular-nums;
		}
		.kicker {
			margin-bottom: 8px;
			font-size: 0.72rem;
			font-weight: 800;
			letter-spacing: 0;
			text-transform: uppercase;
		}
		.briefing {
			display: grid;
			grid-template-columns: minmax(0, 1fr) minmax(360px, 540px);
			gap: 28px;
			padding: 26px 0 30px;
			border-bottom: var(--hairline);
			align-items: end;
		}
		.briefing h2 {
			max-width: 980px;
			margin: 0;
			font-size: clamp(1.65rem, 2.4vw, 3rem);
			line-height: 1.06;
			font-weight: 800;
			letter-spacing: 0;
			overflow-wrap: anywhere;
		}
		.briefing p {
			max-width: 760px;
			margin: 12px 0 0;
			color: var(--text);
			font-size: 1rem;
			line-height: 1.58;
			overflow-wrap: anywhere;
		}
		.metric-row {
			display: grid;
			grid-template-columns: repeat(3, minmax(0, 1fr));
			gap: 10px;
		}
		.metric {
			background: rgba(255, 255, 255, 0.82);
			border: var(--hairline);
			border-radius: var(--radius);
			padding: 14px 15px;
			box-shadow: 0 8px 28px rgba(23, 32, 29, 0.06);
			min-width: 0;
		}
		.metric strong {
			display: block;
			font-size: 1.55rem;
			line-height: 1;
			font-variant-numeric: tabular-nums;
		}
		.metric span {
			display: block;
			margin-top: 6px;
			color: var(--muted);
			font-size: 0.78rem;
		}
		.section {
			min-width: 0;
			padding-top: 24px;
			border-top: var(--hairline);
		}
		.section + .section {
			margin-top: 34px;
		}
		.section-header {
			display: flex;
			align-items: baseline;
			justify-content: space-between;
			gap: 16px;
			margin-bottom: 14px;
			min-width: 0;
		}
		.section h2 {
			margin: 0;
			font-size: 1.02rem;
			font-weight: 900;
			letter-spacing: 0;
		}
		.todo-board {
			border-top: 0;
			padding-top: 24px;
		}
		.board-grid {
			display: grid;
			grid-template-columns: repeat(7, minmax(260px, 1fr));
			gap: 16px;
			align-items: start;
			min-width: 0;
			overflow-x: auto;
			padding-bottom: 12px;
		}
		.board-column {
			min-width: 0;
			padding-top: 10px;
			border-top: 4px solid var(--line-strong);
		}
		.board-inbox { border-top-color: var(--attention); }
		.board-active { border-top-color: var(--active); }
		.board-in-progress { border-top-color: var(--progress); }
		.board-done { border-top-color: var(--done); }
		.board-wont-do { border-top-color: var(--wont); }
		.board-archived { border-top-color: var(--archived); }
		.board-other { border-top-color: var(--other); }
		.board-column header {
			display: grid;
			grid-template-columns: minmax(0, 1fr) auto;
			gap: 10px;
			align-items: start;
			padding: 0 0 10px;
			border: 0;
		}
		.board-column h3 {
			margin: 0;
			font-size: 0.95rem;
			line-height: 1.2;
			font-weight: 900;
			letter-spacing: 0;
		}
		.board-count {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			min-width: 32px;
			height: 26px;
			padding: 0 8px;
			border-radius: 999px;
			background: var(--ink);
			color: #fff;
			font-size: 0.76rem;
			font-weight: 900;
			font-variant-numeric: tabular-nums;
		}
		.board-description {
			margin: 4px 0 0;
			line-height: 1.35;
		}
		.board-list {
			display: grid;
			gap: 10px;
			min-width: 0;
		}
		.board-card,
		.memory-card,
		.diary-entry,
		.calendar-month {
			background: var(--paper);
			border: var(--hairline);
			border-radius: var(--radius);
			box-shadow: var(--shadow);
			min-width: 0;
			overflow: hidden;
		}
		.board-card {
			padding: 12px;
			box-shadow: 0 10px 28px rgba(23, 32, 29, 0.07);
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.card-meta {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: 8px;
			margin-bottom: 8px;
		}
		.card-right {
			display: inline-flex;
			align-items: center;
			gap: 6px;
			flex: 0 0 auto;
		}
		.card-type {
			font-size: 0.68rem;
			font-weight: 900;
			color: var(--muted);
			text-transform: uppercase;
			letter-spacing: 0;
		}
		.card-id {
			font-weight: 900;
			color: var(--ink);
		}
		.priority-badge {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			min-width: 30px;
			height: 20px;
			padding: 0 7px;
			border-radius: 999px;
			background: #fff4d6;
			color: #8a4b00;
			border: 1px solid #e8c36c;
			font-size: 0.68rem;
			font-weight: 950;
			font-variant-numeric: tabular-nums;
		}
		.board-card strong {
			display: block;
			font-size: 0.88rem;
			line-height: 1.38;
			font-weight: 850;
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.board-card p {
			margin: 7px 0 0;
			color: var(--text);
			font-size: 0.82rem;
			line-height: 1.45;
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.source {
			margin-top: 9px;
			line-height: 1.35;
			overflow-wrap: anywhere;
		}
		.source a,
		.entry-link {
			color: var(--accent);
			text-decoration: none;
			font-weight: 850;
		}
		.source a:hover,
		.entry-link:hover {
			text-decoration: underline;
		}
		.empty {
			border: 1px dashed var(--line-strong);
			border-radius: var(--radius);
			color: var(--muted);
			font-size: 0.86rem;
			padding: 16px;
			background: rgba(255, 255, 255, 0.58);
		}
		.content-grid {
			display: grid;
			grid-template-columns: minmax(0, 1fr) 380px;
			gap: 28px;
			margin-top: 30px;
			align-items: start;
			min-width: 0;
		}
		main,
		aside,
		.diary-entry {
			min-width: 0;
		}
		.calendar-months {
			display: grid;
			gap: 12px;
		}
		.calendar-month summary {
			list-style: none;
			cursor: pointer;
			display: flex;
			align-items: baseline;
			justify-content: space-between;
			gap: 12px;
			padding: 13px 14px;
			border-bottom: var(--hairline);
		}
		.calendar-month summary::-webkit-details-marker { display: none; }
		.calendar-month strong {
			font-size: 0.92rem;
			font-variant-numeric: tabular-nums;
		}
		.calendar-weekdays,
		.calendar-grid {
			display: grid;
			grid-template-columns: repeat(7, minmax(0, 1fr));
			gap: 4px;
		}
		.calendar-weekdays {
			padding: 12px 14px 0;
			color: var(--muted);
			font-size: 0.72rem;
			font-weight: 850;
			text-align: center;
		}
		.calendar-grid {
			padding: 8px 14px 14px;
		}
		.calendar-cell {
			min-width: 0;
			min-height: 46px;
			border: var(--hairline);
			border-radius: 7px;
			padding: 6px;
			background: var(--paper-soft);
			color: var(--muted);
			font-size: 0.78rem;
			font-variant-numeric: tabular-nums;
			text-decoration: none;
		}
		.calendar-cell.is-padding {
			visibility: hidden;
		}
		.calendar-day {
			display: flex;
			flex-direction: column;
			gap: 5px;
			justify-content: space-between;
		}
		.calendar-day.is-written {
			background: #e5f5ef;
			border-color: #a8d8c5;
			color: var(--accent);
			font-weight: 850;
		}
		.calendar-day.is-today {
			border-color: var(--attention);
			color: var(--attention);
		}
		.calendar-day small {
			color: inherit;
			font-size: 0.65rem;
			font-weight: 800;
			white-space: nowrap;
		}
		.diary-entry {
			margin-bottom: 14px;
		}
		.diary-entry summary {
			list-style: none;
			cursor: pointer;
			padding: 18px 20px;
			display: grid;
			grid-template-columns: 1fr auto;
			gap: 16px;
			align-items: start;
			min-width: 0;
		}
		.diary-entry summary > div {
			min-width: 0;
		}
		.diary-entry summary::-webkit-details-marker { display: none; }
		.entry-title {
			margin: 0;
			font-size: 1.08rem;
			line-height: 1.25;
			font-weight: 900;
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.entry-excerpt {
			margin: 7px 0 0;
			color: var(--muted);
			font-size: 0.88rem;
			line-height: 1.45;
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.section-pills {
			display: flex;
			flex-wrap: wrap;
			gap: 6px;
			padding: 0 20px 16px;
			min-width: 0;
		}
		.pill {
			display: inline-flex;
			align-items: center;
			min-height: 24px;
			max-width: 100%;
			padding: 3px 8px;
			border-radius: 999px;
			background: var(--quiet);
			color: var(--muted);
			font-size: 0.72rem;
			font-variant-numeric: tabular-nums;
		}
		.markdown {
			border-top: var(--hairline);
			padding: 22px 20px 28px;
			font-family: "Iowan Old Style", "Songti SC", Georgia, serif;
			font-size: 1.04rem;
			line-height: 1.72;
			color: var(--text);
			max-width: 820px;
			min-width: 0;
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.markdown h1 { display: none; }
		.markdown h2,
		.markdown h3 {
			font-family: "Avenir Next", "Hiragino Sans", "PingFang SC", "Helvetica Neue", Arial, sans-serif;
			font-size: 0.9rem;
			line-height: 1.3;
			margin: 1.6rem 0 0.65rem;
			padding-top: 0.9rem;
			border-top: var(--hairline);
			color: var(--ink);
			font-weight: 900;
		}
		.markdown h2:first-child,
		.markdown h3:first-child {
			margin-top: 0;
			padding-top: 0;
			border-top: 0;
		}
		.markdown p { margin: 0.75rem 0; }
		.markdown ul,
		.markdown ol {
			margin: 0.75rem 0;
			padding-left: 1.3rem;
		}
		.markdown li { margin: 0.35rem 0; }
		.markdown blockquote {
			margin: 1rem 0;
			padding: 0 0 0 1rem;
			border-left: 2px solid var(--line-strong);
			color: #4b5563;
		}
		.markdown code {
			background: var(--quiet);
			border-radius: 4px;
			padding: 0.1rem 0.28rem;
			font-family: "SF Mono", Menlo, Consolas, monospace;
			font-size: 0.88em;
		}
		.markdown pre {
			overflow-x: auto;
			background: #111827;
			color: #f8fafc;
			border-radius: var(--radius);
			padding: 14px;
		}
		.markdown pre code {
			background: transparent;
			color: inherit;
			padding: 0;
		}
		.markdown table {
			width: 100%;
			border-collapse: collapse;
			margin: 1rem 0;
			font-family: "Avenir Next", "Hiragino Sans", "PingFang SC", "Helvetica Neue", Arial, sans-serif;
			font-size: 0.88rem;
		}
		.markdown img {
			max-width: 100%;
			height: auto;
		}
		.markdown th,
		.markdown td {
			border-bottom: var(--hairline);
			padding: 8px 10px;
			text-align: left;
			vertical-align: top;
		}
		.memory-list {
			display: grid;
			gap: 10px;
		}
		.memory-card {
			padding: 13px;
			box-shadow: 0 10px 28px rgba(23, 32, 29, 0.06);
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.memory-card strong {
			display: block;
			color: var(--memory);
			font-size: 0.88rem;
			line-height: 1.35;
			font-weight: 900;
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		.memory-card p {
			margin: 7px 0 0;
			color: var(--text);
			font-size: 0.82rem;
			line-height: 1.45;
			overflow-wrap: anywhere;
			word-break: break-word;
		}
		@media (max-width: 1180px) {
			.briefing { grid-template-columns: 1fr; }
			.metric-row { grid-template-columns: repeat(3, minmax(0, 1fr)); }
			.board-grid {
				grid-template-columns: repeat(7, minmax(250px, 1fr));
			}
		}
		@media (max-width: 980px) {
			.app { width: calc(100% - 24px); max-width: 820px; padding-top: 22px; }
			header { align-items: flex-start; flex-direction: column; gap: 8px; }
			.metric-row { grid-template-columns: repeat(2, minmax(0, 1fr)); }
			.content-grid { grid-template-columns: 1fr; gap: 18px; }
		}
		@media (max-width: 640px) {
			.app { width: calc(100% - 20px); max-width: 820px; }
			.briefing h2 { font-size: 1.45rem; line-height: 1.18; }
			.metric-row,
			.board-grid { grid-template-columns: 1fr; }
			.section-header { align-items: flex-start; flex-direction: column; gap: 4px; }
			.calendar-month summary { align-items: flex-start; flex-direction: column; gap: 4px; }
			.calendar-grid { gap: 3px; padding-left: 10px; padding-right: 10px; }
			.calendar-weekdays { padding-left: 10px; padding-right: 10px; }
			.calendar-cell { min-height: 42px; padding: 5px; }
			.calendar-day small { display: none; }
			.diary-entry summary { grid-template-columns: 1fr; }
			.markdown { font-size: 1rem; padding: 18px 16px 22px; }
			.section-pills { padding-left: 16px; padding-right: 16px; }
		}
		@media (prefers-reduced-motion: reduce) {
			* { scroll-behavior: auto; }
		}
	</style>
</head>
<body>
	<div class="app">
		<header>
			<div>
				<div class="kicker">Read-only dashboard</div>
				<h1>Dear Diary</h1>
			</div>
			<div class="meta">Generated {{.GeneratedAt.Format "2006-01-02 15:04"}}</div>
		</header>

		<section class="briefing" aria-labelledby="briefing-title">
			<div>
				<div class="kicker">Daily closure</div>
				<h2 id="briefing-title">
					{{if .Today.Exists}}今天已写 {{.Today.Sections}} 段。{{else}}今天还没有写日记。{{end}}
					{{if .CandidateCount}}{{.CandidateCount}} 条候选还在 AI Inbox。{{else}}AI Inbox 已清空。{{end}}
				</h2>
				<p>
					{{.TodoStatusCounts.Active}} 个 active，{{.TodoStatusCounts.InProgress}} 个正在做，{{.TodoStatusCounts.Done}} 个已完成，{{.TodoStatusCounts.WontDo}} 个不打算做，{{.TodoStatusCounts.Archived}} 个已归档。
					日记共 {{.DiaryCount}} 篇，长期记忆 {{.MemoryCount}} 条。
				</p>
			</div>
			<div class="metric-row" aria-label="Dashboard counts">
				<div class="metric"><strong>{{.CandidateCount}}</strong><span>AI Inbox</span></div>
				<div class="metric"><strong>{{.TodoStatusCounts.Active}}</strong><span>Active todos</span></div>
				<div class="metric"><strong>{{.TodoStatusCounts.InProgress}}</strong><span>In progress</span></div>
				<div class="metric"><strong>{{.TodoStatusCounts.Done}}</strong><span>Done todos</span></div>
				<div class="metric"><strong>{{.TodoStatusCounts.WontDo}}</strong><span>Won't do</span></div>
				<div class="metric"><strong>{{.DiaryCount}}</strong><span>Diary entries</span></div>
			</div>
		</section>

		<section class="section todo-board" aria-labelledby="todo-board-title">
			<div class="section-header">
				<h2 id="todo-board-title">Todo Board</h2>
				<div class="meta">全部展示 · 有 priority 时按高到低排序</div>
			</div>
			<div class="board-grid">
				{{range .TodoBoardColumns}}
				<section class="board-column board-{{.Key}}" aria-labelledby="board-{{.Key}}">
					<header>
						<div>
							<h3 id="board-{{.Key}}">{{.Title}}</h3>
							<p class="board-description">{{.Description}}</p>
						</div>
						<span class="board-count">{{.Count}}</span>
					</header>
					{{if .Items}}
					<div class="board-list">
						{{range .Items}}
						<article class="board-card">
							<div class="card-meta">
								<span class="card-type">{{.Eyebrow}}</span>
								<span class="card-right">
									{{if .HasPriority}}<span class="priority-badge">P{{.Priority}}</span>{{end}}
									<span class="card-id">#{{.ID}}</span>
								</span>
							</div>
							<strong>{{.Title}}</strong>
							{{if .Body}}<p>{{.Body}}</p>{{end}}
							{{if .SourceDate}}
								<div class="source">{{if .SourceURL}}<a href="{{.SourceURL}}">{{.SourceDate}}</a>{{else}}{{.SourceDate}}{{end}}</div>
							{{else if .SourceFile}}
								<div class="source">{{.SourceFile}}</div>
							{{end}}
						</article>
						{{end}}
						</div>
						{{else}}
							<div class="empty">{{.EmptyText}}</div>
						{{end}}
				</section>
				{{end}}
			</div>
		</section>

		<div class="content-grid">
			<main>
				<section class="section" aria-labelledby="diary-title">
					<div class="section-header">
						<h2 id="diary-title">最近日记</h2>
						<div class="meta">最近 {{len .Diaries}} 篇{{if .DiaryOverflow}} · 还有 {{.DiaryOverflow}} 篇在日历里{{end}}</div>
					</div>
					{{if .Diaries}}
						{{range .Diaries}}
						<details class="diary-entry" {{if .Open}}open{{end}}>
							<summary>
								<div>
									<h3 class="entry-title">{{.Title}}</h3>
									{{if .Excerpt}}<p class="entry-excerpt">{{.Excerpt}}</p>{{end}}
								</div>
								<div class="entry-meta">{{.Date}} · {{len .Sections}} 段 · <a class="entry-link" href="{{.URL}}">日记页</a></div>
							</summary>
							{{if .Sections}}
							<div class="section-pills" aria-label="Diary sections">
								{{range .Sections}}<span class="pill">{{.}}</span>{{end}}
							</div>
							{{end}}
							<div class="markdown">{{.HTML}}</div>
						</details>
						{{end}}
					{{else}}
						<div class="empty">还没有找到 canonical diary 文件。</div>
					{{end}}
				</section>
			</main>

			<aside>
				<section class="section" aria-labelledby="calendar-title">
					<div class="section-header">
						<h2 id="calendar-title">日历入口</h2>
						<div class="meta">周一开头</div>
					</div>
					{{if .CalendarMonths}}
					<div class="calendar-months">
						{{range .CalendarMonths}}
						<details class="calendar-month" {{if .Open}}open{{end}}>
							<summary>
								<strong>{{.Title}}</strong>
								<span class="month-count">{{.Count}}/{{.DaysInMonth}} 天</span>
							</summary>
							<div class="calendar-weekdays" aria-hidden="true">
								<span>一</span><span>二</span><span>三</span><span>四</span><span>五</span><span>六</span><span>日</span>
							</div>
							<div class="calendar-grid">
								{{range .Days}}
									{{if .IsPadding}}
										<span class="calendar-cell is-padding"></span>
									{{else if .IsWritten}}
										<a class="calendar-cell calendar-day is-written{{if .IsToday}} is-today{{end}}" href="{{.URL}}" aria-label="打开 {{.Date}} 日记">
											<span>{{.Day}}</span>
											<small>{{if .IsToday}}◆ 今天{{else}}● 日记{{end}}</small>
										</a>
									{{else}}
										<span class="calendar-cell calendar-day{{if .IsToday}} is-today{{end}}" aria-label="{{.Date}} 没有日记">
											<span>{{.Day}}</span>
											<small>{{if .IsToday}}◆ 今天{{end}}</small>
										</span>
									{{end}}
								{{end}}
							</div>
						</details>
						{{end}}
					</div>
					{{else}}
						<div class="empty">暂无日历数据。</div>
					{{end}}
				</section>

				<section class="section" aria-labelledby="memory-title">
					<div class="section-header">
						<h2 id="memory-title">长期记忆</h2>
						<div class="meta">{{.MemoryCount}}</div>
					</div>
					{{if .Memories}}
					<div class="memory-list">
						{{range .Memories}}
						<article class="memory-card">
							<strong>{{.Topic}}</strong>
							<p>{{.Summary}}</p>
							{{if .SourceDate}}<div class="source">{{.SourceDate}}</div>{{else if .SourceFile}}<div class="source">{{.SourceFile}}</div>{{end}}
						</article>
						{{end}}
					</div>
					{{if .MemoryOverflow}}<p class="overflow-note">还有 {{.MemoryOverflow}} 条记忆没有显示。</p>{{end}}
					{{else}}
						<div class="empty">还没有长期记忆。</div>
					{{end}}
				</section>
			</aside>
		</div>
	</div>
</body>
</html>
`

const diaryPageTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>{{.Entry.Title}} · Dear Diary</title>
	<style>
		:root {
			color-scheme: light;
			--bg: #f6f7f8;
			--paper: #ffffff;
			--ink: #202124;
			--text: #31343a;
			--muted: #727780;
			--quiet: #eef0f3;
			--line: #dfe3e8;
			--line-strong: #c7cdd5;
			--accent: #0f766e;
			--radius: 8px;
			--shadow: 0 1px 2px rgba(30, 41, 59, 0.08);
		}
		* { box-sizing: border-box; }
		body {
			margin: 0;
			background: var(--bg);
			color: var(--ink);
			font-family: "Avenir Next", "SF Pro Text", "Helvetica Neue", Arial, sans-serif;
			-webkit-font-smoothing: antialiased;
			-moz-osx-font-smoothing: grayscale;
		}
		.page {
			width: min(860px, calc(100% - 32px));
			margin: 0 auto;
			padding: 28px 0 56px;
		}
		header {
			padding-bottom: 18px;
			border-bottom: 1px solid var(--line);
		}
		.back {
			color: var(--accent);
			font-size: 0.84rem;
			font-weight: 800;
			text-decoration: none;
		}
		.back:hover { text-decoration: underline; }
		h1 {
			margin: 14px 0 6px;
			font-size: clamp(1.6rem, 4vw, 2.4rem);
			line-height: 1.15;
			letter-spacing: 0;
		}
		.meta {
			color: var(--muted);
			font-size: 0.82rem;
			font-variant-numeric: tabular-nums;
		}
		.section-pills {
			display: flex;
			flex-wrap: wrap;
			gap: 6px;
			margin-top: 16px;
		}
		.pill {
			display: inline-flex;
			align-items: center;
			min-height: 24px;
			padding: 3px 8px;
			border-radius: 999px;
			background: var(--quiet);
			color: var(--muted);
			font-size: 0.72rem;
			font-variant-numeric: tabular-nums;
		}
		.article {
			margin-top: 22px;
			background: var(--paper);
			border: 1px solid var(--line);
			border-radius: var(--radius);
			box-shadow: var(--shadow);
			padding: 30px;
			min-width: 0;
		}
		.markdown {
			font-family: Georgia, "Iowan Old Style", "Times New Roman", serif;
			font-size: 1.08rem;
			line-height: 1.76;
			color: var(--text);
			overflow-wrap: anywhere;
		}
		.markdown h1 { display: none; }
		.markdown h2,
		.markdown h3 {
			font-family: "Avenir Next", "SF Pro Text", "Helvetica Neue", Arial, sans-serif;
			font-size: 0.92rem;
			line-height: 1.3;
			margin: 1.8rem 0 0.7rem;
			padding-top: 1rem;
			border-top: 1px solid var(--line);
			color: var(--ink);
			font-weight: 800;
		}
		.markdown h2:first-child,
		.markdown h3:first-child {
			margin-top: 0;
			padding-top: 0;
			border-top: 0;
		}
		.markdown p { margin: 0.8rem 0; }
		.markdown ul,
		.markdown ol {
			margin: 0.8rem 0;
			padding-left: 1.35rem;
		}
		.markdown li { margin: 0.35rem 0; }
		.markdown blockquote {
			margin: 1rem 0;
			padding: 0 0 0 1rem;
			border-left: 2px solid var(--line-strong);
			color: #4b5563;
		}
		.markdown code {
			background: var(--quiet);
			border-radius: 4px;
			padding: 0.1rem 0.28rem;
			font-family: "SF Mono", Menlo, Consolas, monospace;
			font-size: 0.88em;
		}
		.markdown pre {
			overflow-x: auto;
			background: #111827;
			color: #f8fafc;
			border-radius: var(--radius);
			padding: 14px;
		}
		.markdown pre code {
			background: transparent;
			color: inherit;
			padding: 0;
		}
		.markdown img {
			max-width: 100%;
			height: auto;
		}
		@media (max-width: 560px) {
			.page { width: min(100% - 20px, 860px); padding-top: 20px; }
			.article { padding: 20px 16px; }
			.markdown { font-size: 1rem; }
		}
	</style>
</head>
<body>
	<div class="page">
		<header>
			<a class="back" href="../dashboard.html">返回 Dashboard</a>
			<h1>{{.Entry.Title}}</h1>
			<div class="meta">{{.Entry.Date}} · {{len .Entry.Sections}} 段 · Generated {{.GeneratedAt.Format "2006-01-02 15:04"}}</div>
			{{if .Entry.Sections}}
			<div class="section-pills" aria-label="Diary sections">
				{{range .Entry.Sections}}<span class="pill">{{.}}</span>{{end}}
			</div>
			{{end}}
		</header>
		<article class="article">
			<div class="markdown">{{.Entry.HTML}}</div>
		</article>
	</div>
</body>
</html>
`
