package sync

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/borankux/dear-diary/internal/process"
	"github.com/borankux/dear-diary/internal/storage"
)

// SyncHandler 提供同步相关的 API
type SyncHandler struct {
	storage *storage.Storage
	store   *process.Store
}

// NewSyncHandler 创建一个新的 SyncHandler
func NewSyncHandler(storage *storage.Storage, store *process.Store) *SyncHandler {
	return &SyncHandler{
		storage: storage,
		store:   store,
	}
}

// ---------- 辅助函数 ----------

func writeJSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func countSections(b []byte) int {
	text := strings.ReplaceAll(string(b), "\r\n", "\n")
	sections := strings.Count(text, "\n## ")
	if strings.HasPrefix(text, "## ") {
		sections++
	}
	return sections
}

// ---------- 响应结构 ----------

type diarySyncEntry struct {
	Date     string `json:"date"`
	Mtime    string `json:"mtime"`
	Sections int    `json:"sections"`
}

type todoSyncEntry struct {
	ID        int    `json:"id"`
	Text      string `json:"text"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

type memorySyncEntry struct {
	ID        int    `json:"id"`
	Topic     string `json:"topic"`
	UpdatedAt string `json:"updated_at"`
}

type candidateSyncEntry struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type syncResponse struct {
	Diaries    []diarySyncEntry     `json:"diaries"`
	Todos      []todoSyncEntry      `json:"todos"`
	Memories   []memorySyncEntry    `json:"memories"`
	Candidates []candidateSyncEntry `json:"candidates"`
}

// HandleSync 增量同步 API
// GET /api/sync?since=2026-06-24T10:00:00Z
func (sh *SyncHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	var since time.Time
	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			writeError(w, 400, "invalid since parameter")
			return
		}
	}

	// ---------- Diaries ----------
	files, err := sh.storage.AllMarkdownFiles()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	var diaries []diarySyncEntry
	thirtyDaysAgo := time.Now().UTC().AddDate(0, 0, -30)
	diaryCutoff := since
	if sinceStr == "" {
		diaryCutoff = thirtyDaysAgo
	}

	for _, path := range files {
		if !storage.IsDiaryFilePath(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		mtime := info.ModTime().UTC()
		if mtime.Before(diaryCutoff) {
			continue
		}
		date, ok := storage.DateFromDiaryPath(path)
		if !ok {
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sections := countSections(b)
		diaries = append(diaries, diarySyncEntry{
			Date:     date.Format("2006-01-02"),
			Mtime:    mtime.Format(time.RFC3339),
			Sections: sections,
		})
	}

	// ---------- Todos ----------
	var todos []todoSyncEntry
	for _, status := range []string{
		process.TodoStatusActive,
		process.TodoStatusInProgress,
		process.TodoStatusDone,
		process.TodoStatusWontDo,
		process.TodoStatusArchived,
		process.TodoStatusOther,
	} {
		list, err := sh.store.ListTodosByStatus(status)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		for _, t := range list {
			if sinceStr != "" && t.UpdatedAt.Before(since) {
				continue
			}
			todos = append(todos, todoSyncEntry{
				ID:        t.ID,
				Text:      t.Text,
				Status:    t.Status,
				UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
			})
		}
	}

	// ---------- Memories ----------
	var memories []memorySyncEntry
	memList, err := sh.store.ListMemories()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	for _, m := range memList {
		if sinceStr != "" && m.UpdatedAt.Before(since) {
			continue
		}
		memories = append(memories, memorySyncEntry{
			ID:        m.ID,
			Topic:     m.Topic,
			UpdatedAt: m.UpdatedAt.Format(time.RFC3339),
		})
	}

	// ---------- Candidates ----------
	var candidates []candidateSyncEntry
	candList, err := sh.store.ListPendingCandidates()
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	for _, c := range candList {
		if sinceStr != "" && c.UpdatedAt.Before(since) {
			continue
		}
		candidates = append(candidates, candidateSyncEntry{
			ID:        c.ID,
			Type:      c.Type,
			Title:     c.Title,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, 200, syncResponse{
		Diaries:    diaries,
		Todos:      todos,
		Memories:   memories,
		Candidates: candidates,
	})
}

// HandleDiaryCreate 创建/更新日记 API（供 Android APP 使用）
// POST /api/diaries
func (sh *SyncHandler) HandleDiaryCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date    string `json:"date"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid body")
		return
	}

	dateStr := body.Date
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeError(w, 400, "invalid date")
		return
	}

	path := sh.storage.PathFor(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	defer f.Close()

	if _, err := f.WriteString(body.Content); err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSON(w, 200, map[string]any{
		"date":  dateStr,
		"saved": true,
	})
}

// HandleHealth 健康检查
// GET /health
func (sh *SyncHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{
		"status":  "ok",
		"version": "0.5.0-server",
	})
}
