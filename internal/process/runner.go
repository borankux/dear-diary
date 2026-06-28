package process

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/borankux/dear-diary/internal/storage"
	"github.com/google/uuid"
)

// Runner executes one process run from start to finish using the state machine.
type Runner struct {
	store        *Store
	scanner      *Scanner
	extractor    *Extractor
	deduplicator *Deduplicator
	writer       *SummaryWriter
	htmlWriter   *HTMLWriter
	runID        string
	baseHash     string
	machine      *Machine
	changed      []FileInfo
	extracted    map[string]*Extracted
	candidates   []Candidate
	inserted     []Candidate
	deduped      *DedupResult
}

// NewRunner builds a runner for the given diary root and output directory.
func NewRunner(rootDir, outDir string) (*Runner, error) {
	return NewRunnerWithStore(rootDir, outDir, "")
}

// NewRunnerWithStore builds a runner with an explicit SQLite path.
// If storePath is empty, the default path is used.
func NewRunnerWithStore(rootDir, outDir, storePath string) (*Runner, error) {
	store, err := NewStore(storePath)
	if err != nil {
		return nil, err
	}
	extractor, err := NewExtractor()
	if err != nil {
		store.Close()
		return nil, err
	}
	return &Runner{
		store:        store,
		scanner:      NewScanner(rootDir),
		extractor:    extractor,
		deduplicator: NewDeduplicator(store),
		writer:       NewSummaryWriter(outDir),
		htmlWriter:   NewHTMLWriter(outDir, rootDir),
		extracted:    make(map[string]*Extracted),
		deduped:      &DedupResult{},
	}, nil
}

// Close releases resources.
func (r *Runner) Close() error {
	return r.store.Close()
}

// Run executes the full processing pipeline.
func (r *Runner) Run() error {
	defer r.persistLog()

	// 1. Build run identity and check idempotency.
	files, err := r.scanner.AllFiles()
	if err != nil {
		return err
	}
	r.baseHash = computeBaseHash(files)
	r.runID = uuid.NewString()

	r.machine = NewMachine(r.runID, r.baseHash, 2)
	if err := r.store.CreateRun(r.runID, r.baseHash); err != nil {
		return err
	}

	// Idempotency guard.
	done, err := r.store.HasSuccessfulRun(r.baseHash)
	if err != nil {
		return err
	}
	if done {
		if err := r.transition(EventIdempotencyHit, "already processed"); err != nil {
			return err
		}
		r.printReport()
		return nil
	}

	// 2. Start the machine.
	if err := r.transition(EventStartProcess, ""); err != nil {
		return err
	}

	// 3. Scanning.
	recent, err := r.scanner.RecentFiles(3)
	if err != nil {
		return err
	}
	r.changed, err = r.store.ChangedFiles(recent)
	if err != nil {
		return err
	}
	if len(r.changed) == 0 {
		if err := r.transition(EventNoChanges, "no changed files in last 3 days"); err != nil {
			return err
		}
		r.printReport()
		return nil
	}
	if err := r.transition(EventChangesFound, fmt.Sprintf("%d changed files", len(r.changed))); err != nil {
		return err
	}

	// 4. Reading.
	contents, err := r.readContents(r.changed)
	if err != nil {
		r.forceFatal(err.Error())
		return err
	}
	if err := r.transition(EventContentLoaded, fmt.Sprintf("loaded %d files", len(contents))); err != nil {
		return err
	}

	// 5. Extracting.
	fmt.Printf("AI Mode: sending %d diary file(s) to configured LLM provider: %s\n", len(contents), r.extractor.ProviderSummary())
	if err := r.extract(contents); err != nil {
		r.forceFatal(err.Error())
		return err
	}
	if err := r.transition(EventExtractionOK, fmt.Sprintf("extracted from %d files", len(r.extracted))); err != nil {
		return err
	}

	// 6. Deduplicating.
	if err := r.dedup(); err != nil {
		r.forceFatal(err.Error())
		return err
	}
	if err := r.transition(EventDuplicatesResolved, fmt.Sprintf("kept %d new candidates", len(r.candidates))); err != nil {
		return err
	}

	// 7. Candidate writing. v0.4 does not write AI output directly to final tables.
	if err := r.merge(); err != nil {
		r.forceFatal(err.Error())
		return err
	}
	if err := r.transition(EventMergeComplete, fmt.Sprintf("created %d pending candidates", len(r.inserted))); err != nil {
		return err
	}

	// 8. Persisting.
	if err := r.persistSnapshots(); err != nil {
		r.forceFatal(err.Error())
		return err
	}
	if err := r.transition(EventPersistOK, "snapshots persisted"); err != nil {
		return err
	}

	// 9. Summarizing.
	if err := r.writer.WriteAll(r.store); err != nil {
		r.forceFatal(err.Error())
		return err
	}
	if err := r.htmlWriter.WriteAll(r.store); err != nil {
		r.forceFatal(err.Error())
		return err
	}
	if err := r.transition(EventSummaryOK, "markdown and html summaries written"); err != nil {
		return err
	}

	// 10. Reporting.
	if err := r.transition(EventReportEmitted, "report printed"); err != nil {
		return err
	}

	r.printReport()
	return nil
}

