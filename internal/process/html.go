package process

import (
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
	Date    string
	HTML    template.HTML
	RawPath string
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
	diaries, err := w.loadDiaries()
	if err != nil {
		return err
	}

	data := struct {
		GeneratedAt time.Time
		TodoCount   int
		MemoryCount int
		DiaryCount  int
		Todos       []Todo
		Memories    []Memory
		Diaries     []DiaryEntry
	}{
		GeneratedAt: time.Now(),
		TodoCount:   len(todos),
		MemoryCount: len(memories),
		DiaryCount:  len(diaries),
		Todos:       todos,
		Memories:    memories,
		Diaries:     diaries,
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
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(path), ".md")
		entries = append(entries, DiaryEntry{
			Date:    name,
			HTML:    renderMarkdown(b),
			RawPath: path,
		})
	}
	return entries, nil
}

func renderMarkdown(src []byte) template.HTML {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(src)
	opts := html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank}
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
			--bg: #0a0a0f;
			--surface: #14141b;
			--surface-2: #1c1c25;
			--surface-3: #252530;
			--text: #f0f0f5;
			--muted: #8a8a98;
			--accent: #6366f1;
			--todo: #f59e0b;
			--memory: #8b5cf6;
			--diary: #10b981;
			--shadow: 0 1px 0 rgba(255,255,255,0.04) inset, 0 8px 24px rgba(0,0,0,0.4);
			--radius-outer: 16px;
			--radius-inner: 12px;
			--radius-button: 8px;
		}
		* { box-sizing: border-box; }
		html, body {
			margin: 0;
			padding: 0;
			height: 100%;
			overflow: hidden;
		}
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
			background: var(--bg);
			color: var(--text);
			-webkit-font-smoothing: antialiased;
			-moz-osx-font-smoothing: grayscale;
			text-wrap: pretty;
		}
		.app {
			display: flex;
			flex-direction: column;
			height: 100vh;
			padding: 16px;
			gap: 16px;
		}
		header {
			display: flex;
			align-items: center;
			justify-content: space-between;
			flex-shrink: 0;
			padding: 0 4px;
		}
		header h1 {
			margin: 0;
			font-size: 1.25rem;
			font-weight: 700;
			letter-spacing: -0.02em;
			text-wrap: balance;
		}
		header .meta {
			color: var(--muted);
			font-size: 0.75rem;
			font-variant-numeric: tabular-nums;
		}
		.stats {
			display: flex;
			gap: 12px;
			flex-shrink: 0;
		}
		.stat {
			background: var(--surface);
			border-radius: var(--radius-button);
			padding: 8px 14px;
			box-shadow: var(--shadow);
			font-size: 0.75rem;
			color: var(--muted);
			min-width: 72px;
			text-align: center;
			transition: transform 0.15s cubic-bezier(0.2, 0, 0, 1);
		}
		.stat:active { transform: scale(0.96); }
		.stat .num {
			display: block;
			font-size: 1.125rem;
			font-weight: 700;
			color: var(--text);
			font-variant-numeric: tabular-nums;
		}
		.stat.todo .num { color: var(--todo); }
		.stat.memory .num { color: var(--memory); }
		.stat.diary .num { color: var(--diary); }
		.grid {
			display: grid;
			grid-template-columns: 1fr 1fr 1.4fr;
			gap: 16px;
			flex: 1;
			min-height: 0;
		}
		.column {
			display: flex;
			flex-direction: column;
			min-height: 0;
			background: var(--surface);
			border-radius: var(--radius-outer);
			box-shadow: var(--shadow);
			overflow: hidden;
			animation: fadeIn 0.5s cubic-bezier(0.2, 0, 0, 1) both;
		}
		.column:nth-child(1) { animation-delay: 0ms; }
		.column:nth-child(2) { animation-delay: 80ms; }
		.column:nth-child(3) { animation-delay: 160ms; }
		.column-header {
			display: flex;
			align-items: center;
			justify-content: space-between;
			padding: 14px 16px;
			border-bottom: 1px solid var(--surface-3);
			flex-shrink: 0;
		}
		.column-header h2 {
			margin: 0;
			font-size: 0.875rem;
			font-weight: 600;
			display: flex;
			align-items: center;
			gap: 8px;
		}
		.column-header .badge {
			font-size: 0.6875rem;
			font-weight: 700;
			padding: 3px 8px;
			border-radius: 999px;
			background: var(--surface-3);
			color: var(--muted);
			font-variant-numeric: tabular-nums;
		}
		.column-content {
			flex: 1;
			overflow-y: auto;
			padding: 12px;
			min-height: 0;
		}
		.todo-item {
			display: flex;
			align-items: flex-start;
			gap: 10px;
			padding: 10px 12px;
			border-radius: var(--radius-button);
			background: var(--surface-2);
			margin-bottom: 8px;
			transition: transform 0.15s cubic-bezier(0.2, 0, 0, 1), background 0.15s ease;
		}
		.todo-item:hover { background: var(--surface-3); }
		.todo-item:active { transform: scale(0.98); }
		.todo-item input[type="checkbox"] {
			appearance: none;
			width: 18px;
			height: 18px;
			border-radius: 5px;
			border: 2px solid var(--surface-3);
			background: var(--surface-3);
			margin-top: 2px;
			flex-shrink: 0;
			cursor: pointer;
			transition: border-color 0.15s ease, background 0.15s ease;
		}
		.todo-item input[type="checkbox"]:checked {
			background: var(--todo);
			border-color: var(--todo);
		}
		.todo-item .text {
			font-size: 0.8125rem;
			line-height: 1.5;
		}
		.memory-item {
			padding: 12px;
			border-radius: var(--radius-button);
			background: var(--surface-2);
			margin-bottom: 8px;
			transition: transform 0.15s cubic-bezier(0.2, 0, 0, 1), background 0.15s ease;
		}
		.memory-item:hover { background: var(--surface-3); }
		.memory-item:active { transform: scale(0.98); }
		.memory-item .topic {
			font-size: 0.8125rem;
			font-weight: 600;
			color: var(--memory);
			margin-bottom: 4px;
		}
		.memory-item .summary {
			font-size: 0.75rem;
			color: var(--text);
			line-height: 1.5;
		}
		.diary-item {
			padding: 14px;
			border-radius: var(--radius-inner);
			background: var(--surface-2);
			margin-bottom: 12px;
			transition: transform 0.15s cubic-bezier(0.2, 0, 0, 1), background 0.15s ease;
		}
		.diary-item:hover { background: var(--surface-3); }
		.diary-item:active { transform: scale(0.99); }
		.diary-item .date {
			font-size: 0.75rem;
			font-weight: 600;
			color: var(--diary);
			margin-bottom: 8px;
			font-variant-numeric: tabular-nums;
		}
		.diary-item .body {
			font-size: 0.8125rem;
			line-height: 1.6;
		}
		.diary-item .body h1,
		.diary-item .body h2,
		.diary-item .body h3 {
			font-size: 0.875rem;
			margin: 0.75rem 0 0.4rem;
			color: var(--text);
		}
		.diary-item .body p { margin: 0.4rem 0; }
		.diary-item .body ul, .diary-item .body ol {
			margin: 0.4rem 0;
			padding-left: 1.25rem;
		}
		.diary-item .body li { margin: 0.15rem 0; }
		.diary-item .body code {
			background: var(--surface-3);
			padding: 2px 5px;
			border-radius: 4px;
			font-size: 0.75rem;
		}
		.empty {
			color: var(--muted);
			font-size: 0.8125rem;
			text-align: center;
			padding: 2rem 1rem;
			font-style: italic;
		}
		.source {
			font-size: 0.6875rem;
			color: var(--muted);
			margin-top: 6px;
		}
		::-webkit-scrollbar { width: 6px; }
		::-webkit-scrollbar-track { background: transparent; }
		::-webkit-scrollbar-thumb {
			background: var(--surface-3);
			border-radius: 3px;
		}
		@keyframes fadeIn {
			from { opacity: 0; transform: translateY(12px); }
			to { opacity: 1; transform: translateY(0); }
		}
		@media (max-width: 900px) {
			.grid {
				grid-template-columns: 1fr;
				grid-template-rows: 1fr 1fr 1.5fr;
			}
			.stats { display: none; }
		}
	</style>
