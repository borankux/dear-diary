package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	stdsync "sync"
)

// Hub 管理所有 SSE 客户端连接
type Hub struct {
	mu        stdsync.RWMutex
	clients   map[chan sseEvent]struct{}
	broadcast chan sseEvent
	closed    bool
	done      chan struct{}
	wg        stdsync.WaitGroup
}

type sseEvent struct {
	eventType string
	data      any
}

// NewHub 创建一个新的 SSE Hub
func NewHub() *Hub {
	h := &Hub{
		clients:   make(map[chan sseEvent]struct{}),
		broadcast: make(chan sseEvent, 100),
		done:      make(chan struct{}),
	}
	h.wg.Add(1)
	go h.run()
	return h
}

func (h *Hub) run() {
	defer h.wg.Done()
	for {
		select {
		case evt := <-h.broadcast:
			h.mu.RLock()
			for ch := range h.clients {
				select {
				case ch <- evt:
				default:
					// 客户端缓冲满，丢弃事件
				}
			}
			h.mu.RUnlock()
		case <-h.done:
			return
		}
	}
}

// Subscribe 处理 SSE 订阅请求
func (h *Hub) Subscribe(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan sseEvent, 10)
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.clients[ch] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
	}()

	ctx := r.Context()
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt.data)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.eventType, string(data))
			flusher.Flush()
		case <-ctx.Done():
			return
		case <-h.done:
			return
		}
	}
}

// Broadcast 广播事件到所有客户端（异步，不阻塞发送方）
func (h *Hub) Broadcast(eventType string, data any) {
	select {
	case h.broadcast <- sseEvent{eventType: eventType, data: data}:
	default:
		// 广播缓冲满，丢弃事件
	}
}

// Close 关闭所有连接
func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	h.mu.Unlock()

	close(h.done)
	h.wg.Wait()

	h.mu.Lock()
	for ch := range h.clients {
		close(ch)
		delete(h.clients, ch)
	}
	h.mu.Unlock()
}
