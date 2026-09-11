package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/yann0917/toolbox/internal/task"
)

type client struct {
	conn *websocket.Conn
	send chan []byte
}

// Hub 维护 WS 客户端并广播任务事件；Notify 作为 task.Engine 的回调。
type Hub struct {
	mu      sync.Mutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[*client]struct{}{}}
}

func (h *Hub) Notify(ev task.Event) {
	raw, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.broadcast(raw)
}

func (h *Hub) broadcast(raw []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- raw:
		default: // 慢客户端直接丢弃，避免阻塞
		}
	}
}

func (h *Hub) register(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	close(c.send)
}

func (h *Hub) writePump(c *client) {
	for raw := range c.send {
		_ = c.conn.WriteMessage(websocket.TextMessage, raw)
	}
}

func (h *Hub) serveWS(w http.ResponseWriter, r *http.Request, snapshotJSON func() []byte) {
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 64)}
	h.register(c)
	go h.writePump(c)
	// 连接建立即补发非终态任务快照，防漏消息
	if snapshotJSON != nil {
		if snap := snapshotJSON(); snap != nil {
			c.send <- snap
		}
	}
	// 读泵：仅处理关闭
	go func() {
		defer func() {
			h.unregister(c)
			_ = conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}