</head>
<body>
	<div class="app">
		<header>
			<div>
				<h1>📓 Dear Diary</h1>
				<div class="meta">Generated {{.GeneratedAt.Format "2006-01-02 15:04"}}</div>
			</div>
			<div class="stats">
				<div class="stat todo"><span class="num">{{.TodoCount}}</span>Todo</div>
				<div class="stat memory"><span class="num">{{.MemoryCount}}</span>Memory</div>
				<div class="stat diary"><span class="num">{{.DiaryCount}}</span>Diary</div>
			</div>
		</header>
		<div class="grid">
			<div class="column">
				<div class="column-header">
					<h2>📝 Todos</h2>
					<span class="badge">{{.TodoCount}}</span>
				</div>
				<div class="column-content">
					{{if .Todos}}
						{{range .Todos}}
						<label class="todo-item">
							<input type="checkbox">
							<div>
								<div class="text">{{.Text}}</div>
								{{if .SourceFile}}<div class="source">{{.SourceFile}}</div>{{end}}
							</div>
						</label>
						{{end}}
					{{else}}
						<div class="empty">No active todos</div>
					{{end}}
				</div>
			</div>
			<div class="column">
				<div class="column-header">
					<h2>🧠 Memories</h2>
					<span class="badge">{{.MemoryCount}}</span>
				</div>
				<div class="column-content">
					{{if .Memories}}
						{{range .Memories}}
						<div class="memory-item">
							<div class="topic">{{.Topic}}</div>
							<div class="summary">{{.Summary}}</div>
							{{if .SourceFile}}<div class="source">{{.SourceFile}}</div>{{end}}
						</div>
						{{end}}
					{{else}}
						<div class="empty">No memories yet</div>
					{{end}}
				</div>
			</div>
			<div class="column">
				<div class="column-header">
					<h2>📄 Diaries</h2>
					<span class="badge">{{.DiaryCount}}</span>
				</div>
				<div class="column-content">
					{{if .Diaries}}
						{{range .Diaries}}
						<div class="diary-item">
							<div class="date">{{.Date}}</div>
							<div class="body">{{.HTML}}</div>
						</div>
						{{end}}
					{{else}}
						<div class="empty">No diaries found</div>
					{{end}}
				</div>
			</div>
		</div>
	</div>
</body>
</html>
`