func (r *Runner) transition(event Event, reason string) error {
	if err := r.machine.Transition(event, reason); err != nil {
		return err
	}
	last := r.machine.Log()[len(r.machine.Log())-1]
	return r.store.AppendTransitionLog(last)
}

func (r *Runner) forceFatal(reason string) {
	if r.machine == nil || r.store == nil {
		return
	}
	if err := r.machine.ForceFatal(reason); err != nil {
		return
	}
	log := r.machine.Log()
	if len(log) == 0 {
		return
	}
	_ = r.store.AppendTransitionLog(log[len(log)-1])
}

func (r *Runner) persistLog() {
	if r.machine == nil || r.store == nil {
		return
	}
	_ = r.store.FinishRun(r.runID, r.machine.State(), r.machine.RetryCount())
}

func (r *Runner) readContents(files []FileInfo) (map[string]string, error) {
	contents := make(map[string]string, len(files))
	for _, f := range files {
		b, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		contents[f.Path] = string(b)
	}
	return contents, nil
}

func (r *Runner) extract(contents map[string]string) error {
	for path, content := range contents {
		var lastErr error
		for attempt := 0; attempt <= r.machine.MaxRetries(); attempt++ {
			res, err := r.extractor.Extract(content)
			if err == nil {
				r.extracted[path] = res
				break
			}
			lastErr = err
			// MVP: treat all extraction errors as transient and retry.
			// In production, classify 401/4xx as fatal and network/5xx/timeout as transient.
			if attempt < r.machine.MaxRetries() {
				if txErr := r.transition(EventTransientAIError, err.Error()); txErr != nil {
					return txErr
				}
				if txErr := r.transition(EventRetry, fmt.Sprintf("attempt %d", attempt+1)); txErr != nil {
					return txErr
				}
				continue
			}
			return fmt.Errorf("extract %s: %w", path, lastErr)
		}
	}
	return nil
}

func (r *Runner) dedup() error {
	seen := make(map[string]struct{})
	files := make(map[string]FileInfo, len(r.changed))
	for _, f := range r.changed {
		files[f.Path] = f
	}
	for path, ext := range r.extracted {
		source := files[path]
		for _, c := range candidatesFromExtracted(ext, source) {
			key := normalize(c.Type + " " + c.SourceHash + " " + c.Title + " " + c.Content + " " + c.EvidenceText)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			r.candidates = append(r.candidates, c)
		}
	}
	return nil
}

func (r *Runner) merge() error {
	for _, c := range r.candidates {
		inserted, err := r.store.InsertCandidateIfNew(c)
		if err != nil {
			return err
		}
		if inserted {
			r.inserted = append(r.inserted, c)
		}
	}
	return nil
}

