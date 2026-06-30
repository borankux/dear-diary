package process

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

// Store persists processing state and extracted entities in SQLite.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) the SQLite database at the given path.
// If path is empty, it first checks DIARY_DB_PATH env, then defaults to
// ~/.local/share/dear-diary/process.db.
func NewStore(path string) (*Store, error) {
	if path == "" {
		path = os.Getenv("DIARY_DB_PATH")
	}
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

CREATE TABLE IF NOT EXISTS ai_candidates (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	candidate_type TEXT NOT NULL,
	title TEXT,
	content TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	source_file TEXT NOT NULL,
	source_date TEXT,
	source_hash TEXT,
	evidence_text TEXT,
	raw_ai_json TEXT,
	confidence REAL,
	content_key TEXT,
	final_item_type TEXT,
	final_item_id INTEGER,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_transition_logs_run_id ON transition_logs(run_id);
CREATE INDEX IF NOT EXISTS idx_ai_candidates_status ON ai_candidates(status);
CREATE INDEX IF NOT EXISTS idx_ai_candidates_type ON ai_candidates(candidate_type);
CREATE INDEX IF NOT EXISTS idx_ai_candidates_source_hash ON ai_candidates(source_hash);
CREATE INDEX IF NOT EXISTS idx_ai_candidates_content_key ON ai_candidates(candidate_type, source_hash, content_key);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	return s.ensureSchemaCompatibility()
}

func (s *Store) ensureSchemaCompatibility() error {
	columns := map[string][]string{
		"todos": {
			"source_date TEXT",
			"source_hash TEXT",
			"evidence_text TEXT",
			"created_from_candidate_id INTEGER",
			"completed_at DATETIME",
			"priority INTEGER",
		},
		"memories": {
			"status TEXT NOT NULL DEFAULT 'active'",
			"source_date TEXT",
			"source_hash TEXT",
			"evidence_text TEXT",
			"created_from_candidate_id INTEGER",
			"archived_at DATETIME",
		},
	}
	for table, defs := range columns {
		for _, def := range defs {
			if err := s.addColumnIfMissing(table, def); err != nil {
				return err
			}
		}
	}
	_, err := s.db.Exec(`
CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
CREATE INDEX IF NOT EXISTS idx_todos_status_priority ON todos(status, priority ASC, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_memories_status ON memories(status);
`)
	return err
}

func (s *Store) addColumnIfMissing(table, definition string) error {
	name := strings.Fields(definition)[0]
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if colName == name {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + definition)
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
	return s.InsertTodoFromCandidate(text, sourceFile, "", "", "", 0)
}

// InsertTodoFromCandidate creates a new active todo with source evidence.
func (s *Store) InsertTodoFromCandidate(text, sourceFile, sourceDate, sourceHash, evidenceText string, candidateID int) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO todos (text, status, source_file, source_date, source_hash, evidence_text, created_from_candidate_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		text, "active", sourceFile, sourceDate, sourceHash, evidenceText, nullableCandidateID(candidateID), now, now,
	)
	return err
}

// InsertMemory creates a new memory.
func (s *Store) InsertMemory(topic, summary, sourceFile string) error {
	return s.InsertMemoryFromCandidate(topic, summary, sourceFile, "", "", "", 0)
}

