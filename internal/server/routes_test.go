package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yann0917/toolbox/internal/service"
	"github.com/yann0917/toolbox/internal/store"
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
	// 最后注册 → 最先执行：先等后台任务落定，再关服务、删临时目录。
	// 任务在独立 goroutine 中执行，测试若先结束，随后写库会与 t.TempDir() 清理竞争
	//（表现为 "directory not empty" / "attempt to write a readonly database"，-race 下更易触发）。
	t.Cleanup(func() { waitTasksSettled(t, ts) })
	return ts, s
}

// waitTasksSettled 轮询任务列表，直到没有 pending/running 的任务或超时。
func waitTasksSettled(t *testing.T, ts *httptest.Server) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(ts.URL + "/api/tasks?size=100")
		if err != nil {
			return // 服务已不可达，交给后续清理步骤
		}
		var e struct {
			Data struct {
				Items []struct {
					Status string `json:"status"`
				} `json:"items"`
			} `json:"data"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&e)
		_ = resp.Body.Close()
		if decodeErr != nil {
			return
		}
		busy := false
		for _, it := range e.Data.Items {
			if it.Status == "pending" || it.Status == "running" {
				busy = true
				break
			}
		}
		if !busy {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Logf("等待任务落定超时（5s），可能仍有后台任务在写库")
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
	if len(tools) != 8 {
		t.Fatalf("tools = %v", e.Data)
	}
	// Registry().List() 基于 map 遍历，顺序不定：按 name 断言而非下标。
	found := map[string]bool{}
	for _, it := range tools {
		m, _ := it.(map[string]any)
		meta, _ := m["meta"].(map[string]any)
		if name, _ := meta["name"].(string); name != "" {
			found[name] = true
		}
	}
	if !found["tts"] || !found["asr"] || !found["podcast"] || !found["separate"] || !found["translate"] {
		t.Fatalf("tool names = %v", found)
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

// TestPutSettingsHotReload 保存凭证后立即 GET 应读到新值（热加载，无需重启）。
// SaveCredentials 写 $HOME/.toolbox/config.yaml，须隔离 HOME。
func TestPutSettingsHotReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ts, _ := newTestServer(t)

	body := `{"app_id":"app-1","access_token":"tok-1","api_key":"key-1","mediakit_api_key":"mk-1"}`
	req, err := http.NewRequest(http.MethodPut, ts.URL+"/api/settings", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var putE envelope
	_ = json.NewDecoder(resp.Body).Decode(&putE)
	if putE.Code != 0 {
		t.Fatalf("put code = %d (%s)", putE.Code, putE.Message)
	}

	got := getEnvelope(t, ts.URL+"/api/settings")
	gotData, _ := got.Data.(map[string]any)
	speech, _ := gotData["volc"].(map[string]any)["speech"].(map[string]any)
	if speech["app_id"] != "app-1" {
		t.Errorf("app_id = %v, want app-1（保存后应即时生效）", speech["app_id"])
	}
	if speech["api_key"] != "key-1" {
		t.Errorf("api_key = %v, want key-1", speech["api_key"])
	}
	mk, _ := gotData["volc"].(map[string]any)["mediakit"].(map[string]any)
	if mk["has_api_key"] != true {
		t.Errorf("mediakit.has_api_key = %v, want true", mk["has_api_key"])
	}
}

// TestSettingsTestConnection 连通性检测响应结构：顶层 ok/message 仍为语音探测结果
// （向后兼容，前端 SettingsPage 直接消费），新增 mediakit 段（独立 ok/message）。
// 测试环境无凭证：语音校验与 MediaKit 未配置检查均在发网络请求前返回，不会外联。
func TestSettingsTestConnection(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/settings/test-connection", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var e envelope
	_ = json.NewDecoder(resp.Body).Decode(&e)
	if e.Code != 0 {
		t.Fatalf("code = %d (%s)", e.Code, e.Message)
	}
	data, _ := e.Data.(map[string]any)
	if _, exists := data["ok"]; !exists {
		t.Errorf("data 缺少顶层 ok 键: %v", data)
	}
	if msg, _ := data["message"].(string); msg == "" {
		t.Errorf("data.message 应为非空字符串: %v", data)
	}
	mk, ok := data["mediakit"].(map[string]any)
	if !ok {
		t.Fatalf("data.mediakit 应为对象: %v", data)
	}
	if _, exists := mk["ok"]; !exists {
		t.Errorf("mediakit 缺少 ok 键: %v", mk)
	}
	if msg, _ := mk["message"].(string); msg == "" {
		t.Errorf("mediakit.message 应为非空字符串: %v", mk)
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

// TestTaskDetailSummaryExposed 详情 DTO 应透出任务 summary JSON（ASR segments 所在），
// 空 summary 的任务不得出现 summary 键（omitempty）。
func TestTaskDetailSummaryExposed(t *testing.T) {
	ts, s := newTestServer(t)
	summary := `{"segments":[{"text":"你好","start_ms":0,"end_ms":900}],"duration_ms":900,"source":"file"}`
	if err := s.svc.DB().CreateTask(&store.Task{
		ID: "t-sum", Provider: "volcengine", Tool: "asr", Status: store.StatusSucceeded,
		Params: `{}`, Summary: summary,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.svc.DB().CreateTask(&store.Task{
		ID: "t-nosum", Provider: "volcengine", Tool: "tts", Status: store.StatusFailed, Params: `{}`,
	}); err != nil {
		t.Fatal(err)
	}

	e := getEnvelope(t, ts.URL+"/api/tasks/t-sum")
	data, _ := e.Data.(map[string]any)
	task, _ := data["task"].(map[string]any)
	if task == nil {
		t.Fatalf("data = %v", e.Data)
	}
	sum, ok := task["summary"].(map[string]any)
	if !ok {
		t.Fatalf("task.summary 应为 JSON 对象: %v", task["summary"])
	}
	segs, _ := sum["segments"].([]any)
	if len(segs) != 1 {
		t.Fatalf("summary.segments = %v, want 1 条", sum["segments"])
	}
	seg0, _ := segs[0].(map[string]any)
	if seg0["text"] != "你好" || seg0["start_ms"].(float64) != 0 || seg0["end_ms"].(float64) != 900 {
		t.Errorf("segments[0] = %v", seg0)
	}

	e2 := getEnvelope(t, ts.URL+"/api/tasks/t-nosum")
	data2, _ := e2.Data.(map[string]any)
	task2, _ := data2["task"].(map[string]any)
	if task2 == nil {
		t.Fatalf("data = %v", e2.Data)
	}
	if _, exists := task2["summary"]; exists {
		t.Errorf("空 summary 不应出现 summary 键: %v", task2)
	}
}

// TestCreateTaskStripsOutParam 验证 Web 入口剥离 params 中的 _out（Task 9 安全修复）。
// _out 是 CLI 内部约定，透传会导致任意路径写文件。无凭证时任务会 failed，
// 但 params 落库不受影响，故以落库后的 params 为最稳断言。
func TestCreateTaskStripsOutParam(t *testing.T) {
	ts, _ := newTestServer(t)
	body := `{"provider":"volcengine","tool":"tts","params":{"text":"剥离测试","format":"mp3","_out":"/tmp/evil.mp3"}}`
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
	taskID, _ := createdData["task_id"].(string)
	if taskID == "" {
		t.Fatalf("created = %v", created)
	}

	e := getEnvelope(t, ts.URL+"/api/tasks/"+taskID)
	data, _ := e.Data.(map[string]any)
	task, _ := data["task"].(map[string]any)
	if task == nil {
		t.Fatalf("data = %v", e.Data)
	}
	params, _ := task["params"].(map[string]any)
	if params == nil {
		t.Fatalf("task.params = %v", task["params"])
	}
	if _, exists := params["_out"]; exists {
		t.Errorf("params 落库后仍含 _out: %v", params)
	}
	if params["text"] != "剥离测试" {
		t.Errorf("其余 params 应保留，got %v", params)
	}
}

// TestCreateTaskWithArtifactInput 跨工具联动：已有产物（如分离任务的人声轨）作为新任务输入。
// 手工造产物记录（db.CreateArtifact 指向 data 目录内真实文件）→ createTask 带 artifact_input
// → 任务创建成功。测试环境无凭证：任务最终 failed 不影响「创建成功」断言。
func TestCreateTaskWithArtifactInput(t *testing.T) {
	ts, s := newTestServer(t)
	dataDir := s.svc.Config().DataDir
	rel := filepath.Join("sep-test", "voice.mp3")
	if err := os.MkdirAll(filepath.Join(dataDir, filepath.Dir(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, rel), []byte("fake-audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.svc.DB().CreateArtifact(&store.Artifact{
		ID: "art-voice", TaskID: "t-src", Kind: "audio", Path: rel,
		Filename: "voice.mp3", Format: "mp3",
	}); err != nil {
		t.Fatal(err)
	}

	body := `{"provider":"volcengine","tool":"asr","params":{"srt":true},"artifact_input":"art-voice"}`
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
	taskID, _ := createdData["task_id"].(string)
	if taskID == "" {
		t.Fatalf("created = %v", created)
	}

	// 断言产物路径真的进了 files：无凭证时 ASR 的失败原因必须是「凭证未配置」，
	// 而不是「缺少输入」——后者说明 artifact → files["audio"] 的组装没生效。
	deadline := time.Now().Add(3 * time.Second)
	for {
		detail := getEnvelope(t, ts.URL+"/api/tasks/"+taskID)
		d, _ := detail.Data.(map[string]any)
		task, _ := d["task"].(map[string]any)
		status, _ := task["status"].(string)
		if status == "succeeded" || status == "failed" || status == "canceled" || status == "interrupted" {
			errMsg, _ := task["error"].(string)
			if !strings.Contains(errMsg, "凭证") {
				t.Errorf("任务失败原因 = %q，期望包含「凭证」（说明 artifact 未注入 files）", errMsg)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("任务未在 3s 内到达终态，status = %q", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestCreateTaskArtifactInputConflict artifact_input 与 file_ids 互斥：同传 → code 2，
// 且在产物存在性校验之前拒绝（不必存在真实产物）。
func TestCreateTaskArtifactInputConflict(t *testing.T) {
	ts, _ := newTestServer(t)
	body := `{"provider":"volcengine","tool":"asr","params":{"srt":true},"artifact_input":"art-x","file_ids":["f-1"]}`
	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var e envelope
	_ = json.NewDecoder(resp.Body).Decode(&e)
	if resp.StatusCode != 200 || e.Code != 2 {
		t.Errorf("status=%d code=%d message=%s", resp.StatusCode, e.Code, e.Message)
	}
	if !strings.Contains(e.Message, "只能提供其一") {
		t.Errorf("message 应说明互斥原因，got %q", e.Message)
	}
}

// TestCreateTaskArtifactNotFound artifact_input 指向不存在的产物 → code 6。
func TestCreateTaskArtifactNotFound(t *testing.T) {
	ts, _ := newTestServer(t)
	body := `{"provider":"volcengine","tool":"asr","params":{"srt":true},"artifact_input":"no-such-artifact"}`
	resp, err := http.Post(ts.URL+"/api/tasks", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var e envelope
	_ = json.NewDecoder(resp.Body).Decode(&e)
	if resp.StatusCode != 200 || e.Code != 6 {
		t.Errorf("status=%d code=%d message=%s", resp.StatusCode, e.Code, e.Message)
	}
}

// TestArtifactAbsPathJail 验证产物路径封闭在 data 目录内（Task 9 安全修复）。
func TestArtifactAbsPathJail(t *testing.T) {
	svc, err := service.NewWithHome(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	s := New(svc)

	// 相对路径穿越 data 目录：必须拒绝
	if abs, err := s.artifactAbsPath("../outside.txt"); err == nil {
		t.Fatalf("../outside.txt 应返回 error，got abs=%q", abs)
	}

	// data 目录内不存在的相对路径：返回 stat 错误（而非穿越错误），且不返回路径
	abs, err := s.artifactAbsPath("sub/ok.txt")
	if err == nil {
		t.Fatalf("sub/ok.txt 不存在，应返回 error，got %q", abs)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("应返回 stat not-exist 错误，got %v", err)
	}
	if abs != "" {
		t.Errorf("出错时不应返回路径，got %q", abs)
	}

	// 正例：data 目录内的真实文件可解析
	dataDir := svc.Config().DataDir
	if err := os.MkdirAll(filepath.Join(dataDir, "ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ok", "a.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, err = s.artifactAbsPath("ok/a.mp3")
	if err != nil {
		t.Fatalf("data 目录内文件应可解析，got %v", err)
	}
	if abs != filepath.Join(dataDir, "ok", "a.mp3") {
		t.Errorf("abs = %q, want %q", abs, filepath.Join(dataDir, "ok", "a.mp3"))
	}
}