func (r *Runner) persistSnapshots() error {
	for _, f := range r.changed {
		if err := r.store.UpdateSnapshot(f); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) countTodos() int {
	c := 0
	for _, ext := range r.extracted {
		for _, item := range ext.Items {
			if item.normalizedType() == CandidateTypeTodo {
				c++
			}
		}
	}
	return c
}

func (r *Runner) countMemories() int {
	c := 0
	for _, ext := range r.extracted {
		for _, item := range ext.Items {
			if item.normalizedType() == CandidateTypeMemory {
				c++
			}
		}
	}
	return c
}

func (r *Runner) countNewTodos() int {
	c := 0
	for _, candidate := range r.inserted {
		if candidate.Type == CandidateTypeTodo {
			c++
		}
	}
	return c
}

func (r *Runner) countNewMemories() int {
	c := 0
	for _, candidate := range r.inserted {
		if candidate.Type == CandidateTypeMemory {
			c++
		}
	}
	return c
}

// State 返回最后一次 Run 的最终状态。若尚未运行，返回 StateIdle。
func (r *Runner) State() State {
	if r.machine == nil {
		return StateIdle
	}
	return r.machine.State()
}

func (r *Runner) printReport() {
	fmt.Println()
	fmt.Println("=== Dear Diary AI 提炼报告 ===")
	fmt.Printf("Run ID:     %s\n", r.runID)
	fmt.Printf("State:      %s\n", r.machine.State())

	switch r.machine.State() {
	case StateDone:
		fmt.Printf("扫描文件:      %d\n", len(r.changed))
		fmt.Printf("候选 Todo:     %d (新增 pending %d)\n", r.countTodos(), r.countNewTodos())
		fmt.Printf("候选 Memory:   %d (新增 pending %d)\n", r.countMemories(), r.countNewMemories())
		fmt.Printf("输出目录:      %s\n", r.writer.outDir)
	case StateNoChanges:
		fmt.Println("最近 3 天没有变更的日记，无需处理。")
	case StateAlreadyProcessed:
		fmt.Println("同一批日记已经处理过，跳过。")
	case StateFatalError:
		fmt.Println("处理过程中发生不可恢复错误，请查看日志。")
	}
	fmt.Println()
}

func candidatesFromExtracted(ext *Extracted, source FileInfo) []Candidate {
	var sourceDate string
	if d, ok := storage.DateFromDiaryPath(source.Path); ok {
		sourceDate = d.Format("2006-01-02")
	}
	candidates := make([]Candidate, 0, len(ext.Items))
	for _, item := range ext.Items {
		candidateType := item.normalizedType()
		if candidateType == "" {
			continue
		}
		title := strings.TrimSpace(item.Title)
		content := strings.TrimSpace(item.Content)
		if candidateType == CandidateTypeMemory {
			if title == "" {
				title = strings.TrimSpace(item.Topic)
			}
			if content == "" {
				content = strings.TrimSpace(item.Summary)
			}
		}
		if title == "" && content != "" {
			title = content
		}
		if content == "" {
			content = title
		}
		if content == "" {
			continue
		}
		candidates = append(candidates, Candidate{
			Type:         candidateType,
			Title:        title,
			Content:      content,
			Status:       CandidateStatusPending,
			SourceFile:   source.Path,
			SourceDate:   sourceDate,
			SourceHash:   source.Hash,
			EvidenceText: strings.TrimSpace(item.EvidenceText),
			RawAIJSON:    ext.RawJSON,
			Confidence:   item.Confidence,
		})
	}
	return candidates
}

func (item CandidateExtract) normalizedType() string {
	switch strings.ToLower(strings.TrimSpace(item.Type)) {
	case CandidateTypeTodo, "task", "action":
		return CandidateTypeTodo
	case CandidateTypeMemory, "memo", "insight", "fact":
		return CandidateTypeMemory
	default:
		return ""
	}
}

func computeBaseHash(files []FileInfo) string {
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s:%s:%d\n", f.Path, f.Hash, f.ModTime.Unix())
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ProcessOutDir returns the default output directory for processed summaries.
func ProcessOutDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "Documents", "dear-diary", "process")
}