// InsertMemoryFromCandidate creates a new active memory with source evidence.
func (s *Store) InsertMemoryFromCandidate(topic, summary, sourceFile, sourceDate, sourceHash, evidenceText string, candidateID int) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`INSERT INTO memories (topic, summary, status, source_file, source_date, source_hash, evidence_text, created_from_candidate_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		topic, summary, "active", sourceFile, sourceDate, sourceHash, evidenceText, nullableCandidateID(candidateID), now, now,
	)
	return err
}

func nullableCandidateID(id int) any {
	if id == 0 {
		return nil
	}
	return id
}

// ListActiveTodos returns active todos.
func (s *Store) ListActiveTodos() ([]Todo, error) {
	return s.ListTodosByStatus(TodoStatusActive)
}

// ListTodosByStatus returns todos in one lifecycle state for dashboard review.
func (s *Store) ListTodosByStatus(status string) ([]Todo, error) {
	status, err := normalizeTodoStatus(status)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT id, text, status, source_file, source_date, source_hash, evidence_text, created_at, updated_at, priority
		 FROM todos
		 WHERE status = ?
		 ORDER BY
		   CASE WHEN priority IS NULL THEN 1 ELSE 0 END,
		   priority DESC,
		   updated_at DESC,
		   created_at DESC,
		   id DESC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTodos(rows)
}

func scanTodos(rows *sql.Rows) ([]Todo, error) {
	var todos []Todo
	for rows.Next() {
		var t Todo
		var sourceFile, sourceDate, sourceHash, evidenceText sql.NullString
		var priority sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Text, &t.Status, &sourceFile, &sourceDate, &sourceHash, &evidenceText, &t.CreatedAt, &t.UpdatedAt, &priority); err != nil {
			return nil, err
		}
		t.SourceFile = sourceFile.String
		t.SourceDate = sourceDate.String
		t.SourceHash = sourceHash.String
		t.EvidenceText = evidenceText.String
		if priority.Valid {
			t.HasPriority = true
			t.Priority = int(priority.Int64)
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// ListMemories returns memories grouped by topic-like ordering.
func (s *Store) ListMemories() ([]Memory, error) {
	rows, err := s.db.Query(
		`SELECT id, topic, summary, status, source_file, source_date, source_hash, evidence_text, created_at, updated_at
		 FROM memories
		 WHERE COALESCE(status, 'active') = 'active'
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		var sourceFile, sourceDate, sourceHash, evidenceText sql.NullString
		if err := rows.Scan(&m.ID, &m.Topic, &m.Summary, &m.Status, &sourceFile, &sourceDate, &sourceHash, &evidenceText, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.SourceFile = sourceFile.String
		m.SourceDate = sourceDate.String
		m.SourceHash = sourceHash.String
		m.EvidenceText = evidenceText.String
		memories = append(memories, m)
	}
	return memories, rows.Err()
}

// InsertCandidateIfNew stores a pending AI candidate unless the same source/content
// has already been seen in any status, including rejected.
func (s *Store) InsertCandidateIfNew(c Candidate) (bool, error) {
	if c.Type == "" || strings.TrimSpace(c.Content) == "" || c.SourceFile == "" {
		return false, errors.New("candidate requires type, content, and source_file")
	}
	c.ContentKey = normalize(c.Type + " " + c.Title + " " + c.Content + " " + c.EvidenceText)
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM ai_candidates
		 WHERE candidate_type = ? AND content_key = ? AND (
		   (COALESCE(?, '') != '' AND COALESCE(source_date, '') = COALESCE(?, ''))
		   OR (COALESCE(?, '') = '' AND source_file = ?)
		 )`,
		c.Type, c.ContentKey, c.SourceDate, c.SourceDate, c.SourceDate, c.SourceFile,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	now := time.Now().UTC()
	_, err = s.db.Exec(
		`INSERT INTO ai_candidates
		 (candidate_type, title, content, status, source_file, source_date, source_hash, evidence_text, raw_ai_json, confidence, content_key, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Type, c.Title, c.Content, CandidateStatusPending, c.SourceFile, c.SourceDate, c.SourceHash,
		c.EvidenceText, c.RawAIJSON, c.Confidence, c.ContentKey, now, now,
	)
	return err == nil, err
}

// ListPendingCandidates returns candidates waiting for human review.
func (s *Store) ListPendingCandidates() ([]Candidate, error) {
	return s.listCandidatesByStatus(CandidateStatusPending)
}

