package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/borankux/dear-diary/internal/process"
	"github.com/borankux/dear-diary/internal/search"
	"github.com/borankux/dear-diary/internal/server/sync"
	"github.com/borankux/dear-diary/internal/storage"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// ---------- Stats ----------

func handleStats(w http.ResponseWriter, r *http.Request) {
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()

	s := storage.New()
	now := time.Now()
	path := s.PathFor(now)
	exists := false
	sections := 0
	if b, err := os.ReadFile(path); err == nil {
		exists = true
		sections = strings.Count(string(b), "\n## ")
		if strings.HasPrefix(string(b), "## ") {
			sections++
		}
	}

	candidateCount, _ := store.PendingCandidateCount()
	todoCounts, _ := store.TodoStatusCounts()
	memories, _ := store.ListMemories()
	diaries := countDiaries(s)

	writeJSON(w, 200, map[string]any{
		"today": map[string]any{
			"date":     now.Format("2006-01-02"),
			"exists":   exists,
			"sections": sections,
		},
		"candidateCount": candidateCount,
		"candidate":      candidateCount,
		"todoCounts": map[string]int{
			"active":      todoCounts.Active,
			"in_progress": todoCounts.InProgress,
			"done":        todoCounts.Done,
			"wont_do":     todoCounts.WontDo,
			"archived":    todoCounts.Archived,
			"other":       todoCounts.Other,
		},
		"todo":          todoCounts.Active,
		"memoryCount":   len(memories),
		"memory":        len(memories),
		"diaryCount":    diaries,
		"diary":         diaries,
		"processing":    "Ready",
	})
}

func countDiaries(s *storage.Storage) int {
	files, _ := s.AllMarkdownFiles()
	count := 0
	for _, f := range files {
		if storage.IsDiaryFilePath(f) {
			count++
		}
	}
	return count
}

// ---------- Todos ----------

func handleTodos(w http.ResponseWriter, r *http.Request) {
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()

	// Return all todos across all statuses
	allTodos := make([]process.Todo, 0)
	for _, status := range []string{process.TodoStatusActive, process.TodoStatusInProgress, process.TodoStatusDone, process.TodoStatusWontDo, process.TodoStatusArchived, process.TodoStatusOther} {
		todos, err := store.ListTodosByStatus(status)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		allTodos = append(allTodos, todos...)
	}
	writeJSON(w, 200, allTodos)
}

func handleUpdateTodoStatus(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var body struct{ Status string `json:"status"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}

	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()

	if err := store.SetTodoStatus(id, body.Status); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- Candidates ----------

func handleCandidates(w http.ResponseWriter, r *http.Request) {
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()

	candidates, err := store.ListPendingCandidates()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if candidates == nil {
		candidates = []process.Candidate{}
	}
	writeJSON(w, 200, candidates)
}

func handleAcceptCandidate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()
	if err := store.AcceptCandidate(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func handleRejectCandidate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()
	if err := store.RejectCandidate(id); err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// ---------- Memories ----------

func handleMemories(w http.ResponseWriter, r *http.Request) {
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()

	memories, err := store.ListMemories()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if memories == nil {
		memories = []process.Memory{}
	}
	writeJSON(w, 200, memories)
}

// ---------- Diaries ----------

type diaryListEntry struct {
	Date    string `json:"date"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
	HTML    string `json:"html"`
	Month   string `json:"month"`
}

func handleDiaries(w http.ResponseWriter, r *http.Request) {
	s := storage.New()
	files, err := s.AllMarkdownFiles()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	// Filter canonical + sort reverse + limit 30
	entries := make([]diaryListEntry, 0)
	for _, path := range files {
		if !storage.IsDiaryFilePath(path) {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		month := filepath.Base(filepath.Dir(path))
		title, excerpt, html := summarizeDiaryForAPI(b)
		entries = append(entries, diaryListEntry{
			Date:    name,
			Title:   title,
			Excerpt: excerpt,
			HTML:    html,
			Month:   month,
		})
	}
	// Most recent first
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date > entries[j].Date
	})
	if len(entries) > 30 {
		entries = entries[:30]
	}
	writeJSON(w, 200, entries)
}

func handleDiaryByDate(w http.ResponseWriter, r *http.Request) {
	dateStr := r.PathValue("date")
	s := storage.New()
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeError(w, 400, "invalid date")
		return
	}
	path := s.PathFor(t)
	b, err := os.ReadFile(path)
	if err != nil {
		writeError(w, 404, "diary not found")
		return
	}
	content := string(b)
	sections := strings.Count(content, "\n## ")
	if strings.HasPrefix(content, "## ") {
		sections++
	}
	info, _ := os.Stat(path)
	mtime := ""
	if info != nil {
		mtime = info.ModTime().Format(time.RFC3339)
	}
	writeJSON(w, 200, map[string]any{
		"date":     dateStr,
		"content":  content,
		"sections": sections,
		"mtime":    mtime,
	})
}

