package web

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/borankux/dear-diary/internal/server/auth"
	"github.com/borankux/dear-diary/internal/server/sync"
	"github.com/borankux/dear-diary/internal/server/watcher"
)

//go:embed dist
var distFS embed.FS

// Server wraps the HTTP server and its dependencies.
type Server struct {
	addr        string
	authConfig  *auth.Config
	hub         *sync.Hub
	autoProcess *watcher.AutoProcess
}

// NewServer creates a new web server on the given address.
// In server mode, DIARY_DATA_DIR and DIARY_DB_PATH env vars control data locations.
func NewServer(addr string) *Server {
	if addr == "" {
		addr = "0.0.0.0:8765"
	}

	// Server-mode path configuration
	dataDir := os.Getenv("DIARY_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, "Documents", "dear-diary")
	}
	dbPath := os.Getenv("DIARY_DB_PATH")
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "process.db")
	}

	// Set env vars so existing handlers (storage.New, process.NewStore) pick them up
	os.Setenv("DIARY_DIR", dataDir)
	os.Setenv("DIARY_DB_PATH", dbPath)

	authConfig := auth.NewConfig()
	hub := sync.NewHub()

	// Start auto-process engine
	autoProcess, err := watcher.NewAutoProcess(dataDir, dbPath, hub)
	if err != nil {
		log.Printf("警告: 无法启动自动处理引擎: %v", err)
	} else {
		autoProcess.Start()
		log.Println("自动处理引擎已启动")
	}

	return &Server{
		addr:        addr,
		authConfig:  authConfig,
		hub:         hub,
		autoProcess: autoProcess,
	}
}

// Start launches the HTTP server and blocks until it exits.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Public routes (no auth required)
	mux.HandleFunc("/auth/login", s.authConfig.LoginHandler)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/events", s.hub.Subscribe)

	// API routes (protected by auth if enabled)
	api := http.NewServeMux()
	api.HandleFunc("GET /stats", handleStats)
	api.HandleFunc("GET /todos", handleTodos)
	api.HandleFunc("POST /todos/{id}/status", handleUpdateTodoStatus)
	api.HandleFunc("GET /candidates", handleCandidates)
	api.HandleFunc("POST /candidates/{id}/accept", handleAcceptCandidate)
	api.HandleFunc("POST /candidates/{id}/reject", handleRejectCandidate)
	api.HandleFunc("GET /memories", handleMemories)
	api.HandleFunc("GET /diaries", handleDiaries)
	api.HandleFunc("GET /diaries/{date}", handleDiaryByDate)
	api.HandleFunc("GET /calendar", handleCalendar)
	api.HandleFunc("GET /search", handleSearch)

	// Wrap API with auth middleware
	authWrapped := s.authConfig.AuthMiddleware(api)
	mux.Handle("/api/", http.StripPrefix("/api", authWrapped))

	// Static SPA files
	static, err := fs.Sub(distFS, "dist")
	if err != nil {
		return fmt.Errorf("embed dist: %w", err)
	}
	fsHandler := http.FileServer(http.FS(static))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			_, err := fs.Stat(static, r.URL.Path[1:])
			if err != nil {
				r.URL.Path = "/"
			}
		}
		fsHandler.ServeHTTP(w, r)
	})

	if s.authConfig != nil {
		log.Printf("Dear Diary server starting on http://%s (auth enabled)", s.addr)
	} else {
		log.Printf("Dear Diary server starting on http://%s (no auth)", s.addr)
	}
	return http.ListenAndServe(s.addr, mux)
}

// OpenBrowser opens the given URL in the default browser.
func OpenBrowser(url string) error {
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