func (s *Store) listCandidatesByStatus(status string) ([]Candidate, error) {
	rows, err := s.db.Query(
		`SELECT id, candidate_type, title, content, status, source_file, source_date, source_hash,
		        evidence_text, raw_ai_json, confidence, content_key, final_item_type, final_item_id,
		        created_at, updated_at
		 FROM ai_candidates
		 WHERE status = ?
		 ORDER BY created_at ASC, id ASC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []Candidate
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// PendingCandidateCount returns the number of candidates waiting for review.
func (s *Store) PendingCandidateCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM ai_candidates WHERE status = ?`, CandidateStatusPending).Scan(&count)
	return count, err
}

// CandidateStatusCounts returns lifecycle counts for the review queue.
func (s *Store) CandidateStatusCounts() (CandidateCounts, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM ai_candidates GROUP BY status`)
	if err != nil {
		return CandidateCounts{}, err
	}
	defer rows.Close()

	var counts CandidateCounts
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return CandidateCounts{}, err
		}
		switch status {
		case CandidateStatusPending:
			counts.Pending = count
		case CandidateStatusAccepted:
			counts.Accepted = count
		case CandidateStatusRejected:
			counts.Rejected = count
		}
	}
	return counts, rows.Err()
}

// TodoStatusCounts returns lifecycle counts for todo board columns.
func (s *Store) TodoStatusCounts() (TodoCounts, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM todos GROUP BY status`)
	if err != nil {
		return TodoCounts{}, err
	}
	defer rows.Close()

	var counts TodoCounts
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return TodoCounts{}, err
		}
		switch status {
		case TodoStatusActive:
			counts.Active = count
		case TodoStatusInProgress:
			counts.InProgress = count
		case TodoStatusDone:
			counts.Done = count
		case TodoStatusWontDo:
			counts.WontDo = count
		case TodoStatusArchived:
			counts.Archived = count
		case TodoStatusOther:
			counts.Other = count
		default:
			counts.Other += count
		}
	}
	return counts, rows.Err()
}

// GetCandidate loads one candidate by id.
func (s *Store) GetCandidate(id int) (Candidate, error) {
	row := s.db.QueryRow(
		`SELECT id, candidate_type, title, content, status, source_file, source_date, source_hash,
		        evidence_text, raw_ai_json, confidence, content_key, final_item_type, final_item_id,
		        created_at, updated_at
		 FROM ai_candidates
		 WHERE id = ?`,
		id,
	)
	return scanCandidate(row)
}