func summarizeDiaryForAPI(b []byte) (string, string, string) {
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
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
	var plainParts []string
	for _, line := range lines[bodyStart:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		if trimmed != "" {
			plainParts = append(plainParts, trimmed)
		}
	}
	body := strings.Join(lines[bodyStart:], "\n")
	excerpt := truncateTextAPI(strings.Join(plainParts, " "), 150)
	html := renderMarkdownAPI([]byte(body))
	return title, excerpt, html
}

func truncateTextAPI(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func renderMarkdownAPI(src []byte) string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(src)
	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank | html.SkipHTML}
	renderer := html.NewRenderer(opts)
	return string(markdown.Render(doc, renderer))
}

// ---------- Calendar ----------

type calendarDayAPI struct {
	Day       int    `json:"day"`
	Date      string `json:"date"`
	IsPadding bool   `json:"isPadding"`
	IsWritten bool   `json:"isWritten"`
	IsToday   bool   `json:"isToday"`
}

type calendarMonthAPI struct {
	Month       string           `json:"month"`
	Title       string           `json:"title"`
	Count       int              `json:"count"`
	DaysInMonth int              `json:"daysInMonth"`
	Days        []calendarDayAPI `json:"days"`
}

func handleCalendar(w http.ResponseWriter, r *http.Request) {
	s := storage.New()
	files, err := s.AllMarkdownFiles()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	written := make(map[string]bool)
	monthStarts := make(map[string]time.Time)
	for _, path := range files {
		if !storage.IsDiaryFilePath(path) {
			continue
		}
		d, ok := storage.DateFromDiaryPath(path)
		if !ok {
			continue
		}
		written[d.Format("2006-01-02")] = true
		monthStarts[d.Format("2006-01")] = time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, time.Local)
	}
	now := time.Now()
	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	monthStarts[currentMonth.Format("2006-01")] = currentMonth

	months := make([]string, 0, len(monthStarts))
	for m := range monthStarts {
		months = append(months, m)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))

	result := make([]calendarMonthAPI, 0)
	for _, month := range months {
		start := monthStarts[month]
		daysInMonth := time.Date(start.Year(), start.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
		firstWeekday := (int(start.Weekday()) + 6) % 7
		var days []calendarDayAPI
		for p := 0; p < firstWeekday; p++ {
			days = append(days, calendarDayAPI{IsPadding: true})
		}
		count := 0
		for day := 1; day <= daysInMonth; day++ {
			date := time.Date(start.Year(), start.Month(), day, 0, 0, 0, 0, time.Local)
			dateStr := date.Format("2006-01-02")
			isWritten := written[dateStr]
			if isWritten {
				count++
			}
			days = append(days, calendarDayAPI{
				Day:       day,
				Date:      dateStr,
				IsPadding: false,
				IsWritten: isWritten,
				IsToday:   sameDay(date, now),
			})
		}
		result = append(result, calendarMonthAPI{
			Month:       month,
			Title:       fmt.Sprintf("%d 年 %d 月", start.Year(), int(start.Month())),
			Count:       count,
			DaysInMonth: daysInMonth,
			Days:        days,
		})
	}
	writeJSON(w, 200, result)
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// ---------- Search ----------

type searchLine struct {
	LineNum int    `json:"lineNum"`
	Text    string `json:"text"`
}

type searchResult struct {
	Date  string       `json:"date"`
	Title string       `json:"title"`
	Lines []searchLine `json:"lines"`
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, 200, []searchResult{})
		return
	}
	s := storage.New()
	results, err := search.Search(s.RootDir(), q)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	// Group by date
	grouped := make(map[string][]searchLine)
	for _, res := range results {
		date := res.Date
		grouped[date] = append(grouped[date], searchLine{LineNum: res.Line, Text: res.Text})
	}
	apiResults := make([]searchResult, 0)
	for date, lines := range grouped {
		apiResults = append(apiResults, searchResult{
			Date:  date,
			Title: date,
			Lines: lines,
		})
	}
	// Sort by date desc
	sort.Slice(apiResults, func(i, j int) bool {
		return apiResults[i].Date > apiResults[j].Date
	})
	writeJSON(w, 200, apiResults)
}

// Need fmt import

// handleSync wraps sync.SyncHandler for incremental sync API.
func handleSync(w http.ResponseWriter, r *http.Request) {
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()
	s := storage.New()
	handler := sync.NewSyncHandler(s, store)
	handler.HandleSync(w, r)
}

// handleDiaryCreate wraps sync.SyncHandler for diary creation API.
func handleDiaryCreate(w http.ResponseWriter, r *http.Request) {
	store, err := process.NewStore("")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer store.Close()
	s := storage.New()
	handler := sync.NewSyncHandler(s, store)
	handler.HandleDiaryCreate(w, r)
}

// handleHealth returns server health status.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{
		"status":  "ok",
		"version": "0.5.0-server",
	})
}
