package volcengine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/yann0917/toolbox/internal/provider"
)

// newASRToolWithMockWS 构造带 mock WS 服务端的 ASRTool（auc 指向不可达地址，URL 模式测试另行注入）。
func newASRToolWithMockWS(t *testing.T, audio []byte, format string) *ASRTool {
	t.Helper()
	wsURL := newMockASRServer(t, nil, func(t *testing.T, conn *websocket.Conn) {
		serveAckThenCollect(t, conn, audio, format, 32*1024)
	})
	cred := SpeechCred{APIKey: "key-1"}
	return &ASRTool{
		ws:     NewASRClientWithURL(cred, wsURL),
		auc:    NewASRAUCClientWithBaseURL(cred, "http://127.0.0.1:1"),
		cred:   cred,
		outDir: t.TempDir(),
	}
}

func writeTestAudio(t *testing.T, dir, name string, audio []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, audio, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestASRToolFileMode(t *testing.T) {
	testAudio := make([]byte, 100*1024) // 4 片：32KB×3 + 4KB 尾片
	for i := range testAudio {
		testAudio[i] = byte(i % 251)
	}
	tool := newASRToolWithMockWS(t, testAudio, "mp3")
	audioFile := writeTestAudio(t, t.TempDir(), "sample.mp3", testAudio)

	out, err := tool.Run(context.Background(), provider.TaskInput{
		Files:  map[string]string{"audio": audioFile},
		Params: map[string]any{},
	}, nopReport)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	if len(out.Artifacts) != 2 {
		t.Fatalf("artifacts 共 %d 个, 期望 2: %+v", len(out.Artifacts), out.Artifacts)
	}
	txtArt, srtArt := out.Artifacts[0], out.Artifacts[1]
	if txtArt.Kind != "transcript" || txtArt.Format != "txt" || !strings.HasPrefix(txtArt.Path, "asr/") {
		t.Errorf("artifacts[0] = %+v", txtArt)
	}
	if srtArt.Kind != "subtitle" || srtArt.Format != "srt" || !strings.HasSuffix(srtArt.Path, ".srt") {
		t.Errorf("artifacts[1] = %+v", srtArt)
	}
	txt, err := os.ReadFile(filepath.Join(tool.outDir, txtArt.Path))
	if err != nil {
		t.Fatal(err)
	}
	if string(txt) != "这是字节跳动，今日头条母公司。" {
		t.Errorf("txt = %q", txt)
	}
	srt, err := os.ReadFile(filepath.Join(tool.outDir, srtArt.Path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srt), "00:00:00,000 --> 00:00:01,705") {
		t.Errorf("srt 缺少时间轴: %s", srt)
	}
	segs, _ := out.Summary["segments"].([]map[string]any)
	if len(segs) != 2 {
		t.Errorf("summary.segments = %v", out.Summary["segments"])
	}
	if out.Summary["source"] != "file" || out.Summary["duration_ms"] != int64(3696) {
		t.Errorf("summary = %v", out.Summary)
	}
}

func TestASRToolURLMode(t *testing.T) {
	// 缩短轮询节奏，避免测试等待真实退避间隔。
	oldInterval, oldMax, oldTimeout := asrPollInterval, asrPollMax, asrPollTimeout
	asrPollInterval, asrPollMax, asrPollTimeout = 10*time.Millisecond, 20*time.Millisecond, 5*time.Second
	defer func() { asrPollInterval, asrPollMax, asrPollTimeout = oldInterval, oldMax, oldTimeout }()

	var mu sync.Mutex
	queries := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Api-Status-Code", asrAUCCodeOK)
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == asrAUCSubmitPath {
			return // submit ack：空 body
		}
		mu.Lock()
		queries++
		body := map[string]any{"id": "task-1", "status": "Running"}
		if queries >= 2 {
			body = map[string]any{
				"id": "task-1", "status": "Completed",
				"result": map[string]any{
					"text": "你好世界",
					"utterances": []any{
						map[string]any{"text": "你好", "start_time": 0, "end_time": 1000},
						map[string]any{"text": "世界", "start_time": 1000, "end_time": 2000},
					},
				},
				"audio_info": map[string]any{"duration": 2000},
			}
		}
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)

	cred := SpeechCred{APIKey: "key-1"}
	tool := &ASRTool{
		ws:     NewASRClientWithURL(cred, "ws://127.0.0.1:1"),
		auc:    NewASRAUCClientWithBaseURL(cred, srv.URL),
		cred:   cred,
		outDir: t.TempDir(),
	}
	out, err := tool.Run(context.Background(), provider.TaskInput{
		Params: map[string]any{"url": "https://example.com/audio.mp3"},
	}, nopReport)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	if queries < 2 {
		t.Errorf("query 次数 = %d, 期望至少 2 次（Running → Completed）", queries)
	}
	if len(out.Artifacts) != 2 || out.Artifacts[0].Kind != "transcript" || out.Artifacts[1].Kind != "subtitle" {
		t.Fatalf("artifacts = %+v", out.Artifacts)
	}
	if out.Summary["source"] != "url" || out.Summary["duration_ms"] != int64(2000) {
		t.Errorf("summary = %v", out.Summary)
	}
}