// RejectCandidate marks a pending candidate as rejected.
func (s *Store) RejectCandidate(id int) error {
	_, err := s.db.Exec(
		`UPDATE ai_candidates SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		CandidateStatusRejected, time.Now().UTC(), id, CandidateStatusPending,
	)
	return err
}

// AcceptCandidate promotes a pending candidate into its final table.
func (s *Store) AcceptCandidate(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	c, err := getCandidateTx(tx, id)
	if err != nil {
		return err
	}
	if c.Status != CandidateStatusPending {
		return fmt.Errorf("candidate %d is %s, not pending", id, c.Status)
	}

	now := time.Now().UTC()
	var finalType string
	var finalID int64
	switch c.Type {
	case CandidateTypeTodo:
		text := c.Title
		if text == "" {
			text = c.Content
		}
		res, err := tx.Exec(
			`INSERT INTO todos (text, status, source_file, source_date, source_hash, evidence_text, created_from_candidate_id, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			text, "active", c.SourceFile, c.SourceDate, c.SourceHash, c.EvidenceText, c.ID, now, now,
		)
		if err != nil {
			return err
		}
		finalType = CandidateTypeTodo
		finalID, err = res.LastInsertId()
		if err != nil {
			return err
		}
	case CandidateTypeMemory:
		topic := c.Title
		if topic == "" {
			topic = "Untitled"
		}
		res, err := tx.Exec(
			`INSERT INTO memories (topic, summary, status, source_file, source_date, source_hash, evidence_text, created_from_candidate_id, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			topic, c.Content, "active", c.SourceFile, c.SourceDate, c.SourceHash, c.EvidenceText, c.ID, now, now,
		)
		if err != nil {
			return err
		}
		finalType = CandidateTypeMemory
		finalID, err = res.LastInsertId()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported candidate type %q", c.Type)
	}

	_, err = tx.Exec(
		`UPDATE ai_candidates
		 SET status = ?, final_item_type = ?, final_item_id = ?, updated_at = ?
		 WHERE id = ?`,
		CandidateStatusAccepted, finalType, finalID, now, id,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// PromoteAllPendingCandidates promotes every pending candidate, then archives
// obvious duplicate active items created by the promotion.
func (s *Store) PromoteAllPendingCandidates() (BulkPromotionResult, error) {
	candidates, err := s.ListPendingCandidates()
	if err != nil {
		return BulkPromotionResult{}, err
	}
	var result BulkPromotionResult
	for _, c := range candidates {
		if err := s.AcceptCandidate(c.ID); err != nil {
			return result, err
		}
		switch c.Type {
		case CandidateTypeTodo:
			result.PromotedTodos++
		case CandidateTypeMemory:
			result.PromotedMemories++
		}
	}
	merged, err := s.MergeDuplicateItems()
	if err != nil {
		return result, err
	}
	result.Merge = merged
	return result, nil
}

// MergeDuplicateItems collapses obvious duplicates without deleting history.
// Pending candidate duplicates are dismissed and point at the kept candidate.
// Active todo/memory duplicates are archived so they disappear from working views.
func (s *Store) MergeDuplicateItems() (MergeResult, error) {
	var result MergeResult

	candidates, err := s.ListPendingCandidates()
	if err != nil {
		return result, err
	}
	candidateGroups := groupCandidatesBySignature(candidates)
	now := time.Now().UTC()
	for _, group := range candidateGroups {
		keep := keepCandidate(group)
		dupes := 0
		for _, c := range group {
			if c.ID == keep.ID {
				continue
			}
			res, err := s.db.Exec(
				`UPDATE ai_candidates
				 SET status = ?, final_item_type = ?, final_item_id = ?, updated_at = ?
				 WHERE id = ? AND status = ?`,
				CandidateStatusRejected, "candidate", keep.ID, now, c.ID, CandidateStatusPending,
			)
			if err != nil {
				return result, err
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return result, err
			}
			if affected > 0 {
				dupes++
			}
		}
		if dupes > 0 {
			result.CandidateGroups++
			result.CandidateMerged += dupes
		}
	}

	todos, err := s.listMergeableTodos()
	if err != nil {
		return result, err
	}
	todoGroups := groupTodosBySignature(todos)
	now = time.Now().UTC()
	for _, group := range todoGroups {
		keep := keepTodo(group)
		dupes := 0
		for _, todo := range group {
			if todo.ID == keep.ID {
				continue
			}
			res, err := s.db.Exec(
				`UPDATE todos SET status = ?, updated_at = ?
				 WHERE id = ? AND status IN (?, ?)`,
				TodoStatusArchived, now, todo.ID, TodoStatusActive, TodoStatusInProgress,
			)
			if err != nil {
				return result, err
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return result, err
			}
			if affected > 0 {
				dupes++
			}
		}
		if dupes > 0 {
			result.TodoGroups++
			result.TodoMerged += dupes
		}
	}

	memories, err := s.ListMemories()
	if err != nil {
		return result, err
	}
	memoryGroups := groupMemoriesBySignature(memories)
	now = time.Now().UTC()
	for _, group := range memoryGroups {
		keep := keepMemory(group)
		dupes := 0
		for _, memory := range group {
			if memory.ID == keep.ID {
				continue
			}
			res, err := s.db.Exec(
				`UPDATE memories SET status = ?, archived_at = ?, updated_at = ?
				 WHERE id = ? AND COALESCE(status, 'active') = 'active'`,
				TodoStatusArchived, now, now, memory.ID,
			)
			if err != nil {
				return result, err
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return result, err
			}
			if affected > 0 {
				dupes++
			}
		}
		if dupes > 0 {
			result.MemoryGroups++
			result.MemoryMerged += dupes
		}
	}

	return result, nil
}

func (s *Store) listMergeableTodos() ([]Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, text, status, source_file, source_date, source_hash, evidence_text, created_at, updated_at, priority
		 FROM todos
		 WHERE status IN (?, ?)
		 ORDER BY updated_at DESC, id ASC`,
		TodoStatusInProgress, TodoStatusActive,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTodos(rows)
}

func groupCandidatesBySignature(candidates []Candidate) [][]Candidate {
	groups := make(map[string][]Candidate)
	for _, c := range candidates {
		key := c.Type + ":" + duplicateSignature(c.Title+" "+c.Content)
		if key == c.Type+":" {
			continue
		}
		groups[key] = append(groups[key], c)
	}
	return candidateDuplicateGroups(groups)
}

func candidateDuplicateGroups(groups map[string][]Candidate) [][]Candidate {
	result := make([][]Candidate, 0)
	for _, group := range groups {
		if len(group) > 1 {
			result = append(result, group)
		}
	}
	return result
}

func groupTodosBySignature(todos []Todo) [][]Todo {
	groups := make(map[string][]Todo)
	for _, todo := range todos {
		key := duplicateSignature(todo.Text)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], todo)
	}
	result := make([][]Todo, 0)
	for _, group := range groups {
		if len(group) > 1 {
			result = append(result, group)
		}
	}
	return result
}

