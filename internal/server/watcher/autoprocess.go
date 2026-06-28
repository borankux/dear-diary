package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/borankux/dear-diary/internal/process"
	"github.com/borankux/dear-diary/internal/storage"
)

// AutoProcess 自动处理引擎，从 Watcher 接收文件变更事件并驱动 AI 处理流水线。
type AutoProcess struct {
	dataDir  string
	dbPath   string
	notifier Notifier
	watcher  *Watcher
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.RWMutex
	running  bool
}

// NewAutoProcess 创建自动处理引擎。
// dataDir 是日记根目录，dbPath 是 SQLite 数据库路径（可为空，使用默认路径）。
func NewAutoProcess(dataDir, dbPath string, notifier Notifier) (*AutoProcess, error) {
	w, err := New(dataDir, notifier)
	if err != nil {
		return nil, err
	}
	return &AutoProcess{
		dataDir:  dataDir,
		dbPath:   dbPath,
		notifier: notifier,
		watcher:  w,
	}, nil
}

// Start 启动后台 goroutine，从 watcher 接收事件并处理。
func (ap *AutoProcess) Start() {
	ap.mu.Lock()
	if ap.running {
		ap.mu.Unlock()
		return
	}
	ap.ctx, ap.cancel = context.WithCancel(context.Background())
	ap.running = true
	ap.mu.Unlock()

	// 启动 watcher（阻塞方法，需放在 goroutine 中）
	ap.wg.Add(1)
	go func() {
		defer ap.wg.Done()
		if err := ap.watcher.Start(); err != nil {
			log.Printf("【autoprocess】watcher 启动失败: %v", err)
		}
	}()

	// 监听 watcher 事件并触发处理
	ap.wg.Add(1)
	go func() {
		defer ap.wg.Done()
		for {
			select {
			case <-ap.ctx.Done():
				return
			case path, ok := <-ap.watcher.Events:
				if !ok {
					return
				}
				log.Printf("【autoprocess】检测到文件变化: %s", path)
				ap.runFullProcess()
			}
		}
	}()

	// 定时全量扫描
	watchInterval := ap.getWatchInterval()
	ticker := time.NewTicker(watchInterval)
	ap.wg.Add(1)
	go func() {
		defer ap.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-ap.ctx.Done():
				return
			case <-ticker.C:
				log.Printf("【autoprocess】定时全量扫描触发")
				ap.runFullProcess()
			}
		}
	}()
}

// Stop 停止自动处理引擎。
func (ap *AutoProcess) Stop() {
	ap.mu.Lock()
	if !ap.running {
		ap.mu.Unlock()
		return
	}
	ap.mu.Unlock()

	ap.cancel()
	ap.watcher.Stop()
	ap.wg.Wait()

	ap.mu.Lock()
	ap.running = false
	ap.mu.Unlock()
}

func (ap *AutoProcess) isAutoProcessEnabled() bool {
	return os.Getenv("DIARY_AUTO_PROCESS") != "false"
}

func (ap *AutoProcess) getWatchInterval() time.Duration {
	val := os.Getenv("DIARY_WATCH_INTERVAL")
	if val == "" {
		val = "30s"
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Printf("【autoprocess】解析 DIARY_WATCH_INTERVAL 失败，使用默认 30s: %v", err)
		return 30 * time.Second
	}
	return d
}

func (ap *AutoProcess) getOutDir() string {
	if os.Getenv("DIARY_DATA_DIR") != "" {
		return filepath.Join(ap.dataDir, "process")
	}
	return process.ProcessOutDir()
}

// runFullProcess 执行一次完整的处理流水线。
func (ap *AutoProcess) runFullProcess() {
	if !ap.isAutoProcessEnabled() {
		log.Printf("【autoprocess】自动处理已禁用，跳过")
		return
	}

	outDir := ap.getOutDir()
	var runner *process.Runner
	var err error

	if ap.dbPath != "" {
		runner, err = process.NewRunnerWithStore(ap.dataDir, outDir, ap.dbPath)
	} else {
		runner, err = process.NewRunner(ap.dataDir, outDir)
	}
	if err != nil {
		log.Printf("【autoprocess】创建 runner 失败: %v", err)
		return
	}
	defer runner.Close()

	err = runner.Run()
	if err != nil {
		log.Printf("【autoprocess】处理流水线失败: %v", err)
		return
	}

	state := runner.State()
	switch state {
	case process.StateDone:
		log.Printf("【autoprocess】处理完成，开始检测 todo 完成状态")
		completedIDs := ap.detectTodoCompletion()
		if ap.notifier != nil {
			stats := map[string]any{
				"state":         state,
				"completed_ids": completedIDs,
			}
			ap.notifier.Broadcast("process_complete", stats)
		}

	case process.StateNoChanges:
		log.Printf("【autoprocess】最近 3 天没有变更的日记，无需处理")
	case process.StateAlreadyProcessed:
		log.Printf("【autoprocess】同一批日记已经处理过，跳过")
	default:
		log.Printf("【autoprocess】处理结束，状态: %s", state)
	}
}

// detectTodoCompletion 读取所有日记内容，检测哪些 active todo 已完成。
func (ap *AutoProcess) detectTodoCompletion() []int {
	store, err := process.NewStore(ap.dbPath)
	if err != nil {
		log.Printf("【autoprocess】打开 store 失败: %v", err)
		return nil
	}
	defer store.Close()

	detector := NewTodoCompletionDetector(store)

	// 读取所有日记内容作为上下文
	s := storage.NewWithRoot(ap.dataDir)
	files, err := s.AllMarkdownFiles()
	if err != nil {
		log.Printf("【autoprocess】读取日记文件失败: %v", err)
		return nil
	}

	var content strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content.WriteString(string(b))
		content.WriteString("\n\n")
	}

	completedIDs, err := detector.Detect(content.String())
	if err != nil {
		log.Printf("【autoprocess】todo 完成检测失败: %v", err)
		return nil
	}

	log.Printf("【autoprocess】检测到 %d 个已完成 todo: %v", len(completedIDs), completedIDs)
	return completedIDs
}