func TestASRToolNoInput(t *testing.T) {
	tool := newASRToolWithMockWS(t, []byte("audio"), "mp3")
	_, err := tool.Run(context.Background(), provider.TaskInput{Params: map[string]any{}}, nopReport)
	if err == nil || !strings.Contains(err.Error(), "缺少输入") {
		t.Fatalf("err = %v, 期望包含「缺少输入」", err)
	}
}

func TestASRToolBadFormat(t *testing.T) {
	tool := newASRToolWithMockWS(t, []byte("audio"), "mp3")
	audioFile := writeTestAudio(t, t.TempDir(), "song.m4a", []byte("fake-m4a"))
	_, err := tool.Run(context.Background(), provider.TaskInput{
		Files: map[string]string{"audio": audioFile},
	}, nopReport)
	if err == nil || !strings.Contains(err.Error(), "暂不支持") {
		t.Fatalf("err = %v, 期望包含「暂不支持」", err)
	}
}

func TestASRToolOutRedirect(t *testing.T) {
	testAudio := make([]byte, 100*1024)
	for i := range testAudio {
		testAudio[i] = byte(i % 251)
	}
	tool := newASRToolWithMockWS(t, testAudio, "mp3")
	audioFile := writeTestAudio(t, t.TempDir(), "sample.mp3", testAudio)
	in := func(outParam string) provider.TaskInput {
		return provider.TaskInput{
			Files:  map[string]string{"audio": audioFile},
			Params: map[string]any{"_out": outParam},
		}
	}

	// 1) 相对 _out：产物落 outDir 相对段下，artifact 路径保持相对。
	out, err := tool.Run(context.Background(), in(filepath.Join("custom", "result.txt")), nopReport)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	if out.Artifacts[0].Path != filepath.Join("custom", "result.txt") ||
		out.Artifacts[1].Path != filepath.Join("custom", "result.srt") {
		t.Fatalf("相对 _out artifact 路径 = %s, %s", out.Artifacts[0].Path, out.Artifacts[1].Path)
	}
	for _, p := range []string{out.Artifacts[0].Path, out.Artifacts[1].Path} {
		if _, err := os.Stat(filepath.Join(tool.outDir, p)); err != nil {
			t.Errorf("产物 %s 未落盘: %v", p, err)
		}
	}

	// 2) 绝对 _out：txt/srt 均为绝对路径，srt = txt 换扩展名。
	absTxt := filepath.Join(t.TempDir(), "out.txt")
	absSrt := strings.TrimSuffix(absTxt, ".txt") + ".srt"
	out, err = tool.Run(context.Background(), in(absTxt), nopReport)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	if out.Artifacts[0].Path != absTxt || out.Artifacts[1].Path != absSrt {
		t.Fatalf("绝对 _out artifact 路径 = %s, %s, 期望 %s, %s",
			out.Artifacts[0].Path, out.Artifacts[1].Path, absTxt, absSrt)
	}
	if _, err := os.Stat(absSrt); err != nil {
		t.Errorf("SRT %s 未落盘: %v", absSrt, err)
	}
}
