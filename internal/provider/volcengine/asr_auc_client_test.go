package volcengine

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// aucRequest 记录 mock 服务收到的请求信息（路径/头/body），供断言。
type aucRequest struct {
	path    string
	headers map[string]string
	body    map[string]any
}

// newAUCMockServer 构造异步 ASR mock 服务：记录请求头与 body，按 respHeader/respBody 回复
// （respBody 为 nil 时返回空 body，对应 submit 的成功响应）。
func newAUCMockServer(t *testing.T, respHeader map[string]string, respBody any, got *aucRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path = r.URL.Path
		got.headers = map[string]string{
			"X-Api-Key":         r.Header.Get("X-Api-Key"),
			"X-Api-App-Key":     r.Header.Get("X-Api-App-Key"),
			"X-Api-Access-Key":  r.Header.Get("X-Api-Access-Key"),
			"X-Api-Resource-Id": r.Header.Get("X-Api-Resource-Id"),
			"X-Api-Request-Id":  r.Header.Get("X-Api-Request-Id"),
			"X-Api-Sequence":    r.Header.Get("X-Api-Sequence"),
		}
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		got.body = m
		for k, v := range respHeader {
			w.Header().Set(k, v)
		}
		w.WriteHeader(http.StatusOK)
		if respBody == nil {
			return
		}
		if s, ok := respBody.(string); ok {
			_, _ = w.Write([]byte(s))
			return
		}
		_ = json.NewEncoder(w).Encode(respBody)
	}))
}

func TestSubmitSuccess(t *testing.T) {
	var got aucRequest
	srv := newAUCMockServer(t, map[string]string{"X-Api-Status-Code": "20000000"}, nil, &got)
	defer srv.Close()

	c := NewASRAUCClientWithBaseURL(SpeechCred{AppID: "app", AccessToken: "tok"}, srv.URL)
	taskID, err := c.Submit(context.Background(), "https://example.com/audio.mp3")
	if err != nil {
		t.Fatal(err)
	}
	// 官方语义：submit 的任务 ID 即客户端传入的 X-Api-Request-Id。
	if taskID == "" || taskID != got.headers["X-Api-Request-Id"] {
		t.Fatalf("taskID = %q, 请求头 X-Api-Request-Id = %q", taskID, got.headers["X-Api-Request-Id"])
	}
	if got.path != "/api/v3/auc/bigmodel/submit" {
		t.Errorf("path = %q", got.path)
	}
	if got.headers["X-Api-Resource-Id"] != "volc.seedasr.auc" {
		t.Errorf("X-Api-Resource-Id = %q", got.headers["X-Api-Resource-Id"])
	}
	if got.headers["X-Api-Sequence"] != "-1" {
		t.Errorf("X-Api-Sequence = %q", got.headers["X-Api-Sequence"])
	}
	if got.headers["X-Api-App-Key"] != "app" || got.headers["X-Api-Access-Key"] != "tok" {
		t.Errorf("老版鉴权头 = %v", got.headers)
	}
	if got.body["audio_url"] != "https://example.com/audio.mp3" {
		t.Errorf("body = %v", got.body)
	}
}

func TestSubmitAuthError(t *testing.T) {
	var got aucRequest
	srv := newAUCMockServer(t, map[string]string{"X-Api-Status-Code": "45000001"}, nil, &got)
	defer srv.Close()

	c := NewASRAUCClientWithBaseURL(SpeechCred{AppID: "a", AccessToken: "bad"}, srv.URL)
	_, err := c.Submit(context.Background(), "https://example.com/audio.mp3")
	if err == nil || !errors.Is(err, ErrAuth) {
		t.Fatalf("err = %v, want ErrAuth", err)
	}
	if !strings.Contains(err.Error(), "45000001") {
		t.Errorf("err 应包含状态码 45000001: %v", err)
	}
}

func TestQueryCompleted(t *testing.T) {
	var got aucRequest
	srv := newAUCMockServer(t, map[string]string{"X-Api-Status-Code": "20000000"}, map[string]any{
		"id":     "task-1",
		"status": "Completed",
		"result": map[string]any{
			"text": "你好世界",
			"utterances": []any{
				map[string]any{"text": "你好", "start_time": 0, "end_time": 1000},
				map[string]any{"text": "世界", "start_time": 1000, "end_time": 2000},
			},
		},
		"audio_info": map[string]any{"duration": 2000},
	}, &got)
	defer srv.Close()

	c := NewASRAUCClientWithBaseURL(SpeechCred{APIKey: "ak"}, srv.URL)
	resp, status, err := c.Query(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if status != "Completed" {
		t.Errorf("status = %q", status)
	}
	if resp.Text != "你好世界" || resp.DurationMS != 2000 {
		t.Fatalf("resp = %+v", resp)
	}
	if len(resp.Segments) != 2 ||
		resp.Segments[0].Text != "你好" || resp.Segments[0].StartMS != 0 || resp.Segments[0].EndMS != 1000 ||
		resp.Segments[1].Text != "世界" || resp.Segments[1].StartMS != 1000 || resp.Segments[1].EndMS != 2000 {
		t.Fatalf("segments = %+v", resp.Segments)
	}
	if got.path != "/api/v3/auc/bigmodel/query" {
		t.Errorf("path = %q", got.path)
	}
	if got.body["id"] != "task-1" {
		t.Errorf("body = %v", got.body)
	}
	// 新版鉴权：APIKey 非空时用 X-Api-Key，且 Query 不携带 X-Api-Sequence。
	if got.headers["X-Api-Key"] != "ak" {
		t.Errorf("X-Api-Key = %q", got.headers["X-Api-Key"])
	}
	if got.headers["X-Api-Sequence"] != "" {
		t.Errorf("Query 不应携带 X-Api-Sequence, got %q", got.headers["X-Api-Sequence"])
	}
}

func TestQueryStillRunning(t *testing.T) {
	var got aucRequest
	srv := newAUCMockServer(t, map[string]string{"X-Api-Status-Code": "20000000"}, map[string]any{
		"id":     "task-1",
		"status": "Running",
	}, &got)
	defer srv.Close()

	c := NewASRAUCClientWithBaseURL(SpeechCred{APIKey: "ak"}, srv.URL)
	resp, status, err := c.Query(context.Background(), "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if status != "Running" {
		t.Errorf("status = %q", status)
	}
	if resp.Text != "" || len(resp.Segments) != 0 {
		t.Errorf("Running 状态不应有识别结果: %+v", resp)
	}
}
