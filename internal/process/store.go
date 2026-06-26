package process

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists processing state and extracted entities in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the SQLite database at the given path.
// If path is empty, it defaults to ~/.local/share/dear-diary/process.db.
func NewStore(path string) (*Store, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".local", "share", "dear-diary", "process.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS processing_runs (
	run_id TEXT PRIMARY KEY,
	base_hash TEXT NOT NULL,
	started_at DATETIME NOT NULL,
	ended_at DATETIME,
	final_state TEXT NOT NULL,
	retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS transition_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id TEXT NOT NULL,
	from_state TEXT NOT NULL,
	to_state TEXT NOT NULL,
	event TEXT NOT NULL,
	reason TEXT,
	created_at DATETIME NOT NULL,
	FOREIGN KEY (run_id) REFERENCES processing_runs(run_id)
);

CREATE TABLE IF NOT EXISTS file_snapshots (
	path TEXT PRIMARY KEY,
	content_hash TEXT NOT NULL,
	mtime DATETIME NOT NULL,
	processed_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS todos (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	text TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	source_file TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	topic TEXT NOT NULL,
	summary TEXT NOT NULL,
	source_file TEXT,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_transition_logs_run_id ON transition_logs(run_id);
`
	_, err := s.db.Exec(schema)
	return err
}

// CreateRun records the start of a process run.
func (s *Store) CreateRun(runID, baseHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO processing_runs (run_id, base_hash, started_at, final_state, retry_count)
		 VALUES (?, ?, ?, ?, ?)`,
		runID, baseHash, time.Now().UTC(), string(StateIdle), 0,
	)
	return err
}

// FinishRun updates the final state and end time of a run.
func (s *Store) FinishRun(runID string, state State, retryCount int) error {
	_, err := s.db.Exec(
		`UPDATE processing_runs SET ended_at = ?, final_state = ?, retry_count = ? WHERE run_id = ?`,
		time.Now().UTC(), string(state), retryCount, runID,
	)
	return err
}

// HasSuccessfulRun reports whether a run with the given base_hash already finished successfully.
func (s *Store) HasSuccessfulRun(baseHash string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM processing_runs WHERE base_hash = ? AND final_state = ?`,
		baseHash, string(StateDone),
	).Scan(&count)
	return count > 0, err
}

// AppendTransitionLog persists a single transition log entry.
func (s *Store) AppendTransitionLog(entry TransitionLog) error {
	_, err := s.db.Exec(
		`INSERT INTO transition_logs (run_id, from_state, to_state, event, reason, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.RunID, string(entry.From), string(entry.To), string(entry.Event), entry.Reason, entry.Timestamp,
	)
	return err
}

// ChangedFiles compares the current files against the stored snapshots.
// It returns files whose content hash or mtime differs, plus files not yet seen.
func (s *Store) ChangedFiles(files []FileInfo) ([]FileInfo, error) {
	changed := make([]FileInfo, 0, len(files))
	for _, f := range files {
		var storedHash string
		var storedMtime time.Time
		err := s.db.QueryRow(
			`SELECT content_hash, mtime FROM file_snapshots WHERE path = ?`, f.Path,
		).Scan(&storedHash, &storedMtime)
		if err == sql.ErrNoRows {
			changed = append(changed, f)
			continue
		}
		if err != nil {
			return nil, err
		}
		if storedHash != f.Hash || !storedMtime.Equal(f.ModTime) {
			changed = append(changed, f)
		}
	}
	return changed, nil
}

// UpdateSnapshot stores or updates the snapshot for a single file.
func (s *Store) UpdateSnapshot(f FileInfo) error {
	_, err := s.db.Exec(
		`INSERT INTO file_snapshots (path, content_hash, mtime, processed_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		   content_hash = excluded.content_hash,
		   mtime = excluded.mtime,
		   processed_at = excluded.processed_at`,
		f.Path, f.Hash, f.ModTime, time.Now().UTC(),
	)
	return err
}

// InsertTodo creates a new todo.
func (s *Store) InsertTodo(text, sourceFile string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO todos (text, status, source_file, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		text, "active", sourceFile, now, now,
	)
	return err
}

// InsertMemory creates a new memory.
func (s *Store) InsertMemory(topic, summary, sourceFile string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO memories (topic, summary, source_file, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		topic, summary, sourceFile, now, now,
	)
	return err
}

// ListActiveTodos returns active todos.
func (s *Store) ListActiveTodos() ([]Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, text, status, source_file, created_at, updated_at FROM todos WHERE status = 'active' ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Text, &t.Status, &t.SourceFile, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// ListMemories returns memories grouped by topic-like ordering.
func (s *Store) ListMemories() ([]Memory, error) {
	rows, err := s.db.Query(
		`SELECT id, topic, summary, source_file, created_at, updated_at FROM memories ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Topic, &m.Summary, &m.SourceFile, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// FileInfo describes a diary file discovered on disk.
type FileInfo struct {
	Path    string
	Hash    string
	ModTime time.Time
}

// HashContent returns a SHA-256 hex digest of content.
func HashContent(b []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

// Todo is an extracted action item.
type Todo struct {
	ID         int
	Text       string
	Status     string
	SourceFile string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Memory is an extracted piece of knowledge.
type Memory struct {
	ID         int
	Topic      string
	Summary    string
	SourceFile string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