func groupMemoriesBySignature(memories []Memory) [][]Memory {
	groups := make(map[string][]Memory)
	for _, memory := range memories {
		key := duplicateSignature(memory.Topic + " " + memory.Summary)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], memory)
	}
	result := make([][]Memory, 0)
	for _, group := range groups {
		if len(group) > 1 {
			result = append(result, group)
		}
	}
	return result
}

func duplicateSignature(text string) string {
	tokens := duplicateTokens(text)
	if len(tokens) < 2 {
		return ""
	}
	sort.Strings(tokens)
	return strings.Join(tokens, "|")
}

func duplicateTokens(text string) []string {
	text = strings.ToLower(text)
	latinTokens := make([]string, 0)
	var current strings.Builder
	var cjk []rune
	flushLatin := func() {
		if current.Len() == 0 {
			return
		}
		token := current.String()
		current.Reset()
		if isDuplicateStopToken(token) || len([]rune(token)) < 2 {
			return
		}
		latinTokens = append(latinTokens, token)
	}
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			flushLatin()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			current.WriteRune(r)
		default:
			flushLatin()
		}
	}
	flushLatin()

	tokenSet := make(map[string]struct{})
	for _, token := range latinTokens {
		tokenSet[token] = struct{}{}
	}
	for i := 0; i+1 < len(cjk); i++ {
		token := string(cjk[i : i+2])
		if !isDuplicateStopToken(token) {
			tokenSet[token] = struct{}{}
		}
	}
	tokens := make([]string, 0, len(tokenSet))
	for token := range tokenSet {
		tokens = append(tokens, token)
	}
	return tokens
}

func isDuplicateStopToken(token string) bool {
	switch token {
	case "the", "and", "or", "for", "with", "from", "this", "that", "todo",
		"一个", "一下", "这个", "那个", "需要", "进行", "完成", "处理", "继续", "相关", "关于", "以及", "可以", "应该", "目前", "今天", "明天", "后面":
		return true
	default:
		return false
	}
}

