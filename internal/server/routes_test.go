package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yann0917/toolbox/internal/service"
)

// envelope 由 apierr.go 提供，测试直接复用生产包络类型。

func getEnvelope(t *testing.T, url string) envelope {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("HTTP status = %d, want 200 (envelope)", resp.StatusCode)
	}
	var e envelope
	_ = json.NewDecoder(resp.Body).Decode(&e)
	return e
}

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	svc, err := service.NewWithHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	s := New(svc)
	svc.StartEngine(s.Hub().Notify, 2)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

func TestHealth(t *testing.T) {
	ts, _ := newTestServer(t)
	e := getEnvelope(t, ts.URL+"/api/health")
	if e.Code != 0 {
		t.Fatalf("code = %d", e.Code)
	}
	data, _ := e.Data.(map[string]any)
	if data["status"] != "ok" {
		t.Errorf("data = %v", data)
	}
}

func TestToolsAndTaskSubmit(t *testing.T) {
	ts, _ := newTestServer(t)
	e := getEnvelope(t, ts.URL+"/api/tools")
	tools, _ := e.Data.([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", e.Data)
	}
	tool, _ := tools[0].(map[string]any)
	meta, _ := tool["meta"].(map[string]any)
	if meta["name"] != "tts" {
		t.Fatalf("tool meta = %v", meta)
	}

	body := `{"provider":"volcengine","tool":"tts","params":{"text":"缺凭证也入库","format":"mp3"}}`
	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var created envelope
	_ = json.NewDecoder(resp.Body).Decode(&created)
	if created.Code != 0 {
		t.Fatalf("submit code = %d (%s)", created.Code, created.Message)
	}
	createdData, _ := created.Data.(map[string]any)
	if createdData["task_id"] == "" {
		t.Errorf("created = %v", created)
	}

	// 任务列表
	list := getEnvelope(t, ts.URL+"/api/tasks")
	listData, _ := list.Data.(map[string]any)
	if listData["total"].(float64) != 1 {
		t.Errorf("list = %v", list)
	}
}

func TestErrorEnvelope(t *testing.T) {
	ts, _ := newTestServer(t)
	// 未知工具：code 2，HTTP 仍 200
	body := `{"provider":"volcengine","tool":"nope","params":{}}`
	resp, _ := http.Post(ts.URL+"/api/tasks", "application/json", strings.NewReader(body))
	var e envelope
	_ = json.NewDecoder(resp.Body).Decode(&e)
	if resp.StatusCode != 200 || e.Code != 2 {
		t.Errorf("status=%d code=%d message=%s", resp.StatusCode, e.Code, e.Message)
	}
	// 不存在的任务：code 6
	resp2, _ := http.Get(ts.URL + "/api/tasks/not-exist")
	var e2 envelope
	_ = json.NewDecoder(resp2.Body).Decode(&e2)
	if resp2.StatusCode != 200 || e2.Code != 6 {
		t.Errorf("status=%d code=%d", resp2.StatusCode, e2.Code)
	}
}
