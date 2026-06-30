package watcher

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/borankux/dear-diary/internal/storage"
	"github.com/fsnotify/fsnotify"
)

// Notifier 是通知接口，用于解耦 watcher 与具体的通知实现。
type Notifier interface {
	Broadcast(eventType string, data any)
}

// Watcher 使用 fsnotify 监听日记文件系统变化。
// 它会对同一文件的变化进行 3 秒防抖，并首次启动时执行一次全量扫描。
type Watcher struct {
	dataDir   string
	notifier  Notifier
	fsWatcher *fsnotify.Watcher
	Events    chan string // 防抖后对外暴露的事件通道
	rawEvents chan string // 内部原始事件通道
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	running   bool
}

// New 创建一个新的 Watcher。
// dataDir 是日记根目录（如 /var/lib/dear-diary）。
func New(dataDir string, notifier Notifier) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		dataDir:   dataDir,
		notifier:  notifier,
		fsWatcher: w,
		Events:    make(chan string, 100),
		rawEvents: make(chan string, 100),
	}, nil
}

// Start 启动监听，阻塞直到 Stop 被调用。
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.running = true
	w.mu.Unlock()

	// 首次全量扫描，将现有日记文件推入事件队列
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.fullScan()
	}()

	// 递归添加 dataDir 及其所有子目录到 fsnotify
	if err := w.addWatchRecursive(w.dataDir); err != nil {
		w.stopInternal()
		return err
	}

	// 启动 fsnotify 事件读取循环
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.watchLoop()
	}()

	// 启动防抖处理循环
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.debounceLoop()
	}()

	// 阻塞直到上下文被取消
	<-w.ctx.Done()
	return nil
}

// Stop 优雅停止 Watcher，关闭所有 goroutine 和通道。
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	w.stopInternal()
	w.wg.Wait()
}

func (w *Watcher) stopInternal() {
	if w.cancel != nil {
		w.cancel()
	}
	w.fsWatcher.Close()
	w.mu.Lock()
	w.running = false
	w.mu.Unlock()
}

// addWatchRecursive 递归遍历 dir，将所有子目录添加到 fsnotify 监听。
func (w *Watcher) addWatchRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// 遇到无法访问的子目录时跳过，不中断整个流程
			return nil
		}
		if info.IsDir() {
			if err := w.fsWatcher.Add(path); err != nil {
				log.Printf("【watcher】添加目录监听失败 %s: %v", path, err)
			}
		}
		return nil
	})
}

// watchLoop 从 fsnotify 读取原始事件，过滤后推入 rawEvents。
func (w *Watcher) watchLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Printf("【watcher】文件系统监听错误: %v", err)
		}
	}
}

// handleEvent 过滤 fsnotify 事件，只保留符合条件的 .md 日记文件。
func (w *Watcher) handleEvent(event fsnotify.Event) {
	if event.Has(fsnotify.Create) {
		// 如果是新目录，自动追加监听（如用户创建新的 YYYY-MM 目录）
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := w.fsWatcher.Add(event.Name); err != nil {
				log.Printf("【watcher】追加新目录监听失败 %s: %v", event.Name, err)
			}
			return
		}
	}
	// 只关注写入和创建事件
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return
	}
	// 只关注 .md 文件
	if !strings.HasSuffix(event.Name, ".md") {
		return
	}
	// 只关注符合日记文件路径规范的（YYYY-MM/YYYY-MM-DD.md）
	if !storage.IsDiaryFilePath(event.Name) {
		return
	}
	select {
	case w.rawEvents <- event.Name:
	case <-w.ctx.Done():
	}
}

// debounceLoop 对同一文件的变化进行 3 秒防抖，防抖结束后将事件推入 Events 通道。
func (w *Watcher) debounceLoop() {
	defer close(w.Events)

	timers := make(map[string]*time.Timer)
	var mu sync.Mutex

	for {
		select {
		case <-w.ctx.Done():
			// 上下文取消时，立即触发所有待处理事件
			mu.Lock()
			for path, timer := range timers {
				timer.Stop()
				delete(timers, path)
				w.notifyFileChanged(path)
			}
			mu.Unlock()
			return
		case path, ok := <-w.rawEvents:
			if !ok {
				return
			}
			mu.Lock()
			if t, exists := timers[path]; exists {
				t.Stop()
			}
			timers[path] = time.AfterFunc(3*time.Second, func() {
				mu.Lock()
				delete(timers, path)
				mu.Unlock()
				w.notifyFileChanged(path)
			})
			mu.Unlock()
		}
	}
}

func (w *Watcher) notifyFileChanged(path string) {
	if w.notifier != nil {
		w.notifier.Broadcast("file_changed", path)
	}
	select {
	case w.Events <- path:
	case <-w.ctx.Done():
	}
}

// fullScan 扫描所有日记文件，将每个文件路径推入 rawEvents 进行首次处理。
func (w *Watcher) fullScan() {
	s := storage.NewWithRoot(w.dataDir)
	files, err := s.AllMarkdownFiles()
	if err != nil {
		log.Printf("【watcher】全量扫描失败: %v", err)
		return
	}
	for _, path := range files {
		select {
		case w.rawEvents <- path:
		case <-w.ctx.Done():
			return
		}
	}
	log.Printf("【watcher】首次全量扫描完成，发现 %d 个日记文件", len(files))
}
