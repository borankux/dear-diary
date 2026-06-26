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
	maxDashboardCandidates = 5
	maxDashboardTodos      = 12
	maxDashboardMemories   = 8
	maxDashboardDiaries    = 7
	diaryPagesDir          = "entries"
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

type dashboardData struct {
	GeneratedAt       time.Time
	Today             TodayStatus
	CandidateCount    int
	TodoCount         int
	MemoryCount       int
	DiaryCount        int
	CandidateOverflow int
	TodoOverflow      int
	MemoryOverflow    int
	DiaryOverflow     int
	Candidates        []Candidate
	Todos             []Todo
	Memories          []Memory
	Diaries           []DiaryEntry
	CalendarMonths    []CalendarMonth
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

	visibleCandidates, candidateOverflow := limitSlice(candidates, maxDashboardCandidates)
	visibleTodos, todoOverflow := limitSlice(todos, maxDashboardTodos)
	visibleMemories, memoryOverflow := limitSlice(memories, maxDashboardMemories)
	visibleDiaries, diaryOverflow := limitSlice(diaries, maxDashboardDiaries)

	data := dashboardData{
		GeneratedAt:       time.Now(),
		Today:             w.todayStatus(),
		CandidateCount:    len(candidates),
		TodoCount:         len(todos),
		MemoryCount:       len(memories),
		DiaryCount:        len(diaries),
		CandidateOverflow: candidateOverflow,
		TodoOverflow:      todoOverflow,
		MemoryOverflow:    memoryOverflow,
		DiaryOverflow:     diaryOverflow,
		Candidates:        visibleCandidates,
		Todos:             visibleTodos,
		Memories:          visibleMemories,
		Diaries:           visibleDiaries,
		CalendarMonths:    buildCalendarMonths(diaries, time.Now()),
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
			--bg: #f6f7f8;
			--paper: #ffffff;
			--ink: #202124;
			--text: #31343a;
			--muted: #727780;
			--quiet: #eef0f3;
			--line: #dfe3e8;
			--line-strong: #c7cdd5;
			--accent: #0f766e;
			--attention: #b45309;
			--memory: #475569;
			--danger: #b91c1c;
			--radius: 8px;
			--shadow: 0 1px 2px rgba(30, 41, 59, 0.08);
		}
		* { box-sizing: border-box; }
		html, body {
			margin: 0;
			padding: 0;
		}
		body {
			font-family: "Avenir Next", "SF Pro Text", "Helvetica Neue", Arial, sans-serif;
			background: var(--bg);
			color: var(--ink);
			-webkit-font-smoothing: antialiased;
			-moz-osx-font-smoothing: grayscale;
			text-wrap: pretty;
		}
		.app {
			width: min(1320px, calc(100% - 32px));
			margin: 0 auto;
			padding: 28px 0 48px;
		}
		header {
			display: flex;
			align-items: flex-end;
			justify-content: space-between;
			gap: 24px;
			padding-bottom: 18px;
			border-bottom: 1px solid var(--line);
		}
		header h1 {
			margin: 0;
			font-size: 1.6rem;
			font-weight: 700;
			letter-spacing: 0;
		}
		.kicker,
		header .meta,
		.overflow-note,
		.source,
		.entry-meta,
		.month-count {
			color: var(--muted);
			font-size: 0.78rem;
			font-variant-numeric: tabular-nums;
		}
		.kicker {
			margin-bottom: 4px;
			text-transform: uppercase;
			font-size: 0.68rem;
			font-weight: 800;
			letter-spacing: 0.08em;
		}
		.briefing {
			padding: 22px 0 24px;
			border-bottom: 1px solid var(--line);
		}
		.briefing h2 {
			margin: 0;
			font-size: clamp(1.55rem, 2vw, 2.25rem);
			line-height: 1.18;
			font-weight: 700;
		}
		.briefing p {
			max-width: 860px;
			margin: 10px 0 0;
			color: var(--text);
			font-size: 1rem;
			line-height: 1.55;
		}
		.metric-row {
			display: grid;
			grid-template-columns: repeat(4, minmax(0, 1fr));
			gap: 10px;
			margin-top: 18px;
		}
		.metric {
			background: var(--paper);
			border: 1px solid var(--line);
			border-radius: var(--radius);
			padding: 12px 14px;
			box-shadow: var(--shadow);
		}
		.metric strong {
			display: block;
			font-size: 1.45rem;
			line-height: 1;
			font-variant-numeric: tabular-nums;
		}
		.metric span {
			display: block;
			margin-top: 5px;
			color: var(--muted);
			font-size: 0.78rem;
		}
		.content-grid {
			display: grid;
			grid-template-columns: minmax(0, 1fr) 360px;
			gap: 28px;
			margin-top: 28px;
			align-items: start;
			min-width: 0;
		}
		main,
		aside,
		.section,
		.diary-entry {
			min-width: 0;
		}
		.section {
			padding-top: 18px;
			border-top: 1px solid var(--line);
		}
		.section + .section {
			margin-top: 30px;
		}
		.section-header {
			display: flex;
			align-items: baseline;
			justify-content: space-between;
			gap: 16px;
			margin-bottom: 12px;
			min-width: 0;
		}
		.section h2 {
			margin: 0;
			font-size: 0.95rem;
			font-weight: 800;
			letter-spacing: 0;
		}
		.calendar-months {
			display: grid;
			gap: 12px;
		}
		.calendar-month {
			background: var(--paper);
			border: 1px solid var(--line);
			border-radius: var(--radius);
			box-shadow: var(--shadow);
			overflow: hidden;
			min-width: 0;
		}
		.calendar-month summary {
			list-style: none;
			cursor: pointer;
			display: flex;
			align-items: baseline;
			justify-content: space-between;
			gap: 12px;
			padding: 13px 14px;
			border-bottom: 1px solid var(--line);
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
			font-weight: 800;
			text-align: center;
		}
		.calendar-grid {
			padding: 8px 14px 14px;
		}
		.calendar-cell {
			min-width: 0;
			min-height: 46px;
			border: 1px solid var(--line);
			border-radius: 7px;
			padding: 6px;
			background: #fbfcfd;
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
			background: #e9f7f3;
			border-color: #b8ddd3;
			color: var(--accent);
			font-weight: 800;
		}
		.calendar-day.is-today {
			border-color: var(--attention);
			color: var(--attention);
		}
		.calendar-day small {
			color: inherit;
			font-size: 0.65rem;
			font-weight: 700;
			white-space: nowrap;
		}
		.diary-entry {
			background: var(--paper);
			border: 1px solid var(--line);
			border-radius: var(--radius);
			box-shadow: var(--shadow);
			margin-bottom: 14px;
			overflow: hidden;
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
			font-size: 1.05rem;
			line-height: 1.25;
			font-weight: 800;
			overflow-wrap: anywhere;
		}
		.entry-excerpt {
			margin: 7px 0 0;
			color: var(--muted);
			font-size: 0.88rem;
			line-height: 1.45;
			overflow-wrap: anywhere;
		}
		.entry-link {
			color: var(--accent);
			text-decoration: none;
			font-weight: 800;
		}
		.entry-link:hover {
			text-decoration: underline;
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
			border-top: 1px solid var(--line);
			padding: 22px 20px 28px;
			font-family: Georgia, "Iowan Old Style", "Times New Roman", serif;
			font-size: 1.03rem;
			line-height: 1.72;
			color: var(--text);
			max-width: 780px;
			min-width: 0;
			overflow-wrap: anywhere;
		}
		.markdown h1 { display: none; }
		.markdown h2,
		.markdown h3 {
			font-family: "Avenir Next", "SF Pro Text", "Helvetica Neue", Arial, sans-serif;
			font-size: 0.88rem;
			line-height: 1.3;
			margin: 1.6rem 0 0.65rem;
			padding-top: 0.9rem;
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
			font-family: "Avenir Next", "SF Pro Text", "Helvetica Neue", Arial, sans-serif;
			font-size: 0.88rem;
		}
		.markdown img {
			max-width: 100%;
			height: auto;
		}
		.markdown th,
		.markdown td {
			border-bottom: 1px solid var(--line);
			padding: 8px 10px;
			text-align: left;
			vertical-align: top;
		}
		.item-list {
			display: flex;
			flex-direction: column;
			gap: 8px;
		}
		.item {
			background: var(--paper);
			border: 1px solid var(--line);
			border-radius: var(--radius);
			padding: 12px 13px;
			box-shadow: var(--shadow);
			min-width: 0;
			overflow-wrap: anywhere;
		}
		.item strong {
			display: block;
			font-size: 0.86rem;
			line-height: 1.38;
			font-weight: 800;
		}
		.item p {
			margin: 6px 0 0;
			color: var(--text);
			font-size: 0.82rem;
			line-height: 1.45;
		}
		.item .type {
			display: inline-block;
			margin-bottom: 6px;
			color: var(--accent);
			font-size: 0.7rem;
			font-weight: 800;
			text-transform: uppercase;
			font-variant-numeric: tabular-nums;
		}
		.todo-id {
			color: var(--attention);
			font-weight: 800;
			font-variant-numeric: tabular-nums;
		}
		.memory-topic {
			color: var(--memory);
		}
		.empty {
			background: var(--paper);
			border: 1px dashed var(--line-strong);
			border-radius: var(--radius);
			color: var(--muted);
			font-size: 0.86rem;
			padding: 18px;
		}
		.source {
			margin-top: 7px;
			overflow-wrap: anywhere;
		}
		@media (max-width: 980px) {
			.app { width: min(100% - 24px, 760px); padding-top: 20px; }
			header { align-items: flex-start; flex-direction: column; gap: 8px; }
			.metric-row { grid-template-columns: repeat(2, minmax(0, 1fr)); }
			.content-grid { grid-template-columns: 1fr; gap: 18px; }
		}
		@media (max-width: 560px) {
			.app { width: min(100% - 20px, 760px); }
			.metric-row { grid-template-columns: 1fr; }
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
			<h2 id="briefing-title">
				{{if .Today.Exists}}今天已写 {{.Today.Sections}} 段。{{else}}今天还没有写日记。{{end}}
				{{if .CandidateCount}}{{.CandidateCount}} 条候选需要看。{{else}}没有待确认候选。{{end}}
			</h2>
			<p>
				当前有 {{.TodoCount}} 个 active todos、{{.MemoryCount}} 条长期记忆、{{.DiaryCount}} 篇日记。
				这个页面只做阅读和判断重点：月历负责进入每天的日记，最近日记优先，注意力队列限量展示。
			</p>
			<div class="metric-row" aria-label="Dashboard counts">
				<div class="metric"><strong>{{.CandidateCount}}</strong><span>候选待确认</span></div>
				<div class="metric"><strong>{{.TodoCount}}</strong><span>Active todos</span></div>
				<div class="metric"><strong>{{.MemoryCount}}</strong><span>长期记忆</span></div>
				<div class="metric"><strong>{{.DiaryCount}}</strong><span>日记总数</span></div>
			</div>
		</section>

		<div class="content-grid">
			<main>
				<section class="section" aria-labelledby="calendar-title">
					<div class="section-header">
						<h2 id="calendar-title">日历入口</h2>
						<div class="meta">周一开头 · 点击有标记的日期打开日记页</div>
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

				<section class="section" aria-labelledby="diary-title">
					<div class="section-header">
						<h2 id="diary-title">最近日记</h2>
						<div class="meta">显示最近 {{len .Diaries}} 篇{{if .DiaryOverflow}}，完整历史从日历进入{{end}}</div>
					</div>
					{{if .Diaries}}
						{{range .Diaries}}
						<details class="diary-entry" {{if .Open}}open{{end}}>
							<summary>
								<div>
									<h3 class="entry-title">{{.Title}}</h3>
									{{if .Excerpt}}<p class="entry-excerpt">{{.Excerpt}}</p>{{end}}
								</div>
								<div class="entry-meta">{{.Date}} · {{len .Sections}} 段 · <a class="entry-link" href="{{.URL}}">打开日记页</a></div>
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
				<section class="section" aria-labelledby="candidate-title">
					<div class="section-header">
						<h2 id="candidate-title">候选待确认</h2>
						<div class="meta">{{.CandidateCount}}</div>
					</div>
					{{if .Candidates}}
					<div class="item-list">
						{{range .Candidates}}
						<div class="item">
							<span class="type">{{.Type}} #{{.ID}}</span>
							<strong>{{if .Title}}{{.Title}}{{else}}{{.Content}}{{end}}</strong>
							{{if .EvidenceText}}<p>{{.EvidenceText}}</p>{{end}}
							{{if .SourceDate}}<div class="source">{{.SourceDate}}</div>{{end}}
						</div>
						{{end}}
					</div>
					{{if .CandidateOverflow}}<p class="overflow-note">还有 {{.CandidateOverflow}} 条候选没有显示。</p>{{end}}
					{{else}}
						<div class="empty">没有待确认候选。</div>
					{{end}}
				</section>

				<section class="section" aria-labelledby="todo-title">
					<div class="section-header">
						<h2 id="todo-title">Active todos</h2>
						<div class="meta">{{.TodoCount}}</div>
					</div>
					{{if .Todos}}
					<div class="item-list">
						{{range .Todos}}
						<div class="item">
							<strong><span class="todo-id">#{{.ID}}</span> {{.Text}}</strong>
							{{if .EvidenceText}}<p>{{.EvidenceText}}</p>{{end}}
							{{if .SourceDate}}<div class="source">{{.SourceDate}}</div>{{else if .SourceFile}}<div class="source">{{.SourceFile}}</div>{{end}}
						</div>
						{{end}}
					</div>
					{{if .TodoOverflow}}<p class="overflow-note">只显示最近 {{len .Todos}} 个，还有 {{.TodoOverflow}} 个未显示。</p>{{end}}
					{{else}}
						<div class="empty">没有 active todo。</div>
					{{end}}
				</section>

				<section class="section" aria-labelledby="memory-title">
					<div class="section-header">
						<h2 id="memory-title">长期记忆</h2>
						<div class="meta">{{.MemoryCount}}</div>
					</div>
					{{if .Memories}}
					<div class="item-list">
						{{range .Memories}}
						<div class="item">
							<strong class="memory-topic">{{.Topic}}</strong>
							<p>{{.Summary}}</p>
							{{if .SourceDate}}<div class="source">{{.SourceDate}}</div>{{else if .SourceFile}}<div class="source">{{.SourceFile}}</div>{{end}}
						</div>
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