func keepCandidate(group []Candidate) Candidate {
	best := group[0]
	bestScore := candidateKeepScore(best)
	for _, c := range group[1:] {
		score := candidateKeepScore(c)
		if score > bestScore || (score == bestScore && c.UpdatedAt.After(best.UpdatedAt)) {
			best = c
			bestScore = score
		}
	}
	return best
}

func candidateKeepScore(c Candidate) int {
	return len([]rune(c.Title)) + len([]rune(c.Content)) + len([]rune(c.EvidenceText))
}

func keepTodo(group []Todo) Todo {
	best := group[0]
	for _, todo := range group[1:] {
		if todoRank(todo) > todoRank(best) || (todoRank(todo) == todoRank(best) && todo.UpdatedAt.After(best.UpdatedAt)) {
			best = todo
		}
	}
	return best
}

func todoRank(todo Todo) int {
	rank := 0
	if todo.Status == TodoStatusInProgress {
		rank += 1000
	}
	if todo.HasPriority {
		rank += todo.Priority
	}
	return rank
}

func keepMemory(group []Memory) Memory {
	best := group[0]
	bestScore := len([]rune(best.Topic)) + len([]rune(best.Summary))
	for _, memory := range group[1:] {
		score := len([]rune(memory.Topic)) + len([]rune(memory.Summary))
		if score > bestScore || (score == bestScore && memory.UpdatedAt.After(best.UpdatedAt)) {
			best = memory
			bestScore = score
		}
	}
	return best
}

// MarkTodoDone closes an active todo as completed.
func (s *Store) MarkTodoDone(id int) error {
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`UPDATE todos SET status = ?, completed_at = ?, updated_at = ?
		 WHERE id = ? AND status IN (?, ?)`,
		TodoStatusDone, now, now, id, TodoStatusActive, TodoStatusInProgress,
	)
	return requireAffected(res, err, "todo")
}

// ArchiveTodo removes an active todo from the working list without marking it done.
func (s *Store) ArchiveTodo(id int) error {
	res, err := s.db.Exec(
		`UPDATE todos SET status = ?, updated_at = ? WHERE id = ? AND status IN (?, ?, ?)`,
		TodoStatusArchived, time.Now().UTC(), id, TodoStatusActive, TodoStatusInProgress, TodoStatusOther,
	)
	return requireAffected(res, err, "todo")
}

// SetTodoStatus updates a todo lifecycle status. It is intentionally small so AI
// workflows can classify todos without writing SQL directly.
func (s *Store) SetTodoStatus(id int, status string) error {
	status, err := normalizeTodoStatus(status)
	if err != nil {
		return err
	}
	var completedAt any
	if status == TodoStatusDone {
		completedAt = time.Now().UTC()
	}
	res, err := s.db.Exec(
		`UPDATE todos SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`,
		status, completedAt, time.Now().UTC(), id,
	)
	return requireAffected(res, err, "todo")
}

// SetTodoPriority stores an optional 0-100 priority where higher means more important.
func (s *Store) SetTodoPriority(id int, priority *int) error {
	var value any
	if priority != nil {
		if *priority < 0 || *priority > 100 {
			return fmt.Errorf("priority must be between 0 and 100")
		}
		value = *priority
	}
	res, err := s.db.Exec(
		`UPDATE todos SET priority = ?, updated_at = ? WHERE id = ?`,
		value, time.Now().UTC(), id,
	)
	return requireAffected(res, err, "todo")
}

func requireAffected(res sql.Result, err error, label string) error {
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%s not found or not in a valid state", label)
	}
	return nil
}

type candidateScanner interface {
	Scan(dest ...any) error
}

