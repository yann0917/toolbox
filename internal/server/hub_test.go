package server

import (
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yann0917/toolbox/internal/task"
)

func TestHubBroadcast(t *testing.T) {
	ts, s := newTestServer(t)
	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	s.Hub().Notify(task.Event{Type: "progress", TaskID: "x", Progress: 42, Note: "测试"})

	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	// 注：Progress 为 JSON 数字，序列化后不含引号，故断言 "42" 调整为 42。
	if !strings.Contains(string(msg), `"progress"`) || !strings.Contains(string(msg), "42") {
		t.Errorf("msg = %s", msg)
	}
}
