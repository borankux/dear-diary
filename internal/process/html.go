package process

import (
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// HTMLWriter renders extracted assets to a single-page HTML dashboard.
type HTMLWriter struct {
	outDir string
}

// NewHTMLWriter creates a writer that outputs to the given directory.
func NewHTMLWriter(outDir string) *HTMLWriter {
	return &HTMLWriter{outDir: outDir}
}

// WriteAll regenerates dashboard.html from the store.
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

	data := struct {
		GeneratedAt time.Time
		TodoCount   int
		MemoryCount int
		Todos       []Todo
		Memories    []Memory
	}{
		GeneratedAt: time.Now(),
		TodoCount:   len(todos),
		MemoryCount: len(memories),
		Todos:       todos,
		Memories:    memories,
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

const dashboardTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Dear Diary Dashboard</title>
	<style>
		:root {
			--bg: #0f172a;
			--card: #1e293b;
			--text: #f8fafc;
			--muted: #94a3b8;
			--accent: #38bdf8;
			--todo: #fbbf24;
			--memory: #a78bfa;
			--border: #334155;
		}
		* { box-sizing: border-box; }
		body {
			margin: 0;
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
			background: var(--bg);
			color: var(--text);
			line-height: 1.6;
		}
		.container {
			max-width: 900px;
			margin: 0 auto;
			padding: 2rem 1rem;
		}
		header {
			margin-bottom: 2rem;
		}
		h1 {
			margin: 0;
			font-size: 1.75rem;
			font-weight: 700;
		}
		.meta {
			color: var(--muted);
			font-size: 0.875rem;
			margin-top: 0.25rem;
		}
		.stats {
			display: grid;
			grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
			gap: 1rem;
			margin-bottom: 2rem;
		}
		.stat-card {
			background: var(--card);
			border: 1px solid var(--border);
			border-radius: 12px;
			padding: 1.25rem;
		}
		.stat-card h3 {
			margin: 0;
			font-size: 0.875rem;
			color: var(--muted);
			font-weight: 500;
		}
		.stat-card .number {
			font-size: 2rem;
			font-weight: 700;
			margin-top: 0.5rem;
		}
		.stat-card.todo .number { color: var(--todo); }
		.stat-card.memory .number { color: var(--memory); }
		.section {
			background: var(--card);
			border: 1px solid var(--border);
			border-radius: 12px;
			padding: 1.5rem;
			margin-bottom: 1.5rem;
		}
		.section h2 {
			margin: 0 0 1rem 0;
			font-size: 1.25rem;
			display: flex;
			align-items: center;
			gap: 0.5rem;
		}
		.todo-item, .memory-item {
			padding: 1rem 0;
			border-bottom: 1px solid var(--border);
		}
		.todo-item:last-child, .memory-item:last-child {
			border-bottom: none;
		}
		.todo-text {
			font-size: 1rem;
		}
		.memory-topic {
			font-weight: 600;
			color: var(--memory);
			margin-bottom: 0.25rem;
		}
		.memory-summary {
			color: var(--text);
		}
		.source {
			font-size: 0.75rem;
			color: var(--muted);
			margin-top: 0.5rem;
		}
		.empty {
			color: var(--muted);
			font-style: italic;
		}
	</style>
</head>
<body>
	<div class="container">
		<header>
			<h1>📓 Dear Diary Dashboard</h1>
			<div class="meta">生成于 {{.GeneratedAt.Format "2006-01-02 15:04"}}</div>
		</header>

		<div class="stats">
			<div class="stat-card todo">
				<h3>活跃 Todo</h3>
				<div class="number">{{.TodoCount}}</div>
			</div>
			<div class="stat-card memory">
				<h3>Memory</h3>
				<div class="number">{{.MemoryCount}}</div>
			</div>
		</div>

		<div class="section">
			<h2>📝 活跃 Todo 列表</h2>
			{{if .Todos}}
				{{range .Todos}}
				<div class="todo-item">
					<div class="todo-text">{{.Text}}</div>
					{{if .SourceFile}}<div class="source">来源: {{.SourceFile}}</div>{{end}}
				</div>
				{{end}}
			{{else}}
				<div class="empty">暂无活跃 Todo</div>
			{{end}}
		</div>

		<div class="section">
			<h2>🧠 Memory 摘要</h2>
			{{if .Memories}}
				{{range .Memories}}
				<div class="memory-item">
					<div class="memory-topic">{{.Topic}}</div>
					<div class="memory-summary">{{.Summary}}</div>
					{{if .SourceFile}}<div class="source">来源: {{.SourceFile}}</div>{{end}}
				</div>
				{{end}}
			{{else}}
				<div class="empty">暂无 Memory</div>
			{{end}}
		</div>
	</div>
</body>
</html>
`

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

	writer := NewHTMLWriter(ProcessOutDir())
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