func scanCandidate(row candidateScanner) (Candidate, error) {
	var c Candidate
	var finalID sql.NullInt64
	var title, sourceDate, sourceHash, evidenceText, rawAIJSON, contentKey, finalItemType sql.NullString
	var confidence sql.NullFloat64
	if err := row.Scan(
		&c.ID, &c.Type, &title, &c.Content, &c.Status, &c.SourceFile, &sourceDate, &sourceHash,
		&evidenceText, &rawAIJSON, &confidence, &contentKey, &finalItemType, &finalID,
		&c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return Candidate{}, err
	}
	c.Title = title.String
	c.SourceDate = sourceDate.String
	c.SourceHash = sourceHash.String
	c.EvidenceText = evidenceText.String
	c.RawAIJSON = rawAIJSON.String
	if confidence.Valid {
		c.Confidence = confidence.Float64
	}
	c.ContentKey = contentKey.String
	c.FinalItemType = finalItemType.String
	if finalID.Valid {
		c.FinalItemID = int(finalID.Int64)
	}
	return c, nil
}

func getCandidateTx(tx *sql.Tx, id int) (Candidate, error) {
	row := tx.QueryRow(
		`SELECT id, candidate_type, title, content, status, source_file, source_date, source_hash,
		        evidence_text, raw_ai_json, confidence, content_key, final_item_type, final_item_id,
		        created_at, updated_at
		 FROM ai_candidates
		 WHERE id = ?`,
		id,
	)
	return scanCandidate(row)
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
	ID           int
	Text         string
	Status       string
	Priority     int
	HasPriority  bool
	SourceFile   string
	SourceDate   string
	SourceHash   string
	EvidenceText string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TodoCounts summarizes todo lifecycle state.
type TodoCounts struct {
	Active     int
	InProgress int
	Done       int
	WontDo     int
	Archived   int
	Other      int
}

// Memory is an extracted piece of knowledge.
type Memory struct {
	ID           int
	Topic        string
	Summary      string
	Status       string
	SourceFile   string
	SourceDate   string
	SourceHash   string
	EvidenceText string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const (
	TodoStatusActive     = "active"
	TodoStatusInProgress = "in_progress"
	TodoStatusDone       = "done"
	TodoStatusWontDo     = "wont_do"
	TodoStatusArchived   = "archived"
	TodoStatusOther      = "other"

	CandidateTypeTodo   = "todo"
	CandidateTypeMemory = "memory"

	CandidateStatusPending  = "pending"
	CandidateStatusAccepted = "accepted"
	CandidateStatusRejected = "rejected"
)

func normalizeTodoStatus(status string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "", "todo", "open", "backlog", TodoStatusActive:
		return TodoStatusActive, nil
	case "doing", "started", "inprogress", TodoStatusInProgress:
		return TodoStatusInProgress, nil
	case "complete", "completed", TodoStatusDone:
		return TodoStatusDone, nil
	case "wontdo", "wont", "cancelled", "canceled", "not_doing", TodoStatusWontDo:
		return TodoStatusWontDo, nil
	case "archive", TodoStatusArchived:
		return TodoStatusArchived, nil
	case TodoStatusOther:
		return TodoStatusOther, nil
	default:
		return "", fmt.Errorf("unsupported todo status %q", status)
	}
}

// CandidateCounts summarizes review lifecycle state.
type CandidateCounts struct {
	Pending  int
	Accepted int
	Rejected int
}

// MergeResult summarizes duplicate cleanup across working tables.
type MergeResult struct {
	CandidateGroups int
	CandidateMerged int
	TodoGroups      int
	TodoMerged      int
	MemoryGroups    int
	MemoryMerged    int
}

// BulkPromotionResult summarizes one-click promotion from AI Inbox.
type BulkPromotionResult struct {
	PromotedTodos    int
	PromotedMemories int
	Merge            MergeResult
}

// Candidate is an AI-proposed item waiting for human review.
type Candidate struct {
	ID            int
	Type          string
	Title         string
	Content       string
	Status        string
	SourceFile    string
	SourceDate    string
	SourceHash    string
	EvidenceText  string
	RawAIJSON     string
	Confidence    float64
	ContentKey    string
	FinalItemType string
	FinalItemID   int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
